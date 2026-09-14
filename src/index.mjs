/**
 * @kubor/reel-kit 的库入口。
 *
 * CLI 用法见 bin/reel.mjs(默认 `reel` 命令)
 * 库用法:
 *   import { renderFrames, compose, ... } from '@kubor/reel-kit'
 *
 * 设计原则:不引副作用,所有导出都是 async 函数;保持 thin wrapper,
 * 真正的合成逻辑仍按模块拆分(render/compose/voice/...)。
 *
 * 注意:`probeDuration` 在 compose.mjs 和 voice.mjs 各有一个,实现都是
 * `ffprobe format=duration`,这里只 re-export compose 的,避免重名。
 * voice.mjs 内部继续用自己那个(避免改它的内部 import 链)。
 */

// 渲染:浏览器 → 每镜 PNG
export { renderFrames, findChrome } from './render.mjs'

// 合成:PNG + 配音 + BGM → mp4
export { compose, probeDuration } from './compose.mjs'

// 配音:合成 + 切分
export {
  synthesizeCaptions,
  durationsFromVoice,
  splitVoiceByChars,
} from './voice.mjs'

// 配乐库:本地文件 / 别名解析
export { resolveBgm, listBgm, NOT_CONFIGURED } from './bgm-library.mjs'

// voxcraft(本地 Qwen3-TTS)定位
export { locateVoxcraft, voxcraftHint } from './voxcraft-locate.mjs'
