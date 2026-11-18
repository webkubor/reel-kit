#!/usr/bin/env bash
# 发布前检查：把该跑的检查一次跑完，不靠记忆。
#
# 用法：bash scripts/preflight.sh [包目录]
# 不带参数时只跑代码侧检查；带包目录时额外核验包。
set -uo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
cd "$here"
fail=0
step() { printf '\n== %s\n' "$1"; }
check() {
  local name=$1; shift
  if "$@" >/tmp/preflight.out 2>&1; then
    printf '  ✓ %s\n' "$name"
  else
    printf '  ✗ %s\n' "$name"
    tail -5 /tmp/preflight.out | sed 's/^/      /'
    fail=1
  fi
}

step '代码'
check 'gofmt 无未格式化文件' bash -c '[ -z "$(gofmt -l .)" ]'
check 'go vet 通过' env GOPROXY=https://goproxy.cn,direct go vet ./...
check '单元测试全部通过' env GOPROXY=https://goproxy.cn,direct go test -count=1 ./...
check '前端产物与源码同步' env GOPROXY=https://goproxy.cn,direct go test -count=1 -run 'FrontendDist|EmbeddedFrontend' .
check '界面回归脚本在仓库里' test -f scripts/run-ui-regression.sh

if [[ $# -ge 1 ]]; then
  pkg=$1
  step "包 $pkg"
  check '归档与校验和在' test -f "$pkg/muse-tv-1.9.0-darwin-arm64.zip"
  check '校验和通过' bash -c "cd '$pkg' && shasum -a 256 -c SHA256SUMS"
  check '来源 commit 在 main 上' bash -c "git -C '$here' merge-base --is-ancestor \$(python3 -c 'import json;print(json.load(open(\"$pkg/build-info.json\"))[\"commit\"])') origin/main"
  check '签名有效' codesign --verify --deep --strict "$pkg/muse-tv.app"
  check '打包界面终检通过' bash scripts/verify-package-ui.sh "$pkg"
fi

printf '\n'
if [[ $fail -eq 0 ]]; then
  printf '全部通过\n'
else
  printf '有检查未通过\n'
fi
exit $fail
