package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Backend 管着本机工作台（Node 服务）这个子进程。
//
// 桌面版自己起一个工作台，端口不用固定的 4390：端口是自己挑的空位，
// 起完就知道连的是谁。固定端口会撞车 —— 撞上了就会出现「窗口里连着的
// 其实是别处跑着的另一个工作台」，这种问题用户根本看不出来，也说不清。
type Backend struct {
	root string // 运行时目录：里面有 bin/node、backend/、chrome/

	mu       sync.Mutex
	child    *childProcess
	port     int
	ready    bool
	starting bool
	errMsg   string
	note     string
	logFile  *os.File
}

// Status 是工作台当前的状态，给界面看。
type Status struct {
	Ready bool   `json:"ready"`
	URL   string `json:"url"`
	Note  string `json:"note"`
	Error string `json:"error"`
}

// NewBackend 造一个工作台管理器。root 是运行时目录，空则自动找。
func NewBackend(root string) *Backend {
	if root == "" {
		root = findRuntimeRoot()
	}
	return &Backend{root: root, note: "正在准备本地工作台…"}
}

// Root 返回运行时目录。
func (b *Backend) Root() string { return b.root }

// Ensure 保证工作台在启动途中，不重复起。
func (b *Backend) Ensure() {
	b.mu.Lock()
	if b.ready || b.starting {
		b.mu.Unlock()
		return
	}
	b.starting = true
	b.errMsg = ""
	b.note = "正在启动本地工作台…"
	b.mu.Unlock()
	go b.run()
}

// Status 返回当前状态。
func (b *Backend) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	st := Status{Ready: b.ready, Note: b.note, Error: b.errMsg}
	if b.ready {
		st.URL = fmt.Sprintf("http://127.0.0.1:%d/", b.port)
	}
	return st
}

// run 起工作台。端口被抢就换一个再试，最多三次。
func (b *Backend) run() {
	if b.root == "" {
		b.fail("找不到本地工作台的运行文件（应用包里的 runtime 目录不完整）")
		return
	}
	if err := b.checkRuntime(); err != nil {
		b.fail(err.Error())
		return
	}
	for attempt := 1; attempt <= 3; attempt++ {
		port, err := freePort()
		if err != nil {
			b.fail("找不到可用端口：" + err.Error())
			return
		}
		child, err := b.spawn(port)
		if err != nil {
			b.fail("启动本地工作台失败：" + err.Error())
			return
		}
		ok, died := waitReady(port, 30*time.Second, child)
		if ok {
			b.mu.Lock()
			b.child = child
			b.port = port
			b.ready = true
			b.starting = false
			b.note = "本地工作台已就绪"
			b.mu.Unlock()
			return
		}
		// 进程早退，多半是端口刚被别人抢了：换一个端口再来。
		terminate(child)
		if !died {
			b.fail("本地工作台没能起来，请看日志：" + b.logPath())
			return
		}
		b.mu.Lock()
		b.note = fmt.Sprintf("端口被占用，正在换一个（%d/3）…", attempt)
		b.mu.Unlock()
	}
	b.fail("本地工作台连续三次都没能起来，请看日志：" + b.logPath())
}

// checkRuntime 检查运行时要用的文件齐不齐。
func (b *Backend) checkRuntime() error {
	for _, rel := range []string{"bin/node", "backend/bin/reel.mjs"} {
		if _, err := os.Stat(filepath.Join(b.root, filepath.FromSlash(rel))); err != nil {
			return fmt.Errorf("本地工作台的文件不完整：缺少 %s", rel)
		}
	}
	return nil
}

// spawn 起一个 Node 进程跑工作台。
func (b *Backend) spawn(port int) (*childProcess, error) {
	node := filepath.Join(b.root, "bin", "node")
	if runtime.GOOS == "windows" {
		node = filepath.Join(b.root, "bin", "node.exe")
	}
	backend := filepath.Join(b.root, "backend")
	entry := filepath.Join(backend, "bin", "reel.mjs")

	cmd := exec.Command(node, entry, "studio")
	cmd.Dir = backend
	cmd.Env = b.env(port)
	configureProcess(cmd)

	logFile, err := os.OpenFile(b.logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		b.mu.Lock()
		b.logFile = logFile
		b.mu.Unlock()
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return nil, err
	}
	child := &childProcess{cmd: cmd, done: make(chan error, 1)}
	go func() {
		err := cmd.Wait()
		child.done <- err
		close(child.done)
	}()
	return child, nil
}

// env 组装子进程的环境。
//
// 打包版把自带的 ffmpeg / ffprobe / node 放到 PATH 最前面，
// 渲染视频帧要用的 Chrome headless 也告诉它在哪 ——
// 用户机器上装没装这些都不影响。
func (b *Backend) env(port int) []string {
	env := os.Environ()
	path := os.Getenv("PATH")
	if runtime.GOOS == "darwin" {
		path = "/opt/homebrew/bin:/usr/local/bin:" + path
	}
	// 从 Finder 启动时 PATH 很短，~/.local/bin 里的 museav 之类会找不到。
	if home, err := os.UserHomeDir(); err == nil {
		localBin := filepath.Join(home, ".local", "bin")
		if !strings.Contains(path, localBin) {
			path = localBin + string(os.PathListSeparator) + path
		}
	}
	binDir := filepath.Join(b.root, "bin")
	env = append(env, "PATH="+binDir+string(os.PathListSeparator)+path)
	env = append(env, "REEL_STUDIO_PORT="+strconv.Itoa(port))
	if chrome := b.bundledChrome(); chrome != "" {
		env = append(env, "CHROME_PATH="+chrome)
	}
	return env
}

// bundledChrome 找打包进来的 Chrome headless shell。
func (b *Backend) bundledChrome() string {
	name := "chrome-headless-shell"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(b.root, "chrome", name)
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p
	}
	return ""
}

// logPath 是工作台的日志位置。出问题时用户能自己看到原因。
func (b *Backend) logPath() string {
	return filepath.Join(os.TempDir(), "reel-studio-backend.log")
}

// fail 记下失败原因。
func (b *Backend) fail(msg string) {
	b.mu.Lock()
	b.starting = false
	b.ready = false
	b.errMsg = msg
	b.note = ""
	b.mu.Unlock()
}

// Stop 停掉自己起的工作台。
func (b *Backend) Stop() {
	b.mu.Lock()
	child := b.child
	b.child = nil
	b.ready = false
	b.starting = false
	logFile := b.logFile
	b.logFile = nil
	b.mu.Unlock()
	if child != nil {
		terminate(child)
	}
	if logFile != nil {
		_ = logFile.Close()
	}
}

// childProcess 是一个工作台子进程。
//
// done 是全程序里唯一的进程回收点：spawn 起动后，只有一个 goroutine 在 cmd.Wait()，
// 回收结果经 done 发给等它的人。谁都不许再调一次 Wait，否则会争抢同一份回收。
type childProcess struct {
	cmd  *exec.Cmd
	done chan error
}

// waitReady 等工作台把端口监听起来。
//
// 返回 (是否就绪, 进程是不是自己退了)。
// 进程自己退了多半是端口被抢 —— 那就换一个端口再试。
func waitReady(port int, timeout time.Duration, child *childProcess) (bool, bool) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	for {
		select {
		case <-child.done:
			return false, true
		default:
		}
		res, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/state", port))
		if err == nil {
			res.Body.Close()
			if res.StatusCode < 500 {
				return true, false
			}
		}
		if time.Now().After(deadline) {
			return false, false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// freePort 找一个空着的本机端口。
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// findRuntimeRoot 找运行时目录（node、后端、Chrome 都在里面）。
//
// 打包后是应用包里的 Contents/Resources/runtime；
// 开发时是仓库里的 apps/reel-studio/resources/<平台>。
func findRuntimeRoot() string {
	if custom := os.Getenv("REEL_STUDIO_RUNTIME"); custom != "" {
		if hasRuntime(custom) {
			return filepath.Clean(custom)
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, "..", "Resources", "runtime"), // macOS 应用包
			filepath.Join(dir, "runtime"),                    // 可执行文件旁边
			filepath.Join(dir, "..", "Resources"),            // 旧布局：resources/<平台>
			filepath.Join(dir, "resources"),
		} {
			if hasRuntime(candidate) {
				return filepath.Clean(candidate)
			}
		}
	}
	// 开发态：从当前目录往上找仓库里的 resources/<平台>
	if wd, err := os.Getwd(); err == nil {
		platform := platformKey()
		for dir := wd; ; {
			for _, candidate := range []string{
				filepath.Join(dir, "apps", "reel-studio", "resources", platform),
				filepath.Join(dir, "resources", platform),
			} {
				if hasRuntime(candidate) {
					return filepath.Clean(candidate)
				}
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

// hasRuntime 判断一个目录是不是完整的运行时。
func hasRuntime(dir string) bool {
	for _, rel := range []string{"bin/node", "backend/bin/reel.mjs"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			return false
		}
	}
	return true
}

// platformKey 返回当前平台的资源目录名。
func platformKey() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}
