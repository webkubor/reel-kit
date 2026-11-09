import { readFileSync } from 'node:fs'
import { randomUUID } from 'node:crypto'
import { setTimeout as delay } from 'node:timers/promises'
import { join } from 'node:path'
import { homedir } from 'node:os'
import { command, commandFull, available, jsonFetch, download, probeBytes } from './process.mjs'

const H3_BASE = 'https://api.minimaxi.com'
const MINIMAX_TTS_URL = 'https://api.minimaxi.com/v1/t2a_v2'
const CUSTOM_CONFIG = () => join(homedir(), '.reel-kit', 'providers.json')

/**
 * MiniMax 的 TTS 模型。
 *
 * 只有真正合成成功过的才列进来 —— 列一个调不通的模型，用户点了只会白花钱。
 * 别的模型名可以在界面上手填（allowCustomModel）。
 */
export const MINIMAX_TTS_MODELS = [
  { id: 'speech-02-hd', name: 'speech-02-hd · 高清' },
]

/**
 * MiniMax 的系统音色。
 *
 * 音色和模型是两回事：模型是「用什么引擎合成」，音色是「谁在说话」。
 * 下面每一条都对着真实接口验过能出声；上游新增/下线不在这里，
 * 所以界面上留了自定义输入框 —— 填别的不在表里的音色 ID 也能试。
 */
export const MINIMAX_VOICES = [
  { id: 'male-qn-qingse', name: '青涩男声' },
  { id: 'male-qn-badao', name: '霸道男声' },
  { id: 'male-qn-jingying', name: '精英男声' },
  { id: 'female-shaonv', name: '少女音' },
  { id: 'female-chengshu', name: '成熟女声' },
  { id: 'female-tianmei', name: '甜美女声' },
  { id: 'presenter_female', name: '女主播' },
  { id: 'presenter_male', name: '男主播' },
  { id: 'audiobook_female_1', name: '有声书女声' },
  { id: 'audiobook_male_1', name: '有声书男声' },
  {
    id: 'Chinese (Mandarin)_Warm_Girl',
    name: '温暖女声（中文）'
  },
  {
    id: 'English_Trustworth_Man',
    name: '可靠男声（英文）'
  },
]

export function h3Body(input, referenceUrl) {
  const content = [{ type: 'text', text: input.prompt }]
  if (referenceUrl) content.push({ type: 'image_url', image_url: { url: referenceUrl }, role: 'reference_image' })
  return { model: input.model || 'MiniMax-H3', content, ratio: input.ratio, duration: input.duration, resolution: input.resolution || '768P' }
}
function outputUrl(stdout) {
  const urls = stdout.split(/\r?\n/).map(s => s.trim()).filter(s => /^https:\/\/\S+$/.test(s))
  if (!urls.length) throw new Error('生成工具未返回素材地址，请检查 CLI 版本')
  return urls.at(-1)
}

/**
 * 从 CLI 的 stderr 里取视频任务 ID。
 *
 * 中台的回执以任务为单位 —— 想知道「这条片子实际是哪个模型跑的」，
 * 必须拿同一个任务 ID 去查，不能拿「最近一个任务」凑。
 */
export function museavJobId(stderr) {
  const match = /视频任务已提交:\s*([0-9a-f-]{8,})/i.exec(stderr || '')
  return match ? match[1] : ''
}

/**
 * 查中台的任务回执，取「实际跑的模型」。
 *
 * requestedModel 是我们请求的，actualModel 是网关回执里写的 —— 两者不一定相同
 * （不指定模型时由中台智能路由）。查不到就写 unknown，不拿请求值冒充实际值。
 */
export async function museavReceipt({ jobId, cdnUrl } = {}) {
  try {
    const stdout = await command('museav', ['jobs', '--limit', '50'], { timeout: 30000 })
    const list = JSON.parse(stdout)
    const hit = list.find(j => (jobId && j.id === jobId) || (cdnUrl && j.cdn_url === cdnUrl))
    if (!hit) return { id: jobId || '', actualModel: 'unknown' }
    return { id: hit.id || jobId || '', actualModel: hit.model || 'unknown' }
  } catch { return { id: jobId || '', actualModel: 'unknown' } }
}

/**
 * 从中台的任务列表里找一条任务的产物地址。
 *
 * 用途只有一个：提交之后下载失败时，把已经生成（已经计费）的那条片子取回来，
 * 而不是让用户重新提交一次、再付一遍钱。
 */
export async function museavJobResult(jobId) {
  if (!jobId) return undefined
  try {
    const stdout = await command('museav', ['jobs', '--limit', '50'], { timeout: 30000 })
    const list = JSON.parse(stdout)
    const hit = list.find(j => j.id === jobId)
    if (!hit) return undefined
    return { id: hit.id, status: hit.status || '', cdnUrl: hit.cdn_url || '', model: hit.model || 'unknown' }
  } catch { return undefined }
}

/**
 * MuseAV 的视频档次（Seedance / Kling / Wan …）从 CLI 读。
 *
 * 不在 reel-kit 里写死一份模型清单 —— 中台随时会加档或改时长上限，
 * 写死的清单会变成「界面上能选、实际调不通」的假选项。
 *
 * CLI 的输出分两处：模型名单独走 stdout（每行一个），
 * 「可用视频档次」那张带时长/分辨率上限的表走 stderr。两边一起读，
 * 上限要露出来 —— 选了时长 12 秒的 Kling v3（只支持 5–10 秒）会白等一次。
 *
 * stderr 必须整段拿：进度回调给的是每块截断后的尾巴，
 * 拿它拼表会缺前半截，解析出半张表就等于没有上限信息。
 */
export async function listMuseavVideoModels() {
  try {
    const { stdout, stderr } = await commandFull('museav', ['models', '--video'], { timeout: 20000 })
    const names = stdout.split(/\r?\n/).map(s => s.trim()).filter(Boolean)
    if (!names.length) return []
    const limits = new Map()
    for (const line of stderr.split(/\r?\n/)) {
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('可用视频档次') || trimmed.startsWith('出视频')) continue
      const [name, ...rest] = trimmed.split(/\s{2,}/)
      if (name && rest.length) limits.set(name.trim(), rest.join(' ').trim())
    }
    return names.map(name => {
      const limit = limits.get(name) || ''
      const range = limit.match(/(\d+)\s*[–\-~]\s*(\d+)\s*秒/)
      const single = limit.match(/最短\s*(\d+)\s*秒/)
      return {
        name,
        limit,
        minSeconds: range ? Number(range[1]) : single ? Number(single[1]) : undefined,
        maxSeconds: range ? Number(range[2]) : undefined,
        resolutions: limit.split(/\s{2,}/).filter(part => /^\d+[pP]$/.test(part) || /^\d+p\/\d+p/.test(part)),
      }
    })
  } catch { return [] }
}

/**
 * MuseAV 的图片模型同样从 CLI 读。
 *
 * 清单里的 value 才是能传给 --model 的对外名（标签是给人看的），
 * 例如标签「ChatGPT Image 2.6」对应 value「gpt-image-2.6」。
 */
export async function listMuseavImageModels() {
  try {
    const { stdout } = await commandFull('museav', ['models'], { timeout: 20000 })
    const values = stdout.split(/\r?\n/).map(s => s.trim()).filter(Boolean)
    return values.map(value => ({ value, label: value }))
  } catch { return [] }
}

/**
 * 自定义通道配置（可选）。
 *
 * 用户自己买了套餐的供应商写在这里，工作台就能把它当第三条通道来选。
 * 关键点：**baseUrl、模型名、密钥环境变量全由使用者给** —— 我们不内置任何
 * 一家的默认地址（猜错了就是别人的服务、别人的账单）。
 *
 * 两种协议：
 *   openai    —— 一次 POST 拿结果（OpenAI 兼容的生图接口都是这个形状）
 *   ark-task  —— 提交任务拿 ID，再轮询到出结果（任务式视频接口）
 */
export function loadCustomChannels() {
  let raw
  try { raw = JSON.parse(readFileSync(CUSTOM_CONFIG(), 'utf8')) } catch { return [] }
  const list = Array.isArray(raw) ? raw : (raw?.channels || [])
  const out = []
  for (const item of list) {
    const id = String(item?.id || '').trim()
    const baseUrl = String(item?.baseUrl || '').trim().replace(/\/+$/, '')
    const model = String(item?.model || '').trim()
    const env = String(item?.env || '').trim()
    // 缺任何一项都不算一条可用通道 —— 宁可界面上不出现，也不给一个必然失败的选项
    if (!id || !baseUrl || !model || !env) continue
    // 自定义 TTS 还没实现：把它降级成生图通道，比明确拒绝更糟 —— 会生成出错的图
    if (item?.kind === 'tts') continue
    const kind = item?.kind === 'video' ? 'video' : 'image'
    // 协议要和类型对得上：ark-task 是「先提交任务再轮询」，
    // 只适用于视频；生图是一次 POST，用 openai 那套。
    const protocol = item?.protocol === 'ark-task' ? 'ark-task' : 'openai'
    if (kind === 'video' && protocol !== 'ark-task') continue
    out.push({
      id,
      label: String(item?.label || id).trim(),
      kind,
      protocol,
      baseUrl,
      model,
      env,
      key: process.env[env] || '',
      imagePath: String(item?.imagePath || '/images/generations').trim(),
      taskPath: String(item?.taskPath || '/contents/generations/tasks').trim(),
      voiceId: String(item?.voiceId || '').trim(),
    })
  }
  return out
}

/**
 * 自定义通道配置的问题清单。
 *
 * 配置错了却一声不吭，用户只会看到「这个通道不存在」——
 * 必须把原因摆出来：哪一条、哪一项、为什么不能用。
 */
export function customChannelIssues() {
  let raw
  try { raw = JSON.parse(readFileSync(CUSTOM_CONFIG(), 'utf8')) } catch (error) {
    return { file: CUSTOM_CONFIG(), problems: error.code === 'ENOENT' ? [] : [`配置文件无法解析：${error.message}`] }
  }
  const list = Array.isArray(raw) ? raw : (raw?.channels || [])
  const problems = []
  if (!Array.isArray(list)) return { file: CUSTOM_CONFIG(), problems: ['channels 必须是数组'] }
  const seen = new Set()
  list.forEach((item, index) => {
    const id = String(item?.id || '').trim()
    const label = id || `第 ${index + 1} 条`
    if (!id) problems.push(`${label}：缺少 id`)
    else if (seen.has(id)) problems.push(`${label}：id 重复`)
    else seen.add(id)
    if (!String(item?.baseUrl || '').trim()) problems.push(`${label}：缺少 baseUrl`)
    if (!String(item?.model || '').trim()) problems.push(`${label}：缺少 model`)
    if (!String(item?.env || '').trim()) problems.push(`${label}：缺少 env（环境变量名）`)
    else if (!process.env[String(item.env).trim()]) problems.push(`${label}：环境变量 ${item.env} 未注入，调用会被拒绝`)
    if (item?.kind && !['image', 'video', 'tts'].includes(item.kind)) problems.push(`${label}：kind 只支持 image / video / tts`)
    if (item?.kind === 'tts') problems.push(`${label}：自定义 TTS 通道尚未实现，请改用 MiniMax TTS 或本地 VoxFlow`)
    if (item?.kind === 'video' && item?.protocol && item.protocol !== 'ark-task') problems.push(`${label}：视频通道目前只支持 ark-task 协议`)
  })
  return { file: CUSTOM_CONFIG(), problems }
}

/** 画幅 → 像素尺寸（生图接口用） */
function sizeFor(ratio) {
  if (ratio === '16:9') return '1920x1080'
  if (ratio === '1:1') return '1024x1024'
  return '1080x1920'
}

/**
 * fetch 失败时真正的原因藏在 cause 里。
 *
 * 不挖出来，界面上只会显示「fetch failed」三个字 —— 用户既不知道是网络断了、
 * 是超时、还是 DNS 不通，更没法自己判断要不要重试（重试可能重复计费）。
 */
function withCause(error, what) {
  const detail = error?.cause?.message || error?.message || '未知原因'
  return new Error(`${what}失败：${detail}`)
}

function readReelConfig() {
  try {
    return JSON.parse(readFileSync(join(homedir(), '.reel-kit', 'config.json'), 'utf8'))
  } catch {
    return {}
  }
}

export function createProviders(store, { fetcher = fetch, pause = delay } = {}) {
  const voiceBase = process.env.REEL_VOXFLOW_URL || 'http://127.0.0.1:8866'
  const cfg = readReelConfig()
  const primaryKey = process.env.MINIMAX_API_KEY || cfg.minimaxApiKey || ''
  const secondaryKey = process.env.MINIMAX_API_KEY_2 || cfg.minimaxApiKey2 || ''
  const profiles = [
    { id: 'primary', label: process.env.REEL_MINIMAX_LABEL || 'MiniMax 主账户', key: primaryKey },
    { id: 'secondary', label: process.env.REEL_MINIMAX_LABEL_2 || 'MiniMax 第二账户', key: secondaryKey },
  ]
  const h3 = async (path, key, data) => {
    let response
    try {
      response = await fetcher(H3_BASE + path, {
        method: data ? 'POST' : 'GET', headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
        body: data ? JSON.stringify(data) : undefined, signal: AbortSignal.timeout(30000),
      })
    } catch (error) { throw withCause(error, 'MiniMax 请求') }
    const result = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(`MiniMax HTTP ${response.status}${result.base_resp?.status_msg ? `：${result.base_resp.status_msg}` : ''}；不会自动切换账户重试`)
    if (result.base_resp?.status_code) throw new Error(`MiniMax 拒绝请求（${result.base_resp.status_code}）：${result.base_resp.status_msg || '请核对权限或额度'}`)
    return result
  }
  const registerVoice = async ({ bytes, mime, name, referenceText, instruction = '' }) => {
    if (!name?.trim() || name.length > 80 || !referenceText?.trim() || referenceText.length > 1000) throw new Error('请填写音色名称（最多 80 字）与录音原文（最多 1000 字）')
    const types = { 'audio/wav': 'wav', 'audio/x-wav': 'wav', 'audio/mpeg': 'mp3', 'audio/mp3': 'mp3', 'audio/mp4': 'm4a', 'audio/x-m4a': 'm4a', 'audio/ogg': 'ogg', 'audio/webm': 'webm', 'audio/flac': 'flac', 'audio/aac': 'aac' }
    const ext = types[mime?.split(';')[0]]
    if (!ext || !bytes.length || bytes.length > 20 * 1024 * 1024) throw new Error('请选择有效的音频文件（最多 20 MB）')
    const info = await jsonFetch(voiceBase + '/api/personas')
    if (!info.capabilities?.reference_text_registration) throw new Error('请将 VoxFlow 升级至支持录音原文注册的版本（0.10.1 或更新），然后重启服务')
    const form = new FormData()
    form.set('key', 'reel_' + randomUUID().replaceAll('-', ''))
    form.set('name', name.trim()); form.set('ref_text', referenceText.trim()); form.set('instruction', instruction)
    form.set('audio', new Blob([bytes], { type: mime }), `reference.${ext}`)
    const result = await jsonFetch(voiceBase + '/api/personas/add', { method: 'POST', body: form })
    if (!result.ok || !result.key || result.ref_text !== referenceText.trim()) throw new Error('VoxFlow 未确认完整的音色与原文，请检查声音库后再操作')
    return { persona: result.key, name: result.name }
  }

  const safeDownload = async url => {
    try { return await download(url) } catch (error) { throw withCause(error, '下载素材') }
  }
  /** MiniMax 文字转语音：返回音频字节 */
  const minimaxTts = async ({ key, text, model, voiceId, speed = 1 }) => {
    const response = await fetcher(MINIMAX_TTS_URL, {
      method: 'POST',
      headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: model || MINIMAX_TTS_MODELS[0].id,
        text,
        stream: false,
        voice_setting: { voice_id: voiceId, speed, vol: 1, pitch: 0 },
        audio_setting: { sample_rate: 32000, bitrate: 128000, format: 'mp3', channel: 1 },
      }),
      signal: AbortSignal.timeout(120000),
    })
    const result = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(`MiniMax TTS HTTP ${response.status}${result.base_resp?.status_msg ? `：${result.base_resp.status_msg}` : ''}`)
    // 注意：成功回执里也带 base_resp，status_code 为 0 才算成功 —— 有 base_resp 不等于失败
    if (result.base_resp?.status_code) throw new Error(`MiniMax 拒绝请求（${result.base_resp.status_code}）：${result.base_resp.status_msg || '请核对权限或额度'}`)
    const hex = result.data?.audio
    if (!hex) throw new Error('MiniMax 未返回音频数据')
    const bytes = Buffer.from(hex, 'hex')
    if (!bytes.length) throw new Error('MiniMax 返回的音频为空')
    return { bytes, mime: 'audio/mpeg' }
  }

  /** 自定义通道：一次 POST 出图的 OpenAI 兼容形状 */
  const customImage = async ({ channel, prompt, ratio }) => {
    const response = await fetcher(channel.baseUrl + channel.imagePath, {
      method: 'POST',
      headers: { Authorization: `Bearer ${channel.key}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ model: channel.model, prompt, size: sizeFor(ratio), response_format: 'url' }),
      signal: AbortSignal.timeout(180000),
    })
    const result = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(`自定义通道 HTTP ${response.status}${result?.error?.message ? `：${result.error.message}` : ''}`)
    const item = result?.data?.[0]
    if (item?.b64_json) return { bytes: Buffer.from(item.b64_json, 'base64'), mime: 'image/png' }
    if (item?.url) return safeDownload(item.url)
    throw new Error('自定义通道未返回图片地址')
  }

  /** 自定义通道：提交任务 + 轮询的形状 */
  const customVideo = async ({ channel, prompt, ratio, duration, onSubmitted }) => {
    const submit = await fetcher(channel.baseUrl + channel.taskPath, {
      method: 'POST',
      headers: { Authorization: `Bearer ${channel.key}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model: channel.model,
        content: [{ type: 'text', text: prompt }],
        ratio,
        duration,
        resolution: '1080p',
      }),
      signal: AbortSignal.timeout(60000),
    })
    const submitted = await submit.json().catch(() => ({}))
    if (!submit.ok) throw new Error(`自定义通道 HTTP ${submit.status}${submitted?.error?.message ? `：${submitted.error.message}` : ''}`)
    const taskId = submitted?.id || submitted?.data?.id
    if (!taskId) throw new Error('自定义通道未返回任务 ID；请先核对上游，不要重复提交')
    onSubmitted?.(String(taskId))
    for (let attempt = 0; attempt < 180; attempt++) {
      await pause(5000)
      const polled = await fetcher(`${channel.baseUrl}${channel.taskPath}/${encodeURIComponent(taskId)}`, {
        headers: { Authorization: `Bearer ${channel.key}` },
        signal: AbortSignal.timeout(30000),
      })
      const body = await polled.json().catch(() => ({}))
      const status = body?.status || body?.data?.status
      if (['failed', 'cancelled', 'canceled'].includes(status)) throw new Error(`任务 ${taskId} 失败：${body?.error?.message || body?.error || status}`)
      if (status === 'succeeded') {
        const url = body?.content?.video_url || body?.content?.video_url || body?.data?.content?.video_url
        if (!url) throw new Error(`任务 ${taskId} 已完成，但未返回下载地址`)
        return safeDownload(url)
      }
    }
    throw new Error(`任务 ${taskId} 仍在处理中，已停止轮询；请核对上游后再操作`)
  }

  return {
    registerVoice,
    /**
     * 接着上一个任务往下走：只查上游、只下载，不重新提交。
     *
     * 付费通道提交即计费。上一次失败往往不是生成失败，而是下载失败 ——
     * 这时候再点一次生成，等于又付一遍钱。所以这里必须能凭同一个任务 ID
     * 把已经生成的产物取回来。
     */
    async resume({ upstream, input }, progress) {
      if (!upstream?.taskId) throw new Error('这条记录没有上游任务 ID，无法继续取回；请重新提交')
      if (upstream.channel === 'minimax') {
        const profile = profiles.find(p => p.id === (upstream.profile || input?.profile || 'primary'))
        if (!profile?.key) throw new Error('对应的 MiniMax 账户未配置，无法查询该任务')
        progress(`正在查询 H3 任务 ${upstream.taskId}…`)
        const result = await h3(`/v2/query/video_generation/${encodeURIComponent(upstream.taskId)}`, profile.key)
        const task = result.task || result
        if (['failed', 'cancelled', 'canceled'].includes(task.status)) throw new Error(`H3 任务 ${upstream.taskId} 失败：${task.error?.message || task.error || task.status}`)
        if (task.status !== 'succeeded') throw new Error(`H3 任务 ${upstream.taskId} 仍是「${task.status || '处理中'}」，尚未出片；过一会儿再试取回，不必重新提交`)
        const url = task.content?.url
        if (!url) throw new Error(`H3 任务 ${upstream.taskId} 已完成，但未返回下载地址`)
        progress('上游已出片，正在下载…')
        const { bytes } = await safeDownload(url)
        const probe = await probeBytes('video.mp4', bytes)
        return store.addAsset(input.projectId, 'H3 视频.mp4', 'video/mp4', bytes, {
          source: 'MiniMax H3', origin: 'ai', engine: `MiniMax H3 视频${upstream.model && upstream.model !== 'MiniMax-H3' ? ' · ' + upstream.model : ''}`,
          upstreamTaskId: String(upstream.taskId), prompt: input.prompt || '', profile: profile.id,
          requestedModel: upstream.model || 'MiniMax-H3', actualModel: task.model || 'unknown', ...probe,
        })
      }
      if (upstream.channel === 'museav') {
        progress(`正在查询中台任务 ${upstream.taskId}…`)
        const found = await museavJobResult(upstream.taskId)
        if (!found) throw new Error(`中台没有找到任务 ${upstream.taskId}；请先核对中台任务列表`)
        if (found.status && !['succeeded', 'done', 'completed'].includes(found.status)) throw new Error(`中台任务 ${upstream.taskId} 仍是「${found.status}」，尚未出片；过一会儿再试取回，不必重新提交`)
        if (!found.cdnUrl) throw new Error(`中台任务 ${upstream.taskId} 没有可下载的产物地址`)
        progress('上游已出片，正在下载…')
        const { bytes } = await safeDownload(found.cdnUrl)
        const probe = await probeBytes('video.mp4', bytes)
        return store.addAsset(input.projectId, 'MuseAV 视频.mp4', 'video/mp4', bytes, {
          source: 'MuseAV', origin: 'ai', engine: `MuseAV 出视频${upstream.model ? ' · ' + upstream.model : ''}`,
          prompt: input.prompt || '', requestedModel: upstream.model || '', actualModel: found.model || 'unknown',
          upstreamTaskId: found.id, ...probe,
        })
      }
      const target = loadCustomChannels().find(c => c.id === upstream.channel)
      if (!target) throw new Error(`未知的通道「${upstream.channel}」，无法继续取回`)
      if (!target.key) throw new Error(`通道 ${target.label} 缺少环境变量 ${target.env}，无法查询该任务`)
      progress(`正在查询 ${target.label} 任务 ${upstream.taskId}…`)
      const polled = await fetcher(`${target.baseUrl}${target.taskPath}/${encodeURIComponent(upstream.taskId)}`, {
        headers: { Authorization: `Bearer ${target.key}` }, signal: AbortSignal.timeout(30000),
      })
      const body = await polled.json().catch(() => ({}))
      if (!polled.ok) throw new Error(`自定义通道 HTTP ${polled.status}`)
      const status = body?.status || body?.data?.status
      if (['failed', 'cancelled', 'canceled'].includes(status)) throw new Error(`任务 ${upstream.taskId} 失败：${body?.error?.message || body?.error || status}`)
      if (status !== 'succeeded') throw new Error(`任务 ${upstream.taskId} 仍是「${status || '处理中'}」，尚未出片；过一会儿再试取回，不必重新提交`)
      const url = body?.content?.video_url || body?.data?.content?.video_url
      if (!url) throw new Error(`任务 ${upstream.taskId} 已完成，但未返回下载地址`)
      progress('上游已出片，正在下载…')
      const { bytes } = await safeDownload(url)
      const probe = await probeBytes('video.mp4', bytes)
      return store.addAsset(input.projectId, `${target.label} 视频.mp4`, 'video/mp4', bytes, {
        source: target.label, origin: 'ai', engine: `${target.label} 出视频 · ${upstream.model || target.model}`,
        prompt: input.prompt || '', requestedModel: upstream.model || target.model, actualModel: 'unknown',
        upstreamTaskId: String(upstream.taskId), ...probe,
      })
    },
    async status() {
      const [museav, ffmpeg, voice] = await Promise.all([
        available('museav'), available('ffmpeg'),
        Promise.all([jsonFetch(voiceBase + '/api/status', { signal: AbortSignal.timeout(3000) }), jsonFetch(voiceBase + '/api/personas', { signal: AbortSignal.timeout(3000) })]).catch(() => null),
      ])
      const quotas = await Promise.all(profiles.filter(p => p.key).map(async ({ id, label, key }) => {
        try {
          const response = await jsonFetch('https://www.minimaxi.com/v1/token_plan/remains', { headers: { Authorization: `Bearer ${key}` }, signal: AbortSignal.timeout(6000) })
          if (response.base_resp?.status_code) throw new Error('quota unavailable')
          const general = response.model_remains?.find(m => m.model_name === 'general')
          return { id, label, weeklyRemaining: general?.current_weekly_remaining_percent ?? null, checkedAt: new Date().toISOString() }
        } catch { return { id, label, weeklyRemaining: null } }
      }))
      const raw = voice?.[1]?.personas || []
      const personas = Array.isArray(raw) ? raw.map(p => ({ key: p.key, name: p.name || p.key })) : Object.entries(raw).map(([key, p]) => ({ key, name: p.name || key, hasSample: Boolean(p.has_audio || p.has_ref) }))
      // MuseAV 的档次清单随中台变动，直接问 CLI，不在 reel-kit 里写死一份
      const [museavModels, museavImageModels] = museav ? await Promise.all([listMuseavVideoModels(), listMuseavImageModels()]) : [[], []]
      const minimaxReady = profiles.some(p => p.key)
      const custom = loadCustomChannels()
      const customVideoChannels = custom.filter(c => c.kind === 'video')
      const customImageChannels = custom.filter(c => c.kind === 'image')
      const voiceLocalReady = Boolean(voice?.[0]?.base_model)

      const channel = (id, label, ready, note, extra = {}) => ({ id, label, ready, note, models: [], voices: [], allowCustomModel: false, allowCustomVoice: false, ...extra })

      return {
        video: {
          ready: minimaxReady || museav || customVideoChannels.some(c => c.key),
          label: '视频生成',
          note: '密钥配置不代表额度已验证；提交按所选账户计费',
          profiles: quotas,
          channels: [
            channel('museav', 'MuseAV 中台', Boolean(museav), museav ? '复用本机 CLI 登录态，按中台当前可用模型' : '请安装并登录 museav CLI', {
              models: museavModels.map(m => ({ id: m.name, name: m.limit ? `${m.name} · ${m.limit}` : m.name, limit: m.limit, minSeconds: m.minSeconds, maxSeconds: m.maxSeconds })),
              allowCustomModel: true,
            }),
            channel('minimax', 'MiniMax 直连', minimaxReady, '走自己的 MiniMax 套餐账户，不经中台', {
              models: [{ id: 'MiniMax-H3', name: 'MiniMax H3 · 4–15 秒 768P', minSeconds: 4, maxSeconds: 15 }],
              allowCustomModel: true,
            }),
            ...customVideoChannels.map(c => channel(c.id, c.label, Boolean(c.key), c.key ? `直连 ${c.baseUrl} · 按你自己的账户计费` : `缺少环境变量 ${c.env}`, {
              models: [{ id: c.model, name: c.model }],
              allowCustomModel: true,
            })),
          ],
        },
        image: {
          ready: Boolean(museav) || customImageChannels.some(c => c.key),
          label: '图片生成',
          note: museav ? '复用本机 CLI 登录态' : '请安装并登录 museav CLI',
          channels: [
            channel('museav', 'MuseAV 中台', Boolean(museav), museav ? '复用本机 CLI 登录态，按中台当前可用模型' : '请安装并登录 museav CLI', {
              models: museavImageModels.map(m => ({ id: m.value, name: m.value })),
              allowCustomModel: true,
            }),
            ...customImageChannels.map(c => channel(c.id, c.label, Boolean(c.key), c.key ? `直连 ${c.baseUrl} · 按你自己的账户计费` : `缺少环境变量 ${c.env}`, {
              models: [{ id: c.model, name: c.model }],
              allowCustomModel: true,
            })),
          ],
        },
        voice: {
          ready: voiceLocalReady || minimaxReady,
          label: '语音生成',
          note: voiceLocalReady
            ? '本地模型已就绪；云端通道按量计费'
            : voice
              ? '服务在线，Base 模型未就绪'
              : minimaxReady
                ? '本机语音服务没连上；云端配音可以用'
                : '本机语音服务没连上，云端配音也还没配密钥：到「设置与反馈 → 云端配音密钥」填一个，或启动 VoxFlow 服务后刷新',
          designReady: Boolean(voice?.[0]?.design_model),
          recordingReady: Boolean(voice?.[1]?.capabilities?.reference_text_registration),
          version: voice?.[1]?.version || 'unknown',
          personas,
          channels: [
            channel('voxflow', 'VoxFlow · 本地', voiceLocalReady, voiceLocalReady ? '本地模型已就绪，本地推理不产生模型费用' : '服务在线，Base 模型未就绪', {
              voices: personas.map(p => ({ id: p.key, name: p.name })),
            }),
            channel('minimax', 'MiniMax TTS', minimaxReady, minimaxReady
              ? '走自己的 MiniMax 套餐，按量计费；只做文字配音，不上传参考音频'
              : '还没配 MiniMax 密钥：到「设置与反馈 → 云端配音密钥」填一个（按量计费）', {
              models: MINIMAX_TTS_MODELS.map(m => ({ id: m.id, name: m.name })),
              voices: MINIMAX_VOICES.map(v => ({ id: v.id, name: v.name })),
              allowCustomModel: true,
              allowCustomVoice: true,
            }),
          ],
        },
        export: { ready: ffmpeg, label: 'reel-kit · 合成', note: ffmpeg ? 'FFmpeg 已就绪' : '请安装 FFmpeg' },
        customChannels: customChannelIssues(),
      }
    },
    async generate(input, progress) {
      const custom = loadCustomChannels()
      const customOf = id => custom.find(c => c.id === id)

      if (input.kind === 'image') {
        const channelId = input.backend || 'museav'
        if (channelId !== 'museav') {
          const target = customOf(channelId)
          if (!target || target.kind !== 'image' || target.protocol !== 'openai') throw new Error(`未知或不支持的图片通道「${channelId}」`)
          if (input.referenceId) throw new Error('此图片通道尚不支持参考图')
          if (!target.key) throw new Error(`通道 ${target.label} 缺少环境变量 ${target.env}，无法调用`)
          progress(`${target.label} 正在生成图片…`)
          const model = input.model || target.model
          const { bytes, mime } = await customImage({ channel: { ...target, model }, prompt: input.prompt, ratio: input.ratio })
          const probe = await probeBytes('image', bytes)
          return store.addAsset(input.projectId, `${target.label} 图片`, mime || 'image/png', bytes, {
            source: target.label, origin: 'ai', engine: `${target.label} 生图 · ${model}`,
            prompt: input.prompt, requestedModel: model, actualModel: 'unknown', ...probe,
          })
        }
        // 图片走 MuseAV 中台。模型可以自填 —— 中台上新一个档次时不用等 reel-kit 发版。
        progress('MuseAV 正在生成图片…')
        const args = ['gen', '--prompt', input.prompt, '--ratio', input.ratio]
        if (input.model) args.push('--model', input.model)
        if (input.referenceId) {
          const asset = store.asset(input.referenceId, input.projectId)
          if (!asset.mime.startsWith('image/')) throw new Error('图片参考必须是图片')
          args.push('--ref', store.path(asset))
        }
        const { stdout, stderr } = await commandFull('museav', args, { timeout: 900000 })
        progress('正在下载图片到本地…')
        const url = outputUrl(stdout)
        const { bytes, mime } = await safeDownload(url)
        const probe = await probeBytes('image', bytes)
        const receipt = await museavReceipt({ cdnUrl: url })
        return store.addAsset(input.projectId, 'MuseAV 图片', mime || 'image/png', bytes, {
          source: 'MuseAV', origin: 'ai', engine: `MuseAV 生图${input.model ? ' · ' + input.model : ''}`,
          prompt: input.prompt, requestedModel: input.model || '', actualModel: receipt.actualModel,
          upstreamTaskId: receipt.id, ...probe,
        })
      }

      if (input.kind === 'video') {
        const channelId = input.backend || 'minimax'
        if (channelId === 'museav') {
          // 走 MuseAV 中台出视频：一条链路同时能调 Seedance / Kling / Wan 等多个模型，
          // 不必每换一个模型就往 reel-kit 里加一条直连实现。
          progress('MuseAV 正在提交视频任务…')
          const args = ['gen', '--video', '--prompt', input.prompt, '--ratio', input.ratio]
          if (input.duration) args.push('--duration', String(input.duration))
          if (input.model) args.push('--model', input.model)
          if (input.referenceId) {
            const asset = store.asset(input.referenceId, input.projectId)
            if (!asset.mime.startsWith('image/')) throw new Error('视频参考必须是图片')
            args.push('--reference-image', store.path(asset))
          }
          // 任务 ID 在 stderr 里一提交就会打印。边跑边存：
          // 中途超时或被杀时，用户至少能拿同一个 ID 去把已生成的片子取回来，不必重新提交计费。
          let stderrTail = ''
          const { stdout, stderr } = await commandFull('museav', args, {
            timeout: 1800000,
            onStderr: chunk => {
              stderrTail = (stderrTail + chunk).slice(-12000)
              const id = museavJobId(stderrTail)
              if (id) progress(`MuseAV 任务 ${id} 已提交，等待生成…`, { upstream: { channel: 'museav', kind: 'video', taskId: id, model: input.model || '' } })
            },
          })
          progress('MuseAV 已完成，正在下载视频…')
          const url = outputUrl(stdout)
          const jobId = museavJobId(stderr || stderrTail)
          let bytes
          try { ({ bytes } = await safeDownload(url)) }
          catch (error) { throw new Error(`${error.message}${jobId ? `（中台任务 ${jobId} 已生成并计费，素材还在上游；重新提交会重复计费）` : ''}`) }
          const probe = await probeBytes('video.mp4', bytes)
          const receipt = await museavReceipt({ jobId, cdnUrl: url })
          return store.addAsset(input.projectId, 'MuseAV 视频.mp4', 'video/mp4', bytes, {
            source: 'MuseAV', origin: 'ai', engine: `MuseAV 出视频${input.model ? ' · ' + input.model : ''}`,
            prompt: input.prompt, requestedModel: input.model || '', actualModel: receipt.actualModel,
            upstreamTaskId: receipt.id, ...probe,
          })
        }
        if (channelId !== 'minimax') {
          const target = customOf(channelId)
          if (!target || target.kind !== 'video' || target.protocol !== 'ark-task') throw new Error(`未知或不支持的视频通道「${channelId}」`)
          if (input.referenceId) throw new Error('此视频通道尚不支持参考图')
          if (!target.key) throw new Error(`通道 ${target.label} 缺少环境变量 ${target.env}，无法调用`)
          progress(`${target.label} 正在提交视频任务…`)
          const model = input.model || target.model
          const { bytes } = await customVideo({ channel: { ...target, model }, prompt: input.prompt, ratio: input.ratio, duration: input.duration, onSubmitted: taskId => progress(`${target.label} 任务 ${taskId} 已提交，等待生成…`, { upstream: { channel: channelId, kind: 'video', taskId, model } }) })
          const probe = await probeBytes('video.mp4', bytes)
          return store.addAsset(input.projectId, `${target.label} 视频.mp4`, 'video/mp4', bytes, {
            source: target.label, origin: 'ai', engine: `${target.label} 出视频 · ${model}`,
            prompt: input.prompt, requestedModel: model, actualModel: 'unknown', ...probe,
          })
        }
        const profile = profiles.find(p => p.id === (input.profile || 'primary'))
        if (!profile?.key) throw new Error('所选 MiniMax 账户未配置。使用密钥工具注入 MINIMAX_API_KEY / MINIMAX_API_KEY_2 后启动工作台')
        let reference
        if (input.referenceId) {
          const asset = store.asset(input.referenceId, input.projectId)
          if (!asset.mime.startsWith('image/')) throw new Error('H3 人物参考须选择图片')
          progress('正在上传参考图供 H3 读取…')
          reference = outputUrl(await command('museav', ['upload', store.path(asset)]))
        }
        progress('正在提交 MiniMax H3…')
        const response = await h3('/v2/video_generation', profile.key, h3Body(input, reference))
        const taskId = response.task_id || response.task?.id
        if (!taskId) throw new Error('MiniMax 未返回任务 ID；请先核对上游，不要重复提交')
        // 任务 ID 一拿到就记下来：提交即计费，后面下载失败不该再提交第二次
        progress(`H3 任务 ${taskId} 已提交，等待生成…`, { upstream: { channel: 'minimax', kind: 'video', taskId: String(taskId), model: input.model || 'MiniMax-H3', profile: profile.id } })
        for (let attempt = 0; attempt < 180; attempt++) {
          await pause(5000)
          const result = await h3(`/v2/query/video_generation/${encodeURIComponent(taskId)}`, profile.key)
          const task = result.task || result
          if (['failed', 'cancelled', 'canceled'].includes(task.status)) throw new Error(`H3 任务 ${taskId} 失败：${task.error?.message || task.error || task.status}`)
          if (task.status === 'succeeded') {
            const url = task.content?.url
            if (!url) throw new Error(`H3 任务 ${taskId} 已完成，但未返回下载地址`)
            progress('H3 已完成，正在下载视频…')
            let bytes
            try { ({ bytes } = await safeDownload(url)) }
            catch (error) { throw new Error(`${error.message}（上游任务 ${taskId} 已生成并计费，素材还在上游；重新提交会重复计费）`) }
            const probe = await probeBytes('video.mp4', bytes)
            return store.addAsset(input.projectId, 'H3 视频.mp4', 'video/mp4', bytes, {
              source: 'MiniMax H3', origin: 'ai', engine: `MiniMax H3 视频${input.model && input.model !== 'MiniMax-H3' ? ' · ' + input.model : ''}`,
              upstreamTaskId: String(taskId), prompt: input.prompt, profile: profile.id,
              requestedModel: input.model || 'MiniMax-H3',
              // 上游任务回执里带着实际跑的模型 —— 有回执就用回执，没有才写 unknown
              actualModel: task.model || 'unknown', ...probe,
            })
          }
        }
        throw new Error(`H3 任务 ${taskId} 仍在处理中，已停止轮询；请核对上游后再操作`)
      }

      if (['speech', 'clone', 'voice-design'].includes(input.kind)) {
        const channelId = input.backend || 'voxflow'
        if (channelId === 'minimax') {
          if (input.kind !== 'speech') throw new Error('MiniMax TTS 只支持文字配音；录音克隆与音色设计请用本地 VoxFlow')
          const profile = profiles.find(p => p.id === (input.profile || 'primary'))
          if (!profile?.key) throw new Error('所选 MiniMax 账户未配置：在应用「设置与反馈 → 云端配音密钥」里填 MiniMax API Key，或用密钥工具注入 MINIMAX_API_KEY 后启动工作台')
          // 音色可以从 voice_id 来，也可以从 persona 来 ——
          // 桌面版只有「音色」一个字段，两条通道的值都存在那里。
          const voiceId = input.voiceId || input.persona
          if (!voiceId) throw new Error('请选择音色（voice_id），或手动填写')
          const model = input.model || MINIMAX_TTS_MODELS[0].id
          progress('MiniMax 正在合成配音…')
          const { bytes, mime } = await minimaxTts({ key: profile.key, text: input.prompt, model, voiceId })
          const probe = await probeBytes('voice.mp3', bytes)
          return store.addAsset(input.projectId, `配音 · ${input.prompt.slice(0, 16)}.mp3`, mime || 'audio/mpeg', bytes, {
            source: 'MiniMax TTS', origin: 'ai', engine: `MiniMax TTS · ${model}`, voiceBackend: 'minimax',
            prompt: input.prompt, persona: voiceId, tone: input.tone || '', voiceName: input.voiceName || '',
            requestedModel: model, actualModel: 'unknown', profile: profile.id, ...probe,
          })
        }
        if (channelId !== 'voxflow') throw new Error(`未知的语音通道「${channelId}」`)
        if (input.prompt.length > 400) throw new Error('VoxFlow 单次配音上限 400 字，请分段生成')
        let persona = input.persona
        if (input.kind === 'clone' && !persona) {
          const asset = store.asset(input.referenceId, input.projectId)
          progress('正在把参考音频与原文保存到全局声音库…')
          const registered = await registerVoice({ bytes: readFileSync(store.path(asset)), mime: asset.mime, name: input.voiceName, referenceText: input.referenceText })
          persona = registered.persona
          // Persist the key before synthesis so retrying a failed job reuses the voice.
          input.persona = persona; store.save()
        }
        const designing = input.kind === 'voice-design'
        if (!designing && !persona) throw new Error('请选择已有音色，或创建新音色')
        progress(designing ? '正在设计音色，首次生成需要加载模型…' : 'VoxFlow 正在合成配音…')
        const payload = designing ? { voice_name: input.voiceName, text: input.prompt, tone: input.tone, emotion: input.emotion || '', commit: true } : { persona, text: input.prompt, tone: input.tone || '' }
        const submitted = await jsonFetch(voiceBase + (designing ? '/api/design' : '/api/clone'), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
        for (let attempt = 0; attempt < 300; attempt++) {
          await pause(2000)
          const { tasks } = await jsonFetch(voiceBase + '/api/tasks')
          const task = tasks.find(t => t.id === submitted.task_id)
          if (!task) continue
          if (['error', 'failed', 'cancelled'].includes(task.status)) throw new Error(task.error || '本地语音生成失败')
          if (task.status === 'done') {
            const result = task.result
            if (!result?.url?.startsWith('/api/audio/')) throw new Error('VoxFlow 未返回音频文件')
            const { bytes } = await safeDownload(new URL(result.url, voiceBase))
            const probe = await probeBytes('voice.wav', bytes)
            return store.addAsset(input.projectId, designing ? `${input.voiceName} · 样音.wav` : `配音 · ${input.prompt.slice(0, 16)}.wav`, 'audio/wav', bytes, { source: 'VoxFlow', origin: 'ai', engine: 'VoxFlow 本地语音', voiceBackend: 'voxflow', prompt: input.prompt, persona: result.persona_key || persona, tone: input.tone || '', voiceName: input.voiceName || '', requestedModel: 'voxflow-local', actualModel: 'unknown', ...probe })
          }
        }
        throw new Error('语音生成超时，请检查 VoxFlow 任务列表')
      }
      throw new Error('未知生成类型')
    },
  }
}
