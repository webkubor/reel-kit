import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, rmSync, writeFileSync, mkdirSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  normalizeChannel, customChannelIssues, upsertChannel, removeChannel,
  loadCustomChannels, probeChannel, CUSTOM_CONFIG, REEL_CONFIG,
} from '../src/studio/channels.mjs'
import { createProviders } from '../src/studio/providers.mjs'
import { createStore } from '../src/studio/store.mjs'
import { createStudio } from '../src/studio/server.mjs'

// 自定义模型通道：配置解析、问题提示、探活。
//
// 这一组守住的是「用户自己填 / Agent 代填」这条底线：
// 配错了要说得清是哪一项；探活不能花钱（不能一检查就生成一张图）；
// 探活报告里「验证通过」和「没验证」必须分得开。

/** 临时通道配置文件；返回文件路径，并保证原来的环境变量被还原 */
function withConfig(t, channels) {
  const dir = mkdtempSync(join(tmpdir(), 'reel-channels-'))
  const file = join(dir, 'providers.json')
  const previous = process.env.REEL_CHANNELS_FILE
  process.env.REEL_CHANNELS_FILE = file
  t.after(() => {
    if (previous === undefined) delete process.env.REEL_CHANNELS_FILE
    else process.env.REEL_CHANNELS_FILE = previous
    rmSync(dir, { recursive: true, force: true })
  })
  if (channels) writeFileSync(file, JSON.stringify({ channels }, null, 2))
  return file
}

/** 临时环境变量；用完还原，不污染同进程里别的测试 */
function withEnv(t, name, value) {
  const previous = process.env[name]
  process.env[name] = value
  t.after(() => {
    if (previous === undefined) delete process.env[name]
    else process.env[name] = previous
  })
}

/** 把 HOME 指到临时目录，用来测「密钥存在设置页」这条路径 */
function withHome(t) {
  const dir = mkdtempSync(join(tmpdir(), 'reel-home-'))
  withEnv(t, 'HOME', dir)
  if (process.platform === 'win32') withEnv(t, 'USERPROFILE', dir)
  t.after(() => rmSync(dir, { recursive: true, force: true }))
  return dir
}

function jsonResponse(body, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  }
}

test('配置缺项要逐条说出来，不能一声不吭', t => {
  withConfig(t, [
    { id: 'a', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'NOPE_KEY' },
    { id: 'museav', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'K' },
    { id: 'b', kind: 'image', protocol: 'openai', baseUrl: '', model: 'm', env: 'K' },
    { id: 'c', kind: 'image', protocol: 'openai', baseUrl: 'http://x.example', model: 'm', env: 'K' },
  ])
  const { problems } = customChannelIssues()
  assert.ok(problems.some(p => /a：还没填密钥/.test(p)), '要指出哪条还没填密钥')
  assert.ok(problems.some(p => /museav：id 是内置通道保留名/.test(p)), '要挡住占用内置 id')
  assert.ok(problems.some(p => /b：缺少 baseUrl/.test(p)), '要指出缺 baseUrl')
  assert.ok(problems.some(p => /c：/.test(p) && /https/.test(p)), '非本机的 http 地址要被挡住')
})

test('缺关键项的通道不算可用通道，不该出现在选择列表里', t => {
  withConfig(t, [
    { id: 'ok', kind: 'image', protocol: 'minimax-image', baseUrl: 'https://api.minimaxi.com', model: 'image-01', env: 'K' },
    { id: 'no-model', kind: 'image', protocol: 'minimax-image', baseUrl: 'https://api.minimaxi.com', env: 'K' },
    { id: 'minimax', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'K' },
    { id: 'insecure', kind: 'image', protocol: 'openai', baseUrl: 'http://x.example', model: 'm', env: 'K' },
  ])
  const ids = loadCustomChannels().map(c => c.id)
  assert.deepEqual(ids, ['ok'])
})

test('中台通道可以不写模型名，直连厂商必须写', t => {
  assert.equal(normalizeChannel({ id: 'm', kind: 'image', protocol: 'museav', baseUrl: 'https://x.example', env: 'K' })?.model, '')
  assert.equal(normalizeChannel({ id: 'm', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', env: 'K' }), null)
})

test('通道配置能写进文件、也能删掉，密钥不进文件', t => {
  const file = withConfig(t, null)
  upsertChannel({ id: 'x1', label: '测试通道', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'X_KEY' })
  const written = JSON.parse(readFileSync(file, 'utf8'))
  assert.equal(written.channels.length, 1)
  assert.equal(written.channels[0].key, undefined, '密钥不能写进配置文件')
  removeChannel('x1')
  assert.equal(JSON.parse(readFileSync(file, 'utf8')).channels.length, 0)
})

test('配置写坏了不能被下一次保存悄悄覆盖掉', t => {
  const file = withConfig(t, null)
  writeFileSync(file, '{ 这不是 JSON')
  assert.throws(() => upsertChannel({ id: 'x1', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'K' }), /无法解析/)
  assert.equal(readFileSync(file, 'utf8'), '{ 这不是 JSON', '坏文件得原样留着，人还能改回来')
})

test('channels 不是数组时，保存不能把用户的东西抹掉', t => {
  const file = withConfig(t, null)
  writeFileSync(file, JSON.stringify({ channels: { id: 'x' } }))
  assert.throws(() => upsertChannel({ id: 'x1', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'K' }), /数组/)
  assert.ok(readFileSync(file, 'utf8').includes('channels'))
})

test('探活不生成任何内容：空提示词被拒只能证明地址与密钥通', async t => {
  withConfig(t, null)
  withEnv(t, 'PROBE_KEY', 'x')
  const channel = normalizeChannel({ id: 'p', kind: 'image', protocol: 'minimax-image', baseUrl: 'https://api.minimaxi.com', model: 'image-01', env: 'PROBE_KEY' })
  const calls = []
  const result = await probeChannel(channel, {
    fetcher: async (url, init) => {
      calls.push({ url, body: JSON.parse(init.body) })
      return jsonResponse({ base_resp: { status_code: 2013, status_msg: 'invalid params, prompt are required' } })
    },
  })
  assert.equal(result.ok, true)
  assert.equal(calls.length, 1, '探活只该发一次请求')
  assert.equal(calls[0].body.prompt, '', '探活不能带真提示词，否则就花钱了')
  assert.ok(result.checks.some(c => c.name === '鉴权' && c.ok === true), '密钥被认了要说通过')
  // 空提示词被拒**不能**证明模型名对不对：多数接口先查提示词再查模型。
  const model = result.checks.find(c => c.name === '模型')
  assert.equal(model.ok, null, '模型没被真验过，就该说未验证，不能说通过')
})

test('上游明说模型不存在时，探活要报模型失败', async t => {
  withConfig(t, null)
  withEnv(t, 'PROBE_KEY', 'x')
  const channel = normalizeChannel({ id: 'p', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'nope', env: 'PROBE_KEY' })
  const result = await probeChannel(channel, {
    fetcher: async () => jsonResponse({ error: { message: 'The model `nope` does not exist' } }, 404),
  })
  assert.equal(result.ok, false)
  assert.ok(result.checks.some(c => c.name === '模型' && c.ok === false))
})

test('密钥没注入时，探活要说清缺哪个环境变量', async t => {
  withConfig(t, null)
  const channel = normalizeChannel({ id: 'p', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'MISSING_KEY' })
  const result = await probeChannel(channel, { fetcher: async () => { throw new Error('不该发请求') } })
  assert.equal(result.ok, false)
  assert.match(result.checks[0].detail, /MISSING_KEY/)
})

test('上游拒绝密钥时，探活要说清是鉴权问题，不是「通道不存在」', async t => {
  withConfig(t, null)
  withEnv(t, 'PROBE_KEY', 'x')
  const channel = normalizeChannel({ id: 'p', kind: 'image', protocol: 'museav', baseUrl: 'https://manager.museav.top', env: 'PROBE_KEY' })
  const result = await probeChannel(channel, { fetcher: async () => jsonResponse({ error: { message: 'unauthorized' } }, 401) })
  assert.equal(result.ok, false)
  assert.ok(result.checks.some(c => c.name === '鉴权' && c.ok === false && /拒绝/.test(c.detail)))
})

test('中台生图：回执里的任务 ID 在 results 里，轮询走 /api/jobs', async t => {
  withConfig(t, [{
    id: 'museav-http', kind: 'image', protocol: 'museav', baseUrl: 'https://manager.museav.top', env: 'MUS_KEY',
  }])
  withEnv(t, 'MUS_KEY', 'x')

  const root = mkdtempSync(join(tmpdir(), 'reel-museav-store-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const store = createStore(root)
  store.state.projects.push({ id: 'p1', name: '测试', createdAt: new Date().toISOString() })
  store.save()

  const seen = []
  const fetcher = async (url, init = {}) => {
    seen.push(`${init.method || 'GET'} ${url}`)
    // 中台的批量回执：任务 ID 在 results 数组里，顶层没有 jobId
    if (url.includes('generate-batch')) return jsonResponse({ results: [{ ok: true, jobId: 'job-1' }] })
    if (url.includes('/api/jobs')) return jsonResponse({ status: 'done', cdn_url: 'https://cdn.example/a.png' })
    return jsonResponse({})
  }
  const providers = createProviders(store, {
    fetcher,
    pause: async () => {},
    download: async () => ({ bytes: Buffer.from('fake-png'), mime: 'image/png' }),
  })

  const asset = await providers.generate({ kind: 'image', projectId: 'p1', backend: 'museav-http', prompt: '一只猫', ratio: '9:16' }, () => {})
  assert.equal(asset.mime, 'image/png')
  assert.ok(seen.some(u => u.includes('generate-batch')), '要先提交任务')
  assert.ok(seen.some(u => u.includes('/api/jobs?id=job-1')), '出图轮询走 /api/jobs')
})

test('中台出视频：提交走 /api/videos，轮询也走 /api/videos（不是 /api/jobs）', async t => {
  withConfig(t, [{
    id: 'museav-video', kind: 'video', protocol: 'museav', baseUrl: 'https://manager.museav.top', env: 'MUS_VID_KEY',
  }])
  withEnv(t, 'MUS_VID_KEY', 'x')

  const root = mkdtempSync(join(tmpdir(), 'reel-museav-video-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const store = createStore(root)
  store.state.projects.push({ id: 'p1', name: '测试', createdAt: new Date().toISOString() })
  store.save()

  const seen = []
  const fetcher = async (url, init = {}) => {
    seen.push(`${init.method || 'GET'} ${url}`)
    if (init.method === 'POST' && url.endsWith('/api/videos')) return jsonResponse({ job_id: 'vid-1' })
    if (url.includes('/api/videos?id=')) return jsonResponse({ status: 'completed', cdn_url: 'https://cdn.example/v.mp4' })
    return jsonResponse({})
  }
  const providers = createProviders(store, {
    fetcher,
    pause: async () => {},
    download: async () => ({ bytes: Buffer.from('fake-mp4'), mime: 'video/mp4' }),
  })

  const asset = await providers.generate({ kind: 'video', projectId: 'p1', backend: 'museav-video', prompt: '一只猫', ratio: '9:16' }, () => {})
  assert.equal(asset.mime, 'video/mp4')
  assert.ok(seen.some(u => u.startsWith('POST') && u.endsWith('/api/videos')), '视频提交走 /api/videos')
  assert.ok(seen.some(u => u.includes('/api/videos?id=vid-1')), '视频轮询走 /api/videos')
  assert.ok(!seen.some(u => u.includes('/api/jobs')), '视频不该查图片的任务端点')
})

test('厂商直连生图：图片地址在 data.image_urls，不在 data[0].url', async t => {
  withConfig(t, [{
    id: 'minimax-image', kind: 'image', protocol: 'minimax-image', baseUrl: 'https://api.minimaxi.com', model: 'image-01', env: 'MM_KEY',
  }])
  withEnv(t, 'MM_KEY', 'x')

  const root = mkdtempSync(join(tmpdir(), 'reel-minimax-store-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const store = createStore(root)
  store.state.projects.push({ id: 'p1', name: '测试', createdAt: new Date().toISOString() })
  store.save()

  let sent = null
  const providers = createProviders(store, {
    fetcher: async (url, init) => {
      sent = JSON.parse(init.body)
      return jsonResponse({ data: { image_urls: ['https://cdn.example/b.png'] } })
    },
    download: async () => ({ bytes: Buffer.from('fake-png'), mime: 'image/png' }),
  })
  const asset = await providers.generate({ kind: 'image', projectId: 'p1', backend: 'minimax-image', prompt: '一只猫', ratio: '9:16' }, () => {})
  assert.equal(asset.mime, 'image/png')
  assert.equal(sent.aspect_ratio, '9:16', '厂商直连用 aspect_ratio，不是 size')
  assert.equal(sent.size, undefined)
})

test('密钥存在设置页（本机配置）时也能用上，不逼用户开终端', t => {
  const home = withHome(t)
  withConfig(t, [{
    id: 'saved-key', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'SAVED_KEY',
  }])
  mkdirSync(join(home, '.reel-kit'), { recursive: true })
  writeFileSync(REEL_CONFIG(), JSON.stringify({ channelKeys: { 'saved-key': 'sk-from-settings' } }))
  const channel = loadCustomChannels().find(c => c.id === 'saved-key')
  assert.equal(channel.key, 'sk-from-settings')
  assert.equal(channel.keySource, 'config')
})

test('环境变量优先于设置页里存的密钥', t => {
  const home = withHome(t)
  withConfig(t, [{
    id: 'saved-key', kind: 'image', protocol: 'openai', baseUrl: 'https://x.example', model: 'm', env: 'SAVED_KEY',
  }])
  mkdirSync(join(home, '.reel-kit'), { recursive: true })
  writeFileSync(REEL_CONFIG(), JSON.stringify({ channelKeys: { 'saved-key': 'sk-from-settings' } }))
  withEnv(t, 'SAVED_KEY', 'sk-from-env')
  const channel = loadCustomChannels().find(c => c.id === 'saved-key')
  assert.equal(channel.key, 'sk-from-env')
  assert.equal(channel.keySource, 'env')
})

// ── HTTP 接口：桌面端设置页靠这三个动作填通道、查通道、删通道 ──────────
test('桌面端能新增、检查、删掉一条通道', async t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-channels-http-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  withConfig(t, null)
  withEnv(t, 'CH_KEY', 'x')

  const app = createStudio({
    root,
    providers: { status: async () => ({ image: { ready: true } }), generate: async () => { throw new Error('offline') } },
  })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const call = async (method, path, input) => {
    const response = await fetch(base + path, {
      method,
      headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json' },
      ...(input === undefined ? {} : { body: JSON.stringify(input) }),
    })
    return { status: response.status, body: await response.json().catch(() => ({})) }
  }

  const saved = await call('POST', '/api/channels', {
    id: 'my-relay', label: '我的中转', kind: 'image', protocol: 'openai',
    baseUrl: 'http://127.0.0.1:1', model: 'm', env: 'CH_KEY',
  })
  assert.equal(saved.status, 200)
  assert.equal(saved.body.channels.length, 1)
  assert.equal(saved.body.channels[0].key, undefined, '接口不能把密钥吐回界面')

  const listed = await call('GET', '/api/providers')
  assert.equal(listed.body.customChannels.channels[0].id, 'my-relay')
  assert.equal(listed.body.customChannels.channels[0].env, 'CH_KEY')
  assert.equal(listed.body.customChannels.channels[0].ready, true, '密钥到位就算配好了')

  // 地址是本机一个没人监听的端口 —— 探活要报「连不通」，而不是说能用了
  const checked = await call('POST', '/api/channels/check', { id: 'my-relay' })
  assert.equal(checked.status, 200)
  assert.equal(checked.body.ok, false)
  assert.ok(checked.body.checks.some(c => c.name === '连通性' && c.ok === false))

  const removed = await call('DELETE', '/api/channels/my-relay')
  assert.equal(removed.status, 200)
  assert.equal(removed.body.channels.length, 0)
})
