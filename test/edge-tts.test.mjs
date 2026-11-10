import test from 'node:test'
import assert from 'node:assert/strict'
import { EDGE_VOICES, formatSpeedRate, edgeSynthesize } from '../src/edge-tts.mjs'

test('formatSpeedRate 格式化正确', () => {
  assert.equal(formatSpeedRate(1.0), '+0%')
  assert.equal(formatSpeedRate(1.1), '+10%')
  assert.equal(formatSpeedRate(1.25), '+25%')
  assert.equal(formatSpeedRate(0.85), '-15%')
  assert.equal(formatSpeedRate(0.5), '-50%')
  assert.equal(formatSpeedRate(null), '+0%')
  assert.equal(formatSpeedRate('invalid'), '+0%')
})

test('EDGE_VOICES 包含常用音色且结构合法', () => {
  assert.ok(EDGE_VOICES.length >= 6)
  const xiaoxiao = EDGE_VOICES.find(v => v.id === 'zh-CN-XiaoxiaoNeural')
  assert.ok(xiaoxiao, '包含晓晓女声')
  const yunxi = EDGE_VOICES.find(v => v.id === 'zh-CN-YunxiNeural')
  assert.ok(yunxi, '包含云希男声')
})

test('edgeSynthesize 真实在线合成能输出有效音频字节', async () => {
  const result = await edgeSynthesize({
    text: '测试 Edge 在线语音合成',
    voice: 'zh-CN-XiaoxiaoNeural',
    speed: 1.1,
  })
  assert.equal(result.mime, 'audio/mpeg')
  assert.ok(Buffer.isBuffer(result.bytes))
  assert.ok(result.bytes.length > 1000, `输出字节数过少: ${result.bytes.length}`)
})
