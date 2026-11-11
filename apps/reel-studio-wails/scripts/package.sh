#!/bin/bash
# 把 Wails 桌面壳打成「下载即用」的应用包。
#
#   scripts/package.sh                     # 打包当前平台
#   scripts/package.sh darwin/arm64 arm64  # 指定平台与 CPU（见 --help）
#   scripts/package.sh --dmg               # 再压一个 DMG
#
# 只做三件事：
#   1. wails build 出 .app
#   2. 把工作台要用的运行时（Node / ffmpeg / Chrome headless / 后端代码）
#      rsync 进 .app 的 Contents/Resources/runtime
#   3. （--dmg）用 hdiutil 压成 DMG
#
# 没有这一步，.app 只是个空壳：它会在启动时找 REEL_STUDIO_RUNTIME，
# 没有就起不来。打包后用户直接拖进「应用程序」双击即可，不用装 Node、不用命令行。
set -euo pipefail

cd "$(dirname "$0")/.."

PLATFORM="${1:-darwin/arm64}"
GOARCH_OVERRIDE="${2:-}"
MAKE_DMG=0
if [ "${1:-}" = "--dmg" ]; then
  MAKE_DMG=1
  PLATFORM="darwin/arm64"
fi

case "$PLATFORM" in
  darwin/arm64) RES_DIR="darwin-arm64" ;;
  darwin/amd64) RES_DIR="darwin-amd64" ;;
  windows/amd64) RES_DIR="windows-amd64" ;;
  *) echo "不支持的平台：$PLATFORM" >&2; exit 2 ;;
esac

WAILS_BIN="${WAILS_BIN:-}"
if [ -z "$WAILS_BIN" ]; then
  WAILS_BIN="$(command -v wails || echo "$HOME/go/bin/wails")"
fi
[ -x "$WAILS_BIN" ] || { echo "找不到 wails CLI（${WAILS_BIN}），先装：go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1" >&2; exit 3; }

# 运行时资源目录：默认在旧壳的资源目录里，目前两个壳共用一份。
# 它是 scripts/prepare-resources.sh 的产物，不在 git 里（跟 node/ffmpeg/chrome 这些
# 大二进制一样是构建输出）。缺了就先跑：scripts/prepare-resources.sh。
RESOURCES="${REEL_STUDIO_RUNTIME_SRC:-$PWD/../reel-studio/resources/$RES_DIR}"
[ -d "$RESOURCES" ] || { echo "找不到运行时资源目录：${RESOURCES}" >&2; echo "先跑 scripts/prepare-resources.sh 生成，或设 REEL_STUDIO_RUNTIME_SRC 指向已有资源。" >&2; exit 4; }

VERSION="${REEL_STUDIO_VERSION:-$(node -e 'process.stdout.write(require("../reel-studio/mygo.json").version)' 2>/dev/null || echo "dev")}"
APP_NAME="Reel Studio"

echo "== 1/3 构建 ${APP_NAME}（${PLATFORM}）"
if [ -n "$GOARCH_OVERRIDE" ]; then
  GOARCH="$GOARCH_OVERRIDE" "$WAILS_BIN" build -platform "$PLATFORM" -clean
else
  "$WAILS_BIN" build -platform "$PLATFORM" -clean
fi

APP="build/bin/$APP_NAME.app"
[ -d "$APP" ] || { echo "构建失败：没看到 ${APP}" >&2; exit 5; }

echo "== 2/3 把运行时打进应用包"
DEST="$APP/Contents/Resources/runtime"
rm -rf "$DEST"
mkdir -p "$DEST"
# 逐目录拷，不用 --delete-protected 之类破坏性参数；runtime 目录是我们自己建的。
rsync -a "$RESOURCES/" "$DEST/"
echo "    运行时大小：$(du -sh "${DEST}" | awk '{print $1}')"

# 核对关键文件，缺一个应用就起不来。
for rel in bin/node backend/bin/reel.mjs bin/ffmpeg bin/ffprobe; do
  [ -e "$DEST/$rel" ] || { echo "打包不完整：缺少 runtime/${rel}" >&2; exit 6; }
done
echo "    关键运行文件齐全"

echo "== 3/3 完成"
APP_SIZE="$(du -sh "${APP}" | awk '{print $1}')"
echo "应用包：${APP}（${APP_SIZE}）"

if [ "$MAKE_DMG" = "1" ]; then
  OUT="build/reel-studio-${VERSION}-${RES_DIR}.dmg"
  rm -f "$OUT"
  hdiutil create -volname "$APP_NAME" -srcfolder "$APP" -ov -format UDZO "$OUT"
  echo "DMG：${OUT}"
fi