//go:build windows

package main

import (
	"os/exec"
	"strconv"
)

// configureProcess：Windows 上没有进程组，taskkill /T 会连子进程一起收。
func configureProcess(cmd *exec.Cmd) {}

// terminate 收掉这个进程和它派生的子进程。
//
// 进程回收（Wait）是 spawn 里那个 goroutine 的活，这里只让 Windows 连根结束、
// 等它转来回收结果。
func terminate(child *childProcess) {
	if child == nil || child.cmd == nil || child.cmd.Process == nil {
		return
	}
	// taskkill /T /F 会连同子孙进程一起结束。
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(child.cmd.Process.Pid)).Run()
	<-child.done
}
