import { readFileSync, unlinkSync } from 'node:fs'
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

// 「照这个方法再做一条」：新开一个项目，复制同一版本的制作方法与素材，
// 内容带入上一条作为起点（可改写）。原视频的记录与发布数据不受影响。
//
// 原子性：中途失败（磁盘满、素材读不了）不能留下半成品项目 ——
// 新项目与它的素材先在临时目录准备好，再一次性落库；失败时把临时文件清掉。
export function remakeProject(store, job) {
  const state = store.state
  const source = state.projects.find(p => p.id === job.projectId)
  if (!source) fail('原项目不存在')
  const sopId = job.sopSnapshot?.sopVersionId || source.sopVersionId
  const versions = state.sopVersions.filter(v => v.pipeline === 'github-recommend')
  const sop = versions.find(v => v.id === sopId) || versions[0]
  if (!sop) fail('没有可用的制作方法版本')
  const project = { id: id(), name: `${source.name} · 再做一条`, pipelineKey: 'github-recommend', sopVersionId: sop.id, createdAt: now(), updatedAt: now() }

  const used = [...new Set([...(job.input.assetIds || []), ...(job.input.voiceIds || []).filter(Boolean), job.input.bgmId, job.input.narrationId].filter(Boolean))]
  // 先把字节读出来，失败就不要动 state
  const payloads = []
  for (const assetId of used) {
    const asset = state.assets.find(a => a.id === assetId)
    if (!asset) continue
    payloads.push({ source: asset, bytes: readFileSync(store.path(asset)) })
  }

  state.projects.push(project)
  const mapping = {}
  try {
    for (const { source: asset, bytes } of payloads) {
      // 授权、来源、模型归属要跟着素材一起搬走 —— 新项目里那份曲子凭什么能商用，得说得清
      const copied = store.addAsset(project.id, asset.name, asset.mime, bytes, { source: asset.source, origin: asset.origin, engine: asset.engine, prompt: asset.prompt, persona: asset.persona, tone: asset.tone, license: asset.license, licenseUrl: asset.licenseUrl, bgmSource: asset.bgmSource, sourceUrl: asset.sourceUrl, requestedModel: asset.requestedModel, actualModel: asset.actualModel, upstreamTaskId: asset.upstreamTaskId, voiceBackend: asset.voiceBackend, profile: asset.profile })
      copied.width = asset.width; copied.height = asset.height; copied.duration = asset.duration; copied.hasAudio = asset.hasAudio
      mapping[asset.id] = copied.id
    }
  } catch (error) {
    // 回滚：撤掉已写入的素材与文件，删掉新项目，不留半成品
    for (const asset of state.assets.filter(a => a.projectId === project.id)) {
      try { unlinkSync(store.path(asset)) } catch { /* 文件已不在就算了 */ }
    }
    state.assets = state.assets.filter(a => a.projectId !== project.id)
    state.projects = state.projects.filter(p => p.id !== project.id)
    store.save()
    throw error
  }
  const settings = Object.fromEntries(['platform', 'voiceSource', 'persona', 'tone', 'speed', 'pause', 'bgmGain', 'imageFit', 'backend', 'model', 'voiceId', 'profile'].filter(k => job.input[k] !== undefined).map(k => [k, job.input[k]]))
  // BGM 与整段旁白要指向**新项目里的副本**，否则新项目引用的是别的项目的素材
  if (job.input.bgmId && mapping[job.input.bgmId]) settings.bgmId = mapping[job.input.bgmId]
  if (job.input.narrationId && mapping[job.input.narrationId]) settings.narrationId = mapping[job.input.narrationId]
  project.recommendationSettings = settings
  store.save()
  return {
    projectId: project.id,
    projectName: project.name,
    sopVersionId: sop.id,
    sopVersion: sop.version,
    settings,
    draft: { url: job.input.repoUrl || '', title: job.input.title || '', captions: job.input.captions || '' },
    assets: mapping,
    imageIds: (job.input.assetIds || []).map(id => mapping[id] || ''),
    voiceIds: (job.input.voiceIds || []).map(id => (id && mapping[id]) || ''),
  }
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
    const settings = Object.fromEntries(['platform', 'voiceSource', 'persona', 'tone', 'speed', 'pause', 'bgmGain', 'imageFit', 'backend', 'model', 'voiceId', 'profile'].filter(k => job.input[k] !== undefined).map(k => [k, job.input[k]]))
    const version = { id: id(), pipeline: 'github-recommend', version: Math.max(...versions.map(v => v.version)) + 1, name: 'GitHub 项目推荐', parentId: job.sopSnapshot?.sopVersionId || null, settings, changeNote: changes, hypothesis, createdAt: now() }
    const record = { id: id(), publishId: publication.id, jobId: job.id, findings, hypothesis, changes, newVersionId: version.id, createdAt: now() }
    state.sopVersions.push(version); state.reviews.push(record); store.save(); return { review: record, version }
  }
  fail('不支持的操作')
}
