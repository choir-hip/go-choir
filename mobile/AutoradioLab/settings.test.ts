// The account source under bun test, `fetch` and the native key reader stubbed. Every value is made up (fixtures/settings).
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { dig, reopen } from './account-phone.ts';
import { currentWorld as theWorld } from './api.ts';
import { bindNative, resetSession, dropHeld, ORIGIN } from './api.ts';
import { settingsAccount, parseWhoami, plainAccountError } from './settings.ts';
import live from '../fixtures/settings/whoami-live.json';

const KEY = 'test-key-0001-not-real';
let calls: string[] = [];
let handler: () => Response;
afterEach(() => { dropHeld(); });      // `reopen` holds the key in the process (api.ts `held`); resetSession does not drop it, so a later file that expects a signed-out process (content.test.ts C15) would find it
beforeEach(() => {
  calls = []; resetSession(); reopen(dig(KEY), KEY, 'comp-1');
  bindNative({ available: true, later: async (r: { op?: string; account?: string }) => (r.op === 'session' ? { found: true, value: KEY, computerId: 'comp-1' } : r.account === 'api-key' ? { found: true, value: KEY } : r.account === 'computer-id' ? { found: true, value: 'comp-1' } : { found: false }) } as never);
  handler = () => new Response(JSON.stringify(live), { status: 200 });
  globalThis.fetch = (async (u: string) => { calls.push(String(u)); return handler(); }) as never;
});

describe('settingsAccount', () => {
  test('A3 signed out: the line, and nothing sent', async () => {
    const v = await settingsAccount({ signedIn: false, tick: 0 });
    expect(v.message).toBe('Not signed in.'); expect(v.signedIn).toBe(false); expect(v.status).toBe('idle'); expect(calls.length).toBe(0);
  });
  test('the real shape: handle, key label, scopes joined; the key never appears', async () => {
    const v = await settingsAccount({ signedIn: true, tick: 1 });
    expect(calls).toEqual([`${ORIGIN}/auth/whoami`]);
    expect(v).toEqual({ status: 'ok', signedIn: true, handle: 'quiet-heron-41', keyID: 'key-live-0001', keyLabel: 'Autoradio on this phone', scopes: 'owner:acts, admin', computerID: 'comp-1', message: '', baseURL: ORIGIN });
    expect(JSON.stringify(v)).not.toContain(KEY);
  });
  test('A2 a refused key, a missing route, garbage and a dead network each say so', async () => {
    handler = () => new Response('{}', { status: 401 });
    expect((await settingsAccount({ signedIn: true, tick: 0 })).message).toContain('no longer accepted');
    handler = () => new Response('{"error":"nope"}', { status: 404 });
    expect((await settingsAccount({ signedIn: true, tick: 0 })).message).toContain('cannot say which account');
    handler = () => new Response('not json', { status: 200 });
    expect((await settingsAccount({ signedIn: true, tick: 0 })).message).toContain('not what was expected');
    globalThis.fetch = (async () => { throw new Error('offline'); }) as never;
    const v = await settingsAccount({ signedIn: true, tick: 0 });
    expect(v.message).toBe('Could not reach choir.news.'); expect(v.status).toBe('failed'); expect(v.signedIn).toBe(true);
  });
  test('A6 a bound computer id wins over the kept one', async () => {
    handler = () => new Response(JSON.stringify({ ...live, computer_id: 'computer-bound' }), { status: 200 });
    expect((await settingsAccount({ signedIn: true, tick: 0 })).computerID).toBe('computer-bound');
  });
  test('A5 a body with no handle is not a half view', async () => {
    handler = () => new Response('{"key_id":"k"}', { status: 200 });
    const v = await settingsAccount({ signedIn: true, tick: 0 });
    expect(v.status).toBe('failed'); expect(v.handle).toBe('');
    expect(parseWhoami(null)).toBeNull(); expect(parseWhoami({ handle: 'a', scopes: [1, 'x'] })?.scopes).toBe('x');
    expect(plainAccountError(new Error('x'))).toBe('Could not reach choir.news.');
  });
});

import { aboutPage } from './settings.ts';
describe('aboutPage', () => {
  test('the version is the platform fact, dev without one; the server is the one origin', () => {
    expect(aboutPage({ appVersion: '1.4' }, true)).toEqual({ app: '1.4', base: ORIGIN, signedIn: true });
    expect(aboutPage({ appVersion: '' }, false)).toEqual({ app: 'dev', base: ORIGIN, signedIn: false });
  });
});
