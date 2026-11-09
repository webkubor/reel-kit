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

/** 极简占位符替换。刻意不引模板引擎 —— 只有 {{key}} 一种语法,够用。 */
export function fill(tpl, vars) {
  return tpl.replace(/\{\{(\w+)\}\}/g, (_, k) => {
    const v = vars[k]
    return v == null ? '' : String(v)
  })
}

/**
 * 组装一帧的模板变量。
 *
 * 画布尺寸必须在这里补进去：模板写 `width:{{width}}px` 而调用方没传 width 时，
 * 占位符会被替换成空串 → `width:px` 是非法声明 → body 高度塌成 0 → 整帧黑屏。
 * 调用方显式给了 width/height 就以调用方为准（模板尺寸与请求尺寸冲突时不该被静默改写）。
 */
export function frameVars(vars = {}, size = {}, shot = {}) {
  return {
    ...vars,
    // 单镜可以覆盖模板级变量：比如每一镜的字幕字号按长度单独定，
    // 长的那句小一号，短的那句大一号，不必整条片子一个字号。
    ...(shot.vars || {}),
    width: vars.width ?? size.width,
    height: vars.height ?? size.height,
    caption: shot.caption ?? '',
    image: shot.image ?? '',
  }
}

/**
 * 从模板里读出 body 的 width/height,作为 viewport 尺寸。
 * 模板作者把画布尺寸写在 body 的 CSS 里(AGENTS.md 强制约定),这里就跟着走,
 * 无需每个模板单独配 --size。读不到时退回参数默认值。
 *
 * 只在 body 自己的规则里找尺寸：早先的整段匹配会跨到后面的规则,
 * 模板里任意一个 `width:8px` 之类的小尺寸就把画布算成 8×8。
 *
 * body 里写 `width:{{width}}px` 这种占位符时读不到数字 —— 这不是错误,
 * 它表示「画布尺寸随调用参数走」,读不到就退回参数默认值,
 * 再由 renderFrames 把最终尺寸回填进占位符。
 */
export function readTemplateSize(tpl) {
  const body = tpl.match(/body\s*\{([^}]*)\}/)
  if (!body) return null
  const width = body[1].match(/width:\s*(\d+)px/)
  const height = body[1].match(/height:\s*(\d+)px/)
  if (!width || !height) return null
  return { width: Number(width[1]), height: Number(height[1]) }
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
  if (!chrome) throw new Error('找不到本机 Chrome,设 CHROME_PATH 指向可执行文件')

  const tpl = readFileSync(template, 'utf-8')
  // 画布尺寸优先从模板 body CSS 读,读不到再用参数默认值
  const size = readTemplateSize(tpl) || { width, height }
  mkdirSync(outDir, { recursive: true })

  const browser = await chromium.launch({ executablePath: chrome, headless: true })
  // deviceScaleFactor 保持 1：画布本身就是 1080×1920 目标分辨率,
  // 放大截图再让 ffmpeg 缩回去只会多一次重采样,白白糊掉文字边缘。
  const ctx = await browser.newContext({ viewport: { width: size.width, height: size.height }, deviceScaleFactor: 1 })
  const page = await ctx.newPage()

  const frames = []
  try {
    for (let i = 0; i < shots.length; i++) {
      const shot = shots[i]
      // 画布尺寸回填进模板占位符：模板写 `width:{{width}}px` 时，
      // 不回填会渲染成 `width:px`（非法声明）→ body 高度塌成 0 → 全屏黑。
      const html = fill(tpl, frameVars(vars, size, {
        caption: shot.caption,
        image: shot.image ? toDataUri(shot.image) : '',
        vars: shot.vars,
      }))
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
