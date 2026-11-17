/**
 * muse-tv 本地控制台。
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
    subjects: [],
    episodes: [],
    shots: [],
    episodeId: '',
    view: 'make',
    castQuery: '',
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
        '这个页面需要通过 muse-tv 桌面应用打开。</div>'
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
    refreshSubjects()
    setView('make')
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
    $('btn-new-cast-character').addEventListener('click', openCharacterDialog)
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
    // Go 端 bridge 一直在发 character:progress，前端此前从没订阅过，
    // 所以建角色时的「正在出第 3 张」这类进度提示根本没显示过。
    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn('character:progress', function (text) {
        if (!state.characterBusy) return
        characterHint(String(text || ''), false)
      })
    }

    $('episode-form').addEventListener('submit', submitEpisode)
    $('btn-cancel-episode').addEventListener('click', function () { $('episode-dialog').close() })
    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn('episode:progress', function (text) {
        if (state.episodeBusy) $('episode-hint').textContent = String(text || '')
      })
    }
    $('btn-bind-workspace').addEventListener('click', openBindDialog)
    $('bind-workspace-form').addEventListener('submit', submitBind)
    $('bind-workspace-cancel').addEventListener('click', function () { $('bind-workspace-dialog').close() })
    $('tab-make').addEventListener('click', function () { setView('make') })
    $('tab-product').addEventListener('click', function () { setView('product') })
    $('tab-cast').addEventListener('click', function () { setView('cast') })
    $('tab-scene').addEventListener('click', function () { setView('scene') })
    $('cast-search').addEventListener('input', function (event) {
      state.castQuery = event.target.value || ''
      paintCast()
    })
    $('btn-new-episode').addEventListener('click', openEpisodeDialog2)
    $('new-episode-form').addEventListener('submit', submitNewEpisode)
    $('btn-cancel-new-episode').addEventListener('click', function () { $('new-episode-dialog').close() })
    $('new-shot-form').addEventListener('submit', submitNewShot)
    $('btn-cancel-new-shot').addEventListener('click', function () { $('new-shot-dialog').close() })
    $('new-scene-form').addEventListener('submit', submitNewScene)
    $('btn-cancel-new-scene').addEventListener('click', function () { $('new-scene-dialog').close() })
    $('btn-new-shot').addEventListener('click', openShotDialog)
    $('btn-new-scene').addEventListener('click', openSceneDialog)
    $('shoot-form').addEventListener('submit', submitShoot)
    $('btn-cancel-shoot').addEventListener('click', function () { $('shoot-dialog').close() })
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

    // 定妆组合稿是独立的参考素材，不冒充已经完成的五视图。
    var sheets = (character.references || []).filter(function (asset) {
      return asset.url && /三视图|定妆|turnaround/i.test(asset.name || '')
    })
    if (sheets.length) {
      var gallery = el('div', 'character-sheets')
      sheets.forEach(function (asset) {
        var figure = el('figure', 'character-sheet')
        var image = el('img')
        image.src = asset.url
        image.alt = (character.name || '角色') + '：' + (asset.name || '定妆参考')
        image.loading = 'lazy'
        figure.appendChild(image)
        figure.appendChild(el('figcaption', 'character-sheet-label', asset.name || '定妆参考'))
        gallery.appendChild(figure)
      })
      card.appendChild(gallery)
    }

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

    var actions = el('div', 'flex justify-end gap-2 mt-3')
    var shoot = el('button', 'btn btn-secondary btn-sm', '出镜')
    shoot.type = 'button'
    // 能不能出镜看 references（含三视图组合稿），不看五视图格子：
    // 导入的定妆稿多是一张拼好的三视图，归不进格子，但它就是最稳的参考。
    var refs = (character.references && character.references.length) ? character.references : (character.assets || [])
    shoot.disabled = !refs.length
    if (shoot.disabled) {
      shoot.title = '角色名下还没有定妆稿，先补图才能出镜'
    } else {
      shoot.addEventListener('click', function () { openShootDialog(character) })
    }
    actions.appendChild(shoot)
    card.appendChild(actions)
    return card
  }

  // 出镜：选角色 → 让 ta 出一段镜头。中台出无声素材，配音合成是下一步的事。
  function openShootDialog(character) {
    state.shootCharacter = character
    $('shoot-dialog-title').textContent = '让「' + (character.name || '未命名角色') + '」出镜'
    $('shoot-hint').textContent = ''
    $('shoot-action').value = ''
    var looks = character.looks || []
    state.shootLook = looks[0] || ''
    var lookRoot = $('select-shoot-look')
    clear(lookRoot)
    $('shoot-look-field').classList.toggle('hidden', looks.length < 2)
    if (looks.length >= 2) {
      window.ReelSelect.create(lookRoot, {
        items: looks.map(function (l) { return { value: l, label: l } }),
        value: state.shootLook,
        onChange: function (value) { state.shootLook = value },
      })
    }
    var dialog = $('shoot-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('shoot-action').focus()
  }

  function submitShoot(event) {
    event.preventDefault()
    if (state.characterBusy || !state.shootCharacter) return
    var action = ($('shoot-action').value || '').trim()
    if (!action) {
      $('shoot-hint').textContent = '先写清角色在镜头里做什么'
      $('shoot-hint').classList.add('help-error')
      return
    }
    state.characterBusy = true
    $('btn-submit-shoot').disabled = true
    $('shoot-hint').classList.remove('help-error')
    $('shoot-hint').textContent = '正在提交出镜任务（联网计费）…'
    app()
      .MakeCharacterVideo({
        projectId: state.projectId || '',
        subjectId: state.shootCharacter.id,
        action: action,
        look: state.shootLook || '',
      })
      .then(function (asset) {
        state.characterBusy = false
        $('btn-submit-shoot').disabled = false
        $('shoot-dialog').close()
        $('shoot-hint').textContent = ''
        if (asset && asset.id) {
          $('stage-note').textContent = '出镜素材已入库：' + (asset.name || '出镜.mp4')
          loadAssets()
        }
      })
      .catch(function (err) {
        state.characterBusy = false
        $('btn-submit-shoot').disabled = false
        $('shoot-hint').classList.add('help-error')
        $('shoot-hint').textContent = String(err && err.message ? err.message : err)
      })
  }

  // 三个工作区：制作 / 角色库 / 场景库。
  // 角色和场景有几十个，塞在制作表单里根本没法用 —— 各给一个整屏。
  function setView(view) {
    state.view = view
    var map = {
      make: ['tab-make', 'view-make'],
      product: ['tab-product', 'view-product'],
      cast: ['tab-cast', 'view-cast'],
      scene: ['tab-scene', 'view-scene'],
    }
    Object.keys(map).forEach(function (key) {
      var tab = $(map[key][0])
      var panel = $(map[key][1])
      var on = key === view
      tab.setAttribute('aria-selected', on ? 'true' : 'false')
      tab.classList.toggle('is-active', on)
      panel.classList.toggle('hidden', !on)
      // 制作视图内部是左右两栏，用 contents 让两个 section 并排；
      // 其余视图是单栏，正常流式。
      if (key === 'make') panel.classList.toggle('contents', on)
      else panel.classList.remove('contents')
    })
  }

  function refreshEpisodes() {
    if (!state.projectId) {
      state.episodes = []
      state.shots = []
      state.episodeId = ''
      paintEpisodes()
      return Promise.resolve()
    }
    var projectID = state.projectId
    return engine()
      .ListEpisodes(projectID)
      .then(function (episodes) {
        if (projectID !== state.projectId) return
        state.episodes = episodes || []
        // 选中的剧集还在就留着，不在就选第一集
        var stillThere = state.episodes.some(function (e) { return e.id === state.episodeId })
        if (!stillThere) state.episodeId = state.episodes.length ? state.episodes[0].id : ''
        paintEpisodes()
        return state.episodeId ? refreshShots() : null
      })
      .catch(function (err) {
        state.episodes = []
        paintEpisodes()
      })
  }

  function refreshShots() {
    if (!state.episodeId) {
      state.shots = []
      paintShots()
      return Promise.resolve()
    }
    var episodeID = state.episodeId
    return engine()
      .ListShots(episodeID)
      .then(function (shots) {
        if (episodeID !== state.episodeId) return
        state.shots = shots || []
        paintShots()
      })
      .catch(function () {
        state.shots = []
        paintShots()
      })
  }

  function paintEpisodes() {
    var list = $('episode-list')
    clear(list)
    if (!state.projectId) {
      list.appendChild(el('p', 'px-2.5 py-2 text-2xs leading-relaxed text-ink-lo', '先选一个项目'))
      return
    }
    if (!state.episodes.length) {
      list.appendChild(el('p', 'px-2.5 py-2 text-2xs leading-relaxed text-ink-lo', '还没有剧集。点右上角 ＋ 建第一集。'))
      return
    }
    state.episodes.forEach(function (episode) {
      var button = el('button', 'project-item' + (episode.id === state.episodeId ? ' is-active' : ''))
      button.type = 'button'
      var row = el('span', 'flex min-w-0 flex-1 items-baseline gap-2')
      row.appendChild(el('span', 'text-[10px] font-semibold tabular-nums text-ink-dim', String(episode.number).padStart(2, '0')))
      row.appendChild(el('span', 'truncate', episode.title || '未命名剧集'))
      button.appendChild(el('span', 'project-dot'))
      button.appendChild(row)
      button.addEventListener('click', function () {
        state.episodeId = episode.id
        paintEpisodes()
        refreshShots()
      })
      list.appendChild(button)
    })
  }

  function paintShots() {
    var box = $('shot-list')
    if (!box) return
    clear(box)
    var episode = state.episodes.filter(function (e) { return e.id === state.episodeId })[0]
    $('shot-episode-title').textContent = episode ? episode.title : '本集镜头'
    if (!state.shots.length) {
      box.appendChild(el('p', 'text-2xs leading-relaxed text-ink-lo', '这一集还没有镜头。点「＋ 镜头」加一个。'))
      return
    }
    state.shots.forEach(function (shot) {
      var card = el('div', 'shot-card' + (shot.status === 'done' ? ' is-done' : ''))
      var head = el('div', 'flex items-center justify-between gap-2')
      var title = el('div', 'min-w-0')
      title.appendChild(el('div', 'flex items-baseline gap-2'))
      title.appendChild(el('span', 'text-[10px] font-semibold tabular-nums text-ink-dim', String(shot.number).padStart(2, '0')))
      title.appendChild(el('span', 'truncate text-sm font-medium text-ink', shot.title || '未命名镜头'))
      head.appendChild(title)
      head.appendChild(el('span', 'badge', shotStatusLabel(shot.status)))
      card.appendChild(head)
      if (shot.action) {
        card.appendChild(el('p', 'mt-1.5 text-2xs leading-relaxed text-ink-lo', shot.action))
      }
      if (shot.lines && shot.lines.length) {
        card.appendChild(el('p', 'mt-1 text-2xs leading-relaxed text-ink-dim', '旁白：' + shot.lines.join(' / ')))
      }
      // 镜头与生成任务的关联尚未接通，避免提交后素材无法回填到镜头。
      card.appendChild(el('p', 'mt-2 text-2xs text-ink-lo', '镜头出镜与成片关联尚未接通'))
      box.appendChild(card)
    })
  }

  function openEpisodeDialog2() {
    if (!state.projectId) {
      fail(new Error('先选一个项目'))
      return
    }
    $('new-episode-form').reset()
    var dialog = $('new-episode-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('ep-title').focus()
  }

  function openShotDialog() {
    if (!state.episodeId) {
      fail(new Error('先选一集'))
      return
    }
    $('new-shot-form').reset()
    var dialog = $('new-shot-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('shot-title').focus()
  }

  function openSceneDialog() {
    var project = currentProject()
    if (!project || !project.workspaceId) {
      fail(new Error('这个项目还没绑定中台项目：先绑定，场景才知道建在哪'))
      return
    }
    $('new-scene-form').reset()
    var dialog = $('new-scene-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('scene-name').focus()
  }

  function submitNewEpisode(event) {
    event.preventDefault()
    engine()
      .CreateEpisode({
        ProjectID: state.projectId || '',
        Number: 0,
        Title: $('ep-title').value || '',
        Summary: $('ep-summary').value || '',
      })
      .then(function (episode) {
        $('new-episode-dialog').close()
        state.episodeId = episode.id
        return refreshEpisodes()
      })
      .catch(fail)
  }

  function submitNewShot(event) {
    event.preventDefault()
    var lines = ($('shot-lines').value || '').split('\n').map(function (l) { return l.trim() }).filter(Boolean)
    engine()
      .CreateShot({
        ProjectID: state.projectId || '',
        EpisodeID: state.episodeId || '',
        Title: $('shot-title').value || '',
        Action: $('shot-action').value || '',
        Lines: lines,
      })
      .then(function () {
        $('new-shot-dialog').close()
        return refreshShots()
      })
      .catch(fail)
  }

  function submitNewScene(event) {
    event.preventDefault()
    engine()
      .CreateProjectScene(state.projectId || '', $('scene-name').value || '', $('scene-desc').value || '')
      .then(function () {
        $('new-scene-dialog').close()
        return refreshSubjects()
      })
      .catch(fail)
  }

  function shotStatusLabel(status) {
    if (status === 'done') return '已成片'
    if (status === 'shot') return '已出镜'
    if (status === 'shooting') return '出镜中'
    return '待拍'
  }

  // ── 角色库 / 场景库 ─────────────────────────────────────
  //
  // 30 个角色、十几个场景，网格 + 搜索才找得动；列表一页放不下几个。
  // 角色卡和场景卡都从同一份主体表来，只按 kind 分开。
  function currentProject() {
    return state.projects.filter(function (p) { return p.id === state.projectId })[0] || null
  }

  function refreshSubjects() {
    var project = currentProject()
    state.subjectsError = ''
    paintBinding()
    if (!project || !project.workspaceId) {
      state.subjects = []
      state.subjectsError = project ? '这个项目还没绑定中台项目，角色和场景不知道该从哪读。点顶部「绑定中台项目」。' : '先选一个项目。'
      paintCast()
      paintScenes()
      return Promise.resolve()
    }
    var projectID = project.id
    return app()
      .ListProjectSubjects(projectID)
      .then(function (subjects) {
        if (projectID !== state.projectId) return
        state.subjects = subjects || []
        paintCast()
        paintScenes()
      })
      .catch(function (err) {
        if (projectID !== state.projectId) return
        state.subjects = []
        state.subjectsError = '读不到中台角色：' + (err && err.message ? err.message : err)
        paintCast()
        paintScenes()
      })
  }

  function paintBinding() {
    var project = currentProject()
    var btn = $('btn-bind-workspace')
    if (!btn) return
    btn.textContent = project && project.workspaceName ? '中台：' + project.workspaceName
      : project && project.workspaceId ? '中台：已绑定' : '绑定中台项目'
    btn.disabled = !project
  }

  function openBindDialog() {
    var project = currentProject()
    if (!project) { fail(new Error('先选一个项目')); return }
    var select = $('bind-workspace-select')
    var err = $('bind-workspace-error')
    clear(select)
    err.textContent = '正在读取中台项目…'
    var dialog = $('bind-workspace-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    app().ListWorkspaces().then(function (rows) {
      state.workspaces = rows || []
      err.textContent = rows && rows.length ? '' : '这个账号下没有中台项目。'
      ;(rows || []).forEach(function (w) {
        var opt = el('option', '', w.name || w.id)
        opt.value = w.id
        if (w.id === project.workspaceId) opt.selected = true
        select.appendChild(opt)
      })
    }).catch(function (e) {
      err.textContent = '读不到中台项目：' + (e && e.message ? e.message : e)
    })
  }

  function submitBind(event) {
    event.preventDefault()
    var project = currentProject()
    var workspaceID = $('bind-workspace-select').value
    if (!project || !workspaceID) return
    app().BindProjectWorkspace(project.id, workspaceID).then(function (saved) {
      project.workspaceId = saved.workspaceId
      project.workspaceName = saved.workspaceName || ''
      $('bind-workspace-dialog').close()
      return refreshSubjects()
    }).catch(function (e) {
      $('bind-workspace-error').textContent = String(e && e.message ? e.message : e)
    })
  }

  function people() {
    return state.subjects.filter(function (s) { return s.kind === 'person' })
  }

  function scenes() {
    return state.subjects.filter(function (s) { return s.kind === 'scene' })
  }

  function paintCast() {
    var grid = $('cast-grid')
    var empty = $('cast-empty')
    clear(grid)
    var query = (state.castQuery || '').trim().toLowerCase()
    var list = people().filter(function (s) {
      if (!query) return true
      var hay = (s.name || '') + ' ' + JSON.stringify(s.traits || {})
      return hay.toLowerCase().indexOf(query) >= 0
    })
    $('cast-count').textContent = list.length + ' 个'
    if (state.subjectsError) {
      empty.textContent = state.subjectsError
      return
    }
    if (!list.length) {
      empty.textContent = query ? '没有匹配「' + query + '」的角色。' : '还没有角色。'
      return
    }
    empty.textContent = ''
    list.forEach(function (subject) {
      var card = el('div', 'character-card')
      var head = el('div', 'character-head')
      var left = el('div', 'min-w-0')
      left.appendChild(el('div', 'character-name', subject.name || '未命名角色'))
      left.appendChild(el('div', 'text-2xs text-ink-lo', (subject.traits || {})['武器'] || (subject.traits || {})['身份'] || ''))
      head.appendChild(left)
      head.appendChild(el('span', 'badge', (subject.asset_count || 0) + ' 图'))
      card.appendChild(head)
      var traits = []
      Object.keys(subject.traits || {}).forEach(function (key) {
        if (key === '武器') return
        traits.push(key + '：' + subject.traits[key])
      })
      if (traits.length) {
        card.appendChild(el('p', 'character-missing', traits.join('；')))
      }
      grid.appendChild(card)
    })
  }

  function paintScenes() {
    var grid = $('scene-grid')
    var empty = $('scene-empty')
    clear(grid)
    var list = scenes()
    $('scene-count').textContent = list.length + ' 个'
    if (state.subjectsError) {
      empty.textContent = state.subjectsError
      return
    }
    if (!list.length) {
      empty.textContent = '还没有场景。新建一个场景主体（kind=scene），给参考图和画面描述。'
      return
    }
    empty.textContent = ''
    list.forEach(function (subject) {
      var card = el('div', 'character-card')
      var head = el('div', 'character-head')
      var left = el('div', 'min-w-0')
      left.appendChild(el('div', 'character-name', subject.name || '未命名场景'))
      left.appendChild(el('div', 'text-2xs text-ink-lo', (subject.traits || {})['画面描述'] || ''))
      head.appendChild(left)
      head.appendChild(el('span', 'badge', (subject.asset_count || 0) + ' 图'))
      card.appendChild(head)
      grid.appendChild(card)
    })
  }

  function refreshCharacters() {
    // 走 App 层而不是 Engine 层：App 层是给界面用的薄封装，
    // Engine 层的方法签名会随内部实现变（带 ctx 之类），界面不该依赖它。
    return app()
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

    app()
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
    // 换项目要把剧集/镜头/角色库一起换掉 —— 上一部戏的集数留在侧栏是错的
    state.episodeId = ''
    refreshEpisodes()
    refreshSubjects()
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
      // 中台出的出镜素材还不是成片：旁边给一个「做成片」，加旁白、混声音。
      if (asset.source === 'museav') {
        var row = el('div', 'flex items-center gap-1')
        button.classList.add('min-w-0', 'flex-1')
        row.appendChild(button)
        var make = el('button', 'btn btn-ghost btn-sm shrink-0', '做成片')
        make.type = 'button'
        make.addEventListener('click', function () { openEpisodeDialog(asset) })
        row.appendChild(make)
        list.appendChild(row)
        return
      }
      list.appendChild(button)
    })
  }

  function openEpisodeDialog(asset) {
    state.episodeClip = asset
    $('episode-dialog-title').textContent = '做成片：' + (asset.name || '出镜素材')
    $('episode-hint').textContent = ''
    $('episode-hint').classList.remove('help-error')
    var dialog = $('episode-dialog')
    if (dialog.showModal) dialog.showModal()
    else dialog.setAttribute('open', 'open')
    $('episode-lines').focus()
  }

  function submitEpisode(event) {
    event.preventDefault()
    if (state.episodeBusy || !state.episodeClip) return
    var lines = ($('episode-lines').value || '').split('\n').map(function (l) { return l.trim() }).filter(Boolean)
    state.episodeBusy = true
    $('btn-submit-episode').disabled = true
    $('episode-hint').classList.remove('help-error')
    $('episode-hint').textContent = '正在本机配音与合成…'
    app()
      .MakeEpisode({
        ProjectID: state.projectId || '',
        ClipAssetID: state.episodeClip.id,
        Lines: lines,
        KeepAmbient: $('episode-ambient').checked,
      })
      .then(function (asset) {
        state.episodeBusy = false
        $('btn-submit-episode').disabled = false
        $('episode-dialog').close()
        return loadAssets().then(function () { if (asset && asset.id) showVideo(asset.id) })
      })
      .catch(function (err) {
        state.episodeBusy = false
        $('btn-submit-episode').disabled = false
        $('episode-hint').classList.add('help-error')
        $('episode-hint').textContent = String(err && err.message ? err.message : err)
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
        '<div class="mp-foot"></div>'
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
        '<div class="ml-foot"></div>' +
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
