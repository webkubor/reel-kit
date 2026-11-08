const $ = id => document.getElementById(id)
const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
const platforms = [{ key: 'channels', label: '视频号' }, { key: 'xiaohongshu', label: '小红书' }, { key: 'landscape', label: '横屏讲解' }]
// 声音是全局资产：所有项目共用这份库；项目表单里只做选择与试听。
export function voiceLibrary({ api, message, connections, refresh }) {
  let recorder, recordStream, recordTimer, voices = []
  function render() {
    $('voice-library-list').innerHTML = voices.length ? voices.map(v => {
      const hints = v.hints || []
      return `<div class="voice-row"><button type="button" class="voice-play" data-play="${escape(v.persona)}" ${v.hasSample ? '' : 'disabled'} aria-label="试听 ${escape(v.name)}">▶</button><div class="voice-info"><strong>${escape(v.name)}</strong><span>${escape(v.note || (v.hasSample ? '已就绪，可在项目中选用' : '暂无样音，生成一句试听后可用'))}</span></div><div class="voice-hints" role="group" aria-label="平台适配标记">${platforms.map(p => `<button type="button" class="hint-chip ${hints.includes(p.key) ? 'on' : ''}" aria-pressed="${hints.includes(p.key)}" data-hint="${p.key}" data-persona="${escape(v.persona)}" title="标记这个音色适合${p.label}">${p.label}</button>`).join('')}</div></div>`
    }).join('') : '<p class="empty-history">还没有音色。录制你自己的声音，或用描述创建一个新音色。</p>'
  }
  async function load() {
    try { voices = (await api('/api/voices')).voices; render() } catch (error) { message(error.message, true) }
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
  $('library-record-voice').onclick = async () => {
    if (recorder && recorder.state === 'recording') { recorder.stop(); return }
    try {
      recordStream = await navigator.mediaDevices.getUserMedia({ audio: true })
      recorder = new MediaRecorder(recordStream)
      const chunks = []
      recorder.ondataavailable = event => { if (event.data.size) chunks.push(event.data) }
      recorder.onstop = async () => {
        clearTimeout(recordTimer); recordStream.getTracks().forEach(track => track.stop())
        $('library-record-voice').textContent = '● 开始录音'; $('library-record-status').textContent = '正在保存录音…'
        try {
          const params = new URLSearchParams({ name: $('library-record-name').value.trim(), instruction: $('library-record-note').value.trim() || '本人真实录音，自然清晰' })
          const response = await fetch(`/api/voice/record?${params}`, { method: 'POST', headers: { 'X-Reel-Action': 'studio', 'Content-Type': recorder.mimeType }, body: new Blob(chunks, { type: recorder.mimeType }) })
          const result = await response.json()
          if (!response.ok) throw new Error(result.error || '录音保存失败')
          $('library-record-status').textContent = '已加入声音库'
          $('library-record-name').value = ''; $('library-record-note').value = ''
          await Promise.allSettled([connections(), load(), refresh()])
          message('录音已加入全局声音库，所有项目都可以选择这个音色。')
        } catch (error) { $('library-record-status').textContent = '保存失败'; message(error.message, true) }
      }
      recorder.start()
      $('library-record-voice').textContent = '■ 结束录音'
      $('library-record-status').textContent = '正在录音，最长 90 秒'
      recordTimer = setTimeout(() => { if (recorder.state === 'recording') recorder.stop() }, 90000)
    } catch (error) {
      recordStream?.getTracks().forEach(track => track.stop())
      message('无法录音，请检查浏览器麦克风权限。' + error.message, true)
    }
  }
  return { load }
}
