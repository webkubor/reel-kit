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
// 配音、字幕、合成是 reel-kit 的活。这里只做「让角色出镜」这一步。

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

	// 参考图：五视图优先（模型能拼出转面），没有就退而求其次用名下任意图。
	refs := characterReferenceURLs(character)
	if len(refs) == 0 {
		return nil, fmt.Errorf("角色「%s」名下还没有图，请先补定妆稿", character.Name)
	}
	if len(refs) > 9 {
		refs = refs[:9] // 中台上限 9 张
	}

	report(Step{ID: "video", Label: "角色出镜", Status: "running", Origin: "cloud-paid", Engine: "museav 视频出片"})
	progress(fmt.Sprintf("正在提交视频任务（计费 %d 张参考图）", len(refs)))
	prompt := characterVideoPrompt(character, input.Action, e.shotPhilosophyText(ctx, character.Workspace, characterTrait(character, "武器")))
	url, err := client.GenerateVideo(ctx, prompt, input.Ratio, input.Duration, refs)
	if err != nil {
		report(Step{ID: "video", Label: "角色出镜", Status: "failed", Origin: "cloud-paid", Detail: err.Error()})
		return nil, err
	}
	report(Step{ID: "video", Label: "角色出镜", Status: "done", Origin: "cloud-paid", Engine: "museav 视频出片"})

	progress("正在下载成片")
	data, mime, err := client.DownloadVideo(ctx, url)
	if err != nil {
		return nil, err
	}
	asset, err := e.store.AddAsset(input.ProjectID, character.Name+"-出镜.mp4", mimeOrVideo(mime), data, map[string]any{
		"source":       "museav",
		"origin":       "cloud",
		"engine":       "museav 视频出片",
		"sourceUrl":    url,
		"sourceTaskId": input.SubjectID,
		"script":       input.Action,
		"ratio":        input.Ratio,
	})
	if err != nil {
		return nil, err
	}
	progress("角色出镜完成")
	return asset, nil
}

// characterReferenceURLs 挑参考图，按「锁脸价值」排序：
//  1. 归位的五视图（按正→左45→侧→右45→背）
//  2. 名字带「三视图 / 定妆 / 设定」的组合稿 —— 设计师定稿，最稳
//  3. 其余（兵器、面具等道具图）—— 有助于道具一致，但不锁脸
//
// 同一 URL 只出现一次，上限 9 张（中台限制）。
func characterReferenceURLs(c *Character) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(u string) {
		if u == "" || seen[u] || len(out) >= 9 {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	for _, view := range CharacterViews {
		for _, a := range c.Assets {
			if a.View == view {
				add(a.URL)
				break
			}
		}
	}
	pool := c.References
	if len(pool) == 0 {
		pool = c.Assets
	}
	for _, a := range pool {
		if isDesignSheet(a.Name) {
			add(a.URL)
		}
	}
	for _, a := range pool {
		add(a.URL)
	}
	return out
}

func isDesignSheet(name string) bool {
	for _, k := range []string{"三视图", "定妆", "设定", "turnaround"} {
		if strings.Contains(strings.ToLower(name), k) {
			return true
		}
	}
	return false
}

// characterVideoPrompt 组装出镜提示词：角色卡管「这个人长什么样」，
// 动作管「这场戏演什么」，哲学管「这类画面长什么样」。
func characterVideoPrompt(c *Character, action, philosophy string) string {
	parts := []string{action}
	traits := []string{}
	for _, t := range c.Traits {
		traits = append(traits, t.Key+"："+t.Value)
	}
	if len(traits) > 0 {
		parts = append(parts, "人物设定："+strings.Join(traits, "；")+"。参考图锁定五官、发型、肤色与服装，不得改变。")
	} else {
		parts = append(parts, "人物外貌以参考图为准，不得改变。")
	}
	if strings.TrimSpace(philosophy) != "" {
		parts = append(parts, philosophy)
	}
	return strings.Join(parts, "\n")
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
