import test from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { compose } from '../src/compose.mjs'

test('single and multiple stills preserve their requested durations', async t => {
  const dir = mkdtempSync(join(tmpdir(), 'reel-compose-test-'))
  t.after(() => rmSync(dir, { recursive: true, force: true }))
  const frame = join(dir, 'frame.png')
  execFileSync('ffmpeg', ['-v', 'error', '-f', 'lavfi', '-i', 'color=red:s=64x64', '-frames:v', '1', '-update', '1', frame])
  for (const durations of [[2], [1, 2]]) {
    const output = join(dir, `${durations.length}.mp4`)
    await compose({ frames: durations.map(() => frame), durations, out: output })
    const actual = Number(execFileSync('ffprobe', ['-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', output], { encoding: 'utf8' }))
    assert.ok(Math.abs(actual - durations.reduce((a, b) => a + b, 0)) < 0.15, `unexpected duration: ${actual}`)
  }
})
