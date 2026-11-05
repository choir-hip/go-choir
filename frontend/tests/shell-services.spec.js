// SDK 1.1.0 shell functions, slice S1 (lib/shell-services.ts). No browser or
// server: the functions against a bound fake host.
//
// Failure modes under test (shell-services.ts F1 to F5):
//  F1 no host bound: answers are empty, nothing throws;
//  F2 a later bind wins, and the earlier disposer does not unbind it;
//  F3 a refused clipboard gives false and one error toast;
//  F4 the toast names the label, never the copied text;
//  F6 windowAction ignores an action the shell has no meaning for; submitPrompt ignores empty text;
//  F5 openFile routes media, text and an unknown type.
import { test, expect } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { createServer } from 'vite';

// The module reads the build's import.meta.glob, so vite loads it (SSR, no browser).
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite;
let S;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  S = await vite.ssrLoadModule('/src/lib/shell-services.ts');
});
test.afterAll(async () => { await vite?.close(); });

const setNavigator = (value) => Object.defineProperty(globalThis, 'navigator', { value, configurable: true, writable: true });

function fakeHost() {
  const calls = [];
  const rec = (name) => (...args) => { calls.push([name, ...args]); };
  return {
    calls,
    host: {
      toast: rec('toast'), requireAuth: rec('requireAuth'), sessionExpired: rec('sessionExpired'),
      openApp: rec('openApp'), openTextFile: rec('openTextFile'), openMediaFile: rec('openMediaFile'),
      setAppContext: rec('setAppContext'), resetDesktop: rec('resetDesktop'),
      windowAction: rec('windowAction'), showOverview: rec('showOverview'), showDesktop: rec('showDesktop'), logout: rec('logout'),
      submitPrompt: async (text, options) => { calls.push(['submitPrompt', text, options]); },
      listWindows: () => [{ windowId: 'w1', appId: 'files', title: 'Files', mode: 'normal', focused: true }],
      closeOtherWindows: async (id) => { calls.push(['closeOtherWindows', id]); },
    },
  };
}

test('F1: with no host nothing throws and answers are empty', async () => {
  S.toast('x'); S.requireAuth({}); S.sessionExpired(); S.openApp('files'); S.openFile('a/b.txt'); S.setAppContext({ windowId: 'w' }); S.resetDesktop();
  expect(S.listWindows()).toEqual([]);
  await S.closeOtherWindows('w');
  expect(S.buildInfo()).toHaveProperty('commit');
});

test('F2: the later bind wins; the older disposer leaves it bound', () => {
  const a = fakeHost();
  const b = fakeHost();
  const offA = S.bindShellHost(a.host);
  const offB = S.bindShellHost(b.host);
  offA();
  S.toast('hi');
  expect(b.calls).toEqual([['toast', 'hi', { kind: 'info', durationMs: undefined }]]);
  expect(a.calls).toEqual([]);
  offB();
  S.toast('gone');
  expect(b.calls).toHaveLength(1);
});

test('functions reach the host with the SDK shapes', async () => {
  const { host, calls } = fakeHost();
  const off = S.bindShellHost(host);
  S.toast('  ', {});
  S.toast('bad', { kind: 'error' });
  S.requireAuth({ kind: 'file_upload', appId: 'files', appName: 'Files' });
  S.sessionExpired('renew failed');
  S.openApp('compute-monitor', { appName: 'Compute Monitor' });
  S.setAppContext({ windowId: 'w1', appContext: { a: 1 }, title: 'T' });
  S.setAppContext({ appContext: {} });
  S.resetDesktop();
  await S.closeOtherWindows('w1');
  expect(S.listWindows()).toHaveLength(1);
  expect(calls.map((c) => c[0])).toEqual(['toast', 'requireAuth', 'sessionExpired', 'openApp', 'setAppContext', 'resetDesktop', 'closeOtherWindows']);
  off();
});

test('F3, F4: copyText toasts the label, not the text; a refused clipboard is false with an error toast', async () => {
  const { host, calls } = fakeHost();
  const off = S.bindShellHost(host);
  const written = [];
  setNavigator({ clipboard: { writeText: async (t) => { written.push(t); } } });
  expect(await S.copyText('sk-secret-value', 'API key')).toBe(true);
  expect(written).toEqual(['sk-secret-value']);
  expect(calls.filter((c) => c[0] === 'toast').map((c) => c[1])).toEqual(['Copied API key']);
  setNavigator({ clipboard: { writeText: async () => { throw new Error('denied'); } } });
  calls.length = 0;
  expect(await S.copyText('x')).toBe(false);
  expect(calls).toEqual([['toast', 'Could not copy to the clipboard', { kind: 'error', durationMs: undefined }]]);
  off();
});

test('F5: openFile routes a media file, a text file, and a type nothing opens', () => {
  const { host, calls } = fakeHost();
  const off = S.bindShellHost(host);
  S.openFile('Music/song.mp3');
  S.openFile('Documents/notes.md');
  const media = calls.find((c) => c[0] === 'openMediaFile');
  expect(media[1]).toMatchObject({ fileName: 'song.mp3', filePath: 'Music/song.mp3', pathSegments: ['Music', 'song.mp3'] });
  expect(media[1].appId).toBe('audio');
  expect(calls.find((c) => c[0] === 'openTextFile')[1]).toEqual({ pathSegments: ['Documents', 'notes.md'], fileName: 'notes.md' });
  // an unknown binary type downloads (no document in this runner: nothing opens, nothing throws)
  S.openFile('Misc/thing.bin');
  expect(calls.filter((c) => c[0] === 'openMediaFile' || c[0] === 'openTextFile')).toHaveLength(2);
  off();
});

test('F6: window verbs reach the host; an unknown action and an empty prompt do not', async () => {
  const { host, calls } = fakeHost();
  const off = S.bindShellHost(host);
  S.windowAction('w1', 'minimize');
  S.windowAction('w1', 'explode');
  S.windowAction('', 'close');
  S.showOverview(); S.showDesktop(); S.logout();
  await S.submitPrompt('   ');
  await S.submitPrompt(' hello ', { continueWindowId: 'w2' });
  expect(calls).toEqual([
    ['windowAction', 'w1', 'minimize'], ['showOverview'], ['showDesktop'], ['logout'],
    ['submitPrompt', 'hello', { continueDocId: '', continueWindowId: 'w2' }],
  ]);
  off();
});

test('W1: the SDK exports the window names the Compute Monitor and the prompt bar use', async () => {
  const sdk = await vite.ssrLoadModule('/src/lib/app-sdk.ts');
  for (const name of ['focus', 'restore', 'suspendBackgroundHeavyWindows', 'isHeavyAppId', 'listWindows', 'closeOtherWindows',
    'windowAction', 'showOverview', 'showDesktop', 'logout', 'submitPrompt']) {
    expect(typeof sdk[name], name).toBe('function');
  }
  expect(sdk.isHeavyAppId('no-such-app')).toBe(false);
});
