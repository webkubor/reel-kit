package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Test声音那节有三档调节
//
// 人声音量、侧链、响度这三项以前只在 CLI 里，桌面端看不到 ——
// 用户在界面上调不了，就只能改代码或跑命令行。
func Test声音那节有三档调节(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	texts := tt.Texts()
	for _, label := range []string{"人声音量", "人声时压低配乐", "统一响度"} {
		if !tt.HasText(label) {
			t.Errorf("声音这一节少了「%s」，实际文字：%q", label, texts)
		}
	}

	// 三档必须落在「声音」这一节里、且在「画面」之前 ——
	// 放错分节用户就找不到（这就是按七块分组的意义所在）。
	voiceAt, visualAt := indexOf(texts, "声音"), indexOf(texts, "画面")
	if voiceAt < 0 || visualAt < 0 {
		t.Fatalf("界面上找不到「声音」或「画面」分节：%q", texts)
	}
	for _, label := range []string{"人声音量", "人声时压低配乐", "统一响度"} {
		at := indexOf(texts, label)
		if at < voiceAt || at > visualAt {
			t.Errorf("「%s」不在声音分节里（位置 %d，声音 %d，画面 %d）", label, at, voiceAt, visualAt)
		}
	}
}

// indexOf 找一段文字第一次出现的位置，找不到返回 -1。
func indexOf(texts []string, want string) int {
	for i, s := range texts {
		if s == want {
			return i
		}
	}
	return -1
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

// Test预设会真的改配乐音量
//
// 预设名字里写着「配乐 8%」，但以前选预设并不改下面那个 BGM 音量输入框 ——
// 用户选了「口播清晰」，出来的片子配乐还是 12%。名字和实际不一致，
// 用户只能自己猜该改哪一栏。
func Test预设会真的改配乐音量(t *testing.T) {
	knobs := defaultAudioKnobs()
	bgm := "12"
	for _, c := range []struct {
		preset string
		gain   string
	}{
		{"clear-speech", "8"},
		{"ambient", "30"},
		{"default", "12"},
	} {
		applyAudioPreset(&knobs, c.preset, &bgm)
		if bgm != c.gain {
			t.Errorf("预设 %s 应当把配乐音量设成 %s，实际是 %s", c.preset, c.gain, bgm)
		}
	}
	// 预设同时要改人声与降乐，否则「一键」只改了一半。
	applyAudioPreset(&knobs, "clear-speech", &bgm)
	if knobs.voiceGain != "1.5" || !knobs.duck {
		t.Errorf("口播清晰预设应当把人声抬到 1.5 并开启降乐，实际是 %q / %v", knobs.voiceGain, knobs.duck)
	}
}

// Test镜内动效在画面分节里
//
// 转场是「镜与镜之间」，镜内动效是「这一镜里画面自己在动」，
// 两件事适用位置不同，放在同一节里用户会以为它们是同一个开关。
func Test镜内动效在画面分节里(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	texts := tt.Texts()
	if !tt.HasText("镜内动效") {
		t.Fatalf("画面这一节少了「镜内动效」，实际文字：%q", texts)
	}
	visualAt, layoutAt := indexOf(texts, "画面"), indexOf(texts, "版式")
	at := indexOf(texts, "镜内动效")
	if visualAt < 0 || layoutAt < 0 {
		t.Fatalf("界面上找不到「画面」或「版式」分节：%q", texts)
	}
	if at < visualAt || at > layoutAt {
		t.Errorf("「镜内动效」不在画面分节里（位置 %d，画面 %d，版式 %d）", at, visualAt, layoutAt)
	}
}

// Test镜内动效进后端输入并能读回来
func Test镜内动效进后端输入并能读回来(t *testing.T) {
	a := newApp(t, stub(t, talkJobs()).URL)
	input := map[string]any{}
	if err := a.applyAudioKnobs(input, audioKnobs{motion: "gentle"}); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if input["motion"] != "gentle" {
		t.Errorf("镜内动效没进输入：%v", input["motion"])
	}
	knobs := audioKnobsFromConfig(map[string]any{"motion": "gentle"})
	if knobs.motion != "gentle" {
		t.Errorf("镜内动效没有载入表单：%q", knobs.motion)
	}
	if got := a.configValue("talk", "motion", map[string]any{"motion": "gentle"}); got != "轻柔运镜" {
		t.Errorf("配置面板的镜内动效显示不对：%q", got)
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
		"preset": "clear-speech", "transition": "fade", "speed": 1.15,
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
	if got := a.configValue("talk", "preset", job); got != "口播清晰" {
		t.Errorf("预设显示不对：%q", got)
	}
	if got := a.configValue("talk", "transition", job); got != "淡入淡出" {
		t.Errorf("转场显示不对：%q", got)
	}
}

// Test预设与转场进后端输入
func Test预设与转场进后端输入(t *testing.T) {
	a := newApp(t, stub(t, talkJobs()).URL)
	input := map[string]any{}
	knobs := audioKnobs{
		preset:     "clear-speech",
		speed:      "1.15",
		transition: "fade",
		voiceGain:  "1.5",
		duck:       true,
		normalize:  true,
	}
	if err := a.applyAudioKnobs(input, knobs); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if input["preset"] != "clear-speech" {
		t.Errorf("预设没进输入：%v", input["preset"])
	}
	if input["speed"] != 1.15 {
		t.Errorf("语速没进输入：%v", input["speed"])
	}
	if input["transition"] != "fade" {
		t.Errorf("转场没进输入：%v", input["transition"])
	}
}
