#!/usr/bin/env bash
# 打 Windows 平铺包：muse-tv.exe 与 Resources/ 同级，双击 exe 即可用。
#
# 与 macOS 的 .app bundle 不同，Windows 不需要 Contents/ 结构。
# 运行时由 resources.go 的 bundledResource 挨个找 Resources/ 与 exe 同级。
#
# 用法：bash scripts/package-windows.sh <新输出目录> <windows-amd64 资源目录>
set -euo pipefail
[[ $# == 2 ]] || { printf '用法：bash scripts/package-windows.sh <新输出目录> <windows-amd64 资源目录>\n' >&2; exit 2; }
output=$1
resources=$2
[[ "$output" = /* && "$resources" = /* ]] || { printf 'Use absolute paths.\n' >&2; exit 2; }
[[ ! -e "$output" ]] || { printf 'Output exists: %s\n' "$output" >&2; exit 1; }

here=$(cd "$(dirname "$0")/.." && pwd)
wails=${WAILS_BIN:-$(command -v wails || true)}
[[ -x "$wails" ]] || { printf 'Set WAILS_BIN to the installed Wails CLI.\n' >&2; exit 1; }

# 资源必须齐：少一个，装机用户点下去就报「找不到 ffmpeg」这类。
for item in \
  "$resources/sherpa/bin" \
  "$resources/chrome/chrome-headless-shell.exe" \
  "$resources/bin/ffmpeg.exe" \
  "$resources/bin/ffprobe.exe" \
  "$resources/bin/node.exe" \
  "$resources/bin/museav.cmd" \
  "$resources/museav/node_modules/museav-cli/dist/index.js"; do
  [[ -e "$item" ]] || { printf '缺少构建资源：%s\n' "$item" >&2; exit 1; }
done

cd "$here/frontend"
bash build.sh
cd "$here"
"$wails" build -platform windows/amd64 -s
exe="$here/build/bin/muse-tv.exe"
[[ -f "$exe" ]] || { printf '没有产出 exe：%s\n' "$exe" >&2; exit 1; }

mkdir -p "$output/Resources"
cp "$exe" "$output/muse-tv.exe"
cp -R "$resources/bin" "$output/Resources/bin"
cp -R "$resources/chrome" "$output/Resources/chrome"
cp -R "$resources/sherpa" "$output/Resources/sherpa"
cp -R "$resources/museav" "$output/Resources/museav"

version=$(python3 -c 'import json;print(json.load(open("wails.json"))["info"]["productVersion"])')
archive="muse-tv-${version}-windows-amd64.zip"
(cd "$output" && zip -qr "$archive" muse-tv.exe Resources)
(cd "$output" && shasum -a 256 "$archive" > SHA256SUMS)

git diff --quiet HEAD -- . || { printf 'Commit source changes before making a release package.\n' >&2; exit 1; }
commit=$(git rev-parse HEAD)
python3 -I - "$output/build-info.json" "$version" "$commit" <<'PY'
import json, sys
with open(sys.argv[1], 'x') as f:
    json.dump({'version': sys.argv[2], 'commit': sys.argv[3], 'platform': 'windows-amd64',
               'runtime': 'Wails + Go', 'nodeRuntime': False,
               'signing': 'unsigned', 'notarized': False}, f, indent=2)
    f.write('\n')
PY
printf 'Windows 归档与校验和：%s\n' "$output"
