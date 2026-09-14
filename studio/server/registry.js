import { readdirSync, readFileSync, statSync } from 'node:fs';
import { basename, extname, join, relative } from 'node:path';
import { getSeasonRoot, PROJECT_ROOT, STUDIO_ROOT } from './lib/paths.js';

/**
 * 资产索引层 —— 把仓库里四套并存的命名映射到一起。
 *
 *   cast/顾栖月.json          内含 "ID": "gu-qi-yue"
 *   references/三视图/顾栖月_v1_玉笛.png   中文名 + 版本后缀
 *   _dashboard/assets/characters/gu_qi_yue.png   下划线
 *   scripts/gen_image.py（已废弃）        gu_qiyue   ← 第四种
 *
 * 索引在运行时构建，**不移动也不重命名任何现有文件**。移目录会炸掉
 * storyboard 里的 @引用、cast 的人物关系、以及一堆硬编码路径。
 */

const IMAGE_EXT = new Set(['.png', '.jpg', '.jpeg', '.webp', '.gif', '.svg']);
const VIDEO_EXT = new Set(['.mp4', '.mov', '.webm']);
/* 字体不进资产索引 —— 它是界面的一部分，不是创作素材 */

/** cast JSON 里可能藏着别名的字段 —— 从真源提取，不手工维护 */
const ALIAS_FIELDS = ['姓名', '曾用名', '花名', '真实姓名', '别名', '绰号'];

let cache = null;

function readJson(path) {
  try {
    return JSON.parse(readFileSync(path, 'utf8'));
  } catch (err) {
    console.warn(`[registry] 跳过无法解析的 JSON: ${path} — ${err.message}`);
    return null;
  }
}

/**
 * 递归列文件。
 *
 * `subdir` 记录相对扫描根的子目录，因为归属信息有时只在目录名里：
 * `references/海报/燕照绫/1.png` 的文件名是「1」，认不出人，得靠父目录。
 */
function walk(dir, out = [], root = dir) {
  let entries;
  try {
    entries = readdirSync(dir, { withFileTypes: true });
  } catch {
    return out; // 目录不存在就当空的，别让整个索引挂掉
  }
  for (const entry of entries) {
    if (entry.name.startsWith('.')) continue;
    const abs = join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(abs, out, root);
    } else {
      const ext = extname(entry.name).toLowerCase();
      if (!IMAGE_EXT.has(ext) && !VIDEO_EXT.has(ext)) continue;
      const st = statSync(abs);
      out.push({
        rel: relative(PROJECT_ROOT, abs),
        name: basename(entry.name, ext),
        subdir: relative(root, dir),
        ext,
        kind: VIDEO_EXT.has(ext) ? 'video' : 'image',
        size: st.size,
        mtime: st.mtimeMs,
      });
    }
  }
  return out;
}

/** 一个角色的全部叫法。含 ID 的下划线/紧凑变体，好接住历史脚本的拼法。 */
function collectAliases(char) {
  const set = new Set();
  const add = (v) => {
    if (typeof v !== 'string') return;
    // 「惊棠 / 小楠」这种一格里塞两个花名的，拆开
    for (const part of v.split(/[\/、,，]/)) {
      const t = part.trim();
      if (t.length >= 2) set.add(t);
    }
  };

  for (const field of ALIAS_FIELDS) add(char.raw?.[field]);
  add(char.name);

  if (char.id) {
    set.add(char.id);
    set.add(char.id.replace(/-/g, '_'));
    set.add(char.id.replace(/-/g, ''));
  }
  return [...set];
}

function loadManualAliases() {
  const data = readJson(join(STUDIO_ROOT, 'server', 'aliases.json')) ?? {};
  return Object.entries(data).filter(([k]) => !k.startsWith('_'));
}

function buildCharacters() {
  const castDir = join(getSeasonRoot(), 'cast');
  const characters = [];

  for (const file of readdirSync(castDir)) {
    if (!file.endsWith('.json')) continue;
    const raw = readJson(join(castDir, file));
    if (!raw) continue;

    const name = raw['姓名'] ?? basename(file, '.json');
    const char = {
      name,
      id: raw['ID'] ?? raw.id ?? null,
      title: raw['身份'] ?? '',
      faction: raw['所属势力'] ?? '',
      rank: raw['天榜排名'] ?? null,
      castPath: `cast/${file}`,
      raw,
    };
    char.aliases = collectAliases(char);
    characters.push(char);
  }

  // 手工诨号（狂刀 → 欧阳狂徒）合并进对应角色的别名
  for (const [alias, canonical] of loadManualAliases()) {
    const target = characters.find((c) => c.name === canonical);
    if (target) target.aliases.push(alias);
    else console.warn(`[registry] aliases.json 里 "${alias}" 指向了不存在的角色 "${canonical}"`);
  }

  return characters;
}

/**
 * 把一个文件匹配到角色，文件名和它所在的子目录名都算数。
 *
 * 返回**全部**命中的角色而不是只取一个：`夜惊鸿VS欧阳狂徒.png` 这种对战图
 * 两边都该能查到。按别名长度降序匹配，避免短别名抢走长名字的图。
 */
function matchCharacters(file, aliasIndex) {
  const haystack = `${file.subdir ?? ''}/${file.name}`;
  const hits = new Map();
  for (const { alias, character } of aliasIndex) {
    if (haystack.includes(alias)) {
      // 同一角色可能通过多个别名命中，只记一次，但保留最长的那次
      const prev = hits.get(character.name);
      if (!prev || alias.length > prev.length) hits.set(character.name, alias);
    }
  }
  return [...hits.keys()];
}

function build() {
  const characters = buildCharacters();

  const aliasIndex = characters
    .flatMap((character) => character.aliases.map((alias) => ({ alias, character })))
    .sort((a, b) => b.alias.length - a.alias.length);

  // references/ 下每个一级目录是一个资产分类（定妆照 / 三视图 / 武器 …）
  const refRoot = join(getSeasonRoot(), 'references');
  const categories = {};
  let refDirs = [];
  try {
    refDirs = readdirSync(refRoot, { withFileTypes: true })
      .filter((e) => e.isDirectory() && !e.name.startsWith('.'))
      .map((e) => e.name);
  } catch {
    console.warn('[registry] references/ 不存在，参考图库为空');
  }

  for (const dir of refDirs) {
    categories[dir] = walk(join(refRoot, dir)).map((f) => ({
      ...f,
      characters: matchCharacters(f, aliasIndex),
    }));
  }

  // 展示站的角色头像也纳入索引 —— 它是唯一一批按 id 下划线命名的图
  const avatars = walk(join(PROJECT_ROOT, '_dashboard', 'assets', 'characters')).map((f) => ({
    ...f,
    characters: matchCharacters(f, aliasIndex),
  }));

  // 反向挂到角色上：char.refs = { 定妆照: [...], 三视图: [...] }
  for (const char of characters) {
    char.refs = {};
    for (const [category, files] of Object.entries(categories)) {
      const mine = files.filter((f) => f.characters.includes(char.name));
      if (mine.length) char.refs[category] = mine;
    }
    char.avatar = avatars.find((f) => f.characters.includes(char.name))?.rel ?? null;
    char.refCount = Object.values(char.refs).reduce((n, arr) => n + arr.length, 0);
  }

  // 天榜在前（按名次），榜外的按素材多少排 —— 素材多的通常就是主力角色。
  // 「未上榜（隐藏实力）」这类没有 Rank 数字的，一律归到榜外。
  const rankOf = (c) => Number(c.rank?.match(/Rank\s*(\d+)/)?.[1] ?? Number.MAX_SAFE_INTEGER);
  characters.sort((a, b) => {
    const [ra, rb] = [rankOf(a), rankOf(b)];
    return ra === rb ? b.refCount - a.refCount : ra - rb;
  });

  const orphans = Object.entries(categories).flatMap(([category, files]) =>
    files.filter((f) => !f.characters.length).map((f) => ({ ...f, category })),
  );

  return {
    characters,
    categories,
    avatars,
    /** 没能归到任何角色的图（场景/武器/海报大多如此，属正常） */
    orphans,
    builtAt: Date.now(),
  };
}

/** 取索引，默认走缓存。改了 cast/ 或加了图之后传 force 重建。 */
export function getRegistry({ force = false } = {}) {
  if (!cache || force) cache = build();
  return cache;
}

export function invalidateRegistry() {
  cache = null;
}
