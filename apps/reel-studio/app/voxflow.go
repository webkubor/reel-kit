package app

// 本机语音服务（VoxFlow）没就绪时，桌面上没有网页可点，
// 用户最容易卡在这一步：音色列表是空的、配音按钮按不动，
// 但没人告诉他该做什么。所以把「怎么装、怎么启动」直接给出来，
// 并且允许把命令复制走。

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// VoxFlow 的地址与安装、启动命令。
//
// 命令照抄 VoxFlow 仓库 README 的快速开始：install.sh 装依赖与模型，
// voice web 起服务，服务默认就在 127.0.0.1:8866 —— 与后端探测的地址一致。
const (
	voxflowRepo    = "https://github.com/webkubor/voxflow"
	voxflowInstall = "git clone https://github.com/webkubor/voxflow.git && cd voxflow && chmod +x install.sh && ./install.sh --yes"
	voxflowStart   = "cd voxflow && source .venv/bin/activate && voice web"
)

// voxflowRunsHere 这台机器上 VoxFlow 跑不跑得起来。
//
// VoxFlow 现在的语音后端是 Apple 芯片上的本地 MLX 模型：只有 Apple 芯片的 Mac
// 装得上、跑得动。别的机器（Windows、Linux、Intel Mac）给安装命令就是把人往坑里引
// —— 那台机器上它装不出来。
func voxflowRunsHere() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

// localVoiceChannel 返回本机语音通道；没有这个通道时返回 nil。
func (a *App) localVoiceChannel() *studioapi.Channel {
	if a.providers == nil {
		return nil
	}
	for _, c := range a.providers.Voice.Channels {
		if c.ID == "voxflow" {
			cc := c
			return &cc
		}
	}
	return nil
}

// voxflowHintText 这台机器上 VoxFlow 跑不跑得起来，决定提示里摆什么。
//
// 拆成纯函数是有原因的：「跑不起来的机器」那条文案只在 Linux/Windows 上出现，
// 要是把判断埋在渲染里，本地（Mac）永远测不到它 —— 错了也要等 CI 才报。
//
// 返回值：文案，以及「要不要摆安装与启动命令」。
func voxflowHintText(supported bool) (string, bool) {
	if !supported {
		// VoxFlow 现在的语音后端是 Apple 芯片上的本地 MLX 模型，
		// 这台机器跑不起来 —— 再给安装命令就是把人往坑里引。
		return "配音用随应用自带的本机离线引擎就行，任何机器都能用。\n" +
			"想换个更好的音色：在「设置与反馈 → 云端配音密钥」填一个 MiniMax 密钥，走云端按量计费。\n" +
			"想克隆自己的声音：VoxFlow 现在的语音后端只跑在 Apple 芯片的 Mac 上，这台机器用不了。\n" +
			"它后续会接别的语音后端（" + voxflowRepo + " 看进展）；在那之前，这台机器上没有本地克隆。", false
	}
	return "配音用随应用自带的本机离线引擎就行，装完就能用。\n" +
		"只有你要克隆自己的声音或设计音色时才需要 VoxFlow（Apple 芯片的 Mac，要装 Python 和模型，5.8 GB）：\n" + voxflowInstall +
		"\n\n启动它（装完每次开机起一次）：\n" + voxflowStart +
		"\n\n服务默认就在 127.0.0.1:8866，起来后回这个窗口重试。", true
}

// voxflowHint 在本机语音服务没就绪时画一段能照着做的提示。
//
// 现在出片不再依赖 VoxFlow：随应用自带的本机离线引擎已经能配音。
// VoxFlow 只剩两件事是它能干 —— 克隆自己的声音、设计新音色 ——
// 所以这段提示讲的是「你可能不需要它」，而不是「没有它你用不了」。
//
// 服务就绪时不画任何东西 —— 正常用户不该看见这段。
func (a *App) voxflowHint(c *ui.Context) {
	ch := a.localVoiceChannel()
	if ch == nil || ch.Ready {
		return
	}
	a.kit.Notice(c, "VoxFlow 没连上：这只影响「克隆自己的声音」和「设计音色」，出片不受影响。", "")
	text, showCommands := voxflowHintText(voxflowRunsHere())
	a.kit.Hint(c, text)
	if !showCommands {
		return
	}
	ui.Row(c).Gap(design.SpaceSM).Children(func() {
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "复制安装命令", false); button.Clicked() {
			_ = mygo.Clipboard.Write(transfer.TextData(voxflowInstall))
			a.kit.Notice(c, "安装命令已复制到剪贴板", "")
		}
		if button := kitButton(a.kit, c, kit.ButtonSecondary, "复制启动命令", false); button.Clicked() {
			_ = mygo.Clipboard.Write(transfer.TextData(voxflowStart))
			a.kit.Notice(c, "启动命令已复制到剪贴板", "")
		}
	})
}
