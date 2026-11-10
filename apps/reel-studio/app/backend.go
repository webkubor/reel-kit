package app

import (
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
//
// 打包版自带 Node 运行时、ffmpeg 与 Chrome headless，都在 resources/<平台>/ 下，
// 不依赖用户机器上装没装这些。
type Backend struct {
	BaseURL string
	// Root 是 reel-kit 的运行目录（里面有 bin/reel.mjs）。
	Root string
	// Bundled 是打包进 App 的运行时目录（resources/<平台>），开发时为空。
	Bundled string

	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan struct{}
	started bool
	logFile *os.File
}

// BackendStatus 是工作台的连接状态。
type BackendStatus struct {
	State string // starting | ready | failed | stopped
	Note  string
	// Started 说明这个工作台是本应用起的（true），还是复用了一个已经在跑的（false）。
	// 重启工作台后要靠它说话：复用上的进程我们没重启过，密钥未必生效，
	// 说成「已重启」就是在骗人。
	Started bool
}

// NewBackend 返回一个后端管理器。
func NewBackend(baseURL, root string) *Backend {
	return &Backend{BaseURL: baseURL, Root: root, done: make(chan struct{})}
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
		return BackendStatus{State: "ready", Note: "已连接本机工作台", Started: false}
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
			return BackendStatus{State: "ready", Note: "已启动本机工作台", Started: true}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return BackendStatus{State: "failed", Note: "本地工作台 20 秒内没有起来，请看日志"}
}

// findNode 寻找 Node.js 运行时。
//
// 优先用打包进来的那份（resources/<平台>/bin/node），这样下载来的 App
// 不要求用户先装 Node；开发模式下才退回系统里的 node。
//
// macOS 从 Finder 启动时 PATH 通常被系统剥离得只剩系统默认路径，
// 所以还要兜底检查 Homebrew 与常见安装位置。
func (b *Backend) findNode() (string, error) {
	if b.Bundled != "" {
		name := "node"
		if runtime.GOOS == "windows" {
			name = "node.exe"
		}
		p := filepath.Join(b.Bundled, "bin", name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath("node"); err == nil {
		return p, nil
	}
	for _, candidate := range []string{
		"/opt/homebrew/bin/node",
		"/usr/local/bin/node",
		"/usr/bin/node",
	} {
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("找不到 Node.js 运行时，请确保已安装 Node（如 brew install node）")
}

// BackendLogPath 返回本地工作台（Node 后端）日志路径。
//
// 无论 macOS 还是 Windows，均放在系统临时目录的 reel-studio-backend.log 中。
// 离线语音、FFmpeg、Chrome 渲染与通道转发的所有标准输出与错误均汇入这里。
func BackendLogPath() string {
	return filepath.Join(os.TempDir(), "reel-studio-backend.log")
}

// spawn 起一个 Node 进程跑工作台。
func (b *Backend) spawn() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cmd != nil {
		return nil
	}
	node, err := b.findNode()
	if err != nil {
		return err
	}
	entry := filepath.Join(b.Root, "bin", "reel.mjs")
	cmd := exec.Command(node, entry, "studio")
	cmd.Dir = b.Root
	// 继承环境并补充 PATH，确保后台子进程能找到 ffmpeg 与各类 CLI
	pathEnv := os.Getenv("PATH")
	if runtime.GOOS == "darwin" && !strings.Contains(pathEnv, "/opt/homebrew/bin") {
		pathEnv = "/opt/homebrew/bin:/usr/local/bin:" + pathEnv
	}
	// 从 Finder 启动时 PATH 很短，~/.local/bin 里的 museav 之类会找不到。
	if home, err := os.UserHomeDir(); err == nil {
		localBin := filepath.Join(home, ".local", "bin")
		if !strings.Contains(pathEnv, localBin) {
			pathEnv = localBin + string(os.PathListSeparator) + pathEnv
		}
	}
	env := append(os.Environ(),
		"PATH="+pathEnv,
		"REEL_STUDIO_PORT="+b.Port(),
	)
	// 打包版：把自带的 ffmpeg/ffprobe 放到 PATH 最前面，
	// 并告诉后端 Chrome headless shell 在哪 —— 渲染视频帧要用它。
	if b.Bundled != "" {
		binDir := filepath.Join(b.Bundled, "bin")
		env = append(env, "PATH="+binDir+string(os.PathListSeparator)+pathEnv)
		if chrome := b.bundledChrome(); chrome != "" {
			env = append(env, "CHROME_PATH="+chrome)
		}
	}
	cmd.Env = env
	// 输出的行数不限，但要落盘：出问题时用户能自己看到原因。
	logPath := BackendLogPath()
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		cmd.Stdout = file
		cmd.Stderr = file
		b.logFile = file
	}
	if err := cmd.Start(); err != nil {
		if file != nil {
			_ = file.Close()
		}
		return err
	}
	b.cmd = cmd
	b.started = true
	done := make(chan struct{})
	b.done = done
	go func() {
		_ = cmd.Wait()
		// 收尾动作放在锁外：Stop 也要拿这把锁，
		// 在锁里等这个通道关闭就是自己等自己。
		b.mu.Lock()
		if b.cmd == cmd {
			b.cmd = nil
			b.started = false
		}
		b.mu.Unlock()
		close(done)
	}()
	return nil
}

// Stop 停掉自己起的工作台。别人开的进程不归我们管。
func (b *Backend) Stop() {
	b.mu.Lock()
	cmd, done, started := b.cmd, b.done, b.started
	b.mu.Unlock()
	if cmd == nil || !started {
		return
	}
	// 先礼后兵：给 3 秒优雅退出（Node 会关掉 HTTP 监听并落盘），
	// 不行再强杀。Windows 没有 SIGINT，直接杀。
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
	} else {
		_ = cmd.Process.Signal(os.Interrupt)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
	b.mu.Lock()
	if b.cmd == cmd {
		b.cmd = nil
		b.started = false
	}
	if b.logFile != nil {
		_ = b.logFile.Close()
		b.logFile = nil
	}
	b.mu.Unlock()
}

// bundledChrome 找打包进来的 Chrome headless shell。
func (b *Backend) bundledChrome() string {
	if b.Bundled == "" {
		return ""
	}
	name := "chrome-headless-shell"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(b.Bundled, "chrome", name)
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p
	}
	return ""
}

// FindRoot 找 reel-kit 的运行目录。
//
// 开发时是仓库根（apps/reel-studio 往上两级）；
// 打包后是应用包里的 resources/<平台>/backend（macOS 是 .app/Contents/Resources，
// Windows 是可执行文件旁边的资源目录）。
func FindRoot() string {
	if custom := os.Getenv("REEL_STUDIO_ROOT"); custom != "" {
		return custom
	}
	for _, candidate := range rootCandidates() {
		candidate = filepath.Clean(candidate)
		if _, err := os.Stat(filepath.Join(candidate, "bin", "reel.mjs")); err == nil {
			return candidate
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
	return ""
}

// rootCandidates 列出可能装着后端的目录。
func rootCandidates() []string {
	var out []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		out = append(out,
			// 打包版：macOS 在 Contents/Resources，Windows 在可执行文件旁边
			filepath.Join(dir, "..", "Resources", "backend"),
			filepath.Join(dir, "backend"),
			// 开发态：apps/reel-studio → 仓库根
			filepath.Join(dir, "..", "..", "..", ".."),
		)
	}
	return out
}

// platformDir 返回当前平台在 resources 下的目录名。
func platformDir() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// FindBundled 找打包进来的运行时目录（含 bin/node）。
//
// 打包时 mygo 把 resources/<平台>/ 里的东西摊平到 App 资源目录，
// 所以 macOS 是 Contents/Resources，Windows 是可执行文件旁边。
//
// 开发态返回空串 —— 那时用系统里的 node 与 ffmpeg。
func FindBundled() string {
	if custom := os.Getenv("REEL_STUDIO_BUNDLED"); custom != "" {
		return custom
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, "..", "Resources"),
			dir,
		} {
			candidate = filepath.Clean(candidate)
			for _, name := range []string{"node", "node.exe"} {
				if _, err := os.Stat(filepath.Join(candidate, "bin", name)); err == nil {
					return candidate
				}
			}
		}
	}
	return ""
}
