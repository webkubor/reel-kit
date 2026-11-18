package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────
// AI 提供方（provider）
//
// 用户既是直连 MiniMax 的用户，也是用 MUSE AV 中台的用户，两种都要能跑，
// 并且可以随时切换。所以提供方是一等配置，不是某条硬编码路径。
//
//   museav    默认：中台出图出视频、读哲学、看素材；登录态在 ~/.museav.json。
//   direct    直连：用户自己的模型服务（OpenAI 兼容），密钥在系统钥匙串。
//
// 直连这边还要区分模型族：H3（MiniMax 视频模型）的提示词和参数形状
// 与 OpenAI 兼容的对话模型不同，H3 走专门的适配。
// ──────────────────────────────────────────────────────────────

// AI 提供方。
const (
	// ProviderMuseAV 走 MUSE AV 中台。
	ProviderMuseAV = "museav"
	// ProviderDirect 直连用户自己的模型服务。
	ProviderDirect = "direct"
)

// 直连模式下的模型族。
const (
	// ModelFamilyChat OpenAI 兼容对话模型（做分镜、文案）。
	ModelFamilyChat = "chat"
	// ModelFamilyH3 MiniMax H3 视频模型（多模态参考生视频）。
	ModelFamilyH3 = "h3"
)

// AIProviderStatus 界面上的提供方状态。
type AIProviderStatus struct {
	// Provider 当前提供方。
	Provider string `json:"provider"`
	// ModelFamily 直连时的模型族。
	ModelFamily string `json:"modelFamily,omitempty"`
	// BaseURL 直连时的服务地址。
	BaseURL string `json:"baseUrl,omitempty"`
	// Model 直连时的模型 id。
	Model string `json:"model,omitempty"`
	// HasKey 是否已有可用密钥（直连看钥匙串，中台看登录态）。
	HasKey bool `json:"hasKey"`
	// Ready 是否可以开始工作。
	Ready bool `json:"ready"`
	// Notice 界面提示：缺什么、怎么补。
	Notice string `json:"notice,omitempty"`
}

// GetAIProviderStatus 报当前提供方状态，供引导页与设置页使用。
func (e *Engine) GetAIProviderStatus() AIProviderStatus {
	status := AIProviderStatus{Provider: ProviderMuseAV, ModelFamily: ModelFamilyChat}
	switch e.provider() {
	case ProviderDirect:
		c, err := e.GetAISettings()
		if err != nil {
			status.Notice = err.Error()
			return status
		}
		status.Provider = ProviderDirect
		status.BaseURL = c.BaseURL
		status.Model = c.Model
		status.ModelFamily = c.ModelFamily
		if c.BaseURL == "" {
			status.Notice = "还没配置直连服务：填地址、模型 ID 和 API Key"
			return status
		}
		status.HasKey = c.HasKey
		if !c.HasKey {
			status.Notice = "直连服务缺少 API Key"
			return status
		}
		status.Ready = true
		status.Notice = "直连已就绪：" + c.Model
	default:
		// MUSE AV：登录态在 ~/.museav.json，命令可用就能用。
		if err := e.museavReady(); err != nil {
			status.Notice = err.Error()
			return status
		}
		status.HasKey = true
		status.Ready = true
		status.Notice = "MUSE AV 中台已就绪"
	}
	return status
}

// provider 读当前提供方；默认 MUSE AV。
func (e *Engine) provider() string {
	if e.store == nil {
		return ProviderMuseAV
	}
	data, err := readJSONFile(e.store.Root(), "ai-provider.json")
	if err != nil || data == nil {
		return ProviderMuseAV
	}
	var cfg struct {
		Provider string `json:"provider"`
	}
	if jsonUnmarshal(data, &cfg) != nil {
		return ProviderMuseAV
	}
	if cfg.Provider == ProviderDirect {
		return ProviderDirect
	}
	return ProviderMuseAV
}

// SetAIProvider 切换提供方：museav 或 direct。
func (e *Engine) SetAIProvider(provider string) (AIProviderStatus, error) {
	provider = strings.TrimSpace(provider)
	if provider != ProviderMuseAV && provider != ProviderDirect {
		return AIProviderStatus{}, errors.New("提供方只能是 MUSE AV 或直连")
	}
	if e.store == nil {
		return AIProviderStatus{}, errNoStore
	}
	if err := writeJSONFile(e.store.Root(), "ai-provider.json", map[string]any{
		"provider":  provider,
		"updatedAt": nowISO(),
	}); err != nil {
		return AIProviderStatus{}, err
	}
	return e.GetAIProviderStatus(), nil
}

// museavReady 检查中台是否可用（命令存在 + 有登录态）。
func (e *Engine) museavReady() error {
	if err := (&MuseavClient{}).Available(context.Background()); err != nil {
		return err
	}
	if _, _, err := museavCredentials(); err != nil {
		return errors.New("MUSE AV 未登录：请在终端执行 museav login")
	}
	return nil
}

// providerRequest 按当前提供方发起模型请求。
//
// MUSE AV：走中台的 OpenAI 兼容网关（用中台登录态当凭据）。
// 直连：走用户自己的服务（钥匙串里的密钥）。
func (e *Engine) providerRequest(ctx context.Context, method, path string, body any) ([]byte, AISettings, error) {
	if e.provider() == ProviderDirect {
		return e.aiRequest(ctx, method, path, body)
	}
	return e.museavChatRequest(ctx, method, path, body)
}

// museavChatRequest 走 MUSE AV 中台的模型网关。
//
// 中台默认支持 OpenAI 兼容形状，所以直连那套请求体可以复用；差别只在凭据
// 与地址。中台的创作哲学、素材、出图出视频都由中台自己负责。
func (e *Engine) museavChatRequest(ctx context.Context, method, path string, body any) ([]byte, AISettings, error) {
	token, baseURL, err := museavCredentials()
	if err != nil {
		return nil, AISettings{}, err
	}
	c := AISettings{BaseURL: baseURL, Model: museavChatModel, HasKey: true}
	var data []byte
	if body != nil {
		if payload, ok := body.(map[string]any); ok && method == http.MethodPost {
			if _, has := payload["model"]; !has {
				payload["model"] = c.Model
			}
		}
		data, err = json.Marshal(body)
		if err != nil {
			return nil, c, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(data))
	if err != nil {
		return nil, c, errors.New("无法创建中台请求")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if e.aiClient != nil {
		client.Transport = e.aiClient.Transport
	}
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, c, ctx.Err()
		}
		return nil, c, errors.New("中台请求未完成，请检查网络")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, c, fmt.Errorf("MUSE AV 返回 HTTP %d；请检查登录态与额度", response.StatusCode)
	}
	result, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return nil, c, errors.New("读取中台响应失败")
	}
	return result, c, nil
}

// museavChatModel 中台网关默认的对话模型。
const museavChatModel = "gpt-4o-mini"

// detectModelFamily 按模型名判断模型族：H3 走视频适配，其余按 OpenAI 兼容对话。
func detectModelFamily(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(m, "h3") {
		return ModelFamilyH3
	}
	return ModelFamilyChat
}

// readJSONFile 读一个 JSON 文件；不存在返回 nil。
func readJSONFile(root, name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

// writeJSONFile 原子写一个 JSON 文件。
func writeJSONFile(root, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(root, name)
	tmp, err := os.CreateTemp(root, ".tmp-*")
	if err != nil {
		return err
	}
	name2 := tmp.Name()
	defer os.Remove(name2)
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name2, target)
}

// jsonUnmarshal 解析 JSON。
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// defaultVoiceBackend 选一个可用的语音通道。
//
// 写死 sherpa 的问题是：它常缺本机模型资源，首选它就等于默认失败。
// 这里按「本机离线优先、不联网优先」的顺序挑第一个就绪的通道。
func (e *Engine) defaultVoiceBackend() string {
	for _, c := range e.VoiceChannels() {
		if c.Ready {
			switch c.Origin {
			case "local-offline":
				return c.ID
			}
		}
	}
	for _, c := range e.VoiceChannels() {
		if c.Ready {
			return c.ID
		}
	}
	return "sherpa"
}
