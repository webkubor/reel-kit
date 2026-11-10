package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func Test脚本字数与镜数即时分析(t *testing.T) {
	// 1. 空文本
	s0 := analyzeCaptions("")
	if s0.lines != 0 || s0.isReady {
		t.Errorf("空文本分析不正确: %+v", s0)
	}

	// 2. 正常 3 句
	textNormal := "第一句短片说明\n第二句功能展开介绍\n第三句行动号召收尾"
	sNormal := analyzeCaptions(textNormal)
	if sNormal.lines != 3 || !sNormal.isReady || sNormal.overCount || sNormal.overLine != 0 {
		t.Errorf("正常 3 句分析不正确: %+v", sNormal)
	}
	if sNormal.maxChars != len([]rune("第二句功能展开介绍")) {
		t.Errorf("最大字数不正确: got %d", sNormal.maxChars)
	}

	// 3. 超出 8 镜上限
	text9 := "1\n2\n3\n4\n5\n6\n7\n8\n9"
	s9 := analyzeCaptions(text9)
	if s9.lines != 9 || !s9.overCount || s9.isReady {
		t.Errorf("9 镜应报超限: %+v", s9)
	}

	// 4. 单句超 80 字
	longSentence := strings.Repeat("字", 85)
	textOverLen := "第一句正常\n" + longSentence + "\n第三句也正常"
	sOver := analyzeCaptions(textOverLen)
	if sOver.overLine != 2 || sOver.overLen != 85 || sOver.isReady {
		t.Errorf("第 2 句超 80 字未捕获: %+v", sOver)
	}
}

func Test口播表单实时显示字数反馈(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	a.talkCaptions = "第一句：讲明白问题\n第二句：给出方案"

	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	// 应该显示合规状态
	if !tt.HasText("✓ 当前 2 镜") {
		t.Errorf("未能即时渲染 2 镜合规提示，实际文字：%v", tt.Texts())
	}

	// 改为超 8 镜
	a.talkCaptions = "1\n2\n3\n4\n5\n6\n7\n8\n9"
	tt.Frame()
	if !tt.HasText("超出 8 镜上限") {
		t.Errorf("未即时提示超出 8 镜上限，实际文字：%v", tt.Texts())
	}

	// 改为单句超 80 字
	a.talkCaptions = "第一句\n" + strings.Repeat("超", 82)
	tt.Frame()
	if !tt.HasText("超出 80 字上限") {
		t.Errorf("未即时提示单句超出 80 字，实际文字：%v", tt.Texts())
	}
}

func Test非Apple芯片平台说明远端也能克隆(t *testing.T) {
	hint, canInstall := voxflowHintText(false)
	if canInstall {
		t.Errorf("非 Apple 芯片 Mac 不该给出安装命令")
	}
	// 本机装不了 ≠ 用不了：远端语音服务这条路任何系统都能走。
	if !strings.Contains(hint, "Apple 芯片的 Mac 装得起来") {
		t.Errorf("缺少「本机装不了」的明确说明: %s", hint)
	}
	if !strings.Contains(hint, "REEL_VOXFLOW_URL") {
		t.Errorf("缺少远端语音服务这条路的说明: %s", hint)
	}
}

func Test成片详情卡片包含保存到本地按钮(t *testing.T) {
	server := stub(t, []map[string]any{{
		"id":      "j_done",
		"status":  "succeeded",
		"assetId": "a1",
		"input": map[string]any{
			"kind":  "talk",
			"title": "测试口播成片",
		},
	}})
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	// 选中第一条成片后，必须展示「保存到本地」
	if !tt.HasText("保存到本地") {
		t.Errorf("成片列表或详情区未找到「保存到本地」按钮，文本: %v", tt.Texts())
	}
}
