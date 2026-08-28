# AGENTS.md — reel-kit 仓库级行为准则

## 这是什么

竖版短视频合成 CLI。**素材 + 逐句文案 + 配音/BGM → mp4**。
定位是「AI 无人值守代劳」那条线：一切参数化、可脚本驱动、不需要 GUI。

## 改代码前必须知道的三条

### 1. concat 列表的最后一帧必须重复一次

`src/compose.mjs` 的 `buildConcatList()` 末尾那行重复**不是笔误**。
ffmpeg 的 concat demuxer 会忽略最后一个 `duration`，不重复的话末镜只有 1 帧。

### 2. 音画对齐靠 apad，不要改成算 offset

每句念白后补「镜头时长 − 念白时长」的静音（`apad=whole_dur=<镜头时长>`），
再首尾相接。这样**不需要计算任何偏移量，也就不会有累积误差**。
改成 `adelay` 之类按 offset 摆放的写法，镜头一多必然漂。

### 3. 时长余量不能省

`--voice-margin`（默认 0.45s）看起来像保守值，其实是必需的：
画面做到「刚好等于念白」时，帧率舍入会让画面略短于音频，每句最后半个字被吃掉。
来源见 `docs/video-compositing-notes.md` 第 4 条。

## 加模板

往 `templates/` 丢一个 HTML，占位符 `{{title}}` `{{subtitle}}` `{{footer}}`
`{{caption}}` `{{image}}` `{{bg}}` `{{accent1}}` `{{accent2}}`。**不用改代码。**

画布尺寸写死在 CSS 里（body 的 width/height），与 `--size` 保持一致。
图片会被转成 data URI 注入，所以模板必须自包含，不要引外部资源。

## 配音后端

| 后端 | 何时用 |
|---|---|
| `voxcraft`（默认） | 批量。本地 Qwen3-TTS，一次性 4.2GB 模型，之后免费 |
| `museav` | 临时试验，或本地模型未就绪时兜底。按 token 计费 |

**voxcraft 的 `voice` 命令在它自己的 venv 里，默认不在 PATH。**
`src/voxcraft-locate.mjs` 负责定位并按完整度打分（能跑 > 有模型 > 有音色），
挑最完整的一份。**不要改成只判断 `which voice`** —— 那会误判成没装，
进而重复下载 4.2GB 模型（这个错真的犯过）。

**voxcraft 必须走 web 服务模式**（`src/voxcraft-server.mjs`）。
它的 CLI 每次调用都重载模型，逐句起进程做 8 句就是加载 8 次，
实测第 5 句崩在 `libc++abi: recursive_mutex lock failed`。

## 验证改动

```bash
node --check bin/reel.mjs src/*.mjs        # 语法
pnpm demo                                  # 用 examples/ 跑一支，应无报错
scorecard webkubor/reel-kit --min 5        # 仓库质检闸门
```

改了合成逻辑后，**验证数据必须来自被验证的那次运行** ——
拿上一轮的时长去切新生成的视频，会得出完全错误的结论（这个坑踩过）。

## 不做什么

- **不做抠图**：那是 `museav remove-bg` 与 `wechat-sticker-submit` skill 的职责。
- **不做通用剪辑**：调色、多轨、转场请用 DaVinci Resolve。
- **不自动装大件**：缺 voxcraft 或模型时明确报出并给命令，
  但绝不自动下载 4.2GB 的东西。
