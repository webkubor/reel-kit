package main

import "sync"

// Engine 是桌面壳的本地引擎：零 HTTP，零 Node，不启动任何子进程、不占端口。
// 所有状态留在内存里，通过 Go 绑定直接让界面调用。
//
// Phase 2 会把合成/TTS/供应商等逻辑从 Node 层搬进来，直接以 Go 方法
// 挂到 Engine 上，不再依赖任何外部 Node 进程或运行时目录。
type Engine struct {
	mu   sync.Mutex
	note string
}

// Status 是引擎当前状态，给界面看。
type Status struct {
	Ready bool   `json:"ready"`
	URL   string `json:"url"`
	Note  string `json:"note"`
	Error string `json:"error"`
}

// NewEngine 造一个本地引擎。
func NewEngine() *Engine {
	return &Engine{note: "本地工作台已就绪"}
}

// Status 返回当前状态：直接就绪，不启动任何子进程。
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{Ready: true, URL: "", Note: e.note, Error: ""}
}

// Stop 是窗口退出钩子：没有子进程要回收。
func (e *Engine) Stop() {}
