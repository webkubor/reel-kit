package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// viewExportComposer 画素材合成的生成配置。
//
// 这也是一条流水线（产出视频），所以按同样的七块分组：
// 选素材 → 写字幕 → 配音 → 选版式 → 定画幅 → 出片。
func (a *App) viewExportComposer(c *ui.Context) {
	// 顺序跟其它流水线一样：脚本 → 声音 → 画面 → 版式 → 画幅 → 输出。
	// 素材合成也一样按七块走，用户在三条流水线之间不用重新学一遍顺序。
	a.partTitle(c, partScript)
	a.kit.Field(c, "标题", "", func() {
		a.kit.Input(c, &a.expTitle, "这条视频的标题")
	})
	a.kit.Field(c, "字幕", "每张素材一行；行数要和素材数一致", func() {
		a.kit.TextArea(c, &a.expCaptions, "第一张配这句\n第二张配这句", 100)
	})

	a.partTitle(c, partVoice)
	{
		choices := []choice{{"", "不用配音"}}
		for _, asset := range a.ProjectAssets() {
			if strings.HasPrefix(asset.Mime, "audio/") {
				choices = append(choices, choice{asset.ID, asset.Name})
			}
		}
		a.pick(c, "配音", "可选；选一条音频素材铺在成片上", choices, a.expAudioID, func(id string) { a.expAudioID = id })
	}

	a.partTitle(c, partVisual)
	a.kit.Field(c, "选择素材", "按勾选顺序合成；一次选图片或 MP4，不要混选。", func() {
		count := 0
		for _, asset := range a.ProjectAssets() {
			if !strings.HasPrefix(asset.Mime, "image/") && asset.Mime != "video/mp4" {
				continue
			}
			count++
			item := asset
			checked := false
			for _, id := range a.assetIDs {
				if id == item.ID {
					checked = true
					break
				}
			}
			if a.kit.Checkbox(c, &checked, item.Name).Changed() {
				if checked {
					a.assetIDs = append(a.assetIDs, item.ID)
				} else {
					for i, id := range a.assetIDs {
						if id == item.ID {
							a.assetIDs = append(a.assetIDs[:i], a.assetIDs[i+1:]...)
							break
						}
					}
				}
				a.redraw()
			}
		}
		if count == 0 {
			a.kit.Hint(c, "当前工作空间还没有图片或 MP4 素材。")
		}
	})

	a.partTitle(c, partLayout)
	{
		choices := []choice{}
		for _, name := range a.templates() {
			choices = append(choices, choice{name, name})
		}
		if len(choices) == 0 {
			a.kit.Hint(c, "没有可用模板")
		} else {
			a.pick(c, "版式", "画面模板", choices, a.expTemplate, func(id string) { a.expTemplate = id })
		}
	}

	a.partTitle(c, partFormat)
	a.pick(c, "画幅", "", []choice{
		{"9:16", "竖屏 9:16"},
		{"16:9", "横屏 16:9"},
	}, a.ratio, func(id string) { a.ratio = id })
	a.kit.Field(c, "每镜时长（秒）", "图片合成时每张停留多久", func() {
		a.kit.Input(c, &a.duration, "3")
	})

	a.partTitle(c, partOutput)
	a.kit.Hint(c, "合成完成后可以在右边「视频」列表里播放、保存到本地。")
}

// templates 返回可用的画面模板。
func (a *App) templates() []string {
	if a.state == nil {
		return nil
	}
	return a.state.Templates
}

func (a *App) submitExport() {
	if a.projectID == "" || len(a.assetIDs) == 0 {
		a.fail("请选择要合成的素材")
		return
	}
	var kind string
	for _, id := range a.assetIDs {
		found := false
		for _, asset := range a.ProjectAssets() {
			if asset.ID != id {
				continue
			}
			found = true
			current := "video"
			if strings.HasPrefix(asset.Mime, "image/") {
				current = "image"
			} else if asset.Mime != "video/mp4" {
				a.fail("请选择图片或 MP4 视频")
				return
			}
			if kind != "" && kind != current {
				a.fail("一次只能选择图片或 MP4 视频，请不要混选")
				return
			}
			kind = current
			break
		}
		if !found {
			a.fail("所选素材不属于当前项目，请重新选择")
			return
		}
	}
	duration, err := parseNumberIn("每镜时长", a.duration, 1, 60)
	if err != nil {
		a.fail(err.Error())
		return
	}
	input := map[string]any{
		"projectId": a.projectID,
		"kind":      "export",
		"assetIds":  append([]string(nil), a.assetIDs...),
		"ratio":     a.ratio,
		"title":     a.expTitle,
		"captions":  a.expCaptions,
		"duration":  duration,
		"template":  a.expTemplate,
	}
	if a.expAudioID != "" {
		input["audioId"] = a.expAudioID
	}
	a.busy.Store(true)
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		job, err := client.SubmitJob(input)
		a.busy.Store(false)
		a.mutate(func() {
			if err != nil {
				a.notice = "提交合成失败：" + err.Error()
				a.noticeErr = true
			} else {
				a.notice = fmt.Sprintf("已开始合成（%s）", job.ID)
				a.noticeErr = false
			}
		})
		a.refresh()
	}()
}
