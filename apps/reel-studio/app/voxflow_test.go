package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestVoxFlow提示按机器给不同内容
//
// 「跑不起来的机器」那条文案只会在非 Apple 芯片的 Mac 上出现 ——
// 如果只靠 UI 渲染来测，它在 Mac 上永远测不到，得等 CI 才报。
// 所以这里直接测选文案的那个纯函数，两条分支都过一遍。
func TestVoxFlow提示按机器给不同内容(t *testing.T) {
	能装, 给命令 := voxflowHintText(true)
	if !给命令 {
		t.Error("Apple 芯片的 Mac 上该摆出安装与启动命令")
	}
	for _, want := range []string{"install.sh", "voice web", "自带"} {
		if !strings.Contains(能装, want) {
			t.Errorf("能装 VoxFlow 的机器上提示少了 %q，实际：%q", want, 能装)
		}
	}

	装不了, 不给命令 := voxflowHintText(false)
	if 不给命令 {
		t.Error("装不了 VoxFlow 的机器上不该摆安装命令")
	}
	for _, unwanted := range []string{"install.sh", "voice web"} {
		if strings.Contains(装不了, unwanted) {
			t.Errorf("装不了 VoxFlow 的机器上不该出现 %q，实际：%q", unwanted, 装不了)
		}
	}
	// 本机装不了不等于用不了：远端跑语音服务这条路任何系统都能走，
	// 提示里必须说清楚，否则用户以为整条路断了。
	for _, want := range []string{"自带", "Apple 芯片的 Mac 装得起来", "REEL_VOXFLOW_URL"} {
		if !strings.Contains(装不了, want) {
			t.Errorf("装不了 VoxFlow 的机器上提示少了 %q，实际：%q", want, 装不了)
		}
	}
}

// Test本机语音没连上时给出装法
//
// 随应用自带的离线引擎兜住了出片，VoxFlow 变成可选（用于克隆声音与设计音色）。
// 提示要告诉用户：没连上不影响出片。
//
// 装与起的命令只在 Apple 芯片的 Mac 上摆出来（VoxFlow 的后端是本地 MLX 模型，
// 别处装不起来）；别的机器上要说清「这台机器用不了」，而不是给一条注定失败的命令。
// 这条用例在 Linux CI 上也跑，所以按平台分别断言。
func Test本机语音没连上时给出装法(t *testing.T) {
	server := stubNoLocalVoice(t)
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if !tt.HasText("出片不受影响") {
		t.Errorf("本机语音没就绪时该说明出片不受影响，实际文字：%q", tt.Texts())
	}
	if voxflowRunsHere() {
		for _, want := range []string{
			"install.sh",
			"voice web",
			"复制安装命令",
			"复制启动命令",
		} {
			if !tt.HasText(want) {
				t.Errorf("本机语音没就绪时少了 %q，实际文字：%q", want, tt.Texts())
			}
		}
		return
	}
	for _, unwanted := range []string{"install.sh", "voice web"} {
		if tt.HasText(unwanted) {
			t.Errorf("这台机器装不了 VoxFlow，不该出现 %q，实际文字：%q", unwanted, tt.Texts())
		}
	}
	for _, want := range []string{"随应用自带的本机离线引擎", "这台机器用不了"} {
		if !tt.HasText(want) {
			t.Errorf("这台机器用不了 VoxFlow 时该说明白，少了 %q，实际文字：%q", want, tt.Texts())
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
