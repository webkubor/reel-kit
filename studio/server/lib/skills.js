/**
 * Skill 浏览器与上下文组装器
 *
 * 数据源：seasons/{season}/novels/.agent-skills/ 下每个子目录的 SKILL.md
 *   - frontmatter 含 name / description / triggers
 *   - body 是 markdown 正文
 *
 * 设计原则：
 *   - read-only：永远不写回 .agent-skills/，那是小说写作 agent 用的真源
 *   - 轻缓存：开发期频繁改 skill，缓存 30s
 *   - 上下文组装：把"选中的 skill + 当前剧集/角色/神兵 + 用户输入"拼成一段
 *     可以直接复制到 Claude/外部 Agent 的 prompt
 */

import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { getSeasonRoot } from './paths.js';

const CACHE_TTL = 30_000;
let _cache = { at: 0, data: null };

/** 列出 novels/.agent-skills/ 下所有 skill 目录 */
function listSkillDirs(season) {
  const root = join(getSeasonRoot(season), 'novels', '.agent-skills');
  try {
    return readdirSync(root)
      .filter((d) => !d.startsWith('.'))
      .filter((d) => {
        try { return statSync(join(root, d)).isDirectory(); } catch { return false; }
      });
  } catch {
    return [];
  }
}

/**
 * 解析单个 SKILL.md 的 frontmatter + body
 * 容错：frontmatter 缺失时整文件当 body
 */
function parseSkillFile(absPath) {
  const raw = readFileSync(absPath, 'utf8');
  const fmMatch = raw.match(/^---\s*\n([\s\S]*?)\n---\s*\n([\s\S]*)$/);
  if (!fmMatch) {
    return { name: '', description: '', triggers: [], body: raw };
  }
  const [, fmBlock, body] = fmMatch;
  // 简易 YAML 解析：只支持 name / description / triggers 这三个字段
  const meta = {};
  const lines = fmBlock.split('\n');
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    const m = line.match(/^([a-zA-Z_]+)\s*:\s*(.*)$/);
    if (m) {
      const key = m[1];
      let val = m[2];
      if (val === '' || val === '|' || val === '>') {
        // 数组或缩进块：聚合接下来以空格/制表符开头的行
        const items = [];
        i += 1;
        while (i < lines.length && /^\s+/.test(lines[i])) {
          items.push(lines[i].trim().replace(/^-\s+/, ''));
          i += 1;
        }
        meta[key] = items;
        continue;
      }
      // 普通值，去首尾引号
      val = val.replace(/^['"]|['"]$/g, '').trim();
      meta[key] = val;
    }
    i += 1;
  }
  return {
    name: meta.name || '',
    description: meta.description || '',
    triggers: Array.isArray(meta.triggers) ? meta.triggers : [],
    body: body.trim(),
  };
}

/** 列出所有 skill（不含 body，body 按需单取） */
export function listSkills(season) {
  const now = Date.now();
  if (_cache.data && _cache.at && now - _cache.at < CACHE_TTL && _cache.season === season) {
    return _cache.data;
  }
  const root = join(getSeasonRoot(season), 'novels', '.agent-skills');
  const dirs = listSkillDirs(season);
  const skills = dirs.map((dir) => {
    const abs = join(root, dir, 'SKILL.md');
    try {
      const parsed = parseSkillFile(abs);
      // 路径用仓库相对路径，便于前端 /api/doc 直读
      const rel = `seasons/${season}/novels/.agent-skills/${dir}/SKILL.md`;
      return {
        id: dir,
        dir,
        name: parsed.name || dir,
        description: parsed.description,
        triggers: parsed.triggers,
        rel,
        path: rel,
        // body 也带回来，避免两次 IO；前端要全文时不用再调
        body: parsed.body,
        bytes: Buffer.byteLength(parsed.body, 'utf8'),
      };
    } catch (err) {
      return { id: dir, dir, error: err.message, rel: `seasons/${season}/novels/.agent-skills/${dir}/SKILL.md` };
    }
  });
  _cache = { at: now, data: skills, season };
  return skills;
}

export function getSkill(season, id) {
  return listSkills(season).find((s) => s.id === id) || null;
}

/** 失效缓存（写文件时调用，目前不写但保留口子） */
export function invalidateSkillsCache() {
  _cache = { at: 0, data: null };
}

/**
 * 上下文组装：把选中的 skill + 用户上下文拼成可复制的 prompt
 *
 * 输入：
 *   - skillId: skill 的目录名
 *   - context.previousChapter / context.currentGoal / context.notes:
 *     用户填的上下文变量（自由文本）
 *   - context.characters / context.weapons: 关联的 ID 列表（用于提示）
 *
 * 输出：组装好的 markdown，可一键复制到外部 Agent
 */
export function composeSkillPrompt(season, { skillId, context = {} }) {
  const skill = getSkill(season, skillId);
  if (!skill) throw Object.assign(new Error(`skill 不存在: ${skillId}`), { statusCode: 404 });
  if (skill.error) throw Object.assign(new Error(`skill 解析失败: ${skill.error}`), { statusCode: 500 });

  const lines = [];
  lines.push(`# 调用 Skill：${skill.name}`);
  lines.push('');
  lines.push(`> 自动从 ${skill.path} 生成。`);
  lines.push('');

  // 1. skill 简介 + triggers
  if (skill.description) {
    lines.push(`## 职责`);
    lines.push(skill.description);
    lines.push('');
  }
  if (skill.triggers?.length) {
    lines.push(`## 触发词`);
    for (const t of skill.triggers) lines.push(`- ${t}`);
    lines.push('');
  }

  // 2. 用户上下文
  lines.push(`## 当前上下文（已自动填入）`);
  if (context.previousChapter) {
    lines.push('');
    lines.push(`### 上一章 / 上一节点`);
    lines.push(context.previousChapter);
  }
  if (context.currentGoal) {
    lines.push('');
    lines.push(`### 本章 / 本次目标`);
    lines.push(context.currentGoal);
  }
  if (Array.isArray(context.characters) && context.characters.length) {
    lines.push('');
    lines.push(`### 涉及角色`);
    for (const c of context.characters) lines.push(`- ${c}`);
  }
  if (Array.isArray(context.weapons) && context.weapons.length) {
    lines.push('');
    lines.push(`### 涉及神兵`);
    for (const w of context.weapons) lines.push(`- ${w}`);
  }
  if (context.notes) {
    lines.push('');
    lines.push(`### 备注`);
    lines.push(context.notes);
  }
  lines.push('');

  // 3. skill 正文（保留全部，工作流/红线/模板都给到外部 Agent）
  lines.push(`---`);
  lines.push('');
  lines.push(`## Skill 全文（直接照做即可）`);
  lines.push('');
  lines.push(skill.body);

  return {
    prompt: lines.join('\n'),
    skill: {
      id: skill.id,
      name: skill.name,
      description: skill.description,
      triggers: skill.triggers,
      path: skill.path,
    },
  };
}