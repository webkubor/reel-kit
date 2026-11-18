package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ──────────────────────────────────────────────────────────────
// 角色素材本地缓存
//
// 中台是真源，但角色库不该每次打开都逐个打网络：
//  1. 一次 RefreshCharacterCache 把每个角色的素材下载进本地素材库
//     （media 目录 + studio.json 账本，头像/定妆/三视图/武器全部）
//  2. 之后 ListCharacterPortraits 全部读本地缓存，一次返回所有头像 URL
//  3. 断网也能看到头像；头像 URL 用 /media/<id>，Wails 从本地磁盘出图
// ──────────────────────────────────────────────────────────────

// CharacterPortrait 角色列表里的一格（头像 + 名字 + 素材数）。
//
// ReferenceCount 只算角色素材（头像/定妆/三视图），WeaponCount 单独算
// 兵器道具——列表上「角色素材」和「兵器」分开显示，不混在一起。
type CharacterPortrait struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	HasAvatar      bool   `json:"hasAvatar"`
	AvatarURL      string `json:"avatarUrl,omitempty"` // /media/<id>（本地）或中台直链
	ReferenceCount int    `json:"referenceCount"`
	WeaponCount    int    `json:"weaponCount"`
	// DesignCount 定妆/三视图张数（角色形象本身）。
	DesignCount int `json:"designCount"`
	// RealFaceCount 真人参考张数（非角色形象，只是建模参考）。
	RealFaceCount int    `json:"realFaceCount"`
	MirrorLocal   bool   `json:"mirrorLocal"` // 素材是否已缓存到本地
	Workspace     string `json:"workspace,omitempty"`
}

// CharacterMirror 角色的本地镜像。
//
// 中台是真源，本地这份是为了「打开就有、断了网也有、导得出」。
// 从 1.0 起：References 里每张图（头像/定妆/三视图/武器）都登记进本地
// 素材库并记下 assetID，角色库列表不再依赖中台网络。
type CharacterMirror struct {
	ID        string            `json:"id"`
	SubjectID string            `json:"subjectId"`
	Name      string            `json:"name"`
	Persona   string            `json:"persona"`
	Traits    []CharacterTrait  `json:"traits"`
	Complete  bool              `json:"complete"`
	Missing   []string          `json:"missing,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
	Views     map[string]string `json:"views"` // view -> 本地文件路径
	// Assets 本地登记的素材（assetID + 原素材信息）。
	Assets []MirrorAsset `json:"assets,omitempty"`
	// SourceBatch 五视图的同源批次标记。
	SourceBatch string `json:"sourceBatch,omitempty"`
	UpdatedAt   string `json:"updatedAt"`
}

// MirrorAsset 一张已落到本地的角色素材。
type MirrorAsset struct {
	AssetID string `json:"assetId"`
	Name    string `json:"name"`
	Role    string `json:"role,omitempty"`
	URL     string `json:"url,omitempty"` // 中台直链（记录来源）
}

func (e *Engine) characterMirrorDir() string {
	return filepath.Join(e.store.Root(), "characters")
}

// mirrorCharacterImage 把中台直链的角色图存一份到本地。
//
// 为什么要存：界面上的角色卡不能只靠远程直链 —— 断了网就白板，
// 而且用户导出的片子里的角色图也得跟着走。
func (e *Engine) mirrorCharacterImage(ctx context.Context, url, name string) (string, error) {
	client := &MuseavClient{}
	data, mime, err := client.DownloadImage(ctx, url)
	if err != nil {
		return "", err
	}
	dir := e.characterMirrorDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	ext := map[string]string{
		"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif",
	}[mime]
	if ext == "" {
		ext = ".png"
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s%s", newID(), sanitizeFileName(name), ext))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// sanitizeFileName 把名字里的路径分隔符换掉，避免拼出目录外的路径。
func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "")
	out := replacer.Replace(strings.TrimSpace(name))
	if out == "" {
		out = "image"
	}
	if len([]rune(out)) > 40 {
		out = string([]rune(out)[:40])
	}
	return out
}

// SaveCharacterMirror 存角色的本地镜像。
// RenameMirrorAssetsToCurrentName 把镜像里不属于当前角色名的素材改挂到当前名。
//
// 角色改名后素材名常留着旧名（「顾七玥-玉尘笛」挂在「顾栖月」名下）：
// 按新名搜不到这把笛子，分类也容易落到错的角色上。
// 这里只改「角色名-类别-描述」这种结构的前缀；纯描述名（如「玉尘笛」）不动 ——
// 猜不出它原本是谁的，硬改会张冠李戴。
func renameMirrorAssetsToCurrentName(mirror *CharacterMirror, currentName string) int {
	if mirror == nil || currentName == "" {
		return 0
	}
	changed := 0
	for i := range mirror.Assets {
		n := mirror.Assets[i].Name
		if n == "" || strings.HasPrefix(n, currentName+"-") {
			continue
		}
		sep := strings.Index(n, "-")
		if sep <= 0 {
			continue
		}
		// 镜像里的素材本来就属于这个角色，改前缀就行；
		// 唯一要排除的是场景参考 —— 那不是角色物料。
		rest := n[sep+1:]
		if strings.Contains(rest, "场景") || strings.Contains(rest, "画面") {
			continue
		}
		mirror.Assets[i].Name = currentName + n[sep:]
		changed++
	}
	return changed
}

// SaveCharacterMirror 缓存一份角色详情到本机。
func (e *Engine) SaveCharacterMirror(character *Character) error {
	if e.store == nil {
		return errNoStore
	}
	dir := e.characterMirrorDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	mirror := &CharacterMirror{
		ID:          character.ID,
		SubjectID:   character.ID,
		Name:        character.Name,
		Persona:     character.Persona,
		Traits:      character.Traits,
		Complete:    character.Complete,
		Missing:     character.Missing,
		Warnings:    character.Warnings,
		Views:       map[string]string{},
		SourceBatch: character.SourceBatch,
		UpdatedAt:   nowISO(),
	}
	for _, a := range character.Assets {
		local, err := e.mirrorCharacterImage(context.Background(), a.URL, string(a.View))
		if err != nil {
			continue
		}
		mirror.Views[string(a.View)] = local
	}
	// 参考素材也要记下：装扮名（「默认」以外的几套）来自定妆稿的名字，
	// 只存视图的话离线就列不出装扮。
	for _, a := range character.References {
		if a.View != "" || a.Name == "" {
			continue
		}
		mirror.Assets = append(mirror.Assets, MirrorAsset{
			AssetID: a.ID, Role: a.Role, Name: a.Name, URL: a.URL,
		})
	}
	data, err := json.MarshalIndent(mirror, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, sanitizeFileName(character.Name)+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

// sourceBatchFor 从本地镜像里读某个主体的同源批次标记。
func (e *Engine) sourceBatchFor(subjectID string) string {
	mirrors, err := e.ListCharacterMirrors()
	if err != nil {
		return ""
	}
	for _, m := range mirrors {
		if m.SubjectID == subjectID {
			return m.SourceBatch
		}
	}
	return ""
}

// ListCharacterMirrors 列本地镜像的角色。
func (e *Engine) ListCharacterMirrors() ([]*CharacterMirror, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	dir := e.characterMirrorDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*CharacterMirror{}, nil
		}
		return nil, err
	}
	out := []*CharacterMirror{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var mirror CharacterMirror
		if err := json.Unmarshal(data, &mirror); err != nil {
			continue
		}
		out = append(out, &mirror)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// mirrorBySubjectID 找某个主体的本地镜像。
func (e *Engine) mirrorBySubjectID(subjectID string) (*CharacterMirror, bool) {
	mirrors, err := e.ListCharacterMirrors()
	if err != nil {
		return nil, false
	}
	for _, m := range mirrors {
		if m.SubjectID == subjectID {
			return m, true
		}
	}
	return nil, false
}

// FixMirrorAssetNames 按角色当前名修正「这个角色自己」的镜像素材名。
//
// 角色改名后素材名常留着旧名（「顾七玥-玉尘笛」挂在「顾栖月」名下）：
// 按新名搜不到，分类也容易落到错的角色上。
//
// 只动这一个角色的镜像 —— 拿当前名去改别人的素材会把整个素材库搞乱。
func (e *Engine) FixMirrorAssetNames(subjectID, currentName string) (int, error) {
	if e.store == nil || currentName == "" {
		return 0, errNoStore
	}
	m, ok := e.mirrorBySubjectID(subjectID)
	if !ok {
		return 0, nil
	}
	// 先记下旧名，改完镜像才知道素材库那边该改哪些
	oldOwners := map[string]bool{}
	for _, a := range m.Assets {
		sep := strings.Index(a.Name, "-")
		if sep > 0 && a.Name[:sep] != currentName {
			oldOwners[a.Name[:sep]] = true
		}
	}
	n := renameMirrorAssetsToCurrentName(m, currentName)
	if n > 0 {
		if err := e.saveMirror(m); err != nil {
			return 0, err
		}
	}
	// 素材库里的同名素材也要改，否则按角色名过滤会漏。
	// 用 assetId 精确对应 —— 两边名字本来就不一致，靠名字猜不准。
	names := map[string]string{}
	for _, a := range m.Assets {
		if a.AssetID != "" {
			names[a.AssetID] = a.Name
		}
	}
	if _, err := e.store.SyncAssetNames(names); err != nil {
		return n, err
	}
	if len(oldOwners) > 0 {
		if _, err := e.store.RenameAssetOwner(oldOwners, currentName); err != nil {
			return n, err
		}
	}
	return n, nil
}

// saveMirror 把镜像写回本机。
func (e *Engine) saveMirror(m *CharacterMirror) error {
	if e.store == nil || m == nil {
		return errNoStore
	}
	dir := e.characterMirrorDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	m.UpdatedAt = nowISO()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, sanitizeFileName(m.Name)+".json"), data, 0o600)
}

// CharacterDetailCache 角色详情的本地缓存目录。
func (e *Engine) characterDetailCacheDir() string {
	return filepath.Join(e.store.Root(), "character-details")
}

// SaveCharacterDetail 缓存一次角色详情。
//
// 中台读一个没素材的角色要 27 秒（实测），点开详情时干等太久。
// 读过一次就存到本地，之后秒开；素材变了再刷新（RefreshCharacterCache 会清）。
func (e *Engine) SaveCharacterDetail(subjectID string, c *Character) error {
	if e.store == nil || c == nil {
		return errNoStore
	}
	dir := e.characterDetailCacheDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, subjectID+".json"), data, 0o600)
}

// LoadCharacterDetail 读缓存的角色详情。
func (e *Engine) LoadCharacterDetail(subjectID string) (*Character, bool) {
	if e.store == nil || strings.ContainsAny(subjectID, `/\\.`) {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(e.characterDetailCacheDir(), subjectID+".json"))
	if err != nil {
		return nil, false
	}
	var c Character
	if json.Unmarshal(data, &c) != nil {
		return nil, false
	}
	return &c, true
}

// RefreshCharacterCache 把项目所有角色的素材下载并登记进本地素材库。
//
// 返回每个角色「本地化了几张、失败了几张」的报告；失败不阻断（网络抖动
// 或某张图损坏时，其余素材照常缓存）。
func (e *Engine) RefreshCharacterCache(projectID string) ([]CharacterPortrait, error) {
	ws, err := e.projectWorkspace(projectID)
	if err != nil {
		return nil, err
	}
	subjects, err := e.ListAllSubjects(ws)
	if err != nil {
		return nil, err
	}
	client := &MuseavClient{Workspace: ws}
	// 拉全部角色详情 + 下载素材，60 秒的常规请求超时不够；缓存用独立的宽超时。
	ctx, cancel := context.WithTimeout(context.Background(), characterVideoTimeout)
	defer cancel()

	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	out := make([]CharacterPortrait, 0, len(subjects))
	for _, sub := range subjects {
		if sub.Kind != "person" {
			continue
		}
		wg.Add(1)
		go func(s Subject) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			portrait, err := e.cacheOneCharacter(ctx, client, s, projectID)
			if err == nil && portrait != nil {
				out = append(out, *portrait)
			}
		}(*sub)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// cacheOneCharacter 缓存一个角色的素材到本地素材库，返回列表用的头像格。
func (e *Engine) cacheOneCharacter(ctx context.Context, client *MuseavClient, sub Subject, projectID string) (*CharacterPortrait, error) {
	detail, err := client.ShowSubject(ctx, sub.ID)
	if err != nil {
		fmt.Printf("[cache] SHOW FAIL %s: %v\n", sub.Name, err)
		return nil, err
	}
	character := subjectToCharacter(*detail, projectID, e.sourceBatchFor(sub.ID))
	refCount, weaponCount := 0, 0
	for _, ref := range character.References {
		if isWeaponAsset(ref.Name) || isWeaponByTraits(ref.Name, sub.Traits) {
			weaponCount++
		} else {
			refCount++
		}
	}
	portrait := &CharacterPortrait{ID: sub.ID, Name: character.Name, Workspace: client.Workspace, ReferenceCount: refCount, WeaponCount: weaponCount}
	// 登记每一张参考图进本地素材库（头像/定妆/三视图/武器）。
	mirrorAssets := []MirrorAsset{}
	for _, ref := range character.References {
		if ref.URL == "" {
			continue
		}
		data, mime, err := (&MuseavClient{}).DownloadImage(ctx, ref.URL)
		if err != nil {
			continue
		}
		asset, err := e.store.AddAsset(projectID, ref.Name, mime, data, map[string]any{
			"source": "museav", "sourceUrl": ref.URL, "museavSubjectId": sub.ID,
			"museavSubjectName": sub.Name, "museavRole": ref.Role,
		})
		if err != nil {
			continue
		}
		mirrorAssets = append(mirrorAssets, MirrorAsset{AssetID: asset.ID, Name: ref.Name, Role: ref.Role, URL: ref.URL})
		if isAvatarAsset(ref.Name) && !isRealFaceAsset(ref.Name) && !portrait.HasAvatar {
			portrait.HasAvatar = true
			portrait.AvatarURL = "/media/" + asset.ID
		}
	}
	// 头像兜底：没有「-头像」命名就取第一张非兵器的参考图。
	if !portrait.HasAvatar {
		for _, ref := range character.References {
			if isWeaponAsset(ref.Name) || isWeaponByTraits(ref.Name, sub.Traits) {
				continue
			}
			asset := findMirrorAsset(mirrorAssets, ref.Name)
			if asset != "" {
				portrait.HasAvatar = true
				portrait.AvatarURL = "/media/" + asset
				break
			}
		}
	}
	portrait.MirrorLocal = len(mirrorAssets) > 0
	// 写镜像 JSON（记录 assetID，供 ListCharacterPortraits 直接读）。
	e.writeMirror(character, mirrorAssets)
	return portrait, nil
}

// assetName 取素材在本机素材库里的实际名字；找不到就用传入的名字。
func (e *Engine) assetName(assetID, fallback string) string {
	if e.store == nil || assetID == "" {
		return fallback
	}
	a, err := e.store.Asset(assetID, "")
	if err != nil || a == nil {
		return fallback
	}
	if a.Name != "" {
		return a.Name
	}
	return fallback
}

// isCostumeAsset 定妆立绘：单人的全身/半身立绘，可以当头像。
func isCostumeAsset(name string) bool {
	return strings.Contains(name, "定妆") || strings.Contains(name, "立绘")
}

func isAvatarAsset(name string) bool { return strings.Contains(name, "头像") }

// weaponNamesFromTraits 从角色的「武器」设定里取出兵器名。
//
// 中台写法是「重刃【破军】，门板重刃焦黑崩口」——方括号里是兵器本名。
// 素材名常直接用这个本名（"欧阳狂徒-破军"），靠通用关键词猜不靠谱
// （"破军"不在任何关键词表里），但设定里有。
func weaponNamesFromTraits(traits map[string]any) []string {
	raw, ok := traits["武器"]
	if !ok {
		return nil
	}
	text := traitText(raw)
	out := []string{}
	for _, seg := range strings.FieldsFunc(text, func(r rune) bool {
		return r == '，' || r == ',' || r == '；' || r == ';' || r == '、'
	}) {
		// 用 rune 切：「【」「】」是多字节字符，按字节下标切会截出乱码
		runes := []rune(seg)
		open := -1
		for i, r := range runes {
			if r == '【' {
				open = i
				break
			}
		}
		if open >= 0 {
			close := -1
			for i := open + 1; i < len(runes); i++ {
				if runes[i] == '】' {
					close = i
					break
				}
			}
			if close > open {
				name := strings.TrimSpace(string(runes[open+1 : close]))
				if name != "" {
					out = append(out, name)
				}
			}
		}
	}
	return out
}

// isWeaponByTraits 素材名里含角色自己的兵器名。
func isWeaponByTraits(name string, traits map[string]any) bool {
	for _, w := range weaponNamesFromTraits(traits) {
		if w != "" && strings.Contains(name, w) {
			return true
		}
	}
	return false
}

// isWeaponAsset 兵器与道具。
//
// 三视图的定妆稿里常带兵器名（「顾栖月-三视图-顾栖月_v1_玉笛」），
// 那张图是角色三视图而不是兵器图 —— 带「三视图/定妆/设定」的先归定妆。
func isWeaponAsset(name string) bool {
	if isDesignSheet(name) {
		return false
	}
	return strings.Contains(name, "武器") || strings.Contains(name, "面具") || strings.Contains(name, "兵器") ||
		strings.Contains(name, "枪") || strings.Contains(name, "剑") || strings.Contains(name, "刀") ||
		strings.Contains(name, "弓") || strings.Contains(name, "笛") || strings.Contains(name, "锤") ||
		strings.Contains(name, "鞭") || strings.Contains(name, "匕") || strings.Contains(name, "枷")
}

// isRealFaceAsset 真人参考。
//
// 发型/妆造参考也是真人照片（陆听潮-纯黑发参考），不能漏进画面素材。
func isRealFaceAsset(name string) bool {
	return strings.Contains(name, "真人参考") || strings.Contains(name, "脸部参考") ||
		strings.Contains(name, "真人") || strings.Contains(name, "脸参考") ||
		strings.Contains(name, "发参考") || strings.Contains(name, "发型参考") ||
		strings.Contains(name, "妆造参考") || strings.Contains(name, "妆参考") ||
		strings.Contains(name, "face")
}

func findMirrorAsset(assets []MirrorAsset, name string) string {
	for _, a := range assets {
		if a.Name == name {
			return a.AssetID
		}
	}
	return ""
}

func (e *Engine) writeMirror(character *Character, assets []MirrorAsset) {
	dir := e.characterMirrorDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	mirror := &CharacterMirror{
		ID: character.ID, SubjectID: character.ID, Name: character.Name, Persona: character.Persona,
		Traits: character.Traits, Complete: character.Complete, Missing: character.Missing,
		Warnings: character.Warnings, Views: map[string]string{}, Assets: assets,
		SourceBatch: character.SourceBatch, UpdatedAt: nowISO(),
	}
	for _, a := range character.Assets {
		mirror.Views[string(a.View)] = a.URL
	}
	data, err := json.MarshalIndent(mirror, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, sanitizeFileName(character.Name)+".json"), data, 0o600)
}

// ListCharacterPortraits 一次返回项目全部角色的头像格。
//
// 优先读本地镜像（素材已缓存 → 头像 URL 是 /media/<id>，断网也能显示）；
// 没有本地镜像的角色返回 hasAvatar=false，界面显示「待定头像」。
func (e *Engine) ListCharacterPortraits(projectID string) ([]CharacterPortrait, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	ws, err := e.projectWorkspace(projectID)
	if err != nil {
		return nil, err
	}
	subjects, err := e.ListAllSubjects(ws)
	if err != nil {
		return nil, err
	}
	out := []CharacterPortrait{}
	for _, sub := range subjects {
		if sub.Kind != "person" {
			continue
		}
		p := CharacterPortrait{ID: sub.ID, Name: sub.Name, Workspace: ws}
		if mirror, ok := e.mirrorBySubjectID(sub.ID); ok {
			p.MirrorLocal = true
			for _, a := range mirror.Assets {
				real := e.assetName(a.AssetID, a.Name)
				switch {
				case isWeaponAsset(real) || isWeaponByTraits(real, sub.Traits):
					p.WeaponCount++
				case isRealFaceAsset(real):
					p.RealFaceCount++
					p.ReferenceCount++
				default:
					p.DesignCount++
					p.ReferenceCount++
				}
			}
			// 判断用「素材库里的实际名字」，不用镜像里记的名字：
			// 两者可能对不上（镜像写「陆听潮-头像」，素材库里那张图其实叫
			// 「陆听潮-真人参考-头.jpg」）。按镜像的名字判会以为那是头像，
			// 结果把真人照片摆到角色列表上。
			for _, a := range mirror.Assets {
				real := e.assetName(a.AssetID, a.Name)
				if isAvatarAsset(real) && !isRealFaceAsset(real) {
					p.HasAvatar = true
					p.AvatarURL = "/media/" + a.AssetID
					break
				}
			}
			if !p.HasAvatar {
				// 回退到「定妆立绘」：三视图是拼图（一张图里几个视角），
				// 拿它当头像会显示成半个人或并排三个人，不对。
				for _, a := range mirror.Assets {
					real := e.assetName(a.AssetID, a.Name)
					if isCostumeAsset(real) && !isRealFaceAsset(real) {
						p.HasAvatar = true
						p.AvatarURL = "/media/" + a.AssetID
						break
					}
				}
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

var _ = errors.New
