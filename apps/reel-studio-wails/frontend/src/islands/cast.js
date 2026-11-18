import { createApp, reactive } from 'vue'
import CastLibrary from '../components/CastLibrary.vue'
import { forgetCharacters } from '../lib/character.js'

// 角色库岛：数据仍由 app.js 拉取（项目绑定、主体列表），这里只负责展示。
const store = reactive({
  subjects: [],
  portraits: [],
  error: '',
  projectId: '',
  onCreate: () => {},
  onShoot: () => {},
  onTemplate: () => {},
})

window.ReelCast = {
  mount(el, handlers) {
    Object.assign(store, handlers || {})
    createApp(CastLibrary, { store }).mount(el)
  },
  setData({ subjects, portraits, error, projectId }) {
    if (projectId !== store.projectId) forgetCharacters()
    store.subjects = subjects || []
    store.portraits = portraits || []
    store.error = error || ''
    store.projectId = projectId || ''
  },
}
