package app

import (
	"fmt"
	"strings"

	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// assetByID 按 ID 找素材。
func (a *App) assetByID(id string) *studioapi.Asset {
	for _, asset := range a.ProjectAssets() {
		if asset.ID == id {
			found := asset
			return &found
		}
	}
	return nil
}

// pipelineJobs 返回某条流水线在当前工作空间里的视频，新的在前。
func (a *App) pipelineJobs(kind string) []studioapi.Job {
	if kind == "" {
		return nil
	}
	var out []studioapi.Job
	for _, job := range a.ProjectJobs() {
		if k, _ := job.Input["kind"].(string); k == kind {
			out = append(out, job)
		}
	}
	return out
}

// jobByID 按 ID 找一条任务。
func jobByID(jobs []studioapi.Job, id string) studioapi.Job {
	for _, job := range jobs {
		if job.ID == id {
			return job
		}
	}
	return studioapi.Job{}
}

// pipelineCount 是这条流水线跑出来的视频条数。
func (a *App) pipelineCount(kind string) int {
	return len(a.pipelineJobs(kind))
}

// jobTitle 给一条视频起个显示用的名字。
func (a *App) jobTitle(job studioapi.Job) string {
	if title, ok := job.Input["title"].(string); ok && title != "" {
		return title
	}
	if asset := a.assetByID(job.AssetID); asset != nil && asset.Name != "" {
		return asset.Name
	}
	return "未命名视频"
}

// jobTime 返回视频的创建时间（短格式）。
func jobTime(job studioapi.Job) string {
	if len(job.CreatedAt) < 10 {
		return ""
	}
	return job.CreatedAt[:10]
}

// expectedSteps 推算一次生成会有多少步。
//
// 不用「已完成步骤数 / 总步骤数」：总步骤数在跑之前并不知道（逐句配音要跑几句，
// 取决于脚本行数）。脚本行数在提交前就知道，所以按它推算，进度条才是准的。
func expectedSteps(kind string, input map[string]any) int {
	lines := 0
	if captions, ok := input["captions"].(string); ok {
		lines = len(splitLines(captions))
	}
	assets := 0
	if list, ok := input["assetIds"].([]any); ok {
		assets = len(list)
	}
	switch kind {
	case "talk":
		// 确认配置 + 逐句配音 + 渲染画面帧 + 合成成片
		total := lines + 3
		if assets == 0 {
			// 没有画面素材时先出占位图，多一步
			total++
		}
		return total
	case "recommendation":
		// 确认配置 + 逐句旁白 + 渲染画面帧 + 合成成片
		return lines + 3
	case "product":
		// 确认配置 + 逐句配音 + 渲染画面帧 + 合成成片
		total := lines + 3
		if assets == 0 {
			// 没有产品画面时先出占位图，多一步
			total++
		}
		return total
	}
	return 0
}

// jobProgress 估算一条视频的生成进度。
//
// 返回进度值（0–1）、当前在做的一步的说明，以及能不能算出确定进度。
// 算不出确定进度时返回 -1，界面上画成「不知道多快」的滚动条，而不是假的百分比。
func (a *App) jobProgress(job studioapi.Job) (float32, string, bool) {
	kind, _ := job.Input["kind"].(string)
	total := expectedSteps(kind, job.Input)

	done, running := 0, false
	runningLabel := ""
	for _, step := range job.Steps {
		switch step.Status {
		case "done":
			done++
		case "running":
			running = true
			runningLabel = step.Label
		}
	}
	if job.Status == "succeeded" {
		return 1, "已完成", true
	}
	if job.Status == "failed" {
		if total > 0 {
			return clampProgress(float64(done) / float64(total)), job.Error, true
		}
		return 0, job.Error, true
	}
	label := job.Message
	if runningLabel != "" {
		label = runningLabel
	}
	if total <= 0 {
		// 这条流水线不报步骤（素材合成就这样），只能给不确定进度
		return -1, label, false
	}
	value := float64(done)
	if running {
		value += 0.5
	}
	return clampProgress(value / float64(total)), label, true
}

// clampProgress 把进度夹到 0–0.99（未完成的不能显示 100%）。
func clampProgress(value float64) float32 {
	if value < 0 {
		return 0
	}
	if value > 0.99 {
		return 0.99
	}
	return float32(value)
}

// pipelineSummary 一句话说明这条流水线当前的状态。
func (a *App) pipelineSummary(kind string) string {
	jobs := a.pipelineJobs(kind)
	if len(jobs) == 0 {
		return "还没有视频"
	}
	running, failed, done := 0, 0, 0
	for _, job := range jobs {
		switch job.Status {
		case "succeeded":
			done++
		case "failed", "interrupted":
			failed++
		default:
			running++
		}
	}
	parts := []string{fmt.Sprintf("%d 条", len(jobs))}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d 条在跑", running))
	}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("%d 条已完成", done))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d 条失败", failed))
	}
	return strings.Join(parts, " · ")
}
