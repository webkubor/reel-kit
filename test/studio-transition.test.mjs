import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStudio } from '../src/studio/server.mjs'
import { SETTING_SCHEMA } from '../src/settings.mjs'

async function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'reel-transition-'))
  const app = createStudio({ root })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)); rmSync(root, { recursive: true, force: true }) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const request = async (path, input) => fetch(base + path, { method: input === undefined ? 'GET' : 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json' }, body: input === undefined ? undefined : JSON.stringify(input) })
  const project = await (await request('/api/projects', { name: '转场白名单' })).json()
  return { base, request, project }
}

// 用「故意缺标题」的提交当探针：转场检查在标题检查之前，
// 所以合法转场会走到标题那一步报错，被拒的转场才报转场错。
// 这样既验证了白名单，又不会真的排队跑配音。
test('界面给的每个转场，服务端都不判非法（选了不能出不了片）', async t => {
  const { request, project } = await fixture(t)
  for (const option of SETTING_SCHEMA.transition.options) {
    const res = await request('/api/jobs', { projectId: project.id, kind: 'talk', captions: '第一句。', transition: option.value })
    const body = await res.json()
    assert.equal(res.status, 400, `转场 ${option.value} 竟然通过了后续校验：${JSON.stringify(body)}`)
    assert.doesNotMatch(body.error, /转场/, `转场 ${option.value} 被服务端判非法：${body.error}`)
  }
})

test('确实不存在的转场仍然被拒', async t => {
  const { request, project } = await fixture(t)
  const res = await request('/api/jobs', { projectId: project.id, kind: 'talk', captions: '第一句。', transition: 'sparkle' })
  assert.equal(res.status, 400)
  assert.match((await res.json()).error, /不支持的转场动效/)
})
