import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStudio } from '../src/studio/server.mjs'
import { createStore } from '../src/studio/store.mjs'
import { createProviders, h3Body, museavJobId } from '../src/studio/providers.mjs'
import { resolveBgm, resolveBgmEntry } from '../src/bgm-library.mjs'

async function fixture(t, generate = async () => { throw new Error('provider offline') }, extraProviders = {}) {
  const root = mkdtempSync(join(tmpdir(), 'reel-studio-channel-'))
  const app = createStudio({ root, providers: { status: async () => ({ image: { ready: true } }), generate, ...extraProviders } })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)); rmSync(root, { recursive: true, force: true }) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const request = async (path, input, headers = {}) => fetch(base + path, { method: input === undefined ? 'GET' : 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json', ...headers }, body: input === undefined ? undefined : JSON.stringify(input) })
  const project = await (await request('/api/projects', { name: 'Test project' })).json()
  return { ...app, root, base, request, project }
}

test('配乐：对外 API 仍是路径字符串，元数据走 resolveBgmEntry', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-bgm-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const music = join(root, '轻快.mp3')
  writeFileSync(music, 'fixture-audio')
  const manifest = join(root, 'manifest.json')
  writeFileSync(manifest, JSON.stringify({ bgm: [{ alias: '轻快', key: music, license: 'CC0 / 免署名可商用', source: 'Mixkit' }] }))
  const previous = process.env.REEL_BGM_MANIFEST
  process.env.REEL_BGM_MANIFEST = manifest
  t.after(() => { if (previous === undefined) delete process.env.REEL_BGM_MANIFEST; else process.env.REEL_BGM_MANIFEST = previous })

  // 公开 API 的形状不能变：字符串路径，不是对象
  assert.equal(await resolveBgm('轻快'), music)
  const entry = await resolveBgmEntry('轻快', { exact: true })
  assert.equal(entry.file, music)
  assert.equal(entry.license, 'CC0 / 免署名可商用')
  assert.equal(entry.source, 'Mixkit')

  // 没给东西不能炸 —— 以前会把 undefined.file 当对象读，直接抛 TypeError
  assert.equal(await resolveBgm(undefined), undefined)
})

test('配乐：HTTP 侧只认清单里的精确别名，不能变成任意文件读取器', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-bgm-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const music = join(root, '轻快.mp3')
  writeFileSync(music, 'fixture-audio')
  const manifest = join(root, 'manifest.json')
  writeFileSync(manifest, JSON.stringify({ bgm: [{ alias: '轻快', key: music, license: 'CC0' }] }))
  const previous = process.env.REEL_BGM_MANIFEST
  process.env.REEL_BGM_MANIFEST = manifest
  t.after(() => { if (previous === undefined) delete process.env.REEL_BGM_MANIFEST; else process.env.REEL_BGM_MANIFEST = previous })

  // 模糊命中会拿「」「x」这类碎片去撞库
  assert.equal(await resolveBgmEntry('快', { exact: true }), undefined)
  // 本地路径默认不放行：HTTP 接口用它等于让服务器替人读文件
  assert.equal(await resolveBgmEntry(music, { exact: true }), undefined)
  assert.equal((await resolveBgmEntry(music, { allowLocalPath: true })).file, music)
})

test('通道与模型可以随任务提交，非法取值在入库前被拒', async t => {
  let received
  const { request, project } = await fixture(t, async input => { received = input; throw new Error('provider offline') })
  const accepted = await request('/api/jobs', { projectId: project.id, kind: 'image', prompt: '一只猫', backend: 'museav', model: 'gpt-image-2.6' })
  assert.equal(accepted.status, 202)
  assert.equal(received.backend, 'museav')
  assert.equal(received.model, 'gpt-image-2.6')
  for (const bad of [{ backend: 'museav; rm -rf /' }, { model: '../../etc/passwd' }, { voiceId: 'a/b' }]) {
    const rejected = await request('/api/jobs', { projectId: project.id, kind: 'image', prompt: 'x', ...bad })
    assert.equal(rejected.status, 400, JSON.stringify(bad))
  }
  // 模型名里本来就有空格与括号（中台的对外名与 MiniMax 音色 ID），不能一刀切拒掉
  const spaced = await request('/api/jobs', { projectId: project.id, kind: 'video', prompt: 'x', backend: 'museav', model: 'Seedance 2.0 Mini', voiceId: 'Chinese (Mandarin)_Warm_Girl' })
  assert.equal(spaced.status, 202)
})

test('MiniMax TTS：合成成功才落库，并记下请求模型与音色', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-tts-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const previous = process.env.MINIMAX_API_KEY
  process.env.MINIMAX_API_KEY = 'fixture-key'
  t.after(() => { if (previous === undefined) delete process.env.MINIMAX_API_KEY; else process.env.MINIMAX_API_KEY = previous })
  const store = createStore(root)
  const project = { id: 'p1' }
  store.state.projects.push({ id: 'p1', name: 'p', createdAt: new Date().toISOString() })
  let sent
  const providers = createProviders(store, {
    fetcher: async (url, options) => {
      sent = { url, body: JSON.parse(options.body) }
      return { ok: true, status: 200, json: async () => ({ base_resp: { status_code: 0 }, data: { audio: '494433' } }) }
    },
  })
  const asset = await providers.generate({ kind: 'speech', projectId: project.id, prompt: '你好', backend: 'minimax', model: 'speech-02-hd', voiceId: 'male-qn-qingse' }, () => {})
  assert.equal(sent.url, 'https://api.minimaxi.com/v1/t2a_v2')
  assert.equal(sent.body.voice_setting.voice_id, 'male-qn-qingse')
  assert.equal(sent.body.model, 'speech-02-hd')
  assert.equal(asset.requestedModel, 'speech-02-hd')
  assert.equal(asset.persona, 'male-qn-qingse')
  assert.equal(asset.mime, 'audio/mpeg')
  // 上游没给回执就不许拿请求值冒充实际模型
  assert.equal(asset.actualModel, 'unknown')
})

test('MiniMax TTS：上游报错时不落库，也不自动换账户重试', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-tts-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const previous = process.env.MINIMAX_API_KEY
  process.env.MINIMAX_API_KEY = 'fixture-key'
  t.after(() => { if (previous === undefined) delete process.env.MINIMAX_API_KEY; else process.env.MINIMAX_API_KEY = previous })
  const store = createStore(root)
  let calls = 0
  const providers = createProviders(store, {
    fetcher: async () => { calls++; return { ok: true, status: 200, json: async () => ({ base_resp: { status_code: 2054, status_msg: 'voice id not exist' } }) } },
  })
  await assert.rejects(
    () => providers.generate({ kind: 'speech', projectId: 'p1', prompt: '你好', backend: 'minimax', voiceId: 'not-a-real-voice' }, () => {}),
    /2054/
  )
  assert.equal(calls, 1)
  assert.equal(store.state.assets.length, 0)
})

test('H3 直连：模型可以自填，但请求体仍是 V2 契约', () => {
  const body = h3Body({ prompt: 'walk', duration: 6, ratio: '9:16', model: 'MiniMax-H3-Turbo' }, undefined)
  assert.equal(body.model, 'MiniMax-H3-Turbo')
  assert.equal(body.resolution, '768P')
  assert.equal(body.content.length, 1)
})

test('MuseAV 任务 ID 从 stderr 里取，取不到就是取不到', () => {
  assert.equal(museavJobId('视频任务已提交: 0c1a9e15-aed3-4395-b08f-b705e8ab8aa4\n✅ 完成'), '0c1a9e15-aed3-4395-b08f-b705e8ab8aa4')
  assert.equal(museavJobId('✅ 完成 (3.2s)'), '')
})
