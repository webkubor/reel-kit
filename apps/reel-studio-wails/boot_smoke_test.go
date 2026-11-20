package main

import (
	"encoding/json"
	"os"
	"testing"
)

// 启动冒烟：按界面实际会走的绑定方法顺序，把引擎跑起来，
// 数据来自一个已绑定「沸腾之雪」的 studio.json（12 集 + 2 镜头）。
func TestBootSequenceAgainstSeededStudio(t *testing.T) {
	root := os.Getenv("REEL_BOOT_STUDIO")
	if root == "" {
		t.Skip("set REEL_BOOT_STUDIO to run")
	}
	s := openTestStore(t, root)
	e := &Engine{store: s}
	projects := e.ListProjects()
	if len(projects) != 1 {
		t.Fatalf("项目数 %d", len(projects))
	}
	p := projects[0]
	if p.WorkspaceName != "沸腾之雪" || p.WorkspaceID == "" {
		t.Fatalf("绑定没读出来：%+v", p)
	}
	eps := e.ListEpisodes(p.ID)
	if len(eps) != 12 {
		t.Fatalf("剧集数 %d", len(eps))
	}
	for i, ep := range eps {
		if ep.Number != i+1 {
			t.Fatalf("集数错序：%d", ep.Number)
		}
	}
	shots := e.ListShots(eps[0].ID)
	if len(shots) != 2 {
		t.Fatalf("镜头数 %d", len(shots))
	}
	if shots[1].Status != "interrupted" {
		t.Fatalf("待核对镜头：%s", shots[1].Status)
	}
	// Wails 就是把返回值 JSON 序列化给界面：这里验证序列化形状
	data, err := json.Marshal(shots)
	if err != nil {
		t.Fatal(err)
	}
	var asList []map[string]any
	if err := json.Unmarshal(data, &asList); err != nil {
		t.Fatal(err)
	}
	for _, m := range asList {
		for _, key := range []string{"id", "number", "title", "status"} {
			if _, ok := m[key]; !ok {
				t.Fatalf("镜头 JSON 缺字段 %s：%v", key, m)
			}
		}
	}
	// 重启后仍在出镜的镜头应当是待核对
	if cur, err := e.store.EpisodeShot(shots[0].ID); err == nil && cur.CharacterID == "" {
		t.Fatal("选角没读出来")
	}
}
