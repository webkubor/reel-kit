package main

import (
	"net/url"
	"strings"
	"testing"
)

// 文件名带中文时要按 UTF-8 字节编码，否则服务端收到的是无效路径。
func TestURLEncodeUTF8(t *testing.T) {
	name := "[克隆]朝朝-元气阳光_20261118_221530.wav"
	got := urlEncode(name)
	// 服务端按 UTF-8 解码后应还原出原文件名
	decoded, err := url.PathUnescape(got)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != name {
		t.Fatalf("解码后不是原名：%s", decoded)
	}
	// 中文必须被编码，不能原样出现在路径里
	if got == name || !strings.Contains(got, "%E5") {
		t.Fatalf("中文没被编码：%s", got)
	}
	if urlEncode("voice_0001.wav") != "voice_0001.wav" {
		t.Fatal("纯 ASCII 不该被编码")
	}
}
