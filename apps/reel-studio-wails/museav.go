package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// museav 中台客户端。
//
// 为什么走 CLI 而不是 HTTP：中台的鉴权、归档、计费都在 CLI 里做完了，
// 而且「提交即计费」这件事必须有任务 ID 落盘 —— CLI 已经在做这件事了，
// 自己再写一遍 HTTP 客户端等于重新实现一遍它的账本。
//
// 两条硬规矩：
//
//  1. 不静默切账户、不静默换模型。失败就把原因报回去，让用户自己决定。
//  2. 付费动作只在用户明确触发的那一步发生。探活只做只读调用。

// MuseavClient 调中台的客户端。
type MuseavClient struct {
	// Bin CLI 可执行文件，默认 museav。
	Bin string
	// Workspace 中台工作区 id 或名字（角色归属的工作区）。
	Workspace string
}

// museavBin 定位 museav CLI。
//
// 顺序与 mediaCommand 一致：先找随应用打包的那份，再找 PATH。
// 桌面版装机用户往往没装 museav —— 但应用包里有（prepare-resources.sh
// 会把 museav-cli 打进 Resources）。只查 PATH 的话，装机用户点「新建角色」
// 会得到「找不到 museav 命令」，而其实它就在应用包里。
func museavBin() string {
	if bin := strings.TrimSpace(os.Getenv("REEL_MUSEAV_BIN")); bin != "" {
		return bin
	}
	if path := bundledMuseav(); path != "" {
		return path
	}
	if path, err := exec.LookPath("museav"); err == nil {
		return path
	}
	return "museav"
}

// bundledMuseav 找应用包里自带的 museav 转发脚本。
//
// Windows 上装包放的是 museav.cmd —— 那是批处理，不能直接 exec，
// 所以 Windows 走 cmd /c 跑。macOS/Linux 上是 shell 脚本，直接跑。
func bundledMuseav() string {
	for _, name := range []string{"museav", "museav.cmd"} {
		path := bundledResource("bin", name)
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// run 跑一条 museav 命令。
//
// 不走 shell：参数原样传，提示词里的引号、换行、$ 都不会被二次解释。
// 出错时只回 stderr 尾部 —— argv 里可能有提示词和路径，不该整段回显。
func (m *MuseavClient) run(ctx context.Context, args ...string) (string, error) {
	bin := m.Bin
	if bin == "" {
		bin = museavBin()
	}
	// Windows 的 .cmd 是批处理，不能直接 exec —— 交给 cmd.exe 跑。
	// 用 exec.Command 而不是 CommandContext 的话取消就掐不断进程，
	// 所以两边都走 CommandContext。
	var cmd *exec.Cmd
	if strings.HasSuffix(strings.ToLower(bin), ".cmd") {
		cmd = exec.CommandContext(ctx, "cmd", append([]string{"/c", bin}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, bin, args...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", errors.New("已取消")
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("中台命令失败：%s", tail(msg, 400))
	}
	return stdout.String(), nil
}

// tail 取字符串尾部 n 个字符（按 rune，不切坏中文）。
func tail(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// Available 检查 museav 是否可用（只读）。
//
// 不发任何请求 —— 探活不该产生计费，也不该替用户登录。
func (m *MuseavClient) Available(ctx context.Context) error {
	bin := m.Bin
	if bin == "" {
		bin = museavBin()
	}
	if _, err := os.Stat(bin); err == nil {
		return nil
	}
	if _, err := exec.LookPath(bin); err != nil {
		return errors.New("没找到 museav 命令：请先安装中台 CLI 并登录（museav login）")
	}
	return nil
}

// ListSubjects 列中台主体。
func (m *MuseavClient) ListSubjects(ctx context.Context, workspace string) ([]Subject, error) {
	args := []string{"subjects", "list"}
	if workspace != "" {
		args = append(args, "--project", workspace)
	}
	out, err := m.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	// list 的 stdout 是「表格 + JSON」两段：表格给人看，JSON 给机器读。
	// 机器只认 JSON 那段 —— 表格里有中文和空行，直接 Unmarshal 会失败。
	return parseSubjectList(extractJSONArray(out))
}

// ShowSubject 拉一个主体的详情（含名下所有图）。
func (m *MuseavClient) ShowSubject(ctx context.Context, idOrName string) (*SubjectDetail, error) {
	args := []string{"subjects", "show"}
	if m.Workspace != "" {
		args = append(args, "--project", m.Workspace)
	}
	args = append(args, idOrName)
	out, err := m.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseSubjectDetail(extractJSONObject(out))
}

// CreateSubject 建一个主体（角色）。
//
// traits 走 JSON 对象，中台原样存。空 traits 不传 —— 传 "{}" 跟不传一样，
// 但会让调用方以为「已经传过了」。
func (m *MuseavClient) CreateSubject(ctx context.Context, name, kind, persona string, traits map[string]string) (*Subject, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("角色名不能为空")
	}
	if len([]rune(name)) > 60 {
		return nil, errors.New("角色名最长 60 字")
	}
	args := []string{"subjects", "create", "--name", name}
	if m.Workspace != "" {
		args = append(args, "--project", m.Workspace)
	}
	if kind != "" {
		args = append(args, "--kind", kind)
	}
	if persona == "real" || persona == "virtual" {
		args = append(args, "--persona", persona)
	}
	if len(traits) > 0 {
		payload, err := json.Marshal(traits)
		if err != nil {
			return nil, err
		}
		args = append(args, "--traits", string(payload))
	}
	out, err := m.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	// create 回执可能带 JSON，可能只有表格。两条路都试。
	if detail, err := parseSubjectDetail(extractJSONObject(out)); err == nil && detail.Subject.ID != "" {
		return &detail.Subject, nil
	}
	if list, err := parseSubjectList(extractJSONArray(out)); err == nil && len(list) > 0 {
		return &list[0], nil
	}
	return nil, errors.New("中台已受理建主体，但回执里没读到主体 ID")
}

// AddSubjectAsset 把一张图登记到主体名下。
//
// file 是本地文件路径；url 是中台直链（免重复上传）。
// view 决定中台 role：正位三个有专属 role，两个 45° 落 raw 并靠名字归位。
func (m *MuseavClient) AddSubjectAsset(ctx context.Context, subjectID string, view CharacterView, name, file, url string) error {
	role := view.subjectRole()
	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = CharacterViewLabels[view]
	}
	args := []string{"subjects", "add-asset"}
	if m.Workspace != "" {
		args = append(args, "--project", m.Workspace)
	}
	args = append(args, subjectID, "--role", role, "--name", displayName)
	if url != "" {
		args = append(args, "--url", url)
	} else if file != "" {
		args = append(args, file)
	} else {
		return errors.New("没有可登记的图")
	}
	_, err := m.run(ctx, args...)
	return err
}

// GenerateImage 用中台出一张图，返回直链。
//
// reference 是参考图本地路径（--ref）。角色补全五视图时以基准人脸为参考，
// 这就是「五视图同源」的第一道保险。
func (m *MuseavClient) GenerateImage(ctx context.Context, prompt, ratio, referencePath, model string) (string, error) {
	args := []string{"gen", "--prompt", prompt}
	if ratio != "" {
		args = append(args, "--ratio", ratio)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if referencePath != "" {
		args = append(args, "--ref", referencePath)
	}
	if m.Workspace != "" {
		args = append(args, "--project", m.Workspace)
	}
	out, err := m.run(ctx, args...)
	if err != nil {
		return "", err
	}
	url := museavOutputURL(out)
	if url == "" {
		return "", errors.New("中台出图成功但没读到结果地址")
	}
	return url, nil
}

// museavOutputURL 从命令输出里取结果地址。
func museavOutputURL(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return ""
}

// extractJSONArray 从混合输出里抠出 JSON 数组。
//
// museav 的 list 先打给人看的表格，再打 JSON。表格里有中文和空行，
// 直接整体 Unmarshal 会失败，所以只取第一个 [ 到最后一个 ] 之间的内容。
func extractJSONArray(out string) []byte {
	start := strings.Index(out, "[")
	end := strings.LastIndex(out, "]")
	if start < 0 || end <= start {
		return []byte("[]")
	}
	return []byte(out[start : end+1])
}

// extractJSONObject 从混合输出里抠出 JSON 对象。
func extractJSONObject(out string) []byte {
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start < 0 || end <= start {
		return []byte("{}")
	}
	return []byte(out[start : end+1])
}

// DownloadImage 把中台直链的图片下载到内存。
//
// 刻意不落盘到角色专属目录：素材有现成的登记机制（AddAsset），它会写进
// 媒体目录、给界面一个 /media/<id> 的本地 URL、并记下来源直链。
// 自己另写一套落盘逻辑等于造第二份素材账本。
func (m *MuseavClient) DownloadImage(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("下载角色图失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("下载角色图失败：HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	return data, http.DetectContentType(data), nil
}
