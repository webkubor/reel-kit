import { dirname, join, resolve, sep } from 'node:path';
import { existsSync, readdirSync, statSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

/** studio/ 自身 */
export const STUDIO_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
/** 仓库根 */
export const PROJECT_ROOT = resolve(STUDIO_ROOT, '..');
/** 生产状态侧车，不进 git */
export const STATE_DIR = join(STUDIO_ROOT, '.state');
/** 多季/多 IP 根目录（默认 s1 在这里） */
export const SEASONS_ROOT = join(PROJECT_ROOT, 'seasons');

/** 默认季名：环境变量 > 写死的 's1' */
export const DEFAULT_SEASON = process.env.STUDIO_SEASON || 's1';

/**
 * 季根目录解析 —— 不存在则降级到 PROJECT_ROOT（向后兼容老布局）。
 * 这样在 git mv 落地前老的 cast/ episodes/ 还能被读到，避免硬切换炸。
 */
export function getSeasonRoot(season = DEFAULT_SEASON) {
  const seasonDir = join(SEASONS_ROOT, season);
  if (existsSync(seasonDir)) return seasonDir;
  // 降级：s1 找不到时仍读老布局；其他季名直接抛
  if (season === DEFAULT_SEASON) return PROJECT_ROOT;
  throw Object.assign(new Error(`季不存在: ${season}`), { statusCode: 404 });
}

/** 列出所有季（基于 seasons/ 下的子目录） */
export function listSeasons() {
  try {
    return readdirSync(SEASONS_ROOT)
      .filter((d) => d.startsWith('s') && !d.startsWith('.'))
      .filter((d) => statSync(join(SEASONS_ROOT, d)).isDirectory())
      .sort();
  } catch {
    return [];
  }
}

/**
 * 季内部可读白名单。在 getSeasonRoot 下加白名单 = 越界保护。
 * 跨季共享的 agents/ 仍在 PROJECT_ROOT 下，单独维护。
 *
 * novels/ 用来让工作台能直接调取小说灵感库的章节/人物小传/设定 + 内嵌的
 * .agent-skills/(Agent 创作技能集)。这是 P1 阶段把"小说→剧集→分镜"链路打通的基础。
 */
function seasonReadableDirs(season) {
  return [
    'cast',
    'episodes',
    'bibles',
    'music',
    'references',
    'scripts/storyboards',
    'novels',
  ].map((d) => join(getSeasonRoot(season), d));
}

/** 跨季共享的可读目录（PROJECT_ROOT 下） */
const SHARED_READABLE = [
  'agents',
  'skills',
  '_dashboard/assets',
];

/** 季内部可写白名单：只有 cast/ */
function seasonWritableDirs(season) {
  return [join(getSeasonRoot(season), 'cast')];
}

function isInside(child, parent) {
  const p = resolve(parent);
  const c = resolve(child);
  return c === p || c.startsWith(p + sep);
}

function isReadable(abs) {
  return [...SHARED_READABLE.map((d) => join(PROJECT_ROOT, d)), ...seasonReadableDirs(DEFAULT_SEASON)]
    .some((dir) => isInside(abs, dir));
}

function isWritable(abs) {
  return seasonWritableDirs(DEFAULT_SEASON).some((dir) => isInside(abs, dir));
}

function checkAgainst(relPath, predicate, action) {
  if (typeof relPath !== 'string' || !relPath.length) {
    throw Object.assign(new Error('缺少路径'), { statusCode: 400 });
  }
  // 先归一化再比对：'cast/../.ssh/id_rsa' 这类逃逸在 resolve 后才现原形
  const abs = resolve(PROJECT_ROOT, relPath);
  if (!predicate(abs)) {
    throw Object.assign(new Error(`路径不在${action}白名单内: ${relPath}`), { statusCode: 403 });
  }
  return abs;
}

/** 解析可读路径，越界抛 403 */
export function resolveReadable(relPath) {
  return checkAgainst(relPath, isReadable, '可读');
}

/** 解析可写路径，越界抛 403。比可读严格：只有 cast/ */
export function resolveWritable(relPath) {
  return checkAgainst(relPath, isWritable, '可写');
}

/** 侧车状态文件路径，限制在 .state/ 内 */
export function resolveState(...segments) {
  const abs = resolve(STATE_DIR, ...segments);
  if (!isInside(abs, STATE_DIR)) {
    throw Object.assign(new Error('状态路径越界'), { statusCode: 403 });
  }
  return abs;
}

/** 把季内相对路径（如 'episodes/ep01.md'）转换成绝对路径。给 server 解析用。 */
export function resolveSeasonPath(season, ...segments) {
  return join(getSeasonRoot(season), ...segments);
}
