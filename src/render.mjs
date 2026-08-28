/**
 * 把「模板 + 每句文案 + 每张素材」渲染成逐帧 PNG。
 *
 * 为什么用浏览器渲染而不是 ffmpeg 的 drawtext：
 *   drawtext 要手写中文字体路径、描边、换行、居中，圆弧装饰更是画不出来；
 *   改一次版式要重调一串滤镜参数。HTML 里这些都是几行 CSS，而且改完能直接看。
 *   代价是每帧要跑一次截图 —— 但一支片子只有 8~20 帧，几秒的事。
 */

import { chromium } from 'playwright-core'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, extname, join, resolve } from 'node:path'

/** 本机 Chrome，避免为这个工具再下一份 chromium */
const CHROME_CANDIDATES = [
  process.env.CHROME_PATH,
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/Applications/Chromium.app/Contents/MacOS/Chromium',
  '/Applications/ego lite.app/Contents/MacOS/ego lite',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
].filter(Boolean)

export function findChrome() {
  for (const p of CHROME_CANDIDATES) if (existsSync(p)) return p
  return null
}

/** 极简占位符替换。刻意不引模板引擎 —— 只有 {{key}} 一种语法，够用。 */
function fill(tpl, vars) {
  return tpl.replace(/\{\{(\w+)\}\}/g, (_, k) => {
    const v = vars[k]
    return v == null ? '' : String(v)
  })
}

/** 图片转 data URI —— 让 HTML 完全自包含，避免 file:// 的相对路径与权限问题 */
function toDataUri(file) {
  const ext = extname(file).toLowerCase()
  const mime = { '.png': 'image/png', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg', '.webp': 'image/webp', '.gif': 'image/gif' }[ext]
  if (!mime) throw new Error(`不支持的图片格式: ${file}`)
  return `data:${mime};base64,${readFileSync(file).toString('base64')}`
}

/**
 * @param {object} opts
 * @param {string} opts.template  模板 HTML 路径
 * @param {Array<{image:string, caption:string}>} opts.shots  逐镜内容
 * @param {object} opts.vars      模板级变量（title/subtitle/footer/bg/accent1/accent2）
 * @param {string} opts.outDir    帧输出目录
 * @param {number} [opts.width]   画布宽（默认 1080）
 * @param {number} [opts.height]  画布高（默认 1920）
 * @returns {Promise<string[]>}   生成的 PNG 路径，按顺序
 */
export async function renderFrames({ template, shots, vars, outDir, width = 1080, height = 1920 }) {
  const chrome = findChrome()
  if (!chrome) throw new Error('找不到本机 Chrome，设 CHROME_PATH 指向可执行文件')

  const tpl = readFileSync(template, 'utf-8')
  mkdirSync(outDir, { recursive: true })

  const browser = await chromium.launch({ executablePath: chrome, headless: true })
  // deviceScaleFactor 保持 1：画布本身就是 1080×1920 目标分辨率，
  // 放大截图再让 ffmpeg 缩回去只会多一次重采样，白白糊掉文字边缘。
  const ctx = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: 1 })
  const page = await ctx.newPage()

  const frames = []
  try {
    for (let i = 0; i < shots.length; i++) {
      const shot = shots[i]
      const html = fill(tpl, {
        ...vars,
        caption: shot.caption ?? '',
        image: shot.image ? toDataUri(shot.image) : '',
      })
      await page.setContent(html, { waitUntil: 'load' })
      // 等字体与图片解码落定，否则偶发截到文字未上屏的空帧
      await page.evaluate(() => document.fonts.ready)
      await page.waitForTimeout(120)

      const out = join(outDir, `frame_${String(i).padStart(4, '0')}.png`)
      await page.screenshot({ path: out, type: 'png' })
      frames.push(out)
    }
  } finally {
    await browser.close()
  }
  return frames
}
