package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Test声音那节有三档调节
//
// 人声音量、侧链、响度这三项以前只在 CLI 里，桌面端看不到 ——
// 用户在界面���调不了，就只能改代码或跑命令行。
func Test声音那节有三档调节(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	for _, label := range []string{"人声音量", "人声时压低配乐", "统一响度"} {
		if !tt.HasText(label) {
			t.Errorf("声音这一节少了「%s」，实际文字：%q", label, tt.Texts())
		}
	}
}

// Test调整后再生成会把三档带回来
func Test调整后再生成会把三档带回来(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	a.applyConfig("talk", map[string]any{
		"kind": "talk", "title": "带调节的口播",
		"voiceGain": 1.6, "duck": false, "normalize": true,
	}, "j9")

	if a.talkAudio.voiceGain != "1.6" {
		t.Errorf("人声音量没有载入表单：%q", a.talkAudio.voiceGain)
	}
	if a.talkAudio.duck {
		t.Errorf("侧链开关没有载入表单：%v", a.talkAudio.duck)
	}
	if !a.talkAudio.normalize {
		t.Errorf("响度开关没有载入表单：%v", a.talkAudio.normalize)
	}
}

// TestBGM音量只换算一次
//
// 回归：项目推荐那条流水线以前在协程里又写了一遍 bgmGain，
// 而且写的是界面的裸百分比（12），把前面算好的 0.12 覆盖成了 12 ——
// 后端拿到 12 会直接报错或把配乐推爆。
func TestBGM音量只换算一次(t *testing.T) {
	gain, err := bgmGainValue("12")
	if err != nil {
		t.Fatalf("换算失败：%v", err)
	}
	if gain != 0.12 {
		t.Errorf("12%% 应当换算成 0.12，实际是 %v", gain)
	}
	if _, err := bgmGainValue("120"); err == nil {
		t.Errorf("超过 50%% 的值应当报错，而不是悄悄放行")
	}
	if gain, err := bgmGainValue(""); err != nil || gain != 0 {
		t.Errorf("留空时应当是 0，实际是 %v / %v", gain, err)
	}
}

// Test三档调节的值会进后端输入
func Test三档调节的值会进后端输入(t *testing.T) {
	a := newApp(t, stub(t, talkJobs()).URL)
	input := map[string]any{}
	knobs := audioKnobs{voiceGain: "1.8", duck: false, normalize: false}
	if err := a.applyAudioKnobs(input, knobs); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if input["voiceGain"] != 1.8 {
		t.Errorf("人声音量没进输入：%v", input["voiceGain"])
	}
	if input["duck"] != false || input["normalize"] != false {
		t.Errorf("开关没进输入：%v", input)
	}
	if err := a.applyAudioKnobs(map[string]any{}, audioKnobs{voiceGain: "9"}); err == nil {
		t.Errorf("9 倍人声音量应当报错")
	}
}

// Test配置面板能显示这三档
func Test配置面板能显示这三档(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	job := map[string]any{
		"voiceGain": 1.35, "duck": true, "normalize": false,
	}
	if got := a.configValue("talk", "voiceGain", job); got != "1.35 倍" {
		t.Errorf("人声音量显示不对：%q", got)
	}
	if got := a.configValue("talk", "duck", job); got != "开" {
		t.Errorf("侧链开关显示不对：%q", got)
	}
	if got := a.configValue("talk", "normalize", job); got != "关" {
		t.Errorf("响度开关显示不对：%q", got)
	}
}
