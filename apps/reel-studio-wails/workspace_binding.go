package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// 本地项目 ↔ 中台工作区绑定。
//
// 一部短剧 = 一个本地项目 = 一个中台工作区。角色、场景存在中台工作区里，
// 剧集、镜头存在本地项目里；两边靠 Project.WorkspaceID 连起来。
//
// 没绑定就不读不写：空工作区会让 museav 列出账号下全部主体，
// 沸腾之雪的角色会混进别的片子，场景也会建到「默认项目」里。

// WorkspaceOption 一个可绑定的中台工作区。
type WorkspaceOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var errProjectUnbound = errors.New("这个项目还没绑定中台项目：先点「绑定中台项目」")

// ListWorkspaces 列当前账号可见的中台工作区（只读，不计费）。
func (e *Engine) ListWorkspaces() ([]WorkspaceOption, error) {
	token, base, err := museavCredentials()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/workspaces", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取中台工作区失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取中台工作区失败：HTTP %d", resp.StatusCode)
	}
	var rows []WorkspaceOption
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// BindProjectWorkspace 把本地项目绑到一个中台工作区。只接受当前账号可见的工作区。
func (e *Engine) BindProjectWorkspace(projectID, workspaceID string) (*Project, error) {
	if e.store == nil {
		return nil, errNoStore
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("请选择中台项目")
	}
	rows, err := e.ListWorkspaces()
	if err != nil {
		return nil, err
	}
	name, ok := workspaceName(rows, workspaceID)
	if !ok {
		return nil, errors.New("中台项目不存在或当前账号无权访问")
	}
	// 换绑：旧工作区留下的角色图要清掉，否则角色库会显示旧项目的角色。
	p, err := e.store.Project(projectID)
	if err != nil {
		return nil, err
	}
	if old := strings.TrimSpace(p.WorkspaceID); old != "" && old != workspaceID {
		if _, err := e.store.RemoveAssetsByMuseavWorkspace(projectID, old); err != nil {
			return nil, err
		}
		e.dropMirrorsForWorkspace(old)
	}
	return e.store.SetProjectWorkspace(projectID, workspaceID, name)
}

func workspaceName(rows []WorkspaceOption, id string) (string, bool) {
	for _, w := range rows {
		if w.ID == id {
			return w.Name, true
		}
	}
	return "", false
}

// SetProjectWorkspace 写入项目的工作区绑定并落盘。
func (s *StudioStore) SetProjectWorkspace(projectID, workspaceID, workspaceName string) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.state.Projects {
		if p.ID != projectID {
			continue
		}
		before, beforeName := p.WorkspaceID, p.WorkspaceName
		p.WorkspaceID, p.WorkspaceName = workspaceID, workspaceName
		if err := s.saveLocked(); err != nil {
			p.WorkspaceID, p.WorkspaceName = before, beforeName
			return nil, err
		}
		cp := *p
		return &cp, nil
	}
	return nil, errors.New("项目不存在")
}

// projectWorkspace 取项目绑定的工作区；没绑定报错，绝不退回空工作区。
func (e *Engine) projectWorkspace(projectID string) (string, error) {
	if e.store == nil {
		return "", errNoStore
	}
	p, err := e.store.Project(projectID)
	if err != nil {
		return "", err
	}
	ws := strings.TrimSpace(p.WorkspaceID)
	if ws == "" {
		return "", errProjectUnbound
	}
	return ws, nil
}

// ListProjectSubjects 列项目绑定工作区里的角色与场景。
func (e *Engine) ListProjectSubjects(projectID string) ([]*Subject, error) {
	ws, err := e.projectWorkspace(projectID)
	if err != nil {
		return nil, err
	}
	return e.ListAllSubjects(ws)
}

// CreateProjectScene 在项目绑定的工作区里建一个场景主体。
func (e *Engine) CreateProjectScene(projectID, name, description string) (*Subject, error) {
	ws, err := e.projectWorkspace(projectID)
	if err != nil {
		return nil, err
	}
	return e.CreateScene(ws, name, description)
}
