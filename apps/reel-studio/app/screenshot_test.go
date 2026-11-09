package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Test截图 把界面渲染成 PNG，供人眼核对布局。
//
// 自动化测试只能证明「文字在不在」，看不出层次、留白与对齐是否成立。
// 这张图就是 UI 走查的证据。
func Test截图(t *testing.T) {
	server := stub(t, []map[string]any{{
		"id": "j1", "projectId": "p1", "status": "failed", "createdAt": "2026-01-01T00:00:00Z",
		"input":    map[string]any{"kind": "video", "prompt": "橘猫在窗台上看雨，晨光斜照，镜头缓慢推近"},
		"error":    "下载超时（上游任务 task-123 已生成并计费，素材还在上游；重新提交会重复计费）",
		"upstream": map[string]any{"channel": "minimax", "taskId": "task-123"},
	}})
	a := newApp(t, server.URL)
	a.prompt = "橘猫在窗台上看雨，晨光斜照，镜头缓慢推近"

	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 840},
		{"narrow", 1024, 720},
	} {
		tt := ui.NewTester(a.View, size.width, size.height)
		tt.Frame()
		path := filepath.Join(os.TempDir(), "reel-studio-"+size.name+".png")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
		file.Close()
		t.Logf("截图：%s（%d×%d）", path, size.width, size.height)
	}
}
