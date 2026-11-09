package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// talkJobs 造两条知识口播的视频：一条已完成、一条在跑。
func talkJobs() []map[string]any {
	return []map[string]any{
		{
			"id": "j1", "projectId": "p1", "status": "succeeded", "createdAt": "2026-11-01T00:00:00Z",
			"assetId": "a1",
			"input": map[string]any{
				"kind": "talk", "title": "第一条口播", "captions": "第一句\n第二句",
				"persona": "demo_narrator", "style": "paper", "ratio": "9:16",
				"bgmGain": 0.12,
			},
		},
		{
			"id": "j2", "projectId": "p1", "status": "running", "createdAt": "2026-11-02T00:00:00Z",
			"message": "正在渲染画面帧…",
			"input": map[string]any{
				"kind": "talk", "title": "第二条口播", "captions": "第一句\n第二句\n第三句",
				"persona": "demo_narrator", "style": "dark", "ratio": "9:16",
				"bgmGain": 0.2,
			},
			"steps": []map[string]any{
				{"id": "config", "label": "确认配置", "status": "done"},
				{"id": "voice-0", "label": "第 1 句配音", "status": "done"},
				{"id": "voice-1", "label": "第 2 句配音", "status": "running"},
			},
		},
	}
}

// Test流水线显示视频条数与列表
//
// 用户反馈「看不到一个流水线跑出来的视频个数和视频列表」——
// 条数要出现在左栏，列表要出现在右栏。
func Test流水线显示视频条数与列表(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	if !tt.HasText("知识口播") {
		t.Errorf("左栏没有流水线入口，实际文字：%q", tt.Texts())
	}
	if got := a.pipelineCount("talk"); got != 2 {
		t.Errorf("流水线的视频条数不对：%d", got)
	}
	if !tt.HasText("第一条口播") || !tt.HasText("第二条口播") {
		t.Errorf("右栏没有列出这条流水线的视频，实际文字：%q", tt.Texts())
	}
	if !tt.HasText("2 条") {
		t.Errorf("没有显示视频条数，实际文字：%q", tt.Texts())
	}
}

// Test生成中的视频显示进度
//
// 「视频进度条也没有」——跑到哪一步必须看得见。
func Test生成中的视频显示进度(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	if !tt.HasText("第 2 句配音") {
		t.Errorf("没有显示当前进度说明，实际文字：%q", tt.Texts())
	}
	value, _, determinate := a.jobProgress(jobByID(a.pipelineJobs("talk"), "j2"))
	if !determinate {
		t.Errorf("口播这种有步骤的任务应该能算出确定进度")
	}
	if value <= 0 || value >= 1 {
		t.Errorf("进度值不合理：%v", value)
	}
}

// Test生成配置按七块展示
func Test生成配置按七块展示(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	if !tt.HasText("生成配置") {
		t.Errorf("没有显示生成配置区块，实际文字：%q", tt.Texts())
	}
	for _, part := range []string{partScript, partVoice, partVisual, partLayout, partFormat, partMusic, partOutput} {
		if !tt.HasText(part) {
			t.Errorf("生成配置里少了「%s」这一块，实际文字：%q", part, tt.Texts())
		}
	}
	if !tt.HasText("第一条口播") {
		t.Errorf("配置面板没有显示标题的实际值")
	}
}

// Test两条视频的配置能对比
func Test两条视频的配置能对比(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	a.selectedJobID = "j1"
	a.compareJobID = "j2"
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	if !tt.HasText("配置对比") {
		t.Errorf("没有显示配置对比区块，实际文字：%q", tt.Texts())
	}
	// 标题与风格不同，应该出现在差异里
	if !tt.HasText("标题") {
		t.Errorf("差异里没有列出标题，实际文字：%q", tt.Texts())
	}
}

// Test按此配置再生成一条会把配置载入表单
func Test按此配置再生成一条会把配置载入表单(t *testing.T) {
	server := stub(t, talkJobs())
	a := newApp(t, server.URL)
	job := jobByID(a.pipelineJobs("talk"), "j1")
	a.applyConfig("talk", job.Input, job.ID)

	if a.talkTitle != "第一条口播" {
		t.Errorf("标题没有载入表单：%s", a.talkTitle)
	}
	if a.talkCaptions != "第一句\n第二句" {
		t.Errorf("脚本没有载入表单：%q", a.talkCaptions)
	}
	if a.talkPersona != "demo_narrator" {
		t.Errorf("音色没有载入表单：%s", a.talkPersona)
	}
	if a.adjustingJobID != job.ID {
		t.Errorf("没有记住正在调整哪条视频：%s", a.adjustingJobID)
	}
}

// Test进度按脚本行数推算
//
// 总步数在跑之前是不知道的，但脚本行数在提交前就知道 ——
// 进度条要按这个推算才准。
func Test进度按脚本行数推算(t *testing.T) {
	// 四镜口播：确认配置 + 占位图 + 四句配音 + 画面帧 + 合成 = 8 步
	input := map[string]any{"kind": "talk", "captions": "一\n二\n三\n四"}
	if got := expectedSteps("talk", input); got != 8 {
		t.Errorf("四镜口播的推算步数不对：%d", got)
	}
	// 有画面素材时不用占位图，少一步
	input["assetIds"] = []any{"a1"}
	if got := expectedSteps("talk", input); got != 7 {
		t.Errorf("有画面素材时不该多算占位图那一步：%d", got)
	}
}
