package main

import "testing"

// 回归：PersonaSource 是 *string，曾被 strOr(any) 断言成 string 失败，
// 导致所有角色都报「缺形象来源」、永远判不了完成。
func TestSubjectPersonaIsRead(t *testing.T) {
	v := "virtual"
	c := subjectToCharacter(SubjectDetail{Subject: Subject{ID: "x", Name: "n", Kind: "person", PersonaSource: &v}}, "", "")
	if c.Persona != "virtual" {
		t.Fatalf("persona 应为 virtual，实际 %q", c.Persona)
	}
	for _, m := range c.Missing {
		if m == "缺形象来源（真人 / 虚拟人）" {
			t.Fatal("已有形象来源却仍报缺")
		}
	}
}
