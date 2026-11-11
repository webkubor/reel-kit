package main

import (
	"context"
)

// App 是桌面壳本身。
//
// 壳里没有业务功能：界面由本地引擎（Engine）提供，通过 Go 绑定直接调用。
// 窗口打开时先显示引导页，引擎就绪后停留在引导页（Phase 6 会把这里换成
// 真正的本地控制台 UI，不再跳转到任何工作台地址）。
type App struct {
	engine *Engine
}

// NewApp 造一个桌面壳。
func NewApp() *App {
	return &App{engine: NewEngine()}
}

// startup 是窗口起来时做的事：拉起任务队列 worker，其余零 HTTP 无子进程要拉起。
func (a *App) startup(_ context.Context) {
	a.engine.Start()
}

// shutdown 是退出时做的事：停掉 worker 与本机语音服务。
func (a *App) shutdown(_ context.Context) {
	a.engine.Stop()
	a.engine.StopVoxcraft()
}

// Status 给界面问引擎状态用（Go 绑定，前端调 window.go…Status()）。
func (a *App) Status() Status {
	return a.engine.Status()
}
