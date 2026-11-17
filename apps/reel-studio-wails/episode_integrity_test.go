package main

import (
	"sync"
	"testing"
)

func TestEpisodeShotProjectBoundaryAndCopies(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("甲")
	other, _ := s.AddProject("乙")
	ids := []string{"角色一"}
	ep, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Title: "第一集", CharacterIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	ids[0] = "污染"
	ep.CharacterIDs[0] = "污染"
	saved, _ := s.Episode(ep.ID)
	if saved.CharacterIDs[0] != "角色一" {
		t.Fatal("剧集数据被外部引用修改")
	}
	if _, err := s.AddShot(ShotInput{ProjectID: other.ID, EpisodeID: ep.ID}); err == nil {
		t.Fatal("接受了跨项目镜头")
	}
	lines := []string{"旁白"}
	shot, err := s.AddShot(ShotInput{ProjectID: p.ID, EpisodeID: ep.ID, Lines: lines})
	if err != nil {
		t.Fatal(err)
	}
	lines[0] = "污染"
	shot.Lines[0] = "污染"
	savedShot, _ := s.EpisodeShot(shot.ID)
	if savedShot.Lines[0] != "旁白" {
		t.Fatal("镜头数据被外部引用修改")
	}
}

func TestConcurrentEpisodeNumbers(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("短剧")
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Title: "剧集"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	seen := map[int]bool{}
	for _, ep := range s.ListEpisodes(p.ID) {
		if seen[ep.Number] {
			t.Fatalf("重复集数 %d", ep.Number)
		}
		seen[ep.Number] = true
	}
	if len(seen) != 12 {
		t.Fatalf("集数 %d", len(seen))
	}
}
