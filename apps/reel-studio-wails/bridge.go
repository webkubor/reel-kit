package main

import (
	"context"
	"encoding/json"
	"net/http"
)

// App 是桌面壳本身。
//
// 壳里没有业务功能：界面是本机工作台（Node 服务）那一份网页。
// 窗口打开时先显示一个引导页，等工作台起来、拿到它的地址后跳过去。
type App struct {
	backend *Backend
	ctx     context.Context
}

// NewApp 造一个桌面壳。
func NewApp() *App {
	return &App{backend: NewBackend("")}
}

// startup 是窗口起来时做的事：把本机工作台拉起来。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.backend.Ensure()
}

// shutdown 是退出时做的事：把本机工作台收掉。
func (a *App) shutdown(ctx context.Context) {
	a.backend.Stop()
}

// Status 给界面问工作台的状态用（Go 绑定，前端调 window.go…Status()）。
func (a *App) Status() Status {
	a.backend.Ensure()
	return a.backend.Status()
}

// serve 是窗口里那个资源服务器的兜底。
//
// 引导页是随应用打包的静态页（frontend/dist/index.html），
// 这里只回答一个问题：工作台好了没有、地址是多少。
func (a *App) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/__studio/status" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(a.Status())
		return
	}
	http.NotFound(w, r)
}
