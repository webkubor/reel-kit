<script setup>
import { ref, computed } from 'vue'
import CastTile from './CastTile.vue'
import CastDetail from './CastDetail.vue'

const props = defineProps({ store: { type: Object, required: true } })

const query = ref('')
const selected = ref(null)

const people = computed(() => props.store.subjects.filter((s) => s.kind === 'person'))
const list = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return people.value
  return people.value.filter((s) => ((s.name || '') + ' ' + JSON.stringify(s.traits || {})).toLowerCase().includes(q))
})
const portraits = computed(() => Object.fromEntries((props.store.portraits || []).map((p) => [p.id, p])))
function portraitOf(s) {
  return portraits.value[s.id] || null
}
</script>

<template>
  <div class="panel">
    <div class="panel-head">
      <h2 class="panel-title">角色</h2>
      <div class="flex items-center gap-2">
        <span class="badge">{{ list.length }} 个</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="store.onTemplate()">素材模板</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="store.onCreate()">＋ 新建角色</button>
      </div>
    </div>
    <div class="panel-body">
      <label class="label" for="cast-search">搜索</label>
      <input id="cast-search" v-model="query" class="input input-md" placeholder="按名字或武器查" />

      <p v-if="store.error" class="mt-3 text-2xs leading-relaxed text-ink-lo">{{ store.error }}</p>
      <p v-else-if="!list.length" class="mt-3 text-2xs leading-relaxed text-ink-lo">
        {{ query ? `没有匹配「${query}」的角色。` : '还没有角色。' }}
      </p>
      <div v-else class="mt-4 grid grid-cols-[repeat(auto-fill,minmax(7.5rem,1fr))] gap-x-3 gap-y-4">
        <CastTile v-for="s in list" :key="`${store.projectId}:${s.id}`" :subject="s" :portrait="portraitOf(s)" @open="selected = $event" />
      </div>
    </div>
  </div>

  <CastDetail :subject="selected" @close="selected = null" @shoot="store.onShoot($event)" />
</template>
