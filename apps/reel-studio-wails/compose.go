package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ComposeOpts 合成选项，对应 Node 版 compose 的参数。
type ComposeOpts struct {
	FPS           int     // 输出帧率，默认 30
	FadeOut       float64 // 片尾音频淡出秒数，默认 1.5
	BGMGain       float64 // BGM 音量，默认 DEFAULT_BGM_GAIN（有配音）或 1（无配音）
	BGMCrossfade  float64 // BGM 头尾交叉淡化秒数，0 = 旧行为
	Transition    string  // 镜间转场名，默认 "none"
	TransitionDur float64 // 转场时长秒，默认 0.4
	VoiceGain     float64 // 人声增益，默认 1.35
	Duck          bool    // 人声是否侧链压低配乐，默认 true
	Normalize     bool    // 是否做响度归一化到 -14 LUFS，默认 true
}

// VoiceClip 一句配音及其文件路径。
type VoiceClip struct {
	File string
}

// Transitions ffmpeg xfade transition 名字 → 内部滤镜名。
// 用白名单而不是透传用户输入 —— xfade 不接受任意 transition,拼错会报奇怪的错。
var Transitions = map[string]string{
	"fade":        "fade",
	"slide-left":  "slideleft",
	"slide-right": "slideright",
	"slide-up":    "slideup",
	"slide-down":  "slidedown",
	"wipe-left":   "wipeleft",
	"wipe-right":  "wiperight",
	"dissolve":    "dissolve",
	"zoom-in":     "zoomin",
}

// Compose 逐帧 PNG + BGM → mp4。
//
// 用 concat demuxer：每帧各带各的 duration，支持「片尾多停一会」等需求。
// 有转场或 mp4 输入时，走 xfade 路径。
func Compose(ctx context.Context, frames []string, durations []float64, bgm string, voiceClips []VoiceClip, out string, opts ComposeOpts) (string, float64, error) {
	if opts.FPS == 0 {
		opts.FPS = 30
	}
	if opts.FadeOut == 0 {
		opts.FadeOut = 1.5
	}
	if len(frames) == 0 {
		return "", 0, fmt.Errorf("没有帧可合成")
	}
	if len(frames) != len(durations) {
		return "", 0, fmt.Errorf("frames 与 durations 长度不一致")
	}
	if len(voiceClips) != 0 && len(voiceClips) != len(frames) {
		return "", 0, fmt.Errorf("voiceClips 与 frames 长度不一致")
	}
	if opts.FPS < 1 || opts.FPS > 60 {
		return "", 0, fmt.Errorf("帧率必须是 1–60 的整数")
	}
	for _, d := range durations {
		if d <= 0 {
			return "", 0, fmt.Errorf("每镜时长必须是正数")
		}
	}
	if opts.Transition != "none" {
		if _, ok := Transitions[opts.Transition]; !ok {
			return "", 0, fmt.Errorf("不支持的转场 %q", opts.Transition)
		}
	}

	total := 0.0
	for _, d := range durations {
		total += d
	}

	// 连续动画片段和多镜转场共用同一条时间轴。
	if (opts.Transition != "none" && len(frames) > 1) || hasMP4Input(frames) {
		return composeWithXfade(ctx, frames, durations, bgm, voiceClips, out, opts, total)
	}

	workDir := filepath.Dir(frames[0])
	listFile := filepath.Join(workDir, "concat.txt")
	if err := writeConcatList(listFile, frames, durations); err != nil {
		return "", 0, err
	}
	if err := mkdirAll(filepath.Dir(out)); err != nil {
		return "", 0, err
	}

	args := []string{"-y", "-f", "concat", "-safe", "0", "-i", listFile}

	// 音轨有三种情形：无音轨 / 只有 BGM / 配音（可叠 BGM）。
	// 配音的关键是对齐：镜头时长 = 念白 + 余量，每句念白后面补静音到镜头长度，
	// 再首尾相接 concat，音画天然同步，不需要算 offset，也就不会有累积误差。
	var filters []string
	var parts []string

	if len(voiceClips) > 0 {
		for i, v := range voiceClips {
			args = append(args, "-i", v.File)
			// 输入序号从 1 开始（0 是画面 concat）
			filters = append(filters, fmt.Sprintf("[%d:a]apad=whole_dur=%.3f[v%d]", i+1, durations[i], i))
			parts = append(parts, fmt.Sprintf("[v%d]", i))
		}
		filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[voice]", stringsJoin(parts, ""), len(voiceClips)))
	}

	if bgm != "" {
		bgmIdx := 1 + len(voiceClips)
		args = append(args, "-i", bgm)
		fadeStart := maxFloat(0, total-opts.FadeOut)
		// 有念白时 BGM 必须压下去，否则念白被盖住。
		// 默认值来自 settings（DefaultBGMGain = 0.12）——全工程只有一个默认。
		gain := opts.BGMGain
		if gain == 0 {
			if len(voiceClips) > 0 {
				gain = DefaultBGMGain
			} else {
				gain = 1
			}
		}
		var bgmFilter string
		if opts.BGMCrossfade > 0 {
			xfade := opts.BGMCrossfade
			bgmFilter = fmt.Sprintf(
				"[%d:a]asplit=2[b1][b2];"+
					"[b1]atrim=0:%.3f,asetpts=PTS-STARTPTS[h];"+
					"[b2]atrim=start=%.3f,asetpts=PTS-STARTPTS[t];"+
					"[h][t]acrossfade=d=%.3f:c1=tri:c2=tri[s];"+
					"[s]aloop=loop=-1:size=2e9,atrim=0:%.3f,volume=%g,afade=t=out:st=%.3f:d=%.3f[bgm]",
				bgmIdx, xfade, xfade, xfade, total, gain, fadeStart, opts.FadeOut,
			)
		} else {
			bgmFilter = fmt.Sprintf(
				"[%d:a]aloop=loop=-1:size=2e9,atrim=0:%.3f,volume=%g,afade=t=out:st=%.3f:d=%.3f[bgm]",
				bgmIdx, total, gain, fadeStart, opts.FadeOut,
			)
		}
		filters = append(filters, bgmFilter)
	}

	hasAudio := len(voiceClips) > 0 || bgm != ""

	if hasAudio {
		mixFilters := BuildMixChain(
			optLabel(voiceLabels(voiceClips)), optLabel(bgmLabel(bgm)),
			opts.VoiceGain, opts.Duck, opts.Normalize, "[a]",
		)
		filters = append(filters, mixFilters...)
		args = append(args, "-filter_complex", stringsJoin(filters, ";"), "-map", "0:v", "-map", "[a]",
			"-c:a", "aac", "-b:a", "192k", "-shortest")
	} else {
		// 无音轨：不挂音频滤镜，输出里没有音频流
		if len(filters) > 0 {
			args = append(args, "-filter_complex", stringsJoin(filters, ";"))
		}
		args = append(args, "-map", "0:v")
	}

	args = append(args,
		"-c:v", "libx264", "-preset", "medium", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-vf", fmt.Sprintf("fps=%d,scale=trunc(iw/2)*2:trunc(ih/2)*2", opts.FPS),
		"-movflags", "+faststart", "-t", strconv.FormatFloat(total+1.0/float64(opts.FPS), 'f', -1, 64),
		out,
	)

	if err := runFfmpeg(ctx, args); err != nil {
		return "", 0, err
	}
	return out, total, nil
}

// composeWithXfade 转场合成：不用 concat demuxer，改用 filter_complex + xfade 滤镜链。
func composeWithXfade(ctx context.Context, frames []string, durations []float64, bgm string, voiceClips []VoiceClip, out string, opts ComposeOpts, total float64) (string, float64, error) {
	xfadeType, ok := Transitions[opts.Transition]
	if ok && (opts.TransitionDur <= 0) {
		return "", 0, fmt.Errorf("转场时长必须 > 0")
	}

	// 短镜头也要能出片；转场最多占最短一镜的一半，且按帧对齐。
	x := 0.0
	if ok && len(frames) > 1 {
		minDur := durations[0]
		for _, d := range durations[1:] {
			if d < minDur {
				minDur = d
			}
		}
		x = maxFloat(0, minFloat(opts.TransitionDur, minDur/2))
		x = float64(int(x*float64(opts.FPS))) / float64(opts.FPS)
	}

	args := []string{"-y", "-filter_complex_threads", "1"}
	for i, f := range frames {
		if !strings.HasSuffix(f, ".mp4") {
			args = append(args, "-loop", "1", "-framerate", strconv.Itoa(opts.FPS))
		}
		args = append(args, "-t", strconv.FormatFloat(durations[i]+x+1.0/float64(opts.FPS), 'f', -1, 64), "-i", f)
	}
	voiceStart := len(frames)
	for _, v := range voiceClips {
		args = append(args, "-i", v.File)
	}
	bgmIdx := voiceStart + len(voiceClips)
	if bgm != "" {
		args = append(args, "-i", bgm)
	}

	var filters []string
	for i := 0; i < len(frames); i++ {
		tail := 0.0
		if i < len(frames)-1 {
			tail = x
		}
		filters = append(filters, fmt.Sprintf(
			"[%d:v]scale=trunc(iw/2)*2:trunc(ih/2)*2,fps=%d,trim=0:%.6f,setpts=PTS-STARTPTS,"+
				"tpad=stop_mode=clone:stop_duration=%.6f,trim=0:%.6f,settb=AVTB,format=yuv420p[v%d]",
			i, opts.FPS, durations[i], tail+1.0/float64(opts.FPS), durations[i]+tail, i,
		))
	}

	var prev string
	if x > 0 {
		prev = "[v0]"
		boundary := durations[0]
		for i := 1; i < len(frames); i++ {
			outLabel := fmt.Sprintf("[vt%d]", i)
			filters = append(filters, fmt.Sprintf(
				"%s[%d:v]xfade=transition=%s:duration=%.6f:offset=%.6f%s",
				prev, i, xfadeType, x, boundary, outLabel,
			))
			prev = outLabel
			boundary += durations[i]
		}
	} else if len(frames) > 1 {
		var concatInputs []string
		for i := 0; i < len(frames); i++ {
			concatInputs = append(concatInputs, fmt.Sprintf("[v%d]", i))
		}
		filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[vout]", stringsJoin(concatInputs, ""), len(frames)))
		prev = "[vout]"
	}

	// 配音：apad 到该镜时长，concat 起来
	if len(voiceClips) > 0 {
		for i := 0; i < len(voiceClips); i++ {
			filters = append(filters, fmt.Sprintf(
				"[%d:a]apad=whole_dur=%.3f,asetpts=PTS-STARTPTS[a%d]",
				voiceStart+i, durations[i], i,
			))
		}
		var voiceParts []string
		for i := 0; i < len(voiceClips); i++ {
			voiceParts = append(voiceParts, fmt.Sprintf("[a%d]", i))
		}
		filters = append(filters, fmt.Sprintf("%sconcat=n=%d:v=0:a=1[voice]", stringsJoin(voiceParts, ""), len(voiceClips)))
	}

	// BGM（同主路径）
	if bgm != "" {
		fadeStart := maxFloat(0, total-opts.FadeOut)
		gain := opts.BGMGain
		if gain == 0 {
			if len(voiceClips) > 0 {
				gain = DefaultBGMGain
			} else {
				gain = 1
			}
		}
		var bgmFilter string
		if opts.BGMCrossfade > 0 {
			xf := opts.BGMCrossfade
			bgmFilter = fmt.Sprintf(
				"[%d:a]asplit=2[b1][b2];"+
					"[b1]atrim=0:%.3f,asetpts=PTS-STARTPTS[h];"+
					"[b2]atrim=start=%.3f,asetpts=PTS-STARTPTS[t];"+
					"[h][t]acrossfade=d=%.3f:c1=tri:c2=tri[s];"+
					"[s]aloop=loop=-1:size=2e9,atrim=0:%.3f,volume=%g,afade=t=out:st=%.3f:d=%.3f[bgm]",
				bgmIdx, xf, xf, xf, total, gain, fadeStart, opts.FadeOut,
			)
		} else {
			bgmFilter = fmt.Sprintf(
				"[%d:a]aloop=loop=-1:size=2e9,atrim=0:%.3f,volume=%g,afade=t=out:st=%.3f:d=%.3f[bgm]",
				bgmIdx, total, gain, fadeStart, opts.FadeOut,
			)
		}
		filters = append(filters, bgmFilter)
	}

	if len(voiceClips) > 0 || bgm != "" {
		filters = append(filters, BuildMixChain(
			optLabel(voiceLabels(voiceClips)), optLabel(bgmLabel(bgm)),
			opts.VoiceGain, opts.Duck, opts.Normalize, "[a]",
		)...)
	}

	args = append(args, "-filter_complex", stringsJoin(filters, ";"), "-map", prev)
	if len(voiceClips) > 0 || bgm != "" {
		args = append(args, "-map", "[a]", "-c:a", "aac", "-b:a", "192k")
	}
	args = append(args,
		"-c:v", "libx264", "-preset", "medium", "-crf", "20",
		"-pix_fmt", "yuv420p", "-r", strconv.Itoa(opts.FPS), "-movflags", "+faststart",
		"-t", strconv.FormatFloat(total, 'f', -1, 64), out,
	)

	if err := runFfmpeg(ctx, args); err != nil {
		return "", 0, err
	}
	return out, total, nil
}

// ProbeDuration 读一个媒体文件的秒数，用于校验/展示。
func ProbeDuration(ctx context.Context, file string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", file,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// hasMP4Input 是否有 mp4 输入（mp4 输入走 xfade 路径）。
func hasMP4Input(frames []string) bool {
	for _, f := range frames {
		if strings.HasSuffix(f, ".mp4") {
			return true
		}
	}
	return false
}

func writeConcatList(listFile string, frames []string, durations []float64) error {
	var b strings.Builder
	for i, f := range frames {
		escaped := strings.ReplaceAll(f, "'", "'\\''")
		b.WriteString(fmt.Sprintf("file '%s'\n", escaped))
		b.WriteString(fmt.Sprintf("duration %g\n", durations[i]))
	}
	// 重复末帧，提供末帧之后的时间戳
	last := frames[len(frames)-1]
	escaped := strings.ReplaceAll(last, "'", "'\\''")
	b.WriteString(fmt.Sprintf("file '%s'\n", escaped))
	return writeFile(listFile, b.String())
}

func voiceLabels(clips []VoiceClip) string {
	if len(clips) == 0 {
		return ""
	}
	return "[voice]"
}

func bgmLabel(bgm string) string {
	if bgm == "" {
		return ""
	}
	return "[bgm]"
}

func optLabel(s string) string {
	if s == "" {
		return ""
	}
	return s
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// mkdirAll 兼容 os.MkdirAll 的简写。
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

// writeFile 写入文件。
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// runFfmpeg 调用 ffmpeg，支持上下文取消，出错时带上 stderr 尾部。
func runFfmpeg(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		tail := strings.TrimSpace(stderr.String())
		lines := strings.Split(tail, "\n")
		if len(lines) > 14 {
			lines = lines[len(lines)-14:]
		}
		return fmt.Errorf("ffmpeg 失败: %s", strings.Join(lines, "\n"))
	}
	return nil
}
