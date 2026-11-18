package main

import "testing"

// 内置模板齐全：角色类要有五视图与三视图，场景类要有场景参考。
func TestBuiltinAssetTemplates(t *testing.T) {
	all := (&Engine{}).ListAssetTemplates("")
	if len(all) < 5 {
		t.Fatalf("模板太少：%d", len(all))
	}
	keys := map[string]bool{}
	for _, tpl := range all {
		keys[tpl.Key] = true
		if len(tpl.Items) == 0 {
			t.Fatalf("模板 %s 没有内容", tpl.Key)
		}
		for _, item := range tpl.Items {
			if item.Prompt == "" || item.Class == "" {
				t.Fatalf("模板 %s 的项不完整：%+v", tpl.Key, item)
			}
		}
	}
	for _, want := range []string{"person-five-views", "person-three-views", "person-costume", "person-portrait", "person-weapon", "scene-reference"} {
		if !keys[want] {
			t.Fatalf("缺模板 %s", want)
		}
	}
	people := (&Engine{}).ListAssetTemplates("person")
	if len(people) != 5 {
		t.Fatalf("角色模板数：%d", len(people))
	}
}

// 五视图模板的视角要与 CharacterViews 一致，否则登记会错位。
func TestFiveViewTemplateViews(t *testing.T) {
	tpl, err := AssetTemplateByKey("person-five-views")
	if err != nil {
		t.Fatal(err)
	}
	if len(tpl.Items) != len(CharacterViews) {
		t.Fatalf("五视图项数 %d，应为 %d", len(tpl.Items), len(CharacterViews))
	}
	for i, item := range tpl.Items {
		if item.View != CharacterViews[i] {
			t.Fatalf("第 %d 项视角 %s，应为 %s", i, item.View, CharacterViews[i])
		}
	}
}

// 素材命名要符合分类规则：头像/定妆/三视图/武器/画面。
func TestTemplateAssetNaming(t *testing.T) {
	cases := []struct {
		class string
		key   string
		want  string
	}{
		{"avatar", "portrait", "夜惊鸿-头像"},
		{"costume", "costume", "夜惊鸿-定妆-costume"},
		{"weapon", "weapon", "夜惊鸿-武器-weapon"},
		{"scene", "scene", "夜惊鸿-画面-scene"},
		{"sheet", "front", "夜惊鸿-三视图-front"},
	}
	for _, c := range cases {
		got := templateAssetName("夜惊鸿", AssetTemplateItem{Key: c.key, Class: c.class})
		if got != c.want {
			t.Fatalf("%s 命名 %s，应为 %s", c.class, got, c.want)
		}
	}
}
