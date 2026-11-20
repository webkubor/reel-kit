package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 角色出镜：选一个角色 → 中台出一段 ta 的镜头 → 落回本地素材库。
//
// 边界（AGENTS.md）：museav 出的是**无声无字幕的素材**，不是成片。
// 配音、字幕、合成是 muse·tv 的活。这里只做「让角色出镜」这一步。

// CharacterVideoInput 角色出镜的入参。
type CharacterVideoInput struct {
	ProjectID string
	// SubjectID 中台角色主体 id。
	SubjectID string
	// Action 角色在镜头里做什么（用户写的动作要求）。
	Action string
	// Ratio 画幅，空 = 交给中台按参考图定。
	Ratio string
	// Duration 时长（秒），0 = 交给中台按模型定。
	Duration int
	// Look 这场戏穿哪一套（定妆稿名里的装扮名），空 = 第一套。
	Look string
	// Scene 场景画面描述（来自场景主体），空 = 不指定场景。
	Scene string
	// SceneID 绑定场景主体；生成时读取其标准参考图。
	SceneID string
	// ExpectWorkspace 镜头所属项目绑定的中台工作区。非空时角色必须属于它：
	// 别的片子的角色不能出现在这部戏里。
	ExpectWorkspace string
}

// characterVideoTimeout 视频比图慢得多（中台通常 1-5 分钟），给 15 分钟。
var characterVideoTimeout = 15 * time.Minute

// MakeCharacterVideo 走一条「角色出镜」任务。
//
// 参考图走 --reference-image 而不是首帧：定妆稿是白底设计稿，
// 拿它当首帧，视频就从一张设定图开场。给参考图是让模型记住「这个人长什么样」。
func (e *Engine) MakeCharacterVideo(ctx context.Context, input CharacterVideoInput, progress ProgressFunc, report func(Step)) (*Asset, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	if report == nil {
		report = func(Step) {}
	}
	if progress == nil {
		progress = func(string) {}
	}
	input.SubjectID = strings.TrimSpace(input.SubjectID)
	input.Action = strings.TrimSpace(input.Action)
	if input.SubjectID == "" {
		return nil, errors.New("请先选择角色")
	}
	if input.Action == "" {
		return nil, errors.New("请填写角色在镜头里做什么")
	}

	ctx, cancel := context.WithTimeout(ctx, characterVideoTimeout)
	defer cancel()

	client := &MuseavClient{}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}

	progress("正在读取角色设定")
	detail, err := client.ShowSubject(ctx, input.SubjectID)
	if err != nil {
		return nil, err
	}
	character := subjectToCharacter(*detail, input.ProjectID, e.sourceBatchFor(detail.Subject.ID))
	// 哲学和计费都跟着角色所属的工作区走：沸腾之雪的角色就用沸腾之雪的
	// 口径、记在沸腾之雪名下。不能用「账号下第一个项目」去猜。
	client.Workspace = character.Workspace
	if input.ExpectWorkspace != "" && character.Workspace != input.ExpectWorkspace {
		return nil, fmt.Errorf("角色「%s」不属于当前项目绑定的中台项目", character.Name)
	}
	action := input.Action
	if scene := strings.TrimSpace(input.Scene); scene != "" {
		action += "\n【场景】" + scene
	}

	look := resolveLook(character, input.Look)
	refs := characterReferences(character, look)
	if len(refs) == 0 {
		return nil, fmt.Errorf("角色「%s」名下还没有图，请先补定妆稿", character.Name)
	}
	if input.SceneID != "" {
		detail, err := client.ShowSubject(ctx, input.SceneID)
		if err != nil {
			return nil, err
		}
		ref, err := sceneReference(*detail, character.Workspace)
		if err != nil {
			return nil, err
		}
		refs = withSceneReference(refs, ref)
	}

	// 项目哲学：画面口径 + 出片规格（画幅、时长）都从角色所属工作区来。
	var philosophy Philosophy
	if ws, err := client.GetWorkspacePhilosophy(ctx, character.Workspace); err == nil && ws != nil {
		philosophy = ws.Philosophy
	}
	spec := h3Profile.resolve(input.Ratio, input.Duration, philosophy)
	prompt := h3ShotPrompt(character, look, action, spec, refs, philosophy.CompileShot(characterTrait(character, "武器")))
	urls := make([]string, 0, len(refs))
	for _, r := range refs {
		urls = append(urls, r.URL)
	}

	report(Step{ID: "video", Label: "角色出镜", Status: "running", Origin: "cloud-paid", Engine: h3Profile.Model})
	progress(fmt.Sprintf("正在提交 %s 任务（%d 秒 · %s · %d 张参考图，占用套餐额度）", h3Profile.Model, spec.Duration, spec.Ratio, len(urls)))
	url, err := client.GenerateVideo(ctx, h3Profile.Model, prompt, spec.Ratio, spec.Duration, urls)
	if err != nil {
		report(Step{ID: "video", Label: "角色出镜", Status: "failed", Origin: "cloud-paid", Detail: err.Error()})
		return nil, err
	}
	report(Step{ID: "video", Label: "角色出镜", Status: "done", Origin: "cloud-paid", Engine: h3Profile.Model})

	progress("正在下载成片")
	data, mime, err := client.DownloadVideo(ctx, url)
	if err != nil {
		return nil, err
	}
	asset, err := e.store.AddAsset(input.ProjectID, character.Name+"-出镜.mp4", mimeOrVideo(mime), data, map[string]any{
		"source":       "museav",
		"origin":       "cloud",
		"engine":       "museav · " + h3Profile.Model,
		"model":        h3Profile.Model,
		"duration":     spec.Duration,
		"prompt":       prompt,
		"sourceUrl":    url,
		"sourceTaskId": input.SubjectID,
		"script":       input.Action,
		"ratio":        spec.Ratio,
		"look":         look,
	})
	if err != nil {
		return nil, err
	}
	progress("角色出镜完成")
	return asset, nil
}

// 角色 = 不变的本体 × 可换的装扮 × 独立的兵器。
//
// 出镜时一次只能穿一套衣服、拿自己的兵器：把「三视图」和「王服三视图」
// 一起送给模型，它会把两套衣服糅成一套；把别人的剑图一起送，它会把
// 别人的剑画进这个人手里。所以参考图必须按装扮、按兵器过滤。

// DefaultLook 名字里没写装扮的定妆稿（如「夜惊鸿-三视图」）归这一套。
const DefaultLook = "默认"

var sheetKeywords = []string{"三视图", "定妆", "设定", "turnaround"}

// weaponHints 素材名里出现这些字就当兵器图（面具、发饰等不算）。
var weaponHints = []string{"剑", "枪", "弓", "刀", "笛", "鞭", "锤", "匕", "枷", "刃", "刺", "箭"}

func isDesignSheet(name string) bool {
	n := strings.ToLower(name)
	for _, k := range sheetKeywords {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

func isWeaponImage(name string) bool {
	if isDesignSheet(name) {
		return false
	}
	for _, k := range weaponHints {
		if strings.Contains(name, k) {
			return true
		}
	}
	return false
}

// lookOf 从定妆稿名字里取装扮名。
//
// 素材命名有三种形态，都要认：
//
//	「夜惊鸿-王服三视图」   → 王服      （装扮名在类别词前面）
//	「夜惊鸿-三视图-王服」   → 王服      （分类规则产出的形态：角色-类别-描述）
//	「夜惊鸿-三视图」       → 默认      （没写装扮）
//	「夜惊鸿-定妆-夜惊鸿-定妆.png」→ 夜惊鸿-定妆（第三段是完整装扮名）
func lookOf(characterName, assetName string) string {
	n := strings.TrimSpace(assetName)
	// 文件后缀不算名字的一部分
	if i := strings.LastIndex(n, "."); i > 0 {
		if ext := n[i:]; len(ext) <= 5 && !strings.ContainsAny(ext, "/\\") {
			n = n[:i]
		}
	}
	if characterName != "" {
		n = strings.TrimPrefix(n, characterName)
	}
	n = strings.TrimLeft(n, "-_ ·")
	if n == "" {
		return DefaultLook
	}
	// 形态二：角色-类别-描述 → 描述段就是装扮名
	parts := strings.Split(n, "-")
	categoryAt := -1
	for i, part := range parts {
		if isCategoryWord(part) {
			categoryAt = i
			break
		}
	}
	if categoryAt >= 0 && categoryAt+1 < len(parts) {
		rest := strings.Trim(strings.Join(parts[categoryAt+1:], "-"), "-_ ·")
		// 描述段里又带角色名时，去掉重复的角色名
		if characterName != "" {
			rest = strings.TrimPrefix(rest, characterName)
			rest = strings.TrimLeft(rest, "-_ ·")
		}
		if rest == "" {
			return DefaultLook
		}
		return rest
	}
	if categoryAt >= 0 {
		return DefaultLook
	}
	// 形态一：装扮名在类别词之前
	for _, k := range sheetKeywords {
		if i := strings.Index(strings.ToLower(n), k); i >= 0 {
			n = n[:i]
			break
		}
	}
	n = strings.Trim(n, "-_ ·")
	if n == "" {
		return DefaultLook
	}
	return n
}

// isCategoryWord 素材名里的类别段：定妆 / 三视图 / 画面 / 武器 / 头像。
func isCategoryWord(s string) bool {
	switch s {
	case "定妆", "三视图", "五视图", "画面", "武器", "兵器", "道具", "头像", "真人参考", "脸部参考":
		return true
	}
	return false
}

// characterLooks 列出角色名下的几套装扮，「默认」排第一。
func characterLooks(c *Character) []string {
	out := []string{}
	seen := map[string]bool{}
	pool := c.References
	if len(pool) == 0 {
		pool = c.Assets
	}
	for _, a := range pool {
		if a.View != "" || !isDesignSheet(a.Name) {
			continue
		}
		l := lookOf(c.Name, a.Name)
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	for i, l := range out {
		if l == DefaultLook && i > 0 {
			out[0], out[i] = out[i], out[0]
		}
	}
	return out
}

// resolveLook 用户没选或选了不存在的装扮时，落到第一套。
func resolveLook(c *Character, look string) string {
	looks := characterLooks(c)
	for _, l := range looks {
		if l == look {
			return l
		}
	}
	if len(looks) > 0 {
		return looks[0]
	}
	return ""
}

// weaponNames 从角色卡「武器」里取【】内的兵器名。
func weaponNames(weapon string) []string {
	out := []string{}
	for {
		i := strings.Index(weapon, "【")
		if i < 0 {
			return out
		}
		rest := weapon[i+len("【"):]
		j := strings.Index(rest, "】")
		if j < 0 {
			return out
		}
		if name := strings.TrimSpace(rest[:j]); name != "" {
			out = append(out, name)
		}
		weapon = rest[j+len("】"):]
	}
}

// characterReferenceURLs 挑出镜参考图：
//  1. 归位的五视图（同一批生成，同一套装扮）
//  2. 选定装扮的定妆稿 —— 只这一套，别的装扮一张不送
//  3. 本命兵器图 —— 名字对上角色卡【兵器名】的；对不上且只有一张兵器图时用它
//  4. 其余道具（面具等）
//
// 同一 URL 只出现一次，上限 9 张（中台限制）。
func characterReferenceURLs(c *Character, look string) []string {
	out := []string{}
	for _, r := range characterReferences(c, look) {
		out = append(out, r.URL)
	}
	return out
}

// shotRef 一张出镜参考图 + 它在提示词里的分工（H3 按「图1、图2…」认图）。
type shotRef struct {
	URL   string
	Label string
}

func characterReferences(c *Character, look string) []shotRef {
	out := []shotRef{}
	seen := map[string]bool{}
	label := ""
	add := func(u string) {
		if u == "" || seen[u] || len(out) >= 9 {
			return
		}
		seen[u] = true
		out = append(out, shotRef{URL: u, Label: label})
	}
	for _, view := range CharacterViews {
		for _, a := range c.Assets {
			if a.View == view {
				label = c.Name + "的" + CharacterViewLabels[view] + "定妆，锁定五官、发型、肤色、体型与服装"
				add(a.URL)
				break
			}
		}
	}
	pool := c.References
	if len(pool) == 0 {
		pool = c.Assets
	}
	look = resolveLook(c, look)
	for _, a := range pool {
		if a.View == "" && isDesignSheet(a.Name) && lookOf(c.Name, a.Name) == look {
			label = c.Name + "的定妆三视图（" + look + "装扮），锁定五官、发型、肤色、体型与这一套服装"
			add(a.URL)
		}
	}
	names := weaponNames(characterTrait(c, "武器"))
	weapons := []CharacterAsset{}
	matched := []CharacterAsset{}
	for _, a := range pool {
		if a.View != "" || !isWeaponImage(a.Name) {
			continue
		}
		weapons = append(weapons, a)
		for _, n := range names {
			if strings.Contains(a.Name, n) {
				matched = append(matched, a)
				break
			}
		}
	}
	if len(matched) == 0 && len(weapons) == 1 {
		matched = weapons
	}
	for _, a := range matched {
		label = c.Name + "的本命兵器（" + strings.TrimLeft(strings.TrimPrefix(a.Name, c.Name), "-_ ·") + "），锁定兵器形制与材质"
		add(a.URL)
	}
	for _, a := range pool {
		if a.View == "" && !isDesignSheet(a.Name) && !isWeaponImage(a.Name) {
			label = c.Name + "的随身道具（" + strings.TrimLeft(strings.TrimPrefix(a.Name, c.Name), "-_ ·") + "）"
			add(a.URL)
		}
	}
	return out
}

// mimeOrVideo 下载回来的 mime 可能是二进制流嗅探结果，
// 视频一律按 mp4 落库，免得素材列表出现 application/octet-stream。
func mimeOrVideo(mime string) string {
	if strings.HasPrefix(mime, "video/") {
		return mime
	}
	return "video/mp4"
}

// characterTrait 取角色卡里某一项体貌/设定。
func characterTrait(c *Character, key string) string {
	for _, t := range c.Traits {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}
