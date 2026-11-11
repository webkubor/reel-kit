# 桌面版发版（Reel Studio）

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

输出：`Reel Studio.app`、版本化 ZIP、`SHA256SUMS`、`build-info.json`。最后者记录来源 commit、平台、版本与真实签名状态。目录资源不从用户配置目录读取密钥。

在成品应用中实际验证：新建项目 → 可选导入图片 → 中文文案 → 本机离线语音 → 合成 → 视频播放 → 原生保存 → ffprobe 检查导出的尺寸/音轨/时长。另检查项目切换、失败提示、1024/1440 窗口、原图查看、历史提示词与步骤耗时、JSON/表单往返编辑。AI 设置与分镜请求使用隔离模拟服务验证保存/只读检查/成功/失败路径，不能冒充真实云模型联调。通过 `REEL_STUDIO_DATA_DIR` 指向独立绝对目录可避免验收数据写入真实项目；测试密钥仅写本次模拟地址对应的钥匙串条目，验完通过界面移除。真实窗口验证不能由 Go 测试或截图替代。

## 签名与发布

当前脚本执行 ad-hoc 签名和 `codesign --verify --deep --strict`，**未 Apple 公证**，也未声称 Developer ID 签名。下载后 macOS 可能拦截首次启动；发布页必须清楚说明此限制，不给出绕过系统安全策略的安装脚本。

验收通过后按个人仓流程合入远端 main，确认构建 commit 在 main 中，再打 `app-v<version>` tag。禁止把未验收平台的旧包装进新版本。

```bash
# 在仓库根目录；tag 必须指向 build-info.json 中的 commit。
scripts/publish-desktop-release.sh app-v<version> <package-dir> <notes-file> --dry-run
scripts/publish-desktop-release.sh app-v<version> <package-dir> <notes-file>
```

发布脚本核验 checksum、版本、来源 commit、main 祖先关系、tag 与未公证披露；不覆盖已有 release。发完从 GitHub 读回 tag 和资源清单，再下载归档核对 SHA256SUMS。README 和官网同步指向新发布。
