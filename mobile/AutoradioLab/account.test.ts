// THE ACCOUNT BOUNDARY AS PARTITIONS (docs/design/account-boundary-2026-11-21.md, "Accounts are partitions"). Station 1. Failure list, written from the design's operations and invariants BEFORE the code:
//  L1  launch with a live key: the sweep deletes every root no live key names (data, cache and tmp), the leftover top-level paths and `app:/cache/mail-*`, keeps the live root and `anonymous`, and READS no file.
//  L2  launch with no key: the world is anonymous; every account root is swept (a root with no key is an orphan), `anonymous` stays.
//  L3  launch with an unreadable key (I6): LOCKED. Nothing is swept, no world opens, every gated answer refuses; when the key reads, the next answer sweeps and opens.
//  L4  changed key: the old account's root is swept, the new key's root is its own, and the world is the new key's.
//  L5  a signed-out process whose key appears (a sign-in) moves to that account's world, once; a second account world is refused (I5).
//  S1  sign out of one: the session ends (the world closes, no world opens again), the native sign-out runs once, the key is gone, the account folders are deleted, the card says to close and reopen.
//  S2  kill mid sign-out: the key is gone and the root is left (a refused removal): the next launch deletes the orphan before any page, and never reads it.
//  S3  the key still there after the native sign-out (the delete failed): the card says it did not finish, the root is KEPT, the process stays ended and the relaunch is still signed in.
//  S4  no native module: nothing changes, and the world stays open.
//  S5  a chain of the account that left writes, sends and asks native nothing after the sign-out (the bound storage and the request are inert), and a write already in flight lands in the account's own root, which no key names.
//  C1  the card: signedIn with the handle (16 hex), signedOut, locked, rejected with Sign out, ended with the three sentences; the handle moves when the account opens.
import { afterEach, describe, expect, test } from 'bun:test';
import * as account from './account.ts';
import * as api from './api.ts';
import { accountState } from './account-state.ts';
import { WORDS } from './words.ts';
import { resetHost } from './host.ts';
import { dig, events, fakeStore, KEY_A, KEY_B, memFs, type Mem } from './account-phone.ts';

// THE KEY IS EXACT'S (Round 12): the "phone" is a fake secret store (`fakeStore`, `hide()` = a Keychain the host could not read yet), the markers under `app:/data`, and a native module that holds the key in memory (`holdKey`) and forgets its files.
type Phone = { key: 'present' | 'absent' | 'unreadable'; forgetDone: boolean; log: string[] };
let W: Phone; let m: Mem; let reads: string[]; let store: ReturnType<typeof fakeStore>; let forgets: number;
const ROOT = (k: string, space = 'data'): string => `app:/${space}/accounts/${dig(k)}`;
const settle = (p: Promise<unknown>): Promise<unknown> => Promise.race([p.then(() => undefined, () => undefined), new Promise((r) => setTimeout(r, 40))]);
const native = () => ({ available: true, later: async (r: { op: string }) => { W.log.push(r.op); if (r.op === 'forgetAccountFiles') { forgets += 1; return { done: W.forgetDone }; } if (r.op === 'hello') return { state: 'native' }; return { ok: true }; } });

/** A process that has just started on a phone whose store holds `key` and whose files are `files`. Every read of a file is logged (the sweep must read none). */
function boot(files: Record<string, string>, key: Phone['key'] = 'present', value = KEY_A): void {
  events.length = 0; reads = []; forgets = 0;
  W = { key, forgetDone: true, log: [] };
  store = fakeStore(key === 'absent' ? {} : { 'api-key': value, 'key-id': 'kid-1' });
  const f = { ...files }; if (key !== 'absent') f[account.SIGNED] = '1'; if (key === 'unreadable') store.hide();
  m = memFs(f);
  const read = m.storage.fs.readFile.bind(m.storage.fs); (m.storage as { fs: { readFile: unknown } }).fs.readFile = async (p: string) => { reads.push(p); return read(p); };
  account.resetBoundary(); api.resetSeam(); api.bindNative(native() as never);
  globalThis.fetch = (async () => new Response('{}', { status: 200 })) as never;
}
afterEach(() => { account.resetBoundary(); api.resetSeam(); delete (globalThis as { exactHost?: unknown }).exactHost; });
const has = (p: string): boolean => [...m.files.keys()].some((k) => k === p || k.startsWith(p + '/'));
const relaunch = (): void => { account.resetBoundary(); api.resetSeam(); api.bindNative(native() as never); };
// Every call is an ANSWER: the store is there in its synchronous first turn only (account-phone.ts fakeStore), as on the host.
const gated = (): Promise<{ readable: boolean; state: string; world: api.World }> => store.turn(() => account.enter(m.storage as never, store as never));
const out = (): Promise<account.SignOutResult> => store.turn(() => account.accountSignOut(m.storage as never, store as never));
const ending = (): Promise<account.SignOutResult> => store.turn(() => account.accountEnding(m.storage as never, store as never));
const card = (tries = 0) => store.turn(() => accountState('', 'native', tries, m.storage as never, store as never));
/** A launch that has to finish a sign-out takes more than one answer (the secrets are forgotten in a turn that has the store): ask until it is no longer starting. */
const launched = async (): Promise<{ readable: boolean; state: string; world: api.World }> => { let r = await gated(); for (let i = 0; i < 6 && r.state === 'locked'; i++) r = await gated(); return r; };

describe('L launch: the sweep and the one-world rule', () => {
  test('L1 a live key: orphan roots, leftovers and old mail originals go, the live root and anonymous stay, and nothing is read', async () => {
    boot({
      [`${ROOT(KEY_A)}/takes.json`]: 'mine', [`${ROOT(KEY_B)}/takes.json`]: 'planted', [`${ROOT(KEY_B, 'cache')}/files/x`]: 'x', [`${ROOT(KEY_B, 'tmp')}/y`]: 'y', 'app:/data/accounts/anonymous/paper-front.json': '{}',
      'app:/data/accounts/not-a-digest/z': 'z', 'app:/data/takes.json': 'leftover', 'app:/data/mail/inbox': 'leftover', 'app:/cache/mail-abc.html': 'leftover', 'app:/cache/other.html': 'keep', 'app:/data/theme-choice.json': '{}',
    });
    const b = await gated();
    expect(b.readable).toBe(true); expect(b.state).toBe('open'); expect(b.world.kind).toBe('account'); expect(b.world.digest).toBe(dig(KEY_A));
    expect(has(`${ROOT(KEY_A)}/takes.json`)).toBe(true); expect(has('app:/data/accounts/anonymous')).toBe(true);
    expect(has(ROOT(KEY_B))).toBe(false); expect(has(ROOT(KEY_B, 'cache'))).toBe(false); expect(has(ROOT(KEY_B, 'tmp'))).toBe(false); expect(has('app:/data/accounts/not-a-digest')).toBe(false);
    expect(has('app:/data/takes.json')).toBe(false); expect(has('app:/data/mail')).toBe(false); expect(has('app:/cache/mail-abc.html')).toBe(false);
    expect(has('app:/cache/other.html')).toBe(true); expect(has('app:/data/theme-choice.json')).toBe(true);
    expect(reads).toEqual([]);
  });
  test('L2 no key: anonymous world; every account root is an orphan and goes', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: 'a', 'app:/data/accounts/anonymous/paper-front.json': '{}' }, 'absent');
    const b = await gated();
    expect(b.readable).toBe(true); expect(b.state).toBe('anonymous'); expect(b.world.kind).toBe('anonymous'); expect(b.world.root).toBe('anonymous');
    expect(has(ROOT(KEY_A))).toBe(false); expect(has('app:/data/accounts/anonymous')).toBe(true); expect(reads).toEqual([]);
  });
  test('L3 a key that does not read yet (the marker is there) is LOCKED: nothing is swept, no world opens; when it reads, the next answer sweeps and opens', async () => {
    boot({ [`${ROOT(KEY_B)}/takes.json`]: 'orphan?', 'app:/data/takes.json': 'leftover' }, 'unreadable');
    const b = await gated();
    expect(b.readable).toBe(false); expect(b.state).toBe('locked'); expect(api.currentWorld().kind).toBe('none');
    expect(has(`${ROOT(KEY_B)}/takes.json`)).toBe(true); expect(has('app:/data/takes.json')).toBe(true);
    store.hide(false);
    const c = await gated();
    expect(c.readable).toBe(true); expect(has(ROOT(KEY_B))).toBe(false); expect(has('app:/data/takes.json')).toBe(false);
  });
  test('L4 a changed key: the old root is swept, the new key has its own, and the world is the new key\'s', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: 'A\'s' }, 'present', KEY_B);
    const b = await gated();
    expect(b.world.digest).toBe(dig(KEY_B)); expect(has(ROOT(KEY_A))).toBe(false);
    const bound = api.bindStorage(m.storage as never, b.world);
    await bound.fs.atomicWriteFile('app:/data/takes.json', new TextEncoder().encode('B'));
    expect(m.text(`${ROOT(KEY_B)}/takes.json`)).toBe('B');
  });
  test('L5 T1: a key held while the world is anonymous moves it to that account ONCE; a second account world is refused', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: 'a' }, 'absent');
    const a = await gated(); expect(a.state).toBe('anonymous'); expect(has(ROOT(KEY_A))).toBe(false);
    store.put('api-key', KEY_A);                                                               // a sign-in stored it
    const b = await gated(); expect(b.state).toBe('open'); expect(b.world.digest).toBe(dig(KEY_A)); expect(a.world.closed).toBe(true);
    expect(() => api.openWorld(dig(KEY_B))).toThrow(/one account world/);
    expect(api.currentWorld()).toBe(b.world);
  });
  test('L6 every root name is 64 hex characters (I8): no key and no path of one appears in a root', async () => {
    boot({}, 'present', KEY_A);
    const b = await gated(); const bound = api.bindStorage(m.storage as never, b.world);
    await bound.fs.atomicWriteFile('app:/data/x', new Uint8Array([1])); await bound.fs.atomicWriteFile('app:/cache/y', new Uint8Array([1]));
    for (const k of m.files.keys()) { expect(k).not.toContain(KEY_A); const r = /\/accounts\/([^/]+)\//.exec(k); if (r) expect(r[1]).toMatch(/^[0-9a-f]{64}$/); }
  });
  test('L8 no storage: locked, no world, nothing latched; with storage the launch then runs', async () => {
    boot({ [`${ROOT(KEY_B)}/takes.json`]: 'orphan' });
    const a = await account.enter(undefined as never, store as never); expect(a.state).toBe('locked'); expect(api.currentWorld().kind).toBe('none');
    const b = await gated(); expect(b.state).toBe('open'); expect(has(ROOT(KEY_B))).toBe(false);
  });
});

describe('S sign-out ends the session', () => {
  const seed = (): Record<string, string> => ({ [`${ROOT(KEY_A)}/takes.json`]: 'a', [`${ROOT(KEY_A, 'cache')}/files/f`]: 'f', [`${ROOT(KEY_A, 'tmp')}/t`]: 't', 'app:/data/accounts/anonymous/paper-front.json': '{}', 'app:/data/theme-choice.json': '{}', 'app:/tmp/picked/doc.pdf': 'pdf' });
  test('S1 sign out: the world closes, no world opens again, the secrets go, NOTHING is wiped in this process, the card says close and reopen', async () => {
    boot(seed());
    const w = (await gated()).world; expect(w.closed).toBe(false);
    const rms: string[] = []; const rm = m.storage.fs.rm.bind(m.storage.fs); (m.storage as { fs: { rm: unknown } }).fs.rm = async (p: string) => { rms.push(p); return rm(p); };
    const r = await out(); await card();      // the secrets are forgotten by the next answer that has the store
    expect(r.done).toBe(true); expect(store.kept.size).toBe(0); expect(w.closed).toBe(true); expect(api.sessionEnded()).toBe(true);
    expect(rms).toEqual([]); expect(has(ROOT(KEY_A))).toBe(true); expect(has('app:/tmp/picked')).toBe(true); expect(m.files.has(account.ENDING)).toBe(true);
    const after = await gated(); expect(after.readable).toBe(false); expect(after.state).toBe('ended');
    expect(() => api.openWorld(dig(KEY_B))).toThrow(/session has ended/); expect(() => api.openWorld('anonymous')).toThrow(/session has ended/);
    expect(await card()).toMatchObject({ state: 'ended', message: WORDS.endedSignedOut, action: '', handle: '' });
  });
  test('S2 kill after the secrets are forgotten: the next launch finds the mark, sweeps every root before any world opens (reading none), has native forget its files and clears both marks', async () => {
    boot(seed()); await gated(); await out(); await card();
    expect(store.kept.size).toBe(0); expect(has(ROOT(KEY_A))).toBe(true); expect(m.files.has(account.ENDING)).toBe(true);
    reads.length = 0; relaunch(); const b = await launched();
    expect(m.files.has(account.ENDING)).toBe(false); expect(m.files.has(account.SIGNED)).toBe(false); expect(forgets).toBe(1);
    expect(b.state).toBe('anonymous'); expect(has(ROOT(KEY_A))).toBe(false); expect(has(ROOT(KEY_A, 'cache'))).toBe(false); expect(has('app:/tmp/picked')).toBe(false); expect(reads.filter((p) => p.includes('/accounts/'))).toEqual([]);
  });
  test('S3 the store refuses to forget: did not finish, the process stays ended, the key is kept, and the relaunch (the mark is the intent) finishes it', async () => {
    boot(seed()); await gated(); const real = store.forget; store.forget = (): void => { throw new Error('keychain refused'); };
    await out();
    expect(await card()).toMatchObject({ state: 'ended', message: WORDS.endedUnfinished }); expect(store.kept.has('api-key')).toBe(true); expect(has(`${ROOT(KEY_A)}/takes.json`)).toBe(true);
    expect((await gated()).state).toBe('ended');
    store.forget = real; relaunch();
    const c = await launched(); expect(c.state).toBe('anonymous'); expect(store.kept.size).toBe(0); expect(has(ROOT(KEY_A))).toBe(false);
  });
  test('S5 a chain of the account that left does nothing after the sign-out; a write already in flight lands in its own root, which the next launch removes', async () => {
    boot(seed()); const w = (await gated()).world;
    const bound = api.bindStorage(m.storage as never, w);
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    m.faults.writeHold.set(`${ROOT(KEY_A)}/inflight.json`, gate);
    const inflight = bound.fs.atomicWriteFile('app:/data/inflight.json', new Uint8Array([1]));
    const calls: string[] = []; globalThis.fetch = (async (u: unknown) => { calls.push(String(u)); return new Response('{}'); }) as never;
    await out();
    calls.length = 0;
    const late = [bound.fs.atomicWriteFile('app:/data/late.json', new Uint8Array([2])), bound.fs.readFile('app:/data/takes.json'), api.authedJSON('/api/x', { world: w }), api.nativeInert()?.({ op: 'takeState' })];
    release();
    await Promise.all(late.map((p) => settle(p as Promise<unknown>)));
    expect(has(`${ROOT(KEY_A)}/late.json`)).toBe(false); expect(calls).toEqual([]);
    await settle(inflight);
    for (const k of m.files.keys()) if (k.includes('inflight')) expect(k.startsWith(ROOT(KEY_A))).toBe(true);
    relaunch(); await launched();
    expect(has(ROOT(KEY_A))).toBe(false);
  });
});

describe('S6 raw reads of a closed world', () => {
  test('after the session ends nothing is held: the computer id and the key read nothing', async () => {
    boot({}); await gated(); expect(api.heldSession()?.key).toBe(KEY_A);
    await ending(); expect(api.heldSession()).toBeNull(); expect(await api.keptComputerId()).toBeNull();
  });
});

describe('E the ending mark: a kill never shows the old account\'s words', () => {
  const seed = (): Record<string, string> => ({ [`${ROOT(KEY_A)}/takes.json`]: 'a' });
  test('E1 the mark is written BEFORE the secrets are forgotten, and stays after a clean sign-out (only the next launch clears it)', async () => {
    boot(seed()); await gated(); let atForget = ''; const real = store.forget; store.forget = (n: string): void => { atForget = m.files.has(account.ENDING) ? 'marked' : 'unmarked'; real(n); };
    const r = await out(); await card();
    expect(atForget).toBe('marked'); expect(r.done).toBe(true); expect(m.files.has(account.ENDING)).toBe(true);
  });
  test('E2 a kill after the mark and before the forget: the mark is the person\'s intent, so the next launch FINISHES the sign-out (revoke, forget, sweep, native forgets, marks removed)', async () => {
    boot({ ...seed(), 'app:/tmp/picked/doc.pdf': 'pdf' }); await gated(); await ending();
    expect(m.files.has(account.ENDING)).toBe(true); expect(store.kept.has('api-key')).toBe(true);
    relaunch(); const b = await launched();
    expect(store.kept.size).toBe(0); expect(m.files.has(account.ENDING)).toBe(false); expect(b.state).toBe('anonymous'); expect(has(ROOT(KEY_A))).toBe(false); expect(has('app:/tmp/picked')).toBe(false); expect(forgets).toBe(1);
  });
  test('E2b two sign-out taps share one revoke and one forget', async () => {
    boot(seed()); await gated(); const calls: string[] = []; globalThis.fetch = (async (u: unknown, i: { method?: string }) => { calls.push(`${i?.method} ${String(u)}`); return new Response('{}'); }) as never;
    const [a, b] = await Promise.all([out(), out()]); await card(); expect(a.done && b.done).toBe(true); expect(calls.filter((c) => c.startsWith('DELETE')).length).toBe(1);
  });
  test('E3 native cannot forget its files: BLOCKED, no world (not even anonymous), one card that promises nothing; the mark stays and the next launch converges', async () => {
    boot(seed()); await gated(); await out(); await card(); W.forgetDone = false;
    relaunch(); reads.length = 0; const a = await launched(); expect(a.readable).toBe(false); expect(a.state).toBe('ended'); expect(api.sessionEnded()).toBe(true); expect(api.currentWorld().kind).toBe('none');
    expect(m.files.has(account.ENDING)).toBe(true);
    expect(await card()).toMatchObject({ state: 'ended', message: WORDS.endedBlocked, action: '' });
    W.forgetDone = true; relaunch(); const c = await launched(); expect(c.state).toBe('anonymous'); expect(has(ROOT(KEY_A))).toBe(false); expect(m.files.has(account.ENDING)).toBe(false);
  });
  test('E5 a mark that cannot be written stops the sign-out before the key is touched', async () => {
    boot(seed()); await gated(); m.faults.write.add(account.ENDING);
    const r = await out();
    expect(r.done).toBe(false); expect(store.kept.has('api-key')).toBe(true); expect(has(`${ROOT(KEY_A)}/takes.json`)).toBe(true);
    expect(await card()).toMatchObject({ state: 'ended', message: WORDS.endedUnfinished });
  });
  test('B1 the sweep of the old pre-partition leftovers finishes BEFORE any world opens, anonymous included; a sweep that fails is BLOCKED, not open', async () => {
    boot({ 'app:/data/takes.json': 'leftover', [`${ROOT(KEY_A)}/takes.json`]: 'a' }, 'absent'); m.faults.rm.add('app:/data/takes.json');
    const a = await gated(); expect(a.readable).toBe(false); expect(a.state).toBe('ended'); expect(api.currentWorld().kind).toBe('none');
    expect(await card()).toMatchObject({ state: 'ended', message: WORDS.endedBlocked });
  });
  test('R1 a sign-out that lands while the launch is running leaves the process ENDED, never anonymous, and no source runs', async () => {
    boot(seed());
    const waiting = gated(); await ending();
    const r = await waiting; expect(r.state).toBe('ended'); expect(r.readable).toBe(false); expect(api.currentWorld().kind).toBe('none');
  });
  test('B1b the sweep of the old leftovers happens while NO world is open: every removal sees `none`', async () => {
    boot({ 'app:/data/takes.json': 'leftover', 'app:/data/mail/inbox': 'leftover', [`${ROOT(KEY_B)}/takes.json`]: 'orphan' }, 'absent');
    const kinds: string[] = []; const rm = m.storage.fs.rm.bind(m.storage.fs); (m.storage as { fs: { rm: unknown } }).fs.rm = async (p: string) => { kinds.push(api.currentWorld().kind); return rm(p); };
    const b = await gated(); expect(b.state).toBe('anonymous'); expect(kinds.length).toBeGreaterThan(0); expect(kinds.every((k) => k === 'none')).toBe(true);
  });
  test('P9 the card draws at once: step one ends the session before step two has run', async () => {
    boot(seed()); await gated(); await ending();
    expect(await card()).toMatchObject({ state: 'ended' }); expect((await gated()).readable).toBe(false);
  });
});

describe('C the card', () => {
  test('C1 signed in: the handle is the first 16 hex of the key digest; signed out; the handle moves when the account opens (T1)', async () => {
    boot({}, 'absent'); const o = await card(); expect(o).toMatchObject({ state: 'signedOut', action: 'signIn', handle: 'anonymous', message: WORDS.signedOut, radioMessage: WORDS.radioSignedOut });
    store.put('api-key', KEY_A); const inn = await card();
    expect(inn).toMatchObject({ state: 'signedIn', action: '', handle: dig(KEY_A).slice(0, 16), message: '' }); expect(inn.handle).not.toBe(o.handle);
  });
  test('C2 starting says so and offers nothing', async () => {
    boot({}, 'unreadable'); expect(await card()).toMatchObject({ state: 'locked', message: WORDS.starting, action: '', handle: '' });
  });
  test('C3 a refused key offers Sign out, not Sign in; a 200 clears it', async () => {
    boot({}, 'present'); const w = (await gated()).world;
    globalThis.fetch = (async () => new Response('', { status: 401 })) as never;
    await api.authedJSON('/api/x', { world: w }).catch(() => undefined);
    expect(await card()).toMatchObject({ state: 'rejected', action: 'signOut', message: WORDS.rejected });
    globalThis.fetch = (async () => new Response('{}', { status: 200 })) as never;
    await api.authedJSON('/api/x', { world: w }).catch(() => undefined);
    expect((await card()).state).toBe('signedIn');
  });
  test('P4 a key that stays unreadable has a bound: after five re-reads the launch is anonymous and Sign in is offered (the key was lost); until then the card only says it is starting', async () => {
    boot({}, 'unreadable');
    expect(await card()).toMatchObject({ state: 'locked', action: '', alsoSignOut: false });
    expect(await card(5)).toMatchObject({ state: 'signedOut', action: 'signIn' });
  });
  test('P4b the key that reads again after the bound moves the anonymous world to the account (T1), once', async () => {
    boot({}, 'unreadable'); await card(5); store.hide(false);
    const after = await card(5); expect(after.state).toBe('signedIn'); expect(after.handle).toBe(dig(KEY_A).slice(0, 16));
  });
});
