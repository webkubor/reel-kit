package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 模板随应用打包。
//
//go:embed all:templates
var templateFS embed.FS

// TemplateNames 当前打包进去的模板名。
var TemplateNames = []string{
	"knowledge-dark",
	"knowledge-gufeng",
	"knowledge-paper",
	"landscape-product",
	"music-card",
	"product-presenter",
	"project-recommendation",
	"quote",
	"sticker-promo",
	"sticker-square",
}

// ReadTemplate 取模板 HTML。
func ReadTemplate(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("缺少版式名")
	}
	// 不接受带路径的名字：模板名只从已知清单里选，
	// 免得 ../ 之类的输入把包外文件读进来。
	for _, known := range TemplateNames {
		if known == name {
			break
		}
	}
	data, err := templateFS.ReadFile("templates/" + name + ".html")
	if err != nil {
		return "", fmt.Errorf("找不到版式 %s", name)
	}
	return string(data), nil
}

// TemplateSourcePath 模板在仓库里的位置（用于同步检查与开发时热改）。
func TemplateSourcePath(name string) string {
	return filepath.Join("templates", name+".html")
}

// WriteTemplateTo 把模板写到磁盘（渲染时用；渲染需要真实文件路径）。
func WriteTemplateTo(name, dir string) (string, error) {
	content, err := ReadTemplate(name)
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, name+".html")
	if err := os.WriteFile(out, []byte(content), 0o600); err != nil {
		return "", err
	}
	return out, nil
}
