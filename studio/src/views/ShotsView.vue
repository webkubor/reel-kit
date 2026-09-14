<script setup>
import { marked } from 'marked';
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { api, assetUrl, onServerEvent } from '../api';

const episodes = ref([]);
const shots = ref([]);
const characters = ref([]);
const selectedId = ref(null);
const activeEpisode = ref(null);
const draft = ref(null);
const outline = ref('');
const outlineOpen = ref(true);
const progress = ref({});
const message = ref(null);
const busy = ref(false);
const refInput = ref('');

const RATIOS = ['16:9', '9:16', '1:1', '4:3', '3:4'];
const STATUS_LABEL = {
  draft: { text: '草稿', cls: '' },
  generating: { text: '出图中', cls: 'tag-azure' },
  review: { text: '待选', cls: 'tag-gold' },
  adopted: { text: '已采用', cls: 'tag-jade' },
  failed: { text: '失败', cls: 'tag-crimson' },
};

const shotsOfEpisode = computed(() => {
  const list = activeEpisode.value
    ? shots.value.filter((s) => s.episode === activeEpisode.value)
    : shots.value.filter((s) => !s.episode);
  return [...list].sort((a, b) => a.order - b.order || a.id.localeCompare(b.id));
});

const selected = computed(() => shots.value.find((s) => s.id === selectedId.value) ?? null);

const dirty = computed(() => {
  if (!selected.value || !draft.value) return false;
  return JSON.stringify(draft.value) !== JSON.stringify(pickEditable(selected.value));
});

const outlineHtml = computed(() => (outline.value ? marked.parse(outline.value) : ''));

const runningTake = computed(() => selected.value?.takes.find((t) => t.status === 'running'));

function pickEditable(shot) {
  const { id, title, episode, order, duration, prompt, ratio, model, characters: cs, refs, notes } = shot;
  return { id, title, episode, order, duration, prompt, ratio, model, characters: [...cs], refs: [...refs], notes };
}

function countFor(episodeId) {
  return shots.value.filter((s) => s.episode === episodeId).length;
}

async function loadAll() {
  const [eps, shotState, reg] = await Promise.all([api.episodes(), api.shots(), api.registry()]);
  episodes.value = eps;
  shots.value = shotState.shots;
  characters.value = reg.characters;
  if (!activeEpisode.value && eps.length) {
    // 优先落在有镜头的那一集，别让首屏停在空荡荡的 ep00
    const withShots = eps.find((ep) => shots.value.some((s) => s.episode === ep.id));
    selectEpisode((withShots ?? eps[0]).id);
  }
}

async function selectEpisode(id) {
  if (id !== activeEpisode.value && !confirmDiscard()) return;
  activeEpisode.value = id;
  outline.value = '';
  const ep = episodes.value.find((e) => e.id === id);
  if (ep) {
    try {
      outline.value = (await api.doc(ep.rel)).text;
    } catch (err) {
      outline.value = `> 读取失败：${err.message}`;
    }
  }
}

/** 切走前拦一下未保存的改动 —— prompt 是手打出来的，丢了很肉疼 */
function confirmDiscard() {
  return !dirty.value || confirm('当前镜头有未保存的改动，切走就丢了。确定？');
}

function selectShot(id) {
  if (id !== selectedId.value && !confirmDiscard()) return;
  selectedId.value = id;
  const shot = shots.value.find((s) => s.id === id);
  draft.value = shot ? pickEditable(shot) : null;
  message.value = null;
}

async function addShot() {
  const created = await api.createShot({
    episode: activeEpisode.value,
    title: `镜头 ${shotsOfEpisode.value.length + 1}`,
    order: shotsOfEpisode.value.length + 1,
    ratio: '16:9',
    duration: 5,
  });
  shots.value.push(created);
  selectShot(created.id);
}

async function saveShot() {
  if (!dirty.value) return;
  busy.value = true;
  try {
    const updated = await api.updateShot(selectedId.value, draft.value);
    const i = shots.value.findIndex((s) => s.id === updated.id);
    shots.value[i] = updated;
    draft.value = pickEditable(updated);
    message.value = { kind: 'ok', text: '已保存到 .state/shots.json' };
  } catch (err) {
    message.value = { kind: 'bad', text: err.message };
  } finally {
    busy.value = false;
  }
}

async function removeShot() {
  if (!confirm(`删除「${selected.value.title}」？出图记录也会一起没。`)) return;
  await api.deleteShot(selectedId.value);
  shots.value = shots.value.filter((s) => s.id !== selectedId.value);
  selectedId.value = null;
  draft.value = null;
}

/** 出图会花钱，所以只在这里、只由用户点击触发 */
async function generate(video = false) {
  if (dirty.value) await saveShot();
  busy.value = true;
  message.value = null;
  progress.value = { ...progress.value, [selectedId.value]: '提交中…' };
  try {
    const { take } = await api.generate(selectedId.value, { video });
    const shot = shots.value.find((s) => s.id === selectedId.value);
    shot.takes.push(take);
    shot.status = 'generating';
  } catch (err) {
    message.value = { kind: 'bad', text: err.message };
    progress.value = { ...progress.value, [selectedId.value]: '' };
  } finally {
    busy.value = false;
  }
}

async function adopt(takeId) {
  const updated = await api.adoptTake(selectedId.value, takeId);
  const i = shots.value.findIndex((s) => s.id === updated.id);
  shots.value[i] = updated;
}

function addRef() {
  const path = refInput.value.trim();
  if (!path) return;
  if (draft.value.refs.length >= 5) {
    message.value = { kind: 'bad', text: '垫图最多 5 张' };
    return;
  }
  draft.value.refs.push(path);
  refInput.value = '';
}

function toggleCharacter(name) {
  const i = draft.value.characters.indexOf(name);
  if (i < 0) draft.value.characters.push(name);
  else draft.value.characters.splice(i, 1);
}

/** 把角色的视觉规范拼进 prompt —— 省得每次手抄一遍人设 */
async function insertCharacterSpec(name) {
  const char = await api.character(name);
  const spec = char.raw['视觉规范'];
  if (!spec) {
    message.value = { kind: 'bad', text: `${name} 的档案里没有「视觉规范」` };
    return;
  }
  const text = Object.entries(spec)
    .filter(([, v]) => typeof v === 'string')
    .map(([k, v]) => `${k}：${v}`)
    .join(' ');
  draft.value.prompt = `${draft.value.prompt}${draft.value.prompt ? '\n' : ''}【${name}】${text}`;
}

let offProgress;
let offTake;

onMounted(async () => {
  await loadAll();
  offProgress = onServerEvent('progress', ({ shotId, line }) => {
    progress.value = { ...progress.value, [shotId]: line };
  });
  offTake = onServerEvent('take', ({ shotId, take }) => {
    const shot = shots.value.find((s) => s.id === shotId);
    if (!shot) return;
    const i = shot.takes.findIndex((t) => t.id === take.id);
    if (i >= 0) shot.takes[i] = take;
    shot.status = take.status === 'done' ? 'review' : 'failed';
    progress.value = { ...progress.value, [shotId]: '' };
  });
});

onUnmounted(() => {
  offProgress?.();
  offTake?.();
});

watch(activeEpisode, () => {
  if (selected.value && selected.value.episode !== activeEpisode.value) {
    selectedId.value = null;
    draft.value = null;
  }
});
</script>

<template>
  <div class="page">
    <!-- 左：剧集 + 镜头 -->
    <aside class="tree">
      <div class="tree-section">
        <div class="tree-head">剧集</div>
        <button
          v-for="ep in episodes"
          :key="ep.id"
          class="tree-item"
          :class="{ active: ep.id === activeEpisode }"
          :title="ep.title"
          @click="selectEpisode(ep.id)"
        >
          <span class="tree-name">{{ ep.id }}</span>
          <span class="tree-n">{{ countFor(ep.id) }}</span>
        </button>
      </div>

      <div class="tree-section flex">
        <div class="tree-head">
          镜头
          <button class="btn btn-sm add" @click="addShot">＋</button>
        </div>
        <div class="scroll-y shot-list">
          <button
            v-for="shot in shotsOfEpisode"
            :key="shot.id"
            class="tree-item shot-item"
            :class="{ active: shot.id === selectedId }"
            @click="selectShot(shot.id)"
          >
            <span class="shot-order mono">{{ String(shot.order).padStart(2, '0') }}</span>
            <span class="tree-name">{{ shot.title }}</span>
            <span class="tag" :class="STATUS_LABEL[shot.status]?.cls">{{
              STATUS_LABEL[shot.status]?.text ?? shot.status
            }}</span>
          </button>
          <p v-if="!shotsOfEpisode.length" class="empty tiny">
            这一集还没有镜头，点 ＋ 建一个
          </p>
        </div>
      </div>
    </aside>

    <!-- 中：编辑器 -->
    <section v-if="draft" class="editor">
      <header class="head">
        <input v-model="draft.title" class="input title-input" />
        <div class="head-actions">
          <button class="btn btn-sm btn-danger" @click="removeShot">删除</button>
          <button class="btn btn-sm" :disabled="!dirty || busy" @click="saveShot">保存</button>
        </div>
      </header>

      <div v-if="message" class="banner" :class="message.kind">{{ message.text }}</div>

      <div class="editor-body scroll-y">
        <div class="row3">
          <label class="field">
            <span class="label">序号</span>
            <input v-model.number="draft.order" type="number" class="input" />
          </label>
          <label class="field">
            <span class="label">时长（秒）</span>
            <input v-model.number="draft.duration" type="number" class="input" />
          </label>
          <label class="field">
            <span class="label">画幅</span>
            <select v-model="draft.ratio" class="select">
              <option v-for="r in RATIOS" :key="r" :value="r">{{ r }}</option>
            </select>
          </label>
        </div>

        <label class="field">
          <span class="label">模型（留空走 auto 路由）</span>
          <input v-model="draft.model" class="input" placeholder="如 artsdance-2-0-pro-260801" />
        </label>

        <div class="field">
          <span class="label">出场角色</span>
          <div class="chips">
            <button
              v-for="char in characters"
              :key="char.name"
              class="chip"
              :class="{ active: draft.characters.includes(char.name) }"
              @click="toggleCharacter(char.name)"
            >
              {{ char.name }}
            </button>
          </div>
          <div v-if="draft.characters.length" class="spec-row">
            <span class="dim">插入视觉规范：</span>
            <button
              v-for="name in draft.characters"
              :key="name"
              class="btn btn-sm"
              @click="insertCharacterSpec(name)"
            >
              {{ name }}
            </button>
          </div>
        </div>

        <label class="field">
          <span class="label">Prompt</span>
          <textarea
            v-model="draft.prompt"
            class="textarea prompt-box"
            placeholder="15秒电影镜头，24fps，16:9，35mm 胶片颗粒感…"
          />
        </label>

        <div class="field">
          <span class="label">垫图 · 最多 5 张（从参考图库复制路径）</span>
          <div class="ref-add">
            <input
              v-model="refInput"
              class="input"
              placeholder="references/定妆照/顾栖月-定妆.png"
              @keyup.enter="addRef"
            />
            <button class="btn" @click="addRef">加入</button>
          </div>
          <div v-if="draft.refs.length" class="ref-list">
            <div v-for="(ref, i) in draft.refs" :key="ref" class="ref-item">
              <img :src="assetUrl(ref)" :alt="ref" />
              <code class="mono ref-path">{{ ref }}</code>
              <button class="btn btn-sm btn-danger" @click="draft.refs.splice(i, 1)">移除</button>
            </div>
          </div>
        </div>

        <label class="field">
          <span class="label">备注</span>
          <input v-model="draft.notes" class="input" />
        </label>

        <!-- 出图 -->
        <div class="gen-bar">
          <button class="btn btn-primary" :disabled="busy || !!runningTake" @click="generate(false)">
            出图（走 museav，消耗额度）
          </button>
          <button class="btn" :disabled="busy || !!runningTake" @click="generate(true)">
            出视频 · {{ draft.duration }}s
          </button>
          <span v-if="progress[selectedId]" class="dim progress mono">{{ progress[selectedId] }}</span>
        </div>

        <!-- 版本 -->
        <div v-if="selected.takes.length" class="takes">
          <div class="label">出图版本 {{ selected.takes.length }}</div>
          <div class="take-grid">
            <figure
              v-for="take in [...selected.takes].reverse()"
              :key="take.id"
              class="take"
              :class="{ adopted: take.adopted }"
            >
              <div class="take-media">
                <img v-if="take.url && !take.video" :src="take.url" :alt="take.id" loading="lazy" />
                <video v-else-if="take.url" :src="take.url" controls />
                <span v-else-if="take.status === 'running'" class="take-state">出图中…</span>
                <span v-else class="take-state bad">{{ take.error ?? '失败' }}</span>
              </div>
              <figcaption>
                <span class="tag" :class="take.adopted ? 'tag-jade' : ''">{{
                  take.adopted ? '已采用' : take.status
                }}</span>
                <span class="dim">{{ take.model || 'auto' }} · {{ take.ratio }}</span>
                <button
                  v-if="take.url && !take.adopted"
                  class="btn btn-sm"
                  @click="adopt(take.id)"
                >
                  采用
                </button>
                <a v-if="take.url" :href="take.url" target="_blank" rel="noreferrer" class="btn btn-sm"
                  >原图</a
                >
              </figcaption>
            </figure>
          </div>
        </div>
      </div>
    </section>

    <div v-else class="editor empty">
      左边选一个镜头，或点 ＋ 新建
    </div>

    <!-- 右：剧集大纲（只读） -->
    <aside class="outline" :class="{ collapsed: !outlineOpen }">
      <div class="outline-head">
        <button class="btn btn-sm" @click="outlineOpen = !outlineOpen">
          {{ outlineOpen ? '收起' : '大纲' }}
        </button>
        <span v-if="outlineOpen" class="dim outline-title">{{ activeEpisode }} · 只读</span>
      </div>
      <div v-if="outlineOpen" class="outline-body scroll-y" v-html="outlineHtml" />
    </aside>
  </div>
</template>

<style scoped>
.page {
  display: grid;
  height: 100%;
  grid-template-columns: 210px minmax(0, 1fr) auto;
}

/* ── 左树 ── */
.tree {
  display: flex;
  min-height: 0;
  flex-direction: column;
  border-right: 1px solid var(--line-1);
  background: var(--ink-1);
}
.tree-section {
  display: flex;
  min-height: 0;
  flex-direction: column;
  padding: var(--sp-2);
  border-bottom: 1px solid var(--line-1);
}
.tree-section.flex {
  flex: 1;
  border-bottom: none;
}
.tree-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 6px var(--sp-1);
  color: var(--text-3);
  font-size: 10px;
  letter-spacing: 0.14em;
}
.add {
  padding: 0 6px;
  line-height: 18px;
}
.shot-list {
  min-height: 0;
  flex: 1;
}
.tree-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border: none;
  border-radius: var(--r-2);
  background: none;
  color: var(--text-2);
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
  text-align: left;
}
.tree-item:hover {
  background: var(--ink-2);
  color: var(--text-1);
}
.tree-item.active {
  background: var(--gold-wash);
  color: var(--gold-1);
}
.tree-name {
  overflow: hidden;
  flex: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tree-n,
.shot-order {
  color: var(--text-3);
  font-size: 10px;
}
.shot-item .tag {
  font-size: 9px;
}
.tiny {
  padding: var(--sp-3);
  font-size: 11px;
}

/* ── 编辑器 ── */
.editor {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.head {
  display: flex;
  gap: var(--sp-2);
  padding: var(--sp-3) var(--sp-4);
  border-bottom: 1px solid var(--line-1);
}
.title-input {
  flex: 1;
  border-color: transparent;
  background: none;
  font-family: var(--font-title);
  font-size: 17px;
  letter-spacing: 0.05em;
}
.title-input:hover {
  border-color: var(--line-2);
}
.head-actions {
  display: flex;
  gap: var(--sp-2);
}

.banner {
  margin: var(--sp-2) var(--sp-4) 0;
  padding: 5px 9px;
  border-radius: var(--r-2);
  font-size: 11px;
}
.banner.ok {
  background: var(--jade-wash);
  color: var(--jade);
}
.banner.bad {
  background: var(--crimson-wash);
  color: var(--crimson);
}

.editor-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: var(--sp-4);
  padding: var(--sp-4);
}
.row3 {
  display: grid;
  gap: var(--sp-3);
  grid-template-columns: repeat(3, 1fr);
}
.field {
  display: block;
}
.prompt-box {
  min-height: 150px;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.chip {
  padding: 2px 9px;
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  background: none;
  color: var(--text-2);
  cursor: pointer;
  font-family: inherit;
  font-size: 11px;
}
.chip.active {
  border-color: var(--line-gold);
  background: var(--gold-wash);
  color: var(--gold-1);
}
.spec-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  font-size: 11px;
}

.ref-add {
  display: flex;
  gap: var(--sp-2);
}
.ref-list {
  margin-top: var(--sp-2);
}
.ref-item {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  padding: 4px 0;
}
.ref-item img {
  width: 48px;
  height: 36px;
  border: 1px solid var(--line-1);
  border-radius: var(--r-1);
  background: var(--ink-0);
  object-fit: cover;
}
.ref-path {
  overflow: hidden;
  flex: 1;
  color: var(--text-2);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.gen-bar {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  padding-top: var(--sp-2);
  border-top: 1px solid var(--line-1);
}
.progress {
  overflow: hidden;
  flex: 1;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.takes {
  padding-top: var(--sp-2);
  border-top: 1px solid var(--line-1);
}
.take-grid {
  display: grid;
  gap: var(--sp-3);
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
}
.take {
  overflow: hidden;
  border: 1px solid var(--line-1);
  border-radius: var(--r-2);
  background: var(--ink-2);
}
.take.adopted {
  border-color: var(--jade);
}
.take-media {
  display: flex;
  height: 150px;
  align-items: center;
  justify-content: center;
  background: var(--ink-0);
}
.take-media img,
.take-media video {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.take-state {
  color: var(--text-3);
  font-size: 11px;
}
.take-state.bad {
  padding: 0 var(--sp-2);
  color: var(--crimson);
  text-align: center;
}
.take figcaption {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 5px;
  padding: 6px 8px;
  font-size: 10px;
}
.take figcaption .dim {
  flex: 1;
}

/* ── 右侧大纲 ── */
.outline {
  display: flex;
  width: 320px;
  min-height: 0;
  flex-direction: column;
  border-left: 1px solid var(--line-1);
  background: var(--ink-1);
}
.outline.collapsed {
  width: auto;
}
.outline-head {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  padding: var(--sp-3);
  border-bottom: 1px solid var(--line-1);
}
.outline-title {
  font-size: 10px;
}
.outline-body {
  flex: 1;
  padding: var(--sp-3) var(--sp-4);
  font-size: 12px;
}
.outline-body :deep(h1) {
  margin-bottom: var(--sp-2);
  color: var(--gold-1);
  font-family: var(--font-title);
  font-size: 16px;
  font-weight: 400;
}
.outline-body :deep(h2) {
  margin: var(--sp-4) 0 var(--sp-1);
  color: var(--gold-2);
  font-size: 13px;
  font-weight: 400;
}
.outline-body :deep(h3) {
  margin: var(--sp-3) 0 var(--sp-1);
  color: var(--text-1);
  font-size: 12px;
}
.outline-body :deep(p),
.outline-body :deep(li) {
  color: var(--text-2);
  line-height: 1.75;
}
.outline-body :deep(ul) {
  padding-left: var(--sp-4);
  margin: var(--sp-1) 0;
}
.outline-body :deep(code) {
  padding: 1px 4px;
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-1);
  font-family: var(--font-mono);
  font-size: 11px;
  overflow-wrap: anywhere;
}
.outline-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--line-1);
  margin: var(--sp-3) 0;
}
.outline-body :deep(strong) {
  color: var(--text-1);
}
</style>
