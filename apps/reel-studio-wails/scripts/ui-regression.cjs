const { spawn } = require('child_process'); const fs = require('fs'); const path = require('path')
const SP=process.argv[2]
const chrome=process.env.REEL_CHROME||'/Users/webkubor/dev/video/muse-tv/apps/reel-studio-wails/resources/darwin-arm64/chrome/chrome-headless-shell'
const dir=fs.mkdtempSync(path.join(SP,'cdp-'))
const proc=spawn(chrome,['--headless','--disable-gpu','--no-sandbox','--remote-debugging-port=0',`--user-data-dir=${dir}`,'about:blank'],{stdio:'ignore'})
const wait=ms=>new Promise(r=>setTimeout(r,ms))
const ok=(c,m)=>console.log((c?'✓':'✗')+' '+m)
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

 ok(await ev(`!document.getElementById('setup-view').classList.contains('hidden')`),'引导页显示（无项目时）')
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
 let made = false
 for (let i = 0; i < 16; i++) {
   await wait(7000)
   const hint = await ev(`document.getElementById('run-hint').textContent.trim()`)
   if (!/正在/.test(hint)) { made = true; break }
 }
 ok(made, '短视频能做出成片')
 const videos = await ev(`(async()=>{const ps=await window.go.main.Engine.ListProjects();const p=ps.find(x=>x.name==='素材来源');if(!p)return 0;const a=await window.go.main.Engine.ListAssets(p.id);return a.filter(x=>(x.mime||'').startsWith('video/')).length})()`)
 ok(videos > 0, '成片进了素材库')

 console.log('页面错误:',JSON.stringify(errs.filter(e=>!e.includes("'nodes'"))))
 ws.close()}catch(e){console.error(e);process.exitCode=1}finally{proc.kill()}})()
