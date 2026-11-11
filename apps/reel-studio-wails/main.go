// Reel Studio 桌面壳（Wails v2）。
//
// 零 HTTP：不再起 Node 工作台、不占端口、不常驻进程。所有逻辑通过
// Go 绑定（window.go.App）直接调用本地引擎。界面由 frontend/dist
// 里的引导页提供，Phase 6 会把它换成本地控制台 UI。
package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "Reel Studio",
		Width:     1280,
		Height:    840,
		MinWidth:  1024,
		MinHeight: 680,
		// 界面就是工作台本身。这里只打包一个引导页：
		// 等工作台起来、拿到它的地址后再跳过去。
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		Bind:               []interface{}{app},
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "online.webkubor.reelstudio"},
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title: "Reel Studio",
				Message: "本地视频工作台。\n" +
					"素材与项目留在这台机器上；语音与出图用在线服务时会联网，费用按各自渠道计。",
			},
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Reel Studio 启动失败：", err)
		os.Exit(1)
	}
}
