package main

import (
	"errors"
	"testing"
)

// 没绑定中台项目时，角色/场景一律拒绝读写 —— 空工作区会列出账号下全部主体。
func TestUnboundProjectRefusesSubjects(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("沸腾之雪", "series")
	e := &Engine{store: s}
	if _, err := e.ListProjectSubjects(p.ID); !errors.Is(err, errProjectUnbound) {
		t.Fatalf("未绑定应拒绝列主体：%v", err)
	}
	if _, err := e.CreateProjectScene(p.ID, "雨夜长街", ""); !errors.Is(err, errProjectUnbound) {
		t.Fatalf("未绑定应拒绝建场景：%v", err)
	}
	if _, err := e.ListAllSubjects(""); !errors.Is(err, errProjectUnbound) {
		t.Fatalf("空工作区不能旁路：%v", err)
	}
	if _, err := e.CreateScene("", "x", ""); !errors.Is(err, errProjectUnbound) {
		t.Fatalf("空工作区不能旁路建场景：%v", err)
	}
}

// 绑定落盘，重开工作台仍在；老项目没有这个字段照常打开。
func TestProjectWorkspaceBindingPersists(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("沸腾之雪", "series")
	if _, err := s.SetProjectWorkspace(p.ID, "ws-snow", "沸腾之雪"); err != nil {
		t.Fatal(err)
	}
	reopened := openTestStore(t, s.Root())
	got, _ := reopened.Project(p.ID)
	if got.WorkspaceID != "ws-snow" || got.WorkspaceName != "沸腾之雪" {
		t.Fatalf("绑定没落盘：%+v", got)
	}
	e := &Engine{store: reopened}
	if ws, err := e.projectWorkspace(p.ID); err != nil || ws != "ws-snow" {
		t.Fatalf("应取到绑定的工作区：%q %v", ws, err)
	}
	if _, err := s.SetProjectWorkspace("不存在", "ws", "x"); err == nil {
		t.Fatal("不存在的项目应报错")
	}
}
