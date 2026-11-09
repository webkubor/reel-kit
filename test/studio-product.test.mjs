import test from 'node:test'
import assert from 'node:assert/strict'
import { PRODUCT_LAYOUTS, makeProductVideo } from '../src/studio/product.mjs'

// 人物商品介绍最容易出错的地方和口播一样，但多一个「画幅」维度：
// 画幅一变，版式和尺寸都要跟着换。写错的话片子会按错的尺寸渲染出来。
const store = {
  asset(id) {
    if (id === 'video-1') return { id, mime: 'video/mp4', name: 'clip.mp4' }
    return { id, mime: 'image/png', name: 'product.png' }
  },
  path() { return '/tmp/does-not-matter' },
  addAsset(projectId, name, mime, bytes, meta) { return { id: 'asset-1', projectId, name, mime, bytes, meta } },
}
const backend = { generate() { throw new Error('不该走到生成配音这一步') } }
const base = { projectId: 'p1', persona: 'demo_narrator', title: 'Reel Studio' }

test('介绍词必须是 1–8 行、每行不超过 80 字', async () => {
  await assert.rejects(() => makeProductVideo({ ...base, captions: '' }, store, backend), /1–8 行/)
  await assert.rejects(() => makeProductVideo({ ...base, captions: '一。\n二。\n三。\n四。\n五。\n六。\n七。\n八。\n九。' }, store, backend), /1–8 行/)
  await assert.rejects(() => makeProductVideo({ ...base, captions: '长'.repeat(81) }, store, backend), /80 字/)
})

test('产品或人物画面必须是图片', async () => {
  await assert.rejects(() => makeProductVideo({ ...base, captions: '一句话。', assetIds: ['video-1'] }, store, backend), /必须是图片/)
})

test('画幅决定版式与尺寸', () => {
  assert.equal(PRODUCT_LAYOUTS['9:16'].template, 'product-presenter')
  assert.equal(PRODUCT_LAYOUTS['9:16'].width, 1080)
  assert.equal(PRODUCT_LAYOUTS['9:16'].height, 1920)
  assert.equal(PRODUCT_LAYOUTS['16:9'].template, 'landscape-product')
  assert.equal(PRODUCT_LAYOUTS['16:9'].width, 1920)
  assert.equal(PRODUCT_LAYOUTS['16:9'].height, 1080)
})
