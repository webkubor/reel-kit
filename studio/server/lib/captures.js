/**
 * 本地轻量库 —— 截图/卡片捕获存储
 *
 * 存到 studio/.state/captures/{uuid}.png + index.json
 * 这是侧车生产状态,gitignore,工作台崩了丢了没事
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync, statSync, unlinkSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { resolveState } from './paths.js';

const CAPTURES_DIR = resolveState('captures');
const INDEX_FILE = resolveState('captures', 'index.json');

function ensureDir() {
  if (!existsSync(CAPTURES_DIR)) mkdirSync(CAPTURES_DIR, { recursive: true });
}

function loadIndex() {
  try {
    return JSON.parse(readFileSync(INDEX_FILE, 'utf8'));
  } catch {
    return { captures: [] };
  }
}

function saveIndex(idx) {
  ensureDir();
  writeFileSync(INDEX_FILE, JSON.stringify(idx, null, 2), 'utf8');
}

function uuid() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

/** 保存 PNG buffer + meta,返回 id */
export function saveCapture(buffer, meta = {}) {
  ensureDir();
  const id = uuid();
  const filename = `${id}.png`;
  const filePath = join(CAPTURES_DIR, filename);
  writeFileSync(filePath, buffer);

  const idx = loadIndex();
  const entry = {
    id,
    filename,
    size: buffer.length,
    createdAt: new Date().toISOString(),
    meta: {
      title: meta.title || '',
      type: meta.type || 'card', // 'card' | 'screenshot'
      season: meta.season || 's1',
      ...meta,
    },
  };
  idx.captures.unshift(entry); // 最新在前
  // 上限 500 条,避免无限增长
  if (idx.captures.length > 500) {
    const removed = idx.captures.splice(500);
    for (const r of removed) {
      try { unlinkSync(join(CAPTURES_DIR, r.filename)); } catch { /* */ }
    }
  }
  saveIndex(idx);
  return entry;
}

/** 列出全部捕获 */
export function listCaptures({ limit = 100, type } = {}) {
  const idx = loadIndex();
  let list = idx.captures;
  if (type) list = list.filter((c) => c.meta?.type === type);
  return list.slice(0, limit);
}

/** 取单个捕获的元数据 + 文件路径 */
export function getCapture(id) {
  const idx = loadIndex();
  return idx.captures.find((c) => c.id === id) || null;
}

/** 删一个 */
export function deleteCapture(id) {
  const idx = loadIndex();
  const i = idx.captures.findIndex((c) => c.id === id);
  if (i < 0) return false;
  const [removed] = idx.captures.splice(i, 1);
  try { unlinkSync(join(CAPTURES_DIR, removed.filename)); } catch { /* */ }
  saveIndex(idx);
  return true;
}

/** 清空 */
export function clearCaptures() {
  const idx = loadIndex();
  for (const c of idx.captures) {
    try { unlinkSync(join(CAPTURES_DIR, c.filename)); } catch { /* */ }
    try { unlinkSync(join(CAPTURES_DIR, c.filename)); } catch { /* */ }
  }
  saveIndex({ captures: [] });
}
