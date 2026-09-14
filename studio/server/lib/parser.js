/**
 * 通用 markdown 解析器 —— 把散落各处的真源文件解析成可消费的结构。
 *
 * 设计原则：
 *   1. graceful：文件不存在 / 格式异常都不抛错，降级返回空。
 *   2. 不做归一化：保留原始 kind 标记，让 UI 按 kind 决定渲染策略。
 *   3. 零依赖：只用 node: 内置模块。
 *
 * 支持的真源（按需读，不一次性全 load）：
 *   - episodes/README.md        剧集索引表
 *   - episodes/epXX.md         14 集剧本（5 种 kind 之一）
 *   - scripts/storyboards/*.md 单镜头 storyboard（15s 5 段式）
 *   - agents/CREATIVE_BIBLE.md 创作法典（章节化）
 *   - bibles/VOICEOVER.md      旁白文案（角色评卷版）
 *   - music/bgm.md             BGM 库（8 段 + 4 段已定曲名）
 *   - music/main.md            主题曲「雪沸」
 *   - skills/镜头美学/SKILL.md 镜头审美偏好收集器
 */

import { readFileSync, readdirSync, statSync } from 'node:fs';
import { basename, join, relative } from 'node:path';
import { DEFAULT_SEASON, getSeasonRoot, PROJECT_ROOT } from './paths.js';
import { invalidateRegistry } from '../registry.js';

// 当前季：单线程 dev-only 服务用 module state 没并发问题
// 改季方式：setSeason('s2')，或 routes 入口从 query string 调
let _season = DEFAULT_SEASON;
export function setSeason(season) {
  const next = season || DEFAULT_SEASON;
  if (next !== _season) {
    _season = next;
    // registry 缓存里也是旧季的 cast/refs，季切了得清掉
    invalidateRegistry();
  }
}
export function getSeason() { return _season; }

// 季特定路径：每季各一份
const epDir = () => join(getSeasonRoot(_season), 'episodes');
const storyDir = () => join(getSeasonRoot(_season), 'scripts', 'storyboards');
const biblesDir = () => join(getSeasonRoot(_season), 'bibles');
const musicDir = () => join(getSeasonRoot(_season), 'music');

// 跨季共享路径：留在仓库根
const agentsDir = () => join(PROJECT_ROOT, 'agents');
const skillsDir = () => join(PROJECT_ROOT, 'skills');

// ─── 工具 ──────────────────────────────────────────────────────────

function readSafe(p, def = '') {
  try { return readFileSync(p, 'utf8'); } catch { return def; }
}
function exists(p) {
  try { statSync(p); return true; } catch { return false; }
}

/** 提取引用链接：`[label](path)` */
function extractRefs(md) {
  const re = /\[([^\]]+)\]\(([^)]+)\)/g;
  const out = [];
  let m;
  while ((m = re.exec(md))) out.push({ label: m[1], href: m[2] });
  return out;
}

/** 提取一级标题 */
function extractTitle(md) {
  const m = md.match(/^#\s+(.+?)$/m);
  return m ? m[1].trim() : null;
}

/**
 * 提取 "## 标题\n\n内容" 直到下一个 ## 或文件尾
 * 容忍 "## N. 标题" 这种编号前缀
 *
 * 用 \Z 而不是 $，因为 m flag 下 $ 是行末（lookahead 会在第一行末满足），
 * \Z 才是字符串末尾。
 */
function extractSection(md, title) {
  const re = new RegExp(`^##\\s+(?:\\d+\\.\\s+)?${escapeRe(title)}\\s*\\n+([\\s\\S]*?)(?=\\n## |\\Z)`, 'm');
  const m = md.match(re);
  return m ? m[1].trim() : null;
}

/** 把小节切成长 KV 对（`- **key**: val`），key 归一化 */
function extractKvs(section) {
  if (!section) return [];
  const out = [];
  const re = /^- \*\*(.+?)\*\*\s*[:：]\s*(.+?)$/gm;
  let m;
  while ((m = re.exec(section))) {
    out.push({ key: m[1].trim(), value: m[2].trim() });
  }
  return out;
}

function escapeRe(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// ─── 剧集 ──────────────────────────────────────────────────────────

const STATUS_MAP = {
  '已定稿': 'finalized',
  '已发布': 'published',
  '已完结': 'completed',
  '待开发': 'pending',
};

const ZH_EP = {
  '零': 0, '一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
  '六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
  '十一': 11, '十二': 12, '十三': 13, '十四': 14, '十五': 15,
  '十六': 16, '十七': 17, '十八': 18, '十九': 19, '二十': 20,
};

/** "第一集" / "第0集" / "第十一集" 形式 → 集号 */
function parseEpNumber(s) {
  // 优先阿拉伯数字
  const m1 = s.match(/第(\d+)集/);
  if (m1) return parseInt(m1[1], 10);
  // 中文数字
  const m2 = s.match(/第([零一二三四五六七八九十]+)集/);
  if (m2) return ZH_EP[m2[1]] ?? NaN;
  return NaN;
}

const STATUS_LABEL = {
  finalized: '已定稿',
  published: '已发布',
  completed: '已完结',
  pending: '待开发',
};

/** 解析 episodes/README.md 的索引表 */
function parseEpisodeIndex() {
  const md = readSafe(join(epDir(), 'README.md'));
  const out = {};
  if (!md) return out;

  for (const line of md.split('\n')) {
    if (!line.startsWith('|')) continue;
    const cells = line.split('|').map((c) => c.trim()).filter(Boolean);
    if (cells.length < 4) continue;
    const ep = parseEpNumber(cells[0]);
    if (Number.isNaN(ep)) continue;
    const title = cells[1].replace(/^[《「『]|》」』]$/g, '').trim();
    const core = cells[2] || '';
    let status = 'pending';
    for (const [zh, en] of Object.entries(STATUS_MAP)) {
      if (cells[3].includes(zh)) { status = en; break; }
    }
    out[ep] = { title, core, status, statusLabel: STATUS_LABEL[status] };
  }
  return out;
}

/** 识别单集 kind */
function detectKind(md, ep) {
  if (ep === 0) return 'prologue';
  // 详细场景分镜：ep12 风格，"## 场景1：xxx"
  if (/^## 场景\d+[：:]/m.test(md)) return 'detailed-scenes';
  // 15s 结构化：ep01 风格，"### 1. xxx" 或 "## 第一幕"
  if (/^## 第[一二三四五六七八九十]幕/m.test(md) || /^### \d+\.\s/m.test(md)) return 'structured-15s';
  // storyboard 单镜头：5 段式 "0-3秒|起势"
  if (/^\d+-\d+秒\|/m.test(md)) return 'storyboard';
  // 默认：剧情大纲
  return 'outline';
}

/** 序幕（ep00）：3-4 段叙事，无明确分镜 —— 用 ### 而不是 ## */
function parsePrologue(md) {
  const parts = extractSectionsByH3(md).filter((s) => s.title.match(/第[一二三四]段/));
  return parts.map((p) => ({ title: p.title, body: p.body }));
}

/** "## 第一幕" 风格（ep01） */
function parseActStructure(md) {
  const acts = [];
  const re = /^## (第[一二三四五六七八九十百]+幕[：:]?.+?)$/gm;
  let m;
  while ((m = re.exec(md))) {
    const title = m[1];
    const start = m.index + m[0].length;
    const next = re.exec(md);
    const end = next ? next.index : md.length;
    re.lastIndex = start; // 退回
    const body = md.slice(start, end).trim();
    // 提取子项 "### N. 标题"
    const shots = [];
    const shotRe = /^### (\d+)\.\s*(.+?)$/gm;
    let sm;
    while ((sm = shotRe.exec(body))) {
      shots.push({ index: parseInt(sm[1], 10), title: sm[2].trim() });
    }
    acts.push({ title, body, shots });
  }
  return acts.length ? acts : null;
}

/** "## 场景N" 风格（ep12） */
function parseSceneStructure(md) {
  const scenes = [];
  const re = /^## 场景(\d+)[：:]\s*(.+?)$/gm;
  let m;
  while ((m = re.exec(md))) {
    const start = m.index + m[0].length;
    re.lastIndex = start;
    const next = re.exec(md);
    const end = next ? next.index : md.length;
    re.lastIndex = start;
    const body = md.slice(start, end).trim();
    scenes.push({
      index: parseInt(m[1], 10),
      title: m[2].trim(),
      body,
      time: extractMetaField(body, '时间'),
      location: extractMetaField(body, '地点'),
      characters: extractMetaField(body, '人物'),
    });
  }
  return scenes.length ? scenes : null;
}

/** storyboard 5 段式 */
function parseStoryboardShots(md) {
  const re = /^(\d+)-(\d+)秒\|(.+?)$/gm;
  const out = [];
  let m;
  while ((m = re.exec(md))) {
    out.push({ from: parseInt(m[1], 10), to: parseInt(m[2], 10), title: m[3].trim() });
  }
  return out;
}

/** 提取元字段：兼容两种格式
 *   1. `- **时间**: xxx`（ep12 早期 / 标准 KV 风格）
 *   2. `### 时间\n- xxx`（ep12 详细场景风格）
 */
function extractMetaField(body, key) {
  // 格式 1：粗体 KV
  const re1 = new RegExp(`^- \\*\\*${escapeRe(key)}\\*\\*[：:]\\s*(.+?)$`, 'm');
  const m1 = body.match(re1);
  if (m1) return m1[1].trim();
  // 格式 2：三级标题 + 列表项
  const re2 = new RegExp(`^###\\s*${escapeRe(key)}\\s*\\n+-\\s*(.+?)$`, 'm');
  const m2 = body.match(re2);
  if (m2) return m2[1].trim();
  return null;
}

/** 提取所有 `## 标题` 段 */
function extractSectionsByH2(md) {
  return extractSectionsByLevel(md, 2);
}

/** 提取所有 `### 标题` 段（用于序幕等） */
function extractSectionsByH3(md) {
  return extractSectionsByLevel(md, 3);
}

function extractSectionsByLevel(md, level) {
  const out = [];
  const prefix = '#'.repeat(level) + ' ';
  const re = new RegExp(`^${escapeRe(prefix)}(.+?)$`, 'gm');
  let m;
  const positions = [];
  while ((m = re.exec(md))) positions.push({ title: m[1], index: m.index + m[0].length });
  for (let i = 0; i < positions.length; i++) {
    const start = positions[i].index;
    const end = positions[i + 1] ? positions[i + 1].index - positions[i].title.length - prefix.length - 1 : md.length;
    out.push({ title: positions[i].title, body: md.slice(start, end).trim() });
  }
  return out;
}

/** 提取 Jimeng 指令（`**Jimeng 指令**：` 后的反引号块） */
function extractJimengPrompts(md) {
  // 容忍 **加粗** 标记、空格、全角/半角冒号
  const re = /\*?\*?Jimeng\s*指令\*?\*?\s*[：:]\s*`([^`]+)`/g;
  const out = [];
  let m;
  while ((m = re.exec(md))) out.push(m[1].trim());
  return out;
}

/** 提取台词（`**台词**：(XXX) "xxx"`） */
function extractVoiceover(md) {
  // 容忍 **加粗**、全角/半角括号、左右引号
  const re = /\*?\*?台词\*?\*?\s*[：:]\s*[（(]([^）)]+)[）)]\s*["“”']([^"“”']+)["“”']/g;
  const out = [];
  let m;
  while ((m = re.exec(md))) out.push({ tone: m[1].trim(), text: m[2].trim() });
  return out;
}

/** 提取音频描述（`**音频**：xxx`） */
function extractAudioCues(md) {
  const re = /\*?\*?音频\*?\*?\s*[：:]\s*(.+?)$/gm;
  const out = [];
  let m;
  while ((m = re.exec(md))) out.push(m[1].trim());
  return out;
}

/** 解析单集 */
function parseEpisode(md, ep, slug) {
  const kind = detectKind(md, ep);
  const title = extractTitle(md);
  const core = extractSection(md, '本集核心')
    || extractSection(md, '本集唯一任务')
    || extractSection(md, '本集作用')
    || null;

  const acts = kind === 'structured-15s' ? parseActStructure(md) : null;
  const scenes = kind === 'detailed-scenes' ? parseSceneStructure(md) : null;
  const prologueParts = kind === 'prologue' ? parsePrologue(md) : null;
  const storyboardShots = kind === 'storyboard' ? parseStoryboardShots(md) : null;
  const jimeng = extractJimengPrompts(md);
  const voiceover = extractVoiceover(md);
  const audio = extractAudioCues(md);
  const refs = extractRefs(md);

  return {
    ep,
    slug,
    kind,
    title,
    core,
    acts,
    scenes,
    prologueParts,
    storyboardShots,
    jimeng,
    voiceover,
    audio,
    refs,
    charCount: md.length,
    updatedAt: safeMtime(join(epDir(), slug)),
  };
}

function safeMtime(p) {
  try { return statSync(p).mtimeMs; } catch { return null; }
}

/** 解析全部剧集 */
export function parseAllEpisodes() {
  const index = parseEpisodeIndex();
  const eps = [];
  for (let i = 0; i <= 13; i++) {
    const slug = i === 0 ? 'ep00.md' : `ep${String(i).padStart(2, '0')}.md`;
    const fp = join(epDir(), slug);
    if (!exists(fp)) {
      eps.push({
        ep: i,
        slug,
        kind: 'pending',
        exists: false,
        ...(index[i] || { title: `第${i}集`, status: 'pending', statusLabel: '待开发', core: '' }),
      });
      continue;
    }
    const md = readSafe(fp);
    const parsed = parseEpisode(md, i, slug);
    // 没在 README 索引里的剧集默认 pending
    const meta = index[i] || { status: 'pending', statusLabel: STATUS_LABEL.pending, title: parsed.title || `第${i}集`, core: '' };
    eps.push({ ...parsed, ...meta, exists: true });
  }
  return eps;
}

/** 解析所有 storyboard（scripts/storyboards/*.md）
 *
 * 提取：天榜排名 / 角色 / 视觉逻辑 / 动态节奏 / 环境背景 / 视觉符号
 */
export function parseStoryboards() {
  if (!exists(storyDir())) return [];
  const out = [];
  for (const file of readdirSync(storyDir())) {
    if (!file.endsWith('.md')) continue;
    const md = readSafe(join(storyDir(), file));
    const title = extractTitle(md);
    const meta = extractCaseMeta(md);
    const shots = parseStoryboardShots(md);
    const jimeng = extractJimengPrompts(md);
    const refs = extractRefs(md);
    out.push({
      file,
      rel: relative(PROJECT_ROOT, join(storyDir(), file)),
      title,
      ...meta,
      shots,
      jimeng,
      refs,
      charCount: md.length,
      updatedAt: safeMtime(join(storyDir(), file)),
    });
  }
  return out;
}

/** 从 case md 提取天榜排名 / 角色 / 视觉逻辑 / 节奏 / 环境 / 视觉符号
 *
 * 兼容多种 key 写法：标准(视觉逻辑/动态节奏/环境背景)和变体(视觉重心/气场设定/场景配置)
 */
function extractCaseMeta(md) {
  const m = md.match(/《沸腾之雪》([^（]+)/);
  const headerText = m ? m[1].trim() : '';

  const rankMatch = headerText.match(/天榜第([\d零一二三四五六七八九十]+)/);
  const rank = rankMatch ? rankMatch[1] : null;

  let character = null;
  if (headerText.includes('·')) {
    const last = headerText.split('·').pop().trim();
    character = last.replace(/\s*[（(].*$/, '').trim() || null;
  }

  // 兼容多种 key
  const visualLogic =
    extractBulletField(md, '视觉逻辑') ||
    extractBulletField(md, '视觉重心') ||
    extractBulletField(md, '视觉') ||
    null;

  const rhythm =
    extractBulletField(md, '动态节奏') ||
    extractBulletField(md, '节奏') ||
    extractBulletField(md, '气场设定') ||
    null;

  const env =
    extractBulletField(md, '环境背景') ||
    extractBulletField(md, '环境') ||
    extractBulletField(md, '场景配置') ||
    null;

  // 武器（额外字段）
  const weapon =
    extractBulletField(md, '武器系统') ||
    extractBulletField(md, '神兵') ||
    null;

  // 元素（顾栖月 style 的"飞瀑、云气、青藤"）
  const element =
    extractBulletField(md, '元素') ||
    null;

  // 地点（顾栖月 style 的"文成百丈漈"）
  const location =
    extractBulletField(md, '地点') ||
    null;

  let visualSymbol = null;
  const sym1 = md.match(/定格[于在]?\s*\*?\*?【(.+?)】/);
  if (sym1) visualSymbol = sym1[1];
  if (!visualSymbol) {
    const sym2 = md.match(/画面定格[在于]?\s*\*?\*?【(.+?)】/);
    if (sym2) visualSymbol = sym2[1];
  }

  return {
    rank,
    rankLabel: rank ? `天榜第${rank}` : (headerText.includes('序章') ? '序章' : null),
    character,
    isPrologue: headerText.includes('序章'),
    visualLogic,
    rhythm,
    env,
    weapon,
    element,
    location,
    visualSymbol,
  };
}

/** 从 md 提取 "**key**: value" 形式的 bullet 字段 */
function extractBulletField(md, key) {
  const re = new RegExp(`-\\s*\\*\\*${escapeRe(key)}\\*\\*[：:]\\s*(.+)`, 'm');
  const m = md.match(re);
  return m ? m[1].trim().replace(/\*\*?/g, '') : null;
}

// ─── 创意法典 + 旁白 + 镜头美学 ─────────────────────────────────

const BIBLE_SECTIONS = [
  { key: 'global',    title: '全局默认配置 (Global Defaults)' },
  { key: 'toolkit',   title: '创作工具包 (Creation Toolkit)' },
  { key: 'redlines',  title: '视觉美学与禁令 (Visual & Red Lines)' },
  { key: 'cinema',    title: '镜头语言与运镜 (Cinematography)' },
  { key: 'action',    title: '武学动作逻辑 (Action Physics)' },
  { key: 'weapons',   title: '神兵镜头专属逻辑' },
  { key: 'character', title: '角色制作与环境音频' },
];

/** 解析 CREATIVE_BIBLE.md 为结构化 sections */
export function parseCreativeBible() {
  const md = readSafe(join(agentsDir(), 'CREATIVE_BIBLE.md'));
  if (!md) return null;
  const sections = BIBLE_SECTIONS.map((s) => ({
    ...s,
    body: extractSection(md, s.title),
  }));
  return {
    title: extractTitle(md),
    sections,
    refs: extractRefs(md),
  };
}

/** 解析 VOICEOVER.md 角色评卷版文案 */
export function parseVoiceover() {
  const md = readSafe(join(biblesDir(), 'VOICEOVER.md'));
  if (!md) return null;
  const blocks = [];
  // 匹配 "《沸腾之雪》[角色] 第三人称【江湖评卷版】文案"
  const re = /《沸腾之雪》(.+?) 第三人称【江湖评卷版】文案[（(]?(\d+s)?[）)]?/g;
  let m;
  const positions = [];
  while ((m = re.exec(md))) {
    positions.push({
      character: m[1].trim(),
      duration: m[2] || '30s',
      index: m.index + m[0].length,
    });
  }
  for (let i = 0; i < positions.length; i++) {
    const end = positions[i + 1] ? positions[i + 1].index - 30 : md.length;
    const body = md.slice(positions[i].index, end).trim();
    const sloganMatch = body.match(/\n\n(.+?)$/m);
    blocks.push({
      ...positions[i],
      body,
      slogan: sloganMatch ? sloganMatch[1].trim() : null,
    });
  }
  return { blocks };
}

/** 解析 SKILL 镜头美学 */
export function parseCameraSkill() {
  const md = readSafe(join(skillsDir(), '镜头美学', 'SKILL.md'));
  if (!md) return null;
  return {
    title: extractTitle(md),
    intro: extractSection(md, '核心定位') || extractSection(md, '交互流程'),
    flow: extractSection(md, '交互流程'),
    output: extractSection(md, '输出格式'),
    mapping: extractSection(md, '偏好→术语映射表'),
    quick: extractSection(md, '快速推荐'),
    examples: extractSection(md, '使用示例'),
  };
}

// ─── 音乐 + 影调 ────────────────────────────────────────────────

/** 解析 music/bgm.md 8 段 BGM + 4 段已定曲名 */
export function parseBgm() {
  const md = readSafe(join(musicDir(), 'bgm.md'));
  if (!md) return null;
  const blocks = [];
  // 匹配 "## N. 《曲名》 (场景)"
  const re = /^## (\d+)\.\s*《(.+?)》\s*[（(]?(.+?)[）)]?\s*(\[?NEW\]?)?$/gm;
  let m;
  const positions = [];
  while ((m = re.exec(md))) {
    positions.push({
      index: parseInt(m[1], 10),
      title: m[2].trim(),
      tag: (m[3] || '').trim(),
      isNew: !!m[4],
      bodyStart: m.index + m[0].length,
    });
  }
  for (let i = 0; i < positions.length; i++) {
    const end = positions[i + 1] ? positions[i + 1].bodyStart - 10 : md.length;
    const body = md.slice(positions[i].bodyStart, end).trim();
    const kvs = extractKvs(body);
    blocks.push({ ...positions[i], body, kvs });
  }

  // 已定曲名存档
  const archiveStart = md.indexOf('## 8. 已定曲名存档');
  let archive = [];
  if (archiveStart >= 0) {
    const archiveMd = md.slice(archiveStart);
    const subRe = /^### (\d+\.\d+)\s*《(.+?)》$/gm;
    let sm;
    while ((sm = subRe.exec(archiveMd))) {
      const titleStart = sm.index + sm[0].length;
      const next = subRe.exec(archiveMd);
      const titleEnd = next ? next.index : archiveMd.length;
      subRe.lastIndex = titleStart;
      archive.push({
        index: sm[1],
        title: sm[2].trim(),
        body: archiveMd.slice(titleStart, titleEnd).trim(),
        kvs: extractKvs(archiveMd.slice(titleStart, titleEnd)),
      });
    }
  }
  return { blocks, archive };
}

/** 解析 music/main.md 主题曲「雪沸」 */
export function parseMainTheme() {
  const md = readSafe(join(musicDir(), 'main.md'));
  if (!md) return null;
  const styleMatch = md.match(/```text\n([\s\S]+?)\n```/);
  const style = styleMatch ? styleMatch[1].trim() : null;
  const lyricsMatch = md.match(/```text\n([\s\S]+?)\n```/g);
  // 第二个 text 代码块通常是歌词
  const lyrics = lyricsMatch && lyricsMatch[1]
    ? lyricsMatch[1].replace(/^```text\n|\n```$/g, '')
    : null;
  return {
    title: extractTitle(md),
    style,
    lyrics,
    sections: extractSectionsByH2(md),
  };
}

/** 解析 CREATIVE_BIBLE 中的影调协议段（用于 AestheticView） */
export function parseAesthetic() {
  const md = readSafe(join(agentsDir(), 'CREATIVE_BIBLE.md'));
  if (!md) return null;
  return {
    global: extractSection(md, '全局默认配置'),
    redlines: extractSection(md, '视觉美学与禁令'),
    cinema: extractSection(md, '镜头语言与运镜'),
    action: extractSection(md, '武学动作逻辑'),
    audio: extractSection(md, '角色制作与环境音频'),
  };
}

// ─── 神兵 ─────────────────────────────────────────────────────

/** 解析 bibles/WEAPONS.md 为结构化神兵 */
export function parseWeapons() {
  const md = readSafe(join(biblesDir(), 'WEAPONS.md'));
  if (!md) return [];
  const weapons = [];
  const re = /^## (\d+)\.\s*[【「](.+?)[】」]\s*[（(]?(.+?)?[）)]?$/gm;
  let m;
  const positions = [];
  while ((m = re.exec(md))) {
    positions.push({
      index: parseInt(m[1], 10),
      name: m[2].trim(),
      holder: (m[3] || '').replace(/[（(]持有者[：:]\s*|持有者[：:]\s*/g, '').replace(/[）)]/g, '').trim() || null,
      bodyStart: m.index + m[0].length,
    });
  }
  for (let i = 0; i < positions.length; i++) {
    const end = positions[i + 1] ? positions[i + 1].bodyStart - 10 : md.length;
    const body = md.slice(positions[i].bodyStart, end).trim();
    weapons.push({
      ...positions[i],
      body,
      kvs: extractKvs(body),
    });
  }
  return weapons;
}
