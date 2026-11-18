#!/usr/bin/env bash
# 发布终检：把打包二进制里嵌的前端解出来，用浏览器实际打开看渲染。
#
# 这是发布前的最后一道 —— 编译通过不代表用户看到的界面是对的。
# 用法：bash scripts/verify-package-ui.sh <包目录>
set -uo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
pkg=${1:-}
[[ -n "$pkg" ]] || { printf '用法：bash scripts/verify-package-ui.sh <包目录>\n' >&2; exit 2; }
[[ -d "$pkg" ]] || { printf '包目录不存在：%s\n' "$pkg" >&2; exit 2; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"; kill ${http_pid:-0} 2>/dev/null' EXIT

# 1 从二进制解出嵌入的前端
cat > "$here/zz_verify_package_ui_test.go" <<'GOEOF'
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestZZVerifyPackageUI(t *testing.T) {
	out := os.Getenv("REEL_EXTRACT_TO")
	if out == "" {
		t.Skip("one-off")
	}
	os.MkdirAll(out, 0o755)
	if err := fs.WalkDir(assets, "frontend/dist", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := assets.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(out, filepath.Base(p)), data, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
}
GOEOF
(cd "$here" && REEL_EXTRACT_TO="$tmp/ui" go test -count=1 -run TestZZVerifyPackageUI . >/dev/null) || {
  printf '解出前端失败\n' >&2; rm -f "$here/zz_verify_package_ui_test.go"; exit 1
}
rm -f "$here/zz_verify_package_ui_test.go"

# 2 起个静态服务，把界面打开
# 服务要在解出文件的目录里起：工作目录不对会 404。
# 端口随机取一个空闲的，避免撞上之前留下的服务。
port=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
(cd "$tmp/ui" && python3 -m http.server "$port" --bind 127.0.0.1) >/dev/null 2>&1 &
http_pid=$!
ready=0
for _ in $(seq 1 30); do
  if curl -sf -o /dev/null "http://127.0.0.1:$port/index.html"; then ready=1; break; fi
  sleep 0.3
done
if [[ $ready -ne 1 ]]; then
  printf '静态服务没起来（端口 %s）\n' "$port" >&2
  exit 1
fi

# 3 浏览器里检查关键结构与渲染
node "$here/scripts/package-ui-check.cjs" "$port" "$tmp" || exit 1
printf '打包界面终检通过\n'
