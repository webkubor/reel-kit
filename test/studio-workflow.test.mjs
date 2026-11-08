import test from 'node:test'
import assert from 'node:assert/strict'
import { initializeWorkflow, snapshotSop, workflowAction } from '../src/studio/workflow.mjs'

test('review gates, publication metrics and immutable SOP lineage', () => {
  const project = { id: 'p' }
  const state = { jobs: [], projects: [project] }
  initializeWorkflow(state)
  const store = { state, save() {}, project(id) { if (id !== project.id) throw Error('missing'); return project } }
  const input = { kind: 'recommendation', persona: 'voice', tone: 'natural', platform: 'channels' }
  const job = { id: 'j', projectId: 'p', input, status: 'succeeded', assetId: 'asset', sopSnapshot: snapshotSop(state, project, input) }
  state.jobs.push(job)
  input.tone = 'changed later'
  assert.equal(job.sopSnapshot.input.tone, 'natural')
  const common = { projectId: 'p', jobId: 'j' }
  assert.throws(() => workflowAction(store, 'publish', common), /检查/)
  job.quality = { passed: true, assetId: 'asset' }
  assert.throws(() => workflowAction(store, 'review-check', { ...common, checks: {} }), /逐项/)
  const checks = Object.fromEntries(['opening','truth','matching','ending','subtitles','audio','visuals'].map(k => [k, true]))
  workflowAction(store, 'review-check', { ...common, checks })
  const pub = workflowAction(store, 'publish', { ...common, platform: '小红书', url: 'https://example.com/video', publishedAt: '2026-11-08', goal: '访问' })
  assert.equal(pub.sopVersionId, job.sopSnapshot.sopVersionId)
  assert.throws(() => workflowAction(store, 'performance', { ...common, publishId: pub.id, window: '7天', plays: -1 }), /非负/)
  const metric = workflowAction(store, 'performance', { ...common, publishId: pub.id, window: '7天', plays: '0', likes: '' })
  assert.deepEqual(metric.metrics, { plays: 0 })
  const result = workflowAction(store, 'review', { ...common, publishId: pub.id, findings: '开头慢', hypothesis: '先展示价值', changes: '改开头' })
  assert.equal(result.version.version, 2)
  assert.equal(result.version.parentId, job.sopSnapshot.sopVersionId)
  assert.equal(job.sopSnapshot.sopVersion, 1)
  assert.equal(result.version.settings.tone, 'changed later')
  project.sopVersionId = result.version.id
  const nextInput = { ...input, tone: 'applied version' }
  const next = snapshotSop(state, project, nextInput)
  assert.equal(next.sopVersion, 2)
  assert.equal(next.sopVersionId, result.version.id)
  nextInput.tone = 'modified after queueing'
  assert.equal(next.input.tone, 'applied version')
  assert.equal(job.sopSnapshot.sopVersion, 1)
  assert.throws(() => workflowAction(store, 'performance', { ...common, publishId: 'other-project', window: '7天', plays: 2 }), /发布记录/)
})
