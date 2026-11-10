/**
 * 人物 / 商品介绍流水线。
 *
 * 为什么单独一条流水线，而不是复用「知识口播」：
 *   口播是「文字为主、图片垫底」—— 版式承担叙事，图片只是氛围。
 *   商品介绍正好相反：观众先看东西，再听一句说明。
 *   版式因此反过来：大图打底，文字压在图上，只留一句标题、一句说明、一句收尾。
 *
 * 底座与口播共用同一套流程（逐句配音 → 按实际语速定每镜时长 → 渲染画面帧 → 合成），
 * 差别只在「版式选哪个」和「尺寸随画幅走」。
 *
 * 竖版用 product-presenter（出镜画面满屏 + 底部字幕条），
 * 横版用 landscape-product（左图右文，1920×1080）。
 */

import { mkdtempSync, rmSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { command, redact } from './process.mjs'
import { renderFrames } from '../render.mjs'
import { compose, probeDuration } from '../compose.mjs'

const esc = s => String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))

/** 每种画幅对应的版式与尺寸。 */
export const PRODUCT_LAYOUTS = {
  '9:16': { template: 'product-presenter', width: 1080, height: 1920 },
  '16:9': { template: 'landscape-product', width: 1920, height: 1080 },
}

const templateFile = name => fileURLToPath(new URL(`../../templates/${name}.html`, import.meta.url))

/**
 * 没有素材时的占位图。
 *
 * 与口播的浅色渐变不同，商品介绍默认是深色版式 ——
 * 占位图跟着版式走，缺素材时看着仍像一条正经片子，而不是半成品。
 */
async function placeholderImage(file, layout) {
  const c0 = (layout.template === 'landscape-product' ? '0x14161a' : '0x0d0f11').replace('#', '0x')
  const c1 = '0x2b2f36'
  await command('ffmpeg', ['-v', 'error', '-y', '-f', 'lavfi',
    '-i', `gradients=size=${layout.width}x${layout.height}:c0=${c0}:c1=${c1}:nb_colors=2`,
    '-frames:v', '1', file])
  return file
}

/**
 * 字幕字号按这一句的长度定。
 *
 * 一句话一镜，长短差别很大：统一一个字号，短句显得空，长句直接溢出画面。
 * 这里按字数往下收一档，保证最长的那句也放得下。
 */
function captionSize(text, layout) {
  const n = [...String(text || '')].length
  if (layout.template === 'landscape-product') {
    if (n <= 24) return 44
    if (n <= 40) return 38
    if (n <= 60) return 32
    return 27
  }
  if (n <= 18) return 60
  if (n <= 30) return 52
  if (n <= 45) return 44
  if (n <= 60) return 38
  return 32
}

export async function makeProductVideo(input, store, backend, progress, report = () => {}) {
  const lines = (input.captions || '').split('\n').map(s => s.trim()).filter(Boolean)
  if (!lines.length || lines.length > 8 || lines.some(s => s.length > 80)) {
    throw new Error('请填写 1–8 行介绍词，每行不超过 80 字；过长的句子请拆成多镜')
  }
  const ratio = PRODUCT_LAYOUTS[input.ratio] ? input.ratio : '9:16'
  const layout = PRODUCT_LAYOUTS[ratio]
  const template = templateFile(layout.template)

  const assets = (input.assetIds || []).map(id => store.asset(id, input.projectId))
  if (assets.some(a => !a.mime.startsWith('image/'))) throw new Error('产品或人物画面必须是图片')

  const temp = mkdtempSync(join(tmpdir(), 'reel-product-'))
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

    await step({ id: 'config', label: '确认配置', origin: 'code', engine: '参数校验', detail: `${layout.template} · ${lines.length} 镜 · ${layout.width}×${layout.height}` }, () => ({}))

    let images = assets.map(asset => store.path(asset))
    if (!images.length) {
      progress('没有产品画面，先用深色占位图顶上…')
      images = await Promise.all(lines.map((_, i) => placeholderImage(join(temp, `placeholder-${i}.png`), layout)))
      await step({ id: 'placeholder', label: '生成占位画面', origin: 'code', engine: 'ffmpeg 渐变', detail: `${images.length} 张占位图，之后可换真图重做` }, async () => ({}))
    }

    // 逐句配音：音色相同、语气相同，只是念的内容不同
    const voiceClips = []
    for (let i = 0; i < lines.length; i++) {
      progress(`正在为第 ${i + 1}/${lines.length} 句生成配音…`)
      const meta = await step({ id: `voice-${i}`, label: `配音 ${i + 1}/${lines.length}`, origin: 'ai', engine: input.voiceBackend === 'minimax' ? 'MiniMax TTS' : input.voiceBackend === 'sherpa' ? '本机离线语音' : 'VoxFlow 本地语音', detail: lines[i].slice(0, 40) }, async () => {
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

    const pause = Number.isFinite(Number(input.pause)) ? Math.max(0, Math.min(2, Number(input.pause))) : 0.4
    const durations = voiceClips.map(v => v.duration + pause)

    progress('正在渲染画面帧…')
    const { _frames: frames } = await step({ id: 'frames', label: '渲染画面帧', origin: 'code', engine: 'playwright 本地渲染', detail: `${lines.length} 帧 · ${layout.width}×${layout.height} · ${layout.template}` }, async () => ({
      _frames: await renderFrames({
        template,
        shots: lines.map((caption, i) => ({
          image: images[i % images.length],
          caption: esc(caption),
          vars: { captionSize: captionSize(caption, layout) },
        })),
        vars: {
          title: esc(input.title || ''), subtitle: esc(input.subtitle || ''),
          source: esc(input.source || ''), footer: esc(input.footer || ''),
          bg: ratio === '16:9' ? '#f4f2ee' : '#0d0f11',
          accent1: ratio === '16:9' ? '#c8502f' : '#e8542f', accent2: '#2f6f6a',
          captionSize: 60,
          width: layout.width, height: layout.height,
        },
        width: layout.width, height: layout.height, outDir: join(temp, 'frames'),
      }),
    }))

    progress('正在合成成片…')
    const out = join(temp, 'product.mp4')
    const bgm = input.bgmId ? store.asset(input.bgmId, input.projectId) : undefined
    const bgmGain = Number.isFinite(Number(input.bgmGain)) ? Math.max(0, Math.min(1, Number(input.bgmGain))) : 0.12
    const { _duration: outputDuration } = await step({ id: 'compose', label: '合成成片', origin: 'code', engine: 'ffmpeg 合成', detail: `${bgm ? `BGM《${bgm.name}》音量 ${Math.round(bgmGain * 100)}% · ` : '无 BGM · '}${layout.width}×${layout.height}` }, async () => {
      await compose({ frames, durations, voiceClips, bgm: bgm && store.path(bgm), bgmGain, bgmCrossfade: 0, out })
      return { _duration: await probeDuration(out) }
    })

    const bytes = readFileSync(out)
    return store.addAsset(input.projectId, `${input.title || '产品介绍'}.mp4`, 'video/mp4', bytes, {
      source: 'reel-kit', origin: 'code', engine: '本地合成（渲染 + ffmpeg）',
      width: layout.width, height: layout.height, duration: outputDuration, hasAudio: true,
      template: layout.template, style: '', script: lines, ratio,
    })
  } finally { rmSync(temp, { recursive: true, force: true }) }
}
