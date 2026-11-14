package main

import (
	"strings"
	"testing"
)

// 角色完成度判定的测试。
//
// 这套判定是「一个角色算不算建好」的唯一判据，也是仿真人视频流水线的
// 闸门 —— 判错了要么卡住用户，要么放行一个换个镜头就换脸的角色。

func traitsAll() []CharacterTrait {
	return []CharacterTrait{
		{Key: "发型", Value: "长直发"},
		{Key: "脸型", Value: "鹅蛋脸"},
		{Key: "肤色", Value: "冷白"},
		{Key: "体型", Value: "纤细高挑"},
	}
}

func characterWithViews(batch string, views ...CharacterView) *Character {
	c := &Character{
		ID:          "c1",
		Name:        "顾栖月",
		Persona:     "real",
		Traits:      traitsAll(),
		SourceBatch: batch,
	}
	for _, v := range views {
		c.Assets = append(c.Assets, CharacterAsset{
			ID: string(v), View: v, Role: v.subjectRole(), Name: "顾栖月-" + CharacterViewLabels[v],
			URL: "https://example.invalid/a.png", JobID: "job-" + string(v), SourceBatch: batch,
		})
	}
	return c
}

func TestEvaluateCharacterComplete(t *testing.T) {
	c := characterWithViews("job-1", CharacterViews...)
	complete, missing, _ := EvaluateCharacter(c)
	if !complete {
		t.Fatalf("五视图齐全、体貌齐全，应当判定完成，实际缺：%v", missing)
	}
	if len(missing) != 0 {
		t.Fatalf("完成时不应有缺件，实际：%v", missing)
	}
}

func TestEvaluateCharacterMissingEachView(t *testing.T) {
	for _, skip := range CharacterViews {
		views := []CharacterView{}
		for _, v := range CharacterViews {
			if v != skip {
				views = append(views, v)
			}
		}
		c := characterWithViews("job-1", views...)
		complete, missing, _ := EvaluateCharacter(c)
		if complete {
			t.Fatalf("缺 %s 时不应判定完成", CharacterViewLabels[skip])
		}
		joined := strings.Join(missing, "、")
		if !strings.Contains(joined, CharacterViewLabels[skip]) {
			t.Fatalf("缺件清单应点名 %s，实际：%s", CharacterViewLabels[skip], joined)
		}
	}
}

func TestEvaluateCharacterMissingTraits(t *testing.T) {
	c := characterWithViews("job-1", CharacterViews...)
	c.Traits = []CharacterTrait{{Key: "发型", Value: "长直发"}}
	complete, missing, _ := EvaluateCharacter(c)
	if complete {
		t.Fatal("体貌缺项时不应判定完成")
	}
	joined := strings.Join(missing, "、")
	for _, key := range []string{"脸型", "肤色", "体型"} {
		if !strings.Contains(joined, key) {
			t.Fatalf("缺件清单应点名体貌 %s，实际：%s", key, joined)
		}
	}
}

func TestEvaluateCharacterMissingNameAndPersona(t *testing.T) {
	c := characterWithViews("job-1", CharacterViews...)
	c.Name = ""
	c.Persona = ""
	complete, missing, _ := EvaluateCharacter(c)
	if complete {
		t.Fatal("缺名称与来源时不应判定完成")
	}
	joined := strings.Join(missing, "、")
	if !strings.Contains(joined, "名称") || !strings.Contains(joined, "形象来源") {
		t.Fatalf("缺件清单应点名名称与来源，实际：%s", joined)
	}
}

// 能证明不是同一批 → 硬拦：五视图分几轮生成必然漂，不该进流水线。
func TestEvaluateCharacterDifferentBatches(t *testing.T) {
	c := characterWithViews("batch-1", CharacterViews...)
	c.Assets[2].SourceBatch = "batch-2"
	complete, missing, warnings := EvaluateCharacter(c)
	if complete {
		t.Fatal("五视图来自不同批次时不应判定完成")
	}
	if !strings.Contains(strings.Join(missing, "、"), "定妆") {
		t.Fatalf("缺件清单应说明不是同一次定妆，实际：%s", strings.Join(missing, "、"))
	}
	if len(warnings) != 0 {
		t.Fatalf("已硬拦，不该再给警示：%v", warnings)
	}
}

// 没有批次记录就说不清同源 —— 不硬拦（可能是在别处建的合法角色），
// 但必须提示风险，不能默默当作达标。
func TestEvaluateCharacterNoBatchInfo(t *testing.T) {
	c := characterWithViews("", CharacterViews...)
	complete, missing, warnings := EvaluateCharacter(c)
	if !complete {
		t.Fatalf("缺批次记录只应警示、不拦截，实际缺件：%v", missing)
	}
	if len(warnings) == 0 {
		t.Fatal("无法确认同源时应有风险提示")
	}
	if !strings.Contains(strings.Join(warnings, "、"), "定妆") {
		t.Fatalf("提示应说明无法确认同一次定妆，实际：%s", strings.Join(warnings, "、"))
	}
}

// 同一批次 → 既不缺件也无警示。
func TestEvaluateCharacterSameBatch(t *testing.T) {
	c := characterWithViews("batch-1", CharacterViews...)
	complete, missing, warnings := EvaluateCharacter(c)
	if !complete {
		t.Fatalf("同批次五视图应判定完成，实际缺件：%v", missing)
	}
	if len(warnings) != 0 {
		t.Fatalf("同批次不应有警示：%v", warnings)
	}
}

func TestCharacterViewRoleMapping(t *testing.T) {
	cases := map[CharacterView]string{
		ViewFront:   "view_front",
		ViewSide:    "view_side",
		ViewBack:    "view_back",
		ViewLeft45:  "raw",
		ViewRight45: "raw",
	}
	for view, want := range cases {
		if got := view.subjectRole(); got != want {
			t.Fatalf("%s 的中台 role 应为 %s，实际 %s", view, want, got)
		}
	}
}

// 45° 视角靠素材名归位 —— 中台没有专门的 role。
func TestCharacterViewFromSubjectRole(t *testing.T) {
	cases := []struct {
		role string
		name string
		want CharacterView
		ok   bool
	}{
		{"view_front", "任意", ViewFront, true},
		{"view_side", "任意", ViewSide, true},
		{"view_back", "任意", ViewBack, true},
		{"raw", "顾栖月-左45°", ViewLeft45, true},
		{"raw", "顾栖月-左 45 度", ViewLeft45, true},
		{"raw", "顾栖月-left45", ViewLeft45, true},
		{"raw", "顾栖月-右45°", ViewRight45, true},
		{"raw", "顾栖月-right45", ViewRight45, true},
		{"raw", "基准人脸", "", false},
		{"standard", "顾栖月", "", false},
	}
	for _, c := range cases {
		got, ok := CharacterViewFromSubjectRole(c.role, c.name)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("role=%s name=%s 应得 (%s,%v)，实际 (%s,%v)", c.role, c.name, c.want, c.ok, got, ok)
		}
	}
}

// 中台主体翻译成本地角色时，认不出来的图不能硬塞进某个视图 ——
// 塞进去会让缺视图的角色看起来是齐的。
func TestSubjectToCharacterIgnoresUnknownAssets(t *testing.T) {
	detail := SubjectDetail{
		Subject: Subject{
			ID: "s1", Kind: "person", Name: "顾栖月",
			PersonaSource: strPtr("real"),
			Traits:        map[string]any{"发型": "长直发", "脸型": "鹅蛋脸", "肤色": "冷白", "体型": "纤细"},
		},
		Assets: []SubjectAsset{
			{ID: "a1", Role: "view_front", Name: "顾栖月-正面", URL: "u1", JobID: "job-1"},
			{ID: "a2", Role: "raw", Name: "基准人脸", URL: "u2", JobID: "job-1"},
		},
	}
	c := subjectToCharacter(detail, "p1", "")
	if len(c.Assets) != 1 {
		t.Fatalf("只应认出 1 张视图图，实际认出 %d 张", len(c.Assets))
	}
	complete, missing, _ := EvaluateCharacter(c)
	if complete {
		t.Fatal("只有一张视图时不应判定完成")
	}
	if !strings.Contains(strings.Join(missing, "、"), "缺视图") {
		t.Fatalf("缺件清单应说明缺视图，实际：%s", strings.Join(missing, "、"))
	}
}

func TestSubjectTraitsOrder(t *testing.T) {
	traits := subjectTraits(map[string]any{
		"体型":  "纤细",
		"发型":  "长直发",
		"自定义": "值",
		"脸型":  "鹅蛋脸",
	})
	if len(traits) != 4 {
		t.Fatalf("应有 4 条体貌，实际 %d 条", len(traits))
	}
	if traits[0].Key != "发型" || traits[1].Key != "脸型" {
		t.Fatalf("体貌应按固定顺序输出，实际开头是 %s、%s", traits[0].Key, traits[1].Key)
	}
	if traits[3].Key != "自定义" {
		t.Fatalf("自定义字段应保留在末尾，实际末尾是 %s", traits[3].Key)
	}
}

// 中台真实回执样张（museav subjects show 实测输出，脱敏域名保留）。
// 用真实格式而不是手搓结构，是为了让解析路径对得上服务端真的输出 ——
// 格式漂了（字段名变了、嵌套变了）这里会当场失败。
const realSubjectReceipt = `{"subject":{"id":"9efdc346-aca6-4b20-a67c-570663a128b1","workspace_id":"6f250169-b42e-498b-a898-09efdc1441bb","kind":"person","name":"古风女主","traits":{},"created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-09-23T17:10:47.828219+00:00","updated_at":"2026-09-23T17:10:47.828219+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","persona_source":null,"asset_count":1,"processed_count":1,"likeness_count":0,"current_likeness":null},"assets":[{"id":"981e9d74-46cd-48ad-89c5-9af79075132e","workspace_id":"6f250169-b42e-498b-a898-09efdc1441bb","subject_id":"9efdc346-aca6-4b20-a67c-570663a128b1","r2_key":"platform/generated/custom/c6a8f524-7ade-4d5d-974e-502844c6de2d.png","media_type":"image","origin":"generated","role":"standard","derived_from":null,"gen_job_id":"c6a8f524-7ade-4d5d-974e-502844c6de2d","name":"自由出图 · 09-16 11:05","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-09-23T17:10:48.724733+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/platform/generated/custom/c6a8f524-7ade-4d5d-974e-502844c6de2d.png"}],"by_role":{"raw":[],"standard":[{"id":"981e9d74-46cd-48ad-89c5-9af79075132e","workspace_id":"6f250169-b42e-498b-a898-09efdc1441bb","subject_id":"9efdc346-aca6-4b20-a67c-570663a128b1","r2_key":"platform/generated/custom/c6a8f524-7ade-4d5d-974e-502844c6de2d.png","media_type":"image","origin":"generated","role":"standard","derived_from":null,"gen_job_id":"c6a8f524-7ade-4d5d-974e-502844c6de2d","name":"自由出图 · 09-16 11:05","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-09-23T17:10:48.724733+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/platform/generated/custom/c6a8f524-7ade-4d5d-974e-502844c6de2d.png"}],"view_front":[],"view_side":[],"view_back":[],"output":[]}}
`

// 中台真实回执样张：五视图半成状态（含 45° 按名归位 + 基准人脸）。
// 这条是 E2E 建主体、挂图、读回真跑出来的，用来证明 role+名字的归位逻辑
// 对得上服务端实际输出。
const e2eSubjectReceipt = `{"subject":{"id":"3f38d8c8-ad41-408e-b075-a3fde1474618","workspace_id":null,"kind":"person","name":"E2E测试角色-可删","traits":{"体型":"纤细","发型":"长直发","肤色":"冷白","脸型":"鹅蛋脸"},"created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-11-14T18:40:16.337152+00:00","updated_at":"2026-11-14T18:40:16.337152+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","persona_source":"real","asset_count":3,"processed_count":1,"likeness_count":0,"current_likeness":null},"assets":[{"id":"d097b342-9eba-434d-806b-ccd3e7e9b62c","workspace_id":null,"subject_id":"3f38d8c8-ad41-408e-b075-a3fde1474618","r2_key":"refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg","media_type":"image","origin":"upload","role":"raw","derived_from":null,"gen_job_id":null,"name":"E2E测试角色-可删-左45°","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-11-14T18:40:49.063341+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg"},{"id":"417630ef-8594-4c54-8b29-9728248129cf","workspace_id":null,"subject_id":"3f38d8c8-ad41-408e-b075-a3fde1474618","r2_key":"refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg","media_type":"image","origin":"upload","role":"view_front","derived_from":null,"gen_job_id":null,"name":"E2E测试角色-可删-正面","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-11-14T18:40:38.532475+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg"},{"id":"4d50f670-0d6e-4575-b835-1ccffe0799a3","workspace_id":null,"subject_id":"3f38d8c8-ad41-408e-b075-a3fde1474618","r2_key":"refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg","media_type":"image","origin":"upload","role":"raw","derived_from":null,"gen_job_id":null,"name":"基准人脸","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-11-14T18:40:25.031077+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg"}],"by_role":{"raw":[{"id":"d097b342-9eba-434d-806b-ccd3e7e9b62c","workspace_id":null,"subject_id":"3f38d8c8-ad41-408e-b075-a3fde1474618","r2_key":"refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg","media_type":"image","origin":"upload","role":"raw","derived_from":null,"gen_job_id":null,"name":"E2E测试角色-可删-左45°","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-11-14T18:40:49.063341+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg"},{"id":"4d50f670-0d6e-4575-b835-1ccffe0799a3","workspace_id":null,"subject_id":"3f38d8c8-ad41-408e-b075-a3fde1474618","r2_key":"refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg","media_type":"image","origin":"upload","role":"raw","derived_from":null,"gen_job_id":null,"name":"基准人脸","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-11-14T18:40:25.031077+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg"}],"standard":[],"view_front":[{"id":"417630ef-8594-4c54-8b29-9728248129cf","workspace_id":null,"subject_id":"3f38d8c8-ad41-408e-b075-a3fde1474618","r2_key":"refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg","media_type":"image","origin":"upload","role":"view_front","derived_from":null,"gen_job_id":null,"name":"E2E测试角色-可删-正面","created_by":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","created_at":"2026-11-14T18:40:38.532475+00:00","account_id":"45d47835-cde2-4022-bd4d-5bc729ff8f1f","likeness_id":null,"url":"https://img.webkubor.online/refs/45d47835-cde2-4022-bd4d-5bc729ff8f1f/5855a6cb-4ed.jpg"}],"view_side":[],"view_back":[],"output":[]}}
`

func strPtr(s string) *string { return &s }

// 用真实回执验解析链路：中台 JSON → 主体 → 本地角色。
func TestParseRealSubjectReceipt(t *testing.T) {
	detail, err := parseSubjectDetail([]byte(realSubjectReceipt))
	if err != nil {
		t.Fatalf("真实回执解析失败：%v", err)
	}
	if detail.Subject.ID != "9efdc346-aca6-4b20-a67c-570663a128b1" {
		t.Fatalf("主体 id 读错了：%s", detail.Subject.ID)
	}
	if detail.Subject.Name != "古风女主" {
		t.Fatalf("主体名读错了：%s", detail.Subject.Name)
	}
	if len(detail.Assets) != 1 {
		t.Fatalf("应读到 1 张名下图，实际 %d", len(detail.Assets))
	}
	// 这张图是 standard role、名字里没有 45° —— 不该被认成任何视图。
	c := subjectToCharacter(*detail, "p1", "")
	if len(c.Assets) != 0 {
		t.Fatalf("standard 图不该被认成视图图，实际认出 %d 张", len(c.Assets))
	}
	complete, missing, warnings := EvaluateCharacter(c)
	if complete {
		t.Fatal("这个主体只有 1 张非视图图，不该判定完成")
	}
	joined := strings.Join(missing, "、")
	if !strings.Contains(joined, "缺视图") {
		t.Fatalf("缺件清单应说明缺视图，实际：%s", joined)
	}
	if !strings.Contains(joined, "缺体貌") {
		t.Fatalf("该主体 traits 为空，缺件清单应说明缺体貌，实际：%s", joined)
	}
	_ = warnings
}

// E2E 真实回执：验证 45° 靠名字归位、基准人脸被排除、缺件清单可解释。
func TestE2ESubjectReceipt(t *testing.T) {
	detail, err := parseSubjectDetail([]byte(e2eSubjectReceipt))
	if err != nil {
		t.Fatalf("E2E 回执解析失败：%v", err)
	}
	if len(detail.Assets) != 3 {
		t.Fatalf("应有 3 张名下图，实际 %d", len(detail.Assets))
	}
	c := subjectToCharacter(*detail, "", "")
	// 三张图里只有两张是视图（正面 + 左45°）；基准人脸不带视图标记，不该被算进去。
	if len(c.Assets) != 2 {
		t.Fatalf("应认出 2 张视图图，实际认出 %d 张：%+v", len(c.Assets), c.Assets)
	}
	got := map[CharacterView]bool{}
	for _, a := range c.Assets {
		got[a.View] = true
	}
	if !got[ViewFront] || !got[ViewLeft45] {
		t.Fatalf("应认出正面与左45°，实际：%v", got)
	}
	if got[ViewBack] || got[ViewSide] || got[ViewRight45] {
		t.Fatalf("不该认出未登记的视图：%v", got)
	}

	complete, missing, _ := EvaluateCharacter(c)
	if complete {
		t.Fatal("只有两个视图，不该判定完成")
	}
	joined := strings.Join(missing, "、")
	for _, label := range []string{"正侧面", "右 45°", "背面"} {
		if !strings.Contains(joined, label) {
			t.Fatalf("缺件清单应点名缺 %s，实际：%s", label, joined)
		}
	}
	// 已有的两个视图不该出现在缺件清单里。
	for _, label := range []string{"正面", "左 45°"} {
		if strings.Contains(joined, label) {
			t.Fatalf("已有 %s 不该被报缺，实际：%s", label, joined)
		}
	}
}
