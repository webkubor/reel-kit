package app

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

func (a *App) importMedia(projectID string) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "导入制作素材", Multiple: true,
		Filters: []mygo.FileFilter{{Name: "图片、视频与音频", Extensions: []string{"png", "jpg", "jpeg", "webp", "mp4", "mp3", "wav", "m4a"}}}})
	if err != nil {
		a.fail("无法打开文件选择窗口：" + err.Error())
		return
	}
	if len(paths) == 0 {
		return
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	imported := 0
	failed := false
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			a.fail("打开素材失败：" + err.Error())
			failed = true
			break
		}
		info, err := f.Stat()
		if err != nil || info.Size() <= 0 || info.Size() > 100*1024*1024 {
			f.Close()
			a.fail("素材必须非空且不超过 100 MB")
			failed = true
			break
		}
		values := url.Values{"projectId": {projectID}, "name": {filepath.Base(path)}}
		req, err := http.NewRequest(http.MethodPost, strings.TrimRight(a.opts.BaseURL, "/")+"/api/assets?"+values.Encode(), f)
		if err != nil {
			f.Close()
			a.fail("准备上传素材失败：" + err.Error())
			failed = true
			break
		}
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		if contentType == "" {
			f.Close()
			a.fail("不支持的素材格式：" + filepath.Ext(path))
			failed = true
			break
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("x-reel-action", "studio")
		req.ContentLength = info.Size()
		res, err := client.Do(req)
		f.Close()
		if err != nil {
			a.fail("导入素材失败：" + err.Error())
			failed = true
			break
		}
		io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		res.Body.Close()
		if res.StatusCode != http.StatusCreated {
			a.fail(fmt.Sprintf("导入素材失败：HTTP %d", res.StatusCode))
			failed = true
			break
		}
		imported++
	}
	if imported > 0 {
		if !failed {
			a.mutate(func() { a.notice = fmt.Sprintf("已导入 %d 个素材", imported); a.noticeErr = false })
		}
		a.refresh()
	}
}
