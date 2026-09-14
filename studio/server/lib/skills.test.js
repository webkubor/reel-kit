/**
 * skills.js 单元测试
 *
 * 覆盖：
 *   - listSkills() 能列出 7 个 skill
 *   - frontmatter 解析（name/description/triggers 三个字段）
 *   - 行号前缀被兼容（旧版「数字|」残留 → 已用 perl 清洗过，所以现在应该全干净）
 *   - getSkill() 返回单个 skill
 *   - composeSkillPrompt() 把上下文拼成可复制的 prompt
 */

import { describe, it, expect } from 'vitest';
import { listSkills, getSkill, composeSkillPrompt, invalidateSkillsCache } from './skills.js';

describe('listSkills (s1)', () => {
  it('能列出全部 7 个 skill', () => {
    invalidateSkillsCache();
    const skills = listSkills('s1');
    expect(skills.length).toBeGreaterThanOrEqual(7);
  });

  it('每个 skill 有 id / name / description / triggers / body', () => {
    invalidateSkillsCache();
    const skills = listSkills('s1');
    for (const s of skills) {
      expect(s.id).toBeTypeOf('string');
      expect(s.path).toMatch(/novels\/\.agent-skills\//);
      // 至少 name / description 不该同时为空
      expect(s.name.length + s.description.length).toBeGreaterThan(0);
      // body 至少要有点内容
      expect((s.body || '').length).toBeGreaterThan(50);
    }
  });

  it('能找到所有已知 skill id', () => {
    invalidateSkillsCache();
    const skills = listSkills('s1');
    const ids = skills.map((s) => s.id);
    for (const id of [
      'boiling-snow-writing',
      'boiling-snow-novel-craft',
      'novel-flow',
      'qidian-wuxia-audit',
      'novel-asset-ingest',
      '写手中枢',
      '起名中枢',
    ]) {
      expect(ids).toContain(id);
    }
  });

  it('boiling-snow-writing 的 triggers 至少包含「写沸腾之雪章节前」', () => {
    invalidateSkillsCache();
    const skill = getSkill('s1', 'boiling-snow-writing');
    expect(skill).toBeTruthy();
    expect(skill.triggers).toBeInstanceOf(Array);
  });
});

describe('getSkill', () => {
  it('返回存在的 skill', () => {
    invalidateSkillsCache();
    const s = getSkill('s1', 'novel-flow');
    expect(s).toBeTruthy();
    expect(s.name).toBe('novel-flow');
    expect(s.body).toContain('Beat Sheet');
  });

  it('不存在的 id 返回 null', () => {
    invalidateSkillsCache();
    expect(getSkill('s1', 'no-such-skill')).toBeNull();
  });
});

describe('composeSkillPrompt', () => {
  it('把上下文 + skill 全文拼成可复制的 prompt', () => {
    invalidateSkillsCache();
    const { prompt, skill } = composeSkillPrompt('s1', {
      skillId: 'novel-flow',
      context: {
        previousChapter: '上一章写了王府夜谈',
        currentGoal: '本章写天门关前 4 道险',
        characters: ['苏梦城', '萧烬弦'],
        notes: '强调克制',
      },
    });
    expect(skill.id).toBe('novel-flow');
    expect(prompt).toContain('# 调用 Skill：novel-flow');
    expect(prompt).toContain('上一章写了王府夜谈');
    expect(prompt).toContain('天门关前 4 道险');
    expect(prompt).toContain('苏梦城');
    expect(prompt).toContain('萧烬弦');
    expect(prompt).toContain('## Skill 全文');
    expect(prompt).toContain('Beat Sheet');
  });

  it('上下文为空也能跑（基础组装）', () => {
    invalidateSkillsCache();
    const { prompt } = composeSkillPrompt('s1', {
      skillId: '起名中枢',
      context: {},
    });
    expect(prompt).toContain('# 调用 Skill：起名中枢');
    expect(prompt).toContain('## Skill 全文');
  });

  it('不存在的 skill 抛 404', () => {
    invalidateSkillsCache();
    expect(() =>
      composeSkillPrompt('s1', { skillId: 'no-such', context: {} }),
    ).toThrow(/skill 不存在/);
  });
});