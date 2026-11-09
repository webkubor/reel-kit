// Package studioapi 是原生界面与本地工作台后端之间的客户端。
//
// 后端还是 reel-kit 的 Node 服务（src/studio/server.mjs）：渲染、FFmpeg、
// 供应商调用、状态机都已经在那里跑通了。原生界面只做前端 ——
// 两端共用同一套接口与同一份数据（~/.reel-kit/studio/），不是两套实现。
package studioapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 调用本地工作台接口。
type Client struct {
	Base string
	http *http.Client
}

// New 返回一个客户端。base 为空时用默认地址。
func New(base string) *Client {
	if base == "" {
		base = "http://127.0.0.1:4390"
	}
	return &Client{Base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 60 * time.Second}}
}

// Asset 是一条素材。
type Asset struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"projectId"`
	Name      string  `json:"name"`
	Mime      string  `json:"mime"`
	URL       string  `json:"url"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Duration  float64 `json:"duration"`
	HasAudio  bool    `json:"hasAudio"`
	Source    string  `json:"source"`
	Origin    string  `json:"origin"`
	Engine    string  `json:"engine"`
	Prompt    string  `json:"prompt"`
	License   string  `json:"license"`
	// 模型的请求值与实际值要分开 —— 上游可能换一个模型跑。
	RequestedModel string `json:"requestedModel"`
	ActualModel    string `json:"actualModel"`
	UpstreamTaskID string `json:"upstreamTaskId"`
	Profile        string `json:"profile"`
	VoiceBackend   string `json:"voiceBackend"`
}

// Project 是一个项目。
type Project struct {
	ID                     string         `json:"id"`
	Name                   string         `json:"name"`
	PipelineKey            string         `json:"pipelineKey"`
	SOPVersionID           string         `json:"sopVersionId"`
	RecommendationSettings map[string]any `json:"recommendationSettings"`
	CreatedAt              string         `json:"createdAt"`
}

// QualityCheck 是成片质量检查项。
type QualityCheck struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Passed bool   `json:"passed"`
}

// JobQuality 是成片自动检测结果。
type JobQuality struct {
	Passed  bool           `json:"passed"`
	AssetID string         `json:"assetId"`
	Checks  []QualityCheck `json:"checks"`
}

// JobApproval 是人工审片确认。
type JobApproval struct {
	Checks     map[string]bool `json:"checks"`
	AssetID    string          `json:"assetId"`
	ReviewedAt string          `json:"reviewedAt"`
}

// SOPSnapshot 是生成时冻结的制作方法。
type SOPSnapshot struct {
	SOPVersionID string         `json:"sopVersionId"`
	SOPVersion   int            `json:"sopVersion"`
	Inferred     bool           `json:"inferred"`
	Input        map[string]any `json:"input"`
}

// Job 是一次生成任务。
type Job struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	Input       map[string]any `json:"input"`
	Status      string         `json:"status"`
	Message     string         `json:"message"`
	Error       string         `json:"error"`
	AssetID     string         `json:"assetId"`
	Upstream    map[string]any `json:"upstream"`
	Steps       []Step         `json:"steps"`
	Quality     *JobQuality    `json:"quality"`
	Approval    *JobApproval   `json:"approval"`
	SOPSnapshot *SOPSnapshot   `json:"sopSnapshot"`
	CreatedAt   string         `json:"createdAt"`
}

// Step 是任务过程里的一步。
type Step struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Origin string `json:"origin"`
	Engine string `json:"engine"`
	Detail string `json:"detail"`
	Status string `json:"status"`
	Error  string `json:"error"`
	MS     int64  `json:"ms"`
}

// Publication 是一次发布记录。
type Publication struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	JobID        string `json:"jobId"`
	AssetID      string `json:"assetId"`
	SOPVersionID string `json:"sopVersionId"`
	Platform     string `json:"platform"`
	URL          string `json:"url"`
	PublishedAt  string `json:"publishedAt"`
	Goal         string `json:"goal"`
	Tags         string `json:"tags"`
	CreatedAt    string `json:"createdAt"`
}

// Performance 是发布后的表现回填。
type Performance struct {
	ID         string         `json:"id"`
	PublishID  string         `json:"publishId"`
	Metrics    map[string]int `json:"metrics"`
	Window     string         `json:"window"`
	Source     string         `json:"source"`
	RecordedAt string         `json:"recordedAt"`
}

// Review 是复盘记录。
type Review struct {
	ID              string `json:"id"`
	PublishID       string `json:"publishId"`
	Findings        string `json:"findings"`
	Hypothesis      string `json:"hypothesis"`
	Changes         string `json:"changes"`
	NewSOPVersionID string `json:"newSopVersionId"`
	RecordedAt      string `json:"recordedAt"`
}

// SOPVersion 是一个版本的制作方法。
type SOPVersion struct {
	ID         string         `json:"id"`
	Pipeline   string         `json:"pipeline"`
	Version    int            `json:"version"`
	ChangeNote string         `json:"changeNote"`
	Settings   map[string]any `json:"settings"`
	CreatedAt  string         `json:"createdAt"`
}

// State 是工作台的完整状态。
type State struct {
	Projects       []Project      `json:"projects"`
	Assets         []Asset        `json:"assets"`
	Jobs           []Job          `json:"jobs"`
	Voices         []any          `json:"voices"`
	Templates      []string       `json:"templates"`
	Publishes      []Publication  `json:"publishes"`
	Performance    []Performance  `json:"performance"`
	Reviews        []Review       `json:"reviews"`
	SOPVersions    []SOPVersion   `json:"sopVersions"`
	PlatformLimits map[string]any `json:"platformLimits"`
}

// Channel 是一条生成通道。
type Channel struct {
	ID               string  `json:"id"`
	Label            string  `json:"label"`
	Ready            bool    `json:"ready"`
	Note             string  `json:"note"`
	Models           []Model `json:"models"`
	Voices           []Voice `json:"voices"`
	AllowCustomModel bool    `json:"allowCustomModel"`
	AllowCustomVoice bool    `json:"allowCustomVoice"`
}

// Model 是一个模型。
type Model struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	MinSeconds float64 `json:"minSeconds"`
	MaxSeconds float64 `json:"maxSeconds"`
}

// Voice 是一个音色。
type Voice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GroupStatus 是一类生成能力（视频/图片/语音）的状态。
type GroupStatus struct {
	Ready    bool      `json:"ready"`
	Label    string    `json:"label"`
	Note     string    `json:"note"`
	Channels []Channel `json:"channels"`
	Profiles []Profile `json:"profiles"`
	Personas []Persona `json:"personas"`
}

// Profile 是一个计费账户。
type Profile struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Persona 是本地音色。
type Persona struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Providers 是所有生成能力的状态。
type Providers struct {
	Video          GroupStatus `json:"video"`
	Image          GroupStatus `json:"image"`
	Voice          GroupStatus `json:"voice"`
	CustomChannels struct {
		File     string   `json:"file"`
		Problems []string `json:"problems"`
	} `json:"customChannels"`
}

// BGMItem 是配乐库中的一首。
type BGMItem struct {
	Alias      string `json:"alias"`
	Key        string `json:"key"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
	Source     string `json:"source"`
	SourceURL  string `json:"sourceUrl"`
}

// BGMLibrary 是配乐库。
type BGMLibrary struct {
	Items      []BGMItem `json:"items"`
	Configured bool      `json:"configured"`
	Error      string    `json:"error"`
}

func (c *Client) do(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.Base+path, reader)
	if err != nil {
		return err
	}
	// 写操作要带这个头，服务端据此拒绝网页以外的来源
	req.Header.Set("X-Reel-Action", "studio")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("连不上本地工作台（%s）：%w", c.Base, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		var payload struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &payload) == nil && payload.Error != "" {
			return fmt.Errorf("%s", payload.Error)
		}
		return fmt.Errorf("接口返回 %d", res.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// State 读取工作台状态。
func (c *Client) State() (*State, error) {
	var s State
	err := c.do(http.MethodGet, "/api/state", nil, &s)
	return &s, err
}

// Providers 读取生成通道状态。
func (c *Client) Providers() (*Providers, error) {
	var p Providers
	err := c.do(http.MethodGet, "/api/providers", nil, &p)
	return &p, err
}

// Channels 返回某一类能力的通道列表。
func (p *Providers) Channels(group string) []Channel {
	switch group {
	case "video":
		return p.Video.Channels
	case "image":
		return p.Image.Channels
	case "voice":
		return p.Voice.Channels
	}
	return nil
}

// BGM 读取配乐库。
func (c *Client) BGM() (*BGMLibrary, error) {
	var b BGMLibrary
	err := c.do(http.MethodGet, "/api/bgm", nil, &b)
	return &b, err
}

// NewProject 建一个项目。
func (c *Client) NewProject(name string) (*Project, error) {
	var p Project
	err := c.do(http.MethodPost, "/api/projects", map[string]any{"name": name}, &p)
	return &p, err
}

// SubmitJob 提交一个生成任务。
func (c *Client) SubmitJob(input map[string]any) (*Job, error) {
	var j Job
	err := c.do(http.MethodPost, "/api/jobs", input, &j)
	return &j, err
}

// ResumeJob 取回已在上游生成、只是没下载下来的产物。
//
// 重新提交等于再付一次钱，所以这条路径独立存在。
func (c *Client) ResumeJob(jobID string) (*Job, error) {
	var j Job
	err := c.do(http.MethodPost, "/api/jobs/resume", map[string]any{"jobId": jobID}, &j)
	return &j, err
}

// SaveSettings 保存项目偏好（含通道、模型、音色、账户）。
func (c *Client) SaveSettings(projectID string, settings map[string]any) error {
	return c.do(http.MethodPost, "/api/project-settings", map[string]any{"projectId": projectID, "settings": settings}, nil)
}

// PullBGM 从配乐库取用一首到项目。
func (c *Client) PullBGM(projectID, alias string) (*Asset, error) {
	var a Asset
	err := c.do(http.MethodPost, "/api/bgm/pull", map[string]any{"projectId": projectID, "alias": alias}, &a)
	return &a, err
}

// Draft 取回 GitHub 项目的分镜草稿。
func (c *Client) Draft(projectID, repoURL string) (map[string]any, error) {
	var out map[string]any
	err := c.do(http.MethodPost, "/api/recommendation/draft", map[string]any{"projectId": projectID, "repoUrl": repoURL}, &out)
	return out, err
}

// Workflow 调用工作流接口（发布、复盘、再做一条等）。
func (c *Client) Workflow(action string, input map[string]any) (map[string]any, error) {
	var out map[string]any
	err := c.do(http.MethodPost, "/api/workflow/"+action, input, &out)
	return out, err
}
