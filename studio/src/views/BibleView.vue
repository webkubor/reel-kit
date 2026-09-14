<script setup>
/**
 * 创意法典 + Prompt 实验室
 *
 * 7 大节法典 + 旁白文案 + 镜头美学 SKILL。
 * 首版以"只读 + 可复制"为主，等 Prompt 实验室成型再开放输入区。
 */
import { computed, onMounted, ref } from 'vue';
import { api } from '../api';

const bible = ref(null);
const voiceover = ref(null);
const skill = ref(null);
const tab = ref('bible');
const copied = ref(null);

async function load() {
  const [b, v, s] = await Promise.all([api.bible(), api.voiceover(), api.cameraSkill()]);
  bible.value = b.bible;
  voiceover.value = v.voiceover;
  skill.value = s.cameraSkill;
}

async function copy(text, id) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = id;
    setTimeout(() => {
      if (copied.value === id) copied.value = null;
    }, 1500);
  } catch { /* */ }
}

const tabs = computed(() => [
  { id: 'bible', label: '创意法典', count: bible.value?.sections?.length || 0 },
  { id: 'voiceover', label: '旁白文案', count: voiceover.value?.blocks?.length || 0 },
  { id: 'skill', label: '镜头美学', count: null },
]);

onMounted(load);
</script>

<template>
  <div class="page">
    <header class="topbar">
      <div>
        <h1 class="title-brush topbar-title">创意法典</h1>
        <span class="topbar-sub">CREATIVE BIBLE · 创作红线与审美沉淀</span>
      </div>
      <nav class="tabs">
        <button
          v-for="t in tabs"
          :key="t.id"
          class="tab"
          :class="{ active: tab === t.id }"
          @click="tab = t.id"
        >{{ t.label }} <span class="dim mono" v-if="t.count !== null">({{ t.count }})</span></button>
      </nav>
    </header>

    <main class="content scroll-y">
      <!-- 创意法典 -->
      <section v-if="tab === 'bible' && bible" class="bible">
        <article v-for="s in bible.sections" :key="s.key" class="section">
          <h2 class="section-title title-brush">{{ s.title }}</h2>
          <pre v-if="s.body" class="section-body mono">{{ s.body }}</pre>
          <p v-else class="empty">本节暂未填充</p>
        </article>
      </section>

      <!-- 旁白 -->
      <section v-else-if="tab === 'voiceover' && voiceover" class="vo">
        <article v-for="(b, i) in voiceover.blocks" :key="i" class="vo-card">
          <header class="vo-head">
            <span class="vo-num">#{{ String(i + 1).padStart(2, '0') }}</span>
            <h3 class="vo-char title-brush">{{ b.character }}</h3>
            <span class="tag tag-gold">{{ b.duration }}</span>
            <button class="btn btn-sm" @click="copy(b.body, `v-${i}`)">
              {{ copied === `v-${i}` ? '已复制' : '复制文案' }}
            </button>
          </header>
          <pre class="vo-body mono">{{ b.body }}</pre>
          <p v-if="b.slogan" class="vo-slogan">"{{ b.slogan }}"</p>
        </article>
        <p v-if="!voiceover.blocks?.length" class="empty">无旁白存档</p>
      </section>

      <!-- 镜头美学 -->
      <section v-else-if="tab === 'skill' && skill" class="skill">
        <article v-for="(v, k) in { intro: skill.intro, flow: skill.flow, output: skill.output, mapping: skill.mapping, quick: skill.quick, examples: skill.examples }" :key="k" class="section">
          <h2 class="section-title title-brush">{{ ({ intro: '核心定位', flow: '交互流程', output: '输出格式', mapping: '偏好→术语映射', quick: '快速推荐', examples: '使用示例' })[k] }}</h2>
          <pre v-if="v" class="section-body mono">{{ v }}</pre>
          <p v-else class="empty">本节暂未填充</p>
        </article>
        <p v-if="!skill" class="empty">加载失败</p>
      </section>

      <p v-else class="empty">加载中…</p>
    </main>
  </div>
</template>

<style scoped>
.page { display: grid; grid-template-rows: auto 1fr; height: 100%; background: var(--ink-0); }
.topbar { display: flex; justify-content: space-between; align-items: center; padding: var(--sp-4) var(--sp-5); border-bottom: 1px solid var(--line-1); background: var(--ink-1); }
.topbar-title { font-size: 22px; color: var(--gold-1); margin: 0; }
.topbar-sub { font-size: 10px; letter-spacing: 0.22em; color: var(--text-3); }
.tabs { display: flex; gap: 2px; padding: 2px; background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); }
.tab { padding: 5px 12px; font-size: 12px; color: var(--text-2); background: none; border: none; border-radius: var(--r-1); cursor: pointer; }
.tab:hover { color: var(--text-1); }
.tab.active { background: var(--gold-wash); color: var(--gold-1); }

.content { padding: var(--sp-5); }
.bible, .vo, .skill { display: flex; flex-direction: column; gap: var(--sp-5); max-width: 920px; }

.section { background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-3); padding: var(--sp-5); }
.section-title { font-size: 18px; color: var(--gold-1); margin: 0 0 var(--sp-3); padding-bottom: var(--sp-2); border-bottom: 1px solid var(--line-gold); }
.section-body { white-space: pre-wrap; word-break: break-word; font-size: 12px; line-height: 1.7; color: var(--text-2); margin: 0; }

.vo-card { background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-3); padding: var(--sp-4); position: relative; }
.vo-head { display: flex; align-items: center; gap: var(--sp-3); margin-bottom: var(--sp-3); }
.vo-num { font-family: var(--font-mono); font-size: 11px; color: var(--gold-2); letter-spacing: 0.15em; }
.vo-char { font-size: 20px; color: var(--gold-1); margin: 0; flex: 1; }
.vo-body { white-space: pre-wrap; word-break: break-word; font-size: 13px; line-height: 1.8; color: var(--text-1); margin: 0; padding: var(--sp-3); background: var(--ink-0); border-left: 2px solid var(--gold-3); border-radius: 0 var(--r-2) var(--r-2) 0; }
.vo-slogan { margin: var(--sp-3) 0 0; padding: var(--sp-3); text-align: center; font-size: 14px; color: var(--gold-1); font-style: italic; }

.empty { color: var(--text-3); text-align: center; padding: var(--sp-5); }
</style>
