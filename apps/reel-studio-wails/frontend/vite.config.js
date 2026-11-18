import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 渐进迁移：Vue 组件先以「岛」的形式挂进现有页面，每个岛打成一个 IIFE。
// 旧的 app.js 仍负责其他页面，等全部迁完再把整页换成 Vue 应用。
// 样式不走 Vite：组件只用 Tailwind 类，由 build.sh 统一编译进 app.css。
export default defineConfig({
  plugins: [vue()],
  define: { 'process.env.NODE_ENV': JSON.stringify('production') },
  build: {
    outDir: 'dist',
    emptyOutDir: false,
    cssCodeSplit: false,
    lib: {
      entry: 'src/islands/cast.js',
      name: 'ReelCast',
      formats: ['iife'],
      fileName: () => 'cast.js',
    },
  },
})
