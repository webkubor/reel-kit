package app

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// finishedVideoIDs includes outputs from completed video-making jobs, including
// older local compositions whose generating job may no longer be present.
func (a *App) finishedVideoIDs() map[string]bool {
	ids := make(map[string]bool)
	for _, j := range a.ProjectJobs() {
		kind, _ := j.Input["kind"].(string)
		if j.Status == "succeeded" && j.AssetID != "" && (kind == "talk" || kind == "recommendation" || kind == "export") {
			ids[j.AssetID] = true
		}
	}
	for _, asset := range a.ProjectAssets() {
		if strings.HasPrefix(asset.Mime, "video/") && asset.Source == "reel-kit" {
			ids[asset.ID] = true
		}
	}
	return ids
}

// libraryAssets 返回「素材」清单：不含各条流水线的成片。
//
// 成片是流水线的产出，不是做下一条视频的原料 ——
// 混在素材里，用户会以为还得再加工一次。
func (a *App) libraryAssets() []studioapi.Asset {
	finished := a.finishedVideoIDs()
	var assets []studioapi.Asset
	for _, asset := range a.ProjectAssets() {
		if !finished[asset.ID] {
			assets = append(assets, asset)
		}
	}
	return assets
}

func (a *App) saveFinishedVideo(asset studioapi.Asset) {
	name := filepath.Base(asset.Name)
	if name == "." || name == "" {
		name = "成片.mp4"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".mp4") {
		name += ".mp4"
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: a.win, Title: "保存成片", DefaultPath: name,
		Filters: []mygo.FileFilter{{Name: "MP4 视频", Extensions: []string{"mp4"}}},
	})
	if err != nil {
		a.fail("无法打开保存窗口：" + err.Error())
		return
	}
	if path == "" {
		return
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	res, err := client.Get(strings.TrimRight(a.opts.BaseURL, "/") + "/media/" + url.PathEscape(asset.ID) + "?download=1")
	if err != nil {
		a.fail("下载成片失败：" + err.Error())
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		a.fail(fmt.Sprintf("下载成片失败：HTTP %d", res.StatusCode))
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".reel-save-*")
	if err != nil {
		a.fail("无法保存成片：" + err.Error())
		return
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, err = io.Copy(f, res.Body)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		a.fail("无法保存成片：" + err.Error())
		return
	}
	a.mutate(func() { a.notice = "成片已保存到 " + path; a.noticeErr = false })
	revealPath(path)
}
