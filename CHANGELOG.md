# 更新日志

## [1.1.1] - 2026-11-10

### 修一处把意思说反的说明

1.1.0 在「配音通道」那一句写的是「本机推理不花钱也不出声」。
「不出声」在配音这件事上意思完全相反 —— 用户读到的是
「选了这条通道就不会有声音」，于是干脆不选本机。
原意是「声音不出本机」，这句改准了：

> 本机推理：声音不出本机、不花钱；云端按量计费，任何机器都能用

### 密钥输入框不再明文显示

「设置与反馈 → 云端配音密钥」里的输入框改成密码框：
输入时屏幕上看不到明文，录屏、截图、旁边有人瞄一眼都不会把密钥带出去。
保存后界面上照旧只显示末四位，配置文件仍是 `~/.reel-kit/config.json`（权限 0600）。

### 还补了

- 1.1.0 的更新日志里「不回显明文」这句说得太满（当时输入框还是明文的），
  现按实际行为改写；本条即修正后的说法。
- 桌面端新增 2 条 Go 用例（密钥输入框按密码框处理、配音通道说明不说反话），
  1.1.0 那次实际新增的是 5 条，之前写成 4 条，在此更正。

## [1.1.0] - 2026-11-10

### Windows 版终于能出声了：配音通道可以改选云端

1.0 的 Windows 包有个硬伤：桌面版把配音通道写死成本机 VoxFlow，
而 VoxFlow 只在 Apple 芯片的 Mac 上跑得起来 —— Windows 用户装上应用，
一条带配音的视频都出不来。

1.1.0 把这个写死的地方松开：

- 「声音」这一段多了一个**配音通道**选择：本机 VoxFlow（不花钱、声音不出本机）
  与云端 MiniMax TTS（按量计费，任何机器都能用）。两条都就绪时默认仍走本机。
- 通道没就绪时**提交前就拦下来**，并说清是哪条通道、为什么不可用 ——
  不再让用户等到合成阶段才收到一句 `fetch failed`。
- 「设置与反馈 → 云端配音密钥」可以填 MiniMax API Key：只写在本机
  `~/.reel-kit/config.json`（权限 0600），不写进仓库、不进日志；
  保存后输入框清空、界面只显示末四位（输入框在 1.1.1 才改成不回显明文）；
  保存后应用重启本机工作台让它生效。
- 换通道后，流水线上那句「不产生模型费用」的提示会跟着变 ——
  选了云端却说免费，那是骗人。

实测：桌面版用云端通道提交一条两镜口播，走真实的 MiniMax TTS 出声，
产出 1080×1920、5.02 秒、h264+aac 的成片，音轨时长与两句配音对得上。

### 还修掉的

- 生成配置面板里的「音色」现在按当时的配音通道显示名字：
  云端音色不再显示成一串 `voice_id`。
- 界面上「逐句用本机 VoxFlow 配音」这句提示不再写死。

### 已知限制（没解决的）

- Windows 包**仍未在 Windows 真机上跑过**（本机只有 macOS）；
  它的声音链路是在 macOS 上用同一套代码与真实 MiniMax 接口验的。
- 本机语音服务（VoxFlow）依然只支持 Apple Silicon；Windows 上的本地音色、
  克隆自己的声音用不了。
- 仍未做 macOS 公证与 Windows 代码签名。

## [1.0.0] - 2026-11-09

### 1.0 MVP：四条流水线都能从填表走到成片

这是 1.0 计划的 MVP 版本。左边是流水线，右边是这条流水线跑出来的视频
（条数、进度、每条背后的生成配置，可以按同一份配置再生成一条，也可以挑一条对比）：

| 流水线 | 做什么 |
|---|---|
| 知识口播 | 写一句一镜的脚本 → 逐句本机配音 → 按实际语速定每镜时长 → 渲染画面帧 → 合成 |
| 项目推荐 | 填 GitHub 仓库 → 本地读 README 与截图 → 分镜 → 逐句旁白 → 带解说的项目介绍视频 |
| 素材合成 | 素材 + 配音 + BGM 按顺序拼成一条，导出到本地项目 |
| 人物商品介绍 | 给产品或人物出一条介绍片：竖版出镜满屏，横版左图右文，画幅决定版式 |

声音库可以录一段自己的声音注册成全局音色，任何流水线都能选它（录音不出本机）。

> 「装完就能用」的前提是先装一次本机语音服务（VoxFlow，只在 Apple 芯片的 Mac 上跑）；
> 1.1.0 起 Windows 上可以改走云端 MiniMax TTS。

### 自动更新链路验通了

此前只验到「签名对」，这次拿发布页上的旧归档与增量包走了一遍生产代码：
增量包把 0.2.2 还原成 0.2.3，6056 个文件逐个 SHA-256 一致；再走 `installUpdate`
把装着 0.2.2 的 `.app` 原地换成 0.2.3（只下了 823 KB 增量包），升级后的应用仍能
出一条 1080×1920、7.03 秒、h264+aac 的竖版视频，抽帧确认标题与字幕可读。

### 发版脚本修了三个会发错东西的缺陷

- Windows 归档被清单循环与通配符循环各收一次，同名产物传了两遍；
- `SHA256SUMS` 里写的是 `build/<目标>/文件名`，用户下完 `shasum -c` 验不了；
- 不检查 tag 与清单里的版本，旧构建留下的清单会被当成新版本发出去。

现在 tag 必须是 `tagPrefix+版本`、清单版本必须等于当前版本、产物名冲突即停，
同名产物只收一次，`SHA256SUMS` 用发布页上的扁平文件名；并新增 `--dry-run`
只检查不发布。

### 第一次用会告诉你怎么装本机语音服务

桌面端没有网页可点：本机语音服务（VoxFlow）没就绪时，用户以前只看到空的音色
列表。现在口播、人物商品介绍、声音库三处直接摆出安装与启动命令，并提供
「复制安装命令 / 复制启动命令」；项目推荐在本地配音通道未就绪时同样给出说明。
官网下载页与 README 同步写明这一步，并把原来「不用另装依赖」的说法改准。

### 还没做的（如实写）

- Windows 产物没有在 Windows 上实跑过，只验了包内容物。
- 安装包未做代码签名与公证：macOS Gatekeeper、Windows SmartScreen 会拦一下。
- 计划里的 5 人可用性测试还没做。

## [0.2.3] - 2026-11-09

### 开放人物商品介绍流水线

第三条流水线之前是「未开放」，现在接上了，可以真的出片。

- 后端新增 `src/studio/product.mjs`：一句一镜 → 本机配音 → 渲染画面帧 → 合成成片。
  竖版用 `product-presenter`（出镜画面满屏 + 底部字幕条），横版用 `landscape-product`（左图右文），画幅决定版式。
- 桌面端新增这一条流水线的生成配置：产品或人物名称、介绍词、一句话卖点、片尾一句、音色、语气、镜间停顿、产品画面、画幅、BGM 与音量；支持按同一份配置再生成一条与配置对比。
- 介绍词限 1–8 行、每行不超过 80 字，超了会在提交前被拦下。
- 字幕字号按这一句的长度收档：短句大一号、长句小一号，不会溢出画面；竖版底部压深，字幕压在图上也读得清。

样片已验证：竖版 1080×1920 三镜 15.07 秒、横版 1920×1080 一镜 11.10 秒、竖版 80 字长介绍词一镜 8.50 秒，均为 h264 + aac 的可播放 mp4。

样片见 `docs/samples/product-intro-9x16.mp4` 与 `docs/samples/product-intro-16x9.mp4`。

### 素材库新增「声音库」

- 可以导入一段本地录音注册成自己的音色：填音色名称 + 录音原文 + 选录音文件（wav/mp3/m4a/aac/flac/ogg，≤20MB），注册完任何流水线都能选它。
- 录音只在本机处理，不上传。

### 修复：安装包里带着旧后端

- 已发布的 0.2.1 / 0.2.2 安装包里，后端是发版那一刻的旧快照，没有知识口播这条流水线：点「生成口播视频」会被后端直接拒绝（任务类型不支持）。0.2.3 重新刷新了打包资源，并把「包里的后端必须与仓库一致」做成门禁 `scripts/check-bundle-freshness.mjs`（接进 `pnpm check`，CI 也跑），以后不会再带着旧后端发版。
- 占位图在横版画幅下仍按竖版尺寸生成。

### Windows 版

- 首次提供 Windows x64 构建产物（未做代码签名，首次运行会被 SmartScreen 拦一下，选「仍要运行」即可）。压缩包里是 `Reel Studio.exe` 与它旁边的 `backend/` `bin/` `chrome/` `museav/` 资源目录，解压后整体保持在同一层目录再运行。

## [0.2.2] - 2026-11-09

### 桌面端：按「视频流水线」重排界面

- 左侧导航从「菜单平铺」改成按流水线分组，每条流水线标出产出条数，未开放的入口明确标「未开放」。
- 每条流水线左栏是它的生成配置（脚本 / 声音 / 画面 / 版式 / 画幅 / 输出），右栏是这条流水线跑出来的视频列表、进度条，以及每条视频背后的生成配置。
- 视频支持「按此配置再生成一条」与「找一条对比」，对比只列不一样的项。

### 补齐每个视频组成部分的配置

- 推荐流水线补上逐镜配音与整段旁白音频的选择；BGM 音量改成百分比并做范围校验；填错数字会拦下并说明该怎么填。
- 素材合成补回标题、字幕、配音、版式、每镜时长的回填，「按此配置再生成一条」不再丢设置。

### 修复

- 输入框下方说明文字掉出约 90 像素：MyGo 的 `LineHeight` 是字号倍数而非像素，改用 `FixedLineHeight`（`apps/reel-studio/ui/kit_test.go` 有几何回归测试）。
- 视频播放不启动：播放循环排在阻塞式轮询之后，改为并发启动。
- 对比选择器打不开：渲染条件与点击逻辑互斥。
- 素材库计数对不上：统计口径改为与列表一致（不含成片）。
- 推荐流水线「整段音频切分」不可用：补上旁白音频字段与提交前校验。

## [0.2.0] - 2026-11-06

### 新增：知识短视频视觉系统与 44 套风格库（吸收自 vibe-knowledge）

不依赖外部高成本视频大模型，纯靠结构化文案 + 权威出处 + 代码级精致排版与转场，零成本快速出片。

- **44 套视觉风格数据字典与配色映射** (`src/knowledge-styles.mjs`)：
  - 覆盖 8 大风格家族：东方（古风、水墨纸本、敦煌、青花等）、暗场概念（星空光点、暗金线描、深色数学 3B1B、霓虹等）、图解设计（粉笔黑板、蓝图专利、等轴测、包豪斯等）、印刷纸本（孔版印刷、半调拼贴、杂志编辑等）、剪辑纪实（字幕新闻、视觉调查、复古桌面等）、游戏（经典像素、RPG、卡牌等）、明亮流行（综艺花字、孟菲斯、手账涂鸦等）、电影感（剪影片头、默片字卡、构成主义等）。
  - 新增 `reel styles` 命令：一键查阅全部 44 套风格代码、RGB 色盘及推荐模板。
- **3 套专属知识短片模板** (`templates/`)：
  - `knowledge-dark.html`：暗场概念/深色科技/数学公式，发光呼吸感、极简微粒网格与高对比图解展示。
  - `knowledge-gufeng.html`：东方古风/文史哲长卷，夜赭底、朱红印章、金线暗调与典雅排版。
  - `knowledge-paper.html`：纸本水墨/档案调查，米黄纸本微噪、深墨排版与文献卡片质感。
- **CLI 原生支持 `--style` 与 `--source` 权威出处参数**：
  - `--style <名>`：自动匹配并注入对应视觉风格的背景色与双强调色，未显式指定模板时自动路由最佳推荐模板。
  - `--source "<文献>"`：原生支持在画面标注权威学术/历史文献（如《史记·平准书》、MIT 课程报告），未提供时自动优雅隐去角标。
- **生产指南与 LLM 提示词沉淀**：
  - `docs/knowledge-video-guide.md`：6 种爆款知识叙事母版（猜谜概念、两难选择、痛点原理、一条主线、闯关挑战、六面轮转）与一句一镜节奏指南。
  - `prompts/knowledge/script-generator.md`：开箱即用的 Prompt，输入知识点自动产出规范文案与执行参数。
- **示例工程收敛**：
  - 将游离的 `project-demo-kit` 收敛并入 `examples/project-demo-kit`，作为现成的项目宣传片录制演示资产。

### 移除：09-14「三合一」拷进来的三样非本仓资产

`studio/`、`prompts/agents/`、`seasons/s1/novels/.agent-skills/` 三处是 2026-09-14
从 `webkubor/boiling-snow` **物理拷入**的，但它们全是**沸雪专属**资产：
工作台读的是沸雪的 `seasons/`（429M 在那边，本仓只有 48K 空壳）、
`CREATIVE_BIBLE.md` 是《沸腾之雪》的创作法典、Skill 是武侠小说生产期的。

**在本仓跑不起来，也从未被跑过**：`studio/` 无 `.state`（工作台跑过就会留）、
无 README，`dist/` 停在 09-14 合并当天。工作台回到 `video/boiling-snow/studio/`。

同步清掉的引用：README 中英 / AGENTS.md / docs/index.html / package.json 的
description 与 keywords（`vue` `vite` `monorepo` 三个随工作台进来的）。

## [0.3.2] - 2026-09-24

### 定位:短视频合成引擎 · AI 无人值守的批量装配器(用户拍板)

不再叫"竖版短视频生产工作台"——它不是生成器、不剪辑现有视频、不是端到端生产线。
它真正的位子是 **短视频 pipeline 里中间那一环:消费上游素材,吐下游成品 mp4**。

更新以下字段(锁定口径):
- `AGENTS.md` 这是什么段(锁定边界)
- `README.md` / `README.en.md` hero banner
- `package.json` description + keywords 字段
- `docs/index.html` title / meta / hero

定位边界:
- **不生成视频**(AI 出视频是上游 Hailuo 这类工具的事)
- **不剪辑已有视频**(没 timeline / trim / 调色)
- **不是端到端生产线**(用户得自己给文案 + 素材 + 审片)

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
