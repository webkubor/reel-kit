import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { fill, frameVars, readTemplateSize } from '../src/render.mjs'

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)))
const templatesDir = join(repoRoot, 'templates')

test('readTemplateSize 只认 body 规则里的画布尺寸', () => {
  const tpl = `
    body { width: 1080px; height: 1920px; margin: 0; }
    .dot::before { width: 8px; height: 8px; }
  `
  assert.deepEqual(readTemplateSize(tpl), { width: 1080, height: 1920 })
})

test('readTemplateSize 对占位符尺寸返回 null（尺寸随调用参数走）', () => {
  const tpl = `body{width:{{width}}px;height:{{height}}px;margin:0}`
  assert.equal(readTemplateSize(tpl), null)
})

test('frameVars 回填画布尺寸，避免 width:px 这类非法声明', () => {
  const vars = frameVars({ title: 'x' }, { width: 1080, height: 1920 }, { caption: '一句' })
  assert.equal(vars.width, 1080)
  assert.equal(vars.height, 1920)
  assert.equal(vars.caption, '一句')
  assert.equal(vars.image, '')
})

test('frameVars 以调用方显式尺寸为准', () => {
  const vars = frameVars({ width: 720, height: 1280 }, { width: 1080, height: 1920 }, {})
  assert.equal(vars.width, 720)
  assert.equal(vars.height, 1280)
})

test('fill 后 body 尺寸声明合法，不会留下空占位符', () => {
  const tpl = `body{width:{{width}}px;height:{{height}}px}`
  const html = fill(tpl, frameVars({}, { width: 1080, height: 1920 }, {}))
  assert.equal(html, `body{width:1080px;height:1920px}`)
  assert.doesNotMatch(html, /\{\{|\bpx\b(?![;\d])/)
})

test('每个模板的画布尺寸都能被解析或走占位符，且占位符都被覆盖', () => {
  const vars = frameVars({ title: 't', subtitle: 's', footer: 'f', accent1: '#fff' }, { width: 1080, height: 1920 }, { caption: 'c', image: 'data:image/png;base64,x' })
  for (const name of ['project-recommendation.html', 'product-presenter.html']) {
    const tpl = readFileSync(join(templatesDir, name), 'utf8')
    const size = readTemplateSize(tpl)
    assert.ok(size === null || (size.width > 0 && size.height > 0), `${name} 画布尺寸异常`)
    const html = fill(tpl, frameVars(vars, size ?? { width: 1080, height: 1920 }, {}))
    assert.doesNotMatch(html, /\{\{\w+\}\}/, `${name} 还有未覆盖的占位符`)
  }
})
