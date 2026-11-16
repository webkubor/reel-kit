package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// stubMuseav 写一个只回一行固定 UUID 的假中台 CLI，返回它的路径。
//
// Windows 上不能放无扩展名的 `#!/bin/sh` 脚本 —— 那个 shebang 只有
// Unix 内核认，Windows 的 exec 会报「executable file not found in %PATH%」。
// 客户端对 `.cmd` 有专门分支（cmd /c），所以这里按平台写对应形态：
// Unix 写 shell 脚本，Windows 写 .cmd 批处理。
func stubMuseav(t *testing.T, stdout string) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		bin := filepath.Join(dir, "museav.cmd")
		body := "@echo off\r\necho " + stdout + "\r\n"
		if err := os.WriteFile(bin, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
		return bin
	}
	bin := filepath.Join(dir, "museav")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '"+stdout+"\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestCreateSubjectPlainIDReceipt(t *testing.T) {
	bin := stubMuseav(t, "3f38d8c8-ad41-408e-b075-a3fde1474618")
	client := &MuseavClient{Bin: bin}
	s, err := client.CreateSubject(context.Background(), "角色", "person", "real", map[string]string{"发型": "长发"})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "3f38d8c8-ad41-408e-b075-a3fde1474618" || s.Traits["发型"] != "长发" {
		t.Fatalf("unexpected subject: %+v", s)
	}
}

func TestCharacterPromptsAreSingleBatchLines(t *testing.T) {
	s := &Subject{Traits: map[string]any{"发型": "长发"}}
	for _, view := range CharacterViews {
		p := characterViewPrompt(s, CharacterCreateInput{}, view)
		for _, r := range p {
			if r == '\n' || r == '\r' {
				t.Fatalf("%s prompt contains line break", view)
			}
		}
	}
}
