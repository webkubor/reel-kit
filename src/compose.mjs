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
import { buildMixChain } from './audio-mix.mjs'
import { DEFAULT_BGM_GAIN } from './settings.mjs'

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

/* concat demuxer 需要末帧之后的时间戳才能呈现最后一镜的时长。
 * 重复末帧提供该时间戳，输出端再用总时长截断，避免额外延长。
 */
function buildConcatList(frames, durations) {
  const lines = []
  frames.forEach((f, i) => {
    lines.push(`file '${f.replace(/'/g, "'\\''")}'`)
    lines.push(`duration ${durations[i]}`)
  })
  lines.push(`file '${frames.at(-1).replace(/'/g, "'\\''")}'`)
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
 * @param {number}   [opts.voiceGain]   人声增益,默认 1.35(见 src/audio-mix.mjs)
 * @param {boolean}  [opts.duck]        人声是否侧链压低配乐,默认 true
 * @param {boolean}  [opts.normalize]   是否做响度归一化到 -14 LUFS,默认 true
 */
export async function compose({
  frames, durations, bgm, voiceClips, out, fps = 30, fadeOut, bgmGain, bgmCrossfade,
  transition = 'none', transitionDuration = 0.4, voiceGain, duck, normalize,
}) {
  if (fadeOut === undefined || Number.isNaN(fadeOut)) fadeOut = 1.5
  if (!frames.length) throw new Error('没有帧可合成')
  if (frames.length !== durations.length) throw new Error('frames 与 durations 长度不一致')
  if (voiceClips && voiceClips.length !== frames.length) {
    throw new Error('voiceClips 与 frames 长度不一致')
  }

  if (!Number.isInteger(fps) || fps < 1 || fps > 60) throw new Error('帧率必须是 1–60 的整数')
  if (durations.some(d => !Number.isFinite(d) || d <= 0)) throw new Error('每镜时长必须是正数')
  if (transition !== 'none' && !TRANSITIONS[transition]) throw new Error(`不支持的转场 "${transition}"`)
  const total = durations.reduce((a, b) => a + b, 0)

  // 连续动画片段和多镜转场共用同一条时间轴。
  if ((transition !== 'none' && frames.length > 1) || frames.some(f => f.endsWith('.mp4'))) {
    return composeWithXfade({
      frames, durations, bgm, voiceClips, out, fps,
      fadeOut, bgmGain, bgmCrossfade, transition, transitionDuration, total,
      voiceGain, duck, normalize,
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
     * 有念白时 BGM 必须压下去,否则念白被盖住。
     * 默认值来自 src/settings.mjs（DEFAULT_BGM_GAIN = 0.12）——
     * 全工程只有一个默认,不在这里另抄一份。
     * 「垫多重」是听感问题,不同曲子差异很大,所以留 --bgm-gain 让人调。
     */
    const gain = bgmGain !== undefined && !Number.isNaN(bgmGain)
      ? bgmGain
      : (voiceClips?.length ? DEFAULT_BGM_GAIN : 1)
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

  const hasAudio = Boolean(voiceClips?.length || bgm)

  if (hasAudio) {
    filters.push(...buildMixChain({
      voiceLabel: voiceClips?.length ? '[voice]' : null,
      bgmLabel: bgm ? '[bgm]' : null,
      voiceGain: voiceGain ?? undefined,
      duck: duck !== false,
      normalize: normalize !== false,
      outLabel: '[a]',
    }))
    args.push('-filter_complex', filters.join(';'), '-map', '0:v', '-map', '[a]',
      '-c:a', 'aac', '-b:a', '192k', '-shortest')
  } else {
    // 无音轨：不挂音频滤镜，输出里没有音频流（保持旧行为）
    if (filters.length) args.push('-filter_complex', filters.join(';'))
    args.push('-map', '0:v')
  }

  args.push(
    // fps 滤镜在打包的 FFmpeg 4.4 与新版本均可用，统一产生恒定帧率。
    '-c:v', 'libx264', '-preset', 'medium', '-crf', '20',
    // yuv420p + even 尺寸：不加这两条，某些播放器与微信/小红书转码会拒或出绿边
    '-pix_fmt', 'yuv420p',
    '-vf', `fps=${fps},scale=trunc(iw/2)*2:trunc(ih/2)*2`,
    '-movflags', '+faststart', '-t', String(total + 1 / fps),
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
 * 每镜按原时长播放；离场镜头额外保持 X 秒，与下一镜开头重叠。
 * 转场从下一句旁白的起点开始，不缩短总片长，也不移动任何一句旁白。
 * 单镜和无音轨同样有效；动画输入保留逐帧画面，不重复首帧。
 */
async function composeWithXfade({
  frames, durations, bgm, voiceClips, out, fps,
  fadeOut, bgmGain, bgmCrossfade, transition, transitionDuration, total,
  voiceGain, duck, normalize,
}) {
  const xfadeType = TRANSITIONS[transition]
  if (xfadeType && (!Number.isFinite(transitionDuration) || transitionDuration <= 0)) throw new Error('转场时长必须 > 0')
  // 短镜头也要能出片；转场最多占最短一镜的一半，且按帧对齐。
  const X = xfadeType && frames.length > 1
    ? Math.floor(Math.min(transitionDuration, ...durations.map(d => d / 2)) * fps) / fps : 0

  const args = ['-y', '-filter_complex_threads', '1']
  frames.forEach((f, i) => {
    if (!f.endsWith('.mp4')) args.push('-loop', '1', '-framerate', String(fps))
    args.push('-t', String(durations[i] + X + 1 / fps), '-i', f)
  })
  const voiceStart = frames.length
  voiceClips?.forEach((v) => args.push('-i', v.file))
  const bgmIdx = voiceStart + (voiceClips?.length || 0)
  if (bgm) args.push('-i', bgm)

  const filters = []
  for (let i = 0; i < frames.length; i++) {
    const tail = i < frames.length - 1 ? X : 0
    filters.push(
      `[${i}:v]scale=trunc(iw/2)*2:trunc(ih/2)*2,fps=${fps},` +
      `trim=0:${durations[i].toFixed(6)},setpts=PTS-STARTPTS,` +
      `tpad=stop_mode=clone:stop_duration=${tail + 1 / fps},trim=0:${(durations[i] + tail).toFixed(6)},` +
      `settb=AVTB,format=yuv420p[v${i}]`,
    )
  }

  let boundary = durations[0]
  let prev = '[v0]'
  if (X > 0) {
    for (let i = 1; i < frames.length; i++) {
      const outLabel = `[vt${i}]`
      filters.push(`${prev}[v${i}]xfade=transition=${xfadeType}:duration=${X}:offset=${boundary.toFixed(6)}${outLabel}`)
      prev = outLabel
      boundary += durations[i]
    }
  } else if (frames.length > 1) {
    filters.push(`${frames.map((_, i) => `[v${i}]`).join('')}concat=n=${frames.length}:v=1:a=0[vout]`)
    prev = '[vout]'
  }
  const accDur = total

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
      : (voiceClips?.length ? DEFAULT_BGM_GAIN : 1)
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

  // 5) 无音轨时不创建未消费的音频输出。
  if (voiceClips?.length || bgm) filters.push(...buildMixChain({
    voiceLabel: voiceClips?.length ? '[voice]' : null,
    bgmLabel: bgm ? '[bgm]' : null,
    voiceGain: voiceGain ?? undefined,
    duck: duck !== false,
    normalize: normalize !== false,
    outLabel: '[a]',
  }))

  args.push('-filter_complex', filters.join(';'), '-map', prev)
  if (voiceClips?.length || bgm) args.push('-map', '[a]', '-c:a', 'aac', '-b:a', '192k')
  args.push(
    '-c:v', 'libx264', '-preset', 'medium', '-crf', '20',
    '-pix_fmt', 'yuv420p', '-r', String(fps), '-movflags', '+faststart',
    '-t', String(total), out,
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
