package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// 前后端的分类规则必须一致。
//
// 角色库列表（后端算）与角色详情（前端算）曾各写一套规则：
// 前端 isWeapon 只排除了「三视图|定妆」，漏了「设定」；
// 前端 isRealFace 没覆盖「发参考/妆参考」。
// 结果「陆听潮-纯黑发参考」在列表被排除、在详情却落到「画面素材」区。
//
// 这里让 node 直接跑前端的 classification.js，与后端逐条对照。
// 对照集是全部真实素材名加一批构造的边界名，不只是几个样例。
func TestFrontendAndBackendClassificationAgree(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("没有 node，跳过前端规则对照")
	}
	names := classificationProbeNames(t)
	frontend := runFrontendClassifier(t, names)
	if len(frontend) != len(names) {
		t.Fatalf("前端返回 %d 条，应为 %d", len(frontend), len(names))
	}
	for i, name := range names {
		if frontend[i].Weapon != isWeaponAsset(name) {
			t.Errorf("%q：前端 isWeapon=%v，后端 %v", name, frontend[i].Weapon, isWeaponAsset(name))
		}
		if frontend[i].RealFace != isRealFaceAsset(name) {
			t.Errorf("%q：前端 isRealFace=%v，后端 %v", name, frontend[i].RealFace, isRealFaceAsset(name))
		}
	}
}

// 前端规则的关键字必须与后端一致。
//
// node 对照需要 node；没有 node 的机器上至少要保证两边的关键字集合
// 没分叉 —— 漏一个词，分类就会在两边不同。
func TestFrontendClassificationKeywordsMatchBackend(t *testing.T) {
	src, err := os.ReadFile("frontend/src/lib/classification.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, word := range backendWeaponWords {
		if !strings.Contains(text, word) {
			t.Errorf("前端规则缺兵器关键词 %q：后端认它，前端不认", word)
		}
	}
	for _, word := range backendRealFaceWords {
		if !strings.Contains(text, word) {
			t.Errorf("前端规则缺真人参考关键词 %q", word)
		}
	}
}

// backendWeaponWords 后端认作兵器的关键词。
var backendWeaponWords = []string{
	"武器", "兵器", "道具", "面具", "面甲",
	"枪", "剑", "刀", "弓", "笛", "锤", "鞭", "匕", "枷",
}

// backendRealFaceWords 后端认作真人参考的关键词。
var backendRealFaceWords = []string{
	"真人参考", "脸部参考", "真人", "脸参考",
	"发参考", "发型参考", "妆造参考", "妆参考", "face",
}

// classificationProbeNames 拿真实素材名 + 构造的边界名当对照集。
func classificationProbeNames(t *testing.T) []string {
	names := []string{
		// 真实素材名
		"顾栖月-三视图-顾栖月_v1_玉笛", "顾栖月-三视图-顾栖月_v4_玉笛",
		"陆听潮-纯黑发参考", "陆听潮-真人参考-头", "陆听潮-脸部参考",
		"柳南栀-脸部参考", "欧阳断木-脸部参考", "慕北歌-脸部参考",
		"夜惊鸿-惊鸿枪", "慕北歌-问雪剑", "慕北歌-问天剑", "夜惊鸿-面具",
		"顾七玥-玉尘笛", "顾栖月-玉尘笛", "欧阳狂徒-破军",
		"顾栖月-头像", "夜惊鸿-定妆-夜惊鸿-定妆", "燕照绫-定妆-燕照绫-poss",
		"夜惊鸿-三视图", "夜惊鸿-王服三视图", "慕夕歌-三视图-慕夕歌_v2",
		"雨夜长街-场景参考-v1", "夜惊鸿-画面-雪夜战场",
		// 构造的边界名
		"某某-三视图-设定稿", "某某-定妆-立绘", "某某-设定-甲",
		"某某-妆造参考", "某某-发型参考", "某某-真人", "某某-face",
		"某某-道具-玉佩", "某某-面甲", "某某-鞭", "某某-匕", "某某-枷",
		"某某-锤", "某某-弓", "某某-刀", "某某-枪", "某某-剑",
		"某某-兵器", "某某-武器", "某某-面具",
		"某某-真人参考-甲", "某某-脸参考-乙", "某某-妆参考-丙",
		"某某-发参考-丁", "某某-turnaround",
	}
	// 真实素材库里出现过的名字
	if s := newTestStore(t); s != nil {
		for _, a := range s.State().Assets {
			names = append(names, a.Name)
		}
	}
	return names
}

type frontendVerdict struct {
	Weapon   bool `json:"weapon"`
	RealFace bool `json:"realFace"`
}

// runFrontendClassifier 用 node 跑前端的分类模块。
func runFrontendClassifier(t *testing.T, names []string) []frontendVerdict {
	t.Helper()
	script := `
import { isWeapon, isRealFace } from './frontend/src/lib/classification.js'
const names = JSON.parse(process.argv[2])
process.stdout.write(JSON.stringify(names.map((n) => ({ weapon: isWeapon(n), realFace: isRealFace(n) }))))
`
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
	var got []frontendVerdict
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("解析 node 输出：%v", err)
	}
	return got
}
