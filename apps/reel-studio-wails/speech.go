package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// 生成一段配音。
//
// 只做 MVP 需要的三个通道：
//
//	system   macOS 系统语音，本机自带，不花钱不联网，零 HTTP
//	voxflow  本机离线语音引擎（voxcraft），不花钱不联网
//	minimax  云端付费 TTS，需要 MINIMAX_API_KEY
//
// 原来还有 edge（微软免费在线语音），它依赖 Node 专用的
// vendor/edge-tts-bundle.cjs，Go 没有等价实现。
// MVP 不带它 —— 带一个调不通的选项比不带更糟：用户选了才发现用不了。
// 通道清单里的 edge 条目先留着，等有 Go 版实现再放开。
func (e *Engine) GenerateSpeech(ctx context.Context, projectID, text, tone, backend, model, voiceID, profile string, speed float64) (*Asset, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	text = trimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("配音文本为空")
	}
	if speed <= 0 {
		speed = 1
	}

	// 测试替身：让端到端测试能跑完整条流水线，而不必在本机装语音引擎。
	// 生产路径永远走下面的真实通道。
	if e.speechOverride != nil {
		return e.speechOverride(ctx, projectID, text, tone, backend, model, voiceID, profile, speed)
	}

	switch backend {
	case "", "sherpa":
		return e.generateSherpaSpeech(ctx, projectID, text)
	case "voxflow":
		return e.generateVoxflowSpeech(ctx, projectID, text, tone, model, voiceID, profile, speed)
	case "minimax":
		return e.generateMinimaxSpeech(ctx, projectID, text, tone, model, voiceID, profile, speed)
	case "edge":
		return nil, fmt.Errorf("Edge 语音通道在这个版本里还不可用，请改用系统语音或 MiniMax")
	default:
		return nil, fmt.Errorf("不支持的语音通道: %s", backend)
	}
}

// generateMinimaxSpeech 用 MiniMax 云端 TTS。
func (e *Engine) generateMinimaxSpeech(ctx context.Context, projectID, text, tone, model, voiceID, profile string, speed float64) (*Asset, error) {
	if MinimaxAPIKey() == "" {
		return nil, fmt.Errorf("未配置 MINIMAX_API_KEY：请用密钥工具注入，或写进 ~/.reel-kit/config.json 的 minimaxApiKey")
	}
	if voiceID == "" {
		return nil, fmt.Errorf("MiniMax 需要指定音色 ID")
	}
	if model == "" {
		model = "speech-02-hd"
	}
	// 先写到临时目录，成品再进素材库 —— 合成失败不留半个文件在素材里。
	tmp, err := os.MkdirTemp("", "reel-voice-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "voice.wav")
	if err := SpeakViaMinimax(ctx, text, out, voiceID); err != nil {
		return nil, err
	}
	return e.storeVoiceAsset(projectID, out, map[string]any{
		"source":         "MiniMax TTS",
		"origin":         "cloud-paid",
		"engine":         "MiniMax TTS",
		"tone":           tone,
		"voiceBackend":   "minimax",
		"requestedModel": model,
		"persona":        voiceID,
		"profile":        profile,
		"speed":          speed,
		"prompt":         text,
	})
}

// generateVoxflowSpeech 用本机 voxcraft 离线合成。
func (e *Engine) generateVoxflowSpeech(ctx context.Context, projectID, text, tone, model, voiceID, profile string, speed float64) (*Asset, error) {
	e.mu.Lock()
	server := e.voxcraft
	e.mu.Unlock()
	if server == nil {
		info := LocateVoxcraft()
		if !info.OK {
			return nil, fmt.Errorf("本机没找到 VoxFlow 语音引擎：%s", info.Reason)
		}
		started, err := EnsureServer(info.Home, info.Bin)
		if err != nil {
			return nil, err
		}
		e.mu.Lock()
		e.voxcraft = started
		e.mu.Unlock()
		server = started
	}
	if voiceID == "" {
		voiceID = "default"
	}
	if model == "" {
		model = "voxflow-local"
	}
	tmp, err := os.MkdirTemp("", "reel-voice-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	files, err := SynthesizeViaServer(server.Base, voiceID, []string{text}, tmp)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("VoxFlow 没有产出音频")
	}
	return e.storeVoiceAsset(projectID, files[0], map[string]any{
		"source":         "VoxFlow",
		"origin":         "local-offline",
		"engine":         "VoxFlow 本地语音",
		"tone":           tone,
		"voiceBackend":   "voxflow",
		"requestedModel": model,
		"persona":        voiceID,
		"profile":        profile,
		"speed":          speed,
		"prompt":         text,
	})
}

// storeVoiceAsset 把音频存进素材库。
func (e *Engine) storeVoiceAsset(projectID, file string, extra map[string]any) (*Asset, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return e.store.AddAsset(projectID, "voice.wav", "audio/wav", data, extra)
}

// trimSpace 去掉首尾空白。
func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\n' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
