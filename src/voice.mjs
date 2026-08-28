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
import { copyFileSync, existsSync, mkdirSync } from 'node:fs'
import { isAbsolute, join } from 'node:path'
import { promisify } from 'node:util'
import { locateVoxcraft, voxcraftHint } from './voxcraft-locate.mjs'

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

async function speakViaVoxcraft({ text, out, voice, bin, home }) {
  if (!voice) throw new Error('voxcraft 后端必须用 --voice 指定已注册音色（voice voice list 可查）')
  /*
   * voxcraft 的 clone 命令自己决定输出路径、不接受 --out，所以要从 stdout 里捞。
   * 两个坑：
   *   ① 它打印的是**相对路径**（out/[克隆]xxx.wav），相对的是它自己的项目根，
   *      而调用方的 cwd 通常不在那儿 —— 必须用 home 拼成绝对路径。
   *   ② 文件名含中文和方括号，正则不能用 \S+ 贪到行尾的其它内容。
   * 顺带把 cwd 设成 home，让它的相对路径逻辑在自己的地盘上成立。
   */
  const { stdout } = await run(bin || 'voice', ['clone', voice, text], {
    maxBuffer: 8 * 1024 * 1024,
    cwd: home || undefined,
  })
  const m = stdout.match(/(\S*\.wav)/)
  if (!m) throw new Error(`没能从 voxcraft 输出里解析出 wav 路径:\n${stdout.slice(0, 400)}`)
  const produced = isAbsolute(m[1]) ? m[1] : join(home || '.', m[1])
  if (!existsSync(produced)) {
    throw new Error(`voxcraft 声称产出 ${m[1]}，解析为 ${produced}，但文件不存在`)
  }
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

  /*
   * voxcraft 的 `voice` 命令在它自己的 venv 里，默认不在 PATH。
   * 只判断 `which voice` 会误判成「没装」，进而重复装一遍 —— 模型 4.2GB，
   * 重复装是实打实的浪费（实测本机就有两份 clone，各自下过一份模型）。
   * 所以先定位再决定，缺什么明确报出来，不自动装那种体量的东西。
   */
  let bin = null
  let home = null
  if (engine === 'voxcraft') {
    const info = locateVoxcraft()
    if (!info.ok) {
      throw new Error(
        `voxcraft 不可用：${info.reason}\n\n${voxcraftHint(info)}`,
      )
    }
    bin = info.bin
    home = info.home
    if (voice && info.personas.length && !info.personas.includes(voice)) {
      throw new Error(
        `音色 "${voice}" 未注册。${info.home} 里已有：${info.personas.join(', ') || '（空）'}\n` +
        `  注册：cd ${info.home} && .venv/bin/voice voice add ${voice} <参考音频>`,
      )
    }
    onProgress?.(0, captions.length, 0, { home: info.home, personas: info.personas })
  }

  for (let i = 0; i < captions.length; i++) {
    const text = captions[i]
    const out = join(outDir, `voice_${String(i).padStart(4, '0')}.wav`)
    try {
      if (engine === 'voxcraft') await speakViaVoxcraft({ text, out, voice, bin, home })
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
