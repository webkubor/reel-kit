package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeTestSnapshot(t *testing.T, root, name, ws, at string) string {
	t.Helper()
	dir := filepath.Join(root, "museav-snapshots", name)
	scene := filepath.Join(dir, "subjects", "scene-1")
	person := filepath.Join(dir, "subjects", "person-1")
	for _, d := range []string{scene, person} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	must := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(filepath.Join(dir, "manifest.json"), `{"workspaceId":"`+ws+`","capturedAt":"`+at+`"}`)
	must(filepath.Join(scene, "subject.json"), `{"subject":{"id":"scene-1","kind":"scene","name":"雨夜长街","created_at":"2"},
"assets":[{"id":"a-raw","role":"raw","media_type":"image"},{"id":"a-std","role":"standard","media_type":"image"}]}`)
	must(filepath.Join(scene, "a-std.png"), "png-"+name)
	must(filepath.Join(scene, "a-raw.png"), "raw")
	must(filepath.Join(person, "subject.json"), `{"subject":{"id":"person-1","kind":"person","name":"夜惊鸿","created_at":"1"},"assets":[]}`)
	return dir
}

// 断网时主体列表从快照出，角色与场景都在。
func TestSnapshotSubjectsOffline(t *testing.T) {
	root := t.TempDir()
	writeTestSnapshot(t, root, "old", "ws-1", "2026-11-01")
	list, err := snapshotSubjects(root, "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "夜惊鸿" || list[1].Kind != "scene" {
		t.Fatalf("快照主体不对：%+v %+v", list[0], list[1])
	}
	if _, err := snapshotSubjects(root, "ws-other"); err == nil {
		t.Fatal("别的工作区不该读到这份快照")
	}
}

// 多份快照取最新；场景图优先 standard。
func TestSnapshotSceneImagePicksLatestStandard(t *testing.T) {
	root := t.TempDir()
	writeTestSnapshot(t, root, "old", "ws-1", "2026-11-01")
	writeTestSnapshot(t, root, "new", "ws-1", "2026-11-18")
	path, err := snapshotSceneImage(root, "ws-1", "scene-1")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "png-new" {
		t.Fatalf("没取最新快照的 standard 图：%s", data)
	}
	if _, err := snapshotSceneImage(root, "ws-1", "person-1"); err == nil {
		t.Fatal("角色不该当场景出图")
	}
	if _, err := snapshotSceneImage(root, "ws-1", "../../etc"); err == nil {
		t.Fatal("路径穿越应被拒绝")
	}
}

// 缓存优先于快照；缓存缺失回退快照。
func TestOfflineSubjectsPrefersCache(t *testing.T) {
	s := newTestStore(t)
	e := &Engine{store: s}
	writeTestSnapshot(t, s.Root(), "snap", "ws-1", "2026-11-18")
	list, err := e.offlineSubjects("ws-1")
	if err != nil || len(list) != 2 {
		t.Fatalf("快照回退失败：%v %d", err, len(list))
	}
	writeSubjectsCache(s.Root(), "ws-1", []*Subject{{ID: "c", Name: "缓存角色", Kind: "person"}})
	list, err = e.offlineSubjects("ws-1")
	if err != nil || len(list) != 1 || list[0].Name != "缓存角色" {
		t.Fatalf("应优先读缓存：%v %+v", err, list)
	}
}

// /scene/<ws>/<id> 从本地快照出图。
func TestServeSnapshotScene(t *testing.T) {
	s := newTestStore(t)
	app := &App{engine: &Engine{store: s}}
	writeTestSnapshot(t, s.Root(), "snap", "ws-1", "2026-11-18")
	rec := httptest.NewRecorder()
	app.mediaHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/scene/ws-1/scene-1", nil))
	if rec.Code != 200 || rec.Body.String() != "png-snap" {
		t.Fatalf("出图失败：%d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	app.mediaHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/scene/ws-1/nope", nil))
	if rec.Code != 404 {
		t.Fatalf("不存在的场景应 404：%d", rec.Code)
	}
}
