package main

import "testing"

// 提供方默认 MUSE AV，可切到直连；切换要落盘。
func TestAIProviderSwitch(t *testing.T) {
	s := newTestStore(t)
	e := &Engine{store: s}
	status := e.GetAIProviderStatus()
	if status.Provider != ProviderMuseAV {
		t.Fatalf("默认应是 MUSE AV：%s", status.Provider)
	}
	if _, err := e.SetAIProvider(ProviderDirect); err != nil {
		t.Fatal(err)
	}
	if e.GetAIProviderStatus().Provider != ProviderDirect {
		t.Fatal("切换没生效")
	}
	reopened, err := NewStudioStoreRoot(s.Root())
	if err != nil {
		t.Fatal(err)
	}
	if (&Engine{store: reopened}).GetAIProviderStatus().Provider != ProviderDirect {
		t.Fatal("切换没落盘")
	}
	if _, err := e.SetAIProvider("别的"); err == nil {
		t.Fatal("非法提供方应拒绝")
	}
}

// 模型族：H3 走视频适配，其余按对话模型。
func TestDetectModelFamily(t *testing.T) {
	if detectModelFamily("MiniMax-H3") != ModelFamilyH3 {
		t.Fatal("H3 应识别为视频族")
	}
	if detectModelFamily("gpt-4o-mini") != ModelFamilyChat {
		t.Fatal("普通模型应是对话族")
	}
}

// H3 请求体：content 数组带角色标签，参考图走 reference_image。
func TestBuildH3Body(t *testing.T) {
	body := buildH3Body("提示词", []string{"https://x/1.jpg", "", "https://x/2.jpg"}, 6, "16:9")
	if body.Model != h3WireModel {
		t.Fatalf("线上模型名：%s", body.Model)
	}
	if len(body.Content) != 3 {
		t.Fatalf("content 项数：%d", len(body.Content))
	}
	if body.Content[0].Type != "text" || body.Content[0].Text != "提示词" {
		t.Fatal("第一项应是提示词")
	}
	for _, c := range body.Content[1:] {
		if c.Type != "image_url" || c.Role != "reference_image" || c.Media == nil || c.Media.URL == "" {
			t.Fatalf("参考图项不对：%+v", c)
		}
	}
	if body.Duration != 6 || body.Ratio != "16:9" {
		t.Fatalf("规格不对：%+v", body)
	}
}

// HTTP 200 也可能是业务失败：base_resp 与 error 两种形状都要认。
func TestH3BusinessFailure(t *testing.T) {
	if err := h3BusinessFailure([]byte(`{"base_resp":{"status_code":1004,"status_msg":"余额不足"}}`)); err == nil {
		t.Fatal("base_resp 失败没识别")
	}
	if err := h3BusinessFailure([]byte(`{"error":{"type":"x","message":"被拒"}}`)); err == nil {
		t.Fatal("error 失败没识别")
	}
	if err := h3BusinessFailure([]byte(`{"base_resp":{"status_code":0},"id":"v1"}`)); err != nil {
		t.Fatalf("成功响应被误判：%v", err)
	}
}
