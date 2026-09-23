# 更新日志

## [0.3.1] - 2026-09-23

### 新增

- **缩略图 / 分镜帧理解接入 mlx-vlm-kit** — 缩略图选帧和分镜帧内容判断改走本机共享识图 CLI(避免每仓装一遍 VLM 权重)。详见 `studio/README.md`。
- **完整英文 README**(`README.en.md`) — GitHub 仓库首页自动识别为英文版,与中文版镜像。
- **`pnpm pub`** — 用 `secret://npm/kubor-company-publish`(kubor 公司 NPM PAT,expiry 2026-12-20,scopes `package:write` + `org:write`)走 `cs kyvault run` 发布,密钥不落 CLI 历史。

### 运维

- **webkubor.xyz DNS 备份**(`docs/dns-backup/webkubor-xyz.json`) — 域名下线前存档 zone meta + 2 条 DNS 记录;`secret://cloudflare/api-token` 加了下线备注。
- **Plausible 埋点 revert** — 改用 Cloudflare Web Analytics(10 个 RUM site 已 auto-install,免费,覆盖多站点)。`webkubor/boiling-snow`、`webkubor/story-to-video`、`webkubor/vite-plugin-agent-eyes`、`webkubor/ai-orb` 四个 GH Pages 同步清理。

### 仓库归档(配套 0.3.0 三合一)

- GitHub 端 `webkubor/boiling-snow` 和 `webkubor/story-to-video` 已 archived(90 天保留期),README 指引用户迁到 reel-kit。

### 文档同步

- `AGENTS.md` 验证脚本名:`pnpm demo` → `pnpm test`(测试套件迁到 `test` script)。
- `README.md` studio views 数 15 → 11;补"末帧不重复(实测 ffmpeg 7.x)"警告并指向 `AGENTS.md` 第 1 条。

## [0.3.0] - 2026-09-14

### 重构:三合一(reel-kit 单仓统一)

reel-kit 不再只是"短视频合成 CLI",而是**单仓统一入口**,覆盖三件事:
- **推广** — `studio/` 工作台 + `prompts/agents/` 创作规范
- **剪辑** — `@kubor/reel-kit` CLI / 库(本来的核心,未动)
- **视频提示词模板开发** — `prompts/handdrawn/` 20 风格库 + `studio/PromptLabView` + `seasons/s1/novels/.agent-skills/` 7 个生产期 Skill

### 新增目录

- `studio/` — 整体迁入原 `webkubor/boiling-snow` 的 `studio/` 子项目,改名为 `@kubor/reel-studio`。Vue 3 + Vite 8 + 自带 Node 后端(Vite middleware,127.0.0.1 only)。15 个 view 覆盖镜头台 / 角色库 / 集数 / 提示词实验室 / 批处理 / 推广位。
- `prompts/agents/` — 原 `boiling-snow/agents`,创作规范 `CREATIVE_BIBLE.md`。
- `prompts/handdrawn/` — 从上游 `gnipbao/story-to-handdrawn-video` 拉的 20 套手绘风格配方。本地 GitLab 镜像里 `handdrawn-style-library.json` 不存在,改从 GitHub upstream 拉。
- `seasons/s1/novels/.agent-skills/` — 7 个 Skill 配置(写手中枢/起名中枢/世界书审计/…)。Studio 工具的必需数据。
- `pnpm-workspace.yaml` — 包含根项目 + studio 两个 workspace。

### UI + CLI 双形态

- **CLI**:`bin/reel.mjs`(原样,0 行为变化)
- **UI**:`pnpm dev --filter @kubor/reel-studio` → 127.0.0.1:5273
- 共享 `src/`:Studio 通过 `studio/server/lib/...` 直接 import 仓内核心模块,**同仓相对路径,无 monorepo 工具、无 npm link**

### 修复

- `html2canvas` 是 boiling-snow 原 package-lock 漏列的依赖,迁入后报 build 失败。已加到 `studio` 的 `dependencies`。

### 验证

- `node --check` 6 个根文件 + 8 个 studio 文件全过
- `pnpm --filter @kubor/reel-studio build` 通过(15 个 view 编译干净)
- `node bin/reel.mjs templates` 列出 5 个模板
- 端到端:`sticker-promo` 出片 12.4s(精度 0.1s)

### 不合并(原仓库保留)

- `boiling-snow` 的 IP 内容(`seasons/s1/{bibles,cast,episodes,music,references,scripts}/`)—— 是"用例",不是"工具"
- `boiling-snow` 的 `scripts/`(Python 自动化,跟 Node.js 不兼容)
- `story-to-video` 的 Remotion 渲染器(技术栈不兼容,只继承风格配方)

### 后续(未在本版做)

- 把 GitHub 端 `webkubor/boiling-snow` 和 `webkubor/story-to-video` 标 archived,README 加迁移说明。

## [0.2.2] - 2026-08-31

### 修复:末帧不再重复(消除 2.5s 时长漂移)

早期 ffmpeg 行为要求重复 concat 列表末帧,这条规则被写进 AGENTS.md 第 1 条和
`docs/video-compositing-notes.md` 第 3 条。**但现行 ffmpeg (7.x) 行为相反**:
重复末帧反而让 ffmpeg 多输出一整段时长。

实测对比(5 镜 2.5s,期望 12.5s):

| 写法 | 实测 | 差 |
|---|---|---|
| 旧实现(重复末行) | 15.0s | +2.5s(多 1 镜) |
| 新实现(不重复末行) | 12.4s | 0.1s(末帧舍入) |

每片之前都打 `⚠️ 实际时长与期望差 2.5s`,这条警告 0.2.2 起消失。
AGENTS.md 第 1 条 + `docs/video-compositing-notes.md` 第 3 条同步修正。

### 验证(零警告)

- 5 镜 2.5s 期望 12.5s,实测 12.4s ✓
- 转场 + BGM 期望 10.9s,实测 10.9s ✓
- caps-split 6 镜 2.5s 期望 15.0s,实测 15.0s ✓
- 横版 + 转场 + caps-split + BGM 期望 13.0s,实测 13.0s ✓

## [0.2.1] - 2026-08-31

### 新增

- **`--transition <name>`** + `--transition-duration <秒>`:镜间转场。
  支持 `fade`(默认)/ `slide-left|right|up|down` / `wipe-left|right` / `dissolve` / `zoom-in`,
  时长默认 0.4s。`--transition none` 仍走旧 concat demuxer 路径,零回归。
- **`--caps-split <文件>`**:长句不重读、字幕分镜显示。文件用**空行**分组,
  每段非空行对应 `--caps` 的一行,可拆成 1~N 行字幕。工具按字数比例从原配音
  atrim 切 N 段,听感无"重新合成"的断音(切 wav 精度 < 70ms)。不开此选项时
  行为完全不变。

### 行为说明

- 转场期间画面用 ffmpeg `xfade` 滤镜链(每对相邻镜重叠 X 秒)。
  音画总时长由视频决定:配音尾部 (N-1)\*X 秒被切,通常落在"配音余量"静音期,
  不影响念白;**末镜配音的尾巴可能被切 X\*(N-1) 秒**,配音时主动留余量更稳。
- 已实测 7 种组合(3 模板 × 3 转场名 + BGM)期望 = 实际,无漂移。
- `splitVoiceByChars` 用 ffmpeg `-c copy` 切 wav,sample 对齐误差 < 70ms。

### 已知(仍未改)

- 无 BGM 时的"末帧多 1 镜(2.5s)"是老问题(ffmpeg concat demuxer 特性),
  跟本次改动无关。

## [0.2.0] - 2026-08-31

### 新增

- **`--transition <name>`** + `--transition-duration <秒>`:镜间转场。
  支持 `fade`(默认)/ `slide-left|right|up|down` / `wipe-left|right` / `dissolve` / `zoom-in`,
  时长默认 0.4s。`--transition none` 仍走旧 concat demuxer 路径,零回归。

### 行为说明

- 转场期间画面用 ffmpeg `xfade` 滤镜链(每对相邻镜重叠 X 秒)。
  音画总时长由视频决定:配音尾部 (N-1)\*X 秒被切,通常落在"配音余量"静音期,
  不影响念白;**末镜配音的尾巴可能被切 X\*(N-1) 秒**,配音时主动留余量更稳。
- 已实测 7 种组合(3 模板 × 3 转场名 + BGM)期望 = 实际,无漂移。

## [0.2.0] - 2026-08-31

针对 README 列出的几条已知局限做了集中优化。

### 新增

- **3 个模板**:`sticker-square`(1080×1080,小红书/朋友圈)、
  `quote`(1080×1920,纯文字金句)、`landscape-product`(1920×1080,产品演示/B 站题图)。
  加模板仍是 `templates/` 丢一个 HTML,零代码改动。
- **`--bgm-crossfade <秒>`**:BGM 短曲子循环接缝的交叉淡化秒数,默认 1.0。
  短于片长的曲子,会先把头 K 秒和尾 K 秒做平滑过渡再去循环,听感无爆音。
  设 0 退回旧行为。
- **BGM 短曲子自动警告**:配乐 < 片长 0.7 倍时,合成前明确告知「需循环 N 次」并提示
  已自动启用 crossfade,而不是静默循环。probe 失败不阻塞合成。

### 行为变更

- `render.mjs` 的 viewport 现在从模板 body 的 `width`/`height` 自动读取,无需再用
  `--size` 指定。`--size` 仅保留为兜底(模板没写 body 尺寸时仍生效)。
- 终端打印的成品尺寸改用 `ffprobe` 探测输出 mp4 的真实尺寸(原来打印的是 `--size`
  参数,跟实际不符)。
- BGM 默认启用 crossfade 1.0s。**这是行为变更**:旧版本直接 loop,新版本默认平滑接缝。
  不想平滑设 `--bgm-crossfade 0`。

### 修复

- `bin/reel.mjs` 加尺寸探测时漏了 `child_process`/`util` 的 import,补上。
- `readTemplateSize` 之前会因模板注释里出现 "body" 字面量而误匹配,现在仍能正确
  读出 body 块里的尺寸(注释里的 width/height 不是 px 单位,正则要求 `\d+px` 收紧)。

### 已知(仍未改)

- **无转场**:镜与镜之间仍是硬切。xfade 方案已设计,需要协调配音余量,排在下一版。
- **一句一镜**:长句不拆。字幕渐进显示(治标方案)排在下一版。
- **横版/多镜模板**:这次补的是「画布不同」的横竖方形,「多镜」还是没动。

## [0.1.1] - 2026-08-28

## [0.1.1] - 2026-08-28

### 修复：配乐库不再指向作者的私有 CDN

0.1.0 里 `src/bgm-library.mjs` 把清单地址写死成了
`https://music.webkubor.online/manifest.json` —— 那是作者自己的 R2。
装了 0.1.0 的人跑 `reel bgm` 或用 `--bgm <别名>`，会去拉那份跟自己无关的清单、
从别人的存储下载文件。这条能力本来就该由使用者自己指清单。

现在清单位置由你指定：

```bash
export REEL_BGM_MANIFEST=https://你的域名/music-manifest.json   # 或本地 json 路径
# 或写进 ~/.reel-kit/config.json： { "bgmManifest": "..." }
```

没配置也完全能用 —— `--bgm ./某文件.mp3` 直接给路径，不经过清单。
未配置时用别名会明确告诉你这两种办法，而不是静默失败或去拉某个陌生域名。

**reel-kit 不自带任何配乐**，音乐授权是使用者自己的事。

其他：
- 清单缓存带上来源地址，换了 `REEL_BGM_MANIFEST` 不会还用旧缓存
- 本地清单不缓存，改完立刻生效（调清单时省事）
- 清单条目的 `key` 支持相对 `cdn` 的路径、完整 URL、或本地绝对路径
- `pnpm check` 补上了 `src/bgm-library.mjs`（新文件此前没进语法检查）

## [0.1.0] - 2026-08-28

首个版本。

### 能力

- **模板化合成**：素材 + 逐句文案 + BGM → 1080×1920 竖版 mp4
- **版式用 HTML/CSS**：加模板 = 往 `templates/` 丢一个 HTML，不改代码
- **念白驱动镜头时长**：开配音后，每镜时长 = 该句念白 + 余量，
  而非固定值。实测同组文案固定 2.5s 出 20.0s、念白驱动出 16.7s
- **两个 TTS 后端**：voxcraft（本地 Qwen3-TTS，批量零成本）/ museav（API）
- **音画零累积偏移**：apad 补齐后 concat，不算 offset。实测 8 镜全对齐

### 已知局限

- 无转场（硬切）
- 一句一镜，长句不拆镜
- 只有 `sticker-promo` 一个模板，横版模板未做
