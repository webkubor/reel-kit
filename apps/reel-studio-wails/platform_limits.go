package main

import (
	"errors"
	"math"
)

// 发布平台的体积/时长上限是可配置阈值，不是平台官方承诺。
// 默认值取保守值；用户按自己账号实际限制改后，检测才按新阈值判定。
var LimitDefaults = Limits{MaxBytes: 100 * 1024 * 1024, MaxDuration: 180}

// Limits 一组阈值。
type Limits struct {
	MaxBytes    int `json:"maxBytes"`
	MaxDuration int `json:"maxDuration"`
}

// LimitPlatforms 只允许给已知平台配阈值；'default' 是所有平台共用的兜底。
var LimitPlatforms = []string{"default", "channels", "xiaohongshu", "landscape"}

// NormalizeLimits 收敛并校验一组阈值。
func NormalizeLimits(value map[string]any) (Limits, error) {
	maxBytes := float64(LimitDefaults.MaxBytes)
	if v, ok := value["maxBytes"]; ok && v != nil {
		f := toFloat(v)
		if f == nil {
			return Limits{}, errors.New("文件大小上限应在 1MB–4GB 之间")
		}
		maxBytes = *f
	}
	maxDuration := float64(LimitDefaults.MaxDuration)
	if v, ok := value["maxDuration"]; ok && v != nil {
		f := toFloat(v)
		if f == nil {
			return Limits{}, errors.New("时长上限应在 1–3600 秒之间")
		}
		maxDuration = *f
	}
	if math.IsNaN(maxBytes) || math.IsInf(maxBytes, 0) || maxBytes < 1024*1024 || maxBytes > 4*1024*1024*1024 {
		return Limits{}, errors.New("文件大小上限应在 1MB–4GB 之间")
	}
	if math.IsNaN(maxDuration) || math.IsInf(maxDuration, 0) || maxDuration < 1 || maxDuration > 3600 {
		return Limits{}, errors.New("时长上限应在 1–3600 秒之间")
	}
	return Limits{MaxBytes: int(math.Round(float64(maxBytes))), MaxDuration: int(math.Round(float64(maxDuration)))}, nil
}

// LimitsFor 取某个平台生效的阈值：平台自己配了就用自己配的，否则用 default。
func LimitsFor(table map[string]any, platform string) (Limits, error) {
	if v, ok := table[platform]; ok {
		if m, ok := v.(map[string]any); ok {
			return NormalizeLimits(m)
		}
	}
	if v, ok := table["default"]; ok {
		if m, ok := v.(map[string]any); ok {
			return NormalizeLimits(m)
		}
	}
	return NormalizeLimits(map[string]any{})
}

// SaveLimits 给某个平台存阈值。
func SaveLimits(table map[string]any, platform string, value map[string]any) (Limits, error) {
	known := false
	for _, p := range LimitPlatforms {
		if p == platform {
			known = true
			break
		}
	}
	if !known {
		return Limits{}, errors.New("不支持的平台")
	}
	limits, err := NormalizeLimits(value)
	if err != nil {
		return Limits{}, err
	}
	if table == nil {
		table = map[string]any{}
	}
	table[platform] = map[string]any{
		"maxBytes":    limits.MaxBytes,
		"maxDuration": limits.MaxDuration,
	}
	return limits, nil
}
