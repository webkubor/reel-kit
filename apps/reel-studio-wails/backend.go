package main

import (
	"context"
	"errors"
	"sync"
)

// Engine 是桌面壳的本地引擎：零 HTTP，零 Node，不启动任何子进程、不占端口。
// 所有状态留在内存里，通过 Go 绑定直接让界面调用。
//
// Phase 2 会把合成/TTS/供应商等逻辑从 Node 层搬进来，直接以 Go 方法
// 挂到 Engine 上，不再依赖任何外部 Node 进程或运行时目录。
type Engine struct {
	mu   sync.Mutex
	note string

	store  *StudioStore
	queue  *JobQueue
	cancel context.CancelFunc

	// voxcraft：本机离线语音引擎的服务句柄（可能复用已有的）。
	voxcraft *VoxcraftServer
}

// Status 是引擎当前状态，给界面看。
type Status struct {
	Ready bool   `json:"ready"`
	URL   string `json:"url"`
	Note  string `json:"note"`
	Error string `json:"error"`
}

// NewEngine 造一个本地引擎。
func NewEngine() *Engine {
	e := &Engine{note: "本地工作台已就绪"}
	store, err := NewStudioStore()
	if err != nil {
		e.note = "工作台状态初始化失败：" + err.Error()
		return e
	}
	e.store = store
	e.queue = NewJobQueue(store, nil)
	return e
}

// Status 返回当前状态：直接就绪，不启动任何子进程。
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{Ready: true, URL: "", Note: e.note, Error: ""}
}

// Start 启动后台 worker（任务队列）。界面加载后调用一次。
func (e *Engine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.queue == nil || e.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go e.queue.Run(ctx)
}

// Stop 退出钩子：停掉 worker。
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
}

// ──────────────────────────────────────────────────────────────
// 配置旋钮（从 settings.mjs 搬）：零副作用，全在内存
// ──────────────────────────────────────────────────────────────

// ListSettingGroups 返回配置分节顺序。
func (e *Engine) ListSettingGroups() []string {
	return SettingGroups
}

// GetSettingSchema 返回全部旋钮定义。
func (e *Engine) GetSettingSchema() map[string]SettingField {
	return SettingSchema
}

// ListPresets 列出所有预设（只给前端看的部分）。
func (e *Engine) ListPresets() []PresetItem {
	return ListPresets()
}

// ResolveSettings 收敛一份完整配置。
// 顺序：预设 → 流水线默认 → 字段默认 → 用户填的值。
func (e *Engine) ResolveSettings(kind string, input map[string]interface{}, presetID string) map[string]interface{} {
	return ResolveSettings(kind, input, presetID)
}

// DescribeSetting 把某个旋钮的当前值配成人话。
func (e *Engine) DescribeSetting(key string, value interface{}) string {
	return DescribeSetting(key, value)
}

// ──────────────────────────────────────────────────────────────
// 知识短视频风格库（从 knowledge-styles.mjs 搬）：零副作用
// ──────────────────────────────────────────────────────────────

// GetKnowledgeStyles 返回 44 套风格元数据。
func (e *Engine) GetKnowledgeStyles() []KnowledgeStyle {
	return KnowledgeStyles
}

// GetStyle 根据风格 key 获取配色与推荐模板。
func (e *Engine) GetStyle(key string) *StylePalette {
	return GetStyle(key)
}

// ──────────────────────────────────────────────────────────────
// 本机状态与任务队列（从 store.mjs / server.mjs 搬）
//
// 注意：这里没有任何 HTTP 路由。Node 版 server.mjs 里的
// GET /api/state、POST /api/jobs、POST /api/jobs/:id/cancel
// 现在直接变成 window.go.engine.State() / SubmitJob() / CancelJob()。
// ──────────────────────────────────────────────────────────────

// ListProjects 列出所有项目。
func (e *Engine) ListProjects() []*Project {
	if e.store == nil {
		return []*Project{}
	}
	return e.store.State().Projects
}

// CreateProject 新建项目。
func (e *Engine) CreateProject(name string) (*Project, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	return e.store.AddProject(name)
}

// ListAssets 列出素材（可按项目过滤）。
func (e *Engine) ListAssets(projectID string) []*Asset {
	if e.store == nil {
		return []*Asset{}
	}
	all := e.store.State().Assets
	out := make([]*Asset, 0, len(all))
	for _, a := range all {
		if projectID == "" || a.ProjectID == projectID {
			out = append(out, e.store.PublicAsset(a))
		}
	}
	return out
}

// ListVoices 列出本地音色记录。
func (e *Engine) ListVoices() []*Voice {
	if e.store == nil {
		return []*Voice{}
	}
	return e.store.State().Voices
}

// State 界面轮询用：一次拿到工作台全量状态。
func (e *Engine) State() *StudioState {
	if e.store == nil {
		return &StudioState{Projects: []*Project{}, Assets: []*Asset{}, Jobs: []*Job{}, Voices: []*Voice{}}
	}
	return e.store.State()
}

// SubmitJob 提交一条出片任务，立刻返回（实际执行在后台 worker）。
func (e *Engine) SubmitJob(projectID string, input map[string]any) (*Job, error) {
	if e.queue == nil {
		return nil, errNoStore
	}
	return e.queue.Submit(projectID, input)
}

// SetJobHandler 接上出片管线。
//
// 目前还没接：渲染与供应商层还在搬。接上之前提交任务会以明确错误失败，
// 而不是假装成功 —— 界面能看到「本地引擎还没有接上出片管线」。
func (e *Engine) SetJobHandler(handler JobHandler) {
	if e.queue == nil {
		return
	}
	e.queue.SetHandler(handler)
}

// ListJobs 列出任务（新→旧），界面轮询用。
func (e *Engine) ListJobs() []*Job {
	if e.queue == nil {
		return []*Job{}
	}
	return e.queue.Jobs()
}

// GetJob 按 id 取一条任务。
func (e *Engine) GetJob(jobID string) *Job {
	if e.queue == nil {
		return nil
	}
	return e.queue.Job(jobID)
}

// CancelJob 取消一条任务。返回 canceling（正在掐断）或 canceled（已取消）。
func (e *Engine) CancelJob(jobID string) (string, error) {
	if e.queue == nil {
		return "", errNoStore
	}
	return e.queue.Cancel(jobID)
}

// ──────────────────────────────────────────────────────────────
// 语音合成（从 voice.mjs / edge-tts.mjs / voxcraft-*.mjs 搬）
// ──────────────────────────────────────────────────────────────

// SpeakViaMinimax 用 MiniMax 云端 TTS 合成一段文字，落到本地 wav。
func (e *Engine) SpeakViaMinimax(text, out, voice string) error {
	return SpeakViaMinimax(context.Background(), text, out, voice)
}

// DurationsFromVoice 由念白时长推导镜头时长。
func (e *Engine) DurationsFromVoice(voiceDurations []float64, margin, min float64) []float64 {
	return DurationsFromVoice(voiceDurations, margin, min)
}

// LocateVoxcraft 定位本机 voxcraft 安装。
func (e *Engine) LocateVoxcraft() VoxcraftInfo {
	return LocateVoxcraft()
}

// StartVoxcraft 确保 voxcraft 服务可用，返回服务基址。
//
// 别人开着就复用，不重复起进程 —— 这是本机唯一的免费离线语音引擎。
func (e *Engine) StartVoxcraft(home, bin string) (string, error) {
	server, err := EnsureServer(home, bin)
	if err != nil {
		return "", err
	}
	e.mu.Lock()
	if e.voxcraft != nil {
		StopServer(e.voxcraft)
	}
	e.voxcraft = server
	e.mu.Unlock()
	return server.Base, nil
}

// StopVoxcraft 停掉自己起的 voxcraft 服务（复用的不动）。
func (e *Engine) StopVoxcraft() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.voxcraft != nil {
		StopServer(e.voxcraft)
		e.voxcraft = nil
	}
}

// SynthesizeVoiceBatch 用 voxcraft 批量合成念白，返回本地 wav 路径列表。
func (e *Engine) SynthesizeVoiceBatch(base, persona string, texts []string, outDir string) ([]string, error) {
	return SynthesizeViaServer(base, persona, texts, outDir)
}

var errNoStore = errors.New("工作台状态还没初始化")
