import { id, now } from './store.mjs'

const fail = message => { throw Object.assign(new Error(message), { status: 400 }) }
const text = (value, max = 1000) => String(value ?? '').trim().slice(0, max)
const copy = value => JSON.parse(JSON.stringify(value))
const reviewKeys = ['opening', 'truth', 'matching', 'ending', 'subtitles', 'audio', 'visuals']

export function initializeWorkflow(state) {
  state.sopVersions ||= []
  state.publishes ||= []
  state.performance ||= []
  state.reviews ||= []
  if (!state.sopVersions.some(v => v.pipeline === 'github-recommend')) {
    state.sopVersions.push({ id: id(), pipeline: 'github-recommend', version: 1, name: 'GitHub 项目推荐', settings: {}, createdAt: now() })
  }
}

export function snapshotSop(state, project, input) {
  const versions = state.sopVersions.filter(v => v.pipeline === 'github-recommend')
  const sop = versions.find(v => v.id === project.sopVersionId) || versions[0]
  return { sopVersionId: sop.id, sopVersion: sop.version, input: copy(input) }
}

export function workflowAction(store, action, input) {
  const state = store.state
  const project = store.project(input.projectId)
  const job = state.jobs.find(j => j.id === input.jobId && j.projectId === project.id && j.input.kind === 'recommendation')
  if (!job) fail('请选择当前项目的一次推荐视频制作记录')
  if (action === 'review-check') {
    if (job.status !== 'succeeded' || !job.assetId) fail('请先生成成片')
    const checks = input.checks || {}
    if (!reviewKeys.every(k => checks[k] === true)) fail('请逐项确认所有人工检查项')
    if (!job.quality?.passed || job.quality.assetId !== job.assetId) fail('请先完成并通过文件检测')
    job.approval = { checks: Object.fromEntries(reviewKeys.map(k => [k, true])), assetId: job.assetId, reviewedAt: now() }
    store.save(); return job.approval
  }
  if (action === 'publish') {
    if (!job.approval || job.approval.assetId !== job.assetId || !job.quality?.passed || job.quality.assetId !== job.assetId) fail('请先通过文件检测并确认人工检查')
    const platform = text(input.platform, 50), url = text(input.url, 2000), publishedAt = text(input.publishedAt, 40), goal = text(input.goal, 200)
    if (!platform || !url || !publishedAt || !goal) fail('请填写平台、发布链接、日期和运营目标')
    try { const u = new URL(url); if (!['https:', 'http:'].includes(u.protocol)) throw Error() } catch { fail('发布链接须为完整的 HTTP 或 HTTPS 地址') }
    const record = { id: id(), projectId: project.id, jobId: job.id, assetId: job.assetId, sopVersionId: job.sopSnapshot?.sopVersionId || null, platform, url, publishedAt, goal, tags: text(input.tags, 300), createdAt: now() }
    state.publishes.push(record); store.save(); return record
  }
  const publication = state.publishes.find(p => p.id === input.publishId && p.jobId === job.id && p.projectId === project.id)
  if (!publication) fail('请选择这次制作的发布记录')
  if (action === 'performance') {
    const metrics = {}
    for (const key of ['plays', 'likes', 'comments', 'shares', 'visits']) {
      if (input[key] === '' || input[key] == null) continue
      const number = Number(input[key]); if (!Number.isInteger(number) || number < 0) fail('指标须为非负整数')
      metrics[key] = number
    }
    if (!Object.keys(metrics).length || !text(input.window, 100)) fail('请至少填写一个指标和观察窗口')
    const record = { id: id(), publishId: publication.id, metrics, window: text(input.window, 100), source: 'manual', recordedAt: now() }
    state.performance.push(record); store.save(); return record
  }
  if (action === 'review') {
    const findings = text(input.findings, 2000), hypothesis = text(input.hypothesis, 2000), changes = text(input.changes, 2000)
    if (!findings || !hypothesis || !changes) fail('请填写问题、假设和具体改动')
    const versions = state.sopVersions.filter(v => v.pipeline === 'github-recommend')
    const settings = Object.fromEntries(['platform', 'voiceSource', 'persona', 'tone', 'speed', 'pause', 'bgmGain', 'imageFit'].filter(k => job.input[k] !== undefined).map(k => [k, job.input[k]]))
    const version = { id: id(), pipeline: 'github-recommend', version: Math.max(...versions.map(v => v.version)) + 1, name: 'GitHub 项目推荐', parentId: job.sopSnapshot?.sopVersionId || null, settings, changeNote: changes, hypothesis, createdAt: now() }
    const record = { id: id(), publishId: publication.id, jobId: job.id, findings, hypothesis, changes, newVersionId: version.id, createdAt: now() }
    state.sopVersions.push(version); state.reviews.push(record); store.save(); return { review: record, version }
  }
  fail('不支持的操作')
}
