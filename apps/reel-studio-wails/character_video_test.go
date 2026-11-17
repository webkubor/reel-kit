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
	got := characterReferenceURLs(c, "")
	want := []string{"u-front", "u-back", "u-sheet"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("顺序错：%v，期望 %v", got, want)
	}
}

// 一张图都没有要报「先补定妆稿」，而不是让中台跑一次必然失真的出镜。
func TestCharacterReferenceURLsEmpty(t *testing.T) {
	if got := characterReferenceURLs(&Character{}, ""); len(got) != 0 {
		t.Fatalf("无图应为空，实际 %v", got)
	}
}

// 提示词三层：动作、角色卡、哲学。缺角色卡就退化为「以参考图为准」。
func TestCharacterVideoPromptLayers(t *testing.T) {
	c := &Character{Name: "慕北歌", Traits: []CharacterTrait{{Key: "发型", Value: "长发"}}}
	p := h3ShotPrompt(c, "", "雪巅枯坐", shotSpec{Ratio: "16:9", Duration: 15}, nil, "【画面哲学】低调摄影。")
	for _, want := range []string{"雪巅枯坐", "发型：长发", "低调摄影", "不得改变"} {
		if !strings.Contains(p, want) {
			t.Fatalf("缺 %q：%s", want, p)
		}
	}
	bare := h3ShotPrompt(&Character{Name: "无名"}, "", "走路", shotSpec{Ratio: "16:9", Duration: 5}, nil, "")
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
	got := characterReferenceURLs(c, "")
	// 角色卡没写【兵器名】、名下又有两把剑：分不清本命兵器就一把都不送，
	// 免得模型把另一把剑画进手里。
	if len(got) != 1 || got[0] != "u-sheet" {
		t.Fatalf("应只送三视图：%v", got)
	}
}

// 五视图 + 同 URL 重复时不重复提交，且总数封顶 9。
func TestCharacterReferenceURLsDedupAndCap(t *testing.T) {
	c := &Character{Assets: []CharacterAsset{{View: ViewFront, URL: "u0"}}}
	c.References = append(c.References, CharacterAsset{View: ViewFront, URL: "u0"})
	for i := 1; i <= 12; i++ {
		c.References = append(c.References, CharacterAsset{Name: "道具", URL: "u" + string(rune('a'+i))})
	}
	got := characterReferenceURLs(c, "")
	if len(got) != 9 || got[0] != "u0" {
		t.Fatalf("应去重并封顶 9：%v", got)
	}
}

// 夜惊鸿的真实形态：两套定妆稿（默认 / 王服）+ 一把枪 + 面具。
// 选哪套就只送哪套，另一套一张不送，免得模型把两套衣服糅成一套。
func TestCharacterReferenceURLsPerLook(t *testing.T) {
	c := &Character{
		Name:   "夜惊鸿",
		Traits: []CharacterTrait{{Key: "武器", Value: "惊鸿枪（九尺三寸银白透甲枪）"}},
		References: []CharacterAsset{
			{Name: "夜惊鸿-面具", URL: "u-mask"},
			{Name: "夜惊鸿-惊鸿枪", URL: "u-spear"},
			{Name: "夜惊鸿-王服三视图", URL: "u-royal"},
			{Name: "夜惊鸿-三视图", URL: "u-default"},
		},
	}
	looks := characterLooks(c)
	if len(looks) != 2 || looks[0] != DefaultLook || looks[1] != "王服" {
		t.Fatalf("装扮应为 [默认 王服]：%v", looks)
	}
	got := strings.Join(characterReferenceURLs(c, ""), ",")
	if got != "u-default,u-spear,u-mask" {
		t.Fatalf("默认装扮参考图错：%s", got)
	}
	got = strings.Join(characterReferenceURLs(c, "王服"), ",")
	if got != "u-royal,u-spear,u-mask" {
		t.Fatalf("王服参考图错：%s", got)
	}
}

// 本命兵器按【】名匹配，别人的兵器图不送。
func TestCharacterReferenceURLsOwnWeaponOnly(t *testing.T) {
	c := &Character{
		Name:   "慕北歌",
		Traits: []CharacterTrait{{Key: "武器", Value: "至尊汉剑【问天】"}},
		References: []CharacterAsset{
			{Name: "慕北歌-问雪剑", URL: "u-wenxue"},
			{Name: "慕北歌-问天剑", URL: "u-wentian"},
			{Name: "慕北歌-三视图", URL: "u-sheet"},
		},
	}
	got := strings.Join(characterReferenceURLs(c, ""), ",")
	if got != "u-sheet,u-wentian" {
		t.Fatalf("应只送三视图 + 问天：%s", got)
	}
}

func TestLookOf(t *testing.T) {
	cases := map[string]string{"夜惊鸿-三视图": DefaultLook, "夜惊鸿-王服三视图": "王服", "夜惊鸿 · 雨夜定妆": "雨夜"}
	for in, want := range cases {
		if got := lookOf("夜惊鸿", in); got != want {
			t.Fatalf("%s → %s，期望 %s", in, got, want)
		}
	}
}

// H3 规格：用户 > 项目出片规格 > 兜底；时长夹在 4–15 秒，非法画幅不下发。
func TestH3ProfileResolve(t *testing.T) {
	proj := Philosophy{"出片规格": map[string]any{"画幅": "16:9", "时长": "15 秒"}}
	if got := h3Profile.resolve("", 0, proj); got != (shotSpec{Ratio: "16:9", Duration: 15}) {
		t.Fatalf("应取项目规格：%+v", got)
	}
	if got := h3Profile.resolve("9:16", 6, proj); got != (shotSpec{Ratio: "9:16", Duration: 6}) {
		t.Fatalf("用户填的优先：%+v", got)
	}
	if got := h3Profile.resolve("2:1", 30, nil); got != (shotSpec{Ratio: "16:9", Duration: 15}) {
		t.Fatalf("非法画幅兜底、时长封顶 15：%+v", got)
	}
	if got := h3Profile.resolve("", 2, nil); got.Duration != 4 {
		t.Fatalf("时长下限 4：%+v", got)
	}
}

// H3 提示词结构：规格在前、参考图逐张点名分工、动作与锁脸在哲学之前。
func TestH3ShotPromptStructure(t *testing.T) {
	c := &Character{
		Name:   "夜惊鸿",
		Traits: []CharacterTrait{{Key: "发型", Value: "高马尾"}, {Key: "武器", Value: "银白透甲枪【惊鸿】"}},
		References: []CharacterAsset{
			{Name: "夜惊鸿-王服三视图", URL: "u-royal"},
			{Name: "夜惊鸿-三视图", URL: "u-default"},
			{Name: "夜惊鸿-惊鸿枪", URL: "u-spear"},
		},
	}
	refs := characterReferences(c, "王服")
	p := h3ShotPrompt(c, "王服", "雨夜长街，单枪挑灯", shotSpec{Ratio: "16:9", Duration: 15}, refs, "【画面哲学】低调摄影。")
	order := []string{"【规格】15 秒，16:9", "图1：夜惊鸿的定妆三视图（王服装扮）", "图2：夜惊鸿的本命兵器（惊鸿枪）", "【动作】雨夜长街", "装扮：王服", "发型：高马尾", "低调摄影"}
	last := -1
	for _, w := range order {
		i := strings.Index(p, w)
		if i < 0 || i < last {
			t.Fatalf("缺 %q 或顺序错：\n%s", w, p)
		}
		last = i
	}
	if strings.Contains(p, "u-default") || len(refs) != 2 {
		t.Fatalf("默认装扮不该送：%+v", refs)
	}
}

// 超 7000 字只截画面口径，动作和锁脸约束不能被截掉。
func TestH3ShotPromptCap(t *testing.T) {
	p := h3ShotPrompt(&Character{Name: "甲"}, "", "拔剑", shotSpec{Ratio: "16:9", Duration: 5}, nil, strings.Repeat("雪", 9000))
	if n := len([]rune(p)); n > h3Profile.MaxPrompt {
		t.Fatalf("超长：%d", n)
	}
	if !strings.Contains(p, "【动作】拔剑") || !strings.Contains(p, "不得改变") {
		t.Fatalf("核心约束被截：%s", p[:200])
	}
}

// 选了非默认装扮：文字里的默认衣着要让位给该装扮的参考图。
func TestH3ShotPromptLookOverridesOutfitText(t *testing.T) {
	c := &Character{Name: "夜惊鸿", Traits: []CharacterTrait{{Key: "外貌", Value: "银色面具，黑金交织战袍"}}}
	royal := h3ShotPrompt(c, "王服", "回望", shotSpec{Ratio: "16:9", Duration: 15}, nil, "")
	if !strings.Contains(royal, "本镜一律以「王服」定妆参考图为准") {
		t.Fatalf("王服应声明衣着以参考图为准：%s", royal)
	}
	def := h3ShotPrompt(c, DefaultLook, "回望", shotSpec{Ratio: "16:9", Duration: 15}, nil, "")
	if strings.Contains(def, "仅适用于默认装扮") {
		t.Fatalf("默认装扮不该加覆盖声明：%s", def)
	}
}
