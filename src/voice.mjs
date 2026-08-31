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
import { ensureServer, stopServer, synthesizeViaServer } from './voxcraft-server.mjs'

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

export async function probeDuration(file) {
  const { stdout } = await run('ffprobe', [
    '-v', 'quiet', '-show_entries', 'format=duration', '-of', 'csv=p=0', file,
  ])
  const d = parseFloat(stdout.trim())
  if (!Number.isFinite(d)) throw new Error(`无法读取时长: ${file}`)
  return d
}

/**
 * 把一句配音按字幕段字数切分成 N 段 wav。
 * 用于 caption-progressive 模式:长句不重读,但每段字幕对应 atrim 切出的
 * 配音片段,听感对齐、不"断音"(每段是原 wav 的连续片段,不是重新合成)。
 *
 * 切分时间按字数比例估算,切完再 probe 实际时长。ffmpeg `-c copy` 切 wav
 * 是按 sample 对齐,精度 < 50ms,对短视频可忽略。
 *
 * @param {string}   voiceFile     原始 wav
 * @param {number[]} segmentChars  每段的字符数(用 [...str].length 算中文友好)
 * @param {string}   outDir        输出目录
 * @returns {Promise<Array<{file:string, duration:number}>>}
 */
export async function splitVoiceByChars(voiceFile, segmentChars, outDir) {
  mkdirSync(outDir, { recursive: true })
  if (!segmentChars.length) return []
  if (segmentChars.length === 1) {
    // 单段:直接复用,不必切
    return [{ file: voiceFile, duration: await probeDuration(voiceFile) }]
  }
  const total = await probeDuration(voiceFile)
  const totalChars = segmentChars.reduce((a, b) => a + b, 0) || 1
  // 累积边界(前 i 段的累计时间)
  const boundaries = [0]
  let acc = 0
  for (let i = 0; i < segmentChars.length - 1; i++) {
    acc += segmentChars[i]
    boundaries.push((acc / totalChars) * total)
  }
  boundaries.push(total)
  const results = []
  for (let i = 0; i < segmentChars.length; i++) {
    const start = boundaries[i]
    const end = boundaries[i + 1]
    const out = join(outDir, `voice_seg_${String(i).padStart(4, '0')}.wav`)
    // -ss 在 input 前是粗略(input demuxer),这里用 input 后的精确切法不可行
    //(因为要保持 -c copy)。但 wav 容器简单,sample 对齐误差 < 50ms,够用。
    await run('ffmpeg', ['-y', '-i', voiceFile, '-ss', start.toFixed(3), '-to', end.toFixed(3), '-c', 'copy', out],
      { maxBuffer: 8 * 1024 * 1024 })
    if (!existsSync(out)) throw new Error(`配音切分失败:段 ${i} 未产出文件`)
    results.push({ file: out, duration: await probeDuration(out) })
  }
  return results
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
  if (engine === 'voxcraft') {
    const info = locateVoxcraft()
    if (!info.ok) {
      throw new Error(`voxcraft 不可用：${info.reason}\n\n${voxcraftHint(info)}`)
    }
    if (voice && info.personas.length && !info.personas.includes(voice)) {
      throw new Error(
        `音色 "${voice}" 未注册。${info.home} 里已有：${info.personas.join(', ') || '（空）'}\n` +
        `  注册：cd ${info.home} && .venv/bin/voice voice add ${voice} <参考音频>`,
      )
    }
    onProgress?.(0, captions.length, 0, { home: info.home, personas: info.personas })

    /*
     * 走 web 服务模式，不逐句起进程。
     *
     * voxcraft 的 CLI 每次调用都重新加载 4.2GB 模型 —— 8 句就是加载 8 次，
     * 慢且不稳（实测第 5 句崩在 libc++ 的 recursive_mutex）。
     * 服务模式一次加载多次调用，代价是要管生命周期，所以用 try/finally 兜住：
     * 无论成功失败都关掉自己起的服务，不留孤儿进程占着模型内存。
     */
    let handle = null
    try {
      handle = await ensureServer({
        home: info.home, bin: info.bin,
        onLog: msg => onProgress?.(0, captions.length, 0, { log: msg }),
      })
      const files = await synthesizeViaServer({
        base: handle.base, persona: voice, texts: captions, outDir,
        onProgress: (i, total) => onProgress?.(i, total, 0, { pending: true }),
      })
      for (const f of files) results.push({ file: f, duration: await probeDuration(f) })
      // 时长要等全部下载完再统一读，避免把网络等待混进单句进度里
      results.forEach((r, i) => onProgress?.(i + 1, captions.length, r.duration))
      return results
    } finally {
      stopServer(handle)
    }
  }

  for (let i = 0; i < captions.length; i++) {
    const text = captions[i]
    const out = join(outDir, `voice_${String(i).padStart(4, '0')}.wav`)
    try {
      await speakViaMuseav({ text, out, voice, design, instruction })
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
