package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDiagnostics包含日志路径与内容(t *testing.T) {
	// 创建临时工作台日志与桌面端日志
	tmpDir := t.TempDir()
	backendLog := filepath.Join(tmpDir, "backend.log")
	appLog := filepath.Join(tmpDir, "app.log")

	_ = os.WriteFile(backendLog, []byte("2026-11-10 12:00:00 [info] server listening on 4390\n2026-11-10 12:00:05 [error] ffmpeg exit code 1\n"), 0o644)
	_ = os.WriteFile(appLog, []byte("2026-11-10 12:00:00 Reel Studio 启动（版本 1.2.0）\n"), 0o644)

	a := &App{
		backendState: BackendStatus{State: "failed", Note: "工作台未响应"},
	}

	diag := a.Diagnostics()

	// 必须包含工作台状态和两份日志的路径标识
	if !strings.Contains(diag, "工作台状态") || !strings.Contains(diag, "failed") {
		t.Errorf("诊断信息未包含工作台状态: %s", diag)
	}
	if !strings.Contains(diag, "reel-studio-backend.log") {
		t.Errorf("诊断信息未包含工作台日志路径: %s", diag)
	}
	if !strings.Contains(diag, "工作台与引擎近期日志") {
		t.Errorf("诊断信息未包含工作台近期日志分区: %s", diag)
	}
}

func Test脱敏处理敏感字段(t *testing.T) {
	raw := "2026-11-10 12:00:00 Authorization: Bearer sk-secret123456789\n" +
		"2026-11-10 12:00:01 request with apikey=abc123xyz\n" +
		"2026-11-10 12:00:02 normal log line without secret\n" +
		"2026-11-10 12:00:03 channelKeys updated for minimax\n"

	sanitized := sanitizeDiagnosticLog(raw)

	if strings.Contains(sanitized, "sk-secret123456789") {
		t.Errorf("脱敏未生效，包含了 sk- 密钥: %s", sanitized)
	}
	if strings.Contains(sanitized, "abc123xyz") {
		t.Errorf("脱敏未生效，包含了 apikey: %s", sanitized)
	}
	if !strings.Contains(sanitized, "normal log line without secret") {
		t.Errorf("正常日志被误删: %s", sanitized)
	}
	if !strings.Contains(sanitized, "[已遮盖]") {
		t.Errorf("未给出脱敏提示标记: %s", sanitized)
	}
	// 脱敏按值遮盖，不整行丢弃 —— 上下文还要留着才能定位问题。
	if !strings.Contains(sanitized, "channelKeys updated for minimax") {
		t.Errorf("不含密钥的普通行被误伤: %s", sanitized)
	}
}

func TestReadTail边界情况(t *testing.T) {
	// 1. 不存在的文件
	if out := readTail("/path/to/non/existent/log", 10); out != "" {
		t.Errorf("不存在的文件应该返回空串，得到: %q", out)
	}

	// 2. 空文件
	tmp := filepath.Join(t.TempDir(), "empty.log")
	_ = os.WriteFile(tmp, []byte(""), 0o644)
	if out := readTail(tmp, 10); out != "" {
		t.Errorf("空文件应该返回空串，得到: %q", out)
	}

	// 3. 行数超过限制
	tmp2 := filepath.Join(t.TempDir(), "lines.log")
	_ = os.WriteFile(tmp2, []byte("line1\nline2\nline3\nline4\nline5\n"), 0o644)
	out := readTail(tmp2, 2)
	expected := "line4\nline5"
	if out != expected {
		t.Errorf("期望末尾 2 行 %q，得到 %q", expected, out)
	}
}

func TestBackendLogPath(t *testing.T) {
	p := BackendLogPath()
	if !strings.HasSuffix(p, "reel-studio-backend.log") {
		t.Errorf("BackendLogPath 未以 reel-studio-backend.log 结尾: %s", p)
	}
}

func Test设置页展示完整双日志与排障入口(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	a.mode = ModeSettings
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	// 必须展示本地工作台日志说明与界面日志说明
	if !tt.HasText("本地工作台日志（Node/FFmpeg/离线语音/生成报错）：") {
		t.Errorf("设置页缺少工作台日志入口")
	}
	if !tt.HasText("打开工作台日志") {
		t.Errorf("设置页缺少「打开工作台日志」按钮")
	}
	if !tt.HasText("桌面端界面日志（窗口与界面状态）：") {
		t.Errorf("设置页缺少桌面端界面日志入口")
	}
	if !tt.HasText("复制完整诊断与近期日志") {
		t.Errorf("设置页缺少一键复制完整诊断按钮")
	}
	// 排障报告要能落成文件：日志上百行时，拖附件比粘贴强。
	if !tt.HasText("导出排障报告") {
		t.Errorf("设置页缺少「导出排障报告」按钮")
	}
}
