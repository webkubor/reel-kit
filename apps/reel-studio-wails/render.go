package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// 把「模板 + 每句文案 + 每张素材」渲染成逐帧 PNG。
//
// 为什么用浏览器渲染而不是 ffmpeg 的 drawtext：
//
//	drawtext 要手写中文字体路径、描边、换行、居中，圆弧装饰更是画不出来；
//	改一次版式要重调一串滤镜参数。HTML 里这些都是几行 CSS，而且改完能直接看。
//	代价是每帧要跑一次截图 —— 但一支片子只有 8~20 帧，几秒的事。
//
// 用本机已有的 Chrome，不给这个工具再下一份 chromium —— 桌面版不塞第二份浏览器。
var chromeCandidates = []string{
	os.Getenv("CHROME_PATH"),
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/Applications/ego lite.app/Contents/MacOS/ego lite",
	"/usr/bin/google-chrome",
	"/usr/bin/chromium",
}

// FindChrome 找本机 Chrome 的可执行文件。
func FindChrome() string {
	if path := bundledResource("chrome", "chrome-headless-shell"); path != "" {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	for _, p := range chromeCandidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Shot 一镜的内容。
type Shot struct {
	Image   string         // 本地图片路径
	Caption string         // 字幕
	Vars    map[string]any // 单镜覆盖的模板变量
}

// Size 画布尺寸。
type Size struct {
	Width  int
	Height int
}

// RenderOpts 渲染参数。
type RenderOpts struct {
	Template   string
	Shots      []Shot
	Vars       map[string]any
	OutDir     string
	Width      int
	Height     int
	Motion     string
	Durations  []float64
	FPS        int
	OnProgress func(text string, ratio float64)
}

var (
	placeholderRe = regexp.MustCompile(`\{\{(\w+)\}\}`)
	bodyRuleRe    = regexp.MustCompile(`body\s*\{([^}]*)\}`)
	widthRe       = regexp.MustCompile(`width:\s*(\d+)px`)
	heightRe      = regexp.MustCompile(`height:\s*(\d+)px`)
	// Go 的 RE2 不支持反向引用 \1（Node 版用的是 src=(["'])\{\{image\}\}\1），
	// 所以这里分开抓两个引号再自己比对 —— 引号不成对的写法不配对。
	shotMarkRe = regexp.MustCompile(`src=(["'])\{\{image\}\}(["'])`)
)

// Fill 极简占位符替换。刻意不引模板引擎 —— 只有 {{key}} 一种语法，够用。
func Fill(tpl string, vars map[string]any) string {
	return placeholderRe.ReplaceAllStringFunc(tpl, func(match string) string {
		key := match[2 : len(match)-2]
		v, ok := vars[key]
		if !ok || v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	})
}

// FrameVars 组装一帧的模板变量。
//
// 画布尺寸必须在这里补进去：模板写 `width:{{width}}px` 而调用方没传 width 时，
// 占位符会被替换成空串 → `width:px` 是非法声明 → body 高度塌成 0 → 整帧黑屏。
// 调用方显式给了 width/height 就以调用方为准（模板尺寸与请求尺寸冲突时不该被静默改写）。
func FrameVars(vars map[string]any, size Size, shot Shot) map[string]any {
	out := map[string]any{}
	for k, v := range vars {
		out[k] = v
	}
	// 单镜可以覆盖模板级变量：比如每一镜的字幕字号按长度单独定，
	// 长的那句小一号，短的那句大一号，不必整条片子一个字号。
	for k, v := range shot.Vars {
		out[k] = v
	}
	if v, ok := vars["width"]; ok && v != nil {
		out["width"] = v
	} else {
		out["width"] = size.Width
	}
	if v, ok := vars["height"]; ok && v != nil {
		out["height"] = v
	} else {
		out["height"] = size.Height
	}
	out["caption"] = shot.Caption
	out["image"] = shot.Image
	return out
}

// markShotImage 给「这一镜的画面」那个 img 加上标记。
//
// 以前是对页面上所有 <img> 做运镜。用户自带模板里放的 logo、角标、装饰图
// 也会跟着缩放漂移 —— 那些是模板的固定装饰，不是这一镜的内容，
// 动它们等于把版式弄坏，还会让用户以为是自己图片有问题。
func markShotImage(tpl string) string {
	return shotMarkRe.ReplaceAllStringFunc(tpl, func(match string) string {
		groups := shotMarkRe.FindStringSubmatch(match)
		if len(groups) != 3 || groups[1] != groups[2] {
			return match
		}
		return `src=` + groups[1] + `{{image}}` + groups[2] + ` data-reel-shot="1"`
	})
}

// ReadTemplateSize 从模板里读出 body 的 width/height，作为 viewport 尺寸。
//
// 模板作者把画布尺寸写在 body 的 CSS 里，这里就跟着走，无需每个模板单独配尺寸。
// 读不到时退回参数默认值。
//
// 只在 body 自己的规则里找尺寸：早先的整段匹配会跨到后面的规则，
// 模板里任意一个 `width:8px` 之类的小尺寸就把画布算成 8×8。
//
// body 里写 `width:{{width}}px` 这种占位符时读不到数字 —— 这不是错误，
// 它表示「画布尺寸随调用参数走」，读不到就退回参数默认值，
// 再由渲染流程把最终尺寸回填进占位符。
func ReadTemplateSize(tpl string) *Size {
	m := bodyRuleRe.FindStringSubmatch(tpl)
	if m == nil {
		return nil
	}
	w := widthRe.FindStringSubmatch(m[1])
	h := heightRe.FindStringSubmatch(m[1])
	if w == nil || h == nil {
		return nil
	}
	wi, err1 := strconv.Atoi(w[1])
	hi, err2 := strconv.Atoi(h[1])
	if err1 != nil || err2 != nil {
		return nil
	}
	return &Size{Width: wi, Height: hi}
}

// ToDataURI 图片转 data URI —— 让 HTML 完全自包含，避免 file:// 的相对路径与权限问题。
func ToDataURI(file string) (string, error) {
	ext := strings.ToLower(filepath.Ext(file))
	mime := map[string]string{
		".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
		".webp": "image/webp", ".gif": "image/gif",
	}[ext]
	if mime == "" {
		return "", fmt.Errorf("不支持的图片格式: %s", file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// RenderFrames 渲染逐镜画面。
//
// motion=none 时每镜出一张 PNG；motion=gentle 时每镜出一段 mp4（运镜+字幕入场）。
func RenderFrames(ctx context.Context, opts RenderOpts) ([]string, error) {
	if opts.Motion != "none" && opts.Motion != "gentle" {
		return nil, fmt.Errorf("不支持的画面动效: %s", opts.Motion)
	}
	// 帧率默认 30（与 Node 版一致），再校验范围。
	if opts.FPS == 0 {
		opts.FPS = 30
	}
	if opts.FPS < 1 || opts.FPS > 60 {
		return nil, fmt.Errorf("帧率必须是 1–60 的整数")
	}
	if opts.Motion != "none" {
		if len(opts.Durations) != len(opts.Shots) {
			return nil, fmt.Errorf("连续动效需要每镜的有效时长")
		}
		for _, d := range opts.Durations {
			if d <= 0 {
				return nil, fmt.Errorf("连续动效需要每镜的有效时长")
			}
		}
	}
	chrome := FindChrome()
	if chrome == "" {
		return nil, fmt.Errorf("找不到本机 Chrome，设 CHROME_PATH 指向可执行文件")
	}

	rawTpl, err := os.ReadFile(opts.Template)
	if err != nil {
		return nil, err
	}
	// 标记「这一镜的画面」：动效只作用在它身上。
	//
	// 以前是对页面上所有 <img> 做运镜。用户自带模板里放的 logo、角标、装饰图
	// 也会跟着缩放漂移 —— 那些是模板的固定装饰，不是这一镜的内容，
	// 动它们等于把版式弄坏，还会让用户以为是自己图片有问题。
	tpl := markShotImage(string(rawTpl))

	// 画布尺寸优先从模板 body CSS 读，读不到再用参数默认值
	size := Size{Width: opts.Width, Height: opts.Height}
	if opts.Width <= 0 {
		size.Width = 1080
	}
	if opts.Height <= 0 {
		size.Height = 1920
	}
	if ts := ReadTemplateSize(tpl); ts != nil {
		size = *ts
	}
	if err := os.MkdirAll(opts.OutDir, 0o700); err != nil {
		return nil, err
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(chrome),
			// 画布本身就是 1080×1920 目标分辨率，放大截图再让 ffmpeg 缩回去
			// 只会多一次重采样，白白糊掉文字边缘。
			//
			// 注意 Flag 的值只能是 string 或 bool，传 int 会报
			// 「invalid exec pool flag」。
			chromedp.Flag("force-device-scale-factor", "1"),
		)...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	tabCtx, cancelTab := chromedp.NewContext(browserCtx)
	defer cancelTab()

	// 先建页面并记住 frame id：后面每镜用 Page.setDocumentContent 换内容，
	// 比 data: URL 导航稳 —— 画面里有大尺寸图片时 data URL 会超长而失败。
	//
	// 这一步同时把浏览器与标签页拉起来（chromedp.Call 内部会分配）。
	tree, err := chromedp.Call(tabCtx, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		return nil, fmt.Errorf("取页面 frame 失败: %w", err)
	}
	if tree.FrameTree == nil || tree.FrameTree.Frame == nil {
		return nil, fmt.Errorf("取页面 frame 失败：没有拿到主 frame")
	}
	frameID := tree.FrameTree.Frame.ID

	if _, err := chromedp.Run(tabCtx, chromedp.EmulateViewport(int64(size.Width), int64(size.Height))); err != nil {
		return nil, fmt.Errorf("设置画布尺寸失败: %w", err)
	}

	frames := make([]string, 0, len(opts.Shots))
	for i, shot := range opts.Shots {
		// 取消要在这里就停：否则一镜渲染完才停，用户按取消后还得等一整镜。
		if err := ctx.Err(); err != nil {
			return nil, ErrCanceled
		}
		image := ""
		if shot.Image != "" {
			uri, err := ToDataURI(shot.Image)
			if err != nil {
				return nil, err
			}
			image = uri
		}
		vars := FrameVars(opts.Vars, size, Shot{
			Caption: shot.Caption,
			Image:   image,
			Vars:    shot.Vars,
		})
		// 画布尺寸回填进模板占位符：模板写 `width:{{width}}px` 时，
		// 不回填会渲染成 `width:px`（非法声明）→ body 高度塌成 0 → 全屏黑。
		html := Fill(tpl, vars)

		if err := setDocumentContent(tabCtx, frameID, html); err != nil {
			return nil, fmt.Errorf("载入第 %d 镜画面失败: %w", i+1, err)
		}
		if err := waitForAssets(tabCtx); err != nil {
			return nil, fmt.Errorf("等待第 %d 镜素材就绪失败: %w", i+1, err)
		}

		if opts.Motion == "none" {
			out := filepath.Join(opts.OutDir, fmt.Sprintf("frame_%04d.png", i))
			png, err := chromedp.Run(tabCtx, chromedp.CaptureScreenshot())
			if err != nil {
				return nil, fmt.Errorf("截取第 %d 镜失败: %w", i+1, err)
			}
			if err := os.WriteFile(out, png, 0o600); err != nil {
				return nil, err
			}
			frames = append(frames, out)
			continue
		}
		out := filepath.Join(opts.OutDir, fmt.Sprintf("frame_%04d.mp4", i))
		if err := encodeMotion(tabCtx, ctx, out, opts.Durations[i], opts.FPS, opts.OnProgress, i, len(opts.Shots)); err != nil {
			return nil, err
		}
		frames = append(frames, out)
	}
	return frames, nil
}

// setDocumentContent 用 CDP 换掉当前页面内容。
func setDocumentContent(ctx context.Context, frameID cdp.FrameID, html string) error {
	_, err := chromedp.Call(ctx, page.SetDocumentContent, page.SetDocumentContentParams{
		FrameID: frameID,
		HTML:    html,
	})
	return err
}

// waitForAssets 等字体与图片解码落定，否则偶发截到文字未上屏的空帧。
func waitForAssets(ctx context.Context) error {
	const js = `(async () => {
		await document.fonts.ready
		await Promise.all([...document.images]
			.filter(img => img.getAttribute('src'))
			.map(img => img.decode().catch(() => {})))
		return true
	})()`
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](js, chromedp.EvalAwaitPromise)); err != nil {
		return err
	}
	// 留一点余量：解码完成不等于已经上屏。
	_, err := chromedp.Run(ctx, chromedp.Sleep(120*time.Millisecond))
	return err
}

// prepareMotion 装上暂停的动画轨道。
//
// 编码器按帧号 seek 它们，从不按墙上时间 —— 所以时间轴是确定的，重跑同一支片子结果一致。
// 标题与背景保持不动；只有这一镜的画面和字幕在动。
// 自定义模板可以自己实现 window.reelSeek({time, duration}) 来接管场景。
func prepareMotion(ctx context.Context, duration float64) error {
	// duration 要插进 JS 里 —— 页面作用域里没有这个变量。
	const js = `(async () => {
		const duration = %f
		const style = document.createElement('style')
		style.textContent = '*,*::before,*::after{animation:none!important;transition:none!important}'
		document.head.appendChild(style)
		const tracks = []
		// 只动这一镜的画面（渲染流程打的 data-reel-shot 标记）。
		// 不是页面上所有图片 —— logo、装饰图不该跟着镜头动。
		for (const image of document.querySelectorAll('img[data-reel-shot]')) {
			if (!image.getAttribute('src')) continue
			image.parentElement.style.overflow = 'hidden'
			const track = image.animate(
				[{ scale: '1' }, { scale: '1.04' }],
				{ duration: duration * 1000, fill: 'both', easing: 'linear' })
			track.pause()
			tracks.push(track)
		}
		for (const caption of document.querySelectorAll('.caption')) {
			const track = caption.animate(
				[{ translate: '0 12px', opacity: 0 }, { translate: '0 0', opacity: 1 }],
				{ duration: Math.min(350, duration * 1000), fill: 'both', easing: 'cubic-bezier(0.16,1,0.3,1)' })
			track.pause()
			tracks.push(track)
		}
		window.__reelTracks = tracks
		return true
	})()`
	_, err := chromedp.Run(ctx, chromedp.Evaluate[bool](fmt.Sprintf(js, duration), chromedp.EvalAwaitPromise))
	return err
}

// seekMotion 把动画轨道拨到指定时刻。
func seekMotion(ctx context.Context, time float64, duration float64) error {
	const js = `(async (t, d) => {
		for (const track of window.__reelTracks || []) track.currentTime = t * 1000
		if (typeof window.reelSeek === 'function') await window.reelSeek({ time: t, duration: d })
		return true
	})(%f, %f)`
	_, err := chromedp.Run(ctx, chromedp.Evaluate[bool](fmt.Sprintf(js, time, duration), chromedp.EvalAwaitPromise))
	return err
}

// encodeMotion 逐帧截图并流式喂给 ffmpeg。
//
// 取消必须真的掐断编码器：被取消的渲染继续跑，上游云服务照样计费。
func encodeMotion(ctx context.Context, parent context.Context, out string, duration float64, fps int, onProgress func(string, float64), index, total int) error {
	if err := prepareMotion(ctx, duration); err != nil {
		return fmt.Errorf("准备第 %d 镜动效失败: %w", index+1, err)
	}

	args := []string{
		"-v", "error", "-y", "-f", "image2pipe", "-vcodec", "png",
		"-framerate", strconv.Itoa(fps), "-i", "pipe:0", "-an",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "18",
		"-pix_fmt", "yuv420p", "-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
		"-movflags", "+faststart", out,
	}
	cmd := exec.CommandContext(parent, mediaCommand("ffmpeg"), args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 ffmpeg 失败: %w", err)
	}

	// 编码器卡死要有兜底，不能无限期占着任务。
	timeout := time.AfterFunc(10*time.Minute, func() {
		_ = cmd.Process.Kill()
	})
	defer timeout.Stop()

	count := int(math.Ceil(duration * float64(fps)))
	for frame := 0; frame < count; frame++ {
		if parent.Err() != nil {
			_ = cmd.Process.Kill()
			return ErrCanceled
		}
		t := float64(frame) / float64(fps)
		if err := seekMotion(ctx, t, duration); err != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("第 %d 镜动效定位失败: %w", index+1, err)
		}
		png, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
		if err != nil {
			_ = cmd.Process.Kill()
			return fmt.Errorf("截取第 %d 镜动效帧失败: %w", index+1, err)
		}
		if _, err := stdin.Write(png); err != nil {
			_ = cmd.Process.Kill()
			// 取消会让 ffmpeg 先死，管道随之断 —— 报「写入失败」是把
			// 用户自己的取消说成了出错。该说的是「已取消」。
			if parent.Err() != nil {
				return ErrCanceled
			}
			return fmt.Errorf("写入动画编码失败: %w", err)
		}
		if onProgress != nil && frame%fps == 0 {
			onProgress(fmt.Sprintf("正在渲染第 %d/%d 镜动效", index+1, total), float64(frame)/float64(count))
		}
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Process.Kill()
		if parent.Err() != nil {
			return ErrCanceled
		}
		return err
	}
	if err := cmd.Wait(); err != nil {
		if parent.Err() != nil {
			return ErrCanceled
		}
		return fmt.Errorf("动画编码失败: %s", tailLines(stderr.String(), 14))
	}
	if onProgress != nil {
		onProgress(fmt.Sprintf("正在渲染第 %d/%d 镜动效", index+1, total), 1)
	}
	return nil
}

// tailLines 取输出尾部若干行。
func tailLines(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "（无输出）"
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
