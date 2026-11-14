/**
 * Reel Studio 本地控制台。
 *
 * 业务操作通过 Go 绑定（window.go.main.Engine / .App），无本地 HTTP 业务服务。
 * 本地媒体由 Wails AssetServer 提供；显式 AI 请求由 Go 连接用户配置的服务。
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
    jobs: [],
    inspectedJob: null,
    mode: 'form',
    motion: 'gentle',
    transition: 'none',
    planId: '',
    pollTimer: null,
    stageScale: 1,
    characters: [],
    characterFace: '',
    characterBusy: false,
  }

  // 五视图的固定顺序：正面 → 背面。
  // 跟 Go 侧 CharacterViews 一致 —— 顺序固定，界面每次刷新都一样，
  // 不按生成顺序排（生成顺序是随机的，会让人以为出问题了）。
  var CHARACTER_VIEWS = [
    { key: 'front', label: '正面' },
    { key: 'left45', label: '左45°' },
    { key: 'side', label: '正侧面' },
    { key: 'right45', label: '右45°' },
    { key: 'back', label: '背面' },
  ]

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
    selects.motion = window.ReelSelect.create($('select-motion'), {
      items: [{ value: 'none', label: '静态画面' }, { value: 'gentle', label: '轻柔运镜' }],
      value: state.motion,
      onChange: function (value) { state.motion = value },
    })
    selects.transition = window.ReelSelect.create($('select-transition'), {
      items: [{ value: 'none', label: '硬切' }, { value: 'fade', label: '淡入淡出' }],
      value: state.transition,
      onChange: function (value) { state.transition = value },
    })
    state.characterPersona = 'real'
    selects.persona = window.ReelSelect.create($('select-persona'), {
      items: [
        { value: 'real', label: '真人', note: '来自真实人物照片' },
        { value: 'virtual', label: '虚拟人', note: '完全虚构的形象' },
      ],
      value: state.characterPersona,
      onChange: function (value) { state.characterPersona = value },
    })
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
    refreshCharacters()
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
    $('btn-new-character').addEventListener('click', openCharacterDialog)
    $('btn-pick-face').addEventListener('click', pickFace)
    $('btn-cancel-character').addEventListener('click', closeCharacterDialog)
    $('character-form').addEventListener('submit', createCharacter)
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

    $('btn-ai-settings').addEventListener('click', openAISettings)
    $('ai-cancel').addEventListener('click', function () { $('ai-key').value = ''; $('ai-dialog').close() })
    $('ai-dialog').addEventListener('close', function () { $('ai-key').value = '' })
    $('ai-settings-form').addEventListener('submit', saveAISettings)
    $('ai-test').addEventListener('click', testAISettings)
    $('ai-clear-key').addEventListener('click', clearAIKey)
    $('btn-generate-plan').addEventListener('click', generatePlan)
    $('mode-form').addEventListener('click', function () { setMode('form') })
    $('mode-code').addEventListener('click', function () { setMode('code') })
    $('btn-view-image').addEventListener('click', showImage)
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

  function openAISettings() {
    $('ai-settings-status').textContent = '读取设置中…'
    $('ai-dialog').showModal()
    engine().GetAISettings().then(function (settings) {
      $('ai-url').value = settings.baseUrl || ''
      $('ai-model').value = settings.model || ''
      $('ai-key').value = ''
      $('ai-key-status').textContent = settings.hasKey ? '此地址已有钥匙串密钥；留空保持不变。' : '密钥保存在系统钥匙串，不写入项目或日志。'
      $('ai-settings-status').textContent = ''
    }).catch(function (err) { $('ai-settings-status').textContent = '读取失败：' + err })
  }

  function saveAISettings(event) {
    event.preventDefault()
    var settings = { baseUrl: $('ai-url').value.trim(), model: $('ai-model').value.trim(), hasKey: false }
    var key = $('ai-key').value
    $('ai-settings-status').textContent = '保存中…'
    engine().SaveAISettings(settings, key).then(function (saved) {
      $('ai-key').value = ''
      $('ai-key-status').textContent = saved.hasKey ? '此地址已有钥匙串密钥；留空保持不变。' : '此地址没有已保存的密钥。'
      $('ai-settings-status').textContent = '已保存。可检查连接；检查不会提交生成请求。'
    }).catch(function (err) { $('ai-settings-status').textContent = '保存失败：' + err })
  }

  function testAISettings() {
    $('ai-settings-status').textContent = '检查中…'
    engine().GetAISettings().then(function (saved) {
      if (saved.baseUrl !== $('ai-url').value.trim().replace(/\/+$/, '') || saved.model !== $('ai-model').value.trim() || $('ai-key').value) {
        throw new Error('请先保存当前填写的设置，再检查连接。')
      }
      return engine().CheckAIConnection()
    }).then(function (message) { $('ai-settings-status').textContent = message })
      .catch(function (err) { $('ai-settings-status').textContent = '检查失败：' + err })
  }

  function clearAIKey() {
    engine().GetAISettings().then(function (saved) {
      if (saved.baseUrl !== $('ai-url').value.trim().replace(/\/+$/, '')) {
        throw new Error('地址尚未保存，请先保存或重新打开设置，再移除该地址的密钥。')
      }
      return engine().ClearAIKey()
    }).then(function () {
      $('ai-key').value = ''
      $('ai-key-status').textContent = '已移除已保存地址的钥匙串密钥。'
    }).catch(function (err) { $('ai-settings-status').textContent = '移除失败：' + err })
  }

  function planFromForm() {
    return { title: $('f-title').value.trim(), subtitle: $('f-subtitle').value.trim(), ratio: state.ratio, lines: splitLines($('f-lines').value) }
  }

  function validatePlan(plan) {
    if (!plan || typeof plan.title !== 'string' || typeof plan.subtitle !== 'string' || !Array.isArray(plan.lines) || ['9:16', '16:9'].indexOf(plan.ratio) < 0 ||
        Array.from(plan.title).length > 40 || Array.from(plan.subtitle).length > 40 || plan.lines.length < 1 || plan.lines.length > 8 ||
        plan.lines.some(function (line) { return typeof line !== 'string' || !line.trim() || Array.from(line).length > 80 })) {
      throw new Error('分镜需包含 title、subtitle、ratio（9:16 或 16:9）和 1–8 条不超过 80 字的 lines。')
    }
    return plan
  }

  function applyPlan(plan) {
    validatePlan(plan)
    $('f-title').value = plan.title
    $('f-subtitle').value = plan.subtitle
    $('f-lines').value = plan.lines.join('\n')
    state.ratio = plan.ratio
    if (selects.ratio) selects.ratio.setItems(layoutItems(), state.ratio)
    paintLineCount()
    paintLayout()
    paintStage()
  }

  function setMode(mode) {
    if (mode === state.mode) return true
    try {
      if (mode === 'code') $('f-code').value = JSON.stringify(planFromForm(), null, 2)
      else applyPlan(validatePlan(JSON.parse($('f-code').value)))
    } catch (err) { fail(err); return false }
    state.mode = mode
    $('form-editor').classList.toggle('hidden', mode !== 'form')
    $('code-editor').classList.toggle('hidden', mode !== 'code')
    $('mode-form').setAttribute('aria-pressed', mode === 'form' ? 'true' : 'false')
    $('mode-code').setAttribute('aria-pressed', mode === 'code' ? 'true' : 'false')
    return true
  }

  function generatePlan() {
    var prompt = $('ai-prompt').value.trim()
    if (!state.projectId || !prompt) { fail('请先选择项目并填写生成要求。'); return }
    var projectID = state.projectId
    setRunning(true)
    state.inspectedJob = null
    engine().SubmitJob(projectID, { kind: 'storyboard', prompt: prompt }).then(function (job) {
      if (projectID !== state.projectId) return
      state.job = job
      renderSteps()
      startPolling()
    }).catch(function (err) { if (projectID === state.projectId) { setRunning(false); fail(err) } })
  }

  function openProjectDialog() {
    var dialog = $('project-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('new-name').focus()
  }

  // ── 角色素材库 ──────────────────────────────────────────
  //
  // 角色资产在中台，这里只管「看完成度」和「从一张脸建一个角色」。
  // 完成度必须显示缺了什么 —— 只给「未完成」用户不知道下一步做什么。
  function paintCharacters() {
    var list = $('character-list')
    var empty = $('character-empty')
    clear(list)
    if (!state.characters.length) {
      empty.classList.remove('hidden')
      return
    }
    empty.classList.add('hidden')
    state.characters.forEach(function (character) {
      list.appendChild(characterCard(character))
    })
  }

  function characterCard(character) {
    var card = el('div', 'character-card' + (character.complete ? ' is-complete' : ''))

    var head = el('div', 'character-head')
    var left = el('div', 'min-w-0')
    left.appendChild(el('div', 'character-name', character.name || '未命名角色'))
    var persona = character.persona === 'virtual' ? '虚拟人' : character.persona === 'real' ? '真人' : ''
    left.appendChild(el('div', 'text-2xs text-ink-lo', persona))
    head.appendChild(left)
    head.appendChild(el('span', 'badge' + (character.complete ? ' badge-accent' : ''), character.complete ? '已建好' : '未建好'))
    card.appendChild(head)

    // 五视图格子。
    var views = el('div', 'character-views')
    var byView = {}
    ;(character.assets || []).forEach(function (asset) {
      byView[asset.view] = asset
    })
    CHARACTER_VIEWS.forEach(function (view) {
      var asset = byView[view.key]
      var cell = el('div', 'character-view' + (asset ? '' : ' is-missing'))
      if (asset && asset.url) {
        var img = el('img')
        img.src = asset.url
        img.alt = character.name + ' ' + view.label
        img.loading = 'lazy'
        cell.appendChild(img)
      }
      cell.appendChild(el('span', 'character-view-label', asset ? view.label : '缺 ' + view.label))
      views.appendChild(cell)
    })
    card.appendChild(views)

    if (character.missing && character.missing.length) {
      card.appendChild(el('p', 'character-missing', '还差：' + character.missing.join('、')))
    }
    // 风险提示不拦出片，但要说出来 —— 「无法确认五视图同源」这种，
    // 悄悄藏起来等于骗用户说样貌锁住了。
    if (character.warnings && character.warnings.length) {
      card.appendChild(el('p', 'character-missing', '注意：' + character.warnings.join('、')))
    }
    return card
  }

  function refreshCharacters() {
    return engine()
      .ListCharacters('')
      .then(function (characters) {
        state.characters = characters || []
        paintCharacters()
      })
      .catch(function (err) {
        // 中台没登录或不可用不该让整个界面报错 —— 只在角色区说清楚。
        state.characters = []
        paintCharacters()
        var empty = $('character-empty')
        empty.classList.remove('hidden')
        empty.textContent = '读不到角色：' + (err && err.message ? err.message : err)
      })
  }

  function openCharacterDialog() {
    var dialog = $('character-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('ch-name').focus()
  }

  function closeCharacterDialog() {
    var dialog = $('character-dialog')
    if (dialog.close) dialog.close()
    else dialog.removeAttribute('open')
  }

  function characterTraits() {
    return {
      发型: ($('ch-trait-hair').value || '').trim(),
      脸型: ($('ch-trait-face').value || '').trim(),
      肤色: ($('ch-trait-skin').value || '').trim(),
      体型: ($('ch-trait-body').value || '').trim(),
    }
  }

  function pickFace() {
    app()
      .PickFaceImage()
      .then(function (path) {
        if (!path) return
        state.characterFace = path
        $('ch-face-name').textContent = path.split('/').pop()
      })
      .catch(fail)
  }

  function createCharacter(event) {
    event.preventDefault()
    if (state.characterBusy) return
    var name = ($('ch-name').value || '').trim()
    if (!name) { characterHint('角色名不能为空', true); return }
    if (!state.characterFace) { characterHint('先选一张人脸照片', true); return }
    var traits = characterTraits()
    var missing = Object.keys(traits).filter(function (key) { return !traits[key] })
    if (missing.length) {
      characterHint('体貌还缺：' + missing.join('、'), true)
      return
    }

    state.characterBusy = true
    $('btn-create-character').disabled = true
    characterHint('正在中台建角色并生成五视图，这一步会联网计费，请稍候…', false)

    engine()
      .CreateCharacter({
        projectId: state.projectId || '',
        workspace: '',
        name: name,
        persona: state.characterPersona || 'real',
        traits: traits,
        facePath: state.characterFace,
        outfit: ($('ch-outfit').value || '').trim(),
        ratio: '3:4',
      })
      .then(function (result) {
        state.characterBusy = false
        $('btn-create-character').disabled = false
        var character = result && result.character
        if (character && character.complete) {
          characterHint('角色建好了，可以出片了。', false)
          closeCharacterDialog()
        } else {
          var gap = character && character.missing && character.missing.length
            ? character.missing.join('、')
            : '（未知原因）'
          characterHint('角色还差：' + gap, true)
        }
        return refreshCharacters()
      })
      .catch(function (err) {
        state.characterBusy = false
        $('btn-create-character').disabled = false
        characterHint(err && err.message ? err.message : String(err), true)
      })
  }

  function characterHint(text, isError) {
    var hint = $('character-hint')
    hint.textContent = text
    if (isError) hint.classList.add('help-error')
    else hint.classList.remove('help-error')
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
      ? layout.label
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
        ratio: state.ratio,
        selectedAsset: state.selectedAsset,
        prompt: $('ai-prompt').value,
        code: $('f-code').value,
        mode: state.mode,
        motion: state.motion,
        transition: state.transition,
        pause: $('f-pause').value,
        planId: state.planId,
      }
    }
    stopPolling()
    state.projectId = projectID
    var draft = state.drafts[projectID] || {}
    $('f-title').value = draft.title || ''
    $('f-subtitle').value = draft.subtitle || ''
    $('f-lines').value = draft.lines || ''
    $('ai-prompt').value = draft.prompt || ''
    $('f-code').value = draft.code || ''
    $('f-pause').value = draft.pause || '0.4'
    state.motion = draft.motion || 'gentle'
    state.transition = draft.transition || 'none'
    if (selects.transition) selects.transition.setItems([{value: 'none', label: '硬切'}, {value: 'fade', label: '淡入淡出'}], state.transition)
    state.planId = draft.planId || ''
    if (selects.motion) selects.motion.setItems([{value: 'none', label: '静态画面'}, {value: 'gentle', label: '轻柔运镜'}], state.motion)
    state.mode = draft.mode || 'form'
    $('form-editor').classList.toggle('hidden', state.mode !== 'form')
    $('code-editor').classList.toggle('hidden', state.mode !== 'code')
    $('mode-form').setAttribute('aria-pressed', state.mode === 'form' ? 'true' : 'false')
    $('mode-code').setAttribute('aria-pressed', state.mode === 'code' ? 'true' : 'false')
    paintLineCount()
    state.ratio = draft.ratio || '9:16'
    if (selects.ratio) selects.ratio.setItems(layoutItems(), state.ratio)
    paintLayout()
    paintVoiceHint()
    state.selectedAsset = draft.selectedAsset || ''
    state.assets = []
    state.videos = []
    state.videoId = ''
    renderVideos()
    state.job = null
    state.jobs = []
    state.inspectedJob = null
    renderJobs()
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
      state.jobs = (jobs || []).filter(function (item) { return item.projectId === projectID })
      renderJobs()
      var job = state.jobs[0]
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
      $('btn-view-image').classList.add('hidden')
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
      if (asset.width && asset.height) {
        cell.appendChild(el('span', 'asset-cell-badge', asset.width + '×' + asset.height))
      }
      cell.addEventListener('click', function () {
        state.selectedAsset = asset.id === state.selectedAsset ? '' : asset.id
        renderAssets()
        paintStage()
      })
      grid.appendChild(cell)
    })
    var selected = findAsset(state.selectedAsset)
    $('btn-view-image').classList.toggle('hidden', !selected)
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
    $('stage-image').classList.add('hidden')
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
    if (state.mode === 'code') {
      try { applyPlan(validatePlan(JSON.parse($('f-code').value))) } catch (err) { fail(err); return }
    }
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

    var pause = Number($('f-pause').value)
    if (!Number.isFinite(pause) || pause < 0 || pause > 2) { fail('句间停顿须在 0 到 2 秒之间。'); return }
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
      prompt: $('ai-prompt').value.trim(),
      planId: state.planId,
      editingMode: state.mode,
      settings: { pause: pause, motion: state.motion, transition: state.transition },
    }

    setRunning(true)
    state.inspectedJob = null
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
        if (submittedProjectID === state.projectId) { setRunning(false); fail(err) }
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
    $('btn-generate-plan').disabled = running
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
    refreshJobs()
    if (job.status === 'succeeded' && job.assetId && job.input && job.input.kind === 'storyboard') {
      var projectID = state.projectId
      engine().ReadStoryboard(projectID, job.assetId).then(function (plan) {
        if (state.projectId !== projectID) return
        applyPlan(plan)
        state.planId = job.assetId
        if (state.mode !== 'code') setMode('code')
        else $('f-code').value = JSON.stringify(plan, null, 2)
        $('run-hint').textContent = '分镜已生成，请审阅后开始制作。'
        $('run-hint').classList.remove('help-error')
      }).catch(fail)
    } else if (job.status === 'succeeded' && job.assetId) {
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

  function showImage() {
    if (!state.selectedAsset) return
    $('stage-video').pause()
    $('stage-video').classList.add('hidden')
    $('stage-canvas').classList.add('hidden')
    $('btn-export').classList.add('hidden')
    $('stage-image').src = '/media/' + state.selectedAsset
    $('stage-image').classList.remove('hidden')
    $('stage-note').textContent = '图片原图 · ' + (findAsset(state.selectedAsset).name || '')
  }

  function showVideo(assetID) {
    $('stage-image').classList.add('hidden')
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

  function refreshJobs() {
    var projectID = state.projectId
    if (!projectID) return
    engine().ListJobs().then(function (jobs) {
      if (state.projectId !== projectID) return
      state.jobs = (jobs || []).filter(function (job) { return job.projectId === projectID })
      renderJobs()
    }).catch(fail)
  }

  function renderJobs() {
    var box = $('job-list')
    clear(box)
    if (!state.jobs.length) {
      box.appendChild(el('p', 'text-2xs text-ink-lo', '这里会保存生成要求与每次制作过程。'))
      return
    }
    state.jobs.forEach(function (job) {
      var input = job.input || {}
      var title = input.kind === 'storyboard' ? 'AI 分镜' : '视频制作'
      var row = el('div', 'flex items-center gap-2')
      var button = el('button', 'btn btn-ghost btn-sm min-w-0 flex-1 justify-start', title + ' · ' + (job.createdAt || '').replace('T', ' ').slice(0, 16))
      button.type = 'button'
      button.addEventListener('click', function () {
        state.inspectedJob = job
        renderSteps()
        if (input.kind === 'product' && job.status === 'succeeded' && job.assetId) showVideo(job.assetId)
      })
      row.appendChild(button)
      if (input.kind === 'storyboard' && job.status === 'succeeded' && job.assetId) {
        var use = el('button', 'btn btn-secondary btn-sm', '载入分镜')
        use.type = 'button'
        use.addEventListener('click', function () {
          var projectID = state.projectId
          engine().ReadStoryboard(projectID, job.assetId).then(function (plan) {
            if (state.projectId !== projectID) return
            applyPlan(plan)
            state.planId = job.assetId
            $('ai-prompt').value = input.prompt || ''
            if (state.mode !== 'code') setMode('code')
            else $('f-code').value = JSON.stringify(plan, null, 2)
          }).catch(fail)
        })
        row.appendChild(use)
      }
      box.appendChild(row)
    })
  }

  // ── 工序 ──────────────────────────────────────────────
  function renderSteps() {
    var box = $('steps')
    var empty = $('steps-empty')
    clear(box)
    var job = state.inspectedJob || state.job
    var labels = { queued: '排队中', running: '制作中', succeeded: '已完成', failed: '制作失败', canceled: '已取消', interrupted: '已中断' }
    var input = job && job.input ? job.input : {}
    var upstream = job && job.upstream ? job.upstream : {}
    var details = []
    if (input.prompt) details.push('生成要求：' + input.prompt)
    if (Array.isArray(input.lines)) details.push('分镜文案：\n' + input.lines.join('\n'))
    if (upstream.requestedModel) details.push('请求模型：' + upstream.requestedModel)
    if (upstream.actualModel) details.push('实际模型：' + upstream.actualModel)
    if (job && job.message) details.push('状态：' + job.message)
    if (job && job.error) details.push('错误：' + job.error)
    $('job-detail').textContent = details.join('\n')
    $('job-state').textContent = job ? (labels[job.status] || job.status || '') : ''
    if (job && job.message && (job.status === 'queued' || job.status === 'running')) $('run-hint').textContent = job.message
    if (!job || !job.steps || !job.steps.length) {
      empty.classList.remove('hidden')
      return
    }
    empty.classList.add('hidden')

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
      if (step.startedAt && step.endedAt) {
        var seconds = (Date.parse(step.endedAt) - Date.parse(step.startedAt)) / 1000
        if (Number.isFinite(seconds) && seconds >= 0) meta += ' · ' + seconds.toFixed(1) + ' 秒'
      }
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
