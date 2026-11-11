package main

import (
	"context"
	"encoding/json"
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
			if _, err := s.AddAsset(p.ID, "image", "image/png", []byte("fixture"), nil); err != nil {
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
