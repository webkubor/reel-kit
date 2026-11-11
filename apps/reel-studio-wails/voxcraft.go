package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// VoxcraftHome 候选安装位置（按完整度排序，最优先的在前）。
var VoxcraftHomes = []string{
	os.Getenv("VOXCRAFT_HOME"),
	filepath.Join(homeDir(), "dev/video/voxflow"),
	filepath.Join(homeDir(), "dev/github/app/voxflow"),
	filepath.Join(homeDir(), "dev/github/webkubor/voxflow"),
	filepath.Join(homeDir(), "dev/github/voxflow"),
	filepath.Join(homeDir(), "voxflow"),
	filepath.Join(homeDir(), "dev/github/app/voxcraft"),
	filepath.Join(homeDir(), "dev/github/webkubor/voxcraft"),
	filepath.Join(homeDir(), "dev/github/voxcraft"),
	filepath.Join(homeDir(), "voxcraft"),
}

const VoxcraftPort = 8866

// VoxcraftInfo 定位结果。
type VoxcraftInfo struct {
	OK       bool     `json:"ok"`
	Bin      string   `json:"bin"`
	Home     string   `json:"home"`
	HasModel bool     `json:"hasModel"`
	Personas []string `json:"personas"`
	Reason   string   `json:"reason"`
}

// LocateVoxcraft 定位本机的 voxcraft 安装，按完整度打分挑最优的一份。
func LocateVoxcraft() VoxcraftInfo {
	// PATH 里有就直接用
	if pathBin, err := exec.LookPath("voice"); err == nil {
		return VoxcraftInfo{OK: true, Bin: pathBin, HasModel: true}
	}

	var found []*candInfo
	seen := map[string]bool{}

	for _, h := range VoxcraftHomes {
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		info := inspectVoxcraft(h)
		if info == nil {
			continue
		}
		found = append(found, info)
	}

	if len(found) == 0 {
		return VoxcraftInfo{
			OK:     false,
			Reason: "本机找不到 voxcraft。它是 Python 项目，需先 clone 并跑 install.sh",
		}
	}

	// 按分数降序
	for i := 0; i < len(found); i++ {
		for j := i + 1; j < len(found); j++ {
			if found[j].score > found[i].score {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	best := found[0]
	if !best.hasModel {
		return VoxcraftInfo{
			OK:     false,
			Home:   best.home,
			Reason: best.home + " 已装依赖但缺 Base-1.7B 模型（4.2GB）",
		}
	}
	if best.bin == "" {
		return VoxcraftInfo{
			OK:     false,
			Home:   best.home,
			Reason: best.home + " 没有 .venv —— 依赖未安装",
		}
	}
	return VoxcraftInfo{OK: true, Bin: best.bin, Home: best.home, HasModel: true, Personas: best.personas}
}

func inspectVoxcraft(home string) *candInfo {
	if !existsFile(filepath.Join(home, "pyproject.toml")) {
		return nil
	}
	venvBin := filepath.Join(home, ".venv/bin/voice")
	bin := ""
	if existsFile(venvBin) {
		bin = venvBin
	}

	hasModel := existsFile(filepath.Join(homeDir(), ".voxflow/models-mlx/Base-1.7B-8bit")) ||
		existsFile(filepath.Join(home, "models/Base-1.7B"))

	var personas []string
	pf := filepath.Join(homeDir(), ".voxflow/configs/personas.json")
	if !existsFile(pf) {
		pf = filepath.Join(home, "configs/personas.json")
	}
	if data, err := os.ReadFile(pf); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(data, &m) == nil {
			for k := range m {
				personas = append(personas, k)
			}
		}
	}

	score := 0
	if bin != "" {
		score += 4
	}
	if hasModel {
		score += 2
	}
	if len(personas) > 0 {
		score += 1
	}
	return &candInfo{home: home, bin: bin, hasModel: hasModel, personas: personas, score: score}
}

type candInfo struct {
	home     string
	bin      string
	hasModel bool
	personas []string
	score    int
}

// VoxcraftServer 管理 voxcraft web 服务进程。
type VoxcraftServer struct {
	Proc   *exec.Cmd
	Base   string
	Reused bool
}

// EnsureServer 确保 voxcraft web 服务可用。别人开着就复用，不重复起。
func EnsureServer(home, bin string) (*VoxcraftServer, error) {
	base := fmt.Sprintf("http://127.0.0.1:%d", VoxcraftPort)

	if probeVoxcraft(base) {
		return &VoxcraftServer{Base: base, Reused: true}, nil
	}

	cmd := exec.Command(bin, "web", "--port", strconv.Itoa(VoxcraftPort))
	cmd.Dir = home
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 voxcraft 服务失败: %w", err)
	}

	deadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(deadline) {
		if probeVoxcraft(base) {
			return &VoxcraftServer{Proc: cmd, Base: base, Reused: false}, nil
		}
		time.Sleep(1 * time.Second)
	}
	cmd.Process.Kill()
	return nil, fmt.Errorf("voxcraft 服务 180s 内未就绪")
}

func StopServer(s *VoxcraftServer) {
	if s == nil || s.Proc == nil || s.Reused {
		return
	}
	s.Proc.Process.Kill()
}

// VoxcraftTask 服务返回的任务。
type VoxcraftTask struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Result   struct {
		OK       bool   `json:"ok"`
		Filename string `json:"filename"`
	} `json:"result"`
	Error string `json:"error"`
}

// SynthesizeViaServer 用服务模式批量合成，返回与 texts 等长的本地 wav 路径。
func SynthesizeViaServer(base, persona string, texts []string, outDir string) ([]string, error) {
	var files []string
	for i, text := range texts {
		filename, err := cloneOne(base, persona, text)
		if err != nil {
			return nil, fmt.Errorf("第 %d 句合成失败: %w", i+1, err)
		}
		out := filepath.Join(outDir, fmt.Sprintf("voice_%04d.wav", i))
		if err := downloadAudio(base, filename, out); err != nil {
			return nil, fmt.Errorf("第 %d 句下载失败: %w", i+1, err)
		}
		files = append(files, out)
	}
	return files, nil
}

func cloneOne(base, persona, text string) (string, error) {
	body, _ := json.Marshal(map[string]string{"persona": persona, "text": text})
	req, err := http.NewRequest("POST", base+"/api/clone", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("提交失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		resp.Body.Read(make([]byte, 200))
		return "", fmt.Errorf("提交失败 HTTP %d", resp.StatusCode)
	}
	var res struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.TaskID == "" {
		return "", fmt.Errorf("服务未返回 task_id")
	}

	deadline := time.Now().Add(180 * time.Second)
	pollMs := 900
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(pollMs) * time.Millisecond)
		tasks, err := fetchTasks(base)
		if err != nil {
			continue
		}
		for _, t := range tasks {
			if t.ID != res.TaskID {
				continue
			}
			switch t.Status {
			case "done":
				if t.Result.Filename == "" {
					return "", fmt.Errorf("任务完成但 result 里没有 filename")
				}
				return t.Result.Filename, nil
			case "error":
				return "", fmt.Errorf("%s", t.Error)
			case "cancelled":
				return "", fmt.Errorf("任务被取消")
			}
		}
	}
	return "", fmt.Errorf("任务 %s 超时未完成", res.TaskID)
}

func fetchTasks(base string) ([]VoxcraftTask, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequest("GET", base+"/api/tasks", nil)
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil
	}
	var res struct {
		Tasks []VoxcraftTask `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Tasks, nil
}

func downloadAudio(base, filename, out string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequest("GET", base+"/api/audio/"+urlEncode(filename), nil)
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("下载音频失败 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(out, data, 0o644)
}

func urlEncode(s string) string {
	// 简单的 URL 编码，足够当前用法（文件名由服务生成，通常安全）
	var b strings.Builder
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteRune(c)
		} else {
			b.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return b.String()
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func existsFile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func probeVoxcraft(base string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequest("GET", base+"/api/status", nil)
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	var res struct {
		BaseModel bool `json:"base_model"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return res.BaseModel
}
