package main

// StylePalette 一套风格的配色与推荐模板。
type StylePalette struct {
	Name     string `json:"name"`
	Bg       string `json:"bg"`
	Accent1  string `json:"accent1"`
	Accent2  string `json:"accent2"`
	Template string `json:"template"`
}

// StylePalettes 44 套知识短视频视觉风格库的具体配色方案
// （底色、主强调、辅强调）。
var StylePalettes = map[string]StylePalette{
	// 东方家族
	"gufeng":   {Name: "古风", Bg: "#140d0b", Accent1: "#d9472b", Accent2: "#e2b45a", Template: "knowledge-gufeng"},
	"paper":    {Name: "水墨纸本", Bg: "#ece4d4", Accent1: "#b23a2e", Accent2: "#3c6e71", Template: "knowledge-paper"},
	"dunhuang": {Name: "敦煌", Bg: "#2b1d16", Accent1: "#c85a32", Accent2: "#2a7b72", Template: "knowledge-gufeng"},
	"qinghua":  {Name: "青花", Bg: "#f4f6fa", Accent1: "#1b3b82", Accent2: "#4a6fa5", Template: "knowledge-paper"},
	"woodcut":  {Name: "木刻", Bg: "#f0ebd8", Accent1: "#1c1b18", Accent2: "#9c2a2a", Template: "knowledge-paper"},
	"minimal":  {Name: "留白极简", Bg: "#ffffff", Accent1: "#111111", Accent2: "#888888", Template: "knowledge-paper"},
	"scroll":   {Name: "长卷", Bg: "#16100c", Accent1: "#e0a94e", Accent2: "#c24830", Template: "knowledge-gufeng"},
	// 暗场概念家族
	"starfield": {Name: "星空光点", Bg: "#05070d", Accent1: "#f2c46d", Accent2: "#7fb8ff", Template: "knowledge-dark"},
	"darkgold":  {Name: "暗金线描", Bg: "#0b0a08", Accent1: "#f0b44c", Accent2: "#ff6a3d", Template: "knowledge-dark"},
	"darkmath":  {Name: "深色数学", Bg: "#0f1014", Accent1: "#58c4dd", Accent2: "#f4d35e", Template: "knowledge-dark"},
	"neon":      {Name: "霓虹赛博", Bg: "#08080f", Accent1: "#00ffcc", Accent2: "#ff007f", Template: "knowledge-dark"},
	"keynote":   {Name: "发布会极简", Bg: "#000000", Accent1: "#0071e3", Accent2: "#86868b", Template: "knowledge-dark"},
	"aurora":    {Name: "极光", Bg: "#070b12", Accent1: "#4ef2bb", Accent2: "#9d4edd", Template: "knowledge-dark"},
	"deco":      {Name: "装饰艺术金线", Bg: "#0d131a", Accent1: "#dfba73", Accent2: "#2a5a52", Template: "knowledge-dark"},
	// 图解与设计家族
	"chalk":     {Name: "粉笔黑板", Bg: "#1c2421", Accent1: "#f4f1de", Accent2: "#e07a5f", Template: "knowledge-dark"},
	"blueprint": {Name: "蓝图专利", Bg: "#0b2545", Accent1: "#8da9c4", Accent2: "#eef4f8", Template: "knowledge-dark"},
	"isometric": {Name: "等轴测剖面", Bg: "#1a1c23", Accent1: "#3a86ff", Accent2: "#ffbe0b", Template: "knowledge-dark"},
	"swiss":     {Name: "瑞士网格", Bg: "#f2f2f2", Accent1: "#e63946", Accent2: "#1d3557", Template: "knowledge-paper"},
	"metro":     {Name: "地铁线路图", Bg: "#ffffff", Accent1: "#e31b23", Accent2: "#0072ce", Template: "knowledge-paper"},
	"isotype":   {Name: "象形统计", Bg: "#faf7f2", Accent1: "#264653", Accent2: "#e76f51", Template: "knowledge-paper"},
	"sketch":    {Name: "手绘过程", Bg: "#f5f0e6", Accent1: "#2b2b2b", Accent2: "#4a6fa5", Template: "knowledge-paper"},
	// 印刷与纸家族
	"riso":      {Name: "孔版印刷", Bg: "#f7f4ee", Accent1: "#ff4848", Accent2: "#2b59c3", Template: "knowledge-paper"},
	"collage":   {Name: "半调拼贴", Bg: "#dfd7c5", Accent1: "#a62b2b", Accent2: "#233d4d", Template: "knowledge-paper"},
	"ransom":    {Name: "剪报拼字", Bg: "#1f1e1c", Accent1: "#f4d06f", Accent2: "#e74c3c", Template: "knowledge-dark"},
	"editorial": {Name: "杂志编辑", Bg: "#f8f8f9", Accent1: "#111827", Accent2: "#4f46e5", Template: "knowledge-paper"},
	"poster":    {Name: "国家公园海报", Bg: "#2d3748", Accent1: "#f6ad55", Accent2: "#68d391", Template: "knowledge-dark"},
	// 剪辑与纪实家族
	"caption-news":  {Name: "字幕新闻", Bg: "#0d1117", Accent1: "#ff3344", Accent2: "#ffffff", Template: "knowledge-dark"},
	"investigation": {Name: "视觉调查", Bg: "#121418", Accent1: "#ffcc00", Accent2: "#00ccff", Template: "knowledge-dark"},
	"desktop":       {Name: "复古桌面", Bg: "#1e1e2e", Accent1: "#f38ba8", Accent2: "#89b4fa", Template: "knowledge-dark"},
	"lighttable":    {Name: "档案灯台", Bg: "#212529", Accent1: "#4dabf7", Accent2: "#ffd43b", Template: "knowledge-dark"},
	"tvnews":        {Name: "电视包装", Bg: "#0a192f", Accent1: "#ff0033", Accent2: "#64ffda", Template: "knowledge-dark"},
	// 游戏家族
	"pixel": {Name: "经典像素", Bg: "#10122a", Accent1: "#ffd23f", Accent2: "#3fe0a0", Template: "knowledge-dark"},
	"rpg":   {Name: "RPG", Bg: "#0b0e14", Accent1: "#e2b45a", Accent2: "#4a90e2", Template: "knowledge-dark"},
	"cards": {Name: "卡牌对战", Bg: "#1a1829", Accent1: "#c084fc", Accent2: "#fbbf24", Template: "knowledge-dark"},
	"farm":  {Name: "田园像素", Bg: "#2b3a2f", Accent1: "#f6d89b", Accent2: "#7ebc89", Template: "knowledge-dark"},
	// 明亮流行家族
	"variety": {Name: "综艺花字", Bg: "#fffbe6", Accent1: "#ff4d4f", Accent2: "#faad14", Template: "knowledge-paper"},
	"memphis": {Name: "孟菲斯波普", Bg: "#fff0f5", Accent1: "#ff1493", Accent2: "#00ffff", Template: "knowledge-paper"},
	"journal": {Name: "手账涂鸦", Bg: "#fdfbf7", Accent1: "#f77f00", Accent2: "#003049", Template: "knowledge-paper"},
	"vapor":   {Name: "蒸汽波", Bg: "#241734", Accent1: "#ff71ce", Accent2: "#01cdfe", Template: "knowledge-dark"},
	"clay":    {Name: "黏土卡哇伊", Bg: "#fef6e4", Accent1: "#f582ae", Accent2: "#8bd3dd", Template: "knowledge-paper"},
	// 电影感家族
	"titles":         {Name: "剪影片头", Bg: "#050505", Accent1: "#e50914", Accent2: "#ffffff", Template: "knowledge-dark"},
	"silent":         {Name: "默片字卡", Bg: "#0a0a0a", Accent1: "#e0d8b0", Accent2: "#8c8266", Template: "knowledge-dark"},
	"constructivist": {Name: "构成主义", Bg: "#e8e2d5", Accent1: "#d62828", Accent2: "#003049", Template: "knowledge-paper"},
}

// KnowledgeStyle 风格元数据。
type KnowledgeStyle struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Bg       string `json:"bg"`
	Accent1  string `json:"accent1"`
	Accent2  string `json:"accent2"`
	Template string `json:"template"`
}

// KnowledgeStyles 44 套风格元数据。
var KnowledgeStyles = func() []KnowledgeStyle {
	out := make([]KnowledgeStyle, 0, len(StylePalettes))
	for k, p := range StylePalettes {
		out = append(out, KnowledgeStyle{
			Key:      k,
			Name:     p.Name,
			Bg:       p.Bg,
			Accent1:  p.Accent1,
			Accent2:  p.Accent2,
			Template: p.Template,
		})
	}
	return out
}()

// GetStyle 根据风格 key 获取配色与推荐模板。
func GetStyle(key string) *StylePalette {
	if key == "" {
		return nil
	}
	k := toLowerTrim(key)
	p, ok := StylePalettes[k]
	if !ok {
		return nil
	}
	return &p
}

func toLowerTrim(s string) string {
	// 简单的小写+去空格，足够当前用法
	var b []byte
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		if c != ' ' {
			b = append(b, byte(c))
		}
	}
	return string(b)
}
