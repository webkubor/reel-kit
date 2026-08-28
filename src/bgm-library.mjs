/**
 * 配乐库 —— 按别名取曲子，不写死路径。
 *
 * 真源是 web-assets 的 `manifest/music.json` 的 `bgm` 段（本体在 R2 `music/bgm/`，
 * CDN music.webkubor.online）。那里每条都带 license 与 source ——
 * 出片是要对外发的，「这首能不能商用、要不要署名」必须当场查得到。
 *
 * 为什么不直接写文件路径：配乐散在各个项目目录里，换台机器就没了，
 * 也没人知道某首曲子的授权是什么。`--bgm 儿童轻快` 走这里解析，
 * `--bgm ./某文件.mp3` 仍然按原样当本地路径用，不影响老用法。
 */
import { existsSync, mkdirSync, createWriteStream } from 'node:fs'
import { readFile, writeFile, unlink } from 'node:fs/promises'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'

const MANIFEST_URL = 'https://music.webkubor.online/manifest.json'
const CDN_BASE = 'https://music.webkubor.online/'
const CACHE_DIR = join(homedir(), '.reel-kit', 'bgm')
const MANIFEST_CACHE = join(CACHE_DIR, 'manifest.json')
// 清单很小且不常变，缓存一天足够；过期只是重新拉一次，拉不到还会退回旧缓存
const MANIFEST_TTL_MS = 24 * 60 * 60 * 1000

async function loadManifest() {
  mkdirSync(CACHE_DIR, { recursive: true })

  let cached = null
  if (existsSync(MANIFEST_CACHE)) {
    try {
      const raw = JSON.parse(await readFile(MANIFEST_CACHE, 'utf8'))
      cached = raw
      if (Date.now() - (raw.__fetchedAt || 0) < MANIFEST_TTL_MS) return raw
    } catch { /* 缓存坏了就当没有 */ }
  }

  try {
    const resp = await fetch(MANIFEST_URL)
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
    const data = await resp.json()
    data.__fetchedAt = Date.now()
    await writeFile(MANIFEST_CACHE, JSON.stringify(data), 'utf8')
    return data
  } catch (e) {
    // 拉不到就用旧缓存：离线时不该让整次出片失败
    if (cached) return cached
    throw new Error(`取配乐清单失败（${e.message}）。可以直接给本地文件路径：--bgm ./xxx.mp3`)
  }
}

/** 列出可用配乐，给 `reel bgm` 用 */
export async function listBgm() {
  const m = await loadManifest()
  return (m.bgm || []).map((b) => ({
    alias: b.alias,
    duration: b.duration,
    mood: b.mood || [],
    license: b.license,
    source: b.source,
    usedBy: b.usedBy || [],
  }))
}

/**
 * 把 `--bgm` 的值解析成本地文件路径。
 *
 * 是本地已存在的文件就原样返回；否则按 alias 去清单里找，命中就下载到
 * ~/.reel-kit/bgm/ 缓存（同一首只下一次）。
 */
export async function resolveBgm(input) {
  if (!input) return undefined
  if (existsSync(input)) return input

  const m = await loadManifest()
  const list = m.bgm || []
  const hit = list.find((b) => b.alias === input)
    || list.find((b) => b.alias?.includes(input))
    || list.find((b) => b.key?.includes(input))

  if (!hit) {
    const names = list.map((b) => b.alias).join('、') || '（清单里还没有配乐）'
    throw new Error(
      `找不到配乐「${input}」。\n` +
      `可用别名：${names}\n` +
      `也可以直接给本地文件路径：--bgm ./xxx.mp3`,
    )
  }

  mkdirSync(CACHE_DIR, { recursive: true })
  const local = join(CACHE_DIR, hit.key.split('/').pop())

  if (!existsSync(local)) {
    const url = CDN_BASE + hit.key.split('/').map(encodeURIComponent).join('/')
    process.stderr.write(`[reel] 下载配乐「${hit.alias}」…`)
    const resp = await fetch(url)
    if (!resp.ok) throw new Error(`下载配乐失败 HTTP ${resp.status}: ${url}`)
    // 写临时文件再改名：中途断了不会留下半个 mp3 被当成缓存命中
    const tmp = local + '.part'
    try {
      await pipeline(Readable.fromWeb(resp.body), createWriteStream(tmp))
      const { rename } = await import('node:fs/promises')
      await rename(tmp, local)
    } catch (e) {
      await unlink(tmp).catch(() => {})
      throw e
    }
    process.stderr.write(` 完成\n`)
  }

  // 授权**每次**都打印，不只是首次下载时 —— 片子是要对外发的，
  // 「这首能不能商用」必须在出片那一刻看得见。只在下载时提示的话，
  // 第二次之后就静默了，而人恰恰是在反复出片时忘掉授权的。
  if (hit.license) process.stderr.write(`[reel] 配乐「${hit.alias}」授权：${hit.license}\n`)
  return local
}
