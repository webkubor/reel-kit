import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createStudio } from '../src/studio/server.mjs'

async function fixture(t, options = {}) {
  const root = mkdtempSync(join(tmpdir(), 'reel-cancel-'))
  const app = createStudio({ root, ...options })
  await new Promise(resolve => app.server.listen(0, '127.0.0.1', resolve))
  t.after(async () => { app.server.closeAllConnections(); await new Promise(resolve => app.server.close(resolve)); rmSync(root, { recursive: true, force: true }) })
  const base = `http://127.0.0.1:${app.server.address().port}`
  const request = async (path, input) => fetch(base + path, { method: input === undefined ? 'GET' : 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': 'application/json' }, body: input === undefined ? undefined : JSON.stringify(input) })
  const project = await (await request('/api/projects', { name: '取消任务' })).json()
  const state = async () => (await (await request('/api/state')).json())
  const jobStatus = async id => (await state()).jobs.find(j => j.id === id)?.status
  return { base, request, project, state, jobStatus }
}

// 取消不是「把结果扔掉」：信号要真的传到底层，
// 否则任务还在跑，上游云服务照样按量计费 —— 用户白花钱换一段没人要的片子。
test('取消正在跑的任务：底层收到中止信号，任务标成「已取消」', async t => {
  let markStarted
  const started = new Promise(resolve => { markStarted = resolve })
  let signal = null
  const exportFn = (input, store, progress, jobSignal) => new Promise((resolve, reject) => {
    signal = jobSignal
    markStarted()
    jobSignal.addEventListener('abort', () => reject(Object.assign(new Error('已取消'), { cancelled: true })))
  })
  const { request, project, jobStatus } = await fixture(t, { exportFn })

  const created = await (await request('/api/jobs', { projectId: project.id, kind: 'export', ratio: '9:16' })).json()
  assert.equal(created.status, 'queued')
  await started

  const res = await request(`/api/jobs/${created.id}/cancel`, {})
  assert.equal(res.status, 200)
  assert.equal((await res.json()).status, 'canceling')

  assert.ok(signal, '底层没有拿到取消信号')
  assert.equal(signal.aborted, true, '信号没有被中止')

  for (let i = 0; i < 50 && await jobStatus(created.id) !== 'canceled'; i++) await new Promise(r => setTimeout(r, 20))
  assert.equal(await jobStatus(created.id), 'canceled')
})

test('取消排队中的任务：它不会再被执行', async t => {
  const ran = []
  let markStarted
  const started = new Promise(resolve => { markStarted = resolve })
  const exportFn = (input, store, progress, signal) => {
    ran.push(input.prompt)
    if (input.prompt !== 'blocker') return Promise.resolve({})
    markStarted()
    // 一直挂着，直到被取消 —— 队列被它占住，后一个任务才会排队。
    return new Promise((resolve, reject) => {
      signal.addEventListener('abort', () => reject(Object.assign(new Error('已取消'), { cancelled: true })))
    })
  }
  const { request, project, jobStatus } = await fixture(t, { exportFn })

  // 先占住队列：有一个任务卡在跑，第二个才会真的排队。
  const blocker = await (await request('/api/jobs', { projectId: project.id, kind: 'export', ratio: '9:16', prompt: 'blocker' })).json()
  await started
  const queued = await (await request('/api/jobs', { projectId: project.id, kind: 'export', ratio: '9:16', prompt: 'queued' })).json()
  assert.equal(await jobStatus(queued.id), 'queued')

  const res = await request(`/api/jobs/${queued.id}/cancel`, {})
  assert.equal(res.status, 200)
  assert.equal((await res.json()).status, 'canceled')
  assert.equal(await jobStatus(queued.id), 'canceled')

  await new Promise(r => setTimeout(r, 200))
  assert.ok(!ran.includes('queued'), '被取消的排队任务还是跑起来了')
  assert.equal(await jobStatus(blocker.id), 'running')

  // 收尾：把占位的任务也停掉，别让它挂着把测试进程拖住。
  await request(`/api/jobs/${blocker.id}/cancel`, {})
  for (let i = 0; i < 50 && await jobStatus(blocker.id) !== 'canceled'; i++) await new Promise(r => setTimeout(r, 20))
})

test('取消不存在的任务 / 已结束的任务，接口要给出明确答复', async t => {
  const { request, project } = await fixture(t)

  const missing = await request('/api/jobs/00000000-0000-0000-0000-000000000000/cancel', {})
  assert.equal(missing.status, 404)
  assert.match((await missing.json()).error, /找不到/)

  const created = await (await request('/api/jobs', { projectId: project.id, kind: 'export', ratio: '9:16' })).json()
  // 把它跑完（默认导出在缺素材时会失败，但状态会落到 failed）
  for (let i = 0; i < 50; i++) {
    const state = await (await request('/api/state')).json()
    const job = state.jobs.find(j => j.id === created.id)
    if (job.status !== 'queued' && job.status !== 'running') {
      const res = await request(`/api/jobs/${created.id}/cancel`, {})
      assert.equal(res.status, 400)
      assert.match((await res.json()).error, /已经结束/)
      return
    }
    await new Promise(r => setTimeout(r, 20))
  }
  assert.fail('任务没有结束，测试没验到')
})
