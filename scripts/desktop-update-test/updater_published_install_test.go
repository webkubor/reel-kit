package mygo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/update"
)

// TestPublishedInstall 走生产同一条路径 installUpdate，把装着旧版的 .app
// 原地换成发布页上的新版本：验签 → 增量还原 → 暂存 → 等应用退出 → 换包。
//
// 环境变量：
//   PUBLISHED_MANIFEST  发布清单的 URL
//   PUBLISHED_KEY       发布公钥（raw ed25519 hex）
//   INSTALL_TARGET      旧版 .app 的路径
//   INSTALL_FROM        旧版本号
//   INSTALL_EXPECT      期望装完后的版本号
func TestPublishedInstall(t *testing.T) {
	manifestURL := os.Getenv("PUBLISHED_MANIFEST")
	key := os.Getenv("PUBLISHED_KEY")
	target := os.Getenv("INSTALL_TARGET")
	from := os.Getenv("INSTALL_FROM")
	expect := os.Getenv("INSTALL_EXPECT")
	if manifestURL == "" || key == "" || target == "" {
		t.Skip("没有 PUBLISHED_MANIFEST/PUBLISHED_KEY/INSTALL_TARGET，跳过")
	}
	res, err := http.Get(manifestURL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var m update.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Version != expect {
		t.Fatalf("清单版本是 %s，不是 %s", m.Version, expect)
	}
	if m.Delta(from) == nil {
		t.Fatalf("清单里没有从 %s 到 %s 的增量包", from, expect)
	}

	// 应用正在跑的时候不能换包 —— 换的是运行中程序的目录。
	// 这里用一个占位进程扮演应用：installUpdate 返回后把它停掉，
	// 换包的助手脚本就会接手。
	placeholder := exec.Command("/bin/sleep", "300")
	if err := placeholder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = placeholder.Process.Kill()
		_ = placeholder.Wait()
	}()
	oldExe, oldPid := startExe, appPid
	startExe = filepath.Join(target, "Contents", "MacOS", "Reel Studio")
	appPid = placeholder.Process.Pid
	defer func() { startExe, appPid = oldExe, oldPid }()

	get := update.Getter(func(ctx context.Context, url string) (io.ReadCloser, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		return r.Body, nil
	})
	var downloaded, total int64
	err = installUpdate(context.Background(), get, m, key, from, target, func(d, tt int64) {
		downloaded, total = d, tt
	})
	if err != nil {
		t.Fatalf("安装更新失败：%v", err)
	}
	t.Logf("下载 %d/%d 字节", downloaded, total)

	// 停掉占位进程，让换包助手接手，然后等新版本到位。
	_ = placeholder.Process.Kill()
	_, _ = placeholder.Process.Wait()

	deadline := time.Now().Add(90 * time.Second)
	for {
		if v := bundleVersion(target); v == expect {
			t.Logf("换包完成，应用版本 %s", v)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待换包超时，当前版本 %q，期望 %q", bundleVersion(target), expect)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// bundleVersion 读 .app 的 Info.plist 里的版本号。
func bundleVersion(app string) string {
	out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleShortVersionString",
		filepath.Join(app, "Contents", "Info.plist")).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
