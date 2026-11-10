import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStore } from '../src/studio/store.mjs'
import { createMagpie, discoverMagpie, parseCatalog, parseReceipt, sizeFor } from '../src/studio/magpie.mjs'
import { createProviders } from '../src/studio/providers.mjs'

const CATALOG_TEXT = 'generate_image: "x/nope" isn\'t an image model that can be used here. Pick one of: codex/gpt-image-2.5, openrouter/google/gemini-nano-banana-2.1, openrouter/openai/gpt-image-2.6-sunburst, codex/gpt-image-2; or leave model out to use the default (codex/gpt-image-2.5)'

const IMAGE_DOCS = `generate_image  (image, /mcp/magpie/image)
  --model <text>
      The image model to use, as provider/model. Leave it out to use the one set in Magpie (now codex/gpt-image-2.5). One of the 67 image models set up in Magpie, such as codex/gpt-image-2.5; an unknown one is refused with the full list.`

const VIDEO_DOCS = `generate_video  (image, /mcp/magpie/image)
  --model <text>
      The video model to use, as provider/model. Leave it out: none is set up in Magpie.`

/** 假 magpie CLI：不联网、不生成、只回固定回执 */
function fakeRun({ videoGen = '', refuse = true, image = null } = {}) {
  const calls = []
  const run = async (args) => {
    calls.push(args.join(' '))
    if (args[0] === '--version') return { code: 0, stdout: 'magpie 0.1.0', stderr: '' }
    if (args[0] === 'tools' && args[1] === 'generate_image') return { code: 0, stdout: IMAGE_DOCS, stderr: '' }
    if (args[0] === 'tools' && args[1] === 'generate_video') return { code: 0, stdout: VIDEO_DOCS, stderr: '' }
    if (args[0] === 'settings' && args[1] === 'get' && args[2] === 'videoGen') return { code: 0, stdout: JSON.stringify(videoGen), stderr: '' }
    if (args[0] === 'tool' && args[1] === 'generate_image') {
      if (refuse) return { code: 1, stdout: JSON.stringify({ content: [{ type: 'text', text: CATALOG_TEXT }], isError: true }), stderr: 'magpie: generate_image answered with an error' }
      return { code: 0, stdout: JSON.stringify(image || { content: [{ type: 'text', text: 'done' }] }), stderr: '' }
    }
    return { code: 0, stdout: JSON.stringify({ content: [{ type: 'text', text: 'ok' }] }), stderr: '' }
  }
  return { run, calls }
}

const storeFixture = t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-magpie-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const store = createStore(root)
  store.state.projects.push({ id: 'p1', name: 'Test', createdAt: new Date().toISOString() })
  store.save()
  return store
}

test('图片清单来自 Magpie 自己的拒绝回执，默认模型也一起拿到', async () => {
  const { run } = fakeRun()
  const status = await discoverMagpie(run)
  assert.equal(status.installed, true)
  assert.equal(status.image.ready, true)
  assert.deepEqual(status.image.models.map(m => m.id), ['codex/gpt-image-2.5', 'openrouter/google/gemini-nano-banana-2.1', 'openrouter/openai/gpt-image-2.6-sunburst', 'codex/gpt-image-2'])
  assert.equal(status.image.defaultModel, 'codex/gpt-image-2.5')
})

test('解析器只认 provider/model 形状，不会把半句话当模型', () => {
  const parsed = parseCatalog(CATALOG_TEXT)
  assert.equal(parsed.models.length, 4)
  assert.equal(parsed.defaultModel, 'codex/gpt-image-2.5')
  assert.deepEqual(parseCatalog('nothing here').models, [])
})

test('Magpie 没配视频模型时，视频通道不可用且不靠生成试探', async () => {
  const { run, calls } = fakeRun({ videoGen: '' })
  const status = await discoverMagpie(run)
  assert.equal(status.video.ready, false)
  assert.match(status.video.note, /未配置视频模型/)
  assert.equal(calls.some(c => c.includes('generate_video') && !c.includes('tools')), false, '不能拿真实生成去试视频通道')
})

test('文档不再保证「未知模型回出完整清单」时，不发探测请求', async () => {
  const calls = []
  const run = async args => {
    calls.push(args.join(' '))
    if (args[0] === '--version') return { code: 0, stdout: 'magpie 0.1.0', stderr: '' }
    if (args[0] === 'tools') return { code: 0, stdout: '--model <text> 用什么模型都行', stderr: '' }
    if (args[0] === 'settings') return { code: 0, stdout: '""', stderr: '' }
    return { code: 0, stdout: '{}', stderr: '' }
  }
  const status = await discoverMagpie(run)
  assert.equal(status.image.ready, false)
  assert.equal(calls.some(c => c.includes('tool generate_image')), false, '没有文档保证就不能发探测')
})

test('回执解析能取出内联图片、结果地址和任务 ID', () => {
  const receipt = parseReceipt({
    content: [{ type: 'text', text: '生成中，id 8a1f0d2e-1111-2222-3333-444455556666' }],
    images: [{ mimeType: 'image/png', data: Buffer.from('fake-png-bytes').toString('base64') }],
    url: 'https://example.com/a.png',
  })
  assert.equal(receipt.images.length, 1)
  assert.ok(receipt.urls.includes('https://example.com/a.png'))
  assert.ok(receipt.ids.includes('8a1f0d2e-1111-2222-3333-444455556666'))
})

test('画幅映射：图片给像素，视频给比例', () => {
  assert.equal(sizeFor('image', '9:16'), '1024x1536')
  assert.equal(sizeFor('image', '16:9'), '1536x1024')
  assert.equal(sizeFor('video', '9:16'), '9:16')
  assert.equal(sizeFor('video', '4:3'), '9:16')
})

test('生成成功时：回执先落任务，再下载素材', async t => {
  const store = storeFixture(t)
  const calls = []
  const run = async args => {
    calls.push(args.join(' '))
    if (args[0] === 'tool' && args[1] === 'generate_image') {
      return { code: 0, stdout: JSON.stringify({ content: [{ type: 'text', text: 'id task-abc' }], url: 'https://example.com/x.png' }), stderr: '' }
    }
    return { code: 0, stdout: '{}', stderr: '' }
  }
  let downloaded = 0
  const magpie = createMagpie({
    run,
    store,
    download: async () => { downloaded++; return { bytes: Buffer.from('png-bytes'), mime: 'image/png' } },
    probe: async () => ({ width: 1024 }),
  })
  const progressLog = []
  const result = await magpie.generateImage({
    input: { prompt: '一只猫', model: 'codex/gpt-image-2.5', ratio: '9:16' },
    progress: (text, meta) => progressLog.push({ text, meta }),
  })
  assert.equal(downloaded, 1)
  assert.equal(result.taskId, 'task-abc')
  const receipt = progressLog.find(p => p.meta?.upstream?.taskId)
  assert.ok(receipt, '上游回执必须在下载前写进任务')
  assert.equal(receipt.meta.upstream.channel, 'magpie')
  assert.equal(receipt.meta.upstream.url, 'https://example.com/x.png')
  assert.ok(calls.some(c => c.includes('--model codex/gpt-image-2.5')))
  assert.ok(calls.some(c => c.includes('--size 1024x1536')))
})

test('下载失败后可以取回上游产物，且不会重新生成', async t => {
  const store = storeFixture(t)
  const calls = []
  const run = async args => {
    calls.push(args.join(' '))
    if (args[0] === 'tool' && args[1] === 'generate_image') {
      return { code: 0, stdout: JSON.stringify({ content: [{ type: 'text', text: 'id task-abc' }], url: 'https://example.com/x.png' }), stderr: '' }
    }
    if (args[0] === 'tool' && args[1] === 'generation_result') {
      return { code: 0, stdout: JSON.stringify({ content: [{ type: 'text', text: '好了' }], url: 'https://example.com/x.png' }), stderr: '' }
    }
    return { code: 0, stdout: '{}', stderr: '' }
  }
  const magpie = createMagpie({
    run,
    store,
    download: async () => { throw new Error('下载超时') },
    probe: async () => ({}),
  })
  await assert.rejects(() => magpie.generateImage({ input: { prompt: '猫' }, progress: () => {} }), /下载超时/)
  const submitsBefore = calls.filter(c => c.includes('generate_image')).length
  assert.equal(submitsBefore, 1)

  // 取回：同一个任务 ID，不重新提交
  let downloaded = 0
  const magpie2 = createMagpie({
    run,
    store,
    download: async () => { downloaded++; return { bytes: Buffer.from('png-bytes'), mime: 'image/png' } },
    probe: async () => ({}),
  })
  const asset = await magpie2.resume({
    upstream: { channel: 'magpie', kind: 'image', taskId: 'task-abc', model: 'codex/gpt-image-2.5' },
    input: { projectId: 'p1', prompt: '猫' },
    progress: () => {},
  })
  assert.ok(asset?.id)
  assert.equal(downloaded, 1)
  assert.equal(calls.filter(c => c.includes('generate_image')).length, 1, '取回不能变成第二次生成')
  assert.equal(asset.actualModel, 'unknown', '回执里没有实际模型名，不能编')
  assert.equal(asset.requestedModel, 'codex/gpt-image-2.5')
})

test('生成被 Magpie 拒绝时，把原因原样抛出来', async t => {
  const store = storeFixture(t)
  const run = async args => {
    if (args[0] === 'tool') return { code: 1, stdout: JSON.stringify({ content: [{ type: 'text', text: 'generate_image: "x/y" isn\'t an image model' }], isError: true }), stderr: '' }
    return { code: 0, stdout: '{}', stderr: '' }
  }
  const magpie = createMagpie({ run, store, download: async () => ({ bytes: Buffer.alloc(0) }), probe: async () => ({}) })
  await assert.rejects(() => magpie.generateImage({ input: { prompt: '猫', model: 'x/y' }, progress: () => {} }), /isn't an image model/)
})

test('状态接口把 Magpie 挂到图片与视频通道上，模型清单不写死', async t => {
  const store = storeFixture(t)
  const { run } = fakeRun()
  const providers = createProviders(store, { magpieRun: run, fetcher: async () => { throw new Error('offline') } })
  const status = await providers.status()
  const imageChannel = status.image.channels.find(c => c.id === 'magpie')
  const videoChannel = status.video.channels.find(c => c.id === 'magpie')
  assert.ok(imageChannel, '图片通道里要有 Magpie')
  assert.ok(videoChannel, '视频通道里要有 Magpie')
  assert.equal(imageChannel.ready, true)
  assert.equal(videoChannel.ready, false)
  assert.ok(imageChannel.models.some(m => m.id === 'openrouter/openai/gpt-image-2.6-sunburst'))
})

test('走 Magpie 通道生成图片：只提交一次，失败后凭上游 ID 取回', async t => {
  const store = storeFixture(t)
  const calls = []
  const run = async args => {
    calls.push(args.join(' '))
    if (args[0] === 'tool' && args[1] === 'generate_image') {
      return { code: 0, stdout: JSON.stringify({ content: [{ type: 'text', text: 'id task-9' }], url: 'https://example.com/y.png' }), stderr: '' }
    }
    if (args[0] === 'tool' && args[1] === 'generation_result') {
      return { code: 0, stdout: JSON.stringify({ content: [{ type: 'text', text: '好了' }], url: 'https://example.com/y.png' }), stderr: '' }
    }
    return { code: 0, stdout: '{}', stderr: '' }
  }
  let downloads = 0
  const providers = createProviders(store, {
    magpieRun: run,
    fetcher: async () => { throw new Error('offline') },
    download: async () => { downloads++; if (downloads === 1) throw new Error('下载超时'); return { bytes: Buffer.from('png'), mime: 'image/png' } },
  })
  const upstreamSeen = []
  await assert.rejects(() => providers.generate(
    { kind: 'image', backend: 'magpie', projectId: 'p1', prompt: '猫', model: 'codex/gpt-image-2.5', ratio: '9:16' },
    (text, meta) => { if (meta?.upstream) upstreamSeen.push(meta.upstream) },
  ), /下载超时/)
  assert.equal(upstreamSeen.length, 1, '上游回执要落到任务上')
  assert.equal(upstreamSeen[0].taskId, 'task-9')

  const asset = await providers.resume({
    upstream: upstreamSeen[0],
    input: { kind: 'image', backend: 'magpie', projectId: 'p1', prompt: '猫' },
  }, () => {})
  assert.ok(asset?.id)
  assert.equal(calls.filter(c => c.includes('tool generate_image')).length, 1, '取回不能变成第二次提交')
})
