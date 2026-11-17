package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// 场景矩阵：提交参数/返回值隔离；轮询快照不随后台改变；项目、素材写入与
// 队列进度并发；重开存储后全部记录保留。由 -race 检查共享状态访问。
func TestQueueSnapshots(t *testing.T) {
	s := newTestStore(t)
	p, err := s.AddProject("snapshot")
	if err != nil {
		t.Fatal(err)
	}
	q := NewJobQueue(s, nil)
	input := map[string]any{"lines": []any{"original"}}
	submitted, err := q.Submit(p.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	before := q.Job(submitted.ID)
	input["lines"].([]any)[0] = "changed"
	submitted.Status = "failed"
	if got := q.Job(submitted.ID); got.Status != "queued" || got.Input["lines"].([]any)[0] != "original" {
		t.Fatal("caller mutations changed queued job")
	}
	if _, err := q.Cancel(submitted.ID); err != nil {
		t.Fatal(err)
	}
	if before.Status != "queued" {
		t.Fatal("background mutation changed an earlier snapshot")
	}
}

func TestStoreConcurrentProgressAndImports(t *testing.T) {
	s := newTestStore(t)
	p, err := s.AddProject("concurrent")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	q := NewJobQueue(s, func(ctx context.Context, job *Job, progress func(string, map[string]any), report func(Step)) (string, error) {
		<-start
		for i := 0; i < 30; i++ {
			progress("rendering", map[string]any{"step": i})
			report(Step{ID: "frame", Status: "running"})
		}
		return "result", nil
	})
	job, err := q.Submit(p.ID, map[string]any{"kind": "product"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); q.pump(context.Background()) }()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 30; i++ {
			// 各用不同内容：同内容会被去重成一份，那测的就不是并发写不丢了。
			if _, err := s.AddAsset(p.ID, "image", "image/png", []byte(fmt.Sprintf("fixture-%d", i)), nil); err != nil {
				t.Error(err)
				return
			}
			if _, err := s.AddProject("another"); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 100; i++ {
			for _, value := range []any{s.State(), q.Jobs(), q.Job(job.ID)} {
				if _, err := json.Marshal(value); err != nil {
					t.Error(err)
				}
			}
		}
	}()
	close(start)
	wg.Wait()
	if got := q.Job(job.ID); got.Status != "succeeded" {
		t.Fatalf("job: %+v", got)
	}
	reopened, err := NewStudioStoreRoot(s.Root())
	if err != nil {
		t.Fatal(err)
	}
	state := reopened.State()
	if len(state.Projects) != 31 || len(state.Assets) != 30 || len(state.Jobs) != 1 || state.Jobs[0].Status != "succeeded" {
		t.Fatal("concurrent writes were not all persisted")
	}
}

// 并发导入同一张图：只有第一份落盘，其余都拿到同一份 ——
// 去重必须在锁内判，否则并发下每份都以为自己是首份，照样写十份。
func TestStoreConcurrentDedup(t *testing.T) {
	s := newTestStore(t)
	p, err := s.AddProject("dedup")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make([]*Asset, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a, err := s.AddAsset(p.ID, fmt.Sprintf("same-%d.png", i), "image/png", []byte("同一张图"), nil)
			if err != nil {
				t.Error(err)
				return
			}
			results[i] = a
		}(i)
	}
	wg.Wait()
	first := results[0]
	if first == nil {
		t.Fatal("首份没落盘")
	}
	for i, a := range results {
		if a == nil {
			t.Fatalf("%d 为空", i)
		}
		if a.ID != first.ID {
			t.Fatalf("%d 拿到不同素材 %s，应全部去重到 %s", i, a.ID, first.ID)
		}
	}
	if got := len(s.State().Assets); got != 1 {
		t.Fatalf("应只存 1 份，实际 %d 份", got)
	}
}
