package app

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// partTitle 画一个「部分」标题。
//
// 一条视频由七块组成，表单就按这七块分节 —— 用户找配置时，
// 心里想的也是「我要改的是画面还是声音」，不是「我要找哪个字段」。
func (a *App) partTitle(c *ui.Context, name string) {
	a.kit.PartTitle(c, name)
}

// viewTalkComposer 画知识口播的生成配置：脚本 → 声音 → 画面 → 版式 → 画幅 → 配乐 → 输出。
func (a *App) viewTalkComposer(c *ui.Context) {
	a.partTitle(c, partScript)
	a.kit.Field(c, "标题", "", func() {
		a.kit.Input(c, &a.talkTitle, "这条视频的标题")
	})
	a.kit.Field(c, "口播脚本", "一行一句，一行一镜；最多 8 镜，每行不超过 80 字", func() {
		a.kit.TextArea(c, &a.talkCaptions, "第一句说什么？\n第二句说什么？", 120)
	})
	a.kit.Field(c, "副标题", "可选，显示在标题下方", func() {
		a.kit.Input(c, &a.talkSubtitle, "可选")
	})

	a.partTitle(c, partVoice)
	if a.providers != nil && len(a.providers.Voice.Personas) > 0 {
		choices := make([]choice, 0, len(a.providers.Voice.Personas))
		for _, p := range a.providers.Voice.Personas {
			choices = append(choices, choice{p.Key, p.Name})
		}
		a.pick(c, "音色", "本机 VoxFlow 音色，不产生模型费用", choices, a.talkPersona, func(id string) {
			a.talkPersona = id
		})
	} else {
		a.kit.Field(c, "音色", "本机 VoxFlow 没连上，暂时没有可用音色", func() {
			a.kit.Hint(c, "暂无可用音色")
		})
		a.voxflowHint(c)
	}
	a.kit.Field(c, "语气", "留空用音色默认；填了会传给配音引擎", func() {
		a.kit.Input(c, &a.talkTone, "例如：亲切、专业")
	})
	a.kit.Field(c, "镜间停顿", "每句之后的停顿秒数（0–2）", func() {
		a.kit.Input(c, &a.talkPause, "0.45")
	})

	a.partTitle(c, partVisual)
	a.viewTalkImages(c)

	a.partTitle(c, partLayout)
	if len(a.talkStyles) > 0 {
		choices := make([]choice, 0, len(a.talkStyles))
		for _, s := range a.talkStyles {
			label := s.Name
			if label == "" {
				label = s.Key
			}
			choices = append(choices, choice{s.Key, label})
		}
		a.pick(c, "视觉风格", "版式与配色跟着风格走", choices, a.talkStyle, func(id string) {
			a.talkStyle = id
		})
	}

	a.partTitle(c, partFormat)
	a.kit.Field(c, "画幅", "", func() {
		a.kit.Hint(c, "固定竖屏 1080×1920（9:16）")
	})

	a.partTitle(c, partMusic)
	a.viewBGMPickerFor(c, &a.talkBGM)
	a.kit.Field(c, "BGM 音量（%）", "0–100，数字越大越响", func() {
		a.kit.Input(c, &a.talkBGMGain, "12")
	})

	a.partTitle(c, partOutput)
	a.kit.Hint(c, "生成完成后，在右边「视频」列表里可以播放、保存到本地，也能按同一份配置再生成一条。")
}

// viewRecommendationComposer 画项目推荐的生成配置。
func (a *App) viewRecommendationComposer(c *ui.Context) {
	a.partTitle(c, partScript)
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

	a.partTitle(c, partVoice)
	a.viewChannelPicker(c)
	a.viewModelPicker(c)
	a.pick(c, "旁白来源", "逐镜指定 = 每句配一条配音；整段音频切分 = 一条音频按句切开", []choice{
		{"scenes", "逐镜指定"},
		{"tts", "AI 配音"},
		{"audio", "整段音频切分"},
	}, a.recVoiceSource, func(id string) { a.recVoiceSource = id })
	a.viewRecVoiceSource(c)
	a.kit.Field(c, "语气", "留空用音色默认", func() {
		a.kit.Input(c, &a.recTone, "例如：专业、克制")
	})
	a.kit.Field(c, "语速", "0.75–1.5 倍", func() {
		a.kit.Input(c, &a.recSpeed, "1")
	})
	a.kit.Field(c, "镜间停顿", "每句之后的停顿秒数", func() {
		a.kit.Input(c, &a.recPause, "0.35")
	})

	a.partTitle(c, partVisual)
	a.viewStoryboardImages(c)
	a.pick(c, "画面填充", "铺满画布会裁掉边缘，完整画面留边", []choice{
		{"contain", "完整画面"},
		{"cover", "铺满画布"},
	}, a.recImageFit, func(id string) { a.recImageFit = id })

	a.partTitle(c, partLayout)
	a.kit.Field(c, "版式", "", func() {
		a.kit.Hint(c, "项目推荐固定用「项目推荐」版式")
	})

	a.partTitle(c, partFormat)
	a.pick(c, "发布平台", "平台决定画幅与字幕排布", []choice{
		{"channels", "视频号"},
		{"xiaohongshu", "小红书"},
		{"landscape", "横屏视频"},
	}, a.recPlatform, func(id string) {
		a.recPlatform = id
		if id == "landscape" {
			a.recRatio = "16:9"
		} else {
			a.recRatio = "9:16"
		}
	})

	a.partTitle(c, partMusic)
	a.viewBGMPickerFor(c, &a.recBGM)
	a.kit.Field(c, "BGM 音量（%）", "0–50，数字越大越响", func() {
		a.kit.Input(c, &a.recBGMGain, "12")
	})

	a.partTitle(c, partOutput)
	a.kit.Hint(c, "生成完成后，在右边「视频」列表里可以播放、保存到本地，也能按同一份配置再生成一条。")
}

// viewRecVoiceSource 画旁白来源对应的配置。
//
// 三种来源要配的东西不一样，少一样后端就会在跑到一半时炸：
//
//	逐镜指定 → 每一句可以选一条现成配音，空着就用 AI 配音
//	AI 配音   → 用上面选的音色与语气
//	整段切分 → 一条长音频，按每句字数切开
func (a *App) viewRecVoiceSource(c *ui.Context) {
	switch a.recVoiceSource {
	case "scenes":
		a.viewSceneVoices(c)
	case "audio":
		choices := a.audioChoices()
		if len(choices) == 0 {
			a.kit.Field(c, "旁白音频", "工作空间里还没有音频素材", func() {
				a.kit.Hint(c, "先在素材库里导入一条音频")
			})
			return
		}
		a.pick(c, "旁白音频", "按每句的字数把这条音频切开", choices, a.recNarrationID, func(id string) {
			a.recNarrationID = id
		})
	}
}

// viewSceneVoices 画逐镜配音：一句一行，可以各选各的。
func (a *App) viewSceneVoices(c *ui.Context) {
	lines := splitLines(a.recCaptions)
	choices := append([]choice{{"", "AI 配音"}}, a.audioChoices()...)
	if len(lines) == 0 {
		a.kit.Field(c, "逐镜配音", "先写下分镜口播，才能一句句指定配音", func() {
			a.kit.Hint(c, "还没有分镜")
		})
		return
	}
	a.kit.Field(c, "逐镜配音", "不选就是用上面的音色生成；选了就用这条现成的", func() {
		ui.Column(c).FillWidth().Gap(design.SpaceXS).Children(func() {
			for i, line := range lines {
				index := i
				current := ""
				if index < len(a.recVoiceIDs) {
					current = a.recVoiceIDs[index]
				}
				label := "AI 配音"
				options := make([]string, 0, len(choices))
				for _, ch := range choices {
					options = append(options, ch.label)
					if ch.id == current {
						label = ch.label
					}
				}
				ui.Row(c).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
					ui.Text(c, fmt.Sprintf("%d", index+1)).
						Width(16).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
					ui.Text(c, truncateRunes(line, 12)).Grow(1).
						FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
					a.kit.Select(c, &label, options, "").Width(130).Changed()
					for _, ch := range choices {
						if ch.label == label {
							a.setRecVoiceID(index, ch.id)
							break
						}
					}
				})
			}
		})
	})
}

// setRecVoiceID 写入某一句的配音素材，保持与分镜行对齐。
func (a *App) setRecVoiceID(index int, id string) {
	for len(a.recVoiceIDs) <= index {
		a.recVoiceIDs = append(a.recVoiceIDs, "")
	}
	if a.recVoiceIDs[index] == id {
		return
	}
	a.recVoiceIDs[index] = id
}

// recSceneVoiceIDs 返回与分镜行对齐的配音素材列表。
func (a *App) recSceneVoiceIDs() []string {
	lines := splitLines(a.recCaptions)
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, len(lines))
	for i := range lines {
		if i < len(a.recVoiceIDs) {
			out[i] = a.recVoiceIDs[i]
		}
	}
	return out
}

// audioChoices 列出工作空间里的音频素材。
func (a *App) audioChoices() []choice {
	var out []choice
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "audio/") {
			name := asset.Name
			if name == "" {
				name = asset.ID
			}
			out = append(out, choice{asset.ID, name})
		}
	}
	return out
}

// truncateRunes 截断文本，尾巴加省略号。
func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

// viewLibraryComposer 画素材库这一屏：导入与说明。
func (a *App) viewLibraryComposer(c *ui.Context) {
	a.kit.Field(c, "导入本地素材", "图片、视频、音频都可以导进来，作为画面的来源", func() {
		ui.Row(c).Gap(design.SpaceSM).Children(func() {
			if button := kitButton(a.kit, c, kit.ButtonSecondary, "导入本地素材", a.projectID == ""); button.Clicked() {
				projectID := a.projectID
				go a.importMedia(projectID)
			}
		})
	})
	images, videos, audios := 0, 0, 0
	// 用 libraryAssets：素材库里不装成片，成片归各自的流水线。
	// 这里要是按 ProjectAssets 数，左边写「1 段视频」，右边列表却是空的。
	for _, asset := range a.libraryAssets() {
		switch {
		case strings.HasPrefix(asset.Mime, "image/"):
			images++
		case strings.HasPrefix(asset.Mime, "video/"):
			videos++
		case strings.HasPrefix(asset.Mime, "audio/"):
			audios++
		}
	}
	a.kit.Field(c, "素材构成", "", func() {
		ui.Row(c).Gap(design.SpaceSM).Children(func() {
			a.kit.Tag(c, fmt.Sprintf("%d 张图片", images), "")
			a.kit.Tag(c, fmt.Sprintf("%d 段视频", videos), "")
			a.kit.Tag(c, fmt.Sprintf("%d 条音频", audios), "")
		})
	})
	a.kit.Hint(c, "素材本身不是视频。素材要用起来，去「素材合成」把它拼成一条。")
	a.viewVoiceLibrary(c)
}

// viewVoiceLibrary 画声音库：现成的音色，加上「用自己的声音」。
//
// 音色是全局的：这里注册一次，任何流水线里都能选到它。
func (a *App) viewVoiceLibrary(c *ui.Context) {
	a.partTitle(c, "声音库")
	var personas []string
	if a.providers != nil {
		for _, p := range a.providers.Voice.Personas {
			personas = append(personas, p.Name)
		}
	}
	a.kit.Field(c, "可用音色", "本机语音服务提供的音色，配音不产生模型费用", func() {
		if len(personas) == 0 {
			a.kit.Hint(c, "本机语音服务还没连上")
			return
		}
		ui.Row(c).FillWidth().Gap(design.SpaceSM).Children(func() {
			for _, name := range personas {
				a.kit.Tag(c, name, "")
			}
		})
	})
	a.voxflowHint(c)
	a.kit.Field(c, "我的声音", "用一段录音注册一个自己的音色，之后任何流水线都能选它", func() {
		if !a.voiceImportOpen {
			ui.Row(c).Gap(design.SpaceSM).Children(func() {
				if button := kitButton(a.kit, c, kit.ButtonSecondary, "导入录音并注册", false); button.Clicked() {
					a.voiceImportOpen = true
					a.redraw()
				}
			})
			return
		}
		ui.Column(c).FillWidth().Gap(design.SpaceSM).Children(func() {
			a.kit.Input(c, &a.voiceName, "音色名称，例如「我的播客声」")
			a.kit.TextArea(c, &a.voiceRefText, "录音原文：你在录音里念的是哪段话", 80)
			ui.Row(c).Gap(design.SpaceSM).Children(func() {
				if button := kitButton(a.kit, c, kit.ButtonPrimary, "选择录音并注册", false); button.Clicked() {
					go a.registerVoice()
				}
				if button := kitButton(a.kit, c, kit.ButtonSecondary, "取消", false); button.Clicked() {
					a.voiceImportOpen = false
					a.redraw()
				}
			})
		})
	})
}

// viewGenerateComposer 画「生成素材」：用 AI 生成图片、视频或配音。
//
// 素材类别不再占左栏一行的位置 —— 它们是工具里的一个选择，不是视频类型。
func (a *App) viewGenerateComposer(c *ui.Context) {
	a.pick(c, "生成什么", "", []choice{
		{"video", "视频素材"},
		{"image", "图片素材"},
		{"speech", "配音素材"},
	}, a.libKind, func(id string) {
		a.libKind = id
		a.model = ""
		a.voiceID = ""
	})
	a.kit.Field(c, "描述", "要生成什么，写清楚", func() {
		a.kit.Input(c, &a.prompt, "例如：一只猫在窗边打哈欠")
	})
	a.viewChannelPicker(c)
	a.viewModelPicker(c)
	if a.libKind == "video" {
		a.pick(c, "画幅", "", []choice{
			{"9:16", "竖屏 9:16"},
			{"16:9", "横屏 16:9"},
			{"1:1", "方形 1:1"},
		}, a.ratio, func(id string) { a.ratio = id })
		a.kit.Field(c, "时长（秒）", "", func() {
			a.kit.Input(c, &a.duration, "5")
		})
	}
	a.kit.Hint(c, "生成的素材会进素材库；想变成成片，去「素材合成」或某条流水线。")
}

// viewTalkImages 画口播画面：勾哪几张，就按列表顺序入片。
//
// 不勾也能出片：后端会用风格色生成占位图，先把片子做出来，
// 之后再换真图重做。总比让新手卡在「先去找图」这一步强。
func (a *App) viewTalkImages(c *ui.Context) {
	var images []studioapi.Asset
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "image/") {
			images = append(images, asset)
		}
	}
	if len(images) == 0 {
		a.kit.Field(c, "画面", "工作空间里还没有图片；不勾也能出片，会先用风格色占位", func() {
			a.kit.Hint(c, "暂无画面素材")
		})
		return
	}
	a.kit.Field(c, "画面", fmt.Sprintf("已选 %d 张，按列表顺序入片；不选则用占位图", len(a.talkImages)), func() {
		ui.Column(c).FillWidth().Gap(design.SpaceXS).Children(func() {
			for _, image := range images {
				item := image
				checked := a.talkSelected[item.ID]
				row := ui.Row(c).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
					box := a.kit.Checkbox(c, &checked, item.Name)
					if box.Changed() {
						a.talkSelected[item.ID] = checked
						a.syncTalkImages()
					}
					a.kit.Tag(c, fmt.Sprintf("%d×%d", item.Width, item.Height), "")
				})
				_ = row
			}
		})
	})
}
