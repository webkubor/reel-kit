package main

import "strconv"

// 默认 BGM 音量（有配音时，配乐作为垫底）。
// 与 compose 的 DEFAULT_BGM_GAIN 一致，全工程只有一个来源。
const DefaultBGMGain = 0.12

// SettingGroups 分节顺序，与前端配置面板同序。
var SettingGroups = []string{"脚本", "声音", "画面", "版式", "画幅", "配乐", "输出"}

// SettingType 旋钮类型。
type SettingType string

const (
	SettingTypeNumber SettingType = "number"
	SettingTypeBool   SettingType = "bool"
	SettingTypeText   SettingType = "text"
	SettingTypeChoice SettingType = "choice"
)

// SettingOption choice 的候选值。
type SettingOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SettingField 单个旋钮的定义。
type SettingField struct {
	Group   string          `json:"group"`
	Label   string          `json:"label"`
	Type    SettingType     `json:"type"`
	Default interface{}     `json:"default"`
	Min     *float64        `json:"min,omitempty"`
	Max     *float64        `json:"max,omitempty"`
	Step    *float64        `json:"step,omitempty"`
	Unit    string          `json:"unit,omitempty"`
	Display string          `json:"display,omitempty"` // percent: 界面显示百分比
	Kinds   []string        `json:"kinds,omitempty"`   // 对哪些流水线有效，空 = 全部
	Hint    string          `json:"hint,omitempty"`
	Options []SettingOption `json:"options,omitempty"`
}

// SettingSchema 旋钮表。
var SettingSchema = map[string]SettingField{
	/* ── 声音 ── */
	"voiceGain": {
		Group: "声音", Label: "人声音量", Type: SettingTypeNumber,
		Default: 1.35, Min: ptrFloat(0.4), Max: ptrFloat(3),
		Step: ptrFloat(0.05), Unit: "倍",
		Hint: "念白本身偏小就调大；配乐压不住念白时先看这个",
	},
	"duck": {
		Group: "声音", Label: "人声时压低配乐", Type: SettingTypeBool,
		Default: true,
		Hint:    "人声一开口就把配乐压下去，人声停时抬回来",
	},
	"tone": {
		Group: "声音", Label: "语气", Type: SettingTypeText,
		Default: "",
		Hint:    "留空用音色默认",
	},
	"speed": {
		Group: "声音", Label: "语速", Type: SettingTypeNumber,
		Default: 1.0, Min: ptrFloat(0.75), Max: ptrFloat(1.5),
		Step: ptrFloat(0.05), Unit: "倍",
		Kinds: []string{"recommendation", "talk", "product"},
	},
	"pause": {
		Group: "声音", Label: "镜间停顿", Type: SettingTypeNumber,
		Default: 0.35, Min: ptrFloat(0), Max: ptrFloat(2),
		Step: ptrFloat(0.05), Unit: "秒",
		Hint: "每句话念完之后停多久",
	},
	/* ── 画面 ── */
	"imageFit": {
		Group: "画面", Label: "画面填充", Type: SettingTypeChoice,
		Default: "contain",
		Options: []SettingOption{{Value: "contain", Label: "完整画面"}, {Value: "cover", Label: "铺满画布"}},
	},
	"motion": {
		Group: "画面", Label: "镜内动效", Type: SettingTypeChoice,
		Default: "gentle",
		Options: []SettingOption{
			{Value: "none", Label: "静态画面（较快）"},
			{Value: "gentle", Label: "轻柔运镜（逐帧渲染）"},
		},
		Hint: "图片缓慢推进、字幕轻柔入场；逐帧渲染，生成时间更长",
	},
	"transition": {
		Group: "画面", Label: "切镜转场", Type: SettingTypeChoice,
		Default: "none",
		Options: []SettingOption{
			{Value: "none", Label: "硬切（无转场）"},
			{Value: "fade", Label: "淡入淡出"},
			{Value: "slide-left", Label: "向左推移"},
			{Value: "slide-right", Label: "向右推移"},
			{Value: "slide-up", Label: "向上推移"},
			{Value: "slide-down", Label: "向下推移"},
			{Value: "wipe-left", Label: "向左擦除"},
			{Value: "wipe-right", Label: "向右擦除"},
			{Value: "dissolve", Label: "溶解交叉"},
			{Value: "zoom-in", Label: "镜头推进"},
		},
		Hint: "分镜切换时的视觉动效",
	},
	"transitionDuration": {
		Group: "画面", Label: "转场时长", Type: SettingTypeNumber,
		Default: 0.35, Min: ptrFloat(0.1), Max: ptrFloat(1.0),
		Step: ptrFloat(0.05), Unit: "秒",
		Hint: "转场动效持续时间",
	},
	/* ── 画幅 ── */
	"platform": {
		Group: "画幅", Label: "发布平台", Type: SettingTypeChoice,
		Default: "channels",
		Options: []SettingOption{
			{Value: "channels", Label: "视频号"},
			{Value: "xiaohongshu", Label: "小红书"},
			{Value: "landscape", Label: "横屏视频"},
		},
		Kinds: []string{"recommendation"},
	},
	/* ── 配乐 ── */
	"bgmGain": {
		Group: "配乐", Label: "BGM 音量", Type: SettingTypeNumber,
		Default: 0.12, Min: ptrFloat(0), Max: ptrFloat(0.5),
		Step: ptrFloat(0.01), Unit: "%", Display: "percent",
		Hint: "0–50%，数字越大越响",
	},
	"normalize": {
		Group: "配乐", Label: "统一响度", Type: SettingTypeBool,
		Default: true,
		Hint:    "把成品推到固定响度，不同片子放一起不会一轻一重",
	},
}

func ptrFloat(v float64) *float64 { return &v }

// PresetItem 一个预设（往默认配置上盖一层）。
type PresetItem struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Desc  string                 `json:"desc"`
	Steps map[string]interface{} `json:"settings"`
}

// Presets 常用组合。
var Presets = map[string]PresetItem{
	"default": {
		ID:    "default",
		Name:  "默认",
		Desc:  "通用设置：人声清楚，配乐垫底，成品响度统一",
		Steps: map[string]interface{}{},
	},
	"clear-speech": {
		ID:    "clear-speech",
		Name:  "口播清晰",
		Desc:  "以念白为主：配乐压到 8%，人声提到 1.5 倍",
		Steps: map[string]interface{}{"bgmGain": 0.08, "voiceGain": 1.5, "duck": true},
	},
	"ambient": {
		ID:    "ambient",
		Name:  "氛围铺底",
		Desc:  "以配乐为主：配乐提到 30%，人声只留一点",
		Steps: map[string]interface{}{"bgmGain": 0.3, "voiceGain": 1.1, "duck": true},
	},
	"raw": {
		ID:    "raw",
		Name:  "原样输出",
		Desc:  "不做响度归一化、不压配乐 —— 自己在后期软件里统一音量时用",
		Steps: map[string]interface{}{"normalize": false, "duck": false},
	},
}

// KindDefaults 各流水线自己的默认值。
var KindDefaults = map[string]map[string]interface{}{
	"talk":           {"pause": 0.45},
	"recommendation": {"pause": 0.35},
	"product":        {"pause": 0.4},
}

// appliesTo 该旋钮对这条流水线是否生效。
func appliesTo(field SettingField, kind string) bool {
	if len(field.Kinds) == 0 {
		return true
	}
	for _, k := range field.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// ListPresets 列出所有预设（只给前端看的部分）。
func ListPresets() []PresetItem {
	out := make([]PresetItem, 0, len(Presets))
	for _, p := range Presets {
		cp := p
		cp.Steps = cloneMap(p.Steps)
		out = append(out, cp)
	}
	return out
}

// ResolveSettings 收敛一份完整配置。
// 顺序：预设 → 流水线默认 → 字段默认 → 用户填的值。用户在预设之上改过的项优先。
func ResolveSettings(kind string, input map[string]interface{}, presetID string) map[string]interface{} {
	preset, hasPreset := Presets[presetID]
	if !hasPreset {
		preset = PresetItem{}
	}
	kindDef := KindDefaults[kind]

	out := make(map[string]interface{})
	for key, field := range SettingSchema {
		if !appliesTo(field, kind) {
			continue
		}
		value := field.Default
		if v, ok := kindDef[key]; ok {
			value = v
		}
		if hasPreset {
			if v, ok := preset.Steps[key]; ok {
				value = v
			}
		}
		if v, ok := input[key]; ok && v != nil {
			if s, isStr := v.(string); isStr && s != "" {
				value = s
			} else if !isStr {
				value = v
			}
		}
		out[key] = coerce(key, field, value)
	}
	return out
}

// coerce 把一个值收敛成合法值。
func coerce(key string, field SettingField, value interface{}) interface{} {
	switch field.Type {
	case SettingTypeBool:
		switch v := value.(type) {
		case bool:
			return v
		case string:
			switch v {
			case "true", "on", "1":
				return true
			case "false", "off", "0":
				return false
			}
			return field.Default
		case float64:
			return v == 1
		default:
			return field.Default
		}
	case SettingTypeNumber:
		n := toFloat(value)
		if n == nil {
			return field.Default
		}
		if field.Min != nil && *n < *field.Min {
			return *field.Min
		}
		if field.Max != nil && *n > *field.Max {
			return *field.Max
		}
		return *n
	case SettingTypeChoice:
		s := toString(value)
		for _, o := range field.Options {
			if o.Value == s {
				return s
			}
		}
		return field.Default
	default:
		return toString(value)
	}
}

// toFloat 尽力转成 float64。
func toFloat(v interface{}) *float64 {
	switch n := v.(type) {
	case float64:
		return &n
	case int:
		f := float64(n)
		return &f
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return nil
		}
		return &f
	default:
		return nil
	}
}

// toString 尽力转成 string。
func toString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		return ""
	}
}

// DescribeSetting 把某个旋钮的当前值配成人话。
func DescribeSetting(key string, value interface{}) string {
	field, ok := SettingSchema[key]
	if !ok {
		return toString(value)
	}
	switch field.Type {
	case SettingTypeBool:
		b, _ := value.(bool)
		if b {
			return "开"
		}
		return "关"
	case SettingTypeNumber:
		if field.Display == "percent" {
			if f, ok := value.(float64); ok {
				return strconv.Itoa(int(f*100)) + "%"
			}
			return toString(value)
		}
		if field.Unit == "倍" {
			return toString(value) + " 倍"
		}
		if field.Unit == "秒" {
			return toString(value) + " 秒"
		}
		return toString(value)
	case SettingTypeChoice:
		for _, o := range field.Options {
			if o.Value == toString(value) {
				return o.Label
			}
		}
		return toString(value)
	default:
		return toString(value)
	}
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
