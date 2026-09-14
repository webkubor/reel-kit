<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { api, setCurrentSeason, getCurrentSeason, sseConnected } from './api';

const health = ref(null);
const error = ref('');
const seasons = ref([]);
const currentSeason = ref(getCurrentSeason());
const router = useRouter();

const NAV = [
  { to: '/shots', label: '镜头台', hint: '分镜 · 出图 · 采用' },
  { to: '/episodes', label: '剧集看板', hint: '14 集 · 分镜 · 指令' },
  { to: '/cast', label: '角色库', hint: 'cast/ 真源可编辑' },
  { to: '/gallery', label: '参考图库', hint: 'references/' },
  { to: '/bible', label: '创意法典', hint: '红线 · 旁白 · 镜头美学' },
  { to: '/aesthetic', label: '三轴预览', hint: '镜头 · 音乐 · 审美' },
  { to: '/cases', label: '经典案例', hint: '8 个天榜 Solo 15s · 分享' },
  { to: '/craft', label: 'Skill 创作中心', hint: 'novels/Agent skills · 一键组装' },
  { to: '/prompt-lab', label: 'Prompt 实验室', hint: '法典 + 角色 + 神兵 → Seedance' },
  { to: '/batch', label: '批量生成', hint: '笛卡尔积 · 一键 N 条 → 草稿 shots' },
  { to: '/queue', label: '渲染队列', hint: 'museav jobs' },
];

const SEASON_LABELS = {
  s1: '第一季 · 沸腾之雪',
  s2: '第二季',
  s3: '第三季',
};

function seasonLabel(name) {
  return SEASON_LABELS[name] || name;
}

async function onSeasonChange(e) {
  const next = e.target.value;
  setCurrentSeason(next);
  currentSeason.value = next;
  router.go(0);
}

// 全局搜索
const searchQ = ref('');
const searchResults = ref([]);
const searchOpen = ref(false);
let searchTimer = null;

watch(searchQ, (q) => {
  clearTimeout(searchTimer);
  if (!q.trim()) { searchResults.value = []; return; }
  searchTimer = setTimeout(async () => {
    try {
      const r = await api.search(q);
      searchResults.value = r.results;
    } catch {
      searchResults.value = [];
    }
  }, 200);
});

const groupedResults = computed(() => {
  const g = { character: [], episode: [], weapon: [], bgm: [] };
  for (const r of searchResults.value) (g[r.type] || (g[r.type] = [])).push(r);
  return g;
});

const RESULT_TYPE_LABEL = { character: '角色', episode: '剧集', weapon: '神兵', bgm: 'BGM' };

function gotoResult(r) {
  searchOpen.value = false;
  searchQ.value = '';
  searchResults.value = [];
  router.push(r.to);
}

function onSearchBlur() {
  // 延迟关，让 click 先触发
  setTimeout(() => { searchOpen.value = false; }, 150);
}

/* ── 全局键盘快捷键（阶段 5） ──────────────────────────────────────── *
 *   ⌘-K / Ctrl-K     聚焦侧栏搜索框
 *   g + e            跳 /episodes
 *   g + c            跳 /cast
 *   g + s            跳 /shots
 *   g + p            跳 /prompt-lab
 *   g + b            跳 /batch
 *   g + r            跳 /craft （craft）
 *   g + a            跳 /aesthetic
 *   g + d            跳 /bible （创世法典 d = doctrine）
 *   g + k            跳 /cases （案例库）
 *   g + q            跳 /queue
 *   ?                打开快捷键面板
 *   Esc              关闭快捷键面板 / 取消搜索
 *   设计参考 Gmail / GitHub: 'g' 作前缀键,500ms 内按第二键触发
 */

const helpOpen = ref(false);
const searchInput = ref(null);

const KEY_MAP = {
  e: '/episodes',
  c: '/cast',
  s: '/shots',
  p: '/prompt-lab',
  b: '/batch',
  r: '/craft',
  a: '/aesthetic',
  d: '/bible',
  k: '/cases',
  q: '/queue',
};

let prefix = null;       // 当前是否在等第二键（按了 g 之后）
let prefixTimer = null;

function isEditableTarget(t) {
  if (!t) return false;
  const tag = t.tagName?.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true;
  if (t.isContentEditable) return true;
  return false;
}

function focusSearch() {
  searchOpen.value = true;
  nextTick(() => {
    searchInput.value?.focus();
    searchInput.value?.select?.();
  });
}

function onKeydown(e) {
  // Esc 优先：关掉帮助 / 清搜索
  if (e.key === 'Escape') {
    if (helpOpen.value) { helpOpen.value = false; e.preventDefault(); return; }
    if (searchQ.value) { searchQ.value = ''; searchResults.value = []; searchOpen.value = false; e.preventDefault(); return; }
  }
  // ? 打开帮助（Shift + / 在大多数键盘）
  if (e.key === '?' && !isEditableTarget(e.target)) {
    helpOpen.value = !helpOpen.value;
    e.preventDefault();
    return;
  }
  // 文本框里不抢键
  if (isEditableTarget(e.target)) return;
  // ⌘-K / Ctrl-K 聚焦搜索
  if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
    e.preventDefault();
    focusSearch();
    return;
  }
  // 'g' 前缀：开始等第二键
  if (prefix === 'g') {
    const dest = KEY_MAP[e.key.toLowerCase()];
    if (dest) {
      e.preventDefault();
      router.push(dest);
    }
    prefix = null;
    clearTimeout(prefixTimer);
    return;
  }
  if (e.key === 'g' || e.key === 'G') {
    prefix = 'g';
    clearTimeout(prefixTimer);
    prefixTimer = setTimeout(() => { prefix = null; }, 500);
    return;
  }
}

onMounted(async () => {
  try {
    const [h, s] = await Promise.all([api.health(), api.seasons()]);
    health.value = h;
    seasons.value = s.seasons.map((x) => x.name);
  } catch (err) {
    error.value = err.message;
  }
  window.addEventListener('keydown', onKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown);
  clearTimeout(prefixTimer);
});
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-title title-brush">沸雪</div>
        <div class="brand-sub">工作台 · LOCAL</div>
      </div>

      <!-- 季/IP 切换器 -->
      <div class="season-picker">
        <label class="season-label" for="season-select">季 / IP</label>
        <select id="season-select" class="season-select" :value="currentSeason" @change="onSeasonChange">
          <option v-for="s in seasons" :key="s" :value="s">{{ seasonLabel(s) }}</option>
        </select>
      </div>

      <!-- 全局搜索 -->
      <div class="search-wrap">
        <input
          ref="searchInput"
          v-model="searchQ"
          class="search-input"
          placeholder="搜角色 / 剧集 / 神兵 / BGM… (⌘-K)"
          @focus="searchOpen = true"
          @blur="onSearchBlur"
        />
        <div v-if="searchOpen && searchQ" class="search-dropdown">
          <div v-if="!searchResults.length" class="search-empty dim">无匹配</div>
          <template v-else>
            <template v-for="(items, type) in groupedResults" :key="type">
              <div v-if="items.length" class="search-group">
                <div class="search-group-head">{{ RESULT_TYPE_LABEL[type] || type }} <span class="dim mono">({{ items.length }})</span></div>
                <button v-for="r in items" :key="r.name || r.title" class="search-item" @mousedown="gotoResult(r)">
                  <span class="search-item-title">{{ r.name || r.title }}</span>
                  <span class="dim search-item-sub">{{ r.title || r.holder || r.tag || r.core || '' }}</span>
                </button>
              </div>
            </template>
          </template>
        </div>
      </div>

      <nav class="nav">
        <RouterLink v-for="item in NAV" :key="item.to" :to="item.to" class="nav-item">
          <span class="nav-label">{{ item.label }}</span>
          <span class="nav-hint">{{ item.hint }}</span>
        </RouterLink>
      </nav>

      <div class="status">
        <div v-if="error" class="status-row">
          <span class="dot dot-crimson" />
          <span class="dim">后端未就绪</span>
        </div>
        <template v-else-if="health">
          <div class="status-row" :title="health.museav.reason ?? ''">
            <span :class="['dot', health.museav.loggedIn ? 'dot-jade' : 'dot-crimson']" />
            <span class="dim">museav {{ health.museav.version ?? '缺失' }}</span>
          </div>
          <div class="status-row">
            <span :class="['dot', sseConnected ? 'dot-jade' : 'dot-dim']" />
            <span class="dim">{{ health.registry.characters }} 角色 · {{ health.registry.orphans }} 待归属</span>
          </div>
        </template>
        <div v-else class="status-row"><span class="dot dot-dim" /><span class="dim">连接中…</span></div>
      </div>
    </aside>

    <main class="main">
      <RouterView />
    </main>

    <!-- 快捷键帮助面板（按 ? 唤起） -->
    <div v-if="helpOpen" class="help-mask" @click.self="helpOpen = false">
      <div class="help-panel">
        <header class="help-head">
          <span class="title-brush help-title">键盘快捷键</span>
          <button class="help-close" @click="helpOpen = false">✕</button>
        </header>
        <div class="help-grid">
          <div class="help-group">
            <div class="help-group-title">全局</div>
            <div class="help-row"><kbd>⌘</kbd><kbd>K</kbd><span>聚焦全局搜索</span></div>
            <div class="help-row"><kbd>?</kbd><span>打开 / 关闭本面板</span></div>
            <div class="help-row"><kbd>Esc</kbd><span>关闭本面板 / 清搜索</span></div>
          </div>
          <div class="help-group">
            <div class="help-group-title">跳转（g + 字母）</div>
            <div class="help-row"><kbd>g</kbd><kbd>e</kbd><span>剧集看板</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>c</kbd><span>角色库</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>s</kbd><span>镜头台</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>p</kbd><span>Prompt 实验室</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>b</kbd><span>批量生成</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>r</kbd><span>Skill 创作中心 (craft)</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>a</kbd><span>三轴预览 (aesthetic)</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>d</kbd><span>创意法典 (doctrine)</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>k</kbd><span>经典案例</span></div>
            <div class="help-row"><kbd>g</kbd><kbd>q</kbd><span>渲染队列</span></div>
          </div>
        </div>
        <p class="help-footer dim">500ms 内连续按 g + 字母 触发跳转。</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: grid;
  grid-template-columns: 168px 1fr;
  height: 100%;
}

.sidebar {
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--line-1);
  background: var(--ink-1);
}

.brand {
  padding: var(--sp-5) var(--sp-4) var(--sp-4);
  border-bottom: 1px solid var(--line-1);
}
.brand-title {
  font-size: 26px;
  line-height: 1.1;
}
.brand-sub {
  margin-top: 2px;
  color: var(--text-3);
  font-size: 10px;
  letter-spacing: 0.22em;
}

.nav {
  flex: 1;
  padding: var(--sp-3) var(--sp-2);
}
.nav-item {
  display: block;
  margin-bottom: 2px;
  padding: 7px 10px;
  border-left: 2px solid transparent;
  border-radius: var(--r-2);
  color: var(--text-2);
  text-decoration: none;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.nav-item:hover {
  background: var(--ink-2);
  color: var(--text-1);
}
.nav-item.router-link-active {
  border-left-color: var(--gold-2);
  background: var(--gold-wash);
  color: var(--gold-1);
}
.nav-label {
  display: block;
  font-size: 13px;
}
.nav-hint {
  display: block;
  color: var(--text-3);
  font-size: 10px;
}
.nav-item.router-link-active .nav-hint {
  color: var(--gold-3);
}

.status {
  padding: var(--sp-3) var(--sp-4);
  border-top: 1px solid var(--line-1);
}

/* 季/IP 切换器 */
.season-picker {
  padding: var(--sp-3) var(--sp-4);
  border-bottom: 1px solid var(--line-1);
  background: var(--ink-2);
}
.season-label {
  display: block;
  margin-bottom: 4px;
  color: var(--text-3);
  font-size: 9px;
  letter-spacing: 0.22em;
}
.season-select {
  width: 100%;
  padding: 4px 6px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-1);
  font-family: inherit;
  font-size: 12px;
  cursor: pointer;
}
.season-select:hover {
  border-color: var(--line-gold);
}
.season-select:focus {
  outline: none;
  border-color: var(--gold-2);
}

/* 全局搜索 */
.search-wrap { position: relative; padding: var(--sp-3) var(--sp-4); border-bottom: 1px solid var(--line-1); }
.search-input {
  width: 100%;
  padding: 4px 6px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-1);
  font-size: 12px;
}
.search-input:focus { outline: none; border-color: var(--gold-2); }
.search-dropdown {
  position: absolute;
  top: calc(100% - 1px);
  left: var(--sp-4);
  right: var(--sp-4);
  max-height: 360px;
  overflow-y: auto;
  background: var(--ink-2);
  border: 1px solid var(--line-gold);
  border-radius: var(--r-2);
  box-shadow: var(--shadow-panel);
  z-index: 50;
}
.search-group { padding: 4px 0; border-bottom: 1px solid var(--line-1); }
.search-group:last-child { border-bottom: none; }
.search-group-head { padding: 4px 8px; color: var(--gold-2); font-size: 9px; letter-spacing: 0.2em; }
.search-item {
  display: block;
  width: 100%;
  padding: 4px 8px;
  border: none;
  background: none;
  color: var(--text-1);
  text-align: left;
  font-family: inherit;
  font-size: 12px;
  cursor: pointer;
}
.search-item:hover { background: var(--gold-wash); }
.search-item-title { display: block; }
.search-item-sub { display: block; font-size: 10px; }
.search-empty { padding: var(--sp-3); text-align: center; font-size: 11px; }
.status-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  line-height: 1.8;
}
.dot {
  width: 6px;
  height: 6px;
  flex-shrink: 0;
  border-radius: 50%;
}
.dot-jade {
  background: var(--jade);
}
.dot-crimson {
  background: var(--crimson);
}
.dot-dim {
  background: var(--text-3);
}

.main {
  min-width: 0;
  height: 100%;
  overflow: hidden;
}

/* ── 快捷键帮助面板（阶段 5） ── */
.help-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.help-panel {
  width: min(560px, 92vw);
  max-height: 84vh;
  overflow-y: auto;
  background: var(--ink-1);
  border: 1px solid var(--line-gold);
  border-radius: var(--r-3);
  box-shadow: var(--shadow-panel);
  padding: var(--sp-5);
}
.help-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: var(--sp-3);
  border-bottom: 1px solid var(--line-1);
  margin-bottom: var(--sp-4);
}
.help-title { font-size: 18px; color: var(--gold-1); }
.help-close {
  background: none;
  border: 1px solid var(--line-2);
  color: var(--text-2);
  font-family: inherit;
  font-size: 14px;
  width: 28px;
  height: 28px;
  border-radius: var(--r-1);
  cursor: pointer;
}
.help-close:hover { border-color: var(--gold-2); color: var(--gold-1); }
.help-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--sp-4);
}
.help-group-title {
  font-size: 10px;
  letter-spacing: 0.2em;
  color: var(--gold-2);
  margin-bottom: var(--sp-2);
}
.help-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 0;
  font-size: 12px;
  color: var(--text-2);
}
.help-row span { margin-left: 8px; }
.help-row kbd {
  display: inline-block;
  padding: 2px 6px;
  min-width: 18px;
  text-align: center;
  background: var(--ink-3);
  border: 1px solid var(--line-2);
  border-bottom-width: 2px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 10px;
  color: var(--gold-1);
}
.help-footer {
  margin-top: var(--sp-4);
  padding-top: var(--sp-3);
  border-top: 1px solid var(--line-1);
  font-size: 11px;
}
</style>
