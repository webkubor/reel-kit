import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStore } from '../src/studio/store.mjs'
import { initializeWorkflow, remakeProject } from '../src/studio/workflow.mjs'
import { limitsFor, normalizeLimits, saveLimits, limitPlatforms } from '../src/studio/platform-limits.mjs'

function storeWithProject() {
  const root = mkdtempSync(join(tmpdir(), 'reel-remake-'))
  const store = createStore(root)
  initializeWorkflow(store.state)
  const project = { id: 'p1', name: 'Bloom · 项目推荐', pipelineKey: 'github-recommend', createdAt: new Date().toISOString() }
  store.state.projects.push(project)
  const image = store.addAsset(project.id, 'shot.png', 'image/png', Buffer.from('fake-png'), { source: 'github' })
  image.width = 1200; image.height = 800
  const voice = store.addAsset(project.id, 'line1.wav', 'audio/wav', Buffer.from('fake-wav'), { source: 'tts', persona: 'p', prompt: '第一句' })
  const bgm = store.addAsset(project.id, 'bgm.mp3', 'audio/mpeg', Buffer.from('fake-mp3'), { source: '配乐库', license: 'Mixkit Free' })
  return { store, project, image, voice, bgm }
}

test('照这个方法再做一条：新项目继承方法与素材，内容带入上一条作为起点', () => {
  const { store, project, image, voice } = storeWithProject()
  const job = { id: 'j1', projectId: project.id, status: 'succeeded', sopSnapshot: { sopVersionId: store.state.sopVersions[0].id, sopVersion: 1, input: {} }, input: { kind: 'recommendation', title: 'Bloom 介绍', repoUrl: 'https://github.com/a/b', captions: '第一句\n第二句', assetIds: [image.id], voiceIds: [voice.id], platform: 'channels', voiceSource: 'tts', persona: 'p', tone: '自然', speed: 1, pause: 0.35, imageFit: 'contain' } }
  store.state.jobs.push(job)
  const result = remakeProject(store, job)
  const created = store.state.projects.find(p => p.id === result.projectId)
  assert.ok(created, '新项目已创建')
  assert.notEqual(created.id, project.id)
  assert.equal(created.sopVersionId, store.state.sopVersions[0].id)
  assert.equal(created.recommendationSettings.platform, 'channels')
  assert.equal(result.draft.url, 'https://github.com/a/b')
  assert.equal(result.draft.captions, '第一句\n第二句', '文案带入作为编辑起点')
  // 素材被复制到新项目，原项目不受影响
  const copies = store.state.assets.filter(a => a.projectId === created.id)
  assert.equal(copies.length, 2)
  assert.ok(copies.every(c => c.id !== image.id))
  assert.equal(result.imageIds.length, 1)
  assert.equal(result.voiceIds.length, 1)
  assert.equal(store.state.assets.filter(a => a.projectId === project.id).length, 3)
})

test('照这个方法再做一条：BGM 与整段旁白指向新项目里的副本', () => {
  const { store, project, image, voice, bgm } = storeWithProject()
  const job = { id: 'j1', projectId: project.id, status: 'succeeded', sopSnapshot: { sopVersionId: store.state.sopVersions[0].id, sopVersion: 1, input: {} }, input: { kind: 'recommendation', title: 'T', assetIds: [image.id], voiceIds: [voice.id], bgmId: bgm.id, narrationId: voice.id, platform: 'channels' } }
  store.state.jobs.push(job)
  const result = remakeProject(store, job)
  const created = store.state.projects.find(p => p.id === result.projectId)
  // 引用必须指向新项目里的副本，否则新项目引用的是别的项目的素材
  assert.ok(result.settings.bgmId, 'BGM 引用已映射')
  assert.notEqual(result.settings.bgmId, bgm.id)
  assert.equal(store.state.assets.find(a => a.id === result.settings.bgmId).projectId, created.id)
  assert.ok(result.settings.narrationId)
  assert.notEqual(result.settings.narrationId, voice.id)
})

test('照这个方法再做一条：复制素材时带上音色复用需要的元数据', () => {
  const { store, project } = storeWithProject()
  const voice = store.state.assets.find(a => a.name === 'line1.wav')
  voice.tone = '自然'
  const job = { id: 'j1', projectId: project.id, status: 'succeeded', sopSnapshot: { sopVersionId: store.state.sopVersions[0].id, sopVersion: 1, input: {} }, input: { kind: 'recommendation', title: 'T', assetIds: [], voiceIds: [voice.id], tone: '自然' } }
  store.state.jobs.push(job)
  const result = remakeProject(store, job)
  const copied = store.state.assets.find(a => a.id === result.voiceIds[0])
  assert.equal(copied.tone, '自然', 'tone 要跟着复制，否则这条音色无法被判定为可复用')
  assert.equal(copied.persona, 'p')
})

test('照这个方法再做一条：中途失败不留半成品项目', () => {
  const { store, project, image, voice } = storeWithProject()
  const job = { id: 'j1', projectId: project.id, status: 'succeeded', sopSnapshot: { sopVersionId: store.state.sopVersions[0].id, sopVersion: 1, input: {} }, input: { kind: 'recommendation', title: 'T', assetIds: [image.id], voiceIds: [voice.id], platform: 'channels' } }
  store.state.jobs.push(job)
  const projectsBefore = store.state.projects.length
  const assetsBefore = store.state.assets.length
  // 制造失败：第二个素材写入时抛错（磁盘满之类）
  let calls = 0
  const original = store.addAsset.bind(store)
  store.addAsset = (...args) => { calls++; if (calls > 1) throw new Error('磁盘满了'); return original(...args) }
  assert.throws(() => remakeProject(store, job), /磁盘满了/)
  store.addAsset = original
  assert.equal(store.state.projects.length, projectsBefore, '失败后不应留下新项目')
  assert.equal(store.state.assets.length, assetsBefore, '失败后不应留下已复制的素材')
})

test('照这个方法再做一条：原视频记录与发布数据不受影响', () => {
  const { store, project } = storeWithProject()
  const job = { id: 'j1', projectId: project.id, status: 'succeeded', sopSnapshot: { sopVersionId: store.state.sopVersions[0].id, sopVersion: 1, input: {} }, input: { kind: 'recommendation', title: 'T', assetIds: [], voiceIds: [] } }
  store.state.jobs.push(job)
  store.state.publishes.push({ id: 'p1', projectId: project.id, jobId: job.id, platform: '小红书', publishedAt: '2026-11-08' })
  remakeProject(store, job)
  assert.equal(store.state.publishes.length, 1)
  assert.equal(store.state.publishes[0].jobId, job.id)
  assert.equal(store.state.jobs.length, 1)
})

test('平台阈值：可配置、越界被拒、按平台取值', () => {
  const state = {}
  assert.deepEqual(limitsFor(state, 'channels'), { maxBytes: 104857600, maxDuration: 180 })
  saveLimits(state, 'channels', { maxBytes: 50 * 1024 * 1024, maxDuration: 60 })
  assert.deepEqual(limitsFor(state, 'channels'), { maxBytes: 52428800, maxDuration: 60 })
  assert.deepEqual(limitsFor(state, 'xiaohongshu'), { maxBytes: 104857600, maxDuration: 180 })
  assert.throws(() => normalizeLimits({ maxBytes: 10 }), /文件大小上限/)
  assert.throws(() => normalizeLimits({ maxDuration: 0 }), /时长上限/)
})

test('平台阈值：只接受已知平台', () => {
  const state = {}
  for (const platform of limitPlatforms) saveLimits(state, platform, { maxBytes: 10 * 1024 * 1024, maxDuration: 30 })
  assert.throws(() => saveLimits(state, 'not-a-platform', { maxBytes: 10 * 1024 * 1024, maxDuration: 30 }), /不支持的平台/)
})
