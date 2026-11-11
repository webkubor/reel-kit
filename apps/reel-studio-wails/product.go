package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 人物 / 商品介绍流水线。
//
// 为什么单独一条流水线，而不是复用「知识口播」：
//
//	口播是「文字为主、图片垫底」—— 版式承担叙事，图片只是氛围。
//	商品介绍正好相反：观众先看东西，再听一句说明。
//	版式因此反过来：大图打底，文字压在图上，只留一句标题、一句说明、一句收尾。
//
// 底座与口播共用同一套流程（逐句配音 → 按实际语速定每镜时长 → 渲染画面帧 → 合成），
// 差别只在「版式选哪个」和「尺寸随画幅走」。
//
// 竖版用 product-presenter（出镜画面满屏 + 底部字幕条），
// 横版用 landscape-product（左图右文，1920×1080）。

// ProductLayout 一种画幅对应的版式与尺寸。
type ProductLayout struct {
	Template string
	Width    int
	Height   int
}

// ProductLayouts 每种画幅对应的版式与尺寸。
var ProductLayouts = map[string]ProductLayout{
	"9:16": {Template: "product-presenter", Width: 1080, Height: 1920},
	"16:9": {Template: "landscape-product", Width: 1920, Height: 1080},
}

// escHTML 转义 HTML 特殊字符。
//
// 文案是用户自己填的，但会直接进模板渲染成 HTML —— 不转义的话，
// 一句带尖括号的文案就能把版式撑坏（渲染是本地浏览器，注入不了什么，
// 但版式会错，用户看到的是自己没写过的排版）。
func escHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
	)
	return r.Replace(s)
}

// captionSize 字幕字号按这一句的长度定。
//
// 一句话一镜，长短差别很大：统一一个字号，短句显得空，长句直接溢出画面。
// 这里按字数往下收一档，保证最长的那句也放得下。
func captionSize(text string, layout ProductLayout) int {
	n := len([]rune(text))
	if layout.Template == "landscape-product" {
		switch {
		case n <= 24:
			return 44
		case n <= 40:
			return 38
		case n <= 60:
			return 32
		default:
			return 27
		}
	}
	switch {
	case n <= 18:
		return 60
	case n <= 30:
		return 52
	case n <= 45:
		return 44
	case n <= 60:
		return 38
	default:
		return 32
	}
}

// placeholderImage 没有素材时的占位图。
//
// 与口播的浅色渐变不同，商品介绍默认是深色版式 ——
// 占位图跟着版式走，缺素材时看着仍像一条正经片子，而不是半成品。
func placeholderImage(ctx context.Context, file string, layout ProductLayout) error {
	c0 := "0x0d0f11"
	c1 := "0x2b2f36"
	if layout.Template == "landscape-product" {
		c0 = "0x14161a"
	}
	return runFfmpeg(ctx, []string{
		"-v", "error", "-y", "-f", "lavfi",
		"-i", fmt.Sprintf("gradients=size=%dx%d:c0=%s:c1=%s:nb_colors=2", layout.Width, layout.Height, c0, c1),
		"-frames:v", "1", file,
	})
}

// ProductInput 产品流水线的入参。
type ProductInput struct {
	ProjectID string
	Title     string
	Subtitle  string
	Footer    string
	Ratio     string
	Lines     []string
	AssetIDs  []string
	BGMID     string

	VoiceBackend string
	VoiceModel   string
	VoiceID      string
	Profile      string
	Tone         string
}

// MakeProductVideo 走完一条产品介绍流水线。
func (e *Engine) MakeProductVideo(ctx context.Context, input ProductInput, progress ProgressFunc, report func(Step)) (*Asset, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	if report == nil {
		report = func(Step) {}
	}
	if progress == nil {
		progress = func(string) {}
	}

	lines := []string{}
	for _, l := range input.Lines {
		if t := trimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) == 0 || len(lines) > 8 {
		return nil, fmt.Errorf("请填写 1–8 行介绍词，每行不超过 80 字；过长的句子请拆成多镜")
	}
	for _, l := range lines {
		if len([]rune(l)) > 80 {
			return nil, fmt.Errorf("请填写 1–8 行介绍词，每行不超过 80 字；过长的句子请拆成多镜")
		}
	}
	ratio := input.Ratio
	if _, ok := ProductLayouts[ratio]; !ok {
		ratio = "9:16"
	}
	layout := ProductLayouts[ratio]

	temp, err := os.MkdirTemp("", "reel-product-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)

	// 每个环节单独上报：失败时能看出是「配音没出来」还是「画面没渲染出来」
	step := func(def Step, run func() (map[string]any, error)) (map[string]any, error) {
		def.Status = "running"
		report(def)
		payload, err := run()
		if err != nil {
			// 被取消不是失败：界面上的工序面板要能区分「用户主动停了」和「做坏了」。
			if err == ErrCanceled {
				def.Status = "canceled"
			} else {
				def.Status = "failed"
				def.Error = Redact(err.Error())
			}
			report(def)
			return nil, err
		}
		for k, v := range payload {
			def.setPayload(k, v)
		}
		def.Status = "done"
		report(def)
		return payload, nil
	}

	if _, err := step(Step{
		ID:     "config",
		Label:  "确认配置",
		Origin: "code", Engine: "参数校验",
		Detail: fmt.Sprintf("%s · %d 镜 · %d×%d", layout.Template, len(lines), layout.Width, layout.Height),
	}, func() (map[string]any, error) { return nil, nil }); err != nil {
		return nil, err
	}

	// 素材：只接受图片
	images := []string{}
	for _, id := range input.AssetIDs {
		asset, err := e.store.Asset(id, input.ProjectID)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(asset.MIME, "image/") {
			return nil, fmt.Errorf("产品或人物画面必须是图片")
		}
		images = append(images, e.store.Path(asset))
	}

	if len(images) == 0 {
		progress("没有产品画面，先用深色占位图顶上…")
		for i := range lines {
			p := filepath.Join(temp, fmt.Sprintf("placeholder-%d.png", i))
			if err := placeholderImage(ctx, p, layout); err != nil {
				return nil, err
			}
			images = append(images, p)
		}
		if _, err := step(Step{
			ID:     "placeholder",
			Label:  "生成占位画面",
			Origin: "code", Engine: "ffmpeg 渐变",
			Detail: fmt.Sprintf("%d 张占位图，之后可换真图重做", len(images)),
		}, func() (map[string]any, error) { return nil, nil }); err != nil {
			return nil, err
		}
	}

	// 旋钮走统一收敛逻辑：预设 → 默认值 → 用户填的值。
	settings := ResolveSettings("product", map[string]any{}, "")

	// 逐句配音：音色相同、语气相同，只是念的内容不同
	voiceClips := []VoiceClip{}
	voiceDurations := []float64{}
	backend := input.VoiceBackend
	if backend == "" {
		backend = "sherpa"
	}
	origin := VoiceChannelInfo(backend)
	speed := floatOr(settings["speed"], 1.0)
	for i, line := range lines {
		progress(fmt.Sprintf("正在为第 %d/%d 句生成配音…", i+1, len(lines)))
		stepDef := Step{
			ID:     fmt.Sprintf("voice-%d", i),
			Label:  fmt.Sprintf("配音 %d/%d", i+1, len(lines)),
			Origin: origin.Origin, Engine: origin.Engine,
			Detail: truncate(line, 40),
		}
		var audioFile string
		var audioDuration float64
		_, err := step(stepDef, func() (map[string]any, error) {
			asset, err := e.GenerateSpeech(ctx, input.ProjectID, line, input.Tone, backend,
				input.VoiceModel, input.VoiceID, input.Profile, speed)
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(asset.MIME, "audio/") {
				return nil, fmt.Errorf("第 %d 句配音没有生成成功", i+1)
			}
			audioFile = e.store.Path(asset)
			audioDuration, err = ProbeDuration(ctx, audioFile)
			if err != nil {
				return nil, err
			}
			return map[string]any{"assetId": asset.ID}, nil
		})
		if err != nil {
			return nil, err
		}
		// 本机离线引擎不支持语速参数，交给后面的 atempo 处理。
		if speed != 1 && backend != "minimax" && backend != "edge" {
			adjusted := filepath.Join(temp, fmt.Sprintf("voice-speed-%d.wav", i))
			if err := runFfmpeg(ctx, []string{"-v", "error", "-i", audioFile, "-af", fmt.Sprintf("atempo=%g", speed), adjusted}); err != nil {
				return nil, err
			}
			audioFile = adjusted
		}
		voiceClips = append(voiceClips, VoiceClip{File: audioFile})
		voiceDurations = append(voiceDurations, audioDuration)
	}

	pause := floatOr(settings["pause"], 0.35)
	durations := make([]float64, len(voiceDurations))
	for i, d := range voiceDurations {
		durations[i] = d + pause
	}

	motion := "none"
	if v, ok := settings["motion"].(string); ok && v != "" {
		motion = v
	}

	progress("正在渲染画面帧…")
	templateFile, err := WriteTemplateTo(layout.Template, temp)
	if err != nil {
		return nil, err
	}
	shots := make([]Shot, len(lines))
	for i, line := range lines {
		shots[i] = Shot{
			Image:   images[i%len(images)],
			Caption: escHTML(line),
			Vars:    map[string]any{"captionSize": captionSize(line, layout)},
		}
	}
	vars := map[string]any{
		"title": escHTML(input.Title), "subtitle": escHTML(input.Subtitle),
		"footer":      escHTML(input.Footer),
		"bg":          "#0d0f11",
		"accent1":     "#e8542f",
		"accent2":     "#2f6f6a",
		"captionSize": 60,
		"width":       layout.Width,
		"height":      layout.Height,
	}
	if ratio == "16:9" {
		vars["bg"] = "#f4f2ee"
		vars["accent1"] = "#c8502f"
	}
	renderMotion := motion
	if renderMotion == "" {
		renderMotion = "none"
	}
	var frames []string
	if _, err := step(Step{
		ID:     "frames",
		Label:  "渲染画面帧",
		Origin: "code", Engine: "本机 Chrome 渲染",
		Detail: fmt.Sprintf("%d 镜 · %d×%d · %s", len(lines), layout.Width, layout.Height, layout.Template),
	}, func() (map[string]any, error) {
		var err error
		frames, err = RenderFrames(ctx, RenderOpts{
			Template:  templateFile,
			Shots:     shots,
			Vars:      vars,
			OutDir:    filepath.Join(temp, "frames"),
			Width:     layout.Width,
			Height:    layout.Height,
			Motion:    renderMotion,
			Durations: durations,
			FPS:       30,
			OnProgress: func(text string, ratio float64) {
				progress(text)
			},
		})
		if err != nil {
			return nil, err
		}
		return nil, nil
	}); err != nil {
		return nil, err
	}

	progress("正在合成成片…")
	out := filepath.Join(temp, "product.mp4")
	bgm := ""
	bgmGain := floatOr(settings["bgmGain"], 1.0)
	if input.BGMID != "" {
		asset, err := e.store.Asset(input.BGMID, input.ProjectID)
		if err != nil {
			return nil, err
		}
		bgm = e.store.Path(asset)
	}
	transition := "none"
	if v, ok := settings["transition"].(string); ok && v != "" {
		transition = v
	}
	transitionDur := floatOr(settings["transitionDuration"], 0.4)
	voiceGain := floatOr(settings["voiceGain"], DefaultVoiceGain)
	duck := true
	if v, ok := settings["duck"].(bool); ok {
		duck = v
	}
	normalize := true
	if v, ok := settings["normalize"].(bool); ok {
		normalize = v
	}
	var outputDuration float64
	if _, err := step(Step{
		ID:     "compose",
		Label:  "合成成片",
		Origin: "code", Engine: "ffmpeg 合成",
		Detail: fmt.Sprintf("%d×%d", layout.Width, layout.Height),
	}, func() (map[string]any, error) {
		var err error
		_, outputDuration, err = Compose(ctx, frames, durations, bgm, voiceClips, out, ComposeOpts{
			FPS:           30,
			BGMGain:       bgmGain,
			VoiceGain:     voiceGain,
			Duck:          duck,
			Normalize:     normalize,
			Transition:    transition,
			TransitionDur: transitionDur,
		})
		if err != nil {
			return nil, err
		}
		return nil, nil
	}); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	hasAudio := true
	return e.store.AddAsset(input.ProjectID, fmt.Sprintf("%s.mp4", firstNonEmpty(input.Title, "产品介绍")), "video/mp4", data, map[string]any{
		"source":   "reel-kit",
		"origin":   "code",
		"engine":   "本地合成（渲染 + ffmpeg）",
		"width":    layout.Width,
		"height":   layout.Height,
		"duration": outputDuration,
		"hasAudio": hasAudio,
		"template": layout.Template,
		"script":   strings.Join(lines, "\n"),
		"ratio":    ratio,
	})
}

// firstNonEmpty 取第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := trimSpace(v); t != "" {
			return t
		}
	}
	return ""
}
