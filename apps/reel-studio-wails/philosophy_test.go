package main

import (
	"context"
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

// 不指定工作区绝不退回第一个：第一个往往是「默认项目」，会套错口径。
func TestPickWorkspacePhilosophyNoFallback(t *testing.T) {
	rows := []workspaceRow{
		{ID: "ws-default", Name: "默认项目", Philosophy: Philosophy{"叙事基调": "温馨"}},
		{ID: "ws-snow", Name: "沸腾之雪", Philosophy: Philosophy{"叙事基调": "举世皆敌"}},
	}
	if _, err := pickWorkspacePhilosophy(rows, ""); err == nil {
		t.Fatal("未指定工作区应报错，不能退回第一个")
	}
	for _, key := range []string{"ws-snow", "沸腾之雪"} {
		got, err := pickWorkspacePhilosophy(rows, key)
		if err != nil || got.Name != "沸腾之雪" {
			t.Fatalf("按 %s 应取到沸腾之雪：%+v %v", key, got, err)
		}
	}
	if _, err := pickWorkspacePhilosophy(rows, "不存在"); err == nil {
		t.Fatal("找不到的工作区应报错")
	}
}

// 出镜提示词不能混进分镜的「只返回 JSON」指令。
func TestCharacterVideoPromptNoStoryboardInstruction(t *testing.T) {
	e := &Engine{}
	p := characterVideoPrompt(&Character{Name: "慕北歌"}, "雪巅枯坐", e.philosophyText(context.Background(), ""))
	if strings.Contains(p, "JSON") || strings.Contains(p, "分镜编辑") {
		t.Fatalf("出镜提示词混进了分镜指令：%s", p)
	}
}

// 单镜头口径：只留画面维度；神兵谱只留总纲和本角色的兵器。
func TestPhilosophyCompileShot(t *testing.T) {
	p := Philosophy{
		"交付红线": []any{"无字幕"},
		"画面哲学": "低调摄影",
		"世界观":  "乾明十七年",
		"配音体系": "说书人视角",
		"神兵谱": map[string]any{
			"总纲": "拒绝玄幻光效",
			"问天": "至尊汉剑",
			"惊鸿": "银白透甲长枪",
		},
	}
	got := p.CompileShot("至尊汉剑【问天】，一体龙骨")
	for _, want := range []string{"无字幕", "低调摄影", "拒绝玄幻光效", "问天：至尊汉剑"} {
		if !strings.Contains(got, want) {
			t.Fatalf("缺 %q：%s", want, got)
		}
	}
	for _, bad := range []string{"乾明十七年", "说书人", "惊鸿", "长枪"} {
		if strings.Contains(got, bad) {
			t.Fatalf("不该带 %q：%s", bad, got)
		}
	}
}
