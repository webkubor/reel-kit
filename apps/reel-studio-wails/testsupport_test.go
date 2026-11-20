package main

import "testing"

// openTestStore 开一个测试用工作台，测试结束时会关掉本地库。
//
// 本地库开着的时候 SQLite 一直占着 local.db 的文件句柄。测试用例跑完就删
// 临时目录 —— Windows 上文件还在被占用，删除直接失败，那个失败跟用例本身
// 无关，却能让整轮 CI 长红。所以句柄必须在临时目录消失之前放开。
func openTestStore(t *testing.T, root string) *StudioStore {
	t.Helper()
	s, err := NewStudioStoreRoot(root)
	if err != nil {
		t.Fatalf("打开工作台失败(%s): %v", root, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
