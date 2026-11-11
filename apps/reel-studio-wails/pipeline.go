package main

import (
	"context"
	"fmt"
)

// 出片流水线入口。
//
// 任务队列收到一条任务后按 kind 分发到对应的流水线。
// 每条流水线做完后返回产出的素材 id，任务队列负责落状态。
//
// 现在只接了 product（人物 / 商品介绍）。口播与项目推荐还在搬 ——
// 宁可少一条入口，也不要留一条点了会报错的入口。

// runPipeline 队列的执行体。
func (e *Engine) runPipeline(ctx context.Context, job *Job, progress func(text string, upstream map[string]any), report func(Step)) (string, error) {
	kind, _ := job.Input["kind"].(string)
	switch kind {
	case "product":
		input, err := productInputFromMap(job.Input)
		if err != nil {
			return "", err
		}
		input.ProjectID = job.ProjectID
		asset, err := e.MakeProductVideo(ctx, input, func(text string) {
			progress(text, nil)
		}, report)
		if err != nil {
			return "", err
		}
		return asset.ID, nil
	case "":
		return "", fmt.Errorf("任务缺少类型")
	default:
		return "", fmt.Errorf("这个版本还不支持「%s」这类出片任务", kind)
	}
}

// productInputFromMap 从任务入参还原产品流水线的参数。
func productInputFromMap(input map[string]any) (ProductInput, error) {
	out := ProductInput{
		ProjectID:    strOr(input["projectId"], ""),
		Title:        strOr(input["title"], ""),
		Subtitle:     strOr(input["subtitle"], ""),
		Footer:       strOr(input["footer"], ""),
		Ratio:        strOr(input["ratio"], "9:16"),
		BGMID:        strOr(input["bgmId"], ""),
		VoiceBackend: strOr(input["voiceBackend"], "sherpa"),
		VoiceModel:   strOr(input["voiceModel"], ""),
		VoiceID:      strOr(input["voiceId"], ""),
		Profile:      strOr(input["profile"], ""),
		Tone:         strOr(input["tone"], ""),
	}
	if lines, ok := input["lines"].([]any); ok {
		for _, l := range lines {
			if s, ok := l.(string); ok {
				out.Lines = append(out.Lines, s)
			}
		}
	}
	if out.Lines == nil {
		// 也接受一段多行文本，界面上更省事。
		if text := strOr(input["captions"], ""); text != "" {
			out.Lines = splitLines(text)
		}
	}
	if ids, ok := input["assetIds"].([]any); ok {
		for _, id := range ids {
			if s, ok := id.(string); ok && s != "" {
				out.AssetIDs = append(out.AssetIDs, s)
			}
		}
	}
	return out, nil
}

// splitLines 按换行拆文本，去掉空行。
func splitLines(text string) []string {
	var out []string
	cur := ""
	for _, r := range text {
		if r == '\n' {
			if t := trimSpace(cur); t != "" {
				out = append(out, t)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if t := trimSpace(cur); t != "" {
		out = append(out, t)
	}
	return out
}
