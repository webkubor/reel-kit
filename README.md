<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/mark-v3-dark.svg" />
    <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/mark-v3.svg" width="96" alt="reel-kit" />
  </picture>
</p>

<p align="center">
  <img src="https://img.shields.io/github/license/webkubor/reel-kit?style=flat-square&color=92a8b3" alt="License" />
  <img src="https://img.shields.io/github/stars/webkubor/reel-kit?style=flat-square&color=cc584d" alt="Stars" />
  <img src="https://img.shields.io/badge/Node-%E2%89%A518-cc584d?style=flat-square" alt="Node >= 18" />
  <img src="https://img.shields.io/badge/deps-ffmpeg%20%2B%20Chrome-cc584d?style=flat-square" alt="ffmpeg + Chrome" />
  <img src="https://img.shields.io/badge/TTS-%E6%9C%AC%E5%9C%B0%20%C2%B7%20%E9%9B%B6%E6%88%90%E6%9C%AC-cc584d?style=flat-square" alt="本地 TTS 零成本" />
</p>

<p align="center">
  <b>短视频合成引擎 · AI 无人值守的批量装配器</b>
  <br />
  素材 + 逐句文案 + 配音/BGM，套模板出片。版式用 <b>HTML/CSS</b> 写，镜头时长由<b>念白</b>决定，配音走<b>本地 TTS</b> 不花钱。
  <br />
  <sub>CLI + 库 + 本地可视化工作台 · 生成素材、组织项目并合成 MP4</sub>
</p>

<p align="center">
  <a href="#-30-秒上手"><strong>快速上手</strong></a> ·
  <a href="#-和其它方案的区别"><strong>差异对比</strong></a> ·
  <a href="#-配音"><strong>配音</strong></a> ·
  <a href="#-仓库结构"><strong>仓库结构</strong></a> ·
  <a href="docs/video-compositing-notes.md"><strong>合成笔记</strong></a>
</p>

<p align="center">
  <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/demo.gif" width="240" alt="reel-kit demo" />
</p>

<p align="center">
  <sub>上图由 reel-kit 自己生成 —— 素材、配音、合成全在本地，
  <a href="examples/">examples/</a> 里有完整可复现的输入</sub>
</p>

<p align="center">
  <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/preview.webp" width="720" alt="三帧预览" />
</p>

---

<!-- 品牌色与排版规范(GitHub README 内联 CSS) -->
<style>
  :root {
    --brand: #cc584d;        /* Reel 红,主品牌色 */
    --brand-soft: #e6a5a0;   /* 浅红,装饰 */
    --ink: #1f1f1f;          /* 正文 */
    --ink-soft: #6b6b6b;     /* 副文 */
    --line: #e8e8e8;         /* 分割线 */
  }
  h1, h2 { color: var(--ink); }
  h2 { border-bottom: 1px solid var(--line); padding-bottom: 6px; }
  a { color: var(--brand); }
  code { color: var(--brand); }
  blockquote {
    border-left: 3px solid var(--brand);
    color: var(--ink-soft);
    padding: 0 12px;
  }
</style>


## 缩略图/分镜帧理解（mlx-vlm-kit）

本工具不含识图模型。要做缩略图质检、分镜帧内容理解、配文案，
用本机共享的 [mlx-vlm-kit](https://github.com/webkubor/mlx-vlm-kit)（免费/离线）：

```bash
vlm describe ./ep07-thumb.jpg                 # 缩略图内容
vlm ask frame01.png --q "这帧符合脚本第3句吗"   # 脚本对齐检查
```

Agent 工作流里也可通过 [cs-mcp](https://github.com/webkubor/cs-mcp) 调用。

## 本地可视化工作台

运行 `npm run studio`，打开 <http://127.0.0.1:4390>。工作台当前主打 **GitHub 项目推荐视频**：输入仓库地址，本地读取 README 与截图、逐句配音（VoxFlow）、渲染合成，产出可直接发布的竖屏/横屏成片；成片经过自动检测（画幅/时长/音轨/完整解码）与人工审片清单确认后才能记录发布，发布后可回填表现、复盘并保存为新版制作方法（SOP），下一次制作自动绑定所选版本，未改动的旁白句复用已生成配音。

其余能力保持可用：项目和素材管理、MuseAV 图片生成、MiniMax H3 视频生成、VoxFlow 音色设计与配音、图片或视频合成导出。项目和生成的素材保存在 `~/.reel-kit/studio/`。

**录制自己的声音**：在全局「声音库」填写名字、照读短句，结束后试听并保存；也可导入音频并填写其中实际说的原文。建议 3–15 秒，最多 30 秒 / 20 MB。保存一次即可在所有项目选择这个音色。需要 VoxFlow **0.10.1 或更新版本**；工作台会检查注册能力，旧服务须升级并重启。音频、原文与 MLX 推理均在本机，不上传云端。

图片生成需要安装并登录 `museav`；语音功能需要启动本机 VoxFlow 服务；合成需要 FFmpeg 和 Chrome。视频使用 MiniMax 的视频 API：启动时通过凭据管理工具注入 `MINIMAX_API_KEY`，可额外注入 `MINIMAX_API_KEY_2` 供第二账户选择。密钥不应写入项目文件。视频生成会消耗账户额度，能否调用以实际权限及接口返回为准。

例如，本机 Kyvault 已保存主账户时可运行：

```bash
kyvault run --env MINIMAX_API_KEY=secret://minimax/api-key -- reel studio
```

### 生成通道：自己选，不替你换

视频、图片、配音各有独立的「通道」，在界面上直接切换：

| 通道 | 能力 | 计费 |
|---|---|---|
| **MuseAV 中台** | 图片与视频（含 Seedance、Kling、Wan 等中台可用模型） | 按中台额度 |
| **MiniMax 直连** | H3 视频、speech-02-hd 配音 | 按你自己的 MiniMax 账户 |
| **本地 VoxFlow** | 音色设计、配音、克隆自己的声音 | 本地推理，不产生模型费用 |
| **自定义通道** | 任意兼容的生图 / 视频接口 | 按你自己的账户 |

规则：**不静默换供应商、不静默换账户**。你选的那条通道没就绪，工作台会禁用生成并写明原因，不会悄悄改用另一条（可能另一家的付费）通道。模型、音色、账户是三件独立的事，界面分开设置。

已付过费的任务如果只是下载失败，工作台会保留上游任务 ID，历史记录里出现「取回已生成的素材」——**取回不会重新提交，因此不会重复计费**。

### 自定义通道配置

在 `~/.reel-kit/providers.json` 里声明自己的通道。**只写环境变量名，不写密钥明文**：

```json
{
  "channels": [
    {
      "id": "seedream",
      "label": "Seedream 生图",
      "kind": "image",
      "protocol": "openai",
      "baseUrl": "https://ark.cn-beijing.volces.com/api/v3",
      "model": "doubao-seedream-4-0-250828",
      "env": "ARK_API_KEY"
    }
  ]
}
```

- `kind`：`image` 或 `video`。`protocol`：`openai`（一次 POST 生图）或 `ark-task`（提交任务再轮询，视频用）。
- `env` 是环境变量名；启动工作台时注入即可：`REEL_STUDIO_HOME=... ARK_API_KEY=... reel studio`。
- 配置有问题时，「连接状态」里会逐条列出是哪一项不对，而不是让通道悄悄消失。
- 自定义 TTS 尚未实现，请用 MiniMax TTS 或本地 VoxFlow。

## 🎬 桌面应用（MyGo 原生界面）

工作台另有 **MyGo 原生桌面版**：窗口、按钮、输入框、下拉、卡片全部是 Go 原生控件，不是网页，不用再写前端。

下载安装（macOS Apple Silicon）：

```bash
curl -fsSL https://github.com/webkubor/reel-kit/releases/download/app-v1.1.3/install.sh | sh
```

Windows（x64，未做代码签名，首次运行被 SmartScreen 拦一下选「仍要运行」）：下载
`reel-studio-1.1.3-windows-amd64.tar.gz` 解压，把 `Reel Studio.exe` 和它旁边的
`backend/` `bin/` `chrome/` `museav/` 资源目录保持在同一层目录再双击。

**第一次用要先装本机语音服务**（配音、克隆自己的声音都要它；本机推理，不产生模型费用）：

```bash
git clone https://github.com/webkubor/voxflow.git && cd voxflow
chmod +x install.sh && ./install.sh --yes
source .venv/bin/activate && voice web
```

服务默认就在 `127.0.0.1:8866`。忘了起也不要紧：应用里会直接把这两条命令摆出来，
点「复制安装命令 / 复制启动命令」贴到终端就行。

**Windows 上没有本机语音服务**（VoxFlow 只在 Apple 芯片上跑）：桌面版的「声音」这一段
可以改选云端 **MiniMax TTS**，配音按量计费、画面与合成仍在本机。密钥在应用里
「设置与反馈 → 云端配音密钥」填，保存后应用会重启本机工作台让它生效（工作台要是你自己起的，就得自己重启一次）；
密钥只写在本机 `~/.reel-kit/config.json`（权限 0600），不进日志也不上传。

应用里左栏是**视频流水线**，不是平铺的菜单；点进一条流水线，中间是它的生成配置
（脚本 / 声音 / 画面 / 版式 / 画幅 / 配乐 / 输出），右栏是这条流水线跑出来的视频
（条数、进度、每条背后的配置，可以按同一份配置再生成一条，也可以找一条对比）。

| 流水线 | 做什么 |
|---|---|
| 知识口播 | 一行一镜的脚本 → 逐句配音 → 按实际语速定每镜时长 → 渲染画面帧 → 合成 |
| 项目推荐 | 读 GitHub 仓库 → 分镜 → 逐句旁白 → 带解说的项目介绍视频 |
| 素材合成 | 把素材、配音、BGM 按顺序拼成一条并导出到本地项目 |
| 人物商品介绍 | 给产品或人物出一条介绍片：竖版出镜画面满屏，横屏左图右文，画幅决定版式 |

素材库这一屏还能**用自己的声音**：填音色名称和录音原文、选一段录音，即可注册一个自己的音色，
之后任何流水线都能选它（录音不出本机）。

```bash
cd apps/reel-studio
go tool mygo dev      # 开发（带热重载）
go test ./...         # 不开窗口测交互
go tool mygo build    # 打包成 .app
```

原生界面与网页版共用同一套本地 HTTP 接口（`src/studio/server.mjs`），数据、素材、成片都还在 `~/.reel-kit/studio/`。

打包进应用的后端是发版那一刻的源码快照（`scripts/prepare-resources.sh`）。改了后端源码却没重新刷新快照，
装出来的应用就会少功能 —— `scripts/check-bundle-freshness.mjs` 会拦住这种发版（已接进 `pnpm check`）。


## ⚡ 30 秒上手

```bash
# CLI:一行命令出片
reel make --template sticker-promo --title "新功能" \
  --assets ./shots --caps ./captions.txt --out out.mp4

```

```bash
npm i -g github:webkubor/reel-kit   # 或 npx github:webkubor/reel-kit ...

reel make --template sticker-promo \
  --title "reel-kit" --subtitle "竖版短视频合成工作台" \
  --assets ./examples/assets --caps ./examples/demo-caps.txt \
  --voice demo_narrator --bgm ./bgm.mp3 \
  --out demo.mp4
```

仓库里带了完整素材与文案，这条命令复现的就是上面那支 demo。

## 🎯 它解决什么问题

做表情包推广、产品短视频这类内容时，每次都是临时拼一串 ffmpeg 命令。
**成品留下了，流程没留下** —— 目录里只有 mp4、素材和文案，合成逻辑用完就丢，
下次做同类片子得从头再拼一遍。

reel-kit 把那段流程固化成可复用的命令。

## ⚖️ 和其它方案的区别

| | 剪映 / CapCut | FFmpeg 裸写 | MoviePy | Remotion | **reel-kit** |
|---|:---:|:---:|:---:|:---:|:---:|
| 批量出片 | ❌ 手工 | ✅ | ✅ | ✅ | ✅ |
| 改版式的成本 | 拖时间线 | **写 drawtext 滤镜链** | 写 Python | 写 React | **写 CSS** |
| 中文排版 | ✅ | ⚠️ 手写字体路径/描边/换行 | ⚠️ 字体度量坑多 | ✅ | ✅ 浏览器排版 |
| 配音 | 手动导入 | 自己接 | 自己接 | 自己接 | ✅ 内置，**本地 TTS 零成本** |
| 镜头时长 | 手动拖 | 固定 | 固定 | 可编程 | ✅ **念白多长镜头多长** |
| 运行时依赖 | 客户端 | ffmpeg | Python + moviepy | Node + React + Chromium | Node + ffmpeg + **本机 Chrome** |
| 适用范围 | 通用剪辑 | 通用 | 通用 | 通用 | **模板化竖版短片** |

**reel-kit 不是通用剪辑器**，它只做「一图一镜 + 逐句文案」这一类模板化短片。
要调色、多轨、精确到帧的对齐，请用 DaVinci Resolve 这类专业工具 —— 那是另一条线的活。

### 三个具体的差异

**① 加模板 = 丢一个 HTML，不用改代码**

版式是 `templates/*.html`，占位符 `{{title}}` `{{caption}}` `{{image}}`。
想改字号、加圆弧装饰、换配色，就是改几行 CSS，改完直接在浏览器里看。

用 ffmpeg 的 `drawtext` 做同样的事，得手写中文字体路径、描边、换行、居中，
圆弧装饰更是画不出来 —— 而且改一次得重调一串滤镜参数，还看不见效果。

**② 念白多长，镜头就多长**

```
固定 2.5s/镜   →  20.0s   念快的句子干等，念慢的被切
念白驱动       →  16.7s   镜长 1.7~2.7s 浮动，节奏自然
```

**③ 配音走本地 TTS，批量不花钱**

默认接 [voxcraft](https://github.com/webkubor/voxcraft)（本地 Qwen3-TTS），
一次性下 4.2GB 模型，之后每句合成都不花钱，还支持声音克隆做 IP 专属嗓音。
也可选择 `--voice-engine minimax` 直连 MiniMax TTS，需注入账户密钥，费用与套餐覆盖以实际账户为准。

---

## 📦 仓库结构(2026-09 三合一后)

```
reel-kit/                          # @kubor/reel-kit
├── bin/                           # CLI 入口(`reel` 命令)
├── src/                           # 视频合成核心(导出为库:import { renderFrames, compose, ... })
├── templates/                     # 5 套 HTML 模板(sticker-promo/square/quote/landscape/music-card)
├── prompts/
│   └── handdrawn/                 # 20 套手绘风格配方(从 story-to-handdrawn-video 上游继承)
├── docs/                          # 设计取舍、合成笔记
└── examples/                      # 完整可复现 demo 素材
```

**两件事怎么映射**:
- **剪辑** = `bin/reel.mjs` CLI + `src/` 视频合成引擎(已暴露为库)
- **视频提示词模板开发** = `prompts/handdrawn/` 风格库(20 套配方)

```bash
npm i -g github:webkubor/reel-kit
```

或者不装，直接用 npx：

```bash
npx github:webkubor/reel-kit make --template sticker-promo ...
```

想改模板 / 跑 demo 就 clone 源码（`examples/` 只在仓库里，不进 npm 包）：

```bash
git clone git@github.com:webkubor/reel-kit
cd reel-kit && pnpm install
npm link
```

依赖：**ffmpeg**（合成）、**本机 Chrome**（渲染版式，不额外下 chromium，路径可用 `CHROME_PATH` 覆盖）。

配音是可选的 —— 不加 `--voice` 就不需要任何 TTS。

## 🎙️ 配音

```bash
reel make ... --voice <音色>                 # voxcraft 本地（默认）
reel make ... --voice <音色ID> --voice-engine minimax   # MiniMax TTS 直连
```

| | voxcraft（默认） | MiniMax TTS |
|---|---|---|
| 位置 | 本地 Qwen3-TTS Base-1.7B | MiniMax 云端 API |
| 成本 | 一次性下载模型，本地推理 | 以账户实际计费为准 |
| 音色 | 已注册的本地音色 | 供应商音色 ID |
| 前置 | 需先注册音色 | 注入 `MINIMAX_API_KEY` |

> `museav speak` 在当前 MuseAV CLI 中不可用，不能作为配音后端。voxcraft 的音色库初始为空，使用前需注册音色。
> reel-kit 会自动定位本机的 voxcraft 安装（它的 `voice` 命令在 venv 里，默认不在 PATH），
> 缺什么会明确报出来并给可复制的修复命令 —— **不会自动去装 4.2GB 的东西**。

### 音画对齐是怎么保证的

每句念白后面补上「镜头时长 − 念白时长」的静音（`apad=whole_dur=<镜头时长>`），
再首尾相接 concat。**不算 offset，也就不会有累积误差。**

实测 8 镜全部对齐：每镜开头 -13~-19dB（念白），镜尾 -91dB（补的静音），到第 8 镜仍然精确。

配 BGM 时自动压到 0.22 增益垫底（可用 `--bgm-gain` 调）。
实测念白段 -17~-21dB、间隙纯 BGM -25~-33dB，差 6~13dB —— 人声在前，BGM 不抢。

---

## 🎵 配乐库（可选）

最简单的用法是直接给文件：

```bash
reel make ... --bgm ./local.mp3
```

如果你有一批常用配乐，可以建一份清单，之后按**别名**取，不必每次翻路径：

```bash
export REEL_BGM_MANIFEST=https://你的域名/music-manifest.json
# 或写进 ~/.reel-kit/config.json： { "bgmManifest": "..." }

reel bgm                        # 看有哪些曲子（别名 / 时长 / 情绪 / 授权）
reel make ... --bgm 轻快         # 按别名取，首次自动下载并缓存到 ~/.reel-kit/bgm/
```

清单格式（`REEL_BGM_MANIFEST` 可以是 URL，也可以是本地 json 路径）：

```json
{
  "cdn": "https://你的域名/",
  "bgm": [
    {
      "alias": "轻快",
      "key": "bgm/upbeat.mp3",
      "duration": 113,
      "mood": ["轻快", "日常"],
      "license": "CC0 / 免署名可商用",
      "source": "曲子从哪来的"
    }
  ]
}
```

`key` 可以是相对 `cdn` 的路径、完整 URL，或本地绝对路径。

> **reel-kit 不自带任何配乐** —— 音乐授权是使用者自己的事，塞一批曲子进来只会给你添麻烦。

### 填了 `license` 就会在出片时打印出来

```
[reel] 配乐「轻快」授权：CC0 / 免署名可商用
```

**每次**都打印，不只首次下载时。这不是装饰：片子是要对外发的，「这首能不能商用、
要不要署名」必须在出片那一刻看得见 —— 等发出去了再翻记录找来源就晚了，
而人恰恰是在反复出片时忘掉授权的。所以建议清单里每条都填。

远端清单会缓存一天，离线时用上次的缓存兜底，拉不到也不会让出片失败；
本地清单不缓存，改完立刻生效。

---

## ⚙️ 常用参数

```
--template <名>      模板，默认 sticker-promo（reel templates 查看全部）
--assets <目录|文件>  素材图，目录按文件名排序
--caps <文件>         逐句文案，一行一句，行数决定镜头数
--title/--subtitle/--footer   模板文字
--bgm <别名|文件>     背景音乐，自动循环补满 + 片尾淡出。别名见 reel bgm
--bgm-gain <0~1>      BGM 增益，有配音时默认 0.22
--voice <音色>        开启配音，镜头时长改由念白决定
--voice-margin <秒>   每镜在念白后多留的时间，默认 0.45
--per-shot <秒>       无配音时的固定镜长，默认 2.5
--size <WxH>          画布，默认 1080x1920
--keep-frames         保留中间产物，调版式用
```

## 🧩 设计取舍

**为什么用浏览器渲染版式** —— 见上面「三个具体的差异」第 ①。代价是每帧跑一次截图，
但一支片子只有 8~20 帧，几秒的事。顺带绕开了 MoviePy 生态里字体度量、
视觉居中、句柄泄漏这一整类问题（详见 [`docs/video-compositing-notes.md`](docs/video-compositing-notes.md)）。

**为什么用 concat demuxer 而不是 `-framerate 1/2.5`** —— 后者要求每镜等长，
而「片尾多停一会」是很自然的需求。

> ⚠️ ffmpeg 7.x 实测：**不要**重复 concat 列表的末帧（重复会让总时长多出一整镜，5 镜 × 2.5s 期望 12.5s 重复末行实测 15.0s，不重复实测 12.43s）。
> 这条跟早期文档描述的"必须重复"相反，**改自实测**，详见 [`AGENTS.md`](AGENTS.md) 第 1 条与 [`docs/video-compositing-notes.md`](docs/video-compositing-notes.md) 第 3 条。

**素材与文案数量不等时**，取较少的一方并**明确报出没用上的是哪些** ——
静默截断会让人以为片子出全了。

**voxcraft 走 web 服务模式** —— 它的 CLI 每次调用都重新加载 4.2GB 模型，
逐句起进程做 8 句就是加载 8 次，慢且不稳（实测第 5 句崩在 `libc++ recursive_mutex`）。
现在起一次服务多次调用，用完关掉；已在跑的服务会被复用。

## 🖼 素材要求

模板把素材当**主体图**居中呈现。要做出贴纸效果，素材需**预先抠图**（透明 PNG）。
本工具不做抠图 —— 那是上游的职责，在这里重做只会变成第二套实现。

## ⚠️ 已知局限

- **无转场**：镜与镜之间硬切。要转场得在 `compose.mjs` 里加 xfade 滤镜链。
- **一句一镜**：长句在模板里会自然折行，但不会拆成两镜。
- **只有一个模板**：`sticker-promo`（1080×1920 竖版）。横版多镜头模板还没做。
- **BGM 短于片长会循环**：已用 `aloop` 补满并淡出，若接缝明显请换更长的曲子。

## 🌐 GitHub Pages 官网

`docs/index.html` 是 reel-kit 的官方单页网站,部署到 GitHub Pages 后可在 `https://webkubor.github.io/reel-kit/` 访问。

**启用方法**(30 秒):

1. GitHub 仓库 → **Settings** → **Pages**
2. **Source** 选 `Deploy from a branch`
3. **Branch** 选 `main` · **Folder** 选 `/docs`
4. 保存,几分钟后自动上线

`docs/` 目录包含:

- `index.html` — 单页官网(hero / features / 5 套模板预览 / 30 秒上手)
- `style.css` — 主色 Reel 红 (`#cc584d`) 品牌样式
- `logo.svg` / `logo-light.svg` — 浅色 / 深色模式 logo
- `analytics-setup.md` — 埋点集成指南(默认关闭,需手动启用)

## Star History

<a href="https://www.star-history.com/#webkubor/reel-kit&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=webkubor/reel-kit&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=webkubor/reel-kit&amp;type=Date" />
    <img alt="Star History Chart for webkubor/reel-kit" src="https://api.star-history.com/svg?repos=webkubor/reel-kit&amp;type=Date" width="100%" />
  </picture>
</a>

## 📄 License

MIT
