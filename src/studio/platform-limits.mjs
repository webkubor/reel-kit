// 发布平台的体积/时长上限是可配置阈值，不是平台官方承诺。
// 默认值取保守值；用户按自己账号实际限制改后，检测才按新阈值判定。
export const limitDefaults = { maxBytes: 100 * 1024 * 1024, maxDuration: 180 }

export function normalizeLimits(value = {}) {
  const maxBytes = Number(value.maxBytes ?? limitDefaults.maxBytes)
  const maxDuration = Number(value.maxDuration ?? limitDefaults.maxDuration)
  if (!Number.isFinite(maxBytes) || maxBytes < 1024 * 1024 || maxBytes > 4 * 1024 * 1024 * 1024) fail('文件大小上限应在 1MB–4GB 之间')
  if (!Number.isFinite(maxDuration) || maxDuration < 1 || maxDuration > 3600) fail('时长上限应在 1–3600 秒之间')
  return { maxBytes: Math.round(maxBytes), maxDuration: Math.round(maxDuration) }
}

export function limitsFor(state, platform) {
  const table = state.platformLimits || {}
  return normalizeLimits(table[platform] || table.default || {})
}

export function saveLimits(state, platform, value) {
  const limits = normalizeLimits(value)
  state.platformLimits ||= {}
  state.platformLimits[platform] = limits
  return limits
}

function fail(message) { throw Object.assign(new Error(message), { status: 400 }) }
