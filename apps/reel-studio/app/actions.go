package app

import (
	"fmt"
	"strings"

	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// readRepoDraft 读 GitHub 仓库，把标题与分镜填进表单。
//
// 草稿只是初稿：它来自 README 与接口截图，可能说得不对，
// 所以填完之后仍然可以改，改完再生成。
func (a *App) readRepoDraft() {
	url := a.repoURL
	if url == "" {
		a.fail("先填 GitHub 仓库地址")
		return
	}
	a.mu.Lock()
	a.busy = true
	a.notice = "正在读取仓库…"
	a.noticeErr = false
	a.mu.Unlock()
	a.redraw()
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		draft, err := client.Draft(a.projectID, url)
		a.mu.Lock()
		a.busy = false
		if err != nil {
			a.notice = err.Error()
			a.noticeErr = true
		} else {
			if title, ok := draft["title"].(string); ok && title != "" {
				a.recTitle = title
			}
			a.recCaptions = joinCaptions(draft["captions"])
			// 读取项目时后端已经把仓库里的展示图收进项目素材，这里默认全选。
			var picked []string
			if list, ok := draft["images"].([]any); ok {
				for _, item := range list {
					asset, ok := item.(map[string]any)
					if !ok {
						continue
					}
					if id, ok := asset["id"].(string); ok && id != "" {
						picked = append(picked, id)
						a.recSelected[id] = true
					}
				}
			}
			a.recImages = picked
			if len(picked) == 0 {
				a.notice = "已读取项目介绍；仓库里没有展示图，请在素材库导入图片并勾选。"
				a.noticeErr = true
			} else {
				a.notice = fmt.Sprintf("已读取项目草稿与 %d 张画面，确认文案后再生成。", len(picked))
				a.noticeErr = false
			}
		}
		a.mu.Unlock()
		a.refresh()
	}()
}

// joinCaptions 把草稿里的分镜整理成一行一句的文本。
//
// 接口给的是数组；给回来的也可能是已经拼好的字符串，两种都认。
func joinCaptions(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		var lines []string
		for _, item := range value {
			if line, ok := item.(string); ok && strings.TrimSpace(line) != "" {
				lines = append(lines, strings.TrimSpace(line))
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

// runQuality 对成片做自动检查。
func (a *App) runQuality(jobID string) {
	a.workflowBusy(jobID, "quality", map[string]any{
		"projectId": a.projectID,
		"jobId":     jobID,
	})
}

// approveReview 记录人工审片结论。
func (a *App) approveReview(jobID string) {
	for _, item := range reviewItems {
		if !a.reviewChecks[item.key] {
			a.fail("还有审片项没确认：" + item.label)
			return
		}
	}
	a.workflowBusy(jobID, "review-check", map[string]any{
		"projectId": a.projectID,
		"jobId":     jobID,
		"checks":    a.reviewChecks,
	})
}

// savePublish 记录一次发布。
func (a *App) savePublish(jobID string) {
	if a.pubURL == "" {
		a.fail("填一下发布链接")
		return
	}
	a.workflowBusy(jobID, "publish", map[string]any{
		"projectId": a.projectID,
		"jobId":     jobID,
		"platform":  a.pubPlatform,
		"url":       a.pubURL,
	})
}

// savePerformance 回填发布后的表现。
func (a *App) savePerformance(jobID string) {
	published := a.publishFor(jobID)
	if published == nil {
		a.fail("这条视频还没有发布记录")
		return
	}
	a.workflowBusy(jobID, "performance", map[string]any{
		"projectId": a.projectID,
		"jobId":     jobID,
		"publishId": published.ID,
		"window":    "7d",
		"metrics": map[string]int{
			"plays": atoi(a.perfPlays),
			"likes": atoi(a.perfLikes),
			"saves": atoi(a.perfSaves),
		},
	})
}

// saveReview 记录复盘，并冻结出下一版制作方法。
func (a *App) saveReview(jobID string) {
	if a.reviewFindings == "" || a.reviewHypothesis == "" || a.reviewChanges == "" {
		a.fail("发现、假设和具体改动都要填")
		return
	}
	a.workflowBusy(jobID, "review", map[string]any{
		"projectId":  a.projectID,
		"jobId":      jobID,
		"findings":   a.reviewFindings,
		"hypothesis": a.reviewHypothesis,
		"changes":    a.reviewChanges,
	})
}

// remakeJob 按当前方法再做一条：复用已有素材，不重新付费生成。
func (a *App) remakeJob(jobID string) {
	a.workflowBusy(jobID, "remake", map[string]any{
		"projectId": a.projectID,
		"jobId":     jobID,
	})
}

// workflowBusy 调一个工作流动作，结束后刷新状态。
func (a *App) workflowBusy(jobID, action string, input map[string]any) {
	a.mu.Lock()
	a.busy = true
	a.notice = "处理中…"
	a.noticeErr = false
	a.mu.Unlock()
	a.redraw()
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		_, err := client.Workflow(action, input)
		a.mu.Lock()
		a.busy = false
		if err != nil {
			a.notice = err.Error()
			a.noticeErr = true
		} else {
			a.notice = fmt.Sprintf("「%s」已完成。", action)
			a.noticeErr = false
		}
		a.mu.Unlock()
		a.refresh()
	}()
}
