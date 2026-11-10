package app

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 排障报告是用户和开发者之间唯一的通道：
// 报告里必须有「哪条任务、报了什么」，否则用户只能再被问一轮。
func Test排障报告包含失败任务的报错原文(t *testing.T) {
	server := stub(t, []map[string]any{{
		"id":        "j_bad",
		"status":    "failed",
		"error":    "upstream 401: invalid api key",
		"message":  "生成失败",
		"createdAt": "2026-11-10T10:00:00Z",
		"input":    map[string]any{"kind": "talk"},
		"steps": []map[string]any{{
			"id": "s1", "label": "生成配音", "status": "failed", "error": "ffmpeg exited with code 1",
		}},
	}})
	a := newApp(t, server.URL)
	report := a.Diagnostics()
	for _, want := range []string{"j_bad", "upstream 401", "ffmpeg exited with code 1", "生成配音"} {
		if !strings.Contains(report, want) {
			t.Errorf("排障报告里缺少 %q：\n%s", want, report)
		}
	}
}

// 脱敏是硬要求：报告会贴到公开 Issue 上，
// 密钥、令牌、签名 URL 与家目录用户名都不能出现在里面。
func Test排障报告脱敏密钥与路径(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		// 不能出现的原文
		forbidden []string
		// 应该保留的上下文
		keep []string
	}{
		{
			name:      "Bearer 令牌",
			raw:       "GET /api/v1 401 Authorization: Bearer abcdef1234567890",
			forbidden: []string{"abcdef1234567890"},
			keep:      []string{"401", "Authorization"},
		},
		{
			name:      "api_key 键值",
			raw:       "call failed api_key=sk-live-9999abcd",
			forbidden: []string{"sk-live-9999abcd"},
			keep:      []string{"call failed"},
		},
		{
			name:      "token 键值",
			raw:       "{\"token\": \"tok_ABCDEFGH12345678\"}",
			forbidden: []string{"tok_ABCDEFGH12345678"},
			keep:      []string{"token"},
		},
		{
			name:      "JWT",
			raw:       "using eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			forbidden: []string{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			keep:      []string{"using"},
		},
		{
			name:      "签名 URL",
			raw:       "GET https://manager.museav.top/api/jobs?id=1&X-Amz-Signature=deadbeefcafe0123",
			forbidden: []string{"deadbeefcafe0123"},
			keep:      []string{"manager.museav.top"},
		},
		{
			name:      "家目录用户名",
			raw:       "打开 /Users/alice/Library/Logs/Reel Studio/reel-studio.log 失败",
			forbidden: []string{"alice"},
			keep:      []string{"/Users/<用户>/Library"},
		},
		{
			name:      "Windows 家目录用户名",
			raw:       `打开 C:\Users\bob\AppData\Local\Temp\reel-studio.log 失败`,
			forbidden: []string{"bob"},
			keep:      []string{`C:\Users\<用户>\AppData`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redactDiagnostic(c.raw)
			for _, bad := range c.forbidden {
				if strings.Contains(got, bad) {
					t.Errorf("脱敏后仍出现敏感内容 %q：%s", bad, got)
				}
			}
			for _, keep := range c.keep {
				if !strings.Contains(got, keep) {
					t.Errorf("脱敏把上下文也丢了，缺少 %q：%s", keep, got)
				}
			}
		})
	}
}

// 脱敏不能误伤普通日志：把整行丢掉的做法会让日志失去可读性。
func Test脱敏保留普通日志内容(t *testing.T) {
	raw := "生成视频完成，耗时 12.4 秒，输出 1080x1920"
	got := redactDiagnostic(raw)
	if got != raw {
		t.Errorf("普通日志被改动了：%s", got)
	}
}

// 报告里必须写清楚日志文件在哪，用户才知道去哪儿找原始日志。
func Test排障报告写明日志文件位置(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	report := a.Diagnostics()
	for _, want := range []string{"桌面端界面日志", "本地工作台日志", "崩溃日志", "工作台状态"} {
		if !strings.Contains(report, want) {
			t.Errorf("排障报告里缺少 %q：\n%s", want, report)
		}
	}
}

// 反馈区要有「导出排障报告」：用户上报问题的路径不能只有「复制」一种。
func Test反馈区提供导出排障报告入口(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	a.mode = ModeSettings
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	if !tt.HasText("导出排障报告") {
		t.Errorf("反馈区缺少「导出排障报告」入口，实际文字：%v", tt.Texts())
	}
	if !tt.HasText("复制诊断信息") {
		t.Errorf("反馈区缺少「复制诊断信息」入口，实际文字：%v", tt.Texts())
	}
}

// 日志与崩溃文件都要落在系统临时目录里：
// Windows 上就是 %TEMP%，用户遇到闪退时按 README 说的路径就能找到。
func Test日志与崩溃文件落在系统临时目录(t *testing.T) {
	tmp := os.TempDir()
	if !strings.HasPrefix(BackendLogPath(), tmp) {
		t.Errorf("工作台日志不在临时目录：%s（临时目录 %s）", BackendLogPath(), tmp)
	}
	if !strings.HasPrefix(CrashLogPath(), tmp) {
		t.Errorf("崩溃日志不在临时目录：%s（临时目录 %s）", CrashLogPath(), tmp)
	}
}
