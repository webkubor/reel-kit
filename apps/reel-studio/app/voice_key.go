package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// viewVoiceKeySettings 画「云端配音密钥」这一段。
//
// 什么时候用得上：Windows 机器装不了本机 VoxFlow（它只在 Apple 芯片上跑），
// 想在 Windows 上出片就得让配音走云端 MiniMax TTS —— 那需要用户自己的密钥。
//
// 密钥只落在本机 ~/.reel-kit/config.json（权限 0600），不上传、不进日志；
// 输入框不回显明文，保存后界面上只显示末四位。
func (a *App) viewVoiceKeySettings(c *ui.Context) {
	a.kit.SectionTitle(c, "云端配音密钥", "没有本机语音服务时，用 MiniMax TTS 走云端配音（按量计费）", nil)
	a.kit.Card(c, func() {
		configured, tail := minimaxKeyConfigured()
		if configured {
			ui.Text(c, "已配置 MiniMax 密钥 "+tail).FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
		} else {
			ui.Text(c, "未配置 MiniMax 密钥").FontSize(design.FontSmall).TextColor(a.kit.T.Muted)
		}
		a.kit.Field(c, "MiniMax API Key", "从 MiniMax 开放平台取；只在你自己机器上保存", func() {
			a.kit.InputSecret(c, &a.minimaxKeyInput, "sk-…")
		})
		ui.Row(c).Gap(design.SpaceSM).Children(func() {
			save := kitButton(a.kit, c, kit.ButtonPrimary, "保存密钥", a.minimaxKeyInput == "")
			if save.Clicked() {
				a.saveMinimaxKeyAndRestart(a.minimaxKeyInput)
			}
			clear := kitButton(a.kit, c, kit.ButtonSecondary, "清除密钥", !configured)
			if clear.Clicked() {
				a.saveMinimaxKeyAndRestart("")
			}
		})
		a.kit.Hint(c, "本机 VoxFlow 不需要密钥：装一次就能用，配音不出本机、不花钱。")
	})
}

// saveMinimaxKeyAndRestart 保存密钥并重启本机工作台。
//
// 为什么要重启：工作台在启动时读一次配置，密钥是那时候读走的。
// 不重启的话，界面显示「已配置」，配音却还报没就绪 —— 用户会以为是自己配错了。
func (a *App) saveMinimaxKeyAndRestart(key string) {
	if err := saveMinimaxKey(key); err != nil {
		a.fail("保存密钥失败：" + err.Error())
		return
	}
	a.mutate(func() {
		a.minimaxKeyInput = ""
	})
	a.logf("已更新本机 MiniMax 密钥配置")
	a.restartWorkbench()
}

// restartWorkbench 重启本机工作台，让新的配置生效。
//
// 只重启自己起的那个；工作台是用户另外起的就告诉用户自己重启 ——
// 别人开的进程不归我们杀。
func (a *App) restartWorkbench() {
	if a.backendProc == nil {
		a.mutate(func() {
			a.notice = "密钥已保存。本机工作台不是这个应用起的，请自己重启它以生效"
			a.noticeErr = false
		})
		return
	}
	go func() {
		a.backendProc.Stop()
		status := a.backendProc.Ensure()
		a.mutate(func() {
			a.backendState = status
		})
		a.refresh()
		a.mutate(func() {
			a.notice = "密钥已保存，本机工作台已重启"
			a.noticeErr = false
		})
	}()
}
