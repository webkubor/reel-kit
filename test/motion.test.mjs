import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { renderFrames, findChrome } from '../src/render.mjs'
import { compose } from '../src/compose.mjs'

// 一张只含图片与字幕的极简模板：动效只需要这两类元素。
const TEMPLATE = `<!doctype html><html><head><meta charset="utf-8"><style>
  body { width: 240px; height: 320px; margin: 0; background: #fff; overflow: hidden; }
  .card { width: 240px; height: 240px; overflow: hidden; }
  .card img { width: 100%; height: 100%; display: block; }
  .caption { font: 16px sans-serif; text-align: center; }
</style></head><body>
  <div class="card"><img src="{{image}}"></div>
  <div class="caption">{{caption}}</div>
</body></html>`

function fixture(t) {
  const dir = mkdtempSync(join(tmpdir(), 'reel-motion-test-'))
  t.after(() => rmSync(dir, { recursive: true, force: true }))
  const template = join(dir, 'template.html')
  writeFileSync(template, TEMPLATE)
  const image = join(dir, 'a.png')
  execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=c=0x3366aa:s=240x240', '-frames:v', '1', '-update', '1', image])
  return { dir, template, image }
}

const probe = (file, entries = 'format=duration') =>
  Number(execFileSync('ffprobe', ['-v', 'error', `-show_entries`, entries, '-of', 'csv=p=0', file], { encoding: 'utf8' }).split('\n')[0])

test('动效模式把每一镜渲染成一段视频，而不是一张静图', async t => {
  if (!findChrome()) { t.skip('本机没有 Chrome（渲染用例需要），跳过'); return }
  const { dir, template, image } = fixture(t)
  const frames = await renderFrames({
    template,
    shots: [{ image, caption: '第一镜' }, { image, caption: '第二镜' }],
    vars: { width: 240, height: 320 },
    outDir: join(dir, 'frames'),
    motion: 'gentle',
    durations: [1, 1],
    fps: 24,
  })
  assert.equal(frames.length, 2)
  for (const frame of frames) assert.match(frame, /\.mp4$/)
  assert.ok(probe(frames[0]) > 0.8, `第一镜时长不对：${probe(frames[0])}`)
})

test('动效模式下画面确实在动：同一镜不同时刻的帧不一样', async t => {
  if (!findChrome()) { t.skip('本机没有 Chrome（渲染用例需要），跳过'); return }
  const { dir, template, image } = fixture(t)
  const frames = await renderFrames({
    template, shots: [{ image, caption: '动' }], vars: { width: 240, height: 320 },
    outDir: join(dir, 'frames'), motion: 'gentle', durations: [1], fps: 24,
  })
  const out = join(dir, 'shot.mp4')
  await compose({ frames, durations: [1], out })
  execFileSync('ffmpeg', ['-v', 'error', '-y', '-i', out, '-vf', 'select=eq(n\\,2)', '-frames:v', '1', join(dir, 'a.png')])
  execFileSync('ffmpeg', ['-v', 'error', '-y', '-i', out, '-vf', 'select=eq(n\\,20)', '-frames:v', '1', join(dir, 'b.png')])
  const psnr = spawnSync('ffmpeg', ['-hide_banner', '-i', join(dir, 'a.png'), '-i', join(dir, 'b.png'), '-lavfi', 'psnr', '-f', 'null', '-'], { encoding: 'utf8' }).stderr
  const match = /average:([0-9.]+)/.exec(psnr)
  assert.ok(match, '没有拿到 PSNR')
  assert.ok(Number(match[1]) < 40, `同一镜首尾画面几乎没变（PSNR ${match[1]}），动效没有生效`)
})

test('转场不吃片长：加了转场，旁白与画面仍然一样长', async t => {
  if (!findChrome()) { t.skip('本机没有 Chrome（渲染用例需要），跳过'); return }
  const { dir, template, image } = fixture(t)
  const durations = [1.2, 1.2]
  const frames = await renderFrames({
    template, shots: [{ image, caption: '一' }, { image, caption: '二' }],
    vars: { width: 240, height: 320 }, outDir: join(dir, 'frames'),
    motion: 'gentle', durations, fps: 24,
  })
  const out = join(dir, 'out.mp4')
  await compose({ frames, durations, transition: 'fade', transitionDuration: 0.4, out })
  const total = durations.reduce((a, b) => a + b, 0)
  assert.ok(Math.abs(probe(out) - total) < 0.15, `片长被转场吃掉了：${probe(out)} ≠ ${total}`)
})

test('没给每镜时长就不给做动效', async t => {
  const { dir, template, image } = fixture(t)
  await assert.rejects(() => renderFrames({
    template, shots: [{ image, caption: '一' }], vars: { width: 240, height: 320 },
    outDir: join(dir, 'frames'), motion: 'gentle',
  }), /时长/)
})

test('静态模式仍然只出静图', async t => {
  if (!findChrome()) { t.skip('本机没有 Chrome（渲染用例需要），跳过'); return }
  const { dir, template, image } = fixture(t)
  const frames = await renderFrames({
    template, shots: [{ image, caption: '一' }], vars: { width: 240, height: 320 },
    outDir: join(dir, 'frames'),
  })
  assert.equal(frames.length, 1)
  assert.match(frames[0], /\.png$/)
})
