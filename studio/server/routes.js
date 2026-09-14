import { createReadStream, readFileSync, statSync, writeFileSync } from 'node:fs';
import { extname, join } from 'node:path';
import { createRouter, fail, json } from './lib/http.js';
import { generate, health, listJobs, postProcess } from './lib/museav.js';
import {
  DEFAULT_SEASON,
  getSeasonRoot,
  listSeasons,
  PROJECT_ROOT,
  resolveReadable,
  resolveWritable,
} from './lib/paths.js';
import { nextId, readState, writeState } from './lib/state.js';
import { getRegistry, invalidateRegistry } from './registry.js';
import { saveCapture, listCaptures, getCapture, deleteCapture, clearCaptures } from './lib/captures.js';
import {
  parseAllEpisodes,
  parseStoryboards,
  parseCreativeBible,
  parseVoiceover,
  parseCameraSkill,
  parseAesthetic,
  parseBgm,
  parseMainTheme,
  parseWeapons,
  setSeason,
} from './lib/parser.js';
import { listSkills, getSkill, composeSkillPrompt } from './lib/skills.js';

/** 全局搜索：合并角色 / 剧集 / 神兵 / BGM。query 太短或空就返回空数组。 */
function globalSearch(q, season) {
  setSeason(season);
  const query = (q || '').trim();
  if (query.length < 1) return [];
  const results = [];
  // 角色
  try {
    for (const c of getRegistry().characters) {
      const hit = c.name?.includes(query) || c.title?.includes(query) || c.aliases?.some((a) => a?.includes(query));
      if (hit) {
        results.push({ type: 'character', name: c.name, title: c.title, faction: c.faction, rank: c.rank, avatar: c.avatar, to: '/cast' });
      }
    }
  } catch { /* */ }
  // 剧集
  try {
    for (const ep of parseAllEpisodes()) {
      const hit = ep.title?.includes(query) || ep.core?.includes(query) || String(ep.ep).includes(query);
      if (hit) {
        results.push({ type: 'episode', ep: ep.ep, title: ep.title, core: ep.core?.slice(0, 60), status: ep.statusLabel, to: '/episodes' });
      }
    }
  } catch { /* */ }
  // 神兵
  try {
    for (const w of parseWeapons()) {
      const hit = w.name?.includes(query) || w.holder?.includes(query);
      if (hit) {
        results.push({ type: 'weapon', name: w.name, holder: w.holder, to: '/cast' });
      }
    }
  } catch { /* */ }
  // BGM
  try {
    const bgm = parseBgm();
    for (const b of bgm?.blocks || []) {
      const hit = b.title?.includes(query) || b.tag?.includes(query);
      if (hit) {
        results.push({ type: 'bgm', title: b.title, tag: b.tag, to: '/aesthetic' });
      }
    }
  } catch { /* */ }
  return results.slice(0, 30);
}

const MIME = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.mp4': 'video/mp4',
  '.mov': 'video/quicktime',
  '.webm': 'video/webm',
  // 标题字体直接读仓库里的原文件，不在 studio 下存副本
  '.ttf': 'font/ttf',
  '.otf': 'font/otf',
  '.woff2': 'font/woff2',
};

/* ── SSE：出图是几十秒的长活，进度得能实时推回前端 ───────────────── */

const sseClients = new Set();

function broadcast(event, data) {
  const payload = `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
  for (const res of sseClients) {
    // 某个页签关了不该影响其它页签
    try {
      res.write(payload);
    } catch {
      sseClients.delete(res);
    }
  }
}

/* ── 镜头（工作台自己的账本，不碰 episodes/） ─────────────────────── */

const SHOTS_FILE = 'shots.json';
const emptyShots = () => ({ shots: [] });

function loadShots() {
  const state = readState(SHOTS_FILE, emptyShots());
  if (!Array.isArray(state.shots)) return emptyShots();
  return state;
}

/** 只挑白名单字段落盘，免得前端顺手把整个对象塞回来污染账本 */
function sanitizeShot(input, base = {}) {
  const pick = (key, fallback) => (input[key] !== undefined ? input[key] : (base[key] ?? fallback));
  return {
    id: base.id,
    episode: pick('episode', null),
    title: pick('title', '未命名镜头'),
    order: Number(pick('order', 0)) || 0,
    duration: Number(pick('duration', 5)) || 5,
    prompt: pick('prompt', ''),
    ratio: pick('ratio', '16:9'),
    model: pick('model', ''),
    characters: Array.isArray(pick('characters')) ? pick('characters') : [],
    refs: Array.isArray(pick('refs')) ? pick('refs') : [],
    status: pick('status', 'draft'),
    notes: pick('notes', ''),
    takes: Array.isArray(base.takes) ? base.takes : [],
    createdAt: base.createdAt ?? new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
}

/* ── 剧集 / 分镜（只读） ──────────────────────────────────────── */

/** 把单集收敛成列表用摘要（不返回 raw、acts、scenes 等大字段） */
function summarizeEpisode(ep) {
  return {
    ep: ep.ep,
    slug: ep.slug,
    title: ep.title || ep.slug,
    core: ep.core || '',
    kind: ep.kind,
    status: ep.status,
    statusLabel: ep.statusLabel,
    exists: ep.exists,
    shotCount:
      (ep.acts?.reduce((n, a) => n + (a.shots?.length || 0), 0) ?? 0)
      || ep.scenes?.length
      || ep.prologueParts?.length
      || ep.storyboardShots?.length
      || 0,
    jimengCount: ep.jimeng?.length || 0,
    voiceoverCount: ep.voiceover?.length || 0,
    audioCount: ep.audio?.length || 0,
    charCount: ep.charCount,
    updatedAt: ep.updatedAt,
  };
}

function summarizeStoryboard(b) {
  return {
    file: b.file,
    rel: b.rel,
    title: b.title,
    shotCount: b.shots.length,
    jimengCount: b.jimeng.length,
    charCount: b.charCount,
    updatedAt: b.updatedAt,
  };
}

/* ── 路由表 ──────────────────────────────────────────────────── */

export const handleApi = createRouter({
  'GET /health': async ({ res }) => {
    const reg = getRegistry();
    json(res, {
      museav: await health(),
      registry: {
        characters: reg.characters.length,
        categories: Object.fromEntries(
          Object.entries(reg.categories).map(([k, v]) => [k, v.length]),
        ),
        orphans: reg.orphans.length,
      },
    });
  },

  'GET /registry': ({ res, query }) => {
    const reg = getRegistry({ force: query.get('refresh') === '1' });
    json(res, {
      // 不回 raw：角色全文按需单取，列表页不需要几百 KB
      characters: reg.characters.map(({ raw, ...rest }) => rest),
      categories: reg.categories,
      orphans: reg.orphans,
      builtAt: reg.builtAt,
    });
  },

  'GET /cast/:name': ({ res, params }) => {
    const char = getRegistry().characters.find((c) => c.name === params.name);
    if (!char) throw Object.assign(new Error(`没有这个角色: ${params.name}`), { statusCode: 404 });
    json(res, char);
  },

  'PUT /cast/:name': ({ res, params, body }) => {
    const char = getRegistry().characters.find((c) => c.name === params.name);
    if (!char) throw Object.assign(new Error(`没有这个角色: ${params.name}`), { statusCode: 404 });
    if (!body || typeof body !== 'object' || Array.isArray(body)) {
      throw Object.assign(new Error('角色档案必须是一个 JSON 对象'), { statusCode: 400 });
    }
    // 缩进 2 + 中文不转义，跟仓库里现有 JSON 的风格一致，diff 才干净
    const abs = resolveWritable(char.castPath);
    writeFileSync(abs, `${JSON.stringify(body, null, 2)}\n`, 'utf8');
    invalidateRegistry();
    json(res, { ok: true, path: char.castPath });
  },

  'GET /episodes': ({ res, query }) => {
    setSeason(query.get('season'));
    const eps = parseAllEpisodes().map(summarizeEpisode);
    json(res, { episodes: eps, total: eps.length, season: query.get('season') || DEFAULT_SEASON });
  },
  'GET /episodes/:n': ({ res, params, query }) => {
    setSeason(query.get('season'));
    const n = parseInt(params.n, 10);
    if (Number.isNaN(n) || n < 0 || n > 13) {
      throw Object.assign(new Error('ep 必须在 0-13 之间'), { statusCode: 400 });
    }
    const ep = parseAllEpisodes()[n];
    json(res, { ep });
  },
  'GET /storyboards': ({ res, query }) => {
    setSeason(query.get('season'));
    const boards = parseStoryboards().map(summarizeStoryboard);
    json(res, { storyboards: boards, total: boards.length, season: query.get('season') || DEFAULT_SEASON });
  },
  'GET /storyboards/:file': ({ res, params, query }) => {
    setSeason(query.get('season'));
    const file = decodeURIComponent(params.file);
    const board = parseStoryboards().find((b) => b.file === file);
    if (!board) throw Object.assign(new Error('storyboard 不存在'), { statusCode: 404 });
    json(res, { storyboard: board });
  },

  // ── 创意法典 + 旁白 + 镜头美学 ──
  'GET /bible': ({ res, query }) => {
    setSeason(query.get('season'));
    json(res, { bible: parseCreativeBible() });
  },
  'GET /voiceover': ({ res, query }) => {
    setSeason(query.get('season'));
    json(res, { voiceover: parseVoiceover() });
  },
  'GET /camera-skill': ({ res, query }) => {
    setSeason(query.get('season'));
    json(res, { cameraSkill: parseCameraSkill() });
  },

  // ── 镜头·音乐·审美 三轴 ──
  'GET /aesthetic': ({ res, query }) => {
    setSeason(query.get('season'));
    json(res, { aesthetic: parseAesthetic() });
  },
  'GET /bgm': ({ res, query }) => {
    setSeason(query.get('season'));
    json(res, { bgm: parseBgm() });
  },
  'GET /theme': ({ res, query }) => {
    setSeason(query.get('season'));
    json(res, { theme: parseMainTheme() });
  },
  'GET /weapons': ({ res, query }) => {
    setSeason(query.get('season'));
    json(res, { weapons: parseWeapons() });
  },

  // ── 季/IP 切换 ──
  'GET /seasons': ({ res }) => {
    const seasons = listSeasons().map((s) => ({
      name: s,
      root: getSeasonRoot(s),
    }));
    json(res, { seasons, current: DEFAULT_SEASON });
  },
  'GET /seasons/:name/manifest': ({ res, params }) => {
    const name = params.name;
    setSeason(name);
    // 给个轻量 manifest：14 集数 + 角色数 + 神兵数 + BGM 数
    const eps = parseAllEpisodes();
    const reg = getRegistry();
    const bgm = parseBgm();
    const weapons = parseWeapons();
    json(res, {
      season: name,
      episodes: { total: eps.length, finalized: eps.filter((e) => e.status === 'finalized').length },
      characters: reg.characters.length,
      weapons: weapons.length,
      bgm: bgm?.blocks?.length || 0,
    });
  },

  // ── 全局搜索 ──
  'GET /search': ({ res, query }) => {
    const q = query.get('q') || '';
    const results = globalSearch(q, query.get('season'));
    json(res, { q, results, total: results.length });
  },

  // ── Skill 浏览器与上下文组装器（来自 novels/.agent-skills/） ──
  'GET /skills': ({ res, query }) => {
    const season = query.get('season') || DEFAULT_SEASON;
    const skills = listSkills(season).map(({ body, ...rest }) => ({
      ...rest,
      // 列表不带 body（KB 级），详情按 :id 取
      bodyPreview: body?.slice(0, 220) || '',
    }));
    json(res, { skills, total: skills.length, season });
  },
  'GET /skills/:id': ({ res, params, query }) => {
    const season = query.get('season') || DEFAULT_SEASON;
    const skill = getSkill(season, params.id);
    if (!skill) throw Object.assign(new Error(`skill 不存在: ${params.id}`), { statusCode: 404 });
    json(res, { skill });
  },
  'POST /skills/:id/compose': ({ res, params, body, query }) => {
    const season = query.get('season') || DEFAULT_SEASON;
    const result = composeSkillPrompt(season, {
      skillId: params.id,
      context: body?.context || {},
    });
    json(res, result);
  },

  // ── 本地轻量库:截图/卡片捕获 ──
  'GET /captures': ({ res, query }) => {
    const list = listCaptures({
      limit: Number(query.get('limit')) || 100,
      type: query.get('type') || undefined,
    });
    json(res, { captures: list, total: list.length });
  },
  'GET /captures/:id': ({ res, params }) => {
    const cap = getCapture(params.id);
    if (!cap) throw Object.assign(new Error('捕获不存在'), { statusCode: 404 });
    json(res, { capture: cap });
  },
  'DELETE /captures/:id': ({ res, params }) => {
    const ok = deleteCapture(params.id);
    if (!ok) throw Object.assign(new Error('捕获不存在'), { statusCode: 404 });
    json(res, { ok: true });
  },
  'DELETE /captures': ({ res }) => {
    clearCaptures();
    json(res, { ok: true });
  },
  'POST /captures raw': async ({ req, res }) => {
    // 上传 PNG 二进制 + meta
    // 客户端用 multipart/form-data,这里简单点:用 raw body + X-Meta header
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const buffer = Buffer.concat(chunks);
    if (buffer.length === 0) throw Object.assign(new Error('empty body'), { statusCode: 400 });
    if (buffer.length > 20 * 1024 * 1024) {
      throw Object.assign(new Error('超过 20MB 上限'), { statusCode: 413 });
    }
    let meta = {};
    try {
      const raw = req.headers['x-meta'];
      if (raw) meta = JSON.parse(raw);
    } catch { /* */ }
    const cap = saveCapture(buffer, meta);
    json(res, { ok: true, capture: cap }, 201);
  },
  'GET /captures/:id/raw': ({ res, params }) => {
    const cap = getCapture(params.id);
    if (!cap) throw Object.assign(new Error('捕获不存在'), { statusCode: 404 });
    const abs = join(PROJECT_ROOT, 'studio', '.state', 'captures', cap.filename);
    const st = statSync(abs);
    res.writeHead(200, {
      'content-type': 'image/png',
      'content-length': st.size,
      'cache-control': 'no-cache',
    });
    createReadStream(abs).pipe(res);
  },

  'GET /doc/:rel': ({ res, params }) => {
    const abs = resolveReadable(params.rel);
    json(res, { rel: params.rel, text: readFileSync(abs, 'utf8') });
  },

  'GET /shots': ({ res }) => json(res, loadShots()),

  'POST /shots': ({ res, body }) => {
    const state = loadShots();
    const shot = sanitizeShot(body);
    shot.id = nextId('shot', state.shots);
    state.shots.push(shot);
    writeState(SHOTS_FILE, state);
    json(res, shot, 201);
  },

  'PUT /shots/:id': ({ res, params, body }) => {
    const state = loadShots();
    const index = state.shots.findIndex((s) => s.id === params.id);
    if (index < 0) throw Object.assign(new Error('镜头不存在'), { statusCode: 404 });
    state.shots[index] = sanitizeShot(body, state.shots[index]);
    writeState(SHOTS_FILE, state);
    json(res, state.shots[index]);
  },

  'DELETE /shots/:id': ({ res, params }) => {
    const state = loadShots();
    const index = state.shots.findIndex((s) => s.id === params.id);
    if (index < 0) throw Object.assign(new Error('镜头不存在'), { statusCode: 404 });
    const [removed] = state.shots.splice(index, 1);
    writeState(SHOTS_FILE, state);
    json(res, { ok: true, removed: removed.id });
  },

  /** 标记采用哪一版出图 */
  'POST /shots/:id/adopt': ({ res, params, body }) => {
    const state = loadShots();
    const shot = state.shots.find((s) => s.id === params.id);
    if (!shot) throw Object.assign(new Error('镜头不存在'), { statusCode: 404 });
    let found = false;
    for (const take of shot.takes) {
      take.adopted = take.id === body.takeId;
      if (take.adopted) found = true;
    }
    if (!found) throw Object.assign(new Error('这一版不存在'), { statusCode: 404 });
    shot.status = 'adopted';
    shot.updatedAt = new Date().toISOString();
    writeState(SHOTS_FILE, state);
    json(res, shot);
  },

  /**
   * 出图 / 出视频。**这一步花钱**，所以只在用户显式点击时触发，
   * 后端任何地方都不会自动调它。立即返回，进度走 SSE。
   */
  'POST /shots/:id/generate': ({ res, params, body }) => {
    const state = loadShots();
    const shot = state.shots.find((s) => s.id === params.id);
    if (!shot) throw Object.assign(new Error('镜头不存在'), { statusCode: 404 });

    const prompt = (body.prompt ?? shot.prompt ?? '').trim();
    if (!prompt) throw Object.assign(new Error('这个镜头还没有 prompt'), { statusCode: 400 });

    // 垫图存的是仓库相对路径，交给 museav 前换成绝对路径
    const refs = (body.refs ?? shot.refs ?? []).map((rel) => resolveReadable(rel));

    const takeId = nextId('take', shot.takes);
    const take = {
      id: takeId,
      prompt,
      video: Boolean(body.video),
      model: body.model || shot.model || '',
      ratio: body.ratio || shot.ratio || '16:9',
      refs: body.refs ?? shot.refs ?? [],
      status: 'running',
      url: null,
      error: null,
      startedAt: new Date().toISOString(),
    };
    shot.takes.push(take);
    shot.status = 'generating';
    writeState(SHOTS_FILE, state);
    json(res, { ok: true, take }, 202);

    generate(
      {
        prompt,
        ratio: take.ratio,
        model: take.model || undefined,
        refs,
        video: take.video,
        duration: take.video ? shot.duration : undefined,
      },
      (line) => broadcast('progress', { shotId: shot.id, takeId, line }),
    )
      // 留下 museav 的原始 stdout：出图结果对不上时，这是唯一能复盘的线索
      .then((result) => finishTake(shot.id, takeId, { status: 'done', url: result.url, raw: result.raw }))
      .catch((err) => finishTake(shot.id, takeId, { status: 'failed', error: err.message }));
  },

  'GET /jobs': async ({ res, query }) => {
    json(res, await listJobs({ limit: Number(query.get('limit')) || 20 }));
  },

  'POST /post-process': async ({ res, body }) => {
    const abs = resolveReadable(body.path);
    json(res, await postProcess(body.action, abs, body.args ?? []));
  },

  /** 图片/视频直出。前端 <img src="/api/asset?p=references/..."> */
  'GET /asset': ({ req, res, query }) => {
    const rel = query.get('p');
    const abs = resolveReadable(rel);
    const st = statSync(abs);
    const type = MIME[extname(abs).toLowerCase()] ?? 'application/octet-stream';

    // 视频要能拖进度条，得支持 Range
    const range = req.headers.range;
    if (range && type.startsWith('video/')) {
      const [startRaw, endRaw] = range.replace(/bytes=/, '').split('-');
      const start = Number(startRaw) || 0;
      const end = endRaw ? Number(endRaw) : st.size - 1;
      res.writeHead(206, {
        'content-type': type,
        'content-range': `bytes ${start}-${end}/${st.size}`,
        'accept-ranges': 'bytes',
        'content-length': end - start + 1,
      });
      createReadStream(abs, { start, end }).pipe(res);
      return;
    }

    res.writeHead(200, {
      'content-type': type,
      'content-length': st.size,
      // 本地文件随时可能被重新出图覆盖，别让浏览器缓存住
      'cache-control': 'no-cache',
    });
    createReadStream(abs).pipe(res);
  },

  'GET /events': ({ req, res }) => {
    res.writeHead(200, {
      'content-type': 'text/event-stream',
      'cache-control': 'no-cache',
      connection: 'keep-alive',
    });
    res.write('retry: 3000\n\n');
    sseClients.add(res);

    const keepAlive = setInterval(() => {
      try {
        res.write(': ping\n\n');
      } catch {
        clearInterval(keepAlive);
      }
    }, 25_000);

    req.on('close', () => {
      clearInterval(keepAlive);
      sseClients.delete(res);
    });
  },
});

/** 出图结束后回写账本并广播——两个 then 分支共用，避免写重复逻辑 */
function finishTake(shotId, takeId, patch) {
  const state = loadShots();
  const shot = state.shots.find((s) => s.id === shotId);
  if (!shot) return;
  const take = shot.takes.find((t) => t.id === takeId);
  if (!take) return;

  Object.assign(take, patch, { finishedAt: new Date().toISOString() });
  // 别把已经采用过的镜头打回去
  if (shot.status === 'generating') {
    shot.status = patch.status === 'done' ? 'review' : 'failed';
  }
  writeState(SHOTS_FILE, state);
  broadcast('take', { shotId, take });
}

/** 挂进 Vite dev server 的 middleware */
export function apiMiddleware() {
  return async (req, res, next) => {
    if (!req.url?.startsWith('/api/')) return next();
    const url = new URL(req.url, 'http://localhost');
    try {
      const matched = await handleApi(req, res, url.pathname.slice('/api'.length), url.searchParams);
      if (!matched) json(res, { error: `未知接口: ${url.pathname}` }, 404);
    } catch (err) {
      if (!res.headersSent) fail(res, err);
      else res.end();
    }
  };
}
