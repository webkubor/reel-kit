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
  if (!['cover', 'contain'].includes(input.imageFit || 'cover')) throw new Error('请选择画面填充方式')
  return { platform, ...platforms[platform], speed, pause, bgmGain, voiceSource, imageFit: input.imageFit || 'cover' }
}
export function reusableVoice(asset, input, line) {
  return asset?.mime?.startsWith('audio/') && asset.persona === input.persona && asset.prompt === line && (asset.tone || '') === (input.tone || '')
}
