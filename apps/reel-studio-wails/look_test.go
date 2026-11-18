package main

import "testing"

// 装扮名要从三种命名形态里都取对。
func TestLookOfNamingForms(t *testing.T) {
	cases := []struct {
		char, asset, want string
	}{
		{"夜惊鸿", "夜惊鸿-王服三视图", "王服"},
		{"夜惊鸿", "夜惊鸿-三视图", "默认"},
		{"夜惊鸿", "夜惊鸿-三视图-王服", "王服"},
		{"夜惊鸿", "夜惊鸿-定妆-夜惊鸿-定妆.png", "定妆"},
		{"欧阳狂徒", "欧阳狂徒-定妆-欧阳狂徒.png", "默认"},
		{"欧阳狂徒", "欧阳狂徒-定妆-欧阳狂徒-骑着狼.png", "骑着狼"},
		{"慕北歌", "慕北歌-定妆-慕北歌-poss.png", "poss"},
		{"", "角色-三视图-战甲", "战甲"},
	}
	for _, c := range cases {
		if got := lookOf(c.char, c.asset); got != c.want {
			t.Fatalf("lookOf(%q,%q)=%q，应为 %q", c.char, c.asset, got, c.want)
		}
	}
}

// 角色名重复出现的形态要去重。
func TestLookOfDedupeCharacterPrefix(t *testing.T) {
	if got := lookOf("慕夕歌", "慕夕歌-定妆-慕夕歌_v2.png"); got != "v2" {
		t.Fatalf("got %q", got)
	}
}
