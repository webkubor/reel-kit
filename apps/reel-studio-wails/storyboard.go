package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"
)

// Storyboard is editable data for the existing renderer, never executable shell/JS.
type Storyboard struct {
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Ratio    string   `json:"ratio"`
	Lines    []string `json:"lines"`
}

func validateStoryboard(p Storyboard) error {
	if p.Ratio != "9:16" && p.Ratio != "16:9" {
		return errors.New("分镜画幅只支持 9:16 或 16:9")
	}
	if utf8.RuneCountInString(p.Title) > 40 || utf8.RuneCountInString(p.Subtitle) > 40 {
		return errors.New("标题与副标题最多 40 字")
	}
	if len(p.Lines) < 1 || len(p.Lines) > 8 {
		return errors.New("分镜须有 1 至 8 句介绍词")
	}
	for _, line := range p.Lines {
		if strings.TrimSpace(line) == "" || utf8.RuneCountInString(line) > 80 {
			return errors.New("每句介绍词须为 1 至 80 字")
		}
	}
	return nil
}
func parseStoryboard(text string) (Storyboard, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if i := strings.Index(text, "\n"); i >= 0 {
			text = text[i+1:]
			text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "```"))
		}
	}
	var p Storyboard
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return p, errors.New("分镜不是有效 JSON，请检查 title、subtitle、ratio、lines")
	}
	return p, validateStoryboard(p)
}

const storyboardInstruction = `你是短视频分镜编辑。只返回一个 JSON 对象，不要 Markdown：{"title":"40字以内标题","subtitle":"40字以内副标题","ratio":"9:16","lines":["每句1至80字，1至8句"]}。ratio 只能是 9:16 或 16:9。内容用于本机中文配音和模板渲染；禁止返回代码、文件路径、命令或其他字段。按用户需求组织自然、可朗读的介绍词，不编造产品事实。`

func (e *Engine) generateStoryboard(ctx context.Context, job *Job, progress func(string, map[string]any), report func(Step)) (string, error) {
	prompt, _ := job.Input["prompt"].(string)
	if strings.TrimSpace(prompt) == "" || utf8.RuneCountInString(prompt) > 4000 {
		return "", errors.New("生成要求须为 1 至 4000 字")
	}
	progress("正在向配置的模型请求分镜（可能计费）", nil)
	report(Step{ID: "ai-plan", Label: "AI 分镜", Status: "running", Origin: "cloud-paid"})
	body := map[string]any{"messages": []map[string]string{{"role": "system", "content": storyboardInstruction}, {"role": "user", "content": prompt}}, "stream": false}
	data, c, err := e.aiRequest(ctx, http.MethodPost, "/chat/completions", body)
	progress("正在校验模型响应", map[string]any{"requestedModel": c.Model, "actualModel": "unknown"})
	if err != nil {
		return "", err
	}
	var response struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(data, &response) != nil || len(response.Choices) == 0 {
		return "", errors.New("模型响应缺少 choices/message/content")
	}
	actual := response.Model
	if actual == "" {
		actual = "unknown"
	}
	progress("正在校验分镜内容", map[string]any{"requestedModel": c.Model, "actualModel": actual})
	p, err := parseStoryboard(response.Choices[0].Message.Content)
	if err != nil {
		return "", err
	}
	encoded, _ := json.MarshalIndent(p, "", "  ")
	progress("分镜已生成，等待审阅后制作", nil)
	asset, err := e.store.AddAsset(job.ProjectID, "分镜 · "+p.Title+".json", "application/json", encoded, map[string]any{"source": "AI 分镜", "origin": "cloud", "prompt": prompt, "requestedModel": c.Model, "actualModel": actual})
	if err != nil {
		return "", err
	}
	report(Step{ID: "ai-plan", Label: "AI 分镜", Status: "done", Detail: fmt.Sprintf("%d 镜 · 请求 %s · 回执 %s", len(p.Lines), c.Model, actual)})
	return asset.ID, nil
}
func (e *Engine) ReadStoryboard(projectID, assetID string) (Storyboard, error) {
	if e.store == nil {
		return Storyboard{}, errNoStore
	}
	a, err := e.store.Asset(assetID, projectID)
	if err != nil {
		return Storyboard{}, err
	}
	if a.MIME != "application/json" {
		return Storyboard{}, errors.New("请选择分镜记录")
	}
	data, err := os.ReadFile(e.store.Path(a))
	if err != nil {
		return Storyboard{}, errors.New("无法读取分镜")
	}
	return parseStoryboard(string(data))
}
