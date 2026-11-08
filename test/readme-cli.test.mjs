import test from 'node:test'
import assert from 'node:assert/strict'
import { documentedOptions } from '../scripts/check-readme-cli.mjs'

test('README checker ignores other tools and CSS, but preserves real reel options', () => {
  const readme = [
    'CSS uses --ink and --brand.',
    '```css', ':root { --line: red; }', '```',
    '```bash',
    'kyvault run --env KEY=ref -- reel studio',
    'reel make --template demo \\',
    '  --out test.mp4 --unsupported value',
    'node bin/reel.mjs make --caps captions.txt',
    '```',
  ].join('\n')
  assert.deepEqual(documentedOptions(readme), ['--caps', '--out', '--template', '--unsupported'])
})
