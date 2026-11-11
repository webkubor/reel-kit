package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// Settings contain routing metadata only. Credentials stay in the OS keychain.
type AISettings struct {
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	HasKey  bool   `json:"hasKey"`
}
type secretStore interface {
	Get(string, string) (string, error)
	Set(string, string, string) error
	Delete(string, string) error
}
type systemSecrets struct{}

func (systemSecrets) Get(s, a string) (string, error) { return keyring.Get(s, a) }
func (systemSecrets) Set(s, a, k string) error        { return keyring.Set(s, a, k) }
func (systemSecrets) Delete(s, a string) error        { return keyring.Delete(s, a) }
func (e *Engine) secrets() secretStore {
	if e.aiSecrets != nil {
		return e.aiSecrets
	}
	return systemSecrets{}
}

const aiKeyService = "Reel Studio AI"

func aiAccount(baseURL string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(baseURL))) }

// Mirrors channels.mjs address rules. Never reuse a saved credential at a new URL.
func validateAISettings(c AISettings) (AISettings, error) {
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model = strings.TrimSpace(c.Model)
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" {
		return c, errors.New("请输入完整的模型 API 地址")
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && local)) {
		return c, errors.New("API 地址须为 HTTPS（本机地址可用 HTTP），不能带账号、查询参数或锚点")
	}
	if c.Model == "" || len(c.Model) > 200 {
		return c, errors.New("请填写有效的模型 ID")
	}
	c.HasKey = false
	return c, nil
}
func (e *Engine) readAISettingsLocked() (AISettings, error) {
	if e.store == nil {
		return AISettings{}, errNoStore
	}
	data, err := os.ReadFile(filepath.Join(e.store.Root(), "ai-settings.json"))
	if os.IsNotExist(err) {
		return AISettings{}, nil
	}
	if err != nil {
		return AISettings{}, errors.New("无法读取 AI 配置")
	}
	var c AISettings
	if json.Unmarshal(data, &c) != nil {
		return c, errors.New("AI 配置损坏，请保留文件后排查")
	}
	return c, nil
}
func (e *Engine) GetAISettings() (AISettings, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, err := e.readAISettingsLocked()
	if err != nil {
		return c, err
	}
	if c.BaseURL != "" {
		key, err := e.secrets().Get(aiKeyService, aiAccount(c.BaseURL))
		c.HasKey = err == nil && key != ""
	}
	return c, nil
}
func (e *Engine) SaveAISettings(c AISettings, apiKey string) (AISettings, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.store == nil {
		return c, errNoStore
	}
	c, err := validateAISettings(c)
	if err != nil {
		return c, err
	}
	// Do not overwrite a corrupt settings file silently.
	if _, err = e.readAISettingsLocked(); err != nil {
		return c, err
	}
	if strings.TrimSpace(apiKey) != "" {
		if err = e.secrets().Set(aiKeyService, aiAccount(c.BaseURL), strings.TrimSpace(apiKey)); err != nil {
			return c, errors.New("系统钥匙串保存失败，密钥未写入配置文件")
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return c, err
	}
	target := filepath.Join(e.store.Root(), "ai-settings.json")
	tmp, err := os.CreateTemp(e.store.Root(), ".ai-settings-*")
	if err != nil {
		return c, errors.New("无法保存 AI 配置")
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		return c, errors.New("无法保存 AI 配置")
	}
	if err = os.Rename(name, target); err != nil {
		return c, errors.New("无法保存 AI 配置")
	}
	key, keyErr := e.secrets().Get(aiKeyService, aiAccount(c.BaseURL))
	c.HasKey = keyErr == nil && key != ""
	return c, nil
}
func (e *Engine) ClearAIKey() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, err := e.readAISettingsLocked()
	if err != nil {
		return err
	}
	if c.BaseURL == "" {
		return nil
	}
	err = e.secrets().Delete(aiKeyService, aiAccount(c.BaseURL))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errors.New("无法移除钥匙串密钥")
	}
	return nil
}
func (e *Engine) aiRequest(ctx context.Context, method, path string, body any) ([]byte, AISettings, error) {
	c, err := e.GetAISettings()
	if err != nil {
		return nil, c, err
	}
	c, err = validateAISettings(c)
	if err != nil {
		return nil, c, err
	}
	key, err := e.secrets().Get(aiKeyService, aiAccount(c.BaseURL))
	if err != nil || key == "" {
		return nil, c, errors.New("请先在 AI 设置中保存 API Key")
	}
	var data []byte
	if body != nil {
		if payload, ok := body.(map[string]any); ok && method == http.MethodPost {
			payload["model"] = c.Model
		}
		data, err = json.Marshal(body)
		if err != nil {
			return nil, c, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, c, errors.New("无法创建模型请求")
	}
	req.Header.Set("Authorization", "Bearer "+key)
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
		return nil, c, errors.New("模型请求未完成，请检查地址、网络或超时")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, c, fmt.Errorf("模型服务返回 HTTP %d；请检查地址、权限与额度", response.StatusCode)
	}
	result, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return nil, c, errors.New("读取模型响应失败")
	}
	if len(result) > 1<<20 {
		return nil, c, errors.New("模型响应超过 1 MB 限制")
	}
	return result, c, nil
}
func (e *Engine) CheckAIConnection() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	data, c, err := e.aiRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return "", err
	}
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &models) != nil || models.Data == nil {
		return "", errors.New("服务返回的不是 OpenAI 兼容模型列表")
	}
	for _, m := range models.Data {
		if m.ID == c.Model {
			return "连接与模型列表检查通过；未提交付费生成", nil
		}
	}
	return "连接成功，但列表未包含当前模型；生成能力尚未验证", nil
}
