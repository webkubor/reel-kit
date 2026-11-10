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
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
const run = promisify(execFile)

import { renderFrames, findChrome } from '../src/render.mjs'
import { compose, probeDuration } from '../src/compose.mjs'
import { synthesizeCaptions, durationsFromVoice, splitVoiceByChars } from '../src/voice.mjs'
import { resolveBgm, listBgm, NOT_CONFIGURED } from '../src/bgm-library.mjs'
import { getStyle, KNOWLEDGE_STYLES } from '../src/knowledge-styles.mjs'
import { PRESETS } from '../src/settings.mjs'

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
  reel studio           # 本地可视化工作台，默认 http://127.0.0.1:4390
  reel make [选项]
  reel templates
  reel styles           # 知识短视频 44 套视觉风格库
  reel bgm              # 配乐库（别名 + 授权 + 时长）
  reel channels         # 自定义模型通道（自己的中转 / 厂商直连）

channels 子命令:
  reel channels list                      # 列出已配通道 + 配置问题
  reel channels add --id <id> --base-url <地址> --env <环境变量名> \\
      [--label <名称>] [--kind image|video] [--protocol <协议>] [--model <模型>] \\
      [--image-path <路径>] [--task-path <路径>] [--auth bearer|api-key]
  reel channels rm <id>                   # 删掉一条通道
  reel channels check <id>                # 探活：不通不花钱，不生成任何内容

  协议：生图 openai / minimax-image / museav，视频 ark-task / museav
  密钥**只写环境变量名**（--env），不写进配置文件；调用前把密钥注入那个变量。

make 选项:
  --template <名>     模板名（见 reel templates），默认 sticker-promo
  --style <名>         知识短片视觉风格（吸收自 vibe-knowledge，如 gufeng、darkmath、starfield、paper 等 44 套），自动注入推荐模板与配色
  --source <文本>      出处与权威文献标注（如《自然》2024 /《史记》），显示在角标
  --assets <目录|文件> 素材图目录（按文件名排序）或逗号分隔的多个文件
  --caps <文件>        逐句文案，一行一句；行数决定镜头数
  --title <文本>       主标题（如专辑名）
  --subtitle <文本>    副标题（如 IP 名）
  --footer <文本>      底部引导语
  --bgm <别名|文件>    背景音乐（自动循环补满 + 片尾淡出）。给文件路径即可用；
                       想按别名取，先配一个配乐清单（REEL_BGM_MANIFEST 或
                       ~/.reel-kit/config.json），清单见 reel bgm
  --bgm-gain <0~1>     BGM 音量增益。**有配音时默认 0.12**(垫底不抢人声),
                       无配音时默认 1.0。觉得垫太轻调 0.3~0.4,太吵调 0.06~0.12
  --bgm-crossfade <秒>  BGM 循环接缝交叉淡化秒数,默认 1.0。短曲子 loop 时
                        会把头 K 秒和尾 K 秒做平滑过渡再去循环,听感无爆音。
                        设 0 退回旧行为(直接 loop)
  --fade-out <秒>      片尾 BGM 淡出时长,默认 1.5
  --preset <预设>       一键套用常用组合:default(默认) / clear-speech(口播清晰,
                       配乐压到 8%、人声提到 1.5 倍) / ambient(氛围铺底,配乐 30%) /
                       raw(原样输出,不归一化不压配乐)。后面的具体参数会覆盖预设
  --voice-gain <倍数>   人声增益,默认 1.35。念白被配乐盖住就调大(1.6~2.0),
                       人声太冲就调小(1.0~1.2)
  --no-duck           不做侧链。默认人声一响就把配乐压下去,人声停时抬回来。
                       配乐本身要跟人声抢位置(比如卡点混剪)时才关掉
  --no-normalize      不做响度归一化。默认输出 -14 LUFS / -1 dBTP,
                       自己要在别处统一音量时关掉(仍会限幅,不会削顶)
  --per-shot <秒>      每镜时长，默认 2.5
  --last-shot <秒>     末镜时长（留给引导语），默认与 per-shot 相同
  --out <文件>         输出 mp4
  --size <WxH>         画布尺寸，默认 1080x1920
  --fps <n>            输出帧率，默认 30
  --bg <色>            背景色，默认 #ffffff
  --accent1 <色>       左上圆弧色，默认 #FADCE4
  --accent2 <色>       右下圆弧色，默认 #CFEBE0
  --keep-frames        保留中间产物(排版/配音调试用)

转场(xfade,镜与镜之间平滑过渡):
  --transition <名>     转场名,默认 none(沿用硬切)。可选:
                       fade · slide-left/right/up/down · wipe-left/right
                       · dissolve · zoom-in
  --transition-duration <秒>  转场时长,默认 0.4

镜内动效（同一镜内画面自己动起来）:
  --motion <名>        gentle(默认,轻柔运镜:图片缓慢推进、字幕入场,逐帧渲染,更慢) | none(静态画面,最快)

配音（让念白驱动镜头时长，而非固定 --per-shot）:
  --voice <音色>       开启配音。edge 默认为 zh-CN-XiaoxiaoNeural；voxcraft 为已注册音色 key；minimax 为音色 ID
  --voice-engine <名>  edge（微软高保真 24kHz 在线免费，免 Key）| voxcraft（默认本地 Qwen3-TTS，免费）| minimax（云端 TTS，按量计费）
  --design <描述>      voxcraft 后端：一句话描述音色
  --instruction <文本> 语气/风格指令（voxcraft 后端）
  --voice-margin <秒>  每镜在念白之后多留的时间，默认 0.45
  --min-shot <秒>      镜头下限，避免极短句一闪而过，默认 1.2

字幕分段（长句不重读，字幕分镜显示）:
  --caps-split <文件>  长句拆成多镜的字幕分段，**用空行分隔对应 --caps 各行**。
                       文件内每段非空行 = 同一 --caps 行的 N 段字幕。
                       工具按字数比例从原配音 atrim 切 N 段，听感无断音。

素材与文案的配对规则:
  按顺序一一对应。数量不等时取较少的一方，并在开头提示实际用了几镜 ——
  多出来的素材/文案会被明确报出来，不会静默丢弃。
`

async function cmdMake(args) {
  const matchedStyle = args.style ? getStyle(args.style) : null
  if (args.style && !matchedStyle) {
    console.warn(`⚠️  未匹配到风格 "${args.style}"，使用默认配置`)
  }

  const template = String(args.template || matchedStyle?.template || 'sticker-promo')
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

  /*
   * --caps-split:长句不重读但字幕分镜显示的"渐进"模式。
   * 文件用空行分组:每段非空行对应 --caps 的一行,可拆成 1~N 行字幕。
   * 工具会按字数比例从原配音 atrim 切出 N 段,听感无"重新合成"的断音。
   * 例:--caps 一行 "今天天气不错,适合出去走走",--caps-split 用空行分两段:
   *   今天天气不错
   *
   *   适合出去走走
   * (空行表示 --caps 句边界)
   */
  let capsSplitGroups = []
  if (args['caps-split']) {
    if (!existsSync(String(args['caps-split']))) { console.error(`❌ 字幕分段文件不存在: ${args['caps-split']}`); process.exit(1) }
    capsSplitGroups = readFileSync(String(args['caps-split']), 'utf-8')
      .split(/\n\s*\n/)
      .map(g => g.split('\n').map(s => s.trim()).filter(Boolean))
    if (capsSplitGroups.length !== caps.length) {
      console.error(`❌ --caps-split 分组数(${capsSplitGroups.length})与 --caps 行数(${caps.length})不一致`)
      console.error(`   --caps-split 用空行分隔段,每段非空行对应 --caps 一行`)
      process.exit(1)
    }
  }

  // 单图模式：当只给了一张素材图（如单曲封面/主KV）而文案有多句时，自动将单图复用于所有镜头
  if (images.length === 1 && caps.length > 1) {
    images = Array(caps.length).fill(images[0])
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

  /*
   * shots 构造:有 capsSplit 时,一行 caps 拆成 capsSplitGroups[i].length 镜,
   * 同图(用 caps 对应素材),字幕逐段显示。配音阶段会按字数切原 wav 配合。
   */
  const shots = []
  for (let i = 0; i < n; i++) {
    if (capsSplitGroups.length) {
      for (const sub of capsSplitGroups[i]) {
        shots.push({ image: images[i], caption: sub })
      }
    } else {
      shots.push({ image: images[i], caption: caps[i] || '' })
    }
  }

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

    /*
     * capsSplit 模式:把每句配音按字数比例 atrim 切分成 N 段,
     * 展平到 shots 同一长度。durations 重新按"每段配音 + 余量"算。
     */
    if (capsSplitGroups.length) {
      const flat = []
      for (let i = 0; i < voiceClips.length; i++) {
        const chars = capsSplitGroups[i].map(c => [...c].length)
        const segDir = join(workDir, 'voice', `seg_${i}`)
        const segs = await splitVoiceByChars(voiceClips[i].file, chars, segDir)
        flat.push(...segs)
      }
      voiceClips = flat
    }

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
    source: args.source || '',
    bg: args.bg || matchedStyle?.bg || '#ffffff',
    accent1: args.accent1 || matchedStyle?.accent1 || '#FADCE4',
    accent2: args.accent2 || matchedStyle?.accent2 || '#CFEBE0',
  }

  const totalDur = durations.reduce((a, b) => a + b, 0)
  const shotDesc = voiceClips
    ? `念白驱动 ${Math.min(...durations).toFixed(1)}~${Math.max(...durations).toFixed(1)}s`
    : `每镜 ${perShot}s(末镜 ${lastShot}s)`
  const transition = String(args.transition || 'none')
  const xdur = Number(args['transition-duration'] || 0.4)
  const motion = String(args.motion || 'gentle')
  const shotDescFull = transition !== 'none'
    ? `${shotDesc} · 转场 ${transition} ${xdur}s`
    : shotDesc
  console.log(`[reel] 模板 ${template} · ${n} 镜 · ${shotDescFull} · 合计 ${totalDur.toFixed(1)}s`)

  const frameDir = join(workDir, 'frames')
  try {
    process.stdout.write('[reel] 渲染帧… ')
    const frames = await renderFrames({ template: tplFile, shots, vars, outDir: frameDir, width: w, height: h,
      motion, durations, fps: Number(args.fps || 30) })
    console.log(frames.length === n ? `${n} 帧` : `${frames.length} 段动画`)

    // --bgm 可以是本地路径,也可以是配乐库别名(清单由 REEL_BGM_MANIFEST 指定,见 bgm-library.mjs)
    const bgmFile = args.bgm ? await resolveBgm(String(args.bgm)) : undefined
    if (bgmFile) {
      // 短曲子提前警告 + 提示接缝处理已自动启用
      // probe 失败不阻塞 —— 让 ffmpeg 自己报
      try {
        const bgmDur = await probeDuration(bgmFile)
        if (bgmDur < totalDur * 0.7) {
          const loops = Math.ceil(totalDur / bgmDur)
          console.log(`⚠️  配乐 ${bgmDur.toFixed(1)}s · 片子 ${totalDur.toFixed(1)}s · 需循环 ${loops} 次,接缝可能仍可闻`)
          console.log(`   已默认启用 --bgm-crossfade 1.0 平滑接缝;曲子太短仍可能拖拍,建议换更长的`)
        }
      } catch { /* 探测失败不阻塞 */ }
    }

    process.stdout.write('[reel] 合成中… ')
    /*
     * 预设 → 具体参数 → 实际生效值。
     * 后面的 flag 覆盖预设：先想「这条片子是什么调子」，再临时拧一个旋钮。
     */
    const presetId = args.preset ? String(args.preset) : 'default'
    if (!PRESETS[presetId]) {
      throw new Error(`没有这个预设：${presetId}。可选：${Object.keys(PRESETS).join(' / ')}`)
    }
    const preset = PRESETS[presetId].settings
    const { out, duration } = await compose({
      frames, durations,
      bgm: bgmFile,
      voiceClips: voiceClips || undefined,
      bgmGain: args['bgm-gain'] !== undefined ? Number(args['bgm-gain']) : preset.bgmGain,
      bgmCrossfade: args['bgm-crossfade'] !== undefined ? Number(args['bgm-crossfade']) : 1.0,
      transition: String(args.transition || 'none'),
      transitionDuration: Number(args['transition-duration'] || 0.4),
      fadeOut: args['fade-out'] !== undefined ? Number(args['fade-out']) : undefined,
      voiceGain: args['voice-gain'] !== undefined ? Number(args['voice-gain']) : preset.voiceGain,
      duck: args['no-duck'] ? false : preset.duck,
      normalize: args['no-normalize'] ? false : preset.normalize,
      out: resolve(String(args.out)),
      fps: Number(args.fps || 30),
    })
    const real = await probeDuration(out)
    // 实际输出尺寸由模板决定(--size 只决定 viewport,模板的 body 才是真尺寸)
    const { stdout: dim } = await run('ffprobe', ['-v', 'error', '-select_streams', 'v:0',
      '-show_entries', 'stream=width,height', '-of', 'csv=p=0', out])
    const [outW, outH] = dim.trim().split(',').map(Number)
    console.log('完成')
    console.log(`\n✅ ${out}`)
    console.log(`   ${outW}×${outH} · 期望 ${duration.toFixed(1)}s · 实际 ${real.toFixed(1)}s${voiceClips ? ' · 含配音' : ''}${args.bgm ? ' · 含 BGM' : ''}${!voiceClips && !args.bgm ? ' · 无音轨' : ''}`)
    if (Math.abs(real - duration) > 0.5) {
      console.log(`   ⚠️  实际时长与期望差 ${(real - duration).toFixed(1)}s，检查 BGM 是否短于片长`)
    }
    if (args['keep-frames']) console.log(`   中间产物保留在 ${workDir}`)
  } finally {
    if (!args['keep-frames']) rmSync(workDir, { recursive: true, force: true })
  }
}

async function cmdChannels(args) {
  const {
    loadCustomChannels, customChannelIssues, upsertChannel, removeChannel, probeChannel, IMAGE_PROTOCOLS, VIDEO_PROTOCOLS,
  } = await import('../src/studio/channels.mjs')
  const action = args._[0] || 'list'

  if (action === 'list') {
    const { file, problems } = customChannelIssues()
    const channels = loadCustomChannels()
    console.log(`通道配置文件：${file}`)
    if (!channels.length) {
      console.log('还没有自定义通道。用 reel channels add 新增一条。')
    }
    for (const c of channels) {
      const keyState = c.key
        ? (c.keySource === 'env' ? `密钥来自环境变量 ${c.env}` : '密钥已存在设置页')
        : `还没填密钥（环境变量 ${c.env} 或设置页）`
      console.log(`  ${c.id.padEnd(14)} ${c.kind.padEnd(5)} ${c.protocol.padEnd(13)} ${c.label} · ${keyState}`)
    }
    if (problems.length) {
      console.log('\n⚠️  配置问题：')
      for (const p of problems) console.log(`  - ${p}`)
    }
    return
  }

  if (action === 'add') {
    const id = String(args.id || '').trim()
    const kind = String(args.kind || 'image').trim()
    const protocol = String(args.protocol || (kind === 'video' ? 'ark-task' : 'openai')).trim()
    if (!id) { console.error('❌ 缺 --id'); process.exit(1) }
    if (!args['base-url']) { console.error('❌ 缺 --base-url'); process.exit(1) }
    if (!args.env) { console.error('❌ 缺 --env（环境变量名，密钥本身不要写进文件）'); process.exit(1) }
    if (kind === 'image' && !IMAGE_PROTOCOLS.includes(protocol)) { console.error(`❌ 生图协议只支持 ${IMAGE_PROTOCOLS.join(' / ')}`); process.exit(1) }
    if (kind === 'video' && !VIDEO_PROTOCOLS.includes(protocol)) { console.error(`❌ 视频协议只支持 ${VIDEO_PROTOCOLS.join(' / ')}`); process.exit(1) }
    const { file } = upsertChannel({
      id, label: String(args.label || id), kind, protocol,
      baseUrl: String(args['base-url']), model: String(args.model || ''), env: String(args.env),
      ...(args['image-path'] ? { imagePath: String(args['image-path']) } : {}),
      ...(args['task-path'] ? { taskPath: String(args['task-path']) } : {}),
      ...(args.auth ? { auth: String(args.auth) } : {}),
    })
    console.log(`✅ 已写入 ${file}`)
    if (!process.env[String(args.env)]) console.log(`⚠️  环境变量 ${args.env} 当前没注入，调用会被拒绝`)
    console.log('   接着跑：reel channels check ' + id)
    return
  }

  if (action === 'rm' || action === 'remove' || action === 'delete') {
    const id = args._[1] || String(args.id || '')
    if (!id) { console.error('❌ 缺通道 id'); process.exit(1) }
    const { file } = removeChannel(id)
    console.log(`✅ 已从 ${file} 删掉 ${id}`)
    return
  }

  if (action === 'check') {
    const id = args._[1] || String(args.id || '')
    const channels = loadCustomChannels()
    const target = id ? channels.find(c => c.id === id) : channels[0]
    if (!target) { console.error(`❌ 没有可检查的通道${id ? `（${id}）` : ''}`); process.exit(1) }
    console.log(`检查通道「${target.label}」（${target.protocol} · ${target.baseUrl}）…`)
    const result = await probeChannel(target)
    // 三态：通过 ✅、失败 ❌、未验证 ⚠️。
    // 「没验证」标成通过就是骗人 —— 用户会以为能用了。
    for (const c of result.checks) {
      const mark = c.ok === true ? '✅' : c.ok === false ? '❌' : '⚠️'
      console.log(`  ${mark} ${c.name}：${c.detail}`)
    }
    const failed = result.checks.some(c => c.ok === false)
    const unknown = result.checks.some(c => c.ok === null)
    if (failed) {
      console.log('\n❌ 这条通道还用不了，先按上面几行改')
      process.exit(1)
    }
    if (unknown) {
      console.log('\n⚠️  地址与密钥都通，但还有没验过的项 —— 真跑一次任务才算数')
      return
    }
    console.log('\n✅ 这条通道可以用')
    return
  }

  console.error(`未知子命令 ${action}。可用：list / add / rm / check`)
  process.exit(1)
}

async function main() {
  const argv = process.argv.slice(2)
  const cmd = argv[0]
  const args = parseArgs(argv.slice(1))

  if (!cmd || cmd === 'help' || args.help || args.h) { console.log(HELP); return }
  if (cmd === 'studio') {
    const { startStudio } = await import('../src/studio/server.mjs')
    const port = Number(args.port || process.env.REEL_STUDIO_PORT || 4390)
    if (!Number.isInteger(port) || port < 1024 || port > 65535) throw new Error('工作台端口须为 1024–65535')
    startStudio({ port })
    return
  }
  if (cmd === 'templates') {
    console.log('可用模板:')
    for (const t of listTemplates()) console.log(`  ${t}`)
    return
  }
  if (cmd === 'styles') {
    console.log('知识短视频 44 套视觉风格库（可配合 --style <名> 一键应用配色与模板）:')
    for (const s of KNOWLEDGE_STYLES) {
      console.log(`  ${s.key.padEnd(16)} 底色: ${s.bg}  强调: ${s.accent1} / ${s.accent2}  (推荐模板: ${s.template})`)
    }
    return
  }
  if (cmd === 'bgm') {
    const list = await listBgm()
    // null = 没配清单（要告诉人怎么配）；[] = 配了但里面没曲子。两种要分开提示
    if (list === null) { console.log(NOT_CONFIGURED); return }
    if (!list.length) { console.log('清单里还没有配乐。'); return }
    console.log('可用配乐（--bgm <别名>）:')
    for (const b of list) {
      const tag = b.kind === 'track' ? '原创' : '配乐'
      console.log(`  ${b.alias.padEnd(14)} ${String(b.duration || '?').padStart(4)}s  ${tag}  ${(b.mood || []).join('/')}`)
      if (b.license) console.log(`  ${' '.repeat(14)} ${b.license}`)
      if (b.usedBy?.length) console.log(`  ${' '.repeat(14)} 用过：${b.usedBy.join('、')}`)
    }
    return
  }
  if (cmd === 'channels') {
    return cmdChannels(args)
  }
  if (cmd === 'make') {
    if (!findChrome()) { console.error('❌ 找不到本机 Chrome，设 CHROME_PATH'); process.exit(1) }
    return cmdMake(args)
  }
  console.error(`未知命令 ${cmd}`); console.log(HELP); process.exit(1)
}

main().catch(e => { console.error(`❌ ${e.message}`); process.exit(1) })
