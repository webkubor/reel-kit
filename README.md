<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/mark-v3-dark.svg" />
    <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/mark-v3.svg" width="96" alt="reel-kit" />
  </picture>
</p>

<p align="center">
  <img src="https://img.shields.io/github/license/webkubor/reel-kit?style=flat-square&color=92a8b3" alt="License" />
  <img src="https://img.shields.io/github/stars/webkubor/reel-kit?style=flat-square&color=cc584d" alt="Stars" />
  <img src="https://img.shields.io/badge/macOS-Apple%20Silicon-cc584d?style=flat-square" alt="macOS" />
  <img src="https://img.shields.io/badge/Windows-x64-cc584d?style=flat-square" alt="Windows" />
  <img src="https://img.shields.io/badge/%E6%96%87%E5%AD%97%E9%85%8D%E9%9F%B3-%E9%9B%B6%E6%88%90%E6%9C%AC%20%C2%B7%20%E7%A6%BB%E7%BA%BF-cc584d?style=flat-square" alt="文字配音零成本" />
</p>

<p align="center">
  <b>Reel Studio · 把文案和素材，变成一条能直接发的短视频</b>
  <br />
  下载即用的桌面应用（macOS / Windows）。写好文案、选一个声音，剩下的交给它：逐句配音、自动配画面、合成导出 MP4。
  <br />
  <sub>文字配音随包自带离线引擎 —— 不联网、不产生费用、声音不出本机。</sub>
</p>

<p align="center">
  <a href="#下载"><strong>下载</strong></a> ·
  <a href="#三步出第一条片子"><strong>三步出片</strong></a> ·
  <a href="#四条流水线"><strong>能做什么</strong></a> ·
  <a href="#声音从哪来要花多少钱"><strong>声音与费用</strong></a> ·
  <a href="#出问题了"><strong>出问题了</strong></a> ·
  <a href="#开发者文档"><strong>开发者文档</strong></a>
</p>

<p align="center">
  <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/demo.gif" width="240" alt="reel-kit demo" />
</p>

<p align="center">
  <sub>上图由 reel-kit 自己生成 —— 素材、配音、合成全在本地</sub>
</p>

<p align="center">
  <img src="https://cdn.jsdelivr.net/gh/webkubor/free-imgbed@main/reel-kit/preview.webp" width="720" alt="三帧预览" />
</p>

---

## 下载

当前版本 **v1.3.0**（2026-11-10 发布）。[发布页与历史版本](https://github.com/webkubor/reel-kit/releases/tag/app-v1.3.0) · [官网](https://webkubor.github.io/reel-kit/)

| 平台 | 下载 | 大小 | 装完在哪 |
|---|---|---|---|
| **macOS**（Apple Silicon） | [reel-studio-1.3.0-arm64.dmg](https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/reel-studio-1.3.0-arm64.dmg) | 237 MB | 拖进「应用程序」 |
| **macOS**（Apple Silicon） | [reel-studio-1.3.0-darwin-arm64.tar.gz](https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/reel-studio-1.3.0-darwin-arm64.tar.gz) | 289 MB | 解压后把 `Reel Studio.app` 拖进「应用程序」 |
| **Windows**（x64） | [reel-studio-1.3.0-windows-amd64.tar.gz](https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/reel-studio-1.3.0-windows-amd64.tar.gz) | 347 MB | 解压后双击 `Reel Studio.exe` |

**Windows 解压后请保持目录结构**：`Reel Studio.exe` 和它旁边的 `backend/`、`bin/`、`chrome/`、`museav/`、`sherpa/` 五个目录要在同一层，少一个应用就起不来。

**第一次打开会被系统拦一下**，两个平台都是因为这个包没有做签名/公证（见[出问题了](#出问题了)），不是下载坏了。

macOS 也可以用一行命令装：

```bash
curl -fsSL https://github.com/webkubor/reel-kit/releases/download/app-v1.3.0/install.sh | sh
```

Linux 或喜欢命令行的用户，看[开发者文档](#开发者文档)。

## 三步出第一条片子

打开应用后，左栏是**四条视频流水线**，点进去中间是这条流水线的配置，右边是它跑出来的视频。

**第一步 · 写脚本**
点「知识口播」，在「口播脚本」里一行一句地写 —— 一行就是一镜，界面里会实时显示镜数。最长 8 镜，每行不超过 80 字。

```
如果给你一万两黄金，代价是必须在大秦服徭役，你接不接？
```

**第二步 · 选声音**
「声音」这一栏里直接选一个本机音色就能用，不需要登录、不需要额外安装、不要钱。选中之后每一句都会按你写的样子念出来，镜头时长也跟着念白走 —— 念得慢的画面就长，念得快就短。

**第三步 · 导出**
右边的「视频」列表里出现成片后，可以直接播放，或者「保存到本地」存成 MP4。同一份配置还能再生成一条，用来对比效果。

没准备素材也能出片：版式和配色会跟着「视觉风格」走（有 44 套可选）。有自己的图片或视频，就在左栏「素材库」里导入。

## 四条流水线

| 流水线 | 做什么 |
|---|---|
| **知识口播** | 一行一镜的脚本 → 逐句配音 → 按实际语速定每镜时长 → 渲染画面 → 合成。竖屏 1080×1920 |
| **项目推荐** | 填一个 GitHub 仓库地址 → 读 README 和截图 → 分镜 → 逐句旁白 → 带解说的项目介绍视频 |
| **素材合成** | 把素材库里的图片/片段，配上配音和 BGM，按顺序拼成一条并导出 |
| **人物商品介绍** | 给产品或人物出一条介绍片：竖屏出镜画面满屏，横屏左图右文，画幅决定版式 |

## 声音从哪来，要花多少钱

| 你想做的事 | 在哪跑 | 花钱吗 | 要准备什么 |
|---|---|---|---|
| **文字配音**（默认） | 随包自带的离线引擎 | ❌ 不花钱 | 什么都不用装，macOS 和 Windows 是同一套 |
| **克隆自己的声音** / 设计音色 | 本机 VoxFlow | ❌ 不花钱 | macOS Apple Silicon 装一下 VoxFlow；也可以让它跑在另一台机器上 |
| **换个更好的云端音色** | MiniMax TTS | ✅ 按量计费 | 在「设置与反馈 → 云端配音密钥」填一次 MiniMax 密钥 |
| **生成图片 / 视频素材** | MuseAV 中台、Magpie 网关、MiniMax 直连，或你自己的通道 | ✅ 按各自账户额度 | 中台在应用内登录一次；Magpie 走本机已配好的模型；厂商直连填密钥；也可以填自己的中转站 |

几条规则，事先说清楚：

- **你选哪条通道就用哪条，不会偷偷换。** 通道没就绪时，界面会写明原因并禁用生成 —— 不会悄悄改走另一家可能更贵的通道。
- **失败的付费任务不会被重复提交。** 如果素材已经生成、只是下载失败，历史记录里会出现「取回已生成的素材」，取回不重新计费。
- **模型、音色、账户是三件独立的事**，界面上分开设置。

## 画面从哪来

- **自己导入**：左栏「素材库」，图片、视频、音频都能进。
- **AI 生成**：左栏「生成素材」，按你选的通道生成图片或视频，生成结果自动进素材库。
- **自己的中转站**：设置 →「模型通道」里填一次就行，不用改配置文件；密钥不会出现在界面上，只存在你本机。
- **Magpie 网关**：本机装好 [Magpie](https://github.com/yetone/magpie) 就能直接选 —— 不用填地址也不用填密钥，模型清单由网关自己公布；详见 [Magpie 网关通道](docs/magpie-gateway.md)。

## 你的东西在哪儿

素材、文案、成片都留在你自己电脑上（`~/.reel-kit/studio/`），应用默认不上报任何内容。生成 AI 素材时会调用你选的通道，那部分受对应服务的条款约束。

## 出问题了

先试这几件小事：

| 现象 | 多半是 |
|---|---|
| 双击没反应 | 下载的文件被系统标记了 —— Windows 见下一节；macOS 用右键「打开」 |
| 配音没声音 | 通道没就绪 —— 看「声音」一栏旁的提示，它会写明缺什么 |
| 任务失败 | 右栏「视频」列表里点开那条失败任务，能看到失败在第几步、报的是什么错 |
| 想确认版本 | 「设置与反馈 → 软件更新」里显示当前版本，也能在这里检查更新 |

**macOS 说「无法验证开发者」**：这个包没做苹果公证（只做了 ad-hoc 签名）。右键点应用选「打开」，再点一次「打开」就进去了；或者在终端里执行：

```bash
xattr -dr com.apple.quarantine "/Applications/Reel Studio.app"
```

**Windows 弹「Windows 已保护你的电脑」**：点「更多信息」→「仍要运行」。如果双击完全没反应，先在资源管理器里选中 `Reel Studio.exe` → 右键「属性」→ 底部勾「解除锁定」→「确定」，再双击。

**Windows 双击闪退**：在资源管理器地址栏输入 `%TEMP%`，找 `reel-studio-crash.log`，里面记了完整崩溃调用栈。少数启动阶段的错误不会生成这个文件。

**任务失败 / 界面报错 —— 这一步能帮我们直接定位**：

1. 打开「设置与反馈」→「运行日志」→「导出排障报告」，得到一个 `reel-studio-diagnostics-*.txt`。
2. 报告里有：版本与系统、通道就绪状态、**失败任务的报错原文和卡住的那一步**、工作台与界面最近几十行日志。常见的密钥和令牌会自动遮盖。
3. 检查一下有没有你自己的私人信息，然后把它作为附件传到 [问题反馈](https://github.com/webkubor/reel-kit/issues/new/choose)。

也可以点「复制完整诊断与近期日志」，直接贴到 Issue 正文里。工作台日志本体在 `%TEMP%\reel-studio-backend.log`，「打开工作台日志」按钮会直接打开。

## 已知限制

说在前面，免得你踩坑：

- **没有签名和公证**：macOS 首次打开被拦、Windows 被 SmartScreen 拦，都是这个原因，绕过方式在上面。
- **Windows 版仍在真实用户实测阶段**：命令行与后端有 CI 覆盖，图形界面还没有走完完整的真人验收流程。遇到问题请按上一节上报 —— 这正是我们需要的样本。
- **离线音色偏机械**：本机免费配音是够用的口播音色，追求自然度就换云端音色。
- **知识口播固定竖屏 1080×1920**，横屏版式在「人物商品介绍」那条流水线里。
- **不做通用剪辑**：调色、多轨、精确到帧的对齐请用 DaVinci Resolve。

## 开发者文档

想用命令行、接自己的流程、或者改引擎源码，看这里：

- **[命令行与引擎文档](docs/cli.md)** —— 完整参数、声音后端、配乐库、通道管理、仓库结构、设计取舍
- [本地 HTTP 接口](docs/studio-api.md) —— 桌面版和网页版共用一套接口，可以拿它做自己的批量出片工具
- [Magpie 网关通道](docs/magpie-gateway.md) —— 把本机的 Magpie 当统一模型网关接进来：前置条件、只读能力发现、提交即计费与失败取回
- [桌面应用发版](docs/desktop-release.md) —— 构建、打包、签名与发布流程
- [知识短视频生产指南](docs/knowledge-video-guide.md) —— 叙事母版与文案节奏
- [合成笔记](docs/video-compositing-notes.md) —— 音画对齐、时长余量等踩过的坑
- [官网源码](docs/index.html) —— `https://webkubor.github.io/reel-kit/`

一行命令跑通第一条片子（准备一个图片目录，和一份一行一句的文案）：

```bash
npx github:webkubor/reel-kit make \
  --template sticker-promo \
  --title "reel-kit" \
  --assets ./images \
  --caps ./captions.txt \
  --out demo.mp4
```

这条命令直接从 Git 仓库执行，**不经过 npm 仓库** —— 本项目没有发布 npm 包，桌面应用也是下载即用。

## License

MIT
