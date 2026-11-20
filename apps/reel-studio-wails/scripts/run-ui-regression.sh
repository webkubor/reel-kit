#!/usr/bin/env bash
# 跑一遍界面回归：从引导页到建项目、建剧集、建镜头、切短视频项目。
#
# 用法：bash scripts/run-ui-regression.sh [端口]
# 前提：本地已经用 wails dev 起好开发服务（默认 127.0.0.1:34117）。
set -euo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
port=${1:-34117}
# chrome-headless-shell 随离线资源分发：优先用环境变量，否则在本机几个常见位置找。
chrome=${REEL_CHROME:-}
if [[ -z "$chrome" ]]; then
  for candidate in \
    "$here/../resources/darwin-arm64/chrome/chrome-headless-shell" \
    "$HOME/.cache/deps/chrome-darwin-arm64/chrome-headless-shell" \
    "/Users/webkubor/dev/video/muse-tv/apps/reel-studio-wails/resources/darwin-arm64/chrome/chrome-headless-shell"; do
    [[ -x "$candidate" ]] && { chrome="$candidate"; break; }
  done
fi
[[ -x "$chrome" ]] || { printf '找不到 chrome-headless-shell（可用 REEL_CHROME 指定）\n' >&2; exit 2; }
curl -s -o /dev/null "http://127.0.0.1:$port/" || { printf '开发服务没起来\n' >&2; exit 2; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
REEL_REGRESSION_URL="http://127.0.0.1:$port/" REEL_CHROME="$chrome" node "$here/scripts/ui-regression.cjs" "$tmp"
