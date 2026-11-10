// 工序面板要回答用户一个问题：这一步用的是哪一类声音，要不要花钱，是不是在本机跑。
//
// 三条流水线（口播 / 项目推荐 / 产品介绍）共用这张表 —— 同一个通道在三条流水线里
// 必须标成同一个来源，否则用户看工序面板时会得出互相矛盾的结论。
//
// 来源只有四类，与 Go 侧的 originLabel 一一对应：
//   free-online  在线免费模型（微软 Edge 神经语音）
//   cloud-paid   云端付费模型（MiniMax TTS）
//   local-offline 本机离线模型（sherpa-onnx、VoxFlow）
export const VOICE_ORIGINS = {
  edge: { origin: 'free-online', engine: 'Edge 在线高保真语音' },
  minimax: { origin: 'cloud-paid', engine: 'MiniMax TTS' },
  sherpa: { origin: 'local-offline', engine: '本机离线语音' },
  voxflow: { origin: 'local-offline', engine: 'VoxFlow 本地语音' },
}

/**
 * 语音通道的来源标签。
 *
 * 未知通道按本机离线通道处理（与界面里「VoxFlow · 本地」的默认一致），
 * 因为服务端只会放行上面四个通道，这里的兜底只防脏数据。
 */
export function voiceChannelInfo(backend) {
  return VOICE_ORIGINS[backend] || VOICE_ORIGINS.voxflow
}
