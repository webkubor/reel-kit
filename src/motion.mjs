import { spawn } from 'node:child_process'

/** Install paused animations once. The encoder seeks them by frame number, never wall time.
 * Titles/backgrounds stay still; only the supplied image and caption move inside their surface.
 * Custom templates may expose window.reelSeek({time, duration}) for their own deterministic scene.
 */
export async function prepareMotion(page, duration) {
  await page.addStyleTag({ content: '*,*::before,*::after{animation:none!important;transition:none!important}' })
  await page.evaluate(duration => {
    const tracks = []
    for (const image of document.querySelectorAll('img')) {
      if (!image.getAttribute('src')) continue
      image.parentElement.style.overflow = 'hidden'
      const track = image.animate([{ scale: '1' }, { scale: '1.04' }], { duration: duration * 1000, fill: 'both', easing: 'linear' })
      track.pause()
      tracks.push(track)
    }
    for (const caption of document.querySelectorAll('.caption')) {
      const track = caption.animate([{ translate: '0 12px', opacity: 0 }, { translate: '0 0', opacity: 1 }], { duration: Math.min(350, duration * 1000), fill: 'both', easing: 'cubic-bezier(0.16,1,0.3,1)' })
      track.pause()
      tracks.push(track)
    }
    window.__reelTracks = tracks
  }, duration)
}

export async function seekMotion(page, time, duration) {
  await page.evaluate(async ({ time, duration }) => {
    for (const track of window.__reelTracks || []) track.currentTime = time * 1000
    if (typeof window.reelSeek === 'function') await window.reelSeek({ time, duration })
  }, { time, duration })
}

/** Stream frames with backpressure; bounded stderr and a deadline prevent stalled encoders.
 * Failed output is never returned to compose. Callers own their temporary directory cleanup.
 */
export async function encodeMotion(page, { out, duration, fps, signal, onProgress = () => {} }) {
  const child = spawn('ffmpeg', ['-v', 'error', '-y', '-f', 'image2pipe', '-vcodec', 'png',
    '-framerate', String(fps), '-i', 'pipe:0', '-an', '-c:v', 'libx264', '-preset', 'veryfast',
    '-crf', '18', '-pix_fmt', 'yuv420p', '-vf', 'scale=trunc(iw/2)*2:trunc(ih/2)*2', '-movflags', '+faststart', out],
  { stdio: ['pipe', 'ignore', 'pipe'] })
  let stderr = '', failure = null
  child.stderr.on('data', chunk => { stderr = (stderr + chunk).slice(-8000) })
  child.stdin.on('error', error => { failure ||= error })
  const done = new Promise(resolve => {
    child.once('error', error => { failure = error; resolve() })
    child.once('close', code => { if (code !== 0) failure ||= new Error(`动画编码失败 (${code}): ${stderr}`); resolve() })
  })
  const abort = () => { failure = new Error('动画渲染已取消'); child.kill('SIGKILL') }
  const timer = setTimeout(() => { failure = new Error('动画编码超时，请缩短脚本后重试'); child.kill('SIGKILL') }, 10 * 60 * 1000)
  signal?.addEventListener('abort', abort, { once: true })
  try {
    if (signal?.aborted) abort()
    const count = Math.ceil(duration * fps)
    for (let frame = 0; frame < count; frame++) {
      if (failure) throw failure
      await seekMotion(page, frame / fps, duration)
      const png = await page.screenshot({ type: 'png', timeout: 30000 })
      if (failure) throw failure
      await new Promise((resolve, reject) => child.stdin.write(png, error => error ? reject(error) : resolve()))
      if (frame % fps === 0) onProgress(frame / count)
    }
    child.stdin.end()
    await done
    if (failure) throw failure
    onProgress(1)
  } catch (error) {
    child.kill('SIGKILL')
    await done
    throw failure || error
  } finally {
    clearTimeout(timer)
    signal?.removeEventListener('abort', abort)
  }
}
