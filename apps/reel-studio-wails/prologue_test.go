package main

import "testing"

// 序章编号固定为 0，不被「自动取下一集」吞成第 N+1 集。
func TestPrologueEpisodeNumberZero(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("沸腾之雪", "series")
	if _, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Number: 1, Title: "雪夜传说"}); err != nil {
		t.Fatal(err)
	}
	ep, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Prologue: true, Title: "天下三分"})
	if err != nil {
		t.Fatal(err)
	}
	if ep.Number != 0 {
		t.Fatalf("序章编号应为 0：%d", ep.Number)
	}
	if _, err := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Prologue: true, Title: "又一个"}); err == nil {
		t.Fatal("一个项目只能有一个序章")
	}
	next, _ := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Title: "第二集"})
	if next.Number != 2 {
		t.Fatalf("自动编号应接在第一集后：%d", next.Number)
	}
	list := s.ListEpisodes(p.ID)
	if list[0].Number != 0 {
		t.Fatalf("序章应排最前：%d", list[0].Number)
	}
}

func TestShortTitleStripsProloguePrefix(t *testing.T) {
	cases := map[string]string{
		"【沸腾之雪·序章】天下三分：这场雪，开始沸腾":   "天下三分：这场雪，开始沸腾",
		"【沸腾之雪·第十二集·大结局】旧梦北荒：兵锋将至": "旧梦北荒：兵锋将至",
		"【沸腾之雪·第一集】雪夜传说":           "雪夜传说",
	}
	for in, want := range cases {
		if got := shortEpisodeTitle(in); got != want {
			t.Fatalf("%s → %s，应为 %s", in, got, want)
		}
	}
	if parseEpisodeNumber("【沸腾之雪·序章】天下三分") != 0 {
		t.Fatal("序章应识别为 0")
	}
}
