package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 造一条「H3 出镜素材」：15 秒、16:9 小尺寸、带一条环境声音轨。
func fakeH3Clip(t *testing.T, s *StudioStore, projectID string, withAudio bool) *Asset {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "clip.mp4")
	args := []string{"-y", "-v", "error", "-f", "lavfi", "-i", "color=c=gray:s=160x90:r=30:d=15"}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=220:duration=15", "-c:a", "aac")
	}
	args = append(args, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-shortest", f)
	if err := runFfmpeg(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(f)
	a, err := s.AddAsset(projectID, fmt.Sprintf("夜惊鸿-出镜-%v.mp4", withAudio), "video/mp4", data, nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// 语音替身：每句按字数出一段 0.25 秒/字的正弦音，模拟真实配音时长。
func fakeSpeech(t *testing.T, s *StudioStore) func(ctx context.Context, projectID, text, tone, backend, model, voiceID, profile string, speed float64) (*Asset, error) {
	return func(ctx context.Context, projectID, text, tone, backend, model, voiceID, profile string, speed float64) (*Asset, error) {
		d := 0.25 * float64(len([]rune(text)))
		f := filepath.Join(t.TempDir(), "v.wav")
		if err := runFfmpeg(ctx, []string{"-y", "-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=660:duration=%.2f", d), f}); err != nil {
			return nil, err
		}
		data, _ := os.ReadFile(f)
		return s.AddAsset(projectID, "voice.wav", "audio/wav", data, map[string]any{"note": text})
	}
}

func probeStreams(t *testing.T, f string) string {
	out, err := exec.Command(mediaCommand("ffprobe"), "-v", "quiet", "-show_entries", "stream=codec_type", "-of", "csv=p=0", f).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(string(out)), ",")
}

// 真 ffmpeg 端到端：出镜素材 + 旁白 + 环境声 → 一集，时长不变、有音轨、画面原样。
func TestMakeEpisodeEndToEnd(t *testing.T) {
	if _, err := exec.LookPath(mediaCommand("ffmpeg")); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	s := newTestStore(t)
	p, _ := s.AddProject("沸腾之雪")
	e := &Engine{store: s}
	e.speechOverride = fakeSpeech(t, s)

	for _, tc := range []struct {
		name    string
		audio   bool
		lines   []string
		ambient bool
		wantAud bool
	}{
		{"旁白+环境声", true, []string{"江湖人只见过那杆枪", "没人见过持枪的人"}, true, true},
		{"只要环境声", true, nil, true, true},
		{"素材无音轨也能配旁白", false, []string{"惊鸿枪落"}, true, true},
		{"什么都不要=静音", true, nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clip := fakeH3Clip(t, s, p.ID, tc.audio)
			steps := []string{}
			a, err := e.MakeEpisode(context.Background(), EpisodeInput{
				ProjectID: p.ID, ClipAssetID: clip.ID, Lines: tc.lines, KeepAmbient: tc.ambient, Title: tc.name,
			}, nil, func(st Step) { steps = append(steps, st.ID+":"+st.Status) })
			if err != nil {
				t.Fatalf("%v steps=%v", err, steps)
			}
			out := s.Path(a)
			d, err := ProbeDuration(context.Background(), out)
			if err != nil || math.Abs(d-15) > 0.15 {
				t.Fatalf("成片应保持 15 秒，实际 %.3f（%v）", d, err)
			}
			streams := probeStreams(t, out)
			if strings.Contains(streams, "audio") != tc.wantAud {
				t.Fatalf("音轨期望 %v，实际流 %s", tc.wantAud, streams)
			}
			if !strings.Contains(steps[len(steps)-1], "compose:done") {
				t.Fatalf("工序没走完：%v", steps)
			}
		})
	}
}

// 旁白超出镜头：报错让人删句，绝不拉长画面或加速念白。
func TestMakeEpisodeNarrationTooLong(t *testing.T) {
	if _, err := exec.LookPath(mediaCommand("ffmpeg")); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	s := newTestStore(t)
	p, _ := s.AddProject("沸腾之雪")
	e := &Engine{store: s}
	e.speechOverride = fakeSpeech(t, s)
	clip := fakeH3Clip(t, s, p.ID, true)
	long := strings.Repeat("雪", 30) // 30 字 × 0.25s = 7.5s
	_, err := e.MakeEpisode(context.Background(), EpisodeInput{
		ProjectID: p.ID, ClipAssetID: clip.ID, Lines: []string{long, long}, KeepAmbient: true,
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "请删减旁白") {
		t.Fatalf("超时应报删减提示，实际 %v", err)
	}
}

func TestEpisodeLinesLimits(t *testing.T) {
	if _, err := episodeLines([]string{"a", "b", "c", "d", "e", "f", "g"}); err == nil {
		t.Fatal("超过 6 句应报错")
	}
	got, err := episodeLines([]string{" 一 ", "", "二"})
	if err != nil || len(got) != 2 {
		t.Fatalf("空行应被忽略：%v %v", got, err)
	}
	if math.Abs(narrationLength([]float64{2, 3})-(episodeLeadIn+2+episodePause+3)) > 1e-9 {
		t.Fatal("旁白时长算错")
	}
}
