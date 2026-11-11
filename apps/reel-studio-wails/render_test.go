package main

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 端到端渲染：真本机 Chrome + 真模板 + 真截图。
//
// 这组用例依赖本机装了 Chrome，没装就跳过（不是失败 —— 渲染是本机能力）。
// 判据不是「文件生成了」，而是「画面里真有字、真的有像素」：
// 一张纯白或纯黑的 PNG 也叫生成成功，但它对用户毫无意义。
const testTemplate = `<!doctype html>
<html><head><meta charset="utf-8"><style>
body { margin:0; width:360px; height:640px; background:#123456; font-family: sans-serif; }
.caption { color:#ffffff; font-size:32px; padding:40px; }
img[data-reel-shot] { width:200px; height:200px; object-fit:cover; background:#ff8800; }
</style></head>
<body>
<img src="{{image}}" data-reel-shot="1">
<div class="caption">{{caption}}</div>
</body></html>`

// writeShotPNG 造一张纯色 PNG 当素材。
func writeShotPNG(t *testing.T, path string, r, g, b uint8) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("建素材文件失败: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("编码素材失败: %v", err)
	}
}

// distinctColors 数一张 PNG 里有几种颜色。
//
// 这是「画面真的画出来了」的判据：模板有底色、有素材、有白字，
// 至少三种颜色。如果只有一种，那这张 PNG 是空白帧 —— 文件生成了也没用。
func distinctColors(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("解码 PNG 失败: %v", err)
	}
	seen := map[color.RGBA]bool{}
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			seen[color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}] = true
		}
	}
	return len(seen)
}

// TestRenderFramesStatic 渲染静帧。
func TestRenderFramesStatic(t *testing.T) {
	if FindChrome() == "" {
		t.Skip("本机没有 Chrome，跳过渲染用例")
	}
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.html")
	if err := os.WriteFile(tpl, []byte(testTemplate), 0o600); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "shot.png")
	writeShotPNG(t, img, 255, 136, 0)

	out := filepath.Join(dir, "out")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	frames, err := RenderFrames(ctx, RenderOpts{
		Template: tpl,
		Shots: []Shot{
			{Image: img, Caption: "第一镜的中文标题"},
			{Image: img, Caption: "第二镜"},
		},
		Vars:   map[string]any{},
		OutDir: out,
		Motion: "none",
	})
	if err != nil {
		t.Fatalf("RenderFrames: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("应有 2 帧，实际 %d", len(frames))
	}
	for _, f := range frames {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatalf("帧文件缺失: %v", err)
		}
		if info.Size() < 1000 {
			t.Fatalf("帧文件太小（可能是空画面）: %s = %d 字节", filepath.Base(f), info.Size())
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data[:8]), "\x89PNG") {
			t.Fatalf("不是 PNG: %s", filepath.Base(f))
		}
		// 画面里要有底色、素材、白字 —— 至少三种颜色。
		// 只有一种颜色说明截到的是空白帧：文件生成了，但用户看到的是一张废片。
		if n := distinctColors(t, f); n < 3 {
			t.Fatalf("第 %s 镜画面是空白的（只有 %d 种颜色）", filepath.Base(f), n)
		}
	}
}

// TestRenderFramesMotion 渲染带运镜的动效片段。
func TestRenderFramesMotion(t *testing.T) {
	if FindChrome() == "" {
		t.Skip("本机没有 Chrome，跳过渲染用例")
	}
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.html")
	if err := os.WriteFile(tpl, []byte(testTemplate), 0o600); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "shot.png")
	writeShotPNG(t, img, 255, 136, 0)

	out := filepath.Join(dir, "out")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var progress []string
	frames, err := RenderFrames(ctx, RenderOpts{
		Template:  tpl,
		Shots:     []Shot{{Image: img, Caption: "动效镜"}},
		Vars:      map[string]any{},
		OutDir:    out,
		Motion:    "gentle",
		Durations: []float64{1.0},
		FPS:       12,
		OnProgress: func(text string, ratio float64) {
			progress = append(progress, text)
		},
	})
	if err != nil {
		t.Fatalf("RenderFrames(motion): %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("应有 1 段，实际 %d", len(frames))
	}
	info, err := os.Stat(frames[0])
	if err != nil {
		t.Fatalf("动效文件缺失: %v", err)
	}
	if info.Size() < 1000 {
		t.Fatalf("动效文件太小: %d 字节", info.Size())
	}
	if len(progress) == 0 {
		t.Error("应当有进度回调")
	}
}

// 取消要真的掐断渲染。
func TestRenderFramesCancel(t *testing.T) {
	if FindChrome() == "" {
		t.Skip("本机没有 Chrome，跳过渲染用例")
	}
	dir := t.TempDir()
	tpl := filepath.Join(dir, "tpl.html")
	if err := os.WriteFile(tpl, []byte(testTemplate), 0o600); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "shot.png")
	writeShotPNG(t, img, 255, 136, 0)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(2 * time.Second)
		cancel()
	}()
	_, err := RenderFrames(ctx, RenderOpts{
		Template:  tpl,
		Shots:     []Shot{{Image: img, Caption: "x"}},
		Vars:      map[string]any{},
		OutDir:    filepath.Join(dir, "out"),
		Motion:    "gentle",
		Durations: []float64{8.0},
		FPS:       30,
	})
	if err == nil {
		t.Fatal("被取消的渲染应当报错")
	}
	// 取消必须报成「已取消」，不能报成「写入失败」——
	// 用户自己按的取消，界面却显示出错，是他操作有问题。
	if err != ErrCanceled {
		t.Fatalf("取消应报 ErrCanceled，实际 %v", err)
	}
}
