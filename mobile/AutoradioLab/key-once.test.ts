// "Signed in" has one reader, and the key is read once per answer (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md). Failure list, written first:
//  K1 a read made after an `await`, in a turn where no store is bound (Exact's door op 2 answers null there), still sees the key the answer read at its start: keyState is present, the session is found.
//  K2 the launch settle with a stored, stamped key never takes such a read as "absent": no sign-out is finished, no native signOut is sent, the stamp stays.
//  K3 the key is read from the store once per answer, at its start, whatever the answer does after (three names, one read each), never again after an await.
//  K4 a store that throws at the start of an answer (locked before the first unlock) is "unreadable", never "absent"; the next answer reads again.
//  K5 what shared code writes or forgets is what the reads after it see, in the same answer: a kept key is present, a forgotten one absent.
//  K6 the account answer carries `signedIn` (the key is there and readable), so the contracts read "signed in" from shared code only.
//  K7 the host fact is Exact's `native.available`: no native call, no timer; a linked module is "native", none (or not available) is "none".
//  K9 a sign-out whose native half answers done after the page's deadline, with a store bound: the late answer forgets the key and wipes; a sign-in kept in between (adoptKey) is left alone.
//  K8 no contract reads a native "signed in": choirAuth, choirPlayer and choirRefresh are gone from the contracts and the interface, and the account answer's key does not carry a native answer.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { answer } from './app.ts';
import { bindStore, keyState, forgetCredentials, resetSeam, authedJSON } from './api.ts';
import { accountSignOut, adoptKey, resetSettled, settleAccount, STAMP_FILE } from './account.ts';
import { accountState } from './account-state.ts';
import { KEY_NAME, KEY_ID_NAME, COMPUTER_ID_NAME } from './app-signin.ts';
import { hostCaps } from './host.ts';
import { resetDemoLeftovers } from './demo-leftovers.ts';

/** Exact's store as an answer sees it: bound in the answer's own turn; `bound = false` models a turn with no store (door op 2: null, not an error). `locked` throws, as a Keychain before the first unlock does. */
function exactStore(seed: Record<string, string>) {
  const values = new Map(Object.entries(seed));
  const s = {
    bound: true, locked: false, gets: 0,
    get(n: string): string | null { s.gets += 1; if (s.locked) throw new Error('InvalidAccessError'); if (!s.bound) return null; return values.get(n) ?? null; },
    set(n: string, v: string): void { if (!s.bound) throw new Error('no store'); values.set(n, v); },
    forget(n: string): void { if (!s.bound) throw new Error('no store'); values.delete(n); },
    values,
  };
  return s;
}
const KEY = { [KEY_NAME]: 'test-key-A-not-real', [KEY_ID_NAME]: 'kid-A', [COMPUTER_ID_NAME]: 'comp-A' };

function memFs(seed: Record<string, string> = {}) {
  const files = new Map<string, Uint8Array>(Object.entries(seed).map(([k, v]) => [k, new TextEncoder().encode(v)]));
  const kids = (p: string) => [...files.keys()].filter((k) => k.startsWith(p + '/'));
  const fs = {
    directories: { data: 'app:/data', cache: 'app:/cache', temporary: 'app:/tmp' },
    atomicWriteFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    stat: async (p: string) => { if (files.has(p) || kids(p).length) return { size: 1, modifiedMs: 0 }; throw new Error('ENOENT'); },
    rm: async (p: string) => { files.delete(p); for (const k of kids(p)) files.delete(k); },
    mkdir: async () => undefined,
    readdir: async (p: string) => { const k = kids(p); if (!k.length) throw new Error('ENOENT'); return [...new Set(k.map((x) => x.slice(p.length + 1).split('/')[0]))]; },
  };
  return { files, storage: { fs, sqlite: {} } as never };
}
const digest = async (s: string): Promise<string> => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s))), (b) => b.toString(16).padStart(2, '0')).join('');

beforeEach(() => { resetSeam(); resetSettled(); resetDemoLeftovers(); });

describe('one key read per answer', () => {
  test('K1 a read after an await in a turn with no store still sees the key the answer read at its start', async () => {
    const st = exactStore(KEY);
    bindStore(st as never);
    st.bound = false;                                                              // the continuation runs with no store bound
    expect((await keyState()).state).toBe('present');
    const real = globalThis.fetch; let auth = '';
    globalThis.fetch = (async (_u: string, o: RequestInit) => { auth = String((o.headers as Record<string, string>).authorization ?? (o.headers as Record<string, string>).Authorization ?? ''); return new Response('{}', { status: 200 }); }) as never;
    try { await authedJSON('/api/whoami'); } finally { globalThis.fetch = real; }
    expect(auth).toBe('Bearer test-key-A-not-real');
  });

  test('K2 the launch settle never takes a store-less read as "absent": no sign-out is finished', async () => {
    const st = exactStore(KEY);
    const m = memFs({ [STAMP_FILE]: await digest(KEY[KEY_NAME]), 'app:/data/takes.json': 'x' });
    const ops: string[] = [];
    const native = { available: true, later: async (r: { op: string }) => { ops.push(r.op); return r.op === 'keystate' ? { state: 'absent' } : { done: true }; } };
    bindStore(st as never);
    st.bound = false;
    const r = await settleAccount(m.storage, st as never, { signOut: async () => { ops.push('signOut'); return true; }, wipe: async () => { ops.push('wipe'); return true; } });
    expect(r.action).toBe('nothing');
    expect(ops).toEqual([]);
    expect(m.files.has(STAMP_FILE)).toBe(true);
    expect(m.files.has('app:/data/takes.json')).toBe(true);
    void native;
  });

  test('K3 the store is read at the start of each answer, three names once each, and not again after an await', async () => {
    const st = exactStore(KEY);
    const m = memFs({ [STAMP_FILE]: await digest(KEY[KEY_NAME]) });
    await answer('answerDismissShared' as never, ['x'] as never, st as never, m.storage, null as never);      // the launch settle runs inside the first answer
    st.gets = 0;
    await answer('answerDismissShared' as never, ['y'] as never, st as never, m.storage, null as never);
    expect(st.gets).toBe(3);
    st.gets = 0; st.bound = false;
    await keyState(); await keyState();
    expect(st.gets).toBe(0);
  });

  test('K4 a store that throws at the start of an answer is unreadable, never absent; the next answer reads again', async () => {
    const st = exactStore(KEY);
    st.locked = true;
    bindStore(st as never);
    expect((await keyState()).state).toBe('unreadable');
    st.locked = false;
    bindStore(st as never);
    expect((await keyState()).state).toBe('present');
  });

  test('K5 a key kept or forgotten in the answer is what the reads after it see', async () => {
    const st = exactStore({});
    bindStore(st as never);
    expect((await keyState()).state).toBe('absent');
    adoptKey(st as never, 'test-key-B-not-real', 'kid-B');                          // the sign-in's keep (app.ts signInEnv.keep)
    st.bound = false;
    expect((await keyState()).state).toBe('present');
    st.bound = true;
    forgetCredentials();
    st.bound = false;
    expect((await keyState()).state).toBe('absent');
  });

  test('K6 the account answer says whether the key is there', async () => {
    const st = exactStore(KEY);
    bindStore(st as never);
    expect(await accountState('', '', 'native', 'web')).toMatchObject({ state: 'signedIn', signedIn: true });
    bindStore(exactStore({}) as never);
    expect(await accountState('', '', 'native', 'web')).toMatchObject({ state: 'signedOut', signedIn: false });
    const locked = exactStore(KEY); locked.locked = true; bindStore(locked as never);
    expect(await accountState('', '', 'native', 'web')).toMatchObject({ state: 'locked', signedIn: false });
  });
});

describe('the host fact', () => {
  test('K7 native.available decides it, with no native call and no timer', () => {
    let calls = 0;
    const realSet = globalThis.setTimeout; let timers = 0;
    globalThis.setTimeout = ((...a: Parameters<typeof setTimeout>) => { timers += 1; return realSet(...a); }) as typeof setTimeout;
    try {
      const linked = { available: true, later: async () => { calls += 1; return {}; }, call: () => { calls += 1; return {}; } };
      expect(hostCaps(linked as never)).toEqual({ mode: 'native' });
      expect(hostCaps({ ...linked, available: false } as never)).toEqual({ mode: 'none' });
      expect(hostCaps(null as never)).toEqual({ mode: 'none' });
      expect(hostCaps(undefined as never)).toEqual({ mode: 'none' });
    } finally { globalThis.setTimeout = realSet; }
    expect(calls).toBe(0); expect(timers).toBe(0);
  });
});

describe('no native "signed in"', () => {
  test('K8 the contracts and the interface carry no native sign-in name, and the account answer is keyed on shared facts only', () => {
    const contracts = readdirSync('.').filter((f) => f.endsWith('.contract')).map((f) => readFileSync(f, 'utf8')).join('\n');
    for (const n of ['choirAuth', 'choirPlayer', 'choirRefresh', 'ChoirAuth', 'ChoirPlayer', 'au.signedIn', 'pl.playing']) expect(contracts).not.toContain(n);
    const iface = readFileSync('../exact-package/exact.json', 'utf8');
    for (const n of ['"choirAuth"', '"choirPlayer"', '"choirRefresh"', '"ChoirAuth"', '"ChoirPlayer"']) expect(iface).not.toContain(n);
    const acs = contracts.split('\n').find((l) => l.includes('resource acs = accountState(')) ?? '';
    expect(acs).not.toContain('au.');
    expect(readFileSync('host.ts', 'utf8')).not.toContain('setTimeout');
    expect(readFileSync('modules/apple/ModelsEntry.swift', 'utf8')).not.toContain('"keystate"');
  });
});

describe('a late native sign-out with a store bound', () => {
  const SEED = { 'app:/data/takes.json': 'x', 'app:/data/podcasts-library.json': 'x' };
  const left = (m: ReturnType<typeof memFs>) => Object.keys(SEED).filter((k) => m.files.has(k));
  const lateNative = () => { let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; }); return { release: () => release(), n: { available: true, later: async (r: { op: string }) => (r.op === 'signOut' ? (await gate, { done: true }) : { ok: true }) } }; };
  const settle = async () => { for (let i = 0; i < 20; i++) await new Promise((r) => setTimeout(r, 5)); };
  test('K9 the late done forgets the key and wipes; a sign-in kept in the window is left alone', async () => {
    const real = globalThis.fetch; globalThis.fetch = (async () => new Response('{}', { status: 200 })) as never;
    try {
      const st = exactStore(KEY); bindStore(st as never);
      const m = memFs({ ...SEED, [STAMP_FILE]: await digest(KEY[KEY_NAME]) });
      const a = lateNative();
      const out = await accountSignOut(m.storage, st as never, a.n as never, { deadlineMs: 20, revoke: async () => 'revoked' });
      expect(out.done).toBe(false);
      st.bound = false; a.release(); await settle();
      expect((await keyState()).state).toBe('absent');
      expect(left(m)).toEqual([]);

      resetSeam(); resetSettled();
      const st2 = exactStore(KEY); bindStore(st2 as never);
      const m2 = memFs({ ...SEED, [STAMP_FILE]: await digest(KEY[KEY_NAME]) });
      const b = lateNative();
      await accountSignOut(m2.storage, st2 as never, b.n as never, { deadlineMs: 20, revoke: async () => 'revoked' });
      adoptKey(st2 as never, 'test-key-C-not-real', 'kid-C');                        // a sign-in finished inside the window
      b.release(); await settle();
      expect((await keyState()).state).toBe('present');
      expect(left(m2).length).toBe(2);
    } finally { globalThis.fetch = real; }
  });
});
