#!/bin/bash
# 发桌面版：把 mygo build 出来的产物整理成一份可发布的清单并传上 GitHub。
#
#   scripts/publish-desktop-release.sh app-v0.2.3 --notes /tmp/notes.md
#   scripts/publish-desktop-release.sh app-v0.2.3 --dry-run    # 只检查不发布
#   scripts/publish-desktop-release.sh app-v1.5.0 --platform darwin/arm64 --carry windows/amd64
#
# 它替你做五件容易漏的事：
#   1. 版本与 tag 对不上就停：tag 必须是 mygo.json 里版本号加 tagPrefix，
#      清单里的版本号也必须等于当前版本（旧构建留下的清单不许混进去）。
#   2. 按清单把该发的归档找齐，缺一个就停，并验一遍每个归档的 Ed25519 签名。
#   3. 同一个产物只发一次：清单点名的东西不再被后面的通配符重复收集。
#   4. 把所有产物的校验和写进 SHA256SUMS，文件名用发布页上的扁平名字，
#      用户下完就能 shasum -c 验。
#   5. --platform 只发部分平台、--carry 沿用其他平台已发布的清单：
#      只发 macOS 时，Windows 用户仍能在新发布页上查到自己的更新清单。
#
# 需要：gh 已登录；签名私钥在发版那一步用 kyvault 注入（见 README）。
set -euo pipefail

cd "$(dirname "$0")/.."
APP=apps/reel-studio
cd "$APP"

tag="${1:-}"
notes=""
skip_build=""
dry_run=""
platforms=""
carried=""
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
	--dry-run)
		dry_run=1
		shift
		;;
	--platform)
		platforms="$platforms $2"
		shift 2
		;;
	--carry)
		carried="$carried $2"
		shift 2
		;;
	*)
		shift
		;;
	esac
done
[ -n "$tag" ] || {
	echo "用法: scripts/publish-desktop-release.sh <tag> [--notes 文件] [--skip-build] [--platform 目标] [--carry 目标] [--dry-run]" >&2
	exit 2
}

version="$(python3 -c "import json;print(json.load(open('mygo.json'))['version'])")"
pubkey="$(python3 -c "import json;print(json.load(open('mygo.json'))['updates']['publicKey'])")"
repo="$(python3 -c "import json;print(json.load(open('mygo.json'))['updates']['github'])")"
prefix="$(python3 -c "import json;print(json.load(open('mygo.json'))['updates']['tagPrefix'])")"

# tag 必须跟版本号对上，发错 tag 的版本号比漏发更麻烦。
want_tag="$prefix$version"
if [ "$tag" != "$want_tag" ]; then
	echo "tag 应为 $want_tag（mygo.json 版本 $version），收到的是 $tag" >&2
	exit 2
fi

# --carry 沿用别的平台已发布的清单时，先跟线上那份逐字段对一遍。
#
# 沿用清单里的 url / signature / deltas 全都指向旧发布页上的资产。
# 只要有一处对不上，用户升级时就会撞到「签名校验失败」或 404 —— 而这类问题
# 只在用户点「检查更新」时才炸，发版当场看不出来。所以这里当场比。
if [ -n "$carried" ]; then
	echo "== 核对沿用的清单与线上已发布版本"
	python3 - "$repo" "$prefix" "$version" "$carried" <<'PYCARRY'
import json, os, subprocess, sys

repo, prefix, version, raw_targets = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
targets = [t.strip().replace("/", "-") for t in raw_targets.split() if t.strip()]
current_tag = prefix + version
errors = []

def gh(args, binary=False):
    # gh api 只收一个 endpoint 参数，所以这里永远传列表而不是字符串。
    out = subprocess.check_output(["gh", "api"] + [list(args) if isinstance(args, (list, tuple)) else args])
    return out if binary else json.loads(out)


def gh_asset(asset_id):
    # 下载发布页上的资产本体：默认的 Accept 头返回的是资产元信息，不是文件内容。
    return subprocess.check_output([
        "gh", "api", "-H", "Accept: application/octet-stream",
        "repos/%s/releases/assets/%d" % (repo, asset_id),
    ])

try:
    releases = gh("repos/%s/releases?per_page=100" % repo)
except Exception as error:
    print("读不到线上发布列表：%s" % error, file=sys.stderr)
    sys.exit(8)

def compare(name, online, local):
    for field in ("version", "url", "size", "signature"):
        if online.get(field) != local.get(field):
            errors.append("%s 的 %s 与线上不一致：线上=%r 本地=%r"
                          % (name, field, online.get(field), local.get(field)))
    for key in ("deltas", "previous"):
        a = {json.dumps(x, sort_keys=True) for x in online.get(key, [])}
        b = {json.dumps(x, sort_keys=True) for x in local.get(key, [])}
        if a != b:
            errors.append("%s 的 %s 列表与线上不一致（线上 %s 条，本地 %s 条）"
                          % (name, key, len(a), len(b)))

for target in targets:
    asset_name = "update-%s.json" % target
    local_path = os.path.join("build", target, asset_name)
    found = None
    for release in releases:
        if release.get("tag_name") == current_tag:
            continue
        for asset in release.get("assets", []):
            if asset.get("name") == asset_name:
                found = (release["tag_name"], asset)
                break
        if found:
            break
    if not found:
        errors.append("%s：线上找不到已发布的 %s，沿用清单没有来源可核对" % (target, asset_name))
        continue
    tag, asset = found
    try:
        online = json.loads(gh_asset(asset["id"]))
    except Exception as error:
        errors.append("%s：读不下线上清单（%s）" % (target, error))
        continue
    if not os.path.isfile(local_path):
        errors.append("%s：本地缺少沿用清单 %s" % (target, local_path))
        continue
    with open(local_path) as handle:
        local = json.load(handle)
    compare(asset_name, online, local)
    url = online.get("url")
    if not url:
        errors.append("%s：线上清单没有 url" % asset_name)
        continue
    code = subprocess.run(["curl", "-sIL", "-o", "/dev/null", "-w", "%{http_code}", url],
                          capture_output=True, text=True).stdout.strip()
    if code != "200":
        errors.append("%s：线上清单指向的归档取不到（HTTP %s）%s" % (asset_name, code, url))

if errors:
    print("沿用清单与线上对不上，已停止：", file=sys.stderr)
    for error in errors:
        print("  - " + error, file=sys.stderr)
    sys.exit(8)
print("沿用清单已与线上逐字段核对一致")
PYCARRY
fi

if [ -z "$skip_build" ]; then
	echo "== 构建 $version"
	if [ -n "$platforms" ]; then
		# 只发哪个平台就只构建哪个平台：别为了发 macOS 再构建一遍 Windows。
		for p in $platforms; do
			kyvault run --env MYGO_UPDATER_PRIVATE_KEY=secret://mygo/reel-studio-update-key -- \
				env GOTOOLCHAIN=go1.27.3 "$(command -v mygo || echo "$HOME/go/bin/mygo")" build -platform "$p"
		done
	else
		kyvault run --env MYGO_UPDATER_PRIVATE_KEY=secret://mygo/reel-studio-update-key -- \
			env GOTOOLCHAIN=go1.27.3 "$(command -v mygo || echo "$HOME/go/bin/mygo")" build
		kyvault run --env MYGO_UPDATER_PRIVATE_KEY=secret://mygo/reel-studio-update-key -- \
			env GOTOOLCHAIN=go1.27.3 "$(command -v mygo || echo "$HOME/go/bin/mygo")" build -platform windows/amd64 -skip-installer
	fi
fi

# 收集产物：名字（发布页上的名字）、本地路径、签名（清单里才有）。
# 只认带清单的构建目录，清单版本必须等于当前版本。
#
# --platform 只发部分平台时用：没点名的平台目录整目录跳过（连它的清单也不带），
#   点名的平台仍然要求版本等于当前版本 —— 「旧构建混进来」这道防线不因为只发一个平台就失效。
# --carry 沿用某个平台已发布的清单，原样带上，不改内容：只发 macOS 时，
#   Windows 用户要能在新发布页上查到自己的清单，否则他们的「检查更新」会变成 404 报错。
python3 - "$version" "$platforms" "$carried" >/tmp/reel-studio-assets.tsv <<'PY'
import glob, json, os, sys

version = sys.argv[1]
wanted = {t.strip().replace("/", "-") for t in sys.argv[2].split() if t.strip()}
carried = {t.strip().replace("/", "-") for t in sys.argv[3].split() if t.strip()}
rows = []
seen = {}


def add(name, path, sig=""):
    if name in seen:
        if seen[name] != path:
            print("产物名冲突：%s 同时指向 %s 和 %s" % (name, seen[name], path), file=sys.stderr)
            sys.exit(3)
        return
    seen[name] = path
    rows.append((name, path, sig))


for d in sorted(glob.glob("build/*")):
    if not os.path.isdir(d):
        continue
    target = os.path.basename(d)
    manifest = os.path.join(d, "update-%s.json" % target)
    if not os.path.isfile(manifest):
        continue
    if target in carried:
        # 沿用旧清单：内容一个字都不改，它指向的归档在旧发布页上仍然有效。
        carried_manifest = json.load(open(manifest))
        if carried_manifest.get("version") == version:
            print("%s 的清单版本也是 %s，不该沿用（本次就是要发这个版本）" % (manifest, version), file=sys.stderr)
            sys.exit(7)
        print("沿用 %s 的旧清单：版本 %s，指向 %s（本次不发布这个平台的安装包）"
              % (target, carried_manifest.get("version"), carried_manifest.get("url")), file=sys.stderr)
        add(os.path.basename(manifest), manifest)
        continue
    if wanted and target not in wanted:
        print("跳过 %s：本次只发 %s" % (target, "、".join(sorted(wanted))), file=sys.stderr)
        continue
    m = json.load(open(manifest))
    if m.get("version") != version:
        print("%s 里的版本是 %s，不是 %s（旧构建留下的？）" % (manifest, m.get("version"), version), file=sys.stderr)
        sys.exit(4)
    items = [(m["url"].rsplit("/", 1)[1], m["signature"])]
    for dlt in m.get("deltas", []):
        items.append((dlt["url"].rsplit("/", 1)[1], dlt["signature"]))
    for name, sig in items:
        path = os.path.join(d, name)
        if not os.path.isfile(path):
            print("缺产物 %s" % path, file=sys.stderr)
            sys.exit(5)
        add(name, path, sig)
    add(os.path.basename(manifest), manifest)
    # 安装脚本与磁盘镜像：按名字收，不写死构建目录名。
    for extra in sorted(glob.glob(os.path.join(d, "install.sh"))):
        add(os.path.basename(extra), extra)
    for extra in sorted(glob.glob(os.path.join(d, "*.dmg"))):
        if version in os.path.basename(extra):
            add(os.path.basename(extra), extra)

for name, path, sig in rows:
    print("%s\t%s\t%s" % (name, path, sig))
PY

sign() { # 验一个归档的签名：清单里的 signature 覆盖归档的 SHA-256 摘要
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
		echo "  签名通过 $(basename "$file")"
	else
		echo "  签名不通过 $file" >&2
		rm -rf "$work"
		exit 1
	fi
	rm -rf "$work"
}

echo "== 验签"
while IFS=$'\t' read -r name path sig; do
	[ -n "${sig:-}" ] || continue
	sign "$path" "$sig"
done </tmp/reel-studio-assets.tsv

echo "== 校验和"
# 文件名用发布页上的扁平名字，用户下完 shasum -c SHA256SUMS 就能验。
: >build/SHA256SUMS
while IFS=$'\t' read -r name path sig; do
	sum="$(shasum -a 256 "$path" | awk '{print $1}')"
	printf '%s  %s\n' "$sum" "$name" >>build/SHA256SUMS
done </tmp/reel-studio-assets.tsv

assets=()
while IFS=$'\t' read -r name path sig; do
	assets+=("$path")
done </tmp/reel-studio-assets.tsv
assets+=(build/SHA256SUMS)

echo "== 待发布产物"
while IFS=$'\t' read -r name path sig; do
	echo "  $name"
done </tmp/reel-studio-assets.tsv
echo "  SHA256SUMS"

if [ -n "$dry_run" ]; then
	echo "== 只检查不发布"
	exit 0
fi

if gh release view "$tag" --repo "$repo" >/dev/null 2>&1; then
	echo "发布 $tag 已经存在，先处理掉它再发" >&2
	exit 6
fi

echo "== 发布 $tag"
gh release create "$tag" --repo "$repo" --title "Reel Studio $version" \
	${notes:+--notes-file "$notes"} "${assets[@]}"
echo "已发布 $tag"
