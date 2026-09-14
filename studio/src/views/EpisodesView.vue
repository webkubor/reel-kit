<script setup>
/**
 * 剧集 / 分镜进度看板（旗舰）
 *
 * 14 集 ep00-ep13 全部数据，按 kind 决定详情渲染策略。
 * 顶部状态分组、左侧列表、详情区、右侧 JImeng 指令侧栏。
 */
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import { api } from '../api';
import EpisodeCard from '../components/cards/EpisodeCard.vue';
import ShareModal from '../components/ShareModal.vue';

const episodes = ref([]);
const selected = ref(null);
const detail = ref(null);
const filterStatus = ref('all');
const filterKind = ref('all');
const keyword = ref('');
const copied = ref(null);
const showShare = ref(false);
const cardRef = ref(null);
const currentSeasonValue = ref(localStorage.getItem('studio.season') || 's1');

const KIND_LABEL = {
  prologue: '序幕',
  'structured-15s': '15s 结构',
  'detailed-scenes': '场景分镜',
  outline: '剧情大纲',
  storyboard: '5 段分镜',
  pending: '待开发',
};

const KIND_ORDER = ['prologue', 'structured-15s', 'detailed-scenes', 'outline', 'storyboard', 'pending'];

const STATUS_ORDER = ['finalized', 'published', 'completed', 'pending'];

const STATUS_LABEL = {
  finalized: '已定稿',
  published: '已发布',
  completed: '已完结',
  pending: '待开发',
};

const filtered = computed(() => {
  let list = episodes.value;
  if (filterStatus.value !== 'all') list = list.filter((e) => e.status === filterStatus.value);
  if (filterKind.value !== 'all') list = list.filter((e) => e.kind === filterKind.value);
  const kw = keyword.value.trim();
  if (kw) {
    list = list.filter(
      (e) =>
        e.title?.includes(kw) ||
        e.core?.includes(kw) ||
        e.slug?.includes(kw) ||
        String(e.ep).includes(kw),
    );
  }
  return list;
});

const groupedByStatus = computed(() => {
  const groups = {};
  for (const ep of filtered.value) {
    const key = ep.status || 'pending';
    if (!groups[key]) groups[key] = [];
    groups[key].push(ep);
  }
  return STATUS_ORDER.filter((s) => groups[s]?.length).map((s) => ({
    status: s,
    statusLabel: STATUS_LABEL[s],
    eps: groups[s],
  }));
});

const totals = computed(() => ({
  total: episodes.value.length,
  shots: episodes.value.reduce((n, e) => n + (e.shotCount || 0), 0),
  jimeng: episodes.value.reduce((n, e) => n + (e.jimengCount || 0), 0),
  voiceover: episodes.value.reduce((n, e) => n + (e.voiceoverCount || 0), 0),
  finalized: episodes.value.filter((e) => e.status === 'finalized').length,
  pending: episodes.value.filter((e) => e.status === 'pending').length,
}));

async function load() {
  const data = await api.episodes();
  episodes.value = data.episodes;
  if (!selected.value && episodes.value.length) select(episodes.value.find((e) => e.exists)?.ep ?? 0);
}

async function select(n) {
  selected.value = n;
  const data = await api.episode(n);
  detail.value = data.ep;
  copied.value = null;
}

function selectBySlug(ep) {
  select(ep.ep);
}

async function copy(text, id) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = id;
    setTimeout(() => {
      if (copied.value === id) copied.value = null;
    }, 1500);
  } catch {
    /* clipboard 不一定可用 */
  }
}

function fmtTime(ms) {
  if (!ms) return '—';
  const d = new Date(ms);
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

watch(selected, (n) => {
  if (n !== null && detail.value?.ep !== n) select(n);
});

onMounted(load);
</script>

<template>
  <div class="page">
    <!-- 顶部统计条 -->
    <header class="topbar">
      <div class="topbar-left">
        <h1 class="title-brush topbar-title">剧集看板</h1>
        <span class="topbar-sub">EPISODES · 14 集从序幕到终章</span>
      </div>
      <div class="topbar-stats">
        <div class="stat"><span class="stat-num">{{ totals.total }}</span><span class="stat-label">集</span></div>
        <div class="stat"><span class="stat-num">{{ totals.shots }}</span><span class="stat-label">分镜</span></div>
        <div class="stat"><span class="stat-num">{{ totals.jimeng }}</span><span class="stat-label">指令</span></div>
        <div class="stat"><span class="stat-num">{{ totals.voiceover }}</span><span class="stat-label">对白</span></div>
        <div class="stat stat-finalized"><span class="stat-num">{{ totals.finalized }}</span><span class="stat-label">定稿</span></div>
        <div class="stat stat-pending"><span class="stat-num">{{ totals.pending }}</span><span class="stat-label">待开发</span></div>
      </div>
    </header>

    <!-- 工具条 -->
    <div class="toolbar">
      <input v-model="keyword" class="input toolbar-search" placeholder="搜剧名 / 集号 / 核心看点" />
      <div class="toolbar-filter">
        <button
          v-for="s in [{ id: 'all', label: '全部' }, { id: 'finalized', label: '定稿' }, { id: 'published', label: '已发' }, { id: 'completed', label: '完结' }, { id: 'pending', label: '待开' }]"
          :key="s.id"
          class="chip"
          :class="{ active: filterStatus === s.id }"
          @click="filterStatus = s.id"
        >{{ s.label }}</button>
      </div>
      <div class="toolbar-filter">
        <button
          v-for="k in [{ id: 'all', label: '所有格式' }, ...KIND_ORDER.filter((k) => k !== 'pending').map((k) => ({ id: k, label: KIND_LABEL[k] }))]"
          :key="k.id"
          class="chip"
          :class="{ active: filterKind === k.id }"
          @click="filterKind = k.id"
        >{{ k.label }}</button>
      </div>
    </div>

    <!-- 主体：左侧列表 + 详情 -->
    <div class="body">
      <aside class="list">
        <div v-for="group in groupedByStatus" :key="group.status" class="group">
          <div class="group-head">
            <span :class="['group-dot', `dot-${group.status}`]" />
            <span class="group-name">{{ group.statusLabel }}</span>
            <span class="group-count">{{ group.eps.length }}</span>
          </div>
          <button
            v-for="ep in group.eps"
            :key="ep.ep"
            class="ep-item"
            :class="{ active: ep.ep === selected }"
            @click="selectBySlug(ep)"
          >
            <div class="ep-num">{{ String(ep.ep).padStart(2, '0') }}</div>
            <div class="ep-main">
              <div class="ep-title">{{ ep.title }}</div>
              <div class="ep-core dim">{{ ep.core || '—' }}</div>
              <div class="ep-meta">
                <span class="tag tag-sm">{{ KIND_LABEL[ep.kind] || ep.kind }}</span>
                <span v-if="ep.shotCount" class="tag tag-sm tag-gold">{{ ep.shotCount }} 镜</span>
                <span v-if="ep.jimengCount" class="tag tag-sm tag-azure">{{ ep.jimengCount }} 指令</span>
                <span v-if="ep.voiceoverCount" class="tag tag-sm tag-crimson">{{ ep.voiceoverCount }} 对白</span>
              </div>
            </div>
          </button>
        </div>
      </aside>

      <main v-if="detail" class="detail">
        <header class="detail-head">
          <div class="detail-num">第 {{ String(detail.ep).padStart(2, '0') }} 集</div>
          <div class="detail-head-row">
            <h2 class="detail-title title-brush">{{ detail.title }}</h2>
            <button class="btn btn-primary share-btn" @click="showShare = true">
              📤 分享为卡片
            </button>
          </div>
          <div class="detail-tags">
            <span class="tag tag-gold">{{ detail.statusLabel }}</span>
            <span class="tag">{{ KIND_LABEL[detail.kind] || detail.kind }}</span>
            <span v-if="detail.updatedAt" class="dim mono">更新 {{ fmtTime(detail.updatedAt) }}</span>
          </div>
          <p v-if="detail.core" class="detail-core">{{ detail.core }}</p>
        </header>

        <!-- 序幕 -->
        <section v-if="detail.kind === 'prologue' && detail.prologueParts?.length" class="block">
          <h3 class="block-title">序幕结构</h3>
          <div class="prologue-grid">
            <article v-for="(p, i) in detail.prologueParts" :key="i" class="prologue-card">
              <div class="prologue-step">{{ String(i + 1).padStart(2, '0') }}</div>
              <h4 class="prologue-head">{{ p.title }}</h4>
              <p class="prologue-body">{{ p.body }}</p>
            </article>
          </div>
        </section>

        <!-- 15s 结构（幕+镜头） -->
        <section v-else-if="detail.kind === 'structured-15s' && detail.acts?.length" class="block">
          <h3 class="block-title">幕 · 镜头</h3>
          <div v-for="(act, ai) in detail.acts" :key="ai" class="act">
            <div class="act-head">
              <span class="act-label">{{ act.title }}</span>
              <span v-if="act.shots?.length" class="dim">{{ act.shots.length }} 镜</span>
            </div>
            <div class="shots-grid">
              <article v-for="shot in act.shots" :key="shot.index" class="shot-card">
                <div class="shot-num">{{ String(shot.index).padStart(2, '0') }}</div>
                <h5 class="shot-title">{{ shot.title }}</h5>
              </article>
            </div>
          </div>
        </section>

        <!-- 详细场景分镜 -->
        <section v-else-if="detail.kind === 'detailed-scenes' && detail.scenes?.length" class="block">
          <h3 class="block-title">场景分镜 · {{ detail.scenes.length }} 场</h3>
          <div class="scenes">
            <article v-for="scene in detail.scenes" :key="scene.index" class="scene">
              <header class="scene-head">
                <span class="scene-num">场景 {{ String(scene.index).padStart(2, '0') }}</span>
                <h4 class="scene-title">{{ scene.title }}</h4>
              </header>
              <dl class="scene-meta">
                <div v-if="scene.time"><dt>时间</dt><dd>{{ scene.time }}</dd></div>
                <div v-if="scene.location"><dt>地点</dt><dd>{{ scene.location }}</dd></div>
                <div v-if="scene.characters"><dt>人物</dt><dd>{{ scene.characters }}</dd></div>
              </dl>
              <details class="scene-body-wrap">
                <summary class="dim">展开场景内容</summary>
                <pre class="scene-body mono">{{ scene.body }}</pre>
              </details>
            </article>
          </div>
        </section>

        <!-- 剧情大纲 -->
        <section v-else-if="detail.kind === 'outline'" class="block">
          <h3 class="block-title">剧情大纲</h3>
          <p class="dim">本集目前为短剧情大纲，尚未展开分镜。</p>
          <pre v-if="detail.raw" class="raw-md mono scroll-y">{{ detail.raw }}</pre>
        </section>

        <!-- 默认：原始 markdown -->
        <section v-else class="block">
          <h3 class="block-title">剧本原文</h3>
          <pre class="raw-md mono scroll-y">{{ detail.raw }}</pre>
        </section>

        <!-- 引用 -->
        <section v-if="detail.refs?.length" class="block">
          <h3 class="block-title">引用真源</h3>
          <div class="refs">
            <a v-for="(r, i) in detail.refs" :key="i" class="ref-link" :href="r.href" target="_blank">
              <span class="ref-icon">↗</span>
              <span class="ref-label">{{ r.label }}</span>
              <span class="ref-href dim mono">{{ r.href }}</span>
            </a>
          </div>
        </section>
      </main>

      <main v-else class="empty detail-empty">选一集查看</main>

      <!-- 分享模态：海报卡片预览 + 下载 + 保存到 .state -->
      <ShareModal
        v-if="detail"
        v-model:open="showShare"
        :title="`分享 · ${detail.title}`"
        :filename="`ep${String(detail.ep).padStart(2, '0')}-${detail.title.replace(/[《》「」『』\s\/\\]/g, '-')}.png`"
        :meta="{ type: 'card', title: detail.title, ep: detail.ep, season: currentSeasonValue, kind: 'episode-poster' }"
      >
        <div class="share-stage">
          <EpisodeCard ref="cardRef" :ep="detail" :season="currentSeasonValue" />
        </div>
      </ShareModal>

      <!-- 右侧：指令/对白/音频侧栏 -->
      <aside v-if="detail" class="rail">
        <div class="rail-section">
          <h3 class="rail-title">Jimeng 指令 <span class="dim">({{ detail.jimeng?.length || 0 }})</span></h3>
          <div v-if="!detail.jimeng?.length" class="rail-empty dim">无</div>
          <article v-for="(p, i) in detail.jimeng" :key="i" class="jimeng-card">
            <header class="jimeng-head">
              <span class="jimeng-num">#{{ i + 1 }}</span>
              <button class="btn btn-sm jimeng-copy" @click="copy(p, `j-${i}`)">
                {{ copied === `j-${i}` ? '已复制' : '复制' }}
              </button>
            </header>
            <pre class="jimeng-body mono">{{ p }}</pre>
          </article>
        </div>

        <div class="rail-section">
          <h3 class="rail-title">对白 <span class="dim">({{ detail.voiceover?.length || 0 }})</span></h3>
          <div v-if="!detail.voiceover?.length" class="rail-empty dim">无</div>
          <div v-for="(v, i) in detail.voiceover" :key="i" class="vo">
            <div class="vo-tone dim mono">{{ v.tone }}</div>
            <blockquote class="vo-text">"{{ v.text }}"</blockquote>
          </div>
        </div>

        <div class="rail-section">
          <h3 class="rail-title">音频 <span class="dim">({{ detail.audio?.length || 0 }})</span></h3>
          <div v-if="!detail.audio?.length" class="rail-empty dim">无</div>
          <div v-for="(a, i) in detail.audio" :key="i" class="audio-line mono dim">♪ {{ a }}</div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.page {
  display: grid;
  height: 100%;
  grid-template-rows: auto auto 1fr;
  background: var(--ink-0);
}

/* 顶部 */
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--sp-4) var(--sp-5);
  border-bottom: 1px solid var(--line-1);
  background: var(--ink-1);
}
.topbar-left { display: flex; align-items: baseline; gap: var(--sp-3); }
.topbar-title { font-size: 22px; color: var(--gold-1); margin: 0; }
.topbar-sub { font-size: 10px; letter-spacing: 0.22em; color: var(--text-3); }
.topbar-stats { display: flex; gap: var(--sp-5); }
.stat { display: flex; align-items: baseline; gap: 4px; }
.stat-num { font-size: 18px; color: var(--text-1); font-family: var(--font-mono); }
.stat-label { font-size: 10px; color: var(--text-3); letter-spacing: 0.1em; }
.stat-finalized .stat-num { color: var(--gold-1); }
.stat-pending .stat-num { color: var(--text-3); }

/* 工具条 */
.toolbar {
  display: flex;
  gap: var(--sp-3);
  padding: var(--sp-3) var(--sp-5);
  border-bottom: 1px solid var(--line-1);
  background: var(--ink-1);
  align-items: center;
  flex-wrap: wrap;
}
.toolbar-search { width: 220px; }
.toolbar-filter { display: flex; gap: 4px; padding: 2px; background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); }
.chip { padding: 3px 10px; font-size: 11px; color: var(--text-2); background: none; border: none; border-radius: var(--r-1); cursor: pointer; }
.chip:hover { color: var(--text-1); }
.chip.active { background: var(--gold-wash); color: var(--gold-1); }

/* 主体三栏 */
.body {
  display: grid;
  min-height: 0;
  grid-template-columns: 280px 1fr 320px;
}

.list {
  border-right: 1px solid var(--line-1);
  background: var(--ink-1);
  overflow-y: auto;
  padding: var(--sp-2);
}
.group { margin-bottom: var(--sp-3); }
.group-head { display: flex; align-items: center; gap: var(--sp-2); padding: var(--sp-2) var(--sp-3); font-size: 10px; letter-spacing: 0.15em; color: var(--text-3); }
.group-dot { width: 6px; height: 6px; border-radius: 50%; }
.dot-finalized { background: var(--gold-2); }
.dot-published { background: var(--jade); }
.dot-completed { background: var(--azure); }
.dot-pending { background: var(--text-3); }
.group-name { flex: 1; }
.group-count { font-family: var(--font-mono); }

.ep-item {
  display: flex;
  width: 100%;
  gap: var(--sp-3);
  padding: var(--sp-3);
  border: none;
  border-radius: var(--r-2);
  background: none;
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 0.12s;
}
.ep-item:hover { background: var(--ink-2); }
.ep-item.active { background: var(--gold-wash); border-left: 2px solid var(--gold-2); padding-left: calc(var(--sp-3) - 2px); }
.ep-num {
  flex-shrink: 0;
  width: 28px;
  font-family: var(--font-mono);
  font-size: 14px;
  color: var(--gold-2);
  text-align: center;
  padding-top: 2px;
}
.ep-item.active .ep-num { color: var(--gold-1); }
.ep-main { flex: 1; min-width: 0; }
.ep-title { font-size: 13px; color: var(--text-1); margin-bottom: 2px; }
.ep-item.active .ep-title { color: var(--gold-1); }
.ep-core { font-size: 11px; line-height: 1.4; overflow: hidden; text-overflow: ellipsis; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
.ep-meta { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 4px; }
.tag-sm { padding: 0 5px; font-size: 9px; }

/* 详情 */
.detail {
  overflow-y: auto;
  padding: var(--sp-5) var(--sp-5);
  min-width: 0;
}
.detail-head { margin-bottom: var(--sp-5); padding-bottom: var(--sp-4); border-bottom: 1px solid var(--line-1); }
.detail-head-row { display: flex; align-items: center; justify-content: space-between; gap: var(--sp-3); }
.detail-num { font-family: var(--font-mono); font-size: 11px; color: var(--text-3); letter-spacing: 0.2em; }
.detail-title { margin: 4px 0; font-size: 28px; color: var(--gold-1); }
.share-btn { font-size: 12px; padding: 5px 12px; }
.detail-tags { display: flex; gap: var(--sp-2); align-items: center; margin: var(--sp-2) 0; }
.detail-core { color: var(--text-2); line-height: 1.7; font-size: 13px; padding: var(--sp-3); background: var(--ink-1); border-left: 2px solid var(--gold-3); border-radius: 0 var(--r-2) var(--r-2) 0; }

.share-stage { transform: scale(1); transform-origin: top center; }

.block { margin-bottom: var(--sp-5); }
.block-title { font-family: var(--font-sans); font-size: 11px; letter-spacing: 0.2em; color: var(--gold-2); text-transform: uppercase; margin: 0 0 var(--sp-3); padding-bottom: var(--sp-2); border-bottom: 1px solid var(--line-1); }

/* 序幕 */
.prologue-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--sp-3); }
.prologue-card { padding: var(--sp-4); background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-3); position: relative; }
.prologue-step { position: absolute; top: var(--sp-3); right: var(--sp-3); font-family: var(--font-mono); font-size: 24px; color: var(--gold-3); opacity: 0.3; }
.prologue-head { font-size: 14px; color: var(--gold-1); margin: 0 0 var(--sp-2); }
.prologue-body { color: var(--text-2); font-size: 12px; line-height: 1.6; white-space: pre-wrap; }

/* 幕 */
.act { margin-bottom: var(--sp-4); }
.act-head { display: flex; justify-content: space-between; align-items: center; padding: var(--sp-2) var(--sp-3); background: var(--ink-1); border-radius: var(--r-2); margin-bottom: var(--sp-2); }
.act-label { color: var(--gold-1); font-size: 13px; }
.shots-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: var(--sp-2); }
.shot-card { padding: var(--sp-3); background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-2); }
.shot-num { font-family: var(--font-mono); font-size: 10px; color: var(--gold-2); letter-spacing: 0.15em; }
.shot-title { font-size: 12px; color: var(--text-1); margin: 4px 0 0; line-height: 1.5; }

/* 场景 */
.scenes { display: flex; flex-direction: column; gap: var(--sp-3); }
.scene { background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-3); overflow: hidden; }
.scene-head { display: flex; align-items: baseline; gap: var(--sp-3); padding: var(--sp-3) var(--sp-4); border-bottom: 1px solid var(--line-1); }
.scene-num { font-family: var(--font-mono); font-size: 10px; color: var(--gold-2); letter-spacing: 0.15em; }
.scene-title { margin: 0; font-size: 15px; color: var(--text-1); }
.scene-meta { display: grid; grid-template-columns: repeat(3, 1fr); gap: var(--sp-2); padding: var(--sp-3) var(--sp-4); margin: 0; }
.scene-meta div { display: flex; flex-direction: column; gap: 2px; }
.scene-meta dt { font-size: 9px; color: var(--text-3); letter-spacing: 0.2em; }
.scene-meta dd { margin: 0; font-size: 12px; color: var(--text-1); }
.scene-body-wrap { padding: 0 var(--sp-4) var(--sp-3); }
.scene-body-wrap summary { cursor: pointer; padding: var(--sp-2) 0; user-select: none; }
.scene-body-wrap summary:hover { color: var(--gold-1); }
.scene-body { white-space: pre-wrap; word-break: break-word; font-size: 12px; line-height: 1.7; color: var(--text-2); margin: var(--sp-2) 0 0; max-height: 400px; overflow-y: auto; }

/* raw */
.raw-md { background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-2); padding: var(--sp-3); font-size: 11px; line-height: 1.6; color: var(--text-2); white-space: pre-wrap; max-height: 500px; overflow-y: auto; }

/* 引用 */
.refs { display: flex; flex-direction: column; gap: 4px; }
.ref-link { display: flex; align-items: center; gap: var(--sp-2); padding: var(--sp-2) var(--sp-3); background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-2); color: var(--text-1); text-decoration: none; transition: border-color 0.12s; }
.ref-link:hover { border-color: var(--gold-3); color: var(--gold-1); }
.ref-icon { color: var(--gold-2); }
.ref-label { font-size: 12px; }
.ref-href { font-size: 10px; }

/* 右侧侧栏 */
.rail {
  border-left: 1px solid var(--line-1);
  background: var(--ink-1);
  overflow-y: auto;
  padding: var(--sp-3);
}
.rail-section { margin-bottom: var(--sp-5); }
.rail-title { font-size: 10px; letter-spacing: 0.2em; color: var(--gold-2); text-transform: uppercase; margin: 0 0 var(--sp-2); }
.rail-empty { font-size: 11px; padding: var(--sp-2); text-align: center; }

.jimeng-card { background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); margin-bottom: var(--sp-2); overflow: hidden; }
.jimeng-head { display: flex; justify-content: space-between; align-items: center; padding: 4px var(--sp-2); border-bottom: 1px solid var(--line-1); background: var(--ink-1); }
.jimeng-num { font-family: var(--font-mono); font-size: 9px; color: var(--gold-2); letter-spacing: 0.1em; }
.jimeng-copy { font-size: 10px; padding: 1px 6px; }
.jimeng-body { font-size: 10px; line-height: 1.5; color: var(--text-2); padding: var(--sp-2); margin: 0; white-space: pre-wrap; word-break: break-word; max-height: 200px; overflow-y: auto; }

.vo { padding: var(--sp-2); border-bottom: 1px solid var(--line-1); }
.vo:last-child { border-bottom: none; }
.vo-tone { font-size: 9px; letter-spacing: 0.15em; margin-bottom: 4px; }
.vo-text { margin: 0; font-size: 12px; line-height: 1.6; color: var(--text-1); padding-left: var(--sp-2); border-left: 2px solid var(--gold-3); }

.audio-line { font-size: 10px; line-height: 1.6; padding: 2px 0; }

.empty { padding: var(--sp-6); color: var(--text-3); text-align: center; }
.detail-empty { background: var(--ink-1); border-left: 1px solid var(--line-1); }
</style>
