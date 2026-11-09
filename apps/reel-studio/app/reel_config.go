package app

import (
	"encoding/json"
	"errors"
	"fmt"
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

// readReelConfigForWrite 读本机配置，供保存时改写。
//
// 与 readReelConfig 的区别：这里对「文件在但读不出」报错。
// 配置文件坏了的时候要是当成空配置接着写，
// 用户原来那个文件里的东西（bgmManifest 等）就被抹掉了 ——
// 那是用户自己的数据，宁可让保存失败，也不要替用户做这个决定。
func readReelConfigForWrite() (map[string]any, error) {
	path := reelConfigPathFn()
	if path == "" {
		return nil, errors.New("找不到用户目录")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("配置文件 %s 读不出来，先修好再保存密钥（内容没动）", path)
	}
	return out, nil
}

// saveMinimaxKey 把密钥写进本机配置。
//
// 权限给 0600：密钥不该被同机的其他账号读走。
// 目录给 0700，同理。
//
// 写文件用「临时文件 + 改名」：直接 WriteFile 的话，
// 中途断电或进程被杀会留下半个文件，用户的配置就废了。
func saveMinimaxKey(key string) error {
	path := reelConfigPathFn()
	if path == "" {
		return errors.New("找不到用户目录")
	}
	cfg, err := readReelConfigForWrite()
	if err != nil {
		return err
	}
	cfg["minimaxApiKey"] = strings.TrimSpace(key)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	// 已存在的文件不会被 WriteFile 改权限，这里补一刀。
	return os.Chmod(path, 0o600)
}
