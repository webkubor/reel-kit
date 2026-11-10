const $ = id => document.getElementById(id)
const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
import { recommendationControls } from '/recommendation-controls.js'
import { voiceLibrary } from '/voice-library.js'
import { workflowPanel } from '/workflow-panel.js'
let state = { projects: [], assets: [], jobs: [], templates: [] }, providers = {}, mode = 'video', filter = 'all'
let projectId = localStorage.getItem('reel-project'), previewId, selected = [], lastAssets = '', lastJobs = '', lastProjects = '', uploadTarget
const kinds = { video: '视频', image: '图片', speech: '配音', clone: '克隆配音', 'voice-design': '新建音色', recommendation: '项目推荐', talk: '口播视频', export: '合成' }
const labels = { queued: '排队中', running: '进行中', succeeded: '已完成', failed: '失败', interrupted: '已中断' }
const origins = { ai: 'AI 生成', code: '本地制作', import: '导入素材', record: '我的录音' }
const examples = {
  video: '写实时尚棚拍，一镜到底，9:16竖屏。浅灰白无缝摄影棚，高调柔光。一位20多岁的成年东亚女性模特，长直黑发，酒红色方领针织上衣、白色短裙与白色高跟凉鞋。她从画面右后方自然走向中央，短暂停步，一手扶腰，转头后看向镜头；随后缓慢走近，轻轻歪头，露出克制自信的微笑。固定正面机位，人物走近使全身景自然变为半身。动作符合重力，脚掌真实接地，人物、服装、首饰保持一致，无切镜、无文字、无水印。',
  image: '写实时尚摄影，全身构图，9:16竖幅。浅灰白色无缝摄影棚，大面积柔光。20多岁成年东亚女性模特，长直黑发，酒红色方领针织上衣、白色短裙和白色高跟凉鞋，自然站姿，一手轻扶腰侧。清冷柔和的商业摄影质感，保留细腻皮肤纹理，画面干净，无文字、无水印。',
  speech: '让一个想法，成为看得见的画面。用自己的声音，讲述自己的故事。',
}
async function api(path, options = {}) {
  const response = await fetch(path, { ...options, headers: { 'Content-Type': 'application/json', 'X-Reel-Action': 'studio', ...options.headers } })
  const result = await response.json()
  if (!response.ok) throw new Error(result.error || '请求失败')
  return result
}
function message(text, error = false) { $('notice').textContent = text; $('notice').hidden = false; $('notice').setAttribute('role', error ? 'alert' : 'status') }
function currentAssets() { return state.assets.filter(a => a.projectId === projectId) }
function draftKey() { return `reel-draft:${projectId}:${mode}` }
function saveDraft() { localStorage.setItem(draftKey(), $('prompt').value) }
function loadDraft() { $('prompt').value = localStorage.getItem(draftKey()) || ''; $('char-count').textContent = `${$('prompt').value.length} 字` }
function optionList(element, values, empty) {
  const previous = element.value
  element.innerHTML = (empty ? `<option value="">${escape(empty)}</option>` : '') + values.map(v => `<option value="${escape(v.id)}">${escape(v.name)}</option>`).join('')
  if (values.some(v => v.id === previous)) element.value = previous
}
function renderOptions() {
  const clone = mode === 'speech' && $('voice-mode').value === 'clone'
  const assets = currentAssets()
  optionList($('reference'), assets.filter(a => a.mime.startsWith(clone ? 'audio/' : 'image/')).map(a => ({ id: a.id, name: a.name })), clone ? '选择参考音频' : '无参考图，使用文字生成')
  optionList($('audio'), assets.filter(a => a.mime.startsWith('audio/')).map(a => ({ id: a.id, name: a.name })), '保留原声 / 不添加音频')
  optionList($('template'), state.templates.map(name => ({ id: name, name })))
  if (!$('template').value) $('template').value = 'sticker-promo'
}
function renderProjects() {
  // 当前项目被删除后要落到一个真实存在的项目上，否则草稿、素材与设置都会指向空项目
  if (state.projects.length && !state.projects.some(p => p.id === projectId)) {
    projectId = state.projects[0].id
    localStorage.setItem('reel-project', projectId)
    selected = []; previewId = null; loadDraft()
    if (mode === 'recommendation') loadRecommendation()
  }
  const key = JSON.stringify([state.projects, projectId])
  if (key === lastProjects) return
  lastProjects = key
  $('projects').innerHTML = state.projects.map(p => `<button class="project-button ${p.id === projectId ? 'active' : ''}" data-project="${p.id}" title="${escape(p.name)}"><span class="folder">▱</span><span>${escape(p.name)}</span></button>`).join('')
  optionList($('active-project'), state.projects.map(p => ({ id: p.id, name: p.name })), '')
  // 下拉必须反映当前项目：新建项目后 projectId 已变，optionList 只会保留旧值
  $('active-project').value = projectId
}
function switchProject(next) {
  if (!next || next === projectId) return
  if (mode === 'recommendation') saveRecommendation()
  saveDraft(); projectId = next; localStorage.setItem('reel-project', projectId); selected = []; previewId = null; loadDraft(); if (mode === 'recommendation') loadRecommendation(); renderProjects(); renderAssets(); lastJobs = ''; renderJobs()
  workflow.render()
  const asset = currentAssets().at(-1); if (asset) showPreview(asset.id)
  else { $('preview').innerHTML = `<div class="preview-empty"><h3>这个项目的第一帧，等你创造</h3><p>生成或导入素材后，在这里预览。</p><span class="preview-caption">${channelCaption()}</span></div>`; $('download').hidden = true; $('preview-name').textContent = '还没有选中素材' }
}
function renderAssets() {
  const assets = currentAssets()
  const key = JSON.stringify([assets, filter, selected, mode])
  if (key === lastAssets) return
  lastAssets = key
  $('asset-count').textContent = assets.length
  const visible = assets.filter(a => filter === 'all' || a.mime.startsWith(filter + '/'))
  $('assets').innerHTML = visible.length ? visible.map(a => `<article class="asset-card ${selected.includes(a.id) ? 'selected' : ''}"><button class="asset-open" data-preview="${a.id}" aria-label="预览 ${escape(a.name)}">${a.mime.startsWith('image/') ? `<img src="${a.url}" alt="${escape(a.name)}" loading="lazy">` : a.mime.startsWith('video/') ? `<video src="${a.url}#t=0.1" preload="metadata" muted playsinline></video>` : '<span class="audio-symbol">≋</span>'}</button>${(mode === 'export' && !a.mime.startsWith('audio/') || mode === 'recommendation' && a.mime.startsWith('image/') || mode === 'talk' && a.mime.startsWith('image/')) ? `<input class="asset-check" type="checkbox" data-select="${a.id}" aria-label="选择 ${escape(a.name)}" ${selected.includes(a.id) ? 'checked' : ''}>` : ''}<div class="asset-info"><strong>${escape(a.name)}</strong><span>${escape(a.source)} · ${new Date(a.createdAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</span>${assetMeta(a) ? `<span class="asset-meta">${assetMeta(a)}</span>` : ''}</div></article>`).join('') : `<button type="button" class="library-empty" id="empty-upload"><span class="plus">＋</span><span>${assets.length ? '这个分类还没有素材' : '导入你的第一份素材'}<small>图片、视频、声音，或从上方开始生成</small></span></button>`
  $('export-order').innerHTML = selected.map((id, i) => `<li>${escape(assets.find(a => a.id === id)?.name)}${i ? `<button type="button" data-up="${id}">↑ 上移</button>` : ''}<button type="button" data-remove="${id}">移除</button></li>`).join('')
  renderOptions()
  rec?.renderAssets()
}
function assetMeta(a) {
  return [origins[a.origin], a.width && a.height ? `${a.width}×${a.height}` : '', a.duration ? `${Number(a.duration).toFixed(1)}s` : ''].filter(Boolean).join(' · ')
}
function renderStep(step, index) {
  const asset = step.assetId && state.assets.find(a => a.id === step.assetId)
  const action = asset ? `<button type="button" class="text-button" data-preview="${asset.id}">${asset.mime.startsWith('audio/') ? '▶ 试听' : asset.mime.startsWith('video/') ? '▶ 播放' : '查看图片'}</button>` : ''
  const fix = step.status === 'failed' ? `<button type="button" class="text-button step-fix" data-fix="${step.id.startsWith('voice') ? 'voice' : step.id.startsWith('frames') ? 'scenes' : 'generate'}">去修改 →</button>` : ''
  const detail = [step.engine, step.detail, step.error && `⚠ ${step.error}`].filter(Boolean).join(' · ')
  return `<li class="step-row ${step.status}"><span class="step-num">${index + 1}</span><span class="origin-badge ${step.origin || 'code'}">${origins[step.origin] || '本地制作'}</span><div class="step-main"><strong>${escape(step.label)}${step.reused ? ' · 复用已生成' : ''}</strong>${detail ? `<span class="step-detail">${escape(detail)}</span>` : ''}</div>${action}${step.ms != null ? `<span class="step-ms">${(step.ms / 1000).toFixed(1)}s</span>` : ''}${fix}</li>`
}
function renderJobs() {
  const jobs = state.jobs.filter(j => j.projectId === projectId).toReversed()
  const key = JSON.stringify(jobs)
  if (key === lastJobs) return
  const previousIds = new Set(JSON.parse(lastJobs || '[]').filter(j => j.status === 'succeeded').map(j => j.id))
  lastJobs = key
  $('job-count').textContent = jobs.length ? `${jobs.length} 次记录` : ''
  $('jobs').innerHTML = jobs.length ? jobs.map(j => {
    const steps = j.steps || []
    const timeline = steps.length ? `<details class="job-steps"${j.status === 'failed' ? ' open' : ''}><summary>制作过程 · ${steps.length} 步${j.status === 'running' ? ' · 进行中' : ''}</summary><ol>${steps.map(renderStep).join('')}</ol></details>` : ''
    // 上游已经提交过、只是没取回素材的，给一个「取回」入口。
    // 重新生成等于再付一次钱 —— 通道是按量计费的。
    const resume = j.upstream?.taskId && ['failed', 'interrupted'].includes(j.status)
      ? `<button class="text-button" data-resume="${j.id}">取回已生成的素材</button>` : ''
    return `<article class="job-item"><div class="job-row"><span class="job-kind">${kinds[j.input.kind]}</span><div><p class="job-prompt" title="${escape(j.input.prompt || j.input.title)}">${escape(j.input.prompt || j.input.title || '合成出片')}</p><p class="job-detail">${escape(j.error || j.message)}</p></div><span class="job-state ${j.status}">${labels[j.status]}</span>${resume}<button class="text-button" data-reuse="${j.id}">复用参数 ↗</button></div>${timeline}</article>`
  }).join('') : '<p class="empty-history">还没有生成记录。每次提交的参数与结果都会保留。</p>'
  const latest = jobs.find(j => j.status === 'succeeded' && !previousIds.has(j.id))
  if (latest?.assetId) showPreview(latest.assetId)
}
function showPreview(id) {
  const asset = state.assets.find(a => a.id === id)
  if (!asset) return
  previewId = id
  if (asset.mime.startsWith('video/') && asset.width && asset.height) {
    $('preview').classList.toggle('portrait', asset.height > asset.width)
    $('preview').classList.toggle('landscape', asset.width >= asset.height)
    $('preview').style.aspectRatio = `${asset.width} / ${asset.height}`
  } else { $('preview').style.aspectRatio = '' }
  $('preview').innerHTML = asset.mime.startsWith('image/') ? `<img src="${asset.url}" alt="${escape(asset.name)}">` : asset.mime.startsWith('video/') ? `<video src="${asset.url}" controls playsinline preload="metadata"></video>` : `<div class="audio-preview"><span class="audio-symbol">≋</span><audio src="${asset.url}" controls preload="metadata"></audio></div>`
  $('preview-name').textContent = asset.name
  const dims = [asset.width && asset.height ? `${asset.width}×${asset.height}` : '', asset.duration ? `${Number(asset.duration).toFixed(1)} 秒` : '', asset.hasAudio === true ? '含音轨' : asset.hasAudio === false ? '无音轨' : ''].filter(Boolean).join(' · ')
  $('preview-meta').textContent = [origins[asset.origin] || asset.source, dims].filter(Boolean).join(' · ')
  $('download').hidden = false; $('download').href = asset.url + '?download=1'
}
async function refresh() {
  const previous = state
  state = await api('/api/state')
  if (state.jobs.some(job => job.input?.kind === 'clone' && job.status === 'succeeded' && previous.jobs.some(old => old.id === job.id && old.status !== 'succeeded'))) await connections()
  if (!state.projects.some(p => p.id === projectId)) projectId = state.projects[0]?.id
  renderProjects(); renderAssets(); renderJobs()
  rec?.jobs()
  // 口播的音色与 BGM 列表跟着项目走：素材变了要跟着变
  if (mode === 'talk') loadTalkOptions()
  const workflowKey = JSON.stringify([projectId, state.jobs.filter(j => j.projectId === projectId && j.input.kind === 'recommendation' && j.status === 'succeeded').map(j => j.id)])
  if (workflowKey !== lastWorkflowKey) { lastWorkflowKey = workflowKey; workflow.render() }
}
const CUSTOM_MODEL = '__custom__'
const CUSTOM_VOICE = '__custom__'
/**
 * 语音通道的选择单独记一份。
 *
 * 生成通道的下拉是共用的：切到图片模式时它显示的是生图通道。
 * 旁白用哪个通道跟生图通道无关，不能被图片模式覆盖 —— 所以语音的选择独立记着，
 * 回到声音模式再取回来。
 */
const voiceState = { backend: '', model: '', voiceId: '', profile: '' }
try { Object.assign(voiceState, JSON.parse(localStorage.getItem('reel-voice-state') || '{}')) } catch { /* 坏数据就当没记过 */ }
function saveVoiceState() { localStorage.setItem('reel-voice-state', JSON.stringify(voiceState)) }
/** 当前模式下生成通道属于哪一组：视频、图片还是语音 */
function channelGroup() {
  if (mode === 'speech' || mode === 'recommendation') return 'voice'
  if (mode === 'video') return 'video'
  if (mode === 'image') return 'image'
  return null
}
function channelsFor(group) {
  const list = providers[group]?.channels || []
  // 克隆是把一段本地录音变成音色，只有本地 VoxFlow 做得了 —— 云端 TTS 只做文字配音
  if (mode === 'speech' && $('voice-mode').value === 'clone') return list.filter(c => c.id === 'voxflow')
  return list
}
function currentChannel() {
  const group = channelGroup()
  if (!group) return null
  return (providers[group]?.channels || []).find(c => c.id === $('backend').value) || null
}
function currentModel() {
  if ($('model-field').hidden) return ''
  if ($('model').value === CUSTOM_MODEL) return $('model-custom').value.trim()
  return $('model').value
}
function currentVoiceId() {
  if ($('voice-field').hidden) return ''
  if ($('voice-id').value === CUSTOM_VOICE) return $('voice-custom').value.trim()
  return $('voice-id').value
}
/**
 * 通道与模型一起同步。
 *
 * 通道决定「谁来花钱、谁的数据会外传」，模型决定「能力与上限」。
 * 两者都要跟着模式切换：语音模式只有语音通道，图片模式只有生图通道。
 */
function syncBackend() {
  const group = channelGroup()
  const field = $('backend-field')
  if (!group) { field.hidden = true; return }
  field.hidden = false
  const list = channelsFor(group)
  const select = $('backend')
  const remembered = group === 'voice' ? voiceState.backend : (localStorage.getItem(`reel-channel:${group}`) || '')
  select.innerHTML = list.map(c => `<option value="${escape(c.id)}">${escape(c.label)}${c.ready ? '' : '（未就绪）'}</option>`).join('')
  for (const option of select.options) if (!list.find(c => c.id === option.value)?.ready) option.disabled = true
  const usable = list.filter(c => c.ready)
  // 记住上次选的通道：每次进来都要重选一遍是折磨。
  //
  // 关键：记住的通道若只是「暂时未就绪」，仍然选它 ——
  // 悄悄换成另一个可用的（可能是另一家的付费）通道，等于替用户改了供应商。
  // 该不该花钱由用户决定，不是由下拉框的默认值决定。
  const rememberedChannel = list.find(c => c.id === remembered)
  const wanted = rememberedChannel ? remembered : (usable[0]?.id || list[0]?.id)
  if (wanted) select.value = wanted
  if (group === 'voice') { voiceState.backend = select.value; saveVoiceState() }
  syncChannel()
}
function syncChannel() {
  const group = channelGroup()
  if (!group) return
  const channel = currentChannel()
  $('backend-help').textContent = channel ? channel.note : ''
  const models = channel?.models || []
  const modelField = $('model-field')
  if (models.length || channel?.allowCustomModel) {
    modelField.hidden = false
    const options = models.map(m => ({ id: m.id, name: m.name }))
    if (channel?.allowCustomModel) options.push({ id: CUSTOM_MODEL, name: '自定义模型…' })
    optionList($('model'), options, models.length ? '由通道自动路由' : '请选择或填写模型')
    $('model-custom').hidden = $('model').value !== CUSTOM_MODEL
    restoreModelPick(group)
  } else {
    // 上一组留下的选项要清掉：否则切回声音模式时可能还留着视频模型
    modelField.hidden = true; $('model').innerHTML = ''; $('model-custom').hidden = true; $('model-custom').value = ''
  }
  applyModelLimits()
  // 音色：本地音色走「音色」下拉（persona），云端 TTS 走独立的 voice_id 选择
  const voiceField = $('voice-field')
  const voices = channel?.voices || []
  if (voices.length && channel?.id !== 'voxflow') {
    voiceField.hidden = false
    const options = voices.map(v => ({ id: v.id, name: v.name }))
    if (channel?.allowCustomVoice) options.push({ id: CUSTOM_VOICE, name: '自定义音色 ID…' })
    optionList($('voice-id'), options, '')
    $('voice-custom').hidden = $('voice-id').value !== CUSTOM_VOICE
  } else { voiceField.hidden = true; $('voice-id').innerHTML = ''; $('voice-custom').hidden = true; $('voice-custom').value = '' }
  // 本地音色只对 VoxFlow 通道有效；克隆也只能走本地
  if (mode === 'speech') $('persona-field').hidden = channel?.id !== 'voxflow' || $('voice-mode').value === 'clone'
  $('profile-field').hidden = channel?.id !== 'minimax'
  if (group === 'voice') {
    restoreVoicePick()
    voiceState.backend = channel?.id || ''
    voiceState.model = currentModel()
    voiceState.voiceId = currentVoiceId()
    voiceState.profile = $('profile').value || ''
    saveVoiceState()
  }
  updateGenerateState()
  applyChannelCopy()
}
/**
 * 通道没就绪时，生成按钮要跟着这一条通道走，不能按整组算。
 *
 * 之前用 group.ready 判断：只要组里还有任一通道可用，按钮就一直可点 ——
 * 于是用户选中的那条（可能没配密钥、可能欠费）照样能提交，提交后才报错。
 */
function updateGenerateState() {
  const channel = currentChannel()
  const button = $('generate')
  if (!button) return
  const group = channelGroup()
  if (!group) { button.disabled = false; return }
  if (mode === 'speech' && $('voice-mode').value === 'clone' && channel?.id !== 'voxflow') {
    button.disabled = true
    button.title = '克隆只能走本地 VoxFlow'
    return
  }
  if (channel && !channel.ready) {
    button.disabled = true
    button.title = `所选通道「${channel.label}」未就绪：${channel.note || ''}`
    return
  }
  button.disabled = false
  button.title = ''
}
/** 把记住的模型放回选择框：视频与图片的模型也要记住，不能每次从头选 */
function restoreModelPick(group) {
  if (group === 'voice') return
  const saved = localStorage.getItem(`reel-model:${group}`) || ''
  if (!saved) return
  if ([...$('model').options].some(o => o.value === saved)) $('model').value = saved
  else { $('model').value = CUSTOM_MODEL; $('model-custom').value = saved }
  $('model-custom').hidden = $('model').value !== CUSTOM_MODEL
}

/**
 * 标题下的一句话与角标要跟着通道走。
 *
 * 以前这里写死「MiniMax H3」，现在通道是用户自己选的 —— 写死就等于撒谎：
 * 界面说 H3，实际可能走的是 MuseAV。
 */
function applyChannelCopy() {
  const channel = currentChannel()
  if (!channel || mode === 'recommendation' || mode === 'export') return
  const subtitles = {
    museav: 'MuseAV 中台 · 复用本机登录态，按中台当前可用模型出图出片',
    magpie: 'Magpie 网关 · 走 Magpie 里已配好的模型，能力清单由网关自己公布',
    minimax: 'MiniMax 直连 · 走你自己的账户与额度',
    voxflow: 'VoxFlow 本地 · 本地推理，不产生模型费用',
  }
  $('mode-subtitle').textContent = subtitles[channel.id] || channel.note || ''
  $('engine-badge').textContent = channel.label.split(/[ ·]/)[0].toUpperCase().slice(0, 12)
}
/** 把记住的语音模型与音色放回选择框 */
function restoreVoicePick() {
  if (voiceState.model) {
    if ([...$('model').options].some(o => o.value === voiceState.model)) $('model').value = voiceState.model
    else if ($('model').options.length) { $('model').value = CUSTOM_MODEL; $('model-custom').value = voiceState.model }
    $('model-custom').hidden = $('model').value !== CUSTOM_MODEL
  }
  if (voiceState.voiceId && !$('voice-field').hidden) {
    if ([...$('voice-id').options].some(o => o.value === voiceState.voiceId)) $('voice-id').value = voiceState.voiceId
    else { $('voice-id').value = CUSTOM_VOICE; $('voice-custom').value = voiceState.voiceId }
    $('voice-custom').hidden = $('voice-id').value !== CUSTOM_VOICE
  }
  // 账户也要还原：换了账户就是另一份额度、另一种声音
  if (voiceState.profile && [...$('profile').options].some(o => o.value === voiceState.profile)) $('profile').value = voiceState.profile
}
function applyModelLimits() {
  const channel = currentChannel()
  const model = (channel?.models || []).find(m => m.id === $('model').value)
  const duration = $('duration')
  // 上一个模型的时长范围要跟着换掉：Kling 限 10 秒，切到 Wan 固定 5 秒时不能还留着 10
  if (model?.minSeconds) duration.min = model.minSeconds
  else if (mode === 'video') duration.min = 4
  if (model?.maxSeconds) duration.max = model.maxSeconds
  else if (mode === 'video') duration.max = 30
  const value = Number(duration.value)
  if (model?.minSeconds && value < model.minSeconds) duration.value = model.minSeconds
  if (model?.maxSeconds && value > model.maxSeconds) duration.value = model.maxSeconds
  // 分辨率只有直连 H3 需要自己指定；中台的档次自带分辨率，填了也是白填
  $('resolution-field').hidden = mode !== 'video' || channel?.id !== 'minimax'
  // H3 的分辨率是接口参数，不能让用户填一个上游不认的值。
  // 768P 是本机实跑验证过的；1080P 接口支持，但本机没验过，界面上据实说明。
  if (channel?.id === 'minimax' && mode === 'video') {
    const select = $('resolution')
    const current = select.value
    select.innerHTML = '<option value="768P">768P（已验证）</option><option value="1080P">1080P（接口支持，未验证）</option>'
    select.value = ['768P', '1080P'].includes(current) ? current : '768P'
  }
}
function generationNote() {
  const channel = currentChannel()
  if (mode === 'recommendation') return '仓库信息经 GitHub API 读取，渲染与合成在本机执行；旁白按上面选定的通道生成。'
  if (mode === 'talk') return '逐句用本机 VoxFlow 生成旁白，渲染与合成在本机执行；不产生模型费用，视频与素材留在本地项目里。'
  if (mode === 'export') return '选中的素材按顺序导出；原始素材会保留。'
  const label = channel?.label || '所选通道'
  if (mode === 'speech') return channel?.id === 'minimax'
    ? '将调用 MiniMax TTS 合成文字配音，按量计费；不会上传任何本地录音。'
    : '使用本机 VoxFlow 模型，本地推理不产生模型费用；首次生成可能需要加载。'
  if (mode === 'video') return channel?.id === 'museav'
    ? '将调用 MuseAV 中台出视频，模型与额度以中台实际返回为准；参考图会上传至 MuseAV。'
    : channel?.id === 'magpie'
    ? '将调用 Magpie 网关出视频，计费与可用性以网关实际返回为准；提交即计费，下载失败可在历史里取回，不会重复提交。'
    : `将调用 ${label} 视频模型，按所选账户额度计费；能否调用以接口返回为准。`
  if (mode === 'image') return channel?.id === 'museav'
    ? '将调用 MuseAV 生图，按账户额度计费；参考图会上传至 MuseAV。'
    : channel?.id === 'magpie'
    ? '将调用 Magpie 网关生图，模型清单由网关自己公布；提交即计费，下载失败可在历史里取回，不会重复提交。'
    : `将调用 ${label} 生图，按你自己的账户计费。`
  return '每次提交生成一个版本，使用所选账户的额度。'
}
async function connections() {
  providers = await api('/api/providers')
  for (const name of ['video', 'image', 'voice']) $(`${name}-dot`).classList.toggle('off', !providers[name]?.ready)
  const rows = ['video', 'image', 'voice', 'export'].map(key => providers[key]).filter(Boolean)
  // 自定义通道的配置问题要单列出来：配置错了却没提示，用户只会以为「这个通道不存在」
  const custom = providers.customChannels
  const customRows = custom?.problems?.length
    ? `<div class="connection-row"><strong><span class="dot off"></span>自定义通道配置</strong><p>${custom.problems.map(escape).join('；')}</p><p>配置文件：${escape(custom.file || '')}</p></div>`
    : ''
  $('connection-list').innerHTML = rows.map(p => `<div class="connection-row"><strong><span class="dot ${p.ready ? '' : 'off'}"></span>${escape(p.label)}</strong><p>${escape(p.note)}</p></div>`).join('') + customRows
  optionList($('profile'), (providers.video?.profiles || []).map(p => ({ id: p.id, name: `${p.label} · 本周余量 ${p.weeklyRemaining == null ? '未知' : p.weeklyRemaining + '%'}` })), providers.video?.profiles?.length ? null : '尚未配置 MiniMax 密钥')
  optionList($('persona'), (providers.voice?.personas || []).map(p => ({ id: p.key, name: p.name })), '选择一个本地音色')
  // 底部连接条列的是「实际可用的通道」，不是写死的三个名字
  const strip = (group, id) => {
    const ready = (providers[group]?.channels || []).filter(c => c.ready).map(c => c.label.split(/[ ·]/)[0])
    $(id).textContent = ready.length ? ready.join(' / ') : '未连接'
  }
  strip('video', 'video-channel-label'); strip('image', 'image-channel-label'); strip('voice', 'voice-channel-label')
  syncBackend()
  rec?.personas(providers.voice || {})
}
/** 空态预览里那行小字：当前真正可用的通道 */
function channelCaption() {
  return ['video', 'image', 'voice'].flatMap(group => (providers[group]?.channels || []).filter(c => c.ready).map(c => c.label.split(/[ ·]/)[0].toUpperCase())).join(' · ') || 'REEL STUDIO'
}
function setMode(next) {
  saveDraft(); mode = next; loadDraft()
  const voice = mode === 'speech', exporting = mode === 'export', recommending = mode === 'recommendation', talking = mode === 'talk', clone = voice && $('voice-mode').value === 'clone'
  document.querySelectorAll('[data-mode]').forEach(button => button.setAttribute('aria-selected', String(button.dataset.mode === mode)))
  const titles = { recommendation: ['从项目到视频', '输入 GitHub 地址 · 编辑分镜并生成旁白', 'PROJECT'], talk: ['把想法讲清楚', '逐句配音 · 知识口播流水线', 'TALK'], video: ['描述你的镜头', 'MiniMax H3 · 让文字与参考图成为动态画面', 'H3'], image: ['创造第一帧', 'MuseAV · 生成图片，再把它交给视频模型', 'IMAGE'], speech: ['让故事有声音', 'VoxFlow · 本地生成与参考音频克隆', 'VOICE'], export: ['把素材变成作品', 'reel-kit · 按选择顺序合成，导出到本地', 'EXPORT'] }
  const [title, subtitle, badge] = titles[mode]
  $('mode-title').textContent = title; $('mode-subtitle').textContent = subtitle; $('engine-badge').textContent = badge
  $('prompt-group').hidden = exporting || recommending || talking; $('export-fields').hidden = !exporting; $('recommendation-fields').hidden = !recommending; $('talk-fields').hidden = !talking
  $('prompt-label').textContent = voice ? '配音文字' : '创作描述'
  $('prompt').placeholder = voice ? '输入你想让声音读出的文字。' : '主体是谁？发生在哪里？镜头如何移动？\n\n试着描述光线、动作和你想留下的情绪。'
  $('voice-work-field').hidden = !voice && !recommending
  $('voice-mode-field').hidden = !voice; $('persona-field').hidden = !voice || clone; $('voice-name-field').hidden = !clone
  $('reference-field').hidden = recommending || exporting || talking || (voice && !clone)
  $('reference-help').textContent = clone ? '使用你拥有或获授权的音频，清晰单人语音更适合克隆。处理在本地完成。' : '选择项目中的图片。参考图会上传至所选通道供模型读取。'
  $('ratio-field').hidden = voice || recommending || talking; $('duration-field').hidden = voice || recommending || talking || mode === 'image'
  $('duration').min = exporting ? 1 : 4; $('duration').max = exporting ? 30 : 15
  $('ratio').querySelector('[value="1:1"]').disabled = exporting
  if (exporting && $('ratio').value === '1:1') $('ratio').value = '9:16'
  $('generate-label').textContent = { recommendation: '生成项目推荐视频', talk: '生成口播视频', video: '生成视频', image: '生成图片', speech: clone ? '克隆并生成配音' : '生成配音', export: '合成并导出' }[mode]
  syncBackend()
  // 本地音色只对 VoxFlow 通道有效；云端 TTS 用独立的音色选择
  const channel = currentChannel()
  if (mode === 'speech' && channel?.id === 'voxflow') $('persona-field').hidden = clone
  $('generation-note').textContent = generationNote()
  if (recommending) loadRecommendation()
  else $('preview').classList.remove('portrait', 'landscape')
  $('use-example').textContent = voice ? '↗ 试试一段旁白' : '↗ 试试时尚棚拍'
  if (talking) loadTalkOptions()
  renderAssets(); renderOptions()
}
const TALK_EXAMPLE = [
  '做短视频，先把信息讲清楚。',
  '一条视频解决一个问题，观众更容易记住。',
  '写好开头、例子和结论，再动手制作。',
].join('\n')
let talkStyles = [], talkStyleReady = false
async function loadTalkOptions() {
  // 风格与版式：一次性取回，之后本地切换
  if (!talkStyles.length) {
    try {
      const data = await api('/api/styles')
      talkStyles = data.styles || []
      optionList($('talk-style'), talkStyles.map(s => ({ id: s.key, name: s.name || s.key })))
      const labels = { 'knowledge-paper': '纸本水墨', 'knowledge-dark': '暗场剪报', 'knowledge-gufeng': '古风卷轴', quote: '金句卡' }
      optionList($('talk-template'), (data.templates || []).map(id => ({ id, name: labels[id] || id })))
      // 默认用水墨纸本：白底黑字，字幕最易读。首次进入才定，之后沿用用户选择。
      if (!talkStyleReady) { $('talk-style').value = talkStyles.find(s => s.key === 'paper')?.key || talkStyles[0]?.key || ''; talkStyleReady = true }
      // 版式跟着风格走（用户手动改过版式就不动）
      const style = talkStyles.find(s => s.key === $('talk-style').value)
      if (style && !$('talk-template').dataset.touched) $('talk-template').value = style.template
    } catch (error) { message(error.message, true) }
  }
  // 音色来自本机 VoxFlow 的人物库；连接状态没拉过就先拉一次
  if (!providers.voice) { try { await connections() } catch { /* 拿不到连接状态就先用空列表 */ } }
  optionList($('talk-persona'), (providers.voice?.personas || []).map(p => ({ id: p.key, name: p.name || p.key })), '选择音色')
  // 背景音乐：项目里的音频素材
  optionList($('talk-bgm'), currentAssets().filter(a => a.mime.startsWith('audio/')).map(a => ({ id: a.id, name: a.name })), '不使用背景音乐')
  updateTalkCount()
}
function updateTalkCount() {
  const lines = $('talk-captions').value.split('\n').map(s => s.trim()).filter(Boolean)
  const over = lines.filter(s => s.length > 80).length
  $('talk-shot-count').textContent = over ? `${lines.length} 镜 · ${over} 行超长` : `${lines.length} 镜`
  $('talk-chars').textContent = `${$('talk-captions').value.length} 字`
}
function saveRecommendation() {
  if (!projectId) return
  localStorage.setItem(`reel-recommendation:${projectId}`, JSON.stringify({ url: $('repo-url').value, title: $('repo-title').value, captions: $('repo-captions').value, images: selected, voices: [...$('scene-voices').querySelectorAll('select')].map(el => el.value), settings: rec?.read() }))
}
function loadRecommendation() {
  let draft = {}
  try { draft = JSON.parse(localStorage.getItem(`reel-recommendation:${projectId}`) || '{}') } catch {}
  $('repo-url').value = draft.url || ''; $('repo-title').value = draft.title || ''; $('repo-captions').value = draft.captions || ''
  selected = Array.isArray(draft.images) ? draft.images.filter(id => currentAssets().some(a => a.id === id)) : []
  renderSceneVoices(); $('scene-voices').querySelectorAll('select').forEach((el, i) => { el.value = draft.voices?.[i] || '' })
  rec?.renderAssets()
  rec?.apply(draft.settings || state.projects.find(p => p.id === projectId)?.recommendationSettings || {})
  restoreChannel(draft.settings || state.projects.find(p => p.id === projectId)?.recommendationSettings || {})
}
function renderSceneVoices() {
  const previous = [...$('scene-voices').querySelectorAll('select')].map(el => el.value)
  const lines = $('repo-captions').value.split('\n').map(s => s.trim()).filter(Boolean)
  const audios = currentAssets().filter(a => a.mime.startsWith('audio/'))
  $('scene-voices').innerHTML = lines.map((line, i) => `<div class="field"><label for="scene-voice-${i}">第 ${i + 1} 镜配音</label><select id="scene-voice-${i}"><option value="">使用所选音色生成</option>${audios.map(a => `<option value="${escape(a.id)}">${escape(a.name)} · ${escape((a.prompt || '').slice(0, 26))}</option>`).join('')}</select></div>`).join('')
  $('scene-voices').querySelectorAll('select').forEach((el, i) => { el.value = previous[i] || '' })
}
const rec = recommendationControls({
  api, message, refresh, connections, optionList,
  getState: () => state,
  getProjectId: () => projectId,
  saveDraft: () => { saveDraft(); saveRecommendation() },
  upload: target => { uploadTarget = target; $('file-input').click() },
  canvas: platform => { $('preview').style.aspectRatio = ''; $('preview').classList.toggle('landscape', platform === 'landscape'); $('preview').classList.toggle('portrait', platform !== 'landscape') },
  voiceChannel: () => ({ backend: voiceState.backend, model: voiceState.model, voiceId: voiceState.voiceId, profile: voiceState.profile }),
})
/**
 * 把一条记录里的通道 / 模型 / 音色载回表单。
 *
 * 没有这一步，「再做一条」出来的片子可能走了另一个供应商、另一个模型，
 * 而界面上看不出来 —— 这正是要避免的（每条片子都要知道是谁做的、用的什么）。
 */
function restoreChannel(input) {
  const group = channelGroup()
  if (!group) return
  const backend = input.backend
  const channel = (providers[group]?.channels || []).find(c => c.id === backend)
  if (channel && $('backend').value !== channel.id) { $('backend').value = channel.id; syncChannel() }
  const model = input.model
  if (model) {
    const known = [...$('model').options].some(o => o.value === model)
    if (known) $('model').value = model
    else { $('model').value = CUSTOM_MODEL; $('model-custom').value = model }
    $('model-custom').hidden = $('model').value !== CUSTOM_MODEL
    applyModelLimits()
  }
  const voiceId = input.voiceId
  if (voiceId) {
    const known = [...$('voice-id').options].some(o => o.value === voiceId)
    if (known) $('voice-id').value = voiceId
    else { $('voice-id').value = CUSTOM_VOICE; $('voice-custom').value = voiceId }
    $('voice-custom').hidden = $('voice-id').value !== CUSTOM_VOICE
  }
  if (input.profile && [...$('profile').options].some(o => o.value === input.profile)) $('profile').value = input.profile
  if (group === 'voice') { voiceState.backend = $('backend').value; voiceState.model = currentModel(); voiceState.voiceId = currentVoiceId(); voiceState.profile = $('profile').value || ''; saveVoiceState() }
}
// 「调整这条视频」：把这条视频的输入与配置载回制作表单，改完再生成。
// 只影响当前项目，不新建项目，也不改动旧记录。
function reuseJob(job) {
  setMode(job.input.kind === 'clone' ? 'speech' : job.input.kind)
  if (job.input.kind === 'clone') { $('voice-mode').value = job.input.persona ? 'speech' : 'clone'; $('voice-name').value = job.input.voiceName || ''; $('reference-text').value = job.input.referenceText || ''; setMode('speech') }
  $('prompt').value = job.input.prompt || ''; $('prompt').dispatchEvent(new Event('input'))
  for (const key of ['ratio', 'duration', 'resolution', 'profile', 'persona', 'template', 'captions']) if (job.input[key]) $(key).value = job.input[key]
  // 通道与模型一起载回来：换了模型再生成，才知道这次跟上次不是同一条路径
  restoreChannel(job.input)
  $('reference').value = job.input.referenceId || ''; $('audio').value = job.input.audioId || ''; $('export-title').value = job.input.title || ''
  selected = job.input.assetIds || []; renderAssets()
  if (job.input.kind === 'recommendation') {
    $('repo-url').value = job.input.repoUrl || ''; $('repo-title').value = job.input.title || ''; $('repo-captions').value = job.input.captions || ''
    renderSceneVoices(); $('scene-voices').querySelectorAll('select').forEach((el, i) => { el.value = job.input.voiceIds?.[i] || '' }); saveRecommendation()
    rec.apply(job.input); rec.renderAssets()
  }
  message('参数已载入，可以修改后再提交。不会自动重新生成。'); $('generate-form').scrollIntoView({ block: 'start', behavior: 'smooth' })
}
// 「照这个方法再做一条」：新开一个项目，继承同一版本的制作方法与素材，
// 内容输入不继承 —— 新一条视频要重新确认内容。
async function applyRemake(result) {
  projectId = result.projectId
  localStorage.setItem('reel-project', projectId)
  await refresh()
  selected = result.imageIds.filter(Boolean)
  $('repo-url').value = result.draft.url || ''; $('repo-title').value = result.draft.title || ''; $('repo-captions').value = result.draft.captions || ''
  rec.apply(result.settings)
  renderSceneVoices(); $('scene-voices').querySelectorAll('select').forEach((el, i) => { el.value = result.voiceIds?.[i] || '' })
  renderAssets(); saveRecommendation(); setMode('recommendation')
  // setMode 会按上次的偏好重置通道 —— 继承回来的版本必须再压回去，否则「沿用同一套方法」是假的
  rec.apply(result.settings); restoreChannel(result.settings)
  $('repo-url').scrollIntoView({ behavior: 'smooth', block: 'center' })
  message(`已新建「${result.projectName}」，沿用 v${result.sopVersion} 的制作方法与素材；确认内容后即可生成。`)
}
let lastWorkflowKey = ''
const workflow = workflowPanel({ api, getState: () => state, getProjectId: () => projectId, refresh, message, applyVersion: version => { rec.apply({ ...rec.read(), ...version.settings }); restoreChannel(version.settings); saveRecommendation() }, onRemake: applyRemake, onAdjust: reuseJob })
const library = voiceLibrary({ api, message, refresh, connections })
$('load-voice-work').onclick = async () => {
  try { const tasks = await api('/api/voice/tasks'); optionList($('voice-work'), tasks.map(t => ({ id: t.id, name: `${t.name} · ${t.text.slice(0, 25)}` })), '选择声音作品'); message(`找到 ${tasks.length} 条声音作品。`) } catch (error) { message(error.message, true) }
}
$('import-voice-work').onclick = async () => {
  try { if (!$('voice-work').value) throw new Error('请选择声音作品'); await api('/api/voice/import', { method: 'POST', body: JSON.stringify({ projectId, taskId: $('voice-work').value }) }); await refresh(); if (mode === 'recommendation') renderSceneVoices(); message('声音作品已加入当前项目素材。') } catch (error) { message(error.message, true) }
}
$('repo-captions').addEventListener('input', () => { renderSceneVoices(); saveRecommendation() })
for (const field of ['repo-url', 'repo-title']) $(field).addEventListener('input', saveRecommendation)
$('scene-voices').addEventListener('change', saveRecommendation)
$('read-repo').onclick = async () => {
  $('read-repo').disabled = true
  try {
    const draft = await api('/api/recommendation/draft', { method: 'POST', body: JSON.stringify({ projectId, repoUrl: $('repo-url').value }) })
    $('repo-title').value = draft.title; $('repo-captions').value = draft.captions.join('\n')
    selected = draft.images.map(image => image.id)
    await refresh(); renderSceneVoices(); saveRecommendation()
    message(draft.images.length ? `已读取 ${draft.repo}；请检查分镜与画面后生成。` : '已读取项目介绍；仓库没有找到可用展示图，请在素材库导入并勾选画面。')
  } catch (error) { message(error.message, true) } finally { $('read-repo').disabled = false }
}
$('prompt').addEventListener('input', () => { saveDraft(); $('char-count').textContent = `${$('prompt').value.length} 字` })
$('use-example').onclick = () => { $('prompt').value = examples[mode] || ''; $('prompt').dispatchEvent(new Event('input')) }
$('step-prepare').onclick = () => setMode('recommendation')
$('start-github').onclick = async () => {
  const button = $('start-github'); button.disabled = true
  try {
    const project = await api('/api/projects', { method: 'POST', body: JSON.stringify({ name: `GitHub 推荐 · ${new Date().toLocaleDateString('zh-CN')}` }) })
    await refresh(); switchProject(project.id); setMode('recommendation')
    $('repo-url').scrollIntoView({ behavior: 'smooth', block: 'center' }); $('repo-url').focus({ preventScroll: true })
  } catch (error) { message(error.message, true) } finally { button.disabled = false }
}
document.querySelectorAll('[data-mode]').forEach(button => button.onclick = () => setMode(button.dataset.mode))
$('talk-example').onclick = () => { $('talk-captions').value = TALK_EXAMPLE; $('talk-title').value = $('talk-title').value || '把一个观点讲清楚的三个动作'; updateTalkCount() }
$('talk-captions').addEventListener('input', updateTalkCount)
$('talk-style').addEventListener('change', () => {
  // 风格自带推荐版式；用户没手动改过版式时跟着风格走
  const style = talkStyles.find(s => s.key === $('talk-style').value)
  if (style && !$('talk-template').dataset.touched) $('talk-template').value = style.template
})
$('talk-template').addEventListener('change', () => { $('talk-template').dataset.touched = '1' })
$('start-talk').onclick = async () => {
  const button = $('start-talk'); button.disabled = true
  try {
    const project = await api('/api/projects', { method: 'POST', body: JSON.stringify({ name: `知识口播 · ${new Date().toLocaleDateString('zh-CN')}` }) })
    await refresh(); switchProject(project.id); setMode('talk')
    $('talk-title').focus({ preventScroll: true }); $('talk-title').scrollIntoView({ behavior: 'smooth', block: 'center' })
  } catch (error) { message(error.message, true) } finally { button.disabled = false }
}
document.querySelectorAll('[data-filter]').forEach(button => button.onclick = () => { filter = button.dataset.filter; document.querySelectorAll('[data-filter]').forEach(b => b.classList.toggle('active', b === button)); renderAssets() })
$('backfill-assets').onclick = async () => {
  try {
    const result = await api('/api/assets/backfill', { method: 'POST', body: '{}' })
    await refresh()
    const parts = []
    if (result.updated) parts.push(`补全 ${result.updated} 条素材的尺寸与时长`)
    if (result.total) parts.push(`共 ${result.total} 条待补`)
    if (result.failed?.length) parts.push(`${result.failed.length} 条探测失败`)
    let text = parts.length ? `${parts.join('，')}。` : '历史素材都是齐的，没有需要补全的。'
    // 探测失败的素材必须能被看见 —— 否则「都补好了」是假象
    if (result.failed?.length) text += ` 失败明细：${result.failed.map(f => `${f.name}（${f.reason}）`).join('；')}`
    message(text, result.failed?.length > 0)
  } catch (err) { message(err.message, true) }
}
$('voice-mode').onchange = () => setMode('speech')
$('backend').addEventListener('change', () => {
  const group = channelGroup()
  if (group) localStorage.setItem(`reel-channel:${group}`, $('backend').value)
  // 换通道 = 换一套模型与音色，旧的选择不再成立
  if (group === 'voice') { voiceState.backend = $('backend').value; voiceState.model = ''; voiceState.voiceId = ''; saveVoiceState() }
  // 换通道后，上一条通道记住的模型不再适用（型号、时长范围、分辨率都不同）
  else if (group) { localStorage.removeItem(`reel-model:${group}`); $('model').value = '' }
  syncChannel()
  $('generation-note').textContent = generationNote()
  rec?.sync()
})
$('model').addEventListener('change', () => {
  $('model-custom').hidden = $('model').value !== CUSTOM_MODEL
  applyModelLimits()
  const group = channelGroup()
  if (group === 'voice') { voiceState.model = currentModel(); saveVoiceState() }
  else if (group) localStorage.setItem(`reel-model:${group}`, currentModel())
})
$('voice-id').addEventListener('change', () => {
  $('voice-custom').hidden = $('voice-id').value !== CUSTOM_VOICE
  if (channelGroup() === 'voice') { voiceState.voiceId = currentVoiceId(); saveVoiceState() }
})
$('model-custom').addEventListener('input', () => {
  const group = channelGroup()
  if (group === 'voice') { voiceState.model = currentModel(); saveVoiceState() }
  else if (group) localStorage.setItem(`reel-model:${group}`, currentModel())
})
$('voice-custom').addEventListener('input', () => {
  if (channelGroup() === 'voice') { voiceState.voiceId = currentVoiceId(); saveVoiceState() }
})
$('profile').addEventListener('change', () => {
  if (channelGroup() === 'voice') { voiceState.profile = $('profile').value || ''; saveVoiceState() }
})
$('projects').onclick = event => switchProject(event.target.closest('[data-project]')?.dataset.project)
$('active-project').onchange = () => switchProject($('active-project').value)
$('new-project-top').onclick = () => { $('project-dialog').showModal(); $('new-name').focus() }
$('new-project').onclick = () => { $('project-dialog').showModal(); $('new-name').focus() }
document.querySelectorAll('[data-close]').forEach(button => button.onclick = () => $(button.dataset.close).close())
$('project-form').onsubmit = async event => {
  event.preventDefault()
  try { const project = await api('/api/projects', { method: 'POST', body: JSON.stringify({ name: $('new-name').value }) }); saveDraft(); projectId = project.id; localStorage.setItem('reel-project', projectId); selected = []; loadDraft(); $('project-dialog').close(); $('new-name').value = ''; await refresh() } catch (error) { message(error.message, true) }
}
$('connections').onclick = async () => { $('connections-dialog').showModal(); try { await connections() } catch (error) { message(error.message, true) } }
$('refresh-connections').onclick = async () => { try { await connections(); message('连接状态已更新') } catch (error) { message(error.message, true) } }
for (const name of ['upload-top', 'upload-reference']) $(name).onclick = () => { uploadTarget = null; $('file-input').click() }
// 配乐库：默认走 R2 上的清单，本地上传仍可用（上传的音频也在同一个下拉里）
let bgmLibrary = []
async function loadBgmLibrary() {
  try {
    const result = await api('/api/bgm')
    bgmLibrary = result.items || []
    const select = $('bgm-library')
    select.innerHTML = bgmLibrary.length
      // 选项文字要短：下拉框宽度有限，堆太多信息会被截断成「轻快/可爱/日…」，反而看不出是什么
      ? bgmLibrary.map(b => `<option value="${escape(b.alias)}">${escape(b.alias)}${b.kind === 'track' ? '（原创）' : ''}</option>`).join('')
      : '<option value="">配乐库为空</option>'
    select.disabled = !bgmLibrary.length
    const first = bgmLibrary[0]
    $('bgm-library-note').textContent = result.error
      ? `配乐库暂时拉不到：${result.error}`
      : result.configured
        ? (bgmLibrary.length
          ? `${bgmLibrary.length} 首可选。${first.license ? '授权：' + first.license : ''}取用后会加入本项目素材，授权与来源一并记录。`
          : '配乐库已连接，但清单里还没有配乐。')
        : '还没配置配乐库。可以在 ~/.reel-kit/config.json 里写 { "bgmManifest": "https://…/manifest.json" }，或直接用上面的上传按钮。'
    // 授权随所选曲目走：换一首就要换一段授权说明，不能一直显示第一首的
    const updateLicense = () => {
      const picked = bgmLibrary.find(b => b.alias === $('bgm-library').value)
      $('bgm-library-note').textContent = picked
        ? `「${picked.alias}」${picked.license ? ' · 授权：' + picked.license : ''}${picked.source ? ' · 来源：' + picked.source : ''}。取用后会加入本项目素材。`
        : $('bgm-library-note').textContent
    }
    select.onchange = updateLicense
    updateLicense()
  } catch (error) {
    $('bgm-library-note').textContent = '配乐库暂时连不上：' + error.message
    $('bgm-library').innerHTML = '<option value="">配乐库不可用</option>'
    $('bgm-library').disabled = true
  }
}
$('bgm-library-pull').onclick = async () => {
  const alias = $('bgm-library').value
  if (!alias) return message('请先选择要取用的配乐', true)
  const button = $('bgm-library-pull'); button.disabled = true
  try {
    const item = bgmLibrary.find(b => b.alias === alias) || {}
    const asset = await api('/api/bgm/pull', { method: 'POST', body: JSON.stringify({ projectId, alias }) })
    await refresh()
    $('rec-bgm').value = asset.id
    rec.sync(); saveRecommendation()
    message(`已取用「${alias}」并加入本项目素材。${item.license ? '授权：' + item.license : ''}`)
  } catch (error) { message(error.message, true) } finally { button.disabled = false }
}
$('assets').onclick = event => {
  if (event.target.closest('#empty-upload')) $('file-input').click()
  const button = event.target.closest('[data-preview]'); if (button) showPreview(button.dataset.preview)
}
$('assets').onchange = event => { const id = event.target.dataset.select; if (!id) return; if (event.target.checked) selected.push(id); else selected = selected.filter(s => s !== id); if (mode === 'recommendation') saveRecommendation(); renderAssets() }
$('export-order').onclick = event => { const { up, remove } = event.target.dataset; if (remove) selected = selected.filter(id => id !== remove); if (up) { const index = selected.indexOf(up); [selected[index - 1], selected[index]] = [selected[index], selected[index - 1]] } renderAssets() }
$('file-input').onchange = async () => {
  const target = uploadTarget; uploadTarget = null
  try {
    let asset
    for (const file of $('file-input').files) {
      asset = await api(`/api/assets?projectId=${encodeURIComponent(projectId)}&name=${encodeURIComponent(file.name)}`, { method: 'POST', headers: { 'Content-Type': file.type || ({ wav: 'audio/wav', mp3: 'audio/mpeg', m4a: 'audio/mp4' }[file.name.split('.').at(-1)] || 'application/octet-stream') }, body: file })
    }
    await refresh()
    if (asset) { showPreview(asset.id); if (!target) $('reference').value = asset.id }
    if (target === 'bgm' && asset) { $('rec-bgm').value = asset.id; rec.sync(); message('BGM 已加入项目并选中，先试听再调整音量。') }
    else if (target === 'narration' && asset) { $('rec-narration').value = asset.id; $('rec-voice-source').value = 'audio'; rec.sync(); message('旁白素材已加入项目并选中。') }
    else message('素材已保存到本地项目')
  } catch (error) { message(error.message, true) } finally { $('file-input').value = '' }
}
$('generate-form').onsubmit = async event => {
  event.preventDefault(); $('generate').disabled = true
  try {
    const job = { projectId, kind: mode === 'speech' ? $('voice-mode').value : mode, prompt: $('prompt').value, ratio: $('ratio').value, duration: Number($('duration').value), resolution: $('resolution').value, profile: $('profile').value || 'primary', backend: currentChannel()?.id, model: currentModel(), voiceId: currentVoiceId(), referenceId: $('reference').value || undefined, persona: currentChannel()?.id === 'voxflow' ? ($('persona').value || undefined) : undefined, voiceName: $('voice-name').value, assetIds: selected, audioId: $('audio').value || undefined, template: $('template').value, title: $('export-title').value, captions: $('captions').value }
    if (job.kind === 'clone') { job.persona = undefined; job.referenceText = $('reference-text').value }
    if (mode === 'recommendation') Object.assign(job, { repoUrl: $('repo-url').value, title: $('repo-title').value, captions: $('repo-captions').value, assetIds: selected, voiceIds: [...$('scene-voices').querySelectorAll('select')].map(el => el.value) }, rec.read(), { backend: currentChannel()?.id, model: currentModel(), voiceId: currentVoiceId(), profile: voiceState.profile || job.profile })
    if (mode === 'talk') Object.assign(job, {
      kind: 'talk', title: $('talk-title').value.trim(), captions: $('talk-captions').value,
      subtitle: $('talk-subtitle').value.trim(), style: $('talk-style').value, template: $('talk-template').value,
      persona: $('talk-persona').value || undefined, tone: $('talk-tone').value.trim(),
      voiceBackend: 'voxflow', bgmId: $('talk-bgm').value || undefined, assetIds: selected,
      ratio: '9:16', profile: 'primary',
    })
    await api('/api/jobs', { method: 'POST', body: JSON.stringify(job) }); message('任务已加入队列，完成后会出现在素材库。'); await refresh()
  } catch (error) { message(error.message, true) } finally { $('generate').disabled = false }
}
$('jobs').onclick = event => {
  const preview = event.target.closest('[data-preview]')
  if (preview) { showPreview(preview.dataset.preview); document.querySelector('.preview-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' }); return }
  const resume = event.target.closest('[data-resume]')?.dataset.resume
  if (resume) {
    const button = event.target.closest('[data-resume]')
    button.disabled = true; button.textContent = '正在取回…'
    api('/api/jobs/resume', { method: 'POST', body: JSON.stringify({ jobId: resume }) })
      .then(() => { message('已取回上游生成的素材，费用没有重复产生。'); return refresh() })
      .catch(error => { message(error.message, true); button.disabled = false; button.textContent = '取回已生成的素材' })
    return
  }
  const fix = event.target.closest('[data-fix]')?.dataset.fix
  if (fix) {
    if (mode !== 'recommendation') setMode('recommendation')
    const target = { voice: 'rec-voice-source', scenes: 'repo-captions', generate: 'generate' }[fix] || 'generate'
    $(target).scrollIntoView({ block: 'center', behavior: 'smooth' })
    return
  }
  const id = event.target.closest('[data-reuse]')?.dataset.reuse
  const job = state.jobs.find(j => j.id === id); if (!job) return
  reuseJob(job)
}
async function boot() {
  try {
    await refresh()
    if (!state.projects.length) { const project = await api('/api/projects', { method: 'POST', body: JSON.stringify({ name: '我的第一支短片' }) }); projectId = project.id; localStorage.setItem('reel-project', projectId); await refresh() }
    loadDraft(); await connections(); loadBgmLibrary()
  } catch (error) { message(error.message, true) }
  let busy = false
  setInterval(async () => { if (busy || document.hidden) return; busy = true; try { await refresh() } catch (error) { message('工作台连接中断，请检查本地服务。' + error.message, true) } finally { busy = false } }, 3000)
}
boot()
