package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Engine 通过 Go 绑定提供业务能力，无 Node 运行时和本地 HTTP 业务服务。
// 任务与素材落盘；本机制作启动随包的语音、渲染和编码子进程。
// 显式的 AI 分镜请求由 Go 连接用户配置的外部服务。
type Engine struct {
	mu   sync.Mutex
	note string

	store     *StudioStore
	queue     *JobQueue
	cancel    context.CancelFunc
	aiSecrets secretStore
	aiClient  *http.Client

	// voxcraft：本机离线语音引擎的服务句柄（可能复用已有的）。
	voxcraft *VoxcraftServer

	// speechOverride 是测试替身：非 nil 时 GenerateSpeech 走它。
	// 生产路径永远是 nil —— 真实通道（VoxFlow / MiniMax）不受影响。
	speechOverride func(ctx context.Context, projectID, text, tone, backend, model, voiceID, profile string, speed float64) (*Asset, error)
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
	e.queue = NewJobQueue(store, e.runPipeline)
	return e
}

// Status 返回当前状态：直接就绪，不启动任何子进程。
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{Ready: e.store != nil && e.queue != nil, URL: "", Note: e.note, Error: ""}
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
	out := []*Project{}
	for _, p := range e.store.State().Projects {
		cp := *p
		out = append(out, &cp)
	}
	return out
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
	if e.queue == nil || e.store == nil {
		return nil, errNoStore
	}
	if _, err := e.store.Project(projectID); err != nil {
		return nil, err
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
//
// 只暴露「查状态」，不暴露「写文件」——
// 绑定参数里的路径与地址如果由前端给，界面就能往任意位置写文件、
// 向任意地址发请求。合成这件事由出片流水线自己调。
// ──────────────────────────────────────────────────────────────

// VoiceChannel 语音通道的信息。
type VoiceChannel struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Origin string `json:"origin"`
	Engine string `json:"engine"`
	Ready  bool   `json:"ready"`
	Note   string `json:"note"`
}

// VoiceChannels 列出可用的语音通道。
func (e *Engine) VoiceChannels() []VoiceChannel {
	vox := LocateVoxcraft()
	_, _, _, _, _, sherpaErr := sherpaFiles()
	channels := []VoiceChannel{
		{
			ID: "sherpa", Label: "本机离线语音", Origin: "local-offline",
			Engine: "sherpa-onnx", Ready: sherpaErr == nil,
			Note: "本机语音模型资源未就绪",
		},
		{
			ID: "voxflow", Label: "VoxFlow 本机语音",
			Origin: "local-offline", Engine: "VoxFlow 本地语音",
			Ready: vox.OK, Note: vox.Reason,
		},
		{
			ID: "minimax", Label: "MiniMax TTS",
			Origin: "cloud-paid", Engine: "MiniMax TTS",
			Ready: MinimaxAPIKey() != "",
			Note:  minimaxNote(),
		},
		{
			ID: "edge", Label: "Edge 在线语音",
			Origin: "free-online", Engine: "Edge 在线高保真语音",
			Ready: false,
			Note:  "这个版本还没有 Go 版实现，暂时不可用",
		},
	}
	return channels
}

// minimaxNote 说明 MiniMax 现在能不能用。
func minimaxNote() string {
	if MinimaxAPIKey() != "" {
		return "已配置密钥，按量计费"
	}
	return "未配置 MINIMAX_API_KEY"
}

// LocateVoxcraft 定位本机 voxcraft 安装。
func (e *Engine) LocateVoxcraft() VoxcraftInfo {
	return LocateVoxcraft()
}

// DurationsFromVoice 由念白时长推导镜头时长。
func (e *Engine) DurationsFromVoice(voiceDurations []float64, margin, min float64) []float64 {
	return DurationsFromVoice(voiceDurations, margin, min)
}

// ProductLayoutInfo 一种画幅的版式信息，给界面下拉用。
type ProductLayoutInfo struct {
	Ratio    string `json:"ratio"`
	Template string `json:"template"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Label    string `json:"label"`
}

// ProductLayoutsInfo 列出支持的画幅。
func (e *Engine) ProductLayoutsInfo() []ProductLayoutInfo {
	labels := map[string]string{"9:16": "竖版 · 1080×1920", "16:9": "横版 · 1920×1080"}
	out := []ProductLayoutInfo{}
	for _, ratio := range []string{"9:16", "16:9"} {
		l := ProductLayouts[ratio]
		out = append(out, ProductLayoutInfo{
			Ratio: ratio, Template: l.Template, Width: l.Width, Height: l.Height,
			Label: labels[ratio],
		})
	}
	return out
}

// ──────────────────────────────────────────────────────────────
// 渲染（从 render.mjs / motion.mjs 搬）
// ──────────────────────────────────────────────────────────────

// RenderRequest 前端发来的渲染请求。
//
// 与内部的 RenderOpts 分开：RenderOpts 带 OnProgress 回调，
// 不是可 JSON 序列化的东西，不能直接当绑定参数用。
type RenderRequest struct {
	// Template 是版式名（不是路径）—— 界面只能从已知清单里选，
	// 不能让它去读任意文件。
	Template  string         `json:"template"`
	Shots     []Shot         `json:"shots"`
	Vars      map[string]any `json:"vars"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	Motion    string         `json:"motion"`
	Durations []float64      `json:"durations"`
	FPS       int            `json:"fps"`
}

// RenderFrames 渲染逐镜画面（静帧出 PNG，动效出 mp4）。
func (e *Engine) RenderFrames(req RenderRequest) ([]string, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	content, err := ReadTemplate(req.Template)
	if err != nil {
		return nil, err
	}
	temp, err := os.MkdirTemp("", "reel-render-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	templateFile := filepath.Join(temp, req.Template+".html")
	if err := os.WriteFile(templateFile, []byte(content), 0o600); err != nil {
		return nil, err
	}
	// 输出目录固定在工作台目录下，不接受前端指定 ——
	// 否则界面能往任意路径写文件。
	outDir := filepath.Join(e.store.Root(), "renders", newID())
	return RenderFrames(context.Background(), RenderOpts{
		Template:  templateFile,
		Shots:     req.Shots,
		Vars:      req.Vars,
		OutDir:    outDir,
		Width:     req.Width,
		Height:    req.Height,
		Motion:    req.Motion,
		Durations: req.Durations,
		FPS:       req.FPS,
	})
}

var errNoStore = errors.New("工作台状态还没初始化")
