package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/webkubor/reel-kit/apps/reel-studio/design"
)

// Test说明文字紧跟控件
//
// 踩过的坑：MyGo 的 LineHeight 是**字号的倍数**，不是像素。
// 写 LineHeight(15) 会让一行 11px 的说明文字占掉 165px，
// 界面上就变成「输入框下面空一大截，说明文字掉到很远」。
// 这里用几何断言把这件事钉住：一行说明就只占一行的高度。
func Test说明文字紧跟控件(t *testing.T) {
	k := New(design.Light())
	value := ""
	tt := ui.NewTester(func(c *ui.Context) {
		k.Field(c, "标题", "这是控件下面的说明", func() {
			k.Input(c, &value, "控件里的提示")
		})
		k.Field(c, "副标题", "下一条的说明", func() {
			k.Input(c, &value, "下一条的提示")
		})
	}, 420, 260)
	tt.Frame()

	hint, ok := tt.Find("这是控件下面的说明")
	if !ok {
		t.Fatalf("没有画出说明文字，实际文字：%q", tt.Texts())
	}
	if hint.H > 30 {
		t.Errorf("一行说明占了 %.0f 像素高：行高写成了字号倍数（要用 FixedLineHeight）", hint.H)
	}
	next, ok := tt.Find("副标题")
	if !ok {
		t.Fatalf("没有画出下一个字段，实际文字：%q", tt.Texts())
	}
	gap := next.Y - (hint.Y + hint.H)
	if gap > 24 {
		t.Errorf("说明文字与下一个字段之间空了 %.0f 像素：说明不该把版面撑开", gap)
	}
}
