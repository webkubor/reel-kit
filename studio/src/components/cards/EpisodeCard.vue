<script setup>
/**
 * 剧集海报卡片 —— 16:9 海报，可截图分享
 *
 * 设计基线：暗夜黑金 35mm 电影感
 *   - 背景：深墨黑 + 宣纸噪点
 *   - 标题：楷书大字（系统字体替代） + 金色
 *   - 装饰：朱砂章 + 雪花纹 + 金边
 *   - 信息：ep 编号、状态、剧名、核心看点、统计
 */
import { computed } from 'vue';

const props = defineProps({
  ep: { type: Object, required: true },
  season: { type: String, default: 's1' },
  seasonName: { type: String, default: '第一季' },
});

const KIND_LABEL = {
  prologue: '序幕',
  'structured-15s': '15s 结构',
  'detailed-scenes': '场景分镜',
  outline: '剧情大纲',
  storyboard: '5 段分镜',
  pending: '待开发',
};

const STATUS_COLOR = {
  finalized: '#c5a059',
  published: '#5f8f6a',
  completed: '#5b7f9c',
  pending: '#6b675f',
};

const seasonText = computed(() => `${props.seasonName} · 沸腾之雪`);
const kindLabel = computed(() => KIND_LABEL[props.ep.kind] || props.ep.kind);
const statusColor = computed(() => STATUS_COLOR[props.ep.status] || STATUS_COLOR.pending);
const statusLabel = computed(() => props.ep.statusLabel || '待开发');
const epNum = computed(() => String(props.ep.ep).padStart(2, '0'));
const cleanTitle = computed(() => {
  // 去掉《》
  let t = props.ep.title || '';
  t = t.replace(/^第[\d零一二三四五六七八九十]+集[：:]\s*【?/, '').replace(/】$/, '');
  t = t.replace(/^《|》$/g, '');
  return t;
});
const subTitle = computed(() => {
  const t = props.ep.title || '';
  const m = t.match(/^第[\d零一二三四五六七八九十]+集[：:]\s*(.+?)(?:[（(](.+?)[）)])?$/);
  if (m) {
    if (m[2]) return `（${m[2]}）`;
    return '';
  }
  return '';
});
</script>

<template>
  <div class="card" :data-ep="epNum">
    <!-- 噪点背景 -->
    <div class="card-bg" />

    <!-- 顶部：金边 + 季名 -->
    <header class="card-head">
      <div class="card-season">{{ seasonText }}</div>
      <div class="card-status" :style="{ color: statusColor, borderColor: statusColor }">
        {{ statusLabel }}
      </div>
    </header>

    <!-- ep 编号 + 序号 -->
    <div class="card-num-row">
      <div class="card-num">第 {{ epNum }} 集</div>
      <div class="card-num-bg" :style="{ color: statusColor }">{{ epNum }}</div>
    </div>

    <!-- 标题 -->
    <div class="card-title-wrap">
      <h1 class="card-title">{{ cleanTitle }}</h1>
      <p v-if="subTitle" class="card-subtitle">{{ subTitle }}</p>
    </div>

    <!-- 核心看点 -->
    <p v-if="ep.core" class="card-core">{{ ep.core }}</p>

    <!-- 装饰：朱砂章 -->
    <div class="card-stamp">
      <div class="stamp-char">{{ epNum }}</div>
      <div class="stamp-sub">EP</div>
    </div>

    <!-- 统计 + 底部 -->
    <footer class="card-foot">
      <div class="card-stats">
        <div class="stat">
          <div class="stat-num">{{ ep.shotCount || 0 }}</div>
          <div class="stat-label">分镜</div>
        </div>
        <div class="stat">
          <div class="stat-num">{{ ep.jimengCount || 0 }}</div>
          <div class="stat-label">指令</div>
        </div>
        <div class="stat">
          <div class="stat-num">{{ ep.voiceoverCount || 0 }}</div>
          <div class="stat-label">对白</div>
        </div>
        <div class="stat">
          <div class="stat-num">{{ kindLabel }}</div>
          <div class="stat-label">格式</div>
        </div>
      </div>
      <div class="card-brand">
        <div class="brand-title">沸雪</div>
        <div class="brand-sub">BOILING SNOW · {{ seasonName.toUpperCase() }}</div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.card {
  position: relative;
  width: 960px;
  height: 540px;
  padding: 48px 56px;
  background: #0b0b0d;
  color: #e8e6e1;
  font-family: 'Songti SC', 'STSong', 'SimSun', serif;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
}
.card-bg {
  position: absolute;
  inset: 0;
  background-image: url("data:image/svg+xml;utf8,<svg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='3'/></filter><rect width='100%' height='100% filter='url(%23n)' opacity='0.04'/></svg>");
  background-attachment: fixed;
  pointer-events: none;
}
.card-head {
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 16px;
  border-bottom: 1px solid rgba(197, 160, 89, 0.3);
  margin-bottom: 32px;
}
.card-season {
  font-size: 12px;
  letter-spacing: 0.3em;
  color: #c5a059;
  font-family: -apple-system, sans-serif;
}
.card-status {
  padding: 3px 12px;
  border: 1px solid;
  border-radius: 2px;
  font-size: 11px;
  letter-spacing: 0.2em;
  font-family: -apple-system, sans-serif;
}
.card-num-row {
  position: relative;
  margin-bottom: 24px;
}
.card-num {
  position: relative;
  z-index: 2;
  font-size: 18px;
  letter-spacing: 0.3em;
  color: #d9bd84;
  font-family: -apple-system, sans-serif;
}
.card-num-bg {
  position: absolute;
  top: -32px;
  right: 0;
  font-size: 200px;
  line-height: 1;
  font-weight: 700;
  font-family: 'SF Mono', 'JetBrains Mono', monospace;
  opacity: 0.08;
  letter-spacing: -0.05em;
  z-index: 1;
}
.card-title-wrap {
  position: relative;
  margin-bottom: 24px;
}
.card-title {
  margin: 0;
  font-size: 64px;
  font-weight: 700;
  line-height: 1.1;
  color: #d9bd84;
  letter-spacing: 0.08em;
  text-shadow: 0 2px 8px rgba(0, 0, 0, 0.6);
}
.card-subtitle {
  margin: 8px 0 0;
  font-size: 18px;
  color: #a29d94;
  letter-spacing: 0.1em;
}
.card-core {
  position: relative;
  margin: 0;
  font-size: 16px;
  line-height: 1.7;
  color: #c5b89e;
  padding: 16px 20px;
  background: rgba(18, 18, 22, 0.6);
  border-left: 2px solid #c5a059;
  border-radius: 0 2px 2px 0;
  max-width: 700px;
  font-family: -apple-system, 'PingFang SC', 'Noto Sans SC', sans-serif;
}
.card-stamp {
  position: absolute;
  top: 48px;
  right: 56px;
  width: 80px;
  height: 80px;
  background: #a8332a;
  color: #f5f1e8;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  transform: rotate(-6deg);
  border: 2px solid #8b2424;
  box-shadow: 0 2px 8px rgba(168, 51, 42, 0.3);
  z-index: 3;
}
.stamp-char {
  font-size: 32px;
  font-weight: 700;
  line-height: 1;
  font-family: -apple-system, monospace;
}
.stamp-sub {
  font-size: 9px;
  letter-spacing: 0.3em;
  margin-top: 2px;
}
.card-foot {
  position: absolute;
  bottom: 48px;
  left: 56px;
  right: 56px;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
}
.card-stats {
  display: flex;
  gap: 32px;
  font-family: -apple-system, sans-serif;
}
.stat {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.stat-num {
  font-size: 24px;
  color: #d9bd84;
  font-weight: 700;
  font-family: 'SF Mono', monospace;
}
.stat-label {
  font-size: 9px;
  color: #6b675f;
  letter-spacing: 0.2em;
  margin-top: 2px;
}
.card-brand {
  text-align: right;
}
.brand-title {
  font-size: 20px;
  color: #c5a059;
  letter-spacing: 0.2em;
  font-family: -apple-system, sans-serif;
  font-weight: 700;
}
.brand-sub {
  font-size: 9px;
  color: #6b675f;
  letter-spacing: 0.25em;
  margin-top: 2px;
  font-family: 'SF Mono', monospace;
}
</style>
