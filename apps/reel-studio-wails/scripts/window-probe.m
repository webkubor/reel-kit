// 读窗口服务器里的窗口信息：位置、尺寸、标题。
//
// 锁屏时窗口照样创建，只是屏幕上看不见 —— 用这个能证明
// 「原生窗口确实起来了，尺寸位置也对」，不必等解锁。
//
// 用法：clang -framework ApplicationServices -framework CoreFoundation -o window-probe window-probe.m
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
    CFIndex n = CFArrayGetCount(list);
    int found = 0;
    for (CFIndex i = 0; i < n; i++) {
        CFDictionaryRef w = (CFDictionaryRef)CFArrayGetValueAtIndex(list, i);
        char owner[256] = {0}, name[256] = {0};
        copyStr(w, kCGWindowOwnerName, owner, sizeof(owner));
        copyStr(w, kCGWindowName, name, sizeof(name));
        if (strcmp(owner, want) != 0) continue;
        double x = 0, y = 0, ww = 0, hh = 0, alpha = 0;
        CFDictionaryRef b = (CFDictionaryRef)CFDictionaryGetValue(w, kCGWindowBounds);
        if (b) {
            copyNum(b, CFSTR("X"), &x); copyNum(b, CFSTR("Y"), &y);
            copyNum(b, CFSTR("Width"), &ww); copyNum(b, CFSTR("Height"), &hh);
        }
        copyNum(w, kCGWindowAlpha, &alpha);
        printf("%s 窗口 name='%s' 位置=(%.0f,%.0f) 尺寸=%.0fx%.0f alpha=%.2f\n",
               want, name, x, y, ww, hh, alpha);
        found++;
    }
    if (!found) printf("%s 没有窗口\n", want);
    return found ? 0 : 1;
}
