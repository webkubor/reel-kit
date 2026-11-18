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

// 头像：有「-头像」素材就用它（每个角色自己的头像），否则退回正面视图 > 定妆稿。
export function pickPortrait(c) {
  if (!c) return null
  const avatar = (c.references || []).find((a) => /头像|avatar/i.test(a.name || '') && a.url)
  if (avatar) return avatar
  const front = (c.assets || []).find((a) => a.view === 'front' && a.url)
  if (front) return front
  const refs = (c.references || []).filter((a) => a.url)
  return refs.find((a) => /三视图|定妆|turnaround/i.test(a.name || '')) || refs.find((a) => !isWeapon(a.name)) || null
}

// 素材按类型分组，角色详情里分开展示，画面素材不进角色物料。
//
// 命名约定（导入时按此归类）：
//   -头像 / avatar           → 头像
//   -定妆 / 定妆照 / costume  → 定妆立绘
//   -三视图 / turnaround     → 三视图
//   兵器/面具/道具关键词      → 兵器与道具
//   其余                      → 画面素材（不进角色详情，另列）
export function groupReferences(c) {
  const refs = (c?.references || []).filter((a) => a.url)
  const avatars = refs.filter((a) => /头像|avatar/i.test(a.name || ''))
  const sheets = refs.filter((a) => /三视图|turnaround/i.test(a.name || ''))
  const costumes = refs.filter((a) => /定妆|costume|立绘/i.test(a.name || ''))
  const weapons = refs.filter((a) => isWeapon(a.name))
  const inRole = new Set([...avatars, ...sheets, ...costumes, ...weapons].map((a) => a.url))
  const scenes = refs.filter((a) => !inRole.has(a.url))
  return { avatars, sheets, costumes, weapons, scenes }
}
