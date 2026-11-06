// Loads a frontend source module through Vite's SSR loader, for a spec that
// tests a package module without a browser. A module under src/packages/
// reaches the shell only through src/lib/app-sdk.ts, which pulls in the
// registry (import.meta.glob, the virtual base-packages module) and JSON
// imports that Node's own loader cannot read; Vite resolves them as the
// build does. One server per spec file; close it in afterAll.
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
let server = null;

export async function loadSource(file) {
  server ||= await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  return server.ssrLoadModule(file);
}

export async function closeSources() {
  await server?.close();
  server = null;
}
