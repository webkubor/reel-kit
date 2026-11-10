# reel-kit 命令行与引擎文档

面向想用命令行、接自己的流程、或改引擎源码的人。只想装个桌面应用出片，见 [README](../README.md)。

## 安装

**本项目不发布到 npm 仓库**，命令行也只从 Git 仓库取 —— 没有 `npm i -g @kubor/reel-kit` 这条路。

一次性执行（从 Git 仓库直接跑，不经过 npm 仓库）：

```bash
npx github:webkubor/reel-kit make --template sticker-promo ...
```

要改模板、跑 `examples/` 里的 demo、或者改引擎源码，就 clone 下来：

```bash
git clone git@github.com:webkubor/reel-kit
cd reel-kit && pnpm install && npm link   # npm link 只是把本仓库挂成全局命令 reel
```

不想装全局命令，就在仓库里直接跑：`node bin/reel.mjs make ...`。

依赖：**ffmpeg**（合成）、**本机 Chrome**（渲染版式，不额外下 chromium，路径可用 `CHROME_PATH` 覆盖）。配音是可选的 —— 不加 `--voice` 就不需要任何 TTS。

只想用应用出片、不碰命令行，见 [README](../README.md) 的下载一节。

## 30 秒上手

仓库里带了完整素材与文案，这条命令复现的就是官网首页那支 demo：

```bash
reel make --template sticker-promo \
  --title "reel-kit" --subtitle "竖版短视频合成工作台" \
  --assets ./examples/assets --caps ./examples/demo-caps.txt \
  --out demo.mp4
```

想让它念出来，再加 `--voice <已注册的音色>`（音色要先在 voxcraft 里注册，命令行的音色库初始是空的）。

其他子命令：

| 命令 | 作用 |
|---|---|
| `reel studio` | 启动本地可视化工作台，默认 <http://127.0.0.1:4390> |
| `reel make` | 按参数出一条片子 |
| `reel templates` | 列出全部模板 |
| `reel styles` | 列出知识短视频视觉风格库 |
| `reel bgm` | 列出配乐库（别名 / 时长 / 情绪 / 授权） |
| `reel channels` | 管理自定义模型通道（自己的中转 / 厂商直连） |

## reel make 常用参数

| 参数 | 说明 |
|---|---|
| `--template <名>` | 模板名（见 `reel templates`），默认 `sticker-promo` |
| `--style <名>` | 知识短片视觉风格（见 `reel styles`），自动带上推荐模板与配色 |
| `--source <文本>` | 出处与权威文献标注（如《自然》2024），显示在角标 |
| `--assets <目录\|文件>` | 素材图目录（按文件名排序）或逗号分隔的多个文件 |
| `--caps <文件>` | 逐句文案，一行一句；行数决定镜头数 |
| `--caps-split <文件>` | 长句拆成多镜的字幕分段，用空行对应 `--caps` 各行 |
| `--title/--subtitle/--footer` | 模板文字 |
| `--bgm <别名\|文件>` | 背景音乐（自动循环补满 + 片尾淡出） |
| `--bgm-gain <0~1>` | BGM 音量，有配音时默认 0.12 |
| `--bgm-crossfade <秒>` | BGM 循环接缝交叉淡化，默认 1.0 |
| `--fade-out <秒>` | 片尾淡出，默认 1.5 |
| `--preset <预设>` | 一键套用常用组合：`default` / `clear-speech`（口播清晰）/ `ambient`（氛围铺底）/ `raw`（原样输出） |
| `--voice-gain <倍数>` | 人声增益，默认 1.35，范围 0.4–3 |
| `--per-shot <秒>` | 无配音时的固定镜长，默认 2.5 |
| `--min-shot <秒>` | 镜头下限，避免极短句一闪而过，默认 1.2 |
| `--last-shot <秒>` | 末镜时长（留给引导语） |
| `--out <文件>` | 输出 mp4 |
| `--size <WxH>` | 画布，默认 1080x1920 |
| `--fps <n>` | 输出帧率，默认 30 |
| `--bg <色>` | 背景色，默认 `#ffffff` |
| `--accent1/--accent2 <色>` | 左上 / 右下圆弧色 |
| `--transition <名>` | 转场，默认 `none`（硬切）；可选 `fade` `slide-left/right/up/down` `wipe-left/right` `dissolve` `zoom-in` |
| `--transition-duration <秒>` | 转场时长，默认 0.4 |
| `--keep-frames` | 保留中间产物，调版式用 |
| `--voice <音色>` | 开启配音，镜头时长改由念白决定 |
| `--voice-engine <名>` | `voxcraft`（默认，本地 Qwen3-TTS，免费）\| `minimax`（云端 TTS，按量计费） |
| `--voice-margin <秒>` | 每镜在念白之后多留的时间，默认 0.45 |
| `--design <描述>` / `--instruction <文本>` | voxcraft 后端：设计音色 / 语气指令 |
| `--no-duck` | 不做侧链：人声一响就把配乐压下去这件事关掉 |
| `--no-normalize` | 不做响度归一化（仍会限幅，不会削顶） |

素材与文案按顺序一一对应，数量不等时取较少的一方，并在开头提示实际用了几镜 —— 多出来的素材或文案会被明确报出来，不会静默丢弃。

## 声音

```bash
reel make ... --voice <音色>                              # voxcraft 本地（默认）
reel make ... --voice <音色ID> --voice-engine minimax     # MiniMax TTS 直连
```

| | voxcraft（默认） | MiniMax TTS |
|---|---|---|
| 位置 | 本地 Qwen3-TTS Base-1.7B | MiniMax 云端 API |
| 成本 | 一次性下载模型，本地推理 | 以账户实际计费为准 |
| 音色 | 已注册的本地音色 | 供应商音色 ID |
| 前置 | 需先注册音色 | 注入 `MINIMAX_API_KEY` |

> `museav speak` 在当前 MuseAV CLI 中不可用，不能作为配音后端。voxcraft 的音色库初始为空，使用前需注册音色。
> reel-kit 会自动定位本机的 voxcraft 安装（它的 `voice` 命令在 venv 里，默认不在 PATH），缺什么会明确报出来并给可复制的修复命令 —— **不会自动去装 4.2GB 的东西**。

桌面版另有一套：随包自带 sherpa-onnx 中文离线模型（免费、离线、macOS 与 Windows 同一套），云端 TTS 与声音克隆见 README。

### 音画对齐是怎么保证的

每句念白后面补上「镜头时长 − 念白时长」的静音（`apad=whole_dur=<镜头时长>`），再首尾相接 concat。**不算 offset，也就不会有累积误差。**

实测 8 镜全部对齐：每镜开头 -13~-19dB（念白），镜尾 -91dB（补的静音），到第 8 镜仍然精确。

配 BGM 时自动压到 0.12 增益垫底（可用 `--bgm-gain` 调），并且用侧链压：人声一开口，配乐被压下去约 10 dB，人声停了慢慢抬回来。压多深取决于念白本身有多响 —— 念白越响压得越深，安静的念白不会触发。

成品统一到 -14 LUFS / -1 dBTP，不同素材、不同后端出来的片子响度一致（`--no-normalize` 可以关掉，仍然会限幅，不会削顶）。

## 配乐库（可选）

最简单的用法是直接给文件：

```bash
reel make ... --bgm ./local.mp3
```

有一批常用配乐，可以建一份清单，之后按**别名**取：

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

填了 `license` 就会在出片时打印出来，**每次**都打印：

```
[reel] 配乐「轻快」授权：CC0 / 免署名可商用
```

片子是要对外发的，「这首能不能商用、要不要署名」必须在出片那一刻看得见。远端清单会缓存一天，离线时用上次的缓存兜底，拉不到也不会让出片失败；本地清单不缓存，改完立刻生效。

## 模型通道（CLI）

界面里配通道最省事，命令行这边是等价的子命令：

```bash
reel channels list                       # 列出已配通道 + 配置问题
reel channels add --id seedream --base-url https://ark.cn-beijing.volces.com/api/v3 \
  --env ARK_API_KEY --kind image --protocol openai --model doubao-seedream-4-0-250828
reel channels rm <id>                    # 删掉一条通道
reel channels check <id>                 # 探活：不通不花钱，不生成任何内容
```

协议：生图 `openai` / `minimax-image` / `museav`，视频 `ark-task` / `museav`。密钥**只写环境变量名**（`--env`），不写进配置文件。

`reel channels` 管的是**自定义**通道。**Magpie 网关**是工作台内置的通道：不用 `channels add`，
不用填地址、不用填密钥 —— 本机装好 [Magpie](https://github.com/yetone/magpie) 并把网关跑起来，
在「生成素材」的通道下拉里直接选「Magpie 网关」，或者调接口时把 `backend` 传 `"magpie"`。
前置条件、只读能力发现与失败取回见 [Magpie 网关通道](magpie-gateway.md)。

## 仓库结构

```
reel-kit/                          # reel-kit 仓库
├── bin/                           # CLI 入口(`reel` 命令)
├── src/                           # 视频合成核心(导出为库:import { renderFrames, compose, ... })
├── templates/                     # 10 套 HTML 模板
├── prompts/                       # 手绘风格配方库
├── studio/                        # 本地可视化工作台(网页版)
├── apps/reel-studio/              # 桌面应用(Go + mygo 原生界面)
├── docs/                          # 设计取舍、合成笔记
└── examples/                      # 完整可复现 demo 素材
```

**两件事怎么映射**：
- **剪辑** = `bin/reel.mjs` CLI + `src/` 视频合成引擎（已暴露为库）
- **视频提示词模板开发** = `prompts/` 风格库

## 缩略图/分镜帧理解（mlx-vlm-kit）

本工具不含识图模型。要做缩略图质检、分镜帧内容理解、配文案，用本机共享的 [mlx-vlm-kit](https://github.com/webkubor/mlx-vlm-kit)（免费/离线）：

```bash
vlm describe ./ep07-thumb.jpg                 # 缩略图内容
vlm ask frame01.png --q "这帧符合脚本第3句吗"   # 脚本对齐检查
```

Agent 工作流里也可通过 [cs-mcp](https://github.com/webkubor/cs-mcp) 调用。

## 设计取舍

**为什么用浏览器渲染版式** —— 版式是 `templates/*.html`，占位符 `{{title}}` `{{caption}}` `{{image}}`。改字号、加圆弧装饰、换配色，就是改几行 CSS，改完直接在浏览器里看。用 ffmpeg 的 `drawtext` 做同样的事，得手写中文字体路径、描边、换行、居中，圆弧装饰更是画不出来。代价是每帧跑一次截图，但一支片子只有 8~20 帧。顺带绕开了 MoviePy 生态里字体度量、视觉居中、句柄泄漏这一整类问题（详见 [`docs/video-compositing-notes.md`](video-compositing-notes.md)）。

**为什么用 concat demuxer 而不是 `-framerate 1/2.5`** —— 后者要求每镜等长，而「片尾多停一会」是很自然的需求。

> ⚠️ ffmpeg 7.x 实测：**不要**重复 concat 列表的末帧（重复会让总时长多出一整镜，5 镜 × 2.5s 期望 12.5s，重复末行实测 15.0s，不重复实测 12.43s）。
> 这条跟早期文档描述的「必须重复」相反，**改自实测**，详见 [`AGENTS.md`](../AGENTS.md) 第 1 条与 [`docs/video-compositing-notes.md`](video-compositing-notes.md) 第 3 条。

**voxcraft 走 web 服务模式** —— 它的 CLI 每次调用都重新加载 4.2GB 模型，逐句起进程做 8 句就是加载 8 次，慢且不稳（实测第 5 句崩在 `libc++ recursive_mutex`）。现在起一次服务多次调用，用完关掉；已在跑的服务会被复用。

## 素材要求

模板把素材当**主体图**居中呈现。要做出贴纸效果，素材需**预先抠图**（透明 PNG）。本工具不做抠图 —— 那是上游的职责，在这里重做只会变成第二套实现。

## 已知局限

- **一句一镜**：长句在模板里会自然折行，但不会拆成两镜（`--caps-split` 可以手动拆）。
- **BGM 短于片长会循环**：已用 `aloop` 补满并淡出，若接缝明显请换更长的曲子。
- **不是通用剪辑器**：只做「一图一镜 + 逐句文案」这一类模板化短片，调色、多轨、精确到帧的对齐请用 DaVinci Resolve。

## 桌面应用开发

```bash
cd apps/reel-studio
go tool mygo dev      # 开发（带热重载）
go test ./...         # 不开窗口测交互
go tool mygo build    # 打包成 .app
```

原生界面与网页版共用同一套本地 HTTP 接口（`studio/server.mjs`），数据、素材、成片都还在 `~/.reel-kit/studio/`。端点清单与调用示例见 [本地 HTTP 接口](studio-api.md)。

打包进应用的后端是发版那一刻的源码快照（`scripts/prepare-resources.sh`）。改了后端源码却没重新刷新快照，装出来的应用就会少功能 —— `scripts/check-bundle-freshness.mjs` 会拦住这种发版（已接进 `pnpm check`）。发版流程见 [桌面应用发版](desktop-release.md)。

## 本地 HTTP 接口

原生界面与网页版共用同一套本地 HTTP 接口，第三方可以拿它做自己的批量出片工具 —— 端点清单见 [本地 HTTP 接口](studio-api.md)。

## 官网

`docs/index.html` 是 reel-kit 的官方单页网站，部署到 GitHub Pages 后可在 <https://webkubor.github.io/reel-kit/> 访问。启用方法：仓库 **Settings → Pages → Source 选 `Deploy from a branch` → Branch 选 `main`、Folder 选 `/docs`**。

## License

MIT
