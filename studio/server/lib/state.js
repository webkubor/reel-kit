import { mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { resolveState, STATE_DIR } from './paths.js';

/**
 * 生产状态侧车。
 *
 * 这里存的是「哪个镜头出到第几版、采用了哪张」——工作台自己的账本，
 * 不是内容真源。整个 .state/ 删掉，`cast/` 和 `episodes/` 一个字不少。
 */

function atomicWrite(absPath, text) {
  mkdirSync(dirname(absPath), { recursive: true });
  // 先写同目录临时文件再 rename：中途崩了也不会留下半个 JSON 把账本毁掉
  const tmp = `${absPath}.${process.pid}.tmp`;
  writeFileSync(tmp, text, 'utf8');
  renameSync(tmp, absPath);
}

export function readState(name, fallback) {
  try {
    return JSON.parse(readFileSync(resolveState(name), 'utf8'));
  } catch (err) {
    if (err.code !== 'ENOENT') {
      console.warn(`[state] ${name} 读取失败，用默认值兜底 — ${err.message}`);
    }
    return fallback;
  }
}

export function writeState(name, data) {
  atomicWrite(resolveState(name), JSON.stringify(data, null, 2));
  return data;
}

export function ensureStateDir() {
  mkdirSync(STATE_DIR, { recursive: true });
}

/** 单调递增 id，够本地用，不引 uuid */
export function nextId(prefix, existing) {
  const used = new Set(existing.map((item) => item.id));
  let n = existing.length + 1;
  while (used.has(`${prefix}-${n}`)) n += 1;
  return `${prefix}-${n}`;
}
