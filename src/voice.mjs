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
 *   minimax         —— MiniMax 云端 TTS，无需本地模型、秒级出声，按量计费。
 *                      只做文字配音（音色用 voice_id），不支持克隆与音色设计。
 *
 * 默认选 voxcraft 是**成本决定的**：批量流水线一支片子几十句，
 * API 调用会持续累积，而本地推理是一次性投入。
 */

import { execFile } from 'node:child_process'
import { copyFileSync, existsSync, mkdirSync, writeFileSync, rmSync, readFileSync } from 'node:fs'
import { isAbsolute, join } from 'node:path'
import { promisify } from 'node:util'
import { homedir } from 'node:os'
import { locateVoxcraft, voxcraftHint } from './voxcraft-locate.mjs'
import { ensureServer, stopServer, synthesizeViaServer } from './voxcraft-server.mjs'

const run = promisify(execFile)

/**
 * 读 ~/.reel-kit/config.json。
 *
 * 桌面版从 Finder 启动时没有终端环境变量，用户没法自己 export MINIMAX_API_KEY，
 * 所以密钥要能落在配置文件里（该文件权限 0600）。
 */
function readReelConfig() {
  try {
    return JSON.parse(readFileSync(join(homedir(), '.reel-kit', 'config.json'), 'utf8'))
  } catch {
    return {}
  }
}

/**
 * 云端配音：MiniMax TTS。
 *
 * 原来这里挂的是 `museav speak` —— 中台**没有这个命令**（3.16.0 的 CLI 里不存在），
 * 调下去必然 unknown command。所谓「MiMo API 兜底」从来没能跑通。
 * 现在换成实测能出声的 MiniMax TTS，并把「不支持的能力」明说。
 */
async function speakViaMinimax({ text, out, voice }) {
  const key = process.env.MINIMAX_API_KEY || readReelConfig().minimaxApiKey || ''
  if (!key) throw new Error('未配置 MINIMAX_API_KEY（用密钥工具注入，或写进 ~/.reel-kit/config.json 的 minimaxApiKey）')
  if (!voice) throw new Error('MiniMax TTS 需要 --voice <音色 ID>，例如 male-qn-qingse')
  const response = await fetch('https://api.minimaxi.com/v1/t2a_v2', {
    method: 'POST',
    headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model: process.env.REEL_MINIMAX_TTS_MODEL || 'speech-02-hd',
      text,
      stream: false,
      voice_setting: { voice_id: voice, speed: 1, vol: 1, pitch: 0 },
      audio_setting: { sample_rate: 32000, bitrate: 128000, format: 'mp3', channel: 1 },
    }),
    signal: AbortSignal.timeout(120000),
  })
  const result = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(`MiniMax HTTP ${response.status}${result.base_resp?.status_msg ? `：${result.base_resp.status_msg}` : ''}`)
  // 成功回执里也带 base_resp，status_code 为 0 才算成功 —— 有 base_resp 不等于失败
  if (result.base_resp?.status_code) throw new Error(`MiniMax 拒绝请求（${result.base_resp.status_code}）：${result.base_resp.status_msg || '请核对权限或额度'}`)
  const hex = result.data?.audio
  if (!hex) throw new Error('MiniMax 未返回音频数据')
  const tmp = out.replace(/\.wav$/, '.mp3')
  writeFileSync(tmp, Buffer.from(hex, 'hex'))
  // 流水线后面统一按 wav 处理（拼接、混音），所以这里转一次
  try {
    await run('ffmpeg', ['-y', '-loglevel', 'error', '-i', tmp, '-ar', '24000', '-ac', '1', out])
  } finally { rmSync(tmp, { force: true }) }
  if (!existsSync(out)) throw new Error(`MiniMax 音频转换失败: ${out}`)
  return out
}

/**
 * 取小米 MiMo 密钥。
 *
 * 三档来源，按优先级：
 *   1. 环境变量 MIMO_API_KEY —— CI / 手动 export 走这条。
 *   2. ~/.reel-kit/config.json 的 mimoApiKey —— 桌面版从 Finder 启动时没有终端环境变量，
 *      用户没法自己 export，所以密钥要能落在配置文件里（该文件权限 0600）。
 *   3. kyvault 的 secret://mimo/api-key —— 本机凭据真源，最后一档兜底。
 *
 * 为什么第 3 档要用子进程而不是 SDK：reel-kit 是零依赖工具（package.json 只依赖
 * playwright-core），引 kyvault SDK 等于给发布包加一条原生依赖链。kyvault 的 CLI
 * 契约是「stdout 是明文，stderr 给人看」，子进程读 stdout 就够。
 *
 * 注意桌面版从 Finder 启动时 PATH 很干净（没有 ~/.local/bin），所以 kyvault 要用
 * 绝对路径兜底，不能只写命令名。
 *
 * 来源顺序（先到先得）：
 *   1. 环境变量 MIMO_API_KEY
 *   2. ~/.reel-kit/config.json 的 mimoApiKey
 *   3. kyvault 的 secret://mimo/api-key
 *
 * 为什么要有第 3 档：桌面版从 Finder 启动时没有终端环境变量，用户没法
 * 自己 export；kyvault 是本机的凭据真源，让「装了就能出声」成立。
 */
async function resolveMimoKey() {
  const fromEnv = process.env.MIMO_API_KEY
  if (fromEnv) return fromEnv.trim()
  const fromConfig = readReelConfig().mimoApiKey
  if (fromConfig) return String(fromConfig).trim()
  try {
    const { stdout } = await run('ky', ['get', 'secret://mimo/api-key'])
    const v = String(stdout || '').trim()
    if (v && !v.includes('secret://')) return v
  } catch { /* 没装 ky 或没有这条凭据 —— 下面统一按「没配」处理 */ }
  return ''
}

/**
 * 云端配音：小米 MiMo TTS（Token Plan 限时免费）。
 *
 * 为什么单独接一个免费后端：批量流水线一条片子几十句，按量计费的 API 会持续累积；
 * MiMo 的 TTS 在 Token Plan 里是免费的，音色质量够用。
 *
 * ## 三个必须记住的点（都踩过）
 *
 * 1. **域名要用 Token Plan 专用的** `token-plan-cn.xiaomimimo.com`。
 *    通用域名 `api.xiaomimimo.com` 对 Token Plan 的 key 一律 401。
 *
 * 2. **端点是 `/v1/chat/completions` 而不是 `/v1/audio/speech`** —— MiMo 的音频要走
 *    对话接口：在 messages 里放一条 role=assistant 的消息当待合成文本，
 *    再用 audio 字段指定输出格式。打 `/v1/audio/speech` 得到 404。
 *
 * 3. **认证用 `api-key` 请求头**。只带 `Authorization: Bearer` 的话，
 *    对话模型能通、TTS 模型会 401 —— 两条路走的是不同的鉴权分支。
 */
async function speakViaMimo({ text, out, voice }) {
  const key = await resolveMimoKey()
  if (!key) throw new Error('未配置小米 MiMo 密钥（MIMO_API_KEY / ~/.reel-kit/config.json 的 mimoApiKey / kyvault 的 secret://mimo/api-key 三处都没有）')
  const base = process.env.MIMO_BASE_URL || 'https://token-plan-cn.xiaomimimo.com/v1'
  const response = await fetch(`${base}/chat/completions`, {
    method: 'POST',
    headers: {
      'api-key': key,
      Authorization: `Bearer ${key}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      model: process.env.REEL_MIMO_TTS_MODEL || 'mimo-v2.5-tts',
      messages: [
        { role: 'assistant', content: text },
      ],
      audio: { format: 'mp3', voice: voice || process.env.REEL_MIMO_TTS_VOICE || 'mimo_default' },
    }),
    signal: AbortSignal.timeout(120000),
  })
  const result = await response.json().catch(() => ({}))
  if (!response.ok) {
    const msg = result?.error?.message || result?.message || ''
    throw new Error(`MiMo HTTP ${response.status}${msg ? `：${msg}` : ''}`)
  }
  const audio = result?.choices?.[0]?.message?.audio
  const b64 = audio?.data
  if (!b64) throw new Error('MiMo 未返回音频数据')
  const tmp = out.replace(/\.wav$/, '.mp3')
  writeFileSync(tmp, Buffer.from(b64, 'base64'))
  // 流水线后面统一按 wav 处理（拼接、混音），所以这里转一次
  try {
    await run('ffmpeg', ['-y', '-loglevel', 'error', '-i', tmp, '-ar', '24000', '-ac', '1', out])
  } finally { rmSync(tmp, { force: true }) }
  if (!existsSync(out)) throw new Error(`MiMo 音频转换失败: ${out}`)
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

  if (engine === 'museav') {
    throw new Error(
      'museav 配音已下线：中台的 CLI 里没有 speak 命令，这条路径从来没能跑通。\n' +
      '  改用 --voice-engine edge（微软 Edge 在线免费高保真）、--voice-engine mimo（小米 MiMo，限时免费）、--voice-engine minimax（云端 TTS），或在 Reel Studio 里选相应通道。',
    )
  }
  if (!['voxcraft', 'minimax', 'edge', 'mimo'].includes(engine)) {
    throw new Error(`未知的配音后端 "${engine}"；可用：edge（微软在线高保真·免费）、mimo（小米 MiMo·限时免费）、voxcraft（本地）、minimax（云端 TTS·按量计费）`)
  }
  if (engine === 'edge') {
    const { edgeSynthesize } = await import('./edge-tts.mjs')
    const selectedVoice = voice || 'zh-CN-XiaoxiaoNeural'
    for (let i = 0; i < captions.length; i++) {
      onProgress?.(i + 1, captions.length, 0, { pending: true })
      const out = join(outDir, `caption_${String(i).padStart(4, '0')}.mp3`)
      const { bytes } = await edgeSynthesize({
        text: captions[i],
        voice: selectedVoice,
      })
      writeFileSync(out, bytes)
      const duration = await probeDuration(out)
      results.push({ file: out, duration })
      onProgress?.(i + 1, captions.length, duration)
    }
    return results
  }
  if (engine === 'minimax' && (design || instruction)) {
    throw new Error('MiniMax TTS 只做文字配音，不支持音色设计（--design）与指令（--instruction）；这两项请用本地 voxcraft')
  }
  if (engine === 'mimo' && (design || instruction)) {
    throw new Error('MiMo TTS 只做文字配音，不支持音色设计（--design）与指令（--instruction）；这两项请用本地 voxcraft')
  }
  // MiMo 走独立分支：它的响应结构（audio.data base64）与 MiniMax 的 hex 不同，
  // 而且要逐句合成才能把出错位置报到句号。
  if (engine === 'mimo') {
    for (let i = 0; i < captions.length; i++) {
      const text = captions[i]
      const out = join(outDir, `voice_${String(i).padStart(4, '0')}.wav`)
      try {
        await speakViaMimo({ text, out, voice })
      } catch (e) {
        throw new Error(`第 ${i + 1} 句配音失败（"${text.slice(0, 20)}"）: ${e.message}`)
      }
      const duration = await probeDuration(out)
      results.push({ file: out, duration })
      onProgress?.(i + 1, captions.length, duration)
    }
    return results
  }

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
      await speakViaMinimax({ text, out, voice })
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
