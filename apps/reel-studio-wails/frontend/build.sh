#!/usr/bin/env bash
# 生成前端产物：Tailwind 编译 + 拷贝静态文件。
#
# 产物提交进仓库，wails build 不再需要 node —— 桌面版用户装的是应用，
# 不是源码。改样式时在本地跑一次这个脚本，把 dist 一起提交。
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC="$HERE/src"
DIST="$HERE/dist"

cd "$HERE"

rm -rf "$DIST"
mkdir -p "$DIST"

npx --yes tailwindcss@3.4.17 \
  -c "$HERE/tailwind.config.js" \
  -i "$SRC/input.css" \
  -o "$DIST/app.css" \
  --minify

cp "$SRC/index.html" "$DIST/index.html"
cp "$SRC/app.js" "$DIST/app.js"
cp "$SRC/select.js" "$DIST/select.js"

echo "前端产物已生成：$DIST"
