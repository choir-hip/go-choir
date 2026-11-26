// Station A1 failure list, written before the implementation:
//  A1.1 Android passkey support is the lab-one package only; other Android identities offer no sign-in.
//  A1.2 choirAuth is a shared TypeScript source over Exact secrets, not an Apple package source with an Android substitute.
//  A1.3 an explicitly unavailable native module does not block shared account state or shared-only sign-out; a missing one still refuses.
//  A1.4 the ChoirAuth shape and Apple player/bar/radio change notifications survive removing the native auth answer.
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { accountState, choirAuthState } from './account-state.ts';
import { accountNative, resetSettled } from './account.ts';
import { answer, resetPlatformSeen } from './app.ts';
import { COMPUTER_ID_NAME, KEY_ID_NAME, KEY_NAME } from './app-signin.ts';
import { adoptNative, bindStore, forgetCredentials, resetSeam } from './api.ts';
import { resetDemoLeftovers } from './demo-leftovers.ts';

const LAB_ONE = 'news.choir.autoradio.lab.one';
const KEY = 'choir_sk_test-android-auth-not-real';
const secretStore = (items = new Map<string, string>([[KEY_NAME, KEY], [KEY_ID_NAME, 'ak_android'], [COMPUTER_ID_NAME, 'comp-android']])) => ({
  items,
  get: (n: string) => items.get(n) ?? null,
  set: (n: string, v: string) => { items.set(n, v); },
  forget: (n: string) => { items.delete(n); },
});

function memoryStorage() {
  const files = new Map<string, Uint8Array>();
  const kids = (p: string) => [...files.keys()].filter((k) => k.startsWith(p + '/'));
  const fs = {
    directories: { data: 'app:/data', cache: 'app:/cache', temporary: 'app:/tmp' },
    atomicWriteFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    stat: async (p: string) => { if (files.has(p) || kids(p).length) return { size: 1, modifiedMs: 0 }; throw new Error('ENOENT'); },
    rm: async (p: string) => { files.delete(p); for (const k of kids(p)) files.delete(k); },
    mkdir: async () => undefined,
    readdir: async (p: string) => [...new Set(kids(p).map((k) => k.slice(p.length + 1).split('/')[0]))],
  };
  return { files, storage: { fs, sqlite: {} } as never };
}

beforeEach(() => { resetSeam(); resetSettled(); resetDemoLeftovers(); resetPlatformSeen(); });
afterEach(() => { resetSeam(); resetSettled(); resetDemoLeftovers(); });

describe('Android shared account state', () => {
  test('A1.1 lab-one offers sign-in with no native module; another Android package does not', async () => {
    const store = secretStore(new Map());
    bindStore(store);
    adoptNative({ available: false, later: async () => { throw new Error('no app module'); } }, 'test');
    expect(await accountState('', '', 'native', 'android', LAB_ONE)).toMatchObject({ state: 'signedOut', action: 'signIn' });
    expect(await accountState('', '', 'native', 'android', 'news.choir.autoradio')).toMatchObject({ state: 'noPasskey', action: '' });
  });

  test('A1.2 choirAuth reads the Exact secret store and keeps its four-field shape', async () => {
    const store = secretStore();
    bindStore(store);
    adoptNative({ available: false, later: async () => { throw new Error('no app module'); } }, 'test');
    expect(await choirAuthState()).toEqual({ signedIn: true, baseURL: 'https://choir.news', status: 'idle', message: '' });
    expect(await accountState('', '', 'native', 'android', LAB_ONE)).toMatchObject({ state: 'signedIn', action: '' });
    forgetCredentials();
    expect(await choirAuthState()).toEqual({ signedIn: false, baseURL: 'https://choir.news', status: 'idle', message: '' });
    expect(store.items.size).toBe(0);
  });

  test('A1.2 an unreadable Exact secret is not reported as signed out', async () => {
    const store = { get: () => { throw new Error('locked'); }, set: () => undefined, forget: () => undefined };
    bindStore(store);
    expect(await choirAuthState()).toMatchObject({ signedIn: false, status: 'locked', message: '' });
    expect(await accountState('', '', 'native', 'android', LAB_ONE)).toMatchObject({ state: 'locked', action: '' });
  });

  test('A1.3 app.ts answers choirAuth without calling an unavailable native module', async () => {
    const store = secretStore();
    bindStore(store);
    let calls = 0;
    const native = { available: true, later: async () => { calls++; throw new Error('no app module'); } };
    const out = await (answer as any)('choirAuth', ['signed-in-tick', 'android'], store, memoryStorage().storage, native);
    expect(out).toEqual({ signedIn: true, baseURL: 'https://choir.news', status: 'idle', message: '' });
    expect(calls).toBe(0);
    expect(await (await accountNative(native, 'android')).signOut()).toBe(true);
  });
});

describe('the native auth source is gone', () => {
  test('A1.4 the interface keeps the shape but no choirAuth source or substitute', () => {
    const path = join(dirname(new URL(import.meta.url).pathname), '..', 'exact-package', 'exact.json');
    const iface = JSON.parse(readFileSync(path, 'utf8'));
    expect(iface.shapes.ChoirAuth).toEqual({ signedIn: 'bool', baseURL: 'string', status: 'string', message: 'string' });
    expect(iface.names.some((n: any) => n.source === 'choirAuth')).toBe(false);
  });

  test('A1.4 shared Contract and app.ts own choirAuth; Apple no longer answers it', () => {
    const dir = dirname(new URL(import.meta.url).pathname);
    const contract = readFileSync(join(dir, 'account.contract'), 'utf8');
    const app = readFileSync(join(dir, 'app.ts'), 'utf8');
    const nativeModule = readFileSync(join(dir, '..', 'exact-package', 'apple', 'AutoradioModule.swift'), 'utf8');
    const nativeApp = readFileSync(join(dir, '..', 'AutoradioKit', 'Sources', 'AutoradioKit', 'Choir', 'ChoirApp.swift'), 'utf8');
    expect(contract).not.toContain('use choirAuth,');
    expect(contract).toContain('resource au = choirAuth(`${sivTick}|${signedOutTick}|${lockTick}|${delKey}|${hpg.visibilityState}|${hpg.hasFocus}`, hpg.platform)');   // deletion and unlock ask it again too
    expect(app).toContain("choirAuth: ([_tick, platform]) => { const p = String(platform ?? ''); if (p !== '') platformSeen = p; return choirAuthState(); }");
    expect(nativeModule).not.toContain('case "choirAuth"');
    expect(nativeApp).not.toContain('func authView()');
    expect(nativeModule).toContain('touch(["choirPlayer", "barFacts", "radioView"])');
  });
});
