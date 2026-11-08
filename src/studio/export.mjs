import { mkdtempSync, readFileSync, writeFileSync, rmSync, renameSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { command } from './process.mjs'

const cli = fileURLToPath(new URL('../../bin/reel.mjs', import.meta.url))
export async function exportVideo(input, store, progress) {
  const assets = input.assetIds.map(id => store.asset(id, input.projectId))
  if (!assets.length) throw new Error('先选择要合成的图片或视频')
  const audio = input.audioId ? store.asset(input.audioId, input.projectId) : null
  if (audio && !audio.mime.startsWith('audio/')) throw new Error('配音必须是音频素材')
  const temp = mkdtempSync(join(tmpdir(), 'reel-export-'))
  const out = join(temp, 'export.mp4')
  try {
    if (assets.every(a => a.mime.startsWith('image/'))) {
      const args = [cli, 'make', '--template', input.template, '--assets', assets.map(a => store.path(a)).join(','), '--out', out,
        '--title', input.title || store.project(input.projectId).name, '--per-shot', String(input.duration), '--size', input.ratio === '16:9' ? '1920x1080' : '1080x1920']
      if (input.captions?.trim()) {
        const lines = input.captions.trim().split('\n').filter(line => line.trim())
        if (lines.length !== assets.length) throw new Error('每张图片需要一行字幕；也可以留空')
        const captions = join(temp, 'captions.txt'); writeFileSync(captions, lines.join('\n'))
        args.push('--caps', captions)
      }
      progress('正在用 reel-kit 渲染并合成图片…')
      await command(process.execPath, args, { timeout: 900000 })
      if (audio) {
        const voiced = join(temp, 'voiced.mp4')
        await command('ffmpeg', ['-y', '-i', out, '-i', store.path(audio), '-filter_complex', '[1:a]apad[a]', '-map', '0:v:0', '-map', '[a]', '-c:v', 'copy', '-c:a', 'aac', '-shortest', voiced])
        renameSync(voiced, out)
      }
    } else if (assets.every(a => a.mime === 'video/mp4')) {
      progress('正在统一视频尺寸并拼接…')
      const [w, h] = input.ratio === '16:9' ? [1920, 1080] : [1080, 1920]
      const list = []
      for (let i = 0; i < assets.length; i++) {
        const clip = join(temp, `${i}.mp4`)
        // Preserve source duration and sound; standardise every clip for concat.
        const probe = JSON.parse(await command('ffprobe', ['-v', 'error', '-show_streams', '-of', 'json', store.path(assets[i])]))
        const hasAudio = probe.streams.some(s => s.codec_type === 'audio')
        const args = ['-y', '-i', store.path(assets[i])]
        if (!hasAudio) args.push('-f', 'lavfi', '-i', 'anullsrc=r=48000:cl=stereo')
        args.push('-map', '0:v:0', '-map', hasAudio ? '0:a:0' : '1:a:0', '-vf', `scale=${w}:${h}:force_original_aspect_ratio=decrease,pad=${w}:${h}:(ow-iw)/2:(oh-ih)/2,setsar=1`, '-r', '30', '-c:v', 'libx264', '-preset', 'fast', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-ar', '48000', '-ac', '2', '-shortest', clip)
        await command('ffmpeg', args)
        list.push(`file '${clip}'`)
      }
      const concat = join(temp, 'clips.txt'); writeFileSync(concat, list.join('\n'))
      const args = ['-y', '-f', 'concat', '-safe', '0', '-i', concat]
      if (audio) args.push('-i', store.path(audio), '-filter_complex', '[1:a]apad[a]', '-map', '0:v:0', '-map', '[a]', '-c:v', 'copy', '-c:a', 'aac', '-shortest')
      else args.push('-c', 'copy')
      args.push('-movflags', '+faststart', out)
      await command('ffmpeg', args)
    } else throw new Error('一次导出请选择同类型的图片或视频；音频在配音栏单独选择')
    const probe = JSON.parse(await command('ffprobe', ['-v', 'error', '-show_streams', '-show_format', '-of', 'json', out]))
    if (!probe.streams?.some(s => s.codec_type === 'video') || !(Number(probe.format?.duration) >= 0.5) || (audio && !probe.streams?.some(s => s.codec_type === 'audio'))) {
      throw new Error('导出文件缺少视频、配音或有效时长，请检查素材后重试')
    }
    const bytes = readFileSync(out)
    return store.addAsset(input.projectId, `${input.title || '成片'}.mp4`, 'video/mp4', bytes, { source: 'reel-kit', exported: true })
  } finally { rmSync(temp, { recursive: true, force: true }) }
}
