// Command reel-studio 是 reel-kit 工作台的原生桌面版。
//
// 界面全部是 Go 原生控件（MyGo 的 ui 包）：没有网页、没有前端构建，
// 窗口开得起来，也不用再写 HTML/CSS/JS。
//
// 后端仍是 reel-kit 的本地 Node 服务（src/studio/server.mjs）——
// 渲染、FFmpeg、供应商调用、状态机都在那边，两端共用同一份数据。
//
//	go run .            # 开发运行
//	go test ./...       # 不开窗口测交互
//	go tool mygo build  # 打包成 .app
package main

import (
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/app"
)

func main() {
	// 界面里所有动作都要能回到主线程改状态，所以状态与窗口都放在 app 包里，
	// 由它在收到后端数据时驱动重绘。
	base := os.Getenv("REEL_STUDIO_URL")
	if base == "" {
		base = "http://127.0.0.1:4390"
	}
	a := app.New(app.Options{BaseURL: base, BackendRoot: os.Getenv("REEL_STUDIO_ROOT")})

	mygo.App.WhenReady(func() {
		window := mygo.NewWindow(mygo.WindowOptions{
			Title:     "Reel Studio",
			Width:     1280,
			Height:    840,
			MinWidth:  1024,
			MinHeight: 680,
			Content:   ui.View(a.View),
		})
		a.SetWindow(window)
		a.Start()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
	// 只停自己起的工作台：别人在终端里跑的那个不动。
	a.Stop()
}
