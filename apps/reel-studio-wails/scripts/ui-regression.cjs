const { spawn } = require('child_process'); const fs = require('fs'); const path = require('path'); const os = require('os')
const SP=process.argv[2]
const chrome=process.env.REEL_CHROME||resolveChrome()
// 随包分发的 chrome-headless-shell 优先；本机没带就找 playwright 缓存里的那份。
function resolveChrome(){
  const roots=[path.join(os.homedir(),'Library/Caches/ms-playwright'),path.join(os.homedir(),'.cache/ms-playwright')]
  const dirs=[]
  for(const root of roots){ if(!fs.existsSync(root)) continue
    for(const name of fs.readdirSync(root)){ const m=name.match(/^chromium(?:_headless_shell)?-(\d+)$/); if(m) dirs.push({root,name,rev:Number(m[1])}) } }
  dirs.sort((a,b)=>b.rev-a.rev)
  const candidates=[
    path.join(__dirname,'..','resources','darwin-arm64','chrome','chrome-headless-shell'),
    ...dirs.map(d=>path.join(d.root,d.name,'chrome-headless-shell-mac-arm64/chrome-headless-shell')),
    ...dirs.map(d=>path.join(d.root,d.name,'chrome-headless-shell-linux/chrome-headless-shell'))]
  const hit=candidates.find(p=>fs.existsSync(p))
  if(!hit) throw new Error('找不到 chrome-headless-shell（可用 REEL_CHROME 指定）')
  return hit
}
const dir=fs.mkdtempSync(path.join(SP,'cdp-'))
const proc=spawn(chrome,['--headless','--disable-gpu','--no-sandbox','--remote-debugging-port=0',`--user-data-dir=${dir}`,'about:blank'],{stdio:'ignore'})
const wait=ms=>new Promise(r=>setTimeout(r,ms))
let fails=0
// 断言失败要能让脚本退非零：只打印 ✗ 的回归脚本等于没有回归。
const ok=(c,m)=>{console.log((c?'✓':'✗')+' '+m);if(!c)fails++}
;(async()=>{try{
 let port
 for(let i=0;i<100&&!port;i++){try{port=fs.readFileSync(path.join(dir,'DevToolsActivePort'),'utf8').split('\n')[0]}catch{await wait(200)}}
 const t=(await (await fetch(`http://127.0.0.1:${port}/json`)).json()).find(x=>x.type==='page')
 const ws=new WebSocket(t.webSocketDebuggerUrl); await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j})
 let id=0;const pend=new Map();const errs=[]
 ws.onmessage=e=>{const v=JSON.parse(e.data);if(v.id&&pend.has(v.id)){const p=pend.get(v.id);pend.delete(v.id);v.error?p.j(Error(JSON.stringify(v.error))):p.r(v.result)}else if(v.method==='Runtime.exceptionThrown')errs.push((v.params.exceptionDetails.exception?.description||'').slice(0,120))}
 const send=(m,p={})=>new Promise((r,j)=>{const n=++id;pend.set(n,{r,j});ws.send(JSON.stringify({id:n,method:m,params:p}))})
 const ev=async x=>{const r=await send('Runtime.evaluate',{expression:x,awaitPromise:true,returnByValue:true});return r.exceptionDetails?('EXC '+(r.exceptionDetails.exception?.description||'').slice(0,150)):r.result.value}
 await send('Runtime.enable');await send('Page.enable');await send('Emulation.setDeviceMetricsOverride',{width:1440,height:1000,deviceScaleFactor:1,mobile:false})
 await send('Page.navigate',{url:process.env.REEL_REGRESSION_URL||'http://127.0.0.1:34117/'});await wait(7000)

// 预检：清掉上一轮自己造的测试项目，回归才可重复跑。
const stale = await ev(`(async()=>{const ps=await window.go.main.Engine.ListProjects();const mine=ps.filter(p=>['回归短剧','回归短片','素材来源'].includes(p.name));for(const p of mine){await window.go.main.App.DeleteProject(p.id)}return mine.length})()`)
if(stale){console.log('已清理上一轮回归项目:',stale);await send('Page.reload');await wait(5000)}

const guideShown = await ev(`!document.getElementById('setup-view').classList.contains('hidden')`)
ok(guideShown,'引导页显示（无项目时）')
if(!guideShown) console.log('提示：回归需要空工作台——起 dev 服务时用 REEL_STUDIO_DATA_DIR 指一个临时目录')
 await ev(`document.getElementById('setup-kind-series').click()`);await wait(300)
 await ev(`document.getElementById('setup-new-project').click()`);await wait(1000)
 ok(await ev(`document.getElementById('project-kind-series').getAttribute('aria-checked')==='true'`),'引导页选的类型传到新建对话框')
 ok(await ev(`document.getElementById('setup-kind-series').getAttribute('aria-checked')==='true'`),'引导页类型按钮状态同步')
 await ev(`(()=>{const i=document.getElementById('new-name');i.value='回归短剧';document.querySelector('#project-form button[type=submit]').click()})()`);await wait(4000)
 ok(!(await ev(`!document.getElementById('setup-view').classList.contains('hidden')`)),'新建后引导页收起')
 ok((await ev(`[...document.querySelectorAll('#workspace-tabs .tab')].map(b=>b.textContent).join(',')`))==='剧集,角色,场景','短剧项目标签为 剧集/角色/场景')
 ok(await ev(`document.getElementById('episode-head').classList.contains('hidden')`)===false,'短剧项目显示剧集区')

 await ev(`document.getElementById('btn-new-episode').click()`);await wait(800)
 await ev(`(()=>{document.getElementById('ep-title').value='第一集 · 雪夜';document.querySelector('#new-episode-form button[type=submit]').click()})()`);await wait(2500)
 ok((await ev(`[...document.querySelectorAll('#episode-list .project-item')].map(b=>b.innerText.split('\\n')[0]).join(',')`))==='01','剧集建好且编号 01')
 await ev(`document.getElementById('btn-new-shot').click()`);await wait(800)
 await ev(`(()=>{const a=document.getElementById('shot-action');a.value='雨夜长街，缓步前行回望';a.dispatchEvent(new Event('input'));document.getElementById('shot-title').value='长街回望';document.querySelector('#new-shot-form button[type=submit]').click()})()`);await wait(2500)
 ok((await ev(`(document.getElementById('shot-list')||{}).innerText||''`)).includes('长街回望'),'镜头建好')

 // 短视频项目
 await ev(`document.getElementById('btn-new-project-icon').click()`);await wait(900)
 await ev(`document.getElementById('project-kind-clip').click()`);await wait(400)
 ok(await ev(`document.getElementById('project-kind-clip').getAttribute('aria-checked')==='true'`),'新建对话框里能切换项目类型')
 await ev(`(()=>{const i=document.getElementById('new-name');i.value='回归短片';document.querySelector('#project-form button[type=submit]').click()})()`);await wait(4000)
 ok((await ev(`[...document.querySelectorAll('#workspace-tabs .tab')].map(b=>b.textContent).join(',')`))==='素材与制作','短视频项目只有一个标签')
 ok(await ev(`document.getElementById('episode-head').classList.contains('hidden')`),'短视频项目隐藏剧集区')
 ok(await ev(`!!document.getElementById('product-asset-grid')`),'短视频有素材面板')

 // 端到端：短视频出成片，成片进素材库
 await ev(`document.getElementById('btn-new-project-icon').click()`);await wait(900)
 await ev(`document.getElementById('project-kind-clip').click()`);await wait(400)
 await ev(`(()=>{const i=document.getElementById('new-name');i.value='素材来源';[...document.querySelectorAll('#project-form button')].find(x=>x.type==='submit').click()})()`);await wait(4000)
 await ev(`(()=>{const set=(id,v)=>{const e=document.getElementById(id);if(e){e.value=v;e.dispatchEvent(new Event('input'))}};set('f-title','回归素材');set('f-lines','第一句。\\n第二句。')})()`)
 await ev(`document.getElementById('btn-run').click()`)
// 任务「停下来」不等于「做成」：失败也会停。
// 所以结束后从后端读真实任务状态，别拿提示文案当结果。
let job = null
 for (let i = 0; i < 16; i++) {
   await wait(7000)
   const hint = await ev(`document.getElementById('run-hint').textContent.trim()`)
  if (!/正在/.test(hint)) {
    job = await ev(`(async()=>{const js=await window.go.main.Engine.ListJobs();return js[0]||null})()`)
    break
  }
 }
ok(job && job.status==='succeeded','短视频能做出成片'+(job&&job.status!=='succeeded'?`（${job.error||job.status||'没有任务记录'}）`:''))
// 取最后一个同名项目：同名是历次回归留下的，按创建顺序排，取第一个会拿到上一次的旧项目冒充通过。
const videos = await ev(`(async()=>{const ps=await window.go.main.Engine.ListProjects();const p=ps.filter(x=>x.name==='素材来源').pop();if(!p)return 0;const a=await window.go.main.Engine.ListAssets(p.id);return a.filter(x=>(x.mime||'').startsWith('video/')).length})()`)
 ok(videos > 0, '成片进了素材库')

 console.log('页面错误:',JSON.stringify(errs.filter(e=>!e.includes("'nodes'"))))
if(fails){console.error(`断言失败 ${fails} 项`);process.exitCode=1}
else{
  // 回归自己造的三个项目，全过就收走，别在真实工作台里留垃圾。
  const left = await ev(`(async()=>{const ps=await window.go.main.Engine.ListProjects();const mine=ps.filter(p=>['回归短剧','回归短片','素材来源'].includes(p.name));for(const p of mine){await window.go.main.App.DeleteProject(p.id)}return mine.length})()`)
  console.log('已清理回归项目:',left)
}
 ws.close()}catch(e){console.error(e);process.exitCode=1}finally{proc.kill()}})()
