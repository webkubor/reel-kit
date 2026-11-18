package main

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func boundShot(t *testing.T) (*Engine, *StudioStore, *EpisodeShot) {
	t.Helper()
	s := newTestStore(t)
	p, _ := s.AddProject("沸腾之雪", "series")
	if _, err := s.SetProjectWorkspace(p.ID, "ws-snow", "沸腾之雪"); err != nil {
		t.Fatal(err)
	}
	ep, _ := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Title: "第3集"})
	sh, err := s.AddShot(ShotInput{ProjectID: p.ID, EpisodeID: ep.ID, Title: "长街回望",
		Action: "雨夜长街回望", CharacterID: "char-1", Look: "王服", SceneID: "scene-1", Lines: []string{"惊鸿枪落"}})
	if err != nil {
		t.Fatal(err)
	}
	return &Engine{store: s}, s, sh
}

// 出镜参数全部来自镜头：角色、装扮、场景描述、绑定的工作区；结果回填镜头。
func TestShootShotUsesShotCastAndBackfills(t *testing.T) {
	e, s, sh := boundShot(t)
	e.sceneOverride = func(_ context.Context, ws, id string) (string, error) {
		if ws != "ws-snow" || id != "scene-1" {
			t.Fatalf("场景查询参数错：%s %s", ws, id)
		}
		return "雨夜石板街", nil
	}
	var got CharacterVideoInput
	e.shootOverride = func(_ context.Context, in CharacterVideoInput) (*Asset, error) {
		got = in
		return s.AddAsset(in.ProjectID, "clip.mp4", "video/mp4", []byte("clip"), nil)
	}
	out, err := e.ShootShot(context.Background(), sh.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubjectID != "char-1" || got.Look != "王服" || got.Scene != "雨夜石板街" || got.ExpectWorkspace != "ws-snow" {
		t.Fatalf("出镜参数没从镜头来：%+v", got)
	}
	if out.Status != "shot" || out.ClipAssetID == "" {
		t.Fatalf("没回填：%+v", out)
	}
}

// 出镜失败退回待拍；没选角色/没绑定都不准出镜（不能白花钱）。
func TestShootShotGuards(t *testing.T) {
	e, s, sh := boundShot(t)
	e.sceneOverride = func(context.Context, string, string) (string, error) { return "", nil }
	e.shootOverride = func(context.Context, CharacterVideoInput) (*Asset, error) { return nil, errors.New("上游失败") }
	if _, err := e.ShootShot(context.Background(), sh.ID, nil, nil); err == nil {
		t.Fatal("应报上游错误")
	}
	// 上游失败不等于没收单：标「待核对」，不能被再次提交
	if cur, _ := s.EpisodeShot(sh.ID); cur.Status != "interrupted" {
		t.Fatalf("上游失败应标待核对：%s", cur.Status)
	}
	if _, err := e.ShootShot(context.Background(), sh.ID, nil, nil); err == nil {
		t.Fatal("待核对的镜头不应能直接重新出镜")
	}
	if cur, err := e.ResolveInterruptedShot(sh.ID); err != nil || cur.Status != "planned" {
		t.Fatalf("核对后应回到待拍：%+v %v", cur, err)
	}
	if _, err := e.ResolveInterruptedShot(sh.ID); err == nil {
		t.Fatal("非待核对状态不应能核对")
	}
	if _, err := e.SetShotCast(ShotCast{ID: sh.ID, Action: "x", CharacterID: ""}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ShootShot(context.Background(), sh.ID, nil, nil); err == nil || !strings.Contains(err.Error(), "角色") {
		t.Fatalf("没角色应拒绝：%v", err)
	}
	p, _ := s.AddProject("未绑定", "series")
	ep, _ := s.AddEpisode(EpisodeDraft{ProjectID: p.ID, Title: "x"})
	un, _ := s.AddShot(ShotInput{ProjectID: p.ID, EpisodeID: ep.ID, Action: "x", CharacterID: "c"})
	if _, err := e.ShootShot(context.Background(), un.ID, nil, nil); !errors.Is(err, errProjectUnbound) {
		t.Fatalf("未绑定应拒绝：%v", err)
	}
}

// 同一镜头并发点出镜，只能付一次钱。
func TestShootShotOnlyOnceConcurrently(t *testing.T) {
	e, s, sh := boundShot(t)
	e.sceneOverride = func(context.Context, string, string) (string, error) { return "", nil }
	var calls int32
	gate := make(chan struct{})
	e.shootOverride = func(_ context.Context, in CharacterVideoInput) (*Asset, error) {
		atomic.AddInt32(&calls, 1)
		<-gate
		return s.AddAsset(in.ProjectID, "c.mp4", "video/mp4", []byte("c"), nil)
	}
	started := make(chan struct{})
	firstDone := make(chan error, 1)
	originalShoot := e.shootOverride
	e.shootOverride = func(ctx context.Context, in CharacterVideoInput) (*Asset, error) {
		close(started)
		return originalShoot(ctx, in)
	}
	go func() { _, err := e.ShootShot(context.Background(), sh.ID, nil, nil); firstDone <- err }()
	<-started
	var wg sync.WaitGroup
	rejected := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := e.ShootShot(context.Background(), sh.ID, nil, nil); rejected <- err }()
	}
	wg.Wait() // 第一笔仍被 gate 阻塞，所有竞争请求必须先被拒绝。
	close(rejected)
	for err := range rejected {
		if err == nil {
			t.Fatal("并发请求应被拒绝")
		}
	}
	close(gate)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("并发出镜调用了 %d 次", calls)
	}
}

// 换角色后旧素材作废；出镜中不准改。
func TestSetShotCastInvalidatesClip(t *testing.T) {
	e, s, sh := boundShot(t)
	_, _ = s.UpdateShot(sh.ID, func(x *EpisodeShot) { x.ClipAssetID, x.Status = "a1", "shot" })
	out, err := e.SetShotCast(ShotCast{ID: sh.ID, Action: "回望", CharacterID: "char-2", Look: "王服", SceneID: "scene-1"})
	if err != nil || out.ClipAssetID != "" || out.Status != "planned" {
		t.Fatalf("换角色应清空素材：%+v %v", out, err)
	}
	_, _ = s.UpdateShot(sh.ID, func(x *EpisodeShot) { x.Status = "shooting" })
	if _, err := e.SetShotCast(ShotCast{ID: sh.ID, Action: "y"}); err == nil {
		t.Fatal("出镜中不应能改")
	}
}

// 成片：真 ffmpeg，从镜头素材 + 镜头旁白出片并回填。
func TestFinishShotBackfills(t *testing.T) {
	if _, err := exec.LookPath(mediaCommand("ffmpeg")); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	e, s, sh := boundShot(t)
	e.speechOverride = fakeSpeech(t, s)
	clip := fakeH3Clip(t, s, sh.ProjectID, true)
	_, _ = s.UpdateShot(sh.ID, func(x *EpisodeShot) { x.ClipAssetID, x.Status = clip.ID, "shot" })
	out, err := e.FinishShot(context.Background(), sh.ID, true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "done" || out.AssetID == "" || out.AssetID == clip.ID {
		t.Fatalf("成片没回填：%+v", out)
	}
}

// 提交前的校验失败（场景不属于本项目等）可安全恢复，不该卡在待核对。
func TestShootShotPreSubmitFailureRestores(t *testing.T) {
	e, s, sh := boundShot(t)
	e.sceneOverride = func(context.Context, string, string) (string, error) {
		return "", errors.New("场景不属于本项目")
	}
	e.shootOverride = func(context.Context, CharacterVideoInput) (*Asset, error) { t.Fatal("不应提交"); return nil, nil }
	if _, err := e.ShootShot(context.Background(), sh.ID, nil, nil); err == nil {
		t.Fatal("应报场景错误")
	}
	if cur, _ := s.EpisodeShot(sh.ID); cur.Status != "planned" {
		t.Fatalf("提交前失败应恢复：%s", cur.Status)
	}
}

// 重启时仍在出镜的镜头标为待核对。
func TestShootingShotInterruptedOnRestart(t *testing.T) {
	_, s, sh := boundShot(t)
	_, _ = s.UpdateShot(sh.ID, func(x *EpisodeShot) { x.Status = "shooting" })
	reopened, err := NewStudioStoreRoot(s.Root())
	if err != nil {
		t.Fatal(err)
	}
	if cur, _ := reopened.EpisodeShot(sh.ID); cur.Status != "interrupted" {
		t.Fatalf("重启后应待核对：%s", cur.Status)
	}
}
