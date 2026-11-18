package main

import (
	"os"
	"testing"
)

// 换绑到另一个中台项目后，旧工作区留下的素材要清掉。
//
// 不清的话：角色库按新工作区列主体，镜像里却是旧角色的图，
// 于是界面上显示的是上一个项目的角色。
func TestRemoveAssetsByMuseavWorkspace(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("换绑", "series")
	oldAsset, err := s.AddAsset(p.ID, "旧角色-头像", "image/png", []byte("old"), map[string]any{
		"source": "museav", "museavWorkspaceId": "ws-old", "museavSubjectName": "旧角色",
	})
	if err != nil {
		t.Fatal(err)
	}
	keepAsset, err := s.AddAsset(p.ID, "新角色-头像", "image/png", []byte("new"), map[string]any{
		"source": "museav", "museavWorkspaceId": "ws-new", "museavSubjectName": "新角色",
	})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := s.AddAsset(p.ID, "手动导入的图", "image/png", []byte("manual"), map[string]any{
		"source": "upload",
	})
	n, err := s.RemoveAssetsByMuseavWorkspace(p.ID, "ws-old")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应删 1 个：%d", n)
	}
	names := map[string]bool{}
	for _, a := range s.State().Assets {
		names[a.Name] = true
	}
	if names["旧角色-头像"] {
		t.Fatal("旧工作区的素材还在")
	}
	if !names["新角色-头像"] || !names["手动导入的图"] {
		t.Fatalf("不该被删的没了：%v", names)
	}
	// 磁盘文件也清掉
	if _, err := os.Stat(s.Path(oldAsset)); err == nil {
		t.Fatal("旧素材文件还在磁盘上")
	}
	if _, err := os.Stat(s.Path(keepAsset)); err != nil {
		t.Fatalf("新素材文件被误删：%v", err)
	}
	if _, err := os.Stat(s.Path(other)); err != nil {
		t.Fatalf("手动导入的文件被误删：%v", err)
	}
}
