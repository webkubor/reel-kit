package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Test对比选择器能打开
//
// 踩过的坑：点「对比这条」时把 comparePick 设成了当前这条，
// 而选择器只在 comparePick != 当前这条时才画 —— 于是永远打不开。
// 这里点一次按钮，看选择器有没有出现。
func Test对比选择器能打开(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	if err := tt.Click("对比这条"); err != nil {
		t.Fatalf("点不到「对比这条」：%v", err)
	}
	tt.Frame()
	if !tt.HasText("和哪一条对比？") {
		t.Errorf("点了「对比这条」但没打开对比选择器，实际文字：%q", tt.Texts())
	}
}

// Test选一条对比之后能看配置差异
func Test选一条对比之后能看配置差异(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	// 对比选择器在详情下方，窗口得够高才够得着它。
	tt := ui.NewTester(a.View, 1280, 1400)
	tt.Frame()

	if err := tt.Click("对比这条"); err != nil {
		t.Fatalf("点不到「对比这条」：%v", err)
	}
	tt.Frame()
	if err := tt.Click("对比「第一条口播」"); err != nil {
		t.Fatalf("点不到要对比的那条：%v", err)
	}
	tt.Frame()
	if a.compareJobID != "j1" {
		t.Errorf("对比对象选错了：%s", a.compareJobID)
	}
	if !tt.HasText("配置对比") {
		t.Errorf("没有显示配置对比，实际文字：%q", tt.Texts())
	}
}

// Test配置回填覆盖每一条流水线的新字段
//
// 「按此配置再生成一条」要能真的把配置搬回表单：
// 少搬一个字段，用户改完再生成就丢了一项设置。
func Test配置回填覆盖每一条流水线的新字段(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)

	// 素材合成：标题、字幕、配音、版式都要搬回去
	a.applyConfig("export", map[string]any{
		"kind": "export", "title": "合成标题", "captions": "第一句\n第二句",
		"audioId": "a2", "template": "sticker-promo", "ratio": "9:16", "duration": 3.0,
		"assetIds": []any{"a1", "a3"},
	}, "j9")
	if a.expTitle != "合成标题" {
		t.Errorf("标题没有回填：%q", a.expTitle)
	}
	if a.expCaptions != "第一句\n第二句" {
		t.Errorf("字幕没有回填：%q", a.expCaptions)
	}
	if a.expAudioID != "a2" {
		t.Errorf("配音没有回填：%q", a.expAudioID)
	}
	if a.expTemplate != "sticker-promo" {
		t.Errorf("版式没有回填：%q", a.expTemplate)
	}
	if len(a.assetIDs) != 2 {
		t.Errorf("素材没有回填：%v", a.assetIDs)
	}

	// 项目推荐：逐镜配音与旁白音频也要回填
	a.applyConfig("recommendation", map[string]any{
		"kind": "recommendation", "title": "推荐标题", "captions": "一\n二",
		"voiceSource": "scenes", "voiceIds": []any{"a1", ""}, "narrationId": "a4",
		"bgmGain": 0.2,
	}, "j10")
	if a.recTitle != "推荐标题" {
		t.Errorf("标题没有回填：%q", a.recTitle)
	}
	if len(a.recVoiceIDs) != 2 || a.recVoiceIDs[0] != "a1" {
		t.Errorf("逐镜配音没有回填：%v", a.recVoiceIDs)
	}
	if a.recNarrationID != "a4" {
		t.Errorf("旁白音频没有回填：%q", a.recNarrationID)
	}
	// 音量在界面上是百分比，后端存的是 0–1 的比例
	if a.recBGMGain != "20" {
		t.Errorf("音量没有按百分比回填：%q", a.recBGMGain)
	}
}

// Test数字填错时拦住
//
// 以前 toFloat 会把「abc」变成 0，用户填错东西却看见「音量 0」。
// 现在要拦下来，并且说清楚哪里错、该填什么。
func Test数字填错时拦住(t *testing.T) {
	if _, err := parseNumberIn("BGM 音量", "abc", 0, 100); err == nil {
		t.Errorf("填了非数字应该被拦住")
	}
	if _, err := parseNumberIn("BGM 音量", "180", 0, 100); err == nil {
		t.Errorf("超出范围应该被拦住")
	}
	if _, err := parseNumberIn("BGM 音量", "12", 0, 100); err != nil {
		t.Errorf("正常数字不该被拦：%v", err)
	}
	// 后端存 0.12，界面上要显示 12
	if got := gainPercent(0.12); got != "12" {
		t.Errorf("音量换算不对：%s", got)
	}
}
