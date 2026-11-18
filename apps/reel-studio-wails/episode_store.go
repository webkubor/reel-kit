package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// 一部短剧的结构：项目 → 剧集 → 镜头。
//
// 为什么是这个层级：12 集 30 个角色的片子，靠「一个项目 = 一条视频」装不下
// —— 角色会被埋在制作表单里，30 个角色翻起来要命，12 集更是无从谈起。
//
// 分工：
//   · 角色、场景存在中台（museav）—— 跨项目共享，agent 直接能用
//   · 剧集、镜头存在本地 —— 每个项目的片子是自己的，别人不该看到
//
// 镜头是「一个可播放的片段」的容器：出镜素材、旁白、成片都挂在它下面，
// 这样一集里的多个镜头能排队、能有状态，也能在没做完时看到进度。

// Episode 一集。
type Episode struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	// Number 集数序号（1 起）。短剧靠它排序，不靠创建时间。
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
	// CharacterIDs 本集出场角色（中台主体 id）。
	CharacterIDs []string `json:"characterIds,omitempty"`
	// SceneID 本集主场景（中台主体 id，kind=scene）。
	SceneID string `json:"sceneId,omitempty"`
	// Status draft | making | done。
	Status string `json:"status,omitempty"`
	// PublishURL 已发布平台的这一集链接（YouTube/B站等），空 = 还没发。
	PublishURL string `json:"publishUrl,omitempty"`
	// PublishTitle 发布平台上的标题。
	PublishTitle string `json:"publishTitle,omitempty"`
	// PublishPlatform 发布平台：youtube | bilibili | douyin | …，空 = 未知。
	PublishPlatform string `json:"publishPlatform,omitempty"`
	// VideoID 发布平台上的视频 id（YouTube 的 11 位 id），用于内嵌播放。
	VideoID string `json:"videoId,omitempty"`
	// ThumbnailURL 封面图。
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// EpisodeShot 一集里的一个镜头。
// （不叫 Shot：render.go 里的 Shot 是分镜模板的一个画面单元，两码事）
type EpisodeShot struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	EpisodeID string `json:"episodeId"`
	// Number 镜号（本集内序号，1 起）。
	Number int `json:"number"`
	// Title 镜头小标题，如「长街回望」。
	Title string `json:"title"`
	// Action 角色在镜头里做什么（出镜提示词的动作部分）。
	Action string `json:"action,omitempty"`
	// Lines 旁白（做成片时用）。
	Lines []string `json:"lines,omitempty"`
	// CharacterID 出镜角色（中台主体 id）。
	CharacterID string `json:"characterId,omitempty"`
	// Look 装扮（定妆稿名里的装扮名）。
	Look string `json:"look,omitempty"`
	// SceneID 场景主体 id。
	SceneID string `json:"sceneId,omitempty"`
	// ClipAssetID 出镜素材（本地素材 id）。
	ClipAssetID string `json:"clipAssetId,omitempty"`
	// AssetID 成片素材（本地素材 id）。
	AssetID string `json:"assetId,omitempty"`
	// Duration 计划时长（秒）。
	Duration float64 `json:"duration,omitempty"`
	// Status planned | shooting | shot | done。
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// EpisodeDraft 新建一集的参数。
// （不叫 EpisodeInput：episode.go 里的 EpisodeInput 是「做成片」的入参，两码事）
type EpisodeDraft struct {
	ProjectID    string
	Number       int
	Title        string
	Summary      string
	CharacterIDs []string
	SceneID      string
	// 发布信息：导入已发布的剧集时带上，界面可直接内嵌播放。
	PublishURL      string
	PublishTitle    string
	PublishPlatform string
	VideoID         string
	ThumbnailURL    string
}

// ShotInput 新建一个镜头的参数。
type ShotInput struct {
	ProjectID   string
	EpisodeID   string
	Title       string
	Action      string
	Lines       []string
	CharacterID string
	Look        string
	SceneID     string
	Duration    float64
}

// AddEpisode 新建一集。
func (s *StudioStore) AddEpisode(in EpisodeDraft) (*Episode, error) {
	if _, err := s.Project(in.ProjectID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" || len([]rune(title)) > 80 {
		return nil, errors.New("集名须为 1–80 字")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	number := in.Number
	if number <= 0 {
		number = 1
		for _, e := range s.state.Episodes {
			if e.ProjectID == in.ProjectID && e.Number >= number {
				number = e.Number + 1
			}
		}
	}
	now := nowISO()
	ep := &Episode{
		ID: newID(), ProjectID: in.ProjectID, Number: number, Title: title,
		Summary: strings.TrimSpace(in.Summary), CharacterIDs: append([]string(nil), in.CharacterIDs...),
		SceneID: strings.TrimSpace(in.SceneID), Status: "draft",
		PublishURL: strings.TrimSpace(in.PublishURL), PublishTitle: strings.TrimSpace(in.PublishTitle),
		PublishPlatform: strings.TrimSpace(in.PublishPlatform), VideoID: strings.TrimSpace(in.VideoID),
		ThumbnailURL: strings.TrimSpace(in.ThumbnailURL),
		CreatedAt:    now, UpdatedAt: now,
	}
	s.state.Episodes = append(s.state.Episodes, ep)
	if err := s.saveLocked(); err != nil {
		s.state.Episodes = s.state.Episodes[:len(s.state.Episodes)-1]
		return nil, err
	}
	cp := *ep
	cp.CharacterIDs = append([]string(nil), ep.CharacterIDs...)
	return &cp, nil
}

// ListEpisodes 列项目的剧集，按集数排序。
func (s *StudioStore) ListEpisodes(projectID string) []*Episode {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*Episode{}
	for _, e := range s.state.Episodes {
		if projectID == "" || e.ProjectID == projectID {
			cp := *e
			cp.CharacterIDs = append([]string(nil), e.CharacterIDs...)
			out = append(out, &cp)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// Episode 取一集。
func (s *StudioStore) Episode(id string) (*Episode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.state.Episodes {
		if e.ID == id {
			cp := *e
			cp.CharacterIDs = append([]string(nil), e.CharacterIDs...)
			return &cp, nil
		}
	}
	return nil, errors.New("剧集不存在")
}

// UpdateEpisode 改一集。
func (s *StudioStore) UpdateEpisode(id string, apply func(*Episode)) (*Episode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.state.Episodes {
		if e.ID != id {
			continue
		}
		apply(e)
		e.UpdatedAt = nowISO()
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		cp := *e
		cp.CharacterIDs = append([]string(nil), e.CharacterIDs...)
		return &cp, nil
	}
	return nil, errors.New("剧集不存在")
}

// AddShot 新建一个镜头。
func (s *StudioStore) AddShot(in ShotInput) (*EpisodeShot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var episode *Episode
	for _, e := range s.state.Episodes {
		if e.ID == in.EpisodeID {
			episode = e
			break
		}
	}
	if episode == nil {
		return nil, errors.New("剧集不存在")
	}
	if episode.ProjectID != in.ProjectID {
		return nil, errors.New("镜头和剧集必须属于同一项目")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "新镜头"
	}
	number := 1
	for _, sh := range s.state.Shots {
		if sh.EpisodeID == in.EpisodeID && sh.Number >= number {
			number = sh.Number + 1
		}
	}
	now := nowISO()
	shot := &EpisodeShot{
		ID: newID(), ProjectID: in.ProjectID, EpisodeID: in.EpisodeID, Number: number,
		Title: title, Action: strings.TrimSpace(in.Action), Lines: append([]string(nil), in.Lines...),
		CharacterID: strings.TrimSpace(in.CharacterID), Look: strings.TrimSpace(in.Look),
		SceneID: strings.TrimSpace(in.SceneID), Duration: in.Duration,
		Status: "planned", CreatedAt: now, UpdatedAt: now,
	}
	s.state.Shots = append(s.state.Shots, shot)
	if err := s.saveLocked(); err != nil {
		s.state.Shots = s.state.Shots[:len(s.state.Shots)-1]
		return nil, err
	}
	cp := *shot
	cp.Lines = append([]string(nil), shot.Lines...)
	return &cp, nil
}

// ListShots 列一集的镜头，按镜号排序。
func (s *StudioStore) ListShots(episodeID string) []*EpisodeShot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*EpisodeShot{}
	for _, sh := range s.state.Shots {
		if episodeID == "" || sh.EpisodeID == episodeID {
			cp := *sh
			cp.Lines = append([]string(nil), sh.Lines...)
			out = append(out, &cp)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// EpisodeShot 取一个镜头。
func (s *StudioStore) EpisodeShot(id string) (*EpisodeShot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sh := range s.state.Shots {
		if sh.ID == id {
			cp := *sh
			cp.Lines = append([]string(nil), sh.Lines...)
			return &cp, nil
		}
	}
	return nil, errors.New("镜头不存在")
}

// UpdateShot 改一个镜头。
func (s *StudioStore) UpdateShot(id string, apply func(*EpisodeShot)) (*EpisodeShot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sh := range s.state.Shots {
		if sh.ID != id {
			continue
		}
		apply(sh)
		sh.UpdatedAt = nowISO()
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		cp := *sh
		cp.Lines = append([]string(nil), sh.Lines...)
		return &cp, nil
	}
	return nil, errors.New("镜头不存在")
}

// ── 引擎层 ──────────────────────────────────────────────

// ListEpisodes 列剧集。
func (e *Engine) ListEpisodes(projectID string) []*Episode {
	if e.store == nil {
		return []*Episode{}
	}
	return e.store.ListEpisodes(projectID)
}

// CreateEpisode 新建一集。
func (e *Engine) CreateEpisode(in EpisodeDraft) (*Episode, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	return e.store.AddEpisode(in)
}

// ListShots 列一集的镜头。
func (e *Engine) ListShots(episodeID string) []*EpisodeShot {
	if e.store == nil {
		return []*EpisodeShot{}
	}
	return e.store.ListShots(episodeID)
}

// CreateShot 新建一个镜头。
func (e *Engine) CreateShot(in ShotInput) (*EpisodeShot, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	return e.store.AddShot(in)
}

// UpdateShot 改一个镜头（动作、旁白、素材挂载、状态）。
func (e *Engine) UpdateShot(id string, apply func(*EpisodeShot)) (*EpisodeShot, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	return e.store.UpdateShot(id, apply)
}

// UpdateEpisode 改一集。
func (e *Engine) UpdateEpisode(id string, apply func(*Episode)) (*Episode, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	return e.store.UpdateEpisode(id, apply)
}

// ListScenes 列中台场景主体（kind=scene）。
//
// 场景和角色一样是中台主体，只是 kind 不同：场景有参考图（空镜/氛围图）
// 和画面描述，没有体貌四项、也不需要五视图。
func (e *Engine) ListScenes(workspace string) ([]*Subject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), characterRequestTimeout)
	defer cancel()
	client := &MuseavClient{Workspace: strings.TrimSpace(workspace)}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}
	subjects, err := client.ListSubjects(ctx, client.Workspace)
	if err != nil {
		return nil, err
	}
	out := []*Subject{}
	for _, s := range subjects {
		if s.Kind == "scene" {
			cp := s
			out = append(out, &cp)
		}
	}
	return out, nil
}

// ListAllSubjects 列中台所有主体，按 kind 分组。
//
// 界面需要一次拿到「人物 + 场景」再自己分组 —— 分两次请求会让角色库
// 面板在载入时闪一下。
func (e *Engine) ListAllSubjects(workspace string) ([]*Subject, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, errProjectUnbound
	}
	ctx, cancel := context.WithTimeout(context.Background(), characterRequestTimeout)
	defer cancel()
	client := &MuseavClient{Workspace: strings.TrimSpace(workspace)}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}
	subjects, err := client.ListSubjects(ctx, client.Workspace)
	if err != nil {
		return nil, err
	}
	out := make([]*Subject, 0, len(subjects))
	for i := range subjects {
		out = append(out, &subjects[i])
	}
	return out, nil
}

// CreateScene 新建一个场景主体。
func (e *Engine) CreateScene(workspace, name, description string) (*Subject, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, errProjectUnbound
	}
	ctx, cancel := context.WithTimeout(context.Background(), characterRequestTimeout)
	defer cancel()
	client := &MuseavClient{Workspace: strings.TrimSpace(workspace)}
	if err := client.Available(ctx); err != nil {
		return nil, err
	}
	traits := map[string]string{}
	if d := strings.TrimSpace(description); d != "" {
		traits["画面描述"] = d
	}
	return client.CreateSubject(ctx, strings.TrimSpace(name), "scene", "", traits)
}

var _ = fmt.Sprint
