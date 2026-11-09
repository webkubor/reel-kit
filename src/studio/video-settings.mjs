export const platforms = {
  channels: { name: '视频号', ratio: '9:16', width: 1080, height: 1920 },
  xiaohongshu: { name: '小红书', ratio: '9:16', width: 1080, height: 1920 },
  landscape: { name: '横屏视频', ratio: '16:9', width: 1920, height: 1080 },
}
export function recommendationSettings(input) {
  const platform = input.platform || 'channels'
  if (!platforms[platform]) throw new Error('请选择支持的发布平台')
  const speed = Number(input.speed ?? 1), pause = Number(input.pause ?? 0.35), bgmGain = Number(input.bgmGain ?? 0.12)
  if (!Number.isFinite(speed) || speed < 0.75 || speed > 1.5) throw new Error('语速范围为 0.75–1.5 倍')
  if (!Number.isFinite(pause) || pause < 0 || pause > 2) throw new Error('镜头停顿范围为 0–2 秒')
  if (!Number.isFinite(bgmGain) || bgmGain < 0 || bgmGain > 0.5) throw new Error('BGM 音量范围为 0–50%')
  const voiceSource = input.voiceSource || 'scenes'
  if (!['tts', 'scenes', 'audio'].includes(voiceSource)) throw new Error('请选择旁白来源')
  if (!['cover', 'contain'].includes(input.imageFit || 'contain')) throw new Error('请选择画面填充方式')
  return {
    platform, ...platforms[platform], speed, pause, bgmGain, voiceSource,
    imageFit: input.imageFit || 'contain',
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
