package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// sherpaFiles locates the existing packaged sherpa-onnx engine and model.
// REEL_SHERPA_HOME supports development and points at the same layout as the
// original desktop package. Production assets live in Contents/Resources/sherpa.
func sherpaFiles() (bin, model, tokens, lexicon, lib string, err error) {
	home := os.Getenv("REEL_SHERPA_HOME")
	if home == "" {
		var executable string
		executable, err = os.Executable()
		if err != nil {
			return
		}
		home = filepath.Join(filepath.Dir(executable), "..", "Resources", "sherpa")
	}
	name := "sherpa-onnx-offline-tts"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin = filepath.Join(home, "bin", name)
	tokens = filepath.Join(home, "model", "tokens.txt")
	lexicon = filepath.Join(home, "model", "lexicon.txt")
	lib = filepath.Join(home, "lib")
	models, globErr := filepath.Glob(filepath.Join(home, "model", "*.onnx"))
	if globErr != nil || len(models) == 0 {
		err = fmt.Errorf("未找到本地语音模型，请检查应用资源")
		return
	}
	model = models[0]
	for _, path := range []string{bin, tokens, lexicon} {
		if _, statErr := os.Stat(path); statErr != nil {
			err = fmt.Errorf("本地语音资源不完整：%w", statErr)
			return
		}
	}
	return
}

func (e *Engine) generateSherpaSpeech(ctx context.Context, projectID, text string) (*Asset, error) {
	bin, model, tokens, lexicon, lib, err := sherpaFiles()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "reel-sherpa-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "voice.wav")
	if strings.HasPrefix(text, "-") {
		text = " " + text
	}
	cmd := exec.CommandContext(ctx, bin, "--vits-model="+model, "--vits-tokens="+tokens, "--vits-lexicon="+lexicon, "--output-filename="+out, text)
	cmd.Env = os.Environ()
	if runtime.GOOS == "darwin" {
		cmd.Env = append(cmd.Env, "DYLD_LIBRARY_PATH="+lib+string(os.PathListSeparator)+os.Getenv("DYLD_LIBRARY_PATH"))
	}
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("本地语音合成失败：%s", Redact(string(output)))
	}
	return e.storeVoiceAsset(projectID, out, map[string]any{
		"source": "本机离线语音", "origin": "local-offline", "engine": "sherpa-onnx · 中文离线模型",
		"voiceBackend": "sherpa", "requestedModel": "sherpa-vits-zh", "actualModel": "unknown", "prompt": strings.TrimSpace(text),
	})
}
