/**
 * 知识口播 / 观点解读流水线。
 *
 * 为什么单独一条流水线，而不是让用户去「合成」页自己拼：
 *   口播视频的难点不在 ffmpeg 命令，而在「一句话 = 一镜 + 一段配音 + 一帧画面」这层对齐。
 *   让人自己填三张表（字幕、素材、配音）几乎必错 —— 错一镜，整条片子就废了。
 *   这里把对齐做进流水线：脚本一行一镜，念白生成后按实际语速决定这一镜多长。
 *
 * 与 GitHub 项目推荐流水线共用同一套底座（renderFrames + compose + backend 语音），
 * 差别只在「内容从哪来」：推荐是仓库读出来的，这里是用户自己写的脚本。
 */

import { mkdtempSync, rmSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { command, redact } from './process.mjs'
import { renderFrames } from '../render.mjs'
import { compose, probeDuration } from '../compose.mjs'
import { getStyle } from '../knowledge-styles.mjs'

const esc = s => String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))

/** 可用于口播的模板。project-recommendation 是仓库专用的（版式里写死了仓库信息），不开放。 */
export const TALK_TEMPLATES = ['knowledge-paper', 'knowledge-dark', 'knowledge-gufeng', 'quote']
const WIDTH = 1080
const HEIGHT = 1920

const templateFile = name => fileURLToPath(new URL(`../../templates/${name}.html`, import.meta.url))

/**
 * 没有素材时也要能出片。
 *
 * 新手第一次用往往手里只有一段话、没有任何图。让他先去找图再回来，
 * 大概率就走了。占位图用风格色做一个渐变 —— 至少版面是对的，之后换真图重做即可。
 */
async function placeholderImage(file, style) {
  const c0 = (style.bg || '#ece4d4').replace('#', '0x')
  const c1 = (style.accent1 || '#b23a2e').replace('#', '0x')
  await command('ffmpeg', ['-v', 'error', '-y', '-f', 'lavfi',
    '-i', `gradients=size=${WIDTH}x${HEIGHT}:c0=${c0}:c1=${c1}:nb_colors=2`,
    '-frames:v', '1', file])
  return file
}

export async function makeTalkVideo(input, store, backend, progress, report = () => {}) {
  const lines = (input.captions || '').split('\n').map(s => s.trim()).filter(Boolean)
  if (!lines.length || lines.length > 8 || lines.some(s => s.length > 80)) {
    throw new Error('请填写 1–8 行脚本，每行不超过 80 字；过长的句子请拆成多镜')
  }
  const style = getStyle(input.style) || { bg: '#ece4d4', accent1: '#b23a2e', accent2: '#3c6e71', template: 'knowledge-paper' }
  const templateName = TALK_TEMPLATES.includes(input.template) ? input.template : (TALK_TEMPLATES.includes(style.template) ? style.template : 'knowledge-paper')
  const template = templateFile(templateName)

  const assets = (input.assetIds || []).map(id => store.asset(id, input.projectId))
  if (assets.some(a => !a.mime.startsWith('image/'))) throw new Error('画面素材必须是图片')

  const temp = mkdtempSync(join(tmpdir(), 'reel-talk-'))
  try {
    // 每个环节单独上报：失败时能看出是「配音没出来」还是「画面没渲染出来」
    const step = (def, run) => {
      const started = Date.now()
      report({ ...def, status: 'running' })
      const settle = result => {
        const payload = {}, meta = {}
        for (const [key, value] of Object.entries(result || {})) (key.startsWith('_') ? meta : payload)[key] = value
        report({ ...def, ...payload, status: 'done', ms: Date.now() - started })
        return meta
      }
      return Promise.resolve().then(run).then(settle, error => {
        report({ ...def, status: 'failed', error: redact(error.message), ms: Date.now() - started })
        throw error
      })
    }

    await step({ id: 'config', label: '确认配置', origin: 'code', engine: '参数校验', detail: `${templateName} · ${lines.length} 镜 · ${WIDTH}×${HEIGHT}` }, () => ({}))

    let images = assets.map(asset => store.path(asset))
    if (!images.length) {
      progress('没有画面素材，先用风格色生成占位图…')
      images = await Promise.all(lines.map((_, i) => placeholderImage(join(temp, `placeholder-${i}.png`), style)))
      await step({ id: 'placeholder', label: '生成占位画面', origin: 'code', engine: 'ffmpeg 渐变', detail: `${images.length} 张占位图，之后可换真图重做` }, async () => ({}))
    }

    // 逐句配音：音色相同、语气相同，只是念的内容不同
    const voiceClips = []
    for (let i = 0; i < lines.length; i++) {
      progress(`正在为第 ${i + 1}/${lines.length} 句生成配音…`)
      const meta = await step({ id: `voice-${i}`, label: `配音 ${i + 1}/${lines.length}`, origin: 'ai', engine: input.voiceBackend === 'minimax' ? 'MiniMax TTS' : 'VoxFlow 本地语音', detail: lines[i].slice(0, 40) }, async () => {
        const asset = await backend.generate({
          kind: 'speech', projectId: input.projectId,
          persona: input.persona, prompt: lines[i], tone: input.tone || '',
          backend: input.voiceBackend || 'voxflow', model: input.voiceModel || undefined,
          voiceId: input.voiceId || undefined, profile: input.profile,
        }, () => {})
        if (!asset?.mime?.startsWith('audio/')) throw new Error(`第 ${i + 1} 句配音没有生成成功`)
        return { _file: store.path(asset), _duration: await probeDuration(store.path(asset)), assetId: asset.id, origin: 'ai', engine: asset.engine || '语音合成', detail: lines[i].slice(0, 40) }
      })
      voiceClips.push({ file: meta._file, duration: meta._duration })
    }

    const pause = Number.isFinite(Number(input.pause)) ? Math.max(0, Math.min(2, Number(input.pause))) : 0.45
    const durations = voiceClips.map(v => v.duration + pause)

    progress('正在渲染画面帧…')
    const { _frames: frames } = await step({ id: 'frames', label: '渲染画面帧', origin: 'code', engine: 'playwright 本地渲染', detail: `${lines.length} 帧 · ${WIDTH}×${HEIGHT} · ${templateName}` }, async () => ({
      _frames: await renderFrames({
        template,
        shots: lines.map((caption, i) => ({ image: images[i % images.length], caption: esc(caption) })),
        vars: {
          title: esc(input.title || ''), subtitle: esc(input.subtitle || ''),
          source: esc(input.source || ''), footer: esc(input.footer || ''),
          bg: style.bg, accent1: style.accent1, accent2: style.accent2,
          width: WIDTH, height: HEIGHT,
        },
        width: WIDTH, height: HEIGHT, outDir: join(temp, 'frames'),
      }),
    }))

    progress('正在合成成片…')
    const out = join(temp, 'talk.mp4')
    const bgm = input.bgmId ? store.asset(input.bgmId, input.projectId) : undefined
    const bgmGain = Number.isFinite(Number(input.bgmGain)) ? Math.max(0, Math.min(1, Number(input.bgmGain))) : 0.12
    const { _duration: outputDuration } = await step({ id: 'compose', label: '合成成片', origin: 'code', engine: 'ffmpeg 合成', detail: `${bgm ? `BGM《${bgm.name}》音量 ${Math.round(bgmGain * 100)}% · ` : '无 BGM · '}${WIDTH}×${HEIGHT}` }, async () => {
      await compose({ frames, durations, voiceClips, bgm: bgm && store.path(bgm), bgmGain, bgmCrossfade: 0, out })
      return { _duration: await probeDuration(out) }
    })

    const bytes = readFileSync(out)
    return store.addAsset(input.projectId, `${input.title || '口播视频'}.mp4`, 'video/mp4', bytes, {
      source: 'reel-kit', origin: 'code', engine: '本地合成（渲染 + ffmpeg）',
      width: WIDTH, height: HEIGHT, duration: outputDuration, hasAudio: true,
      template: templateName, style: input.style || '', script: lines,
    })
  } finally { rmSync(temp, { recursive: true, force: true }) }
}
