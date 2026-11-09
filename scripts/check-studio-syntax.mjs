#!/usr/bin/env node
/**
 * 检查 studio/ 下的浏览器代码，以及 src/studio/ 下的服务端模块语法。
 *
 * 这些文件由浏览器直接加载或被服务端直接 import，没有打包步骤 —— 语法错一个括号，
 * 页面白屏或服务起不来，控制台才报错。CI 里必须拦住。
 */
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'

const DIRS = ['studio', 'src/studio']
let fail = 0
let checked = 0
for (const DIR of DIRS) {
  for (const file of readdirSync(DIR).filter(f => f.endsWith('.js') || f.endsWith('.mjs'))) {
    const path = join(DIR, file)
    checked++
    try {
      execFileSync(process.execPath, ['--input-type=module', '--check'], { input: readFileSync(path, 'utf8'), stdio: ['pipe', 'pipe', 'pipe'] })
    } catch (error) {
      console.error(`❌ ${path} 语法错误\n${error.stderr?.toString() || error.message}`)
      fail = 1
    }
  }
}
if (!fail) console.log(`✅ studio 语法通过（${checked} 个文件）`)
process.exit(fail)
