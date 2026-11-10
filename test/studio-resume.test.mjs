import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStudio } from '../src/studio/server.mjs'
import { createStore } from '../src/studio/store.mjs'
import { createProviders, customChannelIssues, loadCustomChannels } from '../src/studio/providers.mjs'
import { recommendationSettings, reusableVoice } from '../src/studio/video-settings.mjs'

async function fixture(t, providers) {
  const root = mkdtempSync(join(tmpdir(), 'reel-resume-'))
  const app = createStudio({ root, providers })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)); rmSync(root, { recursive: true, force: true }) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const request = async (path, input) => fetch(base + path, { method: input === undefined ? 'GET' : 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json' }, body: input === undefined ? undefined : JSON.stringify(input) })
  const project = await (await request('/api/projects', { name: 'Test project' })).json()
  return { ...app, root, base, request, project }
}

test('付费任务失败后可以取回上游产物，不必重新提交（不重复计费）', async t => {
  let submitted = 0, resumed = 0
  const providers = {
    status: async () => ({ video: { ready: true } }),
    // 第一次生成：提交成功但下载失败 —— 上游已经计费
    generate: async (input, progress) => {
      submitted++
      progress('已提交', { upstream: { channel: 'minimax', kind: 'video', taskId: 'task-123', model: 'MiniMax-H3', profile: 'primary' } })
      throw new Error('下载超时')
    },
    // 取回：只查同一个任务，不重新提交
    resume: async ({ upstream, input }) => {
      resumed++
      assert.equal(upstream.taskId, 'task-123')
      return { id: 'asset-1', projectId: input.projectId, mime: 'video/mp4', name: 'H3 视频.mp4' }
    },
  }
  const { request, project } = await fixture(t, providers)
  const accepted = await request('/api/jobs', { projectId: project.id, kind: 'video', prompt: '一只猫走过窗台', backend: 'minimax' })
  assert.equal(accepted.status, 202)
  const jobId = (await accepted.json()).id
  await new Promise(resolve => setTimeout(resolve, 60))
  const failed = await (await request(`/api/state`)).json()
  const job = failed.jobs.find(j => j.id === jobId)
  assert.equal(job.status, 'failed')
  // 上游任务 ID 必须留在任务里，否则失败后就找不回那个已经计费的任务
  assert.equal(job.upstream.taskId, 'task-123')

  const resumedJob = await request('/api/jobs/resume', { jobId })
  assert.equal(resumedJob.status, 200)
  assert.equal(resumed, 1)
  assert.equal(submitted, 1, '取回不能变成第二次提交')
  const after = await (await request(`/api/state`)).json()
  assert.equal(after.jobs.find(j => j.id === jobId).status, 'succeeded')
})

test('没有上游任务 ID 时不假装能取回', async t => {
  const providers = { status: async () => ({ video: { ready: true } }), generate: async () => { throw new Error('provider offline') } }
  const { request, project } = await fixture(t, providers)
  const accepted = await request('/api/jobs', { projectId: project.id, kind: 'video', prompt: 'x', backend: 'minimax' })
  const jobId = (await accepted.json()).id
  await new Promise(resolve => setTimeout(resolve, 60))
  const refused = await request('/api/jobs/resume', { jobId })
  assert.equal(refused.status, 400)
})

test('项目偏好会记住通道、模型、音色与账户', async t => {
  const providers = { status: async () => ({}), generate: async () => { throw new Error('provider offline') } }
  const { request, project } = await fixture(t, providers)
  const saved = await request('/api/project-settings', { projectId: project.id, settings: { platform: 'channels', voiceSource: 'tts', backend: 'minimax', model: 'speech-02-hd', voiceId: 'male-qn-qingse', profile: 'secondary' } })
  assert.equal(saved.status, 200)
  const state = await (await request(`/api/state`)).json()
  const settings = state.projects.find(p => p.id === project.id).recommendationSettings
  assert.equal(settings.backend, 'minimax')
  assert.equal(settings.model, 'speech-02-hd')
  assert.equal(settings.voiceId, 'male-qn-qingse')
  assert.equal(settings.profile, 'secondary')
})

test('偏好里塞路径或控制字符会被拒', async t => {
  const providers = { status: async () => ({}), generate: async () => { throw new Error('provider offline') } }
  const { request, project } = await fixture(t, providers)
  for (const bad of [{ model: '../../etc/passwd' }, { voiceId: 'a/b' }]) {
    const rejected = await request('/api/project-settings', { projectId: project.id, settings: { platform: 'channels', voiceSource: 'tts', ...bad } })
    assert.equal(rejected.status, 400, JSON.stringify(bad))
  }
})

test('推荐设置的通道、模型、音色、账户会跟着片子走', () => {
  const settings = recommendationSettings({ platform: 'channels', backend: 'minimax', model: 'speech-02-hd', voiceId: 'male-qn-qingse', profile: 'secondary' })
  assert.equal(settings.voiceBackend, 'minimax')
  assert.equal(settings.voiceModel, 'speech-02-hd')
  assert.equal(settings.voiceId, 'male-qn-qingse')
  assert.equal(settings.voiceProfile, 'secondary')
})

test('不同账户的同一段声音不可复用', () => {
  const asset = { mime: 'audio/mpeg', prompt: '一句', tone: '', persona: 'male-qn-qingse', requestedModel: 'speech-02-hd', source: 'MiniMax TTS', profile: 'primary' }
  assert.equal(reusableVoice(asset, { backend: 'minimax', model: 'speech-02-hd', voiceId: 'male-qn-qingse', profile: 'primary' }, '一句'), true)
  assert.equal(reusableVoice(asset, { backend: 'minimax', model: 'speech-02-hd', voiceId: 'male-qn-qingse', profile: 'secondary' }, '一句'), false)
})

// 在线语音把语速烤进了音频：语速对不上还复用，用户就会看到「填了语速没反应」。
test('语速对不上的旧配音不可复用，本机离线引擎不受影响', () => {
  const online = { mime: 'audio/mpeg', prompt: '一句', tone: '', persona: 'v', requestedModel: 'edge-neural', source: '微软 Edge · 在线高保真', voiceBackend: 'edge', speed: 1 }
  assert.equal(reusableVoice(online, { backend: 'edge', model: 'edge-neural', voiceId: 'v', speed: 1 }, '一句'), true)
  assert.equal(reusableVoice(online, { backend: 'edge', model: 'edge-neural', voiceId: 'v', speed: 1.2 }, '一句'), false)
  // 本机离线引擎的语速是出片后再变速的，复用旧配音不受语速影响
  const local = { mime: 'audio/wav', prompt: '一句', tone: '', persona: 'v', requestedModel: 'voxflow-local', source: 'VoxFlow', voiceBackend: 'voxflow' }
  assert.equal(reusableVoice(local, { backend: 'voxflow', model: 'voxflow-local', voiceId: 'v', speed: 1.2 }, '一句'), true)
})

test('自定义通道：配置有问题要说得出来，配错的通道不出现', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-providers-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const config = join(root, '.reel-kit', 'providers.json')
  mkdirSync(join(root, '.reel-kit'), { recursive: true })
  writeFileSync(config, JSON.stringify({ channels: [
    { id: 'good-image', kind: 'image', protocol: 'openai', baseUrl: 'https://example.com', model: 'seedream-4', env: 'REEL_TEST_IMAGE_KEY' },
    { id: 'bad-video', kind: 'video', protocol: 'openai', baseUrl: 'https://example.com', model: 'seedance', env: 'REEL_TEST_VIDEO_KEY' },
    { id: 'no-model', kind: 'image', baseUrl: 'https://example.com', env: 'REEL_TEST_IMAGE_KEY' },
    { id: 'tts-one', kind: 'tts', baseUrl: 'https://example.com', model: 'm', env: 'REEL_TEST_IMAGE_KEY' },
  ] }))
  const previous = process.env.HOME
  const previousProfile = process.env.USERPROFILE
  process.env.HOME = root
  if (process.platform === 'win32') process.env.USERPROFILE = root
  t.after(() => {
    if (previous === undefined) delete process.env.HOME
    else process.env.HOME = previous
    if (previousProfile === undefined) delete process.env.USERPROFILE
    else process.env.USERPROFILE = previousProfile
  })
  // 配置路径取自 homedir()，改 HOME 后重新读取
  const channels = loadCustomChannels()
  assert.equal(channels.some(c => c.id === 'good-image'), true, '配置正确的通道应当出现')
  assert.equal(channels.some(c => c.id === 'bad-video'), false, '协议不匹配的通道不该出现')
  assert.equal(channels.some(c => c.id === 'no-model'), false, '缺 model 的通道不该出现')
  assert.equal(channels.some(c => c.id === 'tts-one'), false, '未实现的自定义 TTS 不该出现')
  const issues = customChannelIssues()
  assert.equal(issues.problems.length >= 3, true, '配置问题要被列出来')
  assert.equal(issues.problems.some(p => p.includes('no-model')), true)
  assert.equal(issues.problems.some(p => p.includes('tts-one')), true)
})

test('自定义生图通道：模型以用户填的为准发出', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-custom-image-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const previous = process.env.HOME
  const previousProfile = process.env.USERPROFILE
  process.env.HOME = root
  if (process.platform === 'win32') process.env.USERPROFILE = root
  t.after(() => {
    if (previous === undefined) delete process.env.HOME
    else process.env.HOME = previous
    if (previousProfile === undefined) delete process.env.USERPROFILE
    else process.env.USERPROFILE = previousProfile
  })
  mkdirSync(join(root, '.reel-kit'), { recursive: true })
  writeFileSync(join(root, '.reel-kit', 'providers.json'), JSON.stringify({ channels: [
    { id: 'seedream', kind: 'image', protocol: 'openai', baseUrl: 'https://example.com', model: 'seedream-4', env: 'REEL_TEST_IMAGE_KEY' },
  ] }))
  process.env.REEL_TEST_IMAGE_KEY = 'test-key'
  const store = createStore(root)
  store.state.projects.push({ id: 'p1', name: 'p', createdAt: new Date().toISOString() })
  let sent
  const providers = createProviders(store, {
    fetcher: async (url, options) => {
      sent = { url, body: JSON.parse(options.body) }
      return { ok: true, status: 200, json: async () => ({ data: [{ b64_json: Buffer.from('image-bytes').toString('base64') }] }) }
    },
  })
  const asset = await providers.generate({ kind: 'image', projectId: 'p1', prompt: '橘猫', backend: 'seedream', model: 'seedream-4-turbo' }, () => {})
  assert.equal(sent.body.model, 'seedream-4-turbo', '用户填的模型要真的发出去')
  assert.equal(sent.url, 'https://example.com/images/generations')
  assert.equal(asset.requestedModel, 'seedream-4-turbo')
})
