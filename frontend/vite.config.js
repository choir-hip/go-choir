import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

const buildInfo = {
  version: process.env.VITE_CHOIR_BUILD_VERSION || process.env.CHOIR_BUILD_VERSION || 'dev',
  commit: process.env.VITE_CHOIR_BUILD_SHA || process.env.CHOIR_BUILD_SHA || 'local',
  builtAt: process.env.VITE_CHOIR_BUILD_TIME || process.env.CHOIR_BUILD_TIME || 'unknown',
};

export default defineConfig({
  plugins: [svelte()],
  define: {
    __CHOIR_BUILD_VERSION__: JSON.stringify(buildInfo.version),
    __CHOIR_BUILD_COMMIT__: JSON.stringify(buildInfo.commit),
    __CHOIR_BUILD_TIME__: JSON.stringify(buildInfo.builtAt),
  },
  build: {
    outDir: 'dist',
  },
  server: {
    proxy: {
      // Proxy /auth/* to the auth service so the Playwright harness and
      // the dev frontend can call same-origin /auth/* routes without
      // hitting direct service ports.
      '/auth': {
        target: 'http://127.0.0.1:8081',
        changeOrigin: true,
      },
      // Proxy /api/* to the proxy service so the frontend can call
      // same-origin protected routes (shell bootstrap, WebSocket)
      // without hitting direct service ports.
      '/api': {
        target: 'http://127.0.0.1:8082',
        ws: true,
        changeOrigin: true,
      },
    },
  },
});
