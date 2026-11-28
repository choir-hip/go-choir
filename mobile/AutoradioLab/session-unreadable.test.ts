// A key read that fails is "unreadable", never "signed out" (docs/problems/a-signed-in-app-asks-the-server-for-the-changelog-about-forty-times-a-second-2026-11-28.md).
// Exact's secret store works only while an answer runs; a read outside one throws (docs/reference/exact/passkeys.md "The secret store"). Every value here is made up.
// Failure list, written before the code (B = fails before the change, G = guards what already held):
//  U1 B a session read the source marks unreadable (or one that throws) is a SignedOutError of reason 'unknown' or 'locked', never 'none', even when the key-state read disagrees; nothing is sent.
//  U2 B the log line `api: session` says `refused=true` for a refused read and `refused=false` for an answer; never a value.
//  U3 G a refused session read never lets `credentialsAbsent` answer true; a refused `get` does not either.
//  U4 G a key state of `present` whose session read is refused is `unreadable`, so `reconcile` wipes nothing (stamp of another account, a pending marker, any mix).
//  U5 G a refused read never clears `blockedDigest` (refreshSealBlock, send), and a sign-out begun while the key is unreadable stays blocked.
//  U6 G sign-out in a shared-only host does not report success while the Exact secrets cannot be read back.
//  U7 B a refused read is not remembered: the next answer that can read gets the session (no stale "unreadable").
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { authedJSON, bindNative, bindStore, beginSignOut, abortSignOut, credentialsAbsent, keyState, refreshSealBlock, resetSeam, sealingBlocked, SignedOutError } from './api.ts';
import { reconcile, accountSignOut, STAMP_FILE, PENDING_FILE } from './account.ts';
import { KEY_NAME } from './app-signin.ts';

const KEY = 'test-key-0001-not-real';
let lines: string[] = [];
let sent = 0;
const realLog = console.log;
beforeEach(() => {
  resetSeam(); lines = []; sent = 0;
  console.log = (...a: unknown[]) => { lines.push(a.map(String).join(' ')); };
  globalThis.fetch = (async () => { sent++; return new Response('{}', { status: 200 }); }) as never;
});
const done = (): void => { console.log = realLog; };
afterEach(() => { console.log = realLog; resetSeam(); });      // a bound store must not leak into the next test file (one process)

/** A reader whose `session` op is refused in the way `refuse` says; the other ops answer as `keystate` says. */
const reader = (refuse: 'flag' | 'throw' | 'none', keystate: string = 'absent') => ({
  available: true,
  later: async (r: { op?: string; account?: string }) => {
    if (r.op === 'session') { if (refuse === 'throw') throw new Error('no store'); return refuse === 'flag' ? { found: false, unreadable: true } : { found: true, value: KEY, computerId: 'c1' }; }
    if (r.op === 'keystate') return { state: keystate };
    if (r.op === 'get') return { found: false, unreadable: true };
    return { done: true };
  },
});
const reasonOf = async (): Promise<string> => { try { await authedJSON('/api/x'); return 'no error'; } catch (e) { return e instanceof SignedOutError ? e.reason : `other: ${String(e)}`; } };

describe('U1 a refused session read is not "no key"', () => {
  test('flagged unreadable, key state says absent: reason is unknown', async () => {
    bindNative(reader('flag', 'absent') as never);
    expect(await reasonOf()).toBe('unknown'); expect(sent).toBe(0); done();
  });
  test('flagged unreadable, key state says present: reason is unknown', async () => {
    bindNative(reader('flag', 'present') as never);
    expect(await reasonOf()).toBe('unknown'); expect(sent).toBe(0); done();
  });
  test('flagged unreadable, key state says unreadable: reason is locked (the existing meaning)', async () => {
    bindNative(reader('flag', 'unreadable') as never);
    expect(await reasonOf()).toBe('locked'); expect(sent).toBe(0); done();
  });
  test('a session read that throws is a SignedOutError (unknown), not a bare Error', async () => {
    bindNative(reader('throw', 'absent') as never);
    expect(await reasonOf()).toBe('unknown'); expect(sent).toBe(0); done();
  });
  test('a secret store that throws on every read: never reason none, nothing sent', async () => {
    bindStore({ get: () => { throw new Error('no turn is running'); }, set: () => undefined, forget: () => undefined });
    const why = await reasonOf();
    expect(['unknown', 'locked']).toContain(why); expect(sent).toBe(0); done();
  });
  test('a store with no key at all is still "none" (absent is not unreadable)', async () => {
    bindStore({ get: () => null, set: () => undefined, forget: () => undefined });
    expect(await reasonOf()).toBe('none'); done();
  });
});

describe('U2 the log says refused or absent', () => {
  test('refused, then absent, then found: three different lines, no value', async () => {
    let mode: 'refuse' | 'absent' | 'found' = 'refuse';
    bindStore({ get: (n: string) => { if (mode === 'refuse') throw new Error('x'); return mode === 'found' ? (n === KEY_NAME ? KEY : 'c1') : null; }, set: () => undefined, forget: () => undefined });
    await reasonOf(); mode = 'absent'; await reasonOf(); mode = 'found'; await authedJSON('/api/x');
    const s = lines.filter((l) => l.startsWith('api: session'));
    done();
    expect(s.length).toBe(3);
    expect(s[0]).toContain('refused=true'); expect(s[1]).toContain('refused=false'); expect(s[1]).toContain('found=false'); expect(s[2]).toContain('found=true');
    expect(lines.join('\n')).not.toContain(KEY);
  });
});

describe('U3 a refused read is not absent', () => {
  test('credentialsAbsent is false when the store refuses', async () => {
    bindStore({ get: () => { throw new Error('x'); }, set: () => undefined, forget: () => undefined });
    expect(await credentialsAbsent()).toBe(false); done();
  });
  test('credentialsAbsent is false when only one item read is refused', async () => {
    bindStore({ get: (n: string) => { if (n === 'choir.key-id') throw new Error('x'); return null; }, set: () => undefined, forget: () => undefined });
    expect(await credentialsAbsent()).toBe(false); done();
  });
  test('credentialsAbsent is true when all three answer absent', async () => {
    bindStore({ get: () => null, set: () => undefined, forget: () => undefined });
    expect(await credentialsAbsent()).toBe(true); done();
  });
});

describe('U4 a refused session read never wipes', () => {
  test.each([['other', 'none'], ['other', 'pending'], ['none', 'pending'], ['same', 'pending']] as const)('key present but session refused: stamp %s, marker %s', async (stamp, marker) => {
    bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'keystate' ? { state: 'present' } : r.op === 'session' ? { found: false, unreadable: true } : { done: true, available: true }) } as never);
    const files = new Map<string, Uint8Array>([['app:/data/takes.json', new Uint8Array([1])]]);
    if (stamp !== 'none') files.set(STAMP_FILE, new TextEncoder().encode('f'.repeat(64)));
    if (marker === 'pending') files.set(PENDING_FILE, new TextEncoder().encode('1'));
    const storage = { fs: {
      stat: async (p: string) => { if (!files.has(p)) throw new Error('absent'); return { size: 1 }; },
      readFile: async (p: string) => { const f = files.get(p); if (!f) throw new Error('absent'); return f; },
      atomicWriteFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
      writeFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
      rm: async (p: string) => { files.delete(p); },
      readdir: async () => [], mkdir: async () => undefined,
    } } as never;
    const natives: string[] = [];
    const before = new Map(files);
    const out = await reconcile(storage, undefined, { signOut: async () => { natives.push('signOut'); return true; }, wipe: async () => { natives.push('wipe'); return true; } } as never);
    done();
    expect(out.action).toBe('nothing'); expect(natives).toEqual([]);
    expect([...files.keys()].sort()).toEqual([...before.keys()].sort());
  });
  test('keyState: present with a refused session read is unreadable, not absent', async () => {
    bindNative(reader('flag', 'present') as never);
    const k = await keyState(); done();
    expect(k.state).toBe('unreadable');
  });
});

describe('U5 the sign-out block', () => {
  test('a refused read does not clear the block, and a sign-out begun on an unreadable key stays blocked', async () => {
    let refuse = false;
    bindNative({ available: true, later: async (r: { op?: string }) => { if (r.op === 'session') { if (refuse) throw new Error('x'); return { found: true, value: KEY, computerId: 'c1' }; } return { state: 'present' }; } } as never);
    await beginSignOut(); expect(sealingBlocked()).toBe(true);
    refuse = true; await refreshSealBlock(); expect(sealingBlocked()).toBe(true);
    expect(await reasonOf()).toBe('unknown'); expect(sealingBlocked()).toBe(true);
    abortSignOut(true); expect(sealingBlocked()).toBe(false);
    refuse = true; await beginSignOut(); done();
    expect(sealingBlocked()).toBe(true);
  });
});

describe('U6 sign-out is not reported done on a refused read', () => {
  test('shared-only sign-out whose read-back is refused answers not done', async () => {
    let reads = false;
    bindStore({ get: () => { if (reads) throw new Error('x'); return null; }, set: () => undefined, forget: () => { reads = true; } });
    const files = new Map<string, Uint8Array>();
    const storage = { fs: { stat: async () => { throw new Error('absent'); }, readFile: async () => { throw new Error('absent'); }, atomicWriteFile: async (p: string, b: Uint8Array) => { files.set(p, b); }, writeFile: async () => undefined, rm: async (p: string) => { files.delete(p); }, readdir: async () => [], mkdir: async () => undefined } } as never;
    const out = await accountSignOut(storage, undefined, { available: true, later: async () => ({ done: true }) } as never, { platform: 'android', held: null });
    done();
    expect(out.done).toBe(false);
  });
});

describe('U7 a refusal is not remembered', () => {
  test('after a refused read the next read that can answer gets the session', async () => {
    let refuse = true;
    bindStore({ get: (n: string) => { if (refuse) throw new Error('x'); return n === KEY_NAME ? KEY : 'c1'; }, set: () => undefined, forget: () => undefined });
    expect(await reasonOf()).not.toBe('no error');
    refuse = false;
    expect(await reasonOf()).toBe('no error'); expect(sent).toBe(1); done();
  });
});
