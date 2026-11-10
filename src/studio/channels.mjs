/**
 * 自定义模型通道：配置的读写、校验与探活。
 *
 * 单独成文件，是因为通道配置有三种填法（用户自己在设置页填、Agent 代填、
 * 手改 JSON），三处都要用同一套解析与校验 —— 不能各写一份。
 *
 * 三件事的边界：
 *   配置（这里）  —— 通道长什么样、放哪儿、哪些字段必填
 *   调用（providers.mjs）—— 真正发请求拿素材
 *   探活（这里）  —— 发一次**只读**的请求，判断通道能不能用
 *
 * 探活只读不写：付费通道一提交就计费，
 * 「检查一下能不能用」不该让用户花钱。
 */
import { mkdirSync, readFileSync, renameSync, unlinkSync, writeFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'

export const CUSTOM_CONFIG = () => (process.env.REEL_CHANNELS_FILE || '').trim() || join(homedir(), '.reel-kit', 'providers.json')

/** 本机配置（密钥落盘处）—— 桌面版写，后端读 */
export const REEL_CONFIG = () => join(homedir(), '.reel-kit', 'config.json')

/** 内置通道的 id：自定义通道不能占用，否则两套实现会打架 */
export const RESERVED_IDS = ['museav', 'minimax', 'sherpa', 'voxflow']

/** 生图通道支持的协议 */
export const IMAGE_PROTOCOLS = ['openai', 'minimax-image', 'museav']
/** 出视频通道支持的协议 */
export const VIDEO_PROTOCOLS = ['ark-task', 'museav']

/** 配置里不该出现的字段：密钥只走环境变量或本机配置的 channelKeys */
const SECRET_FIELDS = ['key', 'apiKey', 'api_key', 'token', 'secret', 'password', 'authorization']

const DEFAULTS = {
  openai: { imagePath: '/images/generations', taskPath: '/contents/generations/tasks' },
  'minimax-image': { imagePath: '/v1/image_generation' },
  'ark-task': { taskPath: '/contents/generations/tasks' },
  museav: { imagePath: '/api/generate-batch', taskPath: '/api/videos', imageJobPath: '/api/jobs', videoJobPath: '/api/videos' },
}

/**
 * 读通道配置文件。
 *
 * 文件不存在返回空数组；**文件存在但结构不对就报错** ——
 * 静默当成空数组的话，下一次保存就把用户原来的配置覆盖掉了。
 */
export function readChannelConfig() {
  let raw
  try {
    raw = JSON.parse(readFileSync(CUSTOM_CONFIG(), 'utf8'))
  } catch (error) {
    if (error.code === 'ENOENT') return []
    throw new Error(`通道配置文件无法解析：${error.message}`)
  }
  const list = Array.isArray(raw) ? raw : (raw?.channels || [])
  if (!Array.isArray(list)) throw new Error('通道配置文件损坏：channels 必须是数组')
  return list
}

/** 剥掉不该进配置文件的字段（密钥与任何看着像密钥的键） */
function stripSecrets(entry) {
  const out = {}
  for (const [k, v] of Object.entries(entry || {})) {
    if (SECRET_FIELDS.includes(k)) continue
    out[k] = v
  }
  return out
}

/**
 * 写通道配置文件。**只写通道配置，不写密钥** —— 密钥永远走环境变量或本机配置。
 *
 * 目录 0700、文件 0600：密钥所在的目录不该被同机其他账号读到。
 * 写文件用「临时文件 + 改名」：中途断电不会留下半个文件。
 */
export function writeChannelConfig(list) {
  const file = CUSTOM_CONFIG()
  mkdirSync(dirname(file), { recursive: true, mode: 0o700 })
  const safe = (Array.isArray(list) ? list : []).map(stripSecrets)
  const tmp = `${file}.tmp`
  writeFileSync(tmp, `${JSON.stringify({ channels: safe }, null, 2)}\n`, { mode: 0o600 })
  try {
    renameSync(tmp, file)
  } catch (error) {
    try { unlinkSync(tmp) } catch { /* 临时文件删不掉就算了，别盖住真正的错 */ }
    throw error
  }
  return file
}

/**
 * 校验接口地址。
 *
 * 密钥会跟着这个地址走，所以只放行 https（本地 127.0.0.1 除外），
 * 也不许地址里嵌着账号密码 —— 那种地址会出现在日志和错误信息里。
 */
export function validateBaseUrl(raw) {
  const value = String(raw || '').trim().replace(/\/+$/, '')
  if (!value) return { ok: false, reason: '缺少 baseUrl' }
  let url
  try {
    url = new URL(value)
  } catch {
    return { ok: false, reason: `地址不是合法 URL：${value}` }
  }
  if (url.username || url.password) return { ok: false, reason: '地址里不能嵌账号密码，密钥单独填' }
  if (url.search || url.hash) return { ok: false, reason: '地址里不能带查询参数或锚点，只填根地址' }
  const local = url.hostname === '127.0.0.1' || url.hostname === 'localhost' || url.hostname === '::1'
  if (url.protocol !== 'https:' && !(local && url.protocol === 'http:')) {
    return { ok: false, reason: `地址必须用 https（本地调试可以用 http）：${value}` }
  }
  return { ok: true, value }
}

/** 环境变量名要合法，否则注入不进去 */
export function validateEnvName(raw) {
  const value = String(raw || '').trim()
  if (!value) return { ok: false, reason: '缺少 env（环境变量名）' }
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(value)) {
    return { ok: false, reason: `环境变量名不合法：${value}（只能用字母、数字、下划线，且不能以数字开头）` }
  }
  return { ok: true, value }
}

/**
 * 已经有存过的密钥，认这些环境变量名。
 *
 * 同一个厂商不该让用户填两遍：用户已经在「云端配音」里填过 MiniMax 密钥，
 * 加一条 MiniMax 生图通道时不该再要一遍。
 */
const KEY_ALIASES = {
  MINIMAX_API_KEY: 'minimaxApiKey',
  MINIMAX_API_KEY_2: 'minimaxApiKey2',
}

/** 读本机配置（密钥落盘处） */
function readReelConfig() {
  try {
    const raw = JSON.parse(readFileSync(REEL_CONFIG(), 'utf8'))
    return raw && typeof raw === 'object' && !Array.isArray(raw) ? raw : {}
  } catch {
    return {}
  }
}

/** 读本机配置里的通道密钥（桌面版在设置页填的） */
export function readReelChannelKeys() {
  const keys = readReelConfig()?.channelKeys
  if (!keys || typeof keys !== 'object' || Array.isArray(keys)) return {}
  const out = {}
  for (const [id, value] of Object.entries(keys)) {
    const text = String(value || '').trim()
    if (text) out[id] = text
  }
  return out
}

/**
 * 找这条通道的密钥。
 *
 * 优先级：环境变量 → 设置页存的本条通道密钥 → 同厂商已存过的密钥。
 *
 * 环境变量优先，其次是桌面设置页存进本机配置的 —— 后者是为了
 * 「下载下来直接用」：普通用户不该被要求先开终端 export 一个变量。
 */
export function resolveChannelKey(channel, savedKeys) {
  const fromEnv = channel.env ? (process.env[channel.env] || '').trim() : ''
  if (fromEnv) return { key: fromEnv, source: 'env' }
  const saved = savedKeys ? savedKeys[channel.id] : ''
  if (saved) return { key: String(saved).trim(), source: 'config' }
  const alias = KEY_ALIASES[channel.env]
  if (alias) {
    const reused = String(readReelConfig()?.[alias] || '').trim()
    if (reused) return { key: reused, source: 'config' }
  }
  return { key: '', source: 'none' }
}

/**
 * 把一条原始配置归一化成通道对象。
 *
 * 缺关键项的直接判为不可用（返回 null）：宁可界面上不出现，
 * 也不给一个必然失败的选项。
 */
export function normalizeChannel(item, savedKeys) {
  const id = String(item?.id || '').trim()
  const model = String(item?.model || '').trim()
  const env = String(item?.env || '').trim()
  // kind 只认 image / video：自定义 TTS 还没实现，
  // 把它悄悄降级成生图通道比直接拒绝更糟 —— 会生成出错的图。
  const rawKind = String(item?.kind || 'image').trim()
  if (!['image', 'video'].includes(rawKind)) return null
  const kind = rawKind
  const rawProtocol = String(item?.protocol || (kind === 'video' ? 'ark-task' : 'openai')).trim()
  const protocol = ['openai', 'ark-task', 'minimax-image', 'museav'].includes(rawProtocol) ? rawProtocol : ''
  if (!id || !env) return null
  if (RESERVED_IDS.includes(id)) return null
  const url = validateBaseUrl(item?.baseUrl)
  if (!url.ok) return null
  const envCheck = validateEnvName(env)
  if (!envCheck.ok) return null
  // 中台自己挑模型，model 可以不给；直连厂商必须写明模型名
  if (!model && protocol !== 'museav') return null
  if (kind === 'video' && !VIDEO_PROTOCOLS.includes(protocol)) return null
  if (kind === 'image' && !IMAGE_PROTOCOLS.includes(protocol)) return null
  const defaults = DEFAULTS[protocol] || DEFAULTS.openai
  const channel = {
    id,
    label: String(item?.label || id).trim(),
    kind,
    protocol,
    baseUrl: url.value,
    model,
    env,
    auth: item?.auth === 'api-key' ? 'api-key' : 'bearer',
    imagePath: String(item?.imagePath || defaults.imagePath || DEFAULTS.openai.imagePath).trim(),
    taskPath: String(item?.taskPath || defaults.taskPath || DEFAULTS.openai.taskPath).trim(),
    // 中台的提交与查询不是同一个端点：出图查 /api/jobs，出视频查 /api/videos
    imageJobPath: String(item?.imageJobPath || defaults.imageJobPath || '/api/jobs').trim(),
    videoJobPath: String(item?.videoJobPath || defaults.videoJobPath || '/api/videos').trim(),
    voiceId: String(item?.voiceId || '').trim(),
  }
  const resolved = resolveChannelKey(channel, savedKeys)
  channel.key = resolved.key
  channel.keySource = resolved.source
  return channel
}

/** 当前可用的自定义通道 */
export function loadCustomChannels() {
  let list
  try { list = readChannelConfig() } catch { return [] }
  const savedKeys = readReelChannelKeys()
  const seen = new Set()
  const out = []
  for (const item of list) {
    const channel = normalizeChannel(item, savedKeys)
    // 重复 id 只留第一条：两套实现抢同一个 id，后一次会静默盖掉前一次
    if (!channel || seen.has(channel.id)) continue
    seen.add(channel.id)
    out.push(channel)
  }
  return out
}

/**
 * 配置的问题清单。
 *
 * 配置错了却一声不吭，用户只会看到「这个通道不存在」——
 * 必须把原因摆出来：哪一条、哪一项、为什么不能用。
 */
export function customChannelIssues() {
  let raw
  try {
    raw = JSON.parse(readFileSync(CUSTOM_CONFIG(), 'utf8'))
  } catch (error) {
    return { file: CUSTOM_CONFIG(), problems: error.code === 'ENOENT' ? [] : [`配置文件无法解析：${error.message}`] }
  }
  const list = Array.isArray(raw) ? raw : (raw?.channels || [])
  if (!Array.isArray(list)) return { file: CUSTOM_CONFIG(), problems: ['channels 必须是数组'] }
  const problems = []
  const seen = new Set()
  list.forEach((item, index) => {
    const id = String(item?.id || '').trim()
    const label = id || `第 ${index + 1} 条`
    if (!id) problems.push(`${label}：缺少 id`)
    else if (seen.has(id)) problems.push(`${label}：id 重复`)
    else seen.add(id)
    if (RESERVED_IDS.includes(id)) problems.push(`${label}：id 是内置通道保留名，换一个（${RESERVED_IDS.join(' / ')}）`)
    const url = validateBaseUrl(item?.baseUrl)
    if (!url.ok) problems.push(`${label}：${url.reason}`)
    if (!String(item?.model || '').trim() && item?.protocol !== 'museav') problems.push(`${label}：缺少 model`)
    const env = validateEnvName(item?.env)
    if (!env.ok) problems.push(`${label}：${env.reason}`)
    else {
      const resolved = resolveChannelKey({ id, env: env.value }, readReelChannelKeys())
      if (!resolved.key) problems.push(`${label}：还没填密钥（环境变量 ${env.value} 或设置页里的密钥）`)
    }
    if (item?.kind && !['image', 'video', 'tts'].includes(item.kind)) problems.push(`${label}：kind 只支持 image / video / tts`)
    if (item?.kind === 'tts') problems.push(`${label}：自定义 TTS 通道尚未实现，请改用 MiniMax TTS 或本地声音引擎`)
    const kind = item?.kind === 'video' ? 'video' : 'image'
    const protocol = String(item?.protocol || '').trim()
    if (protocol && kind === 'video' && !VIDEO_PROTOCOLS.includes(protocol)) problems.push(`${label}：视频通道协议只支持 ${VIDEO_PROTOCOLS.join(' / ')}`)
    if (protocol && kind === 'image' && !IMAGE_PROTOCOLS.includes(protocol)) problems.push(`${label}：生图通道协议只支持 ${IMAGE_PROTOCOLS.join(' / ')}`)
  })
  return { file: CUSTOM_CONFIG(), problems }
}

/** 新增或更新一条通道（按 id 覆盖） */
export function upsertChannel(entry) {
  const list = readChannelConfig()
  const normalized = normalizeChannel(entry, readReelChannelKeys())
  if (!normalized) throw new Error('这条通道配置不完整：id、baseUrl、env 必填（museav 协议除外 model）')
  const withoutKey = stripSecrets(entry)
  const index = list.findIndex(item => String(item?.id || '').trim() === normalized.id)
  if (index >= 0) list[index] = { ...stripSecrets(list[index]), ...withoutKey }
  else list.push(withoutKey)
  const file = writeChannelConfig(list)
  return { file, channel: normalized }
}

/** 删掉一条通道 */
export function removeChannel(id) {
  const list = readChannelConfig()
  const kept = list.filter(item => String(item?.id || '').trim() !== String(id || '').trim())
  if (kept.length === list.length) throw new Error(`没有 id 为「${id}」的通道`)
  const file = writeChannelConfig(kept)
  return { file, removed: id }
}

/** 中台的鉴权头：个人登录是 Bearer，租户是 X-API-Key */
export function channelHeaders(channel, extra = {}) {
  const headers = { 'Content-Type': 'application/json', ...extra }
  if (channel.auth === 'api-key') headers['X-API-Key'] = channel.key
  else headers.Authorization = `Bearer ${channel.key}`
  return headers
}

/**
 * 探活：判断这条通道能不能用。
 *
 * 每段单独报，用户看得见到底卡在哪 —— 而不是笼统一句「通道不可用」。
 *
 * 每条检查有三种结果，别混为一谈：
 *   ok=true   —— 这一项验证通过了
 *   ok=false  —— 这一项验证失败了
 *   ok=null   —— 上游没给得出能判断的信息，**未验证**（不是「能用」）
 *
 * 探活只发只读或必然被拒的请求，绝不生成内容。
 */
export async function probeChannel(channel, { fetcher = fetch } = {}) {
  const checks = []
  const add = (name, ok, detail) => checks.push({ name, ok, detail })
  if (!channel.baseUrl) add('配置', false, '缺少 baseUrl')
  if (!channel.env) add('配置', false, '缺少 env（环境变量名）')
  else if (!channel.key) add('配置', false, `还没填密钥：设置环境变量 ${channel.env}，或在设置页把密钥填进来`)
  if (channel.key && !channel.model && channel.protocol !== 'museav') add('配置', false, '缺少 model')
  if (checks.some(c => c.ok === false)) return { ok: false, checks }
  const source = channel.keySource === 'env' ? `环境变量 ${channel.env}` : '设置页保存的密钥'
  add('配置', true, `baseUrl、模型与密钥（${source}）都就位`)

  try {
    if (channel.protocol === 'museav') return await probeMuseav(channel, fetcher, add, checks)
    if (channel.protocol === 'ark-task') return await probeArkTask(channel, fetcher, add, checks)
    return await probeImageLike(channel, fetcher, add, checks)
  } catch (error) {
    add('连通性', false, `请求没打通：${error?.cause?.message || error?.message || '未知原因'}`)
    return { ok: false, checks }
  }
}

/** 中台：读一次任务列表。只读，不提交任何任务，个人 token 与租户 apiKey 都适用 */
async function probeMuseav(channel, fetcher, add, checks) {
  const response = await fetcher(`${channel.baseUrl}/api/jobs`, {
    headers: channelHeaders(channel), signal: AbortSignal.timeout(15000),
  })
  const body = await response.json().catch(() => ({}))
  if (response.status === 401 || response.status === 403) {
    add('连通性', true, `地址 ${channel.baseUrl} 可达`)
    add('鉴权', false, `中台拒绝（HTTP ${response.status}）：密钥无效、没权限，或该端点只认个人登录`)
    return { ok: false, checks }
  }
  if (response.status === 404) {
    add('连通性', false, `中台没有 /api/jobs（HTTP 404）：检查 baseUrl 是否是中台根地址`)
    return { ok: false, checks }
  }
  if (!response.ok) {
    add('连通性', false, `中台返回 HTTP ${response.status}${body?.error?.message ? `：${body.error.message}` : ''}`)
    return { ok: false, checks }
  }
  add('连通性', true, `地址 ${channel.baseUrl} 可达`)
  add('鉴权', true, '中台认了这把密钥（读任务列表成功）')
  // 读到任务列表只证明读接口通，不证明出图/出视频的协议形状对得上。
  add('协议', null, '没有提交任务的读接口，无法验证出图/出视频的请求形状；真跑一次才知道')
  return { ok: checks.every(c => c.ok !== false), checks }
}

/** 任务式视频接口：上游一般没有「只读」端点，用一次空提交探边界（不创建任务） */
async function probeArkTask(channel, fetcher, add, checks) {
  const response = await fetcher(`${channel.baseUrl}${channel.taskPath}`, {
    method: 'POST',
    headers: channelHeaders(channel),
    body: JSON.stringify({}),
    signal: AbortSignal.timeout(15000),
  })
  const body = await response.json().catch(() => ({}))
  if (response.status === 401 || response.status === 403) {
    add('连通性', true, `地址 ${channel.baseUrl} 可达`)
    add('鉴权', false, `上游拒绝（HTTP ${response.status}）：密钥无效或没权限`)
    return { ok: false, checks }
  }
  const message = body?.error?.message || body?.error || body?.message || ''
  if (response.status === 404) {
    add('连通性', false, `路径不存在（HTTP 404）：检查 taskPath（当前 ${channel.taskPath}）`)
    return { ok: false, checks }
  }
  add('连通性', true, `地址 ${channel.baseUrl} 可达`)
  add('鉴权', true, '上游没有拒绝这把密钥')
  // 上游按参数校验回话，说明地址、鉴权与请求形状都对得上；
  // 但没真提交过任务，所以「能不能出片」仍然未验证。
  add('协议', true, `上游按参数校验回话（${String(message).slice(0, 80) || '空请求被拒'}），说明地址与密钥都通`)
  add('能力', null, '没有真提交任务，模型能不能出片还没验证')
  return { ok: checks.every(c => c.ok !== false), checks }
}

/** 一次 POST 形状的生图接口：空提示词必然被拒，但足以证明地址与密钥通 */
async function probeImageLike(channel, fetcher, add, checks) {
  const body = channel.protocol === 'minimax-image'
    ? { model: channel.model, prompt: '' }
    : { model: channel.model, prompt: '', size: '1024x1024' }
  const response = await fetcher(channel.baseUrl + channel.imagePath, {
    method: 'POST',
    headers: channelHeaders(channel),
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(20000),
  })
  const result = await response.json().catch(() => ({}))
  const message = String(result?.base_resp?.status_msg || result?.error?.message || result?.error || result?.message || '')
  if (response.status === 401 || response.status === 403) {
    add('连通性', true, `地址 ${channel.baseUrl} 可达`)
    add('鉴权', false, `上游拒绝（HTTP ${response.status}）：密钥无效或没权限`)
    return { ok: false, checks }
  }
  // 模型不认的报错要排在 404 前面判：
  // 上游常常用 404 说「这个模型不存在」，那不是路径写错。
  if (/model.*not|not.*support.*model|unsupported model|模型.*不存在|无此模型/i.test(message)) {
    add('连通性', true, `地址 ${channel.baseUrl} 可达`)
    add('鉴权', true, '上游没有拒绝这把密钥')
    add('模型', false, `模型名上游不认：${message.slice(0, 120)}`)
    return { ok: false, checks }
  }
  if (response.status === 404) {
    add('连通性', false, `路径不存在（HTTP 404）：检查 imagePath（当前 ${channel.imagePath}）`)
    return { ok: false, checks }
  }
  // 业务错误也可能落在 HTTP 200 上（MiniMax 就是这样），所以不能只看 status。
  const businessCode = result?.base_resp?.status_code
  const authRejected = /api key|unauthor|invalid.*key|鉴权|认证|token|额度|余额/i.test(message)
  if (authRejected && businessCode) {
    add('连通性', true, `地址 ${channel.baseUrl} 可达`)
    add('鉴权', false, `上游拒绝了请求（${businessCode}）：${message.slice(0, 120)}`)
    return { ok: false, checks }
  }
  if (!response.ok && !message) {
    add('连通性', false, `上游返回 HTTP ${response.status}`)
    return { ok: false, checks }
  }
  add('连通性', true, `地址 ${channel.baseUrl} 可达`)
  add('鉴权', true, '上游没有拒绝这把密钥')
  // 模型名是否被认，只能靠上游明说。空提示词被拒**不能**证明模型对 ——
  // 多数接口先查提示词再查模型，这时回「提示词为空」跟模型名对不对没关系。
  if (/model.*not|not.*support.*model|unsupported model|模型.*不存在|无此模型/i.test(message)) {
    add('模型', false, `模型名上游不认：${message.slice(0, 120)}`)
    return { ok: false, checks }
  }
  add('模型', null, `上游只回了「${message.slice(0, 80) || '空请求被拒'}」，没说到模型名，模型是否可用未验证`)
  add('协议', true, `接口按参数校验回话（${message.slice(0, 80) || '空请求被拒'}），说明请求形状对得上`)
  return { ok: checks.every(c => c.ok !== false), checks }
}
