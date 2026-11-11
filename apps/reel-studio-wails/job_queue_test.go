package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestStore 在临时目录里造一个隔离的工作台存储。
func newTestStore(t *testing.T) *StudioStore {
	t.Helper()
	root := filepath.Join(t.TempDir(), "studio")
	s, err := NewStudioStoreRoot(root)
	if err != nil {
		t.Fatalf("NewStudioStoreRoot: %v", err)
	}
	return s
}

func TestStoreProjectAndAsset(t *testing.T) {
	s := newTestStore(t)

	p, err := s.AddProject("我的第一支短片")
	if err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if p.ID == "" || p.CreatedAt == "" {
		t.Fatalf("项目字段不全: %+v", p)
	}

	// 空名与超长名要报错。
	if _, err := s.AddProject("  "); err == nil {
		t.Fatal("空项目名应当报错")
	}
	if _, err := s.AddProject(strings.Repeat("长", 81)); err == nil {
		t.Fatal("超长项目名应当报错")
	}

	a, err := s.AddAsset(p.ID, "shot.png", "image/png", []byte("\x89PNG fake"), map[string]any{
		"source": "upload", "origin": "import",
	})
	if err != nil {
		t.Fatalf("AddAsset: %v", err)
	}
	if a.File == "" {
		t.Fatal("素材应有落盘文件名")
	}
	// 文件应真的写到 media 目录里。
	if _, err := os.Stat(s.Path(a)); err != nil {
		t.Fatalf("素材未落盘: %v", err)
	}
	// 不支持的格式要报错。
	if _, err := s.AddAsset(p.ID, "x.exe", "application/x-msdownload", []byte("x"), nil); err == nil {
		t.Fatal("不支持的素材格式应当报错")
	}
	// 素材必须属于已存在的项目。
	if _, err := s.AddAsset("nope", "x.png", "image/png", []byte("x"), nil); err == nil {
		t.Fatal("不存在的项目应当报错")
	}
}

// 重启防线：排队中/执行中的任务在重启后必须变成 interrupted，
// 且带明确的「别重复付费」提示，不能悄悄变回可执行状态。
func TestStoreInterruptedOnRestart(t *testing.T) {
	root := filepath.Join(t.TempDir(), "studio")
	s1, err := NewStudioStoreRoot(root)
	if err != nil {
		t.Fatalf("第一次打开: %v", err)
	}
	p, _ := s1.AddProject("项目")
	for _, status := range []string{"queued", "running"} {
		s1.state.Jobs = append(s1.state.Jobs, &Job{
			ID: newID(), ProjectID: p.ID, Status: status,
			Input: map[string]any{}, CreatedAt: nowISO(), UpdatedAt: nowISO(),
		})
	}
	// 加一条已完成的，重启后不该被动。
	s1.state.Jobs = append(s1.state.Jobs, &Job{
		ID: newID(), ProjectID: p.ID, Status: "succeeded",
		Input: map[string]any{}, CreatedAt: nowISO(), UpdatedAt: nowISO(),
	})
	s1.Save()

	s2, err := NewStudioStoreRoot(root)
	if err != nil {
		t.Fatalf("第二次打开: %v", err)
	}
	var interrupted, succeeded int
	for _, j := range s2.State().Jobs {
		switch j.Status {
		case "interrupted":
			interrupted++
			if !strings.Contains(j.Error, "避免重复消耗额度") {
				t.Errorf("interrupted 任务缺少防重复付费提示: %q", j.Error)
			}
		case "succeeded":
			succeeded++
		default:
			t.Errorf("重启后状态不应为 %q", j.Status)
		}
	}
	if interrupted != 2 {
		t.Errorf("应有 2 条被标记 interrupted，实际 %d", interrupted)
	}
	if succeeded != 1 {
		t.Errorf("已完成任务应保持 succeeded，实际 %d", succeeded)
	}
}

// 队列要真的执行任务，并把结果写回 job。
func TestQueueRunsJob(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("项目")

	started := make(chan struct{})
	q := NewJobQueue(s, func(ctx context.Context, job *Job, progress func(string, map[string]any), report func(Step)) (string, error) {
		close(started)
		progress("干活中", map[string]any{"taskId": "up-123"})
		report(Step{ID: "s1", Label: "第一步", Status: "done"})
		return "asset-1", nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)

	job, err := q.Submit(p.ID, map[string]any{"kind": "image"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("任务没有被执行")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got := q.Job(job.ID)
		if got != nil && got.Status == "succeeded" {
			if got.AssetID != "asset-1" {
				t.Errorf("素材 id 不对: %q", got.AssetID)
			}
			if got.Upstream["taskId"] != "up-123" {
				t.Errorf("上游任务 id 应被记住（防重复付费）: %+v", got.Upstream)
			}
			if len(got.Steps) != 1 || got.Steps[0].ID != "s1" {
				t.Errorf("步骤应被记录: %+v", got.Steps)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("任务未在期限内完成: %+v", q.Job(job.ID))
}

// 取消要真的掐断执行中的任务，而不是等它跑完再把结果扔掉。
func TestQueueCancelStopsRunningJob(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("项目")

	running := make(chan struct{})
	canceled := make(chan error, 1)
	q := NewJobQueue(s, func(ctx context.Context, job *Job, progress func(string, map[string]any), report func(Step)) (string, error) {
		close(running)
		<-ctx.Done() // 真实阻塞，等取消信号
		canceled <- ctx.Err()
		return "", ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)

	job, err := q.Submit(p.ID, map[string]any{"kind": "image"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	select {
	case <-running:
	case <-time.After(5 * time.Second):
		t.Fatal("任务未开始执行")
	}

	status, err := q.Cancel(job.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if status != "canceling" {
		t.Errorf("运行中的任务取消应返回 canceling，实际 %q", status)
	}

	select {
	case err := <-canceled:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("执行体应收到 context.Canceled，实际 %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消没有掐断执行中的任务")
	}

	// 最终状态必须是 canceled，不是 failed —— 这两件事该分开。
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got := q.Job(job.ID)
		if got != nil && got.Status == "canceled" {
			if got.Error != "" {
				t.Errorf("取消不该记错误信息，实际 %q", got.Error)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("任务状态应为 canceled，实际 %+v", q.Job(job.ID))
}

// 排队的任务取消后直接结束，不必等 worker 轮到它。
func TestQueueCancelQueuedJob(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("项目")

	// 不启动 worker，任务就一直排队。
	q := NewJobQueue(s, func(ctx context.Context, job *Job, progress func(string, map[string]any), report func(Step)) (string, error) {
		return "x", nil
	})
	job, err := q.Submit(p.ID, map[string]any{"kind": "image"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	status, err := q.Cancel(job.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if status != "canceled" {
		t.Errorf("排队中的任务取消应返回 canceled，实际 %q", status)
	}
	if got := q.Job(job.ID); got.Status != "canceled" {
		t.Errorf("状态应为 canceled，实际 %q", got.Status)
	}
}

// 队列满了要拒绝新任务，别无限堆积。
func TestQueueRejectsWhenFull(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("项目")
	q := NewJobQueue(s, func(ctx context.Context, job *Job, progress func(string, map[string]any), report func(Step)) (string, error) {
		return "x", nil
	})
	for i := 0; i < 10; i++ {
		if _, err := q.Submit(p.ID, map[string]any{"kind": "image"}); err != nil {
			t.Fatalf("第 %d 条不该被拒: %v", i+1, err)
		}
	}
	if _, err := q.Submit(p.ID, map[string]any{"kind": "image"}); err == nil {
		t.Fatal("队列满时应当拒绝新任务")
	}
}

// 已结束的任务不能再取消。
func TestQueueCancelFinishedJob(t *testing.T) {
	s := newTestStore(t)
	p, _ := s.AddProject("项目")
	s.state.Jobs = append(s.state.Jobs, &Job{
		ID: newID(), ProjectID: p.ID, Status: "succeeded",
		Input: map[string]any{}, CreatedAt: nowISO(), UpdatedAt: nowISO(),
	})
	q := NewJobQueue(s, nil)
	if _, err := q.Cancel(s.state.Jobs[0].ID); err == nil {
		t.Fatal("已结束的任务不该能被取消")
	}
}

// Redact 行为要与 Node 版一致：抹掉密钥与服务地址，但不吃掉「480p/720p」这类分辨率。
func TestRedact(t *testing.T) {
	// 假密钥用拼接构造，源码里不留完整 token 形态。
	fakeKey := "sk-" + strings.Repeat("a", 20)
	cases := []struct {
		in   string
		want string
	}{
		{"Authorization: Bearer " + fakeKey, "Authorization: Bearer [redacted]"},
		{"key " + fakeKey, "key [redacted]"},
		{"见 https://api.example.com/v1/x", "见 [服务地址]"},
		{"文件在 /Users/me/work/a.mp4", "文件在 [本地路径]"},
		{"分辨率 480p/720p/1080p", "分辨率 480p/720p/1080p"},
	}
	for _, c := range cases {
		if got := Redact(c.in); got != c.want {
			t.Errorf("Redact(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}
