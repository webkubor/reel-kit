package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// bundledResource 解析随应用一起分发的离线资源（Chrome、ffmpeg、sherpa、museav）。
//
// 找得到就返回路径，找不到返回空串 —— 调用方各自决定是报错还是退回 PATH。
// 不同平台的包布局不一样，所以按候选位置挨个试，不猜平台。
//
//	macOS：  muse-tv.app/Contents/MacOS/muse-tv → ../Resources/<parts>
//	Windows：muse-tv.exe 与 Resources 同级   → ./Resources/<parts>
//	源码运行：没有 bundle，退回 PATH（mediaCommand 自己处理）
func bundledResource(parts ...string) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(executable)
	candidates := [][]string{
		append([]string{dir, "..", "Resources"}, parts...), // macOS .app bundle
		append([]string{dir, "Resources"}, parts...),       // Windows 平铺包
		append([]string{dir}, parts...),                    // 资源与 exe 同级
	}
	if runtime.GOOS == "windows" {
		// Windows 上优先平铺布局，macOS bundle 布局作为兜底
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}
	for _, c := range candidates {
		p := filepath.Join(c...)
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// bundledDir 解析随应用分发的资源目录（sherpa 这类多文件目录）。
func bundledDir(parts ...string) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(executable)
	candidates := [][]string{
		append([]string{dir, "..", "Resources"}, parts...),
		append([]string{dir, "Resources"}, parts...),
		append([]string{dir}, parts...),
	}
	if runtime.GOOS == "windows" {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}
	for _, c := range candidates {
		p := filepath.Join(c...)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return ""
}

// mediaCommand prefers the application-bundled tool; source builds use PATH.
func mediaCommand(name string) string {
	if p := bundledResource("bin", name); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		if p := bundledResource("bin", name+".exe"); p != "" {
			return p
		}
	}
	return name
}
