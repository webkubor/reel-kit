package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Test声音克隆入口能打开
//
// 「用自己的声音」是录制类内容最刚性的需求，入口必须够得着：
// 素材库这一屏要能看到「导入录音并注册」，点开后要能看到要填的两样东西
// ——音色名称和录音原文。少一样，用户就注册不成。
func Test声音克隆入口能打开(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	a.mode = ModeLibrary
	tt := ui.NewTester(a.View, 1280, 900)
	tt.Frame()

	if !tt.HasText("声音库") {
		t.Errorf("素材库里没有声音库这一段，实际文字：%q", tt.Texts())
	}
	if err := tt.Click("导入录音并注册"); err != nil {
		t.Fatalf("点不到「导入录音并注册」：%v", err)
	}
	tt.Frame()
	// 表单里的两个必填项是输入框的占位提示，不是画出来的文字，
	// 所以要去无障碍树里找 —— 用户看到的也是这两个提示。
	var placeholders []string
	for _, item := range tt.AccessAll() {
		if item.Placeholder != "" {
			placeholders = append(placeholders, item.Placeholder)
		}
	}
	joined := strings.Join(placeholders, "|")
	if !strings.Contains(joined, "音色名称") {
		t.Errorf("没有让填音色名称，实际提示：%q", placeholders)
	}
	if !strings.Contains(joined, "录音原文") {
		t.Errorf("没有让填录音原文，实际提示：%q", placeholders)
	}
	if !tt.HasText("选择录音并注册") {
		t.Errorf("没有提交按钮，实际文字：%q", tt.Texts())
	}
}

// Test声音克隆按工作台能力开，不按本机系统开
//
// 能不能克隆，取决于工作台连不连得上语音服务 ——
// 服务可能在本机（Apple 芯片的 Mac），也可能在另一台机器（REEL_VOXFLOW_URL）。
// 客户端按操作系统一刀切，Windows 用户配了远端服务照样注册不了。
func Test声音克隆按工作台能力开不按本机系统开(t *testing.T) {
	// 1. 工作台连上了语音服务 → 无论本机是什么系统，入口都得在
	server := stubWithVoice(t, nil, true)
	a := newApp(t, server.URL)
	a.mode = ModeLibrary
	tt := ui.NewTester(a.View, 1280, 900)
	tt.Frame()
	if !tt.HasText("导入录音并注册") {
		t.Errorf("工作台已连上语音服务时应有克隆入口，实际文字：%q", tt.Texts())
	}

	// 2. 工作台连不上 → 不给表单，但要说明为什么、以及还有哪些路可走
	server2 := stubWithVoice(t, nil, false)
	b := newApp(t, server2.URL)
	b.mode = ModeLibrary
	tt2 := ui.NewTester(b.View, 1280, 900)
	tt2.Frame()
	if tt2.HasText("导入录音并注册") {
		t.Errorf("工作台连不上语音服务时不该给克隆入口，实际文字：%q", tt2.Texts())
	}
	if !tt2.HasText("克隆自己的声音要工作台连上 VoxFlow 语音服务") {
		t.Errorf("连不上时应说明原因，实际文字：%q", tt2.Texts())
	}
	if !tt2.HasText("REEL_VOXFLOW_URL") {
		t.Errorf("应说明远端语音服务这条路，实际文字：%q", tt2.Texts())
	}
}
