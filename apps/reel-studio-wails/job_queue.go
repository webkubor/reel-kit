package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
)

// JobQueue 是本机任务队列（从 server.mjs 的 pump 搬来）。
//
// 为什么不放在前端起 goroutine、也不放在 HTTP 里：桌面壳是零 HTTP 的，
// 耗时操作（TTS / 渲染 / ffmpeg）必须由 Go 侧排队执行，前端只轮询状态。
// 这跟 Node 版的模型一致，但去掉了 HTTP 那一层。
//
// 取消要真的掐断渲染、ffmpeg、语音，而不是把结果扔掉 ——
// 被取消的任务如果继续跑，上游云服务照样计费。所以每个在跑的任务配一个
// context.CancelFunc，取消时调用它，让下游的 exec.CommandContext 与
// HTTP 请求一起中止。
type JobQueue struct {
	mu      *sync.Mutex
	store   *StudioStore
	running bool

	// controllers：正在跑的任务 → 取消函数。
	controllers map[string]context.CancelFunc

	// wake：唤醒信号。worker 空转时在 select 上等它，不忙轮询。
	wake chan struct{}

	// handler：真正执行任务的函数，由上层注入（渲染 / TTS / 合成）。
	handler JobHandler
}

// JobHandler 执行一条任务。返回生成的素材 id；返回错误则任务失败。
// ctx 被取消时必须返回错误（通常是 context.Canceled）。
type JobHandler func(ctx context.Context, job *Job, progress func(text string, upstream map[string]any), report func(step Step)) (string, error)

// ErrCanceled 任务被用户主动取消。
var ErrCanceled = errors.New("已取消")

// NewJobQueue 造一个任务队列。
func NewJobQueue(store *StudioStore, handler JobHandler) *JobQueue {
	return &JobQueue{
		mu:          &store.mu,
		store:       store,
		controllers: map[string]context.CancelFunc{},
		wake:        make(chan struct{}, 1),
		handler:     handler,
	}
}

// SetHandler 换掉执行任务的函数（出片管线接上后调用）。
func (q *JobQueue) SetHandler(handler JobHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handler = handler
}

// Submit 入队一条任务，立刻返回；实际执行在后台 worker 里。
func (q *JobQueue) Submit(projectID string, input map[string]any) (*Job, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var ownedInput map[string]any
	if err := json.Unmarshal(data, &ownedInput); err != nil {
		return nil, err
	}
	q.mu.Lock()
	active := 0
	for _, j := range q.store.state.Jobs {
		if j.Status == "queued" || j.Status == "running" {
			active++
		}
	}
	if active >= 10 {
		q.mu.Unlock()
		return nil, errors.New("队列已满，请等待当前任务完成")
	}
	job := &Job{
		ID:        newID(),
		ProjectID: projectID,
		Input:     ownedInput,
		Status:    "queued",
		CreatedAt: nowISO(),
		UpdatedAt: nowISO(),
		Message:   "等待执行",
		Steps:     []Step{},
	}
	q.store.state.Jobs = append(q.store.state.Jobs, job)
	if err := q.store.saveLocked(); err != nil {
		q.store.state.Jobs = q.store.state.Jobs[:len(q.store.state.Jobs)-1]
		q.mu.Unlock()
		return nil, err
	}
	snapshot := cloneJob(job)
	q.mu.Unlock()

	q.poke()
	return snapshot, nil
}

// Cancel 取消一条任务。
//
// queued：直接改状态，立刻结束。
// running：调用它的 CancelFunc，让渲染/ffmpeg/语音各自停下来。
func (q *JobQueue) Cancel(jobID string) (string, error) {
	q.mu.Lock()
	var job *Job
	for _, j := range q.store.state.Jobs {
		if j.ID == jobID {
			job = j
			break
		}
	}
	if job == nil {
		q.mu.Unlock()
		return "", errors.New("找不到这个任务")
	}
	switch job.Status {
	case "succeeded", "failed", "canceled", "interrupted":
		q.mu.Unlock()
		return "", errors.New("任务已经结束了，取消不了")
	case "queued":
		job.Status = "canceled"
		job.UpdatedAt = nowISO()
		q.store.saveLocked()
		q.mu.Unlock()
		return "canceled", nil
	}
	cancel := q.controllers[jobID]
	q.mu.Unlock()

	if cancel != nil {
		cancel()
		return "canceling", nil
	}
	return "canceling", nil
}

// Jobs 返回当前所有任务（新→旧）。
func (q *JobQueue) Jobs() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	all := q.store.state.Jobs
	out := make([]*Job, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		out = append(out, cloneJob(all[i]))
	}
	return out
}

// Job 按 id 找一条任务。
func (q *JobQueue) Job(jobID string) *Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.store.state.Jobs {
		if j.ID == jobID {
			return cloneJob(j)
		}
	}
	return nil
}

// poke 唤醒 worker。
func (q *JobQueue) poke() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Run 启动 worker。它阻塞直到 ctx 取消，是引擎的后台协程。
func (q *JobQueue) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
			q.pump(ctx)
		}
	}
}

// pump 取一条 queued 的任务执行，直到队列空。
func (q *JobQueue) pump(parent context.Context) {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.running = false
		q.mu.Unlock()
	}()

	for {
		job := q.nextQueued()
		if job == nil {
			return
		}
		q.run(parent, job)
	}
}

// nextQueued 取下一条排队的任务并置为 running。
func (q *JobQueue) nextQueued() *Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.store.state.Jobs {
		if j.Status == "queued" {
			j.Status = "running"
			j.Steps = []Step{}
			j.Message = "正在准备任务…"
			j.UpdatedAt = nowISO()
			q.store.saveLocked()
			return j
		}
	}
	return nil
}

// run 执行一条任务的全过程：建 context、登记取消函数、更新状态。
func (q *JobQueue) run(parent context.Context, job *Job) {
	ctx, cancel := context.WithCancel(parent)

	q.mu.Lock()
	q.controllers[job.ID] = cancel
	handler := q.handler
	input := cloneJob(job)
	q.mu.Unlock()

	progress := func(text string, upstream map[string]any) {
		q.mu.Lock()
		defer q.mu.Unlock()
		job.Message = text
		if len(upstream) > 0 {
			// 提交即计费的通道必须在提交那一刻就把上游任务 ID 存下来 ——
			// 否则下载失败或进程重启后，用户只能重新提交一次，等于重复花钱。
			if job.Upstream == nil {
				job.Upstream = map[string]any{}
			}
			for k, v := range upstream {
				job.Upstream[k] = v
			}
		}
		job.UpdatedAt = nowISO()
		q.store.saveLocked()
	}
	report := func(step Step) {
		q.mu.Lock()
		defer q.mu.Unlock()
		steps := make([]Step, 0, len(job.Steps)+1)
		for _, s := range job.Steps {
			if s.ID != step.ID {
				steps = append(steps, s)
			}
		}
		steps = append(steps, step)
		job.Steps = steps
		job.UpdatedAt = nowISO()
		q.store.saveLocked()
	}

	defer func() {
		q.mu.Lock()
		delete(q.controllers, job.ID)
		q.mu.Unlock()
		cancel()
	}()

	if handler == nil {
		q.finish(job, "", errors.New("本地引擎还没有接上出片管线"))
		return
	}

	assetID, err := handler(ctx, input, progress, report)
	if err != nil {
		// 用户主动停的，标「已取消」而不是「失败」：这两件事该分开，用户才知道要不要重来。
		if errors.Is(err, context.Canceled) || errors.Is(err, ErrCanceled) {
			q.finish(job, "", ErrCanceled)
		} else {
			q.finish(job, "", err)
		}
		return
	}
	q.finish(job, assetID, nil)
}

// finish 收尾一条任务的状态。
func (q *JobQueue) finish(job *Job, assetID string, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	switch {
	case err == nil:
		job.AssetID = assetID
		job.Status = "succeeded"
		job.Message = "已完成，结果保存在本地项目中"
	case errors.Is(err, ErrCanceled):
		job.Status = "canceled"
		job.Error = ""
		job.Message = "已取消"
	default:
		job.Status = "failed"
		job.Error = Redact(err.Error())
		job.Message = "执行失败；输入已保留，可查看步骤定位问题"
	}
	job.UpdatedAt = nowISO()
	q.store.saveLocked()
}

// Redact 抹掉错误信息里的密钥、服务地址与本机绝对路径（对齐 Node 版 redact）。
func Redact(msg string) string {
	if msg == "" {
		return ""
	}
	out := bearerRe.ReplaceAllString(msg, "Bearer [redacted]")
	out = secretRe.ReplaceAllString(out, "[redacted]")
	out = urlRe.ReplaceAllString(out, "[服务地址]")
	// 本地绝对路径贴到界面上既没用也泄露目录结构。
	// 必须以 / 开头才算路径，否则「480p/720p/1080p」这种分辨率会被误当成路径吃掉。
	//
	// Go 的 RE2 不支持 JS 的后行断言 (?<=\s)，改用捕获组还原：
	// 分隔符本身要留在结果里，所以用 ${1} 把它带回来。
	out = absPathRe.ReplaceAllString(out, "${1}[本地路径]")
	return out
}

var (
	bearerRe  = regexp.MustCompile(`(?i)Bearer\s+\S+`)
	secretRe  = regexp.MustCompile(`\b(?:sk-|eyJ)[\w.-]{15,}`)
	urlRe     = regexp.MustCompile(`https?://\S+`)
	absPathRe = regexp.MustCompile(`(^|\s)/(?:[\w.@+-]+/)+[\w.@+-]*`)
)

// DescribeJob 给界面用的一句话状态说明。
func DescribeJob(job *Job) string {
	if job == nil {
		return ""
	}
	switch job.Status {
	case "queued":
		return "排队中"
	case "running":
		if job.Message != "" {
			return job.Message
		}
		return "执行中"
	case "succeeded":
		return "已完成"
	case "failed":
		return fmt.Sprintf("失败：%s", job.Error)
	case "canceled":
		return "已取消"
	case "interrupted":
		return "已中断（工作台重启过）"
	}
	return job.Status
}
