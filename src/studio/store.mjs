import { mkdirSync, readFileSync, writeFileSync, renameSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { randomUUID } from 'node:crypto'

export const id = () => randomUUID()
export const now = () => new Date().toISOString()
export function createStore(root) {
  mkdirSync(join(root, 'media'), { recursive: true, mode: 0o700 })
  const file = join(root, 'studio.json')
  const state = existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : { projects: [], assets: [], jobs: [], voices: [] }
  if (!state.voices) state.voices = []
  const save = () => {
    writeFileSync(`${file}.tmp`, JSON.stringify(state, null, 2), { mode: 0o600 })
    renameSync(`${file}.tmp`, file)
  }
  // A restart must not silently submit the same paid request a second time.
  for (const job of state.jobs) {
    if (['queued', 'running'].includes(job.status)) {
      job.status = 'interrupted'
      job.error = '工作台已重启。请先核对上游任务，避免重复消耗额度。'
      job.updatedAt = now()
    }
  }
  save()
  return {
    root, state, save,
    project(projectId) {
      const project = state.projects.find(p => p.id === projectId)
      if (!project) throw Object.assign(new Error('项目不存在'), { status: 404 })
      return project
    },
    asset(assetId, projectId) {
      const asset = state.assets.find(a => a.id === assetId && (!projectId || a.projectId === projectId))
      if (!asset) throw Object.assign(new Error('素材不存在或不属于此项目'), { status: 404 })
      return asset
    },
    path(asset) { return join(root, 'media', asset.file) },
    addAsset(projectId, name, mime, bytes, extra = {}) {
      this.project(projectId)
      const assetId = id()
      const extensions = { 'image/png': '.png', 'image/jpeg': '.jpg', 'image/webp': '.webp', 'video/mp4': '.mp4', 'audio/wav': '.wav', 'audio/mpeg': '.mp3', 'audio/mp4': '.m4a', 'audio/ogg': '.ogg' }
      const extension = extensions[mime]
      if (!extension) throw new Error('不支持此素材格式')
      const asset = { id: assetId, projectId, name: String(name).slice(0, 160), mime, file: assetId + extension, createdAt: now(), ...extra }
      writeFileSync(this.path(asset), bytes, { mode: 0o600 })
      state.assets.push(asset)
      save()
      return asset
    },
    publicAsset(asset) {
      const { file: _file, ...publicValue } = asset
      return { ...publicValue, url: `/media/${asset.id}` }
    },
  }
}
