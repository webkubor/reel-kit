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
// 这张图就是 UI 走查的证据：每条流水线都要有，宽窄两种窗口都要有。
//
// 无头渲染（MyGo 的 Tester）不依赖屏幕，锁屏、远程、CI 上都能跑。
func Test截图(t *testing.T) {
	// 截图要可复现：密钥状态读的是本机配置，这里换成一个临时文件，
	// 不然这台机器配没配密钥会决定截图长什么样，密钥尾号还会漏进图里。
	old := reelConfigPathFn
	reelConfigPathFn = func() string { return filepath.Join(t.TempDir(), "config.json") }
	t.Cleanup(func() { reelConfigPathFn = old })

	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	a.talkTitle = "3 个 AI 小技巧"
	a.talkCaptions = "第一个技巧：先写脚本再配音\n第二个技巧：一句话一镜，别贪多\n第三个技巧：做完立刻看效果"
	a.talkSubtitle = "新手也能用"
	a.recTitle = "Reel Kit：把想法变成视频"
	a.recCaptions = "一句话说清它解决什么问题\n第二句给出具体用法\n第三句说清谁能用"
	a.repoURL = "https://github.com/webkubor/reel-kit"
	a.prodTitle = "Reel Studio 本地视频工作台"
	a.prodSubtitle = "素材不出本机"
	a.prodCaptions = "Reel Studio 是跑在自己电脑上的视频工作台\n写一句一镜的脚本，它就配好音、渲好画面、合成成片\n每条视频背后的生成配置都留着"
	a.prodFooter = "现在就去试试"

	for _, mode := range []struct {
		name string
		mode Mode
	}{
		{"talk", ModeTalk},
		{"recommend", ModeRecommend},
		{"export", ModeExport},
		{"product", ModeProduct},
		{"library", ModeLibrary},
		{"settings", ModeSettings},
	} {
		a.mode = mode.mode
		for _, size := range []struct {
			name          string
			width, height int
		}{
			{"desktop", 1280, 840},
			{"narrow", 1024, 720},
		} {
			tt := ui.NewTester(a.View, size.width, size.height)
			tt.Frame()
			path := filepath.Join(os.TempDir(), "reel-studio-"+mode.name+"-"+size.name+".png")
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
}
