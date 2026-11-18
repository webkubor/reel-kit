package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// 工作台就绪状态：给首次进入的引导页用。
//
// 用户第一次打开 app 时最该知道的是「我现在能做什么、还差什么」：
//   · MUSE AV 中台登录了没有（角色、场景、出图出视频都靠它）
//   · 直连模型服务配了没有（分镜、文案靠它）
//   · 本机有没有项目
//
// 一次读完，界面不用来回问。

// SetupCheck 一项准备检查。
type SetupCheck struct {
	// Key 检查项标识。
	Key string `json:"key"`
	// Label 界面显示名。
	Label string `json:"label"`
	// Ready 是否已就绪。
	Ready bool `json:"ready"`
	// Detail 当前状态说明。
	Detail string `json:"detail"`
	// Action 缺什么时该怎么补（界面给按钮）。
	Action string `json:"action,omitempty"`
	// ActionURL 补充说明的链接（可空）。
	ActionURL string `json:"actionUrl,omitempty"`
}

// SetupState 整个工作台的准备情况。
type SetupState struct {
	// Provider 当前 AI 提供方。
	Provider string `json:"provider"`
	// Checks 逐项检查。
	Checks []SetupCheck `json:"checks"`
	// ProjectCount 本机已有项目数。
	ProjectCount int `json:"projectCount"`
	// Ready 是否可以直接开始工作。
	Ready bool `json:"ready"`
	// Summary 一句话总结。
	Summary string `json:"summary"`
}

// GetSetupState 汇总首次进入需要知道的状态。
func (e *Engine) GetSetupState() SetupState {
	state := SetupState{Provider: e.provider()}
	// 中台登录态
	museav := SetupCheck{Key: "museav", Label: "MUSE AV 中台"}
	if err := e.museavReady(); err != nil {
		museav.Detail = err.Error()
		museav.Action = "在终端执行 museav login 完成登录"
	} else {
		museav.Ready = true
		museav.Detail = "已登录；角色、场景、出图出视频都在中台"
	}
	state.Checks = append(state.Checks, museav)
	// 直连（当前选它时才需要）
	if state.Provider == ProviderDirect {
		direct := SetupCheck{Key: "direct", Label: "直连模型服务"}
		c, err := e.GetAISettings()
		switch {
		case err != nil:
			direct.Detail = err.Error()
		case c.BaseURL == "":
			direct.Detail = "还没填服务地址与模型"
			direct.Action = "在「AI 设置」里填地址、模型 ID 和 API Key"
		case !c.HasKey:
			direct.Detail = "地址已填，缺 API Key"
			direct.Action = "在「AI 设置」里补 API Key"
		default:
			direct.Ready = true
			direct.Detail = "已配置：" + c.Model
		}
		state.Checks = append(state.Checks, direct)
	}
	if e.store != nil {
		state.ProjectCount = len(e.store.State().Projects)
	}
	ready := true
	for _, c := range state.Checks {
		if !c.Ready {
			ready = false
		}
	}
	state.Ready = ready
	switch {
	case state.Ready && state.ProjectCount == 0:
		state.Summary = "可以开始了：先新建一个项目"
	case state.Ready:
		state.Summary = "就绪，选择或新建一个项目"
	default:
		state.Summary = "还差一步就能开工"
	}
	return state
}

// museavLoginHint 生成登录命令，供界面复制。
func museavLoginHint() string {
	bin := museavBin()
	if strings.Contains(bin, " ") {
		bin = "\"" + bin + "\""
	}
	return bin + " login"
}

// OpenMuseavLogin 在系统终端里跑 museav login（设备授权流程）。
//
// 登录是本机动作，不需要用户把密钥交给应用；这里只是把命令送进终端，
// 用户自己在浏览器里完成授权。
func (e *Engine) OpenMuseavLogin() error {
	bin := museavBin()
	cmd := exec.Command(bin, "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法启动登录命令：%w（可手动在终端执行 %s）", err, museavLoginHint())
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
