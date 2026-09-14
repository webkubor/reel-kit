<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { api } from '../api';

const jobs = ref([]);
const loading = ref(true);
const error = ref('');
let timer = null;

const stats = computed(() => {
  const by = { done: 0, running: 0, failed: 0 };
  for (const j of jobs.value) {
    if (j.status === 'done') by.done += 1;
    else if (j.status === 'failed' || j.error) by.failed += 1;
    else by.running += 1;
  }
  return by;
});

async function load() {
  try {
    jobs.value = await api.jobs(30);
    error.value = '';
  } catch (err) {
    error.value = err.message;
  } finally {
    loading.value = false;
  }
}

function statusClass(job) {
  if (job.status === 'done') return 'tag-jade';
  if (job.status === 'failed' || job.error) return 'tag-crimson';
  return 'tag-azure';
}

function elapsed(job) {
  if (!job.elapsed_ms) return '—';
  return `${(job.elapsed_ms / 1000).toFixed(1)}s`;
}

function when(job) {
  if (!job.created_at) return '';
  return new Date(job.created_at).toLocaleString('zh-CN', { hour12: false });
}

onMounted(() => {
  load();
  // 队列是中台的状态，本地没有推送通道，只能轮询。20s 够用，不会把 CLI 压垮。
  timer = setInterval(load, 20_000);
});
onUnmounted(() => clearInterval(timer));
</script>

<template>
  <div class="page">
    <header class="head">
      <div>
        <h1 class="title-brush head-title">渲染队列</h1>
        <p class="dim head-sub">
          数据来自 <code class="mono">museav jobs</code> —— 中台侧的真实工作流，含网页端提交的
        </p>
      </div>
      <div class="head-right">
        <span class="tag tag-jade">完成 {{ stats.done }}</span>
        <span class="tag tag-azure">进行 {{ stats.running }}</span>
        <span class="tag tag-crimson">失败 {{ stats.failed }}</span>
        <button class="btn btn-sm" :disabled="loading" @click="load">刷新</button>
      </div>
    </header>

    <div v-if="error" class="banner">{{ error }}</div>
    <div v-else-if="loading" class="empty">加载中…</div>
    <div v-else-if="!jobs.length" class="empty">还没有任何出图记录</div>

    <div v-else class="list scroll-y">
      <article v-for="job in jobs" :key="job.id" class="card row">
        <a v-if="job.cdn_url" :href="job.cdn_url" target="_blank" rel="noreferrer" class="thumb">
          <img v-if="job.media_type !== 'video'" :src="job.cdn_url" :alt="job.id" loading="lazy" />
          <span v-else class="thumb-video">▶ 视频</span>
        </a>
        <div v-else class="thumb thumb-empty">—</div>

        <div class="body">
          <div class="row-top">
            <span class="tag" :class="statusClass(job)">{{ job.status }}</span>
            <span class="tag">{{ job.model ?? 'auto' }}</span>
            <span class="tag">{{ job.ratio }}</span>
            <span class="dim mono">{{ elapsed(job) }}</span>
            <span class="dim spacer">{{ when(job) }}</span>
          </div>
          <p class="prompt">{{ job.prompt || '（无 prompt）' }}</p>
          <p v-if="job.error" class="err">{{ job.error }}</p>
          <div v-if="job.steps?.length" class="steps">
            <span
              v-for="step in job.steps"
              :key="step.name"
              class="step"
              :class="{ bad: step.status !== 'ok' }"
            >
              {{ step.name }}<template v-if="step.ms"> {{ step.ms }}ms</template>
            </span>
          </div>
        </div>
      </article>
    </div>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  height: 100%;
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
  font-size: 20px;
  font-weight: 400;
}
.head-sub {
  font-size: 11px;
}
.head-right {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
}

.banner {
  margin: var(--sp-4) var(--sp-5);
  padding: var(--sp-3);
  border: 1px solid var(--crimson);
  border-radius: var(--r-2);
  background: var(--crimson-wash);
  color: var(--crimson);
  font-size: 12px;
}

.list {
  flex: 1;
  padding: var(--sp-4) var(--sp-5);
}
.row {
  display: flex;
  gap: var(--sp-3);
  margin-bottom: var(--sp-3);
  padding: var(--sp-3);
}
.thumb {
  display: flex;
  width: 96px;
  height: 72px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--line-1);
  border-radius: var(--r-2);
  background: var(--ink-0);
}
.thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.thumb-video,
.thumb-empty {
  color: var(--text-3);
  font-size: 11px;
}
.body {
  min-width: 0;
  flex: 1;
}
.row-top {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  margin-bottom: 6px;
  font-size: 11px;
}
.spacer {
  margin-left: auto;
}
.prompt {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  color: var(--text-2);
  font-size: 12px;
}
.err {
  margin-top: 4px;
  color: var(--crimson);
  font-size: 11px;
}
.steps {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}
.step {
  padding: 1px 6px;
  border-radius: var(--r-1);
  background: var(--ink-3);
  color: var(--text-3);
  font-size: 10px;
}
.step.bad {
  background: var(--crimson-wash);
  color: var(--crimson);
}
</style>
