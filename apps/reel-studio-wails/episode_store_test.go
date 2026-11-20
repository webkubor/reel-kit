package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 老工作台（studio.json 里没有 episodes/shots）必须照常打开：
// 换层级不该让存了项目的机器打不开应用。
func TestStoreLoadsLegacyStateWithoutEpisodes(t *testing.T) {
	root := t.TempDir()
	legacy := map[string]any{
		"projects": []map[string]any{{"id": "p1", "name": "旧项目", "createdAt": "2026-01-01T00:00:00.000Z"}},
		"assets":   []map[string]any{},
		"jobs":     []map[string]any{},
		"voices":   []map[string]any{},
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(root, "studio.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t, root)
	if len(s.State().Projects) != 1 {
		t.Fatalf("老项目应还在：%+v", s.State().Projects)
	}
	if s.State().Episodes == nil || s.State().Shots == nil {
		t.Fatal("剧集/镜头列表不该是 nil")
	}
	if len(s.State().Episodes) != 0 || len(s.State().Shots) != 0 {
		t.Fatal("老数据不该凭空长出剧集")
	}
}

// 剧集按集数排序；镜号在本集内递增。
func TestEpisodeAndShotNumbering(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("短剧", "series")

	ep3, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Number: 3, Title: "第三集"})
	if err != nil {
		t.Fatal(err)
	}
	if ep3.Number != 3 {
		t.Fatalf("集数应为 3：%d", ep3.Number)
	}
	// 不给集数 → 接在已有集数后面
	epNext, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Title: "新的一集"})
	if err != nil {
		t.Fatal(err)
	}
	if epNext.Number <= ep3.Number {
		t.Fatalf("新集数应更大：%d vs %d", epNext.Number, ep3.Number)
	}

	eps := s.ListEpisodes(p.ID)
	if len(eps) != 2 || eps[0].Number > eps[1].Number {
		t.Fatalf("剧集应按集数排序：%+v", eps)
	}

	sh1, _ := s.AddShot(ShotInput{ProjectID: p.ID, EpisodeID: ep3.ID, Title: "开场"})
	sh2, _ := s.AddShot(ShotInput{ProjectID: p.ID, EpisodeID: ep3.ID, Title: "回望"})
	if sh1.Number != 1 || sh2.Number != 2 {
		t.Fatalf("镜号应递增：%d %d", sh1.Number, sh2.Number)
	}
	shots := s.ListShots(ep3.ID)
	if len(shots) != 2 || shots[0].Title != "开场" {
		t.Fatalf("镜头列表不对：%+v", shots)
	}
}

// 镜头状态与素材挂载要能改并落盘。
func TestEpisodeShotUpdatePersists(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("短剧", "series")
	ep, _ := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Title: "第一集"})
	sh, _ := s.AddShot(ShotInput{ProjectID: p.ID, EpisodeID: ep.ID, Title: "长街回望"})

	if _, err := s.UpdateShot(sh.ID, func(x *EpisodeShot) {
		x.Status = "shot"
		x.ClipAssetID = "asset-1"
		x.Action = "雨夜长街，缓步前行"
	}); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, s.Root())
	got, err := reopened.EpisodeShot(sh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "shot" || got.ClipAssetID != "asset-1" || got.Action != "雨夜长街，缓步前行" {
		t.Fatalf("改动没落盘：%+v", got)
	}
}

// 剧集要属于存在的项目。
func TestAddEpisodeRequiresProject(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.AddEpisode(EpisodeDraft{ProjectID: "不存在", Title: "第一集"}); err == nil {
		t.Fatal("项目不存在应报错")
	}
	if _, err := s.AddEpisode(EpisodeDraft{ProjectID: s.Root(), Title: "  "}); err == nil {
		t.Fatal("空集名应报错")
	}
}
