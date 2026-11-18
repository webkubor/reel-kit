package main

import "testing"

// 真人参考 / 脸部参考绝不能当角色头像。
//
// 角色是剧里的形象；真人照片只是给 AI 建模的参考。
// 曾经踩过的坑：镜像里记的是「陆听潮-头像」，素材库里那张图实际叫
// 「陆听潮-真人参考-头.jpg」——按镜像名判断就把真人照摆到了角色列表上。
func TestRealFaceNeverBecomesAvatar(t *testing.T) {
	for _, name := range []string{
		"陆听潮-真人参考-头.jpg",
		"柳南栀-脸部参考",
		"欧阳断木-真人参考",
	} {
		if !isRealFaceAsset(name) {
			t.Errorf("%q 应被认作真人参考", name)
		}
	}
}

// 三视图不能当头像：一张图里并排几个视角，缩成小方图看不清人。
func TestTurnaroundIsNotAvatar(t *testing.T) {
	if isAvatarAsset("顾栖月-三视图-顾栖月_v1_玉笛.png") {
		t.Error("三视图不该被认作头像")
	}
	if isAvatarAsset("顾栖月-头像") != true {
		t.Error("头像应被认作头像")
	}
	// 定妆立绘是单人像，可以回退当头像
	if !isCostumeAsset("夜惊鸿-定妆-夜惊鸿-定妆.png") {
		t.Error("定妆应被认作可当头像的立绘")
	}
	if isCostumeAsset("顾栖月-三视图-顾栖月_v1_玉笛.png") {
		t.Error("三视图不是立绘")
	}
}

// 兵器名要能从角色的「武器」设定里取出来。
//
// 中台写法是「重刃【破军】，门板重刃焦黑崩口」，方括号里是兵器本名；
// 素材名常直接用它（"欧阳狂徒-破军"），通用关键词猜不出来。
func TestWeaponNameFromTraits(t *testing.T) {
	traits := map[string]any{"武器": "重刃【破军】，门板重刃焦黑崩口"}
	names := weaponNamesFromTraits(traits)
	if len(names) != 1 || names[0] != "破军" {
		t.Fatalf("兵器名：%v", names)
	}
	if !isWeaponByTraits("欧阳狂徒-破军", traits) {
		t.Fatal("应认出这是兵器")
	}
	if isWeaponByTraits("欧阳狂徒-三视图", traits) {
		t.Fatal("三视图不是兵器")
	}
	// 没有武器设定时不该误判
	if isWeaponByTraits("欧阳狂徒-破军", map[string]any{}) {
		t.Fatal("没有设定就不该判成兵器")
	}
}

// 三视图定妆稿里带兵器名时，要归定妆而不是兵器。
func TestTurnaroundWithWeaponNameIsNotWeapon(t *testing.T) {
	if isWeaponAsset("顾栖月-三视图-顾栖月_v1_玉笛") {
		t.Fatal("带兵器名的三视图不该算兵器")
	}
	if !isDesignSheet("顾栖月-三视图-顾栖月_v1_玉笛") {
		t.Fatal("应认作定妆图")
	}
}

// 发型/妆造参考也是真人照片，不能漏进画面素材。
func TestHairReferenceIsRealFace(t *testing.T) {
	if !isRealFaceAsset("陆听潮-纯黑发参考") {
		t.Fatal("发型参考应算真人参考")
	}
}

// 角色改名后，素材名要跟着改成当前名。
//
// 「顾七玥-玉尘笛」挂在「顾栖月」名下 —— 按新名搜不到这把笛子，
// 分类也容易落到错的角色上。
func TestRenameMirrorAssetsToCurrentName(t *testing.T) {
	m := &CharacterMirror{Name: "顾栖月", Assets: []MirrorAsset{
		{Name: "顾七玥-三视图-旧版"},
		{Name: "顾七玥-定妆-旧版"},
		{Name: "顾七玥-玉尘笛"},
		{Name: "顾栖月-三视图-顾栖月_v1_玉笛"},
		{Name: "顾栖月-头像"},
		{Name: "雨夜长街-场景参考-v1"},
	}}
	n := renameMirrorAssetsToCurrentName(m, "顾栖月")
	// 旧名的三视图与定妆要改；当前名的不动；场景参考不动
	if n != 3 {
		t.Fatalf("应改 3 个：%d", n)
	}
	want := []string{
		"顾栖月-三视图-旧版",
		"顾栖月-定妆-旧版",
		"顾栖月-玉尘笛",
		"顾栖月-三视图-顾栖月_v1_玉笛",
		"顾栖月-头像",
		"雨夜长街-场景参考-v1",
	}
	for i, w := range want {
		if m.Assets[i].Name != w {
			t.Fatalf("第 %d 个素材：%s，应为 %s", i, m.Assets[i].Name, w)
		}
	}
}
