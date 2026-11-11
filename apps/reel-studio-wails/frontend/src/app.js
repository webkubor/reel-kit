/**
 * Reel Studio 本地控制台。
 *
 * 所有数据都来自 Go 绑定（window.go.main.Engine / .App），不发一个 HTTP 请求 ——
 * 界面调的就是本机内存里的 Go 方法。
 *
 * 布局：左边「写什么」，右边「出来什么样」。
 * 右边那块深色画面台按成片尺寸排版再缩放，比例和成片完全一致。
 */
;(function () {
  'use strict'

  // ── 状态 ──────────────────────────────────────────────
  var state = {
    projects: [],
    projectId: '',
    drafts: Object.create(null),
    assets: [],
    videos: [],
    videoId: '',
    selectedAsset: '',
    layouts: [],
    voiceChannels: [],
    ratio: '9:16',
    voiceBackend: 'sherpa',
    job: null,
    pollTimer: null,
    stageScale: 1,
  }

  // 下拉组件只建一次，之后用 setItems 换内容 ——
  // 重建会在同一个节点上叠事件监听，点一次触发多次。
  var selects = {}

  // ── 小工具 ────────────────────────────────────────────
  function $(id) {
    return document.getElementById(id)
  }
  function el(tag, className, text) {
    var node = document.createElement(tag)
    if (className) node.className = className
    if (text !== undefined && text !== null) node.textContent = text
    return node
  }
  function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild)
  }

  // Wails 的绑定按 Go 的「包名.结构体名」挂到 window.go 上。App 和 Engine
  // 都在 package main，所以是 go.main.App / go.main.Engine。没有绑定时
  // （比如在浏览器里打开这个页面）要给出明确提示，而不是报一堆 undefined。
  function engine() {
    if (!window.go || !window.go.main || !window.go.main.Engine) {
      document.body.innerHTML =
        '<div style="padding:40px;font:14px sans-serif;color:#868d95">' +
        '这个页面需要通过 Reel Studio 桌面应用打开。</div>'
      throw new Error('no engine binding')
    }
    return window.go.main.Engine
  }
  function app() {
    return window.go.main.App
  }

  // ── 启动 ──────────────────────────────────────────────
  function boot() {
    wire()
    engine()
      .Status()
      .then(function (status) {
        paintStatus(status)
      })
      .catch(fail)

    engine()
      .ProductLayoutsInfo()
      .then(function (layouts) {
        state.layouts = layouts || []
        selects.ratio = window.ReelSelect.create($('select-ratio'), {
          items: layoutItems(),
          value: state.ratio,
          onChange: function (value) {
            state.ratio = value
            paintLayout()
            paintStage()
          },
        })
        paintLayout()
        paintStage()
      })
      .catch(fail)

    engine()
      .VoiceChannels()
      .then(function (channels) {
        state.voiceChannels = channels || []
        selects.voice = window.ReelSelect.create($('select-voice'), {
          items: voiceItems(),
          value: state.voiceBackend,
          onChange: function (value) {
            state.voiceBackend = value
            paintVoiceHint()
          },
        })
        paintVoiceHint()
      })
      .catch(fail)

    refreshProjects()
  }

  function layoutItems() {
    return state.layouts.map(function (l) {
      return {
        value: l.ratio,
        label: l.label || l.ratio,
        note: l.width + '×' + l.height,
      }
    })
  }

  function voiceItems() {
    return state.voiceChannels.filter(function (c) { return c.id === 'sherpa' }).map(function (c) {
      return { value: c.id, label: c.label, note: c.ready ? '本机离线' : '未就绪' }
    })
  }

  function wire() {
    $('btn-new-project').addEventListener('click', openProjectDialog)
    $('btn-new-project-icon').addEventListener('click', openProjectDialog)
    $('btn-cancel-project').addEventListener('click', function () {
      $('project-dialog').close()
    })
    $('project-form').addEventListener('submit', function (event) {
      var name = ($('new-name').value || '').trim()
      if (!name) return
      event.preventDefault()
      $('project-dialog').close()
      engine()
        .CreateProject(name)
        .then(function (project) {
          $('new-name').value = ''
          return refreshProjects(project ? project.id : '')
        })
        .catch(fail)
    })

    $('btn-pick-images').addEventListener('click', pickImages)
    $('btn-export').addEventListener('click', function () {
      if (!state.projectId || !state.videoId) return
      app().ExportVideo(state.projectId, state.videoId).then(function (path) {
        if (path) $('stage-note').textContent = '成片已导出'
      }).catch(fail)
    })
    $('btn-run').addEventListener('click', submit)
    $('btn-cancel').addEventListener('click', cancel)

    $('f-title').addEventListener('input', paintStage)
    $('f-subtitle').addEventListener('input', paintStage)
    $('f-lines').addEventListener('input', function () {
      paintLineCount()
      paintStage()
    })

    window.addEventListener('resize', layoutStage)
  }

  function openProjectDialog() {
    var dialog = $('project-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('new-name').focus()
  }

  function fail(err) {
    console.error(err)
    $('run-hint').textContent = '出错了：' + (err && err.message ? err.message : err)
    $('run-hint').classList.add('help-error')
  }

  // ── 状态条 ────────────────────────────────────────────
  function paintStatus(status) {
    var dot = $('engine-dot')
    var text = $('engine-status')
    if (!status || !status.ready) {
      dot.className = 'h-1.5 w-1.5 rounded-full bg-err'
      text.textContent = status && status.note ? status.note : '工作台未就绪'
      return
    }
    dot.className = 'h-1.5 w-1.5 rounded-full bg-ok'
    text.textContent = '本地工作台已就绪'
  }

  function paintVoiceHint() {
    var channel = state.voiceChannels.filter(function (c) {
      return c.id === state.voiceBackend
    })[0]
    if (!channel) return
    var map = {
      'local-offline': '本机离线合成，不联网、不花钱',
      'cloud-paid': '云端合成，会联网并按该渠道计费',
      'free-online': '免费在线合成，会联网',
    }
    $('run-hint').textContent = channel.ready ? (map[channel.origin] || channel.origin || '') : (channel.note || '该语音通道暂不可用')
    $('run-hint').classList.remove('help-error')
  }

  function paintLayout() {
    var layout = currentLayout()
    $('layout-badge').textContent = layout
      ? layout.label + ' · ' + layout.width + '×' + layout.height
      : '—'
    $('preview-meta').textContent = layout ? layout.width + ' × ' + layout.height : '—'
  }

  function currentLayout() {
    for (var i = 0; i < state.layouts.length; i++) {
      if (state.layouts[i].ratio === state.ratio) return state.layouts[i]
    }
    return state.layouts[0]
  }

  // ── 项目 ──────────────────────────────────────────────
  function refreshProjects(selectID) {
    return engine()
      .ListProjects()
      .then(function (projects) {
        state.projects = projects || []
        var targetID = selectID || state.projectId || (state.projects[0] && state.projects[0].id) || ''
        if (targetID !== state.projectId) switchProject(targetID)
        renderProjects()
        if (!selects.project) {
          selects.project = window.ReelSelect.create($('project-select'), {
            items: projectItems(),
            value: state.projectId,
            placeholder: '还没有项目',
            onChange: function (value) {
              switchProject(value)
            },
          })
        } else {
          selects.project.setItems(projectItems(), state.projectId)
        }
        return loadAssets()
      })
      .catch(fail)
  }

  function projectItems() {
    return state.projects.map(function (p) {
      return { value: p.id, label: p.name }
    })
  }

  function renderProjects() {
    var list = $('project-list')
    clear(list)
    if (!state.projects.length) {
      var empty = el('p', 'px-2.5 py-2 text-2xs leading-relaxed text-ink-lo', '还没有项目。点上面的 ＋ 建一个。')
      list.appendChild(empty)
      return
    }
    state.projects.forEach(function (project) {
      var item = el('button', 'project-item')
      if (project.id === state.projectId) {
        item.classList.add('is-active')
        item.setAttribute('aria-current', 'true')
      }
      item.appendChild(el('span', 'project-dot'))
      item.appendChild(el('span', 'min-w-0 flex-1 truncate', project.name))
      item.addEventListener('click', function () {
        if (selects.project) selects.project.pick(project.id)
        else switchProject(project.id)
      })
      list.appendChild(item)
    })
  }

  function switchProject(projectID) {
    if (state.projectId === projectID) return
    if (state.projectId) {
      state.drafts[state.projectId] = {
        title: $('f-title').value,
        subtitle: $('f-subtitle').value,
        lines: $('f-lines').value,
      }
    }
    stopPolling()
    state.projectId = projectID
    var draft = state.drafts[projectID] || {}
    $('f-title').value = draft.title || ''
    $('f-subtitle').value = draft.subtitle || ''
    $('f-lines').value = draft.lines || ''
    paintLineCount()
    state.selectedAsset = ''
    state.assets = []
    state.videos = []
    state.videoId = ''
    renderVideos()
    state.job = null
    setRunning(false)
    renderProjects()
    renderSteps()
    paintStage()
    loadAssets()
    restoreProjectJob(projectID)
  }

  function restoreProjectJob(projectID) {
    if (!projectID) return
    engine().ListJobs().then(function (jobs) {
      if (state.projectId !== projectID || state.job) return
      var job = (jobs || []).filter(function (item) { return item.projectId === projectID })[0]
      if (!job) return
      state.job = job
      renderSteps()
      if (job.status === 'queued' || job.status === 'running') {
        setRunning(true)
        startPolling()
      }
    }).catch(fail)
  }

  // ── 素材 ──────────────────────────────────────────────
  function loadAssets() {
    if (!state.projectId) {
      state.assets = []
      renderAssets()
      return Promise.resolve()
    }
    var projectID = state.projectId
    return engine()
      .ListAssets(projectID)
      .then(function (assets) {
        if (projectID !== state.projectId) return
        state.videos = (assets || []).filter(function (asset) { return asset.mime === 'video/mp4' })
        renderVideos()
        state.assets = (assets || []).filter(function (asset) {
          return (asset.mime || '').indexOf('image/') === 0
        })
        if (state.selectedAsset && !findAsset(state.selectedAsset)) state.selectedAsset = ''
        renderAssets()
      })
      .catch(fail)
  }

  function renderVideos() {
    var list = $('video-list')
    clear(list)
    if (!state.videos.length) {
      list.appendChild(el('p', 'text-2xs text-ink-lo', '还没有成片。'))
      return
    }
    state.videos.slice().reverse().forEach(function (asset) {
      var button = el('button', 'project-item', asset.name || '成片')
      button.type = 'button'
      button.addEventListener('click', function () { showVideo(asset.id) })
      list.appendChild(button)
    })
  }

  function findAsset(id) {
    for (var i = 0; i < state.assets.length; i++) {
      if (state.assets[i].id === id) return state.assets[i]
    }
    return null
  }

  function renderAssets() {
    var grid = $('asset-grid')
    var empty = $('asset-empty')
    clear(grid)
    if (!state.assets.length) {
      empty.classList.remove('hidden')
      $('asset-selected').textContent = '未选择'
      return
    }
    empty.classList.add('hidden')
    state.assets.forEach(function (asset) {
      var cell = el('button', 'asset-cell')
      if (asset.id === state.selectedAsset) cell.classList.add('is-selected')
      cell.setAttribute('aria-pressed', asset.id === state.selectedAsset ? 'true' : 'false')
      cell.setAttribute('aria-label', asset.name || '图片素材')
      var img = el('img')
      img.src = '/media/' + asset.id
      img.alt = asset.name
      cell.appendChild(img)
      cell.appendChild(el('span', 'asset-cell-badge', asset.width + '×' + asset.height))
      cell.addEventListener('click', function () {
        state.selectedAsset = asset.id === state.selectedAsset ? '' : asset.id
        renderAssets()
        paintStage()
      })
      grid.appendChild(cell)
    })
    var selected = findAsset(state.selectedAsset)
    $('asset-selected').textContent = selected ? selected.name : '未选择'
  }

  function pickImages() {
    if (!state.projectId) {
      $('run-hint').textContent = '先建一个项目，再导入素材。'
      $('run-hint').classList.add('help-error')
      return
    }
    app()
      .PickImages(state.projectId)
      .then(function () {
        return loadAssets()
      })
      .catch(fail)
  }

  // ── 画面台 ────────────────────────────────────────────
  function paintStage() {
    var video = $('stage-video')
    video.pause()
    video.classList.add('hidden')
    $('btn-export').classList.add('hidden')
    $('stage-note').textContent = '画面示意 · 最终以成片为准'
    var canvas = $('stage-canvas')
    canvas.classList.remove('hidden')
    var layout = currentLayout()
    var title = ($('f-title').value || '').trim()
    var subtitle = ($('f-subtitle').value || '').trim()
    var lines = splitLines($('f-lines').value)
    var caption = lines.length ? lines[0] : ''
    var asset = findAsset(state.selectedAsset)
    var imageURL = asset ? '/media/' + asset.id : ''

    if (!layout || layout.ratio === '9:16') {
      canvas.className = 'stage-canvas mp'
      canvas.innerHTML =
        '<img class="mp-scene" src="' + imageURL + '" alt="" />' +
        '<div class="mp-shade"></div>' +
        '<div class="mp-bar">' +
        '<span class="mp-tag">' + escapeHTML(subtitle) + '</span>' +
        '<h3 class="mp-title">' + escapeHTML(title) + '</h3>' +
        '</div>' +
        '<div class="mp-caption"><p>' + escapeHTML(caption) + '</p></div>' +
        '<div class="mp-foot">Reel Studio</div>'
    } else {
      canvas.className = 'stage-canvas ml'
      canvas.innerHTML =
        '<div class="ml-arc ml-arc-tl"></div>' +
        '<div class="ml-arc ml-arc-br"></div>' +
        '<div class="ml-pic"><img src="' + imageURL + '" alt="" /></div>' +
        '<div class="ml-panel">' +
        '<div class="ml-title">' + escapeHTML(title) + '</div>' +
        '<div class="ml-subtitle">' + escapeHTML(subtitle) + '</div>' +
        '<div class="ml-caption">' + escapeHTML(caption) + '</div>' +
        '<div class="ml-foot">Reel Studio</div>' +
        '</div>'
    }
    layoutStage()
  }

  // 画面台按成片尺寸排版，再整体缩放到面板宽度 —— 比例与成片一致。
  function layoutStage() {
    var canvas = $('stage-canvas')
    var wrap = $('stage-wrap')
    var portrait = canvas.classList.contains('mp')
    var cw = portrait ? 1080 : 1920
    var ch = portrait ? 1920 : 1080

    var available = wrap.parentElement.clientWidth
    var maxHeight = 460
    var scale = Math.min(available / cw, maxHeight / ch)
    if (!isFinite(scale) || scale <= 0) scale = 0.2

    state.stageScale = scale
    canvas.style.transform = 'scale(' + scale + ')'
    wrap.style.width = Math.round(cw * scale) + 'px'
    wrap.style.height = Math.round(ch * scale) + 'px'
  }

  function splitLines(text) {
    var out = []
    String(text || '')
      .split('\n')
      .forEach(function (line) {
        var t = line.trim()
        if (t) out.push(t)
      })
    return out
  }

  function escapeHTML(text) {
    return String(text === undefined || text === null ? '' : text)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
  }

  function paintLineCount() {
    var lines = splitLines($('f-lines').value)
    $('lines-count').textContent = lines.length + ' / 8 行'
  }

  // ── 跑任务 ────────────────────────────────────────────
  function submit() {
    var lines = splitLines($('f-lines').value)
    if (!state.projectId) {
      $('run-hint').textContent = '先建一个项目。'
      $('run-hint').classList.add('help-error')
      return
    }
    if (!lines.length) {
      $('run-hint').textContent = '先写几句介绍词，一行一句。'
      $('run-hint').classList.add('help-error')
      $('f-lines').focus()
      return
    }
    var channel = state.voiceChannels.filter(function (item) { return item.id === state.voiceBackend })[0]
    if (!channel || !channel.ready || state.voiceBackend !== 'sherpa') {
      $('run-hint').textContent = channel && channel.note ? channel.note : '当前语音通道不可用。'
      $('run-hint').classList.add('help-error')
      return
    }
    if (lines.length > 8 || lines.some(function (line) { return Array.from(line).length > 80 })) {
      $('run-hint').textContent = '最多 8 句，每句不超过 80 字。'
      $('run-hint').classList.add('help-error')
      $('f-lines').focus()
      return
    }
    $('run-hint').classList.remove('help-error')

    var layout = currentLayout()
    var input = {
      kind: 'product',
      title: ($('f-title').value || '').trim(),
      subtitle: ($('f-subtitle').value || '').trim(),
      ratio: state.ratio,
      lines: lines,
      voiceBackend: state.voiceBackend,
      template: layout ? layout.template : '',
      width: layout ? layout.width : 1080,
      height: layout ? layout.height : 1920,
      assetIds: state.selectedAsset ? [state.selectedAsset] : [],
    }

    setRunning(true)
    var submittedProjectID = state.projectId
    engine()
      .SubmitJob(submittedProjectID, input)
      .then(function (job) {
        if (submittedProjectID !== state.projectId) return
        state.job = job
        renderSteps()
        startPolling()
      })
      .catch(function (err) {
        setRunning(false)
        fail(err)
      })
  }

  function cancel() {
    if (!state.job) return
    engine()
      .CancelJob(state.job.id)
      .catch(fail)
  }

  function setRunning(running) {
    $('btn-run').disabled = running
    $('btn-run').textContent = running ? '制作中…' : '开始制作'
    $('btn-cancel').classList.toggle('hidden', !running)
  }

  function startPolling() {
    stopPolling()
    var projectID = state.projectId
    var jobID = state.job.id
    state.pollTimer = window.setInterval(function () {
      engine()
        .GetJob(jobID)
        .then(function (job) {
          if (state.projectId !== projectID || !state.job || state.job.id !== jobID || !job) return
          state.job = job
          renderSteps()
          if (job.status === 'succeeded' || job.status === 'failed' || job.status === 'canceled') {
            stopPolling()
            setRunning(false)
            onJobFinished(job)
          }
        })
        .catch(function (err) {
          if (state.projectId !== projectID || !state.job || state.job.id !== jobID) return
          stopPolling()
          setRunning(false)
          fail(err)
        })
    }, 700)
  }

  function stopPolling() {
    if (state.pollTimer) {
      window.clearInterval(state.pollTimer)
      state.pollTimer = null
    }
  }

  function onJobFinished(job) {
    if (job.status === 'succeeded' && job.assetId) {
      showVideo(job.assetId)
      loadAssets()
      $('run-hint').textContent = '成片好了。'
      $('run-hint').classList.remove('help-error')
    } else if (job.status === 'canceled') {
      $('run-hint').textContent = '已取消，没有产生成片。'
      $('run-hint').classList.add('help-error')
    } else {
      $('run-hint').textContent = '没做成：' + (job.error || '未知原因')
      $('run-hint').classList.add('help-error')
    }
  }

  function showVideo(assetID) {
    var video = $('stage-video')
    var canvas = $('stage-canvas')
    state.videoId = assetID
    video.src = '/media/' + assetID
    $('btn-export').classList.remove('hidden')
    video.classList.remove('hidden')
    canvas.classList.add('hidden')
    $('stage-note').textContent = '成片'
    layoutStage()
  }

  // ── 工序 ──────────────────────────────────────────────
  function renderSteps() {
    var box = $('steps')
    var empty = $('steps-empty')
    clear(box)
    var job = state.job
    if (!job || !job.steps || !job.steps.length) {
      empty.classList.remove('hidden')
      $('job-state').textContent = ''
      return
    }
    empty.classList.add('hidden')
    $('job-state').textContent = job.status || ''

    job.steps.forEach(function (step) {
      var row = el('div', 'step-row')
      var mark = el('span', 'step-mark')
      var status = step.status || 'idle'
      if (status === 'done') {
        mark.classList.add('is-done')
        mark.textContent = '✓'
      } else if (status === 'running') {
        mark.classList.add('is-running')
        mark.textContent = '·'
        row.classList.add('is-running')
      } else if (status === 'failed') {
        mark.classList.add('is-failed')
        mark.textContent = '✕'
        row.classList.add('is-failed')
      } else if (status === 'canceled') {
        mark.classList.add('is-idle')
        mark.textContent = '–'
      } else {
        mark.classList.add('is-idle')
        mark.textContent = '·'
      }
      row.appendChild(mark)
      row.appendChild(el('span', 'step-label', step.label))
      var meta = step.engine || step.origin || ''
      if (step.detail) meta = meta ? meta + ' · ' + step.detail : step.detail
      if (step.error) meta = step.error
      row.appendChild(el('span', 'step-meta', meta))
      box.appendChild(row)
    })
  }

  document.addEventListener('DOMContentLoaded', function () {
    try {
      boot()
    } catch (err) {
      console.error(err)
    }
  })
})()
