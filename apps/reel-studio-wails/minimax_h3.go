package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MiniMax H3 直连适配。
//
// 线格式来自 hypit-provider-minimax 的实测实现（GitLab webkubor/hypit-provider-minimax）：
//   · 请求体 { model, content[], resolution, duration, ratio }
//   · content 是一个数组，项按类型与角色标注：text / image_url(reference_image)
//     / image_url(first_frame) / video_url(reference_video) / audio_url(reference_audio)
//   · 线上模型名是 MiniMax-H3
//   · HTTP 200 不代表成功：业务失败藏在 base_resp.status_code 或 error 对象里
//   · 「已发出但没收到回执」是独立状态：可能已经计费，不能当成没提交
//
// 中台（museav）已经支持这套形状，所以只有直连才需要这层适配。

// H3 线上模型名。
const h3WireModel = "MiniMax-H3"

// h3Request H3 的请求体。
type h3Request struct {
	Model      string      `json:"model"`
	Content    []h3Content `json:"content"`
	Resolution string      `json:"resolution,omitempty"`
	Duration   int         `json:"duration,omitempty"`
	Ratio      string      `json:"ratio,omitempty"`
}

// h3Content content 数组里的一项。
type h3Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// 图片/视频/音频统一用 {url} 形状。
	URL   string `json:"url,omitempty"`
	Role  string `json:"role,omitempty"`
	Media *h3URL `json:"image_url,omitempty"`
	Video *h3URL `json:"video_url,omitempty"`
	Audio *h3URL `json:"audio_url,omitempty"`
}

// h3URL 媒体引用。
type h3URL struct {
	URL string `json:"url"`
}

// h3Response H3 的响应。
type h3Response struct {
	ID       string `json:"id"`
	BaseResp struct {
		StatusCode json.RawMessage `json:"status_code"`
		StatusMsg  string          `json:"status_msg"`
	} `json:"base_resp"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
	// 任务结果（轮询时才有）。
	FileID string `json:"file_id"`
	URL    string `json:"url"`
	Video  struct {
		URL string `json:"url"`
	} `json:"video"`
	Status string `json:"status"`
}

// h3BusinessFailure 识别业务失败：HTTP 200 也可能失败。
func h3BusinessFailure(body []byte) error {
	var r h3Response
	if json.Unmarshal(body, &r) != nil {
		return nil // 不是 H3 的形状，交给上层按普通响应处理
	}
	if r.Error != nil && r.Error.Message != "" {
		return fmt.Errorf("MiniMax 拒绝了请求：%s", r.Error.Message)
	}
	if len(r.BaseResp.StatusCode) > 0 {
		code := strings.Trim(string(r.BaseResp.StatusCode), `"`)
		if code != "" && code != "0" {
			msg := r.BaseResp.StatusMsg
			if msg == "" {
				msg = "status " + code
			}
			return fmt.Errorf("MiniMax 拒绝了请求：%s", msg)
		}
	}
	return nil
}

// buildH3Body 组装 H3 请求体。
//
// 参考图按 H3 的互斥规则给角色：首帧/尾帧与参考图互斥，本适配只送参考图。
func buildH3Body(prompt string, referenceImages []string, duration int, ratio string) h3Request {
	content := []h3Content{{Type: "text", Text: prompt}}
	for _, url := range referenceImages {
		if url == "" {
			continue
		}
		content = append(content, h3Content{
			Type: "image_url", Role: "reference_image", Media: &h3URL{URL: url},
		})
	}
	return h3Request{
		Model:      h3WireModel,
		Content:    content,
		Resolution: "768P",
		Duration:   duration,
		Ratio:      ratio,
	}
}

// h3SubmitUnknown 「已发出但没收到回执」。
//
// 这个状态和「请求没发出去」必须分开：前者可能已经被上游受理并计费，
// 盲目重试会重复扣额度。
var errH3SubmissionUnknown = errors.New("请求已发出但没有回执：可能已被受理并计费，请先到服务商后台核对任务，不要直接重试")

// h3DirectGenerate 直连 MiniMax 提交一次 H3 生成。
func (e *Engine) h3DirectGenerate(ctx context.Context, prompt string, referenceImages []string, duration int, ratio string) (string, error) {
	c, err := e.GetAISettings()
	if err != nil {
		return "", err
	}
	if c.BaseURL == "" {
		return "", errors.New("直连服务还没配置地址")
	}
	key, err := e.secrets().Get(aiKeyService, aiAccount(c.BaseURL))
	if err != nil || key == "" {
		return "", errors.New("直连服务缺少 API Key")
	}
	body := buildH3Body(prompt, referenceImages, duration, ratio)
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/video/generation", bytes.NewReader(payload))
	if err != nil {
		return "", errors.New("无法创建 H3 请求")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 90 * time.Second}
	if e.aiClient != nil {
		client.Transport = e.aiClient.Transport
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// 发出去了但没拿到回执：可能已计费。
		return "", errH3SubmissionUnknown
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return "", errH3SubmissionUnknown
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("MiniMax 返回 HTTP %d；请检查地址、权限与额度", resp.StatusCode)
	}
	if err := h3BusinessFailure(data); err != nil {
		return "", err
	}
	var r h3Response
	if json.Unmarshal(data, &r) != nil {
		return "", errors.New("MiniMax 返回的不是预期格式")
	}
	if r.ID == "" {
		return "", errors.New("MiniMax 没有返回任务 id")
	}
	return r.ID, nil
}

// h3DirectPoll 轮询一次 H3 任务。
//
// 返回 (成片URL, 完成, 错误)：未完成时完成=false。
func (e *Engine) h3DirectPoll(ctx context.Context, taskID string) (string, bool, error) {
	c, err := e.GetAISettings()
	if err != nil {
		return "", false, err
	}
	key, err := e.secrets().Get(aiKeyService, aiAccount(c.BaseURL))
	if err != nil || key == "" {
		return "", false, errors.New("直连服务缺少 API Key")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(c.BaseURL, "/")+"/query/video_generation?task_id="+taskID, nil)
	if err != nil {
		return "", false, errors.New("无法创建 H3 查询请求")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client := http.Client{Timeout: 60 * time.Second}
	if e.aiClient != nil {
		client.Transport = e.aiClient.Transport
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", false, ctx.Err()
		}
		return "", false, errH3SubmissionUnknown
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return "", false, errH3SubmissionUnknown
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("MiniMax 查询返回 HTTP %d", resp.StatusCode)
	}
	if err := h3BusinessFailure(data); err != nil {
		return "", false, err
	}
	var r h3Response
	if json.Unmarshal(data, &r) != nil {
		return "", false, errors.New("MiniMax 查询返回的不是预期格式")
	}
	if url := r.Video.URL; url != "" {
		return url, true, nil
	}
	if url := r.URL; url != "" {
		return url, true, nil
	}
	switch strings.ToUpper(r.Status) {
	case "SUCCESS", "FINISHED", "COMPLETED", "":
		if r.FileID != "" {
			// 文件已生成但没带链接：给文件 id，调用方用下载接口取。
			return "file:" + r.FileID, true, nil
		}
		return "", false, nil
	case "FAIL", "FAILED", "ERROR":
		return "", false, errors.New("MiniMax 任务失败：" + r.BaseResp.StatusMsg)
	default:
		return "", false, nil
	}
}
