package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// 更新状态。
const (
	UpdateIdle    = "idle"    // 没查过
	UpdateChecking = "checking" // 正在查
	UpdateLatest  = "latest"  // 已是最新
	UpdateReady   = "ready"   // 有新版可装
	UpdateFailed  = "failed"  // 查不了（离线 / 无签名密钥）
	UpdateInstalling = "installing" // 正在下载安装
	UpdateInstalled = "installed"  // 装好了，重启生效
)

// UpdateInfo 是界面上的更新提示。
type UpdateInfo struct {
	State   string
	Version string
	Notes   string
	Error   string
}

// App 的更新状态。
//
// 写入统一走 mutate（排到渲染线程），读取也在渲染线程 ——
// 所以这里不需要锁。曾经加的 mutex 是防写入方和读取方撞车，
// 但写入方实际被排到渲染线程了，锁只是摆设，反而让人以为可以随便读写。
type updates struct {
	info UpdateInfo
}

// checkUpdate 查有没有新版。
//
// 用 mygo.Updater —— 它读 GitHub Release 上带签名的清单，
// 只有用我们私钥签过的包才会被装上。
func (a *App) checkUpdate() {
	a.setUpdateInfo(UpdateInfo{State: UpdateChecking})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	update, err := mygo.Updater.Check(ctx)
	a.mutate(func() {
		switch {
		case err != nil:
			// 没启用更新的开发版构建不算故障，静默即可
			if errors.Is(err, mygo.ErrUpdatesDisabled) {
				a.updates.info = UpdateInfo{State: UpdateIdle}
			} else {
				a.updates.info = UpdateInfo{State: UpdateFailed, Error: err.Error()}
				a.logf("检查更新失败：%s", err.Error())
			}
		case update == nil:
			a.updates.info = UpdateInfo{State: UpdateLatest, Version: mygo.App.Version()}
			a.logf("已是最新版本 %s", mygo.App.Version())
		default:
			a.updates.info = UpdateInfo{State: UpdateReady, Version: update.Version, Notes: update.Notes}
			a.logf("发现新版本 %s", update.Version)
		}
	})
}

// installUpdate 下载并安装新版，装完提示重启。
func (a *App) installUpdate() {
	a.setUpdateInfo(UpdateInfo{State: UpdateInstalling})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	update, err := mygo.Updater.Check(ctx)
	if err != nil || update == nil {
		msg := "没有找到可安装的更新"
		if err != nil {
			msg = err.Error()
		}
		a.setUpdateInfo(UpdateInfo{State: UpdateFailed, Error: msg})
		return
	}
	err = update.Install(ctx, func(downloaded, total int64) {
		if total > 0 {
			a.setUpdateInfo(UpdateInfo{
				State:   UpdateInstalling,
				Version: update.Version,
				Error:   fmt.Sprintf("下载中 %d%%", downloaded*100/total),
			})
		}
	})
	if err != nil {
		a.setUpdateInfo(UpdateInfo{State: UpdateFailed, Error: err.Error()})
		a.logf("安装更新失败：%s", err.Error())
		return
	}
	a.setUpdateInfo(UpdateInfo{State: UpdateInstalled, Version: update.Version})
	a.logf("更新 %s 已安装，重启后生效", update.Version)
}

// setUpdateInfo 记录更新状态并重绘。
func (a *App) setUpdateInfo(info UpdateInfo) {
	a.mutate(func() {
		a.updates.info = info
	})
}

// UpdateState 返回当前更新状态，供界面显示。
func (a *App) UpdateState() UpdateInfo {
	return a.updates.info
}

// ── 日志 ────────────────────────────────────────────────────

// LogWriter 是应用自己的日志。
//
// 后端的输出另有一份（临时目录里的 reel-studio-backend.log），
// 那份含供应商返回内容，不进这里，避免把密钥之类的写进用户可见的日志。
type LogWriter struct {
	mu   sync.Mutex
	file *os.File
	// size 是当前日志字节数，用来轮转。
	size int64
}

// maxLogSize 是单个日志文件的上限，超过就换一份新的。
//
// 不轮转的话，日志会一直涨到磁盘满 —— 用户不会去看，但也不会删。
const maxLogSize = 2 * 1024 * 1024

// NewLogWriter 打开日志文件。
//
// macOS 放在 ~/Library/Logs/Reel Studio/，其余平台放用户配置目录。
func NewLogWriter() *LogWriter {
	dir := ""
	if path, err := mygo.App.Path(mygo.PathLogs); err == nil {
		dir = path
	} else if path, err := mygo.App.Path(mygo.PathAppData); err == nil {
		dir = filepath.Join(path, "logs")
	}
	if dir == "" {
		return &LogWriter{}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return &LogWriter{}
	}
	path := filepath.Join(dir, "reel-studio.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return &LogWriter{}
	}
	var size int64
	if fi, err := file.Stat(); err == nil {
		size = fi.Size()
	}
	return &LogWriter{file: file, size: size}
}

// Logf 写一条带时间的日志。
func (l *LogWriter) Logf(format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
	n, err := l.file.WriteString(line)
	if err != nil {
		return
	}
	l.size += int64(n)
	if l.size > maxLogSize {
		l.rotate()
	}
}

// rotate 把当前日志改名留存，重新开一个空文件。
//
// 保留上一份（reel-studio.log.1）比直接删掉有用：出问题时能看上一轮发生了什么。
func (l *LogWriter) rotate() {
	if l.file == nil {
		return
	}
	name := l.file.Name()
	_ = l.file.Close()
	_ = os.Remove(name + ".1")
	_ = os.Rename(name, name+".1")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		l.file = nil
		return
	}
	l.file = file
	l.size = 0
}

// LogPath 返回日志文件的路径，界面上要显示给用户。
func (l *LogWriter) LogPath() string {
	if l == nil {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return ""
	}
	return l.file.Name()
}

// Close 关闭日志文件。
func (l *LogWriter) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	_ = l.file.Close()
	l.file = nil
}

// logf 写一条应用日志。
func (a *App) logf(format string, args ...any) {
	if a.log == nil {
		return
	}
	a.log.Logf(format, args...)
}

// LogFilePath 返回日志文件路径。
func (a *App) LogFilePath() string {
	if a.log == nil {
		return ""
	}
	return a.log.LogPath()
}

// revealPath 用系统文件管理器打开某个路径。
//
// macOS 用 open，Windows 用 explorer；打不开就算了，不影响主流程。
func revealPath(path string) {
	if path == "" {
		return
	}
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{"-R", path}
	case "windows":
		cmd = "explorer"
		args = []string{"/select,", path}
	default:
		cmd = "xdg-open"
		args = []string{filepath.Dir(path)}
	}
	_ = startDetached(cmd, args...)
}
