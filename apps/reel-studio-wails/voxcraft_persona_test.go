package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 音色清单在 ~/.voxflow/configs/personas.json，跟二进制不在一处。
// 定位函数早退不读它，就会「体检说就绪、合成时才发现没音色」。
func TestLocateVoxcraftLoadsPersonasFromPath(t *testing.T) {
	home := t.TempDir()
	setFakeHome(t, home)

	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(home, ".voxflow", "models-mlx", "Base-1.7B-8bit"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".voxflow", "configs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".voxflow", "configs", "personas.json"),
		[]byte(`{"朝朝-元气阳光":{"name":"朝朝-元气阳光"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	info := locateVoxcraftInPath(filepath.Join(binDir, "voice"))
	if !info.OK {
		t.Fatalf("PATH 里有 voice、本机有模型就该判就绪：%+v", info)
	}
	if len(info.Personas) != 1 || info.Personas[0] != "朝朝-元气阳光" {
		t.Fatalf("PATH 分支没读音色清单：%+v", info)
	}
}

// 没有清单时返回空，不报错——那是「还没登记音色」，不是「程序坏了」。
func TestLoadPersonasMissing(t *testing.T) {
	setFakeHome(t, t.TempDir())
	if got := loadPersonas(""); len(got) != 0 {
		t.Fatalf("没有清单却读出了音色：%v", got)
	}
}

// 顺序必须稳定：默认音色不能每次随机换人。
func TestLoadPersonasSorted(t *testing.T) {
	home := t.TempDir()
	setFakeHome(t, home)
	dir := filepath.Join(home, ".voxflow", "configs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "personas.json"),
		[]byte(`{"beta":{"name":"beta"},"alpha":{"name":"alpha"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadPersonas("")
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("音色没排序：%v", got)
	}
}

// 项目目录里的旧格式只当兜底。
func TestLoadPersonasFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	setFakeHome(t, t.TempDir())
	if err := os.MkdirAll(filepath.Join(home, "configs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "configs", "personas.json"),
		[]byte(`{"老音色":{"name":"老音色"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadPersonas(home)
	if len(got) != 1 || got[0] != "老音色" {
		t.Fatalf("兜底路径没读到音色：%v", got)
	}
}

// setFakeHome 把家目录指到临时目录。os.UserHomeDir() 在 Windows 上读
// USERPROFILE、其它平台读 HOME，两个都得设，否则 Windows CI 上
// 会读到真实家目录、测试直接红。
func setFakeHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}
