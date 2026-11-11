# Reel Studio · 本地视频工作台

把文案和图片变成短视频：逐句配音、按语音时长渲染、合成、播放并导出 MP4。

桌面应用采用 **Wails + Go**，不携带 Node 运行时、不启动本地 HTTP 业务服务。离线中文语音、Chrome 渲染器和 ffmpeg 随包提供，制作过程在本机完成。

## 下载

**1.6.0** · macOS Apple Silicon · macOS 13 或更新。

- [下载 ZIP](https://github.com/webkubor/reel-kit/releases/download/app-v1.6.0/reel-studio-1.6.0-darwin-arm64.zip)
- [发布说明与 SHA256SUMS](https://github.com/webkubor/reel-kit/releases/tag/app-v1.6.0)
- [官网](https://webkubor.github.io/reel-kit/)

解压后将 `Reel Studio.app` 放入应用程序目录。当前包为 **ad-hoc 签名、未 Apple 公证**，macOS 可能拦截首次启动；这不是已经过 Developer ID 验证的分发包。

1.6.0 是当前桌面产品，接替原实现。采用手动下载安装；暂不提供自动升级、旧数据自动迁移和本版 Windows 安装包。旧项目数据不会被删除。

## 三步出片

1. 新建项目，填写标题和介绍词。每行一句、每句一镜，最多 8 句、每句 80 字。
2. 可选导入图片作为画面，选择竖版 1080×1920 或横版 1920×1080，点击「开始制作」。没有图片时使用占位画面。
3. 在右侧播放成片，从「项目成片」找回历史结果，点击「导出成片」打开系统保存对话框。

默认且当前界面唯一提供的配音通道是 **sherpa-onnx 本机离线中文语音**，不联网、不按调用计费。逐句配音决定镜头时长。制作中可以取消；重启后未完成任务标记为中断，不自动重提。

## MVP 范围

| 已提供 | 当前边界 |
|---|---|
| 人物/商品介绍短片，横竖两种画幅 | 未提供通用时间线剪辑 |
| 项目、图片素材、成片历史、任务过程 | 编辑草稿仅在当前进程内按项目暂存，退出不保存 |
| 本机离线配音、渲染、合成 | 尚未开放云 TTS、声音克隆、AI 素材生成界面 |
| 成片播放与原生导出 | 编辑画面是示意，最终效果以成片为准 |

当前桌面 MVP 没有知识口播、GitHub 推荐、44 套视觉风格、云端账户设置等入口；不将历史实现的能力表述为新版已经提供。

## 数据与问题反馈

桌面数据位于 `~/.reel-kit/studio-desktop/`，素材与成片留在本机。未读取或迁移旧工作台数据目录。

制作失败时查看「制作过程」中的失败步骤与可复制错误。反馈时注明应用版本、系统、复现操作；检查并移除错误文本中的私人路径，再提交 [Issue](https://github.com/webkubor/reel-kit/issues/new/choose)。

## 开发者文档

- 当前桌面应用：`apps/reel-studio-wails`；[设计约定](apps/reel-studio-wails/DESIGN.md)、[构建与发版](docs/desktop-release.md)。
- [命令行与引擎文档](docs/cli.md)：仓库仍提供独立 CLI/库，和桌面运行时分开。
- [合成笔记](docs/video-compositing-notes.md)：音画对齐、时长余量等。
- [知识短视频生产指南](docs/knowledge-video-guide.md)：CLI 内容工作流参考。

```bash
cd apps/reel-studio-wails
go test -race ./...
```

测试需要 ffmpeg/Chrome 才能覆盖真实渲染；缺依赖时相关用例会跳过。真实 sherpa 测试配置见发版手册。Go 测试不能替代打包后窗口交互验收。

CLI 示例（开发机器使用 Node，不表示桌面包携带 Node）：

```bash
npx github:webkubor/reel-kit make \
  --template sticker-promo --title "reel-kit" \
  --assets ./images --caps ./captions.txt --out demo.mp4
```

## License

MIT
