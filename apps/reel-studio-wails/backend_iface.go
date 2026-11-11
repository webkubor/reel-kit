package main

import "context"

// 出片后端接口。
//
// Node 版 providers.mjs 的 createProviders() 返回一个大对象，界面与
// product/talk/recommendation 三个编排只用得到四个方法。桌面版把它收成
// 这一个接口：
//
//   - Generate     生成一段视频或一张图
//   - RegisterVoice 注册一条音色
//   - Resume       拿已提交的上游任务把结果取回来（提交后下载失败时用，
//     不重新提交 —— 重新提交等于再付一遍钱）
//   - Status       各通道的可用状态
//
// 为什么要收成接口：出片管线（product/talk/recommendation）只依赖这四个
// 方法，不依赖具体走哪个通道。以后加通道只改实现，不改编排。
type VideoBackend interface {
	Generate(ctx context.Context, input map[string]any, progress ProgressFunc) (*Asset, error)
	RegisterVoice(ctx context.Context, req VoiceRegistration) (*RegisteredVoice, error)
	Resume(ctx context.Context, upstream map[string]any, input map[string]any, progress ProgressFunc) (*Asset, error)
	Status(ctx context.Context) (*ProviderStatus, error)
}

// ProgressFunc 进度回报。
type ProgressFunc func(text string)

// VoiceRegistration 注册音色的入参。
type VoiceRegistration struct {
	Bytes         []byte
	MIME          string
	Name          string
	ReferenceText string
	Instruction   string
}

// RegisteredVoice 注册音色后的结果。
type RegisteredVoice struct {
	Persona string `json:"persona"`
	Name    string `json:"name"`
}

// ChannelStatus 单个通道的状态。
type ChannelStatus struct {
	ID               string         `json:"id"`
	Label            string         `json:"label"`
	Ready            bool           `json:"ready"`
	Note             string         `json:"note"`
	Models           []ChannelModel `json:"models"`
	AllowCustomModel bool           `json:"allowCustomModel"`
}

// ChannelModel 通道可选的模型。
type ChannelModel struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Limit      string  `json:"limit,omitempty"`
	MinSeconds float64 `json:"minSeconds,omitempty"`
	MaxSeconds float64 `json:"maxSeconds,omitempty"`
}

// ProviderStatus 全通道状态。
type ProviderStatus struct {
	Video ChannelKind `json:"video"`
	Image ChannelKind `json:"image"`
	Voice ChannelKind `json:"voice"`
}

// ChannelKind 一类通道（视频/图片/语音）的状态。
type ChannelKind struct {
	Ready    bool            `json:"ready"`
	Label    string          `json:"label"`
	Note     string          `json:"note"`
	Profiles []QuotaProfile  `json:"profiles,omitempty"`
	Channels []ChannelStatus `json:"channels"`
}

// QuotaProfile 账户额度。
type QuotaProfile struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Key   string `json:"key"`
	Quota string `json:"quota,omitempty"`
	Note  string `json:"note,omitempty"`
}
