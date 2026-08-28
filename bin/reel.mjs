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
import { synthesizeCaptions, durationsFromVoice } from '../src/voice.mjs'
import { resolveBgm, listBgm, NOT_CONFIGURED } from '../src/bgm-library.mjs'

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
  reel bgm              # 配乐库（别名 + 授权 + 时长）

make 选项:
  --template <名>     模板名（见 reel templates），默认 sticker-promo
  --assets <目录|文件> 素材图目录（按文件名排序）或逗号分隔的多个文件
  --caps <文件>        逐句文案，一行一句；行数决定镜头数
  --title <文本>       主标题（如专辑名）
  --subtitle <文本>    副标题（如 IP 名）
  --footer <文本>      底部引导语
  --bgm <别名|文件>    背景音乐（自动循环补满 + 片尾淡出）。给文件路径即可用；
                       想按别名取，先配一个配乐清单（REEL_BGM_MANIFEST 或
                       ~/.reel-kit/config.json），清单见 reel bgm
  --bgm-gain <0~1>     BGM 音量增益。**有配音时默认 0.22**（垫底不抢人声），
                       无配音时默认 1.0。觉得垫太轻调 0.3~0.4，太吵调 0.12~0.18
  --fade-out <秒>      片尾 BGM 淡出时长，默认 1.5
  --per-shot <秒>      每镜时长，默认 2.5
  --last-shot <秒>     末镜时长（留给引导语），默认与 per-shot 相同
  --out <文件>         输出 mp4
  --size <WxH>         画布尺寸，默认 1080x1920
  --fps <n>            输出帧率，默认 30
  --bg <色>            背景色，默认 #ffffff
  --accent1 <色>       左上圆弧色，默认 #FADCE4
  --accent2 <色>       右下圆弧色，默认 #CFEBE0
  --keep-frames        保留中间产物（排版/配音调试用）

配音（让念白驱动镜头时长，而非固定 --per-shot）:
  --voice <音色>       开启配音。voxcraft 后端此项为已注册音色 key（voice list 可查）
  --voice-engine <名>  voxcraft（默认，本地 Qwen3-TTS，免费）| museav（MiMo API，按量计费）
  --design <描述>      museav 后端：一句话描述音色，当场造一个
  --instruction <文本> 语气/风格指令
  --voice-margin <秒>  每镜在念白之后多留的时间，默认 0.45
  --min-shot <秒>      镜头下限，避免极短句一闪而过，默认 1.2

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

  // 工作目录要在配音之前建好 —— 配音产物和帧都放这儿，末尾统一清理
  const workDir = mkdtempSync(join(tmpdir(), 'reel-'))

  /*
   * 镜头时长有两种来源：
   *   固定（默认）      每镜 --per-shot 秒，无念白时用
   *   念白驱动（--voice）每镜 = 该句配音时长 + 余量
   *
   * 后者才是「AI 全程代劳」那条线该有的样子 —— 念快的句子不用干等，
   * 念慢的不会被切。余量不能省：画面做到「刚好等于念白」时，帧率舍入
   * 会让画面略短于音频，每句最后半个字被吃掉。
   */
  const perShot = Number(args['per-shot'] || 2.5)
  const lastShot = Number(args['last-shot'] || perShot)
  let durations = shots.map((_, i) => (i === shots.length - 1 ? lastShot : perShot))
  let voiceClips = null

  if (args.voice || args['voice-engine']) {
    if (!caps.length) { console.error('❌ --voice 需要配合 --caps（配音内容来自逐句文案）'); process.exit(1) }
    const engine = String(args['voice-engine'] || 'voxcraft')
    console.log(`[reel] 配音后端 ${engine}${typeof args.voice === 'string' ? ` · 音色 ${args.voice}` : ''}`)
    const voiceDir = join(workDir, 'voice')
    voiceClips = await synthesizeCaptions({
      captions: shots.map(s => s.caption),
      outDir: voiceDir,
      engine,
      voice: typeof args.voice === 'string' ? args.voice : undefined,
      design: args.design ? String(args.design) : undefined,
      instruction: args.instruction ? String(args.instruction) : undefined,
      onProgress: (i, total, d, meta) => {
        if (meta?.log) { console.log(`[reel] ${meta.log}`); return }
        if (meta?.home) { console.log(`[reel] voxcraft: ${meta.home}  音色库: ${meta.personas.join(', ') || '（空）'}`); return }
        if (meta?.pending) { process.stdout.write(`\r[reel] 配音 ${i}/${total}   `); return }
        process.stdout.write(`\r[reel] 配音 ${i}/${total}（本句 ${d.toFixed(1)}s）   `)
      },
    })
    console.log('')
    durations = durationsFromVoice(
      voiceClips.map(v => v.duration),
      Number(args['voice-margin'] || 0.45),
      Number(args['min-shot'] || 1.2),
    )
  }

  const [w, h] = String(args.size || '1080x1920').split('x').map(Number)

  const vars = {
    title: args.title || '',
    subtitle: args.subtitle || '',
    footer: args.footer || '',
    bg: args.bg || '#ffffff',
    accent1: args.accent1 || '#FADCE4',
    accent2: args.accent2 || '#CFEBE0',
  }

  const totalDur = durations.reduce((a, b) => a + b, 0)
  const shotDesc = voiceClips
    ? `念白驱动 ${Math.min(...durations).toFixed(1)}~${Math.max(...durations).toFixed(1)}s`
    : `每镜 ${perShot}s（末镜 ${lastShot}s）`
  console.log(`[reel] 模板 ${template} · ${n} 镜 · ${shotDesc} · 合计 ${totalDur.toFixed(1)}s`)

  const frameDir = join(workDir, 'frames')
  try {
    process.stdout.write('[reel] 渲染帧… ')
    const frames = await renderFrames({ template: tplFile, shots, vars, outDir: frameDir, width: w, height: h })
    console.log(`${frames.length} 帧`)

    // --bgm 可以是本地路径，也可以是配乐库别名（真源 web-assets/manifest/music.json 的 bgm 段）
    const bgmFile = args.bgm ? await resolveBgm(String(args.bgm)) : undefined

    process.stdout.write('[reel] 合成中… ')
    const { out, duration } = await compose({
      frames, durations,
      bgm: bgmFile,
      voiceClips: voiceClips || undefined,
      bgmGain: args['bgm-gain'] !== undefined ? Number(args['bgm-gain']) : undefined,
      fadeOut: args['fade-out'] !== undefined ? Number(args['fade-out']) : undefined,
      out: resolve(String(args.out)),
      fps: Number(args.fps || 30),
    })
    const real = await probeDuration(out)
    console.log('完成')
    console.log(`\n✅ ${out}`)
    console.log(`   ${w}×${h} · 期望 ${duration.toFixed(1)}s · 实际 ${real.toFixed(1)}s${voiceClips ? ' · 含配音' : ''}${args.bgm ? ' · 含 BGM' : ''}${!voiceClips && !args.bgm ? ' · 无音轨' : ''}`)
    if (Math.abs(real - duration) > 0.5) {
      console.log(`   ⚠️  实际时长与期望差 ${(real - duration).toFixed(1)}s，检查 BGM 是否短于片长`)
    }
    if (args['keep-frames']) console.log(`   中间产物保留在 ${workDir}`)
  } finally {
    if (!args['keep-frames']) rmSync(workDir, { recursive: true, force: true })
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
  if (cmd === 'bgm') {
    const list = await listBgm()
    // null = 没配清单（要告诉人怎么配）；[] = 配了但里面没曲子。两种要分开提示
    if (list === null) { console.log(NOT_CONFIGURED); return }
    if (!list.length) { console.log('清单里还没有配乐。'); return }
    console.log('可用配乐（--bgm <别名>）:')
    for (const b of list) {
      console.log(`  ${b.alias.padEnd(14)} ${String(b.duration || '?').padStart(4)}s  ${(b.mood || []).join('/')}`)
      if (b.license) console.log(`  ${' '.repeat(14)} ${b.license}`)
      if (b.usedBy?.length) console.log(`  ${' '.repeat(14)} 用过：${b.usedBy.join('、')}`)
    }
    return
  }
  if (cmd === 'make') {
    if (!findChrome()) { console.error('❌ 找不到本机 Chrome，设 CHROME_PATH'); process.exit(1) }
    return cmdMake(args)
  }
  console.error(`未知命令 ${cmd}`); console.log(HELP); process.exit(1)
}

main().catch(e => { console.error(`❌ ${e.message}`); process.exit(1) })
