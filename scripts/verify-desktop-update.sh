#!/usr/bin/env bash
# 验「发布页上的增量包」与「应用内的更新流程」两件事。
#
# 为什么单独写这个脚本：签名只能证明「包没被改过」，证不了「包是对的」。
# 真要确认更新链路没坏，得拿发布页上的归档与增量包，
# 用 MyGo 的生产代码走一遍：ApplyDelta 把旧版还原成新版，再 installUpdate
# 把装着旧版的 .app 原地换成新版。
#
# 这两段代码在 MyGo 模块内部（internal/update 与包私有函数），
# 外部仓库跑不了，所以这里把 MyGo 模块复制一份到临时目录，
# 把测试文件放进去再跑 —— 跑的仍然是发布用的那份代码。
#
# 用法：
#   bash scripts/verify-desktop-update.sh <tag>          # 打印要准备什么，自己按下面对照填
#   bash scripts/verify-desktop-update.sh <tag> --run    # 自动下载归档并跑完两段验证
set -euo pipefail

TAG="${1:-}"
RUN="${2:-}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO="${REEL_REPO:-webkubor/reel-kit}"
WORK="${WORK:-/tmp/reel-update-verify}"
MODCACHE="$(GOTOOLCHAIN=go1.27.3 go env GOMODCACHE)"
MODULE="$(grep 'github.com/egoist/mygo ' "$ROOT/apps/reel-studio/go.mod" | awk '{print $NF}')"
SRC="$MODCACHE/github.com/egoist/mygo@$MODULE"

[ -n "$TAG" ] || { echo "用法：bash scripts/verify-desktop-update.sh <tag> [--run]" >&2; exit 2; }
[ -d "$SRC" ] || { echo "本机没有 MyGo 模块：${SRC}（先跑一次 go mod download）" >&2; exit 2; }

platform_manifest() {
	gh release view "$TAG" --repo "$REPO" --json assets \
		--jq '.assets[].name | select(startswith("update-") and (contains("darwin") or contains("arm64")))' | head -1
}
platform_archive() {
	gh release view "$TAG" --repo "$REPO" --json assets \
		--jq '.assets[].name | select(endswith(".tar.gz") and (contains("darwin") or contains("arm64")))' | head -1
}

if [ "$RUN" != "--run" ]; then
	echo "tag:        $TAG"
	echo "清单:       $(platform_manifest)"
	echo "归档:       $(platform_archive)"
	echo
	echo "要跑完整验证，请准备好："
	echo "  1. 旧版本号的归档（解开后的 .app 目录）"
	echo "  2. 新版本号的归档（解开后的 .app 目录）"
	echo "  3. 旧版本 → 新版本的增量包"
	echo "然后带这些环境变量跑："
	echo "  DELTA_FILE= OLD_DIR= NEW_DIR= FROM= TO= go test ./internal/update/ -run TestPublishedDelta"
	echo "  PUBLISHED_MANIFEST= PUBLISHED_KEY= INSTALL_TARGET= INSTALL_FROM= INSTALL_EXPECT= go test . -run TestPublishedInstall"
	exit 0
fi

echo "== 准备 MyGo 模块副本"
rm -rf "$WORK"
mkdir -p "$WORK"
cp -R "$SRC" "$WORK/mygo"
chmod -R u+w "$WORK/mygo"
cp "$ROOT/scripts/desktop-update-test/reel_delta_test.go" "$WORK/mygo/internal/update/"
cp "$ROOT/scripts/desktop-update-test/updater_published_install_test.go" "$WORK/mygo/"

FROM_VERSION="${FROM_VERSION:-1.0.0}"
TO_VERSION="$(python3 -c "import json;print(json.load(open('$ROOT/apps/reel-studio/mygo.json'))['version'])")"

echo "== 下载 $FROM_VERSION 与 $TO_VERSION 的归档"
mkdir -p "$WORK/archives"
for v in "$FROM_VERSION" "$TO_VERSION"; do
	name="$(gh release view "app-v$v" --repo "$REPO" --json assets --jq '.assets[].name | select(endswith(".tar.gz") and (contains("darwin") or contains("arm64")))' | head -1)"
	mkdir -p "$WORK/archives/$v"
	curl -fsSL -o "$WORK/archives/$v/app.tar.gz" \
		"https://github.com/$REPO/releases/download/app-v$v/$name"
	mkdir -p "$WORK/archives/$v/app"
	tar -xzf "$WORK/archives/$v/app.tar.gz" -C "$WORK/archives/$v/app"
done

DELTA_NAME="$(gh release view "app-v$TO_VERSION" --repo "$REPO" --json assets --jq ".assets[].name | select(contains(\"$FROM_VERSION-to-$TO_VERSION\") and (contains(\"darwin\") or contains(\"arm64\")))" | head -1)"
curl -fsSL -o "$WORK/$DELTA_NAME" \
	"https://github.com/$REPO/releases/download/app-v$TO_VERSION/$DELTA_NAME"

echo "== 1. 增量包还原"
cd "$WORK/mygo"
DELTA_FILE="$WORK/$DELTA_NAME" \
	OLD_DIR="$WORK/archives/$FROM_VERSION/app/Reel Studio.app" \
	NEW_DIR="$WORK/archives/$TO_VERSION/app/Reel Studio.app" \
	FROM="$FROM_VERSION" TO="$TO_VERSION" \
	GOTOOLCHAIN=go1.27.3 go test ./internal/update/ -run TestPublishedDelta -v

echo "== 2. 应用内更新流程（生产函数 installUpdate）"
rm -rf "$WORK/target"
mkdir -p "$WORK/target"
cp -R "$WORK/archives/$FROM_VERSION/app/Reel Studio.app" "$WORK/target/Reel Studio.app"
PUBLISHED_MANIFEST="https://github.com/$REPO/releases/download/app-v$TO_VERSION/$(platform_manifest)" \
	PUBLISHED_KEY="$(python3 -c "import json;print(json.load(open('$ROOT/apps/reel-studio/mygo.json'))['updates']['publicKey'])")" \
	INSTALL_TARGET="$WORK/target/Reel Studio.app" \
	INSTALL_FROM="$FROM_VERSION" INSTALL_EXPECT="$TO_VERSION" \
	GOTOOLCHAIN=go1.27.3 go test . -run TestPublishedInstall -v
