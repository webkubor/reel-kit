<script setup>
import { computed, onMounted, ref, watch } from 'vue';
import { api, assetUrl } from '../api';

const characters = ref([]);
const orphans = ref([]);
const weapons = ref([]);
const selected = ref(null);
const detail = ref(null);
const draft = ref('');
const tab = ref('profile');
const saving = ref(false);
const message = ref(null);
const keyword = ref('');
const onlyMine = ref(false);
const copied = ref(null);

const filtered = computed(() => {
  const kw = keyword.value.trim();
  if (!kw) return characters.value;
  return characters.value.filter(
    (c) => c.name.includes(kw) || c.title?.includes(kw) || c.aliases?.some((a) => a.includes(kw)),
  );
});

/** 有图却没有 cast 档案的角色 —— 缺口该被看见，而不是悄悄消失在 orphans 里 */
const undocumented = computed(() => {
  const names = new Set();
  for (const o of orphans.value) {
    // 只有人像类目录里的孤儿才可能是「缺档案的角色」，场景图和神兵不算
    if (['三视图', '定妆照', '脸部参考'].includes(o.category)) names.add(o.name.split(/[_\-]/)[0]);
  }
  return [...names];
});

const jsonInvalid = computed(() => {
  if (!draft.value) return false;
  try {
    JSON.parse(draft.value);
    return false;
  } catch {
    return true;
  }
});

const dirty = computed(
  () => detail.value && draft.value !== JSON.stringify(detail.value.raw, null, 2),
);

async function loadList() {
  const reg = await api.registry();
  characters.value = reg.characters;
  orphans.value = reg.orphans;
  // 神兵谱只加载一次；cast 列表变动不影响武器
  if (!weapons.value.length) {
    const w = await api.weapons();
    weapons.value = w.weapons || [];
  }
  if (!selected.value && characters.value.length) select(characters.value[0].name);
}

async function select(name) {
  selected.value = name;
  detail.value = null;
  detail.value = await api.character(name);
  draft.value = JSON.stringify(detail.value.raw, null, 2);
  message.value = null;
}

async function save() {
  if (jsonInvalid.value) return;
  saving.value = true;
  message.value = null;
  try {
    await api.saveCharacter(selected.value, JSON.parse(draft.value));
    message.value = { kind: 'ok', text: `已写入 ${detail.value.castPath}` };
    await loadList();
    await select(selected.value);
  } catch (err) {
    message.value = { kind: 'bad', text: err.message };
  } finally {
    saving.value = false;
  }
}

function revert() {
  draft.value = JSON.stringify(detail.value.raw, null, 2);
  message.value = null;
}

/** cast JSON 每个角色的字段都不一样，所以按值的形状渲染，不写死表单 */
function shapeOf(value) {
  if (Array.isArray(value)) return value.every((v) => typeof v === 'string') ? 'tags' : 'list';
  if (value && typeof value === 'object') return 'object';
  return 'text';
}

const allRefs = computed(() => {
  if (!detail.value?.refs) return [];
  return Object.entries(detail.value.refs).flatMap(([category, files]) =>
    files.map((f) => ({ ...f, category })),
  );
});

/** 当前角色持有的神兵 —— holder 字段包含角色名就算 */
const myWeapons = computed(() => {
  if (!detail.value?.name) return [];
  const name = detail.value.name;
  return weapons.value.filter((w) => w.holder?.includes(name));
});

/** 武器 tab 显示列表：默认全 12 把，开启 onlyMine 只看自己的 */
const weaponsForTab = computed(() => {
  if (onlyMine.value) return myWeapons.value;
  return weapons.value;
});

const weaponKv = (kvs, key) => kvs?.find((k) => k.key === key)?.value;

async function copy(text, id) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = id;
    setTimeout(() => { if (copied.value === id) copied.value = null; }, 1500);
  } catch { /* */ }
}

watch(selected, () => {
  tab.value = 'profile';
});

onMounted(loadList);
</script>

<template>
  <div class="page">
    <aside class="list-pane">
      <div class="search">
        <input v-model="keyword" class="input" placeholder="搜角色 / 别名 / 身份" />
      </div>
      <div class="roster scroll-y">
        <button
          v-for="char in filtered"
          :key="char.name"
          class="roster-item"
          :class="{ active: char.name === selected }"
          @click="select(char.name)"
        >
          <img v-if="char.avatar" :src="assetUrl(char.avatar)" class="avatar" :alt="char.name" />
          <span v-else class="avatar avatar-empty">{{ char.name[0] }}</span>
          <span class="roster-body">
            <span class="roster-name">{{ char.name }}</span>
            <span class="roster-title">{{ char.title || '—' }}</span>
          </span>
          <span class="roster-count" :class="{ zero: !char.refCount }">{{ char.refCount }}</span>
        </button>

        <div v-if="undocumented.length" class="gap-note">
          <div class="gap-title">有图无档案</div>
          <div class="gap-body">
            {{ undocumented.join(' · ') }}
            <p class="dim">这些名字在 references/ 里有图，但 cast/ 下没有 JSON 档案。</p>
          </div>
        </div>
      </div>
    </aside>

    <section v-if="detail" class="detail">
      <header class="head">
        <div class="head-main">
          <h1 class="title-brush head-title">{{ detail.name }}</h1>
          <p class="dim head-sub">{{ detail.title }}</p>
          <div class="head-tags">
            <span v-if="detail.rank" class="tag tag-gold">{{ detail.rank }}</span>
            <span v-if="detail.faction" class="tag">{{ detail.faction }}</span>
            <span class="tag mono">{{ detail.id }}</span>
          </div>
        </div>
        <div class="head-actions">
          <button class="btn btn-sm" :disabled="!dirty" @click="revert">还原</button>
          <button
            class="btn btn-sm btn-primary"
            :disabled="!dirty || jsonInvalid || saving"
            @click="save"
          >
            {{ saving ? '写入中…' : '保存到 cast/' }}
          </button>
        </div>
      </header>

      <div v-if="message" class="banner" :class="message.kind">{{ message.text }}</div>

      <nav class="tabs">
        <button
          v-for="t in [
            { id: 'profile', label: '档案' },
            { id: 'refs', label: `参考图 ${allRefs.length}` },
            { id: 'weapons', label: `神兵 ${myWeapons.length}/${weapons.length}` },
            { id: 'raw', label: '原文 JSON' },
          ]"
          :key="t.id"
          class="tab"
          :class="{ active: tab === t.id }"
          @click="tab = t.id"
        >
          {{ t.label }}
        </button>
      </nav>

      <div class="body scroll-y">
        <!-- 档案：按值的形状自适应渲染 -->
        <div v-if="tab === 'profile'" class="fields">
          <section v-for="(value, key) in detail.raw" :key="key" class="field">
            <h3 class="field-key">{{ key }}</h3>
            <p v-if="shapeOf(value) === 'text'" class="field-text">{{ value }}</p>
            <div v-else-if="shapeOf(value) === 'tags'" class="field-tags">
              <span v-for="(item, i) in value" :key="i" class="tag">{{ item }}</span>
            </div>
            <pre v-else class="field-json mono">{{ JSON.stringify(value, null, 2) }}</pre>
          </section>
        </div>

        <!-- 参考图 -->
        <div v-else-if="tab === 'refs'" class="grid">
          <figure v-for="file in allRefs" :key="file.rel" class="shot">
            <img :src="assetUrl(file.rel)" :alt="file.name" loading="lazy" />
            <figcaption>
              <span class="tag">{{ file.category }}</span>
              <span class="dim">{{ file.name }}</span>
            </figcaption>
          </figure>
          <p v-if="!allRefs.length" class="empty">
            这个角色还没有任何参考图。去镜头台出一张，或往 references/ 里放。
          </p>
        </div>

        <!-- 神兵 -->
        <div v-else-if="tab === 'weapons'" class="weapons-tab">
          <div class="weapons-toolbar">
            <label class="weapons-toggle">
              <input type="checkbox" v-model="onlyMine" />
              <span>只看 {{ detail.name }} 的 ({{ myWeapons.length }})</span>
            </label>
            <span class="dim mono">{{ weaponsForTab.length }} / {{ weapons.length }}</span>
          </div>
          <p v-if="!weapons.length" class="empty">神兵谱未加载</p>
          <p v-else-if="!weaponsForTab.length" class="empty">
            {{ onlyMine ? `${detail.name} 在神兵谱里没有专属武器` : '无' }}
          </p>
          <div v-else class="weapons-list">
            <article
              v-for="w in weaponsForTab"
              :key="w.index"
              class="weapon-card"
              :class="{ mine: myWeapons.includes(w) }"
            >
              <header class="weapon-card-head">
                <span class="weapon-card-num">{{ String(w.index).padStart(2, '0') }}</span>
                <h3 class="weapon-card-name">【{{ w.name }}】</h3>
                <span v-if="w.holder" class="dim weapon-card-holder">{{ w.holder }}</span>
                <button
                  v-if="myWeapons.includes(w)"
                  class="btn btn-sm weapon-card-copy"
                  @click="copy(weaponKv(w.kvs, 'IP符号') || w.body, `w-${w.index}`)"
                >{{ copied === `w-${w.index}` ? '已复制' : '复制' }}</button>
              </header>
              <dl class="weapon-card-meta">
                <div v-if="weaponKv(w.kvs, '形制')">
                  <dt>形制</dt><dd>{{ weaponKv(w.kvs, '形制') }}</dd>
                </div>
                <div v-if="weaponKv(w.kvs, '物理逻辑')">
                  <dt>物理</dt><dd>{{ weaponKv(w.kvs, '物理逻辑') }}</dd>
                </div>
                <div v-if="weaponKv(w.kvs, 'IP符号')">
                  <dt>符号</dt><dd>{{ weaponKv(w.kvs, 'IP符号') }}</dd>
                </div>
              </dl>
              <details class="weapon-card-body-wrap">
                <summary class="dim">展开完整描述</summary>
                <pre class="weapon-card-body mono">{{ w.body }}</pre>
              </details>
            </article>
          </div>
        </div>

        <!-- 原文 -->
        <div v-else class="raw">
          <p class="dim raw-hint">
            直接改真源 <code class="mono">{{ detail.castPath }}</code
            >。点保存才会写盘。
          </p>
          <textarea v-model="draft" class="textarea raw-editor" spellcheck="false" />
          <p v-if="jsonInvalid" class="raw-error">JSON 语法有误，保存已锁住</p>
        </div>
      </div>
    </section>

    <div v-else class="empty detail">选一个角色</div>
  </div>
</template>

<style scoped>
.page {
  display: grid;
  height: 100%;
  grid-template-columns: 236px 1fr;
}

.list-pane {
  display: flex;
  min-height: 0;
  flex-direction: column;
  border-right: 1px solid var(--line-1);
  background: var(--ink-1);
}
.search {
  padding: var(--sp-3);
  border-bottom: 1px solid var(--line-1);
}
.roster {
  flex: 1;
  padding: var(--sp-2);
}
.roster-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: var(--sp-2);
  padding: 6px;
  border: none;
  border-radius: var(--r-2);
  background: none;
  color: inherit;
  cursor: pointer;
  font-family: inherit;
  text-align: left;
}
.roster-item:hover {
  background: var(--ink-2);
}
.roster-item.active {
  background: var(--gold-wash);
}
.avatar {
  display: flex;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--line-2);
  border-radius: var(--r-2);
  object-fit: cover;
}
.avatar-empty {
  background: var(--ink-3);
  color: var(--text-3);
}
.roster-body {
  min-width: 0;
  flex: 1;
}
.roster-name {
  display: block;
  font-size: 13px;
}
.roster-item.active .roster-name {
  color: var(--gold-1);
}
.roster-title {
  display: block;
  overflow: hidden;
  color: var(--text-3);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.roster-count {
  color: var(--text-3);
  font-size: 10px;
}
.roster-count.zero {
  color: var(--crimson);
}

.gap-note {
  margin: var(--sp-3) 6px;
  padding: var(--sp-2);
  border: 1px dashed var(--line-2);
  border-radius: var(--r-2);
}
.gap-title {
  margin-bottom: 4px;
  color: var(--text-2);
  font-size: 10px;
  letter-spacing: 0.1em;
}
.gap-body {
  color: var(--text-2);
  font-size: 11px;
}
.gap-body p {
  margin-top: 4px;
  font-size: 10px;
}

.detail {
  display: flex;
  min-width: 0;
  flex-direction: column;
}
.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--sp-4);
  padding: var(--sp-4) var(--sp-5);
  border-bottom: 1px solid var(--line-1);
}
.head-title {
  font-size: 22px;
  font-weight: 400;
}
.head-sub {
  font-size: 11px;
}
.head-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}
.head-actions {
  display: flex;
  gap: var(--sp-2);
}

.banner {
  margin: var(--sp-3) var(--sp-5) 0;
  padding: 6px 10px;
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

.tabs {
  display: flex;
  gap: var(--sp-1);
  padding: var(--sp-3) var(--sp-5) 0;
  border-bottom: 1px solid var(--line-1);
}
.tab {
  padding: 5px 12px;
  border: none;
  border-bottom: 2px solid transparent;
  background: none;
  color: var(--text-2);
  cursor: pointer;
  font-family: inherit;
  font-size: 12px;
}
.tab:hover {
  color: var(--text-1);
}
.tab.active {
  border-bottom-color: var(--gold-2);
  color: var(--gold-1);
}

.body {
  flex: 1;
  padding: var(--sp-4) var(--sp-5);
}

.fields {
  display: flex;
  max-width: 900px;
  flex-direction: column;
  gap: var(--sp-4);
}
.field-key {
  margin-bottom: var(--sp-1);
  color: var(--gold-2);
  font-size: 12px;
  font-weight: 400;
  letter-spacing: 0.08em;
}
.field-text {
  color: var(--text-1);
  font-size: 13px;
}
.field-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.field-json {
  overflow-x: auto;
  padding: var(--sp-3);
  border: 1px solid var(--line-1);
  border-radius: var(--r-2);
  background: var(--ink-1);
  color: var(--text-2);
  font-size: 11px;
  line-height: 1.7;
  white-space: pre-wrap;
}

.grid {
  display: grid;
  gap: var(--sp-3);
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
}
.shot {
  overflow: hidden;
  border: 1px solid var(--line-1);
  border-radius: var(--r-2);
  background: var(--ink-2);
}
.shot img {
  display: block;
  width: 100%;
  height: 180px;
  background: var(--ink-0);
  object-fit: cover;
}
.shot figcaption {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  font-size: 10px;
}
.shot figcaption .dim {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.raw-hint {
  margin-bottom: var(--sp-2);
  font-size: 11px;
}
.raw-editor {
  min-height: 60vh;
}
.raw-error {
  margin-top: var(--sp-2);
  color: var(--crimson);
  font-size: 11px;
}

/* 神兵 tab */
.weapons-tab { display: flex; flex-direction: column; gap: var(--sp-3); max-width: 920px; }
.weapons-toolbar { display: flex; justify-content: space-between; align-items: center; padding: var(--sp-2) var(--sp-3); background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-2); }
.weapons-toggle { display: flex; gap: var(--sp-2); align-items: center; font-size: 12px; color: var(--text-2); cursor: pointer; }
.weapons-toggle input { accent-color: var(--gold-2); }
.weapons-list { display: flex; flex-direction: column; gap: var(--sp-2); }
.weapon-card { padding: var(--sp-3); background: var(--ink-1); border: 1px solid var(--line-1); border-radius: var(--r-3); border-left: 2px solid var(--ink-4); }
.weapon-card.mine { border-left-color: var(--gold-2); background: linear-gradient(90deg, var(--gold-wash) 0%, var(--ink-1) 30%); }
.weapon-card-head { display: flex; align-items: center; gap: var(--sp-2); margin-bottom: var(--sp-2); }
.weapon-card-num { font-family: var(--font-mono); font-size: 10px; color: var(--gold-2); letter-spacing: 0.15em; }
.weapon-card-name { margin: 0; font-size: 15px; color: var(--gold-1); flex: 0 0 auto; }
.weapon-card-holder { font-size: 11px; flex: 1; }
.weapon-card-copy { font-size: 10px; padding: 1px 6px; }
.weapon-card-meta { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: var(--sp-2); padding: var(--sp-2); background: var(--ink-0); border-radius: var(--r-2); margin: 0 0 var(--sp-2); }
.weapon-card-meta div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.weapon-card-meta dt { font-size: 9px; color: var(--text-3); letter-spacing: 0.2em; }
.weapon-card-meta dd { margin: 0; font-size: 11px; color: var(--text-1); line-height: 1.5; }
.weapon-card-body-wrap summary { cursor: pointer; padding: var(--sp-1) 0; user-select: none; font-size: 10px; }
.weapon-card-body-wrap summary:hover { color: var(--gold-1); }
.weapon-card-body { white-space: pre-wrap; word-break: break-word; font-size: 11px; line-height: 1.6; color: var(--text-2); margin: var(--sp-1) 0 0; max-height: 320px; overflow-y: auto; padding: var(--sp-2); background: var(--ink-0); border-radius: var(--r-2); }
</style>
