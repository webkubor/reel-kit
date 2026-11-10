package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// 配音通道：把一句话变成配音，有两条路。
//
//	本机 VoxFlow —— 本地推理，声音不出本机也不花钱，但只在 Apple 芯片的 Mac 上跑得起来；
//	云端 MiniMax TTS —— 任何机器都能用，按量计费，文字会发到云端。
//
// 桌面版原来把通道写死成本机（submitTalk 里直接传 voiceBackend=voxflow），
// 于是 Windows 版装上了也出不了声 —— 拿到的只是一个不能配音的空壳。
// 这里把通道还给用户选：能本机跑就本机跑，Windows 用户走云端。

// voiceChannels 返回工作台报告的所有语音通道。
func (a *App) voiceChannels() []studioapi.Channel {
	if a.providers == nil {
		return nil
	}
	return a.providers.Voice.Channels
}

// voiceChannel 按 ID 取一条语音通道。
func (a *App) voiceChannel(id string) *studioapi.Channel {
	channels := a.voiceChannels()
	for i := range channels {
		if channels[i].ID == id {
			return &channels[i]
		}
	}
	return nil
}

// voiceChannelReady 报告通道是否就绪。
//
// 没就绪还硬着头皮提交，用户只会看到一句「fetch failed」——
// 那时候他连问题出在哪一步都判断不了。
func (a *App) voiceChannelReady(id string) bool {
	ch := a.voiceChannel(id)
	return ch != nil && ch.Ready
}

// voiceChannelNote 写通道没就绪的原因；通道压根不存在就说不知道。
func (a *App) voiceChannelNote(id string) string {
	if ch := a.voiceChannel(id); ch != nil && ch.Note != "" {
		return ch.Note
	}
	return "该配音通道当前不可用"
}

// defaultVoiceChannel 挑一条就绪的通道。
//
// 一条都不就绪时返回第一条：界面要让人看见「有通道，但没就绪」，
// 而不是让通道凭空消失。
func (a *App) defaultVoiceChannel() string {
	channels := a.voiceChannels()
	for _, ch := range channels {
		if ch.Ready {
			return ch.ID
		}
	}
	if len(channels) > 0 {
		return channels[0].ID
	}
	return ""
}

// viewVoicePicker 画「配音通道」与该通道下的「音色」。
//
// backend 存通道 ID，voiceID 存该通道里的音色 ID ——
// VoxFlow 的音色 ID 是本机音色 key，MiniMax 的是 voice_id，
// 同一个字段存两边的值，换通道时这个值要作废。
func (a *App) viewVoicePicker(c *ui.Context, backend *string, voiceID *string) {
	channels := a.voiceChannels()
	if len(channels) == 0 {
		a.kit.Field(c, "配音通道", "工作台没有报告任何语音通道", func() {
			a.kit.Hint(c, "暂无可用配音通道，请先启动本机工作台")
		})
		return
	}
	if *backend == "" {
		*backend = a.defaultVoiceChannel()
	}
	choices := make([]choice, 0, len(channels))
	for _, ch := range channels {
		label := ch.Label
		if !ch.Ready {
			label += "（未就绪）"
		}
		choices = append(choices, choice{ch.ID, label})
	}
	a.pick(c, "配音通道", a.voiceChannelPickerHelp(), choices, *backend, func(id string) {
		if id != *backend {
			*voiceID = "" // 换通道等于换音色列表，旧音色不再有意义
		}
		*backend = id
	})
	ch := a.voiceChannel(*backend)
	if ch == nil {
		return
	}
	if !ch.Ready {
		a.kit.Field(c, "音色", "这条通道还没就绪", func() {
			a.kit.Hint(c, ch.Note)
		})
		if ch.ID == "voxflow" {
			a.voxflowHint(c)
		}
		return
	}
	if len(ch.Voices) == 0 {
		a.kit.Field(c, "音色", "这条通道没有可用音色", func() {
			a.kit.Hint(c, "暂无可用音色")
		})
		return
	}
	voices := make([]choice, 0, len(ch.Voices))
	for _, v := range ch.Voices {
		voices = append(voices, choice{v.ID, v.Name})
	}
	a.pick(c, "音色", voiceChannelHelp(ch.ID), voices, *voiceID, func(id string) { *voiceID = id })
}

// voiceChannelPickerHelp 写清楚三条通道分别在哪跑、花不花钱。
//
// VoxFlow 现在的语音后端是 Apple 芯片上的本地 MLX 模型，Windows 与 Intel Mac
// 都装不了也用不了 —— 那是 VoxFlow 当前的部署形态，不是这条路走不通。
// 所以这句限制只在跑得起来的机器上省略，别在别的机器上也写着「仅 macOS」。
//
// 但「本机装不了」不等于「用不了」：工作台连的是远端语音服务时，
// Windows 一样能克隆 —— 所以这句按工作台实际连上的通道写，不按本机系统写。
func (a *App) voiceChannelPickerHelp() string {
	if a.voiceCloneAvailable() {
		return "本机离线：随应用自带、不联网不花钱；VoxFlow 可克隆自己的声音；云端按量计费"
	}
	if voxflowRunsHere() {
		return "本机离线：随应用自带、不联网不花钱；VoxFlow 可克隆自己的声音（要本机先启动语音服务）；云端按量计费"
	}
	return "本机离线：随应用自带、不联网不花钱；VoxFlow（克隆自己的声音）要工作台连上语音服务，本机没起服务时用不了；云端按量计费"
}

// voiceChannelHelp 写清楚这条通道的钱和隐私成本。
func voiceChannelHelp(id string) string {
	switch id {
	case "minimax":
		return "MiniMax 云端音色，按量计费；文字会发到云端"
	case "sherpa":
		return "本机离线音色，随应用自带；不联网、不产生费用"
	default:
		return "本机音色，推理不出本机、不产生模型费用"
	}
}

// voicePipelineHint 写这条流水线的配音发生在哪、花不花钱。
//
// 通道能换了，这句话就不能写死「本机」——
// 用户选了云端却看见「不产生模型费用」，那是骗人。
func (a *App) voicePipelineHint(backend string) string {
	switch backend {
	case "minimax":
		return "逐句用 MiniMax TTS 配音（按量计费，文字发到云端），画面与合成仍在本机执行。"
	case "sherpa":
		return "逐句用随应用自带的本机离线引擎配音（不联网、不产生费用），画面与合成都在本机执行。"
	default:
		return "逐句用本机 VoxFlow 配音，画面与合成都在本机执行，不产生模型费用。"
	}
}

// voiceNameFor 按配音通道找音色的显示名。
//
// 同一条生成配置里，「音色」可能是本机音色 key，也可能是云端 voice_id ——
// 取决于当时选的是哪条通道，不能一律按本机音色查。
func (a *App) voiceNameFor(id string, input map[string]any) string {
	if id == "" {
		return "—"
	}
	backend, _ := input["voiceBackend"].(string)
	if backend == "" {
		backend = "voxflow"
	}
	if ch := a.voiceChannel(backend); ch != nil {
		for _, v := range ch.Voices {
			if v.ID == id {
				return v.Name
			}
		}
	}
	if a.providers != nil {
		for _, p := range a.providers.Voice.Personas {
			if p.Key == id {
				return p.Name
			}
		}
	}
	return orDash(id)
}
