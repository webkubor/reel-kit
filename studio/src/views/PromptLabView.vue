<script setup>
/**
 * Prompt 实验室 —— 把创意法典 + 角色 + 神兵 + 镜头美学自动套用成 S-A-C-S 结构化 Prompt
 *
 * 左侧：场景描述 + 角色/神兵多选 + 镜头偏好 8 维度
 * 右侧：实时生成的 S-A-C-S Prompt + 负面约束 + 一键复制
 *
 * 设计目标：让用户填最少、产出最贴近 Seedance 2.0 黄金案例的可投喂 Prompt
 */
import { computed, onMounted, ref, watch } from 'vue';
import { api } from '../api';

// ── 维度选项（来自 CREATIVE_BIBLE + skills/镜头美学/SKILL.md） ──

const MOODS = [
  { id: '杀意', label: '杀意', desc: '一击必杀，冷酷凌厉' },
  { id: '试探', label: '试探', desc: '双方测距，有来有回' },
  { id: '绝望', label: '绝望', desc: '以命换命，拼死挣扎' },
  { id: '愤怒', label: '愤怒', desc: '大开大合，情绪外放' },
  { id: '冷静', label: '冷静', desc: '从容对弈，掌控全场' },
];

const POWERS = [
  { id: '极致暴力', label: '极致暴力', desc: '天崩地裂' },
  { id: '点到为止', label: '点到为止', desc: '一招见血' },
  { id: '飘逸灵动', label: '飘逸灵动', desc: '轻盈如风' },
  { id: '刚猛无俦', label: '刚猛无俦', desc: '一力降十会' },
];

const RHYTHMS = [
  { id: '快切急促', label: '快切急促', desc: '每秒切换多次' },
  { id: '流畅连贯', label: '流畅连贯', desc: '3-5 次切镜' },
  { id: '沉稳大气', label: '沉稳大气', desc: '大景别少切镜' },
];

const SHOTS = [
  { id: '近景', label: '近景', desc: '以人为主' },
  { id: '中景', label: '中景', desc: '人+物' },
  { id: '远景', label: '远景', desc: '大场面' },
  { id: '特写', label: '特写', desc: '极尽细节' },
  { id: '全景', label: '全景', desc: '环境空间' },
];

const LIGHTS = [
  { id: '高对比硬光', label: '高对比硬光', desc: '戏剧感强' },
  { id: '柔和体积光', label: '柔和体积光', desc: '丁达尔写实' },
  { id: '低调暗调', label: '低调暗调', desc: '神秘压抑' },
  { id: '冷白高调', label: '冷白高调', desc: '清冷疏离' },
];

const TONES = [
  { id: '冷色系', label: '冷色系', desc: '蓝青低饱和' },
  { id: '暖色系', label: '暖色系', desc: '橙红高饱和' },
  { id: '水墨古风', label: '水墨古风', desc: '低饱和留白' },
  { id: '写实原色', label: '写实原色', desc: '无滤镜' },
];

const GRAINS = [
  { id: '强颗粒', label: '强颗粒', desc: '35mm 粗颗粒' },
  { id: '轻颗粒', label: '轻颗粒', desc: '数字+轻微' },
  { id: '无颗粒', label: '无颗粒', desc: '数字干净' },
];

const DURATIONS = [
  { id: 5, label: '5s · 短切' },
  { id: 10, label: '10s · 中段' },
  { id: 15, label: '15s · 标准' },
];

const CAMERA_PROMPT_TEMPLATE = {
  '杀意+极致暴力': '贴身短焦，低机位仰拍，急速切镜，hard light，暗调',
  '杀意+点到为止': '贴身短焦，一招即切，hard light，高对比',
  '杀意+飘逸灵动': '低机位仰拍，tracking shot，dynamic motion blur',
  '杀意+刚猛无俦': 'crane shot，大远景，低机位跟拍，体积光',
  '试探+飘逸灵动': '中景，平稳切镜，soft light，留白感',
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

// ── 状态 ──

const characters = ref([]);
const weapons = ref([]);
const skills = ref([]);
const sceneDesc = ref('');
const selectedChars = ref([]);
const selectedWeapons = ref([]);
const selectedSkillId = ref(''); // 选中的 skill（用于 prompt 头部约束注入）
const mood = ref('杀意');
const power = ref('极致暴力');
const rhythm = ref('流畅连贯');
const shot = ref('中景');
const light = ref('高对比硬光');
const tone = ref('冷色系');
const grain = ref('强颗粒');
const duration = ref(15);
const model = ref('Seedance 2.0');
const copied = ref(null);

// ── 加载 ──

async function load() {
  const [reg, w, sk] = await Promise.all([
    api.registry(),
    api.weapons(),
    api.skills(),
  ]);
  characters.value = reg.characters;
  weapons.value = w.weapons;
  skills.value = sk.skills;
}

onMounted(load);

// ── 角色/神兵 多选 chip ──

function toggleChar(c) {
  const i = selectedChars.value.findIndex((x) => x.name === c.name);
  if (i >= 0) selectedChars.value.splice(i, 1);
  else selectedChars.value.push(c);
}

function toggleWeapon(w) {
  const i = selectedWeapons.value.findIndex((x) => x.index === w.index);
  if (i >= 0) selectedWeapons.value.splice(i, 1);
  else selectedWeapons.value.push(w);
}

function isCharSelected(c) {
  return selectedChars.value.some((x) => x.name === c.name);
}

function isWeaponSelected(w) {
  return selectedWeapons.value.some((x) => x.index === w.index);
}

function charVisualOneLiner(c) {
  if (!c.aliases) return c.title;
  return c.title;
}

function charDesc(c) {
  // 提取 cast JSON 的视觉规范做一句话描述
  const parts = [];
  if (c.avatar) parts.push('面部特征见参考图');
  if (c.title) parts.push(c.title);
  if (c.faction) parts.push(`所属 ${c.faction}`);
  return parts.join(' · ');
}

// ── Prompt 生成 ──

const cameraShotTerm = computed(() => CAMERA_PROMPT_TEMPLATE[`${mood.value}+${power.value}`] || '中景侧跟');

const sceneSection = computed(() => {
  const parts = [];
  if (sceneDesc.value.trim()) parts.push(sceneDesc.value.trim());
  if (selectedChars.value.length) {
    parts.push(
      selectedChars.value
        .map((c) => `${c.name}（${charVisualOneLiner(c)}）`)
        .join('，'),
    );
  }
  if (selectedWeapons.value.length) {
    parts.push(
      selectedWeapons.value
        .map((w) => `手持【${w.name}】${w.kvs?.find((k) => k.key === '形制')?.value || ''}`.trim())
        .join('，'),
    );
  }
  return parts.join('。') || '（请描述场景）';
});

const actionSection = computed(() => {
  // 武器物理逻辑 → 动作链路
  const weaponActions = selectedWeapons.value
    .map((w) => w.kvs?.find((k) => k.key === '物理逻辑')?.value)
    .filter(Boolean)
    .join('；');
  const moodHints = {
    '杀意': '一击必杀，无多余动作',
    '试探': '双方测距，互相引而不发',
    '绝望': '以命换命，拼死挣扎',
    '愤怒': '大开大合，情绪外放',
    '冷静': '从容对弈，掌控全场',
  };
  return [
    moodHints[mood.value] || '',
    weaponActions,
  ].filter(Boolean).join('。') || '动作：见参考';
});

const cameraSection = computed(() => {
  const rhythmMap = {
    '快切急促': '短镜头密集切换，每秒切换多次',
    '流畅连贯': '3-5 次切镜，长镜头跟拍为主',
    '沉稳大气': '少切镜，大景别为主，留白感',
  };
  const lightMap = {
    '高对比硬光': '高对比硬光，明暗分界锐利，戏剧感强',
    '柔和体积光': '柔和体积光 + Tyndall 效果，写实质感',
    '低调暗调': '低调暗调，阴影为主，神秘压抑',
    '冷白高调': '冷白高调，亮调为主，清冷疏离',
  };
  return [
    cameraShotTerm.value,
    lightMap[light.value] || '',
    `${shot.value}（${rhythmMap[rhythm.value] || ''}）`,
  ].filter(Boolean).join('，');
});

const styleSection = computed(`35mm 胶片颗粒，电影级低调摄影，${duration.value}s 24fps，16:9 写实高武，丁达尔体积光，自然光`);

const toneTag = computed(() => {
  const map = {
    '冷色系': '冷蓝青低饱和',
    '暖色系': '暖橙红高饱和',
    '水墨古风': '水墨低饱和灰调留白',
    '写实原色': '写实无滤镜',
  };
  return map[tone.value] || '';
});

const grainTag = computed(() => {
  const map = { '强颗粒': '35mm 强颗粒', '轻颗粒': '轻颗粒', '无颗粒': '数字干净' };
  return map[grain.value] || '';
});

// 把 Skill 的 description + 触发词注入到 Prompt 头部
const selectedSkill = computed(() => skills.value.find((s) => s.id === selectedSkillId.value) || null);
const skillSection = computed(() => {
  if (!selectedSkill.value) return '';
  const s = selectedSkill.value;
  const triggers = (s.triggers || []).slice(0, 3).map((t) => `「${t}」`).join('、');
  const triggerLine = triggers ? `\n触发词：${triggers}` : '';
  return `[Skill 约束：${s.name}]\n${s.description || ''}${triggerLine}`;
});

const fullPrompt = computed(() => {
  const head = skillSection.value
    ? `${skillSection.value}\n\n---\n\n`
    : '';
  return `${head}${duration.value}s 电影镜头，16:9，35mm 胶片感，${toneTag.value}，${grainTag.value}。\n\n[Scene]\n${sceneSection.value}\n\n[Action]\n${actionSection.value}\n\n[Camera]\n${cameraSection.value}\n\n[Style]\n${styleSection.value}`;
});

const negativePrompt = computed(`约束：no extra characters, no crowd, no extra fingers, no extra limbs, no distortion, no deformed hands, no warped faces, no jump cuts, no lens flares, no text overlays, no watermarks, no blurry motion, no snap zooms, no magic circles, no glowing runes, no modern elements, no subtitles`);

async function copy(text, id) {
  try {
    await navigator.clipboard.writeText(text);
    copied.value = id;
    setTimeout(() => { if (copied.value === id) copied.value = null; }, 1500);
  } catch { /* */ }
}

function reset() {
  sceneDesc.value = '';
  selectedChars.value = [];
  selectedWeapons.value = [];
}

// 字符统计
const stats = computed(() => ({
  promptLen: fullPrompt.value.length,
  negLen: negativePrompt.value.length,
  chars: selectedChars.value.length,
  weapons: selectedWeapons.value.length,
}));
</script>

<template>
  <div class="page">
    <header class="topbar">
      <div>
        <h1 class="title-brush topbar-title">Prompt 实验室</h1>
        <span class="topbar-sub">PROMPT LAB · 套用法典 → 一键投喂 {{ model }}</span>
      </div>
      <div class="topbar-actions">
        <button class="btn btn-sm" @click="reset">清空</button>
        <select v-model="model" class="model-select">
          <option>Seedance 2.0</option>
          <option>Google Veo 3.1</option>
          <option>即梦 AI</option>
        </select>
      </div>
    </header>

    <div class="body">
      <!-- 左侧：输入 -->
      <section class="pane input-pane scroll-y">
        <!-- 1. 场景描述 -->
        <div class="block">
          <h3 class="block-title">① 场景描述</h3>
          <textarea
            v-model="sceneDesc"
            class="textarea"
            placeholder="例：北荒绝地，狂暴暴雪。慕北歌手按'问天'剑柄，回眸一瞬间。"
            rows="4"
          />
        </div>

        <!-- 2. 角色 -->
        <div class="block">
          <h3 class="block-title">② 角色 <span class="dim mono">({{ selectedChars.length }})</span></h3>
          <div class="chip-grid">
            <button
              v-for="c in characters"
              :key="c.name"
              class="chip"
              :class="{ active: isCharSelected(c) }"
              @click="toggleChar(c)"
              :title="charDesc(c)"
            >
              <img v-if="c.avatar" :src="`/api/asset?p=${encodeURIComponent(c.avatar)}`" class="chip-avatar" :alt="c.name" />
              <span v-else class="chip-avatar chip-avatar-empty">{{ c.name[0] }}</span>
              <span class="chip-label">{{ c.name }}</span>
            </button>
          </div>
        </div>

        <!-- 3. 神兵 -->
        <div class="block">
          <h3 class="block-title">③ 神兵 <span class="dim mono">({{ selectedWeapons.length }})</span></h3>
          <div class="chip-grid">
            <button
              v-for="w in weapons"
              :key="w.index"
              class="chip chip-wide"
              :class="{ active: isWeaponSelected(w) }"
              @click="toggleWeapon(w)"
              :title="w.holder ? `持有者：${w.holder}` : ''"
            >
              <span class="chip-num">{{ String(w.index).padStart(2, '0') }}</span>
              <span class="chip-label">【{{ w.name }}】</span>
            </button>
          </div>
        </div>

        <!-- 3.5 Skill 约束注入（P1.3：从 novels/.agent-skills/ 拉 skill 加到 prompt 头部） -->
        <div class="block">
          <h3 class="block-title">③⁵ Skill 约束 <span class="dim mono">（可选 · novels/.agent-skills/）</span></h3>
          <div class="skill-row">
            <button
              class="chip skill-chip"
              :class="{ active: !selectedSkillId }"
              @click="selectedSkillId = ''"
            >无</button>
            <button
              v-for="s in skills"
              :key="s.id"
              class="chip skill-chip"
              :class="{ active: selectedSkillId === s.id }"
              @click="selectedSkillId = s.id"
              :title="s.description || ''"
            >{{ s.name }}</button>
          </div>
          <p v-if="selectedSkill" class="skill-hint dim">
              <span class="dim mono">↑</span> 选中后该 skill 的 description + triggers 会自动注入到 Prompt 头部，与法典/角色/神兵自动拼接。
            </p>
        </div>

        <!-- 4. 镜头偏好 8 维度 -->
        <div class="block">
          <h3 class="block-title">④ 镜头偏好</h3>
          <div class="dim-block">
            <div class="dim-row">
              <span class="dim-label">情绪基调</span>
              <div class="opt-row">
                <button v-for="m in MOODS" :key="m.id" class="opt" :class="{ active: mood === m.id }" @click="mood = m.id" :title="m.desc">{{ m.label }}</button>
              </div>
            </div>
            <div class="dim-row">
              <span class="dim-label">力量感</span>
              <div class="opt-row">
                <button v-for="p in POWERS" :key="p.id" class="opt" :class="{ active: power === p.id }" @click="power = p.id" :title="p.desc">{{ p.label }}</button>
              </div>
            </div>
            <div class="dim-row">
              <span class="dim-label">节奏</span>
              <div class="opt-row">
                <button v-for="r in RHYTHMS" :key="r.id" class="opt" :class="{ active: rhythm === r.id }" @click="rhythm = r.id" :title="r.desc">{{ r.label }}</button>
              </div>
            </div>
            <div class="dim-row">
              <span class="dim-label">景别</span>
              <div class="opt-row">
                <button v-for="s in SHOTS" :key="s.id" class="opt" :class="{ active: shot === s.id }" @click="shot = s.id" :title="s.desc">{{ s.label }}</button>
              </div>
            </div>
            <div class="dim-row">
              <span class="dim-label">光影</span>
              <div class="opt-row">
                <button v-for="l in LIGHTS" :key="l.id" class="opt" :class="{ active: light === l.id }" @click="light = l.id" :title="l.desc">{{ l.label }}</button>
              </div>
            </div>
            <div class="dim-row">
              <span class="dim-label">色调</span>
              <div class="opt-row">
                <button v-for="t in TONES" :key="t.id" class="opt" :class="{ active: tone === t.id }" @click="tone = t.id" :title="t.desc">{{ t.label }}</button>
              </div>
            </div>
            <div class="dim-row">
              <span class="dim-label">胶片质感</span>
              <div class="opt-row">
                <button v-for="g in GRAINS" :key="g.id" class="opt" :class="{ active: grain === g.id }" @click="grain = g.id" :title="g.desc">{{ g.label }}</button>
              </div>
            </div>
            <div class="dim-row">
              <span class="dim-label">时长</span>
              <div class="opt-row">
                <button v-for="d in DURATIONS" :key="d.id" class="opt" :class="{ active: duration === d.id }" @click="duration = d.id">{{ d.label }}</button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- 右侧：输出 -->
      <section class="pane output-pane">
        <div class="output-head">
          <h3 class="output-title">📤 S-A-C-S 结构化 Prompt</h3>
          <div class="output-stats dim mono">
            {{ stats.promptLen }} 字 · {{ stats.chars }} 角色 · {{ stats.weapons }} 神兵
          </div>
        </div>
        <pre class="output-prompt mono">{{ fullPrompt }}</pre>
        <div class="output-actions">
          <button class="btn btn-primary" @click="copy(fullPrompt, 'prompt')">
            {{ copied === 'prompt' ? '✓ 已复制' : '复制 Prompt' }}
          </button>
        </div>

        <div class="output-head" style="margin-top: var(--sp-4)">
          <h3 class="output-title">🚫 负面约束</h3>
          <div class="output-stats dim mono">{{ stats.negLen }} 字</div>
        </div>
        <pre class="output-prompt output-negative mono">{{ negativePrompt }}</pre>
        <div class="output-actions">
          <button class="btn" @click="copy(negativePrompt, 'neg')">
            {{ copied === 'neg' ? '✓ 已复制' : '复制负面约束' }}
          </button>
          <button class="btn btn-primary" @click="copy(fullPrompt + '\n\n' + negativePrompt, 'all')">
            {{ copied === 'all' ? '✓ 已复制' : '复制全部' }}
          </button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.page { display: grid; grid-template-rows: auto 1fr; height: 100%; background: var(--ink-0); }
.topbar { display: flex; justify-content: space-between; align-items: center; padding: var(--sp-4) var(--sp-5); border-bottom: 1px solid var(--line-1); background: var(--ink-1); }
.topbar-title { font-size: 22px; color: var(--gold-1); margin: 0; }
.topbar-sub { font-size: 10px; letter-spacing: 0.22em; color: var(--text-3); }
.topbar-actions { display: flex; gap: var(--sp-2); align-items: center; }
.model-select { padding: 4px 8px; border: 1px solid var(--line-2); border-radius: var(--r-2); background: var(--ink-3); color: var(--text-1); font-size: 12px; cursor: pointer; }

.body { display: grid; grid-template-columns: 1fr 1fr; min-height: 0; }
.pane { display: flex; flex-direction: column; min-height: 0; }
.input-pane { padding: var(--sp-4); border-right: 1px solid var(--line-1); background: var(--ink-1); gap: var(--sp-4); }
.output-pane { padding: var(--sp-4); background: var(--ink-0); }

.block { display: flex; flex-direction: column; gap: var(--sp-2); }
.block-title { font-size: 10px; letter-spacing: 0.2em; color: var(--gold-2); text-transform: uppercase; margin: 0; padding-bottom: 4px; border-bottom: 1px solid var(--line-1); }

.chip-grid { display: flex; flex-wrap: wrap; gap: 4px; }
.chip {
  display: flex; align-items: center; gap: 6px;
  padding: 4px 8px;
  background: var(--ink-2);
  border: 1px solid var(--line-2);
  border-radius: var(--r-1);
  color: var(--text-2);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.12s;
}
.chip:hover { color: var(--text-1); border-color: var(--line-gold); }
.chip.active { background: var(--gold-wash); border-color: var(--gold-2); color: var(--gold-1); }
.chip-wide { padding: 4px 10px; }
.chip-avatar { width: 18px; height: 18px; border-radius: var(--r-1); object-fit: cover; flex-shrink: 0; }
.chip-avatar-empty { display: flex; align-items: center; justify-content: center; background: var(--ink-3); color: var(--text-3); font-size: 9px; }
.chip-num { font-family: var(--font-mono); font-size: 9px; color: var(--gold-2); letter-spacing: 0.1em; }
.chip-label { white-space: nowrap; }

.dim-block { display: flex; flex-direction: column; gap: var(--sp-2); padding: var(--sp-3); background: var(--ink-2); border: 1px solid var(--line-1); border-radius: var(--r-2); }

/* Skill 约束注入（P1.3） */
.skill-row { display: flex; flex-wrap: wrap; gap: 4px; }
.skill-chip { font-size: 11px; }
.skill-hint {
  margin: 6px 0 0;
  font-size: 10px;
  line-height: 1.5;
}
.dim-row { display: grid; grid-template-columns: 90px 1fr; gap: var(--sp-2); align-items: center; }
.dim-label { font-size: 10px; color: var(--text-3); letter-spacing: 0.15em; }
.opt-row { display: flex; flex-wrap: wrap; gap: 3px; }
.opt { padding: 3px 8px; font-size: 11px; color: var(--text-2); background: var(--ink-3); border: 1px solid var(--line-1); border-radius: var(--r-1); cursor: pointer; }
.opt:hover { color: var(--text-1); border-color: var(--line-gold); }
.opt.active { background: var(--gold-wash); color: var(--gold-1); border-color: var(--gold-2); }

/* 输出 */
.output-head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: var(--sp-2); }
.output-title { font-size: 12px; letter-spacing: 0.15em; color: var(--gold-2); margin: 0; }
.output-stats { font-size: 10px; }
.output-prompt {
  flex: 1; min-height: 200px;
  padding: var(--sp-3);
  background: var(--ink-1);
  border: 1px solid var(--line-gold);
  border-radius: var(--r-2);
  color: var(--text-1);
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-y: auto;
  margin: 0;
}
.output-negative { border-color: var(--crimson); background: var(--crimson-wash); color: var(--crimson); font-size: 11px; }
.output-actions { display: flex; gap: var(--sp-2); margin-top: var(--sp-2); }
</style>
