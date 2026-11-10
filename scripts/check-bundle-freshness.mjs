#!/usr/bin/env node
// 检查打应用包用的后端快照是不是最新的。
//
// 为什么要有这个门禁：
//   桌面包里的后端是「发版那一刻的源码快照」，由 scripts/prepare-resources.sh 复制。
//   快照不会自动跟着仓库更新 —— 改了 src/studio/*.mjs 却没重新跑那个脚本，
//   包里就是旧后端：界面上多了一条流水线，点下去后端却说不支持这个任务类型。
//   这种问题在本地跑源码永远看不出来，只有装上安装包的用户会遇到。
//
// 所以这里比对「仓库里的后端源码」与「各个平台资源目录里的后端源码」：
// 只要有一处不一致就报错，并指出该重跑哪条命令。

import { readFileSync, existsSync, readdirSync } from 'node:fs'
import { join, relative, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const platforms = ['darwin-arm64', 'darwin-amd64', 'windows-amd64']

// 仓库里的哪些文件会进包，以及它们在包里对应的位置。
const pairs = []
const add = (repoPath, bundlePath) => pairs.push([repoPath, bundlePath])
for (const name of readdirSync(join(root, 'src/studio'))) {
  if (name.endsWith('.mjs')) add(join('src/studio', name), join('src/studio', name))
}
for (const name of ['render.mjs', 'compose.mjs', 'voice.mjs', 'bgm-library.mjs', 'knowledge-styles.mjs', 'audio-mix.mjs', 'settings.mjs']) {
  if (existsSync(join(root, 'src', name))) add(join('src', name), join('src', name))
}
for (const name of readdirSync(join(root, 'templates'))) {
  if (name.endsWith('.html')) add(join('templates', name), join('templates', name))
}
add('bin/reel.mjs', 'bin/reel.mjs')

let bad = 0
for (const platform of platforms) {
  const bundle = join(root, 'apps/reel-studio/resources', platform, 'backend')
  if (!existsSync(bundle)) continue
  for (const [repoPath, bundlePath] of pairs) {
    const src = join(root, repoPath)
    const dst = join(bundle, bundlePath)
    if (!existsSync(dst)) {
      console.log(`❌ ${platform}：包里的后端少了 ${bundlePath}`)
      bad++
      continue
    }
    if (readFileSync(src, 'utf8') !== readFileSync(dst, 'utf8')) {
      console.log(`❌ ${platform}：包里的后端与仓库不一致 —— ${bundlePath}`)
      bad++
    }
  }
}

if (bad) {
  console.log('\n发版前先刷新打包资源：')
  for (const platform of platforms) {
    console.log(`  bash scripts/prepare-resources.sh ${platform}`)
  }
  process.exit(1)
}
console.log(`✅ 打包用的后端快照是最新的（${pairs.length} 个文件 × ${platforms.length} 个平台）`)
