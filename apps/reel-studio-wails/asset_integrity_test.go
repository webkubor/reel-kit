package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 素材库完整性：登记的都要有文件，文件内容也要对得上登记的哈希。
//
// 素材是所有制作流程的地基：角色头像、场景参考、成片都用它。
// 文件丢了或被替换，界面还看得出「有 N 张」，实际打开却是坏的 ——
// 所以这里直接查磁盘。
func TestAssetStoreIntegrity(t *testing.T) {
	s := newTestStore(t)
	p, err := s.AddProject("完整性", "series")
	if err != nil {
		t.Fatal(err)
	}
	// 一张真的图（最小 PNG）
	png := []byte{
		0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
		0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
		0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00,
		0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60,
		0x82,
	}
	asset, err := s.AddAsset(p.ID, "测试图", "image/png", png, map[string]any{"source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	// 文件在
	if _, err := os.Stat(s.Path(asset)); err != nil {
		t.Fatalf("素材没落盘：%v", err)
	}
	// 内容哈希对得上
	data, err := os.ReadFile(s.Path(asset))
	if err != nil {
		t.Fatal(err)
	}
	if HashBytes(data) != asset.Hash {
		t.Fatal("登记的哈希与文件内容不符")
	}
	// 磁盘上的文件都在登记里
	registered := map[string]bool{}
	for _, a := range s.State().Assets {
		registered[a.File] = true
	}
	ents, err := os.ReadDir(filepath.Join(s.Root(), "media"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range ents {
		if !registered[ent.Name()] {
			t.Errorf("磁盘上有未登记的文件：%s", ent.Name())
		}
	}
	// 登记的都有文件
	for _, a := range s.State().Assets {
		if _, err := os.Stat(s.Path(a)); err != nil {
			t.Errorf("登记的素材缺文件：%s", a.Name)
		}
	}
}
