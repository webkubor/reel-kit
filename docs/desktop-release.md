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

先加 `--dry-run` 跑一遍，只检查不发布：tag 与 `mygo.json` 的版本对不上、
清单里的版本是旧的、产物缺失、签名不通过，任何一条都会当场停。

它会：按清单把归档找齐（缺一个就停）→ 逐个归档验签 → 生成扁平文件名的
`SHA256SUMS` → `gh release create` 传全部产物。同名产物只发一次
（清单点名的东西不会再被通配符重复收集）。

### 只发一个平台时（macOS 先闭环）

Windows 保持旧版本、这次只发 macOS 时，用两个开关：

```bash
scripts/publish-desktop-release.sh app-v1.5.0 --notes /tmp/notes.md \
	--platform darwin/arm64 --carry windows/amd64
```

- `--platform`：**只发**列出的平台。没点名的平台整目录跳过（包括它的清单），
  点名的平台仍然要求清单版本等于当前版本 —— 「旧构建混进来」这道防线不因为
  只发一个平台就失效。构建步骤也跟着走：只发 macOS 就不会再构建一遍 Windows。
- `--carry`：把另一个平台**已发布**的清单原样带上（内容一个字都不改）。
  这一步不能省：安装版的更新检查按「最新发布页」找清单，只发 macOS 时新发布页上
  没有 Windows 的清单，Windows 用户的「检查更新」会直接报 404。
  清单里指向的归档仍在旧发布页上，照样能用。

### 清单的名字与安装脚本要对得上

macOS 的 `install.sh` 按架构找清单：`update-darwin-<架构>.json`（Apple Silicon 上
就是 `update-darwin-arm64.json`）。mygo 在 macOS 上的目标名恰好带上了架构，
所以 `build/<目标>/` 里产出的清单文件名直接就是对的那一个 —— **发版时别手动
改名，也别只发其中一个**。发完用这行确认发布页上的清单能取到：

```bash
curl -fsSLO https://github.com/webkubor/reel-kit/releases/download/<tag>/update-darwin-arm64.json
```

（Windows 目标不需要这一步，Windows 用 `update-<目标>.json`。）

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
8. **自动更新链路**（发增量包的那一版必做）：签名对了不等于装得上，
   要拿发布页上的旧归档 + 增量包真的走一遍生产代码：

   - 用发布页清单里的公钥验增量包的签名（验的是**增量包 SHA-256 的摘要**）；
   - `internal/update.ApplyDelta` 把旧应用还原成新版，逐文件比对 SHA-256；
   - 走生产同一条路径 `installUpdate` 把装着旧版的 `.app` 原地换成新版，
     核对换完之后的版本号。

   MyGo 的 `internal/update` 是内部包，只能在它的模块副本里写测试 ——
   仓库里已经备好了测试文件与脚本：

   ```bash
   bash scripts/verify-desktop-update.sh <tag>          # 打印要准备什么
   bash scripts/verify-desktop-update.sh <tag> --run    # 自动下载归档并跑完两段验证
   ```

   脚本会把 MyGo 模块复制一份到临时目录，把
   `scripts/desktop-update-test/` 下的两个测试放进去再跑 ——
   跑的仍然是发布用的那份生产代码（`ApplyDelta` 与 `installUpdate`）。

   2026-11-09 用发布页上的 0.2.2 归档与 0.2.3 增量包跑过：6056 个文件逐个
   SHA-256 一致，原地升级后应用包变成 0.2.3，升级后的应用仍能出一条
   1080×1920 的竖版视频。

   2026-11-10 用 1.0.0 → 1.1.0 再跑一遍：增量包还原出 6056 个文件逐个
   SHA-256 一致；`installUpdate` 只下了 717 KB 增量包就把装着 1.0.0 的
   `.app` 换成 1.1.0，换完的应用能起来（日志「Reel Studio 启动（版本 1.1.0）」
   且本机工作台 ready）。

## 四、发版后

- `CHANGELOG.md` 写清这一版改了什么、修了什么缺陷。
- `README.md` / 官网写清安装方式与各平台的坑。
- 删掉临时目录，别把构建产物提交进仓库。
