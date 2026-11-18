package main

import "testing"

// 引导状态：默认提供方是 MUSE AV，检查项要含中台登录态。
func TestSetupStateIncludesMuseavCheck(t *testing.T) {
	s := newTestStore(t)
	e := &Engine{store: s}
	state := e.GetSetupState()
	if state.Provider != ProviderMuseAV {
		t.Fatalf("默认提供方：%s", state.Provider)
	}
	found := false
	for _, c := range state.Checks {
		if c.Key == "museav" {
			found = true
			if c.Ready && c.Detail == "" {
				t.Fatal("就绪时要说明状态")
			}
			if !c.Ready && c.Action == "" {
				t.Fatal("未就绪时要给出怎么补")
			}
		}
	}
	if !found {
		t.Fatal("引导里没有中台检查项")
	}
	if state.ProjectCount != 0 {
		t.Fatalf("新工作台项目数：%d", state.ProjectCount)
	}
}

// 切到直连后，引导里要出现直连检查项。
func TestSetupStateIncludesDirectCheckWhenSelected(t *testing.T) {
	s := newTestStore(t)
	e := &Engine{store: s}
	if _, err := e.SetAIProvider(ProviderDirect); err != nil {
		t.Fatal(err)
	}
	state := e.GetSetupState()
	for _, c := range state.Checks {
		if c.Key == "direct" && c.Ready {
			t.Fatal("没配地址与密钥时不该报就绪")
		}
	}
}
