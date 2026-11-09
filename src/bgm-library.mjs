/**
 * 配乐库 —— 按别名取曲子，不写死路径。
 *
 * ## 库在哪：由你指定，本工具不自带
 *
 * reel-kit 不内置任何配乐（也不该 —— 音乐授权是使用者自己的事）。要用别名取曲子，
 * 先指一个清单，两种方式任选：
 *
 *   export REEL_BGM_MANIFEST=https://你的域名/music-manifest.json   # 远端
 *   export REEL_BGM_MANIFEST=~/music/manifest.json                  # 本地文件
 *
 * 或写进 `~/.reel-kit/config.json`（配一次长期生效）：
 *
 *   { "bgmManifest": "https://你的域名/music-manifest.json" }
 *
 * 没配置也完全能用 —— `--bgm ./某文件.mp3` 直接给路径，不经过这里。
 *
 * ## 清单长什么样
 *
 * ```json
 * {
 *   "cdn": "https://你的域名/",           // 可选：key 是相对路径时用它拼绝对地址
 *   "bgm": [
 *     {
 *       "alias": "轻快",                  // --bgm 用这个名字取
 *       "key": "bgm/upbeat.mp3",          // 相对 cdn，或直接写完整 URL / 本地绝对路径
 *       "duration": 113,
 *       "mood": ["轻快", "日常"],
 *       "license": "CC0 / 免署名可商用",   // 强烈建议填：见下
 *       "source": "曲子从哪来的"
 *     }
 *   ]
 * }
 * ```
 *
 * `license` 与 `source` 不是装饰。片子是要对外发的，「这首能不能商用、要不要署名」
 * 必须在出片那一刻看得见 —— 所以只要清单里填了，每次取用都会打印出来。
 * 等发出去了再翻记录找来源就晚了。
 */
import { existsSync, mkdirSync, createWriteStream } from 'node:fs'
import { readFile, writeFile, unlink, rename } from 'node:fs/promises'
import { createHash, randomUUID } from 'node:crypto'
import { homedir } from 'node:os'
import { isAbsolute, join, resolve, basename } from 'node:path'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'

const HOME_DIR = join(homedir(), '.reel-kit')
const CACHE_DIR = join(HOME_DIR, 'bgm')
const CONFIG_FILE = join(HOME_DIR, 'config.json')
const MANIFEST_CACHE = join(CACHE_DIR, 'manifest.json')
// 清单很小且不常变，缓存一天足够；过期只是重新拉一次，拉不到还会退回旧缓存
const MANIFEST_TTL_MS = 24 * 60 * 60 * 1000

/** 展开 ~ 开头的路径 —— 环境变量里写 ~ 是很自然的事，shell 不总会帮你展开 */
function expandHome(p) {
  return p.startsWith('~/') ? join(homedir(), p.slice(2)) : p
}

/** 配乐清单的位置：环境变量优先（CI / 临时切换），其次 ~/.reel-kit/config.json */
async function manifestLocation() {
  if (process.env.REEL_BGM_MANIFEST) return expandHome(process.env.REEL_BGM_MANIFEST.trim())
  try {
    const cfg = JSON.parse(await readFile(CONFIG_FILE, 'utf8'))
    if (cfg.bgmManifest) return expandHome(String(cfg.bgmManifest).trim())
  } catch { /* 没有配置文件是正常情况 */ }
  return null
}

const NOT_CONFIGURED =
  '没有配置配乐库，所以用不了别名。两个办法：\n' +
  '  1. 直接给文件路径： --bgm ./某文件.mp3\n' +
  '  2. 指一个清单：     export REEL_BGM_MANIFEST=<清单 URL 或本地 json 路径>\n' +
  '     或写进 ~/.reel-kit/config.json： { "bgmManifest": "..." }\n' +
  '     清单格式见 src/bgm-library.mjs 顶部注释'

async function loadManifest() {
  const loc = await manifestLocation()
  if (!loc) return null

  // 本地清单：直接读，不缓存（改完立刻生效，调清单时省事）
  if (!/^https?:\/\//.test(loc)) {
    const path = isAbsolute(loc) ? loc : resolve(loc)
    try {
      return JSON.parse(await readFile(path, 'utf8'))
    } catch (e) {
      throw new Error(`读不到配乐清单 ${path}：${e.message}`)
    }
  }

  mkdirSync(CACHE_DIR, { recursive: true })
  let cached = null
  if (existsSync(MANIFEST_CACHE)) {
    try {
      const raw = JSON.parse(await readFile(MANIFEST_CACHE, 'utf8'))
      cached = raw
      // 换了清单地址就别用旧缓存，否则改了 REEL_BGM_MANIFEST 却还是老曲子
      if (raw.__source === loc && Date.now() - (raw.__fetchedAt || 0) < MANIFEST_TTL_MS) return raw
    } catch { /* 缓存坏了就当没有 */ }
  }

  try {
    const resp = await fetch(loc)
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
    const data = await resp.json()
    data.__fetchedAt = Date.now()
    data.__source = loc
    await writeFile(MANIFEST_CACHE, JSON.stringify(data), 'utf8')
    return data
  } catch (e) {
    // 拉不到就用旧缓存：离线时不该让整次出片失败
    if (cached?.__source === loc) return cached
    throw new Error(`取配乐清单失败（${e.message}）。可以直接给本地文件路径：--bgm ./xxx.mp3`)
  }
}

/**
 * 是否配置了配乐库。
 *
 * 和「库里有没有曲子」是两回事：没配置是「还没接库」，配了但拉不到是「接了、暂时不通」。
 * 界面上必须分开说 —— 把不通说成没配，用户会去反复检查根本没配的东西。
 */
export async function bgmConfigured() {
  return Boolean(await manifestLocation())
}

/**
 * 列出可用配乐，给 `reel bgm` 与工作台用。没配置清单时返回 null（不是空数组 —— 两种情况要分开提示）
 *
 * 只取清单里的 `bgm`（配乐素材库）。清单里的 `tracks` 是作者自己的对外作品，
 * 授权与用途和配乐完全不同（清单自己也写了「刻意分开」），不该混进 BGM 列表 ——
 * 混进来之后列表里那条「原创曲库 · 对外发布需署名」就没人真的会去遵守。
 */
export async function listBgm() {
  const m = await loadManifest()
  if (!m) return null
  const items = (m.bgm || []).map((b) => ({
    kind: 'bgm',
    alias: b.alias,
    key: b.key,
    duration: b.duration,
    mood: b.mood || [],
    license: b.license || '',
    source: b.source || '',
    usedBy: b.usedBy || [],
  }))
  // 清单里的条目可能已经失效（对象被删了但清单没更新）——探一下，取不到的不列
  const probed = await Promise.all(items.map(async item => {
    try {
      const url = trackUrl(m, item)
      if (!/^https?:\/\//.test(url)) return existsSync(url) ? item : null
      const resp = await fetch(url, { method: 'HEAD', signal: AbortSignal.timeout(10000) })
      return resp.ok ? item : null
    } catch { return null }
  }))
  return probed.filter(Boolean)
}

/** 清单里一条记录 → 可下载的绝对地址（或本地路径） */
function trackUrl(manifest, hit) {
  const key = String(hit.key || '')
  if (/^https?:\/\//.test(key)) return key
  if (isAbsolute(key) || key.startsWith('~/')) return expandHome(key)
  const base = String(manifest.cdn || '').replace(/\/+$/, '')
  if (!base) throw new Error(`清单里「${hit.alias}」的 key 是相对路径，但清单没有 cdn 字段`)
  return `${base}/${key.split('/').map(encodeURIComponent).join('/')}`
}

/**
 * 把 `--bgm` 的值解析成本地文件路径。
 *
 * 是本地已存在的文件就原样返回；否则按 alias 去清单里找，命中就下载到
 * ~/.reel-kit/bgm/ 缓存（同一首只下一次）。
 *
 * 返回值是**文件路径字符串** —— 这是对外公开的 API（src/index.mjs 导出），
 * 形状不能随便改；要连授权一起拿，用 resolveBgmEntry。
 */
export async function resolveBgm(input) {
  const entry = await resolveBgmEntry(input, { allowLocalPath: true })
  return entry?.file
}

/**
 * 同 resolveBgm，但返回 { file, alias, license, source }。
 *
 * allowLocalPath：是否接受「本地已存在的路径」。
 * CLI 传 --bgm ./x.mp3 是合理用法，默认开；
 * 走 HTTP 接口的调用方默认关 —— 否则接口等于一个任意文件读取器。
 *
 * exact：只接受清单里的精确别名。
 * 模糊匹配（alias.includes）会让「」「x」「./」这类碎片命中随便一条曲子，
 * 走 HTTP 时必须关掉。
 */
export async function resolveBgmEntry(input, { allowLocalPath = false, exact = false } = {}) {
  if (!input) return undefined
  const asPath = expandHome(input)
  if (allowLocalPath && existsSync(asPath)) {
    return { file: asPath, alias: basename(asPath), license: '', source: '本地文件' }
  }

  const m = await loadManifest()
  if (!m) throw new Error(`配乐「${input}」既不是本地文件，也无法按别名查找。\n${NOT_CONFIGURED}`)

  const list = m.bgm || []
  const hit = exact
    ? list.find((b) => b.alias === input)
    : list.find((b) => b.alias === input)
      || list.find((b) => b.alias?.includes(input))
      || list.find((b) => b.key?.includes(input))

  if (!hit) {
    // 精确模式：调用方自己决定怎么措辞（HTTP 接口要说「库里没有这一首」）
    if (exact) return undefined
    const names = list.map((b) => b.alias).filter(Boolean).join('、') || '（清单里还没有配乐）'
    throw new Error(
      `找不到配乐「${input}」。\n` +
      `可用别名：${names}\n` +
      `也可以直接给本地文件路径：--bgm ./xxx.mp3`,
    )
  }

  const url = trackUrl(m, hit)
  // 清单里直接指本地文件的情况，不必下载也不必缓存
  if (!/^https?:\/\//.test(url)) {
    if (!existsSync(url)) throw new Error(`清单里「${hit.alias}」指向的文件不存在：${url}`)
    announce(hit)
    return { file: url, alias: hit.alias, key: hit.key || '', license: hit.license || '', licenseUrl: hit.licenseUrl || '', source: hit.source || '', sourceUrl: '' }
  }

  mkdirSync(CACHE_DIR, { recursive: true })
  // 缓存文件名带上 URL 指纹：只取 basename 会让不同目录下的同名文件互相覆盖。
  // basename 要去掉 query/hash —— 否则 ?v=2 这种参数会进文件名，缓存每次都命中不了。
  const local = join(CACHE_DIR, `${cacheStamp(url)}-${cacheName(url)}`)
  if (!existsSync(local)) {
    process.stderr.write(`[reel] 下载配乐「${hit.alias}」…`)
    const resp = await fetch(url, { signal: AbortSignal.timeout(120000) })
    if (!resp.ok) throw new Error(`下载配乐失败 HTTP ${resp.status}: ${url}`)
    // 写临时文件再改名：中途断了不会留下半个 mp3 被当成缓存命中
    // 临时名带随机数：并发的两次取用不会抢同一个 .part
    const tmp = `${local}.${randomUUID()}.part`
    try {
      await pipeline(Readable.fromWeb(resp.body), createWriteStream(tmp))
      await rename(tmp, local)
    } catch (e) {
      await unlink(tmp).catch(() => {})
      throw e
    }
    process.stderr.write(' 完成\n')
  }
  announce(hit)
  return { file: local, alias: hit.alias, key: hit.key || '', license: hit.license || '', licenseUrl: hit.licenseUrl || '', source: hit.source || '', sourceUrl: url }
}

/** URL 的短指纹，用于区分同名不同源的缓存文件 */
function cacheStamp(url) {
  return createHash('sha1').update(url).digest('hex').slice(0, 10)
}

/** 缓存文件名用的最后一段：去掉 query 与 hash，只留文件名本身 */
function cacheName(url) {
  const withoutQuery = url.split(/[?#]/)[0]
  const name = withoutQuery.split('/').pop() || 'track'
  try { return decodeURIComponent(name).replace(/[^\w.@+-]+/g, '_') } catch { return 'track' }
}

/** 授权**每次**都打印，不只首次下载时 —— 人恰恰是在反复出片时忘掉授权的 */
function announce(hit) {
  if (hit.license) process.stderr.write(`[reel] 配乐「${hit.alias}」授权：${hit.license}\n`)
}

export { NOT_CONFIGURED }
