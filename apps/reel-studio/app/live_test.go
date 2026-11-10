package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Test真机界面 连真实本地工作台渲染一屏，检查客户端契约与布局。
//
// 默认跳过：本机没起服务时会失败。跑法：
//
//	REEL_STUDIO_LIVE=1 go test ./app/ -run Test真机界面 -v
func Test真机界面(t *testing.T) {
	if os.Getenv("REEL_STUDIO_LIVE") != "1" {
		t.Skip("设置 REEL_STUDIO_LIVE=1 才连真实工作台")
	}
	a := New(Options{BaseURL: "http://127.0.0.1:4390"})
	a.refresh()
	// refresh 是通过 mutate 排队改状态的，状态要等渲染线程排空队列才落地。
	// 先渲染一帧，再看状态 —— 否则这里永远读到 nil。
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if a.state == nil {
		t.Fatal("连不上本地工作台")
	}
	t.Logf("项目 %d 个，素材 %d 条，任务 %d 条", len(a.state.Projects), len(a.state.Assets), len(a.state.Jobs))
	t.Logf("默认曲库 %d 首", len(a.bgm.Items))
	t.Logf("推荐模式通道：%v", a.Channels())
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"live-desktop", 1280, 840},
		{"live-narrow", 1024, 720},
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
