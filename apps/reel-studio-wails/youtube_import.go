package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// 从已发布的 YouTube 频道导入剧集。
//
// 为什么需要：一部短剧已经在多个平台发布时，工作台里的剧集应该
// 指向真实的那一集（可点开、可内嵌播放），而不是空白行。
// 只读公开 RSS，不写平台数据。

// YouTubeEpisode 一集在 YouTube 上的信息。
type YouTubeEpisode struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	VideoID   string `json:"videoId"`
	URL       string `json:"url"`
	Thumbnail string `json:"thumbnail"`
	Published string `json:"published,omitempty"`
}

// youtubeChannelRSS 拉取频道最近视频的 RSS（公开接口，无需鉴权）。
func youtubeChannelRSS(ctx context.Context, channelID string) ([]YouTubeEpisode, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, errors.New("缺少 YouTube 频道 id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.youtube.com/feeds/videos.xml?channel_id="+url.QueryEscape(channelID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取 YouTube 频道失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取 YouTube 频道失败：HTTP %d", resp.StatusCode)
	}
	var feed struct {
		Entries []struct {
			Title     string `xml:"title"`
			VideoID   string `xml:"videoId"`
			Published string `xml:"published"`
		} `xml:"entry"`
	}
	if err := xmlUnmarshal(resp.Body, &feed); err != nil {
		return nil, fmt.Errorf("解析 YouTube 频道失败: %w", err)
	}
	out := make([]YouTubeEpisode, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		out = append(out, YouTubeEpisode{
			Title:     e.Title,
			VideoID:   e.VideoID,
			URL:       "https://www.youtube.com/watch?v=" + e.VideoID,
			Thumbnail: "https://i.ytimg.com/vi/" + e.VideoID + "/hqdefault.jpg",
			Published: e.Published,
		})
	}
	return out, nil
}

// 中文集数词 → 序号。
var cnEpisodeNumbers = map[string]int{
	"序章": 0, "楔子": 0, "第一集": 1, "第二集": 2, "第三集": 3, "第四集": 4,
	"第五集": 5, "第六集": 6, "第七集": 7, "第八集": 8, "第九集": 9, "第十集": 10,
	"第十一集": 11, "第十二集": 12, "第十三集": 13, "第十四集": 14, "第十五集": 15,
}

var reEpisodeNumber = regexp.MustCompile(`第([一二三四五六七八九十]+)集`)

// parseEpisodeNumber 从标题里认集数；认不出返回 -1。
func parseEpisodeNumber(title string) int {
	if m := reEpisodeNumber.FindStringSubmatch(title); len(m) == 2 {
		if n, ok := cnEpisodeNumbers["第"+m[1]+"集"]; ok {
			return n
		}
	}
	for word, n := range cnEpisodeNumbers {
		if n > 0 && strings.Contains(title, word) {
			return n
		}
	}
	if strings.Contains(title, "序章") || strings.Contains(title, "楔子") {
		return 0
	}
	return -1
}

// ListYouTubeEpisodes 列频道里能认出局数的集（供导入预览）。
func (e *Engine) ListYouTubeEpisodes(ctx context.Context, channelID string) ([]YouTubeEpisode, error) {
	items, err := youtubeChannelRSS(ctx, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]YouTubeEpisode, 0, len(items))
	for _, it := range items {
		n := parseEpisodeNumber(it.Title)
		if n < 0 {
			continue // 花絮、预告等不入剧集
		}
		it.Number = n
		out = append(out, it)
	}
	return out, nil
}

// ImportYouTubeEpisodes 把频道剧集导入到项目（已存在的集更新发布链接，不重复建）。
func (e *Engine) ImportYouTubeEpisodes(ctx context.Context, projectID, channelID string) ([]*Episode, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	items, err := e.ListYouTubeEpisodes(ctx, channelID)
	if err != nil {
		return nil, err
	}
	existing := e.store.ListEpisodes(projectID)
	byNumber := map[int]*Episode{}
	for _, ep := range existing {
		byNumber[ep.Number] = ep
	}
	out := []*Episode{}
	for _, it := range items {
		if ep, ok := byNumber[it.Number]; ok {
			updated, err := e.store.UpdateEpisode(ep.ID, func(x *Episode) {
				x.PublishURL, x.PublishTitle = it.URL, it.Title
				x.PublishPlatform, x.VideoID, x.ThumbnailURL = "youtube", it.VideoID, it.Thumbnail
				if x.Title == "" {
					x.Title = it.Title
				}
			})
			if err != nil {
				return nil, err
			}
			out = append(out, updated)
			continue
		}
		ep, err := e.store.AddEpisode(EpisodeDraft{
			ProjectID: projectID, Number: it.Number, Title: it.Title,
			PublishURL: it.URL, PublishTitle: it.Title, PublishPlatform: "youtube",
			VideoID: it.VideoID, ThumbnailURL: it.Thumbnail,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, ep)
	}
	return out, nil
}

// xmlUnmarshal 解析 XML（标准库）。
func xmlUnmarshal(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, v)
}
