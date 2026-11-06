// 项目演示视频素材批量截图
//
// 用法：
//   1. 复制 frames/ 下的模板，改成你这个项目的内容（每个模板一个镜头）
//   2. 改下面的 frames 数组 —— 顺序必须与 captions.txt 的句子一一对应
//   3. 改 outputDir / filePrefix
//   4. bun run shoot.mjs
//
// 截图用本机已有的 chromium（reel-kit 装过的那份），不额外下载浏览器。
import { chromium } from '/Users/webkubor/dev/video/reel-kit/node_modules/.pnpm/playwright-core@1.62.1/node_modules/playwright-core/index.mjs'

const CHROME = '/Users/webkubor/Library/Caches/ms-playwright/chromium-1243/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing'

const frames = [
  'terminal-frame',      // ① 问题：明文出现在哪
  'terminal-frame-2',    // ② 解决：怎么处理的
  'terminal-frame-3',    // ③ 结果：达到什么效果
  'feature-cards',       // ④ 特性三连
  'terminal-frame-list', // ⑤ 清单/命令一览
  'logo-endcard',        // ⑥ 收尾：logo + 链接
]

const outputDir = '/Users/webkubor/dev/video/project-demo-kit/shots'
const filePrefix = 'shot'

const browser = await chromium.launch({ executablePath: CHROME })
const page = await browser.newPage({ viewport: { width: 1000, height: 1000 }, deviceScaleFactor: 2 })
for (let i = 0; i < frames.length; i++) {
  await page.goto(`file:///Users/webkubor/dev/video/project-demo-kit/frames/${frames[i]}.html`)
  await page.waitForTimeout(250)
  const name = `${filePrefix}${String(i + 1).padStart(2, '0')}.png`
  await page.screenshot({ path: `${outputDir}/${name}` })
  console.log(`✓ ${name} ← ${frames[i]}`)
}
await browser.close()
console.log(`\n出 ${frames.length} 张素材，配音台词见 captions.example.txt`)
