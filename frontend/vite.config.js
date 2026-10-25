import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { fileURLToPath } from 'node:url';
import packageBoundary from './scripts/package-boundary-plugin.mjs';

const buildInfo = {
  version: process.env.VITE_CHOIR_BUILD_VERSION || process.env.CHOIR_BUILD_VERSION || 'dev',
  commit: process.env.VITE_CHOIR_BUILD_SHA || process.env.CHOIR_BUILD_SHA || 'local',
  builtAt: process.env.VITE_CHOIR_BUILD_TIME || process.env.CHOIR_BUILD_TIME || 'unknown',
};

export default defineConfig({
  // The package boundary plugin goes first so its pre hooks run before svelte's.
  // No inline step: the preview page loads its built module from /assets like
  // every other entry. Concatenating minified chunks into one module scope is
  // not sound — sibling chunks reuse top-level identifiers and the browser
  // rejects the re-declaration.
  plugins: [packageBoundary({ root: fileURLToPath(new URL('.', import.meta.url)) }), svelte()],
  define: {
    __CHOIR_BUILD_VERSION__: JSON.stringify(buildInfo.version),
    __CHOIR_BUILD_COMMIT__: JSON.stringify(buildInfo.commit),
    __CHOIR_BUILD_TIME__: JSON.stringify(buildInfo.builtAt),
  },
  build: {
    outDir: 'dist',
    rollupOptions: {
      input: {
        // The shell, plus the sign-in-free Reader preview page at /read.html.
        // It has to live outside public/ so Vite emits it once, at the root.
        app: 'index.html',
        reader: 'read.html',
        // The preview page's shell stylesheet. read.html reaches it through
        // reader.js's import (so it comes out hashed, with its own link);
        // naming it here keeps it a stable, verifiable asset of the build.
        'reader-chrome': 'reader-chrome.css',
      },
    },
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
