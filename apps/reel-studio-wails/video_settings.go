package main

import "errors"

// Platform 一条发布平台的画幅与尺寸。
type Platform struct {
	Name   string `json:"name"`
	Ratio  string `json:"ratio"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Platforms 支持的发布平台。
var Platforms = map[string]Platform{
	"channels":    {Name: "视频号", Ratio: "9:16", Width: 1080, Height: 1920},
	"xiaohongshu": {Name: "小红书", Ratio: "9:16", Width: 1080, Height: 1920},
	"landscape":   {Name: "横屏视频", Ratio: "16:9", Width: 1920, Height: 1080},
}

// RecommendationSettings 项目推荐的配置 —— 旋钮的默认值与范围都来自 settings.go。
//
// 这里只补两件这个流水线特有的事：画幅平台的尺寸，以及「这条片子是谁合成的」
// （通道 / 模型 / 音色 / 账户是四件独立的事，都要跟着片子走；
// 少存一个，接手的人就不知道这条片子当时是「谁」的声音）。
func RecommendationSettings(input map[string]any) (map[string]any, error) {
	platform, _ := input["platform"].(string)
	if platform == "" {
		platform = "channels"
	}
	p, ok := Platforms[platform]
	if !ok {
		return nil, errors.New("请选择支持的发布平台")
	}
	voiceSource, _ := input["voiceSource"].(string)
	if voiceSource == "" {
		voiceSource = "scenes"
	}
	switch voiceSource {
	case "tts", "scenes", "audio":
	default:
		return nil, errors.New("请选择旁白来源")
	}
	// 旋钮走统一的收敛逻辑：预设 → 默认值 → 用户填的值。
	// 之前这里逐项 strictSetting，预设（口播清晰 / 氛围铺底）在这条流水线上是失效的。
	settings := ResolveSettings("recommendation", input, "")
	out := map[string]any{}
	for k, v := range settings {
		out[k] = v
	}
	out["platform"] = platform
	out["name"] = p.Name
	out["ratio"] = p.Ratio
	out["width"] = p.Width
	out["height"] = p.Height
	out["voiceSource"] = voiceSource
	// 通道 / 模型 / 音色 / 账户是四件独立的事，都要跟着这条片子走。
	// 少存一个，接手的人就不知道这条片子当时是「谁」的声音。
	out["voiceBackend"] = strOr(input["backend"], "voxflow")
	out["voiceModel"] = strOr(input["model"], "")
	out["voiceId"] = strOr(input["voiceId"], "")
	out["voiceProfile"] = strOr(input["profile"], "")
	return out, nil
}

// ReusableVoice 这条配音能不能复用。
//
// 必须连「谁合成的、用哪个模型、哪个音色」一起比 ——
// 换了通道或换了音色的旧配音，语速、咬字、情绪都不一样，直接套用会听出接缝。
func ReusableVoice(asset *Asset, input map[string]any, line string) bool {
	if asset == nil || !hasPrefix(asset.MIME, "audio/") {
		return false
	}
	if asset.Prompt != line {
		return false
	}
	if assetExtraStr(asset, "tone") != strOr(input["tone"], "") {
		return false
	}
	backend := strOr(input["backend"], "voxflow")
	assetBackend := assetExtraStr(asset, "voiceBackend")
	if assetBackend == "" {
		if asset.Source == "MiniMax TTS" {
			assetBackend = "minimax"
		} else {
			assetBackend = "voxflow"
		}
	}
	if backend != assetBackend {
		return false
	}
	model := strOr(input["model"], "")
	if model == "" {
		if backend == "minimax" {
			model = "speech-02-hd"
		} else {
			model = "voxflow-local"
		}
	}
	assetModel := assetExtraStr(asset, "requestedModel")
	if assetModel == "" {
		assetModel = "voxflow-local"
	}
	if assetModel != model {
		return false
	}
	voice := strOr(input["voiceId"], "")
	if voice == "" {
		voice = strOr(input["persona"], "")
	}
	if asset.Persona != voice {
		return false
	}
	// 在线语音把语速烤进了音频：语速对不上就不能复用，否则「填了语速没反应」。
	// 本机离线引擎的语速是出片后再变速的，复用旧配音不受影响。
	if backend == "edge" || backend == "minimax" {
		assetSpeed := assetExtraFloat(asset, "speed", 1)
		inputSpeed := 1.0
		if v := toFloat(input["speed"]); v != nil {
			inputSpeed = *v
		}
		if assetSpeed != inputSpeed {
			return false
		}
	}
	// 同一个模型在两个账户下也不是同一段声音 —— 账户也要比
	return assetExtraStr(asset, "profile") == strOr(input["profile"], "")
}
