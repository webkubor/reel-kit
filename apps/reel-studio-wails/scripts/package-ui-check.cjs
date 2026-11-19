// 打开打包版解出的界面，检查关键结构在不在、有没有报错。
const { spawn } = require('child_process')
const fs = require('fs')
const path = require('path')
const os = require('os')

const port = process.argv[2] || '34122'
const outDir = process.argv[3] || '/tmp'
const chrome = process.env.REEL_CHROME ||
  '/Users/webkubor/dev/video/reel-kit/apps/reel-studio-wails/resources/darwin-arm64/chrome/chrome-headless-shell'
const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'pkgui-'))
const proc = spawn(chrome, ['--headless', '--disable-gpu', '--no-sandbox',
  '--remote-debugging-port=0', `--user-data-dir=${dir}`, 'about:blank'], { stdio: 'ignore' })
const wait = (ms) => new Promise((r) => setTimeout(r, ms))

;(async () => {
  let port2 = ''
  for (let i = 0; i < 100 && !port2; i++) {
    try { port2 = fs.readFileSync(path.join(dir, 'DevToolsActivePort'), 'utf8').split('\n')[0] } catch { await wait(200) }
  }
  const t = (await (await fetch(`http://127.0.0.1:${port2}/json`)).json()).find((x) => x.type === 'page')
  const ws = new WebSocket(t.webSocketDebuggerUrl)
  await new Promise((r, j) => { ws.onopen = r; ws.onerror = j })
  let id = 0; const pend = new Map(); const errs = []
  ws.onmessage = (e) => {
    const v = JSON.parse(e.data)
    if (v.id && pend.has(v.id)) { const p = pend.get(v.id); pend.delete(v.id); v.error ? p.j(Error(JSON.stringify(v.error))) : p.r(v.result) }
    else if (v.method === 'Runtime.exceptionThrown') errs.push((v.params.exceptionDetails.exception?.description || '').slice(0, 200))
  }
  const send = (m, p = {}) => new Promise((r, j) => { const n = ++id; pend.set(n, { r, j }); ws.send(JSON.stringify({ id: n, method: m, params: p })) })
  const ev = async (x) => { const r = await send('Runtime.evaluate', { expression: x, awaitPromise: true, returnByValue: true }); return r.exceptionDetails ? null : r.result.value }
  await send('Runtime.enable'); await send('Page.enable')
  // 注入假的 Wails 桥接：不在桌面应用里时，页面会主动换成「请用桌面应用打开」的提示
  const stub = `
window.go = { main: {
  App: { GetAIProviderStatus: () => Promise.resolve({provider:'museav', ready:true, notice:'ok'}),
         SetAIProvider: () => Promise.resolve({}),
         ListAssetTemplates: () => Promise.resolve([]),
         GenerateTemplate: () => Promise.resolve([]),
         GetSetupState: () => Promise.resolve({provider:'museav', checks:[], projectCount:1, ready:true, summary:'ok'}),
         OpenMuseavLogin: () => Promise.resolve(),
         ListCharacters: () => Promise.resolve([]),
         ProductLayoutsInfo: () => Promise.resolve([{ratio:'9:16',label:'竖版 · 1080×1920',width:1080,height:1920}]) },
  Engine: { Status: () => Promise.resolve({ready:true, url:'', note:'ok', error:''}),
            ListProjects: () => Promise.resolve([{id:'p1', name:'沸腾之雪', kind:'series', workspaceName:'沸腾之雪'}]),
            ListEpisodes: () => Promise.resolve([]), ListAssets: () => Promise.resolve([]),
            ListCharacterPortraits: () => Promise.resolve([]), ListProjectSubjects: () => Promise.resolve([]),
            ListScenes: () => Promise.resolve([]), ListLooks: () => Promise.resolve([]),
            VoiceChannels: () => Promise.resolve([]),
            GetAISettings: () => Promise.resolve({baseUrl:'', model:'', hasKey:false}),
            ProductLayoutsInfo: () => Promise.resolve([{ratio:'9:16',label:'竖版 · 1080×1920',width:1080,height:1920}]),
            ListLayouts: () => Promise.resolve([{ratio:'9:16',label:'竖版 · 1080×1920',width:1080,height:1920}]),
            GetCharacter: () => Promise.resolve({}), GetJob: () => Promise.resolve(null),
            ListJobs: () => Promise.resolve([]), ListWorkspaces: () => Promise.resolve([]),
            ListVoices: () => Promise.resolve([]) }
} }
window.runtime = { EventsOn: () => {}, EventsOff: () => {} }
`
  await send('Page.addScriptToEvaluateOnNewDocument', { source: stub })
  await send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false })
  await send('Page.navigate', { url: `http://127.0.0.1:${port}/index.html` })
  await wait(4000)
  const diag = await ev(`({ url: location.href, hasApp: typeof window.go, body: (document.body.innerText||'').slice(0,80) })`)
  console.log('  诊断:', JSON.stringify(diag))
  const checks = [
    ['引导页在', await ev(`!!document.getElementById('setup-view')`)],
    ['标签容器在', await ev(`!!document.getElementById('workspace-tabs')`)],
    ['模板对话框在', await ev(`!!document.getElementById('template-dialog')`)],
    ['YouTube 导入对话框在', await ev(`!!document.getElementById('import-episode-dialog')`)],
    ['侧栏品牌在', await ev(`(document.querySelector('aside')||{}).innerText?.includes('muse·tv')`)],
    ['深色样式生效', await ev(`getComputedStyle(document.body).backgroundColor === 'rgb(11, 12, 14)'`)],
  ]
  let bad = 0
  for (const [name, ok] of checks) { console.log((ok ? '  ✓ ' : '  ✗ ') + name); if (!ok) bad++ }
  const real = errs.filter((e) => !e.includes("'nodes'"))
  if (real.length) { console.log('  页面报错：' + JSON.stringify(real.slice(0, 3))); bad++ }
  fs.writeFileSync(path.join(outDir, 'package-ui.png'), Buffer.from((await send('Page.captureScreenshot', { format: 'png' })).data, 'base64'))
  ws.close(); proc.kill()
  process.exit(bad ? 1 : 0)
})()
