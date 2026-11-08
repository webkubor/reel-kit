import { mkdtempSync, rmSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { command } from './process.mjs'
import { renderFrames } from '../render.mjs'
import { compose, probeDuration } from '../compose.mjs'
import { recommendationSettings, reusableVoice } from './video-settings.mjs'

const template = fileURLToPath(new URL('../../templates/project-recommendation.html', import.meta.url))
const esc = s => String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
export function githubRepo(url) {
  let u
  try { u = new URL(url) } catch { throw new Error('请输入完整的 GitHub 项目地址') }
  const parts = u.pathname.split('/').filter(Boolean)
  if (u.protocol !== 'https:' || u.hostname !== 'github.com' || u.username || u['pass' + 'word'] || u.port || parts.length !== 2 || !parts.every(x => /^[\w.-]+$/.test(x))) throw new Error('仅支持 https://github.com/用户名/仓库名')
  return parts.join('/').replace(/\.git$/, '')
}
async function gh(path) { return JSON.parse(await command('gh', ['api', path], { timeout: 30000 })) }
function clean(line) {
  return line.replace(/^\s*[-*+]\s+/, '').replace(/<[^>]*>/g, '').replace(/!\[([^\]]*)\]\([^)]*\)/g, '').replace(/\[([^\]]+)\]\([^)]*\)/g, '$1').replace(/[*`#]/g, '').replace(/\s+/g, ' ').trim()
}
export async function projectDraft(url, projectId, store) {
  const repo = githubRepo(url)
  const [info, file, tree] = await Promise.all([gh(`repos/${repo}`), gh(`repos/${repo}/readme`), gh(`repos/${repo}/git/trees/HEAD?recursive=1`)])
  const text = Buffer.from(file.content || '', 'base64').toString('utf8')
  const short = value => value.split(/[。！？]/).find(Boolean)?.slice(0, 90) || ''
  const lines = text.split(/\r?\n/).filter(line => /^-\s+\*\*/.test(line)).map(clean).map(short).filter(line => line.length >= 8).slice(0, 4)
  const summary = short(clean((info.description || '').split(' / ')[0]))
  const captions = [summary || `这是开源项目 ${info.name}。`, ...lines, `想了解更多，可以访问 GitHub 上的 ${repo}。`].slice(0, 6)
  const candidates = (tree.tree || []).filter(x => x.type === 'blob' && /\.(png|jpe?g|webp)$/i.test(x.path) && x.size > 1000 && x.size < 500000 && /(?:showcase|screenshots|preview)/i.test(x.path)).sort((a, b) => Number(/showcase|preview/i.test(b.path)) - Number(/showcase|preview/i.test(a.path))).slice(0, 4)
  const images = []
  for (const item of candidates) {
    try {
      const result = await gh(`repos/${repo}/contents/${item.path.split('/').map(encodeURIComponent).join('/')}`)
      if (result.encoding !== 'base64') continue
      const bytes = Buffer.from(result.content, 'base64')
      if (!bytes.length || bytes.length > 500000) continue
      const ext = item.path.split('.').pop().toLowerCase()
      images.push(store.publicAsset(store.addAsset(projectId, item.path.split('/').pop(), ext === 'png' ? 'image/png' : ext === 'webp' ? 'image/webp' : 'image/jpeg', bytes, { source: `GitHub ${repo}`, sourcePath: item.path })))
    } catch { /* 图片不可用时继续处理其他素材 */ }
  }
  return { repo, title: info.name, description: summary, captions, images, source: `https://github.com/${repo}` }
}
export async function makeRecommendation(input, store, backend, progress) {
  const lines = input.captions.split('\n').map(s => s.trim()).filter(Boolean)
  if (!lines.length || lines.length > 8 || lines.some(s => s.length > 150)) throw new Error('请填写 1–8 行旁白，每行不超过 150 字')
  if (!input.assetIds.length) throw new Error('请先读取项目并选择至少一张项目画面')
  const images = input.assetIds.map(id => store.asset(id, input.projectId))
  if (images.some(a => !a.mime.startsWith('image/'))) throw new Error('项目画面必须是图片')
  const settings = recommendationSettings(input)
  const temp = mkdtempSync(join(tmpdir(), 'reel-recommend-'))
  try {
    const voiceClips = []
    input.voiceIds ||= lines.map(() => '')
    const narration = settings.voiceSource === 'audio' ? store.asset(input.narrationId, input.projectId) : null
    const narrationDuration = narration ? await probeDuration(store.path(narration)) : 0
    const totalChars = lines.reduce((sum, line) => sum + line.length, 0)
    let offset = 0
    for (let i = 0; i < lines.length; i++) {
      progress(`正在处理第 ${i + 1}/${lines.length} 句旁白…`)
      let file
      if (narration) {
        const end = i === lines.length - 1 ? narrationDuration : offset + narrationDuration * lines[i].length / totalChars
        file = join(temp, `voice-${i}.wav`)
        await command('ffmpeg', ['-v', 'error', '-i', store.path(narration), '-af', `atrim=start=${offset}:end=${end},asetpts=PTS-STARTPTS,atempo=${settings.speed}`, file])
        offset = end
      } else {
        let asset = input.voiceIds[i] ? store.asset(input.voiceIds[i], input.projectId) : null
        if (settings.voiceSource === 'tts' && !reusableVoice(asset, input, lines[i])) {
          asset = store.state.assets.find(a => a.projectId === input.projectId && reusableVoice(a, input, lines[i]))
        }
        if (!asset) asset = await backend.generate({ kind: 'speech', projectId: input.projectId, persona: input.persona, prompt: lines[i], tone: input.tone || '' }, () => {})
        if (!asset.mime.startsWith('audio/')) throw new Error(`第 ${i + 1} 句选择的素材不是音频`)
        input.voiceIds[i] = asset.id
        progress(`第 ${i + 1}/${lines.length} 句旁白已就绪`)
        file = store.path(asset)
        if (settings.speed !== 1) {
          const adjusted = join(temp, `voice-${i}.wav`)
          await command('ffmpeg', ['-v', 'error', '-i', file, '-af', `atempo=${settings.speed}`, adjusted])
          file = adjusted
        }
      }
      voiceClips.push({ file, duration: await probeDuration(file) })
    }
    const durations = voiceClips.map(v => v.duration + (narration ? 0 : settings.pause))
    progress(`正在渲染 ${settings.name} ${settings.width}×${settings.height} 画面…`)
    const frames = await renderFrames({ template, shots: lines.map((caption, i) => ({ image: store.path(images[i % images.length]), caption: esc(caption) })), vars: { title: esc(input.title), subtitle: esc(input.repoUrl), width: settings.width, height: settings.height, fit: settings.imageFit }, width: settings.width, height: settings.height, outDir: join(temp, 'frames') })
    progress('正在混合旁白与 BGM，合成成片…')
    const out = join(temp, 'recommendation.mp4')
    const bgm = input.bgmId ? store.path(store.asset(input.bgmId, input.projectId)) : undefined
    await compose({ frames, durations, voiceClips, bgm, bgmGain: settings.bgmGain, bgmCrossfade: 0, out })
    const result = await command('ffprobe', ['-v', 'error', '-show_streams', '-show_format', '-of', 'json', out])
    const probe = JSON.parse(result), video = probe.streams?.find(s => s.codec_type === 'video')
    if (!probe.streams?.some(s => s.codec_type === 'audio') || !(Number(probe.format?.duration) > 1) || video?.width !== settings.width || video?.height !== settings.height) throw new Error('导出文件的画幅、音轨或时长校验失败')
    return store.addAsset(input.projectId, `${input.title} · ${settings.name}.mp4`, 'video/mp4', readFileSync(out), { source: 'GitHub 项目推荐', repoUrl: input.repoUrl, platform: settings.platform, width: settings.width, height: settings.height, ratio: settings.ratio, settings })
  } finally { rmSync(temp, { recursive: true, force: true }) }
}
