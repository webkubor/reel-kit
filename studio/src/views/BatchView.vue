<script setup>
/**
 * 批量 Prompt 生成器 —— 让"无数高赞武侠短片"的流水线真正跑起来
 *
 * 用户从 角色 × 神兵 × 镜头 × 时长 的笛卡尔积里勾选组合，工作台一次性生成 N 条
 * 可投喂 Seedance 2.0 的 Prompt。可以：
 *   - 一键复制全部（粘到外部 Agent / 手敲 museav）
 *   - 批量导入为草稿 shots（落到 /shots 账本，再去逐条出图）
 *
 * 不做实际出图：批量出图烧钱 + 不允许未经审视就批量花 token，先用 Prompt 草稿落账。
 */
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { api } from '../api';

const router = useRouter();

// 数据
const characters = ref([]);
const weapons = ref([]);
const skills = ref([]);
const bgmList = ref([]);

// 维度选项（与 PromptLabView 保持一致）
const MOODS = ['杀意', '试探', '绝望', '愤怒', '冷静'];
const POWERS = ['极致暴力', '点到为止', '飘逸灵动', '刚猛无俦'];
const RHYTHMS = ['快切急促', '流畅连贯', '沉稳大气'];
const SHOTS = ['近景', '中景', '远景', '特写', '全景'];
const LIGHTS = ['高对比硬光', '柔和体积光', '低调暗调', '冷白高调'];
const TONES = ['冷色系', '暖色系', '水墨古风', '写实原色'];
const GRAINS = ['强颗粒', '轻颗粒', '无颗粒'];
const DURATIONS = [5, 10, 15];
const MODELS = ['Seedance 2.0', 'Veo 3.1', '即梦 AI (Jimeng)'];

// 选中的多选项
const selectedChars = ref([]);
const selectedWeapons = ref([]);
const selectedMoods = ref(['杀意']);
const selectedPowers = ref(['极致暴力']);
const selectedRhythms = ref(['流畅连贯']);
const selectedShots = ref(['中景']);
const selectedLights = ref(['高对比硬光']);
const selectedTones = ref(['冷色系']);
const selectedGrains = ref(['强颗粒']);
const selectedDurations = ref([15]);
const selectedSkillId = ref('');
const sceneTemplate = ref('雪夜古巷，单人 Solo，敌人 4-8 人围攻');
const model = ref('Seedance 2.0');
const ratio = ref('16:9');
const copied = ref(null);
const importing = ref(false);
const importedCount = ref(0);

// 加载
async function load() {
  const [reg, w, sk, bgm] = await Promise.all([
    api.registry(),
    api.weapons(),
    api.skills(),
    api.bgm(),
  ]);
  characters.value = reg.characters;
  weapons.value = w.weapons;
  skills.value = sk.skills;
  bgmList.value = bgm.bgm?.blocks || [];
}

// 多选 toggle
function toggle(arr, item) {
  const i = arr.value.indexOf(item);
  if (i >= 0) arr.value.splice(i, 1);
  else arr.value.push(item);
}

// Prompt 模板（沿用 PromptLabView 的 S-A-C-S 结构）
const TONE_MAP = {
  '冷色系': '蓝青低饱和',
  '暖色系': '橙红高饱和',
  '水墨古风': '低饱和留白',
  '写实原色': '无滤镜',
};
const GRAIN_MAP = {
  '强颗粒': '35mm 粗颗粒',
  '轻颗粒': '数字+轻微',
  '无颗粒': '数字干净',
};
const CAMERA_MAP = {
  '杀意+极致暴力': '贴身短焦，低机位仰拍，急速切镜，hard light，暗调',
  '杀意+点到为止': '贴身短焦，一招即切，hard light，高对比',
  '杀意+飘逸灵动': '低机位仰拍，tracking shot，dynamic motion blur',
  '杀意+刚猛无俦': 'crane shot，大远景，低机位跟拍，体积光',
  '试探+极致暴力': '中景，平稳切镜，soft light',
  '试探+点到为止': '中景侧跟，少切镜，soft light，冷色调',
  '绝望+极致暴力': 'handheld 跟拍，急促切镜，低饱和暗调',
  '绝望+飘逸灵动': '贴身跟拍，慢切，手持晃动，灰调低饱和',
  '绝望+刚猛无俦': '低机位俯拍，大景别，慢切，体积光',
  '愤怒+极致暴力': '低机位，hard light，急促切镜，高饱和暖色',
  '愤怒+刚猛无俦': '低机位跟拍，大景别，体积光，中速切镜',
  '愤怒+点到为止': '贴身短焦，快速切镜，hard light',
  '冷静+点到为止': '中景侧跟，平稳切镜，soft light，冷色调',
  '冷静+飘逸灵动': '长镜头跟随，tracking shot，soft light',
  '冷静+刚猛无俦': 'crane shot，大远景，体积光，缓拉',
};

function buildSceneSection(c, w) {
  const cNames = c.map((x) => x.name).join('、') || '单人';
  const wNames = w.map((x) => `【${x.name}】`).join('+') || '无兵器';
  return `${sceneTemplate.value}。${cNames} 持 ${wNames}。`;
}

function buildActionSection(c) {
  const cNames = c.map((x) => x.name).join('、');
  return `${cNames} 主动出击，起手蓄力 → 极速位移 → 命中反馈 → 完整收势。Windup → Path → Impact → Recovery 闭环。0-4s 起势，4-10s 撞击，10-15s 余势。`;
}

function buildStyleSection(dur, tone, grain) {
  return `35mm 胶片颗粒，电影级低调摄影，${dur}s 24fps，${ratio.value} 写实高武，丁达尔体积光，自然光`;
}

function buildSkillHead() {
  if (!selectedSkillId.value) return '';
  const s = skills.value.find((x) => x.id === selectedSkillId.value);
  if (!s) return '';
  const triggers = (s.triggers || []).slice(0, 3).map((t) => `「${t}」`).join('、');
  const triggerLine = triggers ? `\n触发词：${triggers}` : '';
  return `[Skill 约束：${s.name}]\n${s.description || ''}${triggerLine}`;
}

function buildNegative() {
  return `约束：no extra characters, no crowd, no extra fingers, no extra limbs, no distortion, no deformed hands, no warped faces, no jump cuts, no lens flares, no text overlays, no watermarks, no blurry motion, no snap zooms, no magic circles, no glowing runes, no modern elements, no subtitles`;
}

function buildPrompt(combo) {
  const { chars, weapons: ws, mood, power, rhythm, shot, light, tone, grain, duration } = combo;
  const sceneSec = buildSceneSection(chars, ws);
  const actionSec = buildActionSection(chars);
  const camera = CAMERA_MAP[`${mood}+${power}`] || `${shot} 侧跟，平稳切镜`;
  const styleSec = buildStyleSection(duration, tone, grain);
  const skillHead = buildSkillHead();
  const head = skillHead ? `${skillHead}\n\n---\n\n` : '';
  return `${head}${duration}s 电影镜头，${ratio.value}，35mm 胶片感，${TONE_MAP[tone] || ''}，${GRAIN_MAP[grain] || ''}。${light}。\n\n[Scene]\n${sceneSec}\n\n[Action]\n${actionSec}\n\n[Camera]\n${shot} · ${rhythm} · ${camera}\n\n[Style]\n${styleSec}`;
}

// 笛卡尔积
const combos = computed(() => {
  const out = [];
  for (const c of selectedChars.value.length ? selectedChars.value : [{ name: '' }]) {
    for (const w of selectedWeapons.value.length ? selectedWeapons.value : [{ name: '' }]) {
      for (const mood of selectedMoods.value) {
        for (const power of selectedPowers.value) {
          for (const rhythm of selectedRhythms.value) {
            for (const shot of selectedShots.value) {
              for (const light of selectedLights.value) {
                for (const tone of selectedTones.value) {
                  for (const grain of selectedGrains.value) {
                    for (const dur of selectedDurations.value) {
                      out.push({
                        chars: c.name ? [c] : [],
                        weapons: w.name ? [w] : [],
                        mood, power, rhythm, shot, light, tone, grain,
                        duration: dur,
                      });
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
  return out;
});

const generated = computed(() =>
  combos.value.map((combo, i) => ({
    id: `batch-${i}`,
    combo,
    prompt: buildPrompt(combo),
    title: titleFor(combo),
    selected: true,
  })),
);

function titleFor(combo) {
  const chars = combo.chars.map((c) => c.name).join('·') || '无名';
  const weapons = combo.weapons.map((w) => w.name).join('·') || '无兵';
  return `${chars}/${weapons} · ${combo.mood}+${combo.power} · ${combo.duration}s`;
}

const totalCount = computed(() => generated.value.length);

const selectedGenerated = computed(() => generated.value.filter((g) => g.selected));

// 复制全部
async function copyAll() {
  const text = selectedGenerated.value
    .map((g) => `### ${g.title}\n\n${g.prompt}\n\n---\n`)
    .join('\n');
  await copyText(text, 'all');
}

// 复制负面约束
async function copyNegative() {
  await copyText(buildNegative(), 'neg');
}

// 复制单条
async function copyOne(g) {
  await copyText(g.prompt + '\n\n' + buildNegative(), g.id);
}

async function copyText(text, key) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = key;
    setTimeout(() => { if (copied.value === key) copied.value = null; }, 1500);
  } catch { /* */ }
}

// 批量导入为 shots
async function importToShots() {
  if (!selectedGenerated.value.length || importing.value) return;
  importing.value = true;
  let count = 0;
  try {
    for (const g of selectedGenerated.value) {
      await api.createShot({
        title: g.title,
        duration: g.combo.duration,
        prompt: g.prompt,
        ratio: ratio.value,
        model: model.value,
        characters: g.combo.chars.map((c) => c.name).filter(Boolean),
        refs: [],
        notes: `[批量生成] ${selectedSkillId.value ? 'skill=' + selectedSkillId.value + ' · ' : ''}组合: ${g.combo.mood}/${g.combo.power}/${g.combo.rhythm}/${g.combo.shot}/${g.combo.light}/${g.combo.tone}/${g.combo.grain}`,
      });
      count++;
    }
    importedCount.value = count;
  } catch (err) {
    alert('部分导入失败：' + err.message);
  } finally {
    importing.value = false;
  }
}

function gotoShots() { router.push('/shots'); }

onMounted(load);
</script>

<template>
  <div class="page">
    <header class="topbar">
      <div>
        <h1 class="topbar-title title-brush">批量 Prompt 生成器</h1>
        <span class="topbar-sub">BATCH PROMPT · 笛卡尔积一键生成 {{ totalCount }} 条 → 投喂 museav</span>
      </div>
      <div class="topbar-stats">
        <span class="stat-pill">{{ totalCount }} 条</span>
        <span class="stat-pill">已选 {{ selectedGenerated.length }}</span>
      </div>
    </header>

    <div class="layout">
      <!-- 左：维度选择器 -->
      <aside class="left scroll-y">
        <div class="block">
          <h3 class="block-title">场景模板</h3>
          <textarea v-model="sceneTemplate" rows="3" />
        </div>

        <div class="block">
          <h3 class="block-title">① 角色 <span class="dim mono">({{ selectedChars.length }})</span></h3>
          <div class="chip-grid">
            <button
              v-for="c in characters"
              :key="c.name"
              class="chip"
              :class="{ active: selectedChars.find((x) => x.name === c.name) }"
              @click="toggle(selectedChars, c)"
            >{{ c.name }}</button>
          </div>
          <p class="dim block-hint">不选 = "单人"</p>
        </div>

        <div class="block">
          <h3 class="block-title">② 神兵 <span class="dim mono">({{ selectedWeapons.length }})</span></h3>
          <div class="chip-grid">
            <button
              v-for="w in weapons"
              :key="w.index"
              class="chip chip-wide"
              :class="{ active: selectedWeapons.find((x) => x.name === w.name) }"
              @click="toggle(selectedWeapons, w)"
            >
              <span class="chip-num">{{ String(w.index).padStart(2, '0') }}</span>
              <span class="chip-label">【{{ w.name }}】</span>
            </button>
          </div>
          <p class="dim block-hint">不选 = "无兵器"</p>
        </div>

        <div class="block">
          <h3 class="block-title">③ 镜头偏好 <span class="dim mono">（多选 = 笛卡尔积）</span></h3>
          <div class="dim-block">
            <div v-for="(opts, key) in {
              '情绪': selectedMoods, '力量': selectedPowers, '节奏': selectedRhythms,
              '景别': selectedShots, '光影': selectedLights, '色调': selectedTones,
              '颗粒': selectedGrains, '时长': selectedDurations,
            }" :key="key" class="dim-row">
              <span class="dim-label">{{ key }}</span>
              <div class="opt-row">
                <button
                  v-for="opt in opts"
                  :key="opt"
                  class="opt"
                  @click="toggle(opts, opt)"
                >{{ opt }}{{ typeof opt === 'number' ? 's' : '' }}</button>
              </div>
            </div>
          </div>
        </div>

        <div class="block">
          <h3 class="block-title">④ 平台</h3>
          <div class="dim-block">
            <div class="dim-row">
              <span class="dim-label">模型</span>
              <select v-model="model" class="select">
                <option v-for="m in MODELS" :key="m">{{ m }}</option>
              </select>
            </div>
            <div class="dim-row">
              <span class="dim-label">画幅</span>
              <select v-model="ratio" class="select">
                <option>16:9</option>
                <option>9:16</option>
                <option>1:1</option>
              </select>
            </div>
          </div>
        </div>

        <div class="block">
          <h3 class="block-title">⑤ Skill 约束 <span class="dim mono">（可选）</span></h3>
          <div class="chip-grid">
            <button class="chip" :class="{ active: !selectedSkillId }" @click="selectedSkillId = ''">无</button>
            <button
              v-for="s in skills"
              :key="s.id"
              class="chip"
              :class="{ active: selectedSkillId === s.id }"
              @click="selectedSkillId = s.id"
            >{{ s.name }}</button>
          </div>
        </div>
      </aside>

      <!-- 右：生成结果 -->
      <main class="right scroll-y">
        <div class="action-bar">
          <div class="action-info dim mono">
            {{ selectedGenerated.length }} / {{ totalCount }} 条已选
          </div>
          <div class="action-buttons">
            <button class="btn" @click="copyNegative">
              {{ copied === 'neg' ? '✓ 已复制' : '复制负面约束' }}
            </button>
            <button class="btn" @click="copyAll">
              {{ copied === 'all' ? '✓ 已复制' : '复制全部 Prompt' }}
            </button>
            <button class="btn btn-primary" :disabled="importing || !selectedGenerated.length" @click="importToShots">
              {{ importing ? '导入中…' : `导入 ${selectedGenerated.length} 条为草稿 shots` }}
            </button>
            <button v-if="importedCount > 0" class="btn btn-jade" @click="gotoShots">
              ✓ 已导入 {{ importedCount }} 条 → 去 /shots 出图
            </button>
          </div>
        </div>

        <div v-if="!totalCount" class="empty dim">从 ① 角色 ② 神兵 ③ 镜头偏好 里勾选维度，工作台会按笛卡尔积生成 N 条 Prompt。</div>

        <div v-else class="card-list">
          <article v-for="g in generated" :key="g.id" class="card" :class="{ off: !g.selected }">
            <header class="card-head">
              <label class="card-check">
                <input type="checkbox" v-model="g.selected" />
                <span class="card-title title-brush">{{ g.title }}</span>
              </label>
              <div class="card-tags">
                <span v-for="(v, k) in {
                  '景': g.combo.shot, '力': g.combo.power, '光': g.combo.light,
                  '调': g.combo.tone, '节': g.combo.rhythm,
                }" :key="k" class="tag">{{ k }}:{{ v }}</span>
              </div>
              <button class="btn btn-sm" @click="copyOne(g)">
                {{ copied === g.id ? '✓' : '复制' }}
              </button>
            </header>
            <pre class="card-prompt mono">{{ g.prompt }}</pre>
          </article>
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; height: 100%; background: var(--ink-0); }
.topbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: var(--sp-4) var(--sp-5);
  border-bottom: 1px solid var(--line-1);
  background: var(--ink-1);
}
.topbar-title { font-size: 22px; line-height: 1.1; color: var(--gold-1); margin: 0; }
.topbar-sub { color: var(--text-3); font-size: 10px; letter-spacing: 0.22em; }
.topbar-stats { display: flex; gap: 8px; }
.stat-pill {
  padding: 4px 12px;
  background: var(--gold-wash);
  border: 1px solid var(--gold-2);
  border-radius: 999px;
  color: var(--gold-1);
  font-size: 11px;
  font-family: var(--font-mono);
}

.layout { display: grid; grid-template-columns: 320px 1fr; flex: 1; min-height: 0; }
.left { padding: var(--sp-4); border-right: 1px solid var(--line-1); background: var(--ink-1); }
.right { padding: var(--sp-4); background: var(--ink-0); }

.block { display: flex; flex-direction: column; gap: var(--sp-2); margin-bottom: var(--sp-4); }
.block-title {
  font-size: 10px; letter-spacing: 0.2em; color: var(--gold-2);
  text-transform: uppercase; margin: 0; padding-bottom: 4px;
  border-bottom: 1px solid var(--line-1);
}
.block-hint { margin: 2px 0 0; font-size: 10px; }

textarea {
  padding: 8px 10px; border: 1px solid var(--line-2); border-radius: var(--r-1);
  background: var(--ink-3); color: var(--text-1); font-family: inherit; font-size: 12px; resize: vertical;
}
textarea:focus { outline: none; border-color: var(--gold-2); }

.chip-grid { display: flex; flex-wrap: wrap; gap: 4px; }
.chip {
  display: flex; align-items: center; gap: 6px;
  padding: 4px 8px; background: var(--ink-2);
  border: 1px solid var(--line-2); border-radius: var(--r-1);
  color: var(--text-2); font-size: 11px; cursor: pointer; transition: all 0.12s;
}
.chip:hover { color: var(--text-1); border-color: var(--line-gold); }
.chip.active { background: var(--gold-wash); border-color: var(--gold-2); color: var(--gold-1); }
.chip-wide { padding: 4px 10px; }
.chip-num { font-family: var(--font-mono); font-size: 9px; color: var(--gold-2); }

.dim-block { display: flex; flex-direction: column; gap: var(--sp-2); padding: var(--sp-3); background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); }
.dim-row { display: grid; grid-template-columns: 50px 1fr; gap: var(--sp-2); align-items: center; }
.dim-label { font-size: 10px; color: var(--text-3); letter-spacing: 0.15em; }
.opt-row { display: flex; flex-wrap: wrap; gap: 3px; }
.opt {
  padding: 3px 8px; font-size: 11px; color: var(--text-2);
  background: var(--ink-3); border: 1px solid var(--line-1);
  border-radius: var(--r-1); cursor: pointer; font-family: inherit;
}
.opt:hover { color: var(--text-1); border-color: var(--line-gold); }

.select {
  padding: 4px 8px; border: 1px solid var(--line-2); border-radius: var(--r-1);
  background: var(--ink-3); color: var(--text-1); font-size: 12px; font-family: inherit;
}

.action-bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: var(--sp-3); margin-bottom: var(--sp-4);
  background: var(--ink-2); border: 1px solid var(--line-gold); border-radius: var(--r-2);
}
.action-buttons { display: flex; gap: 8px; flex-wrap: wrap; }

.btn {
  padding: 6px 14px; border: 1px solid var(--line-2); border-radius: var(--r-1);
  background: var(--ink-3); color: var(--text-1); font-family: inherit; font-size: 12px; cursor: pointer;
}
.btn:hover { border-color: var(--line-gold); }
.btn-sm { padding: 4px 10px; font-size: 11px; }
.btn-primary { background: var(--gold-2); border-color: var(--gold-2); color: var(--ink-1); font-weight: 600; }
.btn-primary:hover { background: var(--gold-1); }
.btn-jade { background: var(--jade); border-color: var(--jade); color: var(--ink-1); font-weight: 600; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }

.empty { padding: var(--sp-6); text-align: center; font-size: 12px; }

.card-list { display: flex; flex-direction: column; gap: var(--sp-3); }
.card {
  padding: var(--sp-3); background: var(--ink-1);
  border: 1px solid var(--line-2); border-radius: var(--r-2);
  transition: opacity 0.15s;
}
.card.off { opacity: 0.5; }
.card-head {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: var(--sp-2);
}
.card-check { display: flex; align-items: center; gap: 8px; cursor: pointer; }
.card-title { font-size: 13px; }
.card-tags { display: flex; gap: 4px; flex-wrap: wrap; flex: 1; justify-content: center; }
.tag {
  padding: 1px 6px; background: var(--ink-3); border: 1px solid var(--line-2);
  border-radius: var(--r-1); color: var(--text-2); font-size: 10px;
  font-family: var(--font-mono);
}
.card-prompt {
  padding: var(--sp-3); background: var(--ink-3); border: 1px solid var(--line-1);
  border-radius: var(--r-1); font-size: 11px; line-height: 1.6;
  white-space: pre-wrap; word-break: break-word;
  max-height: 240px; overflow-y: auto;
}
</style>