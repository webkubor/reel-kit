/**
 * 创意法典 + 旁白 + 镜头美学 API
 *
 * 路由：
 *   GET /api/bible            CREATIVE_BIBLE 结构化
 *   GET /api/voiceover        VOICEOVER 角色评卷版
 *   GET /api/camera-skill     镜头美学 SKILL
 */

import { parseCreativeBible, parseVoiceover, parseCameraSkill } from '../lib/parser.js';

let _cache = null;
let _cacheTime = 0;
const TTL = 60_000;

function getBible() {
  const now = Date.now();
  if (!_cache || now - _cacheTime > TTL) {
    _cache = {
      bible: parseCreativeBible(),
      voiceover: parseVoiceover(),
      cameraSkill: parseCameraSkill(),
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

export function bibleHandler(req, res, next) {
  const url = new URL(req.url, 'http://x');
  if (url.pathname === '/api/bible') {
    return send(res, 200, { bible: getBible().bible });
  }
  if (url.pathname === '/api/voiceover') {
    return send(res, 200, { voiceover: getBible().voiceover });
  }
  if (url.pathname === '/api/camera-skill') {
    return send(res, 200, { cameraSkill: getBible().cameraSkill });
  }
  next();
}
