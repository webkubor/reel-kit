import { spawn } from 'node:child_process'
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import http from 'node:http'
import https from 'node:https'

export function redact(text) {
  return String(text)
    .replace(/Bearer\s+\S+/gi, 'Bearer [redacted]')
    .replace(/\b(?:sk-|eyJ)[\w.-]{15,}/g, '[redacted]')
    .replace(/https?:\/\/\S+/g, '[服务地址]')
    // 本地绝对路径贴到界面上既没用也泄露目录结构。
    // 必须以 / 开头才算路径，否则「480p/720p/1080p」这种分辨率会被误当成路径吃掉。
    .replace(/(?:^|(?<=\s))\/(?:[\w.@+-]+\/)+[\w.@+-]*/g, '[本地路径]')
}
// 取消信号：被取消的子进程必须真的停掉，而不是等它跑完再把结果扔掉。
// 上游任务可能已经计费 —— 让它跑完再丢弃等于白花钱，所以 abort 要一路传到进程。
export function cancelledError() {
  return Object.assign(new Error('已取消'), { cancelled: true })
}
export function throwIfCancelled(signal) {
  if (signal?.aborted) throw cancelledError()
}
// No shell, bounded output and lifetime. Never include argv (prompts/keys) in errors.
export function command(bin, args, { timeout = 600000, onProgress, env = process.env, cwd, signal } = {}) {
  return commandFull(bin, args, { timeout, onProgress, env, cwd, signal }).then(result => result.stdout)
}
/**
 * 同 command，但把 stderr 原样带回来。
 *
 * 有些工具把「有用的东西」写在 stderr（CLI 的清单表、任务 ID 都在那儿），
 * 而 onProgress 传出去的是截断过的片段 —— 拿它拼不出完整内容，就不能拿它当依据。
 */
export function commandFull(bin, args, { timeout = 600000, onProgress, onStderr, env = process.env, cwd, signal } = {}) {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(cancelledError())
    const child = spawn(bin, args, { env, cwd, stdio: ['ignore', 'pipe', 'pipe'] })
    let stdout = '', stderr = '', timedOut = false, aborted = false
    const timer = setTimeout(() => { timedOut = true; child.kill('SIGKILL') }, timeout)
    // 取消：先礼后兵。SIGTERM 给 ffmpeg 收尾的机会（写完 moov），
    // 两秒还不走就 SIGKILL —— 留着半截 mp4 反而会让后面的合成拿到坏文件。
    const onAbort = () => {
      aborted = true
      child.kill('SIGTERM')
      const hard = setTimeout(() => { try { child.kill('SIGKILL') } catch {} }, 2000)
      if (typeof hard.unref === 'function') hard.unref()
    }
    signal?.addEventListener('abort', onAbort, { once: true })
    child.stdout.on('data', chunk => { stdout = (stdout + chunk).slice(-1024 * 1024) })
    child.stderr.on('data', chunk => {
      const text = String(chunk)
      stderr = (stderr + text).slice(-12000)
      // onStderr 给原始增量：上游的任务 ID 就写在这里，必须在命令结束前就拿到，
      // 否则一旦进程被超时杀掉，已经提交并计费的任务 ID 就跟着丢了 ——
      // 那样用户只能重新提交一遍，等于重复计费。
      onStderr?.(text)
      onProgress?.(redact(text).trim().slice(-240))
    })
    child.on('error', error => { clearTimeout(timer); signal?.removeEventListener('abort', onAbort); reject(new Error(error.code === 'ENOENT' ? `${bin} 未安装或不在 PATH` : `${bin} 启动失败`)) })
    child.on('close', code => {
      clearTimeout(timer)
      signal?.removeEventListener('abort', onAbort)
      if (aborted) reject(cancelledError())
      else if (timedOut) reject(new Error('执行超时。请核对上游任务后再重试，避免重复计费。'))
      else if (code !== 0) reject(new Error(redact(stderr).slice(-600) || `${bin} 退出码 ${code}`))
      else resolve({ stdout: stdout.trim(), stderr })
    })
  })
}
export async function available(bin) {
  try { await command(bin, [bin === 'ffmpeg' ? '-version' : '--version'], { timeout: 4000 }); return true } catch { return false }
}
export async function jsonFetch(url, options = {}) {
  const { signal, ...rest } = options
  const timeout = AbortSignal.timeout(20000)
  const combined = signal ? AbortSignal.any([timeout, signal]) : timeout
  const response = await fetch(url, { ...rest, signal: combined })
  if (!response.ok) throw new Error(`服务返回 HTTP ${response.status}`)
  return response.json()
}
/**
 * 下载素材到内存。
 *
 * 用 http/https 而不是 fetch：fetch 的连接超时固定 10 秒，
 * 上游对象存储（OSS/COS 那种）握手慢一点就直接失败 —— 而素材已经在上游生成并计费了，
 * 因为下载超时就丢掉，是最亏的一种失败。这里给 60 秒连接窗口，并允许重定向。
 *
 * 对象存储的握手本来就会抽风，所以重试几次；重试的是下载，不是重新生成，
 * 不会重复计费。
 */
export async function download(url, max = 200 * 1024 * 1024, redirects = 0, signal = undefined) {
  const parsed = new URL(url)
  if (!['https:', 'http:'].includes(parsed.protocol)) throw new Error('生成服务返回了无效素材地址')
  let lastError
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      return await downloadOnce(parsed, max, redirects, signal)
    } catch (error) {
      if (error?.cancelled) throw error
      lastError = error
      // 素材超过上限、地址非法这类错误重试也没用
      if (/超过|无效素材地址/.test(error.message)) break
      if (attempt < 2) await new Promise(resolve => setTimeout(resolve, 1500 * (attempt + 1)))
    }
  }
  throw lastError
}

function downloadOnce(parsed, max, redirects, signal) {
  return new Promise((resolve, reject) => {
    const lib = parsed.protocol === 'https:' ? https : http
    const request = lib.get(parsed, { timeout: 60000 }, response => {
      const status = response.statusCode || 0
      if (status >= 300 && status < 400 && response.headers.location) {
        response.resume()
        if (redirects >= 5) return reject(new Error('素材地址重定向次数过多'))
        return resolve(download(new URL(response.headers.location, parsed), max, redirects + 1, signal))
      }
      if (status !== 200) { response.resume(); return reject(new Error(`下载素材失败 HTTP ${status}`)) }
      const chunks = []
      let size = 0
      response.on('data', chunk => {
        size += chunk.length
        if (size > max) { request.destroy(); return reject(new Error('素材超过 200MB，请在上游下载')) }
        chunks.push(chunk)
      })
      response.on('end', () => resolve({ bytes: Buffer.concat(chunks), mime: response.headers['content-type']?.split(';')[0] }))
      response.on('error', reject)
    })
    request.on('timeout', () => request.destroy(new Error('连接 60 秒无响应')))
    request.on('error', reject)
    // 取消时立刻断开下载，别让一个已经没人要的素材继续占用带宽。
    const onAbort = () => { request.destroy(cancelledError()) }
    if (signal?.aborted) return reject(cancelledError())
    signal?.addEventListener('abort', onAbort, { once: true })
    request.on('close', () => signal?.removeEventListener('abort', onAbort))
  })
}
// ffprobe 探测素材真实尺寸/时长；失败不阻断入库（尺寸未知只是不显示）。
export async function probeBytes(name, bytes) {
  const dir = mkdtempSync(join(tmpdir(), 'reel-probe-'))
  try {
    writeFileSync(join(dir, name), bytes, { mode: 0o600 })
    const out = await command('ffprobe', ['-v', 'error', '-print_format', 'json', '-show_streams', '-show_format', join(dir, name)], { timeout: 20000 })
    const data = JSON.parse(out)
    const video = (data.streams || []).find(s => s.codec_type === 'video')
    const audio = (data.streams || []).find(s => s.codec_type === 'audio')
    return {
      width: video?.width, height: video?.height,
      duration: Number(data.format?.duration) || undefined,
      hasAudio: Boolean(audio),
    }
  } catch { return {} } finally { rmSync(dir, { recursive: true, force: true }) }
}
