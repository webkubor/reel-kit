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
export async function compose({ frames, durations, bgm, out, fps = 30, fadeOut = 1.5 }) {
  if (!frames.length) throw new Error('没有帧可合成')
  if (frames.length !== durations.length) throw new Error('frames 与 durations 长度不一致')

  const total = durations.reduce((a, b) => a + b, 0)
  const workDir = dirname(frames[0])
  const listFile = join(workDir, 'concat.txt')
  writeFileSync(listFile, buildConcatList(frames, durations), 'utf-8')
  mkdirSync(dirname(out), { recursive: true })

  const args = ['-y', '-f', 'concat', '-safe', '0', '-i', listFile]

  if (bgm) {
    args.push('-i', bgm)
    // BGM 比片子短就循环补满，比片子长就截断到片长；片尾淡出，避免硬切
    const fadeStart = Math.max(0, total - fadeOut)
    args.push(
      '-filter_complex',
      `[1:a]aloop=loop=-1:size=2e9,atrim=0:${total},afade=t=out:st=${fadeStart}:d=${fadeOut}[a]`,
      '-map', '0:v', '-map', '[a]',
      '-c:a', 'aac', '-b:a', '192k', '-shortest',
    )
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
