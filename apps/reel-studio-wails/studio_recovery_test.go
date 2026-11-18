package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 账本坏了要留底，不能覆盖 —— 素材文件还在 media/ 里，
// 覆盖掉原文件就再也拼不回来了。
func TestCorruptStateIsPreserved(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "media"), 0o700); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "studio.json")
	if err := os.WriteFile(bad, []byte("{这不是合法的 JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := NewStudioStoreRoot(root)
	if err == nil {
		t.Fatal("损坏的账本应当报错")
	}
	if !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("报错要说清是损坏：%v", err)
	}
	// 原文件被改名留底了，没被覆盖
	entries, _ := os.ReadDir(root)
	kept := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "studio.json.corrupt-") {
			kept = true
			data, _ := os.ReadFile(filepath.Join(root, e.Name()))
			if string(data) != "{这不是合法的 JSON" {
				t.Fatal("留底的内容不对")
			}
		}
	}
	if !kept {
		t.Fatal("损坏的账本没有留底")
	}
}

// 素材文件还在，即使账本没了也能重新登记 —— 素材库不该依赖账本才能恢复。
func TestAssetsSurviveStateLoss(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("素材源", "series")
	data := []byte("image-bytes")
	a, err := s.AddAsset(p.ID, "图", "image/png", data, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	path := s.Path(a)
	// 账本没了，文件还在
	if err := os.Remove(filepath.Join(s.Root(), "studio.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("素材文件不该被删：%v", err)
	}
	// 重新打开：项目与素材记录没了，但磁盘文件还在，可以重建
	s2, err := NewStudioStoreRoot(s.Root())
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.State().Assets) != 0 {
		t.Fatal("账本没了就该没有素材记录")
	}
	entries, _ := os.ReadDir(filepath.Join(s.Root(), "media"))
	kept := 0
	for _, e := range entries {
		if !e.IsDir() {
			kept++
		}
	}
	if kept != 1 {
		t.Fatalf("磁盘上的素材文件应还在，实际 %d 个", kept)
	}
}

// 账本丢了之后能从磁盘把素材补回来。
func TestRebuildAssetsFromDisk(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("重建", "series")
	a, err := s.AddAsset(p.ID, "顾栖月-头像", "image/png", []byte("bytes"), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	// 再造一个没登记的文件（模拟账本丢失前就在磁盘上的）
	orphan := filepath.Join(s.Root(), "media", "11111111-2222-3333-4444-555555555555-夜惊鸿-三视图.png")
	if err := os.WriteFile(orphan, []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 清掉账本里的素材记录，模拟账本丢了
	s.mu.Lock()
	s.state.Assets = nil
	s.mu.Unlock()

	out, err := s.RebuildAssetsFromDisk(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("应补回 2 个：%d", len(out))
	}
	names := map[string]bool{}
	for _, x := range out {
		names[x.Name] = true
	}
	if !names["顾栖月-头像"] {
		t.Fatalf("原名没还原：%v", names)
	}
	if !names["夜惊鸿-三视图"] {
		t.Fatalf("孤儿文件没补进来：%v", names)
	}
	if len(s.State().Assets) != 2 {
		t.Fatalf("账本里应有 2 条：%d", len(s.State().Assets))
	}
	// 文件名能被还原（供下一轮重建用）
	if got := recoverAssetName(a.File); got != "顾栖月-头像" {
		t.Fatalf("文件名还原：%s", got)
	}
}
