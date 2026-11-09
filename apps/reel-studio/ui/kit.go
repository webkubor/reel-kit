// Package ui 提供工作台的界面组件。
//
// 这里不是"再写一遍 MyGo 的控件"，而是把产品语义包起来：
// 一个 Primary 按钮知道自己是主操作，一个 Field 知道标签该长什么样，
// 一个 Card 知道自己是块内容而不是装饰。
//
// 好处是换主题、换控件库时只改这一层，界面代码不动。
package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
)

// Comps 持有一套令牌，供组件取用。
type Comps struct {
	T design.Tokens
}

// New 返回一组组件。
func New(t design.Tokens) Comps { return Comps{T: t} }

// Button 变体。
const (
	// ButtonPrimary 是页面上的主操作，一个视图里最多一个。
	ButtonPrimary = "primary"
	// ButtonSecondary 是次要操作，描边。
	ButtonSecondary = "secondary"
	// ButtonQuiet 是弱操作，只有文字（复用参数、查看连接状态）。
	ButtonQuiet = "quiet"
	// ButtonDanger 是危险操作。
	ButtonDanger = "danger"
)

// Action 渲染一个按钮。variant 决定外观，label 是文字，disabled 置灰。
//
// 四种变体对应四种意图，一个视图里主操作最多一个：
//
//	primary   —— 这一屏要做的那件事
//	secondary —— 旁边的另一个动作
//	quiet     —— 行内的小动作（查看、对比、取消）
//	danger    —— 会丢东西的动作
func (c Comps) Action(ctx *ui.Context, variant, label string, disabled bool) ui.Element {
	var el ui.Element
	switch variant {
	case ButtonPrimary:
		el = ui.PrimaryButton(ctx, label)
		el = el.Radius(design.RadiusSM).Padding(0, design.SpaceXL).Height(design.ButtonHigh)
		el = el.Bold()
	case ButtonSecondary:
		el = ui.Button(ctx, label)
		el = el.Radius(design.RadiusSM).Padding(0, design.SpaceXL).Height(design.ButtonHigh)
		el = el.Background(c.T.Surface).Border(1, c.T.Line).TextColor(c.T.Ink)
	case ButtonDanger:
		el = ui.Button(ctx, label)
		el = el.Radius(design.RadiusSM).Padding(0, design.SpaceXL).Height(design.ButtonHigh)
		el = el.Background(c.T.Surface).Border(1, c.T.Accent).TextColor(c.T.Accent)
	default:
		el = ui.Button(ctx, label)
		el = el.TextColor(c.T.Muted).Radius(design.RadiusSM).Padding(design.SpaceXS, design.SpaceMD)
	}
	return el.FontSize(design.FontMedium).Disabled(disabled)
}

// Field 是一个带标签的表单项：标签在上、内容在下、说明在下再下。
//
// 网页版每个字段都要手写三段结构，这里统一掉。
func (c Comps) Field(ctx *ui.Context, label, help string, content func()) ui.Element {
	return ui.Column(ctx).Gap(design.SpaceSM).Children(func() {
		ui.Text(ctx, label).FontSize(design.FontMedium).TextColor(c.T.Ink)
		content()
		if help != "" {
			ui.Text(ctx, help).FontSize(design.FontTiny).TextColor(c.T.Muted).FixedLineHeight(15)
		}
	})
}

// Input 是单行输入框。
//
// 输入框是浅底 + 细描边：跟面板的白底拉开一层，
// 一眼能看出「这里能打字」，不用靠光标去试。
func (c Comps) Input(ctx *ui.Context, value *string, placeholder string) ui.Element {
	return ui.TextInput(ctx, value).Placeholder(placeholder).
		Background(c.T.SurfaceAlt).Border(1, c.T.Line).
		Radius(design.RadiusSM).Height(design.ControlHigh).Padding(design.SpaceMD, 0).
		FontSize(design.FontBody).TextColor(c.T.Ink).PlaceholderColor(c.T.Muted).
		CaretColor(c.T.Accent)
}

// TextArea 是多行输入框。
func (c Comps) TextArea(ctx *ui.Context, value *string, placeholder string, minHeight float32) ui.Element {
	return ui.TextArea(ctx, value).Placeholder(placeholder).
		Background(c.T.SurfaceAlt).Border(1, c.T.Line).
		Radius(design.RadiusSM).Padding(design.SpaceMD).FontSize(design.FontBody).
		TextColor(c.T.Ink).PlaceholderColor(c.T.Muted).CaretColor(c.T.Accent).
		Height(minHeight).FixedLineHeight(18)
}

// Select 是下拉选择。
//
// options 是显示文字；选择结果写回 selected（与某个 option 相同的文字）。
func (c Comps) Select(ctx *ui.Context, selected *string, options []string, placeholder string) ui.Element {
	el := ui.Select(ctx, selected, options).
		Background(c.T.SurfaceAlt).Border(1, c.T.Line).
		Radius(design.RadiusSM).Height(design.ControlHigh).Padding(design.SpaceMD, 0).
		FontSize(design.FontBody).TextColor(c.T.Ink)
	if placeholder != "" {
		el = el.Placeholder(placeholder)
	}
	return el
}

// Checkbox 是复选框。
func (c Comps) Checkbox(ctx *ui.Context, checked *bool, label string) ui.Element {
	return ui.Checkbox(ctx, checked, label).FontSize(design.FontBody)
}

// Card 是一块内容面板：白底、描边、圆角。
func (c Comps) Card(ctx *ui.Context, content func()) ui.Element {
	return ui.Column(ctx).FillWidth().Gap(design.SpaceLG).
		Background(c.T.Surface).Border(1, c.T.Line).Radius(design.RadiusLG).
		Padding(design.SpaceXL).Children(content)
}

// SectionTitle 是区块标题，右侧可放一个附属动作。
func (c Comps) SectionTitle(ctx *ui.Context, title, note string, action func()) ui.Element {
	return ui.Row(ctx).AlignItems(ui.Center).Gap(design.SpaceLG).Children(func() {
		ui.Column(ctx).Gap(design.SpaceXS).Grow(1).Children(func() {
			ui.Text(ctx, title).FontSize(design.FontTitle).TextColor(c.T.Ink)
			if note != "" {
				ui.Text(ctx, note).FontSize(design.FontTiny).TextColor(c.T.Muted)
			}
		})
		if action != nil {
			action()
		}
	})
}

// Tag 是状态标签：来源、就绪与否、成功与否。
//
// kind 决定颜色，只有三种：中性（默认）、品牌（accent）、成功（green）。
// 不给标签加第四种颜色 —— 状态多到要第五种颜色时，问题在流程不在配色。
func (c Comps) Tag(ctx *ui.Context, text, kind string) ui.Element {
	fill, ink := c.T.SurfaceAlt, c.T.Muted
	switch kind {
	case "brand":
		fill, ink = c.T.AccentSoft, c.T.Accent
	case "success":
		fill, ink = c.T.GreenSoft, c.T.Green
	}
	return ui.Text(ctx, text).FontSize(design.FontTiny).TextColor(ink).
		Background(fill).Radius(design.RadiusSM).Padding(design.SpaceXS, design.SpaceMD).
		MinWidth(0).TextAlign(ui.Center)
}

// Hint 是一行提示文字。
func (c Comps) Hint(ctx *ui.Context, text string) ui.Element {
	return ui.Text(ctx, text).FontSize(design.FontTiny).TextColor(c.T.Muted)
}

// Notice 是一条提示条：浅底 + 左侧强调，用来放生成前的说明与错误。
func (c Comps) Notice(ctx *ui.Context, text string, kind string) ui.Element {
	fill, ink := c.T.AccentSoft, c.T.Accent
	if kind == "error" {
		fill, ink = c.T.AccentSoft, c.T.Accent
	}
	// 左边一道色条：提示和正文一眼分得开，不用靠底色猜。
	return ui.Row(ctx).FillWidth().Gap(design.SpaceMD).AlignItems(ui.Center).
		Background(fill).Radius(design.RadiusSM).Padding(design.SpaceMD, design.SpaceLG).Children(func() {
		ui.Column(ctx).Width(3).Height(16).Radius(2).Background(ink)
		ui.Text(ctx, text).Grow(1).FontSize(design.FontBody).TextColor(ink).FixedLineHeight(17)
	})
}

// Empty 是一块空态：标题 + 说明 + 可选动作。
func (c Comps) Empty(ctx *ui.Context, title, note string, action func()) ui.Element {
	return ui.Column(ctx).FillWidth().Gap(design.SpaceMD).AlignItems(ui.Center).
		Padding(design.Space2X).Background(c.T.SurfaceAlt).Border(1, c.T.Line).
		Radius(design.RadiusMD).Children(func() {
		ui.Text(ctx, title).FontSize(design.FontMedium).TextColor(c.T.Ink)
		if note != "" {
			ui.Text(ctx, note).FontSize(design.FontSmall).TextColor(c.T.Muted).TextAlign(ui.Center)
		}
		if action != nil {
			action()
		}
	})
}

// Muted 是次要文字。
func (c Comps) Muted(ctx *ui.Context, text string) ui.Element {
	return ui.Text(ctx, text).FontSize(design.FontSmall).TextColor(c.T.Muted)
}

// Body 是正文文字。
func (c Comps) Body(ctx *ui.Context, text string) ui.Element {
	return ui.Text(ctx, text).FontSize(design.FontBody).TextColor(c.T.Ink)
}

// Strong 是强调正文。
func (c Comps) Strong(ctx *ui.Context, text string) ui.Element {
	return ui.Text(ctx, text).FontSize(design.FontMedium).TextColor(c.T.Ink)
}
