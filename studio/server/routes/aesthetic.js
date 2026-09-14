/**
 * 镜头·音乐·审美 三轴 API
 *
 * 路由：
 *   GET /api/aesthetic      影调协议（来自 CREATIVE_BIBLE）
 *   GET /api/bgm            BGM 库
 *   GET /api/theme          主题曲「雪沸」
 *   GET /api/weapons        神兵谱
 *   GET /api/aesthetic/all  一次性返回所有
 */

import { parseAesthetic, parseBgm, parseMainTheme, parseWeapons } from '../lib/parser.js';

let _cache = null;
let _cacheTime = 0;
const TTL = 60_000;

function getAll() {
  const now = Date.now();
  if (!_cache || now - _cacheTime > TTL) {
    _cache = {
      aesthetic: parseAesthetic(),
      bgm: parseBgm(),
      theme: parseMainTheme(),
      weapons: parseWeapons(),
    };
    _cacheTime = now;
  }
  return _cache;
}

function send(res, status, body) {
  res.statusCode = status;
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  res.end(JSON.stringify(body));
}

export function aestheticHandler(req, res, next) {
  const url = new URL(req.url, 'http://x');
  const all = getAll();
  if (url.pathname === '/api/aesthetic') return send(res, 200, { aesthetic: all.aesthetic });
  if (url.pathname === '/api/bgm') return send(res, 200, { bgm: all.bgm });
  if (url.pathname === '/api/theme') return send(res, 200, { theme: all.theme });
  if (url.pathname === '/api/weapons') return send(res, 200, { weapons: all.weapons });
  if (url.pathname === '/api/aesthetic/all') return send(res, 200, all);
  next();
}
