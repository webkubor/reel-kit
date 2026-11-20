import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { synthesizeCaptions } from '../src/voice.mjs'

// museav 配音后端这条链路以前是直接抛错的（当时 CLI 里没有 speak 命令）。
// 现在它逐句调用 `museav speak`。这里用假的 museav 可执行文件验证：
// ① 每句一次调用、② 逐句产出 wav、③ 时长能读出来。
// 不打真实上游（要花钱、要 key），真链路另外手工验过。
//
// 假 museav 怎么造：Windows 不能直接 spawn 一个带 shebang 的 .mjs ——
// CreateProcess 只认 PE 可执行文件，Node 也不肯替 .cmd 之类的脚本开 shell。
// 所以改成「node 解释器 + 一个叫 speak.js 的入口」：
// MUSEA_BIN 指向 process.execPath，cwd 指到放假程序的目录，
// node 收到的第一个参数 `speak` 会被当成主入口去 cwd 里找 speak.js。
// macOS 与 Windows 走同一条路径，不用按平台分叉。

const work = join(tmpdir(), `reel-voice-museav-${process.pid}`)
const callsPath = join(work, 'calls.json')

// 假 museav 用 CommonJS 写：node 的主入口默认按 CJS 加载，
// 混进 ESM 语法会被 detect-module 翻成 ESM，行为就不确定了。
function writeFakeMuseav(dir, body) {
  mkdirSync(dir, { recursive: true })
  writeFileSync(join(dir, 'speak.js'), body)
}

const goodMuseav = `const { writeFileSync, appendFileSync } = require('node:fs')
const args = ['speak', ...process.argv.slice(2)]
const out = args[args.indexOf('--out') + 1]
const voice = args[args.indexOf('--voice') + 1]
appendFileSync(${JSON.stringify(callsPath)}, JSON.stringify({ args, out, voice }) + '\\n')
// 一个最小可用的 wav：44 字节头 + 一点 PCM
const header = Buffer.alloc(44)
header.write('RIFF', 0); header.writeUInt32LE(36 + 8, 4); header.write('WAVE', 8)
header.write('fmt ', 12); header.writeUInt32LE(16, 16); header.writeUInt16LE(1, 20)
header.writeUInt16LE(1, 22); header.writeUInt32LE(24000, 24); header.writeUInt32LE(48000, 28)
header.writeUInt16LE(2, 32); header.writeUInt16LE(16, 34)
header.write('data', 36); header.writeUInt32LE(8, 40)
writeFileSync(out, Buffer.concat([header, Buffer.alloc(8)]))
process.stdout.write(out + '\\n')
`

const badMuseav = `process.stderr.write("error: unknown command 'speak'\\n")
process.exit(1)
`

// 在指定目录里跑假 museav：改 cwd 让 node 能找到 speak.js，
// 再把 MUSEA_BIN 指到 node 解释器。跑完必须把 cwd 还原 ——
// 同一个文件里还有要读仓库内相对路径的用例。
async function withFakeMuseav(dir, fn) {
  const oldBin = process.env.MUSEA_BIN
  const oldCwd = process.cwd()
  process.env.MUSEA_BIN = process.execPath
  process.chdir(dir)
  try {
    return await fn()
  } finally {
    process.chdir(oldCwd)
    if (oldBin === undefined) delete process.env.MUSEA_BIN
    else process.env.MUSEA_BIN = oldBin
  }
}

test('museav 后端：逐句调用 museav speak 并产出 wav', async () => {
  const dir = join(work, 'ok')
  writeFakeMuseav(dir, goodMuseav)
  rmSync(callsPath, { force: true })
  const outDir = join(work, 'voice')
  try {
    const results = await withFakeMuseav(dir, () => synthesizeCaptions({
      captions: ['第一句。', '第二句。'],
      outDir,
      engine: 'museav',
      voice: 'male-qn-qingse',
    }))
    assert.equal(results.length, 2)
    for (const r of results) {
      const head = readFileSync(r.file).subarray(0, 12)
      assert.equal(head.subarray(0, 4).toString(), 'RIFF')
      assert.equal(head.subarray(8, 12).toString(), 'WAVE')
      assert.ok(r.duration > 0)
    }
    const calls = readFileSync(callsPath, 'utf8').trim().split('\n').map(JSON.parse)
    assert.equal(calls.length, 2)
    assert.equal(calls[0].args[0], 'speak')
    assert.equal(calls[0].voice, 'male-qn-qingse')
    assert.ok(calls[0].args.includes('--format'))
    assert.equal(calls[0].args[calls[0].args.indexOf('--format') + 1], 'wav')
    // 每句一次调用，产物就是 --out 指的那个文件
    assert.ok(calls[0].out.endsWith('voice_0000.wav'))
    assert.ok(calls[1].out.endsWith('voice_0001.wav'))
  } finally {
    rmSync(work, { recursive: true, force: true })
  }
})

test('museav speak 报错时，错误信息指到具体句子并给出版本要求', async () => {
  const dir = join(work, 'bad')
  writeFakeMuseav(dir, badMuseav)
  try {
    await withFakeMuseav(dir, () => assert.rejects(
      () => synthesizeCaptions({ captions: ['会失败的一句。'], outDir: join(work, 'v2'), engine: 'museav' }),
      /第 1 句配音失败[\s\S]*3\.17\.0/,
    ))
  } finally {
    rmSync(work, { recursive: true, force: true })
  }
})

test('CLI 帮助里列出 museav 配音后端', () => {
  const help = execFileSync('node', ['bin/reel.mjs', 'make', '--help'], { encoding: 'utf8' })
  assert.match(help, /--voice-engine[\s\S]*museav/)
})
