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

// registerVoice 用一段录音注册一个自己的音色（声音克隆）。
//
// 为什么放在桌面端而不是只留命令行：
//   「用自己的声音」是录制类内容最刚性的需求，但注册一次要准备两样东西 ——
//   录音文件和录音里念的原文。命令行里给这两个参数很别扭，
//   这里用一个表单把两样都收齐，选完文件就能注册。
//
// 录音留在本机：文件只发到本机工作台（127.0.0.1），再由它交给本机语音服务。
func (a *App) registerVoice() {
	if strings.TrimSpace(a.voiceName) == "" {
		a.fail("先给这个音色起个名字")
		return
	}
	if strings.TrimSpace(a.voiceRefText) == "" {
		a.fail("还要填录音原文：录音里你念的是哪段话")
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "选择录音文件",
		Filters: []mygo.FileFilter{{Name: "录音", Extensions: []string{"wav", "mp3", "m4a", "aac", "flac", "ogg"}}}})
	if err != nil {
		a.fail("无法打开文件选择窗口：" + err.Error())
		return
	}
	if len(paths) == 0 {
		return
	}
	path := paths[0]
	f, err := os.Open(path)
	if err != nil {
		a.fail("打开录音失败：" + err.Error())
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() <= 0 || info.Size() > 20*1024*1024 {
		a.fail("录音必须非空且不超过 20 MB")
		return
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	values := url.Values{
		"name":          {strings.TrimSpace(a.voiceName)},
		"referenceText": {strings.TrimSpace(a.voiceRefText)},
		"source":        {"import"},
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(a.opts.BaseURL, "/")+"/api/voice/record?"+values.Encode(), f)
	if err != nil {
		a.fail("准备注册音色失败：" + err.Error())
		return
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-reel-action", "studio")
	req.ContentLength = info.Size()

	client := &http.Client{Timeout: 5 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		a.fail("注册音色失败：" + err.Error())
		return
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusCreated {
		a.fail(fmt.Sprintf("注册音色失败：HTTP %d %s", res.StatusCode, strings.TrimSpace(string(body))))
		return
	}
	a.mutate(func() {
		a.notice = "音色已注册，可以在流水线里选了"
		a.noticeErr = false
		a.voiceImportOpen = false
		a.voiceName = ""
		a.voiceRefText = ""
	})
	a.refresh()
}
