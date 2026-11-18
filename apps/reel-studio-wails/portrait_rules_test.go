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
