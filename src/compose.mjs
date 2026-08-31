/**
 * 逐帧 PNG + BGM → mp4。
 *
 * 用 concat demuxer 而不是 `-framerate 1/2.5 -i frame_%04d.png`：
 * 后者要求每镜时长完全一致，而「片尾多停一会」「某句字多停 0.5 秒」是很自然的需求。
 * concat 列表里每帧各带各的 duration，想改哪镜改哪镜。
 */

import { execFile } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { promisify } from 'node:util'

const run = promisify(execFile)

/**
 * ffmpeg xfade transition 名字 → 内部滤镜名。
 * 用白名单而不是透传用户输入 —— xfade 不接受任意 transition,拼错会报奇怪的错。
 */
const TRANSITIONS = {
  fade: 'fade',
  'slide-left': 'slideleft',
  'slide-right': 'slideright',
  'slide-up': 'slideup',
  'slide-down': 'slidedown',
  'wipe-left': 'wipeleft',
  'wipe-right': 'wiperight',
  dissolve: 'dissolve',
  'zoom-in': 'zoomin',
}

/*
 * ffmpeg concat 列表不再重复末帧。
 *
 * 历史:旧版 ffmpeg 忽略末行 duration,需要重复末帧"骗"它生效。
 * 现行 ffmpeg (>= 4.x,实测 7.x) 行为相反:末行 duration 被忽略是默认行为
 * (忽略 = 不算那一行),但**重复末行**反而会让 ffmpeg 多输出一整段时长
 * (实测 5 镜 2.5s 期望 12.5s,重复末行 = 15.0s,差 1 镜)。
 *
 * 5 镜 2.5s 不重复末行实测 12.43s,差 < 0.1s(末帧舍入),可接受。
 * 拆配音后末镜用 --last-shot 单独配时长,逻辑保持一致。
 */
function buildConcatList(frames, durations) {
  const lines = []
  frames.forEach((f, i) => {
    lines.push(`file '${f.replace(/'/g, "'\\''")}'`)
    lines.push(`duration ${durations[i]}`)
  })
  return lines.join('\n') + '\n'
}

/**
 * @param {object} opts
 * @param {string[]} opts.frames     帧路径(有序)
 * @param {number[]} opts.durations  每帧秒数,与 frames 等长
 * @param {string}   [opts.bgm]      背景音乐路径
 * @param {string}   opts.out        输出 mp4
 * @param {number}   [opts.fps]      输出帧率,默认 30
 * @param {number}   [opts.fadeOut]  片尾音频淡出秒数,默认 1.5
 * @param {number}   [opts.bgmCrossfade]  BGM 头尾交叉淡化秒数,>0 时启用(默认 0 = 旧行为)
 * @param {string}   [opts.transition]  镜间转场名,默认 'none'(concat demuxer 旧路径)
 * @param {number}   [opts.transitionDuration]  转场时长秒,默认 0.4
 */
export async function compose({ frames, durations, bgm, voiceClips, out, fps = 30, fadeOut, bgmGain, bgmCrossfade, transition = 'none', transitionDuration = 0.4 }) {
  if (fadeOut === undefined || Number.isNaN(fadeOut)) fadeOut = 1.5
  if (!frames.length) throw new Error('没有帧可合成')
  if (frames.length !== durations.length) throw new Error('frames 与 durations 长度不一致')
  if (voiceClips && voiceClips.length !== frames.length) {
    throw new Error('voiceClips 与 frames 长度不一致')
  }

  const total = durations.reduce((a, b) => a + b, 0)

  // 转场模式走 filter_complex 路径,不走 concat demuxer
  if (transition !== 'none') {
    return composeWithXfade({
      frames, durations, bgm, voiceClips, out, fps,
      fadeOut, bgmGain, bgmCrossfade, transition, transitionDuration, total,
    })
  }
  const workDir = dirname(frames[0])
  const listFile = join(workDir, 'concat.txt')
  writeFileSync(listFile, buildConcatList(frames, durations), 'utf-8')
  mkdirSync(dirname(out), { recursive: true })

  const args = ['-y', '-f', 'concat', '-safe', '0', '-i', listFile]

  /*
   * 音轨有三种情形：无音轨 / 只有 BGM / 配音（可叠 BGM）。
   *
   * 配音这条的关键是**对齐**：镜头时长 = 念白 + 余量，所以每句念白后面要补上
   * 那段余量的静音，否则第 2 句起就会提前于画面响起，越往后错得越多。
   * 用 apad=whole_dur=<镜头时长> 把每句补齐到镜头长度，再首尾相接 concat，
   * 音画就天然同步 —— 不需要算 offset，也就不会有累积误差。
   */
  const filters = []
  const parts = []

  if (voiceClips?.length) {
    voiceClips.forEach((v, i) => {
      args.push('-i', v.file)
      // 输入序号从 1 开始（0 是画面 concat）
      filters.push(`[${i + 1}:a]apad=whole_dur=${durations[i].toFixed(3)}[v${i}]`)
      parts.push(`[v${i}]`)
    })
    filters.push(`${parts.join('')}concat=n=${voiceClips.length}:v=0:a=1[voice]`)
  }

  if (bgm) {
    const bgmIdx = 1 + (voiceClips?.length || 0)
    args.push('-i', bgm)
    const fadeStart = Math.max(0, total - fadeOut)
    /*
     * 有配音时 BGM 必须压下去,否则念白被盖住。0.22 是实测值:
     * 念白段 -17~-21dB、间隙纯 BGM -25~-33dB,差 6~13dB,人声清晰在前。
     * 但「垫多重」是听感问题,不同曲子差异很大,所以留 --bgm-gain 让人调。
     */
    const gain = bgmGain !== undefined && !Number.isNaN(bgmGain)
      ? bgmGain
      : (voiceClips?.length ? 0.22 : 1)
    /*
     * 短曲子循环接缝:把 BGM 头 K 秒和尾 K 秒叠成一段平滑版,再拿去 loop。
     * 这样循环点落在 crossfade 的过渡区,听感自然不爆音。
     * K=0 时退回旧行为(直接 loop,适合曲子够长不需要平滑的场景)。
     */
    const xfade = bgmCrossfade && bgmCrossfade > 0 ? bgmCrossfade : 0
    const bgmFilter = xfade > 0
      ? `[${bgmIdx}:a]asplit=2[b1][b2];` +
        `[b1]atrim=0:${xfade},asetpts=PTS-STARTPTS[h];` +
        `[b2]atrim=start=${xfade},asetpts=PTS-STARTPTS[t];` +
        `[h][t]acrossfade=d=${xfade}:c1=tri:c2=tri[s];` +
        `[s]aloop=loop=-1:size=2e9,atrim=0:${total},volume=${gain},afade=t=out:st=${fadeStart}:d=${fadeOut}[bgm]`
      : `[${bgmIdx}:a]aloop=loop=-1:size=2e9,atrim=0:${total},volume=${gain},afade=t=out:st=${fadeStart}:d=${fadeOut}[bgm]`
    filters.push(bgmFilter)
  }

  if (voiceClips?.length && bgm) {
    // duration=first：以配音轨为准，避免 BGM 把片子拖长
    filters.push('[voice][bgm]amix=inputs=2:duration=first:dropout_transition=0[a]')
  } else if (voiceClips?.length) {
    filters.push('[voice]anull[a]')
  } else if (bgm) {
    filters.push('[bgm]anull[a]')
  }

  if (filters.length) {
    args.push('-filter_complex', filters.join(';'), '-map', '0:v', '-map', '[a]',
      '-c:a', 'aac', '-b:a', '192k', '-shortest')
  }

  args.push(
    '-vsync', 'cfr', '-r', String(fps),
    '-c:v', 'libx264', '-preset', 'medium', '-crf', '20',
    // yuv420p + even 尺寸：不加这两条，某些播放器与微信/小红书转码会拒或出绿边
    '-pix_fmt', 'yuv420p',
    '-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2',
    '-movflags', '+faststart',
    out,
  )

  try {
    await run('ffmpeg', args, { maxBuffer: 32 * 1024 * 1024 })
  } catch (e) {
    const tail = String(e.stderr || e.message).split('\n').slice(-14).join('\n')
    throw new Error(`ffmpeg 合成失败:\n${tail}`)
  }
  return { out, duration: total }
}

/**
 * 转场合成 —— 不用 concat demuxer,改用 filter_complex + xfade 滤镜链。
 *
 * 为什么不能用 concat + 单独做转场:
 *   concat demuxer 是"硬拼",每对相邻镜的衔接是 1 帧硬切。
 *   xfade 需要两个有重叠的视频流,只能在 filter_complex 里把每帧 PNG 先
 *   loop+trim 成不等时长小视频,再 xfade 串联。
 *
 * 音画时长:
 *   视频流 = N 镜时长 - (N-1) * 转场时长  (xfade 重叠)
 *   配音流 = N 镜时长(不变)
 *   amix 用 video 当 first,配音尾部 (N-1) * 转场时长 秒被切。
 *   默认转场 0.4s,前几镜的配音余量(默认 0.45s)是静音,够覆盖,
 *   末镜配音的尾巴可能被切 —— 用户主动加余量即可。
 */
async function composeWithXfade({
  frames, durations, bgm, voiceClips, out, fps,
  fadeOut, bgmGain, bgmCrossfade, transition, transitionDuration, total,
}) {
  const xfadeType = TRANSITIONS[transition]
  if (!xfadeType) {
    throw new Error(
      `不支持的转场 "${transition}"。可用: ${[...Object.keys(TRANSITIONS), 'none'].join(', ')}`,
    )
  }
  const X = transitionDuration
  if (!(X > 0)) throw new Error('转场时长必须 > 0')

  // 输入:每帧 PNG(loop 1,靠 filter 控时长)+ 每镜配音 + BGM
  const args = ['-y']
  frames.forEach((f) => {
    // -loop 1:让 ffmpeg 把 PNG 当视频读,而不是 image2 默认 1 帧就结束
    // -framerate fps:loop 输出帧率(影响 xfade 的帧精度)
    // -t:限定 input 时长(防止 ffmpeg 觉得 input 无穷)
    args.push('-loop', '1', '-framerate', String(fps), '-t', '999999', '-i', f)
  })
  const voiceStart = frames.length
  voiceClips?.forEach((v) => args.push('-i', v.file))
  const bgmIdx = voiceStart + (voiceClips?.length || 0)
  if (bgm) args.push('-i', bgm)

  // 1) 每帧 PNG → 视频流(loop+trim+setpts,固定到该镜时长)
  const filters = []
  for (let i = 0; i < frames.length; i++) {
    // loop 的 size 参数上限 32767(帧数),不指定 = 0 = 无限,这里直接不写
    filters.push(
      `[${i}:v]scale=trunc(iw/2)*2:trunc(ih/2)*2,fps=${fps},` +
      `loop=loop=-1:start=0,trim=0:${durations[i].toFixed(3)},` +
      `setpts=PTS-STARTPTS,format=yuv420p[v${i}]`,
    )
  }

  // 2) xfade 串联:每对相邻镜重叠 X 秒
  //    第 i 对(i=1..N-1)的 offset = sum(durations[0..i-1]) - i*X
  let accDur = durations[0]
  let prev = '[v0]'
  for (let i = 1; i < frames.length; i++) {
    const last = i === frames.length - 1
    const outLabel = last ? '[vout]' : `[vt${i}]`
    const offset = accDur - X
    filters.push(
      `${prev}[v${i}]xfade=transition=${xfadeType}:duration=${X}:offset=${offset.toFixed(3)}${outLabel}`,
    )
    prev = outLabel
    accDur += durations[i] - X
  }

  // 3) 配音:apad 到该镜时长,concat 起来
  if (voiceClips?.length) {
    voiceClips.forEach((_, i) => {
      filters.push(
        `[${voiceStart + i}:a]apad=whole_dur=${durations[i].toFixed(3)},asetpts=PTS-STARTPTS[a${i}]`,
      )
    })
    filters.push(
      `${voiceClips.map((_, i) => `[a${i}]`).join('')}concat=n=${voiceClips.length}:v=0:a=1[voice]`,
    )
  }

  // 4) BGM(同主路径)
  if (bgm) {
    const fadeStart = Math.max(0, accDur - fadeOut)
    const gain = bgmGain !== undefined && !Number.isNaN(bgmGain)
      ? bgmGain
      : (voiceClips?.length ? 0.22 : 1)
    const xf = bgmCrossfade && bgmCrossfade > 0 ? bgmCrossfade : 0
    const bgmFilter = xf > 0
      ? `[${bgmIdx}:a]asplit=2[b1][b2];` +
        `[b1]atrim=0:${xf},asetpts=PTS-STARTPTS[h];` +
        `[b2]atrim=start=${xf},asetpts=PTS-STARTPTS[t];` +
        `[h][t]acrossfade=d=${xf}:c1=tri:c2=tri[s];` +
        `[s]aloop=loop=-1:size=2e9,atrim=0:${accDur.toFixed(3)},volume=${gain},afade=t=out:st=${fadeStart.toFixed(3)}:d=${fadeOut}[bgm]`
      : `[${bgmIdx}:a]aloop=loop=-1:size=2e9,atrim=0:${accDur.toFixed(3)},volume=${gain},afade=t=out:st=${fadeStart.toFixed(3)}:d=${fadeOut}[bgm]`
    filters.push(bgmFilter)
  }

  // 5) amix(用 video 当 first,配音尾部被切的那 (N-1)*X 秒是静音期)
  if (voiceClips?.length && bgm) {
    filters.push('[voice][bgm]amix=inputs=2:duration=first:dropout_transition=0[a]')
  } else if (voiceClips?.length) {
    filters.push('[voice]anull[a]')
  } else if (bgm) {
    filters.push('[bgm]anull[a]')
  } else {
    // 无音轨也要有 [a] 标签(被 -map 引用),用 anullsrc 生成静音占位
    filters.push('anullsrc=r=44100:cl=stereo[a]')
  }

  args.push(
    '-filter_complex', filters.join(';'),
    '-map', '[vout]', '-map', '[a]',
    '-c:a', 'aac', '-b:a', '192k',
    '-c:v', 'libx264', '-preset', 'medium', '-crf', '20',
    '-pix_fmt', 'yuv420p',
    '-r', String(fps),
    '-movflags', '+faststart',
    '-shortest',
    out,
  )

  try {
    await run('ffmpeg', args, { maxBuffer: 32 * 1024 * 1024 })
  } catch (e) {
    const tail = String(e.stderr || e.message).split('\n').slice(-14).join('\n')
    throw new Error(`ffmpeg 合成失败:\n${tail}`)
  }
  return { out, duration: accDur }
}

/** 读一个媒体文件的秒数，用于校验/展示 */
export async function probeDuration(file) {
  const { stdout } = await run('ffprobe', [
    '-v', 'quiet', '-show_entries', 'format=duration', '-of', 'csv=p=0', file,
  ])
  return parseFloat(stdout.trim())
}
