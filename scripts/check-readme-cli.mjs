import { readFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

// Only shell examples invoking this CLI define its documented options.
// CSS custom properties and other programs' flags have separate contracts.
export function documentedOptions(markdown) {
  const options = new Set()
  for (const block of markdown.matchAll(/^```(?:bash|sh|shell)\s*\n([\s\S]*?)^```/gm)) {
    for (const line of block[1].replace(/\\\r?\n/g, ' ').split('\n')) {
      if (!/^\s*(?:reel\b|npx\s+(?:@kubor\/reel-kit|reel)\b|node\s+(?:\.\/)?bin\/reel\.mjs\b)/.test(line)) continue
      for (const flag of line.matchAll(/(?:^|\s)(--[a-z][a-z-]*)(?=[\s=]|$)/g)) options.add(flag[1])
    }
  }
  return [...options].sort()
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const root = new URL('../', import.meta.url)
  const help = execFileSync(process.execPath, [fileURLToPath(new URL('bin/reel.mjs', root)), 'make', '--help'], { encoding: 'utf8' })
  const supported = new Set(help.match(/--[a-z][a-z-]*/g))
  const options = documentedOptions(readFileSync(new URL('README.md', root), 'utf8'))
  if (!options.length) throw new Error('No reel CLI examples found in README')
  const missing = options.filter(option => !supported.has(option))
  if (missing.length) throw new Error(`README documents unsupported reel options: ${missing.join(', ')}`)
  console.log(`README CLI check passed: ${options.length} options`)
}
