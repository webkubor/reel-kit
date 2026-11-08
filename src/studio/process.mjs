import { spawn } from 'node:child_process'

export function redact(text) {
  return String(text).replace(/Bearer\s+\S+/gi, 'Bearer [redacted]').replace(/\b(?:sk-|eyJ)[\w.-]{15,}/g, '[redacted]').replace(/https?:\/\/\S+/g, '[服务地址]')
}
// No shell, bounded output and lifetime. Never include argv (prompts/keys) in errors.
export function command(bin, args, { timeout = 600000, onProgress, env = process.env, cwd } = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(bin, args, { env, cwd, stdio: ['ignore', 'pipe', 'pipe'] })
    let stdout = '', stderr = '', timedOut = false
    const timer = setTimeout(() => { timedOut = true; child.kill('SIGKILL') }, timeout)
    child.stdout.on('data', chunk => { stdout = (stdout + chunk).slice(-1024 * 1024) })
    child.stderr.on('data', chunk => {
      stderr = (stderr + chunk).slice(-12000)
      onProgress?.(redact(String(chunk)).trim().slice(-240))
    })
    child.on('error', error => { clearTimeout(timer); reject(new Error(error.code === 'ENOENT' ? `${bin} 未安装或不在 PATH` : `${bin} 启动失败`)) })
    child.on('close', code => {
      clearTimeout(timer)
      if (timedOut) reject(new Error('执行超时。请核对上游任务后再重试，避免重复计费。'))
      else if (code !== 0) reject(new Error(redact(stderr).slice(-600) || `${bin} 退出码 ${code}`))
      else resolve(stdout.trim())
    })
  })
}
export async function available(bin) {
  try { await command(bin, [bin === 'ffmpeg' ? '-version' : '--version'], { timeout: 4000 }); return true } catch { return false }
}
export async function jsonFetch(url, options = {}) {
  const response = await fetch(url, { signal: AbortSignal.timeout(20000), ...options })
  if (!response.ok) throw new Error(`服务返回 HTTP ${response.status}`)
  return response.json()
}
export async function download(url, max = 200 * 1024 * 1024) {
  const parsed = new URL(url)
  if (!['https:', 'http:'].includes(parsed.protocol)) throw new Error('生成服务返回了无效素材地址')
  const response = await fetch(parsed, { signal: AbortSignal.timeout(120000) })
  if (!response.ok || !response.body) throw new Error(`下载素材失败 HTTP ${response.status}`)
  const chunks = []; let size = 0
  for await (const chunk of response.body) {
    size += chunk.length
    if (size > max) throw new Error('素材超过 200MB，请在上游下载')
    chunks.push(chunk)
  }
  return { bytes: Buffer.concat(chunks), mime: response.headers.get('content-type')?.split(';')[0] }
}
