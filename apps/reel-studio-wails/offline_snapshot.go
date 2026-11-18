package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 中台离线快照。
//
// 角色与场景都住在 MUSE AV 中台，但工作台不能一断网就什么都看不见。
// 两层兜底，按新鲜程度排：
//
//  1. subjects-cache/<工作区>.json：每次联网列主体成功就写一份；
//  2. museav-snapshots/<名>/：一次性整体导入的快照（manifest.json +
//     subjects/<主体 id>/subject.json + 原图），断网时角色、场景、场景参考图都从这里出。
//
// 快照是只读的：只拿来看，不拿来写回中台。

const subjectsCacheDir = "subjects-cache"

// snapshotManifest 快照清单（只取用得到的字段）。
type snapshotManifest struct {
	WorkspaceID string `json:"workspaceId"`
	CapturedAt  string `json:"capturedAt"`
}

// snapshotSubjectFile 快照里单个主体的 subject.json。
type snapshotSubjectFile struct {
	Subject Subject `json:"subject"`
	Assets  []struct {
		ID        string `json:"id"`
		Role      string `json:"role"`
		Name      string `json:"name"`
		MediaType string `json:"media_type"`
	} `json:"assets"`
}

// latestSnapshotDir 找该工作区最新的快照目录；没有返回空串。
func latestSnapshotDir(root, workspace string) string {
	base := filepath.Join(root, "museav-snapshots")
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	type found struct{ dir, at string }
	var hits []found
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(base, ent.Name())
		data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			continue
		}
		var m snapshotManifest
		if json.Unmarshal(data, &m) != nil || m.WorkspaceID != workspace {
			continue
		}
		hits = append(hits, found{dir, m.CapturedAt})
	}
	if len(hits) == 0 {
		return ""
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].at > hits[j].at })
	return hits[0].dir
}

// snapshotSubjects 从快照读出全部主体。
func snapshotSubjects(root, workspace string) ([]*Subject, error) {
	dir := latestSnapshotDir(root, workspace)
	if dir == "" {
		return nil, errors.New("本机没有这个中台项目的快照")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "subjects"))
	if err != nil {
		return nil, errors.New("快照目录不完整")
	}
	out := []*Subject{}
	for _, ent := range entries {
		file, err := readSnapshotSubject(dir, ent.Name())
		if err != nil {
			continue
		}
		s := file.Subject
		out = append(out, &s)
	}
	if len(out) == 0 {
		return nil, errors.New("快照里没有主体")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out, nil
}

func readSnapshotSubject(dir, subjectID string) (*snapshotSubjectFile, error) {
	if subjectID == "" || strings.ContainsAny(subjectID, `/\.`) {
		return nil, errors.New("主体 id 不合法")
	}
	data, err := os.ReadFile(filepath.Join(dir, "subjects", subjectID, "subject.json"))
	if err != nil {
		return nil, err
	}
	var f snapshotSubjectFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// snapshotSceneImage 快照里场景的正式参考图（本地文件路径）。
//
// 选图规则与联网时一致：优先 standard 角色的图片；名字带「真人」的不算。
func snapshotSceneImage(root, workspace, sceneID string) (string, error) {
	dir := latestSnapshotDir(root, workspace)
	if dir == "" {
		return "", errors.New("本机没有这个中台项目的快照")
	}
	f, err := readSnapshotSubject(dir, sceneID)
	if err != nil {
		return "", errors.New("快照里没有这个场景")
	}
	if f.Subject.Kind != "scene" {
		return "", errors.New("不是场景主体")
	}
	pick := ""
	for _, a := range f.Assets {
		if a.MediaType != "" && a.MediaType != "image" {
			continue
		}
		if a.Role == "standard" {
			pick = a.ID
			break
		}
		if pick == "" {
			pick = a.ID
		}
	}
	if pick == "" {
		return "", errors.New("快照里这个场景没有参考图")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "subjects", sceneID, pick+".*"))
	for _, m := range matches {
		if !strings.HasSuffix(m, ".json") {
			return m, nil
		}
	}
	return "", errors.New("快照里的参考图文件缺失")
}

// writeSubjectsCache 联网成功后落一份主体列表。
func writeSubjectsCache(root, workspace string, subjects []*Subject) {
	if root == "" || workspace == "" || strings.ContainsAny(workspace, `/\.`) {
		return
	}
	dir := filepath.Join(root, subjectsCacheDir)
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	_ = writeJSONFile(dir, workspace+".json", subjects)
}

// readSubjectsCache 读上次联网时落的主体列表。
func readSubjectsCache(root, workspace string) ([]*Subject, error) {
	if strings.ContainsAny(workspace, `/\.`) {
		return nil, errors.New("工作区 id 不合法")
	}
	data, err := os.ReadFile(filepath.Join(root, subjectsCacheDir, workspace+".json"))
	if err != nil {
		return nil, err
	}
	var out []*Subject
	if err := json.Unmarshal(data, &out); err != nil || len(out) == 0 {
		return nil, errors.New("主体缓存为空")
	}
	return out, nil
}

// offlineSubjects 断网时的主体列表：先缓存，再快照。
func (e *Engine) offlineSubjects(workspace string) ([]*Subject, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	if list, err := readSubjectsCache(e.store.Root(), workspace); err == nil {
		return list, nil
	}
	return snapshotSubjects(e.store.Root(), workspace)
}

// sceneDescriptionFromSnapshot 从本机快照读场景的画面描述。
//
// 场景描述在出镜提示词里是硬要求；断网时从快照取，行为与联网一致。
func (e *Engine) sceneDescriptionFromSnapshot(workspace, sceneID string) (string, error) {
	if e.store == nil {
		return "", errNoStore
	}
	dir := latestSnapshotDir(e.store.Root(), workspace)
	if dir == "" {
		return "", errors.New("本机没有这个中台项目的快照")
	}
	f, err := readSnapshotSubject(dir, sceneID)
	if err != nil {
		return "", errors.New("快照里没有这个场景")
	}
	if f.Subject.Kind != "scene" {
		return "", errors.New("选的不是场景")
	}
	desc := traitText(f.Subject.Traits["画面描述"])
	if desc == "" {
		return "", errors.New("快照里这个场景没有画面描述")
	}
	return desc, nil
}
