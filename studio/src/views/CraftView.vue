<script setup>
/**
 * Skill 创作中心 —— 小说 → 剧集 → 分镜 → Prompt 链路打通
 *
 * 数据源：seasons/{season}/novels/.agent-skills/ 下的 7 个 skill
 *   - boiling-snow-writing   项目总原则
 *   - boiling-snow-novel-craft  单章构思
 *   - novel-flow             6 阶段流水线
 *   - qidian-wuxia-audit     起点武侠专项审核
 *   - novel-asset-ingest     资料入库
 *   - 写手中枢              文笔优化
 *   - 起名中枢              命名
 *
 * 两个模式：
 *   1. 浏览（默认）—— 看 skill 正文
 *   2. 组装 —— 选 skill + 填上下文 → 一键复制到外部 Agent
 */
import { computed, onMounted, ref, watch } from 'vue';
import { api } from '../api';

const skills = ref([]);
const selectedId = ref(null);
const selected = ref(null);
const mode = ref('browse'); // 'browse' | 'compose'
const loading = ref(false);
const error = ref('');
const copied = ref(null);

// 组装上下文表单
const ctx = ref({
  previousChapter: '',
  currentGoal: '',
  characters: '',
  weapons: '',
  notes: '',
});
const composed = ref('');

async function loadList() {
  loading.value = true;
  error.value = '';
  try {
    const r = await api.skills();
    skills.value = r.skills;
    if (!selectedId.value && skills.value.length) {
      selectedId.value = skills.value[0].id;
    }
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

async function loadDetail(id) {
  if (!id) { selected.value = null; return; }
  loading.value = true;
  error.value = '';
  try {
    const r = await api.skill(id);
    selected.value = r.skill;
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

watch(selectedId, (id) => {
  if (mode.value === 'browse') loadDetail(id);
});

watch(mode, async (m) => {
  if (m === 'browse' && selectedId.value) {
    await loadDetail(selectedId.value);
  } else if (m === 'compose') {
    await doCompose();
  }
});

async function doCompose() {
  if (!selectedId.value) return;
  loading.value = true;
  error.value = '';
  try {
    const r = await api.composeSkill(selectedId.value, {
      previousChapter: ctx.value.previousChapter.trim(),
      currentGoal: ctx.value.currentGoal.trim(),
      notes: ctx.value.notes.trim(),
      characters: ctx.value.characters
        .split(/[,，、\s]+/).map((s) => s.trim()).filter(Boolean),
      weapons: ctx.value.weapons
        .split(/[,，、\s]+/).map((s) => s.trim()).filter(Boolean),
    });
    composed.value = r.prompt;
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

async function copy(text, key) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = key;
    setTimeout(() => { if (copied.value === key) copied.value = null; }, 1500);
  } catch { /* */ }
}

// 简单把 markdown 渲染成预览（不引第三方）
function renderMarkdown(md) {
  if (!md) return '';
  // 转义
  const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  let html = esc(md);
  // 标题
  html = html.replace(/^######\s+(.+)$/gm, '<h6>$1</h6>')
             .replace(/^#####\s+(.+)$/gm, '<h6>$1</h6>')
             .replace(/^####\s+(.+)$/gm, '<h5>$1</h5>')
             .replace(/^###\s+(.+)$/gm, '<h4>$1</h4>')
             .replace(/^##\s+(.+)$/gm, '<h3>$1</h3>')
             .replace(/^#\s+(.+)$/gm, '<h2>$1</h2>');
  // 代码块
  html = html.replace(/```([\s\S]*?)```/g, (_, code) => `<pre class="md-pre"><code>${code}</code></pre>`);
  // 行内代码
  html = html.replace(/`([^`]+)`/g, '<code class="md-code">$1</code>');
  // 加粗
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  // 列表
  html = html.replace(/^(\s*)[-*]\s+(.+)$/gm, '$1<li>$2</li>');
  html = html.replace(/(<li>[\s\S]*?<\/li>\n?)+/g, (m) => `<ul class="md-ul">${m}</ul>`);
  // 引用
  html = html.replace(/^&gt;\s*(.+)$/gm, '<blockquote class="md-quote">$1</blockquote>');
  // 段落：双换行
  html = html.split(/\n{2,}/).map((block) => {
    if (/^<(h\d|ul|pre|blockquote|li)/.test(block.trim())) return block;
    return `<p>${block.replace(/\n/g, '<br/>')}</p>`;
  }).join('\n');
  return html;
}

const renderedBody = computed(() => renderMarkdown(selected.value?.body || ''));
const renderedComposed = computed(() => renderMarkdown(composed.value));

function formatBytes(n) {
  if (!n) return '0';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(2)} MB`;
}

const selectedTriggers = computed(() => selected.value?.triggers || []);

onMounted(loadList);
</script>

<template>
  <div class="page">
    <header class="topbar">
      <div>
        <h1 class="title-brush topbar-title">Skill 创作中心</h1>
        <span class="topbar-sub">NOVEL → EPISODE → STORYBOARD · 7 个 Agent skill 一键调用</span>
      </div>
      <nav class="tabs">
        <button class="tab" :class="{ active: mode === 'browse' }" @click="mode = 'browse'">浏览 Skill</button>
        <button class="tab" :class="{ active: mode === 'compose' }" @click="mode = 'compose'">组装上下文</button>
      </nav>
    </header>

    <div class="layout">
      <!-- 左：skill 列表 -->
      <aside class="left">
        <div class="left-head">
          <span class="dim mono">SKILLS</span>
          <span class="dim mono">{{ skills.length }}</span>
        </div>
        <button
          v-for="s in skills"
          :key="s.id"
          class="skill-card"
          :class="{ active: selectedId === s.id }"
          @click="selectedId = s.id"
        >
          <div class="skill-card-head">
            <span class="skill-card-name title-brush">{{ s.name }}</span>
            <span class="dim mono skill-card-bytes">{{ formatBytes(s.bytes) }}</span>
          </div>
          <div class="skill-card-desc">{{ s.description || '（无描述）' }}</div>
          <div v-if="s.triggers?.length" class="skill-card-triggers">
            <span v-for="(t, i) in s.triggers.slice(0, 3)" :key="i" class="trigger">{{ t }}</span>
          </div>
        </button>
      </aside>

      <!-- 右：详情 / 组装 -->
      <main class="right scroll-y">
        <div v-if="error" class="banner banner-error">⚠️ {{ error }}</div>

        <!-- 模式 1：浏览 -->
        <template v-if="mode === 'browse' && selected">
          <div class="head-card">
            <div class="head-card-top">
              <h2 class="title-brush">{{ selected.name }}</h2>
              <button class="btn btn-sm" @click="copy(`# ${selected.name}\n\n${selected.body}`, 'body')">
                {{ copied === 'body' ? '已复制' : '复制全文' }}
              </button>
            </div>
            <p class="head-desc">{{ selected.description }}</p>
            <div v-if="selectedTriggers.length" class="head-triggers">
              <span class="dim mono">触发词：</span>
              <span v-for="(t, i) in selectedTriggers" :key="i" class="trigger">{{ t }}</span>
            </div>
            <div class="head-meta dim mono">
              {{ selected.path }} · {{ formatBytes(selected.bytes) }}
            </div>
          </div>
          <article class="markdown" v-html="renderedBody" />
        </template>

        <!-- 模式 2：组装上下文 -->
        <template v-else-if="mode === 'compose' && selected">
          <div class="head-card">
            <div class="head-card-top">
              <h2 class="title-brush">组装：{{ selected.name }}</h2>
              <span class="dim mono">复制后可直接粘到外部 Agent</span>
            </div>
            <p class="head-desc">{{ selected.description }}</p>
          </div>

          <form class="form" @submit.prevent="doCompose">
            <label class="field">
              <span class="field-label">上一章 / 上一节点 <span class="dim">（可空）</span></span>
              <textarea v-model="ctx.previousChapter" rows="3" placeholder="例：上一章写了王府夜谈，苏梦城拒了皇帝的赏赐..." />
            </label>
            <label class="field">
              <span class="field-label">本章 / 本次目标 <span class="dim">（必填）</span></span>
              <textarea v-model="ctx.currentGoal" rows="3" placeholder="例：写天门关前 4 道险的第三险，王爷压阵" />
            </label>
            <div class="field-row">
              <label class="field">
                <span class="field-label">涉及角色 <span class="dim">（逗号分隔）</span></span>
                <input v-model="ctx.characters" placeholder="苏梦城, 萧烬弦" />
              </label>
              <label class="field">
                <span class="field-label">涉及神兵 <span class="dim">（逗号分隔）</span></span>
                <input v-model="ctx.weapons" placeholder="惊鸿, 乌麟" />
              </label>
            </div>
            <label class="field">
              <span class="field-label">备注 <span class="dim">（可空）</span></span>
              <textarea v-model="ctx.notes" rows="2" placeholder="强调克制；不许光球；BGM 偏箫声..." />
            </label>
            <div class="form-actions">
              <button type="submit" class="btn btn-primary" :disabled="loading || !ctx.currentGoal.trim()">
                {{ loading ? '生成中…' : '生成 Prompt' }}
              </button>
            </div>
          </form>

          <div v-if="composed" class="output">
            <div class="output-head">
              <span class="dim mono">OUTPUT · 组装后的 Prompt（可直接粘到 Claude / 外部 Agent）</span>
              <button class="btn btn-sm" @click="copy(composed, 'composed')">
                {{ copied === 'composed' ? '已复制' : '复制全文' }}
              </button>
            </div>
            <article class="markdown markdown-output" v-html="renderedComposed" />
          </div>
        </template>

        <div v-if="!selected && !error" class="empty">选一个 Skill 开始浏览或组装</div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--sp-4) var(--sp-5);
  border-bottom: 1px solid var(--line-1);
  background: var(--ink-1);
}
.topbar-title { font-size: 22px; line-height: 1.1; }
.topbar-sub { color: var(--text-3); font-size: 10px; letter-spacing: 0.22em; }

.tabs { display: flex; gap: 4px; }
.tab {
  padding: 6px 12px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-2);
  font-family: inherit;
  font-size: 12px;
  cursor: pointer;
}
.tab:hover { border-color: var(--line-gold); color: var(--text-1); }
.tab.active { background: var(--gold-wash); border-color: var(--gold-2); color: var(--gold-1); }

.layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  flex: 1;
  min-height: 0;
}

.left {
  border-right: 1px solid var(--line-1);
  background: var(--ink-2);
  overflow-y: auto;
  padding: var(--sp-3);
}
.left-head {
  display: flex;
  justify-content: space-between;
  padding: 0 4px var(--sp-3);
  font-size: 9px;
  letter-spacing: 0.22em;
}

.skill-card {
  display: block;
  width: 100%;
  text-align: left;
  margin-bottom: 6px;
  padding: 10px 12px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-2);
  background: var(--ink-1);
  color: var(--text-1);
  font-family: inherit;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.skill-card:hover { border-color: var(--line-gold); background: var(--ink-2); }
.skill-card.active { border-color: var(--gold-2); background: var(--gold-wash); }

.skill-card-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 4px;
}
.skill-card-name { font-size: 13px; }
.skill-card-bytes { font-size: 10px; }
.skill-card-desc {
  color: var(--text-2);
  font-size: 11px;
  line-height: 1.45;
  margin-bottom: 6px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.skill-card-triggers { display: flex; flex-wrap: wrap; gap: 4px; }

.trigger {
  padding: 1px 6px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-2);
  font-size: 10px;
}

.right { padding: var(--sp-5); }

.banner {
  padding: 10px 14px;
  border-radius: var(--r-2);
  margin-bottom: var(--sp-4);
}
.banner-error {
  background: rgba(176, 32, 40, 0.12);
  border: 1px solid var(--crimson);
  color: var(--crimson);
}

.head-card {
  padding: var(--sp-4);
  margin-bottom: var(--sp-4);
  border: 1px solid var(--line-gold);
  border-radius: var(--r-2);
  background: var(--ink-2);
}
.head-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.head-desc { color: var(--text-2); font-size: 13px; line-height: 1.6; margin-bottom: 8px; }
.head-triggers { display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 8px; }
.head-meta { font-size: 10px; }

.form { display: flex; flex-direction: column; gap: 14px; margin-bottom: var(--sp-5); }
.field { display: flex; flex-direction: column; gap: 4px; }
.field-label { font-size: 11px; color: var(--gold-2); letter-spacing: 0.1em; }
.field input, .field textarea {
  padding: 8px 10px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-1);
  font-family: inherit;
  font-size: 13px;
  resize: vertical;
}
.field input:focus, .field textarea:focus {
  outline: none;
  border-color: var(--gold-2);
}
.field-row { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.form-actions { display: flex; gap: 8px; }

.btn {
  padding: 6px 14px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-1);
  font-family: inherit;
  font-size: 12px;
  cursor: pointer;
}
.btn:hover { border-color: var(--line-gold); }
.btn-sm { padding: 4px 10px; font-size: 11px; }
.btn-primary { background: var(--gold-2); border-color: var(--gold-2); color: var(--ink-1); font-weight: 600; }
.btn-primary:hover { background: var(--gold-1); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }

.output {
  border-top: 1px solid var(--line-1);
  padding-top: var(--sp-4);
}
.output-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--sp-3);
  font-size: 10px;
  letter-spacing: 0.2em;
}

.markdown {
  font-size: 13px;
  line-height: 1.7;
  color: var(--text-1);
}
.markdown :deep(h2) {
  margin: 1.4em 0 0.4em;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--line-1);
  font-size: 16px;
}
.markdown :deep(h3) { margin: 1em 0 0.4em; font-size: 14px; color: var(--gold-2); }
.markdown :deep(h4) { margin: 0.8em 0 0.3em; font-size: 13px; color: var(--text-1); }
.markdown :deep(p) { margin: 0.6em 0; }
.markdown :deep(ul.md-ul) { margin: 0.6em 0; padding-left: 1.4em; }
.markdown :deep(li) { margin: 0.2em 0; }
.markdown :deep(strong) { color: var(--gold-1); }
.markdown :deep(code.md-code) {
  padding: 1px 5px;
  background: var(--ink-3);
  border-radius: 3px;
  font-size: 12px;
}
.markdown :deep(pre.md-pre) {
  padding: 10px 12px;
  background: var(--ink-3);
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  overflow-x: auto;
  font-size: 12px;
  line-height: 1.5;
}
.markdown :deep(blockquote.md-quote) {
  margin: 0.6em 0;
  padding: 6px 14px;
  border-left: 3px solid var(--gold-2);
  background: var(--gold-wash);
  color: var(--text-2);
}

.markdown-output {
  padding: var(--sp-4);
  background: var(--ink-2);
  border: 1px solid var(--line-1);
  border-radius: var(--r-2);
}

.empty {
  padding: var(--sp-6);
  text-align: center;
  color: var(--text-3);
  font-size: 12px;
}
</style>