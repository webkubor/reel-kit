# 桌面版发版（muse-tv）

当前桌面产品为 `apps/reel-studio-wails`：Wails 原生窗口、Go 业务引擎、绑定调用，无 Node 运行时或本地 HTTP 业务服务。1.6.0 接替 1.5.1；MVP 是功能范围，不回退应用版本号。

## 支持范围

当前打包入口交付 macOS Apple Silicon，最低 macOS 13。其他平台的编译或单测不等于安装包验收，不沿用旧平台包冒充新版本。自动更新迁移尚未实现，这次通过下载新包手动替换应用；保留旧数据，不自动删除。

语音使用已有 sherpa-onnx 中文模型，渲染使用 Chrome headless，编码使用 ffmpeg/ffprobe。复用已有离线资源目录，但只打包这些原生资源，不打包其中的 Node、脚本服务器或云服务凭据。

## 构建与验收

依赖：Go（版本见 go.mod）、Wails v2 CLI、开发期 Node/npm（仅编译 Tailwind）、已有离线资源、GitHub CLI。

```bash
cd apps/reel-studio-wails
go vet ./...
go test -race -count=1 ./...
# 使用实际资源中的 ffmpeg 和 sherpa 跑出片；真实模型测试不得当作云 TTS 测试。
PATH="<resources>/bin:$PATH" REEL_TEST_REAL_SHERPA=1 \
  REEL_SHERPA_HOME=<resources>/sherpa go test -count=1 ./...

# 提交源码后，从新目录出包；输出已存在则拒绝覆盖。
WAILS_BIN=<installed-wails-cli> bash scripts/package.sh <new-output-directory> <resources>
```

输出：`muse-tv.app`、版本化 ZIP、`SHA256SUMS`、`build-info.json`。最后者记录来源 commit、平台、版本与真实签名状态。目录资源不从用户配置目录读取密钥。

在成品应用中实际验证：新建项目 → 可选导入图片 → 中文文案 → 本机离线语音 → 合成 → 视频播放 → 原生保存 → ffprobe 检查导出的尺寸/音轨/时长。另检查项目切换、失败提示、1024/1440 窗口、原图查看、历史提示词与步骤耗时、JSON/表单往返编辑。AI 设置与分镜请求使用隔离模拟服务验证保存/只读检查/成功/失败路径，不能冒充真实云模型联调。通过 `REEL_STUDIO_DATA_DIR` 指向独立绝对目录可避免验收数据写入真实项目；测试密钥仅写本次模拟地址对应的钥匙串条目，验完通过界面移除。真实窗口验证不能由 Go 测试或截图替代。

## 当前版本验收状态

1.9.0 的验收结论，逐条如实记录（未做的写未做，不拿编译通过代替）：

> 1.8.1 已于 2026-11-17 发布；本节对应 1.9.0（`app-v1.9.0`）。

**已用真实数据验收**

- 角色库：18 个角色详情逐个打开，48 张图零破图；真人参考独立分区并标注「非角色形象」；画面素材标注「不参与角色物料」。
- 角色头像：导出 14 个角色的头像拼成对照图逐一核对，人物与角色一一对应。修正过两个真 bug —— 真人照被当成头像（判断用了镜像里的名字，而素材库里那张图实际叫「真人参考」）、三视图拼图被当成头像。
- 场景快照：4 个场景的参考图与画面描述逐张比对，图与描述完全对应（烈阳城·黄沙大漠 / 丹阳城·王府长廊 / 孤山·雪巅 / 雨夜长街·天门关）。
- 断网可用：把 MUSE AV 命令指向不存在的路径，角色 18 个、场景 4 个、场景图 4 张全部照常显示。
- 短视频成片：配音 2/2 → 渲染画面帧 → 合成成片全绿，产出 1080×1920、h264+aac、5.23 秒的 mp4（ffprobe 核验）。
- 短剧成片：给本机成片加两句旁白重混，产出「一支笔-成片.mp4」，无重复产物。
- 剧集导入：13 集（序章 + 12 集）从 YouTube 频道 RSS 导入，标题、链接、内嵌播放均正常；序章编号 0 排在最前。
- 界面回归：`bash scripts/run-ui-regression.sh` 覆盖引导页 → 建项目 → 建剧集 → 建镜头 → 短视频项目，10 项全过、页面零错误。

**未验收（不得当作已通过）**

- **真实桌面窗口验收未做**：验收期间屏幕一直锁定（`IOConsoleLocked = true`），截不到窗口、点不了原生控件。原生窗口、菜单栏、系统文件选择器、导出对话框这些只有真窗口才有的路径**一次都没点过**。上文的界面验收都是用无界面浏览器打开同一个前端做的。
- **出镜（联网生成视频）未跑**：要提交 H3 套餐任务并计费，属于付费动作，未擅自执行。短剧成片因此是用本机成片素材验证的，不是「出镜 → 合成」的真实链路。
- **Apple 公证未做**：只有 ad-hoc 本机签名。
- **13 集的出场角色与分镜是空的**：YouTube 只提供标题与链接，这些信息视频里没有。工作台已把这一点显式标出（侧栏「待分镜」标记 + 本集状态面板），没有编造内容。

## 签名与发布

当前脚本执行 ad-hoc 签名和 `codesign --verify --deep --strict`，**未 Apple 公证**，也未声称 Developer ID 签名。下载后 macOS 可能拦截首次启动；发布页必须清楚说明此限制，不给出绕过系统安全策略的安装脚本。

验收通过后按个人仓流程合入远端 main，确认构建 commit 在 main 中，再打 `app-v<version>` tag。禁止把未验收平台的旧包装进新版本。

```bash
# 在仓库根目录；tag 必须指向 build-info.json 中的 commit。
scripts/publish-desktop-release.sh app-v<version> <package-dir> <notes-file> --dry-run
scripts/publish-desktop-release.sh app-v<version> <package-dir> <notes-file>
```

发布脚本核验 checksum、版本、来源 commit、main 祖先关系、tag 与未公证披露；不覆盖已有 release。发完从 GitHub 读回 tag 和资源清单，再下载归档核对 SHA256SUMS。README 和官网同步指向新发布。
