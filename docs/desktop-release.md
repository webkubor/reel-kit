# 桌面版发版手册（Reel Studio）

这份手册记的是「怎么把一次桌面端发版做完，并且验得住」。踩过的坑都写在
对应步骤旁边，别再靠记忆。

## 前提

- `mygo` CLI 在 PATH 里（`~/go/bin/mygo`）。
- Go 1.27.3（本仓 go.mod 要求 ≥1.27.3，本机系统 Go 是 1.27.2，所有构建命令
  都要带 `GOTOOLCHAIN=go1.27.3`）。
- 签名私钥在 kyvault 里：`secret://mygo/reel-studio-update-key`。
- `gh` 已登录，且对 `webkubor/reel-kit` 有发版权限。

## 一、构建

```bash
cd apps/reel-studio
kyvault run --env MYGO_UPDATER_PRIVATE_KEY=secret://mygo/reel-studio-update-key -- \
  env GOTOOLCHAIN=go1.27.3 mygo build
kyvault run --env MYGO_UPDATER_PRIVATE_KEY=secret://mygo/reel-studio-update-key -- \
  env GOTOOLCHAIN=go1.27.3 mygo build -platform windows/amd64 -skip-installer
```

产物在 `apps/reel-studio/build/<目标>/`：

| 文件 | 说明 |
|---|---|
| `Reel Studio.app` | macOS 应用包 |
| `reel-studio-<版本>-<目标>.tar.gz` | macOS 安装归档（安装脚本装的就是它） |
| `reel-studio-<版本>-arm64.dmg` | 磁盘镜像，给手动拖拽安装的人 |
| `reel-studio-<旧>-to-<新>-<目标>.delta` | 增量更新包，每个几百 KB |
| `update-<目标>.json` | 更新清单：版本号、归档 URL、**归档 SHA-256 的 Ed25519 签名** |
| `install.sh` | 一行安装脚本 |

Windows 的 `-skip-installer` 是因为 NSIS 安装器只能在 Windows 上打；Windows 用户
解压后直接跑 `Reel Studio.exe`，**exe 和旁边的 `backend/` `bin/` `chrome/` `museav/`
必须留在同一层目录**，只拷 exe 会起不来。

### ⚠️ 签名盖的是归档的摘要，不是清单文件

`update-<目标>.json` 里的 `signature` 是对**归档的 SHA-256 摘要**做的 Ed25519
签名，清单本身没有签名。验签时要验归档的摘要：

```bash
openssl dgst -sha256 -binary <归档 > digest.bin
openssl pkeyutl -verify -pubin -inkey pub.pem -rawin -in digest.bin -sigfile sig.bin
```

## 二、发布

```bash
scripts/publish-desktop-release.sh app-v0.2.3 --notes /tmp/notes.md
```

它会：把清单复制成安装脚本认的名字 → 逐个归档验签 → 生成 `SHA256SUMS` →
`gh release create` 传全部产物。

### ⚠️ 必须有 `update-<架构>.json`

`install.sh` 在 macOS 上找的是 `update-<架构>.json`（Apple Silicon 上就是
`update-arm64.json`），而 mygo 产出的清单叫 `update-<目标>.json`（目标是签名
身份 `darwin-arm64`）。少了前者，README 里那行

```bash
curl -fsSL .../install.sh | sh
```

会直接失败（找不到清单）。发布脚本已经代为复制，别再手工发版漏掉它。

## 三、发完必做的验证

1. **签名**：对每个归档的 SHA-256 摘要验 Ed25519（见上）。
2. **校验和**：`cd build && shasum -a 256 -c SHA256SUMS`。
3. **安装演练**（别跳过，这是唯一能证明 README 那行命令可用的办法）：

   ```bash
   cd /tmp && curl -fsSL <release>/install.sh -o install.sh
   REEL_STUDIO_INSTALL_DIR=/tmp/rs-install/Applications REEL_STUDIO_NO_OPEN=1 sh install.sh
   ```

   装完核对：应用包版本号、`<应用>/Contents/Resources/backend/src/studio/` 下
   有 `server.mjs` `talk.mjs` `product.mjs`。
4. **包里的后端必须是仓库当前的**：跑 `node scripts/check-bundle-freshness.mjs`
   （已接进 `pnpm check`）。0.2.1 / 0.2.2 就是带着旧后端发的，点「生成口播
   视频」会被后端拒（任务类型不支持）。
5. **启动**：跑一次应用，日志里要有「Reel Studio 启动（版本 x.y.z）」和
   「本地工作台：ready」。日志在 `~/Library/Logs/Reel Studio/`。
6. **页面**：官网 `docs/index.html` 的下载链接要指向新 tag；GitHub Pages 是
   直接从 `main` 的 `docs/` 目录发的，push 后等一两分钟再看。Pages 偶尔并发
   构建失败（`Page build failed`），重新触发一次构建即可。
7. **CI**：`gh run list` 看 `check` 与 `desktop` 两个 job 都绿。

## 四、发版后

- `CHANGELOG.md` 写清这一版改了什么、修了什么缺陷。
- `README.md` / 官网写清安装方式与各平台的坑。
- 删掉临时目录，别把构建产物提交进仓库。
