#!/usr/bin/env bash
# 检查原生窗口是否真的创建出来了。
#
# 锁屏时窗口服务器不创建窗口，程序会跑起来但没有窗口 ——
# 这时除了看屏幕没有别的办法区分「程序没起来」和「屏幕锁着」。
# 这个脚本用辅助功能 API 问窗口服务器：能读到窗口就是起来了。
#
# 用法：bash scripts/check-native-window.sh [应用路径]
set -uo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
app=${1:-}
[[ -n "$app" ]] || { printf '用法：bash scripts/check-native-window.sh <muse-tv.app 路径>\n' >&2; exit 2; }

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

# 问窗口服务器要窗口列表
windows=$(osascript -e 'tell application "System Events" to tell (first process whose name is "muse-tv") to count windows' 2>/dev/null)
if [[ "$windows" =~ ^[0-9]+$ ]] && [[ "$windows" -gt 0 ]]; then
  printf '✓ 原生窗口已创建（%s 个）\n' "$windows"
  osascript -e 'tell application "System Events" to tell (first process whose name is "muse-tv") to get {name, position, size} of every window' 2>/dev/null
  kill "$app_pid" 2>/dev/null
  exit 0
fi
printf '⚠ 没有读到窗口。屏幕锁定时窗口服务器不创建窗口 —— 属正常，\n'
printf '  解锁后重跑这个脚本即可确认。\n'
printf '  （程序本身在跑：%s）\n' "$(head -1 /tmp/muse-window-check.log 2>/dev/null)"
kill "$app_pid" 2>/dev/null
exit 2
