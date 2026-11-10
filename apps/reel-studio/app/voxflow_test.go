package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egoist/mygo/ui"
)

// stubNoLocalVoice 是一个「本机语音服务没启动」的假后端。
func stubNoLocalVoice(t *testing.T) *httptest.Server {
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
				"ready":    false,
				"label":    "语音生成",
				"personas": []map[string]any{},
				"channels": []map[string]any{
					{"id": "voxflow", "label": "VoxFlow · 本地", "ready": false,
						"note": "服务未启动", "voices": []map[string]any{}},
				},
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

// Test本机语音没连上时给出装法
//
// 随应用自带的离线引擎兜住了出片，VoxFlow 变成可选（用于克隆声音与设计音色）。
// 提示要告诉用户：没连上不影响出片，只有需要克隆声音时才去装；
// 装与起的命令依然摆出来，点复制就能走。
func Test本机语音没连上时给出装法(t *testing.T) {
	server := stubNoLocalVoice(t)
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	for _, want := range []string{
		"出片不受影响",
		"install.sh",
		"voice web",
		"复制安装命令",
		"复制启动命令",
	} {
		if !tt.HasText(want) {
			t.Errorf("本机语音没就绪时少了 %q，实际文字：%q", want, tt.Texts())
		}
	}
}

// Test本机语音就绪时不打扰
//
// 服务正常时不该出现安装说明。
func Test本机语音就绪时不打扰(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if tt.HasText("install.sh") {
		t.Errorf("本机语音就绪时不该出现安装说明，实际文字：%q", tt.Texts())
	}
}
