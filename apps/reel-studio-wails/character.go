package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 角色素材库：模型、完成度判定与本地镜像。
//
// 分工（见 docs/character-spec.md）：角色资产存在 museav 中台，这里只做
// 两件事 —— 把中台的形状翻译成界面能懂的结构，以及算出「这个角色算不算
// 创建完成」。完成判定必须是可解释的：缺哪一项就报哪一项，不能只给一个
// 「未完成」，否则用户不知道下一步该补什么。
//
// 判定标准：字段齐 + 五视图齐 + 视觉标记达标。三者缺一即 draft。

// CharacterView 角色的一套视图。
//
// 五视图是全身正交转面组：正面 / 左45° / 正侧面 / 右45° / 背面。
// 中台的 role 词表只有三视图（view_front / view_side / view_back），
// 两个 45° 视角没有专属 role，只能落在 raw 里靠素材名归位 —— 所以
// 这里的本地 role 跟中台不是一一对应。
type CharacterView string

const (
	ViewFront   CharacterView = "front"   // 正面
	ViewLeft45  CharacterView = "left45"  // 左 45°
	ViewSide    CharacterView = "side"    // 正侧面
	ViewRight45 CharacterView = "right45" // 右 45°
	ViewBack    CharacterView = "back"    // 背面
)

// CharacterViews 五视图的固定顺序（正面 → 背面）。
//
// 顺序固定是为了界面能按「正面到背面」排，而不是按生成顺序排 ——
// 生成顺序是随机的，界面每次刷新顺序都不一样会让人以为出问题了。
var CharacterViews = []CharacterView{ViewFront, ViewLeft45, ViewSide, ViewRight45, ViewBack}

// CharacterViewLabels 五视图的中文名。
var CharacterViewLabels = map[CharacterView]string{
	ViewFront:   "正面",
	ViewLeft45:  "左 45°",
	ViewSide:    "正侧面",
	ViewRight45: "右 45°",
	ViewBack:    "背面",
}

// CharacterView 中台 role 映射。
//
// 正位三个有专属 role，直接落上去；两个 45° 只能落 raw，靠名字归位。
// 这样中台那边也能用 role 直接看到「三视图齐了没」。
func (v CharacterView) subjectRole() string {
	switch v {
	case ViewFront:
		return "view_front"
	case ViewSide:
		return "view_side"
	case ViewBack:
		return "view_back"
	default:
		return "raw"
	}
}

// CharacterViewFromSubjectRole 从中台 role 反推本地视图。
//
// 中台 role 只能定位三个正位；raw 里的 45° 靠素材名判断。
func CharacterViewFromSubjectRole(role, name string) (CharacterView, bool) {
	switch role {
	case "view_front":
		return ViewFront, true
	case "view_side":
		return ViewSide, true
	case "view_back":
		return ViewBack, true
	case "raw", "standard":
		// 45° 视角靠素材名归位：名字里带「左45」「右45」才算。
		// 「左 45°」「左45度」「left45」几种写法都要认。
		n := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, " ", ""), "°", "度"))
		switch {
		case strings.Contains(n, "左45"), strings.Contains(n, "left45"):
			return ViewLeft45, true
		case strings.Contains(n, "右45"), strings.Contains(n, "right45"):
			return ViewRight45, true
		}
	}
	return "", false
}

// CharacterTrait 一条体貌特征。
type CharacterTrait struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// RequiredTraitKeys 体貌必填四项。
//
// 这四项是「脸像不像」的地基：发型、脸型、肤色、体型。
// 少了任何一项，换一个镜头就可能长成另一个人 —— 那样的角色不算建好。
var RequiredTraitKeys = []string{"发型", "脸型", "肤色", "体型"}

// CharacterAsset 一张角色视图图。
type CharacterAsset struct {
	ID          string        `json:"id"`
	View        CharacterView `json:"view"`
	Role        string        `json:"role"`
	Name        string        `json:"name"`
	URL         string        `json:"url"`
	JobID       string        `json:"jobId,omitempty"`
	SourceBatch string        `json:"sourceBatch,omitempty"`
}

// Character 一个角色（中台主体 + 本地镜像）。
type Character struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	// Persona 形象来源：real 真人 / virtual 虚拟人。
	Persona string           `json:"persona"`
	Traits  []CharacterTrait `json:"traits"`
	Assets  []CharacterAsset `json:"assets"`
	// Complete 是否创建完成。
	Complete bool `json:"complete"`
	// Missing 缺件清单（可读中文），Complete 为 false 时告诉用户还差什么。
	Missing []string `json:"missing,omitempty"`
	// Warnings 风险提示：不拦住出片，但要让用户知道（如无法确认五视图同源）。
	Warnings []string `json:"warnings,omitempty"`
	// SourceBatch 五视图的同源批次标记。
	//
	// 由发起 0→1 创建的那一方打标并存进本地镜像 —— 平台不存这个。
	// 平台的 gen_job_id 是逐图的（批量出图每行一个 job），判不了同源。
	SourceBatch string `json:"sourceBatch,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// CharacterViewAsset 取某个视图的图。
func (c *Character) CharacterViewAsset(v CharacterView) (CharacterAsset, bool) {
	for _, a := range c.Assets {
		if a.View == v {
			return a, true
		}
	}
	return CharacterAsset{}, false
}

// traitValue 取某条体貌特征的值。
func (c *Character) traitValue(key string) string {
	for _, t := range c.Traits {
		if t.Key == key {
			return strings.TrimSpace(t.Value)
		}
	}
	return ""
}

// EvaluateCharacter 算一个角色的完成度、缺件清单与风险提示。
//
// 判定标准见 docs/character-spec.md：
//
//	字段齐（名称 / 来源 / 体貌四项）→ 五视图齐 → 视觉标记达标。
//
// 返回三样东西：complete（能不能进流水线）、missing（缺什么，必须补）、
// warnings（有什么风险，但不拦）。
//
// 为什么把「同源」和「缺件」分开：缺名称、缺视图是能验的事实，补上就好；
// 「五视图是不是同一次定妆」平台侧没有可靠信号（gen_job_id 是逐图的），
// 只有发起 0→1 的那一方知道。所以它不硬拦，但要把「不能确认同源」说清楚 ——
// 假装它达标，用户出片发现换了脸会更难受。
func EvaluateCharacter(c *Character) (bool, []string, []string) {
	var missing []string
	var warnings []string

	if strings.TrimSpace(c.Name) == "" {
		missing = append(missing, "缺名称")
	}
	if c.Persona != "real" && c.Persona != "virtual" {
		missing = append(missing, "缺形象来源（真人 / 虚拟人）")
	}
	for _, key := range RequiredTraitKeys {
		if c.traitValue(key) == "" {
			missing = append(missing, "缺体貌："+key)
		}
	}

	// 五视图。
	for _, v := range CharacterViews {
		if _, ok := c.CharacterViewAsset(v); !ok {
			missing = append(missing, "缺视图："+CharacterViewLabels[v])
		}
	}

	// 同源：能证明不是同批就硬拦，不能证明就提示风险。
	if len(missing) == 0 {
		if hard, warn := c.consistencyCheck(); hard != "" {
			missing = append(missing, hard)
		} else if warn != "" {
			warnings = append(warnings, warn)
		}
	}
	return len(missing) == 0, missing, warnings
}

// consistencyCheck 判断五视图是否同源。
//
// 返回 (硬缺件, 风险提示)：证明了不是同批 → 前者非空（拦住）；
// 无法确认 → 后者非空（提示但不拦）；确认同批 → 都为空。
//
// 判据是 SourceBatch —— 由发起 0→1 创建的那一方打的标记，本地镜像记住。
// 平台的 gen_job_id 判不了这个：批量出图每行是一个独立 job，同一批的 5 张
// 图各有各的 job ID，拿它判会把同批反被判成漂移。
func (c *Character) consistencyCheck() (string, string) {
	batches := map[string]bool{}
	for _, a := range c.Assets {
		if a.SourceBatch != "" {
			batches[a.SourceBatch] = true
		}
	}
	if len(batches) > 1 {
		return fmt.Sprintf("五视图来自 %d 个不同批次（不是同一次定妆），样貌会漂", len(batches)), ""
	}
	if len(batches) == 0 {
		return "", "五视图的生成批次没记录，无法确认是不是同一次定妆；出片前留意样貌是否漂"
	}
	return "", ""
}

// ──────────────────────────────────────────────────────────────
// 中台主体（museav subjects）形状
// ──────────────────────────────────────────────────────────────

// Subject 中台主体。
type Subject struct {
	ID            string         `json:"id"`
	WorkspaceID   string         `json:"workspace_id"`
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Traits        map[string]any `json:"traits"`
	PersonaSource *string        `json:"persona_source"`
	AssetCount    int            `json:"asset_count"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

// SubjectAsset 中台主体名下的一张图。
type SubjectAsset struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	JobID     string `json:"gen_job_id"`
	MediaType string `json:"media_type"`
}

// SubjectDetail `museav subjects show` 的输出。
type SubjectDetail struct {
	Subject Subject                   `json:"subject"`
	Assets  []SubjectAsset            `json:"assets"`
	ByRole  map[string][]SubjectAsset `json:"by_role"`
}

// museavTraitOrder traits 的输出顺序。
//
// 中台把 traits 存成 JSON 对象，对象本身无序。这里按固定顺序读出来，
// 界面上的体貌列表每次刷新都一样的顺序。
var museavTraitOrder = []string{
	"发型", "脸型", "肤色", "体型", "眉形", "眼型", "鼻型", "唇形",
	"年龄感", "身高", "服装", "配饰", "鞋靴", "气质", "辨识特征",
}

// subjectTraits 把中台 traits 读成有序列表。
func subjectTraits(traits map[string]any) []CharacterTrait {
	out := []CharacterTrait{}
	seen := map[string]bool{}
	for _, key := range museavTraitOrder {
		value, ok := traits[key]
		if !ok {
			continue
		}
		text := traitText(value)
		if text == "" {
			continue
		}
		out = append(out, CharacterTrait{Key: key, Value: text})
		seen[key] = true
	}
	// 剩下的键按字典序补在后面，避免自定义字段被丢掉。
	rest := []string{}
	for key := range traits {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		text := traitText(traits[key])
		if text == "" {
			continue
		}
		out = append(out, CharacterTrait{Key: key, Value: text})
	}
	return out
}

// traitText 把 traits 里的值读成一句话。
func traitText(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
	case bool:
		return fmt.Sprintf("%t", v)
	case []any:
		parts := []string{}
		for _, item := range v {
			if s := traitText(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "、")
	case map[string]any:
		parts := []string{}
		for _, key := range museavTraitOrder {
			if item, ok := v[key]; ok {
				if s := traitText(item); s != "" {
					parts = append(parts, key+"："+s)
				}
			}
		}
		return strings.Join(parts, "；")
	}
	return ""
}

// subjectToCharacter 把中台主体翻译成本地角色。
//
// 视图映射靠 role + 素材名：正位三个由 role 定位，两个 45° 由名字归位。
// 认不出来的图不硬塞进某个视图 —— 塞进去会污染完成度判定，
// 让一个缺视图的角色看起来是齐的。
func subjectToCharacter(detail SubjectDetail, projectID, sourceBatch string) *Character {
	c := &Character{
		ID:          detail.Subject.ID,
		ProjectID:   projectID,
		Workspace:   detail.Subject.WorkspaceID,
		Name:        detail.Subject.Name,
		Kind:        detail.Subject.Kind,
		Persona:     strings.TrimSpace(strOr(detail.Subject.PersonaSource, "")),
		Traits:      subjectTraits(detail.Subject.Traits),
		Assets:      []CharacterAsset{},
		SourceBatch: sourceBatch,
		CreatedAt:   detail.Subject.CreatedAt,
		UpdatedAt:   detail.Subject.UpdatedAt,
	}
	for _, a := range detail.Assets {
		view, ok := CharacterViewFromSubjectRole(a.Role, a.Name)
		if !ok {
			continue
		}
		c.Assets = append(c.Assets, CharacterAsset{
			ID: a.ID, View: view, Role: a.Role, Name: a.Name, URL: a.URL, JobID: a.JobID, SourceBatch: sourceBatch,
		})
	}
	c.Complete, c.Missing, c.Warnings = EvaluateCharacter(c)
	return c
}

// parseSubjectList 解析 `museav subjects list` 的 JSON。
func parseSubjectList(data []byte) ([]Subject, error) {
	var out []Subject
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("中台主体列表解析失败: %w", err)
	}
	return out, nil
}

// parseSubjectDetail 解析 `museav subjects show` 的 JSON。
func parseSubjectDetail(data []byte) (*SubjectDetail, error) {
	var out SubjectDetail
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("中台主体详情解析失败: %w", err)
	}
	return &out, nil
}
