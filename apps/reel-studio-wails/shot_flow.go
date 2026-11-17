package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// 镜头工作流：镜头 → 角色 / 装扮 / 场景 → 出镜 → 回填素材 → 成片。
//
// 镜头是这条线的唯一真源：出镜和成片都从镜头读参数、把结果写回镜头，
// 这样一集里哪些镜头拍完了、哪些还空着，打开工作台就看得到。

// ShotCast 改镜头的内容与选角（出镜前可随时改）。
type ShotCast struct {
	ID          string
	Title       string
	Action      string
	Lines       []string
	CharacterID string
	Look        string
	SceneID     string
}

// SetShotCast 更新镜头的内容与选角。出镜中的镜头不让改，免得结果对不上参数。
func (e *Engine) SetShotCast(in ShotCast) (*EpisodeShot, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	var busy bool
	shot, err := e.store.UpdateShot(in.ID, func(s *EpisodeShot) {
		if s.Status == "shooting" {
			busy = true
			return
		}
		if t := strings.TrimSpace(in.Title); t != "" {
			s.Title = t
		}
		s.Action = strings.TrimSpace(in.Action)
		s.Lines = cleanLines(in.Lines)
		// 换了角色/装扮/场景，旧素材就不是这个镜头了
		changed := s.CharacterID != strings.TrimSpace(in.CharacterID) ||
			s.Look != strings.TrimSpace(in.Look) || s.SceneID != strings.TrimSpace(in.SceneID)
		s.CharacterID = strings.TrimSpace(in.CharacterID)
		s.Look = strings.TrimSpace(in.Look)
		s.SceneID = strings.TrimSpace(in.SceneID)
		if changed && s.ClipAssetID != "" {
			s.ClipAssetID, s.AssetID, s.Status = "", "", "planned"
		}
	})
	if busy {
		return nil, errors.New("镜头正在出镜，等它结束再改")
	}
	return shot, err
}

func cleanLines(in []string) []string {
	out := []string{}
	for _, l := range in {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// claimShot 原子地把镜头标成「出镜中」：同一镜头不能被点两次、付两次钱。
func (s *StudioStore) claimShot(id string) (*EpisodeShot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sh := range s.state.Shots {
		if sh.ID != id {
			continue
		}
		if sh.Status == "shooting" || sh.Status == "interrupted" {
			return nil, errors.New("镜头正在出镜或上次任务状态待核对，请先核对中台任务")
		}
		before := sh.Status
		sh.Status = "shooting"
		sh.UpdatedAt = nowISO()
		if err := s.saveLocked(); err != nil {
			sh.Status = before
			return nil, err
		}
		cp := *sh
		cp.Lines = append([]string(nil), sh.Lines...)
		return &cp, nil
	}
	return nil, errors.New("镜头不存在")
}

// ShootShot 按镜头的选角出镜（付费：中台 H3），结果回填到镜头。
func (e *Engine) ShootShot(ctx context.Context, shotID string, progress ProgressFunc, report func(Step)) (*EpisodeShot, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	shot, err := e.store.claimShot(shotID)
	if err != nil {
		return nil, err
	}
	// 出镜参数只能从已锁定的快照读取；提交前的失败可安全恢复原状态。
	restore := func() {
		_, _ = e.store.UpdateShot(shotID, func(s *EpisodeShot) {
			if s.ClipAssetID != "" {
				s.Status = "shot"
			} else {
				s.Status = "planned"
			}
		})
	}
	if shot.CharacterID == "" {
		restore()
		return nil, errors.New("镜头还没选角色")
	}
	if strings.TrimSpace(shot.Action) == "" {
		restore()
		return nil, errors.New("镜头还没写动作")
	}
	ws, err := e.projectWorkspace(shot.ProjectID)
	if err != nil {
		restore()
		return nil, err
	}
	scene := ""
	if shot.SceneID != "" {
		if scene, err = e.sceneDescription(ctx, ws, shot.SceneID); err != nil {
			restore()
			return nil, err
		}
	}
	input := CharacterVideoInput{
		ProjectID: shot.ProjectID, SubjectID: shot.CharacterID, Action: shot.Action,
		Look: shot.Look, Scene: scene, ExpectWorkspace: ws,
	}
	var clip *Asset
	if e.shootOverride != nil {
		clip, err = e.shootOverride(ctx, input)
	} else {
		clip, err = e.MakeCharacterVideo(ctx, input, progress, report)
	}
	if err != nil {
		// 上游可能已接收请求或已生成，不能自动恢复成可重复提交的状态。
		_, _ = e.store.UpdateShot(shotID, func(s *EpisodeShot) { s.Status = "interrupted" })
		return nil, fmt.Errorf("出镜未完成，请先在中台核对任务后再重试：%w", err)
	}
	return e.store.UpdateShot(shotID, func(s *EpisodeShot) {
		s.ClipAssetID, s.AssetID, s.Status = clip.ID, "", "shot"
	})
}

// ResolveInterruptedShot 用户已在中台核对过中断的出镜：放回可操作状态。
// 只接受 interrupted；已有素材的回到「已出镜」，没有的回到「待拍」。
func (e *Engine) ResolveInterruptedShot(shotID string) (*EpisodeShot, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	var wrong bool
	shot, err := e.store.UpdateShot(shotID, func(s *EpisodeShot) {
		if s.Status != "interrupted" {
			wrong = true
			return
		}
		if s.ClipAssetID != "" {
			s.Status = "shot"
		} else {
			s.Status = "planned"
		}
	})
	if err != nil {
		return nil, err
	}
	if wrong {
		return nil, errors.New("这个镜头不在待核对状态")
	}
	return shot, nil
}

// sceneDescription 读场景主体的画面描述，并核对它属于项目绑定的工作区。
func (e *Engine) sceneDescription(ctx context.Context, workspace, sceneID string) (string, error) {
	if e.sceneOverride != nil {
		return e.sceneOverride(ctx, workspace, sceneID)
	}
	detail, err := (&MuseavClient{}).ShowSubject(ctx, sceneID)
	if err != nil {
		return "", err
	}
	if detail.Subject.Kind != "scene" {
		return "", errors.New("选的不是场景")
	}
	if detail.Subject.WorkspaceID != workspace {
		return "", fmt.Errorf("场景「%s」不属于当前项目绑定的中台项目", detail.Subject.Name)
	}
	desc := traitText(detail.Subject.Traits["画面描述"])
	if desc == "" {
		desc = detail.Subject.Name
	}
	return desc, nil
}

// FinishShot 把镜头的出镜素材做成成片（本机配音 + 混音，不计费），回填到镜头。
func (e *Engine) FinishShot(ctx context.Context, shotID string, keepAmbient bool, progress ProgressFunc, report func(Step)) (*EpisodeShot, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	shot, err := e.store.EpisodeShot(shotID)
	if err != nil {
		return nil, err
	}
	if shot.ClipAssetID == "" {
		return nil, errors.New("镜头还没出镜素材")
	}
	asset, err := e.MakeEpisode(ctx, EpisodeInput{
		ProjectID: shot.ProjectID, ClipAssetID: shot.ClipAssetID, Title: shot.Title,
		Lines: shot.Lines, KeepAmbient: keepAmbient,
	}, progress, report)
	if err != nil {
		return nil, err
	}
	return e.store.UpdateShot(shotID, func(s *EpisodeShot) {
		s.AssetID, s.Status = asset.ID, "done"
	})
}
