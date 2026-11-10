package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// viewPipelinePanel 画流水线右栏：这条流水线跑出来的视频列表 + 选中视频的详情。
//
// 之前只有「我的成片」一个总数和一张列表，用户看不出
// 「哪条流水线出了几条」「这条视频是怎么来的」「现在跑到哪一步了」。
// 这里一次给全：条数、每条的状态与进度、每条背后的生成配置。
func (a *App) viewPipelinePanel(c *ui.Context) {
	kind := pipelineKind(a.mode)
	jobs := a.pipelineJobs(kind)
	a.kit.SectionTitle(c, "视频", fmt.Sprintf("%d 条", len(jobs)), nil)
	if len(jobs) == 0 {
		a.kit.Empty(c, "这条流水线还没有产出视频",
			"在左边把配置填好，点「生成视频」，产出的视频会出现在这里", nil)
		return
	}
	ui.Column(c).FillWidth().Gap(design.SpaceSM).Children(func() {
		for _, job := range jobs {
			a.viewVideoRow(c, job, kind)
		}
	})
	if selected := a.selectedJob(kind); selected != nil {
		a.viewVideoDetail(c, *selected, kind)
	}
}

// selectedJob 返回当前选中的那条视频。
//
// 没选过就选最近的一条：用户点进流水线就想看最新的产出，
// 不该面对一个空白的详情区。
func (a *App) selectedJob(kind string) *studioapi.Job {
	jobs := a.pipelineJobs(kind)
	if len(jobs) == 0 {
		return nil
	}
	for i := range jobs {
		if jobs[i].ID == a.selectedJobID {
			return &jobs[i]
		}
	}
	return &jobs[0]
}

// compareJob 返回被拿来对比的那条视频。
func (a *App) compareJob(kind string) *studioapi.Job {
	if a.compareJobID == "" {
		return nil
	}
	for _, job := range a.pipelineJobs(kind) {
		if job.ID == a.compareJobID {
			return &job
		}
	}
	return nil
}

// viewVideoRow 画视频列表里的一条：标题、状态、进度条、动作。
func (a *App) viewVideoRow(c *ui.Context, job studioapi.Job, kind string) {
	selected := job.ID == a.selectedJobID
	border := a.kit.T.Line
	if selected {
		border = a.kit.T.Accent
	}
	ui.Column(c).FillWidth().Gap(design.SpaceSM).
		Background(a.kit.T.Surface).Border(1, border).Radius(design.RadiusSM).
		Padding(design.SpaceMD).Children(func() {
		ui.Row(c).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
			title := ui.Button(c, a.jobTitle(job)).Grow(1).Justify(ui.Start).TextAlign(ui.Start).
				TextColor(a.kit.T.Ink).Background(a.kit.T.Surface).Border(0, a.kit.T.Surface).
				FontSize(design.FontBody)
			if title.Clicked() {
				a.selectedJobID = job.ID
				a.compareJobID = ""
				a.redraw()
			}
			a.kit.Tag(c, statusLabel(job.Status), statusTagKind(job.Status))
			if t := jobTime(job); t != "" {
				ui.Text(c, t).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
			}
		})
		a.viewJobProgress(c, job)
		ui.Row(c).FillWidth().Gap(design.SpaceSM).Children(func() {
			if job.Status == "succeeded" {
				if asset := a.assetByID(job.AssetID); asset != nil {
					if button := kitButton(a.kit, c, kit.ButtonSecondary, "保存到本地", false); button.Clicked() {
						go a.saveFinishedVideo(*asset)
					}
				}
			}
			if job.AssetID == "" && job.Status == "failed" {
				if button := kitButton(a.kit, c, kit.ButtonSecondary, "取回产物", false); button.Clicked() {
					go a.resume(job.ID)
				}
			}
			compare := kitButton(a.kit, c, kit.ButtonSecondary, "对比这条", false)
			if compare.Clicked() {
				a.selectedJobID = job.ID
				a.compareJobID = ""
				a.comparePick = job.ID
				a.redraw()
			}
		})
	})
}

// viewJobProgress 画一条任务的进度条。
//
// 进度是用户最想看的一件事：合成一条视频要几十秒，没有进度条，
// 用户只能盯着一个「已完成」反复刷新。
func (a *App) viewJobProgress(c *ui.Context, job studioapi.Job) {
	value, label, determinate := a.jobProgress(job)
	if job.Status == "succeeded" {
		return
	}
	a.kit.Meter(c, value, label, determinate)
}

// statusTagKind 把任务状态映射到标签颜色。
func statusTagKind(status string) string {
	switch status {
	case "succeeded":
		return "success"
	case "failed", "interrupted":
		return "brand"
	}
	return ""
}

// viewVideoDetail 画选中视频的详情：画面、生成配置、调整与对比。
func (a *App) viewVideoDetail(c *ui.Context, job studioapi.Job, kind string) {
	a.kit.SectionTitle(c, "这条视频", a.jobTitle(job), nil)
	ui.Column(c).FillWidth().Gap(design.SpaceMD).Children(func() {
		if asset := a.assetByID(job.AssetID); asset != nil {
			a.viewPlayer(c, *asset)
			var parts []string
			if asset.Width > 0 && asset.Height > 0 {
				parts = append(parts, fmt.Sprintf("%d×%d", asset.Width, asset.Height))
			}
			if asset.Duration > 0 {
				parts = append(parts, fmt.Sprintf("%.1f 秒", asset.Duration))
			}
			if asset.Engine != "" {
				parts = append(parts, asset.Engine)
			}
			if len(parts) > 0 {
				ui.Text(c, strings.Join(parts, " · ")).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
			}
			itemAsset := *asset
			ui.Row(c).Gap(design.SpaceSM).Children(func() {
				if button := kitButton(a.kit, c, kit.ButtonPrimary, "保存到本地", false); button.Clicked() {
					go a.saveFinishedVideo(itemAsset)
				}
			})
		} else if job.Status == "failed" {
			a.kit.Hint(c, "没出成片："+orDash(job.Error))
		} else if job.Status == "succeeded" {
			a.kit.Hint(c, "成片还没取回本地缓存，点「取回产物」。")
		}
	})
	// 对比：选了另一条就把差异列出来
	if other := a.compareJob(kind); other != nil && other.ID != job.ID {
		a.kit.SectionTitle(c, "配置对比", "只列不一样的项", nil)
		a.viewConfigDiff(c, kind, job, *other, a.jobTitle(job), a.jobTitle(*other))
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "结束对比", false); button.Clicked() {
			a.compareJobID = ""
			a.redraw()
		}
		return
	}
	a.viewJobSteps(c, job.Steps)
	a.kit.SectionTitle(c, "生成配置", "这条视频是怎么来的", nil)
	a.viewConfigPanel(c, kind, job.Input)
	ui.Row(c).FillWidth().Gap(design.SpaceSM).Children(func() {
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "按此配置再生成一条", false); button.Clicked() {
			a.applyConfig(kind, job.Input, job.ID)
			a.notice = "已把这条视频的配置载入左边表单，改完点「生成视频」就是新的一条"
			a.noticeErr = false
			a.redraw()
		}
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "找一条对比", false); button.Clicked() {
			a.comparePick = job.ID
			a.redraw()
		}
	})
	if a.comparePick != "" {
		a.viewComparePicker(c, kind, job)
	}
}

// viewComparePicker 画「拿哪一条来对比」的选择器。
//
// comparePick 非空就意味着「正在挑对比对象」，列的是除当前这条以外的视频。
func (a *App) viewComparePicker(c *ui.Context, kind string, job studioapi.Job) {
	others := a.pipelineJobs(kind)
	ui.Column(c).FillWidth().Gap(design.SpaceXS).Children(func() {
		ui.Text(c, "和哪一条对比？").FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
		for _, other := range others {
			if other.ID == job.ID {
				continue
			}
			item := other
			button := ui.Button(c, "对比「"+a.jobTitle(item)+"」").FillWidth().Justify(ui.Start).TextAlign(ui.Start).
				TextColor(a.kit.T.Ink).Background(a.kit.T.Surface).Border(1, a.kit.T.Line).
				FontSize(design.FontSmall)
			if button.Clicked() {
				a.compareJobID = item.ID
				a.comparePick = ""
				a.redraw()
			}
		}
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "不对比了", false); button.Clicked() {
			a.comparePick = ""
			a.redraw()
		}
	})
}

// viewPlayer 画素材画面：视频带进度与传输控件，图片直接显示。
func (a *App) viewPlayer(c *ui.Context, asset studioapi.Asset) {
	img, vid, failed, _ := a.mediaFor(asset)
	isImage := strings.HasPrefix(asset.Mime, "image/")
	isVideo := strings.HasPrefix(asset.Mime, "video/")
	frame := func(fn func()) {
		ui.Column(c).FillWidth().Height(320).Center().Radius(design.RadiusMD).
			Background(a.kit.T.SurfaceAlt).Children(fn)
	}
	switch {
	case isImage && img != nil:
		frame(func() { ui.Image(c, img).Fill().Fit(ui.Contain) })
	case isVideo && vid != nil:
		frame(func() { ui.Video(c, vid).Fill().Fit(ui.Contain) })
		a.viewTransport(c, vid)
	case failed != "":
		frame(func() { a.kit.Hint(c, "预览加载失败："+failed) })
	default:
		frame(func() { a.kit.Hint(c, "正在加载预览…") })
	}
}

// viewTransport 画播放进度与传输控件。
//
// 只有画面没有进度，用户不知道这条视频有多长、播到哪了，
// 也不知道是不是卡住了 —— 播放条是最省事的答案。
func (a *App) viewTransport(c *ui.Context, vid *ui.VideoPlayer) {
	duration := vid.Duration()
	position := vid.Position()
	value := float32(0)
	if duration > 0 {
		value = float32(position) / float32(duration)
		if value > 1 {
			value = 1
		}
	}
	ui.Progress(c, float64(value)).FillWidth().Height(4).Radius(design.RadiusSM).TextColor(a.kit.T.Accent)
	// 时间单独一行，按钮单独一行：挤在一行时窄窗口下按钮会被切掉。
	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf("%.1f 秒", position.Seconds())).
			Grow(1).FontSize(design.FontTiny).TextColor(a.kit.T.Ink)
		ui.Text(c, fmt.Sprintf("共 %.1f 秒", duration.Seconds())).
			FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
	})
	ui.Row(c).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "开头", false); button.Clicked() {
			vid.Seek(0)
		}
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "-5 秒", false); button.Clicked() {
			vid.Seek(maxDuration(0, position-5*time.Second))
		}
		label := "播放"
		if vid.Playing() {
			label = "暂停"
		}
		if button := kitButton(a.kit, c, kit.ButtonPrimary, label, false); button.Clicked() {
			if vid.Playing() {
				vid.Pause()
			} else {
				vid.Play()
			}
		}
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "+5 秒", false); button.Clicked() {
			vid.Seek(minDuration(duration, position+5*time.Second))
		}
	})
}

// maxDuration 取较大的时长。
func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

// minDuration 取较小的时长。
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// originLabel 把底层工序来源翻译成清晰的人话标签。
func originLabel(origin string) string {
	switch origin {
	case "code":
		return "本地代码渲染"
	case "free-online":
		return "在线免费模型"
	case "local-offline":
		return "本机离线模型"
	case "cloud-paid", "ai":
		return "AI 模型生成"
	case "import":
		return "用户导入素材"
	default:
		return "本地工序"
	}
}

// viewJobSteps 画制作这支视频经历的每一道工序。
//
// 让用户一眼看清：哪几步是代码执行、哪几步是 AI 模型、哪几步用了自己的素材。
func (a *App) viewJobSteps(c *ui.Context, steps []studioapi.Step) {
	if len(steps) == 0 {
		return
	}
	a.kit.SectionTitle(c, "制作工序与分层", "底层由代码渲染、AI 模型生成与本地素材分工协作", nil)
	ui.Column(c).FillWidth().Gap(design.SpaceXS).Children(func() {
		for _, step := range steps {
			ui.Row(c).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).
				Background(a.kit.T.Surface).Border(1, a.kit.T.Line).Radius(design.RadiusSM).
				Padding(design.SpaceSM, design.SpaceMD).Children(func() {
				ui.Text(c, step.Label).FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
				a.kit.Tag(c, originLabel(step.Origin), "secondary")
				if step.Engine != "" {
					ui.Text(c, step.Engine).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
				}
				ui.Spacer(c)
				if step.Detail != "" {
					ui.Text(c, truncateRunes(step.Detail, 20)).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
				}
				if step.MS > 0 {
					ui.Text(c, fmt.Sprintf("%.1fs", float64(step.MS)/1000)).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
				}
			})
		}
	})
}
