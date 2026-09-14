<script setup>
/**
 * 经典案例卡片 —— 天榜 X 角色 Solo 15s 海报
 *
 * 设计：暗夜黑金 + 朱砂章 + 印章式天榜徽章
 *   - 顶部: 季名 + Rank 徽章
 *   - 角色名 (楷书大字)
 *   - 视觉逻辑 一句话
 *   - 动态节奏 / 环境背景 (双列)
 *   - 视觉符号 (大字【X】)
 *   - 底部: BGM/时长/brand
 */
import { computed } from 'vue';

const props = defineProps({
  case: { type: Object, required: true },
  season: { type: String, default: 's1' },
  seasonName: { type: String, default: '第一季' },
});

// `case` 是 JS 关键字，模板里不能直接用。转一道。
const item = computed(() => props.case);

const rankColor = computed(() => {
  if (item.value.isPrologue) return '#a8332a';
  if (['一', '二', '三'].includes(item.value.rank)) return '#d9bd84';
  return '#5b7f9c';
});

const seasonText = computed(() => `${props.seasonName} · 沸腾之雪`);
const subtitle = computed(() => {
  if (item.value.isPrologue) return '天榜引子 · 15s · 万宝楼黑市';
  return `天榜第 ${item.value.rank} · 15s Solo · 写实高武`;
});
</script>

<template>
  <div class="case-card">
    <!-- 噪点背景 -->
    <div class="card-bg" />

    <!-- 顶部 -->
    <header class="case-head">
      <div class="case-season">{{ seasonText }}</div>
      <div class="case-rank" :style="{ color: rankColor, borderColor: rankColor }">
        <span v-if="!item.isPrologue" class="case-rank-num">{{ item.rank }}</span>
        <span v-else>序章</span>
      </div>
    </header>

    <!-- 序号底纹 -->
    <div class="case-num-bg" :style="{ color: rankColor }">
      {{ item.isPrologue ? '序' : item.rank }}
    </div>

    <!-- 角色名 -->
    <div class="case-character-wrap">
      <div v-if="item.isPrologue" class="case-pre-title">天榜引子</div>
      <h1 class="case-character">{{ item.character || '天榜引子' }}</h1>
      <div class="case-subtitle">{{ subtitle }}</div>
    </div>

    <!-- 视觉逻辑 一句话 -->
    <blockquote v-if="item.visualLogic" class="case-quote">
      "{{ item.visualLogic }}"
    </blockquote>

    <!-- 双列:节奏 / 环境 -->
    <div class="case-meta">
      <div v-if="item.rhythm" class="meta-col">
        <div class="meta-label">▷ 动态节奏</div>
        <div class="meta-text">{{ item.rhythm }}</div>
      </div>
      <div v-if="item.env" class="meta-col">
        <div class="meta-label">◉ 环境背景</div>
        <div class="meta-text">{{ item.env }}</div>
      </div>
    </div>

    <!-- 视觉符号 -->
    <div v-if="item.visualSymbol" class="case-symbol" :style="{ color: rankColor, borderColor: rankColor }">
      <span class="symbol-char">【{{ item.visualSymbol }}】</span>
    </div>

    <!-- 底部 -->
    <footer class="case-foot">
      <div class="case-stats">
        <div class="stat">
          <div class="stat-num">15s</div>
          <div class="stat-label">时长</div>
        </div>
        <div class="stat">
          <div class="stat-num">24fps</div>
          <div class="stat-label">帧率</div>
        </div>
        <div class="stat">
          <div class="stat-num">16:9</div>
          <div class="stat-label">画幅</div>
        </div>
        <div class="stat">
          <div class="stat-num">35mm</div>
          <div class="stat-label">胶片</div>
        </div>
      </div>
      <div class="case-brand">
        <div class="brand-title">沸雪</div>
        <div class="brand-sub">CLASSIC CASE · {{ seasonName.toUpperCase() }}</div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.case-card {
  position: relative;
  width: 960px;
  height: 720px; /* 4:3 比例更适合案例展示 */
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
  pointer-events: none;
}
.case-head {
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 16px;
  border-bottom: 1px solid rgba(197, 160, 89, 0.3);
  margin-bottom: 28px;
  z-index: 2;
}
.case-season {
  font-size: 12px;
  letter-spacing: 0.3em;
  color: #c5a059;
  font-family: -apple-system, sans-serif;
}
.case-rank {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  border: 2px solid;
  border-radius: 50%;
  font-family: -apple-system, sans-serif;
  font-weight: 700;
  background: rgba(11, 11, 13, 0.6);
}
.case-rank-num {
  font-size: 32px;
  letter-spacing: 0;
}
.case-num-bg {
  position: absolute;
  top: 80px;
  right: 40px;
  font-size: 280px;
  line-height: 0.8;
  font-weight: 700;
  font-family: 'SF Mono', 'JetBrains Mono', monospace;
  opacity: 0.06;
  letter-spacing: -0.05em;
  z-index: 0;
  pointer-events: none;
}
.case-character-wrap {
  position: relative;
  margin-bottom: 24px;
  z-index: 2;
}
.case-pre-title {
  font-size: 14px;
  color: #a29d94;
  letter-spacing: 0.3em;
  margin-bottom: 4px;
  font-family: -apple-system, sans-serif;
}
.case-character {
  margin: 0 0 8px;
  font-size: 80px;
  font-weight: 700;
  line-height: 1;
  color: #d9bd84;
  letter-spacing: 0.12em;
  text-shadow: 0 2px 12px rgba(0, 0, 0, 0.7);
}
.case-subtitle {
  font-size: 14px;
  color: #a29d94;
  letter-spacing: 0.1em;
  font-family: -apple-system, sans-serif;
}
.case-quote {
  position: relative;
  margin: 0 0 28px;
  padding: 16px 24px;
  font-size: 18px;
  font-style: italic;
  line-height: 1.6;
  color: #c5b89e;
  background: rgba(18, 18, 22, 0.7);
  border-left: 3px solid #c5a059;
  border-radius: 0 2px 2px 0;
  font-family: 'Songti SC', serif;
  z-index: 2;
}
.case-meta {
  position: relative;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  margin-bottom: 28px;
  z-index: 2;
}
.meta-col {
  padding: 12px 16px;
  background: rgba(18, 18, 22, 0.5);
  border: 1px solid rgba(197, 160, 89, 0.15);
  border-radius: 2px;
}
.meta-label {
  font-size: 10px;
  color: #c5a059;
  letter-spacing: 0.2em;
  margin-bottom: 6px;
  font-family: -apple-system, sans-serif;
}
.meta-text {
  font-size: 13px;
  line-height: 1.6;
  color: #c5b89e;
  font-family: -apple-system, 'PingFang SC', 'Noto Sans SC', sans-serif;
}
.case-symbol {
  position: absolute;
  bottom: 140px;
  right: 56px;
  padding: 8px 24px;
  border: 2px solid;
  border-radius: 2px;
  background: rgba(11, 11, 13, 0.8);
  font-family: 'Songti SC', serif;
  font-weight: 700;
  z-index: 2;
}
.symbol-char {
  font-size: 36px;
  letter-spacing: 0.1em;
}
.case-foot {
  position: absolute;
  bottom: 32px;
  left: 56px;
  right: 56px;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  z-index: 2;
}
.case-stats {
  display: flex;
  gap: 28px;
  font-family: -apple-system, sans-serif;
}
.stat { display: flex; flex-direction: column; align-items: center; }
.stat-num { font-size: 20px; color: #d9bd84; font-weight: 700; font-family: 'SF Mono', monospace; }
.stat-label { font-size: 9px; color: #6b675f; letter-spacing: 0.2em; margin-top: 2px; }
.case-brand { text-align: right; }
.brand-title { font-size: 20px; color: #c5a059; letter-spacing: 0.2em; font-family: -apple-system, sans-serif; font-weight: 700; }
.brand-sub { font-size: 9px; color: #6b675f; letter-spacing: 0.25em; margin-top: 2px; font-family: 'SF Mono', monospace; }
</style>
