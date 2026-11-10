package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// audioKnobs 是三条流水线（口播 / 项目推荐 / 产品介绍）共用的声音与节奏调节。
//
// 为什么用结构体而不是平铺字段：这些旋钮在三条流水线里含义完全一样，
// 进后端也是同一组参数（preset / speed / transition / voiceGain / duck / normalize）。
// 写成结构体，三处表单共用同一段画法与同一段组装代码 —— 少抄三遍也就少错三遍。
//
// 默认值与取值范围与后端 src/settings.mjs 里的 SETTING_SCHEMA 一致。
type audioKnobs struct {
	preset     string // 预设：default / clear-speech / ambient / raw
	speed      string // 念白语速倍数，如 "1.0"
	transition string // 切镜转场动效，如 "none", "fade", "slide-left"
	motion     string // 镜内动效，如 "none", "gentle"
	voiceGain  string // 人声音量倍数，如 "1.35"
	duck       bool   // 人声一开口是否把配乐压下去
	normalize  bool   // 是否把成品响度归一到固定值
}

// 默认值。后端也会兜底，但界面上先给一个能看见的数，用户才知道有这几个旋钮。
func defaultAudioKnobs() audioKnobs {
	return audioKnobs{
		preset:     "default",
		speed:      "1.0",
		transition: "none",
		motion:     "gentle",
		voiceGain:  "1.35",
		duck:       true,
		normalize:  true,
	}
}

// presetBGMLink 是「混音预设」联动到配乐音量时用的对应关系。
//
// 预设的名字里写着配乐音量（如「口播清晰 · 配乐 8%」），
// 但以前选预设并不会真的改下面那个 BGM 音量输入框 —— 名字与实际不一致。
var presetBGMLink = map[string]string{
	"default":     "12",
	"clear-speech": "8",
	"ambient":     "30",
	"raw":         "12",
}

// applyAudioPreset 把一个预设套到调节项上，并联动配乐音量。
//
// 预设是一整套声音关系：人声多响、配乐多响、要不要压配乐。
// 只改其中一项，用户看到的预设名字就和实际声音对不上。
func applyAudioPreset(k *audioKnobs, id string, bgmGain *string) {
	k.preset = id
	switch id {
	case "clear-speech":
		k.voiceGain = "1.5"
		k.duck = true
		k.normalize = true
	case "ambient":
		k.voiceGain = "1.1"
		k.duck = true
		k.normalize = true
	case "raw":
		k.voiceGain = "1"
		k.duck = false
		k.normalize = false
	case "default":
		k.voiceGain = "1.35"
		k.duck = true
		k.normalize = true
	}
	if bgmGain != nil {
		if value, ok := presetBGMLink[id]; ok {
			*bgmGain = value
		}
	}
}

// viewAudioKnobs 画「声音与节奏」那一节里的调节项。
//
// bgmGain 是配乐音量输入框的绑定值：选了预设要真的改它，
// 否则「预设」只是名字，不是一键联动。
func (a *App) viewAudioKnobs(c *ui.Context, k *audioKnobs, bgmGain *string) {
	presetChoices := []choice{
		{"default", "标准平衡（人声 1.35x · 配乐 12% · 智能降乐）"},
		{"clear-speech", "口播清晰（人声 1.5x · 配乐 8% · 深度降乐）"},
		{"ambient", "氛围铺底（人声 1.1x · 配乐 30% · 轻微降乐）"},
		{"raw", "原样输出（不压配乐 · 不归一化）"},
	}
	a.pick(c, "混音预设", "一键套用常用声音组合，选后下方数值仍可微调", presetChoices, k.preset, func(id string) {
		applyAudioPreset(k, id, bgmGain)
	})

	speedChoices := []choice{
		{"0.85", "0.85x 沉稳深情（较慢）"},
		{"1.0", "1.0x 标准正常（推荐）"},
		{"1.15", "1.15x 轻快利落（稍快）"},
		{"1.25", "1.25x 紧凑高效（快节奏）"},
	}
	a.pick(c, "念白语速", "0.75–1.5 倍速（支持 Edge 在线语音与 MiniMax）", speedChoices, k.speed, func(id string) {
		k.speed = id
	})

	a.kit.Field(c, "人声音量", "0.4–3 倍。念白被配乐盖住就调大，人声太冲就调小", func() {
		a.kit.Input(c, &k.voiceGain, "1.35")
	})
	a.kit.Field(c, "人声时压低配乐", "人声一开口就把配乐压下去，人声停了抬回来", func() {
		a.kit.Checkbox(c, &k.duck, "开启侧链压低")
	})
	a.kit.Field(c, "统一响度", "把成品推到固定响度，不同片子放一起不会忽轻忽响", func() {
		a.kit.Checkbox(c, &k.normalize, "开启响度归一化")
	})
}

// viewPictureKnobs 画「画面」那一节里跟画面怎么动有关的两个开关。
//
// 为什么从声音那一节挪过来：转场是「镜与镜之间」，镜内动效是「这一镜里画面自己在动」，
// 两件都是画面的事。放在「声音与节奏」里，用户按「我要改的是画面」去找会找不到。
func (a *App) viewPictureKnobs(c *ui.Context, k *audioKnobs) {
	transitionChoices := []choice{
		{"none", "硬切（无转场）"},
		{"fade", "淡入淡出（柔和过渡）"},
		{"slide-left", "向左推移（流畅连贯）"},
		{"slide-right", "向右推移"},
		{"slide-up", "向上推移"},
		{"slide-down", "向下推移"},
		{"wipe-left", "向左擦除"},
		{"wipe-right", "向右擦除"},
		{"dissolve", "交叉溶解"},
		{"zoom-in", "镜头推进（动感聚焦）"},
	}
	a.pick(c, "切镜转场", "分镜切换时的过渡动效", transitionChoices, k.transition, func(id string) {
		k.transition = id
	})
	motionChoices := []choice{
		{"none", "静态画面（生成更快）"},
		{"gentle", "轻柔运镜（图片缓慢推进、字幕入场）"},
	}
	a.pick(c, "镜内动效", "同一镜里画面自己动起来；逐帧渲染，生成时间更长", motionChoices, k.motion, func(id string) {
		k.motion = id
	})
}

// applyAudioKnobs 把这些调节写进后端要的 input。
func (a *App) applyAudioKnobs(input map[string]any, k audioKnobs) error {
	if k.preset != "" {
		input["preset"] = k.preset
	}
	if k.speed != "" {
		spd, err := parseNumberIn("念白语速", k.speed, 0.75, 1.5)
		if err != nil {
			return err
		}
		input["speed"] = spd
	}
	if k.transition != "" {
		input["transition"] = k.transition
	}
	if k.motion != "" {
		input["motion"] = k.motion
	}
	if k.voiceGain != "" {
		gain, err := parseNumberIn("人声音量", k.voiceGain, 0.4, 3)
		if err != nil {
			return err
		}
		input["voiceGain"] = gain
	}
	input["duck"] = k.duck
	input["normalize"] = k.normalize
	return nil
}

// audioKnobsFromConfig 从配置里读回调节参数（调整生成时用）。
func audioKnobsFromConfig(config map[string]any) audioKnobs {
	k := defaultAudioKnobs()
	if v, ok := config["preset"].(string); ok && v != "" {
		k.preset = v
	}
	if v, ok := config["speed"]; ok {
		if spd, err := parseNumberIn("念白语速", fmt.Sprintf("%v", v), 0.75, 1.5); err == nil {
			k.speed = fmt.Sprintf("%g", spd)
		}
	}
	if v, ok := config["transition"].(string); ok && v != "" {
		k.transition = v
	}
	if v, ok := config["motion"].(string); ok && v != "" {
		k.motion = v
	}
	if v, ok := config["voiceGain"]; ok {
		if gain, err := parseNumberIn("人声音量", fmt.Sprintf("%v", v), 0.4, 3); err == nil {
			k.voiceGain = fmt.Sprintf("%g", gain)
		}
	}
	if v, ok := config["duck"].(bool); ok {
		k.duck = v
	}
	if v, ok := config["normalize"].(bool); ok {
		k.normalize = v
	}
	return k
}

// bgmGainValue 把界面上的百分比换成后端要的比例（12% → 0.12）。
func bgmGainValue(raw string) (float64, error) {
	if raw == "" {
		return 0, nil
	}
	pct, err := parseNumberIn("配乐音量", raw, 0, 50)
	if err != nil {
		return 0, err
	}
	return pct / 100, nil
}
