import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const require = createRequire(import.meta.url)
const __dirname = dirname(fileURLToPath(import.meta.url))
const bundlePath = join(__dirname, 'vendor', 'edge-tts-bundle.cjs')

let edgeBundle = null
function getBundle() {
  if (!edgeBundle) {
    edgeBundle = require(bundlePath)
  }
  return edgeBundle
}

/** 默认精选高保真音色表 */
export const EDGE_VOICES = [
  { id: 'zh-CN-XiaoxiaoNeural', name: '晓晓 · 温暖亲切女声（首选）' },
  { id: 'zh-CN-YunxiNeural', name: '云希 · 阳光自然男声（解说/口播）' },
  { id: 'zh-CN-YunjianNeural', name: '云健 · 影视解说男声（沉稳）' },
  { id: 'zh-CN-YunyangNeural', name: '云扬 · 专业男主播（新闻/科技）' },
  { id: 'zh-CN-XiaoyiNeural', name: '晓伊 · 甜美明亮女声' },
  { id: 'en-US-JennyNeural', name: 'Jenny · 纯正英文女声' },
]

/**
 * 将相对语速（0.75x ~ 1.5x）转为 Edge TTS prosody rate 字符串（如 "+10%"、"-15%"）。
 */
export function formatSpeedRate(speed = 1) {
  const num = Number(speed)
  if (!Number.isFinite(num) || num <= 0) return '+0%'
  const clamped = Math.max(0.5, Math.min(2.0, num))
  const pct = Math.round((clamped - 1) * 100)
  return pct >= 0 ? `+${pct}%` : `${pct}%`
}

/**
 * 使用微软 Edge 免费高保真在线神经语音合成音频。
 *
 * 24kHz 采样率，96kbps MP3 单声道格式，免 API Key、零网络账号绑定。
 *
 * @param {Object} opts
 * @param {string} opts.text       合成文本
 * @param {string} [opts.voice]    音色短名，默认 zh-CN-XiaoxiaoNeural
 * @param {number} [opts.speed]    语速倍率（0.75 ~ 1.5），默认 1.0
 * @param {Function} [opts.onProgress] 进度回调
 * @returns {Promise<{ bytes: Buffer, mime: string }>}
 */
export async function edgeSynthesize({
  text,
  voice = 'zh-CN-XiaoxiaoNeural',
  speed = 1,
  onProgress,
} = {}) {
  const trimmed = text?.trim()
  if (!trimmed) throw new Error('合成文本不能为空')

  const { MsEdgeTTS, OUTPUT_FORMAT } = getBundle()
  const tts = new MsEdgeTTS()
  const rate = formatSpeedRate(speed)

  try {
    if (typeof onProgress === 'function') {
      onProgress('正在连接微软 Edge 语音服务…')
    }
    await tts.setMetadata(voice, OUTPUT_FORMAT.AUDIO_24KHZ_96KBITRATE_MONO_MP3)

    const { audioStream } = tts.toStream(trimmed, { rate })
    const chunks = []

    await new Promise((resolve, reject) => {
      audioStream.on('data', chunk => chunks.push(chunk))
      audioStream.on('end', resolve)
      audioStream.on('error', reject)
    })

    const bytes = Buffer.concat(chunks)
    if (!bytes.length) {
      throw new Error('Edge TTS 未返回任何音频字节')
    }

    return { bytes, mime: 'audio/mpeg' }
  } finally {
    try {
      tts.close()
    } catch {
      // 忽略连接关闭异常
    }
  }
}
