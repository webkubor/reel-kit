#!/usr/bin/env bash
# Publish the current Wails product. No MyGo runtime or carried-forward packages.
# Usage: scripts/publish-desktop-release.sh app-vVERSION PACKAGE_DIR NOTES_FILE [--dry-run]
set -euo pipefail
[[ $# == 3 || ( $# == 4 && "$4" == --dry-run ) ]] || {
  printf 'Usage: %s app-vVERSION PACKAGE_DIR NOTES_FILE [--dry-run]\n' "$0" >&2; exit 2;
}
tag=$1
package=$2
notes=$3
repo_root=$(cd "$(dirname "$0")/.." && pwd)
version=$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["info"]["productVersion"])' "$repo_root/apps/reel-studio-wails/wails.json")
[[ "$tag" == "app-v$version" ]] || { printf 'Tag must be app-v%s\n' "$version" >&2; exit 2; }
[[ -f "$notes" && -f "$package/build-info.json" ]] || { printf 'Missing release notes or build provenance.\n' >&2; exit 1; }
archive="reel-studio-${version}-darwin-arm64.zip"
[[ -f "$package/$archive" && -f "$package/SHA256SUMS" ]] || { printf 'Missing archive or checksums.\n' >&2; exit 1; }
(cd "$package" && shasum -a 256 -c SHA256SUMS)
commit=$(python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["version"]==sys.argv[2]; assert d["platform"]=="darwin-arm64"; print(d["commit"])' "$package/build-info.json" "$version")
git -C "$repo_root" fetch origin main
git -C "$repo_root" merge-base --is-ancestor "$commit" origin/main
[[ "$(git -C "$repo_root" rev-parse "$tag^{commit}")" == "$commit" ]] || { printf 'Tag differs from package source.\n' >&2; exit 1; }
# The release notes must disclose that this archive has no Apple notarization.
rg -q '未公证|not notarized' "$notes"
if [[ ${4:-} == --dry-run ]]; then
  printf 'Verified %s at %s; no release created.\n' "$archive" "$commit"
  exit 0
fi
gh release create "$tag" --repo webkubor/reel-kit --verify-tag \
  --title "Reel Studio $version" --notes-file "$notes" \
  "$package/$archive" "$package/SHA256SUMS" "$package/build-info.json"
