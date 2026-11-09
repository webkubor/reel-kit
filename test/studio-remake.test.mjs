import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStore } from '../src/studio/store.mjs'
import { initializeWorkflow, remakeProject } from '../src/studio/workflow.mjs'
import { limitsFor, normalizeLimits, saveLimits } from '../src/studio/platform-limits.mjs'

function storeWithProject() {
  const root = mkdtempSync(join(tmpdir(), 'reel-remake-'))
  const store = createStore(root)
  initializeWorkflow(store.state)
  const project = { id: 'p1', name: 'Bloom · 项目推荐', pipelineKey: 'github-recommend', createdAt: new Date().toISOString() }
  store.state.projects.push(project)
  const image = store.addAsset(project.id, 'shot.png', 'image/png', Buffer.from('fake-png'), { source: 'github' })
  image.width = 1200; image.height = 800
  const voice = store.addAsset(project.id, 'line1.wav', 'audio/wav', Buffer.from('fake-wav'), { source: 'tts', persona: 'p', prompt: '第一句' })
  return { store, project, image, voice }
}

test('照这个方法再做一条：新项目继承方法与素材，但不继承内容', () => {
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
  // 素材被复制到新项目，原项目不受影响
  const copies = store.state.assets.filter(a => a.projectId === created.id)
  assert.equal(copies.length, 2)
  assert.ok(copies.every(c => c.id !== image.id))
  assert.equal(result.imageIds.length, 1)
  assert.equal(result.voiceIds.length, 1)
  assert.equal(store.state.assets.filter(a => a.projectId === project.id).length, 2)
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

test('历史素材回填目标判定：视觉素材缺尺寸才回填', () => {
  const visual = { mime: 'image/png' }
  const audio = { mime: 'audio/wav', duration: 2, hasAudio: true }
  const need = a => (a.mime?.startsWith('image/') || a.mime?.startsWith('video/')) && (!a.width || !a.height) || a.duration === undefined || a.hasAudio === undefined
  assert.equal(need(visual), true)
  assert.equal(need(audio), false)
})
