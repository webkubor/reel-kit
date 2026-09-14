/**
 * 剧集 API —— 14 集 ep00-ep13 全部数据 + 单集详情 + storyboards
 *
 * 路由：
 *   GET /api/episodes          14 集摘要列表
 *   GET /api/episodes/:n       单集详情（n=0..13）
 *   GET /api/storyboards       单镜头 storyboard 列表
 *   GET /api/storyboards/:file 单个 storyboard
 */

import { readFileSync } from 'node:fs';
import { join, basename } from 'node:path';
import { PROJECT_ROOT } from '../lib/paths.js';
import { parseAllEpisodes, parseStoryboards } from '../lib/parser.js';

let _cache = null;
let _cacheTime = 0;
const TTL = 30_000; // 30s 缓存（开发期频繁改文件，足够）

function getEpisodes() {
  const now = Date.now();
  if (!_cache || now - _cacheTime > TTL) {
    _cache = parseAllEpisodes();
    _cacheTime = now;
  }
  return _cache;
}

function getStoryboards() {
  return parseStoryboards();
}

function summarize(ep) {
  return {
    ep: ep.ep,
    slug: ep.slug,
    title: ep.title || ep.slug,
    core: ep.core || '',
    kind: ep.kind,
    status: ep.status,
    statusLabel: ep.statusLabel,
    exists: ep.exists,
    shotCount: ep.acts?.reduce((n, a) => n + (a.shots?.length || 0), 0)
      || ep.scenes?.length
      || ep.prologueParts?.length
      || ep.storyboardShots?.length
      || 0,
    jimengCount: ep.jimeng?.length || 0,
    voiceoverCount: ep.voiceover?.length || 0,
    charCount: ep.charCount,
    updatedAt: ep.updatedAt,
  };
}

function send(res, status, body) {
  res.statusCode = status;
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  res.end(JSON.stringify(body));
}

export function episodesHandler(req, res, next) {
  const url = new URL(req.url, 'http://x');
  const m = url.pathname.match(/^\/api\/episodes\/(\d+)$/);
  if (m) {
    const n = parseInt(m[1], 10);
    if (n < 0 || n > 13) return send(res, 404, { error: 'ep out of range' });
    const eps = getEpisodes();
    const ep = eps[n];
    if (!ep) return send(res, 404, { error: 'ep not found' });
    return send(res, 200, { ep });
  }
  if (url.pathname === '/api/episodes') {
    const eps = getEpisodes().map(summarize);
    return send(res, 200, { episodes: eps, total: eps.length });
  }
  next();
}

export function storyboardsHandler(req, res, next) {
  const url = new URL(req.url, 'http://x');
  if (url.pathname === '/api/storyboards') {
    const boards = getStoryboards().map((b) => ({
      file: b.file,
      rel: b.rel,
      title: b.title,
      shotCount: b.shots.length,
      jimengCount: b.jimeng.length,
      charCount: b.charCount,
      updatedAt: b.updatedAt,
    }));
    return send(res, 200, { storyboards: boards, total: boards.length });
  }
  const m = url.pathname.match(/^\/api\/storyboards\/(.+)$/);
  if (m) {
    const file = decodeURIComponent(m[1]);
    const board = getStoryboards().find((b) => b.file === file);
    if (!board) return send(res, 404, { error: 'not found' });
    return send(res, 200, { storyboard: board });
  }
  next();
}
