package main

import (
	"strings"
	"testing"
)

// 编译顺序必须是「禁令在前、美学在后」：模型对开头约束服从度最高。
func TestPhilosophyCompileOrder(t *testing.T) {
	p := Philosophy{
		"交付红线": []any{"无字幕", "无水印"},
		"画面哲学": map[string]any{"影调": []any{"低调摄影"}},
		"出片规格": map[string]any{"画幅": "16:9"},
		"叙事基调": "举世皆敌",
	}
	f := p.Compile()
	if len(f.Redlines) != 2 {
		t.Fatalf("红线应为 2 条：%v", f.Redlines)
	}
	text := f.Text
	if !strings.Contains(text, "【交付红线】无字幕；无水印") {
		t.Fatalf("红线未置顶：%s", text)
	}
	if strings.Index(text, "【交付红线】") > strings.Index(text, "【叙事基调】") {
		t.Fatalf("红线应在叙事基调之前：%s", text)
	}
	if !strings.Contains(text, "16:9") {
		t.Fatalf("出片规格未进编译：%s", text)
	}
}

// 嵌套结构要压平成人话，不是 JSON dump。
func TestPhilosophyCompileFlattens(t *testing.T) {
	p := Philosophy{
		"动作美学": map[string]any{
			"轻功规则": "能飞但不悬浮",
			"打击感":  []any{"禁止光球", "物理化内力"},
		},
	}
	text := p.Compile().Text
	for _, want := range []string{"轻功规则：能飞但不悬浮", "禁止光球", "物理化内力"} {
		if !strings.Contains(text, want) {
			t.Fatalf("缺 %q：%s", want, text)
		}
	}
	if strings.ContainsAny(text, "{}\"[]") {
		t.Fatalf("编译结果不该带 JSON 痕迹：%s", text)
	}
}

// 空哲学不该编译出任何字，也不该报错。
func TestPhilosophyCompileEmpty(t *testing.T) {
	if got := (Philosophy{}).Compile(); got.Text != "" || len(got.Redlines) != 0 {
		t.Fatalf("空哲学应编译为空，实际 %q", got.Text)
	}
}

// 未列名的键不能丢：哲学维度是各项目自己长出来的。
func TestPhilosophyCompileKeepsUnknownKeys(t *testing.T) {
	p := Philosophy{"神兵镜头逻辑": "螺旋环绕"}
	text := p.Compile().Text
	if !strings.Contains(text, "【神兵镜头逻辑】螺旋环绕") {
		t.Fatalf("未知维度被丢掉了：%s", text)
	}
}
