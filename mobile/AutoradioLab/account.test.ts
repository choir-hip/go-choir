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
//  S11 a demo sign-out answers done false BEFORE any native call, marker or wipe (the real account is not the demo's to end).
//  S12 the drift guard runs the real wipes against a recording storage: every `app:/` literal the modules name is among the recorded removals (or under one),
//      and the native mail key is forgotten.   S13 the clips remembered in memory are forgotten at sign-out.
//  S10 a different account's key at launch (the stamp) wipes the old data first; the same account, or a phone with no key, keeps it.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { accountSignOut, WIPES, wipeAll, wipedPaths, settleAccount, resetSettled, PENDING_FILE, STAMP_FILE } from './account.ts';
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
    const r = await accountSignOut({ demo: false }, storage, undefined, n as never, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
    expect(order).toEqual(['native:signOut', 'wipe:a']);
    expect(r).toEqual({ done: true, failed: '', message: '' });
  });
  test('S2 a native sign-out that fails wipes nothing shared and leaves no marker', async () => {
    const order: string[] = []; const m = memFs();
    const n = { available: true, later: async () => { throw new Error('boom'); } };
    const r = await accountSignOut({ demo: false }, m.storage, undefined, n as never, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
    expect(order).toEqual([]); expect(r.done).toBe(false); expect(r.message).toContain('Sign-out did not finish'); expect(m.files.has(PENDING_FILE)).toBe(false);
  });
  test('S2 a native answer that says not done wipes nothing shared', async () => {
    const r = await accountSignOut({ demo: false }, memFs().storage, undefined, native(false) as never, { wipes: [{ name: 'a', run: async () => { throw new Error('must not run'); } }] });
    expect(r.done).toBe(false);
  });
  test('S7 no native module: refuses and wipes nothing', async () => {
    const order: string[] = [];
    for (const n of [null, undefined, { available: false, later: async () => ({}) }]) {
      const r = await accountSignOut({ demo: false }, memFs().storage, undefined, n as never, { wipes: [{ name: 'a', run: async () => { order.push('wipe:a'); } }] });
      expect(r.done).toBe(false);
    }
    expect(order).toEqual([]);
  });
  test('S7 a native call that never answers is a deadline: done false, nothing wiped, no marker', async () => {
    const m = memFs(['app:/data/takes.json']);
    const n = { available: true, later: () => new Promise(() => undefined) };
    const r = await accountSignOut({ demo: false }, m.storage, undefined, n as never, { deadlineMs: 20 });
    expect(r.done).toBe(false); expect(m.files.has('app:/data/takes.json')).toBe(true); expect(m.files.has(PENDING_FILE)).toBe(false);
  });
  test('S3 a throwing wipe does not stop the rest and is named; the marker stays for the next launch', async () => {
    const order: string[] = []; const m = memFs();
    const r = await accountSignOut({ demo: false }, m.storage, undefined, native() as never, { wipes: [
      { name: 'first', run: async () => { throw new Error('secret words of the listener'); } },
      { name: 'second', run: async () => { order.push('second'); } },
    ] });
    expect(order).toEqual(['second']); expect(r.done).toBe(true); expect(r.failed).toBe('first'); expect(JSON.stringify(r)).not.toContain('secret words');
    expect(m.files.has(PENDING_FILE)).toBe(true);
  });
  test('S6 the marker is written BEFORE the native call and gone after a clean sign-out', async () => {
    const m = memFs(); let during = false;
    const n = { available: true, later: async () => { during = m.files.has(PENDING_FILE); return { done: true }; } };
    await accountSignOut({ demo: false }, m.storage, undefined, n as never, { wipes: [] });
    expect(during).toBe(true); expect(m.files.has(PENDING_FILE)).toBe(false);
  });
  test('S6 killed after the native call: the next launch finishes the wipes before it answers', async () => {
    const m = memFs([PENDING_FILE, 'app:/data/takes.json', 'app:/data/mail/rows-inbox.bin', 'app:/data/podcasts-library.json']);
    resetSettled(); bindNative(undefined);
    await settleAccount(m.storage, undefined);
    expect([...m.files.keys()]).toEqual([]);
  });
});

describe('the wipes', () => {
  test('wipeAll runs every registered wipe and returns the failed names', async () => {
    const ran: string[] = [];
    const failed = await wipeAll(undefined, undefined, [{ name: 'x', run: async () => { ran.push('x'); } }, { name: 'y', run: async () => { throw new Error('no'); } }]);
    expect(ran).toEqual(['x']); expect(failed).toEqual(['y']);
  });
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
    const unknown = found.filter((p) => !roots.includes(p) && !covered(p) && p !== 'app:/cache/mail-' && !p.startsWith('app:/cache/mail-'));
    expect(unknown).toEqual([]);
  });
  test('S8 the real wipes leave an in-memory file system empty (seeded at every path the modules name, mail files and the stamp included)', async () => {
    const seed = [...allPaths().filter((p) => p.split('/').length > 2 && p !== PENDING_FILE).flatMap((p) => (p.endsWith('/') ? [] : [p, `${p}/child.bin`])), 'app:/data/mail/msg-1.bin', 'app:/cache/mail-1.html', STAMP_FILE];
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
    const r = await accountSignOut({ demo: false }, m.storage, undefined, reader as never);
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

describe('S10 the account stamp', () => {
  const sess = (key: string | null) => bindNative({ available: true, later: async () => (key === null ? { found: false } : { found: true, value: key, computerId: '' }) } as never);
  test('the first launch adopts the account; the same account keeps its data; another one wipes it first; no key wipes nothing', async () => {
    const m = memFs(['app:/data/takes.json']);
    resetSeam(); resetSettled(); sess('test-key-A-not-real'); await settleAccount(m.storage, undefined);
    expect(m.files.has(STAMP_FILE)).toBe(true); expect(m.files.has('app:/data/takes.json')).toBe(true);
    resetSettled(); await settleAccount(m.storage, undefined); expect(m.files.has('app:/data/takes.json')).toBe(true);
    resetSettled(); sess(null); await settleAccount(m.storage, undefined); expect(m.files.has('app:/data/takes.json')).toBe(true);
    resetSettled(); sess('test-key-B-not-real'); await settleAccount(m.storage, undefined);
    expect(m.files.has('app:/data/takes.json')).toBe(false); expect(m.files.has(STAMP_FILE)).toBe(true);
  });
});

describe('S11 a demo sign-out', () => {
  test('answers done false before any native call, marker or wipe, and says why', async () => {
    const log: string[] = []; const m = memFs(['app:/data/takes.json']);
    const n = { available: true, later: async (r: { op: string }) => { log.push(r.op); return { done: true }; } };
    const r = await accountSignOut({ demo: true }, m.storage, undefined, n as never, { wipes: [{ name: 'a', run: async () => { log.push('wipe'); } }] });
    expect(log).toEqual([]); expect(r.done).toBe(false); expect(r.message).toContain('Demo'); expect(m.files.has('app:/data/takes.json')).toBe(true); expect(m.files.has(PENDING_FILE)).toBe(false);
  });
});

describe('S12 the drift guard against what the wipes really remove', () => {
  test('every app:/ literal in the shared modules is removed (or sits under a removed path) by the real wipes; the mail key is forgotten', async () => {
    resetSeam(); resetSettled();
    const removed: string[] = []; const ops: string[] = [];
    bindNative({ available: true, later: async (r: { op: string }) => { ops.push(r.op); return { ok: true }; } } as never);
    const m = memFs();
    const storage = { fs: { ...m.storage.fs, rm: async (p: string) => { removed.push(p); } }, sqlite: {} } as never;
    const failed = await wipeAll(storage, undefined, WIPES);
    expect(failed).toEqual([]);
    const dir = new URL('.', import.meta.url).pathname;
    const literals = new Set<string>();
    for (const f of readdirSync(dir).filter((x) => x.endsWith('.ts') && !x.endsWith('.test.ts') && !x.endsWith('.d.ts') && x !== 'app.ts')) {
      for (const mm of readFileSync(dir + f, 'utf8').matchAll(/['"`](app:\/(?:data|cache|tmp)\/[^'"`$]*)/g)) literals.add(mm[1].replace(/\/$/, ''));
    }
    expect(literals.size).toBeGreaterThan(10);
    const under = (p: string) => removed.some((r) => p === r || p.startsWith(r + '/') || p.startsWith(r + '-'));
    // PENDING_FILE is removed by accountSignOut itself (after a clean wipe), not by the wipes.
    const missing = [...literals].filter((p) => !under(p) && p !== PENDING_FILE && !p.startsWith('app:/cache/mail-'));
    expect(missing).toEqual([]);
    expect(removed).toContain(STAMP_FILE);
    expect(ops).toContain('mailForget');
  });
});

describe('S13 clips in memory', () => {
  test('a sign-out forgets the clips a page asked for', async () => {
    globalThis.fetch = (async () => new Response(JSON.stringify({ clips: [{ clip_id: 'c1', audio_url: 'https://a.example/a.mp3', feed_url: 'https://f.example/x', start: 1, end: 9 }] }), { status: 200 })) as never;
    await clipAct('play', 'c1');
    expect(clipsRemembered()).toBe(1);
    await wipeAll(undefined, undefined, WIPES);
    expect(clipsRemembered()).toBe(0);
  });
});
