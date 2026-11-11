#!/usr/bin/env bash
# Assemble a macOS development build using already-prepared offline resources.
# Usage: bash package-local.sh /path/to/new-output-directory /path/to/darwin-arm64-resources
set -euo pipefail

if [[ $# -ne 2 ]]; then
  printf '用法：bash package-local.sh <新输出目录> <darwin-arm64 资源目录>\n' >&2
  exit 2
fi
output=$1
resources=$2
here=$(cd "$(dirname "$0")" && pwd)
source_app="$here/build/bin/Reel Studio.app"
if [[ -e "$output" ]]; then
  printf '输出目录已存在，拒绝覆盖：%s\n' "$output" >&2
  exit 1
fi
for item in "$source_app" "$resources/sherpa" "$resources/chrome/chrome-headless-shell" "$resources/bin/ffmpeg" "$resources/bin/ffprobe"; do
  if [[ ! -e "$item" ]]; then
    printf '缺少构建资源：%s\n' "$item" >&2
    exit 1
  fi
done
mkdir -p "$output"
app="$output/Reel Studio.app"
ditto "$source_app" "$app"
mkdir -p "$app/Contents/Resources/bin"
ditto "$resources/sherpa" "$app/Contents/Resources/sherpa"
ditto "$resources/chrome" "$app/Contents/Resources/chrome"
cp "$resources/bin/ffmpeg" "$resources/bin/ffprobe" "$app/Contents/Resources/bin/"
codesign --force --deep --sign - "$app"
codesign --verify --deep --strict "$app"
printf '本地测试应用：%s\n' "$app"
printf '注意：这是临时自签名构建，未公证，不能作为正式分发包。\n'
