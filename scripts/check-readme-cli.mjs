import { readFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

// 只有调用本 CLI 的 shell 示例才定义了对外承诺的选项。
// CSS 自定义属性和其他程序的参数各有各的契约，不在这里校验。
//
// `npx github:webkubor/reel-kit` 也是官方推荐的一次性用法，
// 不收进正则的话这些示例里的参数就等于没被校验过。
const CLI_COMMAND =
  /^\s*(?:reel\b|npx\s+(?:@kubor\/reel-kit|reel|github:webkubor\/reel-kit)\b|node\s+(?:\.\/)?bin\/reel\.mjs\b)/

export function documentedOptions(markdown) {
  const options = new Set()
  for (const block of markdown.matchAll(/^```(?:bash|sh|shell)\s*\n([\s\S]*?)^```/gm)) {
    for (const line of block[1].replace(/\\\r?\n/g, ' ').split('\n')) {
      if (!CLI_COMMAND.test(line)) continue
      for (const flag of line.matchAll(/(?:^|\s)(--[a-z][a-z-]*)(?=[\s=]|$)/g)) options.add(flag[1])
    }
  }
  return [...options].sort()
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const root = new URL('../', import.meta.url)
  const help = execFileSync(process.execPath, [fileURLToPath(new URL('bin/reel.mjs', root)), 'make', '--help'], { encoding: 'utf8' })
  const supported = new Set(help.match(/--[a-z][a-z-]*/g))
  // CLI 的参数表从 README 挪到了 docs/cli.md，两边都要校验，
  // 免得「搬走了」变成「没人看了」。
  const docs = ['README.md', 'docs/cli.md']
  let total = 0
  for (const doc of docs) {
    const options = documentedOptions(readFileSync(new URL(doc, root), 'utf8'))
    if (!options.length) continue
    total += options.length
    const missing = options.filter(option => !supported.has(option))
    if (missing.length) throw new Error(`${doc} documents unsupported reel options: ${missing.join(', ')}`)
  }
  if (!total) throw new Error('No reel CLI examples found in README or docs/cli.md')
  console.log(`README CLI check passed: ${total} options`)
}
