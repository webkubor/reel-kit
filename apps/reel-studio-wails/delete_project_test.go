package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 删项目要把它的素材、剧集、镜头和磁盘文件一起清掉，
// 而且不能碰到别的项目的数据。
func TestDeleteProjectRemovesOnlyItsOwn(t *testing.T) {
	s := newTestStore(t)
	keep, _ := s.AddProject("留下的", "series")
	del, _ := s.AddProject("要删的", "series")
	data := []byte("some-image-bytes")
	aKeep, err := s.AddAsset(keep.ID, "保留图", "image/png", data, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	aDel, err := s.AddAsset(del.ID, "删除图", "image/png", data, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	epKeep, _ := s.AddEpisode(EpisodeDraft{ProjectID: keep.ID, Title: "留着的集"})
	epDel, _ := s.AddEpisode(EpisodeDraft{ProjectID: del.ID, Title: "要删的集"})
	if _, err := s.AddShot(ShotInput{ProjectID: del.ID, EpisodeID: epDel.ID, Title: "镜头"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddShot(ShotInput{ProjectID: keep.ID, EpisodeID: epKeep.ID, Title: "镜头"}); err != nil {
		t.Fatal(err)
	}
	delFile := s.Path(aDel)
	keepFile := s.Path(aKeep)
	if err := s.DeleteProject(del.ID); err != nil {
		t.Fatal(err)
	}
	// 项目没了
	for _, p := range s.State().Projects {
		if p.ID == del.ID {
			t.Fatal("项目还在")
		}
	}
	// 它的素材、剧集、镜头都没了
	for _, a := range s.State().Assets {
		if a.ID == aDel.ID {
			t.Fatal("素材还在")
		}
	}
	for _, e := range s.State().Episodes {
		if e.ID == epDel.ID {
			t.Fatal("剧集还在")
		}
	}
	for _, sh := range s.State().Shots {
		if sh.ProjectID == del.ID {
			t.Fatal("镜头还在")
		}
	}
	// 磁盘文件删了
	if _, err := os.Stat(delFile); err == nil {
		t.Fatal("素材文件还在磁盘上")
	}
	// 别的项目的不受影响
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("别的项目的素材被误删：%v", err)
	}
	if len(s.State().Episodes) != 1 || len(s.State().Shots) != 1 {
		t.Fatal("别的项目的剧集或镜头被误删")
	}
	// 删不存在的项目要报错
	if err := s.DeleteProject("不存在"); err == nil {
		t.Fatal("删不存在的项目应报错")
	}
}

// 删项目不能把文件删到工作目录外面去。
func TestDeleteProjectStaysInsideRoot(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("边界", "series")
	if _, err := s.AddAsset(p.ID, "图", "image/png", []byte("x"), map[string]any{}); err != nil {
		t.Fatal(err)
	}
	// 在工作目录外面放一个同名文件，删项目不该碰它
	outside := filepath.Join(filepath.Dir(s.Root()), "keep-me.png")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)
	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("工作目录外的文件被删了：%v", err)
	}
}
