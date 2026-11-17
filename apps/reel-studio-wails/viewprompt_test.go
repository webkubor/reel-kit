package main

import "strings"
import "testing"

// 回归：「向左转/向右转」实测两个 45° 朝同一边，且没约束抬头。
// 视图方向必须写成画面方向，且锁住头部姿态。
func TestViewPromptDirectionsDiffer(t *testing.T) {
	s := &Subject{Traits: map[string]any{}}
	l := characterViewPrompt(s, CharacterCreateInput{}, ViewLeft45)
	r := characterViewPrompt(s, CharacterCreateInput{}, ViewRight45)
	if l == r {
		t.Fatal("左右 45° 提示词不应相同")
	}
	if !strings.Contains(l, "画面左侧") || !strings.Contains(r, "画面右侧") {
		t.Fatalf("方向应按画面描述：L=%q R=%q", l, r)
	}
	for _, v := range CharacterViews {
		p := characterViewPrompt(s, CharacterCreateInput{}, v)
		if !strings.Contains(p, "不仰头") {
			t.Fatalf("%s 缺头部姿态约束", v)
		}
		if strings.ContainsAny(p, "\r\n") {
			t.Fatalf("%s 提示词含换行，会被批量拆成多条任务", v)
		}
	}
}
