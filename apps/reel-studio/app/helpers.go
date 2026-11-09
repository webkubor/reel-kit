package app

import (
	"github.com/egoist/mygo/ui"
	kit "github.com/webkubor/reel-kit/apps/reel-studio/ui"
)

// kitButton 是个薄封装，让「按钮」这件事在应用层只有一处入口。
func kitButton(c kit.Comps, ctx *ui.Context, variant, label string, disabled bool) ui.Element {
	return c.Action(ctx, variant, label, disabled)
}
