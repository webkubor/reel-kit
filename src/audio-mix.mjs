/**
 * 混音链 —— 人声、配乐怎么合成一条最终音轨。
 *
 * 为什么单独抽出来：concat 路径（无转场）与 xfade 路径（有转场）本来各写了一份
 * amix，两份迟早会走偏（第一次改的时候只改了其中一处，响度对不上）。
 * 现在两条路径共用同一段滤镜链，参数只在这里定义一次。
 *
 * 这一版修掉的四个问题（每个都有可复现的实测）：
 *
 * 1) 采样率被输入带跑。
 *    amix 沿用**第一个输入**的格式。念白来自本地 TTS，是 16 kHz 单声道；
 *    配乐是 44.1 kHz 立体声。两者一混，整条音轨被压成 16 kHz 单声道 ——
 *    立体声的宽度没了，7 kHz 以上的能量掉了约 11 dB。
 *    所以进 amix 之前先把每条支路统一到 48 kHz 立体声。
 *
 * 2) amix 默认把每路乘 1/N。
 *    两路输入各按 1/2 衰减再相加，整体比不衰减低 6 dB。
 *    同一份素材实测：默认是 -26.2 LUFS，不衰减是 -20.1 LUFS。
 *
 * 3) 固定增益压不住配乐。
 *    固定增益只能让人声和配乐「同时变小」，念白与间隙的相对关系不变。
 *    侧链压缩（sidechaincompress）才是拿人声当控制信号：
 *    人声一开口就把配乐压下去，人声停了配乐抬回来。
 *
 * 4) 成品没有响度目标。
 *    自己的片子响度随素材浮动。这条链把它归一化到固定响度与真峰，
 *    不同素材、不同后端出来的片子放在一起不会一轻一重。
 */

/** 目标综合响度（LUFS）。 */
export const TARGET_LUFS = -14

/** 目标真峰（dBTP）。 */
export const TARGET_TP = -1.0

/**
 * 统一到的采样率与声道数。
 *
 * aformat 那段不能省，两个原因：
 *   一是桌面版内置的 FFmpeg 4.4 里，sidechaincompress 的两个输入格式必须
 *     完全一致，否则直接报 "could not choose their formats"；
 *   二是单声道念白要变成双声道才能与配乐对齐 —— aformat 会自动补上
 *     声道转换（实测桌面版内置的 2018 与 4.4 两个版本都成立）。
 */
export const MIX_SAMPLE_RATE = 48000

/** 送进混音链的统一格式：48 kHz 立体声、浮点 planar。 */
const MIX_FORMAT = `aformat=sample_fmts=fltp:sample_rates=${MIX_SAMPLE_RATE}:channel_layouts=stereo`

/**
 * 人声默认增益。
 *
 * 1.35 ≈ +2.6 dB：让念白稳在配乐之上，又不至于把底噪一起抬起来。
 */
export const DEFAULT_VOICE_GAIN = 1.35

/** 人声高通：砍掉 110 Hz 以下的隆隆声与手持噪声。它不是「让人声变清楚」的开关。 */
const VOICE_HIGH_PASS = 110

/**
 * 配乐侧链参数。
 *
 * threshold=0.030（线性幅度，约 -30 dBFS）：只有人声真的响起来才触发，
 * 气音、句尾的轻声不会让配乐抖一下。
 * ratio=8：人声段配乐压下去 8–12 dB —— 够把位置让给念白，又不会让音乐整段消失。
 * release=320ms：人声停后配乐慢慢回来，不是硬切。
 *
 * 压多深取决于人声比阈值高多少，同样是 ratio=8，
 * 高出 8 dB 的人声压下去 7 dB，高出 16 dB 的人声压下去 14 dB。
 * （实测见 test/audio-mix.test.mjs 的侧链那条。）
 */
const DUCK_THRESHOLD = 0.030
const DUCK_RATIO = 8
const DUCK_ATTACK = 4
const DUCK_RELEASE = 320

/**
 * 限幅：兜底，防止叠加后削顶。
 *
 * limit=0.891 ≈ -1.0 dBFS，与 TARGET_TP 对齐。
 * level=disabled 是必须的：alimiter 默认开着 auto level，会把输出重新拉到限幅值，
 * 那样就把上面 loudnorm 定下来的响度又推翻了。
 */
const LIMIT = 0.891
const LIMIT_ATTACK = 5
const LIMIT_RELEASE = 50

/**
 * 构造混音滤镜链。
 *
 * @param {object}   opts
 * @param {string?}  opts.voiceLabel  人声输入标签，如 '[voice]'；没有就传 null
 * @param {string?}  opts.bgmLabel    配乐输入标签，如 '[bgm]'；没有就传 null
 * @param {number}   [opts.voiceGain] 人声增益，默认 DEFAULT_VOICE_GAIN
 * @param {boolean}  [opts.duck]      人声是否侧链压低配乐，默认 true
 * @param {boolean}  [opts.normalize] 是否做响度归一化，默认 true
 * @param {string}   [opts.outLabel]  输出标签，默认 '[aout]'
 * @returns {string[]} ffmpeg -filter_complex 的片段
 */
export function buildMixChain({
  voiceLabel = null,
  bgmLabel = null,
  voiceGain = DEFAULT_VOICE_GAIN,
  duck = true,
  normalize = true,
  outLabel = '[aout]',
}) {
  const filters = []
  const mixIn = []

  const useDuck = Boolean(duck && voiceLabel && bgmLabel)

  // 增益要是个数：NaN 或负数会让整条链静默失灵（volume=NaN 直接不报错，但音量归零）。
  let gain = Number(voiceGain)
  if (!Number.isFinite(gain) || gain <= 0) gain = DEFAULT_VOICE_GAIN

  let voiceOut = null
  let duckKey = null

  if (voiceLabel) {
    // 统一到 48 kHz 立体声：单声道念白由 aformat 补成双声道，与配乐对齐。
    const head =
      `${voiceLabel}aresample=${MIX_SAMPLE_RATE},${MIX_FORMAT},` +
      `highpass=f=${VOICE_HIGH_PASS},volume=${gain}`
    if (useDuck) {
      // 分两路：一路进混音，一路当侧链的触发信号。
      filters.push(`${head},asplit=2[voxA][voxS]`)
      voiceOut = '[voxA]'
      duckKey = '[voxS]'
    } else {
      filters.push(`${head}[voxA]`)
      voiceOut = '[voxA]'
    }
  }

  let bgmOut = null
  if (bgmLabel) {
    filters.push(`${bgmLabel}aresample=${MIX_SAMPLE_RATE},${MIX_FORMAT}[bgmR]`)
    if (useDuck && duckKey) {
      filters.push(
        `[bgmR]${duckKey}sidechaincompress=threshold=${DUCK_THRESHOLD}:ratio=${DUCK_RATIO}:` +
        `attack=${DUCK_ATTACK}:release=${DUCK_RELEASE}:level_sc=1[bgmD]`,
      )
    } else {
      filters.push('[bgmR]anull[bgmD]')
    }
    bgmOut = '[bgmD]'
  }

  if (voiceOut) mixIn.push(voiceOut)
  if (bgmOut) mixIn.push(bgmOut)

  if (!mixIn.length) {
    // 没有任何音频输入：出等长静音，音轨时长由调用方的 -shortest 决定。
    filters.push(`anullsrc=r=${MIX_SAMPLE_RATE}:cl=stereo[${outLabel.slice(1, -1)}]`)
    return filters
  }

  if (mixIn.length === 1) {
    filters.push(`${mixIn[0]}anull[mixed]`)
  } else {
    /*
     * 不写 normalize=0：桌面版内置的 macOS-Intel（2018 构建）与 Windows（4.1）
     * 的 amix 没有这个选项，写了直接报 "Option not found"。
     * amix 默认把每路乘 1/N，后面再 volume=N 乘回来，电平与 normalize=0 一致
     * （4.4 上逐样本相同，2018 构建上峰值/响度相同）。
     * 两路时长在调用方已对齐，不会触发 amix 的掉线重归一。
     */
    filters.push(
      `${mixIn.join('')}amix=inputs=${mixIn.length}:duration=first:dropout_transition=0,` +
      `volume=${mixIn.length}[mixed]`,
    )
  }

  if (normalize) {
    /*
     * loudnorm 单次扫描：自己把整条音轨推到目标响度。
     * 它把输出采样率设成 192 kHz（内部精度需要），后面要拉回 48 kHz，
     * 否则封装出来的 AAC 采样率高得离谱、体积翻倍。
     */
    filters.push(`[mixed]loudnorm=I=${TARGET_LUFS}:TP=${TARGET_TP}:LRA=11[ln]`)
    filters.push(`[ln]aresample=${MIX_SAMPLE_RATE}[lnr]`)
    filters.push(
      `[lnr]alimiter=limit=${LIMIT}:attack=${LIMIT_ATTACK}:release=${LIMIT_RELEASE}:level=disabled${outLabel}`,
    )
    return filters
  }

  /*
   * 不做响度归一化，但限幅仍然要留：
   * 归一化关掉后两路直接相加，峰值可能超过 0 dBFS，编码出来是削顶的波形。
   * 限幅只压过峰，不改整体响度 —— 「原样输出」要的是这个，不是「允许削顶」。
   */
  filters.push(
    `[mixed]alimiter=limit=${LIMIT}:attack=${LIMIT_ATTACK}:release=${LIMIT_RELEASE}:level=disabled${outLabel}`,
  )
  return filters
}
