const esc = v => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
const items = [['opening','开头说明项目价值'],['truth','信息真实且有来源'],['matching','画面与讲解相符'],['ending','结尾有明确行动'],['subtitles','字幕清晰且基本同步'],['audio','声音清晰，音乐不盖人声'],['visuals','画面清晰、关键内容未被裁切']]
export function workflowPanel({ api, getState, getProjectId, refresh, message, applyVersion }) {
  const root = document.getElementById('workflow-panel')
  let selectedJob = '', selectedPublish = '', lastProject = ''
  function render() {
    const state = getState(), projectId = getProjectId()
    if (lastProject !== projectId) { selectedJob = ''; selectedPublish = ''; lastProject = projectId }
    const jobs = state.jobs.filter(j => j.projectId === projectId && j.input.kind === 'recommendation' && j.status === 'succeeded' && j.assetId).toReversed()
    if (!jobs.some(j => j.id === selectedJob)) selectedJob = jobs[0]?.id || ''
    const job = jobs.find(j => j.id === selectedJob)
    const snapshot = job?.sopSnapshot?.input
    const snapshotInfo = snapshot ? [['平台', snapshot.platform], ['音色', snapshot.persona], ['表达方式', snapshot.tone], ['语速', snapshot.speed], ['停顿（秒）', snapshot.pause], ['画面适配', snapshot.imageFit]].filter(([,value]) => value !== undefined && value !== '').map(([label,value]) => `<li>${esc(label)}：${esc(value)}</li>`).join('') : ''
    const pubs = (state.publishes || []).filter(p => p.projectId === projectId && p.jobId === selectedJob).toReversed()
    if (!pubs.some(p => p.id === selectedPublish)) selectedPublish = pubs[0]?.id || ''
    const versions = (state.sopVersions || []).filter(v => v.pipeline === 'github-recommend')
    const project = state.projects.find(p => p.id === projectId)
    const currentVersion = project?.sopVersionId || versions[0]?.id
    const metrics = (state.performance || []).filter(p => p.publishId === selectedPublish)
    const reviews = (state.reviews || []).filter(p => p.publishId === selectedPublish)
    const status = reviews.length ? '已复盘' : pubs.length ? '已发布' : job?.approval ? '可发布' : '待检查'
    const comparison = (state.performance || []).flatMap(m => {
      const publication = (state.publishes || []).find(p => p.id === m.publishId)
      if (!publication) return []
      const associated = state.jobs.find(j => j.id === publication.jobId && j.input.kind === 'recommendation')
      if (!associated) return []
      const fields = Object.entries(m.metrics).map(([key, value]) => `${({ plays:'播放', likes:'点赞', comments:'评论', shares:'分享', visits:'访问' })[key] || key} ${value}`).join(' · ')
      return [`<li>${esc(associated.input.title)} · ${esc(publication.platform)} · ${esc(m.window)} · ${associated.sopSnapshot ? `v${associated.sopSnapshot.sopVersion}` : '历史版本未知'} · ${esc(fields)} · ${esc(new Date(m.recordedAt).toLocaleDateString())}</li>`]
    }).join('')
    root.innerHTML = `<div class="workflow-box"><label>后续制作使用的方法 <select id="workflow-sop">${versions.map(v => `<option value="${esc(v.id)}" ${v.id === currentVersion ? 'selected' : ''}>GitHub 推荐 v${v.version}</option>`).join('')}</select></label><p class="field-help">选择版本用于后续制作；旧视频的记录不会改变。方法变更说明：${esc(versions.find(v => v.id === currentVersion)?.changeNote || '初始方法')}</p><label>查看视频 <select id="workflow-job"><option value="">选择已生成的推荐视频</option>${jobs.map(j => `<option value="${esc(j.id)}" ${j.id === selectedJob ? 'selected' : ''}>${esc(j.input.title)} · ${esc(new Date(j.createdAt).toLocaleString())}</option>`).join('')}</select></label>${job ? `<p class="field-help">制作方法：${job.sopSnapshot ? `v${job.sopSnapshot.sopVersion}` : '历史记录，未绑定版本'} · ${esc(status)}</p>${snapshotInfo ? `<details><summary>查看本次制作配置</summary><ul>${snapshotInfo}</ul></details>` : ''}<button type="button" class="secondary" id="workflow-check">检测成片文件</button><div class="workflow-checks">${job.quality ? job.quality.checks.map(c => `<p>${c.passed ? '✓' : '✕'} ${esc(c.label)}：${esc(c.value)}</p>`).join('') : '<p>尚未检测文件。</p>'}<p class="field-help">字幕安全区、内容真实性和声音质量需要人工检查；平台文件大小限制尚未自动校验。</p></div><fieldset><legend>人工审片</legend>${items.map(([key,label]) => `<label class="workflow-check"><input type="checkbox" data-check="${key}" ${job.approval?.checks?.[key] ? 'checked' : ''}>${label}</label>`).join('')}</fieldset><button type="button" class="secondary" id="workflow-approve">确认检查通过，可发布</button><form id="workflow-publish"><h3>记录发布</h3><div class="settings-grid"><label>平台 <input name="platform" required placeholder="例如 小红书"></label><label>发布日期 <input name="publishedAt" type="date" required></label></div><label>发布链接 <input name="url" type="url" required placeholder="https://"></label><label>运营目标 <input name="goal" required placeholder="例如 获得项目访问"></label><label>选题标签 <input name="tags" placeholder="例如 AI工具、开源"></label><button class="secondary">保存发布记录</button></form>${pubs.length ? `<label>发布记录 <select id="workflow-publication">${pubs.map(p => `<option value="${esc(p.id)}" ${p.id === selectedPublish ? 'selected' : ''}>${esc(p.platform)} · ${esc(p.publishedAt)}</option>`).join('')}</select></label><form id="workflow-performance"><h3>回填表现</h3><div class="settings-grid">${[['plays','播放'],['likes','点赞'],['comments','评论'],['shares','分享'],['visits','访问']].map(([n,l]) => `<label>${l} <input name="${n}" type="number" min="0" step="1" placeholder="未知留空"></label>`).join('')}</div><label>观察窗口 <input name="window" required placeholder="例如 发布后 7 天"></label><button class="secondary">保存表现</button></form><p class="field-help">已记录 ${metrics.length} 次表现。比较时考虑平台、选题和观察窗口，小样本无法证明因果。</p><ul class="workflow-comparison">${comparison}</ul><form id="workflow-review"><h3>复盘并创建新版制作方法</h3><label>发现的问题 <textarea name="findings" required></textarea></label><label>改动假设 <textarea name="hypothesis" required></textarea></label><label>下一版具体改动 <textarea name="changes" required></textarea></label><button class="secondary">保存复盘并创建新版本</button></form><p class="field-help">已记录 ${reviews.length} 次复盘。创建后可在上方选择新版；内容改动需要在制作时执行并验证。</p>` : '<p class="field-help">确认可发布后，保存第一条发布记录，即可回填表现。</p>'}` : '<p class="field-help">生成一条 GitHub 推荐视频后即可开始检查。</p>'}</div>`
    root.querySelector('#workflow-sop')?.addEventListener('change', async e => {
      try { const version = await api('/api/workflow/select-sop', { method: 'POST', body: JSON.stringify({ projectId, sopVersionId: e.target.value }) }); applyVersion(version); await refresh(); render(); message('已选择制作方法；请核对配置并按版本说明调整内容。') } catch (err) { message(err.message, true) }
    })
    root.querySelector('#workflow-job')?.addEventListener('change', e => { selectedJob = e.target.value; selectedPublish = ''; render() })
    root.querySelector('#workflow-publication')?.addEventListener('change', e => { selectedPublish = e.target.value; render() })
    const request = async (action, payload) => { try { const result = await api('/api/workflow/' + action, { method: 'POST', body: JSON.stringify({ projectId, jobId: selectedJob, publishId: selectedPublish, ...payload }) }); await refresh(); render(); message('已保存。'); return result } catch (err) { message(err.message, true) } }
    root.querySelector('#workflow-check')?.addEventListener('click', () => request('quality', {}))
    root.querySelector('#workflow-approve')?.addEventListener('click', () => request('review-check', { checks: Object.fromEntries([...root.querySelectorAll('[data-check]')].map(e => [e.dataset.check, e.checked])) }))
    for (const [formId, action] of [['workflow-publish','publish'],['workflow-performance','performance'],['workflow-review','review']]) root.querySelector('#' + formId)?.addEventListener('submit', e => { e.preventDefault(); request(action, Object.fromEntries(new FormData(e.currentTarget))) })
  }
  return { render }
}
