package main

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

// Matrix: one animated shot (UI minimum), multiple cuts, multiple fades.
// Each uses actual MP4 inputs and checks output duration, covering filter linkage.
func TestComposeAnimatedShots(t *testing.T) {
	if _, err := exec.LookPath(mediaCommand("ffmpeg")); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	ctx := context.Background()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.mp4")
	if err := runFfmpeg(ctx, []string{"-y", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=30:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", input}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		count      int
		transition string
	}{
		{"single", 1, "none"}, {"cuts", 2, "none"}, {"fade", 2, "fade"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frames := make([]string, tc.count)
			durations := make([]float64, tc.count)
			for i := range frames {
				frames[i] = input
				durations[i] = 0.7
			}
			out := filepath.Join(dir, tc.name+".mp4")
			_, want, err := Compose(ctx, frames, durations, "", nil, out, ComposeOpts{FPS: 30, Transition: tc.transition, TransitionDur: 0.2})
			if err != nil {
				t.Fatal(err)
			}
			got, err := ProbeDuration(ctx, out)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-want) > 0.1 {
				t.Fatalf("duration = %v, want %v", got, want)
			}
		})
	}
}
