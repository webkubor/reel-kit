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
source_app="$here/build/bin/muse-tv.app"
if [[ -e "$output" ]]; then
  printf '输出目录已存在，拒绝覆盖：%s\n' "$output" >&2
  exit 1
fi
for item in "$source_app" "$resources/sherpa" "$resources/chrome/chrome-headless-shell" "$resources/bin/ffmpeg" "$resources/bin/ffprobe" "$resources/bin/museav" "$resources/bin/node" "$resources/museav/node_modules/museav-cli"; do
  if [[ ! -e "$item" ]]; then
    printf '缺少构建资源：%s\n' "$item" >&2
    exit 1
  fi
done
mkdir -p "$output"
app="$output/muse-tv.app"
ditto "$source_app" "$app"
mkdir -p "$app/Contents/Resources/bin"
ditto "$resources/sherpa" "$app/Contents/Resources/sherpa"
ditto "$resources/chrome" "$app/Contents/Resources/chrome"
cp "$resources/bin/ffmpeg" "$resources/bin/ffprobe" "$app/Contents/Resources/bin/"
# MuseAV 中台：角色素材库与云端素材都靠它。少了它，桌面版点「新建角色」
# 只会得到「找不到 museav 命令」—— 用户机器上多半根本没装。
# 带上自带的 node 与 CLI，用户登录态仍读本机的 ~/.museav.json。
ditto "$resources/museav" "$app/Contents/Resources/museav"
cp "$resources/bin/museav" "$resources/bin/node" "$app/Contents/Resources/bin/"
codesign --force --deep --sign - "$app"
codesign --verify --deep --strict "$app"
printf '本地测试应用：%s\n' "$app"
printf '签名：ad-hoc；未 Apple 公证。分发说明必须保留此状态。\n'
