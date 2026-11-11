package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 端到端：跑完整条商品介绍流水线，产出真的 mp4。
//
// 这条用例守的是「1.0 能不能出片」这件事本身：
// 逐句配音 → 按语速定镜长 → 本机 Chrome 渲染画面帧 → ffmpeg 合成 → 落素材库。
//
// 配音用替身（本机 ffmpeg 造一段等长静音），因为真配音依赖本机 voxcraft
// 引擎，测试不能假设它装了 —— 但渲染与合成必须是真的，那才是最容易坏的部分。
// 静音与语音一样有确定时长，镜头节奏的推导逻辑照样被覆盖到。
func TestProductPipelineProducesVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("本机没有 ffmpeg，跳过")
	}
	if FindChrome() == "" {
		t.Skip("本机没有 Chrome，跳过")
	}

	ctx := context.Background()
	root := t.TempDir()
	store, err := NewStudioStoreRoot(root)
	if err != nil {
		t.Fatalf("建素材库失败：%v", err)
	}
	e := &Engine{store: store, note: "test"}
	e.queue = NewJobQueue(store, e.runPipeline)

	project, err := store.AddProject("端到端测试")
	if err != nil {
		t.Fatalf("建项目失败：%v", err)
	}

	// 配音替身：每句造一段静音 wav，长度按字数估（中文约每秒 5 字）。
	e.speechOverride = func(ctx context.Context, projectID, text, tone, backend, model, voiceID, profile string, speed float64) (*Asset, error) {
		seconds := float64(len([]rune(text)))/5 + 0.4
		tmp, err := os.MkdirTemp("", "reel-test-voice-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		wav := filepath.Join(tmp, "voice.wav")
		if err := runFfmpeg(ctx, []string{
			"-v", "error", "-y",
			"-f", "lavfi", "-i", fmt.Sprintf("anullsrc=r=24000:cl=mono:d=%.3f", seconds),
			"-c:a", "pcm_s16le", wav,
		}); err != nil {
			return nil, err
		}
		return e.storeVoiceAsset(projectID, wav, map[string]any{
			"source": "测试替身", "origin": "local-offline", "engine": "静音占位",
			"voiceBackend": backend, "prompt": text,
		})
	}

	if os.Getenv("REEL_TEST_REAL_SHERPA") == "1" {
		e.speechOverride = nil
	}

	steps := []Step{}
	asset, err := e.MakeProductVideo(ctx, ProductInput{
		ProjectID:    project.ID,
		Title:        "一支笔",
		Subtitle:     "两年前开始做的事",
		Ratio:        "9:16",
		Lines:        []string{"这是我们花了两年做出来的一支笔", "笔尖是 0.2 毫米，转起来很稳"},
		VoiceBackend: "sherpa",
	}, func(text string) {}, func(s Step) { steps = append(steps, s) })
	if err != nil {
		t.Fatalf("流水线失败：%v", err)
	}
	if asset == nil {
		t.Fatal("流水线没有产出素材")
	}

	out := store.Path(asset)
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("成片文件不存在：%v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("读成片信息失败：%v", err)
	}
	if info.Size() < 10*1024 {
		t.Fatalf("成片只有 %d 字节，明显没合成出内容", info.Size())
	}

	// 成片必须真的能被 ffprobe 读成视频：尺寸、时长都要对。
	duration, err := ProbeDuration(ctx, out)
	if err != nil {
		t.Fatalf("成片时长探测失败：%v", err)
	}
	if duration < 0.5 {
		t.Fatalf("成片时长 %.2fs 太短，镜头节奏没算对", duration)
	}
	dims, err := probeVideoSize(ctx, out)
	if err != nil {
		t.Fatalf("成片尺寸探测失败：%v", err)
	}
	if dims != "1080x1920" {
		t.Fatalf("成片尺寸是 %s，期望 1080x1920（竖版 9:16）", dims)
	}

	// 每一道工序都要走完，不能有一步是空转。
	// 工序会先报 running 再报 done，这里按 id 取最终状态。
	final := map[string]Step{}
	for _, s := range steps {
		final[s.ID] = s
	}
	if len(final) < 5 {
		t.Fatalf("工序只有 %d 步，流水线没走完：%+v", len(final), final)
	}
	for id, s := range final {
		if s.Status != "done" {
			t.Fatalf("工序 %q 状态是 %q（错误：%s）", id, s.Status, s.Error)
		}
	}
	if !strings.HasSuffix(asset.MIME, "mp4") {
		t.Fatalf("成片 MIME 是 %q，期望 video/mp4", asset.MIME)
	}
}

// probeVideoSize 用 ffprobe 读视频的宽高。
func probeVideoSize(ctx context.Context, file string) (string, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x", file).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
