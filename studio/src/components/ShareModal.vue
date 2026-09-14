<script setup>
/**
 * 通用分享模态 —— 预览卡片 + 下载 + 保存到 .state
 *
 * 用法：
 *   <ShareModal v-model:open="show" :title="..." :preview-ref="cardEl">
 *     <EpisodeCard :ep="..." ref="cardEl" />
 *   </ShareModal>
 */
import { ref, onMounted, watch, nextTick } from 'vue';
import { api } from '../api';
import { screenshotDownload, screenshot } from '../lib/screenshot.js';

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: '分享卡片' },
  filename: { type: String, default: 'card.png' },
  // 卡片 metadata（用于 .state/captures 索引）
  meta: { type: Object, default: () => ({}) },
});
const emit = defineEmits(['update:open', 'saved']);

const cardEl = ref(null);
const previewing = ref(false);
const saving = ref(false);
const saved = ref(null); // {id, url}

async function download() {
  if (!cardEl.value) return;
  await screenshotDownload(cardEl.value, props.filename);
}

async function save() {
  if (!cardEl.value) return;
  saving.value = true;
  try {
    const blob = await screenshot(cardEl.value);
    const result = await api.uploadCapture(blob, {
      filename: props.filename,
      ...props.meta,
    });
    saved.value = result;
    emit('saved', result);
  } finally {
    saving.value = false;
  }
}

function close() {
  emit('update:open', false);
  saved.value = null;
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="modal-overlay" @click.self="close">
      <div class="modal">
        <header class="modal-head">
          <h2 class="modal-title">{{ title }}</h2>
          <button class="modal-close" @click="close">×</button>
        </header>
        <div class="modal-body">
          <div class="card-stage">
            <slot />
          </div>
        </div>
        <footer class="modal-foot">
          <div class="dim mono" v-if="saved">
            ✓ 已存到 .state/captures/{{ saved.id }}.png
          </div>
          <div class="modal-actions">
            <button class="btn" :disabled="!cardEl" @click="download">
              ⬇ 下载 PNG
            </button>
            <button class="btn btn-primary" :disabled="!cardEl || saving" @click="save">
              {{ saving ? '保存中…' : '💾 保存到 .state' }}
            </button>
            <button class="btn" @click="close">关闭</button>
          </div>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.75);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 32px;
}
.modal {
  display: flex;
  flex-direction: column;
  max-width: 1100px;
  max-height: calc(100vh - 64px);
  background: #121216;
  border: 1px solid #2e2e37;
  border-radius: 8px;
  box-shadow: 0 16px 64px rgba(0, 0, 0, 0.6);
  overflow: hidden;
}
.modal-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid #2e2e37;
}
.modal-title {
  margin: 0;
  font-size: 16px;
  color: #d9bd84;
  font-family: 'Songti SC', 'STSong', serif;
  letter-spacing: 0.1em;
}
.modal-close {
  border: none;
  background: none;
  color: #6b675f;
  font-size: 24px;
  cursor: pointer;
  padding: 0;
  width: 24px;
  height: 24px;
  line-height: 1;
}
.modal-close:hover { color: #e8e6e1; }
.modal-body {
  flex: 1;
  overflow: auto;
  padding: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0b0b0d;
}
.card-stage {
  transform-origin: top center;
}
.modal-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  border-top: 1px solid #2e2e37;
  background: #1a1a1f;
  gap: 16px;
}
.modal-actions { display: flex; gap: 8px; margin-left: auto; }
</style>
