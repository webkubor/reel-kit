package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// 一条视频由七块组成，界面按这七块分组，生成配置面板也按这七块展示。
//
// 为什么要定死这七块：用户问「这条视频是怎么来的」时，答案永远是同样的七件事。
// 表单和配置面板用同一套分组，用户看表单的顺序和回看配置时的顺序一致，
// 调整时不用在两套顺序之间来回换脑。
//
//	脚本 —— 说什么
//	声音 —— 谁在念、念多快、什么情绪、从哪来
//	画面 —— 看到什么
//	版式 —— 长什么样
//	画幅 —— 发到哪
//	配乐 —— 声音层次
//	输出 —— 产物
const (
	partScript = "脚本"
	partVoice  = "声音"
	partVisual = "画面"
	partLayout = "版式"
	partFormat = "画幅"
	partMusic  = "配乐"
	partOutput = "输出"
)

// partOrder 是七块的固定顺序。
var partOrder = []string{partScript, partVoice, partVisual, partLayout, partFormat, partMusic, partOutput}

// configField 是生成配置里的一个字段。
type configField struct {
	Group string
	Key   string
	Label string
}

// configFields 返回某个任务类型的全部配置字段，按七块分组。
func configFields(kind string) []configField {
	switch kind {
	case "talk":
		return []configField{
			{partScript, "title", "标题"},
			{partScript, "captions", "口播脚本"},
			{partScript, "subtitle", "副标题"},
			{partVoice, "persona", "音色"},
			{partVoice, "voiceBackend", "配音引擎"},
			{partVoice, "tone", "语气"},
			{partVoice, "pause", "镜间停顿"},
			{partVisual, "assetIds", "画面素材"},
			{partLayout, "style", "视觉风格"},
			{partLayout, "template", "版式模板"},
			{partFormat, "ratio", "画幅"},
			{partMusic, "bgmId", "背景音乐"},
			{partMusic, "bgmGain", "BGM 音量"},
		}
	case "recommendation":
		return []configField{
			{partScript, "repoUrl", "内容源"},
			{partScript, "title", "标题"},
			{partScript, "captions", "分镜口播"},
			{partVoice, "backend", "配音通道"},
			{partVoice, "model", "模型"},
			{partVoice, "voiceId", "音色"},
			{partVoice, "profile", "计费账户"},
			{partVoice, "voiceSource", "旁白来源"},
			{partVoice, "voiceIds", "逐镜配音"},
			{partVoice, "narrationId", "旁白音频"},
			{partVoice, "tone", "语气"},
			{partVoice, "speed", "语速"},
			{partVoice, "pause", "镜间停顿"},
			{partVisual, "assetIds", "分镜画面"},
			{partVisual, "imageFit", "画面填充"},
			{partLayout, "template", "版式"},
			{partFormat, "platform", "发布平台"},
			{partFormat, "ratio", "画幅"},
			{partMusic, "bgmId", "背景音乐"},
			{partMusic, "bgmGain", "BGM 音量"},
		}
	case "export":
		return []configField{
			{partScript, "title", "标题"},
			{partScript, "captions", "字幕"},
			{partVisual, "assetIds", "素材"},
			{partVoice, "audioId", "配音"},
			{partLayout, "template", "版式"},
			{partFormat, "ratio", "画幅"},
			{partFormat, "duration", "每镜时长"},
		}
	case "product":
		return []configField{
			{partScript, "title", "产品或人物名称"},
			{partScript, "captions", "介绍词"},
			{partScript, "subtitle", "一句话卖点"},
			{partScript, "footer", "片尾一句"},
			{partVoice, "persona", "音色"},
			{partVoice, "voiceBackend", "配音引擎"},
			{partVoice, "tone", "语气"},
			{partVoice, "pause", "镜间停顿"},
			{partVisual, "assetIds", "产品画面"},
			{partLayout, "template", "版式"},
			{partFormat, "ratio", "画幅"},
			{partMusic, "bgmId", "背景音乐"},
			{partMusic, "bgmGain", "BGM 音量"},
		}
	}
	return nil
}

// configGroups 返回该任务类型用到的分组，保持七块的固定顺序。
func configGroups(kind string) []string {
	seen := map[string]bool{}
	for _, f := range configFields(kind) {
		seen[f.Group] = true
	}
	var out []string
	for _, g := range partOrder {
		if seen[g] {
			out = append(out, g)
		}
	}
	return out
}

// assetName 按 ID 找素材名。
func (a *App) assetName(id string) string {
	for _, asset := range a.ProjectAssets() {
		if asset.ID == id {
			return asset.Name
		}
	}
	return ""
}

// configValue 把一条生成配置里的某个字段写成给人看的值。
//
// 空的写「—」而不是空串：空串在界面上和「这一行没画出来」分不清。
func (a *App) configValue(kind, key string, input map[string]any) string {
	raw, ok := input[key]
	if !ok || raw == nil {
		return "—"
	}
	switch key {
	case "captions":
		lines := splitLines(fmt.Sprintf("%v", raw))
		if len(lines) == 0 {
			return "—"
		}
		return fmt.Sprintf("%d 镜", len(lines))
	case "assetIds":
		list, _ := raw.([]any)
		if len(list) == 0 {
			return "未使用（用占位画面）"
		}
		images, videos := 0, 0
		for _, item := range list {
			id, _ := item.(string)
			asset := a.assetByID(id)
			switch {
			case asset == nil:
			case strings.HasPrefix(asset.Mime, "image/"):
				images++
			case strings.HasPrefix(asset.Mime, "video/"):
				videos++
			}
		}
		parts := []string{}
		if images > 0 {
			parts = append(parts, fmt.Sprintf("%d 张图片", images))
		}
		if videos > 0 {
			parts = append(parts, fmt.Sprintf("%d 段视频", videos))
		}
		if len(parts) == 0 {
			return fmt.Sprintf("%d 项", len(list))
		}
		return strings.Join(parts, " + ")
	case "voiceIds":
		list, _ := raw.([]any)
		picked := 0
		for _, item := range list {
			if id, _ := item.(string); id != "" {
				picked++
			}
		}
		if picked == 0 {
			return "全部用 AI 配音"
		}
		return fmt.Sprintf("%d/%d 句指定配音", picked, len(list))
	case "bgmId", "audioId", "narrationId":
		id, _ := raw.(string)
		if id == "" {
			return "—"
		}
		if name := a.assetName(id); name != "" {
			return name
		}
		return id
	case "persona":
		id, _ := raw.(string)
		if a.providers != nil {
			for _, p := range a.providers.Voice.Personas {
				if p.Key == id {
					return p.Name
				}
			}
		}
		return orDash(id)
	case "style":
		key, _ := raw.(string)
		for _, s := range a.talkStyles {
			if s.Key == key {
				if s.Name != "" {
					return s.Name
				}
				return s.Key
			}
		}
		return orDash(key)
	case "platform":
		key, _ := raw.(string)
		switch key {
		case "channels":
			return "视频号（竖屏）"
		case "xiaohongshu":
			return "小红书（竖屏）"
		case "landscape":
			return "横屏视频"
		case "bilibili":
			return "横屏（16:9）"
		}
		return orDash(key)
	case "voiceSource":
		key, _ := raw.(string)
		switch key {
		case "tts":
			return "AI 配音"
		case "scenes":
			return "逐镜指定"
		case "audio":
			return "整段音频切分"
		}
		return orDash(key)
	case "imageFit":
		key, _ := raw.(string)
		switch key {
		case "cover":
			return "铺满画布"
		case "contain":
			return "完整画面"
		}
		return orDash(key)
	case "voiceBackend", "backend":
		key, _ := raw.(string)
		switch key {
		case "voxflow":
			return "本机 VoxFlow"
		case "minimax":
			return "MiniMax TTS"
		}
		return orDash(key)
	case "pause":
		return fmt.Sprintf("%s 秒", fmt.Sprintf("%v", raw))
	case "speed":
		return fmt.Sprintf("%v 倍", raw)
	case "bgmGain":
		return fmt.Sprintf("%d%%", int(toFloat(raw)*100+0.5))
	case "duration":
		return fmt.Sprintf("%v 秒", raw)
	case "ratio":
		return fmt.Sprintf("%v", raw)
	case "template":
		// 版式名写成人话：配置面板是给用户看的，不是给开发者看的。
		return templateLabel(fmt.Sprintf("%v", raw))
	default:
		return fmt.Sprintf("%v", raw)
	}
}

// templateLabel 把版式 ID 翻成人话。
//
// 这一句在配置面板和对比面板里都会出现：写「出镜竖版」用户一眼知道是什么，
// 写「product-presenter」用户得猜。
func templateLabel(id string) string {
	switch id {
	case "product-presenter":
		return "出镜竖版"
	case "landscape-product":
		return "左图右文横版"
	case "knowledge-paper":
		return "纸本文献"
	case "knowledge-dark":
		return "暗场概念"
	case "knowledge-gufeng":
		return "古风长卷"
	case "project-recommendation":
		return "项目推荐"
	case "sticker-promo":
		return "贴纸促销"
	case "sticker-square":
		return "方形贴纸"
	case "music-card":
		return "音乐卡片"
	case "quote":
		return "金句卡片"
	case "":
		return "—"
	}
	return id
}

// configText 返回字段的完整文本（用于对比，脚本这类长文本不能只写「3 镜」）。
func (a *App) configText(kind, key string, input map[string]any) string {
	raw, ok := input[key]
	if !ok || raw == nil {
		return ""
	}
	switch key {
	case "captions":
		return strings.Join(splitLines(fmt.Sprintf("%v", raw)), " / ")
	case "assetIds":
		list, _ := raw.([]any)
		var names []string
		for _, item := range list {
			id, _ := item.(string)
			if name := a.assetName(id); name != "" {
				names = append(names, name)
			}
		}
		return strings.Join(names, "、")
	}
	return a.configValue(kind, key, input)
}

// viewConfigPanel 画一条视频的生成配置。
//
// 这块是「这条视频是怎么来的」的唯一答案：按七块分组，逐项写清值。
func (a *App) viewConfigPanel(c *ui.Context, kind string, input map[string]any) {
	fields := configFields(kind)
	if len(fields) == 0 {
		return
	}
	a.kit.Panel(c, func() {
		for _, group := range configGroups(kind) {
			a.kit.PartTitle(c, group)
			ui.Column(c).FillWidth().Gap(design.SpaceSM).Children(func() {
				for _, f := range fields {
					if f.Group != group {
						continue
					}
					a.kit.KeyValue(c, f.Label, a.configValue(kind, f.Key, input))
				}
			})
		}
	})
}

// viewConfigDiff 画两条视频的配置对比。
//
// 只列不一样的项：一样的项在对比视图里没有信息量，占满屏幕反而看不出差别。
func (a *App) viewConfigDiff(c *ui.Context, kind string, left, right studioapi.Job, leftName, rightName string) {
	fields := configFields(kind)
	var diffs []configField
	for _, f := range fields {
		l := a.configText(kind, f.Key, left.Input)
		r := a.configText(kind, f.Key, right.Input)
		if l != r {
			diffs = append(diffs, f)
		}
	}
	if len(diffs) == 0 {
		a.kit.Hint(c, "两条视频的生成配置完全一致。")
		return
	}
	a.kit.Panel(c, func() {
		ui.Row(c).FillWidth().Gap(design.SpaceMD).Children(func() {
			ui.Text(c, leftName).Width(110).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
			ui.Text(c, rightName).Grow(1).FontSize(design.FontTiny).TextColor(a.kit.T.Accent)
		})
		for _, f := range diffs {
			ui.Column(c).FillWidth().Gap(design.SpaceXS).
				Background(a.kit.T.SurfaceAlt).Radius(design.RadiusSM).Padding(design.SpaceMD).Children(func() {
				ui.Text(c, f.Group+" · "+f.Label).FontSize(design.FontTiny).TextColor(a.kit.T.Muted)
				ui.Row(c).FillWidth().Gap(design.SpaceMD).Children(func() {
					ui.Text(c, a.configText(kind, f.Key, left.Input)).Width(110).
						FontSize(design.FontSmall).TextColor(a.kit.T.Ink)
					ui.Text(c, a.configText(kind, f.Key, right.Input)).Grow(1).
						FontSize(design.FontSmall).TextColor(a.kit.T.Accent)
				})
			})
		}
	})
}

// applyConfig 把一条视频的生成配置载入表单，供「调整后再生成一条」使用。
//
// 载入的是**当时那次生成的输入**，不是界面上的当前值 ——
// 用户要的是「照着这条改」，所以以那条为准。
func (a *App) applyConfig(kind string, input map[string]any, jobID string) {
	str := func(key string) string {
		if v, ok := input[key].(string); ok {
			return v
		}
		return ""
	}
	num := func(key, fallback string) string {
		if v, ok := input[key]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return fallback
	}
	ids := func(key string) []string {
		list, _ := input[key].([]any)
		out := make([]string, 0, len(list))
		for _, item := range list {
			if id, ok := item.(string); ok {
				out = append(out, id)
			}
		}
		return out
	}
	switch kind {
	case "talk":
		a.talkTitle = str("title")
		a.talkCaptions = str("captions")
		a.talkSubtitle = str("subtitle")
		a.talkPersona = str("persona")
		a.talkStyle = str("style")
		a.talkTone = str("tone")
		a.talkPause = num("pause", a.talkPause)
		if v, ok := input["bgmGain"]; ok && v != nil {
			a.talkBGMGain = gainPercent(v)
		}
		a.talkImages = ids("assetIds")
		a.talkSelected = make(map[string]bool)
		for _, id := range a.talkImages {
			a.talkSelected[id] = true
		}
		a.talkBGM = a.bgmChoiceFor(str("bgmId"))
	case "recommendation":
		a.repoURL = str("repoUrl")
		a.recTitle = str("title")
		a.recCaptions = str("captions")
		a.recPlatform = str("platform")
		a.recRatio = str("ratio")
		a.backend = str("backend")
		a.model = str("model")
		a.voiceID = str("voiceId")
		if a.voiceID == "" {
			a.voiceID = str("persona")
		}
		a.profile = str("profile")
		a.recVoiceSource = str("voiceSource")
		a.recTone = str("tone")
		a.recSpeed = num("speed", a.recSpeed)
		a.recPause = num("pause", a.recPause)
		if v, ok := input["bgmGain"]; ok && v != nil {
			a.recBGMGain = gainPercent(v)
		}
		a.recImageFit = str("imageFit")
		a.recImages = ids("assetIds")
		a.recSelected = make(map[string]bool)
		for _, id := range a.recImages {
			a.recSelected[id] = true
		}
		a.recVoiceIDs = ids("voiceIds")
		a.recNarrationID = str("narrationId")
		a.recBGM = a.bgmChoiceFor(str("bgmId"))
	case "export":
		a.assetIDs = ids("assetIds")
		a.ratio = str("ratio")
		a.duration = num("duration", a.duration)
		a.expTitle = str("title")
		a.expCaptions = str("captions")
		a.expAudioID = str("audioId")
		a.expTemplate = str("template")
		if a.expTemplate == "" {
			a.expTemplate = "sticker-promo"
		}
	case "product":
		a.prodTitle = str("title")
		a.prodSubtitle = str("subtitle")
		a.prodCaptions = str("captions")
		a.prodFooter = str("footer")
		a.prodPersona = str("persona")
		a.prodTone = str("tone")
		a.prodPause = num("pause", a.prodPause)
		a.prodRatio = str("ratio")
		if a.prodRatio == "" {
			a.prodRatio = "9:16"
		}
		if v, ok := input["bgmGain"]; ok && v != nil {
			a.prodBGMGain = gainPercent(v)
		}
		a.prodImages = ids("assetIds")
		a.prodSelected = make(map[string]bool)
		for _, id := range a.prodImages {
			a.prodSelected[id] = true
		}
		a.prodBGM = a.bgmChoiceFor(str("bgmId"))
	}
	a.adjustingJobID = jobID
}

// bgmChoiceFor 把素材 ID 还原成表单里用的选择值。
func (a *App) bgmChoiceFor(assetID string) string {
	if assetID == "" {
		return ""
	}
	for _, asset := range a.ProjectAssets() {
		if asset.ID == assetID && strings.HasPrefix(asset.Mime, "audio/") {
			return "asset:" + asset.ID
		}
	}
	return "bgm:" + assetID
}

// splitLines 拆多行文本，去掉空行。
func splitLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// toFloat 把任意数值转成 float64。
func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case string:
		var f float64
		fmt.Sscanf(v, "%g", &f)
		return f
	}
	return 0
}

// parseNumberIn 解析一个数字并检查范围。
//
// 表单里的数字都要过这一道：以前直接 toFloat，「abc」会变成 0，
// 用户填错东西却看见「音量 0」，问题被静默吞掉。
func parseNumberIn(label, text string, lo, hi float64) (float64, error) {
	trimmed := strings.TrimSpace(text)
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("%s要填数字，现在填的是「%s」", label, trimmed)
	}
	if value < lo || value > hi {
		return 0, fmt.Errorf("%s要在 %g–%g 之间，现在是 %g", label, lo, hi, value)
	}
	return value, nil
}

// gainPercent 把后端存的音量比例（0–1）变成界面上的百分比（12）。
//
// 界面用百分比：让人对着「12%」填数字，比对着「0.12」自然得多。
func gainPercent(value any) string {
	return strconv.FormatFloat(math.Round(toFloat(value)*100), 'f', -1, 64)
}
