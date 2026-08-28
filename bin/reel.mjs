#!/usr/bin/env node
/**
 * reel — 竖版短视频合成工作台
 *
 * 起因：表情包推广视频、产品短视频这类内容，每次都是临时拼一串 ffmpeg 命令。
 * 成品留下了，流程没留下 —— 下次做同类片子得从头再拼一遍。这个 CLI 把那段流程固化。
 *
 *   reel make --template sticker-promo \
 *     --title "莓啾日常" --subtitle "莓啾" \
 *     --assets ./stickers --caps caps-莓啾日常.txt \
 *     --bgm 配乐候选/682.mp3 --per-shot 2.5 \
 *     --out 推广视频-莓啾日常.mp4
 */

import { existsSync, readFileSync, readdirSync, rmSync, statSync } from 'node:fs'
import { basename, dirname, extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { tmpdir } from 'node:os'
import { mkdtempSync } from 'node:fs'

import { renderFrames, findChrome } from '../src/render.mjs'
import { compose, probeDuration } from '../src/compose.mjs'

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const TEMPLATES = join(ROOT, 'templates')
const IMG_EXT = new Set(['.png', '.jpg', '.jpeg', '.webp', '.gif'])

function parseArgs(argv) {
  const out = { _: [] }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]
    if (a.startsWith('--')) {
      const key = a.slice(2)
      const next = argv[i + 1]
      if (next === undefined || next.startsWith('--')) out[key] = true
      else { out[key] = next; i++ }
    } else out._.push(a)
  }
  return out
}

function listTemplates() {
  return readdirSync(TEMPLATES).filter(f => f.endsWith('.html')).map(f => basename(f, '.html'))
}

const HELP = `
reel — 竖版短视频合成工作台

用法:
  reel make [选项]
  reel templates

make 选项:
  --template <名>     模板名（见 reel templates），默认 sticker-promo
  --assets <目录|文件> 素材图目录（按文件名排序）或逗号分隔的多个文件
  --caps <文件>        逐句文案，一行一句；行数决定镜头数
  --title <文本>       主标题（如专辑名）
  --subtitle <文本>    副标题（如 IP 名）
  --footer <文本>      底部引导语
  --bgm <文件>         背景音乐（自动循环补满 + 片尾淡出）
  --per-shot <秒>      每镜时长，默认 2.5
  --last-shot <秒>     末镜时长（留给引导语），默认与 per-shot 相同
  --out <文件>         输出 mp4
  --size <WxH>         画布尺寸，默认 1080x1920
  --fps <n>            输出帧率，默认 30
  --bg <色>            背景色，默认 #ffffff
  --accent1 <色>       左上圆弧色，默认 #FADCE4
  --accent2 <色>       右下圆弧色，默认 #CFEBE0
  --keep-frames        保留中间帧（排版调试用）

素材与文案的配对规则:
  按顺序一一对应。数量不等时取较少的一方，并在开头提示实际用了几镜 ——
  多出来的素材/文案会被明确报出来，不会静默丢弃。
`

async function cmdMake(args) {
  const template = String(args.template || 'sticker-promo')
  const tplFile = join(TEMPLATES, `${template}.html`)
  if (!existsSync(tplFile)) {
    console.error(`❌ 没有模板 ${template}。可用: ${listTemplates().join(', ')}`)
    process.exit(1)
  }
  if (!args.assets) { console.error('❌ 缺 --assets'); process.exit(1) }
  if (!args.out) { console.error('❌ 缺 --out'); process.exit(1) }

  // 素材：目录则按名排序，逗号分隔则按给定顺序
  let images = []
  const assetsArg = String(args.assets)
  if (existsSync(assetsArg) && statSync(assetsArg).isDirectory()) {
    images = readdirSync(assetsArg)
      .filter(f => IMG_EXT.has(extname(f).toLowerCase()))
      .sort()
      .map(f => join(assetsArg, f))
  } else {
    images = assetsArg.split(',').map(s => resolve(s.trim())).filter(Boolean)
  }
  const missing = images.filter(f => !existsSync(f))
  if (missing.length) { console.error(`❌ 素材不存在: ${missing.join(', ')}`); process.exit(1) }
  if (!images.length) { console.error('❌ 没找到素材图'); process.exit(1) }

  let caps = []
  if (args.caps) {
    if (!existsSync(String(args.caps))) { console.error(`❌ 文案文件不存在: ${args.caps}`); process.exit(1) }
    caps = readFileSync(String(args.caps), 'utf-8').split('\n').map(s => s.trim()).filter(Boolean)
  }

  // 数量不等时说清楚丢了什么 —— 静默截断会让人以为片子出全了
  const n = caps.length ? Math.min(images.length, caps.length) : images.length
  if (caps.length && images.length !== caps.length) {
    console.log(`⚠️  素材 ${images.length} 张、文案 ${caps.length} 句，按较少的取 ${n} 镜`)
    const dropped = images.length > caps.length
      ? images.slice(n).map(f => basename(f)).join(', ')
      : caps.slice(n).join(' / ')
    console.log(`   未使用: ${dropped}`)
  }

  const shots = Array.from({ length: n }, (_, i) => ({
    image: images[i],
    caption: caps[i] || '',
  }))

  const perShot = Number(args['per-shot'] || 2.5)
  const lastShot = Number(args['last-shot'] || perShot)
  const durations = shots.map((_, i) => (i === shots.length - 1 ? lastShot : perShot))

  const [w, h] = String(args.size || '1080x1920').split('x').map(Number)

  const vars = {
    title: args.title || '',
    subtitle: args.subtitle || '',
    footer: args.footer || '',
    bg: args.bg || '#ffffff',
    accent1: args.accent1 || '#FADCE4',
    accent2: args.accent2 || '#CFEBE0',
  }

  console.log(`[reel] 模板 ${template} · ${n} 镜 · 每镜 ${perShot}s（末镜 ${lastShot}s）· 合计 ${durations.reduce((a, b) => a + b, 0).toFixed(1)}s`)

  const frameDir = mkdtempSync(join(tmpdir(), 'reel-'))
  try {
    process.stdout.write('[reel] 渲染帧… ')
    const frames = await renderFrames({ template: tplFile, shots, vars, outDir: frameDir, width: w, height: h })
    console.log(`${frames.length} 帧`)

    process.stdout.write('[reel] 合成中… ')
    const { out, duration } = await compose({
      frames, durations,
      bgm: args.bgm ? String(args.bgm) : undefined,
      out: resolve(String(args.out)),
      fps: Number(args.fps || 30),
    })
    const real = await probeDuration(out)
    console.log('完成')
    console.log(`\n✅ ${out}`)
    console.log(`   ${w}×${h} · 期望 ${duration.toFixed(1)}s · 实际 ${real.toFixed(1)}s${args.bgm ? ' · 含 BGM' : ' · 无音轨'}`)
    if (Math.abs(real - duration) > 0.5) {
      console.log(`   ⚠️  实际时长与期望差 ${(real - duration).toFixed(1)}s，检查 BGM 是否短于片长`)
    }
    if (args['keep-frames']) console.log(`   帧保留在 ${frameDir}`)
  } finally {
    if (!args['keep-frames']) rmSync(frameDir, { recursive: true, force: true })
  }
}

async function main() {
  const argv = process.argv.slice(2)
  const cmd = argv[0]
  const args = parseArgs(argv.slice(1))

  if (!cmd || cmd === 'help' || args.help || args.h) { console.log(HELP); return }
  if (cmd === 'templates') {
    console.log('可用模板:')
    for (const t of listTemplates()) console.log(`  ${t}`)
    return
  }
  if (cmd === 'make') {
    if (!findChrome()) { console.error('❌ 找不到本机 Chrome，设 CHROME_PATH'); process.exit(1) }
    return cmdMake(args)
  }
  console.error(`未知命令 ${cmd}`); console.log(HELP); process.exit(1)
}

main().catch(e => { console.error(`❌ ${e.message}`); process.exit(1) })
