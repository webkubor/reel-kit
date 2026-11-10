import test from 'node:test'
import assert from 'node:assert/strict'
import { createStudio } from '../src/studio/server.mjs'
import { mkdtempSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { promisify } from 'node:util'
import { execFile } from 'node:child_process'

const run = promisify(execFile)

test('Mac 端端到端闭环实测：Edge 在线高保真配音 + 转场动效 + 混音预设', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-studio-mac-e2e-'))
  const { server, store } = createStudio({ root })

  await new Promise(r => server.listen(4398, '127.0.0.1', r))
  t.after(() => server.close())

  // 1. 创建项目
  const resProj = await fetch('http://127.0.0.1:4398/api/projects', {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'x-reel-action': 'studio' },
    body: JSON.stringify({ name: 'Mac 闭环验证项目' }),
  })
  assert.equal(resProj.status, 201)
  const proj = await resProj.json()
  assert.ok(proj.id)

  // 2. 提交带有新功能（Edge TTS、预设、转场、语速）的口播任务
  const payload = {
    kind: 'talk',
    projectId: proj.id,
    title: 'Mac 闭环验证成片',
    captions: '代码与模型分层架构\n微软高保真神经语音\n一键预设与平滑转场',
    voiceBackend: 'edge',
    persona: 'zh-CN-XiaoxiaoNeural',
    preset: 'clear-speech',
    speed: 1.1,
    transition: 'fade',
    transitionDuration: 0.35,
  }

  const resJob = await fetch('http://127.0.0.1:4398/api/jobs', {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'x-reel-action': 'studio' },
    body: JSON.stringify(payload),
  })
  if (resJob.status !== 202) {
    const errText = await resJob.text()
    console.error('Job submission failed with status:', resJob.status, errText)
  }
  assert.equal(resJob.status, 202)
  const job = await resJob.json()
  assert.ok(job.id)

  // 3. 轮询直到生成完成
  let completed = null
  for (let i = 0; i < 90; i++) {
    await new Promise(r => setTimeout(r, 1000))
    const resState = await fetch('http://127.0.0.1:4398/api/state')
    const state = await resState.json()
    const current = state.jobs.find(j => j.id === job.id)
    if (current && ['succeeded', 'failed', 'interrupted'].includes(current.status)) {
      completed = current
      break
    }
  }

  assert.ok(completed, '任务未在 90 秒内完成')
  assert.equal(completed.status, 'succeeded', `任务失败: ${completed.error || completed.message}`)

  // 4. 断言工序分层与引擎标签
  assert.ok(completed.steps && completed.steps.length >= 4)
  const voiceSteps = completed.steps.filter(s => s.id.startsWith('voice-'))
  assert.equal(voiceSteps.length, 3, '应生成 3 句配音')
  for (const vs of voiceSteps) {
    assert.equal(vs.origin, 'free-online')
    assert.match(vs.engine, /Edge Neural TTS/)
  }

  const composeStep = completed.steps.find(s => s.id === 'compose')
  assert.ok(composeStep)
  assert.equal(composeStep.origin, 'code')
  assert.match(composeStep.detail, /转场: fade/)

  // 5. 校验成片流元信息
  const resState = await fetch('http://127.0.0.1:4398/api/state')
  const state = await resState.json()
  const asset = state.assets.find(a => a.id === completed.assetId)
  assert.ok(asset)
  assert.equal(asset.mime, 'video/mp4')
  assert.equal(asset.width, 1080)
  assert.equal(asset.height, 1920)
  assert.ok(asset.duration > 3, `成片时长过短: ${asset.duration}`)
  assert.equal(asset.settings.transition, 'fade')
  assert.equal(asset.settings.voiceGain, 1.5) // clear-speech 预设人声增益为 1.5

  const assetFile = join(root, 'media', `${asset.id}.mp4`)
  const { stdout } = await run('ffprobe', [
    '-v', 'quiet', '-print_format', 'json',
    '-show_streams', '-show_format', assetFile,
  ])
  const probe = JSON.parse(stdout)
  const vStream = probe.streams.find(s => s.codec_type === 'video')
  const aStream = probe.streams.find(s => s.codec_type === 'audio')

  assert.ok(vStream, '成片必须包含视频流')
  assert.equal(vStream.width, 1080)
  assert.equal(vStream.height, 1920)

  assert.ok(aStream, '成片必须包含音频流')
  assert.equal(aStream.sample_rate, '48000') // 48kHz 标准广播级采样率
  assert.equal(aStream.channels, 2)          // 立体声
})
