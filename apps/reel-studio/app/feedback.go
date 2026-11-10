package app

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// 反馈类型。
const (
	FeedbackBug     = "bug"
	FeedbackFeature = "feature"
)

// Feedback 是用户写的一条反馈。
type Feedback struct {
	Kind    string
	Title   string
	Detail  string
	Include bool // 是否附带诊断信息
}

// feedback 状态。
type feedbackState struct {
	kind    string
	title   string
	detail  string
	include bool
	// submitted 是最近一次提交的结果，给界面显示。
	submitted string
}

// 反馈仓库地址。
const (
	feedbackRepo  = "webkubor/reel-kit"
	feedbackLabel = "app-feedback"
)

// Diagnostics 收集排障需要的环境信息与最近报错日志。
//
// 自动带上系统版本、通道连接状态、失败任务的报错原文，以及工作台与桌面端
// 最近的日志片段，并自动遮盖密钥、令牌、签名 URL 与家目录用户名。
func (a *App) Diagnostics() string {
	return a.DiagnosticReport()
}

// SubmitFeedback 生成反馈内容并复制到剪贴板，同时给出 GitHub Issue 链接。
//
// 不直接调 GitHub API —— 那要在 App 里塞一个令牌，代价与风险都不划算。
// 改成把内容准备好放进剪贴板，再打开 Issue 页面，用户自己确认后提交。
func (a *App) SubmitFeedback() {
	fb := a.feedback
	title := strings.TrimSpace(fb.title)
	detail := strings.TrimSpace(fb.detail)
	if title == "" {
		a.feedback.submitted = "先写一句标题"
		a.redraw()
		return
	}
	kindLabel := "需求建议"
	if fb.kind == FeedbackBug {
		kindLabel = "问题反馈"
	}
	body := fmt.Sprintf("## %s\n\n%s\n", kindLabel, detail)
	if fb.include {
		body += "\n---\n\n### 诊断信息\n\n```\n" + a.DiagnosticReport() + "```\n"
	}

	clipErr := mygo.Clipboard.Write(transfer.TextData(body))

	// Issue 的 URL 里只放短摘要：完整日志可能有上百 KB，塞进查询串浏览器
	// 会直接拒绝，而且日志进了 URL 也会留在浏览器历史里。完整内容放在
	// 剪贴板（用户自己粘贴），需要存档时在设置里导出成文件附在 Issue 上。
	summary := fmt.Sprintf("## %s\n\n%s\n", kindLabel, detail)
	if fb.include {
		summary += fmt.Sprintf("\n> 版本 %s｜%s/%s\n> 完整诊断信息已复制到剪贴板，可直接粘贴到本 Issue 正文；\n> 也可在「设置 → 运行日志 → 导出排障报告」导出文件后作为附件上传。\n",
			mygo.App.Version(), runtime.GOOS, runtime.GOARCH)
	}
	issueURL := fmt.Sprintf(
		"https://github.com/%s/issues/new?title=%s&body=%s&labels=%s",
		feedbackRepo,
		url.QueryEscape(fmt.Sprintf("[%s] %s", kindLabel, title)),
		url.QueryEscape(summary),
		url.QueryEscape(feedbackLabel),
	)
	if err := openURL(issueURL); err != nil {
		if clipErr != nil {
			a.feedback.submitted = "打开浏览器失败：" + err.Error()
		} else {
			a.feedback.submitted = "内容已复制到剪贴板，但打开浏览器失败：" + err.Error()
		}
	} else {
		if clipErr != nil {
			a.feedback.submitted = "已在浏览器打开 Issue 页面"
		} else {
			a.feedback.submitted = "内容已复制到剪贴板，在浏览器确认后即可提交"
		}
	}
	a.logf("准备提交反馈（类型：%s）", kindLabel)
	a.redraw()
}

// openURL 用系统默认浏览器打开链接。
func openURL(raw string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{raw}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", raw}
	default:
		cmd = "xdg-open"
		args = []string{raw}
	}
	return startDetached(cmd, args...)
}

// CopyDiagnostics 把诊断信息复制到剪贴板。
func (a *App) CopyDiagnostics() {
	_ = mygo.Clipboard.Write(transfer.TextData(a.Diagnostics()))
	a.feedback.submitted = "诊断信息已复制到剪贴板"
	a.redraw()
}

// viewFeedback 画反馈面板：报告问题、提需求、看版本。
func (a *App) viewFeedback(c *ui.Context) {
	a.kit.SectionTitle(c, "反馈与诊断", "问题和建议都欢迎，附上诊断信息能省一轮来回", nil)
	a.kit.Card(c, func() {
		ui.Row(c).Gap(design.SpaceSM).Children(func() {
			for _, item := range []struct{ kind, label string }{
				{FeedbackBug, "报告问题"},
				{FeedbackFeature, "提需求"},
			} {
				kind := item.kind
				label := item.label
				variant := kit.ButtonSecondary
				if a.feedback.kind == kind {
					variant = kit.ButtonPrimary
				}
				button := kitButton(a.kit, c, variant, label, false)
				if button.Clicked() {
					a.feedback.kind = kind
					a.feedback.submitted = ""
				}
			}
		})
		a.kit.Field(c, "标题", "一句话说清是什么问题或想要什么", func() {
			a.kit.Input(c, &a.feedback.title, "比如：读取仓库时提示 404")
		})
		a.kit.Field(c, "详情", "发生了什么、期望是什么", func() {
			a.kit.TextArea(c, &a.feedback.detail, "尽量写清楚复现步骤", 96)
		})
		a.kit.Field(c, "诊断信息", "勾上会把版本、系统、失败任务的报错原文与最近排障日志（已自动脱敏）一并带上，省去来回索要日志", func() {
			a.kit.Checkbox(c, &a.feedback.include, "附上诊断信息与排障日志")
		})
		ui.Row(c).Gap(design.SpaceSM).Children(func() {
			submit := kitButton(a.kit, c, kit.ButtonPrimary, "准备反馈并打开 Issue", a.feedback.title == "")
			if submit.Clicked() {
				a.SubmitFeedback()
			}
			diag := kitButton(a.kit, c, kit.ButtonSecondary, "复制诊断信息", false)
			if diag.Clicked() {
				a.CopyDiagnostics()
			}
			exportDiag := kitButton(a.kit, c, kit.ButtonSecondary, "导出排障报告", false)
			if exportDiag.Clicked() {
				go a.ExportDiagnostics()
			}
		})
		if a.feedback.submitted != "" {
			a.kit.Notice(c, a.feedback.submitted, "")
		}
	})
}

// readTail 读指定文件末尾若干行。
func readTail(path string, lines int) string {
	if path == "" {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return ""
	}
	const maxRead = 64 * 1024
	offset := int64(0)
	size := info.Size()
	if size > maxRead {
		offset = size - maxRead
	}
	buf := make([]byte, size-offset)
	_, err = file.ReadAt(buf, offset)
	if err != nil {
		return ""
	}
	all := strings.Split(strings.TrimRight(string(buf), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n")
}

// sanitizeDiagnosticLog 对诊断日志进行脱敏处理。
//
// 旧实现是「整行替换」：日志里只要出现 api-key 就丢掉整行，连上下文也没了。
// 现在改成按值遮盖（见 diagnostics.go 的 redactDiagnostic），既不泄密也留线索。
func sanitizeDiagnosticLog(s string) string {
	return redactDiagnostic(s)
}

// logTail 读桌面端日志末尾若干行，给界面预览用。
func (a *App) logTail(lines int) string {
	return redactDiagnostic(readTail(a.LogFilePath(), lines))
}

// BackendLogTail 读工作台日志末尾若干行，给界面预览用。
func (a *App) BackendLogTail(lines int) string {
	return redactDiagnostic(readTail(BackendLogPath(), lines))
}
