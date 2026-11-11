#!/usr/bin/env bash
# Build the current Wails desktop product with offline resources, never Node.
# Usage: bash scripts/package.sh NEW_OUTPUT_DIRECTORY RESOURCE_DIRECTORY
set -euo pipefail
[[ $# == 2 ]] || { printf 'Usage: %s NEW_OUTPUT_DIRECTORY RESOURCE_DIRECTORY\n' "$0" >&2; exit 2; }
output=$1
resources=$2
[[ "$output" = /* && "$resources" = /* ]] || { printf 'Use absolute paths.\n' >&2; exit 2; }
[[ ! -e "$output" ]] || { printf 'Output exists: %s\n' "$output" >&2; exit 1; }
[[ $(uname -s) == Darwin && $(uname -m) == arm64 ]] || { printf 'This package target requires Apple Silicon macOS.\n' >&2; exit 1; }
here=$(cd "$(dirname "$0")/.." && pwd)
wails=${WAILS_BIN:-$(command -v wails || true)}
[[ -x "$wails" ]] || { printf 'Set WAILS_BIN to the installed Wails CLI.\n' >&2; exit 1; }
cd "$here/frontend"
bash build.sh
cd "$here"
# Keep the deployment target consistent with the Go toolchain and plist.
MACOSX_DEPLOYMENT_TARGET=13.0 \
  CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=13.0" \
  CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=13.0" \
  "$wails" build -platform darwin/arm64 -s
bash package-local.sh "$output" "$resources"
version=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$output/Reel Studio.app/Contents/Info.plist")
ditto -c -k --sequesterRsrc --keepParent "$output/Reel Studio.app" "$output/reel-studio-${version}-darwin-arm64.zip"
(cd "$output" && shasum -a 256 "reel-studio-${version}-darwin-arm64.zip" > SHA256SUMS)
# Only a clean committed source tree can provide release provenance.
git diff --quiet HEAD -- . || { printf 'Commit source changes before making a release package.\n' >&2; exit 1; }
commit=$(git rev-parse HEAD)
python3 -I - "$output/build-info.json" "$version" "$commit" <<'PY'
import json, sys
with open(sys.argv[1], 'x') as f:
    json.dump({'version': sys.argv[2], 'commit': sys.argv[3], 'platform': 'darwin-arm64',
               'runtime': 'Wails + Go', 'nodeRuntime': False,
               'signing': 'ad-hoc', 'notarized': False}, f, indent=2)
    f.write('\n')
PY
printf 'Archive and checksum: %s\n' "$output"
