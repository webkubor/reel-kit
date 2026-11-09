package app

// 导航只有三类东西：视频流水线、工具、设置。
//
// 之前的做法是把所有东西平铺成一行标签：
//
//	项目推荐 | 知识口播 | 视频素材 | 图片素材 | 配音素材 | 素材合成 | 设置与反馈
//
// 这里面混着三种完全不同的东西：
//   - 视频类型（项目推荐、知识口播）—— 每一条背后是一条流水线、一份生成配置
//   - 素材类别（视频素材、图片素材、配音素材）—— 只是素材，不是视频
//   - 工具与菜单（素材合成、设置与反馈）—— 不产出视频
//
// 平铺的结果是用户点进去之前根本不知道自己在选什么：
// 「项目推荐」是视频类型，「视频素材」是素材，「素材合成」又是工具。
//
// 现在的分法：
//
//	视频流水线：知识口播 / 项目推荐 / 素材合成 / 人物商品介绍
//	工具：素材库 / 生成素材
//	设置与反馈
//
// 一个视频类型 = 一条流水线。点进去，中间是这条流水线的生成配置表单，
// 右边是这条流水线跑出来的视频列表（条数、进度、每条的配置）。

// navItem 是左栏的一个导航项。
type navItem struct {
	Mode Mode
	Name string
	Desc string
	// Icon 是图标名（ui.Icon 里查）。
	Icon string
	// Enabled 为 false 时只展示不进入（能力还没开放）。
	Enabled bool
}

// videoPipelines 是能产出成片的流水线，顺序即左栏顺序。
var videoPipelines = []navItem{
	{ModeTalk, "知识口播", "写一句一镜的脚本，本机配音与合成", "talk", true},
	{ModeRecommend, "项目推荐", "读一个 GitHub 仓库，出推荐视频", "repo", true},
	{ModeExport, "素材合成", "把图片或片段合成一条视频", "film", true},
	{ModeProduct, "人物商品介绍", "给产品或人物出一条介绍片", "person", true},
}

// studioTools 是工具，不产出视频。
var studioTools = []navItem{
	{ModeLibrary, "素材库", "导入与浏览图片、视频、配音", "folder", true},
	{ModeGenerate, "生成素材", "用 AI 生成图片、视频或配音", "sparkle", true},
}

// pipelineKind 把导航项映射到后端任务类型。
func pipelineKind(mode Mode) string {
	switch mode {
	case ModeTalk:
		return "talk"
	case ModeRecommend:
		return "recommendation"
	case ModeExport:
		return "export"
	case ModeProduct:
		return "product"
	}
	return ""
}

// modeTitle 返回导航项的名字。
func modeTitle(mode Mode) string {
	for _, item := range videoPipelines {
		if item.Mode == mode {
			return item.Name
		}
	}
	for _, item := range studioTools {
		if item.Mode == mode {
			return item.Name
		}
	}
	switch mode {
	case ModeSettings:
		return "设置与反馈"
	}
	return "Reel Studio"
}

// modeDesc 返回导航项的一句话说明。
func modeDesc(mode Mode) string {
	for _, item := range videoPipelines {
		if item.Mode == mode {
			return item.Desc
		}
	}
	for _, item := range studioTools {
		if item.Mode == mode {
			return item.Desc
		}
	}
	return ""
}
