// Sign-out orchestrated from shared code (account.ts). Failure list, each line a test below:
//  S1 the native sign-out (key delete, Swift stores, revoke) runs FIRST and the shared wipes only after it answered: the old key is
//     still held while the native side settles what it posts; pending shared work is never sent under a later account.
//  S2 when the native sign-out fails or is missing in a native build, nothing shared is wiped and the answer says so (retry is safe).
//  S3 one wipe that throws does not stop the others; the answer names what failed, never why in the listener's words.
//  S4 every path the shared modules keep under app:/data or app:/cache is removed by some wipe (a new store without a wipe fails here).
//  S5 the answer never carries a key.
//  S6 a kill between the native sign-out and the wipes: the marker is still there at the next launch, which finishes the wipes before any
//     source answers; a clean sign-out leaves no marker; a native sign-out that did not finish leaves none either and wipes nothing.
//  S7 a native build whose native module is absent refuses (nothing wiped, done false); a native call that never answers is a deadline, done false.
//  S8 the real wipes empty a real in-memory file system seeded at EVERY `app:/` path any shared module names; a path no wipe covers fails here.
//  S9 sign out A, sign in B: B sees nothing of A (files, memory, live content store, live vectors, remembered computer, a reply still in flight).
//  S12 the drift guard runs the real wipes against a recording storage: every `app:/` literal the modules name is among the recorded removals (or under one),
//      and the native mail key is forgotten.   S13 the clips remembered in memory are forgotten at sign-out.
//  S14 THE TABLE (auth round 2, review 2026-11-19): key state (3) x stamp (3) x marker (2): what a launch does, and that it never wipes while the key is unreadable or present+same.
//  S15 a kill at each step boundary of a sign-out, then a relaunch: (a) a signed-in account's data is never wiped while its key is present, (b) once the key is gone no
//      store of the old account is left, shared or Swift, and a new account is adopted clean. Findings 1 to 4 of the round are rows here.
//  S10 a different account's key at launch (the stamp) wipes the old data first; the same account, or a phone with no key, keeps it.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { accountSignOut, WIPES, DATA_WIPES, wipeAll, wipedPaths, settleAccount, resetSettled, reconcile, decide, nativeOf, PENDING_FILE, STAMP_FILE, type Facts } from './account.ts';
import { createHash } from 'node:crypto';
import { authedJSON, bindNative, resetSeam, SignedOutError } from './api.ts';
import { ContentStore } from './content.ts';
import { VectorStore } from './search.ts';
import { clipAct, remembered as clipsRemembered } from './clips.ts';


/** A real in-memory file system: files and folders, rm removes a folder's children, readdir lists them. */
function memFs(seed: string[] = []) {
  const files = new Map<string, Uint8Array>();
  for (const p of seed) files.set(p, new TextEncoder().encode('x'));
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
const native = (done = true, log: string[] = []) => ({ available: true, later: async (r: { op: string }) => { log.push(r.op); return { done }; } });

describe('accountSignOut', () => {
  test('S1 native first, then the shared wipes', async () => {
    const order: string[] = [];
    const { storage } = memFs();
    const n = { available: true, later: async (r: { op: string }) => { order.push(`native:${r.op}`); return { done: true }; } };
    const r = await accountSignOut(storage, undefined, n as never, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
    expect(order).toEqual(['native:signOut', 'wipe:a']);
    expect(r).toMatchObject({ done: true, failed: '', message: '' });
  });
  test('S2 a native answer that says not done wipes nothing shared', async () => {
    const r = await accountSignOut(memFs().storage, undefined, native(false) as never, { wipes: [{ name: 'a', run: async () => { throw new Error('must not run'); } }] });
    expect(r.done).toBe(false);
  });
  test('S7 no native module: refuses and wipes nothing', async () => {
    const order: string[] = [];
    for (const n of [null, undefined, { available: false, later: async () => ({}) }]) {
      const r = await accountSignOut(memFs().storage, undefined, n as never, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
      expect(r.done).toBe(false);
    }
    expect(order).toEqual([]);
  });
  test('S3 a throwing wipe does not stop the rest and is named; the marker stays for the next launch', async () => {
    const order: string[] = []; const m = memFs();
    const r = await accountSignOut(m.storage, undefined, native() as never, { wipes: [
      { name: 'first', run: async () => { throw new Error('secret words of the listener'); } },
      { name: 'second', run: async () => { order.push('second'); } },
    ] });
    expect(order).toEqual(['second']); expect(r.done).toBe(true); expect(r.failed).toBe('first'); expect(JSON.stringify(r)).not.toContain('secret words');
    expect(m.files.has(PENDING_FILE)).toBe(true);
  });
  test('S6 the marker is written BEFORE the native call and gone after a clean sign-out', async () => {
    const m = memFs(); let during = false;
    const n = { available: true, later: async () => { during = m.files.has(PENDING_FILE); return { done: true }; } };
    await accountSignOut(m.storage, undefined, n as never, { wipes: [] });
    expect(during).toBe(true); expect(m.files.has(PENDING_FILE)).toBe(false);
  });
});

describe('the wipes', () => {
  test('wipeAll runs every registered wipe and returns the failed names', async () => {
    const ran: string[] = [];
    const failed = await wipeAll(undefined, undefined, [{ name: 'x', run: async () => { ran.push('x'); } }, { name: 'y', run: async () => { throw new Error('no'); } }]);
    expect(ran).toEqual(['x']); expect(failed).toEqual(['y']);
  });
  // The phone's own settings, not the account's: the theme choice and the themes the person installed (themes.ts). The Swift app kept them in UserDefaults and Application Support/themes, which no
  // sign-out ever touched; a sign-out leaves the phone looking the way its owner set it. (Colours and a name; nothing of the account.)
  const PHONE_KEPT = new Set(['app:/data/theme-choice.json', 'app:/data/themes']);
  const allPaths = (): string[] => {
    const dir = new URL('.', import.meta.url).pathname;
    const files = readdirSync(dir).filter((f) => f.endsWith('.ts') && !f.endsWith('.test.ts') && !f.endsWith('.d.ts') && f !== 'app.ts');
    const found = new Set<string>();
    for (const f of files) {
      const src = readFileSync(dir + f, 'utf8');
      // any `app:/data`, `app:/cache` or `app:/tmp` literal, in any quotes, template strings included (the part up to a `${` or the quote)
      for (const m of src.matchAll(/['"`](app:\/(?:data|cache|tmp)(?:\/[^'"`$]*)?)/g)) found.add(m[1].replace(/\/$/, ''));
    }
    return [...found];
  };
  test('S4 every app:/ path any shared module names is covered by the wipe list', () => {
    const found = allPaths();
    expect(found.length).toBeGreaterThan(10);
    const covered = (p: string) => wipedPaths().some((w) => p === w || p.startsWith(w + '/') || p.startsWith(w + '-'));
    // `app:/data`, `app:/cache` and `app:/tmp` themselves are the roots (a `directories` default), and `app:/data/exports`, `app:/cache/mail-*.html` are named below
    const roots = ['app:/data', 'app:/cache', 'app:/tmp'];
    const unknown = found.filter((p) => !roots.includes(p) && !covered(p) && !PHONE_KEPT.has(p) && p !== 'app:/cache/mail-' && !p.startsWith('app:/cache/mail-'));
    expect(unknown).toEqual([]);
  });
  test('S19 the phone\'s theme (the choice and the themes under app:/data/themes, a Texture\'s included) survives a sign-out wipe: parity with the Swift AccountWipe, which never touched the UserDefaults choice or Application Support/Autoradio/themes', async () => {
    const m = memFs(['app:/data/theme-choice.json', 'app:/data/themes/texture-ember.json', 'app:/data/themes/mine.json', 'app:/data/recent-files.json']);
    const failed = await wipeAll(m.storage, undefined, WIPES);
    expect(failed).toEqual([]);
    expect([...m.files.keys()].sort()).toEqual(['app:/data/theme-choice.json', 'app:/data/themes/mine.json', 'app:/data/themes/texture-ember.json']);
  });
  test('S8 the real wipes leave an in-memory file system empty (seeded at every path the modules name, mail files and the stamp included)', async () => {
    const seed = [...allPaths().filter((p) => p.split('/').length > 2 && p !== PENDING_FILE && !PHONE_KEPT.has(p)).flatMap((p) => (p.endsWith('/') ? [] : [p, `${p}/child.bin`])), 'app:/data/mail/msg-1.bin', 'app:/cache/mail-1.html', STAMP_FILE];
    const m = memFs(seed);
    const failed = await wipeAll(m.storage, undefined, WIPES);
    expect(failed).toEqual([]);
    expect([...m.files.keys()]).toEqual([]);
  });
});

describe('S9 sign out A, sign in B', () => {
  const KEYA = 'test-key-A-not-real'; const KEYB = 'test-key-B-not-real';
  test('B sees nothing of A: files, a live content store, a live vector store, the remembered computer; and a reply still in flight', async () => {
    resetSeam(); resetSettled();
    let key = KEYA; const seenComputers: string[] = [];
    const reader = { available: true, later: async (r: { op: string }) => (r.op === 'session' ? { found: true, value: key, computerId: '' } : { done: true }) };
    bindNative(reader as never);
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async (u: string, init: RequestInit) => {
      const h = init.headers as Record<string, string>;
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: `comp-of-${h.Authorization.includes('-A-') ? 'A' : 'B'}`, current: true }] }), { status: 200 });
      seenComputers.push(h['X-Choir-Computer']);
      if (String(u).endsWith('/late')) { await gate; return new Response('{"a":"data"}', { status: 200 }); }
      return new Response('{}', { status: 200 });
    }) as never;
    const m = memFs(['app:/data/takes.json', 'app:/data/podcasts-library.json', 'app:/data/mail/msg-1.bin']);
    const db = { execute: async () => undefined, query: async () => ({ rows: [] }) };
    const content = new ContentStore(db as never); const vectors = new VectorStore(); vectors.wipe(); vectors.dirty = false;
    await authedJSON('/api/one');                                          // A discovers its computer
    const late = authedJSON('/late').then(() => 'wrote', (e) => (e instanceof SignedOutError ? 'dropped' : 'error'));
    await new Promise((r) => setTimeout(r, 5));
    const gen0 = content.generation;
    const r = await accountSignOut(m.storage, undefined, reader as never);
    expect(r.done).toBe(true);
    expect([...m.files.keys()]).toEqual([]);
    expect(content.generation).toBeGreaterThan(gen0);                       // a running sync would drop its writes
    release();
    expect(await late).toBe('dropped');                                    // the late reply of A never reaches a caller
    key = KEYB;                                                             // B signs in
    await authedJSON('/api/two');
    expect(seenComputers[0]).not.toBe(seenComputers[seenComputers.length - 1]);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------- the table and the kills

const dig = (k: string): string => createHash('sha256').update(k).digest('hex');
type World = { key: 'present' | 'absent' | 'unreadable'; value: string; swift: boolean; native: string[]; killInNative: boolean; hang: boolean; lateRelease: (() => void) | null };
let W: World;
const newWorld = (key: World['key'] = 'present', value = 'test-key-A-not-real'): World => ({ key, value, swift: true, native: [], killInNative: false, hang: false, lateRelease: null });
/** The app module as the model of the phone: the Keychain item (3 answers), the Swift stores, and sign-out's two native steps (key delete, then Swift wipe). */
const phone = () => ({ available: true, later: async (r: { op: string }) => {
  if (r.op === 'keystate') return { state: W.key };
  if (r.op === 'session') return W.key === 'present' ? { found: true, value: W.value, computerId: '' } : { found: false };
  if (r.op === 'signOut') {
    W.native.push('signOut');
    if (W.hang) await new Promise<void>((res) => { W.lateRelease = res; });
    W.key = 'absent';                                            // ChoirSignIn.swift:128, the key goes first
    if (W.killInNative) throw new Error('killed between the key delete and the Swift wipe');
    W.swift = false;                                             // :129, AccountWipe
    return { done: true };
  }
  if (r.op === 'wipe') { W.native.push('wipe'); W.swift = false; return { done: true }; }
  return { ok: true };
} });
const SEED = ['app:/data/takes.json', 'app:/data/podcasts-library.json', 'app:/data/mail/msg-1.bin'];
const launch = async (m: ReturnType<typeof memFs>) => { resetSeam(); resetSettled(); bindNative(phone() as never); return settleAccount(m.storage, undefined); };
const dataLeft = (m: ReturnType<typeof memFs>): string[] => [...m.files.keys()].filter((k) => SEED.includes(k));

describe('S14 the table: key x stamp x marker', () => {
  const keys: World['key'][] = ['present', 'absent', 'unreadable'];
  const stamps = ['none', 'same', 'other'] as const;
  const markers = ['none', 'pending'] as const;
  // What must happen, written out (not derived from decide): the data, the Swift stores, the stamp and the marker after one launch.
  const expected = (key: World['key'], stamp: string, marker: string) => {
    if (key === 'unreadable') return { data: true, swift: true, stamp: stamp === 'none' ? 'none' : stamp, marker, native: [] as string[] };
    if (key === 'present') {
      if (stamp === 'other') return { data: false, swift: false, stamp: 'new', marker: 'none', native: ['wipe'] };
      return { data: true, swift: true, stamp: 'new', marker: 'none', native: [] as string[] };         // none: adopt; same: kept; a pending marker is cleared, nothing wiped
    }
    if (stamp !== 'none' || marker === 'pending') return { data: false, swift: false, stamp: 'none', marker: 'none', native: ['signOut'] };
    return { data: true, swift: true, stamp: 'none', marker: 'none', native: [] as string[] };
  };
  for (const key of keys) for (const stamp of stamps) for (const marker of markers) {
    test(`key ${key} / stamp ${stamp} / marker ${marker}`, async () => {
      W = newWorld(key);
      const m = memFs(SEED);
      if (stamp === 'same') m.files.set(STAMP_FILE, new TextEncoder().encode(dig(W.value)));
      if (stamp === 'other') m.files.set(STAMP_FILE, new TextEncoder().encode('f'.repeat(64)));
      if (marker === 'pending') m.files.set(PENDING_FILE, new TextEncoder().encode('1'));
      const stampBefore = m.files.get(STAMP_FILE);
      await launch(m);
      const e = expected(key, stamp, marker);
      expect(dataLeft(m).length > 0).toBe(e.data); expect(W.swift).toBe(e.swift); expect(W.native).toEqual(e.native);
      expect(m.files.has(PENDING_FILE)).toBe(e.marker === 'pending');
      const st = m.files.get(STAMP_FILE);
      if (e.stamp === 'new') expect(new TextDecoder().decode(st)).toBe(dig(W.value));
      else if (e.stamp === 'none') expect(st).toBeUndefined();
      else expect(st).toEqual(stampBefore);
    });
  }
  test('decide() covers all 18 combinations', () => {
    let n = 0;
    for (const key of keys) for (const stamp of stamps) for (const marker of markers) { expect(typeof decide({ key, stamp, marker } as Facts)).toBe('string'); n++; }
    expect(n).toBe(18);
  });
});

describe('S15 a kill at each step of a sign-out, then a relaunch', () => {
  const steps = ['marker', 'begun', 'native', 'wipe:vectors', 'wiped'];
  for (const step of steps) {
    test(`killed after "${step}"`, async () => {
      W = newWorld('present'); const m = memFs(SEED);
      resetSeam(); resetSettled(); bindNative(phone() as never); await settleAccount(m.storage, undefined);       // the signed-in phone, adopted
      resetSettled();
      await accountSignOut(m.storage, undefined, phone() as never, { crash: (s) => { if (s === step) throw new Error('killed'); } }).catch(() => undefined);
      const keyGoneAtKill = W.key === 'absent';
      await launch(m);                                                      // the relaunch
      if (!keyGoneAtKill) {                                                 // (a) the key was still there: nothing was signed out, nothing may be wiped
        expect(dataLeft(m).length).toBe(SEED.length); expect(W.swift).toBe(true); expect(m.files.has(PENDING_FILE)).toBe(false);
      } else {                                                              // (b) the key is gone: nothing of the old account remains, shared or Swift
        expect(dataLeft(m)).toEqual([]); expect(W.swift).toBe(false); expect(m.files.has(PENDING_FILE)).toBe(false); expect(m.files.has(STAMP_FILE)).toBe(false);
      }
    });
  }
  test('killed between the key delete and the Swift wipe (finding 2): the relaunch reruns the Swift wipe', async () => {
    W = newWorld('present'); W.killInNative = true; const m = memFs(SEED);
    await launch(m); resetSettled();
    await accountSignOut(m.storage, undefined, phone() as never).catch(() => undefined);
    expect(W.key).toBe('absent'); expect(W.swift).toBe(true); expect(m.files.has(PENDING_FILE)).toBe(true);
    W.killInNative = false; W.native = [];
    await launch(m);
    expect(W.swift).toBe(false); expect(W.native).toEqual(['signOut']); expect(dataLeft(m)).toEqual([]); expect(m.files.has(PENDING_FILE)).toBe(false);
  });
  test('after the marker and before the key delete (finding 1): the relaunch wipes nothing, clears the marker, keeps the stamp', async () => {
    W = newWorld('present'); const m = memFs(SEED);
    await launch(m); resetSettled();
    await accountSignOut(m.storage, undefined, phone() as never, { crash: (s) => { if (s === 'marker') throw new Error('killed'); } }).catch(() => undefined);
    expect(m.files.has(PENDING_FILE)).toBe(true);
    W.native = []; await launch(m);
    expect(dataLeft(m).length).toBe(SEED.length); expect(W.native).toEqual([]); expect(m.files.has(PENDING_FILE)).toBe(false); expect(m.files.has(STAMP_FILE)).toBe(true);
  });
  test('a stamp is not advanced by a wipe that failed (finding 3): the old stamp stays and the next launch tries again', async () => {
    W = newWorld('present', 'test-key-B-not-real'); const m = memFs(SEED);
    m.files.set(STAMP_FILE, new TextEncoder().encode(dig('test-key-A-not-real')));
    const rm = m.storage.fs.rm; let broken = true;
    (m.storage as never as { fs: { rm: (p: string) => Promise<void> } }).fs.rm = async (p: string) => { if (broken && p === 'app:/data/takes.json') throw new Error('disk'); return rm(p); };
    const r = await launch(m);
    expect(r.failed.length).toBeGreaterThan(0); expect(new TextDecoder().decode(m.files.get(STAMP_FILE))).toBe(dig('test-key-A-not-real'));      // not relabelled as B's
    broken = false; await launch(m);
    expect(dataLeft(m)).toEqual([]); expect(new TextDecoder().decode(m.files.get(STAMP_FILE))).toBe(dig('test-key-B-not-real'));
  });
  // (The rows "wipe:vectors" and "wiped" above model a wipe that FAILED at that point, not a process kill: a kill is the rows named for the marker, the native steps and the relaunch.)
  test('round 3, 1: a native sign-out that fails or hangs does not leave the previous account\'s shared stores readable (the shared wipe starts regardless; the marker stays)', async () => {
    W = newWorld('absent'); W.swift = true; const m = memFs(SEED);
    m.files.set(STAMP_FILE, new TextEncoder().encode(dig('test-key-A-not-real'))); m.files.set(PENDING_FILE, new TextEncoder().encode('1'));
    resetSeam(); resetSettled(); bindNative({ ...phone(), later: async (r: { op: string }) => (r.op === 'signOut' ? { done: false } : phone().later(r)) } as never);
    const r = await settleAccount(m.storage, undefined);
    expect(r.failed).toContain('native'); expect(dataLeft(m)).toEqual([]);                 // the shared stores are gone
    expect(m.files.has(PENDING_FILE)).toBe(true); expect(m.files.has(STAMP_FILE)).toBe(true);   // and the retry is kept
  });
  test('round 3, 1b: the same for an account change whose native wipe fails: shared stores wiped, the old stamp stays', async () => {
    W = newWorld('present', 'test-key-B-not-real'); const m = memFs(SEED);
    m.files.set(STAMP_FILE, new TextEncoder().encode(dig('test-key-A-not-real')));
    resetSeam(); resetSettled(); bindNative({ ...phone(), later: async (r: { op: string }) => (r.op === 'wipe' ? { done: false } : phone().later(r)) } as never);
    const r = await settleAccount(m.storage, undefined);
    expect(r.failed).toContain('native'); expect(dataLeft(m)).toEqual([]); expect(new TextDecoder().decode(m.files.get(STAMP_FILE))).toBe(dig('test-key-A-not-real'));
  });
  test('round 3, 2: a late native answer does not wipe a sign-in that completed inside the window (sign-out, and the launch\'s finish)', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const later = async (r: { op: string }) => (r.op === 'signOut' ? (await gate, { done: true }) : phone().later(r));
    W = newWorld('present'); const m = memFs(SEED);
    await launch(m); resetSettled();
    const out = await accountSignOut(m.storage, undefined, { available: true, later } as never, { deadlineMs: 20 });
    expect(out.done).toBe(false);                                                      // the page was told; the native call is still out
    // a new sign-in completed inside the window: the key is present (and the new account has written its own data)
    release(); for (let i = 0; i < 10; i++) await new Promise((r) => setTimeout(r, 5));
    expect(dataLeft(m).length).toBe(SEED.length);                                      // the late done:true wiped nothing of the new account's
    // the same for the launch's finish: key absent + marker, the native call hangs past the deadline, a sign-in lands, the late answer arrives
    let release2: () => void = () => undefined; const gate2 = new Promise<void>((r) => { release2 = r; });
    W = newWorld('absent'); const m2 = memFs(SEED); m2.files.set(PENDING_FILE, new TextEncoder().encode('1'));
    resetSeam(); resetSettled(); bindNative(phone() as never);
    const r = await settleAccount(m2.storage, undefined, nativeOf(async (q) => (q.op === 'signOut' ? (await gate2, { done: true }) : phone().later(q)), 20));
    expect(r.failed).toContain('native');
    for (const k of SEED) m2.files.set(k, new TextEncoder().encode('new'));             // the shared wipe already ran (round 3, 1); the new account writes afresh
    W.key = 'present'; W.value = 'test-key-B-not-real';
    release2(); for (let i = 0; i < 10; i++) await new Promise((q) => setTimeout(q, 5));
    expect(dataLeft(m2).length).toBe(SEED.length);
  });
  test('round 4: an account change whose native wipe answers AFTER the deadline does not wipe the new account\'s data again; it only finishes the stamp', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    W = newWorld('present', 'test-key-B-not-real'); const m = memFs(SEED);
    m.files.set(STAMP_FILE, new TextEncoder().encode(dig('test-key-A-not-real')));
    resetSeam(); resetSettled(); bindNative(phone() as never);
    const r = await settleAccount(m.storage, undefined, nativeOf(async (q) => (q.op === 'wipe' ? (await gate, { done: true }) : phone().later(q)), 20));
    expect(r.failed).toContain('native'); expect(dataLeft(m)).toEqual([]);              // the shared stores were wiped at the launch
    for (const k of SEED) m.files.set(k, new TextEncoder().encode('B wrote this'));        // the new account is signed in and has written data in between
    release(); for (let i = 0; i < 10; i++) await new Promise((q) => setTimeout(q, 5));
    expect(dataLeft(m).length).toBe(SEED.length);                                          // it survives the late answer
    expect(new TextDecoder().decode(m.files.get(STAMP_FILE))).toBe(dig('test-key-B-not-real'));   // and the bookkeeping is finished
  });
  test('round 5: the launch finish\'s late answer belongs to ITS sign-out: a later sign-out\'s marker and stamp are not cleared by it', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    W = newWorld('absent'); const m = memFs(SEED);
    m.files.set(PENDING_FILE, new TextEncoder().encode('token-1')); m.files.set(STAMP_FILE, new TextEncoder().encode('stamp-1'));
    resetSeam(); resetSettled(); bindNative(phone() as never);
    const r = await settleAccount(m.storage, undefined, nativeOf(async (q) => (q.op === 'signOut' ? (await gate, { done: true }) : phone().later(q)), 20));
    expect(r.failed).toContain('native');
    // the user signed in and signed out again: a NEW sign-out's marker and stamp exist, its wipes have not run
    m.files.set(PENDING_FILE, new TextEncoder().encode('token-2')); m.files.set(STAMP_FILE, new TextEncoder().encode('stamp-2'));
    release(); for (let i = 0; i < 10; i++) await new Promise((q) => setTimeout(q, 5));
    expect(new TextDecoder().decode(m.files.get(PENDING_FILE))).toBe('token-2'); expect(new TextDecoder().decode(m.files.get(STAMP_FILE))).toBe('stamp-2');
  });
  test('round 5: and when nothing changed in between, the late answer does clear its own marker and stamp', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    W = newWorld('absent'); const m = memFs(SEED);
    m.files.set(PENDING_FILE, new TextEncoder().encode('token-1')); m.files.set(STAMP_FILE, new TextEncoder().encode('stamp-1'));
    resetSeam(); resetSettled(); bindNative(phone() as never);
    await settleAccount(m.storage, undefined, nativeOf(async (q) => (q.op === 'signOut' ? (await gate, { done: true }) : phone().later(q)), 20));
    release(); for (let i = 0; i < 10; i++) await new Promise((q) => setTimeout(q, 5));
    expect(m.files.has(PENDING_FILE)).toBe(false); expect(m.files.has(STAMP_FILE)).toBe(false);
  });
  test('the marker holds a token of 16 random bytes in hex (from getRandomValues), different for each sign-out', async () => {
    const seen: string[] = [];
    for (let i = 0; i < 2; i++) {
      W = newWorld('present'); W.hang = true; const m = memFs(SEED); await launch(m); resetSettled();
      await accountSignOut(m.storage, undefined, phone() as never, { deadlineMs: 10 });
      seen.push(new TextDecoder().decode(m.files.get(PENDING_FILE))); W.lateRelease?.();
    }
    expect(seen[0]).toMatch(/^[0-9a-f]{32}$/); expect(seen[1]).toMatch(/^[0-9a-f]{32}$/); expect(seen[0]).not.toBe(seen[1]);
  });
  test('a native sign-out that outlives its deadline keeps its continuation and the marker (finding 4)', async () => {
    W = newWorld('present'); W.hang = true; const m = memFs(SEED);
    await launch(m); resetSettled();
    const out = await accountSignOut(m.storage, undefined, phone() as never, { deadlineMs: 20 });
    expect(out.done).toBe(false); expect(m.files.has(PENDING_FILE)).toBe(true); expect(dataLeft(m).length).toBe(SEED.length);
    W.lateRelease!();                                                       // the native call answers done:true after the page was told
    for (let i = 0; i < 20; i++) await new Promise((r) => setTimeout(r, 5));
    expect(dataLeft(m)).toEqual([]); expect(m.files.has(PENDING_FILE)).toBe(false); expect(W.swift).toBe(false);
  });
  test('a failed native sign-out ("not done") removes the marker and wipes nothing; a throw keeps the marker for the next launch to judge by the key', async () => {
    W = newWorld('present'); const m = memFs(SEED);
    const n = (r: unknown) => ({ available: true, later: async () => r });
    const a = await accountSignOut(m.storage, undefined, n({ done: false }) as never);
    expect(a.done).toBe(false); expect(m.files.has(PENDING_FILE)).toBe(false); expect(dataLeft(m).length).toBe(SEED.length);
    const b = await accountSignOut(m.storage, undefined, { available: true, later: async () => { throw new Error('x'); } } as never);
    expect(b.done).toBe(false); expect(m.files.has(PENDING_FILE)).toBe(true); expect(dataLeft(m).length).toBe(SEED.length);
    resetSeam(); bindNative(phone() as never); await settleAccount(m.storage, undefined);
    expect(m.files.has(PENDING_FILE)).toBe(false); expect(dataLeft(m).length).toBe(SEED.length);                   // the key is there: cleared, nothing wiped
  });
  test('no storage: a sign-out does not start (it fails closed)', async () => {
    W = newWorld('present'); const out = await accountSignOut(undefined, undefined, phone() as never);
    expect(out.done).toBe(false); expect(W.native).toEqual([]);
  });
  test('a partial wipe after a signed-out phone says which stores and keeps the marker', async () => {
    W = newWorld('present'); const m = memFs(SEED);
    const out = await accountSignOut(m.storage, undefined, phone() as never, { wipes: [{ name: 'a', run: async () => { throw new Error('x'); } }, { name: 'b', run: async () => undefined }] });
    expect(out.done).toBe(true); expect(out.failed).toBe('a'); expect(out.message).toContain('a'); expect(m.files.has(PENDING_FILE)).toBe(true);
  });
  test('a different account B after A signs out is adopted clean: nothing of A remains (shared or Swift)', async () => {
    W = newWorld('present'); const m = memFs(SEED);
    await launch(m); resetSettled();
    expect((await accountSignOut(m.storage, undefined, phone() as never)).done).toBe(true);
    W.key = 'present'; W.value = 'test-key-B-not-real'; W.swift = false;
    await launch(m);
    expect(dataLeft(m)).toEqual([]); expect(new TextDecoder().decode(m.files.get(STAMP_FILE))).toBe(dig('test-key-B-not-real'));
  });
  test('the first launch over an install with a key, old Swift stores and NO stamp adopts and wipes nothing', async () => {
    W = newWorld('present'); const m = memFs(SEED);
    const r = await launch(m);
    expect(r.action).toBe('adopt'); expect(dataLeft(m).length).toBe(SEED.length); expect(W.swift).toBe(true); expect(W.native).toEqual([]);
  });
});

describe('S16 the paper page passes the story title with the route (the loss the deck builder named)', () => {
  test('the action, the paper buttons and the interface carry the title', async () => {
    const c = await Bun.file(new URL('./account.contract', import.meta.url)).text();
    expect(c).toContain('action playStory(route: string, title: string)'); expect(c).toContain('choirPlay(route=route, title=title)');
    const p = await Bun.file(new URL('./paper.contract', import.meta.url)).text();
    expect((p.match(/playStory\(s\.route, s\.title\)/g) ?? []).length).toBeGreaterThanOrEqual(1); expect(p).not.toContain('playStory(s.route)');
    const i = JSON.parse(await Bun.file(new URL('../exact-package/exact.json', import.meta.url)).text());
    expect(i.names.find((n: { send?: string }) => n.send === 'choirPlay').options).toEqual({ route: 'string', title: 'string' });
  });
});

describe('S17 the grants and the wipes name the same paths, exactly, in both directions', () => {
  const grantsOf = (): { read: Set<string>; write: Set<string> } => {
    const app = readFileSync(new URL('./app.ts', import.meta.url), 'utf8');
    const read = new Set<string>(), write = new Set<string>();
    for (const m of app.matchAll(/^fs\.(read|write) (app:\/[^\s`;]+)/gm)) (m[1] === 'read' ? read : write).add(m[2]);
    return { read, write };
  };
  // Not granted on purpose: app:/tmp/picked is read only (a picked file is the system's copy); the vector store and the content database are not wired to a page yet (their wipes exist for when they are).
  const WIPED_NOT_GRANTED = new Set(['app:/tmp/picked', 'app:/data/vectors', 'app:/data/content.db']);
  // Granted but not a wipe path of its own: `app:/data/exports` holds files the listener asked to export (not account data); `app:/cache` is the root the mail originals sit under (mail-*.html is removed by wipeMail).
  const GRANTED_NOT_WIPED = new Set(['app:/data/exports', 'app:/cache', 'app:/data/theme-choice.json', 'app:/data/themes']);   // the last two: the phone's theme (themes.ts), kept through a sign-out
  test('every wiped path is granted for read and write (a denied path fails silently), but the named exceptions', () => {
    const { read, write } = grantsOf();
    expect(wipedPaths().filter((p) => !WIPED_NOT_GRANTED.has(p)).filter((p) => !(read.has(p) && write.has(p)))).toEqual([]);
    for (const p of WIPED_NOT_GRANTED) expect(wipedPaths()).toContain(p);                // an exception that is no longer wiped is a stale exception
  });
  test('every granted path is wiped (exact match), but the named exceptions: no store keeps data the sign-out does not know', () => {
    const { read, write } = grantsOf();
    const granted = new Set([...read, ...write]);
    expect([...granted].filter((p) => !GRANTED_NOT_WIPED.has(p)).filter((p) => !wipedPaths().includes(p))).toEqual([]);
    for (const p of GRANTED_NOT_WIPED) expect(granted.has(p)).toBe(true);
  });
});

describe('S18 the sign-out counter (round 6)', () => {
  test('every completed sign-out carries a larger epoch, the second in a process too; a wipe that finishes late bumps it', async () => {
    W = newWorld('present'); const m = memFs(SEED);
    await launch(m); resetSettled();
    const a = await accountSignOut(m.storage, undefined, phone() as never);
    W = newWorld('present'); await launch(m); resetSettled();
    const b = await accountSignOut(m.storage, undefined, phone() as never);
    expect(a.done && b.done).toBe(true); expect(b.epoch).toBeGreaterThan(a.epoch);
    const { wipeEpoch } = await import('./account.ts');
    const before = wipeEpoch(); await wipeAll(m.storage, undefined, []);
    expect(wipeEpoch()).toBeGreaterThan(before);                                           // the late continuation runs wipeAll too: it is counted
  });
  test('the contracts pass the epoch and the sign-in state to the shared notebook', async () => {
    const c = await Bun.file(new URL('./members-notebook.contract', import.meta.url)).text();
    expect(c).toContain('notebookPageShared(nbSel, nbTick, pg.top == "notebook:", signedOutTick, au.signedIn)');
    expect(await Bun.file(new URL('./account-types.contract', import.meta.url)).text()).toContain('epoch: number');
  });
});
