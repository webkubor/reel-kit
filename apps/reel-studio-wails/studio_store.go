package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StudioStore 是本机工作台的状态落盘（从 store.mjs 搬）。
//
// 为什么保留这套而不是改用其他方式：任务在跑的过程中，上游「提交即计费」的
// 任务 ID 必须立刻落盘；进程重启后绝不能把同一条付费请求再提交一次。所以
// 一个天然的 JSON 文件 + 原子替换（tmp+rename）就够了，不引入数据库。
type StudioStore struct {
	mu    sync.Mutex // Shared by queue updates, imports and snapshots.
	root  string
	state *StudioState
}

// StudioState 对应 store.mjs 里 studio.json 的整体结构。
type StudioState struct {
	Projects []*Project `json:"projects"`
	Assets   []*Asset   `json:"assets"`
	Jobs     []*Job     `json:"jobs"`
	Voices   []*Voice   `json:"voices"`
	Version  int        `json:"version,omitempty"`
}

// Project 一个视频项目。
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// Asset 一份素材（图片/视频/音频）。
type Asset struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	MIME      string `json:"mime"`
	File      string `json:"file"`
	CreatedAt string `json:"createdAt"`

	Width    int     `json:"width,omitempty"`
	Height   int     `json:"height,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	HasAudio *bool   `json:"hasAudio,omitempty"`

	Source       string `json:"source,omitempty"`
	Origin       string `json:"origin,omitempty"`
	Engine       string `json:"engine,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
	Persona      string `json:"persona,omitempty"`
	License      string `json:"license,omitempty"`
	LicenseURL   string `json:"licenseUrl,omitempty"`
	BGMSource    string `json:"bgmSource,omitempty"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	SourceTaskID string `json:"sourceTaskId,omitempty"`

	// Extra 装认不出来的字段。
	//
	// Node 版 addAsset 把附加字段平铺进素材对象（tone、voiceBackend、
	// requestedModel、profile、speed、actualModel…），任意字段都能直接读。
	// Go 逐个字段抄一遍迟早漏 —— 漏掉的那个字段是「这条片子当时是谁合成的」
	// 这类信息，丢了没人会发现，只在复用旧配音时莫名其妙不灵。
	// 所以这里原样存、落盘时平铺回去。
	Extra map[string]any `json:"-"`
}

// assetKnownFields 是 Asset 上已有专门字段的键。
// 落盘时这些键由字段本身负责，不进 Extra，避免同一份数据存两遍。
var assetKnownFields = map[string]bool{
	"id": true, "projectId": true, "name": true, "mime": true, "file": true,
	"createdAt": true, "width": true, "height": true, "duration": true,
	"hasAudio": true, "source": true, "origin": true, "engine": true,
	"prompt": true, "persona": true, "license": true, "licenseUrl": true,
	"bgmSource": true, "sourceUrl": true, "sourceTaskId": true,
}

// MarshalJSON 把 Extra 平铺进对象，和 Node 版的 addAsset 行为一致。
func (a Asset) MarshalJSON() ([]byte, error) {
	type alias Asset
	base, err := json.Marshal(alias(a))
	if err != nil {
		return nil, err
	}
	if len(a.Extra) == 0 {
		return base, nil
	}
	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}
	for k, v := range a.Extra {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}
	return json.Marshal(merged)
}

// UnmarshalJSON 把不认识的键收进 Extra。
func (a *Asset) UnmarshalJSON(data []byte) error {
	type alias Asset
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*a = Asset(aux)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for k, v := range raw {
		if assetKnownFields[k] {
			continue
		}
		if a.Extra == nil {
			a.Extra = map[string]any{}
		}
		a.Extra[k] = v
	}
	return nil
}

// GetExtra 读附加字段。
func (a *Asset) GetExtra(key string) (any, bool) {
	if a.Extra == nil {
		return nil, false
	}
	v, ok := a.Extra[key]
	return v, ok
}

// SetExtra 写附加字段。
func (a *Asset) SetExtra(key string, value any) {
	if a.Extra == nil {
		a.Extra = map[string]any{}
	}
	a.Extra[key] = value
}

// Voice 一条本地音色记录。
type Voice struct {
	Persona   string   `json:"persona"`
	Name      string   `json:"name"`
	Hints     []string `json:"hints,omitempty"`
	Note      string   `json:"note,omitempty"`
	Origin    string   `json:"origin,omitempty"`
	CreatedAt string   `json:"createdAt,omitempty"`
}

// Job 一次出片任务，状态机：queued → running → succeeded|failed|canceled|interrupted。
type Job struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	Input       map[string]any `json:"input"`
	Status      string         `json:"status"`
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	Message     string         `json:"message"`
	Error       string         `json:"error"`
	Steps       []Step         `json:"steps"`
	AssetID     string         `json:"assetId,omitempty"`
	Upstream    map[string]any `json:"upstream,omitempty"`
	Quality     *Quality       `json:"quality,omitempty"`
	Approval    *bool          `json:"approval,omitempty"`
	SOPSnapshot map[string]any `json:"sopSnapshot,omitempty"`
}

// Step 任务进度的一个步骤。
type Step struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	// Origin 这个环节的东西从哪来：code 本机算的、cloud-paid 云端花钱的、
	// local-offline 本机跑的。工序面板要靠它告诉用户「这一步会不会花钱」。
	Origin string `json:"origin,omitempty"`
	Engine string `json:"engine,omitempty"`
	Detail string `json:"detail,omitempty"`
	// AssetID 这一步产出的素材。
	AssetID   string  `json:"assetId,omitempty"`
	StartedAt *string `json:"startedAt,omitempty"`
	EndedAt   *string `json:"endedAt,omitempty"`
	Error     string  `json:"error,omitempty"`
	Message   string  `json:"message,omitempty"`
}

// setPayload 把附加信息写进步骤。
func (s *Step) setPayload(key string, value any) {
	switch key {
	case "assetId":
		if v, ok := value.(string); ok {
			s.AssetID = v
		}
	case "detail":
		if v, ok := value.(string); ok {
			s.Detail = v
		}
	case "message":
		if v, ok := value.(string); ok {
			s.Message = v
		}
	}
}

// Quality 推荐视频的检测结果。
type Quality struct {
	Checks    []QualityCheck `json:"checks"`
	Passed    bool           `json:"passed"`
	CheckedAt string         `json:"checkedAt"`
	AssetID   string         `json:"assetId"`
}

// QualityCheck 单条质量检测项。
type QualityCheck struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Passed bool   `json:"passed"`
}

// NewStudioStore 打开（或初始化）本机工作台存储。
func NewStudioStore() (*StudioStore, error) {
	return NewStudioStoreRoot(DefaultStudioRoot())
}

// DefaultStudioRoot 默认工作台数据目录。
func DefaultStudioRoot() string {
	if override := strings.TrimSpace(os.Getenv("REEL_STUDIO_DATA_DIR")); override != "" && filepath.IsAbs(override) {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	// 桌面版的数据模型尚未覆盖旧工作台全部字段，迁移完成前使用独立目录。
	return filepath.Join(home, ".reel-kit", "studio-desktop")
}

// NewStudioStoreRoot 用指定的 root 打开存储。
func NewStudioStoreRoot(root string) (*StudioStore, error) {
	media := filepath.Join(root, "media")
	if err := os.MkdirAll(media, 0o700); err != nil {
		return nil, fmt.Errorf("创建工作台目录失败: %w", err)
	}
	file := filepath.Join(root, "studio.json")
	state := &StudioState{Projects: []*Project{}, Assets: []*Asset{}, Jobs: []*Job{}, Voices: []*Voice{}}
	if data, err := os.ReadFile(file); err == nil {
		if err := json.Unmarshal(data, state); err != nil {
			return nil, fmt.Errorf("工作台状态损坏: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取工作台状态失败: %w", err)
	}
	if state.Projects == nil {
		state.Projects = []*Project{}
	}
	if state.Assets == nil {
		state.Assets = []*Asset{}
	}
	if state.Jobs == nil {
		state.Jobs = []*Job{}
	}
	if state.Voices == nil {
		state.Voices = []*Voice{}
	}
	s := &StudioStore{root: root, state: state}

	// A restart must not silently submit the same paid request a second time.
	for _, job := range state.Jobs {
		if job.Status == "queued" || job.Status == "running" {
			job.Status = "interrupted"
			job.Error = "工作台已重启。请先核对上游任务，避免重复消耗额度。"
			job.UpdatedAt = nowISO()
		}
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	return s, nil
}

// Save serializes all writers, including queue progress and imported assets.
func (s *StudioStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *StudioStore) saveLocked() error {
	file := filepath.Join(s.root, "studio.json")
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// State returns an owned snapshot so Wails can serialize it after unlocking.
func (s *StudioStore) State() *StudioState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := &StudioState{Version: s.state.Version, Projects: []*Project{}, Assets: []*Asset{}, Jobs: []*Job{}, Voices: []*Voice{}}
	for _, p := range s.state.Projects {
		copy := *p
		out.Projects = append(out.Projects, &copy)
	}
	for _, a := range s.state.Assets {
		out.Assets = append(out.Assets, cloneAsset(a))
	}
	for _, j := range s.state.Jobs {
		out.Jobs = append(out.Jobs, cloneJob(j))
	}
	for _, v := range s.state.Voices {
		copy := *v
		copy.Hints = append([]string(nil), v.Hints...)
		out.Voices = append(out.Voices, &copy)
	}
	return out
}

// Root 数据目录。
func (s *StudioStore) Root() string { return s.root }

// Project 按 id 找一个项目，不存在报错。
func (s *StudioStore) Project(projectID string) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Projects {
		if p.ID == projectID {
			copy := *p
			return &copy, nil
		}
	}
	return nil, errors.New("项目不存在")
}

// Asset 按 id 找素材，可选限定项目。
func (s *StudioStore) Asset(assetID, projectID string) (*Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.state.Assets {
		if a.ID == assetID && (projectID == "" || a.ProjectID == projectID) {
			return cloneAsset(a), nil
		}
	}
	return nil, errors.New("素材不存在或不属于此项目")
}

// Path 素材在磁盘上的绝对路径。
func (s *StudioStore) Path(a *Asset) string {
	return filepath.Join(s.root, "media", a.File)
}

// AddProject 新建项目。
func (s *StudioStore) AddProject(name string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return nil, errors.New("项目名称须为 1–80 字")
	}
	p := &Project{ID: newID(), Name: name, CreatedAt: nowISO()}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Projects = append(s.state.Projects, p)
	if err := s.saveLocked(); err != nil {
		s.state.Projects = s.state.Projects[:len(s.state.Projects)-1]
		return nil, err
	}
	copy := *p
	return &copy, nil
}

// AddAsset 写一份素材到媒体目录并登记。
func (s *StudioStore) AddAsset(projectID, name, mime string, bytes []byte, extra map[string]any) (*Asset, error) {
	if _, err := s.Project(projectID); err != nil {
		return nil, err
	}
	ext := map[string]string{
		"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif",
		"video/mp4": ".mp4", "audio/wav": ".wav", "audio/mpeg": ".mp3", "application/json": ".json",
		"audio/mp4": ".m4a", "audio/ogg": ".ogg",
	}[mime]
	if ext == "" {
		return nil, errors.New("不支持此素材格式")
	}
	a := &Asset{
		ID: newID(), ProjectID: projectID,
		Name: truncate(name, 160), MIME: mime, File: newID() + ext,
		CreatedAt: nowISO(),
	}
	if extra != nil {
		// Normalize JSON values before ownership transfer; reject non-serializable data.
		data, err := json.Marshal(extra)
		if err != nil {
			return nil, err
		}
		var normalized map[string]any
		if err := json.Unmarshal(data, &normalized); err != nil {
			return nil, err
		}
		extra = normalized
		if n, ok := extra["width"].(float64); ok {
			a.Width = int(n)
		}
		if n, ok := extra["height"].(float64); ok {
			a.Height = int(n)
		}
		a.Duration, _ = extra["duration"].(float64)
		if v, ok := extra["hasAudio"].(bool); ok {
			a.HasAudio = &v
		}
		a.Source, _ = extra["source"].(string)
		a.Origin, _ = extra["origin"].(string)
		a.Engine, _ = extra["engine"].(string)
		a.Prompt, _ = extra["prompt"].(string)
		a.Persona, _ = extra["persona"].(string)
		a.License, _ = extra["license"].(string)
		a.LicenseURL, _ = extra["licenseUrl"].(string)
		a.BGMSource, _ = extra["bgmSource"].(string)
		a.SourceURL, _ = extra["sourceUrl"].(string)
		a.SourceTaskID, _ = extra["sourceTaskId"].(string)
		// 其余字段原样存进 Extra —— 认不出来的那些也是数据，
		// 丢了就再也说不清这条片子当时是怎么来的。
		for k, v := range extra {
			if assetKnownFields[k] {
				continue
			}
			a.SetExtra(k, v)
		}
	}
	if err := os.WriteFile(s.Path(a), bytes, 0o600); err != nil {
		return nil, fmt.Errorf("写入素材失败: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Assets = append(s.state.Assets, a)
	if err := s.saveLocked(); err != nil {
		s.state.Assets = s.state.Assets[:len(s.state.Assets)-1]
		_ = os.Remove(s.Path(a)) // Only the file created by this failed import.
		return nil, err
	}
	return cloneAsset(a), nil
}

// PublicAsset 去掉落到磁盘的文件名，换成界面能用的 url。
func (s *StudioStore) PublicAsset(a *Asset) *Asset {
	pub := *a
	pub.File = ""
	return &pub
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

func newID() string {
	return uuid.NewString()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
