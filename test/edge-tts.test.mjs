import test from 'node:test'
import assert from 'node:assert/strict'
import { EDGE_VOICES, formatSpeedRate, edgeSynthesize, withTimeout } from '../src/edge-tts.mjs'

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

// 网络不通时不能永远挂着：没有时限的话，任务会一直转圈，既不出片也不报错。
test('withTimeout 超时会报错，promise 提前结束就不报错', async () => {
  await assert.rejects(
    withTimeout(new Promise(() => {}), 30, 'Edge 语音服务连接超时'),
    /连接超时/,
  )
  const ok = await withTimeout(Promise.resolve('done'), 1000, '不该触发')
  assert.equal(ok, 'done')
})
