package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是桌面壳本身。
//
// 壳里没有业务功能：界面由本地引擎（Engine）提供，通过 Go 绑定直接调用。
// 窗口打开时先显示引导页，引擎就绪后停留在引导页（Phase 6 会把这里换成
// 真正的本地控制台 UI，不再跳转到任何工作台地址）。
type App struct {
	engine *Engine
	ctx    context.Context
}

// NewApp 造一个桌面壳。
func NewApp() *App {
	return &App{engine: NewEngine()}
}

// startup 是窗口起来时做的事：拉起任务队列 worker，其余零 HTTP 无子进程要拉起。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.engine.Start()
}

// shutdown 是退出时做的事：停掉 worker。
func (a *App) shutdown(_ context.Context) {
	a.engine.Stop()
}

// PickFaceImage 选一张人脸照片，只返回磁盘路径。
//
// 不导入成素材：人脸是「建角色的输入」，由 0→1 流程直接交给中台。
// 先存一份进素材库再从素材库读回来只是多绕一圈，还会让素材列表
// 混进一堆不是成片素材的人脸图。
func (a *App) PickFaceImage() (string, error) {
	if a.ctx == nil {
		return "", errNoStore
	}
	paths, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择人脸照片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片", Pattern: "*.png;*.jpg;*.jpeg;*.webp"},
		},
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return string(paths[0]), nil
}

// ListCharacters 列中台角色（含完成度）。
func (a *App) ListCharacters(workspace string) ([]*Character, error) {
	return a.engine.ListCharacters(a.ctx, workspace)
}

// CreateCharacter 走完 0→1 建角色：一张脸 → 名称 + 体貌 + 五视图齐全的角色。
//
// 联网且可能计费（中台出图），所以调用方必须明确触发。
func (a *App) CreateCharacter(input CharacterCreateInput) (*CharacterResult, error) {
	return a.engine.CreateCharacter(a.ctx, input, func(text string) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "character:progress", text)
		}
	})
}

// Status 给界面问引擎状态用（Go 绑定，前端调 window.go…Status()）。
func (a *App) Status() Status {
	return a.engine.Status()
}

// PickImages 打开系统文件选择器选图片，导入到项目里。
//
// 走系统选择器而不是让界面自己搞 <input type=file>：
// 桌面上用户习惯在访达里翻目录，浏览器文件框既不像本地应用，
// 也拿不到真实的磁盘路径（拿不到路径就没法把素材存进工作台）。
func (a *App) PickImages(projectID string) ([]*Asset, error) {
	if a.ctx == nil {
		return nil, errNoStore
	}
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择图片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.gif"},
		},
	})
	if err != nil {
		return nil, err
	}
	out := []*Asset{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		mime := mimeForImage(path)
		if mime == "" {
			continue
		}
		asset, err := a.engine.store.AddAsset(projectID, fileBaseName(path), mime, data, map[string]any{
			"source": "upload",
			"origin": "import",
			"engine": "本机导入",
		})
		if err != nil {
			return nil, err
		}
		out = append(out, a.engine.store.PublicAsset(asset))
	}
	return out, nil
}

// ExportVideo saves a project video to a location chosen in the native dialog.
func (a *App) ExportVideo(projectID, assetID string) (string, error) {
	if a.ctx == nil || a.engine.store == nil {
		return "", errNoStore
	}
	if _, err := a.engine.store.Project(projectID); err != nil {
		return "", err
	}
	asset, err := a.engine.store.Asset(assetID, projectID)
	if err != nil {
		return "", err
	}
	if asset.MIME != "video/mp4" {
		return "", fmt.Errorf("只能导出 MP4 成片")
	}
	destination, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "导出成片", DefaultFilename: "reel-studio.mp4",
		Filters: []runtime.FileFilter{{DisplayName: "MP4 视频", Pattern: "*.mp4"}},
	})
	if err != nil || destination == "" {
		return "", err
	}
	source, err := os.Open(a.engine.store.Path(asset))
	if err != nil {
		return "", err
	}
	defer source.Close()
	temp, err := os.CreateTemp(filepath.Dir(destination), ".reel-export-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	_, copyErr := io.Copy(temp, source)
	closeErr := temp.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := os.Rename(temp.Name(), destination); err != nil {
		return "", err
	}
	return destination, nil
}

// mimeForImage 按扩展名猜图片类型。
func mimeForImage(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	}
	return ""
}

// fileBaseName 取文件名（不含目录）。
func fileBaseName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// mediaHandler 给界面提供本机素材。
//
// 这是 Wails 内置的资源服务器的一个处理分支：不是自己起的 HTTP 服务，
// 不占端口、不监听网络，只是把界面要的那张图/那段视频递过去。
// 界面里用 <img src="/media/<素材 id>"> 就能显示本机素材。
func (a *App) mediaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/media/") {
			http.NotFound(w, r)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/media/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		if a.engine.store == nil {
			http.NotFound(w, r)
			return
		}
		asset, err := a.engine.store.Asset(id, "")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", asset.MIME)
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, a.engine.store.Path(asset))
	})
}
