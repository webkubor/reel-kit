package app

import (
	"os/exec"
	"runtime"
)

// startDetached 起一个后台进程，后台回收等待，不阻塞当前流程。
func startDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if runtime.GOOS == "darwin" {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
