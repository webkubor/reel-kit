package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MinimaxTTS 云端 TTS（MiniMax speech-02-hd，按量计费，无需本地模型）。
type MinimaxTTS struct {
	APIKey  string
	Model   string
	VoiceID string
	HTTP    *http.Client
}

const (
	MinimaxEndpoint = "https://api.minimaxi.com/v1/t2a_v2"
	MinimaxTimeout  = 120 * time.Second
)

// TTSResult 合成结果。
type TTSResult struct {
	File     string
	Duration float64
}

// ReadReelConfig 读 ~/.reel-kit/config.json（桌面版从 Finder 启动时没有终端环境变量）。
func ReadReelConfig() map[string]interface{} {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".reel-kit", "config.json"))
	if err != nil {
		return nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return cfg
}

// MinimaxAPIKey 优先环境变量，再配置文件。
func MinimaxAPIKey() string {
	if k := os.Getenv("MINIMAX_API_KEY"); k != "" {
		return k
	}
	cfg := ReadReelConfig()
	if v, ok := cfg["minimaxApiKey"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SpeakViaMinimax 合成一段文字为 wav 文件。
func SpeakViaMinimax(ctx context.Context, text, out, voice string) error {
	key := MinimaxAPIKey()
	if key == "" {
		return fmt.Errorf("未配置 MINIMAX_API_KEY（用密钥工具注入，或写进 ~/.reel-kit/config.json 的 minimaxApiKey）")
	}
	if voice == "" {
		return fmt.Errorf("MiniMax TTS 需要 --voice <音色 ID>，例如 male-qn-qingse")
	}

	model := os.Getenv("REEL_MINIMAX_TTS_MODEL")
	if model == "" {
		model = "speech-02-hd"
	}

	body := map[string]interface{}{
		"model":  model,
		"text":   text,
		"stream": false,
		"voice_setting": map[string]interface{}{
			"voice_id": voice,
			"speed":    1,
			"vol":      1,
			"pitch":    0,
		},
		"audio_setting": map[string]interface{}{
			"sample_rate": 32000,
			"bitrate":     128000,
			"format":      "mp3",
			"channel":     1,
		},
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", MinimaxEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("MiniMax 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
	}
	_ = json.Unmarshal(respBody, &result)

	if resp.StatusCode != 200 {
		return fmt.Errorf("MiniMax HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	if result.BaseResp.StatusCode != 0 {
		return fmt.Errorf("MiniMax 拒绝请求（%d）：%s", result.BaseResp.StatusCode, result.BaseResp.StatusMsg)
	}
	hex := result.Data.Audio
	if hex == "" {
		return fmt.Errorf("MiniMax 未返回音频数据")
	}

	audioBytes, err := base64.StdEncoding.DecodeString(hex)
	if err != nil {
		return fmt.Errorf("MiniMax 音频 base64 解码失败: %w", err)
	}

	// 先写 mp3，再 ffmpeg 转 wav（流水线后面统一按 wav 处理）
	tmp := strings.TrimSuffix(out, ".wav") + ".mp3"
	if err := os.WriteFile(tmp, audioBytes, 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-loglevel", "error", "-i", tmp, "-ar", "24000", "-ac", "1", out)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("MiniMax 音频转换失败: %w", err)
	}
	return nil
}

// DurationsFromVoice 由念白时长推导镜头时长（每镜 = 念白 + 余量，下限 min）。
func DurationsFromVoice(voiceDurations []float64, margin, min float64) []float64 {
	if margin == 0 {
		margin = 0.45
	}
	if min == 0 {
		min = 1.2
	}
	out := make([]float64, len(voiceDurations))
	for i, d := range voiceDurations {
		v := d + margin
		if v < min {
			v = min
		}
		out[i] = v
	}
	return out
}
