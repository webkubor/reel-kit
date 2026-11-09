package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Backend 是本地工作台（Node 服务）的进程管理。
//
// 桌面版要能双击就用：如果本机没有已经在跑的工作台，App 自己起一个；
// 如果已经有（在别处开着终端跑 reel.mjs studio），就复用它，绝不抢端口、
// 更不把别人的进程杀掉。
type Backend struct {
	BaseURL string
	// Root 是 reel-kit 的运行目录（里面有 bin/reel.mjs）。
	Root string

	mu      sync.Mutex
	cmd     *exec.Cmd
	started bool
	logFile *os.File
}

// BackendStatus 是工作台的连接状态。
type BackendStatus struct {
	State string // starting | ready | failed | stopped
	Note  string
}

// NewBackend 返回一个后端管理器。
func NewBackend(baseURL, root string) *Backend {
	return &Backend{BaseURL: baseURL, Root: root}
}

// Port 从 BaseURL 里取端口。
func (b *Backend) Port() string {
	_, port, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(b.BaseURL, "http://"), "https://"))
	if err != nil {
		return "4390"
	}
	return port
}

// Alive 报告工作台是否在响应。
func (b *Backend) Alive() bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	res, err := client.Get(b.BaseURL + "/api/state")
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode < 500
}

// Ensure 保证工作台可用：能连上就用，连不上就自己起一个。
func (b *Backend) Ensure() BackendStatus {
	if b.Alive() {
		return BackendStatus{State: "ready", Note: "已连接本机工作台"}
	}
	if b.Root == "" {
		return BackendStatus{State: "failed", Note: "找不到本地工作台的运行文件（resources/backend）"}
	}
	entry := filepath.Join(b.Root, "bin", "reel.mjs")
	if _, err := os.Stat(entry); err != nil {
		return BackendStatus{State: "failed", Note: "本地工作台的文件不完整：" + err.Error()}
	}
	if err := b.spawn(); err != nil {
		return BackendStatus{State: "failed", Note: "启动本地工作台失败：" + err.Error()}
	}
	// 等它把端口监听起来，最多 20 秒
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if b.Alive() {
			return BackendStatus{State: "ready", Note: "已启动本机工作台"}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return BackendStatus{State: "failed", Note: "本地工作台 20 秒内没有起来，请看日志"}
}

// spawn 起一个 Node 进程跑工作台。
func (b *Backend) spawn() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cmd != nil {
		return nil
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("找不到 node：%w", err)
	}
	entry := filepath.Join(b.Root, "bin", "reel.mjs")
	cmd := exec.Command(node, entry, "studio")
	cmd.Dir = b.Root
	cmd.Env = append(os.Environ(),
		"REEL_STUDIO_PORT="+b.Port(),
	)
	// 输出的行数不限，但要落盘：出问题时用户能自己看到原因。
	logPath := filepath.Join(os.TempDir(), "reel-studio-backend.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		cmd.Stdout = file
		cmd.Stderr = file
		b.logFile = file
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	b.cmd = cmd
	b.started = true
	go func() {
		_ = cmd.Wait()
		b.mu.Lock()
		b.cmd = nil
		b.mu.Unlock()
	}()
	return nil
}

// Stop 停掉自己起的工作台。别人开的进程不归我们管。
func (b *Backend) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cmd == nil || !b.started {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = b.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { _, _ = b.cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		_ = b.cmd.Process.Kill()
	}
	b.cmd = nil
	if b.logFile != nil {
		_ = b.logFile.Close()
		b.logFile = nil
	}
}

// FindRoot 找 reel-kit 的运行目录。
//
// 开发时是仓库根（apps/reel-studio 往上两级）；
// 打包后是应用包里的 resources/backend（macOS 是 .app/Contents/Resources，
// Windows 是可执行文件旁边的 backend 目录）。
func FindRoot() string {
	if custom := os.Getenv("REEL_STUDIO_ROOT"); custom != "" {
		return custom
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, "backend"),
			filepath.Join(dir, "..", "Resources", "backend"),
			filepath.Join(dir, "..", "..", "..", ".."),
		} {
			candidate = filepath.Clean(candidate)
			if _, err := os.Stat(filepath.Join(candidate, "bin", "reel.mjs")); err == nil {
				return candidate
			}
		}
	}
	// 开发态：从源码目录往上找
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			if _, err := os.Stat(filepath.Join(dir, "bin", "reel.mjs")); err == nil {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if runtime.GOOS == "darwin" {
		return ""
	}
	return ""
}
