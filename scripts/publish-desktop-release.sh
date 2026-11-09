#!/bin/bash
# 发桌面版：把 mygo build 出来的产物整理成一份可发布的清单并传上 GitHub。
#
#   scripts/publish-desktop-release.sh app-v0.2.3 --notes /tmp/notes.md
#
# 它替你做三件容易漏的事：
#   1. 按清单把该发的归档找齐，缺一个就停。
#   2. 发之前用公钥把每个归档的 Ed25519 签名验一遍。
#   3. 把所有产物的校验和写进 SHA256SUMS（扁平文件名，跟发布页上的
#      资产名一一对应，用户下完就能 shasum -c 验）。
#
# 需要：gh 已登录；签名私钥在发版那一步用 kyvault 注入（见 README）。
set -euo pipefail

cd "$(dirname "$0")/.."
APP=apps/reel-studio
cd "$APP"

tag="${1:-}"
notes=""
skip_build=""
while [ $# -gt 0 ]; do
	case "$1" in
	--notes)
		notes="$2"
		shift 2
		;;
	--skip-build)
		skip_build=1
		shift
		;;
	*)
		shift
		;;
	esac
done
[ -n "$tag" ] || {
	echo "用法: scripts/publish-desktop-release.sh <tag> [--notes 文件] [--skip-build]" >&2
	exit 2
}

version="$(python3 -c "import json;print(json.load(open('mygo.json'))['version'])")"
pubkey="$(python3 -c "import json;print(json.load(open('mygo.json'))['updates']['publicKey'])")"
repo="$(python3 -c "import json;print(json.load(open('mygo.json'))['updates']['github'])")"
prefix="$(python3 -c "import json;print(json.load(open('mygo.json'))['updates']['tagPrefix'])")"

if [ -z "$skip_build" ]; then
	echo "== 构建 $version"
	kyvault run --env MYGO_UPDATER_PRIVATE_KEY=secret://mygo/reel-studio-update-key -- \
		env GOTOOLCHAIN=go1.27.3 "$(command -v mygo || echo "$HOME/go/bin/mygo")" build
	kyvault run --env MYGO_UPDATER_PRIVATE_KEY=secret://mygo/reel-studio-update-key -- \
		env GOTOOLCHAIN=go1.27.3 "$(command -v mygo || echo "$HOME/go/bin/mygo")" build -platform windows/amd64 -skip-installer
fi

assets=()
sign() { # 验一个归档的签名：manifest 的 signature 覆盖归档的 SHA-256 摘要
	local file="$1" sig="$2" work
	work="$(mktemp -d)"
	printf '%s' "$sig" | base64 -D >"$work/sig.bin"
	# Ed25519 公钥的 DER 头 + 32 字节裸公钥
	python3 -c "
import base64, sys
raw = base64.b64decode(sys.argv[1])
der = bytes.fromhex('302a300506032b6570032100') + raw
body = base64.b64encode(der).decode()
lines = [body[i:i+64] for i in range(0, len(body), 64)]
print('-----BEGIN PUBLIC KEY-----')
print('\n'.join(lines))
print('-----END PUBLIC KEY-----')
" "$pubkey" >"$work/pub.pem"
	openssl dgst -sha256 -binary <"$file" >"$work/digest.bin"
	if openssl pkeyutl -verify -pubin -inkey "$work/pub.pem" -rawin \
		-in "$work/digest.bin" -sigfile "$work/sig.bin" >/dev/null 2>&1; then
		echo "  签名通过 $file"
	else
		echo "  签名不通过 $file" >&2
		rm -rf "$work"
		exit 1
	fi
	rm -rf "$work"
}

for dir in build/*/; do
	target="$(basename "$dir")"
	manifest="$dir/update-$target.json"
	[ -f "$manifest" ] || continue
	echo "== $target"
	assets+=("$manifest")
	# 归档清单里点名的每个文件都要在，并验签
	python3 - "$manifest" <<'PY' >"$dir/.assets.list"
import json, sys
m = json.load(open(sys.argv[1]))
print(m["url"].rsplit("/", 1)[1])
for d in m.get("deltas", []):
    print(d["url"].rsplit("/", 1)[1])
PY
	while read -r name; do
		[ -n "$name" ] || continue
		[ -f "$dir/$name" ] || {
			echo "  缺产物 $dir/$name" >&2
			exit 1
		}
		sig="$(python3 -c "
import json,sys
m=json.load(open('$manifest'))
names=[m['url'].rsplit('/',1)[1]]+[d['url'].rsplit('/',1)[1] for d in m.get('deltas',[])]
sigs=[m['signature']]+[d['signature'] for d in m.get('deltas',[])]
print(dict(zip(names,sigs))['$name'])")"
		sign "$dir/$name" "$sig"
		assets+=("$dir/$name")
	done <"$dir/.assets.list"
	rm -f "$dir/.assets.list"
done

# 非 mac 目标的归档（Windows 等）也一并带上
for f in build/windows-amd64/*.tar.gz build/windows-amd64/*.exe build/windows-amd64/*.json; do
	[ -f "$f" ] || continue
	assets+=("$f")
done
[ -f build/darwin-arm64/install.sh ] && assets+=(build/darwin-arm64/install.sh)
for f in build/darwin-arm64/*.dmg; do
	[ -f "$f" ] || continue
	assets+=("$f")
done

echo "== 校验和"
(cd build && shasum -a 256 $(cd .. && printf '%s\n' "${assets[@]}" | sed 's#^build/##') >SHA256SUMS)
assets+=(build/SHA256SUMS)

echo "== 发布 $tag"
gh release create "$tag" --repo "$repo" --title "Reel Studio $version" \
	${notes:+--notes-file "$notes"} "${assets[@]}"
echo "已发布 $tag"
