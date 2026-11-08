import { setTimeout as delay } from 'node:timers/promises'
import { command, available, jsonFetch, download, probeBytes } from './process.mjs'

const H3_BASE = 'https://api.minimaxi.com'
export function h3Body(input, referenceUrl) {
  const content = [{ type: 'text', text: input.prompt }]
  if (referenceUrl) content.push({ type: 'image_url', image_url: { url: referenceUrl }, role: 'reference_image' })
  return { model: 'MiniMax-H3', content, ratio: input.ratio, duration: input.duration, resolution: input.resolution || '768P' }
}
function outputUrl(stdout) {
  const urls = stdout.split(/\r?\n/).map(s => s.trim()).filter(s => /^https:\/\/\S+$/.test(s))
  if (!urls.length) throw new Error('生成工具未返回素材地址，请检查 CLI 版本')
  return urls.at(-1)
}
export function createProviders(store, { fetcher = fetch, pause = delay } = {}) {
  const voiceBase = process.env.REEL_VOXFLOW_URL || 'http://127.0.0.1:8866'
  const profiles = [
    { id: 'primary', label: process.env.REEL_MINIMAX_LABEL || 'MiniMax 主账户', key: process.env.MINIMAX_API_KEY },
    { id: 'secondary', label: process.env.REEL_MINIMAX_LABEL_2 || 'MiniMax 第二账户', key: process.env.MINIMAX_API_KEY_2 },
  ]
  const h3 = async (path, key, data) => {
    const response = await fetcher(H3_BASE + path, {
      method: data ? 'POST' : 'GET', headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
      body: data ? JSON.stringify(data) : undefined, signal: AbortSignal.timeout(30000),
    })
    const result = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(`MiniMax HTTP ${response.status}${result.base_resp?.status_msg ? `：${result.base_resp.status_msg}` : ''}；不会自动切换账户重试`)
    if (result.base_resp?.status_code) throw new Error(`MiniMax 拒绝请求（${result.base_resp.status_code}）：${result.base_resp.status_msg || '请核对权限或额度'}`)
    return result
  }
  return {
    async status() {
      const [museav, ffmpeg, voice] = await Promise.all([
        available('museav'), available('ffmpeg'),
        Promise.all([jsonFetch(voiceBase + '/api/status', { signal: AbortSignal.timeout(3000) }), jsonFetch(voiceBase + '/api/personas', { signal: AbortSignal.timeout(3000) })]).catch(() => null),
      ])
      const quotas = await Promise.all(profiles.filter(p => p.key).map(async ({ id, label, key }) => {
        try {
          const response = await jsonFetch('https://www.minimaxi.com/v1/token_plan/remains', { headers: { Authorization: `Bearer ${key}` }, signal: AbortSignal.timeout(6000) })
          if (response.base_resp?.status_code) throw new Error('quota unavailable')
          const general = response.model_remains?.find(m => m.model_name === 'general')
          return { id, label, weeklyRemaining: general?.current_weekly_remaining_percent ?? null, checkedAt: new Date().toISOString() }
        } catch { return { id, label, weeklyRemaining: null } }
      }))
      const raw = voice?.[1]?.personas || []
      const personas = Array.isArray(raw) ? raw.map(p => ({ key: p.key, name: p.name || p.key })) : Object.entries(raw).map(([key, p]) => ({ key, name: p.name || key, hasSample: Boolean(p.has_audio || p.has_ref) }))
      return {
        video: { ready: profiles.some(p => p.key), label: 'MiniMax H3 · 直连', note: '密钥配置不代表额度已验证；提交按所选账户计费', profiles: quotas },
        image: { ready: museav, label: 'MuseAV · 图片生成', note: museav ? '复用本机 CLI 登录态' : '请安装并登录 museav CLI' },
        voice: { ready: Boolean(voice?.[0]?.base_model), label: 'VoxFlow · 本地语音', note: voice ? (voice[0].base_model ? '本地模型已就绪' : '服务在线，Base 模型未就绪') : '未连接：启动 VoxFlow 服务后刷新', designReady: Boolean(voice?.[0]?.design_model), personas },
        export: { ready: ffmpeg, label: 'reel-kit · 合成', note: ffmpeg ? 'FFmpeg 已就绪' : '请安装 FFmpeg' },
      }
    },
    async generate(input, progress) {
      if (input.kind === 'image') {
        progress('MuseAV 正在生成图片…')
        const args = ['gen', '--prompt', input.prompt, '--ratio', input.ratio]
        if (input.referenceId) {
          const asset = store.asset(input.referenceId, input.projectId)
          if (!asset.mime.startsWith('image/')) throw new Error('图片参考必须是图片')
          args.push('--ref', store.path(asset))
        }
        const stdout = await command('museav', args, { timeout: 900000 })
        progress('正在下载图片到本地…')
        const { bytes, mime } = await download(outputUrl(stdout))
        const probe = await probeBytes('image', bytes)
        return store.addAsset(input.projectId, 'MuseAV 图片', mime || 'image/png', bytes, { source: 'MuseAV', origin: 'ai', engine: 'MuseAV 生图', prompt: input.prompt, ...probe })
      }
      if (input.kind === 'video') {
        const profile = profiles.find(p => p.id === (input.profile || 'primary'))
        if (!profile?.key) throw new Error('所选 MiniMax 账户未配置。使用密钥工具注入 MINIMAX_API_KEY / MINIMAX_API_KEY_2 后启动工作台')
        let reference
        if (input.referenceId) {
          const asset = store.asset(input.referenceId, input.projectId)
          if (!asset.mime.startsWith('image/')) throw new Error('H3 人物参考须选择图片')
          progress('正在上传参考图供 H3 读取…')
          reference = outputUrl(await command('museav', ['upload', store.path(asset)]))
        }
        progress('正在提交 MiniMax H3…')
        const response = await h3('/v2/video_generation', profile.key, h3Body(input, reference))
        const taskId = response.task_id || response.task?.id
        if (!taskId) throw new Error('MiniMax 未返回任务 ID；请先核对上游，不要重复提交')
        progress(`H3 任务 ${taskId} 已提交，等待生成…`)
        for (let attempt = 0; attempt < 180; attempt++) {
          await pause(5000)
          const result = await h3(`/v2/query/video_generation/${encodeURIComponent(taskId)}`, profile.key)
          const task = result.task || result
          if (['failed', 'cancelled', 'canceled'].includes(task.status)) throw new Error(`H3 任务 ${taskId} 失败：${task.error?.message || task.error || task.status}`)
          if (task.status === 'succeeded') {
            const url = task.content?.url
            if (!url) throw new Error(`H3 任务 ${taskId} 已完成，但未返回下载地址`)
            progress('H3 已完成，正在下载视频…')
            const { bytes } = await download(url)
            const probe = await probeBytes('video.mp4', bytes)
            return store.addAsset(input.projectId, 'H3 视频.mp4', 'video/mp4', bytes, { source: 'MiniMax H3', origin: 'ai', engine: 'MiniMax H3 视频', upstreamTaskId: String(taskId), prompt: input.prompt, profile: profile.id, ...probe })
          }
        }
        throw new Error(`H3 任务 ${taskId} 仍在处理中，已停止轮询；请核对上游后再操作`)
      }
      if (['speech', 'clone', 'voice-design'].includes(input.kind)) {
        if (input.prompt.length > 400) throw new Error('VoxFlow 单次配音上限 400 字，请分段生成')
        let persona = input.persona
        if (input.kind === 'clone') throw new Error('当前声音服务无法保存克隆所需的参考文本，参考音频克隆暂不可用；可选择已有音色')
        const designing = input.kind === 'voice-design'
        if (!designing && !persona) throw new Error('请选择已有音色，或创建新音色')
        progress(designing ? '正在设计音色，首次生成需要加载模型…' : 'VoxFlow 正在合成配音…')
        const payload = designing ? { voice_name: input.voiceName, text: input.prompt, tone: input.tone, emotion: input.emotion || '', commit: true } : { persona, text: input.prompt, tone: input.tone || '' }
        const submitted = await jsonFetch(voiceBase + (designing ? '/api/design' : '/api/clone'), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) })
        for (let attempt = 0; attempt < 300; attempt++) {
          await pause(2000)
          const { tasks } = await jsonFetch(voiceBase + '/api/tasks')
          const task = tasks.find(t => t.id === submitted.task_id)
          if (!task) continue
          if (['error', 'failed', 'cancelled'].includes(task.status)) throw new Error(task.error || '本地语音生成失败')
          if (task.status === 'done') {
            const result = task.result
            if (!result?.url?.startsWith('/api/audio/')) throw new Error('VoxFlow 未返回音频文件')
            const { bytes } = await download(new URL(result.url, voiceBase))
            const probe = await probeBytes('voice.wav', bytes)
            return store.addAsset(input.projectId, designing ? `${input.voiceName} · 样音.wav` : `配音 · ${input.prompt.slice(0, 16)}.wav`, 'audio/wav', bytes, { source: 'VoxFlow', origin: 'ai', engine: 'VoxFlow 本地语音', prompt: input.prompt, persona: result.persona_key || persona, tone: input.tone || '', voiceName: input.voiceName || '', ...probe })
          }
        }
        throw new Error('语音生成超时，请检查 VoxFlow 任务列表')
      }
      throw new Error('未知生成类型')
    },
  }
}
