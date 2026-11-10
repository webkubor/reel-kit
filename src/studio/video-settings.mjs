import { resolveSettings } from '../settings.mjs'

export const platforms = {
  channels: { name: '视频号', ratio: '9:16', width: 1080, height: 1920 },
  xiaohongshu: { name: '小红书', ratio: '9:16', width: 1080, height: 1920 },
  landscape: { name: '横屏视频', ratio: '16:9', width: 1920, height: 1080 },
}

/**
 * 项目推荐的配置 —— 旋钮的默认值与范围都来自 src/settings.mjs。
 *
 * 这里只补两件这个流水线特有的事：画幅平台的尺寸，以及「这条片子是谁合成的」
 * （通道 / 模型 / 音色 / 账户是四件独立的事，都要跟着片子走；
 * 少存一个，接手的人就不知道这条片子当时是「谁」的声音）。
 */
export function recommendationSettings(input) {
  const platform = input.platform || 'channels'
  if (!platforms[platform]) throw new Error('请选择支持的发布平台')
  const voiceSource = input.voiceSource || 'scenes'
  if (!['tts', 'scenes', 'audio'].includes(voiceSource)) throw new Error('请选择旁白来源')
  // 旋钮走统一的收敛逻辑：预设 → 默认值 → 用户填的值。
  // 之前这里逐项 strictSetting，预设（口播清晰 / 氛围铺底）在这条流水线上是失效的。
  const settings = resolveSettings('recommendation', input)
  return {
    ...settings,
    platform, ...platforms[platform],
    voiceSource,
    // 通道 / 模型 / 音色 / 账户是四件独立的事，都要跟着这条片子走。
    // 少存一个，接手的人就不知道这条片子当时是「谁」的声音。
    voiceBackend: input.backend || 'voxflow',
    voiceModel: input.model || '',
    voiceId: input.voiceId || '',
    voiceProfile: input.profile || '',
  }
}

/**
 * 这条配音能不能复用。
 *
 * 必须连「谁合成的、用哪个模型、哪个音色」一起比 ——
 * 换了通道或换了音色的旧配音，语速、咬字、情绪都不一样，直接套用会听出接缝。
 */
export function reusableVoice(asset, input, line) {
  if (!asset?.mime?.startsWith('audio/')) return false
  if (asset.prompt !== line) return false
  if ((asset.tone || '') !== (input.tone || '')) return false
  const backend = input.backend || 'voxflow'
  const assetBackend = asset.voiceBackend || (asset.source === 'MiniMax TTS' ? 'minimax' : 'voxflow')
  if (backend !== assetBackend) return false
  const model = input.model || (backend === 'minimax' ? 'speech-02-hd' : 'voxflow-local')
  if ((asset.requestedModel || 'voxflow-local') !== model) return false
  const voice = input.voiceId || input.persona || ''
  if ((asset.persona || '') !== voice) return false
  // 在线语音把语速烤进了音频：语速对不上就不能复用，否则「填了语速没反应」。
  // 本机离线引擎的语速是出片后再变速的，复用旧配音不受影响。
  if (backend === 'edge' || backend === 'minimax') {
    if (Number(asset.speed || 1) !== Number(input.speed || 1)) return false
  }
  // 同一个模型在两个账户下也不是同一段声音 —— 账户也要比
  const profile = input.profile || ''
  return (asset.profile || '') === profile
}
