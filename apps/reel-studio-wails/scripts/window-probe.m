// 读窗口服务器里的窗口信息：位置、尺寸、是否可见、是否上屏、是否完整在屏内。
//
// 锁屏时屏幕上看不见窗口，但窗口服务器照样记录它 —— 用 Quartz 能证明
// 「原生窗口确实起来了，尺寸位置对、可见、落在屏幕里」，不必等解锁。
// （辅助功能 API 在没授权时读不到，Quartz 不需要授权。）
//
// 用法：clang -framework ApplicationServices -framework CoreFoundation -o window-probe window-probe.m [进程名]
#include <ApplicationServices/ApplicationServices.h>
#include <stdio.h>
#include <string.h>

static void copyStr(CFDictionaryRef d, CFStringRef key, char *out, size_t n) {
    CFTypeRef v = d ? CFDictionaryGetValue(d, key) : NULL;
    if (!v || CFGetTypeID(v) != CFStringGetTypeID()) return;
    CFStringGetCString((CFStringRef)v, out, (CFIndex)n, kCFStringEncodingUTF8);
}
static int copyNum(CFDictionaryRef d, CFStringRef key, double *out) {
    CFTypeRef v = d ? CFDictionaryGetValue(d, key) : NULL;
    if (!v || CFGetTypeID(v) != CFNumberGetTypeID()) return 0;
    return CFNumberGetValue((CFNumberRef)v, kCFNumberDoubleType, out);
}

int main(int argc, char **argv) {
    const char *want = argc > 1 ? argv[1] : "muse-tv";
    CFArrayRef list = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);
    if (!list) { printf("拿不到窗口列表\n"); return 1; }
    CGRect screen = CGDisplayBounds(CGMainDisplayID());
    printf("主屏 %.0fx%.0f\n", screen.size.width, screen.size.height);
    CFIndex n = CFArrayGetCount(list);
    int found = 0;
    for (CFIndex i = 0; i < n; i++) {
        CFDictionaryRef w = (CFDictionaryRef)CFArrayGetValueAtIndex(list, i);
        char owner[256] = {0}, name[256] = {0};
        copyStr(w, kCGWindowOwnerName, owner, sizeof(owner));
        copyStr(w, kCGWindowName, name, sizeof(name));
        if (strcmp(owner, want)) continue;
        double x = 0, y = 0, ww = 0, hh = 0, alpha = 0, onscreen = 0;
        CFDictionaryRef b = (CFDictionaryRef)CFDictionaryGetValue(w, kCGWindowBounds);
        if (b) { copyNum(b, CFSTR("X"), &x); copyNum(b, CFSTR("Y"), &y);
                  copyNum(b, CFSTR("Width"), &ww); copyNum(b, CFSTR("Height"), &hh); }
        copyNum(w, kCGWindowAlpha, &alpha);
        copyNum(w, kCGWindowIsOnscreen, &onscreen);
        int inScreen = (x >= screen.origin.x && y >= screen.origin.y &&
                        x + ww <= screen.origin.x + screen.size.width + 1 &&
                        y + hh <= screen.origin.y + screen.size.height + 1);
        printf("%s 窗口 name='%s' 位置=(%.0f,%.0f) 尺寸=%.0fx%.0f alpha=%.2f 上屏=%s 完整在屏内=%s\n",
               want, name, x, y, ww, hh, alpha, onscreen ? "是" : "否", inScreen ? "是" : "否");
        found++;
    }
    if (!found) { printf("%s 没有窗口\n", want); return 1; }
    return 0;
}
