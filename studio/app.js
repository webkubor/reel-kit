const $ = id => document.getElementById(id)
const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
import { recommendationControls } from '/recommendation-controls.js'
import { voiceLibrary } from '/voice-library.js'
import { workflowPanel } from '/workflow-panel.js'
let state = { projects: [], assets: [], jobs: [], templates: [] }, providers = {}, mode = 'video', filter = 'all'
let projectId = localStorage.getItem('reel-project'), previewId, selected = [], lastAssets = '', lastJobs = '', lastProjects = '', uploadTarget
const kinds = { video: '视频', image: '图片', speech: '配音', clone: '克隆配音', 'voice-design': '新建音色', recommendation: '项目推荐', export: '合成' }
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
  else { $('preview').innerHTML = '<div class="preview-empty"><h3>这个项目的第一帧，等你创造</h3><p>生成或导入素材后，在这里预览。</p></div>'; $('download').hidden = true; $('preview-name').textContent = '还没有选中素材' }
}
function renderAssets() {
  const assets = currentAssets()
  const key = JSON.stringify([assets, filter, selected, mode])
  if (key === lastAssets) return
  lastAssets = key
  $('asset-count').textContent = assets.length
  const visible = assets.filter(a => filter === 'all' || a.mime.startsWith(filter + '/'))
  $('assets').innerHTML = visible.length ? visible.map(a => `<article class="asset-card ${selected.includes(a.id) ? 'selected' : ''}"><button class="asset-open" data-preview="${a.id}" aria-label="预览 ${escape(a.name)}">${a.mime.startsWith('image/') ? `<img src="${a.url}" alt="${escape(a.name)}" loading="lazy">` : a.mime.startsWith('video/') ? `<video src="${a.url}#t=0.1" preload="metadata" muted playsinline></video>` : '<span class="audio-symbol">≋</span>'}</button>${(mode === 'export' && !a.mime.startsWith('audio/') || mode === 'recommendation' && a.mime.startsWith('image/')) ? `<input class="asset-check" type="checkbox" data-select="${a.id}" aria-label="选择 ${escape(a.name)}" ${selected.includes(a.id) ? 'checked' : ''}>` : ''}<div class="asset-info"><strong>${escape(a.name)}</strong><span>${escape(a.source)} · ${new Date(a.createdAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</span>${assetMeta(a) ? `<span class="asset-meta">${assetMeta(a)}</span>` : ''}</div></article>`).join('') : `<button type="button" class="library-empty" id="empty-upload"><span class="plus">＋</span><span>${assets.length ? '这个分类还没有素材' : '导入你的第一份素材'}<small>图片、视频、声音，或从上方开始生成</small></span></button>`
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
    return `<article class="job-item"><div class="job-row"><span class="job-kind">${kinds[j.input.kind]}</span><div><p class="job-prompt" title="${escape(j.input.prompt || j.input.title)}">${escape(j.input.prompt || j.input.title || '合成出片')}</p><p class="job-detail">${escape(j.error || j.message)}</p></div><span class="job-state ${j.status}">${labels[j.status]}</span><button class="text-button" data-reuse="${j.id}">复用参数 ↗</button></div>${timeline}</article>`
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
  const workflowKey = JSON.stringify([projectId, state.jobs.filter(j => j.projectId === projectId && j.input.kind === 'recommendation' && j.status === 'succeeded').map(j => j.id)])
  if (workflowKey !== lastWorkflowKey) { lastWorkflowKey = workflowKey; workflow.render() }
}
async function connections() {
  providers = await api('/api/providers')
  for (const name of ['video', 'image', 'voice']) $(`${name}-dot`).classList.toggle('off', !providers[name]?.ready)
  $('connection-list').innerHTML = Object.values(providers).map(p => `<div class="connection-row"><strong><span class="dot ${p.ready ? '' : 'off'}"></span>${escape(p.label)}</strong><p>${escape(p.note)}</p></div>`).join('')
  optionList($('profile'), (providers.video?.profiles || []).map(p => ({ id: p.id, name: `${p.label} · 本周余量 ${p.weeklyRemaining == null ? '未知' : p.weeklyRemaining + '%'}` })), providers.video?.profiles?.length ? null : '尚未配置 MiniMax 密钥')
  optionList($('persona'), (providers.voice?.personas || []).map(p => ({ id: p.key, name: p.name })), '选择一个本地音色')
  rec?.personas(providers.voice || {})
}
function setMode(next) {
  saveDraft(); mode = next; loadDraft()
  const voice = mode === 'speech', exporting = mode === 'export', recommending = mode === 'recommendation', clone = voice && $('voice-mode').value === 'clone'
  document.querySelectorAll('[data-mode]').forEach(button => button.setAttribute('aria-selected', String(button.dataset.mode === mode)))
  const titles = { recommendation: ['从项目到视频', '输入 GitHub 地址 · 编辑分镜并生成旁白', 'PROJECT'], video: ['描述你的镜头', 'MiniMax H3 · 让文字与参考图成为动态画面', 'H3'], image: ['创造第一帧', 'MuseAV · 生成图片，再把它交给视频模型', 'IMAGE'], speech: ['让故事有声音', 'VoxFlow · 本地生成与参考音频克隆', 'VOICE'], export: ['把素材变成作品', 'reel-kit · 按选择顺序合成，导出到本地', 'EXPORT'] }
  const [title, subtitle, badge] = titles[mode]
  $('mode-title').textContent = title; $('mode-subtitle').textContent = subtitle; $('engine-badge').textContent = badge
  $('prompt-group').hidden = exporting || recommending; $('export-fields').hidden = !exporting; $('recommendation-fields').hidden = !recommending
  $('prompt-label').textContent = voice ? '配音文字' : '创作描述'
  $('prompt').placeholder = voice ? '输入你想让声音读出的文字。' : '主体是谁？发生在哪里？镜头如何移动？\n\n试着描述光线、动作和你想留下的情绪。'
  $('voice-work-field').hidden = !voice && !recommending
  $('voice-mode-field').hidden = !voice; $('persona-field').hidden = !voice || clone; $('voice-name-field').hidden = !clone
  $('reference-field').hidden = recommending || exporting || (voice && !clone)
  $('reference-help').textContent = clone ? '使用你拥有或获授权的音频，清晰单人语音更适合克隆。处理在本地完成。' : '选择项目中的图片。参考图会上传至 MuseAV 供模型读取。'
  $('ratio-field').hidden = voice || recommending; $('duration-field').hidden = voice || recommending || mode === 'image'
  $('resolution-field').hidden = mode !== 'video'; $('profile-field').hidden = mode !== 'video'
  $('duration').min = exporting ? 1 : 4; $('duration').max = exporting ? 30 : 15
  $('ratio').querySelector('[value="1:1"]').disabled = exporting
  if (exporting && $('ratio').value === '1:1') $('ratio').value = '9:16'
  $('generate-label').textContent = { recommendation: '生成项目推荐视频', video: '生成视频', image: '生成图片', speech: clone ? '克隆并生成配音' : '生成配音', export: '合成并导出' }[mode]
  $('generation-note').textContent = recommending
    ? '本次制作全程本地完成：仓库信息经 GitHub API 读取，配音用本机 VoxFlow，渲染与合成在本机执行，不调用付费模型。'
    : voice ? '使用本机 VoxFlow 模型，本地推理不产生模型费用；首次生成可能需要加载。'
    : exporting ? '选中的素材按顺序导出；原始素材会保留。'
    : mode === 'video' ? '将调用 MiniMax H3 视频模型，按所选账户额度计费；能否调用以接口返回为准。'
    : mode === 'image' ? '将调用 MuseAV 生图，按账户额度计费；参考图会上传至 MuseAV。'
    : '每次提交生成一个版本，使用所选账户的额度。'
  if (recommending) loadRecommendation()
  else $('preview').classList.remove('portrait', 'landscape')
  $('use-example').textContent = voice ? '↗ 试试一段旁白' : '↗ 试试时尚棚拍'
  renderAssets(); renderOptions()
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
})
let lastWorkflowKey = ''
const workflow = workflowPanel({ api, getState: () => state, getProjectId: () => projectId, refresh, message, applyVersion: version => { rec.apply({ ...rec.read(), ...version.settings }); saveRecommendation() } })
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
document.querySelectorAll('[data-filter]').forEach(button => button.onclick = () => { filter = button.dataset.filter; document.querySelectorAll('[data-filter]').forEach(b => b.classList.toggle('active', b === button)); renderAssets() })
$('voice-mode').onchange = () => setMode('speech')
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
    const job = { projectId, kind: mode === 'speech' ? $('voice-mode').value : mode, prompt: $('prompt').value, ratio: $('ratio').value, duration: Number($('duration').value), resolution: $('resolution').value, profile: $('profile').value || 'primary', referenceId: $('reference').value || undefined, persona: $('persona').value || undefined, voiceName: $('voice-name').value, assetIds: selected, audioId: $('audio').value || undefined, template: $('template').value, title: $('export-title').value, captions: $('captions').value }
    if (job.kind === 'clone') { job.persona = undefined; job.referenceText = $('reference-text').value }
    if (mode === 'recommendation') Object.assign(job, { repoUrl: $('repo-url').value, title: $('repo-title').value, captions: $('repo-captions').value, assetIds: selected, voiceIds: [...$('scene-voices').querySelectorAll('select')].map(el => el.value) }, rec.read())
    await api('/api/jobs', { method: 'POST', body: JSON.stringify(job) }); message('任务已加入队列，完成后会出现在素材库。'); await refresh()
  } catch (error) { message(error.message, true) } finally { $('generate').disabled = false }
}
$('jobs').onclick = event => {
  const preview = event.target.closest('[data-preview]')
  if (preview) { showPreview(preview.dataset.preview); document.querySelector('.preview-panel')?.scrollIntoView({ behavior: 'smooth', block: 'start' }); return }
  const fix = event.target.closest('[data-fix]')?.dataset.fix
  if (fix) {
    if (mode !== 'recommendation') setMode('recommendation')
    const target = { voice: 'rec-voice-source', scenes: 'repo-captions', generate: 'generate' }[fix] || 'generate'
    $(target).scrollIntoView({ block: 'center', behavior: 'smooth' })
    return
  }
  const id = event.target.closest('[data-reuse]')?.dataset.reuse
  const job = state.jobs.find(j => j.id === id); if (!job) return
  setMode(job.input.kind === 'clone' ? 'speech' : job.input.kind)
  if (job.input.kind === 'clone') { $('voice-mode').value = job.input.persona ? 'speech' : 'clone'; $('voice-name').value = job.input.voiceName || ''; $('reference-text').value = job.input.referenceText || ''; setMode('speech') }
  $('prompt').value = job.input.prompt || ''; $('prompt').dispatchEvent(new Event('input'))
  for (const key of ['ratio', 'duration', 'resolution', 'profile', 'persona', 'template', 'captions']) if (job.input[key]) $(key).value = job.input[key]
  $('reference').value = job.input.referenceId || ''; $('audio').value = job.input.audioId || ''; $('export-title').value = job.input.title || ''
  selected = job.input.assetIds || []; renderAssets()
  if (job.input.kind === 'recommendation') {
    $('repo-url').value = job.input.repoUrl || ''; $('repo-title').value = job.input.title || ''; $('repo-captions').value = job.input.captions || ''
    renderSceneVoices(); $('scene-voices').querySelectorAll('select').forEach((el, i) => { el.value = job.input.voiceIds?.[i] || '' }); saveRecommendation()
    rec.apply(job.input); rec.renderAssets()
  }
  message('参数已载入，可以修改后再提交。不会自动重新生成。'); $('generate-form').scrollIntoView({ block: 'start', behavior: 'smooth' })
}
async function boot() {
  try {
    await refresh()
    if (!state.projects.length) { const project = await api('/api/projects', { method: 'POST', body: JSON.stringify({ name: '我的第一支短片' }) }); projectId = project.id; localStorage.setItem('reel-project', projectId); await refresh() }
    loadDraft(); await connections()
  } catch (error) { message(error.message, true) }
  let busy = false
  setInterval(async () => { if (busy || document.hidden) return; busy = true; try { await refresh() } catch (error) { message('工作台连接中断，请检查本地服务。' + error.message, true) } finally { busy = false } }, 3000)
}
boot()
