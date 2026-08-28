#!/usr/bin/env node
/**
 * 轻量 lint —— 不引 ESLint 整套，只查几条真会出事的。
 *
 * 为什么不上 ESLint：这个项目只有 6 个源文件、无框架、无 TS。
 * 引一套配置 + 插件生态，维护成本大于收益。这里查的三条都是
 * 「混进发布会真的出问题」的，不是风格偏好。
 */
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

const FILES = [
  'bin/reel.mjs',
  ...readdirSync('src').filter(f => f.endsWith('.mjs')).map(f => join('src', f)),
]

const RULES = [
  { re: /^\s*debugger\b/m, msg: 'debugger 语句' },
  { re: /\bconsole\.log\(\s*['"`]?(test|debug|xxx|aaa)/i, msg: '调试用的 console.log' },
  // 硬编码本机路径：换台机器直接失效，而且会泄漏用户名
  { re: /['"`]\/Users\/[a-z]/i, msg: '硬编码 /Users/ 绝对路径' },
]

let fail = 0
for (const f of FILES) {
  const src = readFileSync(f, 'utf-8')
  for (const { re, msg } of RULES) {
    const m = src.match(re)
    if (m) {
      const line = src.slice(0, m.index).split('\n').length
      console.error(`❌ ${f}:${line}  ${msg}`)
      fail = 1
    }
  }
}
console.log(fail ? '' : `✅ lint 通过（${FILES.length} 个文件）`)
process.exit(fail)
