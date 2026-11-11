package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StudioStore 是本机工作台的状态落盘（从 store.mjs 搬）。
//
// 为什么保留这套而不是改用其他方式：任务在跑的过程中，上游「提交即计费」的
// 任务 ID 必须立刻落盘；进程重启后绝不能把同一条付费请求再提交一次。所以
// 一个天然的 JSON 文件 + 原子替换（tmp+rename）就够了，不引入数据库。
type StudioStore struct {
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
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Status    string  `json:"status"`
	StartedAt *string `json:"startedAt,omitempty"`
	EndedAt   *string `json:"endedAt,omitempty"`
	Error     string  `json:"error,omitempty"`
	Message   string  `json:"message,omitempty"`
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
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".reel-kit", "studio")
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
	s.Save()
	return s, nil
}

// Save 原子落盘。
func (s *StudioStore) Save() {
	file := filepath.Join(s.root, "studio.json")
	data, _ := json.MarshalIndent(s.state, "", "  ")
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, file)
}

// State 暴露内部状态给导入逻辑（jobs 遍历等）。
func (s *StudioStore) State() *StudioState { return s.state }

// Root 数据目录。
func (s *StudioStore) Root() string { return s.root }

// Project 按 id 找一个项目，不存在报错。
func (s *StudioStore) Project(projectID string) (*Project, error) {
	for _, p := range s.state.Projects {
		if p.ID == projectID {
			return p, nil
		}
	}
	return nil, errors.New("项目不存在")
}

// Asset 按 id 找素材，可选限定项目。
func (s *StudioStore) Asset(assetID, projectID string) (*Asset, error) {
	for _, a := range s.state.Assets {
		if a.ID == assetID && (projectID == "" || a.ProjectID == projectID) {
			return a, nil
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
	s.state.Projects = append(s.state.Projects, p)
	s.Save()
	return p, nil
}

// AddAsset 写一份素材到媒体目录并登记。
func (s *StudioStore) AddAsset(projectID, name, mime string, bytes []byte, extra map[string]any) (*Asset, error) {
	if _, err := s.Project(projectID); err != nil {
		return nil, err
	}
	ext := map[string]string{
		"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp",
		"video/mp4": ".mp4", "audio/wav": ".wav", "audio/mpeg": ".mp3",
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
	}
	if err := os.WriteFile(s.Path(a), bytes, 0o600); err != nil {
		return nil, fmt.Errorf("写入素材失败: %w", err)
	}
	s.state.Assets = append(s.state.Assets, a)
	s.Save()
	return a, nil
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
