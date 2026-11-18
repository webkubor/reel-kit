package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 前后端的分类规则必须一致。
//
// 角色库列表（后端算）与角色详情（前端算）曾各写一套：
// 前端 isWeapon 只排除了「三视图|定妆」，漏了「设定」；
// 前端 isRealFace 没覆盖「发参考/妆参考」。
// 结果「陆听鸿-纯黑发参考」在列表被排除、在详情却落到「画面素材」区。
//
// 这里让 node 直接跑前端的 classification.js，与后端逐条对照。
func TestFrontendAndBackendClassificationAgree(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("没有 node，跳过前端规则对照")
	}
	cases := []struct {
		name     string
		weapon   bool
		realFace bool
	}{
		{"顾栖月-三视图-顾栖月_v1_玉笛", false, false}, // 带兵器名的三视图归定妆
		{"顾栖月-三视图-顾栖月_v4_玉笛", false, false},
		{"陆听潮-纯黑发参考", false, true},      // 发型参考是真人照片
		{"陆听潮-真人参考-头", false, true},     //
		{"柳南栀-脸部参考", false, true},       //
		{"夜惊鸿-惊鸿枪", true, false},        //
		{"慕北歌-问雪剑", true, false},        //
		{"夜惊鸿-面具", true, false},         //
		{"顾栖月-头像", false, false},        //
		{"夜惊鸿-定妆-夜惊鸿-定妆", false, false}, //
	}
	// 前端对每条名字的判定
	names := make([]string, len(cases))
	for i, c := range cases {
		names[i] = c.name
	}
	script := `
import { isWeapon, isRealFace } from './frontend/src/lib/classification.js'
const names = JSON.parse(process.argv[2])
const out = names.map((n) => ({ weapon: isWeapon(n), realFace: isRealFace(n) }))
process.stdout.write(JSON.stringify(out))
`
	dir := t.TempDir()
	file := filepath.Join(dir, "check.mjs")
	if err := os.WriteFile(file, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	// 让 node 能解析 import：从仓库根跑，脚本放在同级的临时目录不行，
	// 所以直接在仓库里跑一个临时脚本。
	tmp := "zz_frontend_rules_check.mjs"
	if err := os.WriteFile(tmp, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)
	payload, _ := json.Marshal(names)
	out, err := exec.Command("node", tmp, string(payload)).Output()
	if err != nil {
		t.Fatalf("node 跑失败：%v", err)
	}
	var got []struct {
		Weapon   bool `json:"weapon"`
		RealFace bool `json:"realFace"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("解析 node 输出：%v", err)
	}
	for i, c := range cases {
		if got[i].Weapon != c.weapon {
			t.Errorf("%q：前端 isWeapon=%v，后端 %v", c.name, got[i].Weapon, isWeaponAsset(c.name))
		}
		if got[i].RealFace != c.realFace {
			t.Errorf("%q：前端 isRealFace=%v，后端 %v", c.name, got[i].RealFace, isRealFaceAsset(c.name))
		}
	}
}
