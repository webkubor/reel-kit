/**
 * Magpie 网关通道。
 *
 * 只用 Magpie 公开的 CLI（magpie tool / magpie tools / magpie settings），
 * 不碰它内部的网关端口、也不读它的凭据文件。
 *
 * 两条硬规矩：
 *
 * 1) 发现能力必须是只读的。
 *    图片模型清单只能靠「未知模型会被拒绝并回出完整清单」这条文档保证拿到；
 *    没有这条保证就不发探测请求 —— 万一模型被接受，就是一次真生成、真计费。
 *    视频没有这条保证，所以视频的可用性只从文档与设置里读，绝不发视频生成请求。
 *
 * 2) 提交即计费。
 *    上游一旦受理就要把任务 ID（或保留一小时的结果地址）先落到任务上，
 *    再去下载素材。下载失败重试的是「取回」，不是「再生成一次」。
 */
import { spawn } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { mkdtempSync, readdirSync, readFileSync, rmSync, statSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { redact } from './process.mjs'

/**
 * 跑一次 magpie CLI。
 *
 * 与 process.mjs 的 commandFull 不同：这里出错时也要留住 stdout。
 * Magpie 拒绝一个模型时，完整的模型清单就在 stdout 的 JSON 里 ——
 * 命令行工具把「有用的东西」写在 stdout、把「出错了」写在 stderr，
 * 失败就丢掉 stdout 等于把唯一有用的信息扔了。
 */
export function magpieCommand(args, { timeout = 60000, maxBytes = 32 * 1024 * 1024, env = process.env, cwd } = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn('magpie', args, { env, cwd, stdio: ['ignore', 'pipe', 'pipe'] })
    const chunks = []
    let size = 0
    let stderr = ''
    let failure = null
    const timer = setTimeout(() => {
      failure = new Error('Magpie 响应超时；请先核对上游记录，不要重复提交')
      child.kill('SIGKILL')
    }, timeout)
    child.stdout.on('data', chunk => {
      size += chunk.length
      if (size > maxBytes) {
        failure = new Error('Magpie 回执过大；请先核对上游记录，不要重复提交')
        child.kill('SIGKILL')
        return
      }
      chunks.push(chunk)
    })
    child.stderr.on('data', chunk => { stderr = (stderr + chunk.toString()).slice(-2000) })
    child.on('error', error => {
      clearTimeout(timer)
      reject(new Error(error.code === 'ENOENT' ? 'magpie CLI 未安装或不在 PATH' : 'magpie CLI 启动失败'))
    })
    child.on('close', code => {
      clearTimeout(timer)
      if (failure) reject(failure)
      else resolve({ code, stdout: Buffer.concat(chunks).toString('utf8'), stderr: redact(stderr) })
    })
  })
}

const textBlocks = reply => (reply?.content || []).filter(c => c?.type === 'text').map(c => c.text || '').join('\n')

/**
 * 从「未知模型被拒绝」的回执里解析图片模型清单。
 *
 * 格式是文档写死的：Pick one of: a, b, c; or leave model out to use the default (x)
 */
export function parseCatalog(text) {
  const listed = /Pick one of:\s*([\s\S]*?); or leave/.exec(text)?.[1]
  const models = listed
    ? listed.split(',').map(value => value.trim()).filter(value => /^[\w.-]+\/[\w./-]+$/.test(value))
    : []
  const defaultModel = /use the default \(([^)]+)\)/.exec(text)?.[1] || ''
  return { models: [...new Set(models)], defaultModel }
}

/** 工具文档里写的当前默认模型（形如 "now codex/gpt-image-2.5)"） */
export function parseDocDefault(text) {
  return /\bnow\s+([\w.-]+\/[\w./-]+)/.exec(text || '')?.[1] || ''
}

/**
 * 只读发现 Magpie 能不能用、有哪些图片模型、视频通道是否就绪。
 *
 * 全程不发生成请求：图片靠「未知模型回出完整清单」拿清单，
 * 视频只看工具文档与 Settings 里配没配视频模型。
 */
export async function discoverMagpie(run = magpieCommand) {
  const unavailable = {
    installed: false,
    image: { ready: false, models: [], defaultModel: '', note: 'Magpie 未安装或不可用，请启动 Magpie 后刷新' },
    video: { ready: false, models: [], note: 'Magpie 视频通道未验证' },
  }
  let imageDocs = { stdout: '' }
  let videoDocs = { stdout: '' }
  try {
    const version = await run(['--version'], { timeout: 10000 })
    if (version.code !== 0) return unavailable
    imageDocs = await run(['tools', 'generate_image'], { timeout: 20000 })
    videoDocs = await run(['tools', 'generate_video'], { timeout: 20000 })
  } catch { return unavailable }

  const image = { ready: false, models: [], defaultModel: '', note: '' }
  const defaultModel = parseDocDefault(imageDocs.stdout)
  // 只有文档明确保证「未知模型会被拒绝并回出完整清单」才发这一次探测。
  if (/an unknown one is refused with the full list/.test(imageDocs.stdout)) {
    try {
      const probe = await run(['tool', 'generate_image', '--json', '--model', `reel-kit-discovery/${randomUUID()}`, '--prompt', 'reel-kit capability discovery'], { timeout: 60000 })
      const reply = JSON.parse(probe.stdout)
      if (reply?.isError) {
        const catalog = parseCatalog(textBlocks(reply))
        image.models = catalog.models.map(id => ({ id, name: id }))
        image.defaultModel = catalog.defaultModel || defaultModel
        image.ready = image.models.length > 0
      }
    } catch { /* 发现失败就如实说发现不到，不猜 */ }
  }
  if (!image.ready) image.note = image.models.length ? 'Magpie 图片清单为空' : '未能从 Magpie 取到图片模型清单；请检查网关是否在运行'

  const video = { ready: false, models: [], note: '' }
  const docSaysNone = /none is set up|no video model is set up/i.test(videoDocs.stdout)
  let videoModel = ''
  try {
    const setting = await run(['settings', 'get', 'videoGen'], { timeout: 15000 })
    const parsed = JSON.parse(setting.stdout)
    if (typeof parsed === 'string' && parsed) videoModel = parsed.trim()
  } catch { /* 读不到就当没配 */ }
  if (videoModel && !docSaysNone) {
    video.ready = true
    video.models = [{ id: videoModel, name: videoModel, minSeconds: 1, maxSeconds: 15 }]
    video.note = 'Magpie 已配置视频模型；能否调用与额度以网关实际返回为准'
  } else if (docSaysNone) {
    video.note = 'Magpie 当前未配置视频模型（Settings → Models）：视频通道不可用'
  } else {
    video.note = 'Magpie 视频模型未验证；配置后再刷新'
  }

  return { installed: true, image, video }
}

/**
 * 从一次工具回执里抽出可用的东西：内联图片、结果地址、任务 ID、模型名。
 *
 * Magpie 的图片可能是内联 base64，也可能给一个保留一小时的地址；
 * 视频给的是带鉴权头的地址，所以视频走「让 Magpie 自己存文件」的路子（见下）。
 */
export function parseReceipt(reply) {
  const receipt = { images: [], urls: [], ids: [], model: '' }
  const addImage = (data, mime) => {
    if (typeof data !== 'string' || !data) return
    if (!/^image\//.test(mime || '')) return
    try {
      const bytes = Buffer.from(data, 'base64')
      if (bytes.length < 8) return
      receipt.images.push({ bytes, mime })
    } catch { /* 不是 base64 就当没有 */ }
  }
  const visit = (node, hint = '') => {
    if (Array.isArray(node)) return node.forEach(item => visit(item, hint))
    if (node && typeof node === 'object') {
      const mime = node.mimeType || node.mime_type || node.mime || hint
      if (typeof node.data === 'string') addImage(node.data, mime)
      if (typeof node.url === 'string' && /^https?:\/\//.test(node.url)) receipt.urls.push(node.url)
      if (typeof node.id === 'string' && node.id) receipt.ids.push(node.id)
      if (typeof node.model === 'string' && node.model && !receipt.model) receipt.model = node.model
      Object.values(node).forEach(value => visit(value, mime))
      return
    }
    if (typeof node === 'string') {
      for (const match of node.matchAll(/https?:\/\/[^\s"'<>)\]]+/g)) receipt.urls.push(match[0])
      const uuid = /\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/i.exec(node)
      if (uuid) receipt.ids.push(uuid[0])
      // 「id <任务号>」这种写法：任务号得像个标识（含数字或连字符），
      // 否则一句说明文字里的 "the id generate_image ..." 也会被当成任务号。
      for (const labelled of node.matchAll(/\b(?:id|task[\s_-]?id)[\s:=#]+([A-Za-z0-9][A-Za-z0-9._:-]{5,})/gi)) {
        const token = labelled[1]
        if (/[0-9-]/.test(token)) receipt.ids.push(token)
      }
    }
  }
  visit(reply)
  receipt.urls = [...new Set(receipt.urls)]
  receipt.ids = [...new Set(receipt.ids)]
  return receipt
}

const IMAGE_SIZE = { '9:16': '1024x1536', '16:9': '1536x1024', '1:1': '1024x1024' }

/** 画幅 → Magpie 认识的尺寸（图片给像素，视频给比例） */
export function sizeFor(kind, ratio) {
  if (kind === 'image') return IMAGE_SIZE[ratio] || '1024x1024'
  return ['9:16', '16:9', '1:1'].includes(ratio) ? ratio : '9:16'
}

export function createMagpie({ run = magpieCommand, download, probe = async () => ({}), store, pause = ms => new Promise(resolve => setTimeout(resolve, ms)) } = {}) {
  if (!store) throw new Error('Magpie 通道需要 store')
  if (!download) throw new Error('Magpie 通道需要 download')

  /** 调一次工具，把 Magpie 自己的拒绝理由原样抛出来 */
  const callTool = async (tool, args, { timeout = 120000 } = {}) => {
    const result = await run(['tool', tool, ...args, '--json'], { timeout })
    let reply = null
    try { reply = JSON.parse(result.stdout) } catch { reply = null }
    if (reply?.isError) throw new Error(textBlocks(reply) || `Magpie 拒绝了 ${tool} 调用`)
    if (result.code !== 0) throw new Error(redact(result.stderr).slice(-300) || `Magpie ${tool} 调用失败（退出码 ${result.code}）`)
    if (!reply) throw new Error('Magpie 未返回完整 JSON 回执；请先核对上游记录，不要重复提交')
    return reply
  }

  /** 把回执变成字节：内联图 → 直接用；地址 → 下载（图片地址无需鉴权） */
  const materialise = async receipt => {
    if (receipt.images.length) return { bytes: receipt.images[0].bytes, mime: receipt.images[0].mime }
    if (receipt.urls.length) {
      const { bytes, mime } = await download(receipt.urls[0])
      return { bytes, mime: mime || 'image/png' }
    }
    return null
  }

  /** 目录里出了文件吗 */
  const filesIn = dir => {
    try { return readdirSync(dir).filter(name => statSync(join(dir, name)).isFile()) } catch { return [] }
  }

  /** 轮询 generation_result，直到 Magpie 把文件落到目录里 */
  const waitForFile = async (dir, taskId) => {
    for (let attempt = 0; attempt < 60; attempt++) {
      await pause(5000)
      const result = await run(['tool', 'generation_result', '--id', taskId, '-o', dir], { timeout: 120000 })
      const saved = filesIn(dir)
      if (saved.length) return { bytes: readFileSync(join(dir, saved[0])) }
      let reply = null
      try { reply = JSON.parse(result.stdout) } catch { reply = null }
      if (reply?.isError) throw new Error(textBlocks(reply) || `Magpie 任务 ${taskId} 未成功`)
    }
    throw new Error(`Magpie 任务 ${taskId} 仍未出片；请过一会儿取回，不必重新提交`)
  }

  /**
   * 视频走「让 Magpie 自己把文件写到磁盘」的路子。
   *
   * 视频结果地址带着网关的鉴权头，reel-kit 拿不到那个头 ——
   * 硬下载只会 401。magpie CLI 手里有头，所以由它把 mp4 落到本地目录。
   */
  const runVideo = async ({ input, progress }) => {
    const dir = mkdtempSync(join(tmpdir(), 'reel-magpie-'))
    try {
      const args = ['tool', 'generate_video', '--prompt', input.prompt]
      if (input.model) args.push('--model', input.model)
      if (input.duration) args.push('--seconds', String(Math.min(15, Math.max(1, Math.round(input.duration)))))
      args.push('--size', sizeFor('video', input.ratio))
      if (input.referencePath) args.push('--image', input.referencePath)
      const result = await run(args.concat(['-o', dir]), { timeout: 600000 })
      const saved = filesIn(dir)
      if (saved.length) return { bytes: readFileSync(join(dir, saved[0])), mime: 'video/mp4', taskId: '' }
      // 没出文件：要么是被拒绝，要么是「还在做，给了个 id」。
      let reply = null
      try { reply = JSON.parse(result.stdout) } catch { reply = null }
      if (reply?.isError) throw new Error(textBlocks(reply) || 'Magpie 拒绝了视频生成')
      const receipt = parseReceipt(reply)
      const taskId = receipt.ids[0] || ''
      if (!taskId) throw new Error('Magpie 没有返回视频任务 ID；请先核对上游记录，不要重复提交')
      progress(`Magpie 视频任务 ${taskId} 已受理，等待生成…`, { upstream: { channel: 'magpie', kind: 'video', taskId, model: input.model || '' } })
      const done = await waitForFile(dir, taskId)
      return { bytes: done.bytes, mime: 'video/mp4', taskId }
    } finally { rmSync(dir, { recursive: true, force: true }) }
  }

  return {
    discover: () => discoverMagpie(run),

    /**
     * 生成一张图。
     *
     * 回执先落任务（任务 ID / 结果地址），再下载素材 ——
     * 下载失败时用户能从任务上取回，不会再付一遍钱。
     */
    async generateImage({ input, progress, referencePath }) {
      const args = ['tool', 'generate_image', '--prompt', input.prompt]
      if (input.model) args.push('--model', input.model)
      args.push('--size', sizeFor('image', input.ratio))
      if (referencePath) args.push('--reference_images', referencePath)
      const reply = await callTool('generate_image', args, { timeout: 300000 })
      const receipt = parseReceipt(reply)
      const taskId = receipt.ids[0] || ''
      const url = receipt.urls[0] || ''
      if (taskId || url) {
        progress('Magpie 已受理，正在取回素材…', { upstream: { channel: 'magpie', kind: 'image', taskId, url, model: input.model || '' } })
      }
      let material = await materialise(receipt)
      if (!material && taskId) {
        for (let attempt = 0; attempt < 60 && !material; attempt++) {
          await pause(5000)
          const polled = await callTool('generation_result', ['--id', taskId], { timeout: 120000 })
          material = await materialise(parseReceipt(polled))
        }
        if (!material) throw new Error(`Magpie 任务 ${taskId} 仍未出图；请过一会儿取回，不必重新提交`)
      }
      if (!material) throw new Error('Magpie 未返回图片素材；请先核对上游记录，不要重复提交')
      return { ...material, taskId, url, actualModel: receipt.model || '' }
    },

    /** 生成视频（可用性由状态接口判定，这里只负责调用） */
    async generateVideo({ input, progress }) {
      const result = await runVideo({ input, progress })
      return { ...result, actualModel: '' }
    },

    /** 取回：只查同一个任务，绝不重新提交 */
    async resume({ upstream, input, progress }) {
      if (upstream.taskId) {
        progress(`正在查询 Magpie 任务 ${upstream.taskId}…`)
        if (upstream.kind === 'video') {
          const dir = mkdtempSync(join(tmpdir(), 'reel-magpie-'))
          try {
            const done = await waitForFile(dir, upstream.taskId)
            const probeInfo = await probe('video.mp4', done.bytes)
            return store.addAsset(input.projectId, 'Magpie 视频.mp4', 'video/mp4', done.bytes, {
              source: 'Magpie', origin: 'ai', engine: `Magpie 出视频${upstream.model ? ' · ' + upstream.model : ''}`,
              prompt: input.prompt || '', requestedModel: upstream.model || '', actualModel: 'unknown',
              upstreamTaskId: String(upstream.taskId), ...probeInfo,
            })
          } finally { rmSync(dir, { recursive: true, force: true }) }
        }
        const polled = await callTool('generation_result', ['--id', upstream.taskId], { timeout: 120000 })
        const receipt = parseReceipt(polled)
        const material = await materialise(receipt)
        if (!material) throw new Error(`Magpie 任务 ${upstream.taskId} 仍未出图；请过一会儿再取回，不必重新提交`)
        const probeInfo = await probe('image', material.bytes)
        return store.addAsset(input.projectId, 'Magpie 图片', material.mime || 'image/png', material.bytes, {
          source: 'Magpie', origin: 'ai', engine: `Magpie 生图${upstream.model ? ' · ' + upstream.model : ''}`,
          prompt: input.prompt || '', requestedModel: upstream.model || '', actualModel: receipt.model || 'unknown',
          upstreamTaskId: String(upstream.taskId), ...probeInfo,
        })
      }
      if (upstream.url) {
        // 图片结果地址在 Magpie 侧保留一小时，可以直接再取一次。
        progress('正在重新取回 Magpie 素材…')
        const { bytes, mime } = await download(upstream.url)
        const probeInfo = await probe('image', bytes)
        return store.addAsset(input.projectId, 'Magpie 图片', mime || 'image/png', bytes, {
          source: 'Magpie', origin: 'ai', engine: `Magpie 生图${upstream.model ? ' · ' + upstream.model : ''}`,
          prompt: input.prompt || '', requestedModel: upstream.model || '', actualModel: 'unknown',
          ...probeInfo,
        })
      }
      throw new Error('这条记录没有 Magpie 的任务 ID 或结果地址，无法取回；请核对 Magpie 记录后再提交')
    },
  }
}
