const $ = id => document.getElementById(id)
const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
const platforms = [{ key: 'channels', label: '视频号' }, { key: 'xiaohongshu', label: '小红书' }, { key: 'landscape', label: '横屏讲解' }]
// 声音是全局资产：所有项目共用这份库；项目表单里只做选择与试听。
export function voiceLibrary({ api, message, connections, refresh }) {
  let recorder, recordStream, recordTimer, voices = [], pendingAudio, previewUrl, source = 'import', saving = false
  function stopRecording() {
    clearTimeout(recordTimer)
    if (recorder?.state === 'recording') recorder.stop()
    recordStream?.getTracks().forEach(track => track.stop())
  }
  function preview(blob, origin) {
    if (previewUrl) URL.revokeObjectURL(previewUrl)
    pendingAudio = blob; source = origin
    previewUrl = URL.createObjectURL(blob)
    $('library-record-preview').src = previewUrl
    $('library-record-preview').hidden = false
    $('library-save-voice').disabled = false
    $('library-record-status').textContent = '请试听并核对原文，再保存到声音库'
  }
  $('voice-library-dialog').addEventListener('close', () => { stopRecording(); $('library-record-preview').pause(); $('library-sample').pause() })
  window.addEventListener('pagehide', stopRecording)
  function render() {
    $('voice-library-list').innerHTML = voices.length ? voices.map(v => {
      const hints = v.hints || []
      return `<div class="voice-row"><button type="button" class="voice-play" data-play="${escape(v.persona)}" ${v.hasSample ? '' : 'disabled'} aria-label="试听 ${escape(v.name)}">▶</button><div class="voice-info"><strong>${escape(v.name)}</strong><span>${escape(v.note || (v.hasSample ? '已就绪，可在项目中选用' : '暂无样音，生成一句试听后可用'))}</span></div><div class="voice-hints" role="group" aria-label="平台适配标记">${platforms.map(p => `<button type="button" class="hint-chip ${hints.includes(p.key) ? 'on' : ''}" aria-pressed="${hints.includes(p.key)}" data-hint="${p.key}" data-persona="${escape(v.persona)}" title="标记这个音色适合${p.label}">${p.label}</button>`).join('')}</div></div>`
    }).join('') : '<p class="empty-history">还没有音色。录制你自己的声音，或用描述创建一个新音色。</p>'
  }
  async function load() {
    try {
      const result = await api('/api/voices')
      voices = result.voices; render()
      $('library-record-voice').disabled = !result.recordingReady
      $('library-audio-file').disabled = !result.recordingReady
      if (!result.recordingReady) $('library-record-status').textContent = '请升级并重启 VoxFlow（需要 0.10.1 或更新版本）'
    } catch (error) { message(error.message, true) }
  }
  const openLibrary = () => { $('voice-library-dialog').showModal(); load() }
  $('open-voice-library').onclick = openLibrary
  $('sidebar-voice-library').onclick = openLibrary
  $('library-refresh').onclick = load
  $('library-sample').addEventListener('error', () => message('这份样音暂时无法播放，可稍后重试。', true))
  $('voice-library-list').onclick = async event => {
    const play = event.target.closest('[data-play]')
    if (play) {
      const el = $('library-sample')
      el.hidden = false
      const src = `/api/voice/sample?key=${encodeURIComponent(play.dataset.play)}`
      if (el.getAttribute('src') !== src) { el.pause(); el.src = src }
      el.currentTime = 0; el.play().catch(() => {})
      return
    }
    const chip = event.target.closest('[data-hint]')
    if (!chip) return
    const voice = voices.find(v => v.persona === chip.dataset.persona)
    if (!voice) return
    const hints = (voice.hints || []).includes(chip.dataset.hint) ? voice.hints.filter(h => h !== chip.dataset.hint) : [...(voice.hints || []), chip.dataset.hint]
    try {
      await api('/api/voice/hints', { method: 'POST', body: JSON.stringify({ persona: voice.persona, hints, note: voice.note || '' }) })
      voice.hints = hints; render(); message('平台标记已保存，可在声音库查看。')
    } catch (error) { message(error.message, true) }
  }
  $('open-voice-design').onclick = () => $('voice-design-dialog').showModal()
  $('clone-own-voice').onclick = () => { $('voice-design-dialog').close(); $('voice-library-dialog').showModal(); load(); $('library-record-name').focus() }
  $('library-audio-file').onchange = event => {
    const file = event.target.files[0]
    if (!file) return
    if (file.size > 20 * 1024 * 1024) { message('参考音频不能超过 20 MB', true); return }
    const fallback = { wav: 'audio/wav', mp3: 'audio/mpeg', m4a: 'audio/mp4', webm: 'audio/webm', ogg: 'audio/ogg', flac: 'audio/flac', aac: 'audio/aac' }[file.name.split('.').pop().toLowerCase()]
    preview(file.type ? file : new Blob([file], { type: fallback || 'application/octet-stream' }), 'import')
  }
  $('library-save-voice').onclick = async () => {
    if (!pendingAudio || saving) return
    const name = $('library-record-name').value.trim(), referenceText = $('library-reference-text').value.trim()
    if (!name || !referenceText) { message('请填写音色名称与录音原文', true); return }
    saving = true
    for (const id of ['library-save-voice', 'library-record-voice', 'library-audio-file']) $(id).disabled = true
    $('library-record-status').textContent = '正在保存音色与参考原文…'
    try {
      const params = new URLSearchParams({ name, referenceText, source, instruction: $('library-record-note').value.trim() })
      const response = await fetch(`/api/voice/record?${params}`, { method: 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': pendingAudio.type }, body: pendingAudio })
      const result = await response.json()
      if (!response.ok) throw new Error(result.error || '保存失败')
      pendingAudio = null
      $('library-record-status').textContent = `已保存「${result.name}」，可在所有项目中选择`
      await Promise.allSettled([connections(), load(), refresh()])
      message('音色已加入全局声音库，可选择它生成一句旁白试听。')
    } catch (error) { $('library-record-status').textContent = '保存失败，录音仍保留，可修正后重试'; message(error.message, true) }
    finally {
      saving = false
      $('library-save-voice').disabled = !pendingAudio
      $('library-record-voice').disabled = false; $('library-audio-file').disabled = false
    }
  }
  $('library-record-voice').onclick = async () => {
    if (recorder?.state === 'recording') { stopRecording(); return }
    if (!$('library-reference-text').value.trim()) { message('先填写要照读的短句', true); return }
    try {
      $('library-record-voice').disabled = true
      recordStream = await navigator.mediaDevices.getUserMedia({ audio: true })
      if (!$('voice-library-dialog').open) { stopRecording(); return }
      recorder = new MediaRecorder(recordStream)
      const chunks = []
      recorder.ondataavailable = event => { if (event.data.size) chunks.push(event.data) }
      recorder.onstop = () => {
        clearTimeout(recordTimer); recordStream.getTracks().forEach(track => track.stop())
        $('library-record-voice').textContent = '● 重新录音'
        $('library-audio-file').disabled = false
        if (chunks.length) preview(new Blob(chunks, { type: recorder.mimeType }), 'record')
      }
      recorder.start()
      $('library-save-voice').disabled = true; $('library-audio-file').disabled = true
      $('library-record-voice').textContent = '■ 结束录音'
      $('library-record-status').textContent = '请照读原文，最长 30 秒'
      recordTimer = setTimeout(stopRecording, 30000)
    } catch (error) {
      stopRecording()
      message('无法录音，请检查麦克风权限，或导入参考音频。' + error.message, true)
    } finally { $('library-record-voice').disabled = false }
  }
  return { load }
}
