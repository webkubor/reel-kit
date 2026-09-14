<script setup>
/**
 * 镜头 · 音乐 · 审美 三轴预览
 *
 * 三栏对比：
 *   - 镜头轴：来自 CREATIVE_BIBLE 的镜头语言 + 影调
 *   - 音乐轴：8 段 BGM + 主题曲「雪沸」+ 4 段已定曲名
 *   - 审美轴：视觉规范 + 神兵专属镜头设计
 */
import { computed, onMounted, ref } from 'vue';
import { api } from '../api';

const aesthetic = ref(null);
const bgm = ref(null);
const theme = ref(null);
const weapons = ref(null);
const copied = ref(null);
const activeBgm = ref(null);

async function load() {
  const [a, b, t, w] = await Promise.all([
    api.aesthetic(),
    api.bgm(),
    api.theme(),
    api.weapons(),
  ]);
  aesthetic.value = a.aesthetic;
  bgm.value = b.bgm;
  theme.value = t.theme;
  weapons.value = w.weapons;
}

async function copy(text, id) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = id;
    setTimeout(() => { if (copied.value === id) copied.value = null; }, 1500);
  } catch { /* */ }
}

const TONE_OF = {
  global: '全局默认',
  redlines: '影调协议',
  cinema: '镜头语言',
  action: '动作物理',
  audio: '音频协议',
};

const bpmOf = (block) => {
  const bpm = block.kvs?.find((k) => /bpm/i.test(k.key))?.value;
  return bpm || null;
};

const durOf = (block) => {
  const d = block.kvs?.find((k) => /duration/i.test(k.key) || /时长/i.test(k.key))?.value;
  return d || null;
};

onMounted(load);
</script>

<template>
  <div class="page">
    <header class="topbar">
      <div>
        <h1 class="title-brush topbar-title">三轴预览</h1>
        <span class="topbar-sub">CINEMATOGRAPHY · MUSIC · AESTHETICS</span>
      </div>
    </header>

    <div class="body">
      <!-- 镜头轴 -->
      <section class="axis">
        <h2 class="axis-head">
          <span class="axis-icon">◐</span>
          <span class="axis-title">镜头</span>
          <span class="axis-sub dim">CINEMATOGRAPHY</span>
        </h2>
        <div class="axis-body scroll-y">
          <article v-for="(body, key) in { global: aesthetic?.global, redlines: aesthetic?.redlines, cinema: aesthetic?.cinema, action: aesthetic?.action, audio: aesthetic?.audio }" :key="key" class="block">
            <h3 class="block-title">{{ TONE_OF[key] }}</h3>
            <pre v-if="body" class="block-body mono">{{ body }}</pre>
            <p v-else class="empty">无</p>
          </article>
        </div>
      </section>

      <!-- 音乐轴 -->
      <section class="axis axis-music">
        <h2 class="axis-head">
          <span class="axis-icon">♪</span>
          <span class="axis-title">音乐</span>
          <span class="axis-sub dim">MUSIC</span>
        </h2>
        <div class="axis-body scroll-y">
          <!-- 主题曲 -->
          <article v-if="theme" class="block theme">
            <h3 class="block-title">主题曲 · 雪沸</h3>
            <div v-if="theme.style" class="theme-style">
              <span class="dim mono">STYLE</span>
              <pre class="mono">{{ theme.style }}</pre>
              <button class="btn btn-sm" @click="copy(theme.style, 'theme-style')">
                {{ copied === 'theme-style' ? '已复制' : '复制 Style' }}
              </button>
            </div>
            <div v-if="theme.lyrics" class="theme-lyrics">
              <span class="dim mono">LYRICS</span>
              <pre class="mono">{{ theme.lyrics }}</pre>
              <button class="btn btn-sm" @click="copy(theme.lyrics, 'theme-lyrics')">
                {{ copied === 'theme-lyrics' ? '已复制' : '复制 Lyrics' }}
              </button>
            </div>
          </article>

          <!-- BGM 库 -->
          <article v-if="bgm?.blocks?.length" class="block">
            <h3 class="block-title">BGM 库 <span class="dim mono">({{ bgm.blocks.length }})</span></h3>
            <div class="bgm-list">
              <button
                v-for="b in bgm.blocks"
                :key="b.index"
                class="bgm-card"
                :class="{ active: activeBgm === b.index }"
                @click="activeBgm = activeBgm === b.index ? null : b.index"
              >
                <div class="bgm-num">{{ String(b.index).padStart(2, '0') }}</div>
                <div class="bgm-main">
                  <div class="bgm-title">{{ b.title }}<span v-if="b.isNew" class="tag tag-sm tag-gold" style="margin-left:6px">NEW</span></div>
                  <div class="bgm-tag dim">{{ b.tag }}</div>
                  <div class="bgm-meta">
                    <span v-if="bpmOf(b)" class="tag tag-sm">{{ bpmOf(b) }} BPM</span>
                    <span v-if="durOf(b)" class="tag tag-sm">{{ durOf(b) }}</span>
                  </div>
                </div>
              </button>
            </div>

            <!-- 展开的 BGM 详情 -->
            <div v-if="activeBgm" class="bgm-detail">
              <pre class="block-body mono">{{ bgm.blocks.find((b) => b.index === activeBgm)?.body }}</pre>
              <button class="btn btn-sm" @click="copy(bgm.blocks.find((b) => b.index === activeBgm)?.body, `bgm-${activeBgm}`)">
                {{ copied === `bgm-${activeBgm}` ? '已复制' : '复制详情' }}
              </button>
            </div>
          </article>

          <!-- 已定曲名 -->
          <article v-if="bgm?.archive?.length" class="block">
            <h3 class="block-title">已定曲名 <span class="dim mono">({{ bgm.archive.length }})</span></h3>
            <div class="archive-grid">
              <div v-for="a in bgm.archive" :key="a.index" class="archive-card">
                <div class="archive-num dim mono">{{ a.index }}</div>
                <div class="archive-title">{{ a.title }}</div>
                <div v-for="k in a.kvs" :key="k.key" class="archive-kv">
                  <span class="archive-key dim">{{ k.key }}</span>
                  <span class="archive-val">{{ k.value }}</span>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>

      <!-- 审美轴 -->
      <section class="axis">
        <h2 class="axis-head">
          <span class="axis-icon">◇</span>
          <span class="axis-title">审美</span>
          <span class="axis-sub dim">AESTHETICS · 武器</span>
        </h2>
        <div class="axis-body scroll-y">
          <p class="hint dim">神兵谱 · 12 把神兵的视觉特征 + 物理逻辑 + 专属镜头设计</p>
          <article v-for="w in weapons" :key="w.index" class="weapon">
            <header class="weapon-head">
              <span class="weapon-num">{{ String(w.index).padStart(2, '0') }}</span>
              <h3 class="weapon-name">【{{ w.name }}】</h3>
              <span v-if="w.holder" class="dim">· {{ w.holder }}</span>
            </header>
            <pre class="block-body mono">{{ w.body }}</pre>
          </article>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.page { display: grid; grid-template-rows: auto 1fr; height: 100%; background: var(--ink-0); }
.topbar { padding: var(--sp-4) var(--sp-5); border-bottom: 1px solid var(--line-1); background: var(--ink-1); }
.topbar-title { font-size: 22px; color: var(--gold-1); margin: 0; }
.topbar-sub { font-size: 10px; letter-spacing: 0.22em; color: var(--text-3); }

.body { display: grid; grid-template-columns: 1fr 1fr 1fr; min-height: 0; }

.axis { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--line-1); background: var(--ink-1); }
.axis:last-child { border-right: none; }
.axis-head { display: flex; align-items: baseline; gap: var(--sp-2); padding: var(--sp-4) var(--sp-4); border-bottom: 1px solid var(--line-1); background: var(--ink-2); }
.axis-icon { font-size: 20px; color: var(--gold-1); }
.axis-title { font-family: var(--font-title); font-size: 18px; color: var(--gold-1); letter-spacing: 0.1em; }
.axis-sub { font-size: 9px; letter-spacing: 0.2em; }

.axis-body { flex: 1; padding: var(--sp-3); }
.hint { font-size: 11px; padding: 0 var(--sp-2) var(--sp-2); }
.block { margin-bottom: var(--sp-4); }
.block-title { font-size: 10px; letter-spacing: 0.2em; color: var(--gold-2); text-transform: uppercase; margin: 0 0 var(--sp-2); padding-bottom: 4px; border-bottom: 1px solid var(--line-1); }
.block-body { white-space: pre-wrap; word-break: break-word; font-size: 11px; line-height: 1.7; color: var(--text-2); margin: 0; padding: var(--sp-2); background: var(--ink-2); border-radius: var(--r-2); }

/* 主题曲 */
.theme { background: var(--ink-2); border: 1px solid var(--line-gold); border-radius: var(--r-3); padding: var(--sp-3); }
.theme-style, .theme-lyrics { margin-bottom: var(--sp-3); }
.theme-style pre, .theme-lyrics pre { background: var(--ink-0); padding: var(--sp-2); border-radius: var(--r-2); font-size: 11px; line-height: 1.7; color: var(--text-1); margin: var(--sp-1) 0; max-height: 200px; overflow-y: auto; }

/* BGM 列表 */
.bgm-list { display: flex; flex-direction: column; gap: 4px; margin-bottom: var(--sp-3); }
.bgm-card { display: flex; gap: var(--sp-2); padding: var(--sp-2); background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); color: inherit; text-align: left; cursor: pointer; }
.bgm-card:hover { border-color: var(--line-gold); }
.bgm-card.active { border-color: var(--gold-2); background: var(--gold-wash); }
.bgm-num { font-family: var(--font-mono); font-size: 11px; color: var(--gold-2); flex-shrink: 0; }
.bgm-main { flex: 1; min-width: 0; }
.bgm-title { font-size: 12px; color: var(--text-1); }
.bgm-tag { font-size: 10px; margin: 2px 0; }
.bgm-meta { display: flex; gap: 4px; }
.bgm-detail { margin-top: var(--sp-2); padding: var(--sp-3); background: var(--ink-2); border: 1px solid var(--line-gold); border-radius: var(--r-2); }
.bgm-detail .block-body { background: var(--ink-0); }

/* 已定曲名 */
.archive-grid { display: grid; grid-template-columns: 1fr 1fr; gap: var(--sp-2); }
.archive-card { padding: var(--sp-2); background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); }
.archive-num { font-size: 9px; letter-spacing: 0.15em; }
.archive-title { font-size: 13px; color: var(--gold-1); margin: 2px 0 var(--sp-2); }
.archive-kv { display: flex; flex-direction: column; gap: 2px; padding: 4px 0; border-top: 1px solid var(--line-1); }
.archive-key { font-size: 9px; letter-spacing: 0.15em; }
.archive-val { font-size: 11px; color: var(--text-2); }

/* 神兵 */
.weapon { margin-bottom: var(--sp-3); padding: var(--sp-3); background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); border-left: 2px solid var(--gold-3); }
.weapon-head { display: flex; align-items: baseline; gap: var(--sp-2); margin-bottom: var(--sp-2); }
.weapon-num { font-family: var(--font-mono); font-size: 10px; color: var(--gold-2); letter-spacing: 0.15em; }
.weapon-name { font-size: 14px; color: var(--gold-1); margin: 0; }

.empty { color: var(--text-3); text-align: center; padding: var(--sp-3); }
</style>
