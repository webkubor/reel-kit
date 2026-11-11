# Reel Studio 桌面壳（Wails v2）

这是桌面版的**壳**，不是功能本身。功能全在本机工作台（Node 服务）里，浏览器和桌面跑的是同一份网页代码。

壳只做三件事：

1. 把本机工作台起起来（自带 Node 运行时，不要求用户装 Node）
2. 开一个原生窗口，跳到工作台页面
3. 退出时把自己起的工作台收掉

## 为什么换到 Wails

旧壳用 MyGo 自绘 UI，功能多但兼容性与维护成本高。Wails 用系统 WebView，
前端就是工作台那份网页，桌面端不会再出现「少一个功能」这种两份维护出来的问题。
目录 `../reel-studio` 是旧 MyGo 壳，仍在发版；两者并行，等这里可用后再切换。

## 目录

```
main.go            Wails 入口：窗口、单实例、退出回收
backend.go         工作台进程管理：动态端口、自带运行时、环境注入
bridge.go          给引导页回答「工作台好了没」
proc_unix.go       退出时回收整个进程组（连 ffmpeg 一起收掉）
proc_windows.go    同上，Windows 版（本轮未用，保留编译通过）
frontend/dist/     引导页：等工作台就绪后跳过去
scripts/package.sh 打成下载即用的 .app（把运行时打进 Contents/Resources/runtime）
```

## 开发

```bash
# 起一套隔离数据的工作台，端口自己挑，不会跟人撞车。
REEL_STUDIO_RUNTIME=../reel-studio/resources/darwin-arm64 \
REEL_STUDIO_HOME=$(mktemp -d) \
  go run . dev
```

## 打包（下载即用）

```bash
scripts/package.sh          # darwin/arm64
scripts/package.sh --dmg    # 再压一个 DMG
```

产物在 `build/bin/Reel Studio.app`（`--dmg` 再出一个 dmg）。
`scripts/package.sh` 会自动把工作台要用的 Node / ffmpeg / Chrome headless / 后端代码
打进应用包。用户拖进「应用程序」双击即可。

## 还没做的

- 签名与公证（当前是 Wails 的 ad-hoc 自签名，首次打开会被 macOS 拦一下）
- 更新机制（旧壳那套 MyGo updater 不能直接沿用，要另起）
- Windows 打包（WebView2 分发，本轮暂缓，macOS 优先）