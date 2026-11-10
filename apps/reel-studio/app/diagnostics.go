package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// 诊断报告是用户遇到问题后交给我们看的唯一东西。
//
// 它必须同时回答三个问题，缺一个我们就得再问用户一轮：
//
// 1. 出的是什么事（哪条流水线、哪次任务、报错原文）
// 2. 出在什么环境（版本、系统、通道是否就绪）
// 3. 去哪看细节（日志文件路径、崩溃日志）
//
// 所以下面几块内容是按「维护者拿到就能查」的顺序排的，不是按代码结构排。

// CrashLogPath 返回崩溃日志路径。
//
// Go 进程未捕获的 panic 会写到这里（main.go 里设过 debug.SetCrashOutput），
// 主协程的 panic 还会额外由 dumpCrash 落一份完整调用栈。
func CrashLogPath() string {
	return filepath.Join(os.TempDir(), "reel-studio-crash.log")
}

// DiagnosticReport 生成完整的排障报告（已脱敏）。
//
// 这份文本同时用于三个出口：复制到剪贴板、导出成文件、附在反馈 Issue 里。
func (a *App) DiagnosticReport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "===== Reel Studio 排障报告 =====\n")
	fmt.Fprintf(&b, "生成时间：%s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "应用版本：%s\n", mygo.App.Version())
	fmt.Fprintf(&b, "系统：%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "Go 运行时：%s\n", runtime.Version())

	// 1. 工作台状态：起没起来、为什么没起来
	fmt.Fprintf(&b, "\n--- 工作台状态 ---\n")
	fmt.Fprintf(&b, "状态：%s\n", a.backendState.State)
	if a.backendState.Note != "" {
		fmt.Fprintf(&b, "说明：%s\n", redactDiagnostic(a.backendState.Note))
	}
	fmt.Fprintf(&b, "是否由本应用启动：%t\n", a.backendState.Started)

	// 2. 通道就绪：密钥没配好时用户常常以为「软件坏了」
	if a.providers != nil {
		fmt.Fprintf(&b, "\n--- 生成通道 ---\n")
		for _, group := range []struct {
			name  string
			items []studioapi.Channel
		}{
			{"视频", a.providers.Video.Channels},
			{"图片", a.providers.Image.Channels},
			{"配音", a.providers.Voice.Channels},
		} {
			for _, ch := range group.items {
				fmt.Fprintf(&b, "[%s] %s（%s）：%s\n", group.name, ch.Label, ch.ID, readyWord(ch.Ready))
				if ch.Note != "" {
					fmt.Fprintf(&b, "    说明：%s\n", redactDiagnostic(ch.Note))
				}
			}
		}
	}

	// 3. 最近失败的任务：报错原文与任务号
	if failures := a.recentFailures(6); len(failures) > 0 {
		fmt.Fprintf(&b, "\n--- 最近失败的任务（最多 6 条）---\n")
		for _, j := range failures {
			fmt.Fprintf(&b, "任务 %s｜流水线 %s｜状态 %s｜时间 %s\n",
				j.ID, jobKind(j), j.Status, jobCreatedAt(j))
			if j.Message != "" {
				fmt.Fprintf(&b, "    提示：%s\n", truncateRunes(redactDiagnostic(j.Message), 300))
			}
			if j.Error != "" {
				fmt.Fprintf(&b, "    报错：%s\n", truncateRunes(redactDiagnostic(j.Error), 800))
			}
			if step, detail := lastFailedStep(j); detail != "" {
				fmt.Fprintf(&b, "    失败步骤：%s｜%s\n", step, truncateRunes(redactDiagnostic(detail), 400))
			}
		}
	} else {
		fmt.Fprintf(&b, "\n--- 最近失败的任务：暂无失败记录 ---\n")
	}

	// 4. 日志文件在哪：让用户能直接把日志发过来
	fmt.Fprintf(&b, "\n--- 日志文件 ---\n")
	if path := a.LogFilePath(); path != "" {
		fmt.Fprintf(&b, "桌面端界面日志：%s\n", maskHomePath(path))
	}
	fmt.Fprintf(&b, "本地工作台日志：%s\n", maskHomePath(BackendLogPath()))
	fmt.Fprintf(&b, "崩溃日志：%s\n", maskHomePath(CrashLogPath()))

	// 5. 崩溃记录：启动就闪退时，这里是唯一线索
	if tail := redactDiagnostic(readTail(CrashLogPath(), 30)); strings.TrimSpace(tail) != "" {
		fmt.Fprintf(&b, "\n--- 崩溃记录（末尾 30 行）---\n%s\n", tail)
	}

	// 6. 工作台与引擎日志
	if tail := redactDiagnostic(readTail(BackendLogPath(), 40)); strings.TrimSpace(tail) != "" {
		fmt.Fprintf(&b, "\n--- 工作台与引擎近期日志（末尾 40 行）---\n%s\n", tail)
	} else {
		fmt.Fprintf(&b, "\n--- 工作台与引擎近期日志：文件暂未生成或为空 ---\n")
	}

	// 7. 界面日志
	if tail := redactDiagnostic(readTail(a.LogFilePath(), 25)); strings.TrimSpace(tail) != "" {
		fmt.Fprintf(&b, "\n--- 桌面端界面近期日志（末尾 25 行）---\n%s\n", tail)
	}

	fmt.Fprintf(&b, "\n（已尝试遮盖常见凭据和用户名；日志可能仍包含私密内容，分享前请检查。）\n")
	return b.String()
}

// recentFailures 返回最近失败的任务，按时间倒序。
func (a *App) recentFailures(limit int) []studioapi.Job {
	if a.state == nil {
		return nil
	}
	var out []studioapi.Job
	for _, j := range a.state.Jobs {
		if j.Status == "failed" || j.Status == "error" || j.Error != "" {
			out = append(out, j)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// jobKind 取任务的流水线类型。
func jobKind(j studioapi.Job) string {
	if kind, ok := j.Input["kind"].(string); ok && kind != "" {
		return kind
	}
	return "未知"
}

// jobCreatedAt 取任务的完整创建时间。
func jobCreatedAt(j studioapi.Job) string {
	if j.CreatedAt == "" {
		return "未知"
	}
	return j.CreatedAt
}

// lastFailedStep 找出任务里第一个失败的步骤。
func lastFailedStep(j studioapi.Job) (string, string) {
	for _, s := range j.Steps {
		if s.Error != "" {
			return s.Label, s.Error
		}
	}
	return "", ""
}

// readyWord 把就绪状态写成中文。
func readyWord(ok bool) string {
	if ok {
		return "就绪"
	}
	return "未就绪"
}

// ExportDiagnostics 把排障报告导出成一个文件。
//
// 报错日志动辄几十上百行，粘进 Issue 很难读；导出成文件后直接拖进 Issue
// 附件即可 —— 这也是 Windows 用户最省事的上报方式。
func (a *App) ExportDiagnostics() {
	content := a.DiagnosticReport()
	name := fmt.Sprintf("reel-studio-diagnostics-%s-%s.txt",
		mygo.App.Version(), time.Now().Format("20060102-150405"))
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: a.win, Title: "导出排障报告", DefaultPath: name,
		Filters: []mygo.FileFilter{{Name: "文本文件", Extensions: []string{"txt"}}},
	})
	if err != nil {
		a.feedback.submitted = "无法打开保存窗口：" + err.Error()
		a.redraw()
		return
	}
	if path == "" {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		a.feedback.submitted = "导出失败：" + err.Error()
		a.redraw()
		return
	}
	_ = mygo.Clipboard.Write(transfer.TextData(content))
	a.feedback.submitted = "排障报告已导出到 " + path + "（同时已复制到剪贴板）"
	a.logf("导出排障报告：%s", path)
	a.redraw()
	revealPath(path)
}

// 脱敏规则。
//
// 这些日志来自本地机器，里面混着用户自己的密钥、令牌、签名链接和家目录路径。
// 报告会被用户贴到公开 Issue 上，所以这些必须遮盖 ——
// 遮盖规则宁可多遮一点：遮多了只是少一条线索，漏了就是泄密。
var diagnosticRedactRules = []struct {
	pattern *regexp.Regexp
	replace string
}{
	// Authorization: Bearer xxx / bearer xxx
	{regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-~+/=]{8,}`), "${1}[已遮盖]"},
	// Authorization: xxx（没有 Bearer 前缀的写法）
	{regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer\s+)?[^\s"',}]{3,}`), "${1}[已遮盖]"},
	// 键值形式：api_key=xxx / apikey: xxx / "token": "xxx" / secret=xxx
	// 引号要一并吃掉：JSON 风格的 "token": "xxx" 里，键名和冒号之间还有一个引号。
	{regexp.MustCompile(`(?i)\b(api[-_ ]?key|apikey|access[-_ ]?key|api[-_ ]?secret|secret[-_ ]?key|channel[-_ ]?keys?|access[-_ ]?token|refresh[-_ ]?token|id[-_ ]?token|auth[-_ ]?token|token|secret|password|passwd|pwd|x[-_ ]api[-_ ]key)("?\s*[:=]\s*)"?[^\s"',}]{3,}`), "${1}${2}[已遮盖]"},
	// 常见前缀的密钥
	{regexp.MustCompile(`(?i)\b(sk-[A-Za-z0-9_\-]{6,}|sk-ant-[A-Za-z0-9_\-]{6,}|ak-[A-Za-z0-9_\-]{6,}|rk-[A-Za-z0-9_\-]{6,})`), "[已遮盖的密钥]"},
	// JWT 形态的令牌
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{4,}`), "[已遮盖的令牌]"},
	// 签名 URL 上的签名参数
	{regexp.MustCompile(`(?i)\b(x-amz-signature|x-amz-credential|x-amz-security-token|accesskeyid|signature|sig|expires|se)=[^&\s"']+`), "${1}=[已遮盖]"},
}

// 路径里的用户名也要遮盖：家目录路径等同于个人信息。
var (
	maskUnixHome = regexp.MustCompile(`(/Users/|/home/)[^/\s:"']+`)
	maskWinHome  = regexp.MustCompile(`([A-Za-z]:\\Users\\)[^\\\s:"']+`)
)

// redactDiagnostic 对一段文本做脱敏。
func redactDiagnostic(s string) string {
	if s == "" {
		return ""
	}
	out := s
	for _, rule := range diagnosticRedactRules {
		out = rule.pattern.ReplaceAllString(out, rule.replace)
	}
	out = maskUnixHome.ReplaceAllString(out, "${1}<用户>")
	out = maskWinHome.ReplaceAllString(out, "${1}<用户>")
	return out
}

// maskHomePath 遮盖路径里的用户名。
func maskHomePath(path string) string {
	if path == "" {
		return ""
	}
	out := maskUnixHome.ReplaceAllString(path, "${1}<用户>")
	return maskWinHome.ReplaceAllString(out, "${1}<用户>")
}
