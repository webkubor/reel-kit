package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// stub 是一个假的本地工作台，只回应用原生界面要读的那几个接口。
func stub(t *testing.T, jobs []map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"projects": []map[string]any{{"id": "p1", "name": "我的第一支短片"}},
			"assets": []map[string]any{{
				"id": "a1", "projectId": "p1", "name": "H3 视频.mp4", "mime": "video/mp4",
				"width": 768, "height": 1344, "duration": 4.5, "engine": "MiniMax H3 视频",
				"requestedModel": "MiniMax-H3", "actualModel": "MiniMax-H3", "source": "MiniMax H3",
			}},
			"jobs": jobs,
		})
	})
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"video": map[string]any{
				"ready": true, "label": "视频生成", "note": "密钥配置不代表额度已验证",
				"channels": []map[string]any{
					{"id": "minimax", "label": "MiniMax 直连", "ready": true, "note": "走自己的套餐账户",
						"models": []map[string]any{{"id": "MiniMax-H3", "name": "MiniMax H3 · 4–15 秒 768P"}}},
					{"id": "museav", "label": "MuseAV 中台", "ready": false, "note": "请安装并登录 museav CLI",
						"models": []map[string]any{{"id": "Seedance 2.0 Mini", "name": "Seedance 2.0 Mini"}}},
				},
				"profiles": []map[string]any{{"id": "primary", "label": "主账户"}, {"id": "secondary", "label": "第二账户"}},
			},
			"image": map[string]any{"ready": true, "label": "图片生成", "channels": []map[string]any{
				{"id": "museav", "label": "MuseAV 中台", "ready": true, "models": []map[string]any{{"id": "gpt-image-2.6", "name": "gpt-image-2.6"}}},
			}},
			"voice": map[string]any{"ready": true, "label": "语音生成", "channels": []map[string]any{
				{"id": "minimax", "label": "MiniMax TTS", "ready": true,
					"models": []map[string]any{{"id": "speech-02-hd", "name": "speech-02-hd"}},
					"voices": []map[string]any{{"id": "male-qn-qingse", "name": "青涩男声"}, {"id": "female-shaonv", "name": "少女音"}}},
				{"id": "voxflow", "label": "VoxFlow · 本地", "ready": true,
					"voices": []map[string]any{{"id": "demo_narrator", "name": "默认旁白"}}},
			}},
			"customChannels": map[string]any{"file": "/root/.reel-kit/providers.json", "problems": []string{"seedream：环境变量 ARK_API_KEY 未注入，调用会被拒绝"}},
		})
	})
	mux.HandleFunc("/api/bgm", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}, "configured": true})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// newApp 造一个连着假后端的应用，并等它把状态拉回来。
func newApp(t *testing.T, base string) *App {
	t.Helper()
	a := New(Options{BaseURL: base})
	a.refresh()
	return a
}

func Test界面显示项目与素材(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if !tt.HasText("Reel Studio") {
		t.Errorf("界面里没有品牌名，实际文字：%q", tt.Texts())
	}
	if !tt.HasText("我的第一支短片") {
		t.Errorf("界面里没有项目名，实际文字：%q", tt.Texts())
	}
	if !tt.HasText("H3 视频.mp4") {
		t.Errorf("界面里没有素材名，实际文字：%q", tt.Texts())
	}
}

func Test通道未就绪时写明原因(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	// 选中未就绪的 MuseAV 通道
	a.mode = ModeVideo
	a.backend = "museav"
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if !tt.HasText("MuseAV 中台（未就绪）") {
		t.Errorf("未就绪的通道没有被标出来，实际文字：%q", tt.Texts())
	}
}

func Test自定义通道的配置问题要显示(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if !tt.HasText("自定义通道配置有问题") {
		t.Errorf("自定义通道的问题没有显示，实际文字：%q", tt.Texts())
	}
	if !tt.HasText("ARK_API_KEY 未注入") {
		t.Errorf("配置问题的具体原因没有显示，实际文字：%q", tt.Texts())
	}
}

func Test失败的付费任务给出取回入口(t *testing.T) {
	jobs := []map[string]any{{
		"id": "j1", "projectId": "p1", "status": "failed", "createdAt": "2026-01-01T00:00:00Z",
		"input":    map[string]any{"kind": "video", "prompt": "橘猫看雨"},
		"error":    "下载超时",
		"upstream": map[string]any{"channel": "minimax", "taskId": "task-123"},
	}}
	server := stub(t, jobs)
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	if !tt.HasText("取回已生成的素材") {
		t.Errorf("没有取回入口，实际文字：%q", tt.Texts())
	}
}

func Test没有上游任务的不给取回入口(t *testing.T) {
	jobs := []map[string]any{{
		"id": "j2", "projectId": "p1", "status": "failed", "createdAt": "2026-01-01T00:00:00Z",
		"input": map[string]any{"kind": "image", "prompt": "橘猫"},
		"error": "接口报错",
	}}
	server := stub(t, jobs)
	a := newApp(t, server.URL)
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()
	for _, text := range tt.Texts() {
		if strings.Contains(text, "取回已生成的素材") {
			t.Errorf("没有上游任务 ID 时不该出现取回入口：%q", text)
		}
	}
}

func Test模式切换会换通道组(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	if got := a.group(); got != "voice" {
		t.Fatalf("项目推荐模式应该走语音通道组：%s", got)
	}
	a.mode = ModeSpeech
	if got := a.group(); got != "voice" {
		t.Fatalf("配音模式的通道组不对：%s", got)
	}
	a.mode = ModeImage
	if got := a.group(); got != "image" {
		t.Fatalf("图片模式的通道组不对：%s", got)
	}
}

func Test通道选择不会被静默换掉(t *testing.T) {
	server := stub(t, nil)
	a := newApp(t, server.URL)
	a.mode = ModeVideo
	// 用户选了未就绪的 MuseAV
	a.backend = "museav"
	a.ensureBackend()
	if a.backend != "museav" {
		t.Fatalf("未就绪的通道被换掉了：%s", a.backend)
	}
	// 只有当通道彻底不见了，才换一条
	a.backend = "gone"
	a.ensureBackend()
	if a.backend == "gone" {
		t.Fatalf("通道不存在时没有换一条可用的：%s", a.backend)
	}
}
