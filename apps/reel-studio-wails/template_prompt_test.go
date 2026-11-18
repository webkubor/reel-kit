package main

import (
	"strings"
	"testing"
)

// 模板出的图要能被分类规则认出来，否则生成完还得手工改名。
func TestTemplateAssetNameIsClassifiable(t *testing.T) {
	cases := []struct {
		class string
		key   string
		want  string
	}{
		{"avatar", "portrait", "夜惊鸿-头像"},
		{"costume", "全身立绘", "夜惊鸿-定妆-全身立绘"},
		{"sheet", "正面", "夜惊鸿-三视图-正面"},
		{"weapon", "兵器单品", "夜惊鸿-武器-兵器单品"},
		{"scene", "空间设定", "夜惊鸿-画面-空间设定"},
	}
	for _, c := range cases {
		got := templateAssetName("夜惊鸿", AssetTemplateItem{Key: c.key, Class: c.class})
		if got != c.want {
			t.Fatalf("%s 命名 %s，应为 %s", c.class, got, c.want)
		}
	}
}

// 生成的图要按类别落对位：头像归头像、三视图归三视图、兵器归兵器。
func TestTemplateNamesLandInRightClass(t *testing.T) {
	avatar := templateAssetName("夜惊鸿", AssetTemplateItem{Key: "portrait", Class: "avatar"})
	if !isAvatarAsset(avatar) || isRealFaceAsset(avatar) {
		t.Fatalf("%s 不该被当头像吗", avatar)
	}
	sheet := templateAssetName("夜惊鸿", AssetTemplateItem{Key: "front", Class: "sheet"})
	if !isDesignSheet(sheet) {
		t.Fatalf("%s 应被认作三视图/定妆", sheet)
	}
	weapon := templateAssetName("夜惊鸿", AssetTemplateItem{Key: "weapon", Class: "weapon"})
	if !isWeaponAsset(weapon) {
		t.Fatalf("%s 应被认作兵器", weapon)
	}
	// 兵器名不能被「三视图」误伤
	if isDesignSheet(weapon) {
		t.Fatalf("%s 不该被认作定妆", weapon)
	}
}

// 提示词要带体貌特征与用途，出图才认得出是谁、拿来干什么。
func TestTemplatePromptCarriesTraitsAndPurpose(t *testing.T) {
	traits := traitSummary(map[string]any{
		"发型": "高马尾",
		"肤色": "古铜",
		"体型": "高大",
	})
	if !strings.Contains(traits, "发型高马尾") || !strings.Contains(traits, "肤色古铜") {
		t.Fatalf("体貌没带上：%s", traits)
	}
	item := AssetTemplateItem{Key: "front", Class: "sheet", Prompt: "正面平视"}
	p := templatePrompt("夜惊鸿", traits, item)
	for _, want := range []string{"夜惊鸿", "发型高马尾", "正面平视", "角色建模用视图"} {
		if !strings.Contains(p, want) {
			t.Fatalf("提示词缺 %q：%s", want, p)
		}
	}
	// 没有体貌特征时也不能崩
	if p := templatePrompt("图门", "", item); !strings.Contains(p, "图门") {
		t.Fatalf("提示词：%s", p)
	}
}
