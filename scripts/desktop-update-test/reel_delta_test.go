package update

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestPublishedDelta 用发布页上的增量包把旧版应用还原成新版，逐个文件比 SHA-256。
//
// 这是发版后最该验的一步：增量包算错了，装更新的用户拿到的是坏应用，
// 而签名只保证「包没被改过」，不保证「包是对的」。
func TestPublishedDelta(t *testing.T) {
	deltaFile := os.Getenv("DELTA_FILE")
	oldDir := os.Getenv("OLD_DIR")
	newDir := os.Getenv("NEW_DIR")
	from := os.Getenv("FROM")
	to := os.Getenv("TO")
	if deltaFile == "" || oldDir == "" || newDir == "" {
		t.Skip("没有 DELTA_FILE/OLD_DIR/NEW_DIR，跳过")
	}
	f, err := os.Open(deltaFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "restored")
	if err := ApplyDelta(f, info.Size(), from, to, oldDir, out); err != nil {
		t.Fatalf("还原失败：%v", err)
	}
	want, err := hashTree(newDir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := hashTree(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatalf("文件数对不上：期望 %d，实际 %d", len(want), len(got))
	}
	for path, sum := range want {
		other, ok := got[path]
		if !ok {
			t.Errorf("少了文件 %s", path)
			continue
		}
		if other != sum {
			t.Errorf("文件 %s 的内容对不上", path)
		}
	}
	t.Logf("还原出 %d 个文件，逐个 SHA-256 一致", len(got))
}

// hashTree 算一棵目录树里每个文件的 SHA-256，键是相对路径。
func hashTree(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	return out, err
}
