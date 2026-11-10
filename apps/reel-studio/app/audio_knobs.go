package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// audioKnobs 是三条流水线（口播 / 项目推荐 / 产品介绍）共用的声音调节。
//
// 为什么用结构体而不是三个平铺字段：这三项在三条流水线里含义完全一样，
// 进后端也是同一组参数（voiceGain / duck / normalize）。写成结构体，
// 三处表单共用同一段画法与同一段组装代码 —— 少抄三遍也就少错三遍。
//
// 默认值与取值范围与后端 src/settings.mjs 里的 SETTING_SCHEMA 一致。
// 两边各写一份必然会漂，改一处要记得改另一处，是最容易忘的事。
type audioKnobs struct {
	voiceGain string // 人声音量倍数，如 "1.35"
	duck      bool   // 人声一开口是否把配乐压下去
	normalize bool   // 是否把成品响度归一到固定值
}

// 默认值。后端也会兜底，但界面上先给一个能看见的数，用户才知道有这几个旋钮。
func defaultAudioKnobs() audioKnobs {
	return audioKnobs{voiceGain: "1.35", duck: true, normalize: true}
}

// viewAudioKnobs 画「声音」那一节里的三档调节。
//
// 放在「声音」而不是「配乐」：这三项改的都是人声与配乐的相对关系，
// 用户找的时候想的是「念白太闷」而不是「BGM 音量」。
func (a *App) viewAudioKnobs(c *ui.Context, k *audioKnobs) {
	a.kit.Field(c, "人声音量", "0.4–3 倍。念白被配乐盖住就调大，人声太冲就调小", func() {
		a.kit.Input(c, &k.voiceGain, "1.35")
	})
	a.kit.Field(c, "人声时压低配乐", "人声一开口就把配乐压下去，人声停了抬回来", func() {
		a.kit.Checkbox(c, &k.duck, "开启侧链压低")
	})
	a.kit.Field(c, "统一响度", "把成品推到固定响度，不同片子放一起不会一轻一重", func() {
		a.kit.Checkbox(c, &k.normalize, "开启响度归一化")
	})
}

// applyAudioKnobs 把这三档写进后端要的 input。
//
// 值非法时直接报错，不悄悄改数：用户填了 5 倍人声音量，
// 默默拉回 3 倍会让人以为自己已经调过了。
func (a *App) applyAudioKnobs(input map[string]any, k audioKnobs) error {
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

// audioKnobsFromConfig 从配置里读回这三档（调整生成时用）。
//
// 配置里存的是数字，这里要变回界面上的字符串。
func audioKnobsFromConfig(config map[string]any) audioKnobs {
	k := defaultAudioKnobs()
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
//
// 三条流水线共用这一处换算。以前各写一遍，其中一处（项目推荐）
// 在协程里又写了一遍裸百分比，把这里算好的 0.12 覆盖成了 12 ——
// 后端拿到 12 会把配乐推爆。
func bgmGainValue(raw string) (float64, error) {
	if raw == "" {
		return 0, nil
	}
	gain, err := parseNumberIn("BGM 音量", raw, 0, 50)
	if err != nil {
		return 0, err
	}
	return gain / 100, nil
}
