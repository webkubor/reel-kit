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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/app"
)

// crashLogFile 打开崩溃日志文件（追加写）。
//
// 写不进去也不能让程序起不来：崩了就崩了，别再因为日志路径不对二次崩溃。
func crashLogFile() *os.File {
	path := filepath.Join(os.TempDir(), "reel-studio-crash.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}

// dumpCrash 在发生未捕获 panic 时把调用栈写入磁盘。
//
// Windows GUI 模式下 stderr 默认会被系统吞掉，双击崩溃时用户屏幕上没有任何提示。
// 把调用栈写在临时目录与可执行文件同级，出问题时用户只需把 crash.log 贴出来就能定位。
func dumpCrash(r any) {
	msg := fmt.Sprintf("[%s] Reel Studio 异常退出 (Crash):\n%v\n\n调用栈:\n%s\n",
		time.Now().Format("2006-01-02 15:04:05"), r, debug.Stack())
	// 1. 系统临时目录（如 Windows 的 %TEMP%\reel-studio-crash.log）
	tmpFile := filepath.Join(os.TempDir(), "reel-studio-crash.log")
	_ = os.WriteFile(tmpFile, []byte(msg), 0o644)
	// 2. 可执行文件同级目录
	if exe, err := os.Executable(); err == nil {
		exeCrash := filepath.Join(filepath.Dir(exe), "reel-studio-crash.log")
		_ = os.WriteFile(exeCrash, []byte(msg), 0o644)
	}
	_ = os.WriteFile("reel-studio-crash.log", []byte(msg), 0o644)
	log.Print(msg)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			dumpCrash(r)
			panic(r)
		}
	}()

	// 未捕获的 panic（包含其它 goroutine 里的）也落盘。
	// 只靠 main 的 recover 只能盖住主协程：后台轮询、媒体播放这些 goroutine
	// 里一旦 panic，Windows 上用户看到的就是「双击没反应」，什么线索都没有。
	if f := crashLogFile(); f != nil {
		debug.SetCrashOutput(f, debug.CrashOptions{})
		defer f.Close()
	}

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
		dumpCrash(err)
		log.Fatal(err)
	}
	// 只停自己起的工作台：别人在终端里跑的那个不动。
	a.Stop()
}
