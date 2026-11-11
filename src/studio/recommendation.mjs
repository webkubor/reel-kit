import { mkdtempSync, rmSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { command, redact } from './process.mjs'
import { renderFrames } from '../render.mjs'
import { compose, probeDuration } from '../compose.mjs'
import { recommendationSettings, reusableVoice } from './video-settings.mjs'
import { voiceChannelInfo } from './voice-origin.mjs'

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
  const short = value => value.split(/[。！？]/).find(Boolean)?.slice(0, 80) || ''
  const lines = text.split(/\r?\n/).filter(line => /^-\s+\*\*/.test(line)).map(clean).map(short).filter(line => line.length >= 8).slice(0, 4)
  const summary = short(clean((info.description || '').split(' / ')[0]))
  const captions = [summary || `这是开源项目 ${info.name.slice(0, 55)}。`, ...lines, '想了解更多，请访问项目主页。'].slice(0, 6)
  const candidates = (tree.tree || []).filter(x => x.type === 'blob' && /\.(png|jpe?g|webp)$/i.test(x.path) && x.size > 1000 && x.size < 500000 && /(?:showcase|screenshots|preview)/i.test(x.path)).sort((a, b) => Number(/showcase|preview/i.test(b.path)) - Number(/showcase|preview/i.test(a.path))).slice(0, 4)
  const images = []
  for (const item of candidates) {
    try {
      const result = await gh(`repos/${repo}/contents/${item.path.split('/').map(encodeURIComponent).join('/')}`)
      if (result.encoding !== 'base64') continue
      const bytes = Buffer.from(result.content, 'base64')
      if (!bytes.length || bytes.length > 500000) continue
      const ext = item.path.split('.').pop().toLowerCase()
      images.push(store.publicAsset(store.addAsset(projectId, item.path.split('/').pop(), ext === 'png' ? 'image/png' : ext === 'webp' ? 'image/webp' : 'image/jpeg', bytes, { source: `GitHub ${repo}`, origin: 'import', engine: 'GitHub 抓取', sourcePath: item.path })))
    } catch { /* 图片不可用时继续处理其他素材 */ }
  }
  return { repo, title: info.name, description: summary, captions, images, source: `https://github.com/${repo}` }
}
export async function makeRecommendation(input, store, backend, progress, report = () => {}, signal = undefined) {
  const lines = input.captions.split('\n').map(s => s.trim()).filter(Boolean)
  if (!lines.length || lines.length > 8 || lines.some(s => s.length > 80)) throw new Error('请填写 1–8 行旁白，每行不超过 80 字；过长的旁白请拆分为多镜')
  if (!input.assetIds.length) throw new Error('请先读取项目并选择至少一张项目画面')
  const images = input.assetIds.map(id => store.asset(id, input.projectId))
  if (images.some(a => !a.mime.startsWith('image/'))) throw new Error('项目画面必须是图片')
  const settings = recommendationSettings(input)
  // 每个环节上报为独立步骤：产物、来源、失败原因都留在任务里，方便回看「哪一步不对」。
  // run() 返回值里 `_` 开头的键是内部数据（文件/帧列表），不会进入步骤上报。
  const step = (def, run) => {
    const started = Date.now()
    report({ ...def, status: 'running' })
    const settle = result => {
      const payload = {}, meta = {}
      for (const [key, value] of Object.entries(result || {})) (key.startsWith('_') ? meta : payload)[key] = value
      report({ ...def, ...payload, status: 'done', ms: Date.now() - started })
      return meta
    }
    return Promise.resolve().then(run).then(settle, error => { report({ ...def, status: error?.cancelled ? 'canceled' : 'failed', error: redact(error.message), ms: Date.now() - started }); throw error })
  }
  const temp = mkdtempSync(join(tmpdir(), 'reel-recommend-'))
  try {
    await step({ id: 'config', label: '确认配置', origin: 'code', engine: '参数校验', detail: `平台 ${settings.name} · ${settings.width}×${settings.height} · 语速 ${settings.speed} · 停顿 ${settings.pause}s` }, () => ({}))
    const voiceClips = []
    input.voiceIds ||= lines.map(() => '')
    const narration = settings.voiceSource === 'audio' ? store.asset(input.narrationId, input.projectId) : null
    const narrationDuration = narration ? await probeDuration(store.path(narration)) : 0
    const totalChars = lines.reduce((sum, line) => sum + line.length, 0)
    let offset = 0
    for (let i = 0; i < lines.length; i++) {
      const label = `旁白 ${i + 1}/${lines.length}`
      progress(`正在处理第 ${i + 1}/${lines.length} 句旁白…`)
      const voiceBackend = settings.voiceBackend || 'edge'
      const voiceOrigin = narration ? 'import' : voiceChannelInfo(voiceBackend).origin
      const voiceEngine = narration ? '整段素材切分' : voiceChannelInfo(voiceBackend).engine
      const meta = await step({ id: `voice-${i}`, label, origin: voiceOrigin, engine: voiceEngine, detail: lines[i].slice(0, 40) }, async () => {
        if (narration) {
          const end = i === lines.length - 1 ? narrationDuration : offset + narrationDuration * lines[i].length / totalChars
          const file = join(temp, `voice-${i}.wav`)
          await command('ffmpeg', ['-v', 'error', '-i', store.path(narration), '-af', `atrim=start=${offset}:end=${end},asetpts=PTS-STARTPTS,atempo=${settings.speed}`, file], { signal })
          offset = end
          return { _file: file, _duration: await probeDuration(file), detail: `${i === 0 ? '开头' : `${Math.round(offset - (end - offset))}s 起`}切分 · ${lines[i].slice(0, 36)}` }
        }
        let asset = input.voiceIds[i] ? store.asset(input.voiceIds[i], input.projectId) : null
        const chosen = Boolean(asset)
        let reused = false
        let generated = false
        // scenes 模式：用户逐句指定了音频就直接用；tts 模式：音色/文案/表达不匹配的旧配音失效，找匹配的或重新生成。
        const voiceInput = { ...input, ...settings, backend: settings.voiceBackend, model: settings.voiceModel, voiceId: settings.voiceId, profile: settings.voiceProfile }
        if (settings.voiceSource === 'tts') {
          if (asset && !reusableVoice(asset, voiceInput, lines[i])) asset = null
          if (!asset) {
            const match = store.state.assets.find(a => a.projectId === input.projectId && reusableVoice(a, voiceInput, lines[i]))
            if (match) { asset = match; reused = true }
          }
        }
        if (!asset) {
          asset = await backend.generate({
            kind: 'speech', projectId: input.projectId,
            persona: input.persona, prompt: lines[i], tone: input.tone || '',
            backend: settings.voiceBackend, model: settings.voiceModel, voiceId: settings.voiceId,
            // 账户也要传：两个 MiniMax 账户是两份额度、两套声音
            profile: settings.voiceProfile,
            // 在线语音直接按语速合成；本地引擎不支持语速参数，交给后面的 atempo 处理。
            speed: ['edge', 'minimax'].includes(voiceBackend) ? settings.speed : 1,
          }, () => {}, { signal })
          generated = true
        }
        if (!asset.mime.startsWith('audio/')) throw new Error(`第 ${i + 1} 句选择的素材不是音频`)
        input.voiceIds[i] = asset.id
        progress(`第 ${i + 1}/${lines.length} 句旁白已就绪`)
        let file = store.path(asset)
        // 在线语音已经在合成时按语速出片；本地引擎不支持语速参数，在这里变速。
        if (settings.speed !== 1 && !['edge', 'minimax'].includes(voiceBackend)) {
          const adjusted = join(temp, `voice-${i}.wav`)
          await command('ffmpeg', ['-v', 'error', '-i', file, '-af', `atempo=${settings.speed}`, adjusted], { signal })
          file = adjusted
        }
        const source = generated ? '新生成' : reused ? '复用已生成' : chosen ? '使用所选素材' : '新生成'
        return { _file: file, _duration: await probeDuration(file), assetId: asset.id, reused, origin: asset.origin || (asset.source === 'VoxFlow' ? 'ai' : 'import'), engine: asset.engine || (asset.source === 'VoxFlow' ? 'VoxFlow 本地语音' : '所选声音素材'), detail: `${source} · ${lines[i].slice(0, 40)}` }
      })
      voiceClips.push({ file: meta._file, duration: meta._duration })
    }
    const durations = voiceClips.map(v => v.duration + (narration ? 0 : settings.pause))
    progress(settings.motion === 'none' ? `正在渲染 ${settings.name} ${settings.width}×${settings.height} 画面…` : `正在逐帧渲染动效画面…`)
    const { _frames: frames } = await step({ id: 'frames', label: '渲染画面帧', origin: 'code', engine: 'playwright 本地渲染', detail: `${lines.length} 镜 · ${settings.width}×${settings.height} · ${settings.imageFit === 'cover' ? '铺满画布' : '完整画面'}${settings.motion === 'none' ? '' : ' · 轻柔运镜'}` }, async () =>
      ({ _frames: await renderFrames({ template, shots: lines.map((caption, i) => ({ image: store.path(images[i % images.length]), caption: esc(caption) })), vars: { title: esc(input.title), subtitle: esc(input.repoUrl), width: settings.width, height: settings.height, fit: settings.imageFit }, width: settings.width, height: settings.height, outDir: join(temp, 'frames'), motion: settings.motion, durations, onProgress: progress, signal }) }))
    progress('正在混合旁白与 BGM，合成成片…')
    const out = join(temp, 'recommendation.mp4')
    const bgm = input.bgmId ? store.asset(input.bgmId, input.projectId) : undefined
    const transitionLabel = settings.transition && settings.transition !== 'none' ? ` · 转场: ${settings.transition}` : ''
    const { _duration: outputDuration } = await step({ id: 'compose', label: '合成成片', origin: 'code', engine: 'ffmpeg 合成', detail: `${bgm ? `BGM《${bgm.name}》音量 ${Math.round(settings.bgmGain * 100)}%` : '无 BGM'} · ${settings.width}×${settings.height}${transitionLabel}` }, async () => {
      await compose({
        frames, durations, voiceClips, bgm: bgm && store.path(bgm),
        bgmGain: settings.bgmGain, bgmCrossfade: 0,
        voiceGain: settings.voiceGain, duck: settings.duck, normalize: settings.normalize,
        transition: settings.transition, transitionDuration: settings.transitionDuration,
        out, signal,
      })
      const result = await command('ffprobe', ['-v', 'error', '-show_streams', '-show_format', '-of', 'json', out])
      const probe = JSON.parse(result), video = probe.streams?.find(s => s.codec_type === 'video')
      if (!probe.streams?.some(s => s.codec_type === 'audio') || !(Number(probe.format?.duration) > 1) || video?.width !== settings.width || video?.height !== settings.height) throw new Error('导出文件的画幅、音轨或时长校验失败')
      return { _duration: Number(probe.format.duration), detail: `成片 ${Math.round(Number(probe.format.duration) * 10) / 10}s · ${video.width}×${video.height}${bgm ? ' · 含 BGM' : ''}` }
    })
    return store.addAsset(input.projectId, `${input.title} · ${settings.name}.mp4`, 'video/mp4', readFileSync(out), { source: 'GitHub 项目推荐', origin: 'code', engine: '本地合成（渲染 + ffmpeg）', repoUrl: input.repoUrl, platform: settings.platform, width: settings.width, height: settings.height, ratio: settings.ratio, duration: outputDuration, hasAudio: true, settings })
  } finally { rmSync(temp, { recursive: true, force: true }) }
}
