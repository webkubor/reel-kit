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

/** ffmpeg concat 列表要求最后一帧重复一次，否则它的 duration 被忽略（ffmpeg 的已知行为） */
function buildConcatList(frames, durations) {
  const lines = []
  frames.forEach((f, i) => {
    lines.push(`file '${f.replace(/'/g, "'\\''")}'`)
    lines.push(`duration ${durations[i]}`)
  })
  lines.push(`file '${frames[frames.length - 1].replace(/'/g, "'\\''")}'`)
  return lines.join('\n') + '\n'
}

/**
 * @param {object} opts
 * @param {string[]} opts.frames     帧路径（有序）
 * @param {number[]} opts.durations  每帧秒数，与 frames 等长
 * @param {string}   [opts.bgm]      背景音乐路径
 * @param {string}   opts.out        输出 mp4
 * @param {number}   [opts.fps]      输出帧率，默认 30
 * @param {number}   [opts.fadeOut]  片尾音频淡出秒数，默认 1.5
 */
export async function compose({ frames, durations, bgm, voiceClips, out, fps = 30, fadeOut = 1.5, bgmGain = 0.22 }) {
  if (!frames.length) throw new Error('没有帧可合成')
  if (frames.length !== durations.length) throw new Error('frames 与 durations 长度不一致')
  if (voiceClips && voiceClips.length !== frames.length) {
    throw new Error('voiceClips 与 frames 长度不一致')
  }

  const total = durations.reduce((a, b) => a + b, 0)
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
    // 有配音时 BGM 压到背景音量，否则念白会被盖住
    const gain = voiceClips?.length ? bgmGain : 1
    filters.push(
      `[${bgmIdx}:a]aloop=loop=-1:size=2e9,atrim=0:${total},volume=${gain},afade=t=out:st=${fadeStart}:d=${fadeOut}[bgm]`,
    )
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

/** 读一个媒体文件的秒数，用于校验/展示 */
export async function probeDuration(file) {
  const { stdout } = await run('ffprobe', [
    '-v', 'quiet', '-show_entries', 'format=duration', '-of', 'csv=p=0', file,
  ])
  return parseFloat(stdout.trim())
}
