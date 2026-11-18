// 角色详情读取：经 Wails 绑定调 Engine.GetCharacter（每次会跑一次 museav CLI）。
// 18～30 个角色同时拉会把 CLI 进程堆满，所以限并发、按 id 缓存。
const cache = new Map()
const queue = []
let running = 0
const LIMIT = 4

function pump() {
  while (running < LIMIT && queue.length) {
    const job = queue.shift()
    running++
    window.go.main.Engine.GetCharacter(job.id)
      .then(job.resolve, job.reject)
      .finally(() => { running--; pump() })
  }
}

export function loadCharacter(id) {
  if (!cache.has(id)) {
    const p = new Promise((resolve, reject) => { queue.push({ id, resolve, reject }); pump() })
    p.catch(() => cache.delete(id)) // 失败不缓存，下次点开可以重试
    cache.set(id, p)
  }
  return cache.get(id)
}

export function forgetCharacters() { cache.clear() }

export const VIEWS = [
  { key: 'front', label: '正面' },
  { key: 'left45', label: '左45°' },
  { key: 'side', label: '正侧面' },
  { key: 'right45', label: '右45°' },
  { key: 'back', label: '背面' },
]

// 兵器与道具（面具、面甲等）单独成组：它们不是人物定妆，不该和三视图挤在一起。
const isWeapon = (name) => /笛|枪|刀|剑|弓|锤|兵器|武器|面具|面甲|道具/.test(name || '') && !/三视图|定妆/.test(name || '')

// 真人参考：现实照片质感或来源标注「真人/脸部参考」，永远不进角色形象区。
const isRealFace = (name) => /真人参考|脸部参考|真人|face|photo/i.test(name || '')

// 头像：只从「角色头像」类选（真人参考除外），否则退回定妆立绘 > 三视图。
// 规则见 docs/asset-classification.md：角色是电影角色，真人参考只是建模参考。
export function pickPortrait(c) {
  if (!c) return null
  const avatar = (c.references || []).find((a) => /头像|avatar/i.test(a.name || '') && a.url && !isRealFace(a.name))
  if (avatar) return avatar
  const front = (c.assets || []).find((a) => a.view === 'front' && a.url)
  if (front) return front
  const refs = (c.references || []).filter((a) => a.url && !isRealFace(a.name))
  return refs.find((a) => /定妆|costume|立绘/i.test(a.name || '')) || refs.find((a) => /三视图|turnaround/i.test(a.name || '')) || null
}

// 素材按类型分组（见 docs/asset-classification.md 六类）：
//   角色头像 / 定妆立绘 / 三视图 / 兵器与道具 / 真人参考 / 画面素材
export function groupReferences(c) {
  const seen = new Set()
  const refs = (c?.references || []).filter((a) => {
    if (!a.url) return false
    // 同一素材（assetId 或 URL）只出现一次：导入去重后仍可能有同名不同 URL
    const key = a.id || a.url
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
  const realFaces = refs.filter((a) => isRealFace(a.name))
  const avatars = refs.filter((a) => /头像|avatar/i.test(a.name || '') && !isRealFace(a.name))
  const sheets = refs.filter((a) => /三视图|turnaround/i.test(a.name || ''))
  const costumes = refs.filter((a) => /定妆|costume|立绘/i.test(a.name || ''))
  const weapons = refs.filter((a) => isWeapon(a.name))
  const inRole = new Set([...realFaces, ...avatars, ...sheets, ...costumes, ...weapons].map((a) => a.url))
  const scenes = refs.filter((a) => !inRole.has(a.url))
  return { avatars, sheets, costumes, weapons, realFaces, scenes }
}
