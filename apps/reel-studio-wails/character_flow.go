package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 角色 0→1 创建流程：一张脸 → 一个可用的角色。
//
// 用户只需要给一张人脸，其余都由流程补齐。但「补齐」不能糊弄：
// 五视图必须同源（同一次定妆产出），否则角色换个镜头就换脸。
//
// 所以流程用**一次批量出图**出五张视图，而不是分五次调单张出图 ——
// 分五次出必然各生成各的，脸会漂。批量提交是一次定妆，五张天然同源。
//
// 中台的两条硬约束（见 docs.museav.top/cli/advanced）：
//   1. 同一张图不能注册两次 —— 五张必须是五张不同的图；
//   2. 主体和它的图必须在同一个工作区里。

// CharacterCreateInput 建角色的输入。
type CharacterCreateInput struct {
	ProjectID string `json:"projectId"`
	// Workspace 中台工作区（角色的归属地）。空 = 个人主体。
	Workspace string `json:"workspace"`
	Name      string `json:"name"`
	Persona   string `json:"persona"`
	// Traits 体貌特征。至少要有发型/脸型/肤色/体型四项。
	Traits map[string]string `json:"traits"`
	// FacePath 用户上传的人脸（本地路径）。
	FacePath string `json:"facePath"`
	// Outfit 服装描述，参与出图提示词。
	Outfit string `json:"outfit"`
	// Ratio 出图画幅，默认 3:4。
	Ratio string `json:"ratio"`
}

// CharacterResult 建角色的结果。
type CharacterResult struct {
	Character *Character `json:"character"`
	// Steps 每一步发生了什么（给界面显示进度与留痕）。
	Steps []string `json:"steps"`
}

// CreateCharacter 走完 0→1 建角色。
//
// 顺序：建主体 → 登记基准人脸 → 一次批量出五视图 → 逐个登记 → 重新读回算完成度。
//
// 最后一步「重新读回」是故意的：完成度必须按中台真实状态算，
// 不能按「我刚发了 5 个请求」算 —— 中台可能拒了其中一张。
func (e *Engine) CreateCharacter(ctx context.Context, input CharacterCreateInput, progress ProgressFunc) (*CharacterResult, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("角色名不能为空")
	}
	persona := strings.TrimSpace(input.Persona)
	if persona != "real" && persona != "virtual" {
		return nil, errors.New("形象来源只能是「真人」或「虚拟人」")
	}
	if strings.TrimSpace(input.FacePath) == "" {
		return nil, errors.New("先上传一张人脸，再创建角色")
	}
	if _, err := os.Stat(input.FacePath); err != nil {
		return nil, errors.New("人脸文件读不到，请重新上传")
	}
	if missing := missingTraitKeys(input.Traits); len(missing) > 0 {
		return nil, fmt.Errorf("体貌还缺：%s", strings.Join(missing, "、"))
	}

	client := &MuseavClient{Workspace: strings.TrimSpace(input.Workspace)}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}

	result := &CharacterResult{Steps: []string{}}
	step := func(format string, args ...any) {
		result.Steps = append(result.Steps, fmt.Sprintf(format, args...))
	}

	// 同源批次标记：这一轮 0→1 产出的五视图共享它。
	// 平台的 gen_job_id 是逐图的（批量每行一个 job），判不了同源；
	// 只有发起这一轮的人知道「这五张是照着同一张基准脸、同一轮生成的」。
	// 所以由本地打标并存进镜像，读取时回填。
	sourceBatch := newID()

	// ── 1. 建主体 ──
	if progress != nil {
		progress("正在中台建立角色档案…")
	}
	subject, err := client.CreateSubject(ctx, name, "person", persona, input.Traits)
	if err != nil {
		return nil, err
	}
	step("已建立角色档案：%s", subject.Name)

	// ── 2. 登记基准人脸 ──
	// 基准人脸是「这个角色长什么样」的唯一依据，后面五张视图都以它为参考。
	if progress != nil {
		progress("正在登记基准人脸…")
	}
	if err := client.AddSubjectAsset(ctx, subject.ID, "", "基准人脸", input.FacePath, ""); err != nil {
		return nil, fmt.Errorf("基准人脸登记失败：%w", err)
	}
	step("已登记基准人脸")

	// ── 3. 一次批量出五视图 ──
	if progress != nil {
		progress("正在生成五视图（同一次定妆）…")
	}
	urls, err := e.generateCharacterViews(ctx, client, input, subject)
	if err != nil {
		return nil, err
	}
	step("已生成 %d 张视图", len(urls))

	// ── 4. 逐个登记 ──
	if progress != nil {
		progress("正在把视图挂到角色名下…")
	}
	for i, view := range CharacterViews {
		if i >= len(urls) || urls[i] == "" {
			step("视图 %s 没生成出来", CharacterViewLabels[view])
			continue
		}
		assetName := fmt.Sprintf("%s-%s", name, CharacterViewLabels[view])
		if err := client.AddSubjectAsset(ctx, subject.ID, view, assetName, "", urls[i]); err != nil {
			step("视图 %s 登记失败：%v", CharacterViewLabels[view], err)
			continue
		}
		step("已登记视图 %s", CharacterViewLabels[view])
	}

	// ── 5. 读回，算完成度 ──
	if progress != nil {
		progress("正在核对角色完成度…")
	}
	detail, err := client.ShowSubject(ctx, subject.ID)
	if err != nil {
		return nil, err
	}
	character := subjectToCharacter(*detail, input.ProjectID, sourceBatch)
	result.Character = character
	if err := e.SaveCharacterMirror(character); err != nil {
		step("本地镜像保存失败（不影响中台角色）：%v", err)
	}
	if character.Complete {
		step("角色创建完成")
	} else {
		step("角色还差：%s", strings.Join(character.Missing, "、"))
	}
	return result, nil
}

// generateCharacterViews 一次批量出五视图。
//
// 关键：五张走**同一次批量提交**，中台按同一批排队消化。
// 分五次单独调出图必然各生成各的 —— 角色换个镜头就换脸，
// 这正是「五视图同源」要防的事。
func (e *Engine) generateCharacterViews(ctx context.Context, client *MuseavClient, input CharacterCreateInput, subject *Subject) ([]string, error) {
	ratio := strings.TrimSpace(input.Ratio)
	if ratio == "" {
		ratio = "3:4"
	}
	// 批量文件：每行一条完整提示词，按五视图顺序排。
	lines := []string{}
	for _, view := range CharacterViews {
		lines = append(lines, characterViewPrompt(subject, input, view))
	}
	temp, err := os.CreateTemp("", "reel-character-views-*.txt")
	if err != nil {
		return nil, err
	}
	defer os.Remove(temp.Name())
	for _, line := range lines {
		if _, err := temp.WriteString(line + "\n"); err != nil {
			temp.Close()
			return nil, err
		}
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}

	out, err := client.run(ctx, "gen", "--batch", temp.Name(), "--ratio", ratio, "--ref", input.FacePath)
	if err != nil {
		return nil, err
	}
	// 批量模式按行序输出直链，每行一个。
	urls := []string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			urls = append(urls, line)
		}
	}
	if len(urls) == 0 {
		return nil, errors.New("五视图没生成出来")
	}
	return urls, nil
}

// characterViewPrompt 出某个视图的提示词。
//
// 按模板的写法：以参考图为「脸部、骨骼结构、肤质与发型」的唯一依据，
// 白底、全身、正交机位、同一套装扮，逐项锁死不变的部分。
func characterViewPrompt(subject *Subject, input CharacterCreateInput, view CharacterView) string {
	traits := subjectTraits(subject.Traits)
	traitText := []string{}
	for _, t := range traits {
		traitText = append(traitText, t.Key+"："+t.Value)
	}
	angle := map[CharacterView]string{
		ViewFront:   "正对镜头的正面全身站姿",
		ViewLeft45:  "向左转 45 度的全身站姿",
		ViewSide:    "正侧面（90 度）全身站姿",
		ViewRight45: "向右转 45 度的全身站姿",
		ViewBack:    "背对镜头的背面全身站姿",
	}[view]

	parts := []string{
		"生成一张写实摄影风格的人物角色设定视图。",
		"参考图是同一个人的脸部、骨骼结构、肤质与发型，五官必须高度匹配参考图，保持年龄、种族、性别特征一致。",
		"机位与姿态：" + angle + "。",
		"纯白背景，柔光棚拍，光线均匀，无投影干扰。",
		"全身入镜，从头顶到脚完整可见，不裁切。",
		"人物体貌：" + strings.Join(traitText, "，") + "。",
	}
	if outfit := strings.TrimSpace(input.Outfit); outfit != "" {
		parts = append(parts, "服装设定："+outfit+"。")
	}
	parts = append(parts,
		"该视图与同一角色的其余视图必须完全一致：五官、发型、肤色、身材、服装、鞋靴、配饰、光影与材质都不得改变，不得出现角色漂移。",
		"画面里只有这一个人物，不要文字标注、不要分格、不要拼图。",
	)
	return strings.Join(parts, "\n")
}

// missingTraitKeys 报出还缺的体貌项。
func missingTraitKeys(traits map[string]string) []string {
	missing := []string{}
	for _, key := range RequiredTraitKeys {
		if strings.TrimSpace(traits[key]) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

// ListCharacters 列中台角色。
//
// 读的是中台主体列表，逐个拉详情算完成度 —— 列表接口不给名下的图，
// 只有详情接口给。
func (e *Engine) ListCharacters(ctx context.Context, workspace string) ([]*Character, error) {
	client := &MuseavClient{Workspace: strings.TrimSpace(workspace)}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}
	subjects, err := client.ListSubjects(ctx, client.Workspace)
	if err != nil {
		return nil, err
	}
	out := []*Character{}
	for _, s := range subjects {
		if s.Kind != "person" {
			continue
		}
		detail, err := client.ShowSubject(ctx, s.ID)
		if err != nil {
			// 一个主体读不到不该拖垮整个列表：跳过它，但保留一个占位，
			// 让用户知道有这个角色、只是没读出来。
			out = append(out, &Character{
				ID: s.ID, Name: s.Name, Kind: s.Kind,
				Persona: strings.TrimSpace(strOr(s.PersonaSource, "")),
				Traits:  subjectTraits(s.Traits),
				Missing: []string{"读取中台详情失败：" + err.Error()},
			})
			continue
		}
		out = append(out, subjectToCharacter(*detail, "", e.sourceBatchFor(s.ID)))
	}
	return out, nil
}

// GetCharacter 拉一个角色的详情与完成度。
func (e *Engine) GetCharacter(ctx context.Context, subjectID string) (*Character, error) {
	client := &MuseavClient{}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}
	detail, err := client.ShowSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	return subjectToCharacter(*detail, "", e.sourceBatchFor(detail.Subject.ID)), nil
}

// CharacterMirrorPath 角色图在本地的镜像目录。
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

// ──────────────────────────────────────────────────────────────
// 本地镜像
// ──────────────────────────────────────────────────────────────

// CharacterMirror 角色的本地镜像。
//
// 中台是真源，本地这份是为了「打开就有、断了网也有、导得出」。
type CharacterMirror struct {
	ID        string            `json:"id"`
	SubjectID string            `json:"subjectId"`
	Name      string            `json:"name"`
	Persona   string            `json:"persona"`
	Traits    []CharacterTrait  `json:"traits"`
	Complete  bool              `json:"complete"`
	Missing   []string          `json:"missing,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
	Views     map[string]string `json:"views"`
	// SourceBatch 五视图的同源批次标记。
	//
	// 只有本地知道「这五张是不是同一轮生成的」，所以存这儿。
	// 读角色时拿它回填，完成度判定才认得出同源。
	SourceBatch string `json:"sourceBatch,omitempty"`
	UpdatedAt   string `json:"updatedAt"`
}

// SaveCharacterMirror 存角色的本地镜像。
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
			// 镜像失败不阻断：角色在中台还是好的，只是本地没缓存。
			continue
		}
		mirror.Views[string(a.View)] = local
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
//
// 读不到就返回空串 —— 空串会让完成度判定报「无法确认同源」，
// 那是诚实的结果，不该猜一个批次号糊弄过去。
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
	return out, nil
}
