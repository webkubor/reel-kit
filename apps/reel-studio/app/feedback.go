package app

import (
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

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
// 自动带上系统版本、通道连接状态，以及工作台与桌面端最近的日志片段，
// 并自动对 API 密钥等敏感字段进行脱敏，绝不包含用户私有素材路径与密钥明文。
func (a *App) Diagnostics() string {
	var b strings.Builder
	fmt.Fprintf(&b, "版本：%s\n", mygo.App.Version())
	fmt.Fprintf(&b, "系统：%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "Go：%s\n", runtime.Version())
	fmt.Fprintf(&b, "时间：%s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "工作台状态：%s\n", a.backendState.State)
	if a.backendState.Note != "" {
		fmt.Fprintf(&b, "工作台说明：%s\n", a.backendState.Note)
	}
	if a.providers != nil {
		fmt.Fprintf(&b, "视频通道就绪：%t\n", a.providers.Video.Ready)
		fmt.Fprintf(&b, "图片通道就绪：%t\n", a.providers.Image.Ready)
		fmt.Fprintf(&b, "配音通道就绪：%t\n", a.providers.Voice.Ready)
	}
	if path := a.LogFilePath(); path != "" {
		fmt.Fprintf(&b, "桌面端日志文件：%s\n", path)
	}
	backendPath := BackendLogPath()
	fmt.Fprintf(&b, "工作台日志文件：%s\n", backendPath)

	// 1. 本地工作台与引擎日志（Node / FFmpeg / 离线语音等所有真实错误都在这里）
	backendTail := sanitizeDiagnosticLog(readTail(backendPath, 35))
	if strings.TrimSpace(backendTail) != "" {
		fmt.Fprintf(&b, "\n--- 工作台与引擎近期日志（末尾 35 行）---\n%s\n", backendTail)
	} else {
		fmt.Fprintf(&b, "\n--- 工作台与引擎近期日志：文件暂未生成或为空 ---\n")
	}

	// 2. 桌面端应用日志
	appTail := sanitizeDiagnosticLog(readTail(a.LogFilePath(), 20))
	if strings.TrimSpace(appTail) != "" {
		fmt.Fprintf(&b, "\n--- 桌面端界面近期日志（末尾 20 行）---\n%s\n", appTail)
	}

	return b.String()
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
		body += "\n---\n\n### 诊断信息\n\n```\n" + a.Diagnostics() + "```\n"
	}

	clipErr := mygo.Clipboard.Write(transfer.TextData(body))

	// URL 长度安全截断，防止浏览器拒绝超长 GET URL
	urlBody := body
	if len(urlBody) > 2500 {
		urlBody = urlBody[:2500] + "\n\n...（诊断已超长，完整内容已复制到剪贴板，可直接粘贴覆盖）\n"
	}

	issueURL := fmt.Sprintf(
		"https://github.com/%s/issues/new?title=%s&body=%s&labels=%s",
		feedbackRepo,
		url.QueryEscape(fmt.Sprintf("[%s] %s", kindLabel, title)),
		url.QueryEscape(urlBody),
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
		a.kit.Field(c, "诊断信息", "勾上会把版本、系统与最近排障日志（已自动脱敏）一并带上，省去来回索要日志", func() {
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

// sanitizeDiagnosticLog 对诊断日志进行脱敏处理，遮蔽包含密钥、令牌的行。
func sanitizeDiagnosticLog(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "bearer ") ||
			strings.Contains(lower, "authorization:") ||
			strings.Contains(lower, "api-key") ||
			strings.Contains(lower, "apikey") ||
			strings.Contains(lower, "channelkeys") ||
			strings.Contains(lower, "sk-") {
			lines[i] = "[脱敏处理：该行含敏感凭据信息已自动遮盖]"
		}
	}
	return strings.Join(lines, "\n")
}

// logTail 读桌面端日志末尾若干行，给界面预览用。
func (a *App) logTail(lines int) string {
	return readTail(a.LogFilePath(), lines)
}

// BackendLogTail 读工作台日志末尾若干行，给界面预览用。
func (a *App) BackendLogTail(lines int) string {
	return readTail(BackendLogPath(), lines)
}
