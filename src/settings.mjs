/**
 * 一条视频的生成配置 —— 全工程唯一的旋钮表。
 *
 * 为什么要有这一层：
 *
 * 改之前，每个旋钮各自散在三个地方 ——
 *   桌面端表单（Go）  自己写一遍默认值、自己写一遍范围
 *   后端流水线（Node） 有的读 input、有的读自己的兜底
 *   CLI（bin/reel.mjs） 再写一遍 flag
 *
 * 同一个「BGM 音量」就有两个默认值：后端 settings 里是 0.12，
 * src/compose.mjs 里是 0.22（有配音时）。谁漏传参数就用哪个，
 * 用户看到的音量就不是他填的那个 —— 这类「说不清来源」的小毛病，
 * 都是因为没有一张唯一的旋钮表。
 *
 * 这一层定下三件事：
 *
 * 1) 每个旋钮是什么、默认多少、允许范围多大 —— SETTING_SCHEMA
 * 2) 常用组合 —— PRESETS（「口播清晰」比「人声增益 1.5 + BGM 8%」好记）
 * 3) 把用户填的一堆值收敛成一份完整配置 —— resolveSettings
 *
 * 分工：这一层管**配置**（填什么、默认值、范围），不管**执行**
 * （怎么渲染、怎么混音）。执行在各流水线里。
 *
 * 界面上仍然按七块分节（脚本/声音/画面/版式/画幅/配乐/输出），
 * 与桌面端的配置面板一致；通用表只放跨流水线共用的旋钮，
 * 「脚本」那块的字段各流水线自己定义（口播是文案，项目推荐是仓库地址）。
 */

/**
 * BGM 的默认音量。
 *
 * 只有配乐、没有念白时不适用 —— 那时配乐是主体，应该按全音量播。
 * 这个区别写在 src/compose.mjs 里，避免各处再抄一遍。
 */
export const DEFAULT_BGM_GAIN = 0.12

/**
 * 分节顺序。
 *
 * 与桌面端 config.go 里的 partOrder 保持一致 —— 两边同序，
 * 用户看表单的顺序和回看配置时的顺序一样。
 */
export const SETTING_GROUPS = ['脚本', '声音', '画面', '版式', '画幅', '配乐', '输出']

/**
 * 旋钮表。
 *
 * 字段含义：
 *   group     属于哪一节
 *   label     给用户看的名字（不是变量名）
 *   type      number | bool | text | choice
 *   default   默认值
 *   min/max   number 的合法区间（闭区间）
 *   options   choice 的候选值 [{value, label}]
 *   display   number 的展示方式：percent 表示界面显示百分比
 *   kinds     这个旋钮对哪些流水线有效；不写 = 全部
 *   hint      界面提示（说清「什么时候需要改」）
 */
export const SETTING_SCHEMA = {
  /* ── 声音 ───────────────────────────────────────────── */

  voiceGain: {
    group: '声音',
    label: '人声音量',
    type: 'number',
    default: 1.35,
    min: 0.4,
    max: 3,
    step: 0.05,
    unit: '倍',
    hint: '念白本身偏小就调大；配乐压不住念白时先看这个',
  },

  duck: {
    group: '声音',
    label: '人声时压低配乐',
    type: 'bool',
    default: true,
    hint: '人声一开口就把配乐压下去，人声停时抬回来',
  },

  tone: {
    group: '声音',
    label: '语气',
    type: 'text',
    default: '',
    hint: '留空用音色默认',
  },

  speed: {
    group: '声音',
    label: '语速',
    type: 'number',
    default: 1,
    min: 0.75,
    max: 1.5,
    step: 0.05,
    unit: '倍',
    kinds: ['recommendation', 'talk', 'product'],
  },

  pause: {
    group: '声音',
    label: '镜间停顿',
    type: 'number',
    default: 0.35,
    min: 0,
    max: 2,
    step: 0.05,
    unit: '秒',
    hint: '每句话念完之后停多久',
  },

  /* ── 画面 ───────────────────────────────────────────── */

  imageFit: {
    group: '画面',
    label: '画面填充',
    type: 'choice',
    default: 'contain',
    options: [
      { value: 'contain', label: '完整画面' },
      { value: 'cover', label: '铺满画布' },
    ],
  },

  motion: {
    group: '画面', label: '镜内动效', type: 'choice', default: 'gentle',
    options: [{ value: 'none', label: '静态画面（较快）' }, { value: 'gentle', label: '轻柔运镜（逐帧渲染）' }],
    hint: '图片缓慢推进、字幕轻柔入场；逐帧渲染，生成时间更长',
  },

  transition: {
    group: '画面',
    label: '切镜转场',
    type: 'choice',
    default: 'none',
    options: [
      { value: 'none', label: '硬切（无转场）' },
      { value: 'fade', label: '淡入淡出' },
      { value: 'slide-left', label: '向左推移' },
      { value: 'slide-right', label: '向右推移' },
      { value: 'slide-up', label: '向上推移' },
      { value: 'slide-down', label: '向下推移' },
      { value: 'wipe-left', label: '向左擦除' },
      { value: 'wipe-right', label: '向右擦除' },
      { value: 'dissolve', label: '溶解交叉' },
      { value: 'zoom-in', label: '镜头推进' },
    ],
    hint: '分镜切换时的视觉动效',
  },

  transitionDuration: {
    group: '画面',
    label: '转场时长',
    type: 'number',
    default: 0.35,
    min: 0.1,
    max: 1.0,
    step: 0.05,
    unit: '秒',
    hint: '转场动效持续时间',
  },

  /* ── 画幅 ───────────────────────────────────────────── */

  platform: {
    group: '画幅',
    label: '发布平台',
    type: 'choice',
    default: 'channels',
    options: [
      { value: 'channels', label: '视频号' },
      { value: 'xiaohongshu', label: '小红书' },
      { value: 'landscape', label: '横屏视频' },
    ],
    kinds: ['recommendation'],
  },

  /* ── 配乐 ───────────────────────────────────────────── */

  bgmGain: {
    group: '配乐',
    label: 'BGM 音量',
    type: 'number',
    default: 0.12,
    min: 0,
    max: 0.5,
    step: 0.01,
    display: 'percent',
    hint: '0–50%，数字越大越响',
  },

  normalize: {
    group: '配乐',
    label: '统一响度',
    type: 'bool',
    default: true,
    hint: '把成品推到固定响度，不同片子放一起不会一轻一重',
  },
}

/**
 * 预设 —— 常用组合，一键套用。
 *
 * 预设不是「另一套默认值」，是往默认配置上盖一层。
 * 用户套了预设之后仍然可以逐项改，改了的项不再被预设覆盖。
 */
export const PRESETS = {
  default: {
    name: '默认',
    desc: '通用设置：人声清楚，配乐垫底，成品响度统一',
    settings: {},
  },
  'clear-speech': {
    name: '口播清晰',
    desc: '以念白为主：配乐压到 8%，人声提到 1.5 倍',
    settings: { bgmGain: 0.08, voiceGain: 1.5, duck: true },
  },
  ambient: {
    name: '氛围铺底',
    desc: '以配乐为主：配乐提到 30%，人声只留一点',
    settings: { bgmGain: 0.3, voiceGain: 1.1, duck: true },
  },
  raw: {
    name: '原样输出',
    desc: '不做响度归一化、不压配乐 —— 自己在后期软件里统一音量时用',
    settings: { normalize: false, duck: false },
  },
}

/** 各流水线自己的默认值。同一旋钮在不同流水线里默认值不同时，在这里写。 */
const KIND_DEFAULTS = {
  talk: { pause: 0.45 },
  recommendation: { pause: 0.35 },
  product: { pause: 0.4 },
}

/** 这个旋钮对这条流水线是否生效。 */
function appliesTo(field, kind) {
  if (!field.kinds) return true
  return field.kinds.includes(kind)
}

/** 预设列表，给界面用（去掉 settings，只留给人看的部分）。 */
export function listPresets() {
  return Object.entries(PRESETS).map(([id, p]) => ({
    id,
    name: p.name,
    desc: p.desc,
    settings: { ...p.settings },
  }))
}

/**
 * 收敛一份完整配置。
 *
 * 顺序是：预设 → 默认值 → 用户填的值。用户在预设之上改过的项优先。
 *
 * @param {string} kind      流水线：talk / recommendation / export / product
 * @param {object} input     用户填的原始值
 * @param {object} [opts]
 * @param {string} [opts.preset] 预设 id
 * @returns {object} 完整配置（只含该流水线生效的旋钮）
 */
export function resolveSettings(kind, input = {}, opts = {}) {
  const presetId = opts.preset || input.preset
  const preset = PRESETS[presetId] ? PRESETS[presetId] : null
  const kindDefaults = KIND_DEFAULTS[kind] || {}

  const out = {}
  for (const [key, field] of Object.entries(SETTING_SCHEMA)) {
    if (!appliesTo(field, kind)) continue

    let value = field.default
    if (kindDefaults[key] !== undefined) value = kindDefaults[key]
    if (preset && preset.settings[key] !== undefined) value = preset.settings[key]
    if (input[key] !== undefined && input[key] !== null && input[key] !== '') {
      value = input[key]
    }

    out[key] = coerce(key, field, value)
  }
  return out
}

/**
 * 把一个值收敛成合法值。
 *
 * 越界不报错，改成边界值 —— 用户填错一个数字时把整次生成打断，
 * 比默默拉到边界更让人恼火。真正填不出来（NaN）的退回默认值。
 */
function coerce(key, field, value) {
  switch (field.type) {
    case 'bool':
      if (typeof value === 'boolean') return value
      if (value === 'true' || value === 'on' || value === 1) return true
      if (value === 'false' || value === 'off' || value === 0) return false
      return field.default
    case 'number': {
      const n = Number(value)
      if (!Number.isFinite(n)) return field.default
      if (n < field.min) return field.min
      if (n > field.max) return field.max
      return n
    }
    case 'choice': {
      const s = String(value)
      const ok = (field.options || []).some(o => o.value === s)
      return ok ? s : field.default
    }
    default:
      return typeof value === 'string' ? value : String(value ?? '')
  }
}

/**
 * 某个旋钮的当前值，配成人话。
 *
 * 与桌面端 config.go 的 configValue 一样：配置面板是给用户看的，
 * 写「1.35」用户得自己猜是什么，写「1.35 倍」才知道。
 */
export function describeSetting(key, value) {
  const field = SETTING_SCHEMA[key]
  if (!field) return String(value ?? '')
  switch (field.type) {
    case 'bool':
      return value ? '开' : '关'
    case 'number':
      if (field.display === 'percent') return `${Math.round(Number(value) * 100)}%`
      if (field.unit === '倍') return `${value} 倍`
      if (field.unit === '秒') return `${value} 秒`
      return String(value)
    case 'choice': {
      const opt = (field.options || []).find(o => o.value === value)
      return opt ? opt.label : String(value ?? '')
    }
    default:
      return String(value ?? '')
  }
}

/**
 * 严格模式取值 —— 越界直接报错，而不是悄悄拉到边界。
 *
 * 桌面端与 CLI 用这个：用户手填的数字填错了，要告诉他「这里填不了」，
 * 而不是把 999 当成 3 用掉 —— 那样用户永远不知道自己填错过。
 */
export function strictSetting(kind, key, input) {
  const field = SETTING_SCHEMA[key]
  if (!field) return undefined
  if (!appliesTo(field, kind)) return undefined

  const raw = input[key]
  if (raw === undefined || raw === null || raw === '') {
    const kindDefaults = KIND_DEFAULTS[kind] || {}
    return kindDefaults[key] !== undefined ? kindDefaults[key] : field.default
  }

  switch (field.type) {
    case 'bool':
      if (typeof raw === 'boolean') return raw
      if (raw === 'true' || raw === 'on' || raw === 1) return true
      if (raw === 'false' || raw === 'off' || raw === 0) return false
      throw new Error(`${field.label}填的是「${raw}」，只能是开或关`)
    case 'number': {
      const n = Number(raw)
      if (!Number.isFinite(n)) throw new Error(`${field.label}要填数字，现在填的是「${raw}」`)
      if (n < field.min || n > field.max) {
        throw new Error(`${field.label}要在 ${field.min}–${field.max} 之间，现在是 ${n}`)
      }
      return n
    }
    case 'choice': {
      const s = String(raw)
      const labels = (field.options || []).map(o => o.label).join(' / ')
      if (!(field.options || []).some(o => o.value === s)) {
        throw new Error(`${field.label}只能是：${labels}`)
      }
      return s
    }
    default:
      return String(raw)
  }
}

