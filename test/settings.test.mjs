import test from 'node:test'
import assert from 'node:assert/strict'
import {
  SETTING_SCHEMA, PRESETS, DEFAULT_BGM_GAIN,
  resolveSettings, strictSetting, describeSetting, listPresets,
} from '../src/settings.mjs'
import { recommendationSettings } from '../src/studio/video-settings.mjs'

test('每个旋钮都有分组、标签与类型', () => {
  for (const [key, field] of Object.entries(SETTING_SCHEMA)) {
    assert.ok(field.group, `${key} 少了分组`)
    assert.ok(field.label, `${key} 少了标签`)
    assert.ok(['number', 'bool', 'text', 'choice'].includes(field.type), `${key} 的类型不认识`)
    if (field.type === 'number') {
      assert.ok(typeof field.default === 'number', `${key} 的默认值不是数字`)
      assert.ok(field.min <= field.default && field.default <= field.max, `${key} 的默认值超出范围`)
    }
  }
})

test('BGM 音量全工程只有一个默认值', () => {
  // 以前 src/compose.mjs 里是 0.22、src/studio/video-settings.mjs 里是 0.12，
  // 谁漏传参数就用哪个。现在只留一处。
  assert.equal(DEFAULT_BGM_GAIN, SETTING_SCHEMA.bgmGain.default)
  const settings = recommendationSettings({})
  assert.equal(settings.bgmGain, DEFAULT_BGM_GAIN)
})

test('预设盖住默认值，用户填的值再盖住预设', () => {
  const base = resolveSettings('talk', {})
  assert.equal(base.bgmGain, 0.12)

  const preset = resolveSettings('talk', { preset: 'clear-speech' })
  assert.equal(preset.bgmGain, 0.08)
  assert.equal(preset.voiceGain, 1.5)

  const overridden = resolveSettings('talk', { preset: 'clear-speech', bgmGain: 0.2 })
  assert.equal(overridden.bgmGain, 0.2, '用户填的值必须压过预设')
})

test('每个预设都只碰旋钮表里有的键', () => {
  for (const [id, preset] of Object.entries(PRESETS)) {
    for (const key of Object.keys(preset.settings)) {
      assert.ok(SETTING_SCHEMA[key], `预设 ${id} 里的 ${key} 不在旋钮表里`)
    }
  }
  const ids = listPresets().map(p => p.id)
  assert.ok(ids.includes('default'))
  assert.ok(ids.length === Object.keys(PRESETS).length)
})

test('越界的值要报错，而且报的是人话', () => {
  assert.throws(
    () => strictSetting('recommendation', 'bgmGain', { bgmGain: 12 }),
    /BGM 音量.*0–0\.5/,
  )
  assert.throws(
    () => strictSetting('recommendation', 'speed', { speed: '快一点' }),
    /语速.*数字/,
  )
  assert.throws(
    () => strictSetting('recommendation', 'imageFit', { imageFit: '拉伸' }),
    /画面填充/,
  )
  // 合法值不报错
  assert.equal(strictSetting('recommendation', 'bgmGain', { bgmGain: 0.2 }), 0.2)
})

test('没填就取默认值，各流水线不同时取各自的', () => {
  assert.equal(strictSetting('talk', 'pause', {}), 0.45)
  assert.equal(strictSetting('recommendation', 'pause', {}), 0.35)
  assert.equal(strictSetting('product', 'pause', {}), 0.4)
})

test('resolveSettings 只给这条流水线用得上的旋钮', () => {
  const talk = resolveSettings('talk', {})
  assert.equal(talk.platform, undefined, '发布平台只对项目推荐有意义')
  const rec = resolveSettings('recommendation', {})
  assert.equal(rec.platform, 'channels')
})

test('值写成给人看的样子', () => {
  assert.equal(describeSetting('bgmGain', 0.12), '12%')
  assert.equal(describeSetting('voiceGain', 1.35), '1.35 倍')
  assert.equal(describeSetting('duck', true), '开')
  assert.equal(describeSetting('duck', false), '关')
  assert.equal(describeSetting('imageFit', 'cover'), '铺满画布')
})
