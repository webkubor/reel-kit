# reel-kit

竖版短视频合成工作台 —— **素材 + 逐句文案 + BGM，套模板出片**。

```bash
reel make --template sticker-promo \
  --title "莓啾日常" --subtitle "莓啾" \
  --footer "微信搜「莓啾日常」添加整套" \
  --assets ./stickers --caps caps-莓啾日常.txt \
  --bgm 配乐候选/682.mp3 --per-shot 2.5 \
  --out 推广视频-莓啾日常.mp4
```

## 为什么有它

表情包推广、产品短视频这类内容，此前每次都是临时拼一串 ffmpeg 命令。
**成品留下了，流程没留下** —— 目录里只有 mp4、素材和 `caps-*.txt`，
合成逻辑用完就丢，下次做同类片子得从头再拼一遍。

这个 CLI 把那段流程固化。版式从已发布成片反推，参数化后可复用。

## 装

```bash
cd reel-kit && pnpm install
npm link          # 或直接 node bin/reel.mjs
```

依赖：**ffmpeg**（合成）、**本机 Chrome**（渲染版式，不额外下 chromium）。
Chrome 路径可用 `CHROME_PATH` 覆盖。

## 设计取舍

### 为什么用浏览器渲染版式，不用 ffmpeg 的 drawtext

`drawtext` 要手写中文字体路径、描边、换行、居中，圆弧装饰更是画不出来；
改一次版式得重调一串滤镜参数，且改完看不见效果。HTML 里这些都是几行 CSS。

代价是每帧跑一次截图 —— 但一支片子只有 8~20 帧，几秒的事。

### 为什么用 concat demuxer，不用 `-framerate 1/2.5`

后者要求每镜时长完全一致。而「片尾多停一会」「某句字多停半秒」是很自然的需求，
concat 列表里每帧各带各的 duration，想改哪镜改哪镜。

> ⚠️ ffmpeg 的 concat 列表**必须把最后一帧重复一次**，否则它的 duration 会被忽略。
> `src/compose.mjs` 里已处理，改那段时别删掉。

### 素材与文案的配对

按顺序一一对应。数量不等时取较少的一方，并**明确报出没用上的是哪些** ——
静默截断会让人以为片子出全了。

## 配音：让念白驱动镜头时长

```bash
reel make ... --voice narrator          # voxcraft 后端（默认）
reel make ... --voice-engine museav     # MiMo API 后端
```

开了配音后，**镜头时长不再是固定的 `--per-shot`，而是「这一句念白的长度 + 余量」**。
念快的句子不用干等，念慢的不会被切。实测同一组文案：固定 2.5s 出 20.0s，
念白驱动出 16.7s，镜长在 1.7~2.7s 之间浮动。

### 两个后端怎么选

| | voxcraft（默认） | museav |
|---|---|---|
| 位置 | 本地 Qwen3-TTS Base-1.7B | 小米 MiMo API |
| 成本 | **一次性 4.2GB 模型，之后免费** | 按 token 计费 |
| 音色 | **声音克隆 / 音色设计**，IP 专属嗓音 | 预置音色（Chloe 等），开箱即用 |
| 前置 | 需先注册音色（`voice voice add <key> <参考音频>`） | 无 |

默认选 voxcraft 是成本决定的：批量流水线一支片子几十句，API 调用会持续累积。

> ⚠️ voxcraft 的音色库**初始为空**。没注册音色前 `--voice` 会失败，
> 此时用 `--voice-engine museav` 兜底。

### voxcraft 走服务模式，不逐句起进程

voxcraft 的 CLI **每次调用都重新加载 4.2GB 模型** —— 一支 8 句的片子就是加载 8 次，
慢且不稳（实测第 5 句崩在 `libc++abi: recursive_mutex lock failed`）。

所以这里起一次 `voice web` 服务（`POST /api/clone` + 轮询 `/api/tasks`），
一次加载多次调用，用完 `try/finally` 关掉。已在跑的服务会被复用而不是重复起。

实测 8 句一次跑通，全程 61 秒（含一次模型加载），结束后端口关闭、无残留进程。

### 对齐是怎么保证的

每句念白后面补上「镜头时长 − 念白时长」的静音（`apad=whole_dur=<镜头时长>`），
再首尾相接 concat。**不算 offset，也就不会有累积误差** ——
实测 8 镜全部对齐：每镜开头 -13~-19dB（念白），镜尾 -91dB（补的静音），
到第 8 镜仍然精确。

余量（`--voice-margin`，默认 0.45s）不能省：画面做到「刚好等于念白」时，
帧率舍入会让画面略短于音频，每句最后半个字被吃掉。
这条来自 `docs/video-compositing-notes.md` 第 4 条。

## 素材要求

模板把素材当**主体图**居中呈现。若要做出表情贴纸那种效果，
素材需要**预先抠图**（透明背景 PNG）：

```bash
museav remove-bg 01.png        # 输出带 alpha 的 PNG
```

直接喂未抠图的原图也能出片，只是呈现为带投影的圆角卡片，不是贴纸感。
本工具**不做抠图** —— 那是 `museav remove-bg` 与 `wechat-sticker-submit` skill 的职责，
在这里重做一遍只会变成第二套实现。

完整链路：

```
原图 → museav remove-bg → 白描边/240×240（wechat-sticker-submit）
                                      ↓
                          reel make（套版式 + 文案 + BGM）→ mp4
```

## 模板

`reel templates` 列出可用模板。模板是 `templates/*.html`，占位符 `{{key}}`：

| 占位符 | 来源 |
|---|---|
| `{{title}}` `{{subtitle}}` `{{footer}}` | 对应 CLI 同名参数 |
| `{{bg}}` `{{accent1}}` `{{accent2}}` | 配色参数 |
| `{{caption}}` | `--caps` 的当前行 |
| `{{image}}` | 当前素材（自动转 data URI，模板离线自包含） |

加新模板 = 往 `templates/` 丢一个 HTML，无需改代码。

### sticker-promo（1080×1920）

表情包推广版式：白底 + 左上/右下双色圆弧、顶部专辑名与 IP 名、
中央素材、下方逐句文案、底部引导语。

## 已知局限

- **无转场**：镜与镜之间是硬切。原成片也是硬切，所以没做；要转场得在
  `compose.mjs` 里加 xfade 滤镜链。
- **BGM 短于片长会循环**：已用 `aloop` 补满并在片尾淡出 1.5s。若循环接缝明显，
  换更长的曲子，或后续加「按镜头数选段」逻辑。
- **字幕不自动换行分镜**：一句一镜。长句在模板里会自然折行，但不会拆成两镜。
