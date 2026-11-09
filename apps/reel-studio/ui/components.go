package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
)

// 这一层是常用件：把同一个产品语义收敛成一个控件，
// 让界面各处看起来是同一套东西，而不是各写各的。
//
// 之前的界面每页各写各的：同一个输入框在三个地方有三种高度，
// 同一个状态标签在两处有两种颜色，一屏看下来全是散的。
// 现在导航项、区块标题、卡片、字段、列表行、键值行、进度条、
// 统计块、标签、按钮都在这里定好，界面代码只管摆位置。

// 尺寸刻度。
const (
	// IconSize 是导航与状态里的图标尺寸。
	IconSize = 16.0
	// IconSizeSmall 是行内小图标。
	IconSizeSmall = 13.0
)

// NavItem 是左栏的导航条目：图标 + 名称 + 计数。
//
// 选中态用「浅底 + 左侧强调条」，不用整块重色 ——
// 一屏十几个条目，重色会互相抢。
//
// disabled 的条目照常显示，但文字转灰并挂一个「未开放」标记：
// 藏起来等于没做，标出来才是「还没做」。
func (c Comps) NavItem(ctx *ui.Context, icon *ui.SVG, label, count string, selected, disabled bool) ui.Element {
	ink, countInk, countFill := c.T.Ink, c.T.Muted, c.T.SurfaceAlt
	fill := c.T.Surface
	if selected {
		fill = c.T.AccentSoft
		ink = c.T.Accent
		countInk = c.T.Accent
		countFill = c.T.Surface
	}
	if disabled {
		ink = c.T.Muted
	}
	return ui.ButtonBase(ctx).
		FillWidth().
		Background(fill).
		Radius(design.RadiusSM).
		Padding(design.SpaceSM, design.SpaceMD).
		Gap(design.SpaceSM).
		AlignItems(ui.Center).
		Children(func() {
			if selected {
				ui.Column(ctx).Width(3).Height(14).Radius(2).Background(c.T.Accent)
			}
			if icon != nil {
				ui.Icon(ctx, icon).Width(IconSize).Height(IconSize).TextColor(ink)
			}
			ui.Text(ctx, label).Grow(1).FontSize(design.FontMedium).TextColor(ink).TextAlign(ui.Start)
			if disabled {
				ui.Text(ctx, "未开放").FontSize(design.FontTiny).TextColor(c.T.Muted).
					Background(c.T.SurfaceAlt).Radius(design.RadiusSM).
					Padding(design.SpaceXS, design.SpaceSM)
			} else if count != "" {
				ui.Text(ctx, count).FontSize(design.FontTiny).TextColor(countInk).
					Background(countFill).Radius(design.RadiusSM).
					Padding(design.SpaceXS, design.SpaceSM).MinWidth(22).TextAlign(ui.Center)
			}
		})
}

// NavLabel 是导航里的分组标题（视频流水线 / 工具）。
func (c Comps) NavLabel(ctx *ui.Context, text string) ui.Element {
	return ui.Text(ctx, text).FontSize(design.FontTiny).TextColor(c.T.Muted).LetterSpacing(0.6)
}

// PageHeader 是页面顶部的标题区：标题 + 一句话说明 + 右侧动作。
//
// 用 Grow 而不是 FillWidth：它总是跟别的控件并排（顶栏右边还有工作空间），
// 填满宽度会把旁边的控件挤出去。
func (c Comps) PageHeader(ctx *ui.Context, title, subtitle string, actions func()) ui.Element {
	return ui.Row(ctx).Grow(1).MinWidth(0).Gap(design.SpaceLG).AlignItems(ui.Center).Children(func() {
		ui.Column(ctx).Gap(design.SpaceXS).Grow(1).MinWidth(0).Children(func() {
			ui.Text(ctx, title).FontSize(design.FontTitle).TextColor(c.T.Ink).TextAlign(ui.Start)
			if subtitle != "" {
				ui.Text(ctx, subtitle).FontSize(design.FontSmall).TextColor(c.T.Muted).TextAlign(ui.Start)
			}
		})
		if actions != nil {
			actions()
		}
	})
}

// Panel 是一块内容面板：白底、描边、圆角、很轻的投影。
func (c Comps) Panel(ctx *ui.Context, content func()) ui.Element {
	return ui.Column(ctx).FillWidth().Gap(design.SpaceLG).
		Background(c.T.Surface).Border(1, c.T.Line).Radius(design.RadiusLG).
		Shadow(0, 1, 6, 0, c.T.Shadow).Padding(design.SpaceXL).Children(content)
}

// PartTitle 是「一条视频由哪几部分组成」里的部分标题。
//
// 前面带一小段强调色：一屏七块，靠字号分不出层次，靠色条才分得出。
func (c Comps) PartTitle(ctx *ui.Context, name string) ui.Element {
	return ui.Row(ctx).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
		ui.Column(ctx).Width(3).Height(12).Radius(2).Background(c.T.Accent)
		ui.Text(ctx, name).FontSize(design.FontSmall).Bold().TextColor(c.T.Ink)
	})
}

// ListItem 是列表里的一行：标题 + 副标题 + 右侧内容。
//
// selected 给当前选中项一条左侧强调条，和导航一致。
func (c Comps) ListItem(ctx *ui.Context, title, subtitle string, trailing func(), selected bool) ui.Element {
	return ui.Column(ctx).FillWidth().Gap(design.SpaceSM).
		Background(c.T.Surface).Border(1, c.T.Line).Radius(design.RadiusMD).
		Padding(design.SpaceMD, design.SpaceLG).Children(func() {
		ui.Row(ctx).FillWidth().Gap(design.SpaceSM).AlignItems(ui.Center).Children(func() {
			if selected {
				ui.Column(ctx).Width(3).Height(14).Radius(2).Background(c.T.Accent)
			}
			ui.Column(ctx).Gap(design.SpaceXS).Grow(1).Children(func() {
				ui.Text(ctx, title).FontSize(design.FontBody).TextColor(c.T.Ink).TextAlign(ui.Start)
				if subtitle != "" {
					ui.Text(ctx, subtitle).FontSize(design.FontTiny).TextColor(c.T.Muted).TextAlign(ui.Start)
				}
			})
			if trailing != nil {
				trailing()
			}
		})
	})
}

// KeyValue 是「字段 → 值」的一行：左边的字段名，右边的实际值。
func (c Comps) KeyValue(ctx *ui.Context, label, value string) ui.Element {
	return ui.Row(ctx).FillWidth().Gap(design.SpaceMD).AlignItems(ui.Start).Children(func() {
		ui.Text(ctx, label).Width(88).FontSize(design.FontSmall).TextColor(c.T.Muted)
		ui.Text(ctx, value).Grow(1).FontSize(design.FontSmall).TextColor(c.T.Ink)
	})
}

// Meter 是进度条 + 一行说明。
//
// determinate 为 false 时画成不确定进度 ——
// 宁可告诉用户「不知道多快」，也不要给一个假的百分比。
func (c Comps) Meter(ctx *ui.Context, value float32, label string, determinate bool) ui.Element {
	return ui.Column(ctx).FillWidth().Gap(design.SpaceXS).Children(func() {
		if determinate {
			ui.Progress(ctx, float64(value)).FillWidth().Height(4).
				Radius(design.RadiusSM).TextColor(c.T.Accent)
		} else {
			ui.Progress(ctx, -1).FillWidth().Height(4).
				Radius(design.RadiusSM).TextColor(c.T.Accent)
		}
		if label != "" {
			ui.Text(ctx, label).FontSize(design.FontTiny).TextColor(c.T.Muted)
		}
	})
}

// Stat 是一个数字加一行说明：条数、时长、占比。
func (c Comps) Stat(ctx *ui.Context, value, label string) ui.Element {
	return ui.Column(ctx).Gap(design.SpaceXS).Children(func() {
		ui.Text(ctx, value).FontSize(design.FontLarge).Bold().TextColor(c.T.Ink)
		ui.Text(ctx, label).FontSize(design.FontTiny).TextColor(c.T.Muted)
	})
}

// Chip 是一个可选中的小标签。
func (c Comps) Chip(ctx *ui.Context, label string, selected bool) ui.Element {
	fill, ink := c.T.Surface, c.T.Muted
	if selected {
		fill, ink = c.T.AccentSoft, c.T.Accent
	}
	return ui.Text(ctx, label).FontSize(design.FontTiny).TextColor(ink).
		Background(fill).Border(1, c.T.Line).Radius(design.RadiusSM).
		Padding(design.SpaceXS, design.SpaceMD)
}

// Divider 是分隔线。
func (c Comps) Divider(ctx *ui.Context) ui.Element {
	return ui.Column(ctx).FillWidth().Height(1).Background(c.T.Line)
}

// SoftButton 是弱操作按钮：只有文字，鼠标在上面才显出底色。
func (c Comps) SoftButton(ctx *ui.Context, label string, disabled bool) ui.Element {
	return ui.Button(ctx, label).
		FontSize(design.FontSmall).TextColor(c.T.Muted).
		Radius(design.RadiusSM).Padding(design.SpaceXS, design.SpaceMD).
		Disabled(disabled)
}
