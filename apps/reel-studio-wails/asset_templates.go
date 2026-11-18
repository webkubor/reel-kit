package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// 素材模板：常用的一组出图，点了就按模板批量生成。
//
// 为什么要有模板：角色库的物料需求是成套的（三视图给建模、五视图给转面、
// 定妆给换装、兵器给道具）。每次让人自己拼提示词既慢又容易漏，
// 所以把常用组合固化成模板，一键产出。
//
// 模板里的每一项都写清了自己的用途 —— 出图提示词按用途写，
// 这样生成出来的图天然能各归各位（见 docs/asset-classification.md）。

// 素材模板里的一项。
type AssetTemplateItem struct {
	// Key 该项的标识（登记素材时写进名字）。
	Key string `json:"key"`
	// Label 界面显示名。
	Label string `json:"label"`
	// View 对应的角色视角（角色类模板用）。
	View CharacterView `json:"view,omitempty"`
	// Prompt 出图提示词主体。
	Prompt string `json:"prompt"`
	// Ratio 画幅。
	Ratio string `json:"ratio"`
	// Class 产出归类：avatar / costume / sheet / weapon / scene。
	Class string `json:"class"`
}

// AssetTemplate 一个素材模板。
type AssetTemplate struct {
	// Key 模板标识。
	Key string `json:"key"`
	// Label 名称。
	Label string `json:"label"`
	// Description 一句话说明什么时候用。
	Description string `json:"description"`
	// Kind 适用对象：person（角色）/ scene（场景）。
	Kind string `json:"kind"`
	// Cost 计费提示：这个模板会出几张图。
	Cost string `json:"cost"`
	// Items 模板包含的出图项。
	Items []AssetTemplateItem `json:"items"`
}

// 内置模板。
var builtinAssetTemplates = []AssetTemplate{
	{
		Key: "person-five-views", Label: "角色五视图", Kind: "person",
		Description: "正面、左右 45°、正侧面、背面；建模与转面镜头用，必须同源。",
		Cost:        "5 张图",
		Items: []AssetTemplateItem{
			{Key: "front", Label: "正面", View: ViewFront, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，正面平视，肩线水平，表情中性，双手自然下垂"},
			{Key: "left45", Label: "左 45°", View: ViewLeft45, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，转向左 45°，与正面同一服装同一发型"},
			{Key: "side", Label: "正侧面", View: ViewSide, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，正侧面 90°，与正面同一服装同一发型"},
			{Key: "right45", Label: "右 45°", View: ViewRight45, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，转向右 45°，与正面同一服装同一发型"},
			{Key: "back", Label: "背面", View: ViewBack, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，背面，能看清背部服装结构与发型"},
		},
	},
	{
		Key: "person-three-views", Label: "角色三视图", Kind: "person",
		Description: "正面、正侧面、背面；预算有限时的最小可用视图组。",
		Cost:        "3 张图",
		Items: []AssetTemplateItem{
			{Key: "front", Label: "正面", View: ViewFront, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，正面平视，肩线水平，表情中性"},
			{Key: "side", Label: "正侧面", View: ViewSide, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，正侧面 90°，与正面同一服装"},
			{Key: "back", Label: "背面", View: ViewBack, Class: "sheet", Ratio: "3:4", Prompt: "同一人物，背面，能看清背部服装结构"},
		},
	},
	{
		Key: "person-costume", Label: "定妆立绘", Kind: "person",
		Description: "单人全身立绘，纯色背景；锁定长相与服装，用于角色卡与换装。",
		Cost:        "1 张图",
		Items: []AssetTemplateItem{
			{Key: "costume", Label: "全身立绘", Class: "costume", Ratio: "3:4", Prompt: "同一人物全身立绘，站姿端正，双手自然下垂，完整展示服装与配饰，纯灰背景"},
		},
	},
	{
		Key: "person-portrait", Label: "角色头像", Kind: "person",
		Description: "半身特写；角色库列表头像用，只锁定脸与神态。",
		Cost:        "1 张图",
		Items: []AssetTemplateItem{
			{Key: "portrait", Label: "半身特写", Class: "avatar", Ratio: "1:1", Prompt: "同一人物半身特写，面部清晰、表情中性，光线均匀，浅灰背景"},
		},
	},
	{
		Key: "person-weapon", Label: "兵器图", Kind: "person",
		Description: "该角色本命兵器的单品图；与角色卡里的兵器名对应才送参考。",
		Cost:        "1 张图",
		Items: []AssetTemplateItem{
			{Key: "weapon", Label: "兵器单品", Class: "weapon", Ratio: "1:1", Prompt: "单件兵器，纯黑或深灰背景，侧光突出材质与纹样，无人物无手"},
		},
	},
	{
		Key: "scene-reference", Label: "场景参考", Kind: "scene",
		Description: "一处空间的空镜设定图；出镜时锁建筑结构、材质与光线。",
		Cost:        "1 张图",
		Items: []AssetTemplateItem{
			{Key: "scene", Label: "空间设定", Class: "scene", Ratio: "16:9", Prompt: "一处完整空间的空镜，前中后景分明，无人物无动物无文字标注"},
		},
	},
}

// ListAssetTemplates 列内置素材模板（可按 kind 过滤：person / scene）。
func (e *Engine) ListAssetTemplates(kind string) []AssetTemplate {
	out := []AssetTemplate{}
	for _, t := range builtinAssetTemplates {
		if kind != "" && t.Kind != kind {
			continue
		}
		out = append(out, t)
	}
	return out
}

// AssetTemplateByKey 取一个模板。
func AssetTemplateByKey(key string) (AssetTemplate, error) {
	for _, t := range builtinAssetTemplates {
		if t.Key == key {
			return t, nil
		}
	}
	return AssetTemplate{}, errors.New("模板不存在")
}

// GenerateTemplate 按模板为一个角色（或场景）生成一组素材。
//
// subjectID 传角色主体 id；场景模板则传场景主体 id。素材登记到该主体名下，
// 命名按 docs/asset-classification.md 的类别规则（头像/定妆/三视图/武器/画面）。
func (e *Engine) GenerateTemplate(ctx context.Context, subjectID, templateKey string, progress ProgressFunc) ([]string, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	tpl, err := AssetTemplateByKey(templateKey)
	if err != nil {
		return nil, err
	}
	client := &MuseavClient{}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}
	detail, err := client.ShowSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	subject := detail.Subject
	client.Workspace = subject.WorkspaceID
	traits := traitSummary(detail.Subject.Traits)
	produced := []string{}
	for i, item := range tpl.Items {
		if progress != nil {
			progress(fmt.Sprintf("正在生成 %s（%d/%d）…", item.Label, i+1, len(tpl.Items)))
		}
		prompt := templatePrompt(subject.Name, traits, item)
		url, err := client.GenerateImage(ctx, prompt, item.Ratio, "", "")
		if err != nil {
			// 出了几张就算几张：不能因为第三张失败就丢掉前两张（钱已经花了）
			return produced, fmt.Errorf("模板生成中断：%s（已完成 %d 张）: %w", item.Label, len(produced), err)
		}
		name := templateAssetName(subject.Name, item)
		if err := client.AddSubjectAsset(ctx, subjectID, item.View, name, "", url); err != nil {
			return produced, fmt.Errorf("素材登记失败：%s（已完成 %d 张）: %w", name, len(produced), err)
		}
		produced = append(produced, name)
	}
	return produced, nil
}

// templatePrompt 组装模板出图提示词。
func templatePrompt(subjectName, traits string, item AssetTemplateItem) string {
	parts := []string{subjectName}
	if traits != "" {
		parts = append(parts, traits)
	}
	parts = append(parts, item.Prompt)
	switch item.Class {
	case "avatar":
		parts = append(parts, "角色面部特写，作为该角色头像")
	case "costume":
		parts = append(parts, "角色定妆立绘，作为该角色服装参考")
	case "sheet":
		parts = append(parts, "角色建模用视图，与其他视角同一套服装同一张脸")
	case "weapon":
		parts = append(parts, "道具单品图，用于该角色的兵器参考")
	case "scene":
		parts = append(parts, "场景设定图，用于空间与光线参考")
	}
	return strings.Join(parts, "，")
}

// templateAssetName 按分类规则命名素材。
//
// 描述段用中文说明（"三视图-正面"、"武器-长枪"），不用英文占位词 ——
// 名字是给人看的，也是分类判断的依据。
func templateAssetName(subjectName string, item AssetTemplateItem) string {
	desc := item.Label
	if desc == "" {
		desc = item.Key
	}
	switch item.Class {
	case "avatar":
		return subjectName + "-头像"
	case "costume":
		return subjectName + "-定妆-" + desc
	case "weapon":
		return subjectName + "-武器-" + desc
	case "scene":
		return subjectName + "-画面-" + desc
	default:
		return subjectName + "-三视图-" + desc
	}
}

// traitSummary 把体貌特征拼成一段可读描述。
func traitSummary(traits map[string]any) string {
	if len(traits) == 0 {
		return ""
	}
	order := []string{"发型", "脸型", "肤色", "体型", "外貌", "武器"}
	parts := []string{}
	for _, key := range order {
		if v, ok := traits[key]; ok {
			if text, ok := v.(string); ok && strings.TrimSpace(text) != "" {
				parts = append(parts, key+text)
			}
		}
	}
	return strings.Join(parts, "，")
}
