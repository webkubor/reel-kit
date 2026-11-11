//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// configureProcess 让子进程自成一个进程组。
//
// 不这么做的话，退出应用时只能杀掉 node 本人，它派生的 ffmpeg 会留在后台
// 继续占着 CPU 和文件句柄。
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminate 先礼后兵地收掉整个进程组。
//
// 进程回收（Wait）是 spawn 里那个 goroutine 的活，这里只发信号、等它转来
// 回收结果 —— 不能自己再 Wait 一次，否则两个人抢同一份回收。
func terminate(child *childProcess) {
	if child == nil || child.cmd == nil || child.cmd.Process == nil {
		return
	}
	pid := child.cmd.Process.Pid
	// 先给 3 秒优雅退出（Node 会关掉 HTTP 监听并落盘），不行再强杀。
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	select {
	case <-child.done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-child.done
	}
}
