#!/usr/bin/env bash
# 检查原生窗口是否真的创建出来了。
#
# 锁屏时屏幕上看不见窗口，但窗口服务器照样记录它 —— 用 Quartz 读窗口列表，
# 能证明「原生窗口起来了，尺寸位置也对」。
# （辅助功能 API 在没授权时读不到，Quartz 不需要授权。）
#
# 用法：bash scripts/check-native-window.sh [应用路径]
set -uo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
app=${1:-}
[[ -n "$app" ]] || { printf '用法：bash scripts/check-native-window.sh <muse-tv.app 路径>\n' >&2; exit 2; }

probe=$(mktemp -d)/probe
trap 'rm -rf "$(dirname "$probe")"' EXIT
clang -framework ApplicationServices -framework CoreFoundation -o "$probe" "$here/scripts/window-probe.m" 2>/dev/null || {
  printf '✗ 编不出窗口探针（缺 clang/框架）\n' >&2; exit 1
}

pkill -f "MacOS/muse-tv" 2>/dev/null
sleep 1
"$app/Contents/MacOS/muse-tv" >/tmp/muse-window-check.log 2>&1 &
app_pid=$!
for _ in $(seq 1 30); do
  ps -p "$app_pid" >/dev/null 2>&1 || break
  sleep 0.5
done
if ! ps -p "$app_pid" >/dev/null 2>&1; then
  printf '✗ 程序没起来\n'
  cat /tmp/muse-window-check.log
  exit 1
fi
printf '✓ 程序起来了（pid %s）\n' "$app_pid"

sleep 3
out=$("$probe" muse-tv 2>/dev/null)
if printf '%s' "$out" | grep -q "name='muse-tv'"; then
  printf '✓ 原生窗口已创建\n'
  printf '%s\n' "$out" | sed 's/^/  /'
  kill "$app_pid" 2>/dev/null
  exit 0
fi
printf '⚠ 没有读到窗口：\n%s\n' "$out"
printf '  程序本身在跑；屏幕锁定时窗口服务器可能还没登记它，解锁后重跑。\n'
kill "$app_pid" 2>/dev/null
exit 2
