<script setup>
import { ref, watch } from 'vue'

// 头像由 CastLibrary 一次性批量传入（后端 ListCharacterPortraits），
// 本组件只负责显示，不再逐个调 GetCharacter —— 角色库打开就是快的。
const props = defineProps({
  subject: { type: Object, required: true },
  portrait: { type: Object, default: null },
})
defineEmits(['open'])

const url = ref('')
const state = ref('empty')

watch(
  () => props.portrait,
  (p) => {
    if (p && p.avatarUrl) {
      url.value = p.avatarUrl
      state.value = 'loading'
    } else {
      url.value = ''
      state.value = 'empty'
    }
  },
  { immediate: true }
)
</script>

<template>
  <button
    type="button"
    class="group flex min-w-0 flex-col items-center gap-2 rounded-lg p-2 text-center transition-colors hover:bg-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60"
    :aria-label="`查看${subject.name || '未命名角色'}的详情`"
    @click="$emit('open', subject)"
  >
    <span class="relative flex aspect-square w-full items-center justify-center overflow-hidden rounded-full border border-line bg-paper transition-colors group-hover:border-line-strong">
      <img
        v-if="url && state !== 'error'"
        :src="url"
        alt=""
        loading="lazy"
        class="h-full w-full object-cover object-top"
        @load="state = 'ready'"
        @error="state = 'error'"
      />
      <span v-if="state !== 'ready'" class="absolute inset-0 flex flex-col items-center justify-center gap-1 text-ink-dim">
        <svg v-if="state === 'empty'" class="h-8 w-8 opacity-80" viewBox="0 0 40 40" fill="none" aria-hidden="true">
          <circle cx="20" cy="14" r="8" stroke="currentColor" stroke-width="2.5" />
          <path d="M6 36c2-8 7-11 14-11s12 3 14 11" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
        </svg>
        <span class="text-2xs leading-none">{{ state === 'loading' ? '读取头像…' : state === 'error' ? '读取失败' : '待定' }}</span>
      </span>
    </span>
    <span class="w-full truncate text-sm font-medium text-ink">{{ subject.name || '未命名角色' }}</span>
    <span class="text-2xs text-ink-dim">
      {{ state === 'loading' ? '读取头像…' : state === 'error' ? '头像读取失败' : state === 'empty' ? '待定头像' : `${subject.asset_count} 张图` }}
    </span>
  </button>
</template>
