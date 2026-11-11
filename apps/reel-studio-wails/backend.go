package main

import "sync"

// Engine 是桌面壳的本地引擎：零 HTTP，零 Node，不启动任何子进程、不占端口。
// 所有状态留在内存里，通过 Go 绑定直接让界面调用。
//
// Phase 2 会把合成/TTS/供应商等逻辑从 Node 层搬进来，直接以 Go 方法
// 挂到 Engine 上，不再依赖任何外部 Node 进程或运行时目录。
type Engine struct {
	mu   sync.Mutex
	note string
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
	return &Engine{note: "本地工作台已就绪"}
}

// Status 返回当前状态：直接就绪，不启动任何子进程。
func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{Ready: true, URL: "", Note: e.note, Error: ""}
}

// Stop 是窗口退出钩子：没有子进程要回收。
func (e *Engine) Stop() {}

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
