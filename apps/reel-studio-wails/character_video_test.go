package main

import (
	"strings"
	"testing"
)

// 参考图顺序：五视图在前（模型能拼出转面），裸图补后。
func TestCharacterReferenceURLsOrder(t *testing.T) {
	c := &Character{Assets: []CharacterAsset{
		{View: "", Name: "三视图", URL: "u-sheet"},
		{View: ViewBack, Name: "背", URL: "u-back"},
		{View: ViewFront, Name: "正", URL: "u-front"},
	}}
	got := characterReferenceURLs(c)
	want := []string{"u-front", "u-back", "u-sheet"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("顺序错：%v，期望 %v", got, want)
	}
}

// 一张图都没有要报「先补定妆稿」，而不是让中台跑一次必然失真的出镜。
func TestCharacterReferenceURLsEmpty(t *testing.T) {
	if got := characterReferenceURLs(&Character{}); len(got) != 0 {
		t.Fatalf("无图应为空，实际 %v", got)
	}
}

// 提示词三层：动作、角色卡、哲学。缺角色卡就退化为「以参考图为准」。
func TestCharacterVideoPromptLayers(t *testing.T) {
	c := &Character{Name: "慕北歌", Traits: []CharacterTrait{{Key: "发型", Value: "长发"}}}
	p := characterVideoPrompt(c, "雪巅枯坐", "【画面哲学】低调摄影。")
	for _, want := range []string{"雪巅枯坐", "发型：长发", "低调摄影", "不得改变"} {
		if !strings.Contains(p, want) {
			t.Fatalf("缺 %q：%s", want, p)
		}
	}
	bare := characterVideoPrompt(&Character{Name: "无名"}, "走路", "")
	if !strings.Contains(bare, "以参考图为准") {
		t.Fatalf("无设定时应退化为参考图为准：%s", bare)
	}
}

// mime 嗅探可能是 octet-stream，落库一律按 video/*，免得素材列表出现流类型。
func TestMimeOrVideo(t *testing.T) {
	if got := mimeOrVideo("application/octet-stream"); got != "video/mp4" {
		t.Fatalf("非视频 mime 应归一为 mp4，实际 %s", got)
	}
	if got := mimeOrVideo("video/webm"); got != "video/webm" {
		t.Fatalf("已是视频 mime 不该动，实际 %s", got)
	}
}

// 沸腾之雪的真实形态：全部 role=raw，名字是「X-三视图 / X-问天剑」。
// 这些图归不进五视图，但必须进出镜参考，且三视图排在兵器图前面。
func TestSubjectToCharacterKeepsRawDesignSheets(t *testing.T) {
	detail := SubjectDetail{
		Subject: Subject{ID: "s1", WorkspaceID: "ws-snow", Name: "慕北歌", Kind: "person"},
		Assets: []SubjectAsset{
			{ID: "a1", Role: "raw", Name: "慕北歌-问雪剑", URL: "u-sword1"},
			{ID: "a2", Role: "raw", Name: "慕北歌-问天剑", URL: "u-sword2"},
			{ID: "a3", Role: "raw", Name: "慕北歌-三视图", URL: "u-sheet"},
		},
	}
	c := subjectToCharacter(detail, "", "")
	if len(c.Assets) != 0 {
		t.Fatalf("组合稿不该被硬塞进五视图格子：%+v", c.Assets)
	}
	if len(c.References) != 3 {
		t.Fatalf("三张图都应进参考：%+v", c.References)
	}
	if c.Workspace != "ws-snow" {
		t.Fatalf("工作区应来自主体：%q", c.Workspace)
	}
	got := characterReferenceURLs(c)
	if len(got) != 3 || got[0] != "u-sheet" {
		t.Fatalf("三视图应排第一：%v", got)
	}
}

// 五视图 + 同 URL 重复时不重复提交，且总数封顶 9。
func TestCharacterReferenceURLsDedupAndCap(t *testing.T) {
	c := &Character{Assets: []CharacterAsset{{View: ViewFront, URL: "u0"}}}
	c.References = append(c.References, CharacterAsset{View: ViewFront, URL: "u0"})
	for i := 1; i <= 12; i++ {
		c.References = append(c.References, CharacterAsset{Name: "道具", URL: "u" + string(rune('a'+i))})
	}
	got := characterReferenceURLs(c)
	if len(got) != 9 || got[0] != "u0" {
		t.Fatalf("应去重并封顶 9：%v", got)
	}
}
