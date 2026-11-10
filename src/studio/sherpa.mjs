import { existsSync, readdirSync, readFileSync, rmSync } from 'node:fs'
import { randomUUID } from 'node:crypto'
import { dirname, join, resolve } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { command } from './process.mjs'

// 本机离线语音引擎：sherpa-onnx + 一个中文模型。
//
// 为什么要有它：
//
// 桌面版原来只有两条配音路 —— 本机 VoxFlow（要用户自己 clone 仓库、装 Python、
// 起服务）或云端 MiniMax（要账号、要花钱）。两条都不是「下载下来直接用」：
// 前者是 CLI 依赖，后者是账号依赖，Windows 上还两条都没有。
//
// 这里补第三条：引擎二进制与中文模型随应用一起打包。用户装完就能配音，
// 不装任何东西、不登录任何账号、不产生任何费用，声音也不出本机。
// macOS 与 Windows 是同一套模型、同一个调用方式。

const BIN_NAME = process.platform === 'win32' ? 'sherpa-onnx-offline-tts.exe' : 'sherpa-onnx-offline-tts'

/**
 * 找引擎装在哪。
 *
 * 打包后它随应用分发在 resources/<平台>/sherpa/；
 * 打包版的后端源码在 resources/<平台>/backend/src/studio/，
 * 往上三层就是 resources 根目录。开发时用 REEL_SHERPA_HOME 指过去。
 */
export function sherpaHome() {
  const fromEnv = (process.env.REEL_SHERPA_HOME || '').trim()
  if (fromEnv) return fromEnv
  try {
    const here = dirname(fileURLToPath(import.meta.url))
    return resolve(here, '..', '..', '..', 'sherpa')
  } catch {
    return ''
  }
}

/**
 * 定位引擎与模型。找不到就返回 null —— 调用方据此把通道标成「未就绪」，
 * 而不是让用户在合成时撞上一个「文件不存在」。
 */
export function sherpaEngine() {
  const home = sherpaHome()
  if (!home) return null
  const bin = join(home, 'bin', BIN_NAME)
  if (!existsSync(bin)) return null
  const modelDir = join(home, 'model')
  const tokens = join(modelDir, 'tokens.txt')
  const lexicon = join(modelDir, 'lexicon.txt')
  if (!existsSync(tokens) || !existsSync(lexicon)) return null
  let model = ''
  try {
    const onnx = readdirSync(modelDir).filter(name => name.endsWith('.onnx')).sort()
    if (!onnx.length) return null
    model = join(modelDir, onnx[0])
  } catch {
    return null
  }
  return { home, bin, model, tokens, lexicon, libDir: join(home, 'lib') }
}

/**
 * 合成一段中文语音，返回 wav 字节。
 *
 * 引擎自己会按句切分（--tts-max-num-sentences），所以整段文字一次传进去就行。
 */
export async function sherpaSynthesize({ text, engine, onProgress }) {
  if (!engine) throw new Error('本机语音引擎没找到，请重新安装应用')
  const content = String(text || '').trim()
  if (!content) throw new Error('没有要合成的文字')
  const out = join(tmpdir(), `reel-sherpa-${randomUUID()}.wav`)
  try {
    onProgress?.('本机离线语音正在合成…')
    await command(engine.bin, [
      `--vits-model=${engine.model}`,
      `--vits-tokens=${engine.tokens}`,
      `--vits-lexicon=${engine.lexicon}`,
      `--output-filename=${out}`,
      // 位置参数以 - 开头会被当成命令选项，正文前面补一个空格挡一下。
      content.startsWith('-') ? ` ${content}` : content,
    ], {
      timeout: 600000,
      env: sherpaEnv(engine),
    })
    if (!existsSync(out)) throw new Error('本机语音引擎没有产出音频')
    return readFileSync(out)
  } finally {
    rmSync(out, { force: true })
  }
}

// macOS 的动态库不在引擎旁边，靠这个环境变量找得到；Windows 靠 exe 同目录。
function sherpaEnv(engine) {
  const env = { ...process.env }
  if (process.platform === 'darwin') {
    const current = env.DYLD_LIBRARY_PATH ? `${env.DYLD_LIBRARY_PATH}:${engine.libDir}` : engine.libDir
    env.DYLD_LIBRARY_PATH = current
  }
  return env
}
