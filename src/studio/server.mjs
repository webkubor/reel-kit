import http from 'node:http'
import { createReadStream, statSync, readFileSync, readdirSync, mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'
import { homedir } from 'node:os'
import { createStore, id, now } from './store.mjs'
import { createProviders } from './providers.mjs'
import { exportVideo } from './export.mjs'
import { redact, command, jsonFetch, download, probeBytes } from './process.mjs'
import { projectDraft, makeRecommendation, githubRepo } from './recommendation.mjs'
import { recommendationSettings } from './video-settings.mjs'

const web = fileURLToPath(new URL('../../studio/', import.meta.url))
const templates = fileURLToPath(new URL('../../templates/', import.meta.url))
const templateNames = readdirSync(templates).filter(f => f.endsWith('.html') && f !== 'project-recommendation.html').map(f => f.slice(0, -5))
const uuid = /^[a-f\d-]{36}$/
function fail(message, status = 400) { throw Object.assign(new Error(message), { status }) }
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
  if (!['image', 'video', 'speech', 'clone', 'voice-design', 'export', 'recommendation'].includes(input.kind)) fail('任务类型不支持')
  const allowed = ['kind', 'projectId', 'prompt', 'ratio', 'duration', 'resolution', 'assetIds', 'audioId', 'referenceId', 'persona', 'referenceText', 'voiceName', 'template', 'captions', 'title', 'profile', 'repoUrl', 'voiceIds', 'platform', 'voiceSource', 'tone', 'emotion', 'speed', 'pause', 'bgmId', 'bgmGain', 'narrationId', 'imageFit']
  const value = Object.fromEntries(allowed.filter(k => input[k] !== undefined).map(k => [k, input[k]]))
  if (typeof value.prompt !== 'string' || !value.prompt.trim() || value.prompt.length > 10000) {
    if (!['export', 'recommendation'].includes(value.kind)) fail('请输入 1–10000 字的提示词或配音文字')
  }
  value.ratio = value.ratio || '9:16'
  if (!['9:16', '16:9', '1:1'].includes(value.ratio)) fail('不支持的画幅')
  value.duration = Number(value.duration || 6)
  if (!Number.isFinite(value.duration) || value.duration < 1 || value.duration > 30) fail('时长须为 1–30 秒')
  if (value.kind === 'video' && (!Number.isInteger(value.duration) || value.duration < 4 || value.duration > 15)) fail('H3 时长请选择 4–15 秒的整数')
  if (value.resolution && !['768P', '1080P', '2K'].includes(value.resolution)) fail('不支持的分辨率')
  value.assetIds = value.assetIds || []
  if (!Array.isArray(value.assetIds) || value.assetIds.length > 30) fail('最多选择 30 个素材')
  value.assetIds.forEach(assetId => store.asset(assetId, value.projectId))
  for (const key of ['referenceId', 'audioId', 'bgmId', 'narrationId']) if (value[key]) store.asset(value[key], value.projectId)
  for (const key of ['persona', 'referenceText', 'voiceName', 'captions', 'title', 'profile', 'tone', 'emotion']) if (value[key] !== undefined && (typeof value[key] !== 'string' || value[key].length > 10000)) fail(`${key} 格式无效`)
  if (value.kind === 'voice-design' && (!value.voiceName?.trim() || value.voiceName.length > 80 || !value.tone?.trim() || value.tone.length > 500 || value.prompt.length > 45)) fail('新音色需要名称、声音描述和不超过 45 字的试听文字')
  if (value.kind === 'recommendation') {
    githubRepo(value.repoUrl)
    if (typeof value.title !== 'string' || !value.title.trim() || value.title.length > 100) fail('项目标题须为 1–100 字')
    const lines = typeof value.captions === 'string' ? value.captions.split('\n').map(s => s.trim()).filter(Boolean) : []
    if (!lines.length || lines.length > 8 || lines.some(s => s.length > 150)) fail('旁白须为 1–8 行，每行不超过 150 字')
    const settings = recommendationSettings(value)
    value.ratio = settings.ratio
    value.voiceSource = settings.voiceSource
    if (!value.assetIds.length) fail('请至少选择一张项目画面')
    for (const id of value.assetIds) if (!store.asset(id, value.projectId).mime.startsWith('image/')) fail('项目画面必须是图片')
    for (const key of ['bgmId', 'narrationId']) if (value[key] && !store.asset(value[key], value.projectId).mime.startsWith('audio/')) fail('旁白和 BGM 须选择音频素材')
    if (settings.voiceSource === 'audio' && !value.narrationId) fail('请选择整段旁白素材')
    if (settings.voiceSource === 'tts' && !value.persona) fail('请选择音色或创建新音色')
    if (!Array.isArray(value.voiceIds) || value.voiceIds.length !== lines.length) fail('旁白素材与分镜数量不匹配')
    value.voiceIds.filter(Boolean).forEach(id => { if (!store.asset(id, value.projectId).mime.startsWith('audio/')) fail('分镜旁白须选择音频') })
    if (settings.voiceSource === 'scenes' && !value.persona && value.voiceIds.some(id => !id)) fail('空缺分镜需要选择生成音色')
  }
  value.template = value.template || 'sticker-promo'
  if (!templateNames.includes(value.template)) fail('模板不存在')
  if (value.kind === 'export' && !['9:16', '16:9'].includes(value.ratio)) fail('导出支持竖屏或横屏')
  return value
}
export function createStudio({ root = process.env.REEL_STUDIO_HOME || join(homedir(), '.reel-kit', 'studio'), providers, exportFn = exportVideo } = {}) {
  const store = createStore(root)
  const backend = providers || createProviders(store)
  let worker = false
  const pump = async () => {
    if (worker) return
    worker = true
    try {
      for (;;) {
        const job = store.state.jobs.find(j => j.status === 'queued')
        if (!job) break
        const progress = text => { job.message = text; job.updatedAt = now(); store.save() }
        const report = step => {
          const steps = (job.steps || []).filter(s => s.id !== step.id)
          steps.push(step)
          job.steps = steps
          job.updatedAt = now(); store.save()
        }
        job.status = 'running'; job.steps = []; progress('正在准备任务…')
        try {
          const asset = job.input.kind === 'export' ? await exportFn(job.input, store, progress) : job.input.kind === 'recommendation' ? await makeRecommendation(job.input, store, backend, progress, report) : await backend.generate(job.input, progress)
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
      if (req.method === 'GET' && path === '/api/providers') return json(res, 200, await backend.status())
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
        return json(res, 200, { voices, voiceReady: Boolean(provider.voice?.ready), designReady: Boolean(provider.voice?.designReady) })
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
      if (req.method === 'POST') {
        if (req.headers['x-reel-action'] !== 'studio') fail('缺少工作台操作标记', 403)
        if (path === '/api/voice/record') {
          // 录音是全局配置：直接登记为 VoxFlow 音色，所有项目共用，不挂在单个项目下。
          const bytes = await body(req, 20 * 1024 * 1024)
          if (!bytes.length) fail('录音为空')
          const name = (url.searchParams.get('name') || '').trim().slice(0, 40) || '我的声音'
          const instruction = (url.searchParams.get('instruction') || '').trim().slice(0, 300) || '本人真实录音，自然清晰'
          const dir = mkdtempSync(join(tmpdir(), 'reel-recording-'))
          try {
            const source = join(dir, 'recording'); const wav = join(dir, 'recording.wav')
            writeFileSync(source, bytes, { mode: 0o600 })
            await command('ffmpeg', ['-v', 'error', '-i', source, '-ac', '1', '-ar', '24000', wav], { timeout: 60000 })
            const form = new FormData()
            const key = `reel-rec-${Date.now().toString(36)}`
            form.set('key', key); form.set('name', name); form.set('instruction', instruction)
            form.set('audio', new Blob([readFileSync(wav)], { type: 'audio/wav' }), 'recording.wav')
            await jsonFetch(`${process.env.REEL_VOXFLOW_URL || 'http://127.0.0.1:8866'}/api/personas/add`, { method: 'POST', body: form, signal: AbortSignal.timeout(120000) })
            return json(res, 201, { persona: key, name })
          } finally { rmSync(dir, { recursive: true, force: true }) }
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
        if (path === '/api/project-settings') {
          const project = store.project(input.projectId)
          const keys = ['platform', 'voiceSource', 'persona', 'tone', 'speed', 'pause', 'bgmId', 'bgmGain', 'narrationId', 'imageFit']
          const preferences = Object.fromEntries(keys.filter(k => input.settings?.[k] !== undefined).map(k => [k, input.settings[k]]))
          recommendationSettings(preferences)
          for (const key of ['persona', 'tone']) if (preferences[key] !== undefined && (typeof preferences[key] !== 'string' || preferences[key].length > 500)) fail('声音设置格式无效')
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
        if (path === '/api/projects') {
          const name = String(input.name || '').trim()
          if (!name || name.length > 80) fail('项目名称须为 1–80 字')
          const project = { id: id(), name, createdAt: now() }; store.state.projects.push(project); store.save()
          return json(res, 201, project)
        }
        if (path === '/api/jobs') {
          const value = validate(input, store)
          if (store.state.jobs.filter(j => ['queued', 'running'].includes(j.status)).length >= 10) fail('队列已满，请等待当前任务完成', 429)
          const job = { id: id(), projectId: value.projectId, input: value, status: 'queued', createdAt: now(), updatedAt: now(), message: '等待执行' }
          store.state.jobs.push(job); store.save(); setImmediate(pump)
          return json(res, 202, job)
        }
        fail('接口不存在', 404)
      }
      if (req.method !== 'GET') fail('方法不支持', 405)
      const files = { '/': ['index.html', 'text/html'], '/app.js': ['app.js', 'text/javascript'], '/recommendation-controls.js': ['recommendation-controls.js', 'text/javascript'], '/voice-library.js': ['voice-library.js', 'text/javascript'], '/style.css': ['style.css', 'text/css'] }
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
