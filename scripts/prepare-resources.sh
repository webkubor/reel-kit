#!/usr/bin/env bash
# 把桌面版打成自包含的安装包：自带 Node 运行时、ffmpeg/ffprobe、Chrome headless
# 与 reel-kit 后端源码，用户下载后双击就能用，不必先装 Node。
#
# 下载产物不进 git（.gitignore 已排除 resources/），每次发版重新跑本脚本取。
#
#   bash scripts/prepare-resources.sh darwin-arm64
#   bash scripts/prepare-resources.sh darwin-amd64
#   bash scripts/prepare-resources.sh windows-amd64
#
# 产物落在 apps/reel-studio/resources/<platform>/，由 mygo build 打进 App 包。

set -euo pipefail

PLATFORM="${1:-}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RES="$ROOT/apps/reel-studio/resources"
CACHE="$ROOT/.cache/deps"

NODE_VERSION="22.14.0"
CHROME_VERSION="157.0.8103.38"
MUSEAV_VERSION="3.16.0"

# 本机离线语音引擎：sherpa-onnx 的官方预编译包 + 一个中文模型。
#
# 随应用一起打包，用户装完就能配音 —— 不用 clone 仓库、不用装 Python、
# 不用登录、不花钱，macOS 与 Windows 是同一套模型、同一个调用方式。
SHERPA_VERSION="v2.0.1"
SHERPA_MODEL="vits-zh-hf-fanchen-C"
SHERPA_BASE="https://github.com/k2-fsa/sherpa-onnx/releases/download"

case "$PLATFORM" in
  darwin-arm64)
    NODE_ARCHIVE="node-v${NODE_VERSION}-darwin-arm64.tar.gz"
    CHROME_FILE="chrome-headless-shell-mac-arm64.zip"
    CHROME_PLATFORM="mac-arm64"
    FFMPEG_PKG="@ffmpeg-installer/darwin-arm64"; FFMPEG_VER="4.1.5"
    FFPROBE_PKG="@ffprobe-installer/darwin-arm64"; FFPROBE_VER="5.0.1"
    SHERPA_ARCHIVE="sherpa-onnx-${SHERPA_VERSION}-osx-arm64-shared.tar.bz2"
    SHERPA_LIB_ARCHIVE="sherpa-onnx-${SHERPA_VERSION}-osx-arm64-shared-lib.tar.bz2"
    ;;
  darwin-amd64)
    NODE_ARCHIVE="node-v${NODE_VERSION}-darwin-x64.tar.gz"
    CHROME_FILE="chrome-headless-shell-mac-x64.zip"
    CHROME_PLATFORM="mac-x64"
    FFMPEG_PKG="@ffmpeg-installer/darwin-x64"; FFMPEG_VER="4.1.0"
    FFPROBE_PKG="@ffprobe-installer/darwin-x64"; FFPROBE_VER="5.1.0"
    SHERPA_ARCHIVE="sherpa-onnx-${SHERPA_VERSION}-osx-x64-shared.tar.bz2"
    SHERPA_LIB_ARCHIVE="sherpa-onnx-${SHERPA_VERSION}-osx-x64-shared-lib.tar.bz2"
    ;;
  windows-amd64)
    NODE_ARCHIVE="node-v${NODE_VERSION}-win-x64.zip"
    CHROME_FILE="chrome-headless-shell-win64.zip"
    CHROME_PLATFORM="win64"
    FFMPEG_PKG="@ffmpeg-installer/win32-x64"; FFMPEG_VER="4.1.0"
    FFPROBE_PKG="@ffprobe-installer/win32-x64"; FFPROBE_VER="5.1.0"
    SHERPA_ARCHIVE="sherpa-onnx-${SHERPA_VERSION}-win-x64-shared-MD-Release.tar.bz2"
    SHERPA_LIB_ARCHIVE="sherpa-onnx-${SHERPA_VERSION}-win-x64-shared-MD-Release-lib.tar.bz2"
    ;;
  "")
    echo "用法：bash scripts/prepare-resources.sh <darwin-arm64|darwin-amd64|windows-amd64>" >&2
    exit 2 ;;
  *)
    echo "不支持的平台：$PLATFORM" >&2; exit 2 ;;
esac

mkdir -p "$CACHE"

fetch() {
  local url="$1" dest="$2"
  if [ -f "$dest" ]; then return 0; fi
  echo "下载 $(basename "$dest")…"
  curl -fsSL --retry 3 --max-time 600 -o "$dest.part" "$url"
  mv "$dest.part" "$dest"
}

# ── Node 运行时 ─────────────────────────────────────────────
fetch "https://nodejs.org/dist/v${NODE_VERSION}/${NODE_ARCHIVE}" "$CACHE/$NODE_ARCHIVE"

node_dir="$CACHE/node-$PLATFORM"
rm -rf "$node_dir"; mkdir -p "$node_dir"
if [ "$PLATFORM" = "windows-amd64" ]; then
  unzip -q "$CACHE/$NODE_ARCHIVE" -d "$node_dir"
  mv "$node_dir"/node-v${NODE_VERSION}-win-x64/* "$node_dir"/
  rmdir "$node_dir"/node-v${NODE_VERSION}-win-x64
else
  tar xzf "$CACHE/$NODE_ARCHIVE" -C "$node_dir" --strip-components=1
fi

# ── ffmpeg / ffprobe ────────────────────────────────────────
fetch "https://registry.npmjs.org/${FFMPEG_PKG}/-/$(basename "$FFMPEG_PKG")-${FFMPEG_VER}.tgz" "$CACHE/ffmpeg-$PLATFORM.tgz"
fetch "https://registry.npmjs.org/${FFPROBE_PKG}/-/$(basename "$FFPROBE_PKG")-${FFPROBE_VER}.tgz" "$CACHE/ffprobe-$PLATFORM.tgz"

# ── Chrome headless shell（渲染视频帧用） ───────────────────
fetch "https://storage.googleapis.com/chrome-for-testing-public/${CHROME_VERSION}/${CHROME_PLATFORM}/${CHROME_FILE}" "$CACHE/$CHROME_FILE"

# ── 组装 resources/<platform>/ ──────────────────────────────
out="$RES/$PLATFORM"
rm -rf "$out"
mkdir -p "$out/bin"

if [ "$PLATFORM" = "windows-amd64" ]; then
  cp "$node_dir/node.exe" "$out/bin/node.exe"
else
  cp "$node_dir/bin/node" "$out/bin/node"
  chmod +x "$out/bin/node"
fi

extract_ffbin() {
  local tgz="$1" want="$2" dest="$3"
  rm -rf "$CACHE/_ffx"; mkdir -p "$CACHE/_ffx"
  tar xzf "$tgz" -C "$CACHE/_ffx"
  local found
  found="$(find "$CACHE/_ffx" -type f -name "$want" | head -1)"
  [ -n "$found" ] || { echo "在 $(basename "$tgz") 里找不到 $want" >&2; exit 1; }
  cp "$found" "$dest"
  chmod +x "$dest"
}

if [ "$PLATFORM" = "windows-amd64" ]; then
  extract_ffbin "$CACHE/ffmpeg-$PLATFORM.tgz" "ffmpeg.exe" "$out/bin/ffmpeg.exe"
  extract_ffbin "$CACHE/ffprobe-$PLATFORM.tgz" "ffprobe.exe" "$out/bin/ffprobe.exe"
else
  extract_ffbin "$CACHE/ffmpeg-$PLATFORM.tgz" "ffmpeg" "$out/bin/ffmpeg"
  extract_ffbin "$CACHE/ffprobe-$PLATFORM.tgz" "ffprobe" "$out/bin/ffprobe"
fi

chrome_dir="$CACHE/chrome-$PLATFORM"
if [ ! -d "$chrome_dir" ]; then
  rm -rf "$chrome_dir"; mkdir -p "$chrome_dir"
  unzip -q "$CACHE/$CHROME_FILE" -d "$chrome_dir"
  inner="$(find "$chrome_dir" -maxdepth 1 -mindepth 1 -type d | head -1)"
  if [ -n "$inner" ]; then
    mv "$inner"/* "$chrome_dir"/ 2>/dev/null || true
    rmdir "$inner" 2>/dev/null || true
  fi
fi
mkdir -p "$out/chrome"
cp -R "$chrome_dir"/* "$out/chrome/" 2>/dev/null || true
if [ "$PLATFORM" = "windows-amd64" ]; then
  chmod +x "$out/chrome/chrome-headless-shell.exe" 2>/dev/null || true
else
  chmod +x "$out/chrome/chrome-headless-shell" 2>/dev/null || true
fi

# ── 后端源码（Node 侧） ─────────────────────────────────────
backend="$out/backend"
mkdir -p "$backend"
for d in src bin templates prompts studio; do
  cp -R "$ROOT/$d" "$backend/"
done
cp "$ROOT/package.json" "$backend/"

# 后端运行时依赖：playwright-core（无其它依赖）
mkdir -p "$backend/node_modules"
rm -rf "$backend/node_modules/playwright-core"
cp -R "$ROOT/node_modules/.pnpm/playwright-core@1.62.1/node_modules/playwright-core" "$backend/node_modules/"

# ── MuseAV 中台 CLI（出图/出视频走它） ──────────────────────
# 装到 resources/<platform>/museav/，再在 bin/ 放一个转发脚本。
# 用户机器上没装 museav 也能用；登录态仍读 ~/.museav.json（用户自己登录）。
#
# 刻意不装可选依赖：onnxruntime-node（293M）与 sharp（本地抠图/超分用），
# 出图出视频一个都不需要。命令缺失时 CLI 自己会提示装可选依赖。
museav_dir="$CACHE/museav-$PLATFORM"
if [ ! -d "$museav_dir/node_modules/museav-cli" ]; then
  rm -rf "$museav_dir"; mkdir -p "$museav_dir"
  # 自带一份 package.json：否则 npm 会往上找父目录的 package.json，
  # 把依赖装进仓库根目录并改写根 package.json。
  printf '{"name":"reel-studio-museav","private":true}\n' > "$museav_dir/package.json"
  (cd "$museav_dir" && NPM_GUARD_OFF=1 npm install --omit=optional --no-audit --no-fund \
    --no-package-lock "museav-cli@${MUSEAV_VERSION}" >/dev/null)
fi
rm -rf "$out/museav"
mkdir -p "$out/museav"
cp -R "$museav_dir/node_modules" "$out/museav/"

if [ "$PLATFORM" = "windows-amd64" ]; then
  cat > "$out/bin/museav.cmd" <<'EOF'
@echo off
"%~dp0node.exe" "%~dp0..\museav\node_modules\museav-cli\dist\index.js" %*
EOF
else
  cat > "$out/bin/museav" <<'EOF'
#!/bin/sh
# MuseAV CLI 转发脚本：用自带的 Node 跑打包进来的 museav-cli。
here="$(cd "$(dirname "$0")" && pwd)"
exec "$here/node" "$here/../museav/node_modules/museav-cli/dist/index.js" "$@"
EOF
  chmod +x "$out/bin/museav"
fi

# ── 本机离线语音引擎（随应用分发） ─────────────────────────
# 装到 resources/<platform>/sherpa/：
#   bin/   引擎二进制（Windows 上 DLL 放这里，和 exe 同目录才找得到）
#   lib/   macOS 的动态库（运行时用 DYLD_LIBRARY_PATH 指过去）
#   model/ 中文模型
#
# 后端按自己所在位置往上找三层：<resources>/backend/src/studio → <resources>/sherpa。
sherpa_dir="$CACHE/sherpa-$PLATFORM"
if [ ! -d "$sherpa_dir/model" ]; then
  rm -rf "$sherpa_dir"; mkdir -p "$sherpa_dir"
  fetch "$SHERPA_BASE/$SHERPA_VERSION/$SHERPA_ARCHIVE" "$CACHE/$SHERPA_ARCHIVE"
  fetch "$SHERPA_BASE/$SHERPA_VERSION/$SHERPA_LIB_ARCHIVE" "$CACHE/$SHERPA_LIB_ARCHIVE"
  tar xjf "$CACHE/$SHERPA_ARCHIVE" -C "$sherpa_dir" --strip-components=1
  tar xjf "$CACHE/$SHERPA_LIB_ARCHIVE" -C "$sherpa_dir" --strip-components=1
  fetch "$SHERPA_BASE/tts-models/${SHERPA_MODEL}.tar.bz2" "$CACHE/${SHERPA_MODEL}.tar.bz2"
  mkdir -p "$sherpa_dir/model"
  tar xjf "$CACHE/${SHERPA_MODEL}.tar.bz2" -C "$sherpa_dir/model" --strip-components=1
fi
rm -rf "$out/sherpa"
mkdir -p "$out/sherpa/bin" "$out/sherpa/model"
# 只留用得上的那一个二进制：官方包里还带着几十个识别、说话人工具，
# 全塞进安装包会白白多出一百多兆。
if [ -f "$sherpa_dir/bin/sherpa-onnx-offline-tts" ]; then
  cp "$sherpa_dir/bin/sherpa-onnx-offline-tts" "$out/sherpa/bin/"
elif [ -f "$sherpa_dir/bin/sherpa-onnx-offline-tts.exe" ]; then
  cp "$sherpa_dir/bin/sherpa-onnx-offline-tts.exe" "$out/sherpa/bin/"
else
  echo "找不到 sherpa-onnx-offline-tts，引擎包没下全" >&2
  exit 1
fi
cp -R "$sherpa_dir/model/." "$out/sherpa/model/"
if [ -d "$sherpa_dir/lib" ]; then
  mkdir -p "$out/sherpa/lib"
  # macOS 的动态库靠 DYLD_LIBRARY_PATH 找；Windows 只认 exe 同目录。
  find "$sherpa_dir/lib" -maxdepth 1 \( -name 'libonnxruntime*.dylib' -o -name '*.dll' \) -exec cp {} "$out/sherpa/lib/" \;
  if [ "$PLATFORM" = "windows-amd64" ]; then
    find "$sherpa_dir/lib" -maxdepth 1 -name '*.dll' -exec cp {} "$out/sherpa/bin/" \; 2>/dev/null || true
  fi
fi
chmod +x "$out/sherpa/bin/sherpa-onnx-offline-tts" 2>/dev/null || true

echo "完成：$out"
du -sh "$out"
