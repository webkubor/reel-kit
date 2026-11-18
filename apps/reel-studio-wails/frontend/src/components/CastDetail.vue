<script setup>
import { ref, watch, computed, nextTick } from 'vue'
import { loadCharacter, groupReferences, VIEWS } from '../lib/character.js'

const props = defineProps({ subject: { type: Object, default: null } })
const emit = defineEmits(['close', 'shoot'])

const dialog = ref(null)
const character = ref(null)
const error = ref('')
const preview = ref(null)

watch(() => props.subject, async (s) => {
  character.value = null
  error.value = ''
  preview.value = null
  if (!s) { dialog.value?.open && dialog.value.close(); return }
  await nextTick()
  if (!dialog.value.open) dialog.value.showModal()
  loadCharacter(s.id)
    .then((c) => { if (props.subject?.id === s.id) character.value = c })
    .catch((e) => { if (props.subject?.id === s.id) error.value = String(e?.message || e) })
})

const groups = computed(() => groupReferences(character.value))
const byView = computed(() => Object.fromEntries((character.value?.assets || []).map((a) => [a.view, a])))
const viewCount = computed(() => VIEWS.filter((v) => byView.value[v.key]).length)
const traits = computed(() => (character.value?.traits || []).filter((t) => t.value))
const persona = computed(() => ({ virtual: '虚拟人', real: '真人' })[character.value?.persona] || '')
const canShoot = computed(() => (character.value?.references || []).length || (character.value?.assets || []).length)
</script>

<template>
  <dialog
    ref="dialog"
    class="w-[860px] max-w-[94vw] rounded-lg border border-line bg-surface p-0 text-ink shadow-pop"
    aria-labelledby="cast-detail-title"
    @close="emit('close')"
  >
    <div v-if="subject" class="flex max-h-[86vh] flex-col">
      <header class="flex items-start justify-between gap-3 border-b border-line px-5 py-4">
        <div class="min-w-0">
          <h2 id="cast-detail-title" class="truncate text-lg font-semibold tracking-tight">{{ subject.name }}</h2>
          <p class="mt-0.5 text-2xs text-ink-lo">
            <span v-if="persona">{{ persona }} · </span>
            <span v-if="character">{{ character.complete ? '已建好' : '未建好' }}</span>
            <span v-else-if="!error">读取中…</span>
          </p>
        </div>
        <div class="flex shrink-0 gap-2">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="!canShoot" @click="emit('shoot', character)">出镜</button>
          <button type="button" class="btn btn-ghost btn-sm" @click="dialog.close()">关闭</button>
        </div>
      </header>

      <div class="overflow-y-auto px-5 py-4">
        <p v-if="error" class="help help-error">读不到角色详情：{{ error }}</p>
        <p v-else-if="!character" class="text-2xs text-ink-lo">正在从中台读取角色物料…</p>

        <template v-else>
          <section v-if="groups.realFaces.length" class="mb-5">
            <h3 class="label">真人参考 <span class="label-hint">{{ groups.realFaces.length }} 张 · 非角色形象</span></h3>
            <p class="mb-2 rounded-md border border-dashed border-line px-3 py-2 text-2xs leading-relaxed text-ink-lo">
              这是给 AI 建模参考的真人照片，不是这个角色在剧里的形象；出镜不会用它锁定长相。
            </p>
            <div class="grid grid-cols-3 gap-3">
              <button v-for="a in groups.realFaces" :key="a.id || a.url" type="button"
                class="overflow-hidden rounded-md border border-line bg-paper text-left transition-colors hover:border-line-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60"
                @click="preview = a">
                <img :src="a.url" :alt="a.name" loading="lazy" class="block h-36 w-full object-contain" />
                <span class="block truncate px-2.5 py-1.5 text-2xs text-ink-lo">{{ a.name }}</span>
              </button>
            </div>
          </section>

          <section v-if="groups.avatars.length" class="mb-5">
            <h3 class="label">头像 <span class="label-hint">{{ groups.avatars.length }} 张</span></h3>
            <div class="grid grid-cols-3 gap-3">
              <button v-for="a in groups.avatars" :key="a.id || a.url" type="button"
                class="overflow-hidden rounded-md border border-line bg-paper text-left transition-colors hover:border-line-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60"
                @click="preview = a">
                <img :src="a.url" :alt="a.name" loading="lazy" class="block h-40 w-full object-contain" />
                <span class="block truncate px-2.5 py-1.5 text-2xs text-ink-lo">{{ a.name }}</span>
              </button>
            </div>
          </section>

          <section v-if="groups.costumes.length" class="mb-5">
            <h3 class="label">定妆立绘 <span class="label-hint">{{ groups.costumes.length }} 张</span></h3>
            <div class="grid grid-cols-2 gap-3">
              <button v-for="a in groups.costumes" :key="a.id || a.url" type="button"
                class="overflow-hidden rounded-md border border-line bg-paper text-left transition-colors hover:border-line-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60"
                @click="preview = a">
                <img :src="a.url" :alt="a.name" loading="lazy" class="block h-48 w-full object-contain" />
                <span class="block truncate px-2.5 py-1.5 text-2xs text-ink-lo">{{ a.name }}</span>
              </button>
            </div>
          </section>

          <section v-if="groups.sheets.length" class="mb-5">
            <h3 class="label">三视图 <span class="label-hint">{{ groups.sheets.length }} 张</span></h3>
            <div class="grid grid-cols-2 gap-3">
              <button v-for="a in groups.sheets" :key="a.id || a.url" type="button"
                class="overflow-hidden rounded-md border border-line bg-paper text-left transition-colors hover:border-line-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60"
                @click="preview = a">
                <img :src="a.url" :alt="a.name" loading="lazy" class="block h-48 w-full object-contain" />
                <span class="block truncate px-2.5 py-1.5 text-2xs text-ink-lo">{{ a.name }}</span>
              </button>
            </div>
          </section>

          <section v-if="groups.scenes.length" class="mb-5">
            <h3 class="label">画面素材 <span class="label-hint">{{ groups.scenes.length }} 张 · 不参与角色物料</span></h3>
            <div class="grid grid-cols-2 gap-3">
              <button v-for="a in groups.scenes" :key="a.id || a.url" type="button"
                class="overflow-hidden rounded-md border border-line bg-paper text-left transition-colors hover:border-line-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60"
                @click="preview = a">
                <img :src="a.url" :alt="a.name" loading="lazy" class="block h-40 w-full object-contain" />
                <span class="block truncate px-2.5 py-1.5 text-2xs text-ink-lo">{{ a.name }}</span>
              </button>
            </div>
          </section>

          <section class="mb-5">
            <h3 class="label">五视图 <span class="label-hint">{{ viewCount }} / 5</span></h3>
            <p v-if="!viewCount" class="rounded-md border border-dashed border-line px-3 py-2.5 text-2xs leading-relaxed text-ink-lo">
              还没有独立五视图。出镜目前用上面的定妆稿锁定长相；补齐五视图后，转面和侧身镜头会更稳。
            </p>
            <div v-else class="grid grid-cols-5 gap-2">
              <div v-for="v in VIEWS" :key="v.key"
                class="flex aspect-[3/4] flex-col overflow-hidden rounded-md border bg-paper"
                :class="byView[v.key] ? 'border-line' : 'border-dashed border-line'">
                <img v-if="byView[v.key]" :src="byView[v.key].url" :alt="`${subject.name} ${v.label}`" loading="lazy" class="min-h-0 flex-1 object-cover object-top" />
                <span v-else class="flex flex-1 items-center justify-center text-2xs text-ink-dim">缺</span>
                <span class="px-1.5 py-1 text-center text-2xs text-ink-lo">{{ v.label }}</span>
              </div>
            </div>
          </section>

          <div class="grid gap-5 md:grid-cols-2">
            <section v-if="traits.length">
              <h3 class="label">人物设定</h3>
              <dl class="space-y-1.5 text-xs">
                <div v-for="t in traits" :key="t.key" class="grid grid-cols-[4rem_1fr] gap-2">
                  <dt class="text-ink-lo">{{ t.key }}</dt>
                  <dd class="leading-relaxed text-ink-mid">{{ t.value }}</dd>
                </div>
              </dl>
            </section>

            <section>
              <template v-if="character.looks?.length">
                <h3 class="label">装扮</h3>
                <div class="mb-4 flex flex-wrap gap-1.5">
                  <span v-for="l in character.looks" :key="l" class="badge">{{ l }}</span>
                </div>
              </template>
              <template v-if="groups.weapons.length">
                <h3 class="label">兵器与道具</h3>
                <div class="mb-4 grid grid-cols-3 gap-2">
                  <button v-for="a in groups.weapons" :key="a.id || a.url" type="button"
                    class="overflow-hidden rounded-md border border-line bg-paper hover:border-line-strong" @click="preview = a">
                    <img :src="a.url" :alt="a.name" loading="lazy" class="block aspect-square w-full object-contain" />
                  </button>
                </div>
              </template>
              <template v-if="character.missing?.length">
                <h3 class="label">还差</h3>
                <p class="mb-3 text-2xs leading-relaxed text-warn">{{ character.missing.join('、') }}</p>
              </template>
              <template v-if="character.warnings?.length">
                <h3 class="label">注意</h3>
                <p class="text-2xs leading-relaxed text-ink-lo">{{ character.warnings.join('、') }}</p>
              </template>
            </section>
          </div>
        </template>
      </div>
    </div>

    <div v-if="preview" class="fixed inset-0 z-10 flex flex-col items-center justify-center gap-3 bg-black/85 p-6" @click="preview = null">
      <img :src="preview.url" :alt="preview.name" class="max-h-[80vh] max-w-full object-contain" />
      <p class="text-xs text-ink-lo">{{ preview.name }} · 点击任意处关闭</p>
    </div>
  </dialog>
</template>
