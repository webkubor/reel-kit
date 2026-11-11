package main

import "fmt"

// 混音链 —— 人声、配乐怎么合成一条最终音轨。
//
// 为什么单独抽出来：concat 路径（无转场）与 xfade 路径（有转场）本来各写了一份
// amix，两份迟早会走偏。现在两条路径共用同一段滤镜链，参数只在这里定义一次。

// 目标综合响度（LUFS）。
const TargetLUFS = -14

// 目标真峰（dBTP）。
const TargetTP = -1.0

// 统一到的采样率与声道数。
const MixSampleRate = 48000

// 送进混音链的统一格式：48 kHz 立体声、浮点 planar。
const MixFormat = `aformat=sample_fmts=fltp:sample_rates=48000:channel_layouts=stereo`

// 人声默认增益。
const DefaultVoiceGain = 1.35

// 人声高通：砍掉 110 Hz 以下的隆隆声与手持噪声。
const VoiceHighPass = 110

// 配乐侧链参数。
const (
	DuckThreshold = 0.030
	DuckRatio     = 8
	DuckAttack    = 4
	DuckRelease   = 320
)

// 限幅：兜底，防止叠加后削顶。
const (
	Limit        = 0.891 // ≈ -1.0 dBFS，与 TargetTP 对齐
	LimitAttack  = 5
	LimitRelease = 50
)

// BuildMixChain 构造混音滤镜链。
//
// @param voiceLabel 人声输入标签，如 "[voice]"；没有就传 ""
// @param bgmLabel 配乐输入标签，如 "[bgm]"；没有就传 ""
// @param voiceGain 人声增益，默认 DefaultVoiceGain
// @param duck 人声是否侧链压低配乐，默认 true
// @param normalize 是否做响度归一化，默认 true
// @param outLabel 输出标签，默认 "[aout]"
// @returns ffmpeg -filter_complex 的片段
func BuildMixChain(voiceLabel, bgmLabel string, voiceGain float64, duck, normalize bool, outLabel string) []string {
	if outLabel == "" {
		outLabel = "[aout]"
	}
	var filters []string
	var mixIn []string

	useDuck := duck && voiceLabel != "" && bgmLabel != ""

	// 增益要是个数：NaN 或负数会让整条链静默失灵。
	gain := voiceGain
	if gain <= 0 || gain != gain { // NaN check
		gain = DefaultVoiceGain
	}

	var voiceOut string
	var duckKey string

	if voiceLabel != "" {
		// 统一到 48 kHz 立体声：单声道念白由 aformat 补成双声道，与配乐对齐。
		head := fmt.Sprintf("%saresample=48000,%s,highpass=f=110,volume=%g", voiceLabel, MixFormat, gain)
		if useDuck {
			// 分两路：一路进混音，一路当侧链的触发信号。
			filters = append(filters, fmt.Sprintf("%s,asplit=2[voxA][voxS]", head))
			voiceOut = "[voxA]"
			duckKey = "[voxS]"
		} else {
			filters = append(filters, fmt.Sprintf("%s[voxA]", head))
			voiceOut = "[voxA]"
		}
	}

	var bgmOut string
	if bgmLabel != "" {
		filters = append(filters, fmt.Sprintf("%saresample=48000,%s[bgmR]", bgmLabel, MixFormat))
		if useDuck && duckKey != "" {
			filters = append(filters, fmt.Sprintf(
				"[bgmR]%ssidechaincompress=threshold=%g:ratio=%d:attack=%d:release=%d:level_sc=1[bgmD]",
				duckKey, DuckThreshold, DuckRatio, DuckAttack, DuckRelease,
			))
		} else {
			filters = append(filters, "[bgmR]anull[bgmD]")
		}
		bgmOut = "[bgmD]"
	}

	if voiceOut != "" {
		mixIn = append(mixIn, voiceOut)
	}
	if bgmOut != "" {
		mixIn = append(mixIn, bgmOut)
	}

	if len(mixIn) == 0 {
		// 没有任何音频输入：出等长静音，音轨时长由调用方的 -shortest 决定。
		label := outLabel[1 : len(outLabel)-1] // strip []
		filters = append(filters, fmt.Sprintf("anullsrc=r=48000:cl=stereo[%s]", label))
		return filters
	}

	if len(mixIn) == 1 {
		filters = append(filters, fmt.Sprintf("%sanull[mixed]", mixIn[0]))
	} else {
		// 不写 normalize=0：旧版 ffmpeg 没这个选项，写了直接报 "Option not found"。
		// amix 默认把每路乘 1/N，后面再 volume=N 乘回来。
		filters = append(filters, fmt.Sprintf(
			"%samix=inputs=%d:duration=first:dropout_transition=0,volume=%d[mixed]",
			stringsJoin(mixIn, ""), len(mixIn), len(mixIn),
		))
	}

	if normalize {
		filters = append(filters, fmt.Sprintf("[mixed]loudnorm=I=%d:TP=%.1f:LRA=11[ln]", TargetLUFS, TargetTP))
		filters = append(filters, "[ln]aresample=48000[lnr]")
		filters = append(filters, fmt.Sprintf("[lnr]alimiter=limit=%g:attack=%d:release=%d:level=disabled%s", Limit, LimitAttack, LimitRelease, outLabel))
		return filters
	}

	// 不做响度归一化，但限幅仍然要留。
	filters = append(filters, fmt.Sprintf("[mixed]alimiter=limit=%g:attack=%d:release=%d:level=disabled%s", Limit, LimitAttack, LimitRelease, outLabel))
	return filters
}

func stringsJoin(ss []string, sep string) string {
	var b []byte
	for i, s := range ss {
		if i > 0 {
			b = append(b, sep...)
		}
		b = append(b, s...)
	}
	return string(b)
}
