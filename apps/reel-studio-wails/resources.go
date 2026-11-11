package main

import (
	"os"
	"path/filepath"
)

// bundledResource resolves files shipped beside the macOS application executable.
func bundledResource(parts ...string) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	components := append([]string{filepath.Dir(executable), "..", "Resources"}, parts...)
	return filepath.Join(components...)
}

// mediaCommand prefers the application-bundled tool; source builds use PATH.
func mediaCommand(name string) string {
	path := bundledResource("bin", name)
	if path != "" {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return name
}
