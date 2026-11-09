package ui

import "github.com/egoist/mygo/ui"

// 图标：16×16 的线性图标，描边用 currentColor，跟文字颜色一致。
//
// 为什么自己画而不是用现成图标包：这个界面只有十来个图标，
// 自己画一份就够，还不用为一个桌面应用拖进一整套依赖。
//
// 注意：画布就按 16×16 写死，不做缩放 —— 图标元件按原始尺寸绘制，
// 拿 24×24 的图硬缩到 16 会被裁掉一块（画出来是半个图形）。
var iconSources = map[string]string{
	// 知识口播：麦克风
	"talk": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<path d="M8 2.2a2 2 0 0 1 2 2v3.4a2 2 0 0 1-4 0V4.2a2 2 0 0 1 2-2Z"/>
		<path d="M4.4 7.6a3.6 3.6 0 0 0 7.2 0"/><path d="M8 11.2v2.3M6.2 13.5h3.6"/>
	</svg>`,
	// 项目推荐：仓库分支
	"repo": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<circle cx="4" cy="3.4" r="1.5"/><circle cx="12" cy="3.4" r="1.5"/><circle cx="11.4" cy="12.6" r="1.5"/>
		<path d="M4 4.9v3.4a3 3 0 0 0 3 3h4.4"/><path d="M12 4.9v1.5a3 3 0 0 1-3 3H5"/>
	</svg>`,
	// 素材合成：胶片
	"film": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<rect x="2.5" y="3.5" width="11" height="9" rx="1.4"/>
		<path d="M5.8 3.5v9M10.2 3.5v9M2.5 6.6h3.3M10.2 6.6h3.3M2.5 9.6h3.3M10.2 9.6h3.3"/>
	</svg>`,
	// 人物商品介绍：人像
	"person": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<circle cx="8" cy="5.8" r="2.9"/><path d="M3.4 13.8a4.8 4.8 0 0 1 9.2 0"/>
	</svg>`,
	// 素材库：文件夹
	"folder": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<path d="M2.2 5.6a1.3 1.3 0 0 1 1.3-1.3h2.3l1.3 1.5h5.7a1.3 1.3 0 0 1 1.3 1.3v5.9a1.3 1.3 0 0 1-1.3 1.3H3.5a1.3 1.3 0 0 1-1.3-1.3Z"/>
	</svg>`,
	// 生成素材：闪光
	"sparkle": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<path d="M6.6 2.4 7.9 5.8l3.4 1.2-3.4 1.2-1.3 3.4-1.2-3.4-3.4-1.2 3.4-1.2Z"/>
		<path d="M12.2 9.4l.7 1.9 1.9.7-1.9.7-.7 1.9-.7-1.9-1.9-.7 1.9-.7Z"/>
	</svg>`,
	// 设置：推子
	"settings": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<path d="M2.4 4.5h11.2M2.4 8h11.2M2.4 11.5h11.2"/>
		<circle cx="10.2" cy="4.5" r="1.6"/><circle cx="5.6" cy="8" r="1.6"/><circle cx="9.4" cy="11.5" r="1.6"/>
	</svg>`,
	// 完成：对勾
	"done": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2">
		<path d="M3.4 8.4 6.4 11.4 12.6 4.8"/>
	</svg>`,
	// 进行中：时钟
	"clock": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<circle cx="8" cy="8" r="5.8"/><path d="M8 4.8V8l2.2 1.4"/>
	</svg>`,
	// 失败：感叹号
	"alert": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<path d="M8 2.4 14 13.1a1.1 1.1 0 0 1-1 1.7H3a1.1 1.1 0 0 1-1-1.7Z"/>
		<path d="M8 6.4v3.2M8 12v.5"/>
	</svg>`,
	// 新建：加号
	"plus": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8">
		<path d="M8 3.6v8.8M3.6 8h8.8"/>
	</svg>`,
	// 对比：两栏
	"compare": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<rect x="2.6" y="3" width="4.3" height="10" rx="1.2"/><rect x="9.1" y="3" width="4.3" height="10" rx="1.2"/>
	</svg>`,
	// 保存：向下箭头进托盘
	"save": `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
		<path d="M8 2.8v7M5.2 7.4 8 10.2l2.8-2.8"/><path d="M3.4 12.2v.6a1 1 0 0 0 1 1h7.2a1 1 0 0 0 1-1v-.6"/>
	</svg>`,
}

// icons 缓存解析结果：解析一次，界面上反复用。
var icons = map[string]*ui.SVG{}

func init() {
	for name, source := range iconSources {
		svg, err := ui.ParseSVG([]byte(source))
		if err != nil {
			continue
		}
		icons[name] = svg
	}
}

// Icon 按名字取图标。名字不对时返回 nil（界面会跳过它，不至于画出一坨）。
func Icon(name string) *ui.SVG {
	return icons[name]
}
