#!/usr/bin/env bash
# 按窗口 ID 直接抓取 muse-tv 窗口的画面，存成 PNG。
#
# 为什么不用「全屏截图再裁剪」：这台机器有多块屏幕，muse-tv 窗口可能落在
# 不同的坐标（甚至别的桌面空间），按坐标裁出来的是别的应用。
# screencapture -x -o -l <窗口ID> 直接抓那一个窗口，跟屏幕位置无关。
#
# 用法：bash scripts/capture-window.sh [输出文件] [进程名]
set -uo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-muse-tv-window.png}
name=${2:-muse-tv}

probe=$(mktemp -d)
trap 'rm -rf "$probe"' EXIT

cat > "$probe/winid.m" <<'EOF'
#include <ApplicationServices/ApplicationServices.h>
#include <stdio.h>
#include <string.h>
static void cs(CFDictionaryRef d, CFStringRef k, char *o, size_t n) {
  CFTypeRef v = d ? CFDictionaryGetValue(d, k) : NULL;
  if (!v || CFGetTypeID(v) != CFStringGetTypeID()) return;
  CFStringGetCString((CFStringRef)v, o, (CFIndex)n, kCFStringEncodingUTF8);
}
int main(int argc, char **argv) {
  const char *want = argc > 1 ? argv[1] : "muse-tv";
  CFArrayRef l = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);
  CFIndex n = CFArrayGetCount(l);
  int found = 0;
  for (CFIndex i = 0; i < n; i++) {
    CFDictionaryRef w = (CFDictionaryRef)CFArrayGetValueAtIndex(l, i);
    char o[128] = {0}, t[128] = {0};
    cs(w, kCGWindowOwnerName, o, sizeof(o));
    cs(w, kCGWindowName, t, sizeof(t));
    if (strcmp(o, want) != 0) continue;
    CFNumberRef num = (CFNumberRef)CFDictionaryGetValue(w, kCGWindowNumber);
    int id = 0; if (num) CFNumberGetValue(num, kCFNumberIntType, &id);
    /* 只认标题就是进程名的主窗口 */
    if (strcmp(t, want) != 0) continue;
    printf("%d\n", id);
    found++;
    break;
  }
  return found ? 0 : 1;
}
EOF

clang -framework ApplicationServices -framework CoreFoundation -o "$probe/winid" "$probe/winid.m" 2>/dev/null || {
  printf '✗ 编不出窗口 ID 探针\n' >&2; exit 1
}

wid=$("$probe/winid" "$name") || {
  printf '✗ 找不到 %s 的主窗口（应用没起来？）\n' "$name" >&2
  exit 2
}
printf '窗口 id = %s\n' "$wid"

# -x 不放快门声，-o 不加窗口阴影；带阴影那版会被 macOS 拒绝（could not create image）。
# screencapture 有自己的输出限制：sandbox 下的临时目录可能写不进去
# （报 "cannot write file to intended destination"），失败就换 /tmp 重试。
if ! screencapture -x -o -l "$wid" "$out" 2>/dev/null || [[ ! -s "$out" ]]; then
  fallback="/tmp/$(basename "$out")"
  if screencapture -x -o -l "$wid" "$fallback" 2>/dev/null && [[ -s "$fallback" ]]; then
    mv "$fallback" "$out" 2>/dev/null || { out="$fallback"; }
  fi
fi
if [[ -s "$out" ]]; then
  printf '✓ 已保存 %s\n' "$out"
else
  printf '✗ 抓不到窗口画面（试试把输出路径放到 /tmp）\n' >&2
  exit 3
fi
