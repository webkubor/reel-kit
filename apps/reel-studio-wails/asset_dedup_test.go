package main

import (
	"os"
	"sync"
	"testing"
)

// 同一张图重复导入只存一份。
func TestAddAssetDeduplicatesByHash(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("去重", "series")
	data := []byte("same-image-bytes")
	a1, err := s.AddAsset(p.ID, "第一次", "image/png", data, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	a2, err := s.AddAsset(p.ID, "第二次", "image/png", data, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if a1.ID != a2.ID {
		t.Fatalf("重复内容应是同一条：%s vs %s", a1.ID, a2.ID)
	}
	if len(s.State().Assets) != 1 {
		t.Fatalf("素材数 %d，应为 1", len(s.State().Assets))
	}
	// 磁盘上也只有一份
	files := 0
	ents, _ := os.ReadDir(s.Root() + "/media")
	for _, e := range ents {
		if !e.IsDir() {
			files++
		}
	}
	if files != 1 {
		t.Fatalf("磁盘上有 %d 个文件，应为 1", files)
	}
}

// 不同项目的同一张图各存一份：去重只在项目内。
func TestAddAssetDedupIsPerProject(t *testing.T) {
	s := newTestStore(t)
	p1, _ := s.AddProject("甲", "series")
	p2, _ := s.AddProject("乙", "series")
	data := []byte("shared")
	a1, _ := s.AddAsset(p1.ID, "甲的图", "image/png", data, map[string]any{})
	a2, _ := s.AddAsset(p2.ID, "乙的图", "image/png", data, map[string]any{})
	if a1.ID == a2.ID {
		t.Fatal("不同项目不该共用一条素材")
	}
	if len(s.State().Assets) != 2 {
		t.Fatalf("素材数 %d，应为 2", len(s.State().Assets))
	}
}

// 并发导入同一张图只能存一份。
//
// 查重到落账本必须全程持锁：否则各判一次「无重复」、各写一份。
func TestAddAssetConcurrentDedup(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("并发", "series")
	data := []byte("concurrent-image")
	const workers = 8
	var wg sync.WaitGroup
	ids := make([]string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a, err := s.AddAsset(p.ID, "并发导入", "image/png", data, map[string]any{})
			if err == nil {
				ids[i] = a.ID
			}
		}(i)
	}
	wg.Wait()
	if len(s.State().Assets) != 1 {
		t.Fatalf("并发导入存了 %d 份，应为 1", len(s.State().Assets))
	}
	for i, id := range ids {
		if id != "" && id != ids[0] {
			t.Fatalf("第 %d 个拿到不同 id：%s vs %s", i, id, ids[0])
		}
	}
}
