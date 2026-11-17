package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// 本地轻数据库：muse-tv 的本地资产与任务账本。
//
// 分层是刻意的，别把两类数据放混：
//   真源在中台（museav）：角色、创作哲学、生成任务 —— 平台账本，改它等于改业务事实。
//   真源在本地（这里）  ：项目、素材、成片、渲染任务 —— 我机器上的片子，
//                        平台不知道也不该知道，丢了就是真丢了。
//
// 为什么用 SQLite 而不是 studio.json：素材去重需要按 hash 查、任务要能在
// 进程崩溃后恢复状态、UI 列表要按项目过滤 —— 这三件事用 JSON 全量读写
// 都得把整个文件搬进内存。SQLite 的 WAL 模式让渲染 worker 写、UI 读互不阻塞。
//
// 只存元数据与路径，媒体文件本体仍落盘在 media 目录：塞进库里只会让
// 备份和迁移变成灾难。

const localDBSchema = `
create table if not exists assets (
  id          text primary key,
  project_id  text not null,
  name        text not null,
  mime        text not null,
  file        text not null,
  hash        text not null,
  bytes       integer not null default 0,
  source      text not null default 'local',
  origin      text not null default '',
  created_at  text not null
);
create index if not exists assets_project_idx on assets(project_id);
create index if not exists assets_hash_idx on assets(project_id, hash);

create table if not exists jobs (
  id          text primary key,
  project_id  text not null,
  kind        text not null,
  status      text not null,
  input_json  text not null default '{}',
  output_json text not null default '{}',
  error       text not null default '',
  created_at  text not null,
  updated_at  text not null
);
create index if not exists jobs_project_idx on jobs(project_id, created_at);
create index if not exists jobs_status_idx on jobs(status);
`

// LocalDB 本地资产与任务账本。
type LocalDB struct {
	db *sql.DB
}

// AssetRow 本地库的素材行。
type AssetRow struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	MIME      string `json:"mime"`
	File      string `json:"file"`
	Hash      string `json:"hash"`
	Bytes     int64  `json:"bytes"`
	Source    string `json:"source"`
	Origin    string `json:"origin"`
	CreatedAt string `json:"createdAt"`
}

// JobRow 本地库的任务行。任务落库是为了**崩溃可恢复**：
// 渲染到一半掉电，重启后能看到「这条跑到哪一步、为什么断」，而不是凭空消失。
type JobRow struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"projectId"`
	Kind      string         `json:"kind"`
	Status    string         `json:"status"`
	Input     map[string]any `json:"input"`
	Output    map[string]any `json:"output"`
	Error     string         `json:"error"`
	CreatedAt string         `json:"createdAt"`
	UpdatedAt string         `json:"updatedAt"`
}

// OpenLocalDB 打开（或建）本地库并做 schema 迁移。
func OpenLocalDB(path string) (*LocalDB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("打开本地库失败: %w", err)
	}
	// WAL 下多连接是安全的，但写仍应串行 —— 限 4 个连接够用且不打架。
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(localDBSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化本地库失败: %w", err)
	}
	return &LocalDB{db: db}, nil
}

// Close 关库。
func (l *LocalDB) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

// HashBytes 算素材内容指纹（sha256）。
//
// 去重按内容不按文件名：同一张图从不同路径导入是同一份素材，
// 文件名可能撞车，内容不会。
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FindDuplicate 查项目里同内容的素材。
func (l *LocalDB) FindDuplicate(projectID, hash string) (*AssetRow, error) {
	row := l.db.QueryRow(
		`select id, project_id, name, mime, file, hash, bytes, source, origin, created_at
		 from assets where project_id = ? and hash = ? limit 1`, projectID, hash)
	return scanAsset(row)
}

// PutAsset 登记素材；同项目同内容直接返回已有的那条（去重）。
//
// 返回 (行, 是否新建)。调用方拿到「已存在」时不该再写一份媒体文件。
func (l *LocalDB) PutAsset(a AssetRow) (*AssetRow, bool, error) {
	if a.Hash == "" {
		return nil, false, errors.New("素材缺 hash，无法去重")
	}
	if dup, err := l.FindDuplicate(a.ProjectID, a.Hash); err != nil {
		return nil, false, err
	} else if dup != nil {
		return dup, false, nil
	}
	if a.CreatedAt == "" {
		a.CreatedAt = nowISO()
	}
	_, err := l.db.Exec(
		`insert into assets (id, project_id, name, mime, file, hash, bytes, source, origin, created_at)
		 values (?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.ProjectID, a.Name, a.MIME, a.File, a.Hash, a.Bytes, a.Source, a.Origin, a.CreatedAt)
	if err != nil {
		return nil, false, err
	}
	return &a, true, nil
}

// ListAssets 列项目的素材（新的在前）。
func (l *LocalDB) ListAssets(projectID string) ([]AssetRow, error) {
	rows, err := l.db.Query(
		`select id, project_id, name, mime, file, hash, bytes, source, origin, created_at
		 from assets where project_id = ? order by created_at desc`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AssetRow{}
	for rows.Next() {
		var a AssetRow
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.MIME, &a.File, &a.Hash, &a.Bytes, &a.Source, &a.Origin, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteAsset 删一条素材登记（媒体文件由调用方决定删不删）。
func (l *LocalDB) DeleteAsset(id string) error {
	_, err := l.db.Exec(`delete from assets where id = ?`, id)
	return err
}

// PutJob 登记/更新任务。
func (l *LocalDB) PutJob(j JobRow) error {
	if j.Status == "" {
		j.Status = "queued"
	}
	now := nowISO()
	if j.CreatedAt == "" {
		j.CreatedAt = now
	}
	j.UpdatedAt = now
	inJSON, err := json.Marshal(j.Input)
	if err != nil {
		return err
	}
	outJSON, err := json.Marshal(j.Output)
	if err != nil {
		return err
	}
	_, err = l.db.Exec(
		`insert into jobs (id, project_id, kind, status, input_json, output_json, error, created_at, updated_at)
		 values (?,?,?,?,?,?,?,?,?)
		 on conflict(id) do update set
		   status = excluded.status, output_json = excluded.output_json,
		   error = excluded.error, updated_at = excluded.updated_at`,
		j.ID, j.ProjectID, j.Kind, j.Status, string(inJSON), string(outJSON), j.Error, j.CreatedAt, j.UpdatedAt)
	return err
}

// UpdateJobStatus 只改状态与错误（高频路径，不重写 input）。
func (l *LocalDB) UpdateJobStatus(id, status, jobErr string) error {
	_, err := l.db.Exec(
		`update jobs set status = ?, error = ?, updated_at = ? where id = ?`,
		status, jobErr, nowISO(), id)
	return err
}

// GetJob 读一条任务。
func (l *LocalDB) GetJob(id string) (*JobRow, error) {
	row := l.db.QueryRow(
		`select id, project_id, kind, status, input_json, output_json, error, created_at, updated_at
		 from jobs where id = ?`, id)
	return scanJob(row)
}

// ListJobs 列项目的任务。
func (l *LocalDB) ListJobs(projectID string) ([]JobRow, error) {
	rows, err := l.db.Query(
		`select id, project_id, kind, status, input_json, output_json, error, created_at, updated_at
		 from jobs where project_id = ? order by created_at desc`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []JobRow{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// InterruptedJobs 找出「进程重启前还在跑」的任务。
//
// 崩溃恢复的关键查询：启动时把 running/queued 的旧任务标成 interrupted，
// 用户能看到「上次渲染到一半断了」，而不是看到一条永远转圈的任务。
func (l *LocalDB) InterruptedJobs() ([]JobRow, error) {
	rows, err := l.db.Query(
		`select id, project_id, kind, status, input_json, output_json, error, created_at, updated_at
		 from jobs where status in ('running','queued') order by created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []JobRow{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAsset(row rowScanner) (*AssetRow, error) {
	var a AssetRow
	err := row.Scan(&a.ID, &a.ProjectID, &a.Name, &a.MIME, &a.File, &a.Hash, &a.Bytes, &a.Source, &a.Origin, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanJob(row rowScanner) (*JobRow, error) {
	var j JobRow
	var inJSON, outJSON string
	err := row.Scan(&j.ID, &j.ProjectID, &j.Kind, &j.Status, &inJSON, &outJSON, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j.Input = map[string]any{}
	j.Output = map[string]any{}
	_ = json.Unmarshal([]byte(inJSON), &j.Input)
	_ = json.Unmarshal([]byte(outJSON), &j.Output)
	return &j, nil
}

// truncateRunes 截断字符串（按 rune，不切坏中文）。
func truncateRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

var _ = time.Now
