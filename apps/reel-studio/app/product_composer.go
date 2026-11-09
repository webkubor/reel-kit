package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// viewProductComposer 画人物商品介绍的生成配置。
//
// 与知识口播的差别在「谁先被看见」：
//   口播是文字承担叙事，画面垫底；
//   商品介绍是画面先说话，文字只补一句。
// 所以这里的顺序是「名称 → 介绍词 → 画面 → 声音 → 画幅 → 配乐」，
// 画幅决定版式（竖版出镜 / 横屏左图右文），用户不用自己挑版式。
func (a *App) viewProductComposer(c *ui.Context) {
	a.partTitle(c, partScript)
	a.kit.Field(c, "产品或人物名称", "显示在片头，1–60 字", func() {
		a.kit.Input(c, &a.prodTitle, "例如：Reel Studio 本地视频工作台")
	})
	a.kit.Field(c, "介绍词", "一行一句，一行一镜；最多 8 镜，每行不超过 80 字", func() {
		a.kit.TextArea(c, &a.prodCaptions, "第一句说什么？\n第二句说什么？", 120)
	})
	a.kit.Field(c, "一句话卖点", "可选，显示在名称旁边", func() {
		a.kit.Input(c, &a.prodSubtitle, "例如：素材不出本机")
	})
	a.kit.Field(c, "片尾一句", "可选，比如价格、行动号召", func() {
		a.kit.Input(c, &a.prodFooter, "例如：现在就去试试")
	})

	a.partTitle(c, partVoice)
	if a.providers != nil && len(a.providers.Voice.Personas) > 0 {
		choices := make([]choice, 0, len(a.providers.Voice.Personas))
		for _, p := range a.providers.Voice.Personas {
			choices = append(choices, choice{p.Key, p.Name})
		}
		a.pick(c, "音色", "本机 VoxFlow 音色，不产生模型费用", choices, a.prodPersona, func(id string) {
			a.prodPersona = id
		})
	} else {
		a.kit.Field(c, "音色", "本机 VoxFlow 还没连上；先在设置里确认连接状态", func() {
			a.kit.Hint(c, "暂无可用音色")
		})
	}
	a.kit.Field(c, "语气", "留空用音色默认；填了会传给配音引擎", func() {
		a.kit.Input(c, &a.prodTone, "例如：亲切、专业")
	})
	a.kit.Field(c, "镜间停顿", "每句之后的停顿秒数（0–2）", func() {
		a.kit.Input(c, &a.prodPause, "0.45")
	})

	a.partTitle(c, partVisual)
	a.viewProductImages(c)

	a.partTitle(c, partLayout)
	a.kit.Field(c, "版式", "", func() {
		if a.prodRatio == "16:9" {
			a.kit.Hint(c, "横屏 16:9：左图右文，适合官网与视频号横版")
		} else {
			a.kit.Hint(c, "竖屏 9:16：出镜画面满屏 + 底部字幕条，适合短视频")
		}
	})

	a.partTitle(c, partFormat)
	a.pick(c, "画幅", "画幅决定版式，不用另外挑", []choice{
		{"9:16", "竖屏 9:16"},
		{"16:9", "横屏 16:9"},
	}, a.prodRatio, func(id string) { a.prodRatio = id })

	a.partTitle(c, partMusic)
	a.viewBGMPickerFor(c, &a.prodBGM)
	a.kit.Field(c, "BGM 音量（%）", "0–100，数字越大越响", func() {
		a.kit.Input(c, &a.prodBGMGain, "12")
	})

	a.partTitle(c, partOutput)
	a.kit.Hint(c, "生成完成后，在右边「视频」列表里可以播放、保存到本地，也能按同一份配置再生成一条。")
}

// viewProductImages 画产品或人物画面：勾哪几张，就按列表顺序入片。
//
// 不勾也能出片：后端会用深色占位图顶上，先把片子做出来，之后再换真图重做。
func (a *App) viewProductImages(c *ui.Context) {
	var images []studioapi.Asset
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "image/") {
			images = append(images, asset)
		}
	}
	if len(images) == 0 {
		a.kit.Field(c, "画面", "工作空间里还没有图片；不勾也能出片，会先用占位图", func() {
			a.kit.Hint(c, "暂无画面素材")
		})
		return
	}
	a.kit.Field(c, "画面", fmt.Sprintf("已选 %d 张，按列表顺序入片；不选则用占位图", len(a.prodImages)), func() {
		ui.Column(c).FillWidth().Gap(design.SpaceXS).Children(func() {
			for _, image := range images {
				item := image
				checked := a.prodSelected[item.ID]
				row := ui.Row(c).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
					box := a.kit.Checkbox(c, &checked, item.Name)
					if box.Changed() {
						a.prodSelected[item.ID] = checked
						a.syncProductImages()
					}
					a.kit.Tag(c, fmt.Sprintf("%d×%d", item.Width, item.Height), "")
				})
				_ = row
			}
		})
	})
}

// syncProductImages 按勾选状态重排产品画面，保持素材列表的先后顺序。
func (a *App) syncProductImages() {
	var picked []string
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "image/") && a.prodSelected[asset.ID] {
			picked = append(picked, asset.ID)
		}
	}
	a.prodImages = picked
}
