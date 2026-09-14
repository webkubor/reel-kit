<script setup>
/**
 * 经典案例库 —— 天榜 Solo 15s 镜头 + 分享
 *
 * 顶部: 全部 / 我的捕获 tab 切换
 * 网格: 每个 case 卡片(角色 + rank + 视觉逻辑)
 * 点击分享: 打开 ShareModal 预览 ClassicCaseCard → 下载/保存
 */
import { computed, onMounted, ref, watch } from 'vue';
import { api } from '../api';
import ClassicCaseCard from '../components/cards/ClassicCaseCard.vue';
import ShareModal from '../components/ShareModal.vue';

const tab = ref('cases'); // 'cases' | 'captures'
const cases = ref([]);
const captures = ref([]);
const showShare = ref(false);
const activeCase = ref(null);
const cardRef = ref(null);

const currentSeasonValue = ref(localStorage.getItem('studio.season') || 's1');

const SEASON_NAMES = { s1: '第一季', s2: '第二季', s3: '第三季' };

async function load() {
  const [cs, caps] = await Promise.all([api.storyboards(), api.captures({ limit: 50 })]);
  cases.value = cs.storyboards
    .filter((c) => c.file !== 'README.md')
    // 排序：序章 → rank 升序
    .sort((a, b) => {
      if (a.isPrologue) return -1;
      if (b.isPrologue) return 1;
      return a.character?.localeCompare(b.character || '') || 0;
    });
  captures.value = caps.captures;
}

onMounted(load);

async function shareCase(c) {
  activeCase.value = c;
  showShare.value = true;
}

function captureUrl(c) {
  return `/api/captures/${c.id}/raw`;
}

async function downloadCapture(c) {
  const res = await fetch(captureUrl(c));
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = c.filename;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

async function deleteCapture(c) {
  if (!confirm(`确认删除 ${c.filename}?`)) return;
  await api.deleteCapture(c.id);
  await load();
}

const activeCaseCard = computed(() => activeCase.value);
const activeCaseMeta = computed(() => {
  if (!activeCase.value) return {};
  return {
    type: 'case',
    title: activeCase.value.title,
    character: activeCase.value.character,
    rank: activeCase.value.rank,
    season: currentSeasonValue.value,
    kind: 'classic-case',
  };
});

const captureTypeLabel = (c) => {
  const t = c.meta?.type;
  if (t === 'case') return '经典案例';
  if (t === 'card') return '剧集海报';
  if (t === 'screenshot') return '截图';
  return t || '其他';
};
</script>

<template>
  <div class="page">
    <header class="topbar">
      <div>
        <h1 class="title-brush topbar-title">经典案例库</h1>
        <span class="topbar-sub">CLASSIC CASES · 8 个天榜 Solo 15s + 你的捕获</span>
      </div>
      <nav class="tabs">
        <button class="tab" :class="{ active: tab === 'cases' }" @click="tab = 'cases'">
          经典案例 <span class="dim mono">({{ cases.length }})</span>
        </button>
        <button class="tab" :class="{ active: tab === 'captures' }" @click="tab = 'captures'">
          我的捕获 <span class="dim mono">({{ captures.length }})</span>
        </button>
      </nav>
    </header>

    <main class="content scroll-y">
      <!-- 经典案例网格 -->
      <div v-if="tab === 'cases'" class="case-grid">
        <article
          v-for="c in cases"
          :key="c.file"
          class="case-card-item"
          @click="shareCase(c)"
        >
          <header class="case-card-head">
            <div :class="['case-card-rank', c.isPrologue && 'rank-prologue']">
              {{ c.isPrologue ? '序' : c.rank || '?' }}
            </div>
            <div class="case-card-name">
              <div class="case-card-character">{{ c.character || '天榜引子' }}</div>
              <div class="case-card-sub dim">{{ c.rankLabel || '序章' }}</div>
            </div>
          </header>
          <blockquote v-if="c.visualLogic" class="case-card-quote">
            {{ c.visualLogic.slice(0, 80) }}{{ c.visualLogic.length > 80 ? '…' : '' }}
          </blockquote>
          <div v-if="c.visualSymbol" class="case-card-symbol">
            【{{ c.visualSymbol }}】
          </div>
          <footer class="case-card-foot">
            <div class="dim mono">{{ c.file.replace('top10_rank', '#').replace('.md', '') }}</div>
            <button class="btn btn-sm btn-primary">📤 分享</button>
          </footer>
        </article>
      </div>

      <!-- 捕获网格 -->
      <div v-else-if="tab === 'captures'" class="capture-grid">
        <div v-if="!captures.length" class="empty-state">
          <div class="empty-icon">📦</div>
          <div class="empty-title">还没有捕获</div>
          <div class="empty-desc dim">在剧集看板或经典案例卡里点 "分享为卡片 / 分享" 即可保存到这里</div>
        </div>
        <article v-for="c in captures" :key="c.id" class="capture-item">
          <a :href="captureUrl(c)" target="_blank" class="capture-thumb">
            <img :src="captureUrl(c)" :alt="c.meta?.title || c.filename" loading="lazy" />
          </a>
          <div class="capture-meta">
            <div class="capture-type">{{ captureTypeLabel(c) }}</div>
            <div class="capture-title">{{ c.meta?.title || c.filename }}</div>
            <div class="capture-info dim mono">
              {{ c.size }} bytes · {{ c.meta?.season || 's1' }} · {{ new Date(c.createdAt).toLocaleString('zh-CN') }}
            </div>
            <div class="capture-actions">
              <button class="btn btn-sm" @click="downloadCapture(c)">⬇ 下载</button>
              <button class="btn btn-sm btn-danger" @click="deleteCapture(c)">删</button>
            </div>
          </div>
        </article>
      </div>
    </main>

    <!-- 分享模态 -->
    <ShareModal
      v-if="activeCaseCard"
      v-model:open="showShare"
      :title="`分享 · ${activeCaseCard.title}`"
      :filename="`case-${activeCaseCard.character || 'prologue'}-${activeCaseCard.rank || '序章'}.png`"
      :meta="activeCaseMeta"
      @saved="load"
    >
      <div class="share-stage">
        <ClassicCaseCard
          ref="cardRef"
          :case="activeCaseCard"
          :season="currentSeasonValue"
          :season-name="SEASON_NAMES[currentSeasonValue] || currentSeasonValue"
        />
      </div>
    </ShareModal>
  </div>
</template>

<style scoped>
.page { display: grid; grid-template-rows: auto 1fr; height: 100%; background: var(--ink-0); }
.topbar { display: flex; justify-content: space-between; align-items: center; padding: var(--sp-4) var(--sp-5); border-bottom: 1px solid var(--line-1); background: var(--ink-1); }
.topbar-title { font-size: 22px; color: var(--gold-1); margin: 0; }
.topbar-sub { font-size: 10px; letter-spacing: 0.22em; color: var(--text-3); }
.tabs { display: flex; gap: 2px; padding: 2px; background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); }
.tab { padding: 5px 14px; font-size: 12px; color: var(--text-2); background: none; border: none; border-radius: var(--r-1); cursor: pointer; }
.tab:hover { color: var(--text-1); }
.tab.active { background: var(--gold-wash); color: var(--gold-1); }

.content { padding: var(--sp-5); }

/* 经典案例网格 */
.case-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: var(--sp-3); }
.case-card-item {
  padding: var(--sp-4);
  background: var(--ink-1);
  border: 1px solid var(--line-1);
  border-left: 3px solid var(--gold-3);
  border-radius: var(--r-3);
  cursor: pointer;
  transition: all 0.15s;
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
}
.case-card-item:hover { border-color: var(--line-gold); border-left-color: var(--gold-2); background: var(--ink-2); transform: translateY(-2px); }
.case-card-head { display: flex; align-items: center; gap: var(--sp-3); }
.case-card-rank {
  width: 56px; height: 56px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  background: var(--gold-wash);
  border: 1px solid var(--line-gold);
  border-radius: 50%;
  font-size: 22px; color: var(--gold-1); font-weight: 700;
  font-family: var(--font-title);
}
.case-card-rank.rank-prologue { background: var(--crimson-wash); border-color: var(--crimson); color: var(--crimson); }
.case-card-name { flex: 1; min-width: 0; }
.case-card-character { font-size: 18px; color: var(--gold-1); font-family: var(--font-title); letter-spacing: 0.08em; }
.case-card-sub { font-size: 10px; letter-spacing: 0.15em; }
.case-card-quote {
  margin: 0;
  padding: 8px 12px;
  background: var(--ink-2);
  border-left: 2px solid var(--ink-4);
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-2);
  font-style: italic;
}
.case-card-symbol { font-size: 24px; color: var(--gold-1); text-align: center; font-family: var(--font-title); padding: 8px 0; }
.case-card-foot { display: flex; justify-content: space-between; align-items: center; padding-top: 8px; border-top: 1px solid var(--line-1); }

/* 捕获网格 */
.capture-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: var(--sp-3); }
.capture-item { background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-3); overflow: hidden; }
.capture-thumb { display: block; background: var(--ink-0); }
.capture-thumb img { display: block; width: 100%; height: 180px; object-fit: cover; }
.capture-meta { padding: var(--sp-3); }
.capture-type { font-size: 9px; color: var(--gold-2); letter-spacing: 0.2em; margin-bottom: 4px; }
.capture-title { font-size: 13px; color: var(--text-1); margin-bottom: 4px; line-height: 1.4; }
.capture-info { font-size: 10px; margin-bottom: 8px; word-break: break-all; }
.capture-actions { display: flex; gap: 4px; }
.empty-state { text-align: center; padding: 80px var(--sp-5); }
.empty-icon { font-size: 48px; margin-bottom: var(--sp-3); opacity: 0.5; }
.empty-title { font-size: 18px; color: var(--text-2); margin-bottom: var(--sp-2); font-family: var(--font-title); }
.empty-desc { font-size: 12px; }

.share-stage { transform: scale(1); transform-origin: top center; }
</style>
