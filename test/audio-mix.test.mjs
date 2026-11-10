import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, existsSync, readdirSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { buildMixChain, TARGET_LUFS, TARGET_TP } from '../src/audio-mix.mjs'

/** 跑一条 ffmpeg 命令，失败即抛 */
function ff(args) {
  return execFileSync('ffmpeg', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
}

/**
 * 读一个音频文件的综合响度与真峰。
 * ebur128 的统计打在 stderr 上，而且过程中会刷好几行进度 ——
 * 取最后一次出现的那组才是最终统计。
 */
function loudness(file) {
  const r = spawnSync('ffmpeg', ['-hide_banner', '-nostats', '-i', file,
    '-af', 'ebur128=peak=true', '-f', 'null', '-'], { encoding: 'utf8' })
  const out = `${r.stderr || ''}\n${r.stdout || ''}`
  const all = [...out.matchAll(/^\s*I:\s+(-?[\d.]+) LUFS/gm)]
  const peaks = [...out.matchAll(/^\s*Peak:\s+(-?[\d.]+) dBFS/gm)]
  return {
    I: all.length ? Number(all[all.length - 1][1]) : NaN,
    peak: peaks.length ? Number(peaks[peaks.length - 1][1]) : NaN,
  }
}

test('mix chain 不产生嵌套方括号（标签已自带括号时不能重复包）', () => {
  const chain = buildMixChain({
    voiceLabel: '[voice]',
    bgmLabel: '[bgm]',
    outLabel: '[a]',
  }).join(';')
  assert.ok(!chain.includes('[['), `滤镜链里出现了 [[：${chain}`)
  assert.ok(!chain.includes(']]'), `滤镜链里出现了 ]]：${chain}`)
})

/**
 * 测某一个时间窗里的平均电平（dB）。
 *
 * lowpass 参数用来只留下某个频段：测配乐就带上低通把高频滤掉。
 */
function windowLevel(file, start, end, lowpassHz) {
  const r = spawnSync('ffmpeg', ['-hide_banner', '-nostats', '-i', file,
    '-af', `atrim=start=${start}:end=${end},lowpass=f=${lowpassHz},volumedetect`,
    '-f', 'null', '-'], { encoding: 'utf8' })
  const out = `${r.stderr || ''}\n${r.stdout || ''}`
  const m = [...out.matchAll(/mean_volume:\s+(-?[\d.]+) dB/g)].pop()
  return m ? Number(m[1]) : NaN
}

test('侧链让人声一开口就把配乐压下去，人声停了抬回来', () => {
  /*
   * 怎么把「配乐被压下去了」测准：
   *
   * 配乐用 80 Hz 正弦，人声用 1 kHz 正弦 —— 人声那一路有 110 Hz 高通，
   * 80 Hz 上几乎没有它的能量。所以 80 Hz 频段的电平变化就是配乐自己的变化，
   * 测的是配乐，不是「人声加配乐的混音」，也就不会被人声本身的变化污染。
   *
   * 人声只占前 3 秒，后面是静默：两段时间窗分别对应「人声在响」与「人声停了」。
   */
  const dir = mkdtempSync(join(tmpdir(), 'reel-duck-test-'))
  try {
    const bgm = join(dir, 'bgm.wav')
    const voice = join(dir, 'voice.wav')
    // 配乐：80 Hz，8 秒
    ff(['-v', 'error', '-y', '-f', 'lavfi', '-i', 'sine=frequency=80:duration=8:sample_rate=48000',
      '-ac', '2', bgm])
    // 人声：1 kHz，3 秒，后面补静音到 8 秒（两路时长要一样，amix 才会用第一条的时长）
    ff(['-v', 'error', '-y', '-f', 'lavfi', '-i', 'sine=frequency=1000:duration=3:sample_rate=48000',
      '-af', 'apad=whole_dur=8', '-ac', '2', voice])

    const measure = (duck) => {
      const out = join(dir, `mix-${duck}.wav`)
      const chain = buildMixChain({
        voiceLabel: '[0:a]', bgmLabel: '[1:a]', duck, normalize: false, outLabel: '[a]',
      }).join(';')
      ff(['-v', 'error', '-y', '-i', voice, '-i', bgm,
        '-filter_complex', chain, '-map', '[a]', '-c:a', 'pcm_s16le', out])
      // 人声在响的那一段 vs 人声停下的那一段
      return {
        speaking: windowLevel(out, 0.5, 2.5, 100),
        quiet: windowLevel(out, 4, 6, 100),
      }
    }

    const off = measure(false)
    const on = measure(true)

    // 不做侧链时，两段的配乐电平应当一样 —— 这是这把尺子的零点。
    assert.ok(Math.abs(off.speaking - off.quiet) < 1,
      `关掉侧链时两段电平应当接近，实际 ${off.speaking} / ${off.quiet}`)

    // 开了侧链，人声在响的那一段配乐必须明显更低。
    // 上限也要卡：压得太深音乐会整段消失（实测过 19 dB 的版本，配乐像被切掉）。
    const depth = on.quiet - on.speaking
    assert.ok(depth > 5, `侧链深度只有 ${depth.toFixed(1)} dB，应当明显压下去`)
    assert.ok(depth < 14, `侧链深度 ${depth.toFixed(1)} dB 太深，配音乐会整段消失`)
    assert.ok(on.quiet > on.speaking, '人声停下后配乐应当抬回来')
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('滤镜链能在真实 ffmpeg 上跑通（人声 + 配乐）', () => {
  const dir = mkdtempSync(join(tmpdir(), 'reel-mix-test-'))
  try {
    // 一段 3 秒的 440Hz 音当念白（比真念白简单，但足够验滤镜链语法与电平）
    const voice = join(dir, 'voice.wav')
    const bgm = join(dir, 'bgm.wav')
    const out = join(dir, 'out.wav')
    ff(['-y', '-v', 'error', '-f', 'lavfi', '-i', 'sine=frequency=440:duration=3',
      '-c:a', 'pcm_s16le', voice])
    ff(['-y', '-v', 'error', '-f', 'lavfi', '-i', 'sine=frequency=120:duration=3',
      '-c:a', 'pcm_s16le', bgm])

    const chain = buildMixChain({
      voiceLabel: '[0:a]',
      bgmLabel: '[1:a]',
      outLabel: '[a]',
    }).join(';')
    ff(['-y', '-v', 'error', '-i', voice, '-i', bgm,
      '-filter_complex', chain, '-map', '[a]', '-c:a', 'pcm_s16le', out])

    const { I, peak } = loudness(out)
    // 归一化后应该落在目标附近（loudnorm 单次扫描有零点几的误差）
    assert.ok(Math.abs(I - TARGET_LUFS) < 1.5, `响度 ${I} LUFS 偏离目标 ${TARGET_LUFS} 太多`)
    assert.ok(peak <= TARGET_TP + 0.5, `真峰 ${peak} dBFS 超过目标 ${TARGET_TP}`)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('无音频输入时输出等长静音', () => {
  const dir = mkdtempSync(join(tmpdir(), 'reel-mix-test-'))
  try {
    const out = join(dir, 'silence.wav')
    const chain = buildMixChain({ outLabel: '[a]' }).join(';')
    ff(['-y', '-v', 'error', '-filter_complex', chain, '-map', '[a]',
      '-t', '2', '-c:a', 'pcm_s16le', out])
    const dur = execFileSync('ffprobe', ['-v', 'error', '-show_entries', 'format=duration',
      '-of', 'csv=p=0', out], { encoding: 'utf8' }).trim()
    assert.ok(Math.abs(Number(dur) - 2) < 0.1, `静音时长 ${dur} 不是 2 秒`)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

test('normalize:false 时不做响度归一化', () => {
  const dir = mkdtempSync(join(tmpdir(), 'reel-mix-test-'))
  try {
    const voice = join(dir, 'voice.wav')
    const out = join(dir, 'out.wav')
    ff(['-y', '-v', 'error', '-f', 'lavfi', '-i', 'sine=frequency=440:duration=2',
      '-c:a', 'pcm_s16le', voice])
    const chain = buildMixChain({
      voiceLabel: '[0:a]',
      normalize: false,
      outLabel: '[a]',
    }).join(';')
    ff(['-y', '-v', 'error', '-i', voice, '-filter_complex', chain, '-map', '[a]',
      '-c:a', 'pcm_s16le', out])
    const { I } = loudness(out)
    // 没归一化就不该在目标响度上
    assert.ok(Math.abs(I - TARGET_LUFS) > 1.5, `未归一化却落在 ${I} LUFS，接近目标说明归一化没关掉`)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})

/**
 * 桌面版内置的是 FFmpeg 4.4，比本机系统版老。
 * 它的 sidechaincompress 要求两个输入格式完全一致，不显式 aformat 会直接报
 * "could not choose their formats" —— 这条链在系统 ffmpeg 上能过、在内置版上挂，
 * 属于只有打包后才会暴露的问题，所以单独钉一条。
 *
 * 内置二进制不在版本库里（resources/ 被 gitignore），所以找不到就跳过。
 * worktree 里没有 resources/，要顺着 git 的公共目录回到主检出找。
 */
function findBundledFfmpeg() {
  const roots = []
  const here = dirname(fileURLToPath(import.meta.url))
  roots.push(join(here, '..'))
  try {
    const common = execFileSync('git', ['rev-parse', '--path-format=absolute', '--git-common-dir'],
      { encoding: 'utf8', cwd: here }).trim()
    roots.push(join(common, '..'))
  } catch { /* 不是 git 仓就算了 */ }

  for (const root of roots) {
    const resources = join(root, 'apps', 'reel-studio', 'resources')
    if (!existsSync(resources)) continue
    for (const dir of readdirSync(resources)) {
      const bin = join(resources, dir, 'bin', 'ffmpeg')
      if (existsSync(bin)) return bin
    }
  }
  return null
}

const BUNDLED_FFMPEG = findBundledFfmpeg()

test('混音链在桌面版内置的 FFmpeg 4.4 上也能跑通', { skip: !BUNDLED_FFMPEG }, () => {
  const dir = mkdtempSync(join(tmpdir(), 'reel-mix-44-'))
  try {
    const voice = join(dir, 'voice.wav')
    const bgm = join(dir, 'bgm.wav')
    const out = join(dir, 'out.wav')
    execFileSync(BUNDLED_FFMPEG, ['-y', '-v', 'error', '-f', 'lavfi',
      '-i', 'sine=frequency=440:duration=3', '-c:a', 'pcm_s16le', voice])
    execFileSync(BUNDLED_FFMPEG, ['-y', '-v', 'error', '-f', 'lavfi',
      '-i', 'sine=frequency=120:duration=3', '-c:a', 'pcm_s16le', bgm])

    const chain = buildMixChain({
      voiceLabel: '[0:a]',
      bgmLabel: '[1:a]',
      outLabel: '[a]',
    }).join(';')
    execFileSync(BUNDLED_FFMPEG, ['-y', '-v', 'error', '-i', voice, '-i', bgm,
      '-filter_complex', chain, '-map', '[a]', '-c:a', 'pcm_s16le', out])

    const { I } = loudness(out)
    assert.ok(Math.abs(I - TARGET_LUFS) < 1.5, `内置 4.4 上响度 ${I} LUFS 偏离目标 ${TARGET_LUFS}`)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})
