package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 导入图片的登记逻辑：纯逻辑部分可以离线验证，不必等真窗口。
func TestImportImagesRegistersAssets(t *testing.T) {
	s := newTestStore(t)
	app := &App{engine: &Engine{store: s}}
	p, err := s.AddProject("导入测试", "series")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	png := filepath.Join(dir, "雪夜长街.png")
	jpg := filepath.Join(dir, "角色定妆.jpg")
	txt := filepath.Join(dir, "说明.txt")
	for _, f := range []string{png, jpg, txt} {
		if err := os.WriteFile(f, []byte("data-"+filepath.Base(f)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := app.importImages(p.ID, []string{png, jpg, txt})
	if err != nil {
		t.Fatal(err)
	}
	// txt 不是图片，应被跳过
	if len(out) != 2 {
		t.Fatalf("登记了 %d 张，应为 2 张", len(out))
	}
	if out[0].MIME != "image/png" || out[1].MIME != "image/jpeg" {
		t.Fatalf("mime 不对：%s %s", out[0].MIME, out[1].MIME)
	}
	if out[0].Name != "雪夜长街" {
		t.Fatalf("名字应为文件名去掉后缀：%s", out[0].Name)
	}
	// 文件真的落盘了
	for _, a := range out {
		if _, err := os.Stat(s.Path(a)); err != nil {
			t.Fatalf("素材没落盘：%v", err)
		}
	}
}

// 原生对话框在上下文缺失时要给一句能看懂的话，不能 panic。
func TestNativeDialogNotReady(t *testing.T) {
	s := newTestStore(t)
	app := &App{engine: &Engine{store: s}}
	if err := app.nativeDialogReady(); err == nil {
		t.Fatal("没有运行时上下文时应当报错")
	}
	if _, err := app.PickImages("p"); err == nil {
		t.Fatal("PickImages 在没有运行时时应当报错")
	}
}

// 素材名里的扩展名要去掉，否则分类判断会被干扰。
func TestNormalizeAssetNames(t *testing.T) {
	s := newTestStore(t)
	app := &App{engine: &Engine{store: s}}
	p, _ := s.AddProject("改名测试", "series")
	dir := t.TempDir()
	f := filepath.Join(dir, "顾栖月-三视图-顾栖月_v1_玉笛.png")
	os.WriteFile(f, []byte("x"), 0o644)
	if _, err := app.importImages(p.ID, []string{f}); err != nil {
		t.Fatal(err)
	}
	before := s.State().Assets[0].Name
	if before != "顾栖月-三视图-顾栖月_v1_玉笛" {
		t.Fatalf("导入时就不该带后缀：%s", before)
	}
	// 造一个带后缀的存量数据，验证迁移
	s.mu.Lock()
	s.state.Assets[0].Name = "顾栖月-三视图-顾栖月_v1_玉笛.png"
	s.mu.Unlock()
	n, err := s.NormalizeAssetNames()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("应改 1 个：%d", n)
	}
	if got := s.State().Assets[0].Name; got != "顾栖月-三视图-顾栖月_v1_玉笛" {
		t.Fatalf("迁移后：%s", got)
	}
}

// 素材名只取文件名：Windows 的路径用反斜杠，
// 只认 "/" 会把整条路径当素材名（Windows CI 曾经红在这里）。
func TestFileBaseNameWindowsPath(t *testing.T) {
	cases := map[string]string{
		`C:\Users\kubor\Temp\雪夜长街.png`: "雪夜长街",
		`D:\素材\顾栖月-三视图.png`:            "顾栖月-三视图",
		"/Users/kubor/Temp/角色定妆.jpg":   "角色定妆",
	}
	for path, want := range cases {
		if got := fileBaseName(path); got != want {
			t.Fatalf("%s 取名应为 %s，实际 %s", path, want, got)
		}
	}
}
