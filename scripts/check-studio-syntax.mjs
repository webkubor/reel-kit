#!/usr/bin/env node
/**
 * 检查 studio/ 下的浏览器代码语法。
 *
 * 这些文件由浏览器直接加载，没有打包步骤 —— 语法错一个括号，
 * 页面白屏，控制台才报错。CI 里必须拦住。
 */
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'

const DIR = 'studio'
let fail = 0
for (const file of readdirSync(DIR).filter(f => f.endsWith('.js'))) {
  const path = join(DIR, file)
  try {
    execFileSync(process.execPath, ['--input-type=module', '--check'], { input: readFileSync(path, 'utf8'), stdio: ['pipe', 'pipe', 'pipe'] })
  } catch (error) {
    console.error(`❌ ${path} 语法错误\n${error.stderr?.toString() || error.message}`)
    fail = 1
  }
}
if (!fail) console.log(`✅ studio 语法通过（${readdirSync(DIR).filter(f => f.endsWith('.js')).length} 个文件）`)
process.exit(fail)
