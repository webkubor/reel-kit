// Package design 是原生界面的设计令牌层。
//
// 这里的每一个值都来自工作台网页版的品牌变量（studio/style.css 的 :root），
// 不是另起一套：同一个产品，桌面版与网页版必须看起来是同一个东西。
//
// 令牌分三层：
//   - 色板：品牌色只有 accent（砖红）与 green（橄榄绿）两个，其余是中性灰阶
//   - 排版：字号档位少而清晰，靠字重与颜色拉开层级，不靠一屏十几种字号
//   - 尺度：间距、圆角、控件高度
//
// 规则：任何界面代码都不得写死颜色或字号，一律从这里取。
package design

import "github.com/egoist/mygo/ui"

// 品牌色。accent 是砖红，用在主操作与当前选中态；green 是橄榄绿，只表示"完成/就绪"。
const (
	AccentHex    = "#a7442d"
	AccentDeep   = "#903922"
	AccentLight  = "#f6eae3"
	GreenHex     = "#52725c"
	GreenLight   = "#e7efe9"
	PaperHex     = "#f6f5f1"
	SurfaceHex   = "#ffffff"
	InkHex       = "#292c28"
	MutedHex     = "#74786f"
	LineHex      = "#e4e5de"
	FieldFillHex = "#fdfdfb"
)

// 排版档位。数值来自网页版的实际使用，不是拍脑袋。
//
// 网页版正文 12px、副标题 13px、标题 15–17px、大标题 30px。
// 桌面版在网页版刻度上整体加一档：桌面是 1:1 的原生控件，
// 没有浏览器缩放兜底，12px 正文在一屏 1280 宽的窗口里读起来太小。
const (
	FontTiny   = 11.0 // 辅助信息、标签
	FontSmall  = 12.0 // 次要说明
	FontBody   = 13.0 // 正文
	FontMedium = 14.0 // 按钮、控件文字
	FontTitle  = 16.0 // 区块标题
	FontLarge  = 18.0 // 卡片标题
	FontBrand  = 24.0 // 品牌字
	FontHero   = 30.0 // 页面主标题
)

// 间距刻度。8 为基数，但小间距取 4、6，保证紧凑处不显得散。
const (
	SpaceXS = 4.0
	SpaceSM = 6.0
	SpaceMD = 8.0
	SpaceLG = 12.0
	SpaceXL = 18.0
	Space2X = 24.0
	Space3X = 32.0
)

// 圆角与控件高度。
const (
	RadiusSM    = 5.0
	RadiusMD    = 7.0
	RadiusLG    = 10.0
	ControlHigh = 39.0 // 输入框、下拉
	// ButtonHigh 是按钮高度。
	//
	// 从 38 降到 34：按钮跟输入框（39）同高时，一行里并排三个按钮
	// 会显得又高又笨；34 仍然好点按，但一屏能多放下内容。
	ButtonHigh = 34.0
)

// Tokens 是一套完整的界面令牌：色板 + 尺度。
//
// 浅色与深色各一套。深色不是把浅色反过来 —— 纸面变深、墨色变亮，
// 品牌色的饱和度也要降一点，否则在深底上会刺眼。
type Tokens struct {
	Paper      ui.Color // 窗口底色
	Surface    ui.Color // 卡片、面板
	SurfaceAlt ui.Color // 次级面（输入框、标签块）
	Hover      ui.Color // 悬停底色（导航项、弱按钮）
	Ink        ui.Color // 主文字
	Muted      ui.Color // 次要文字
	Line       ui.Color // 分隔线、描边
	Accent     ui.Color // 品牌强调色
	AccentDeep ui.Color // 强调色的按下态
	AccentSoft ui.Color // 强调色的浅底
	AccentText ui.Color // 强调色底上的文字
	Green      ui.Color // 就绪、成功
	GreenSoft  ui.Color // 成功标签的浅底
	Shadow     ui.Color // 卡片投影
}

// Light 是浅色令牌。
func Light() Tokens {
	return Tokens{
		Paper:      ui.Hex(PaperHex),
		Surface:    ui.Hex(SurfaceHex),
		SurfaceAlt: ui.Hex(FieldFillHex),
		Hover:      ui.Hex(SurfaceHex).Mix(ui.Hex(MutedHex), 0.07),
		Ink:        ui.Hex(InkHex),
		Muted:      ui.Hex(MutedHex),
		Line:       ui.Hex(LineHex),
		Accent:     ui.Hex(AccentHex),
		AccentDeep: ui.Hex(AccentDeep),
		AccentSoft: ui.Hex(AccentLight),
		AccentText: ui.Hex("#ffffff"),
		Green:      ui.Hex(GreenHex),
		GreenSoft:  ui.Hex(GreenLight),
		Shadow:     ui.RGBA(41, 44, 40, 0.05),
	}
}

// Dark 是深色令牌。
func Dark() Tokens {
	return Tokens{
		Paper:      ui.Hex("#1c1e1b"),
		Surface:    ui.Hex("#242723"),
		SurfaceAlt: ui.Hex("#2b2e29"),
		Hover:      ui.Hex("#242723").Mix(ui.Hex("#9aa094"), 0.12),
		Ink:        ui.Hex("#e8eae3"),
		Muted:      ui.Hex("#9aa094"),
		Line:       ui.Hex("#3a3e37"),
		Accent:     ui.Hex("#c26a4f"),
		AccentDeep: ui.Hex("#a95239"),
		AccentSoft: ui.Hex("#3a2a23"),
		AccentText: ui.Hex("#ffffff"),
		Green:      ui.Hex("#7fa087"),
		GreenSoft:  ui.Hex("#28332b"),
		Shadow:     ui.RGBA(0, 0, 0, 0.35),
	}
}

// Theme 把令牌翻译成 MyGo 的主题，供控件取用。
func (t Tokens) Theme() *ui.Theme {
	th := ui.LightTheme()
	th.Background = t.Paper
	th.Surface = t.Surface
	th.SurfaceHover = t.Surface.Mix(t.Muted, 0.08)
	th.SurfacePressed = t.Surface.Mix(t.Muted, 0.16)
	th.Border = t.Line
	th.Text = t.Ink
	th.TextMuted = t.Muted
	th.Accent = t.Accent
	th.AccentHover = t.Accent.Mix(t.Ink, 0.08)
	th.AccentPressed = t.AccentDeep
	th.AccentText = t.AccentText
	th.Success = t.Green
	th.Radius = 7
	th.FontSize = 13
	return th
}

// DarkTheme 是深色主题。
func (t Tokens) DarkTheme() *ui.Theme {
	th := t.Theme()
	th.Dark = true
	return th
}
