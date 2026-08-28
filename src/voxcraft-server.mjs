/**
 * voxcraft 的 web 服务模式 —— 起一次服务，合成多句。
 *
 * 为什么必须这样：voxcraft 的 CLI 每次调用都要重新加载 4.2GB 模型。
 * 逐句起进程做一支 8 句的片子，就是加载 8 次模型 —— 慢，而且不稳：
 * 实测第 5 句崩在 `libc++abi: recursive_mutex lock failed`。
 *
 * 服务模式一次加载、多次调用，既快又稳。代价是要管服务生命周期
 * （起、等就绪、用完关），以及它的任务是异步的（提交拿 task_id，再轮询）。
 *
 * API 契约（读 voxcraft 的 web/app.py 得来，不是猜的）：
 *   GET  /api/status                → { base_model: bool, ... }
 *   POST /api/clone {persona,text}  → { task_id, status:"queued" }
 *   GET  /api/tasks                 → { tasks:[{id,status,progress,stage,result,error}] }
 *   GET  /api/audio/<filename>      → wav 二进制
 *   任务 status: queued | running | done | error | cancelled
 *   成功时 result = { ok, filename, url, persona, text }
 */

import { spawn } from 'node:child_process'
import { writeFileSync } from 'node:fs'
import { join } from 'node:path'

const DEFAULT_PORT = 8866

const sleep = ms => new Promise(r => setTimeout(r, ms))

/** 服务是否已经在跑 —— 别人开着就复用，不重复起 */
async function probe(base) {
  try {
    const r = await fetch(`${base}/api/status`, { signal: AbortSignal.timeout(2500) })
    if (!r.ok) return null
    return await r.json()
  } catch { return null }
}

/**
 * 确保 voxcraft web 服务可用。
 * @returns {Promise<{base:string, proc:import('node:child_process').ChildProcess|null, reused:boolean}>}
 */
export async function ensureServer({ home, bin, port = DEFAULT_PORT, timeoutMs = 180000, onLog }) {
  const base = `http://127.0.0.1:${port}`

  const existing = await probe(base)
  if (existing) {
    onLog?.(`复用已在跑的 voxcraft 服务（:${port}）`)
    return { base, proc: null, reused: true }
  }

  onLog?.(`启动 voxcraft 服务（:${port}，首次要加载 4.2GB 模型）…`)
  // detached:false —— 父进程退出时一并收走，避免留下孤儿服务占着模型内存
  const proc = spawn(bin, ['web', '--port', String(port)], {
    cwd: home,
    stdio: ['ignore', 'pipe', 'pipe'],
    detached: false,
  })
  let stderr = ''
  proc.stderr?.on('data', d => { stderr += d; if (stderr.length > 20000) stderr = stderr.slice(-8000) })
  proc.stdout?.on('data', () => {})

  let exited = false
  proc.on('exit', code => { exited = true; if (code) stderr += `\n[进程退出码 ${code}]` })

  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (exited) throw new Error(`voxcraft 服务启动即退出:\n${stderr.slice(-600)}`)
    const st = await probe(base)
    if (st) {
      if (st.base_model === false) {
        try { proc.kill('SIGTERM') } catch {}
        throw new Error('voxcraft 服务起来了，但 Base 模型未就绪（/api/status 报 base_model=false）')
      }
      onLog?.('服务就绪')
      return { base, proc, reused: false }
    }
    await sleep(1200)
  }
  try { proc.kill('SIGTERM') } catch {}
  throw new Error(`voxcraft 服务 ${Math.round(timeoutMs / 1000)}s 内未就绪:\n${stderr.slice(-600)}`)
}

export function stopServer(handle) {
  if (!handle?.proc || handle.reused) return   // 复用别人的就不要关
  try { handle.proc.kill('SIGTERM') } catch {}
}

/** 提交一句并等它出结果 */
async function cloneOne({ base, persona, text, pollMs = 900, timeoutMs = 180000 }) {
  const r = await fetch(`${base}/api/clone`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ persona, text }),
    signal: AbortSignal.timeout(20000),
  })
  if (!r.ok) throw new Error(`提交失败 HTTP ${r.status}: ${(await r.text()).slice(0, 200)}`)
  const { task_id } = await r.json()
  if (!task_id) throw new Error('服务未返回 task_id')

  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    await sleep(pollMs)
    const tr = await fetch(`${base}/api/tasks`, { signal: AbortSignal.timeout(15000) })
    if (!tr.ok) continue
    const { tasks } = await tr.json()
    const t = (tasks || []).find(x => x.id === task_id)
    if (!t) continue
    if (t.status === 'done') {
      if (!t.result?.filename) throw new Error('任务完成但 result 里没有 filename')
      return t.result.filename
    }
    if (t.status === 'error') throw new Error(t.error || '任务失败且未给出原因')
    if (t.status === 'cancelled') throw new Error('任务被取消')
  }
  throw new Error(`任务 ${task_id} 超时未完成`)
}

/** 把服务生成的音频下载到本地路径 */
async function download({ base, filename, out }) {
  const r = await fetch(`${base}/api/audio/${encodeURIComponent(filename)}`, {
    signal: AbortSignal.timeout(30000),
  })
  if (!r.ok) throw new Error(`下载 ${filename} 失败 HTTP ${r.status}`)
  writeFileSync(out, Buffer.from(await r.arrayBuffer()))
  return out
}

/**
 * 用服务模式批量合成。
 * @returns {Promise<string[]>} 与 texts 等长、同序的本地 wav 路径
 */
export async function synthesizeViaServer({ base, persona, texts, outDir, onProgress }) {
  const files = []
  for (let i = 0; i < texts.length; i++) {
    const filename = await cloneOne({ base, persona, text: texts[i] })
    const out = join(outDir, `voice_${String(i).padStart(4, '0')}.wav`)
    await download({ base, filename, out })
    files.push(out)
    onProgress?.(i + 1, texts.length)
  }
  return files
}
