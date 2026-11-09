package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Test密钥输入框不回显
//
// 密钥是能直接花钱的东西。输入框要是明文，录个屏、截个图、
// 旁边有人瞄一眼，密钥就出去了。
// 界面上要把它当密码框处理，保存后也只显示末四位。
func Test密钥输入框不回显(t *testing.T) {
	// 状态读的是本机配置，换成临时文件：不然这台机器配没配密钥
	// 会决定界面长什么样，密钥尾号还会漏进截图与断言里。
	old := reelConfigPathFn
	reelConfigPathFn = func() string { return filepath.Join(t.TempDir(), "config.json") }
	t.Cleanup(func() { reelConfigPathFn = old })

	server := stubVoiceChannels(t, []map[string]any{
		{"id": "voxflow", "label": "VoxFlow · 本地", "ready": true,
			"voices": []map[string]any{{"id": "demo_narrator", "name": "默认旁白"}}},
	})
	a := newApp(t, server.URL)
	a.mode = ModeSettings
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	var keyField *ui.AccessInfo
	for _, info := range tt.AccessAll() {
		if info.Role == ui.RoleTextField && strings.Contains(info.Placeholder, "sk-") {
			f := info
			keyField = &f
		}
	}
	if keyField == nil {
		t.Fatalf("设置页没找到 MiniMax 密钥输入框，实际控件：%+v", tt.AccessAll())
	}
	if !keyField.Password {
		t.Error("密钥输入框是明文的，应该按密码框处理")
	}
}

// Test配音通道说明不说「不出声」
//
// 1.1.0 的说明写的是「本机推理不花钱也不出声」。
// 「不出声」在配音这件事上意思完全反了 ——
// 用户看到的是「这条通道不会出声」，于是不敢选本机。
// 原意是「声音不出本机」，这句话要说准。
func Test配音通道说明不说反话(t *testing.T) {
	server := stubVoiceChannels(t, []map[string]any{
		{"id": "voxflow", "label": "VoxFlow · 本地", "ready": true,
			"voices": []map[string]any{{"id": "demo_narrator", "name": "默认旁白"}}},
	})
	a := newApp(t, server.URL)
	a.mode = ModeTalk
	tt := ui.NewTester(a.View, 1280, 840)
	tt.Frame()

	texts := tt.Texts()
	joined := strings.Join(texts, "\n")
	if strings.Contains(joined, "不出声") {
		t.Errorf("界面里还有「不出声」这种把意思说反的写法：%q", texts)
	}
	if !strings.Contains(joined, "声音不出本机") {
		t.Errorf("配音通道说明没写「声音不出本机」，实际文字：%q", texts)
	}
}

// Test配置文件坏了不许被覆盖
//
// 用户的 ~/.reel-kit/config.json 里还有别的键（bgmManifest 等）。
// 文件读不出来的时候要是当成空配置接着写，那些内容就没了。
// 保存密钥这时必须失败，并且一个字都不许动。
func Test配置文件坏了不许被覆盖(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	broken := []byte("{ 这不是 JSON")
	if err := os.WriteFile(path, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	old := reelConfigPathFn
	reelConfigPathFn = func() string { return path }
	t.Cleanup(func() { reelConfigPathFn = old })

	if err := saveMinimaxKey("sk-test-key"); err == nil {
		t.Fatal("配置文件读不出来时保存应该失败")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(broken) {
		t.Errorf("坏掉的配置文件被改了：%q", after)
	}
}

// Test重启工作台的说法要对得上事实
//
// 保存密钥之后会说「已重启」。这句话只有在真的重启了工作台时才成立：
// 复用了一个已经在跑的工作台（我们没重启它），密钥未必生效；
// 工作台压根没起来，更不能说成功。
func Test重启工作台的说法要对得上事实(t *testing.T) {
	cases := []struct {
		name    string
		status  BackendStatus
		wantErr bool
		want    string
	}{
		{
			name:   "自己起的，已重启",
			status: BackendStatus{State: "ready", Note: "已启动本机工作台", Started: true},
			want:   "密钥已保存，本机工作台已重启",
		},
		{
			name:   "复用别人的工作台",
			status: BackendStatus{State: "ready", Note: "已连接本机工作台", Started: false},
			want:   "请自己重启它才会生效",
		},
		{
			name:    "工作台没起来",
			status:  BackendStatus{State: "failed", Note: "本地工作台 20 秒内没有起来，请看日志"},
			wantErr: true,
			want:    "本地工作台 20 秒内没有起来",
		},
	}
	for _, c := range cases {
		notice, isErr := restartNotice(c.status)
		if isErr != c.wantErr {
			t.Errorf("%s：报错标记不对，%v", c.name, isErr)
		}
		if !strings.Contains(notice, c.want) {
			t.Errorf("%s：提示不对，实际 %q", c.name, notice)
		}
	}
}
