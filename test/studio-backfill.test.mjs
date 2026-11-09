import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStudio } from '../src/studio/server.mjs'

async function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'reel-backfill-test-'))
  const app = createStudio({ root, providers: { status: async () => ({ image: { ready: true } }), generate: async () => { throw new Error('unused') } } })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)); rmSync(root, { recursive: true, force: true }) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const request = async (path, input, headers = {}) => fetch(base + path, { method: input === undefined ? 'GET' : 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json', ...headers }, body: input === undefined ? undefined : JSON.stringify(input) })
  const project = await (await request('/api/projects', { name: 'Backfill test' })).json()
  return { root, base, request, project, store: app.store }
}

/** 造一个「历史素材」：文件在磁盘上，但 state 里没有尺寸/时长 —— 模拟早期入库的数据 */
function legacyAsset(store, projectId, name, mime, bytes) {
  const asset = store.addAsset(projectId, name, mime, bytes, { source: 'upload', origin: 'import', engine: '导入素材' })
  delete asset.width; delete asset.height; delete asset.duration; delete asset.hasAudio
  store.save()
  return asset
}

/** 真造一个 2 秒的 mp4：ffprobe 能读出尺寸与时长，回填才有真东西可验 */
function makeVideo(file) {
  execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=c=blue:s=320x240:d=2', '-f', 'lavfi', '-i', 'anullsrc=r=44100:cl=stereo', '-shortest', '-pix_fmt', 'yuv420p', file], { stdio: 'ignore' })
  return readFileSync(file)
}

test('素材回填：真视频能补出尺寸与时长，图片不再被反复当成待补项', async t => {
  const { request, project, store } = await fixture(t)
  const dir = mkdtempSync(join(tmpdir(), 'reel-backfill-media-'))
  t.after(() => rmSync(dir, { recursive: true, force: true }))
  const videoFile = join(dir, 'clip.mp4')
  const videoBytes = makeVideo(videoFile)
  const videoAsset = legacyAsset(store, project.id, 'clip.mp4', 'video/mp4', videoBytes)

  // 一张真 PNG：尺寸能补出来，但它没有时长 —— 不该每轮都被当成待回填
  const png = join(dir, 'shot.png')
  execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=c=red:s=64x48:d=1', '-frames:v', '1', png], { stdio: 'ignore' })
  const pngAsset = legacyAsset(store, project.id, 'shot.png', 'image/png', readFileSync(png))

  const first = await (await request('/api/assets/backfill', {})).json()
  assert.equal(first.failed.length, 0, '不该有探测失败的项')
  assert.equal(first.updated, 2, '视频与图片各补出缺失字段')

  const state = await (await request('/api/state')).json()
  const video = state.assets.find(a => a.id === videoAsset.id)
  assert.equal(video.width, 320); assert.equal(video.height, 240)
  assert.ok(Math.abs(video.duration - 2) < 0.3, `时长应接近 2 秒，实际 ${video.duration}`)
  assert.equal(video.hasAudio, true)
  const image = state.assets.find(a => a.id === pngAsset.id)
  assert.equal(image.width, 64); assert.equal(image.height, 48)

  // 第二轮：图片不该再被当成待回填（静态图没有时长是正常的）
  const second = await (await request('/api/assets/backfill', {})).json()
  assert.equal(second.total, 0, `第二轮不该还有待补项，实际 ${JSON.stringify(second.failed)}`)
  assert.equal(second.updated, 0)
})

test('素材回填：探测失败要看得见，不能吞掉后说「都补好了」', async t => {
  const { request, project, store } = await fixture(t)
  const asset = legacyAsset(store, project.id, 'broken.mp4', 'video/mp4', Buffer.from('not a real video'))
  const result = await (await request('/api/assets/backfill', {})).json()
  assert.equal(result.updated, 0)
  assert.equal(result.failed.length, 1, '探测失败的素材必须出现在失败明细里')
  assert.equal(result.failed[0].id, asset.id)
  assert.ok(result.failed[0].reason, '失败要有原因')
})

test('改平台阈值后，之前按旧阈值给出的检测结果作废', async t => {
  const { request, project, store } = await fixture(t)
  store.state.jobs.push({
    id: 'job-old', projectId: project.id, kind: 'recommendation', status: 'succeeded',
    input: { kind: 'recommendation', platform: 'channels', assetIds: [] },
    quality: { checks: [{ label: '文件大小', passed: true }] },
    approval: { opening: true },
    createdAt: new Date().toISOString(),
  })
  store.save()
  const saved = await (await request('/api/platform-limits', { platform: 'channels', limits: { maxBytes: 50 * 1024 * 1024, maxDuration: 60 } })).json()
  assert.equal(saved.cleared, 1, '旧检测结果应被作废')
  assert.deepEqual(saved.limits, { maxBytes: 52428800, maxDuration: 60 })
  const state = await (await request('/api/state')).json()
  const job = state.jobs.find(j => j.id === 'job-old')
  assert.equal(job.quality, undefined, '旧检测结果应清掉 —— 阈值变了，旧结论不再成立')
  assert.equal(job.approval, undefined)
})

test('改 default 阈值时，所有平台的旧检测结果都作废', async t => {
  const { request, project, store } = await fixture(t)
  store.state.jobs.push({
    id: 'job-a', projectId: project.id, kind: 'recommendation', status: 'succeeded',
    input: { kind: 'recommendation', platform: 'channels', assetIds: [] }, quality: { checks: [] }, createdAt: new Date().toISOString(),
  })
  store.state.jobs.push({
    id: 'job-b', projectId: project.id, kind: 'recommendation', status: 'succeeded',
    input: { kind: 'recommendation', platform: 'xiaohongshu', assetIds: [] }, quality: { checks: [] }, createdAt: new Date().toISOString(),
  })
  store.save()
  const saved = await (await request('/api/platform-limits', { platform: 'default', limits: { maxBytes: 50 * 1024 * 1024, maxDuration: 60 } })).json()
  assert.equal(saved.cleared, 2)
})

test('平台阈值：拒绝未知平台', async t => {
  const { request } = await fixture(t)
  const response = await request('/api/platform-limits', { platform: 'not-a-platform', limits: { maxBytes: 1024 * 1024, maxDuration: 10 } })
  assert.equal(response.status, 400)
})
