package app

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
	"github.com/webkubor/reel-kit/apps/reel-studio/studioapi"
)

// mediaCache 缓存预览用的位图与视频播放器。
//
// 视图每一帧都会重建，媒体必须在后台加载一次、之后反复取用；
// 否则每帧都去下载一遍，窗口会卡死。
//
// 它有自己的锁：View 渲染时已经持有状态锁，不能再拿同一把锁。
type mediaCache struct {
	mu      sync.Mutex
	images  map[string]*ui.Bitmap
	videos  map[string]*ui.VideoPlayer
	failed  map[string]string
	loading map[string]bool
}

const maxPreviewBytes = 60 << 20

// mediaFor 返回素材的预览；没加载过就在后台开始加载。
func (a *App) mediaFor(asset studioapi.Asset) (*ui.Bitmap, *ui.VideoPlayer, string, bool) {
	a.media.mu.Lock()
	defer a.media.mu.Unlock()
	m := &a.media
	if m.images == nil {
		m.images = map[string]*ui.Bitmap{}
		m.videos = map[string]*ui.VideoPlayer{}
		m.failed = map[string]string{}
		m.loading = map[string]bool{}
	}
	if bm := m.images[asset.ID]; bm != nil {
		return bm, nil, "", false
	}
	if vp := m.videos[asset.ID]; vp != nil {
		return nil, vp, "", false
	}
	if reason := m.failed[asset.ID]; reason != "" {
		return nil, nil, reason, false
	}
	if asset.URL == "" {
		return nil, nil, "后端没有给出素材地址", false
	}
	if !m.loading[asset.ID] {
		m.loading[asset.ID] = true
		go a.loadMedia(asset)
	}
	return nil, nil, "", true
}

// loadMedia 在后台下载并解码一条素材。
func (a *App) loadMedia(asset studioapi.Asset) {
	url := strings.TrimRight(a.opts.BaseURL, "/") + asset.URL
	var (
		bm  *ui.Bitmap
		vp  *ui.VideoPlayer
		err error
	)
	if strings.HasPrefix(asset.Mime, "image/") {
		bm, err = fetchBitmap(url)
	} else {
		vp, err = video.OpenURL(url)
	}
	a.media.mu.Lock()
	delete(a.media.loading, asset.ID)
	switch {
	case err != nil:
		a.media.failed[asset.ID] = err.Error()
	case bm != nil:
		a.media.images[asset.ID] = bm
	case vp != nil:
		a.media.videos[asset.ID] = vp
	}
	a.media.mu.Unlock()
	a.redraw()
}

func fetchBitmap(url string) (*ui.Bitmap, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("素材接口返回 %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxPreviewBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPreviewBytes {
		return nil, fmt.Errorf("图片超过 %d MB，不在窗口里预览", maxPreviewBytes>>20)
	}
	return ui.DecodeBitmap(data)
}
