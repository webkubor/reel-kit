import { strictSetting } from '../settings.mjs'

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
  const platform = strictSetting('recommendation', 'platform', input) || 'channels'
  if (!platforms[platform]) throw new Error('请选择支持的发布平台')
  const voiceSource = input.voiceSource || 'scenes'
  if (!['tts', 'scenes', 'audio'].includes(voiceSource)) throw new Error('请选择旁白来源')
  return {
    platform, ...platforms[platform],
    speed: strictSetting('recommendation', 'speed', input),
    pause: strictSetting('recommendation', 'pause', input),
    bgmGain: strictSetting('recommendation', 'bgmGain', input),
    voiceGain: strictSetting('recommendation', 'voiceGain', input),
    duck: strictSetting('recommendation', 'duck', input),
    normalize: strictSetting('recommendation', 'normalize', input),
    voiceSource,
    imageFit: strictSetting('recommendation', 'imageFit', input),
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
  // 同一个模型在两个账户下也不是同一段声音 —— 账户也要比
  const profile = input.profile || ''
  return (asset.profile || '') === profile
}
