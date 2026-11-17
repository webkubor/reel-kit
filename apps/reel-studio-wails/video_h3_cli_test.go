package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 下发给 CLI 的参数必须点名 H3、带规格、参考图逐张 --reference-image，且没有首帧。
func TestGenerateVideoPinsH3(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	bin := filepath.Join(dir, "museav")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > " + argsFile + "\necho https://x/y.mp4\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	c := &MuseavClient{Bin: bin, Workspace: "ws-snow"}
	url, err := c.GenerateVideo(context.Background(), h3Profile.Model, "p", "16:9", 15, []string{"u1", "u2"})
	if err != nil || url != "https://x/y.mp4" {
		t.Fatalf("url=%q err=%v", url, err)
	}
	data, _ := os.ReadFile(argsFile)
	got := strings.Join(strings.Split(strings.TrimSpace(string(data)), "\n"), " ")
	want := "gen --video --prompt p --model MiniMax H3 --ratio 16:9 --duration 15 --reference-image u1 --reference-image u2 --project ws-snow"
	if got != want {
		t.Fatalf("参数：\n%s\n期望：\n%s", got, want)
	}
}
