package main

import "testing"

// 同一集重复导入要更新而不是新增。
func TestImportEpisodesIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("导入", "series")
	e := &Engine{store: s}

	first, err := e.store.AddEpisode(EpisodeDraft{
		ProjectID: p.ID, Number: 1, Title: "雪夜传说",
		PublishURL: "https://youtu.be/old", PublishTitle: "旧标题",
		PublishPlatform: "youtube", VideoID: "oldid",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 模拟再导入一次：同集号、不同链接
	updated, err := e.store.UpdateEpisode(first.ID, func(x *Episode) {
		x.PublishURL, x.PublishTitle = "https://youtu.be/new", "新标题"
		x.PublishPlatform, x.VideoID = "youtube", "newid"
	})
	if err != nil {
		t.Fatal(err)
	}
	list := e.store.ListEpisodes(p.ID)
	if len(list) != 1 {
		t.Fatalf("重复导入产生了 %d 集，应为 1", len(list))
	}
	if updated.PublishURL != "https://youtu.be/new" || updated.VideoID != "newid" {
		t.Fatalf("链接没更新：%+v", updated)
	}
	// 用户自己改过的标题不该被覆盖
	custom, err := e.store.UpdateEpisode(first.ID, func(x *Episode) {
		x.Title = "我自己起的名字"
	})
	if err != nil {
		t.Fatal(err)
	}
	if custom.Title != "我自己起的名字" {
		t.Fatalf("用户改的标题被覆盖：%s", custom.Title)
	}
}

// 集号唯一：不能有两个第一集。
func TestEpisodeNumbersUnique(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("集号", "series")
	if _, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Number: 1, Title: "第一集"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Number: 1, Title: "又一个第一集"}); err == nil {
		t.Fatal("同集号不该允许两个")
	}
}
