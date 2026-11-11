package main

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestFreePort给的是真空着的端口(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("挑端口失败：%v", err)
	}
	if port <= 0 {
		t.Fatalf("端口不合法：%d", port)
	}
	// 这个端口当下应该还空着：能重新监听到才算真的空。
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("端口 %d 不空：%v", port, err)
	}
	_ = l.Close()
}

func TestHasRuntime认得出完整的运行时目录(t *testing.T) {
	dir := t.TempDir()
	if hasRuntime(dir) {
		t.Fatal("空目录不该被当成完整运行时")
	}
	mkFile(t, filepath.Join(dir, "bin", "node"))
	if hasRuntime(dir) {
		t.Fatal("只有 node 不算完整运行时")
	}
	mkFile(t, filepath.Join(dir, "backend", "bin", "reel.mjs"))
	if !hasRuntime(dir) {
		t.Fatal("node 与后端都在，应该认得出")
	}
}

func TestPlatformKey对得上资源目录命名(t *testing.T) {
	want := runtime.GOOS + "-" + runtime.GOARCH
	if got := platformKey(); got != want {
		t.Fatalf("平台目录名不对：得到 %q，期望 %q", got, want)
	}
}

func mkFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败：%v", err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("建文件失败：%v", err)
	}
}
