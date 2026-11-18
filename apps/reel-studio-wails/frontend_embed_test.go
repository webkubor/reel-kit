package main

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// 前端产物必须和源码同步，否则装出来的 app 会缺界面。
//
// 桌面版把 frontend/dist 用 go:embed 打进二进制；dist 是生成物，
// 改完 src 忘了跑 build.sh 的话，编译能过、包也能打，但用户看到的是旧界面。
// 这个测试把「关键界面标识」钉住，dist 一旦落后就红。
func TestFrontendDistIsInSync(t *testing.T) {
	html, err := os.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Fatalf("读不到前端产物：%v（先跑 frontend/build.sh）", err)
	}
	cast, err := os.ReadFile("frontend/dist/cast.js")
	if err != nil {
		t.Fatalf("读不到角色库产物：%v（先跑 frontend/build.sh）", err)
	}
	htmlText := string(html)
	castText := string(cast)
	for _, k := range []string{
		"setup-kind-series",  // 引导页的项目类型
		"product-asset-grid", // 短视频的素材面板
		"template-dialog",    // 素材模板对话框
		"workspace-tabs",     // 按项目类型生成的标签
		"import-episode-dialog",
	} {
		if !strings.Contains(htmlText, k) {
			t.Errorf("前端产物缺 %q：改了 src 没跑 frontend/build.sh", k)
		}
	}
	for _, k := range []string{"可选装扮", "真人参考", "五视图"} {
		if !strings.Contains(castText, k) {
			t.Errorf("角色库产物缺 %q：改了组件没跑 frontend/build.sh", k)
		}
	}
}

// 嵌入二进制的前端资源要完整：少一个文件，装出来的 app 就少一块界面。
func TestEmbeddedFrontendHasAllFiles(t *testing.T) {
	want := []string{"index.html", "app.js", "app.css", "cast.js", "select.js"}
	for _, name := range want {
		if _, err := fs.Stat(assets, "frontend/dist/"+name); err != nil {
			t.Errorf("嵌入资源缺 %s：%v", name, err)
		}
	}
}
