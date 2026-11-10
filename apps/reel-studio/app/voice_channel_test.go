package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// stubVoiceChannels 造一个「语音通道」可配置的假工作台。
//
// voiceChannels 传 nil 时用默认的两条通道（本机就绪、云端就绪）。
func stubVoiceChannels(t *testing.T, voiceChannels []map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{{"id": "p1", "name": "我的第一支短片"}},
			"assets":   []map[string]any{},
			"jobs":     []map[string]any{},
		})
	})
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"video": map[string]any{"ready": true, "channels": []map[string]any{}},
			"image": map[string]any{"ready": true, "channels": []map[string]any{}},
			"voice": map[string]any{
				"ready": true, "label": "语音生成",
				"personas": []map[string]any{},
				"channels": voiceChannels,
			},
		})
	})
	mux.HandleFunc("/api/bgm", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}, "configured": true})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// Test配音通道两条都在界面上
//
// 桌面版原来把通道写死成本机 VoxFlow，Windows 装上了也出不了声。
// 现在两条通道都要在选择里，用户才有得选。
func Test配音通道两条都在界面上(t *testing.T) {
	server := stubVoiceChannels(t, []map[string]any{
		{"id": "voxflow", "label": "VoxFlow · 本地", "ready": true,
			"voices": []map[string]any{{"id": "demo_narrator", "name": "默认旁白"}}},
		{"id": "minimax", "label": "MiniMax TTS", "ready": true,
			"voices": []map[string]any{{"id": "female-shaonv", "name": "少女音"}}},
	})
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	// 下拉只画当前选中的那一条，所以看得到的是「现在这条通道」。
	if !tt.HasText("配音通道") {
		t.Errorf("声音这一段没有配音通道，实际文字：%q", tt.Texts())
	}
	if !tt.HasText("VoxFlow · 本地") {
		t.Errorf("本机通道没出现在界面上，实际文字：%q", tt.Texts())
	}
	// 两条通道都在待选列表里。
	channels := a.voiceChannels()
	if len(channels) != 2 || channels[0].ID != "voxflow" || channels[1].ID != "minimax" {
		t.Fatalf("待选通道不对：%+v", channels)
	}
	// 两条都就绪时默认走本机：声音不出本机，也不花钱。
	if a.talkVoice != "voxflow" {
		t.Errorf("默认该选本机通道，实际 %q", a.talkVoice)
	}
	if a.talkPersona != "demo_narrator" {
		t.Errorf("默认音色该是本机音色，实际 %q", a.talkPersona)
	}
}

// Test本机没就绪时改走云端
//
// 本机语音服务没起来时，界面不该只有一条死路：
// 云端通道就绪就能用，音色列表也要换成云端的。
func Test本机没就绪时改走云端(t *testing.T) {
	server := stubVoiceChannels(t, []map[string]any{
		{"id": "voxflow", "label": "VoxFlow · 本地", "ready": false, "note": "服务未启动",
			"voices": []map[string]any{}},
		{"id": "minimax", "label": "MiniMax TTS", "ready": true,
			"voices": []map[string]any{{"id": "female-shaonv", "name": "少女音"}}},
	})
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if !tt.HasText("少女音") {
		t.Errorf("本机没就绪时该用云端音色，实际文字：%q", tt.Texts())
	}
	if a.talkVoice != "minimax" {
		t.Errorf("默认该落在就绪的云端通道，实际是 %q", a.talkVoice)
	}
}

// Test通道没就绪不提交
//
// 没就绪还提交，用户只会拿到一句 fetch failed。
// 提交前就该说清楚哪条通道没就绪、为什么。
func Test通道没就绪不提交(t *testing.T) {
	server := stubVoiceChannels(t, []map[string]any{
		{"id": "voxflow", "label": "VoxFlow · 本地", "ready": false, "note": "服务未启动",
			"voices": []map[string]any{}},
	})
	a := newApp(t, server.URL)
	a.talkTitle = "测试标题"
	a.talkCaptions = "第一句\n第二句"
	a.talkPersona = "demo_narrator"
	a.talkVoice = "voxflow"
	a.submitTalk()
	a.drainPending()
	if !a.noticeErr {
		t.Fatalf("通道没就绪时不该提交成功，notice=%q", a.notice)
	}
	if a.notice == "" {
		t.Error("通道没就绪时应给出原因")
	}
}

// Test提交时带上选中的配音通道
//
// 界面选了云端通道，提交出去的配置里就得是云端 ——
// 否则界面说一套、工作台跑另一条，用户拿到的东西和配置对不上。
func Test提交时带上选中的配音通道(t *testing.T) {
	var submitted map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{{"id": "p1", "name": "我的第一支短片"}},
			"assets":   []map[string]any{},
			"jobs":     []map[string]any{},
		})
	})
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"voice": map[string]any{"ready": true, "channels": []map[string]any{
				{"id": "minimax", "label": "MiniMax TTS", "ready": true,
					"voices": []map[string]any{{"id": "female-shaonv", "name": "少女音"}}},
			}},
		})
	})
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&submitted)
		json.NewEncoder(w).Encode(map[string]any{"id": "j1", "status": "queued"})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	a := newApp(t, server.URL)
	a.talkTitle = "测试标题"
	a.talkCaptions = "第一句\n第二句"
	a.talkVoice = "minimax"
	a.talkPersona = "female-shaonv"
	a.submitTalk()
	for i := 0; i < 50 && submitted == nil; i++ {
		a.drainPending()
		time.Sleep(20 * time.Millisecond)
	}
	if submitted == nil {
		t.Fatal("没有提交出任务")
	}
	if submitted["voiceBackend"] != "minimax" {
		t.Errorf("提交时配音通道不对，实际 %v", submitted["voiceBackend"])
	}
	if submitted["persona"] != "female-shaonv" {
		t.Errorf("提交时音色不对，实际 %v", submitted["persona"])
	}
}

// Test默认走随应用自带的离线通道
//
// 「下载下来直接用」这条底线：装完应用就该能配音，
// 不该要求用户先 clone 一个仓库、装 Python，或者先填一个云端密钥。
// 离线通道排在最前且就绪时，它就该是默认。
func Test默认走随应用自带的离线通道(t *testing.T) {
	server := stubVoiceChannels(t, []map[string]any{
		{"id": "sherpa", "label": "本机 · 离线（自带）", "ready": true,
			"voices": []map[string]any{{"id": "zh-fanchen-c", "name": "中文女声 · 离线"}}},
		{"id": "voxflow", "label": "VoxFlow · 本地", "ready": false, "note": "服务未启动",
			"voices": []map[string]any{}},
		{"id": "minimax", "label": "MiniMax TTS", "ready": false, "note": "还没配密钥",
			"voices": []map[string]any{}},
	})
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if a.talkVoice != "sherpa" {
		t.Errorf("默认该选随应用自带的离线通道，实际 %q", a.talkVoice)
	}
	if a.talkPersona != "zh-fanchen-c" {
		t.Errorf("默认音色该是离线音色，实际 %q", a.talkPersona)
	}
	if !tt.HasText("中文女声 · 离线") {
		t.Errorf("离线音色没出现在界面上，实际文字：%q", tt.Texts())
	}
}

// Test离线通道说清楚不花钱不联网
//
// 选了哪条通道，界面就得说清这条通道的钱和隐私成本。
// 离线通道的说法要跟「随应用自带、离线合成」对得上。
func Test离线通道说清楚不花钱不联网(t *testing.T) {
	if !strings.Contains(voiceChannelHelp("sherpa"), "不产生费用") {
		t.Errorf("离线通道的帮助文字没说清费用：%q", voiceChannelHelp("sherpa"))
	}
	if !strings.Contains(voiceChannelHelp("sherpa"), "不联网") {
		t.Errorf("离线通道的帮助文字没说清隐私：%q", voiceChannelHelp("sherpa"))
	}
	server := stubVoiceChannels(t, []map[string]any{
		{"id": "sherpa", "label": "本机 · 离线（自带）", "ready": true,
			"voices": []map[string]any{{"id": "zh-fanchen-c", "name": "中文女声 · 离线"}}},
	})
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if !strings.Contains(a.voicePipelineHint("sherpa"), "不产生费用") {
		t.Errorf("流水线的配音说明没说清费用：%q", a.voicePipelineHint("sherpa"))
	}
}

// TestVoxFlow没连上不再说配音用不了
//
// 以前 VoxFlow 没起来就直接说「配音、克隆自己的声音都用不了」。
// 现在出片靠随应用自带的离线引擎，VoxFlow 只管克隆声音和设计音色 ——
// 再说「用不了」就是把用户往一条死路上引。
func TestVoxFlow没连上不再说配音用不了(t *testing.T) {
	server := stubVoiceChannels(t, []map[string]any{
		{"id": "voxflow", "label": "VoxFlow · 本地", "ready": false, "note": "服务未启动",
			"voices": []map[string]any{}},
	})
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if tt.HasText("都用不了") {
		t.Errorf("还写着「都用不了」，实际文字：%q", tt.Texts())
	}
	if !tt.HasText("出片不受影响") {
		t.Errorf("该说明 VoxFlow 缺失不影响出片，实际文字：%q", tt.Texts())
	}
}

// Test保存密钥只动一个键
//
// ~/.reel-kit/config.json 是用户的配置文件（里面还有配乐清单），
// 写密钥不能把别的键冲掉。
func Test保存密钥只动一个键(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := `{"minimaxApiKey":"old","bgmManifest":{"items":["a.mp3"]}}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	old := reelConfigPathFn
	reelConfigPathFn = func() string { return path }
	defer func() { reelConfigPathFn = old }()

	if err := saveMinimaxKey("  new-key  "); err != nil {
		t.Fatalf("保存失败：%v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["minimaxApiKey"] != "new-key" {
		t.Errorf("密钥没写对，实际 %v", cfg["minimaxApiKey"])
	}
	if _, ok := cfg["bgmManifest"]; !ok {
		t.Error("保存密钥把别的键冲掉了")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("配置文件权限应是 0600，实际 %o", perm)
	}
	configured, tail := minimaxKeyConfigured()
	if !configured || tail != "****-key" {
		t.Errorf("界面显示的密钥状态不对：%v %q", configured, tail)
	}
}
