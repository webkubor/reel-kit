import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { chmodSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { synthesizeCaptions } from '../src/voice.mjs'

// museav 配音后端这条链路以前是直接抛错的（当时 CLI 里没有 speak 命令）。
// 现在它逐句调用 `museav speak`。这里用假的 museav 可执行文件验证：
// ① 每句一次调用、② 逐句产出 wav、③ 时长能读出来。
// 不打真实上游（要花钱、要 key），真链路另外手工验过。

const work = join(tmpdir(), `reel-voice-museav-${process.pid}`)
const fakeBin = join(work, 'fake-museav.mjs')
const calls = join(work, 'calls.json')

function writeFakeMuseav() {
  mkdirSync(work, { recursive: true })
  const script = `#!/usr/bin/env node
import { writeFileSync, appendFileSync } from 'node:fs'
const args = process.argv.slice(2)
const out = args[args.indexOf('--out') + 1]
const voice = args[args.indexOf('--voice') + 1]
appendFileSync(${JSON.stringify(calls)}, JSON.stringify({ args, out, voice }) + '\\n')
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
  writeFileSync(fakeBin, script)
  chmodSync(fakeBin, 0o755)
}

test('museav 后端：逐句调用 museav speak 并产出 wav', async () => {
  writeFakeMuseav()
  rmSync(calls, { force: true })
  const outDir = join(work, 'voice')
  const oldBin = process.env.MUSEA_BIN
  process.env.MUSEA_BIN = fakeBin
  try {
    const results = await synthesizeCaptions({
      captions: ['第一句。', '第二句。'],
      outDir,
      engine: 'museav',
      voice: 'male-qn-qingse',
    })
    assert.equal(results.length, 2)
    for (const r of results) {
      const head = readFileSync(r.file).subarray(0, 12)
      assert.equal(head.subarray(0, 4).toString(), 'RIFF')
      assert.equal(head.subarray(8, 12).toString(), 'WAVE')
      assert.ok(r.duration > 0)
    }
    const lines = readFileSync(calls, 'utf8').trim().split('\n').map(JSON.parse)
    assert.equal(lines.length, 2)
    assert.equal(lines[0].args[0], 'speak')
    assert.equal(lines[0].voice, 'male-qn-qingse')
    assert.ok(lines[0].args.includes('--format'))
    assert.equal(lines[0].args[lines[0].args.indexOf('--format') + 1], 'wav')
  } finally {
    if (oldBin === undefined) delete process.env.MUSEA_BIN
    else process.env.MUSEA_BIN = oldBin
    rmSync(work, { recursive: true, force: true })
  }
})

test('museav speak 报错时，错误信息指到具体句子并给出版本要求', async () => {
  const badBin = join(work, 'bad-museav.mjs')
  mkdirSync(work, { recursive: true })
  writeFileSync(badBin, '#!/usr/bin/env node\nprocess.stderr.write("error: unknown command \'speak\'\\n");process.exit(1)\n')
  chmodSync(badBin, 0o755)
  const oldBin = process.env.MUSEA_BIN
  process.env.MUSEA_BIN = badBin
  try {
    await assert.rejects(
      () => synthesizeCaptions({ captions: ['会失败的一句。'], outDir: join(work, 'v2'), engine: 'museav' }),
      /第 1 句配音失败[\s\S]*3\.17\.0/,
    )
  } finally {
    if (oldBin === undefined) delete process.env.MUSEA_BIN
    else process.env.MUSEA_BIN = oldBin
    rmSync(work, { recursive: true, force: true })
  }
})

test('CLI 帮助里列出 museav 配音后端', () => {
  const help = execFileSync('node', ['bin/reel.mjs', 'make', '--help'], { encoding: 'utf8' })
  assert.match(help, /--voice-engine[\s\S]*museav/)
})
