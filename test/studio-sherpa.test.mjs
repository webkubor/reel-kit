import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStudio } from '../src/studio/server.mjs'
import { createProviders } from '../src/studio/providers.mjs'
import { createStore } from '../src/studio/store.mjs'

// 随应用分发的本机离线语音引擎（sherpa-onnx）。
//
// 这一组测试守住的是「下载即用」这条底线：
// 引擎没装时不能谎称可用；装了就必须自己认出来，不该再让用户去装 Python。

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'reel-studio-sherpa-'))
  const store = createStore(root)
  const providers = createProviders(store, { fetcher: async () => { throw new Error('offline') } })
  t.after(() => rmSync(root, { recursive: true, force: true }))
  return { root, store, providers }
}

// 造一份「装好了」的目录布局：bin 里一个引擎、model 里一个模型。
function fakeEngine(t) {
  const home = mkdtempSync(join(tmpdir(), 'reel-sherpa-home-'))
  t.after(() => rmSync(home, { recursive: true, force: true }))
  mkdirSync(join(home, 'bin'), { recursive: true })
  mkdirSync(join(home, 'model'), { recursive: true })
  writeFileSync(join(home, 'bin', process.platform === 'win32' ? 'sherpa-onnx-offline-tts.exe' : 'sherpa-onnx-offline-tts'), '')
  writeFileSync(join(home, 'model', 'zh.onnx'), '')
  writeFileSync(join(home, 'model', 'tokens.txt'), '')
  writeFileSync(join(home, 'model', 'lexicon.txt'), '')
  return home
}

function withHome(t, home) {
  const previous = process.env.REEL_SHERPA_HOME
  process.env.REEL_SHERPA_HOME = home
  t.after(() => { if (previous === undefined) delete process.env.REEL_SHERPA_HOME; else process.env.REEL_SHERPA_HOME = previous })
}

test('本机离线引擎没随应用带上时，这条通道必须说未就绪', async t => {
  const { providers } = fixture(t)
  withHome(t, mkdtempSync(join(tmpdir(), 'reel-sherpa-empty-')))

  const status = await providers.status()
  const channel = status.voice.channels.find(c => c.id === 'sherpa')
  assert.ok(channel, '得有「本机 · 离线」这条通道')
  assert.equal(channel.ready, false)
  assert.match(channel.note, /没找到随应用分发的本机语音引擎/)
})

test('引擎随应用带上了，通道自己就绪，不用再让用户装任何东西', async t => {
  const { providers } = fixture(t)
  withHome(t, fakeEngine(t))

  const status = await providers.status()
  const channel = status.voice.channels.find(c => c.id === 'sherpa')
  assert.equal(channel.ready, true)
  assert.match(channel.note, /随应用一起打包/)
  assert.equal(status.voice.ready, true)
  assert.match(status.voice.note, /不用装东西/)
  // 离线音色要给得出名字，否则用户看到一个空的下拉
  assert.ok(channel.voices.length > 0)
  assert.match(channel.voices[0].name, /离线/)
})

test('模型不齐（有引擎没模型）时不算就绪 —— 半套引擎合不出声音', async t => {
  const { providers } = fixture(t)
  const home = fakeEngine(t)
  rmSync(join(home, 'model', 'lexicon.txt'))
  withHome(t, home)

  const status = await providers.status()
  const channel = status.voice.channels.find(c => c.id === 'sherpa')
  assert.equal(channel.ready, false)
})

test('离线通道要能真的提交口播任务，不能在校验那一步就被拒掉', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-sherpa-job-'))
  const app = createStudio({
    root,
    providers: {
      status: async () => ({ image: { ready: true } }),
      generate: async () => { throw new Error('provider offline') },
    },
  })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)); rmSync(root, { recursive: true, force: true }) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const request = async (path, input) => fetch(base + path, {
    method: input === undefined ? 'GET' : 'POST',
    headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json' },
    body: input === undefined ? undefined : JSON.stringify(input),
  })
  const project = await (await request('/api/projects', { name: '离线配音' })).json()

  const res = await request('/api/jobs', {
    kind: 'talk',
    projectId: project.id,
    title: '离线配音试一条',
    captions: '这一句的旁白由随应用自带的本机离线引擎合成。',
    voiceBackend: 'sherpa',
    persona: 'zh-fanchen-c',
  })
  const body = await res.json()
  assert.equal(res.status, 202, `任务没被接受：${JSON.stringify(body)}`)
  assert.equal(body.input.voiceBackend, 'sherpa')

  // 等这个任务跑完再收尾：测试目录一删，还在写状态的任务会写到空目录上。
  for (let i = 0; i < 60; i++) {
    const state = await (await request('/api/state')).json()
    const job = state.jobs.find(j => j.id === body.id)
    if (job && ['failed', 'succeeded', 'interrupted'].includes(job.status)) break
    await new Promise(resolve => setTimeout(resolve, 100))
  }
})
