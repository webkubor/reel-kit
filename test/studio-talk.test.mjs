import test from 'node:test'
import assert from 'node:assert/strict'
import { TALK_TEMPLATES, makeTalkVideo } from '../src/studio/talk.mjs'

// 口播流水线里最容易出错的是「用户写了什么」这一层：
// 空脚本、超长句、超过 8 镜、图片塞了视频 —— 这几种都要在开跑之前拦住，
// 而不是渲染到一半失败。
const store = {
  asset(id) {
    if (id === 'audio-1') return { id, mime: 'audio/wav', name: 'bgm.wav' }
    if (id === 'video-1') return { id, mime: 'video/mp4', name: 'clip.mp4' }
    return { id, mime: 'image/png', name: 'shot.png' }
  },
  path() { return '/tmp/does-not-matter' },
  addAsset(projectId, name, mime, bytes, meta) { return { id: 'asset-1', projectId, name, mime, bytes, meta } },
}
const backend = { generate() { throw new Error('不该走到生成配音这一步') } }
const base = { projectId: 'p1', persona: 'demo_narrator', style: 'paper', template: 'knowledge-paper' }

test('口播脚本必须是 1–8 行、每行不超过 80 字', async () => {
  await assert.rejects(() => makeTalkVideo({ ...base, captions: '' }, store, backend), /1–8 行/)
  await assert.rejects(() => makeTalkVideo({ ...base, captions: '一。\n二。\n三。\n四。\n五。\n六。\n七。\n八。\n九。' }, store, backend), /1–8 行/)
  await assert.rejects(() => makeTalkVideo({ ...base, captions: '长'.repeat(81) }, store, backend), /80 字/)
})

test('口播画面素材必须是图片', async () => {
  await assert.rejects(() => makeTalkVideo({ ...base, captions: '一句话。', assetIds: ['video-1'] }, store, backend), /必须是图片/)
})

test('口播模板只开放知识类版式，不含仓库推荐专用版式', () => {
  assert.ok(TALK_TEMPLATES.includes('knowledge-paper'))
  assert.ok(TALK_TEMPLATES.includes('knowledge-dark'))
  assert.ok(!TALK_TEMPLATES.includes('project-recommendation'))
})
