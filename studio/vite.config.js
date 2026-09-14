import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';
import { ensureStateDir } from './server/lib/state.js';
import { apiMiddleware } from './server/routes.js';

/**
 * 后端就挂在 dev server 上，不单独起进程 —— `pnpm dev` 一条命令即是整个工作台。
 */
function studioServer() {
  return {
    name: 'studio-server',
    configureServer(server) {
      ensureStateDir();
      server.middlewares.use(apiMiddleware());
    },
  };
}

export default defineConfig({
  plugins: [vue(), studioServer()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5273,
    // 这个后端能读写仓库文件，只许本机访问，绝不监听 0.0.0.0
    host: '127.0.0.1',
    strictPort: false,
  },
});
