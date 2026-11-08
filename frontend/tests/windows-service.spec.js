// The windows service, slice MA5.3 (lib/services/windows.ts). No browser: the
// service on the real kernel, loaded through vite. The failure modes W1 to W3
// are in the service's header. The window host itself is covered, unchanged,
// by desktop-window-manager.spec.js and desktop-state-persistence.spec.js.
import { test, expect } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite; let W; let K;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  W = await vite.ssrLoadModule('/src/lib/services/windows.ts');
  K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
  await K.shellKernel.services.settled();
});
test.afterAll(async () => { await vite?.close(); });

const impl = (over = {}) => ({
  list: () => [], focus() {}, restore() {}, action() {}, overview() {}, desktop() {},
  suspendBackgroundHeavy() {}, isHeavy: () => false, ...over,
});

test('the shell is the base provider and serves every verb of the contract', () => {
  const info = K.shellKernel.services.describe().services['windows'];
  expect(info.active).toBe('shell');
  const svc = W.windowsService();
  for (const verb of W.WINDOWS_VERBS) expect(typeof svc[verb]).toBe('function');
  expect(W.windowsService().list()).toEqual([]); // no desktop bound: nothing open, no throw
});

test('W1/W3: a provider that misses a verb, or takes the shell\'s id, is refused', () => {
  const scope = K.shellKernel.root;
  expect(() => W.registerWindowsProvider(scope, { id: 'tiler', impl: impl({ action: undefined }) })).toThrow(/impl\.action/);
  expect(() => W.registerWindowsProvider(scope, { id: 'Tiler', impl: impl() })).toThrow(/lowercase/);
  expect(() => W.registerWindowsProvider(scope, { id: 'shell', impl: impl() })).toThrow(/shell's/);
  expect(K.shellKernel.services.describe().services['windows'].active).toBe('shell');
});

test('a conforming provider ranks over the base and W2: a throwing call is dropped', async () => {
  const calls = [];
  const off = W.registerWindowsProvider(K.shellKernel.root, { id: 'tiler', priority: 10, impl: impl({ focus: (id) => calls.push(id), restore() { throw new Error('boom'); } }) });
  await K.shellKernel.services.settled();
  expect(K.shellKernel.services.describe().services['windows'].active).toBe('tiler');
  W.callWindows('focus', 'w1');
  expect(calls).toEqual(['w1']);
  expect(W.callWindows('restore', 'w1')).toBeUndefined();
  off();
  await K.shellKernel.services.settled();
  expect(K.shellKernel.services.describe().services['windows'].active).toBe('shell');
});
