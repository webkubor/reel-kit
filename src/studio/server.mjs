import http from 'node:http'
import { createReadStream, statSync, readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'
import { homedir } from 'node:os'
import { createStore, id, now } from './store.mjs'
import { createProviders } from './providers.mjs'
import { sherpaEngine } from './sherpa.mjs'
import { exportVideo } from './export.mjs'
import { redact, command, jsonFetch, download, probeBytes } from './process.mjs'
import { projectDraft, makeRecommendation, githubRepo } from './recommendation.mjs'
import { makeTalkVideo, TALK_TEMPLATES } from './talk.mjs'
import { makeProductVideo, PRODUCT_LAYOUTS } from './product.mjs'
import { KNOWLEDGE_STYLES } from '../knowledge-styles.mjs'
import { recommendationSettings } from './video-settings.mjs'
import { listBgm, resolveBgmEntry, bgmConfigured } from '../bgm-library.mjs'
import { initializeWorkflow, snapshotSop, workflowAction, remakeProject } from './workflow.mjs'
import { limitsFor, saveLimits } from './platform-limits.mjs'
import { loadCustomChannels, customChannelIssues, upsertChannel, removeChannel, probeChannel } from './channels.mjs'

const web = fileURLToPath(new URL('../../studio/', import.meta.url))
const templates = fileURLToPath(new URL('../../templates/', import.meta.url))
const templateNames = readdirSync(templates).filter(f => f.endsWith('.html') && f !== 'project-recommendation.html').map(f => f.slice(0, -5))
const uuid = /^[a-f\d-]{36}$/
function fail(message, status = 400) { throw Object.assign(new Error(message), { status }) }

/** 自定义通道给界面看的清单：不含密钥，只说这条通道的密钥从哪来、有没有配 */
function channelList() {
  return loadCustomChannels().map(c => ({
    id: c.id, label: c.label, kind: c.kind, protocol: c.protocol,
    baseUrl: c.baseUrl, model: c.model, env: c.env,
    auth: c.auth, imagePath: c.imagePath, taskPath: c.taskPath,
    // ready 只代表「密钥到位」，不代表上游模型可用 —— 那是另一回事，得跑一次才知道
    ready: Boolean(c.key),
    keySource: c.keySource,
    note: c.key
      ? (c.keySource === 'env' ? `密钥来自环境变量 ${c.env}` : '密钥已存在设置页')
      : `还没填密钥：设置环境变量 ${c.env}，或在设置页把密钥填进来`,
  }))
}
async function body(req, max = 1000000) {
  const chunks = []; let length = 0
  for await (const chunk of req) {
    length += chunk.length
    if (length > max) fail('文件过大', 413)
    chunks.push(chunk)
  }
  return Buffer.concat(chunks)
}
function json(res, code, value) { res.writeHead(code, { 'content-type': 'application/json; charset=utf-8' }); res.end(JSON.stringify(value)) }
function validate(input, store) {
  store.project(input.projectId)
  if (!['image', 'video', 'speech', 'clone', 'voice-design', 'export', 'recommendation', 'talk', 'product'].includes(input.kind)) fail('任务类型不支持')
  const allowed = ['kind', 'projectId', 'prompt', 'ratio', 'duration', 'resolution', 'assetIds', 'audioId', 'referenceId', 'persona', 'referenceText', 'voiceName', 'template', 'captions', 'title', 'profile', 'repoUrl', 'voiceIds', 'platform', 'voiceSource', 'tone', 'emotion', 'speed', 'pause', 'bgmId', 'bgmGain', 'narrationId', 'imageFit', 'backend', 'model', 'voiceId', 'style', 'voiceBackend', 'voiceModel', 'subtitle', 'source', 'footer']
  const value = Object.fromEntries(allowed.filter(k => input[k] !== undefined).map(k => [k, input[k]]))
  if (typeof value.prompt !== 'string' || !value.prompt.trim() || value.prompt.length > 10000) {
    if (!['export', 'recommendation', 'talk', 'product'].includes(value.kind)) fail('请输入 1–10000 字的提示词或配音文字')
  }
  value.ratio = value.ratio || '9:16'
  if (!['9:16', '16:9', '1:1'].includes(value.ratio)) fail('不支持的画幅')
  value.duration = Number(value.duration || 6)
  if (!Number.isFinite(value.duration) || value.duration < 1 || value.duration > 30) fail('时长须为 1–30 秒')
  // 各通道的时长上限不一样（H3 4–15 秒、Kling 5–10 秒、Wan 固定 5 秒）：
  // 界面上按所选模型给范围，服务端只做基本约束，具体上限由上游说了算
  if (value.kind === 'video' && !Number.isInteger(value.duration)) fail('时长须为整数秒')
  if (value.resolution && !['768P', '1080P', '2K'].includes(value.resolution)) fail('不支持的分辨率')
  // 通道与模型：允许自填（中台上新一个模型时不用等 reel-kit 发版）。
  // 模型名里常有空格与括号（"Seedance 2.0"、"Chinese (Mandarin)_Warm_Girl"），不能一刀切；
  // 真正要挡的是控制字符与路径分隔符 —— 这两个字段不会拿去拼路径或进 shell。
  if (value.backend !== undefined) {
    if (typeof value.backend !== 'string' || !/^[A-Za-z0-9_.-]{1,60}$/.test(value.backend)) fail('生成通道格式无效')
  }
  for (const key of ['model', 'voiceId']) {
    if (value[key] === undefined) continue
    if (typeof value[key] !== 'string' || value[key].length > 120 || /[\u0000-\u001f/\\]/.test(value[key])) fail(`${key} 格式无效`)
  }
  value.assetIds = value.assetIds || []
  if (!Array.isArray(value.assetIds) || value.assetIds.length > 30) fail('最多选择 30 个素材')
  value.assetIds.forEach(assetId => store.asset(assetId, value.projectId))
  for (const key of ['referenceId', 'audioId', 'bgmId', 'narrationId']) if (value[key]) store.asset(value[key], value.projectId)
  for (const key of ['persona', 'referenceText', 'voiceName', 'captions', 'title', 'profile', 'tone', 'emotion', 'style', 'subtitle', 'source', 'footer', 'voiceBackend', 'voiceModel']) if (value[key] !== undefined && (typeof value[key] !== 'string' || value[key].length > 10000)) fail(`${key} 格式无效`)
  if (value.kind === 'voice-design' && (!value.voiceName?.trim() || value.voiceName.length > 80 || !value.tone?.trim() || value.tone.length > 500 || value.prompt.length > 45)) fail('新音色需要名称、声音描述和不超过 45 字的试听文字')
  if (value.kind === 'recommendation') {
    githubRepo(value.repoUrl)
    if (typeof value.title !== 'string' || !value.title.trim() || value.title.length > 100) fail('项目标题须为 1–100 字')
    const lines = typeof value.captions === 'string' ? value.captions.split('\n').map(s => s.trim()).filter(Boolean) : []
    if (!lines.length || lines.length > 8 || lines.some(s => s.length > 80)) fail('旁白须为 1–8 行，每行不超过 80 字；过长的旁白请拆分为多镜')
    const settings = recommendationSettings(value)
    value.ratio = settings.ratio
    value.voiceSource = settings.voiceSource
    if (!value.assetIds.length) fail('请至少选择一张项目画面')
    for (const id of value.assetIds) if (!store.asset(id, value.projectId).mime.startsWith('image/')) fail('项目画面必须是图片')
    for (const key of ['bgmId', 'narrationId']) if (value[key] && !store.asset(value[key], value.projectId).mime.startsWith('audio/')) fail('旁白和 BGM 须选择音频素材')
    if (settings.voiceSource === 'audio' && !value.narrationId) fail('请选择整段旁白素材')
    if (settings.voiceSource === 'tts' && !value.persona && !value.voiceId) fail('请选择音色或创建新音色')
    if (!Array.isArray(value.voiceIds) || value.voiceIds.length !== lines.length) fail('旁白素材与分镜数量不匹配')
    value.voiceIds.filter(Boolean).forEach(id => { if (!store.asset(id, value.projectId).mime.startsWith('audio/')) fail('分镜旁白须选择音频') })
    if (settings.voiceSource === 'scenes' && !value.persona && !value.voiceId && value.voiceIds.some(id => !id)) fail('空缺分镜需要选择生成音色')
  }
  if (value.kind === 'talk') {
    // 口播：脚本一行一镜，配音按实际语速决定这一镜多长 —— 用户不用自己算时间轴
    const lines = typeof value.captions === 'string' ? value.captions.split('\n').map(s => s.trim()).filter(Boolean) : []
    if (!lines.length || lines.length > 8 || lines.some(s => s.length > 80)) fail('脚本须为 1–8 行，每行不超过 80 字；过长的句子请拆成多镜')
    if (typeof value.title !== 'string' || !value.title.trim() || value.title.length > 60) fail('请填写视频标题（1–60 字）')
    if (!TALK_TEMPLATES.includes(value.template)) value.template = 'knowledge-paper'
    for (const id of value.assetIds) if (!store.asset(id, value.projectId).mime.startsWith('image/')) fail('画面素材必须是图片')
    if (value.bgmId && !store.asset(value.bgmId, value.projectId).mime.startsWith('audio/')) fail('BGM 须选择音频素材')
    if (!value.persona && !value.voiceId) fail('请选择音色')
    // 没指定通道时优先用随应用自带的离线引擎 —— 它是唯一不需要任何前置条件的路。
    value.voiceBackend = value.voiceBackend || (sherpaEngine() ? 'sherpa' : 'voxflow')
    if (!['sherpa', 'voxflow', 'minimax'].includes(value.voiceBackend)) fail('语音通道不支持')
  }
  if (value.kind === 'product') {
    // 人物商品介绍：一句一镜，版式随画幅走（竖版出镜、横版左图右文）
    const lines = typeof value.captions === 'string' ? value.captions.split('\n').map(s => s.trim()).filter(Boolean) : []
    if (!lines.length || lines.length > 8 || lines.some(s => s.length > 80)) fail('介绍词须为 1–8 行，每行不超过 80 字；过长的句子请拆成多镜')
    if (typeof value.title !== 'string' || !value.title.trim() || value.title.length > 60) fail('请填写产品或人物名称（1–60 字）')
    if (!PRODUCT_LAYOUTS[value.ratio]) value.ratio = '9:16'
    value.template = PRODUCT_LAYOUTS[value.ratio].template
    for (const id of value.assetIds) if (!store.asset(id, value.projectId).mime.startsWith('image/')) fail('产品或人物画面必须是图片')
    if (value.bgmId && !store.asset(value.bgmId, value.projectId).mime.startsWith('audio/')) fail('BGM 须选择音频素材')
    if (!value.persona && !value.voiceId) fail('请选择音色')
    // 没指定通道时优先用随应用自带的离线引擎 —— 它是唯一不需要任何前置条件的路。
    value.voiceBackend = value.voiceBackend || (sherpaEngine() ? 'sherpa' : 'voxflow')
    if (!['sherpa', 'voxflow', 'minimax'].includes(value.voiceBackend)) fail('语音通道不支持')
  }
  value.template = value.template || 'sticker-promo'
  if (!templateNames.includes(value.template)) fail('模板不存在')
  if (value.kind === 'export' && !['9:16', '16:9'].includes(value.ratio)) fail('导出支持竖屏或横屏')
  return value
}
export function createStudio({ root = process.env.REEL_STUDIO_HOME || join(homedir(), '.reel-kit', 'studio'), providers, exportFn = exportVideo } = {}) {
  const store = createStore(root)
  initializeWorkflow(store.state)
  store.save()
  const backend = providers || createProviders(store)
  let worker = false
  const pump = async () => {
    if (worker) return
    worker = true
    try {
      for (;;) {
        const job = store.state.jobs.find(j => j.status === 'queued')
        if (!job) break
        // progress 除了文案，还可以带一条上游任务回执（meta.upstream）。
        // 提交即计费的通道必须在提交那一刻就把上游任务 ID 存下来 ——
        // 否则下载失败或进程重启后，用户只能重新提交一次，等于重复花钱。
        const progress = (text, meta) => {
          job.message = text
          if (meta?.upstream) {
            job.upstream = { ...(job.upstream || {}), ...meta.upstream }
            job.updatedAt = now(); store.save()
          }
          job.updatedAt = now(); store.save()
        }
        const report = step => {
          const steps = (job.steps || []).filter(s => s.id !== step.id)
          steps.push(step)
          job.steps = steps
          job.updatedAt = now(); store.save()
        }
        job.status = 'running'; job.steps = []; progress('正在准备任务…')
        try {
          const asset = job.input.kind === 'export' ? await exportFn(job.input, store, progress) : job.input.kind === 'recommendation' ? await makeRecommendation(job.input, store, backend, progress, report) : job.input.kind === 'talk' ? await makeTalkVideo(job.input, store, backend, progress, report) : job.input.kind === 'product' ? await makeProductVideo(job.input, store, backend, progress, report) : await backend.generate(job.input, progress)
          job.assetId = asset?.id; job.status = 'succeeded'; progress('已完成，结果保存在本地项目中')
        } catch (error) { job.status = 'failed'; job.error = redact(error.message); progress('执行失败；输入已保留，可查看步骤定位问题') }
      }
    } finally { worker = false }
  }
  const server = http.createServer(async (req, res) => {
    res.setHeader('X-Content-Type-Options', 'nosniff')
    res.setHeader('Cache-Control', 'no-store')
    res.setHeader('Content-Security-Policy', "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob:; media-src 'self' blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
    try {
      const host = req.headers.host || ''
      if (!/^(localhost|127\.0\.0\.1):\d+$/.test(host)) fail('只接受本机访问', 403)
      if (req.headers.origin && req.headers.origin !== `http://${host}`) fail('跨站请求已拒绝', 403)
      const url = new URL(req.url, `http://${host}`)
      const path = url.pathname
      if (req.method === 'GET' && path === '/api/state') return json(res, 200, { ...store.state, assets: store.state.assets.map(a => store.publicAsset(a)), templates: templateNames })
      if (req.method === 'GET' && path === '/api/platform-limits') {
        const platform = url.searchParams.get('platform') || 'default'
        return json(res, 200, { platform, limits: limitsFor(store.state, platform) })
      }
      if (req.method === 'GET' && path === '/api/providers') {
        const status = await backend.status()
        status.customChannels = status.customChannels || {}
        status.customChannels.channels = channelList()
        return json(res, 200, status)
      }
      if (req.method === 'GET' && path === '/api/styles') {
        // 视觉风格库：口播视频用。只暴露口播模板能用的风格，别把别的版式混进来。
        const styles = KNOWLEDGE_STYLES.filter(s => TALK_TEMPLATES.includes(s.template))
        return json(res, 200, { styles, templates: TALK_TEMPLATES })
      }
      if (req.method === 'GET' && path === '/api/voice/sample') {
        const key = url.searchParams.get('key')
        if (!key || key.length > 200) fail('音色标识无效')
        const { bytes } = await download(`${process.env.REEL_VOXFLOW_URL || 'http://127.0.0.1:8866'}/api/persona-audio?key=${encodeURIComponent(key)}`, 20 * 1024 * 1024)
        res.writeHead(200, { 'content-type': 'audio/wav', 'content-length': bytes.length }); return res.end(bytes)
      }
      if (req.method === 'GET' && path === '/api/voices') {
        const provider = await backend.status()
        const voices = (provider.voice?.personas || []).map(p => ({ persona: p.key, name: p.name, hasSample: p.hasSample !== false, hints: [], note: '' }))
        for (const local of store.state.voices || []) {
          const match = voices.find(v => v.persona === local.persona)
          if (match) { match.hints = local.hints || []; match.note = local.note || '' } else voices.push({ persona: local.persona, name: local.persona, hasSample: false, hints: local.hints || [], note: local.note || '' })
        }
        return json(res, 200, { voices, voiceReady: Boolean(provider.voice?.ready), recordingReady: Boolean(provider.voice?.recordingReady), designReady: Boolean(provider.voice?.designReady) })
      }
      if (req.method === 'GET' && path === '/api/bgm') {
        // 配乐库默认来自 R2 上的清单（~/.reel-kit/config.json 的 bgmManifest）。
        // 「没配库」和「配了但暂时拉不到」必须分开说 —— 否则用户会去检查根本没配的东西。
        const configured = await bgmConfigured()
        let items = [], error = ''
        try { items = await listBgm() || [] } catch (e) { error = redact(e.message).slice(0, 200) }
        return json(res, 200, { items, configured, error })
      }
      if (req.method === 'GET' && path === '/api/voice/tasks') {
        const data = await jsonFetch((process.env.REEL_VOXFLOW_URL || 'http://127.0.0.1:8866') + '/api/tasks')
        return json(res, 200, (data.tasks || []).filter(t => t.status === 'done' && t.result?.url?.startsWith('/api/audio/')).map(t => ({ id: t.id, name: t.result.filename || '声音作品', text: t.result.text || '', persona: t.result.persona || '' })).slice(0, 100))
      }
      if (req.method === 'GET' && path.startsWith('/media/')) {
        const assetId = path.slice(7)
        if (!uuid.test(assetId)) fail('素材不存在', 404)
        const asset = store.asset(assetId); const file = store.path(asset); const size = statSync(file).size
        const range = req.headers.range?.match(/^bytes=(\d+)-(\d*)$/)
        const start = range ? Number(range[1]) : 0
        const end = range?.[2] ? Math.min(Number(range[2]), size - 1) : size - 1
        if (start > end || start >= size) { res.writeHead(416, { 'content-range': `bytes */${size}` }); return res.end() }
        const headers = { 'content-type': asset.mime, 'accept-ranges': 'bytes', 'content-length': end - start + 1 }
        if (url.searchParams.has('download')) headers['content-disposition'] = `attachment; filename="asset${asset.mime === 'video/mp4' ? '.mp4' : ''}"; filename*=UTF-8''${encodeURIComponent(asset.name).replace(/'/g, '%27')}`
        if (range) headers['content-range'] = `bytes ${start}-${end}/${size}`
        res.writeHead(range ? 206 : 200, headers)
        return createReadStream(file, { start, end }).pipe(res)
      }
      if (req.method === 'DELETE' && path.startsWith('/api/channels/')) {
        const channelID = decodeURIComponent(path.slice('/api/channels/'.length))
        removeChannel(channelID)
        return json(res, 200, { ok: true, channels: channelList() })
      }
      if (req.method === 'POST') {
        if (req.headers['x-reel-action'] !== 'studio') fail('缺少工作台操作标记', 403)
        if (path === '/api/voice/record') {
          const name = url.searchParams.get('name')?.trim()
          const referenceText = url.searchParams.get('referenceText')?.trim()
          if (!name || name.length > 80 || !referenceText || referenceText.length > 1000) fail('请填写音色名称与录音原文')
          if (!backend.registerVoice) fail('声音服务不支持注册，请升级 VoxFlow', 503)
          const registered = await backend.registerVoice({ bytes: await body(req, 20 * 1024 * 1024), mime: req.headers['content-type'], name, referenceText, instruction: (url.searchParams.get('instruction') || '').slice(0, 500) })
          store.state.voices.push({ persona: registered.persona, name: registered.name, hints: [], note: (url.searchParams.get('instruction') || '').slice(0, 200), origin: url.searchParams.get('source') === 'record' ? 'record' : 'import', createdAt: now() })
          store.save()
          return json(res, 201, registered)
        }
        if (path === '/api/assets') {
          const projectId = url.searchParams.get('projectId'); store.project(projectId)
          const bytes = await body(req, 100 * 1024 * 1024)
          if (!bytes.length) fail('不能上传空文件')
          const asset = store.addAsset(projectId, url.searchParams.get('name') || '素材', req.headers['content-type']?.split(';')[0], bytes, { source: 'upload', origin: 'import', engine: '导入素材' })
          const probe = await probeBytes(asset.file, bytes)
          Object.assign(asset, probe); store.save()
          return json(res, 201, store.publicAsset(asset))
        }
        let input
        try { input = JSON.parse((await body(req)).toString()) } catch (error) { if (error.status) throw error; fail('无效 JSON') }
        if (path === '/api/voice/hints') {
          // 平台适配标记（全局）：某个音色适合哪些发布平台，方便选声音时判断。
          const persona = String(input.persona || '').slice(0, 200)
          if (!persona) fail('请指定音色')
          const hints = Array.isArray(input.hints) ? input.hints.filter(h => ['channels', 'xiaohongshu', 'landscape'].includes(h)) : []
          const record = store.state.voices.find(v => v.persona === persona)
          if (record) { record.hints = hints; record.note = String(input.note || '').slice(0, 200) } else store.state.voices.push({ persona, hints, note: String(input.note || '').slice(0, 200) })
          store.save()
          return json(res, 200, { persona, hints })
        }
        if (path === '/api/bgm/pull') {
          // 从配乐库取一首，落到本项目素材里：授权与来源一起记进素材，出片时可查
          const projectId = String(input.projectId || ''); store.project(projectId)
          const alias = String(input.alias || '').trim()
          if (!alias) fail('请选择配乐')
          // exact：接口只接受清单里的精确别名 —— 模糊匹配会让任意碎片命中某首曲子
          const resolved = await resolveBgmEntry(alias, { exact: true })
          if (!resolved) fail('配乐库里没有这一首，请刷新列表后重试')
          const bytes = readFileSync(resolved.file)
          if (!bytes.length) fail('配乐文件为空')
          const mime = resolved.file.endsWith('.wav') ? 'audio/wav' : resolved.file.endsWith('.m4a') ? 'audio/mp4' : resolved.file.endsWith('.flac') ? 'audio/flac' : 'audio/mpeg'
          const asset = store.addAsset(projectId, `${resolved.alias} · 配乐`, mime, bytes, {
            source: '配乐库', origin: 'import', engine: '配乐库',
            license: resolved.license, licenseUrl: resolved.licenseUrl || '',
            bgmSource: resolved.source || '', sourceUrl: resolved.sourceUrl || '',
          })
          const probe = await probeBytes(asset.file, bytes)
          Object.assign(asset, probe); store.save()
          return json(res, 201, store.publicAsset(asset))
        }
        if (path === '/api/workflow/select-sop') {
          const project = store.project(input.projectId)
          const version = store.state.sopVersions.find(v => v.id === input.sopVersionId && v.pipeline === 'github-recommend')
          if (!version) fail('制作方法不存在')
          project.sopVersionId = version.id; store.save()
          return json(res, 200, version)
        }
        if (path === '/api/workflow/quality') {
          const project = store.project(input.projectId)
          const job = store.state.jobs.find(j => j.id === input.jobId && j.projectId === project.id && j.input.kind === 'recommendation')
          if (!job || job.status !== 'succeeded' || !job.assetId) fail('请先选择已生成的推荐视频')
          const asset = store.asset(job.assetId, project.id)
          const expected = recommendationSettings(job.input)
          const info = JSON.parse(await command('ffprobe', ['-v', 'error', '-print_format', 'json', '-show_streams', '-show_format', store.path(asset)], { timeout: 30000 }))
          const video = (info.streams || []).find(s => s.codec_type === 'video')
          const audio = (info.streams || []).some(s => s.codec_type === 'audio')
          let decodable = true
          try { await command('ffmpeg', ['-v', 'error', '-xerror', '-i', store.path(asset), '-f', 'null', '-'], { timeout: 120000 }) } catch { decodable = false }
          const duration = Number(info.format?.duration) || 0
          const limits = limitsFor(store.state, job.input.platform)
          const size = statSync(store.path(asset)).size
          const checks = [
            { label: '画幅与尺寸', value: video ? `${video.width}×${video.height}` : '无视频流', passed: video?.width === expected.width && video?.height === expected.height },
            { label: '视频时长', value: `${duration.toFixed(1)} 秒 / 上限 ${limits.maxDuration} 秒`, passed: duration > 0 && duration <= limits.maxDuration },
            { label: '音轨', value: audio ? '存在' : '缺失', passed: audio },
            { label: '完整解码', value: decodable ? '通过' : '失败', passed: decodable },
            { label: '文件大小', value: `${(size / 1024 / 1024).toFixed(1)} MB / 上限 ${(limits.maxBytes / 1024 / 1024).toFixed(0)} MB`, passed: size <= limits.maxBytes },
          ]
          job.quality = { checks, passed: checks.every(c => c.passed), checkedAt: now(), assetId: asset.id }
          delete job.approval; store.save()
          return json(res, 200, job.quality)
        }
        if (path === '/api/workflow/remake') {
          const project = store.project(input.projectId)
          const job = store.state.jobs.find(j => j.id === input.jobId && j.projectId === project.id && j.input.kind === 'recommendation')
          if (!job || job.status !== 'succeeded') fail('请选择一条已完成的推荐视频')
          return json(res, 201, remakeProject(store, job))
        }
        if (path === '/api/workflow/') fail('接口不存在', 404)
        if (path.startsWith('/api/workflow/')) return json(res, 200, workflowAction(store, path.slice('/api/workflow/'.length), input))
        if (path === '/api/platform-limits') {
          const platform = String(input.platform || 'default')
          const limits = saveLimits(store.state, platform, input.limits || {})
          // 阈值变了，之前按旧阈值给出的「通过」就不再成立 —— 清掉，让用户重新检测。
          // 改 default 会影响所有平台，所以全都清。
          const affected = platform === 'default' ? store.state.jobs : store.state.jobs.filter(j => j.input?.platform === platform)
          let cleared = 0
          for (const job of affected) {
            if (job.input?.kind !== 'recommendation') continue
            if (job.quality) { delete job.quality; cleared++ }
            delete job.approval
          }
          store.save()
          return json(res, 200, { platform, limits, cleared })
        }
        if (path === '/api/assets/backfill') {
          // 历史素材入库时可能没有尺寸/时长；用 ffprobe 补齐，不改动任何已有文件。
          // 按媒体种类判定缺什么：图片没有时长是正常的，不该每次都被当成「待回填」。
          const needs = asset => {
            const missing = []
            if (asset.mime?.startsWith('image/')) {
              if (!asset.width || !asset.height) missing.push('尺寸')
            } else if (asset.mime?.startsWith('video/')) {
              if (!asset.width || !asset.height) missing.push('尺寸')
              if (asset.duration === undefined) missing.push('时长')
              if (asset.hasAudio === undefined) missing.push('音轨')
            } else if (asset.mime?.startsWith('audio/')) {
              if (asset.duration === undefined) missing.push('时长')
            }
            return missing
          }
          const targets = store.state.assets.map(a => ({ asset: a, missing: needs(a) })).filter(t => t.missing.length)
          let updated = 0
          const failed = []
          for (const { asset, missing } of targets) {
            try {
              const info = JSON.parse(await command('ffprobe', ['-v', 'error', '-print_format', 'json', '-show_streams', '-show_format', store.path(asset)], { timeout: 20000 }))
              const video = (info.streams || []).find(s => s.codec_type === 'video')
              const audio = (info.streams || []).some(s => s.codec_type === 'audio')
              const before = { width: asset.width, height: asset.height, duration: asset.duration, hasAudio: asset.hasAudio }
              if (missing.includes('尺寸') && video?.width) { asset.width = video.width; asset.height = video.height }
              if (missing.includes('时长') && info.format?.duration) asset.duration = Number(info.format.duration)
              if (missing.includes('音轨')) asset.hasAudio = audio
              if (asset.width !== before.width || asset.height !== before.height || asset.duration !== before.duration || asset.hasAudio !== before.hasAudio) updated++
              else failed.push({ id: asset.id, name: asset.name, reason: '探测成功但没有可补的字段' })
            } catch (error) {
              // 探测失败的素材要能看见，否则「回填完成」是假象。
              // 只留第一行原因：ffprobe 的报错会带本地绝对路径，贴到界面上既没用也泄露目录结构。
              const reason = redact(String(error.message || '').split('\n')[0]).slice(0, 200)
              failed.push({ id: asset.id, name: asset.name, reason })
            }
          }
          store.save()
          return json(res, 200, { updated, total: targets.length, failed })
        }
        if (path === '/api/project-settings') {
          const project = store.project(input.projectId)
          const keys = ['platform', 'voiceSource', 'persona', 'tone', 'speed', 'pause', 'bgmId', 'bgmGain', 'narrationId', 'imageFit', 'backend', 'model', 'voiceId', 'profile']
          const preferences = Object.fromEntries(keys.filter(k => input.settings?.[k] !== undefined).map(k => [k, input.settings[k]]))
          recommendationSettings(preferences)
          for (const key of ['persona', 'tone']) if (preferences[key] !== undefined && (typeof preferences[key] !== 'string' || preferences[key].length > 500)) fail('声音设置格式无效')
          // 通道、模型、音色、账户也要存：换台机器或换个人接手，配音仍得是同一个声音
          for (const key of ['backend', 'model', 'voiceId', 'profile']) if (preferences[key] !== undefined && (typeof preferences[key] !== 'string' || preferences[key].length > 120 || /[\u0000-\u001f/\\]/.test(preferences[key]))) fail('生成通道设置格式无效')
          for (const key of ['bgmId', 'narrationId']) if (preferences[key]) { if (!store.asset(preferences[key], project.id).mime.startsWith('audio/')) fail('请选择当前项目的音频素材') }
          project.recommendationSettings = preferences; store.save()
          return json(res, 200, { settings: preferences })
        }
        if (path === '/api/voice/import') {
          store.project(input.projectId)
          const base = process.env.REEL_VOXFLOW_URL || 'http://127.0.0.1:8866'
          const data = await jsonFetch(base + '/api/tasks')
          const task = (data.tasks || []).find(t => t.id === input.taskId && t.status === 'done' && t.result?.url?.startsWith('/api/audio/'))
          if (!task) fail('声音作品不存在或尚未完成')
          const { bytes } = await download(new URL(task.result.url, base))
          const asset = store.addAsset(input.projectId, task.result.filename || '声音作品.wav', 'audio/wav', bytes, { source: 'VoxFlow', origin: 'ai', engine: 'VoxFlow 本地语音', prompt: task.result.text || '', persona: task.result.persona || '', sourceTaskId: task.id })
          const probe = await probeBytes(asset.file, bytes)
          Object.assign(asset, probe); store.save()
          return json(res, 201, store.publicAsset(asset))
        }
        if (path === '/api/recommendation/draft') {
          store.project(input.projectId)
          return json(res, 200, await projectDraft(input.repoUrl, input.projectId, store))
        }
        if (path === '/api/jobs/resume') {
          // 取回上次已经提交、但下载失败的上游产物。
          // 不重新生成：付费通道提交即计费，重新提交等于再付一次。
          const job = store.state.jobs.find(j => j.id === input.jobId)
          if (!job) fail('任务不存在', 404)
          if (!job.upstream?.taskId) fail('这条任务没有可继续的上游任务；请重新提交')
          if (['queued', 'running'].includes(job.status)) fail('任务正在执行中，请稍候')
          if (!backend.resume) fail('当前后端不支持取回，请升级 reel-kit', 503)
          store.project(job.projectId)
          try {
            const asset = await backend.resume({ upstream: job.upstream, input: job.input }, text => { job.message = text; job.updatedAt = now(); store.save() })
            job.assetId = asset.id; job.status = 'succeeded'; delete job.error; job.updatedAt = now(); store.save()
            return json(res, 200, job)
          } catch (error) {
            job.status = 'failed'; job.error = redact(error.message); job.updatedAt = now(); store.save()
            throw error
          }
        }
        if (path === '/api/projects') {
          const name = String(input.name || '').trim()
          if (!name || name.length > 80) fail('项目名称须为 1–80 字')
          const project = { id: id(), name, createdAt: now() }; store.state.projects.push(project); store.save()
          return json(res, 201, project)
        }
        if (path === '/api/channels/check') {
          const channel = loadCustomChannels().find(c => c.id === String(input.id || '').trim())
          if (!channel) fail('没有这条通道，先在设置里添加')
          const result = await probeChannel(channel)
          return json(res, 200, { id: channel.id, label: channel.label, ok: result.ok, checks: result.checks })
        }
        if (path === '/api/channels') {
          const entry = {
            id: String(input.id || '').trim(),
            label: String(input.label || '').trim(),
            kind: String(input.kind || 'image').trim(),
            protocol: String(input.protocol || 'openai').trim(),
            baseUrl: String(input.baseUrl || '').trim(),
            model: String(input.model || '').trim(),
            env: String(input.env || '').trim(),
            ...(input.imagePath ? { imagePath: String(input.imagePath).trim() } : {}),
            ...(input.taskPath ? { taskPath: String(input.taskPath).trim() } : {}),
            ...(input.auth ? { auth: String(input.auth).trim() } : {}),
          }
          if (!entry.id) fail('通道 id 不能为空')
          if (!/^[a-z0-9][a-z0-9._-]{0,40}$/i.test(entry.id)) fail('通道 id 只能用字母、数字、点、下划线和短横线')
          upsertChannel(entry)
          return json(res, 200, { ok: true, channels: channelList() })
        }
        if (path === '/api/jobs') {
          const value = validate(input, store)
          if (store.state.jobs.filter(j => ['queued', 'running'].includes(j.status)).length >= 10) fail('队列已满，请等待当前任务完成', 429)
          const job = { id: id(), projectId: value.projectId, input: value, status: 'queued', createdAt: now(), updatedAt: now(), message: '等待执行' }
          if (value.kind === 'recommendation') job.sopSnapshot = snapshotSop(store.state, store.project(value.projectId), value)
          store.state.jobs.push(job); store.save(); setImmediate(pump)
          return json(res, 202, job)
        }
        fail('接口不存在', 404)
      }
      if (req.method !== 'GET') fail('方法不支持', 405)
      const files = { '/': ['index.html', 'text/html'], '/app.js': ['app.js', 'text/javascript'], '/recommendation-controls.js': ['recommendation-controls.js', 'text/javascript'], '/voice-library.js': ['voice-library.js', 'text/javascript'], '/workflow-panel.js': ['workflow-panel.js', 'text/javascript'], '/style.css': ['style.css', 'text/css'] }
      const entry = files[path]
      if (!entry) fail('页面不存在', 404)
      res.setHeader('content-type', `${entry[1]}; charset=utf-8`); res.end(readFileSync(join(web, entry[0])))
    } catch (error) { if (!res.headersSent) json(res, error.status || 400, { error: redact(error.message) }); else res.destroy() }
  })
  return { server, store }
}
export function startStudio({ port = Number(process.env.REEL_STUDIO_PORT || 4390) } = {}) {
  const { server } = createStudio()
  server.on('error', error => { console.error(error.code === 'EADDRINUSE' ? `端口 ${port} 已占用，请设置 REEL_STUDIO_PORT。` : error.message); process.exitCode = 1 })
  server.listen(port, '127.0.0.1', () => console.log(`reel-kit Studio → http://127.0.0.1:${port}`))
  return server
}
