// Reel Studio 桌面壳（Wails v2）。
//
// 桌面版只做三件事：把本机工作台（Node 服务）起起来、用原生窗口打开它、
// 退出时把它收掉。功能全在网页那一侧 —— 浏览器里跑的和桌面里跑的是同一份代码，
// 不会出现「桌面端少一个功能」这种两份维护出来的问题。
package main

import (
	"embed"
	"fmt"
	"net/http"
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
			Assets:  assets,
			Handler: http.HandlerFunc(app.serve),
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
