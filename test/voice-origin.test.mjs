import test from 'node:test'
import assert from 'node:assert/strict'
import { VOICE_ORIGINS, voiceChannelInfo } from '../src/studio/voice-origin.mjs'

// 工序面板要给用户看明白「这一步用的是哪一类声音、要不要花钱、是不是在本机跑」。
// 以前本机离线引擎（VoxFlow / sherpa）和云端付费引擎（MiniMax）都被标成
// 「AI 模型生成」—— 用户据此以为要花钱，或者以为要联网，两种都判错。
test('语音通道的来源标签：免费在线 / 云端付费 / 本机离线分得开', () => {
  assert.equal(voiceChannelInfo('edge').origin, 'free-online')
  assert.equal(voiceChannelInfo('minimax').origin, 'cloud-paid')
  assert.equal(voiceChannelInfo('sherpa').origin, 'local-offline')
  assert.equal(voiceChannelInfo('voxflow').origin, 'local-offline')
})

test('本机离线引擎不会被标成云端，本机跑的步骤不花钱', () => {
  for (const id of ['sherpa', 'voxflow']) {
    const { origin, engine } = voiceChannelInfo(id)
    assert.notEqual(origin, 'cloud-paid', `${id} 是本机跑的，不能标成云端付费`)
    assert.notEqual(origin, 'free-online', `${id} 是本机跑的，不是在线模型`)
    assert.ok(engine.length > 0)
  }
})

test('未知通道标成「未知」，不替用户猜来源', () => {
  // 以前这里回落成「本机离线」，等于告诉用户这一步不花钱、不联网。
  // 实际通道可能是在线付费的 —— 猜错比不猜贵：用户会放心地反复重试。
  const { origin } = voiceChannelInfo('nope')
  assert.equal(origin, 'unknown')
  assert.notEqual(origin, 'local-offline')
  assert.notEqual(origin, 'free-online')
  assert.notEqual(origin, 'cloud-paid')
  assert.ok(Object.keys(VOICE_ORIGINS).length >= 4)
})
