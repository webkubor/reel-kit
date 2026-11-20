// 项目演示视频素材批量截图
//
// 用法：
//   1. 复制 frames/ 下的模板，改成你这个项目的内容（每个模板一个镜头）
//   2. 改下面的 frames 数组 —— 顺序必须与 captions.txt 的句子一一对应
//   3. 改 outputDir / filePrefix
//   4. bun run shoot.mjs
//
// 截图用本机已有的 chromium（muse·tv 装过的那份），不额外下载浏览器。
// 可执行文件在 playwright 缓存里按版本目录存放，这里动态解析，不写死版本号；
// 素材目录也从本文件位置派生，仓库改名 / 换机都不用改。
import { chromium } from 'playwright-core'
import { existsSync, mkdirSync, readdirSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))

// playwright 缓存根目录：macOS 与 Linux 各一份。
const CACHE_ROOTS = [
  join(homedir(), 'Library/Caches/ms-playwright'),
  join(homedir(), '.cache/ms-playwright'),
]

function cachedChromiumDirs() {
  const dirs = []
  for (const root of CACHE_ROOTS) {
    if (!existsSync(root)) continue
    for (const name of readdirSync(root)) {
      const matched = name.match(/^chromium(?:_headless_shell)?-(\d+)$/)
      if (!matched) continue
      dirs.push({ root, name, revision: Number(matched[1]) })
    }
  }
  return dirs.sort((a, b) => b.revision - a.revision)
}

// 优先用完整版 Chrome for Testing，其次 headless shell，最后 Linux 版 chrome。
function resolveChrome() {
  const dirs = cachedChromiumDirs()
  const candidates = [
    ...dirs.map(dir => join(dir.root, dir.name, 'chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing')),
    ...dirs.map(dir => join(dir.root, dir.name, 'chrome-headless-shell-mac-arm64/chrome-headless-shell')),
    ...dirs.map(dir => join(dir.root, dir.name, 'chrome-linux/chrome')),
  ]
  const hit = candidates.find(candidate => existsSync(candidate))
  if (!hit) {
    throw new Error('找不到 chromium 可执行文件：用 REEL_CHROME 指定，或先 npx playwright install chromium')
  }
  return hit
}

const CHROME = process.env.REEL_CHROME || resolveChrome()

const frames = [
  'terminal-frame',      // ① 问题：明文出现在哪
  'terminal-frame-2',    // ② 解决：怎么处理的
  'terminal-frame-3',    // ③ 结果：达到什么效果
  'feature-cards',       // ④ 特性三连
  'terminal-frame-list', // ⑤ 清单/命令一览
  'logo-endcard',        // ⑥ 收尾：logo + 链接
]

const outputDir = join(here, 'shots')
const framesDir = join(here, 'frames')
const filePrefix = 'shot'

mkdirSync(outputDir, { recursive: true })

const browser = await chromium.launch({ executablePath: CHROME })
const page = await browser.newPage({ viewport: { width: 1000, height: 1000 }, deviceScaleFactor: 2 })
for (let i = 0; i < frames.length; i++) {
  await page.goto(pathToFileURL(join(framesDir, `${frames[i]}.html`)).href)
  await page.waitForTimeout(250)
  const name = `${filePrefix}${String(i + 1).padStart(2, '0')}.png`
  await page.screenshot({ path: `${outputDir}/${name}` })
  console.log(`✓ ${name} ← ${frames[i]}`)
}
await browser.close()
console.log(`\n出 ${frames.length} 张素材，配音台词见 captions.example.txt`)
