package app

// 模型通道：用户自己填的中转站或厂商直连。
//
// 为什么要让用户自己填：市面上的中转站、厂商接口地址各家不同，
// 我们替用户猜地址就是替用户选别人家的服务、别人的账单。
// 填一次之后，这条通道跟工作台自带的通道一样可选。
//
// 密钥不进配置文件里的通道条目，也不进日志：
// 通道条目只记「密钥放在哪个环境变量名」，
// 密钥本身存在本机配置里（桌面版设置页填的）或环境变量里。

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// 生图与出视频各支持哪些协议。
var (
	channelImageProtocols = []choice{
		{"openai", "openai · 一次 POST 拿结果（OpenAI 兼容的生图接口）"},
		{"minimax-image", "minimax-image · 厂商直连（aspect_ratio，不是 size）"},
		{"museav", "museav · 中台（先提交任务，再轮询）"},
	}
	channelVideoProtocols = []choice{
		{"ark-task", "ark-task · 提交任务拿 ID，再轮询出结果"},
		{"museav", "museav · 中台（先提交任务，再轮询）"},
	}
)

// customChannels 返回工作台报告的自定义通道。
func (a *App) customChannels() []studioapi.CustomChannel {
	if a.providers == nil {
		return nil
	}
	return a.providers.CustomChannels.Channels
}

// viewChannels 画「模型通道」这一段。
func (a *App) viewChannels(c *ui.Context) {
	a.kit.SectionTitle(c, "模型通道", "自己买的中转站或厂商直连：填一次，之后当第三条通道用", nil)
	// 配置问题的明细放在这儿，不放左栏：左栏窄，那几行小字挤成一堵墙，
	// 读不下去。这里有整屏的空间，还能一边看一边改。
	if a.providers != nil {
		if problems := a.providers.CustomChannels.Problems; len(problems) > 0 {
			a.kit.Card(c, func() {
				a.kit.Strong(c, "这几条通道还差东西")
				for _, p := range problems {
					ui.Text(c, "· "+p).FontSize(design.FontSmall).TextColor(a.kit.T.Accent).TextAlign(ui.Start)
				}
			})
		}
	}
	channels := a.customChannels()
	if len(channels) == 0 {
		a.kit.Empty(c, "还没有自定义通道", "在下面填一条：地址、模型名、密钥", nil)
	}
	for _, ch := range channels {
		ch := ch
		a.kit.Card(c, func() {
			ui.Row(c).Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
				ui.Text(c, ch.Label).FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
				ui.Text(c, ch.Protocol).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
			})
			a.kit.Hint(c, fmt.Sprintf("%s · 模型 %s · 密钥变量 %s", ch.BaseURL, orDash(ch.Model), ch.Env))
			a.kit.Hint(c, a.channelKeyStatus(ch))
			if !ch.Ready {
				a.kit.Notice(c, ch.Note, "warn")
			}
			if report, ok := a.chanChecks[ch.ID]; ok && report != "" {
				a.kit.Hint(c, report)
			}
			ui.Row(c).Gap(design.SpaceSM).Children(func() {
				if button := a.kit.IconAction(c, kit.ButtonSecondary, kit.Icon("check"), "检查", false); button.Clicked() {
					a.checkChannel(ch.ID)
				}
				if button := a.kit.IconAction(c, kit.ButtonQuiet, kit.Icon("edit"), "编辑", false); button.Clicked() {
					a.editChannel(ch)
				}
				if button := a.kit.IconAction(c, kit.ButtonDanger, kit.Icon("trash"), "删掉", false); button.Clicked() {
					a.deleteChannel(ch.ID)
				}
			})
		})
	}
	a.viewChannelForm(c)
}

// channelKeyStatus 说这条通道的密钥现在在哪儿。
//
// 不回明文，只回末四位：这块界面会被截图、被转发。
func (a *App) channelKeyStatus(ch studioapi.CustomChannel) string {
	if ch.KeySource == "env" {
		return fmt.Sprintf("密钥来自环境变量 %s", ch.Env)
	}
	if ok, tail := channelKeyConfigured(ch.ID); ok {
		return fmt.Sprintf("密钥已存在设置页（%s）", tail)
	}
	return fmt.Sprintf("还没填密钥：环境变量 %s 或设置页里的密钥都可以", ch.Env)
}

// editChannel 把一条已有通道填进表单。
func (a *App) editChannel(ch studioapi.CustomChannel) {
	a.mutate(func() {
		a.chanEditing = ch.ID
		a.chanID = ch.ID
		a.chanLabel = ch.Label
		a.chanKind = ch.Kind
		a.chanProtocol = ch.Protocol
		a.chanBaseURL = ch.BaseURL
		a.chanModel = ch.Model
		a.chanEnv = ch.Env
		a.chanKey = ""
	})
}

// viewChannelForm 画新增/修改一条通道的表单。
func (a *App) viewChannelForm(c *ui.Context) {
	if a.chanKind == "" {
		a.chanKind = "image"
	}
	if a.chanProtocol == "" {
		a.chanProtocol = "openai"
	}
	editing := a.chanEditing != ""
	title := "加一条模型通道"
	subtitle := "厂商直连或中转：填地址、模型、密钥，存一次以后直接用"
	if editing {
		title = "改这条模型通道"
		subtitle = "改完点保存；密钥留空表示不改"
	}
	a.kit.SectionTitle(c, title, subtitle, nil)
	a.kit.Field(c, "通道 id", "英文小写标识，提交任务时用它指定通道", func() {
		a.kit.Input(c, &a.chanID, "minimax-image")
	})
	a.kit.Field(c, "显示名称", "给你自己看的", func() {
		a.kit.Input(c, &a.chanLabel, "MiniMax 生图（厂商直连）")
	})
	a.kit.Field(c, "类型", "这条通道出图还是出视频", func() {
		a.pick(c, "类型", "", []choice{{"image", "生图"}, {"video", "出视频"}}, a.chanKind, func(id string) {
			a.chanKind = id
			// 协议随类型换：留着上一次的协议会配出对不上的组合。
			if id == "video" {
				a.chanProtocol = "ark-task"
			} else if a.chanProtocol == "ark-task" {
				a.chanProtocol = "openai"
			}
		})
	})
	protocols := channelImageProtocols
	if a.chanKind == "video" {
		protocols = channelVideoProtocols
	}
	a.pick(c, "协议", "接口的形状", protocols, a.chanProtocol, func(id string) { a.chanProtocol = id })
	a.kit.Field(c, "接口地址", "厂商或中台的根地址（不含具体路径）", func() {
		a.kit.Input(c, &a.chanBaseURL, "https://api.minimaxi.com")
	})
	a.kit.Field(c, "模型名", "中台可以不填；直连厂商必须填", func() {
		a.kit.Input(c, &a.chanModel, "image-01")
	})
	a.kit.Field(c, "密钥变量名", "只写变量名，密钥本身不进这里", func() {
		a.kit.Input(c, &a.chanEnv, "MINIMAX_API_KEY")
	})
	keyHelp := "直接填在这里就行，存进本机配置（0600），界面只显示末四位"
	if editing {
		keyHelp = "留空表示不改；环境变量里已有同名变量时以环境变量为准"
	}
	a.kit.Field(c, "密钥", keyHelp, func() {
		a.kit.InputSecret(c, &a.chanKey, "sk-…")
	})
	ui.Row(c).Gap(design.SpaceSM).Children(func() {
		if button := kitButton(a.kit, c, kit.ButtonPrimary, "保存通道", a.chanID == "" || a.chanBaseURL == "" || a.chanEnv == ""); button.Clicked() {
			a.saveChannel()
		}
	})
	a.kit.Hint(c, "「检查这条通道」只发只读或必然被拒的请求，不会生成内容、不产生费用；它会分开告诉你地址通不通、密钥认不认、模型有没有被验过。")
}

// saveChannel 保存一条自定义通道。
//
// 密钥存本机配置，通道配置存工作台的通道文件 —— 两处各管各的。
func (a *App) saveChannel() {
	id := strings.TrimSpace(a.chanID)
	baseURL := strings.TrimSpace(a.chanBaseURL)
	env := strings.TrimSpace(a.chanEnv)
	key := strings.TrimSpace(a.chanKey)
	if id == "" || baseURL == "" || env == "" {
		a.fail("通道 id、接口地址、密钥变量名都要填")
		return
	}
	entry := map[string]any{
		"id": id, "label": strings.TrimSpace(a.chanLabel), "kind": a.chanKind,
		"protocol": a.chanProtocol, "baseUrl": baseURL,
		"model": strings.TrimSpace(a.chanModel), "env": env,
	}
	go func() {
		if key != "" {
			if err := saveChannelKey(id, key); err != nil {
				a.fail("保存密钥失败：" + err.Error())
				return
			}
		}
		client := studioapi.New(a.opts.BaseURL)
		if _, err := client.SaveChannel(entry); err != nil {
			a.fail("保存通道失败：" + err.Error())
			return
		}
		a.mutate(func() {
			a.chanChecks = map[string]string{}
			a.chanID = ""
			a.chanLabel = ""
			a.chanBaseURL = ""
			a.chanModel = ""
			a.chanEnv = ""
			a.chanKey = ""
			a.chanEditing = ""
		})
		a.logf("已保存自定义模型通道 %s", id)
		a.refresh()
		a.mutate(func() {
			a.notice = "通道已保存。点「检查这条通道」确认它能不能用。"
			a.noticeErr = false
		})
	}()
}

// deleteChannel 删掉一条自定义通道。
func (a *App) deleteChannel(id string) {
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		if _, err := client.DeleteChannel(id); err != nil {
			a.fail("删掉通道失败：" + err.Error())
			return
		}
		a.mutate(func() {
			delete(a.chanChecks, id)
		})
		a.refresh()
	}()
}

// checkChannel 探活一条通道：只发只读或必然被拒的请求，看它能不能用。
func (a *App) checkChannel(id string) {
	go func() {
		client := studioapi.New(a.opts.BaseURL)
		result, err := client.CheckChannel(id)
		if err != nil {
			a.fail("检查通道失败：" + err.Error())
			return
		}
		lines := make([]string, 0, len(result.Checks))
		for _, c := range result.Checks {
			// 三态：通过 √、失败 ×、未验证 ？。
			// 「没验证」写成 √ 就是骗人 —— 用户会以为这条通道能用了。
			mark := "？"
			if c.OK != nil {
				if *c.OK {
					mark = "√"
				} else {
					mark = "×"
				}
			}
			lines = append(lines, mark+" "+c.Name+"："+c.Detail)
		}
		report := strings.Join(lines, "\n")
		a.mutate(func() {
			if a.chanChecks == nil {
				a.chanChecks = map[string]string{}
			}
			a.chanChecks[id] = report
			a.notice = "检查完成"
			a.noticeErr = !result.OK
		})
	}()
}
