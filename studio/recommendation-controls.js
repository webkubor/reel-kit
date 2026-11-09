const $ = id => document.getElementById(id)
const defaults = { platform: 'channels', voiceSource: 'tts', persona: '', tone: '', speed: 1, pause: 0.35, bgmId: '', bgmGain: 0.12, narrationId: '', imageFit: 'contain' }
const fields = { platform: 'publish-platform', voiceSource: 'rec-voice-source', persona: 'rec-persona', tone: 'rec-tone', speed: 'rec-speed', pause: 'rec-pause', bgmId: 'rec-bgm', narrationId: 'rec-narration', imageFit: 'image-fit' }
const styles = {
  channels: { tone: '自然清晰，温和可信，像面对面分享经验，不使用夸张播音腔', speed: 1, pause: 0.35, note: '建议起点：清晰、自然、留出停顿。最终以你的试听判断为准。' },
  xiaohongshu: { tone: '亲切轻快，像向朋友分享发现，咬字清楚，有交流感，不刻意卖萌', speed: 1.15, pause: 0.2, note: '建议起点：亲切分享、节奏轻快。平台不限定声音性别或年龄。' },
  landscape: { tone: '自然清晰，节奏平稳，像在演示产品功能', speed: 1, pause: 0.35, note: '建议起点：清楚讲解，给观众观察画面的时间。' },
}
export function recommendationControls({ api, getState, getProjectId, optionList, message, refresh, connections, saveDraft, upload, canvas, voiceChannel }) {
  let pendingDesign, pendingTrial
  function read() {
    const result = Object.fromEntries(Object.entries(fields).map(([key, id]) => [key, $(id).value]))
    result.speed = Number(result.speed); result.pause = Number(result.pause); result.bgmGain = Number($('rec-bgm-gain').value) / 100
    // 通道、模型、音色也存进项目设置 —— 换一台机器、换一个人接手时，配音还是同一个声音
    Object.assign(result, voiceChannel?.() || {})
    return result
  }
  function audio(id, src) {
    const el = $(id)
    if (el.getAttribute('src') !== src) { el.pause(); if (src) el.src = src; else { el.removeAttribute('src'); el.load() } }
    el.hidden = !src
  }
  function sync() {
    const s = read(), assets = getState().assets.filter(a => a.projectId === getProjectId())
    $('rec-tts-settings').hidden = s.voiceSource === 'audio'
    $('rec-narration-settings').hidden = s.voiceSource !== 'audio'
    $('scene-voices').hidden = s.voiceSource !== 'scenes'
    $('rec-pause').disabled = s.voiceSource === 'audio'
    $('rec-bgm-gain').disabled = !s.bgmId
    $('bgm-level').textContent = `${Math.round(s.bgmGain * 100)}%`
    $('canvas-size').textContent = s.platform === 'landscape' ? '1920 × 1080 · 横屏' : '1080 × 1920 · 竖屏'
    $('platform-style-note').textContent = styles[s.platform]?.note || ''
    audio('rec-voice-sample', s.persona ? `/api/voice/sample?key=${encodeURIComponent(s.persona)}` : '')
    audio('rec-narration-preview', assets.find(a => a.id === s.narrationId)?.url || '')
    audio('rec-bgm-preview', assets.find(a => a.id === s.bgmId)?.url || '')
    for (const id of ['rec-voice-sample', 'rec-narration-preview']) { $(id).playbackRate = s.speed; $(id).preservesPitch = true }
    $('rec-bgm-preview').volume = s.bgmGain
    // 走云端 TTS 时，本地音色选择没有意义（音色是另一个供应商的 voice_id）
    const cloud = s.backend && s.backend !== 'voxflow'
    $('rec-persona').closest('.field').hidden = cloud
    $('preview-persona').disabled = cloud
    // 云端试听：以前直接禁掉，用户就没法判断「这个音色听着对不对」——
    // 只能先花钱生成整条片子再听。改成照常发一句试听任务。
    $('trial-voice').disabled = false
    $('trial-voice').textContent = cloud ? '用这句文案试听（云端计费）' : '用这句文案试听'
    // MiniMax 的 TTS 接口没有「情绪」参数：tone 只在本地 VoxFlow 上真正生效。
    // 界面不能让人以为填了就会生效。
    $('rec-tone').disabled = cloud
    $('rec-tone').closest('.field').hidden = false
    $('voice-selection-help').textContent = cloud
      ? '当前用云端配音通道，音色在上方「生成通道」里选；本地音色不参与这条片子。' + (s.backend === 'minimax' ? 'MiniMax TTS 没有情绪参数，「表达方式」对它不生效。' : '')
      : s.voiceSource === 'scenes' ? '下方逐句选择的旧配音会保留。需要全片换音色时，改选「用所选音色生成旁白」。'
      : s.persona ? '切换音色或表达方式后会生成新旁白；画面与原配音保留。' : '请选择已有音色并试听，或点击「新建音色」。不会自动使用默认音色。'
  }
  function renderAssets() {
    const assets = getState().assets.filter(a => a.projectId === getProjectId() && a.mime.startsWith('audio/'))
    // 授权与来源直接写进选项里：片子是要对外发的，「这首能不能商用」必须在选它的时候就看得见
    const values = assets.map(a => ({ id: a.id, name: a.license ? `${a.name}（${a.license}）` : a.name }))
    optionList($('rec-narration'), assets.map(a => ({ id: a.id, name: a.name })), '选择当前项目的音频')
    optionList($('rec-bgm'), values, '不使用背景音乐')
    sync()
  }
  let wantedPersona = ''
  function personas(provider) {
    const list = provider.personas || []
    const preferred = $('rec-persona').value || wantedPersona
    optionList($('rec-persona'), list.map(p => ({ id: p.key, name: p.name })), list.length ? '请选择音色' : '音色加载中…')
    // 列表没到之前不让选，否则会把「没选上」当成「用户没选」
    $('rec-persona').disabled = !list.length
    if (preferred && [...$('rec-persona').options].some(o => o.value === preferred)) $('rec-persona').value = preferred
    $('design-submit').disabled = !provider.designReady
    $('design-status').textContent = provider.designReady ? '本地模型生成，不会自动提交视频。' : '声音服务的音色设计模型未就绪，请先在声音工作台准备模型。'
    sync()
  }
  for (const id of [...Object.values(fields), 'rec-bgm-gain']) $(id).addEventListener('change', () => { if (id === 'rec-persona') wantedPersona = $(id).value; sync(); saveDraft(); $('settings-saved').textContent = '设置已修改，保存后作为本项目默认值'; if (id === 'publish-platform') canvas(read().platform) })
  $('rec-bgm-gain').addEventListener('input', sync)
  $('apply-platform-style').onclick = () => { const s = styles[read().platform]; $('rec-tone').value = s.tone; $('rec-speed').value = s.speed; $('rec-pause').value = s.pause; sync(); saveDraft() }
  function apply(value = {}) {
    const s = { ...defaults, ...value }
    wantedPersona = s.persona || ''
    for (const [key, id] of Object.entries(fields)) $(id).value = String(s[key] ?? '')
    $('rec-bgm-gain').value = Math.round(s.bgmGain * 100)
    sync(); canvas(s.platform)
  }
  $('preview-persona').onclick = () => {
    const s = read()
    if (!s.persona) return message('请先选择音色，或到声音库录制一个', true)
    audio('rec-voice-sample', `/api/voice/sample?key=${encodeURIComponent(s.persona)}`)
    const el = $('rec-voice-sample')
    el.hidden = false; el.currentTime = 0
    el.play().catch(() => message('样音暂时无法播放，可稍后重试。', true))
  }
  $('upload-bgm').onclick = () => upload('bgm')
  $('upload-narration').onclick = () => upload('narration')
  $('save-project-settings').onclick = async () => {
    try { const result = await api('/api/project-settings', { method: 'POST', body: JSON.stringify({ projectId: getProjectId(), settings: read() }) }); const project = getState().projects.find(p => p.id === getProjectId()); project.recommendationSettings = result.settings; saveDraft(); $('settings-saved').textContent = '已保存到本项目' } catch (error) { message(error.message, true) }
  }
  $('voice-design-form').onsubmit = async event => {
    event.preventDefault(); $('design-submit').disabled = true
    try {
      const job = await api('/api/jobs', { method: 'POST', body: JSON.stringify({ kind: 'voice-design', projectId: getProjectId(), voiceName: $('design-name').value, prompt: $('design-text').value, tone: $('design-tone').value }) })
      pendingDesign = { id: job.id, projectId: getProjectId() }; $('voice-design-dialog').close(); message('新音色已开始生成，完成后会加入音色选择器，并保存一份样音。'); await refresh()
    } catch (error) { message(error.message, true) } finally { $('design-submit').disabled = false }
  }
  $('trial-voice').onclick = async () => {
    const s = read()
    const cloud = s.backend && s.backend !== 'voxflow'
    if (cloud && !s.voiceId) return message('请先在上方「生成通道」里选择音色', true)
    if (!cloud && !s.persona) return message('请先选择或创建音色', true)
    $('trial-voice').disabled = true
    try {
      const prompt = $('repo-captions').value.split('\n').find(line => line.trim()) || '今天分享一个实用的开源项目，一起来看看它能做什么。'
      // 云端试听必须带上通道、模型、音色与账户 —— 否则这条试听走的是本地音色，
      // 听到的声音根本不是最终要用的那个声音，等于白试听。
      const payload = cloud
        ? { projectId: getProjectId(), kind: 'speech', prompt, backend: s.backend, model: s.model, voiceId: s.voiceId, profile: s.profile || 'primary' }
        : { projectId: getProjectId(), kind: 'speech', persona: s.persona, tone: s.tone, prompt }
      const job = await api('/api/jobs', { method: 'POST', body: JSON.stringify(payload) })
      pendingTrial = { id: job.id, projectId: getProjectId() }; message(cloud ? '正在用所选云端音色生成一句试听（按量计费）。' : '正在用当前音色和表达方式生成一句试听。'); await refresh()
    } catch (error) { message(error.message, true); $('trial-voice').disabled = false }
  }
  async function jobs() {
    for (const [kind, pending] of [['design', pendingDesign], ['trial', pendingTrial]]) {
      if (!pending) continue
      const job = getState().jobs.find(j => j.id === pending.id)
      if (!job || !['succeeded', 'failed', 'interrupted'].includes(job.status)) continue
      if (kind === 'design') pendingDesign = null; else { pendingTrial = null; $('trial-voice').disabled = false }
      if (job.status !== 'succeeded') { message(job.error || '声音任务未完成，请查看生成记录', true); continue }
      if (kind === 'design') await connections()
      if (pending.projectId !== getProjectId()) continue
      const asset = getState().assets.find(a => a.id === job.assetId)
      if (kind === 'design' && asset?.persona) { $('rec-persona').value = asset.persona; $('rec-voice-source').value = 'tts'; sync(); saveDraft(); message('新音色已选中，请试听后决定是否生成视频。') }
      if (kind === 'trial' && asset) { audio('rec-voice-sample', asset.url); $('rec-voice-sample').playbackRate = read().speed; message('一句试听已生成，点击声音设置中的播放器试听。') }
    }
  }
  for (const id of ['rec-voice-sample', 'rec-narration-preview', 'rec-bgm-preview']) $(id).addEventListener('error', () => message('这份样音暂时无法播放，可重新生成一句试听。', true))
  return { read, apply, sync, personas, renderAssets, jobs }
}
