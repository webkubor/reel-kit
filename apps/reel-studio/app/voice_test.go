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
