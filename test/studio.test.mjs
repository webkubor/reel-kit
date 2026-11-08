import test from 'node:test'
import assert from 'node:assert/strict'
import http from 'node:http'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStudio } from '../src/studio/server.mjs'
import { createStore } from '../src/studio/store.mjs'
import { h3Body } from '../src/studio/providers.mjs'

async function fixture(t, generate = async () => { throw new Error('provider offline') }) {
  const root = mkdtempSync(join(tmpdir(), 'reel-studio-test-'))
  const app = createStudio({ root, providers: { status: async () => ({ image: { ready: true } }), generate } })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)); rmSync(root, { recursive: true, force: true }) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const request = async (path, input, headers = {}) => fetch(base + path, { method: input === undefined ? 'GET' : 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json', ...headers }, body: input === undefined ? undefined : JSON.stringify(input) })
  const project = await (await request('/api/projects', { name: 'Test project' })).json()
  return { ...app, root, base, request, project }
}

test('project asset isolation, downloads and video range requests', async t => {
  const { request, base, project } = await fixture(t)
  const upload = await fetch(`${base}/api/assets?projectId=${project.id}&name=video.mp4`, { method: 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'video/mp4' }, body: Buffer.from('test-video-bytes') })
  assert.equal(upload.status, 201)
  const asset = await upload.json()
  const partial = await fetch(base + asset.url, { headers: { Range: 'bytes=5-9' } })
  assert.equal(partial.status, 206); assert.equal(await partial.text(), 'video')
  const other = await (await request('/api/projects', { name: 'Other' })).json()
  const rejected = await request('/api/jobs', { kind: 'export', projectId: other.id, assetIds: [asset.id] })
  assert.equal(rejected.status, 404)
  const state = await (await request('/api/state')).json()
  assert.equal(state.assets[0].file, undefined)
})

test('blocks cross-site writes, DNS rebinding and unmarked requests', async t => {
  const { base } = await fixture(t)
  for (const headers of [{ Origin: 'https://evil.example', 'X-Reel-Action': 'studio' }, { Host: 'evil.example:4390', 'X-Reel-Action': 'studio' }, {}]) {
    const status = await new Promise((resolve, reject) => {
      const request = http.request(base + '/api/projects', { method: 'POST', headers }, response => { response.resume(); resolve(response.statusCode) })
      request.on('error', reject); request.end('{"name":"bad"}')
    })
    assert.equal(status, 403, JSON.stringify(headers))
  }
})

test('failed generation keeps input and does not expose credentials', async t => {
  const { request, project } = await fixture(t, async () => { throw new Error('Bearer sk-private-key-leak-me https://secret.example/token') })
  const response = await request('/api/jobs', { projectId: project.id, kind: 'image', prompt: 'original prompt' })
  assert.equal(response.status, 202)
  const job = await response.json()
  // HTTP requests yield to the server worker; no remote service involved.
  const state = await (await request('/api/state')).json()
  const done = state.jobs.find(j => j.id === job.id)
  assert.equal(done.status, 'failed'); assert.equal(done.input.prompt, 'original prompt')
  assert.ok(!done.error.includes('sk-private')); assert.ok(!done.error.includes('secret.example'))
})

test('restarts mark in-flight jobs interrupted without paid resubmission', t => {
  const root = mkdtempSync(join(tmpdir(), 'reel-studio-restart-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const store = createStore(root)
  store.state.jobs.push({ status: 'running', input: { prompt: 'keep this' } }); store.save()
  const restarted = createStore(root)
  assert.equal(restarted.state.jobs[0].status, 'interrupted')
  assert.equal(restarted.state.jobs[0].input.prompt, 'keep this')
})

test('H3 direct request uses V2 content reference contract and fixed model', () => {
  const body = h3Body({ prompt: 'walk', duration: 6, ratio: '9:16', resolution: '768P' }, 'https://fixture.example/ref.png')
  assert.equal(body.model, 'MiniMax-H3')
  assert.deepEqual(body.content[1], { type: 'image_url', image_url: { url: 'https://fixture.example/ref.png' }, role: 'reference_image' })
  assert.equal(body.ratio, '9:16'); assert.equal(body.duration, 6)
})

test('rejects fractional duration and path injection before submitting paid job', async t => {
  const { request, project } = await fixture(t)
  for (const input of [{ kind: 'video', duration: 6.6 }, { kind: 'export', template: '../../secrets' }]) {
    const response = await request('/api/jobs', { projectId: project.id, prompt: 'prompt', ...input })
    assert.equal(response.status, 400)
  }
  const state = await (await request('/api/state')).json()
  assert.equal(state.jobs.length, 0)
})

test('recording does not create an unusable global voice without reference text support', async t => {
  const { base, request } = await fixture(t)
  const response = await fetch(`${base}/api/voice/record`, { method: 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'audio/wav' }, body: Buffer.from('test recording') })
  assert.equal(response.status, 503)
  assert.match((await response.json()).error, /参考文本/)
  assert.deepEqual((await (await request('/api/state')).json()).voices, [])
})
