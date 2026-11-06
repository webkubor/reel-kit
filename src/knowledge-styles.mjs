/**
 * 44 套知识短视频视觉风格库（吸收自 vibe-knowledge）
 * 涵盖 8 个风格家族：东方、暗场概念、游戏、剪辑与纪实、明亮流行、图解与设计、印刷与纸、电影感
 */

// 常用风格的具体配色方案（底色、主强调、辅强调）
export const STYLE_PALETTES = {
  // 东方家族
  gufeng:       { bg: '#140d0b', accent1: '#d9472b', accent2: '#e2b45a', template: 'knowledge-gufeng' }, // 古风（夜赭底、朱红、金）
  paper:        { bg: '#ece4d4', accent1: '#b23a2e', accent2: '#3c6e71', template: 'knowledge-paper' },  // 水墨纸本（米黄、茜红、墨青）
  dunhuang:     { bg: '#2b1d16', accent1: '#c85a32', accent2: '#2a7b72', template: 'knowledge-gufeng' }, // 敦煌（深褐底、矿红、石绿）
  qinghua:      { bg: '#f4f6fa', accent1: '#1b3b82', accent2: '#4a6fa5', template: 'knowledge-paper' },  // 青花（白瓷、青花蓝）
  woodcut:      { bg: '#f0ebd8', accent1: '#1c1b18', accent2: '#9c2a2a', template: 'knowledge-paper' },  // 木刻（版画纸、浓墨、印泥红）
  minimal:      { bg: '#ffffff', accent1: '#111111', accent2: '#888888', template: 'knowledge-paper' },  // 留白极简（纯白、纯黑、灰）
  scroll:       { bg: '#16100c', accent1: '#e0a94e', accent2: '#c24830', template: 'knowledge-gufeng' }, // 长卷（深墨、金丝、朱印）

  // 暗场概念家族
  starfield:    { bg: '#05070d', accent1: '#f2c46d', accent2: '#7fb8ff', template: 'knowledge-dark' },   // 星空光点（深空蓝、星芒金、天青）
  darkgold:     { bg: '#0b0a08', accent1: '#f0b44c', accent2: '#ff6a3d', template: 'knowledge-dark' },   // 暗金线描（近黑、纯金、琥珀）
  darkmath:     { bg: '#0f1014', accent1: '#58c4dd', accent2: '#f4d35e', template: 'knowledge-dark' },   // 深色数学（3B1B 极黑、数轴蓝、结果黄）
  neon:         { bg: '#08080f', accent1: '#00ffcc', accent2: '#ff007f', template: 'knowledge-dark' },   // 霓虹赛博（午夜黑、荧光青、霓虹洋红）
  keynote:      { bg: '#000000', accent1: '#0071e3', accent2: '#86868b', template: 'knowledge-dark' },   // 发布会极简（Apple 纯黑、科技蓝、钛灰）
  aurora:       { bg: '#070b12', accent1: '#4ef2bb', accent2: '#9d4edd', template: 'knowledge-dark' },   // 极光（深蓝黑、极光绿、夜光紫）
  deco:         { bg: '#0d131a', accent1: '#dfba73', accent2: '#2a5a52', template: 'knowledge-dark' },   // 装饰艺术金线（藏青、奢华金、孔雀绿）

  // 图解与设计家族
  chalk:        { bg: '#1c2421', accent1: '#f4f1de', accent2: '#e07a5f', template: 'knowledge-dark' },   // 粉笔黑板（墨绿黑板、粉笔白、柔橙）
  blueprint:    { bg: '#0b2545', accent1: '#8da9c4', accent2: '#eef4f8', template: 'knowledge-dark' },   // 蓝图专利（工程蓝、线条浅蓝、白色标注）
  isometric:    { bg: '#1a1c23', accent1: '#3a86ff', accent2: '#ffbe0b', template: 'knowledge-dark' },   // 等轴测剖面（深冷灰、机械蓝、警示黄）
  swiss:        { bg: '#f2f2f2', accent1: '#e63946', accent2: '#1d3557', template: 'knowledge-paper' },  // 瑞士网格（冷白、国际红、深蓝黑）
  metro:        { bg: '#ffffff', accent1: '#e31b23', accent2: '#0072ce', template: 'knowledge-paper' },  // 地铁线路图（白底、线路红、枢纽蓝）
  isotype:      { bg: '#faf7f2', accent1: '#264653', accent2: '#e76f51', template: 'knowledge-paper' },  // 象形统计（复古灰白、统计墨绿、对比橙红）
  sketch:       { bg: '#f5f0e6', accent1: '#2b2b2b', accent2: '#4a6fa5', template: 'knowledge-paper' },  // 手绘过程（手账纸本、铅笔灰、彩铅）

  // 印刷与纸家族
  riso:         { bg: '#f7f4ee', accent1: '#ff4848', accent2: '#2b59c3', template: 'knowledge-paper' },  // 孔版印刷（米色纸本、荧光粉红、大豆油墨蓝）
  collage:      { bg: '#dfd7c5', accent1: '#a62b2b', accent2: '#233d4d', template: 'knowledge-paper' },  // 半调拼贴（粗粝报纸灰、复古深红、深蓝炭）
  ransom:       { bg: '#1f1e1c', accent1: '#f4d06f', accent2: '#e74c3c', template: 'knowledge-dark' },   // 剪报拼字（暗黑底、剪字黄、警示红）
  editorial:    { bg: '#f8f8f9', accent1: '#111827', accent2: '#4f46e5', template: 'knowledge-paper' },  // 杂志编辑（冷灰白、精排黑、靛蓝）
  poster:       { bg: '#2d3748', accent1: '#f6ad55', accent2: '#68d391', template: 'knowledge-dark' },   // 国家公园海报（自然暗岩灰、落日橙、林木绿）

  // 剪辑与纪实家族
  'caption-news':{ bg: '#0d1117', accent1: '#ff3344', accent2: '#ffffff', template: 'knowledge-dark' },  // 字幕新闻（演播厅暗场、突发红、白字）
  investigation:{ bg: '#121418', accent1: '#ffcc00', accent2: '#00ccff', template: 'knowledge-dark' },   // 视觉调查（档案暗房、荧光笔黄、标记青）
  desktop:      { bg: '#1e1e2e', accent1: '#f38ba8', accent2: '#89b4fa', template: 'knowledge-dark' },   // 复古桌面（终端暗底、高亮粉、代码蓝）
  lighttable:   { bg: '#212529', accent1: '#4dabf7', accent2: '#ffd43b', template: 'knowledge-dark' },   // 档案灯台（胶片观片台暗灰、幻灯冷蓝、高亮黄）
  tvnews:       { bg: '#0a192f', accent1: '#ff0033', accent2: '#64ffda', template: 'knowledge-dark' },   // 电视包装（新闻深蓝、醒目红、青色角标）

  // 游戏家族
  pixel:        { bg: '#10122a', accent1: '#ffd23f', accent2: '#3fe0a0', template: 'knowledge-dark' },   // 经典像素（复古夜空紫、金币黄、HP绿）
  rpg:          { bg: '#0b0e14', accent1: '#e2b45a', accent2: '#4a90e2', template: 'knowledge-dark' },   // RPG 对话框（暗金边框黑底、金色对话、法力蓝）
  cards:        { bg: '#1a1829', accent1: '#c084fc', accent2: '#fbbf24', template: 'knowledge-dark' },   // 卡牌对战（魔幻紫黑、史诗紫、稀有金）
  farm:         { bg: '#2b3a2f', accent1: '#f6d89b', accent2: '#7ebc89', template: 'knowledge-dark' },   // 田园像素（森林绿、麦田暖黄、生机嫩绿）

  // 明亮流行家族
  variety:      { bg: '#fffbe6', accent1: '#ff4d4f', accent2: '#faad14', template: 'knowledge-paper' },  // 综艺花字（奶油黄、爆点红、弹幕橙）
  memphis:      { bg: '#fff0f5', accent1: '#ff1493', accent2: '#00ffff', template: 'knowledge-paper' },  // 孟菲斯波普（淡粉、亮洋红、电光青）
  journal:      { bg: '#fdfbf7', accent1: '#f77f00', accent2: '#003049', template: 'knowledge-paper' },  // 手账涂鸦（牛皮纸、水笔橙、墨水深蓝）
  vapor:        { bg: '#241734', accent1: '#ff71ce', accent2: '#01cdfe', template: 'knowledge-dark' },   // 蒸汽波（梦幻紫黑、蒸汽粉、赛博天青）
  clay:         { bg: '#fef6e4', accent1: '#f582ae', accent2: '#8bd3dd', template: 'knowledge-paper' },  // 黏土卡哇伊（温暖马卡龙暖底、粉红、浅青）

  // 电影感家族
  titles:       { bg: '#050505', accent1: '#e50914', accent2: '#ffffff', template: 'knowledge-dark' },   // 剪影片头（影院全黑、片头猩红、亮白）
  silent:       { bg: '#0a0a0a', accent1: '#e0d8b0', accent2: '#8c8266', template: 'knowledge-dark' },   // 默片字卡（百年胶片炭黑、老胶卷米白、边框灰金）
  constructivist:{ bg: '#e8e2d5', accent1: '#d62828', accent2: '#003049', template: 'knowledge-paper' }, // 构成主义（红军工装米灰、革命红、钢铁黑蓝）
};

/** 44 套风格元数据 */
export const KNOWLEDGE_STYLES = Object.keys(STYLE_PALETTES).map(key => ({
  key,
  ...STYLE_PALETTES[key],
}));

/** 根据风格 key 获取配色与推荐模板 */
export function getStyle(key) {
  if (!key) return null;
  const k = String(key).toLowerCase().trim();
  return STYLE_PALETTES[k] || null;
}
