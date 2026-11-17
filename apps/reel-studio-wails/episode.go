package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 成片：一条 H3 出镜素材 + 旁白 + 环境声/配乐 → 一集。
//
// H3 单条最长 15 秒，正好是沸腾之雪一集的长度，所以一集 = 一条镜头。
// 画面不动（不重渲、不加字），只做 reel-kit 本行：配音与混音。
//
// 字幕：不烧。沸腾之雪的交付红线明确「无字幕；无字卡」，红线高于习惯。
//
// 音轨口径：
//   - 旁白按句生成，句间留停顿，从 LeadIn 秒后开始念，整段必须在镜头内念完
//     —— 超时就报错让人删句，不拉长画面也不加速念白（拉长会冻帧，加速会失真）
//   - 环境声：默认保留 H3 素材自带的音轨当底（风雪、脚步），有旁白时自动压低；
//     选了配乐就用配乐，两种底不叠（叠起来糊）
type EpisodeInput struct {
	ProjectID string
	// ClipAssetID 出镜素材（本地素材库里的 mp4）。
	ClipAssetID string
	Title       string
	// Lines 旁白，0–6 句；0 句 = 只保留环境声/配乐。
	Lines []string
	// KeepAmbient 保留素材自带的环境声当底（无配乐时）。
	KeepAmbient bool
	BGMID       string

	VoiceBackend string
	VoiceModel   string
	VoiceID      string
	Profile      string
	Tone         string
}

const (
	episodeMaxLines    = 6
	episodeLineMax     = 60
	episodeLeadIn      = 0.6  // 开场先让画面立住再开口
	episodePause       = 0.45 // 句间停顿
	episodeTailMargin  = 0.4  // 末句念完离片尾至少留这么多
	episodeAmbientGain = 0.35
)

// MakeEpisode 把一条出镜素材做成一集。
func (e *Engine) MakeEpisode(ctx context.Context, input EpisodeInput, progress ProgressFunc, report func(Step)) (*Asset, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	if report == nil {
		report = func(Step) {}
	}
	if progress == nil {
		progress = func(string) {}
	}
	lines, err := episodeLines(input.Lines)
	if err != nil {
		return nil, err
	}
	clipAsset, err := e.store.Asset(input.ClipAssetID, input.ProjectID)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(clipAsset.MIME, "video/") {
		return nil, errors.New("成片要选一条出镜视频素材")
	}
	clip := e.store.Path(clipAsset)
	clipDur, err := ProbeDuration(ctx, clip)
	if err != nil || clipDur <= 0 {
		return nil, fmt.Errorf("读不出素材时长：%v", err)
	}

	temp, err := os.MkdirTemp("", "reel-episode-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)

	// 旁白：逐句出音，拼成一条带停顿的整轨。
	var voiceClips []VoiceClip
	if len(lines) > 0 {
		backend := firstNonEmpty(input.VoiceBackend, "sherpa")
		origin := VoiceChannelInfo(backend)
		files := []string{}
		durs := []float64{}
		for i, line := range lines {
			progress(fmt.Sprintf("正在为第 %d/%d 句旁白配音…", i+1, len(lines)))
			def := Step{ID: fmt.Sprintf("voice-%d", i), Label: fmt.Sprintf("旁白 %d/%d", i+1, len(lines)), Origin: origin.Origin, Engine: origin.Engine, Detail: truncate(line, 40), Status: "running"}
			report(def)
			asset, err := e.GenerateSpeech(ctx, input.ProjectID, line, input.Tone, backend, input.VoiceModel, input.VoiceID, input.Profile, 1)
			if err == nil && !strings.HasPrefix(asset.MIME, "audio/") {
				err = fmt.Errorf("第 %d 句配音没有生成成功", i+1)
			}
			var d float64
			if err == nil {
				d, err = ProbeDuration(ctx, e.store.Path(asset))
			}
			if err != nil {
				def.Status, def.Error = "failed", Redact(err.Error())
				report(def)
				return nil, err
			}
			def.Status = "done"
			report(def)
			files = append(files, e.store.Path(asset))
			durs = append(durs, d)
		}
		need := narrationLength(durs)
		if need > clipDur-episodeTailMargin {
			return nil, fmt.Errorf("旁白念完要 %.1f 秒，镜头只有 %.1f 秒；请删减旁白（不拉长画面、不加速念白）", need, clipDur)
		}
		track := filepath.Join(temp, "narration.wav")
		if err := buildNarration(ctx, files, durs, track); err != nil {
			return nil, err
		}
		voiceClips = []VoiceClip{{File: track}}
	}

	// 声音的底：配乐优先；否则按需保留素材自带的环境声。
	bed, bedGain, bedLabel := "", 0.0, "无"
	if input.BGMID != "" {
		a, err := e.store.Asset(input.BGMID, input.ProjectID)
		if err != nil {
			return nil, err
		}
		bed, bedLabel = e.store.Path(a), "配乐"
	} else if input.KeepAmbient && hasAudioStream(ctx, clip) {
		bed, bedLabel = clip, "素材环境声"
		if len(voiceClips) > 0 {
			bedGain = episodeAmbientGain
		} else {
			bedGain = 1
		}
	}

	out := filepath.Join(temp, "episode.mp4")
	def := Step{ID: "compose", Label: "合成成片", Origin: "code", Engine: "ffmpeg 合成", Status: "running",
		Detail: fmt.Sprintf("%.1f 秒 · 旁白 %d 句 · 底：%s · 无字幕（交付红线）", clipDur, len(lines), bedLabel)}
	report(def)
	progress("正在合成成片…")
	_, total, err := Compose(ctx, []string{clip}, []float64{clipDur}, bed, voiceClips, out, ComposeOpts{
		FPS: 30, BGMGain: bedGain, VoiceGain: DefaultVoiceGain, Duck: true, Normalize: true, Transition: "none",
	})
	if err != nil {
		def.Status, def.Error = "failed", Redact(err.Error())
		report(def)
		return nil, err
	}
	def.Status = "done"
	report(def)

	data, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	hasAudio := len(voiceClips) > 0 || bed != ""
	return e.store.AddAsset(input.ProjectID, firstNonEmpty(input.Title, strings.TrimSuffix(clipAsset.Name, ".mp4")+"-成片")+".mp4", "video/mp4", data, map[string]any{
		"source":   "reel-kit",
		"origin":   "code",
		"engine":   "本地合成（出镜素材 + 旁白 + ffmpeg）",
		"duration": total,
		"hasAudio": hasAudio,
		"script":   strings.Join(lines, "\n"),
		"clipId":   clipAsset.ID,
		"bed":      bedLabel,
	})
}

func episodeLines(in []string) ([]string, error) {
	out := []string{}
	for _, l := range in {
		if t := strings.TrimSpace(l); t != "" {
			if len([]rune(t)) > episodeLineMax {
				return nil, fmt.Errorf("每句旁白不超过 %d 字", episodeLineMax)
			}
			out = append(out, t)
		}
	}
	if len(out) > episodeMaxLines {
		return nil, fmt.Errorf("旁白最多 %d 句；15 秒念不完更多", episodeMaxLines)
	}
	return out, nil
}

// narrationLength 整段旁白所需时长：开场留白 + 每句 + 句间停顿。
func narrationLength(durs []float64) float64 {
	total := episodeLeadIn
	for i, d := range durs {
		total += d
		if i < len(durs)-1 {
			total += episodePause
		}
	}
	return total
}

// buildNarration 拼旁白整轨：开场静音 + 每句 apad 到「句长+停顿」后首尾相接。
// 与 compose 同一口径（apad 补齐再 concat），不用 adelay 算偏移，不会累积误差。
func buildNarration(ctx context.Context, files []string, durs []float64, out string) error {
	args := []string{"-y", "-v", "error", "-f", "lavfi", "-t", fmt.Sprintf("%.3f", episodeLeadIn), "-i", "anullsrc=r=48000:cl=mono"}
	filters := []string{"[0:a]aformat=sample_rates=48000:channel_layouts=mono[s0]"}
	parts := []string{"[s0]"}
	for i, f := range files {
		args = append(args, "-i", f)
		whole := durs[i]
		if i < len(files)-1 {
			whole += episodePause
		}
		filters = append(filters, fmt.Sprintf("[%d:a]aresample=48000,aformat=sample_rates=48000:channel_layouts=mono,apad=whole_dur=%.3f[s%d]", i+1, whole, i+1))
		parts = append(parts, fmt.Sprintf("[s%d]", i+1))
	}
	filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[out]", strings.Join(parts, ""), len(parts)))
	args = append(args, "-filter_complex", strings.Join(filters, ";"), "-map", "[out]", out)
	return runFfmpeg(ctx, args)
}

// hasAudioStream 素材里有没有音轨（H3 一般带，其他上游不一定）。
func hasAudioStream(ctx context.Context, file string) bool {
	out, err := exec.CommandContext(ctx, mediaCommand("ffprobe"), "-v", "quiet", "-select_streams", "a",
		"-show_entries", "stream=index", "-of", "csv=p=0", file).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}
