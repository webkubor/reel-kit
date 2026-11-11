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

// 带装饰图的模板：条带是模板自带的固定装饰，不是这一镜的内容。
// 以前动效对页面上所有 <img> 生效，装饰条带也会跟着缩放 —— 版式被弄坏，
// 用户还以为是自己的图有问题。
//
// 装饰图必须大到「动一下就看得出」：40px 的小图缩放 4% 只挪 0.8 像素，
// 像素比对根本抓不住，测了等于没测。这里用整条底边的竖条纹。
const STRIPES = 'data:image/svg+xml;utf8,' + encodeURIComponent(
  '<svg xmlns="http://www.w3.org/2000/svg" width="240" height="120">' +
  '<rect width="240" height="120" fill="white"/>' +
  '<rect x="0" width="12" height="120" fill="red"/><rect x="24" width="12" height="120" fill="red"/>' +
  '<rect x="48" width="12" height="120" fill="red"/><rect x="72" width="12" height="120" fill="red"/>' +
  '<rect x="96" width="12" height="120" fill="red"/><rect x="120" width="12" height="120" fill="red"/>' +
  '<rect x="144" width="12" height="120" fill="red"/><rect x="168" width="12" height="120" fill="red"/>' +
  '<rect x="192" width="12" height="120" fill="red"/><rect x="216" width="12" height="120" fill="red"/>' +
  '</svg>')

const TEMPLATE_WITH_LOGO = `<!doctype html><html><head><meta charset="utf-8"><style>
  body { width: 240px; height: 320px; margin: 0; background: #fff; overflow: hidden; }
  .card { width: 240px; height: 200px; overflow: hidden; }
  .card img { width: 100%; height: 100%; display: block; }
  .caption { position: absolute; top: 8px; left: 0; right: 0; font: 16px sans-serif; text-align: center; }
  .stripes { position: absolute; left: 0; bottom: 0; width: 240px; height: 120px; }
</style></head><body>
  <div class="card"><img src="{{image}}"></div>
  <div class="caption">{{caption}}</div>
  <img class="stripes" src="${STRIPES}">
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


// 判据：同一镜首尾，镜头画面要变（动效生效），而 logo 那一角必须一模一样。
// 只用整帧 PSNR 会把「整帧在动」和「连 logo 都在动」混为一谈 —— 必须分区量。
test('动效只作用于这一镜的画面，模板自带的装饰图不动', async t => {
  if (!findChrome()) { t.skip('本机没有 Chrome（渲染用例需要），跳过'); return }
  const dir = mkdtempSync(join(tmpdir(), 'reel-motion-logo-'))
  t.after(() => rmSync(dir, { recursive: true, force: true }))
  const template = join(dir, 'template.html')
  writeFileSync(template, TEMPLATE_WITH_LOGO)
  const image = join(dir, 'a.png')
  execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=c=0x3366aa:s=240x240', '-frames:v', '1', '-update', '1', image])

  const frames = await renderFrames({
    template, shots: [{ image, caption: '动' }], vars: { width: 240, height: 320 },
    outDir: join(dir, 'frames'), motion: 'gentle', durations: [1], fps: 24,
  })
  const out = join(dir, 'shot.mp4')
  await compose({ frames, durations: [1], out })
  execFileSync('ffmpeg', ['-v', 'error', '-y', '-i', out, '-vf', 'select=eq(n\\,2)', '-frames:v', '1', join(dir, 'a.png')])
  execFileSync('ffmpeg', ['-v', 'error', '-y', '-i', out, '-vf', 'select=eq(n\\,20)', '-frames:v', '1', join(dir, 'b.png')])

  const psnrOf = (a, b, crop) => {
    const args = ['-hide_banner', '-i', a, '-i', b]
    // crop 必须排在 psnr 前面：先裁同一块区域，再比这一块。
    // 两条流各自裁完再交给 psnr 对比 —— psnr 要两个输入。
    if (crop) args.push('-lavfi', `[0:v]crop=${crop}[c0];[1:v]crop=${crop}[c1];[c0][c1]psnr`, '-f', 'null', '-')
    else args.push('-lavfi', 'psnr', '-f', 'null', '-')
    const stderr = spawnSync('ffmpeg', args, { encoding: 'utf8' }).stderr
    const match = /average:([0-9.]+|inf)/.exec(stderr)
    assert.ok(match, '没有拿到 PSNR')
    return match[1] === 'inf' ? Infinity : Number(match[1])
  }

  const whole = psnrOf(join(dir, 'a.png'), join(dir, 'b.png'))
  assert.ok(whole < 40, `镜头画面没有变化（PSNR ${whole}），动效没生效`)

  // 底部 120px 是装饰条带所在：它一动，条纹就会平移，整块区域的差异立刻可见。
  const stripes = psnrOf(join(dir, 'a.png'), join(dir, 'b.png'), '240:120:0:200')
  assert.ok(stripes > 45, `模板自带的装饰图跟着动了（条带区域 PSNR ${stripes}）`)
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
