package main

import "strings"

// 这几个小工具在好几个模块里都要用，放一处免得各写各的。

// strOr 取字符串值，没有或不是字符串就用兜底。
func strOr(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

// hasPrefix 判断前缀。
func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

// assetExtraStr 读素材的附加字段（字符串）。
func assetExtraStr(a *Asset, key string) string {
	if a == nil || a.Extra == nil {
		return ""
	}
	return strOr(a.Extra[key], "")
}

// assetExtraFloat 读素材的附加字段（浮点），没有就用兜底。
func assetExtraFloat(a *Asset, key string, fallback float64) float64 {
	if a == nil || a.Extra == nil {
		return fallback
	}
	if f := toFloat(a.Extra[key]); f != nil {
		return *f
	}
	return fallback
}

// floatOr 取浮点值，取不到就用兜底。
func floatOr(v any, fallback float64) float64 {
	if f := toFloat(v); f != nil {
		return *f
	}
	return fallback
}
