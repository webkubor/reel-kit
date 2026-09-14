<script setup>
import { computed, onMounted, ref } from 'vue';
import { api, assetUrl } from '../api';

const categories = ref({});
const orphans = ref([]);
const activeCategory = ref('');
const charFilter = ref('');
const preview = ref(null);
const copied = ref('');

const categoryNames = computed(() => Object.keys(categories.value));

const files = computed(() => {
  const list = categories.value[activeCategory.value] ?? [];
  if (!charFilter.value) return list;
  return list.filter((f) => f.characters.includes(charFilter.value));
});

/** 当前分类下出现过的角色，用来做筛选条 */
const charsInCategory = computed(() => {
  const set = new Set();
  for (const f of categories.value[activeCategory.value] ?? []) {
    for (const name of f.characters) set.add(name);
  }
  return [...set].sort((a, b) => a.localeCompare(b, 'zh'));
});

const unmatchedHere = computed(
  () => (categories.value[activeCategory.value] ?? []).filter((f) => !f.characters.length).length,
);

async function load(refresh = false) {
  const reg = await api.registry(refresh);
  categories.value = reg.categories;
  orphans.value = reg.orphans;
  if (!activeCategory.value) activeCategory.value = categoryNames.value[0] ?? '';
}

function pickCategory(name) {
  activeCategory.value = name;
  charFilter.value = '';
}

async function copyPath(rel) {
  await navigator.clipboard.writeText(rel);
  copied.value = rel;
  setTimeout(() => {
    if (copied.value === rel) copied.value = '';
  }, 1500);
}

function sizeText(bytes) {
  return bytes > 1024 * 1024
    ? `${(bytes / 1024 / 1024).toFixed(1)}MB`
    : `${Math.round(bytes / 1024)}KB`;
}

onMounted(() => load());
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1 class="title-brush head-title">参考图库</h1>
        <p class="dim head-sub">
          直读 <code class="mono">references/</code>，归属由索引层自动推断（文件名 + 目录名）
        </p>
      </div>
      <button class="btn btn-sm" @click="load(true)">重建索引</button>
    </header>

    <nav class="cats">
      <button
        v-for="name in categoryNames"
        :key="name"
        class="cat"
        :class="{ active: name === activeCategory }"
        @click="pickCategory(name)"
      >
        {{ name }}<span class="cat-n">{{ categories[name].length }}</span>
      </button>
    </nav>

    <div v-if="charsInCategory.length" class="filters">
      <button class="chip" :class="{ active: !charFilter }" @click="charFilter = ''">全部</button>
      <button
        v-for="name in charsInCategory"
        :key="name"
        class="chip"
        :class="{ active: charFilter === name }"
        @click="charFilter = name"
      >
        {{ name }}
      </button>
      <span v-if="unmatchedHere" class="dim unmatched">{{ unmatchedHere }} 张未归属</span>
    </div>

    <div class="grid scroll-y">
      <figure v-for="file in files" :key="file.rel" class="item" @click="preview = file">
        <img v-if="file.kind === 'image'" :src="assetUrl(file.rel)" :alt="file.name" loading="lazy" />
        <span v-else class="video-badge">▶ {{ file.ext }}</span>
        <figcaption>
          <span class="cap-name">{{ file.name }}</span>
          <span class="cap-meta">
            <span v-if="file.characters.length" class="tag tag-gold">{{
              file.characters.join('·')
            }}</span>
            <span v-else class="tag">未归属</span>
            <span class="dim">{{ sizeText(file.size) }}</span>
          </span>
        </figcaption>
      </figure>
      <p v-if="!files.length" class="empty">这个分类下没有图</p>
    </div>

    <!-- 大图 -->
    <div v-if="preview" class="overlay" @click="preview = null">
      <div class="viewer" @click.stop>
        <img v-if="preview.kind === 'image'" :src="assetUrl(preview.rel)" :alt="preview.name" />
        <video v-else :src="assetUrl(preview.rel)" controls />
        <div class="viewer-bar">
          <code class="mono viewer-path">{{ preview.rel }}</code>
          <button class="btn btn-sm" @click="copyPath(preview.rel)">
            {{ copied === preview.rel ? '已复制' : '复制路径' }}
          </button>
          <button class="btn btn-sm" @click="preview = null">关闭</button>
        </div>
        <p class="dim viewer-hint">复制的路径可直接粘进镜头台的「垫图」栏当参考图</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  height: 100%;
  flex-direction: column;
}
.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: var(--sp-4) var(--sp-5);
  border-bottom: 1px solid var(--line-1);
}
.head-title {
  font-size: 20px;
  font-weight: 400;
}
.head-sub {
  font-size: 11px;
}

.cats {
  display: flex;
  overflow-x: auto;
  gap: var(--sp-1);
  padding: var(--sp-3) var(--sp-5) 0;
  border-bottom: 1px solid var(--line-1);
}
.cat {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 12px;
  border: none;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--text-2);
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
  white-space: nowrap;
}
.cat:hover {
  color: var(--text-1);
}
.cat.active {
  border-bottom-color: var(--gold-2);
  color: var(--gold-1);
}
.cat-n {
  color: var(--text-3);
  font-size: 10px;
}

.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  padding: var(--sp-3) var(--sp-5) 0;
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
.chip:hover {
  border-color: var(--line-gold);
}
.chip.active {
  border-color: var(--line-gold);
  background: var(--gold-wash);
  color: var(--gold-1);
}
.unmatched {
  margin-left: auto;
  font-size: 10px;
}

.grid {
  display: grid;
  flex: 1;
  align-content: start;
  gap: var(--sp-3);
  padding: var(--sp-4) var(--sp-5);
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
}
.item {
  overflow: hidden;
  border: 1px solid var(--line-1);
  border-radius: var(--r-2);
  background: var(--ink-2);
  cursor: pointer;
  transition: border-color 0.15s;
}
.item:hover {
  border-color: var(--line-gold);
}
.item img {
  display: block;
  width: 100%;
  height: 170px;
  background: var(--ink-0);
  object-fit: cover;
}
.video-badge {
  display: flex;
  height: 170px;
  align-items: center;
  justify-content: center;
  background: var(--ink-0);
  color: var(--text-3);
  font-size: 12px;
}
figcaption {
  padding: 6px 8px;
}
.cap-name {
  display: block;
  overflow: hidden;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cap-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  margin-top: 3px;
  font-size: 10px;
}
.cap-meta .tag {
  overflow: hidden;
  max-width: 130px;
  text-overflow: ellipsis;
}

.overlay {
  position: fixed;
  z-index: var(--z-modal);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--sp-6);
  background: rgba(0, 0, 0, 0.86);
  inset: 0;
}
.viewer {
  display: flex;
  max-width: 92vw;
  max-height: 92vh;
  flex-direction: column;
  gap: var(--sp-2);
}
.viewer img,
.viewer video {
  max-width: 100%;
  min-height: 0;
  max-height: 78vh;
  border: 1px solid var(--line-2);
  border-radius: var(--r-2);
  object-fit: contain;
}
.viewer-bar {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
}
.viewer-path {
  overflow: hidden;
  flex: 1;
  color: var(--text-2);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.viewer-hint {
  font-size: 10px;
}
</style>
