package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// choice 是一个下拉选项：id 写回状态，label 显示给人看。
type choice struct{ id, label string }

// pick 画一个下拉，把「写回的值」与「显示的文字」分开。
//
// MyGo 的 Select 没有占位：当前值不在选项里时只显示一片空白，
// 用户看不出是「没选」还是「坏了」。所以「由通道决定」这类选择
// 必须是一个真实的选项；当前值为空时落到第一项，并写回状态，
// 这样界面显示的就是将要提交的值。
//
// 当前值是「非空但不在选项里」时**不能**悄悄换成第一项 —— 那等于替用户
// 换供应商或换模型，用户点了才知道。这种情况把原值保留成一个标注过的选项，
// 让用户自己决定换不换。
func (a *App) pick(c *ui.Context, label, help string, choices []choice, current string, set func(string)) {
	if len(choices) == 0 {
		return
	}
	options := make([]string, len(choices))
	selected := ""
	found := false
	for i, ch := range choices {
		options[i] = ch.label
		if ch.id == current {
			selected = ch.label
			found = true
		}
	}
	if !found {
		if current == "" {
			// 从没选过：落到第一项并写回，界面显示的就是将要提交的值
			selected = choices[0].label
			set(choices[0].id)
		} else {
			// 选过但当前选项里没有（通道换了、上游下架了）：原值留着并标注
			options = append(options, current+"（当前不可选）")
			selected = options[len(options)-1]
		}
	}
	a.kit.Field(c, label, help, func() {
		if a.kit.Select(c, &selected, options, "").FillWidth().Changed() {
			for _, ch := range choices {
				if ch.label == selected {
					set(ch.id)
					a.redraw()
					break
				}
			}
		}
	})
}

// projects 返回项目列表；还没连上后端时为空，不能让界面因为 nil 崩掉。
func (a *App) projects() []studioapi.Project {
	if a.state == nil {
		return nil
	}
	return a.state.Projects
}

// viewChannelPicker 画通道选择。
func (a *App) viewChannelPicker(c *ui.Context) {
	channels := a.Channels()
	if len(channels) == 0 {
		a.kit.Hint(c, "这一类能力还没有可用通道，左下角「连接状态」写明了原因。")
		return
	}
	choices := make([]choice, 0, len(channels))
	for _, ch := range channels {
		label := ch.Label
		if !ch.Ready {
			label += "（未就绪）"
		}
		choices = append(choices, choice{ch.ID, label})
	}
	a.pick(c, "生成通道", "", choices, a.backend, func(id string) {
		if id != a.backend {
			// 换通道 = 换一套模型、音色与计费方式，旧选择一律作废
			a.backend, a.model, a.voiceID = id, "", ""
		}
	})
	if ch := a.CurrentChannel(); ch != nil && ch.Note != "" {
		a.kit.Hint(c, ch.Note)
	}
}

// viewModelPicker 画模型、音色与计费账户。三者是独立的选择，分开画。
func (a *App) viewModelPicker(c *ui.Context) {
	ch := a.CurrentChannel()
	if ch == nil {
		return
	}
	if len(ch.Models) > 0 {
		var choices []choice
		// 只有中台能「自动路由」；直连通道的空值其实是固定默认模型，不能叫自动
		if ch.ID == "museav" {
			choices = append(choices, choice{"", "由中台自动选择"})
		}
		for _, m := range ch.Models {
			choices = append(choices, choice{m.ID, m.Name})
		}
		a.pick(c, "模型", "", choices, a.model, func(id string) { a.model = id })
	}
	if len(ch.Voices) > 0 {
		choices := make([]choice, 0, len(ch.Voices))
		for _, v := range ch.Voices {
			choices = append(choices, choice{v.ID, v.Name})
		}
		label := "音色"
		if ch.ID == "voxflow" {
			label = "本地音色"
		}
		a.pick(c, label, "", choices, a.voiceID, func(id string) { a.voiceID = id })
	}
	if ch.ID == "minimax" && a.providers != nil && len(a.providers.Video.Profiles) > 0 {
		choices := make([]choice, 0, len(a.providers.Video.Profiles))
		for _, p := range a.providers.Video.Profiles {
			choices = append(choices, choice{p.ID, p.Label})
		}
		a.pick(c, "计费账户", "不同账户是不同的额度；不会自动换账户重试", choices, a.profile, func(id string) { a.profile = id })
	}
}

// previewAsset 返回预览区要显示的素材：用户点过的，否则最新一条。
func (a *App) previewAsset() *studioapi.Asset {
	assets := a.ProjectAssets()
	for i := range assets {
		if assets[i].ID == a.previewID {
			return &assets[i]
		}
	}
	if len(assets) > 0 {
		return &assets[len(assets)-1]
	}
	return nil
}

// viewPreview 画预览：真实的画面在上，元数据在下。
func (a *App) viewPreview(c *ui.Context) {
	a.kit.SectionTitle(c, "预览", "", nil)
	current := a.previewAsset()
	if current == nil {
		a.kit.Empty(c, "这个项目的第一帧，等你创造", "生成或导入素材后在这里预览", nil)
		return
	}
	ui.Column(c).FillWidth().Gap(design.SpaceSM).Background(a.kit.T.Surface).
		Border(1, a.kit.T.Line).Radius(design.RadiusLG).Padding(design.SpaceLG).Children(func() {
		a.viewMedia(c, *current)
		ui.Text(c, current.Name).FontSize(design.FontLarge).TextColor(a.kit.T.Ink)
		var parts []string
		if current.Width > 0 && current.Height > 0 {
			parts = append(parts, fmt.Sprintf("%d×%d", current.Width, current.Height))
		}
		if current.Duration > 0 {
			parts = append(parts, fmt.Sprintf("%.1f 秒", current.Duration))
		}
		if current.Engine != "" {
			parts = append(parts, current.Engine)
		}
		if len(parts) > 0 {
			ui.Text(c, strings.Join(parts, " · ")).FontSize(design.FontSmall).TextColor(a.kit.T.Muted)
		}
		// 请求模型与实际模型分开写：拿不到回执就是 unknown，不猜
		if current.RequestedModel != "" || current.ActualModel != "" {
			actual := current.ActualModel
			if actual == "" {
				actual = "unknown"
			}
			ui.Text(c, "请求模型："+orDash(current.RequestedModel)+" · 实际模型："+actual).
				FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
		}
		if current.License != "" {
			ui.Text(c, "授权："+current.License).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
		}
	})
}

// viewMedia 画素材本身：图片、视频直接显示，其余给一句说明。
func (a *App) viewMedia(c *ui.Context, asset studioapi.Asset) {
	isImage := strings.HasPrefix(asset.Mime, "image/")
	isVideo := strings.HasPrefix(asset.Mime, "video/")
	if !isImage && !isVideo {
		return
	}
	img, vid, failed, _ := a.mediaFor(asset)
	frame := func(fn func()) {
		ui.Column(c).FillWidth().Height(320).Center().Radius(design.RadiusMD).
			Background(a.kit.T.SurfaceAlt).Children(fn)
	}
	switch {
	case img != nil:
		frame(func() { ui.Image(c, img).Fill().Fit(ui.Contain) })
	case vid != nil:
		frame(func() { ui.Video(c, vid).Fill().Fit(ui.Contain) })
	case failed != "":
		frame(func() { a.kit.Hint(c, "预览加载失败："+failed) })
	default:
		frame(func() { a.kit.Hint(c, "正在加载预览…") })
	}
}

// viewAssets 画素材库：名称左对齐，来源标签跟在同一行。
func (a *App) viewAssets(c *ui.Context) {
	assets := a.ProjectAssets()
	a.kit.SectionTitle(c, "素材", fmt.Sprintf("%d 项", len(assets)), nil)
	if len(assets) == 0 {
		a.kit.Hint(c, "这个项目还没有素材。")
		return
	}
	current := a.previewAsset()
	ui.Column(c).FillWidth().Gap(design.SpaceSM).Children(func() {
		for _, asset := range assets {
			item := asset
			border := a.kit.T.Line
			if current != nil && item.ID == current.ID {
				border = a.kit.T.Accent
			}
			ui.Row(c).FillWidth().Gap(design.SpaceMD).AlignItems(ui.Center).
				Background(a.kit.T.Surface).Border(1, border).Radius(design.RadiusSM).
				Padding(design.SpaceXS, design.SpaceSM).Children(func() {
				b := ui.Button(c, item.Name).Grow(1).Justify(ui.Start).TextAlign(ui.Start).
					TextColor(a.kit.T.Ink).Background(a.kit.T.Surface).Border(0, a.kit.T.Surface).
					FontSize(design.FontBody)
				if b.Clicked() {
					a.previewID = item.ID
					a.redraw()
				}
				if item.Source != "" {
					a.kit.Tag(c, item.Source, "brand")
				}
			})
		}
	})
}

// viewRecommendationComposer 画项目推荐这一屏：仓库 → 分镜 → 配音 → 配乐 → 出片。
//
// 这条是本产品的主线：读一个 GitHub 仓库，直接得到一条能发的推荐视频。
func (a *App) viewRecommendationComposer(c *ui.Context) {
	a.kit.Field(c, "GitHub 仓库", "填仓库地址或 owner/repo", func() {
		a.kit.Input(c, &a.repoURL, "https://github.com/owner/repo")
	})
	ui.Row(c).Gap(design.SpaceSM).Children(func() {
		button := kitButton(a.kit, c, kit.ButtonSecondary, "读取项目", a.busy.Load() || a.repoURL == "")
		if button.Clicked() {
			a.readRepoDraft()
		}
		if a.busy.Load() {
			a.kit.Hint(c, "正在读取…")
		}
	})
	a.kit.Field(c, "标题", "", func() {
		a.kit.Input(c, &a.recTitle, "这条视频的标题")
	})
	a.kit.Field(c, "分镜口播", "一行一句，念出来就是这条视频的正文", func() {
		a.kit.TextArea(c, &a.recCaptions, "第一句说清这个项目解决什么问题", 120)
	})
	a.pick(c, "发布平台", "平台决定画幅与字幕排布", []choice{
		{"channels", "视频号"},
		{"xiaohongshu", "小红书"},
		{"bilibili", "横屏（16:9）"},
	}, a.recPlatform, func(id string) {
		a.recPlatform = id
		if id == "bilibili" {
			a.recRatio = "16:9"
		} else {
			a.recRatio = "9:16"
		}
	})
	a.viewChannelPicker(c)
	a.viewModelPicker(c)
	a.viewBGMPicker(c)
	a.viewStoryboardImages(c)
}

// viewBGMPicker 画配乐选择：默认曲库（带授权）与项目里的本地音频。
func (a *App) viewBGMPicker(c *ui.Context) {
	choices := []choice{{"", "不用配乐"}}
	if a.bgm != nil {
		for _, item := range a.bgm.Items {
			label := item.Alias
			if item.License != "" {
				label += "（" + item.License + "）"
			}
			choices = append(choices, choice{"r2:" + item.Alias, label})
		}
	}
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "audio/") {
			choices = append(choices, choice{"asset:" + asset.ID, "本地：" + asset.Name})
		}
	}
	if len(choices) == 1 && a.bgm == nil {
		a.kit.Field(c, "配乐", "默认曲库暂时拉不到，稍后再试；也可以先用本地音频", func() {
			a.kit.Hint(c, "没有可用的配乐")
		})
		return
	}
	a.pick(c, "配乐", "选用后会复制进项目，来源与授权一并记录", choices, a.recBGM, func(id string) {
		a.recBGM = id
	})
}

// viewStoryboardImages 画分镜画面：勾哪几张，就按列表顺序入片。
func (a *App) viewStoryboardImages(c *ui.Context) {
	var images []studioapi.Asset
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "image/") {
			images = append(images, asset)
		}
	}
	if len(images) == 0 {
		a.kit.Field(c, "分镜画面", "项目里还没有图片；先「读取项目」抓仓库的展示图，或先生成图片", func() {
			a.kit.Hint(c, "暂无画面")
		})
		return
	}
	a.kit.Field(c, "分镜画面", fmt.Sprintf("已选 %d 张，按列表顺序入片", len(a.recImages)), func() {
		ui.Column(c).FillWidth().Gap(design.SpaceXS).Children(func() {
			for _, image := range images {
				item := image
				checked := a.recSelected[item.ID]
				row := ui.Row(c).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
					box := a.kit.Checkbox(c, &checked, item.Name)
					if box.Changed() {
						a.recSelected[item.ID] = checked
						a.syncSelectedImages()
					}
					a.kit.Tag(c, fmt.Sprintf("%d×%d", item.Width, item.Height), "")
				})
				_ = row
			}
		})
	})
}

// syncSelectedImages 按勾选状态重排分镜画面，保持素材列表的先后顺序。
func (a *App) syncSelectedImages() {
	var picked []string
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "image/") && a.recSelected[asset.ID] {
			picked = append(picked, asset.ID)
		}
	}
	a.recImages = picked
}

// viewWorkflow 画制作闭环：自动检查 → 人工审片 → 发布 → 表现 → 复盘 → 再做一条。
func (a *App) viewWorkflow(c *ui.Context) {
	job := a.recommendationJob()
	if job == nil {
		a.kit.SectionTitle(c, "制作闭环", "", nil)
		a.kit.Hint(c, "先做出一条推荐视频，这里才有发布、表现与复盘可记。")
		return
	}
	a.kit.SectionTitle(c, "制作闭环", "当前成片："+job.ID, nil)
	ui.Column(c).FillWidth().Gap(design.SpaceLG).Children(func() {
		a.viewQuality(c)
		a.viewReview(c)
		a.viewPublish(c)
		a.viewPerformance(c)
		a.viewRetrospective(c)
	})
}

// recommendationJob 返回本项目最近一条成功的推荐视频。
func (a *App) recommendationJob() *studioapi.Job {
	jobs := a.ProjectJobs()
	for i := range jobs {
		j := jobs[i]
		if j.Status == "succeeded" && j.AssetID != "" && j.Input["kind"] == "recommendation" {
			return &j
		}
	}
	return nil
}

// viewQuality 画自动检查。
func (a *App) viewQuality(c *ui.Context) {
	job := a.recommendationJob()
	if job == nil {
		return
	}
	if job.Quality == nil {
		button := kitButton(a.kit, c, kit.ButtonSecondary, "自动检查成片", a.busy.Load())
		if button.Clicked() {
			a.runQuality(job.ID)
		}
		return
	}
	q := job.Quality
	status := "未通过"
	kind := "brand"
	if q.Passed {
		status, kind = "通过", "success"
	}
	ui.Row(c).FillWidth().Gap(design.SpaceMD).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "自动检查").FontSize(design.FontBody).TextColor(a.kit.T.Ink).Grow(1)
		a.kit.Tag(c, status, kind)
	})
	for _, check := range q.Checks {
		ui.Row(c).FillWidth().Gap(design.SpaceSM).Children(func() {
			mark := "○"
			if check.Passed {
				mark = "●"
			}
			ui.Text(c, mark).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
			ui.Text(c, check.Label).FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
		})
	}
}

// reviewItems 是人工审片的七项，顺序与网页版一致。
var reviewItems = []struct{ key, label string }{
	{"opening", "开头说明项目价值"},
	{"truth", "信息真实且有来源"},
	{"matching", "画面与讲解相符"},
	{"ending", "结尾有明确行动"},
	{"subtitles", "字幕清晰且基本同步"},
	{"audio", "声音清晰，音乐不盖人声"},
	{"visuals", "画面清晰、关键内容未被裁切"},
}

// viewReview 画人工审片。
func (a *App) viewReview(c *ui.Context) {
	job := a.recommendationJob()
	if job == nil || job.Quality == nil || !job.Quality.Passed {
		return
	}
	if job.Approval != nil {
		ui.Text(c, "人工审片：已确认").FontSize(design.FontBody).TextColor(a.kit.T.Ink)
		return
	}
	ui.Text(c, "人工审片").FontSize(design.FontBody).TextColor(a.kit.T.Ink)
	for _, item := range reviewItems {
		key := item.key
		checked := a.reviewChecks[key]
		row := a.kit.Checkbox(c, &checked, item.label)
		if row.Changed() {
			a.reviewChecks[key] = checked
		}
	}
	button := kitButton(a.kit, c, kit.ButtonSecondary, "确认审片通过", a.busy.Load())
	if button.Clicked() {
		a.approveReview(job.ID)
	}
}

// viewPublish 画发布登记。
func (a *App) viewPublish(c *ui.Context) {
	job := a.recommendationJob()
	if job == nil || job.Approval == nil {
		return
	}
	published := a.publishFor(job.ID)
	if published != nil {
		ui.Text(c, "发布："+published.Platform+" · "+published.URL).FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
		return
	}
	a.kit.Field(c, "发布平台", "", func() {
		a.kit.Input(c, &a.pubPlatform, "视频号 / 小红书 / B 站")
	})
	a.kit.Field(c, "发布链接", "", func() {
		a.kit.Input(c, &a.pubURL, "https://")
	})
	button := kitButton(a.kit, c, kit.ButtonSecondary, "记录发布", a.busy.Load() || a.pubURL == "")
	if button.Clicked() {
		a.savePublish(job.ID)
	}
}

// viewPerformance 画表现回填。
func (a *App) viewPerformance(c *ui.Context) {
	job := a.recommendationJob()
	if job == nil {
		return
	}
	published := a.publishFor(job.ID)
	if published == nil {
		return
	}
	recorded := a.performanceFor(published.ID)
	if recorded != nil {
		ui.Text(c, fmt.Sprintf("表现：播放 %d · 点赞 %d · 收藏 %d", recorded.Metrics["plays"], recorded.Metrics["likes"], recorded.Metrics["saves"])).
			FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
		return
	}
	a.kit.Field(c, "表现数据", "播放 / 点赞 / 收藏（窗口按 7 天算）", func() {
		ui.Row(c).Gap(design.SpaceSM).Children(func() {
			a.kit.Input(c, &a.perfPlays, "播放")
			a.kit.Input(c, &a.perfLikes, "点赞")
			a.kit.Input(c, &a.perfSaves, "收藏")
		})
	})
	button := kitButton(a.kit, c, kit.ButtonSecondary, "回填表现", a.busy.Load())
	if button.Clicked() {
		a.savePerformance(job.ID)
	}
}

// viewRetrospective 画复盘与再制作。
func (a *App) viewRetrospective(c *ui.Context) {
	job := a.recommendationJob()
	if job == nil {
		return
	}
	published := a.publishFor(job.ID)
	if published == nil {
		return
	}
	ui.Text(c, "复盘").FontSize(design.FontBody).TextColor(a.kit.T.Ink)
	a.kit.Field(c, "发现", "这条视频哪里没做到位", func() {
		a.kit.TextArea(c, &a.reviewFindings, "例：前 3 秒没讲清用途，划走率高", 60)
	})
	a.kit.Field(c, "假设", "你觉得问题出在哪", func() {
		a.kit.TextArea(c, &a.reviewHypothesis, "例：开头先讲功能，再讲技术", 60)
	})
	a.kit.Field(c, "具体改动", "下一条要改什么", func() {
		a.kit.TextArea(c, &a.reviewChanges, "例：分镜第一句换成「把 X 变成 Y」", 60)
	})
	ui.Row(c).Gap(design.SpaceSM).Children(func() {
		button := kitButton(a.kit, c, kit.ButtonSecondary, "记录复盘并生成新方法", a.busy.Load())
		if button.Clicked() {
			a.saveReview(job.ID)
		}
		remake := kitButton(a.kit, c, kit.ButtonPrimary, "按新方法再做一条", a.busy.Load())
		if remake.Clicked() {
			a.remakeJob(job.ID)
		}
	})
}

// publishFor 返回某个任务对应的发布记录。
func (a *App) publishFor(jobID string) *studioapi.Publication {
	if a.state == nil {
		return nil
	}
	for i := range a.state.Publishes {
		if a.state.Publishes[i].JobID == jobID {
			p := a.state.Publishes[i]
			return &p
		}
	}
	return nil
}

// performanceFor 返回某个发布对应的表现记录。
func (a *App) performanceFor(publishID string) *studioapi.Performance {
	if a.state == nil {
		return nil
	}
	for i := range a.state.Performance {
		if a.state.Performance[i].PublishID == publishID {
			p := a.state.Performance[i]
			return &p
		}
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
