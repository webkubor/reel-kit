package main

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *LocalDB {
	t.Helper()
	db, err := OpenLocalDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// 同内容重复导入是同一份素材：按 hash 去重，不按文件名。
func TestLocalDBDedup(t *testing.T) {
	db := openTestDB(t)
	data := []byte("同一张图，从两个路径导进来")
	h := HashBytes(data)

	first, isNew, err := db.PutAsset(AssetRow{ID: "a1", ProjectID: "p1", Name: "图.png", MIME: "image/png", File: "f1.png", Hash: h, Bytes: int64(len(data))})
	if err != nil || !isNew {
		t.Fatalf("首次登记应新建：err=%v isNew=%v", err, isNew)
	}

	// 换个文件名、换个导入路径，内容没变 → 应返回已有的那条
	dup, isNew2, err := db.PutAsset(AssetRow{ID: "a2", ProjectID: "p1", Name: "图-副本.png", MIME: "image/png", File: "f2.png", Hash: h, Bytes: int64(len(data))})
	if err != nil || isNew2 {
		t.Fatalf("重复内容不该新建：err=%v isNew=%v", err, isNew2)
	}
	if dup.ID != first.ID {
		t.Fatalf("应回到已有素材 %s，实际 %s", first.ID, dup.ID)
	}

	// 不同项目不算重复：每个项目的片子是独立的
	other, isNew3, err := db.PutAsset(AssetRow{ID: "a3", ProjectID: "p2", Name: "图.png", MIME: "image/png", File: "f3.png", Hash: h, Bytes: int64(len(data))})
	if err != nil || !isNew3 || other.ID == first.ID {
		t.Fatalf("跨项目不该去重：err=%v isNew=%v id=%s", err, isNew3, other.ID)
	}
}

// 缺 hash 不能入库 —— 没指纹就没法去重，等于白存。
func TestLocalDBRejectsHashless(t *testing.T) {
	db := openTestDB(t)
	if _, _, err := db.PutAsset(AssetRow{ID: "x", ProjectID: "p", Name: "n", MIME: "image/png", File: "f"}); err == nil {
		t.Fatal("缺 hash 应报错")
	}
}

// 任务落库是为崩溃恢复：重启时能把「还在跑」的捞出来标成中断。
func TestLocalDBInterruptedJobs(t *testing.T) {
	db := openTestDB(t)
	if err := db.PutJob(JobRow{ID: "j1", ProjectID: "p1", Kind: "render", Status: "running", Input: map[string]any{"k": "v"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.PutJob(JobRow{ID: "j2", ProjectID: "p1", Kind: "render", Status: "done"}); err != nil {
		t.Fatal(err)
	}

	interrupted, err := db.InterruptedJobs()
	if err != nil {
		t.Fatal(err)
	}
	if len(interrupted) != 1 || interrupted[0].ID != "j1" {
		t.Fatalf("应捞出 j1 一条在跑的，实际 %v", interrupted)
	}

	// 标记中断后不该再被捞出
	if err := db.UpdateJobStatus("j1", "interrupted", "进程退出时中断"); err != nil {
		t.Fatal(err)
	}
	again, err := db.InterruptedJobs()
	if err != nil || len(again) != 0 {
		t.Fatalf("标记后不该再捞出，实际 %v err=%v", again, err)
	}
}

// input/output 往返不丢，出错信息要能存住。
func TestLocalDBJobRoundTrip(t *testing.T) {
	db := openTestDB(t)
	err := db.PutJob(JobRow{
		ID: "j1", ProjectID: "p1", Kind: "render", Status: "failed",
		Input: map[string]any{"ratio": "16:9", "lines": 4}, Output: map[string]any{"assetId": "a1"},
		Error: "ffmpeg exit 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.GetJob("j1")
	if err != nil || got == nil {
		t.Fatalf("读回失败：%v", err)
	}
	if got.Input["ratio"] != "16:9" || got.Output["assetId"] != "a1" || got.Error != "ffmpeg exit 1" {
		t.Fatalf("往返丢数据：%+v", got)
	}

	// 状态更新不该冲掉 input
	if err := db.UpdateJobStatus("j1", "queued", ""); err != nil {
		t.Fatal(err)
	}
	after, _ := db.GetJob("j1")
	if after.Input["ratio"] != "16:9" || after.Status != "queued" {
		t.Fatalf("状态更新伤到了 input：%+v", after)
	}
}
