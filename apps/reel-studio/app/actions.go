package app

import (
	"fmt"
	"strings"

	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// submitTalk 提交一条知识口播视频。
//
// 与推荐视频的区别只有「内容从哪来」：这里是用户自己写的脚本，
// 一行一镜；配音、渲染、合成都在本机完成，不产生模型费用。
func (a *App) submitTalk() {
	if a.talkTitle == "" || a.talkCaptions == "" {
		a.fail("标题和口播脚本都要填")
		return
	}
	if a.talkPersona == "" {
		a.fail("先选一个音色")
		return
	}
	backend := a.talkVoice
	if backend == "" {
		backend = a.defaultVoiceChannel()
	}
	if !a.voiceChannelReady(backend) {
		a.fail("配音通道没就绪：" + a.voiceChannelNote(backend))
		return
	}
	lines := 0
	for _, line := range strings.Split(a.talkCaptions, "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	if lines == 0 || lines > 8 {
		a.fail("脚本要 1–8 行，一行一镜")
		return
	}
	for _, line := range strings.Split(a.talkCaptions, "\n") {
		if len([]rune(strings.TrimSpace(line))) > 80 {
			a.fail("有一行超过 80 字，请拆成多镜")
			return
		}
	}
	input := map[string]any{
		"projectId":    a.projectID,
		"kind":         "talk",
		"title":        a.talkTitle,
		"captions":     a.talkCaptions,
		"subtitle":     a.talkSubtitle,
		"style":        a.talkStyle,
		"persona":      a.talkPersona,
		"voiceBackend": backend,
		"ratio":        "9:16",
	}
	if len(a.talkImages) > 0 {
		input["assetIds"] = a.talkImages
	}
	if a.talkTone != "" {
		input["tone"] = a.talkTone
	}
	if a.talkPause != "" {
		pause, err := parseNumberIn("镜间停顿", a.talkPause, 0, 2)
		if err != nil {
			a.fail(err.Error())
			return
		}
		input["pause"] = pause
	}
	if a.talkBGMGain != "" {
		gain, err := bgmGainValue(a.talkBGMGain)
		if err != nil {
			a.fail(err.Error())
			return
		}
		input["bgmGain"] = gain
	}
	if err := a.applyAudioKnobs(input, a.talkAudio); err != nil {
		a.fail(err.Error())
		return
	}
	bgmChoice := a.talkBGM
	projectID := a.projectID
	a.busy.Store(true)
	a.mutate(func() {
		a.notice = ""
	})
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		if bgmChoice != "" {
			bgmID, err := resolveBGM(client, projectID, bgmChoice)
			if err != nil {
				a.busy.Store(false)
				a.mutate(func() {
					a.notice = "取配乐失败：" + err.Error()
					a.noticeErr = true
				})
				a.refresh()
				return
			}
			input["bgmId"] = bgmID
		}
		job, err := client.SubmitJob(input)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
				a.logf("提交口播任务失败：%s", err.Error())
			} else {
				a.notice = "已提交口播视频任务：" + job.ID
				a.noticeErr = false
				a.logf("已提交口播视频任务 %s（%d 镜 · 音色 %s）", job.ID, lines, a.talkPersona)
			}
		})
		a.refresh()
	}()
}

// submitProduct 提交一条人物商品介绍视频。
//
// 与口播共用同一套底座（一句一镜 → 本机配音 → 渲染画面帧 → 合成），
// 差别是内容口径：这里先说清楚「这是谁/这是什么」，再说卖点。
func (a *App) submitProduct() {
	if strings.TrimSpace(a.prodTitle) == "" {
		a.fail("先填产品或人物名称")
		return
	}
	if a.prodPersona == "" {
		a.fail("先选一个音色")
		return
	}
	backend := a.prodVoice
	if backend == "" {
		backend = a.defaultVoiceChannel()
	}
	if !a.voiceChannelReady(backend) {
		a.fail("配音通道没就绪：" + a.voiceChannelNote(backend))
		return
	}
	lines := splitLines(a.prodCaptions)
	if len(lines) == 0 || len(lines) > 8 {
		a.fail("介绍词要 1–8 行，一行一镜")
		return
	}
	for _, line := range lines {
		if len([]rune(line)) > 80 {
			a.fail("有一行超过 80 字，请拆成多镜")
			return
		}
	}
	ratio := a.prodRatio
	if ratio != "16:9" {
		ratio = "9:16"
	}
	input := map[string]any{
		"projectId":    a.projectID,
		"kind":         "product",
		"title":        strings.TrimSpace(a.prodTitle),
		"captions":     a.prodCaptions,
		"subtitle":     a.prodSubtitle,
		"footer":       a.prodFooter,
		"persona":      a.prodPersona,
		"voiceBackend": backend,
		"ratio":        ratio,
	}
	if len(a.prodImages) > 0 {
		input["assetIds"] = a.prodImages
	}
	if a.prodTone != "" {
		input["tone"] = a.prodTone
	}
	if a.prodPause != "" {
		pause, err := parseNumberIn("镜间停顿", a.prodPause, 0, 2)
		if err != nil {
			a.fail(err.Error())
			return
		}
		input["pause"] = pause
	}
	if a.prodBGMGain != "" {
		gain, err := bgmGainValue(a.prodBGMGain)
		if err != nil {
			a.fail(err.Error())
			return
		}
		input["bgmGain"] = gain
	}
	if err := a.applyAudioKnobs(input, a.prodAudio); err != nil {
		a.fail(err.Error())
		return
	}
	bgmChoice := a.prodBGM
	projectID := a.projectID
	persona := a.prodPersona
	a.busy.Store(true)
	a.mutate(func() {
		a.notice = ""
	})
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		if bgmChoice != "" {
			bgmID, err := resolveBGM(client, projectID, bgmChoice)
			if err != nil {
				a.busy.Store(false)
				a.mutate(func() {
					a.notice = "取配乐失败：" + err.Error()
					a.noticeErr = true
				})
				a.refresh()
				return
			}
			input["bgmId"] = bgmID
		}
		job, err := client.SubmitJob(input)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
				a.logf("提交人物商品介绍任务失败：%s", err.Error())
			} else {
				a.notice = "已提交人物商品介绍任务：" + job.ID
				a.noticeErr = false
				a.logf("已提交人物商品介绍任务 %s（%d 镜 · 音色 %s · %s）", job.ID, len(lines), persona, ratio)
			}
		})
		a.refresh()
	}()
}

// readRepoDraft 读 GitHub 仓库，把标题与分镜填进表单。
//
// 草稿只是初稿：它来自 README 与接口截图，可能说得不对，
// 所以填完之后仍然可以改，改完再生成。
func (a *App) readRepoDraft() {
	url := a.repoURL
	projectID := a.projectID
	if url == "" {
		a.fail("先填 GitHub 仓库地址")
		return
	}
	if projectID == "" {
		a.fail("请先选择或新建一个项目")
		return
	}
	a.busy.Store(true)
	a.mutate(func() {
		a.notice = "正在读取仓库…"
		a.noticeErr = false
	})
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		draft, err := client.Draft(projectID, url)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
				a.logf("读取仓库草稿失败：%s", err.Error())
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
				a.logf("读取仓库草稿完成，得到 %d 张画面", len(picked))
			}
		})
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
	projectID := a.projectID
	a.busy.Store(true)
	a.mutate(func() {
		a.notice = "正在按当前方法创建新制作项目…"
		a.noticeErr = false
	})
	a.logf("按当前方法再做一条（原项目 %s）", projectID)
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		res, err := client.Workflow("remake", map[string]any{
			"projectId": projectID,
			"jobId":     jobID,
		})
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
				a.logf("复制制作项目失败：%s", err.Error())
				return
			}
			// 后端返回的是 projectId（不是 id）：拿错键就永远切不过去，
			// 用户点完还停在原项目，以为没生效。
			newID, _ := res["projectId"].(string)
			if newID == "" {
				a.notice = "已克隆制作项目，但没有拿到新项目编号，请刷新后手动选择。"
				a.noticeErr = true
				return
			}
			a.projectID = newID
			// 草稿跟着复制过来了，直接填进表单，省一次重新读取
			if draft, ok := res["draft"].(map[string]any); ok {
				if title, ok := draft["title"].(string); ok && title != "" {
					a.recTitle = title
				}
				if captions, ok := draft["captions"].(string); ok && captions != "" {
					a.recCaptions = captions
				}
				if url, ok := draft["url"].(string); ok && url != "" {
					a.repoURL = url
				}
			}
			a.notice = "已复制项目与素材，可调整分镜与参数后重新出片（本次未提交生成请求）。"
			a.noticeErr = false
			a.logf("已复制制作项目 → %s", newID)
		})
		a.refresh()
	}()
}

// workflowBusy 调一个工作流动作，结束后刷新状态。
func (a *App) workflowBusy(jobID, action string, input map[string]any) {
	a.busy.Store(true)
	a.mutate(func() {
		a.notice = "处理中…"
		a.noticeErr = false
	})
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		_, err := client.Workflow(action, input)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = err.Error()
				a.noticeErr = true
				a.logf("工作流 %s 失败：%s", action, err.Error())
			} else {
				a.notice = fmt.Sprintf("「%s」已完成。", action)
				a.noticeErr = false
				a.logf("工作流 %s 完成", action)
			}
		})
		a.refresh()
	}()
}
