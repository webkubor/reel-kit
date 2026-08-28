/**
 * 逐句配音 —— 把 caps 的每一行合成为独立 wav，用于「念白驱动镜头时长」。
 *
 * 为什么每句一个文件而不是整段合成：镜头时长要按**这一句**的念白长度定。
 * 整段合成后再切分，得做静音检测切点，既不准又脆；分句合成天然对齐。
 *
 * 两个后端：
 *   voxcraft（默认）—— 本地 Qwen3-TTS（Base-1.7B，4.2GB 权重）。**一次性成本，
 *                      之后每句合成都不花钱**；且支持声音克隆与音色设计，
 *                      IP 专属嗓音只能靠它。自研开源项目，用自己的东西。
 *   museav          —— 走小米 MiMo API，无需本地模型、秒级出声，
 *                      但按 token 计费。适合临时试验或本地模型未就绪时兜底。
 *
 * 默认选 voxcraft 是**成本决定的**：批量流水线一支片子几十句，
 * API 调用会持续累积，而本地推理是一次性投入。
 */

import { execFile } from 'node:child_process'
import { existsSync, mkdirSync } from 'node:fs'
import { join } from 'node:path'
import { promisify } from 'node:util'

const run = promisify(execFile)

/** museav speak 依赖 MIMO_API_KEY；密钥库里是 secret://mimo/api-key */
const MIMO_SECRET = 'secret://mimo/api-key'

async function speakViaMuseav({ text, out, voice, design, instruction }) {
  const inner = ['speak', text, '--out', out]
  if (voice) inner.push('--voice', voice)
  if (design) inner.push('--design', design)
  if (instruction) inner.push('--instruction', instruction)

  // 用 cs kyvault run 注入 key —— 明文不落盘、不进 argv、不进 shell history
  const args = ['kyvault', 'run', '--env', `MIMO_API_KEY=${MIMO_SECRET}`, '--', 'museav', ...inner]
  await run('cs', args, { maxBuffer: 8 * 1024 * 1024 })
  if (!existsSync(out)) throw new Error(`museav speak 未产出文件: ${out}`)
  return out
}

async function speakViaVoxcraft({ text, out, voice }) {
  if (!voice) throw new Error('voxcraft 后端必须用 --voice 指定已注册音色（见 voice list）')
  // voxcraft 的 clone 命令自动生成输出路径，不接受 --out，
  // 故读它 stdout 里的路径再交由调用方搬运。
  const { stdout } = await run('voice', ['clone', voice, text], { maxBuffer: 8 * 1024 * 1024 })
  const m = stdout.match(/(\S+\.wav)/)
  if (!m) throw new Error(`没能从 voxcraft 输出里解析出 wav 路径:\n${stdout.slice(0, 400)}`)
  const produced = m[1]
  if (!existsSync(produced)) throw new Error(`voxcraft 声称产出 ${produced} 但文件不存在`)
  const { copyFileSync } = await import('node:fs')
  copyFileSync(produced, out)
  return out
}

export async function probeDuration(file) {
  const { stdout } = await run('ffprobe', [
    '-v', 'quiet', '-show_entries', 'format=duration', '-of', 'csv=p=0', file,
  ])
  const d = parseFloat(stdout.trim())
  if (!Number.isFinite(d)) throw new Error(`无法读取时长: ${file}`)
  return d
}

/**
 * 为每句文案生成配音。
 *
 * @returns {Promise<Array<{file:string, duration:number}>>} 与 captions 等长、同序
 */
export async function synthesizeCaptions({
  captions, outDir, engine = 'voxcraft', voice, design, instruction, onProgress,
}) {
  mkdirSync(outDir, { recursive: true })
  const results = []

  for (let i = 0; i < captions.length; i++) {
    const text = captions[i]
    const out = join(outDir, `voice_${String(i).padStart(4, '0')}.wav`)
    try {
      if (engine === 'voxcraft') await speakViaVoxcraft({ text, out, voice })
      else await speakViaMuseav({ text, out, voice, design, instruction })
    } catch (e) {
      throw new Error(`第 ${i + 1} 句配音失败（"${text.slice(0, 20)}"）: ${e.message}`)
    }
    const duration = await probeDuration(out)
    results.push({ file: out, duration })
    onProgress?.(i + 1, captions.length, duration)
  }
  return results
}

/**
 * 由念白时长推导镜头时长。
 *
 * 余量不是保守，是必需：做到「刚好等于念白」时，帧率舍入会让画面**略短于**音频，
 * 结果是每句最后半个字被切掉。这条是从 MoneyPrinterTurbo 的 video.py 里学来的
 * （见 docs/video-compositing-notes.md 第 4 条），那边同样加了安全余量。
 *
 * @param {number[]} voiceDurations
 * @param {number} margin  每镜额外留白秒数
 * @param {number} min     镜头下限，避免极短句一闪而过
 */
export function durationsFromVoice(voiceDurations, margin = 0.45, min = 1.2) {
  return voiceDurations.map(d => Math.max(min, d + margin))
}
