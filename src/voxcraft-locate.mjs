/**
 * 定位本机的 voxcraft 安装。
 *
 * 为什么需要这个：voxcraft 是 Python 项目，装完后 `voice` 命令在它自己的 venv 里
 * （`<repo>/.venv/bin/voice`），**默认不在 PATH**。只判断 `which voice` 会误判成
 * 「没装」，进而重复装一遍 —— 而模型有 4.2GB，重复装是实打实的浪费。
 *
 * 更麻烦的是同一台机器可能有多份 clone（实测就有两份，一份装了模型、
 * 另一份存着注册好的音色库）。所以不能找到一个就用，要按「完整度」挑。
 */

import { existsSync, readFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { homedir } from 'node:os'
import { join } from 'node:path'

/** 常见安装位置。按用户的目录约定，app/ 是成品应用该在的地方，排最前。 */
const CANDIDATE_HOMES = [
  process.env.VOXCRAFT_HOME,
  join(homedir(), 'dev/github/app/voxcraft'),
  join(homedir(), 'dev/github/webkubor/voxcraft'),
  join(homedir(), 'dev/github/voxcraft'),
  join(homedir(), 'voxcraft'),
].filter(Boolean)

/**
 * 检查一个目录是不是可用的 voxcraft 安装，并给出完整度。
 * @returns {null | {home:string, bin:string|null, hasModel:boolean, personas:string[], score:number}}
 */
function inspect(home) {
  if (!existsSync(join(home, 'pyproject.toml'))) return null

  const venvBin = join(home, '.venv/bin/voice')
  const bin = existsSync(venvBin) ? venvBin : null

  // Base 模型是克隆合成的必需项；VoiceDesign 只有做音色设计才要
  const hasModel = existsSync(join(home, 'models/Base-1.7B'))

  let personas = []
  const pf = join(home, 'configs/personas.json')
  if (existsSync(pf)) {
    try { personas = Object.keys(JSON.parse(readFileSync(pf, 'utf-8')) || {}) } catch { /* 坏文件当没有 */ }
  }

  // 打分：能跑(4) > 有模型(2) > 有音色(1)。挑最完整的那份，而不是第一个找到的。
  const score = (bin ? 4 : 0) + (hasModel ? 2 : 0) + (personas.length ? 1 : 0)
  return { home, bin, hasModel, personas, score }
}

/**
 * 找出最完整的一份 voxcraft。
 * @returns {{ok:boolean, bin:string|null, home:string|null, hasModel:boolean, personas:string[], all:object[], reason?:string}}
 */
export function locateVoxcraft() {
  // PATH 里有就直接用 —— 用户显式装到全局，尊重这个选择
  let pathBin = null
  try {
    pathBin = execFileSync('sh', ['-c', 'command -v voice'], { encoding: 'utf-8' }).trim() || null
  } catch { /* 不在 PATH */ }

  const found = []
  const seen = new Set()
  for (const h of CANDIDATE_HOMES) {
    if (seen.has(h)) continue
    seen.add(h)
    const info = inspect(h)
    if (info) found.push(info)
  }
  found.sort((a, b) => b.score - a.score)

  const best = found[0]
  if (pathBin && !best) {
    return { ok: true, bin: pathBin, home: null, hasModel: true, personas: [], all: [] }
  }
  if (!best) {
    return {
      ok: false, bin: null, home: null, hasModel: false, personas: [], all: [],
      reason: '本机找不到 voxcraft。它是 Python 项目，需先 clone 并跑 install.sh',
    }
  }
  if (!best.bin) {
    return { ...best, ok: false, all: found, reason: `找到 ${best.home} 但没有 .venv —— 依赖未安装` }
  }
  if (!best.hasModel) {
    return { ...best, ok: false, all: found, reason: `${best.home} 已装依赖但缺 Base-1.7B 模型（4.2GB）` }
  }
  return { ...best, ok: true, all: found }
}

/** 给人看的修复指引 —— 不自动装 4.2GB 的东西，那种体量的操作必须让人知情 */
export function voxcraftHint(info) {
  const lines = []
  if (!info.home) {
    lines.push('  git clone https://github.com/webkubor/voxcraft ~/dev/github/app/voxcraft')
    lines.push('  cd ~/dev/github/app/voxcraft && ./install.sh --yes')
  } else if (!info.bin) {
    lines.push(`  cd ${info.home} && ./install.sh --yes`)
  } else if (!info.hasModel) {
    lines.push(`  cd ${info.home} && ./install.sh --yes   # 会补下 Base-1.7B (4.2GB)`)
  }
  lines.push('')
  lines.push('  或临时改用 API 后端（无需本地模型）：--voice-engine museav')
  return lines.join('\n')
}
