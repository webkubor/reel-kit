import { ref } from 'vue';

const SEASON_KEY = 'studio.season';
const currentSeason = ref(localStorage.getItem(SEASON_KEY) || 's1');

/** 切季后所有后续 request 会自动带 ?season= */
export function setCurrentSeason(name) {
  currentSeason.value = name || 's1';
  localStorage.setItem(SEASON_KEY, currentSeason.value);
}
export function getCurrentSeason() { return currentSeason.value; }

function withSeason(path) {
  const sep = path.includes('?') ? '&' : '?';
  return `${path}${sep}season=${encodeURIComponent(currentSeason.value)}`;
}

async function request(path, options = {}) {
  const url = withSeason(path);
  const res = await fetch(`/api${url}`, {
    headers: { 'content-type': 'application/json' },
    ...options,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) throw new Error(data?.error ?? `请求失败 (${res.status})`);
  return data;
}

export const api = {
  health: () => request('/health'),
  registry: (refresh = false) => request(`/registry${refresh ? '?refresh=1' : ''}`),

  character: (name) => request(`/cast/${encodeURIComponent(name)}`),
  saveCharacter: (name, raw) =>
    request(`/cast/${encodeURIComponent(name)}`, { method: 'PUT', body: raw }),

  // 季切换
  seasons: () => request('/seasons'),
  seasonManifest: (name) => request(`/seasons/${encodeURIComponent(name)}/manifest`),

  // 全局搜索
  search: (q) => request(`/search?q=${encodeURIComponent(q)}`),

  // 截图/卡片捕获
  captures: (opts = {}) => {
    const params = new URLSearchParams();
    if (opts.limit) params.set('limit', String(opts.limit));
    if (opts.type) params.set('type', opts.type);
    return request(`/captures${params.toString() ? '?' + params : ''}`);
  },
  deleteCapture: (id) => request(`/captures/${id}`, { method: 'DELETE' }),
  clearCaptures: () => request('/captures', { method: 'DELETE' }),
  async uploadCapture(blob, meta = {}) {
    const buf = await blob.arrayBuffer();
    const res = await fetch(`/api/captures`, {
      method: 'POST',
      headers: {
        'content-type': 'image/png',
        'x-meta': JSON.stringify(meta),
      },
      body: buf,
    });
    const text = await res.text();
    const data = text ? JSON.parse(text) : null;
    if (!res.ok) throw new Error(data?.error ?? `上传失败 (${res.status})`);
    return data.capture;
  },

  episodes: () => request('/episodes'),
  episode: (n) => request(`/episodes/${n}`),
  storyboards: () => request('/storyboards'),
  storyboard: (file) => request(`/storyboards/${encodeURIComponent(file)}`),
  doc: (rel) => request(`/doc/${encodeURIComponent(rel)}`),

  // 创意法典 + 旁白 + 镜头美学
  bible: () => request('/bible'),
  voiceover: () => request('/voiceover'),
  cameraSkill: () => request('/camera-skill'),

  // 镜头·音乐·审美 三轴
  aesthetic: () => request('/aesthetic'),
  bgm: () => request('/bgm'),
  theme: () => request('/theme'),
  weapons: () => request('/weapons'),

  // Skill 浏览器与上下文组装器（来自 novels/.agent-skills/）
  skills: () => request('/skills'),
  skill: (id) => request(`/skills/${encodeURIComponent(id)}`),
  composeSkill: (id, context) =>
    request(`/skills/${encodeURIComponent(id)}/compose`, {
      method: 'POST',
      body: { context },
    }),

  shots: () => request('/shots'),
  createShot: (shot) => request('/shots', { method: 'POST', body: shot }),
  updateShot: (id, shot) => request(`/shots/${id}`, { method: 'PUT', body: shot }),
  deleteShot: (id) => request(`/shots/${id}`, { method: 'DELETE' }),
  adoptTake: (id, takeId) => request(`/shots/${id}/adopt`, { method: 'POST', body: { takeId } }),
  generate: (id, opts) => request(`/shots/${id}/generate`, { method: 'POST', body: opts }),

  jobs: (limit = 20) => request(`/jobs?limit=${limit}`),
  postProcess: (body) => request('/post-process', { method: 'POST', body }),
};

/** 仓库相对路径 → 可直接塞进 <img src> 的 URL */
export function assetUrl(rel) {
  return rel ? `/api/asset?p=${encodeURIComponent(rel)}` : '';
}

/* ── SSE：出图要跑几十秒，进度靠它推回来 ─────────────────────── */

const listeners = new Map();
export const sseConnected = ref(false);
let source = null;

function ensureSource() {
  if (source) return;
  source = new EventSource('/api/events');
  source.onopen = () => {
    sseConnected.value = true;
  };
  source.onerror = () => {
    // EventSource 自带重连，这里只反映状态，不手动重建（会连出两条流）
    sseConnected.value = false;
  };
  for (const event of ['progress', 'take']) {
    source.addEventListener(event, (e) => {
      const data = JSON.parse(e.data);
      for (const fn of listeners.get(event) ?? []) fn(data);
    });
  }
}

export function onServerEvent(event, handler) {
  ensureSource();
  if (!listeners.has(event)) listeners.set(event, new Set());
  listeners.get(event).add(handler);
  return () => listeners.get(event)?.delete(handler);
}
