package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// 本机配置 ~/.reel-kit/config.json —— 后端也在读这个文件。
//
// 后端读它拿 minimaxApiKey（MiniMax 云端配音的密钥）与 bgmManifest（配乐清单）。
// 桌面版只写 minimaxApiKey 这一个键，别的键原样保留：
// 这是用户的配置文件，不是我们的存储。

// reelConfigPath 返回本机配置的路径；取不到用户目录时返回空串。
func reelConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".reel-kit", "config.json")
}

// reelConfigPathFn 指向 reelConfigPath，测试里换成临时目录 ——
// 测试不该读用户真正的家目录配置。
var reelConfigPathFn = reelConfigPath

// readReelConfig 读本机配置。读不到或格式不对就当是空配置 ——
// 配置文件坏了不该让整个应用起不来。
func readReelConfig() map[string]any {
	out := map[string]any{}
	path := reelConfigPathFn()
	if path == "" {
		return out
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// minimaxKeyConfigured 报告本机是否已配置 MiniMax 密钥。
//
// 只回「有没有」和末四位，不回明文：这块界面会被截图、被转发。
func minimaxKeyConfigured() (bool, string) {
	key, _ := readReelConfig()["minimaxApiKey"].(string)
	key = strings.TrimSpace(key)
	if key == "" {
		return false, ""
	}
	if len(key) <= 4 {
		return true, "****"
	}
	return true, "****" + key[len(key)-4:]
}

// saveMinimaxKey 把密钥写进本机配置。
//
// 权限给 0600：密钥不该被同机的其他账号读走。
// 目录给 0700，同理。
func saveMinimaxKey(key string) error {
	path := reelConfigPathFn()
	if path == "" {
		return errors.New("找不到用户目录")
	}
	cfg := readReelConfig()
	cfg["minimaxApiKey"] = strings.TrimSpace(key)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	// 已存在的文件不会被 WriteFile 改权限，这里补一刀。
	return os.Chmod(path, 0o600)
}
