// The account generation shared code stamps on what it pushes to native (radioSet, editionMirror, pageText) and what native does with it (AutoradioKit SharedGeneration). The failure list, written first
// (the second review of the account-generation commits, D1 to D4):
//  G1 (D2) a JS context that boots BELOW the generation native last saw (a reload with the process alive: the native module instance lives per session, the JS `generation` starts at 0) announces its
//     generation before its first call, native adopts it even though it lowers the newest, and its pushes are accepted: not silently refused until the process dies.
//  G2 (D1) the wipe ends by announcing a generation newer than anything computed before it: a pulse that began after beginSignOut (it reads the announced generation, the store still holds the old
//     account's sheet) is refused when it lands after the wipe, and a push computed after the wipe (the public paper, signed out) is accepted.
//  G3 (D3) a radioSet native refused (ok: false) is not recorded as sent: the next pulse sends it again with a fresh stamp, once per pulse; an accepted one is not sent again.
//  G4 (D3) editionMirror returns native's refusal, the mirror is not marked pushed, and the stamp the call carries is the one it checked.
import { beforeEach, describe, expect, test } from 'bun:test';
import { setAnnounceDeadlineForTest, abortSignOut, adoptNative, accountGeneration, beginSignOut, nextGeneration, nativeLater, resetBootForTest, resetSeam } from './api.ts';
import { wipeAll } from './account.ts';
import { sealingBlocked } from './api.ts';
import { radioPulse, wipeRadio, getRadio, SHEET_FILE } from './radio-bridge.ts';
import { nativeDevice, forgetPicks, picksPage, type Device } from './edition-picks.ts';

type Call = { op: string; gen?: number; boot?: boolean; [k: string]: unknown };
/** A native that keeps SharedGeneration's rule: a boot is adopted even when it lowers the newest, a push below the newest is refused, an announcement raises it. */
function fakeNative(initialNewest = 0) {
  const s = { newest: initialNewest, calls: [] as Call[], pushed: [] as Call[], refuse: 0 };
  const later = async (r: Call): Promise<unknown> => {
    s.calls.push(r);
    if (r.op === 'generation') { if (r.boot === true) s.newest = r.gen ?? 0; else s.newest = Math.max(s.newest, r.gen ?? 0); return { ok: true }; }
    if (r.op === 'radioSet' || r.op === 'editionMirror') {
      if (s.refuse > 0) { s.refuse -= 1; return { ok: false }; }
      const gen = r.gen ?? -1;
      if (gen < s.newest) return { ok: false };
      s.newest = gen; s.pushed.push(r); return { ok: true };
    }
    if (r.op === 'radioFacts') return { at: 1_700_000_000_000, ready: false, inbox: [], position: { block: 0, seconds: 0 }, foreground: true };
    if (r.op === 'deck') return { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };
    return {};
  };
  return { s, native: { available: true, later } };
}
beforeEach(async () => { resetSeam(); resetBootForTest(); forgetPicks(); await wipeRadio(undefined); });

describe('G1 a boot below the newest', () => {
  test('the first native call of a JS context announces the generation as a boot, before anything else', async () => {
    const f = fakeNative(1000);                                                    // native saw generation 1000 in an earlier JS context
    adoptNative(f.native as never, 'test');
    const later = nativeLater()!;
    await later({ op: 'editionMirror', topics: ['x'], gen: accountGeneration() });
    expect(f.s.calls[0]).toMatchObject({ op: 'generation', boot: true, gen: accountGeneration() });
    expect(f.s.pushed.length).toBe(1);                                            // not refused for being below 1000
    await later({ op: 'editionMirror', topics: ['y'], gen: accountGeneration() });
    expect(f.s.calls.filter((c) => c.op === 'generation').length).toBe(1);        // once per JS context
  });
});

describe('G2 the wipe closes the window', () => {
  test('a pulse that began after the sign-out and lands after the wipe is refused; a push after the wipe is accepted', async () => {
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    await nativeLater()!({ op: 'noop' });
    let release: () => void = () => undefined; const hold = new Promise<void>((r) => { release = r; });
    const slow = { available: true, later: async (r: Call) => { if (r.op === 'radioSet' && r.snapshot !== undefined && f.s.pushed.length === 0) { const gen = r.gen ?? -1; await hold; return f.native.later({ ...r, gen }); } return f.native.later(r); } };
    adoptNative(slow as never, 'test');
    nextGeneration();                                                              // beginSignOut: the generation moved, the old account's sheet is still in the store
    const pulse = radioPulse(undefined, slow as never);
    for (let i = 0; i < 50 && f.s.calls.filter((c) => c.op === 'radioSet').length === 0; i++) await new Promise((r) => setTimeout(r, 5));
    await wipeAll(undefined, undefined, []);                                       // the account's stores are wiped; its last act is the announcement
    release(); await pulse;
    expect(f.s.pushed.filter((c) => c.op === 'radioSet').length).toBe(0);         // the pulse of the old account's queue is refused
    adoptNative(f.native as never, 'test');
    await radioPulse(undefined, f.native as never);                                // signed out: the public paper still plays
    expect(f.s.pushed.filter((c) => c.op === 'radioSet').length).toBe(1);
  });
  test('the wipe announces a generation newer than the one it started under, as its last native call', async () => {
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    const before = accountGeneration();
    await wipeAll(undefined, undefined, []);
    const last = f.s.calls.at(-1) as Call;
    expect(last).toMatchObject({ op: 'generation' }); expect(last.boot).not.toBe(true); expect(last.gen as number).toBeGreaterThan(before);
    expect(last.gen).toBe(accountGeneration());
  });
});

describe('G3 a refused radioSet is sent again, once per pulse', () => {
  test('ok:false is not recorded as sent', async () => {
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    await getRadio(undefined, f.native as never);
    f.s.refuse = 2;
    await radioPulse(undefined, f.native as never); await radioPulse(undefined, f.native as never);
    expect(f.s.calls.filter((c) => c.op === 'radioSet').length).toBe(2);           // refused twice: sent again each pulse, with the stamp of that pulse
    await radioPulse(undefined, f.native as never);
    expect(f.s.pushed.filter((c) => c.op === 'radioSet').length).toBe(1);
    await radioPulse(undefined, f.native as never);
    expect(f.s.calls.filter((c) => c.op === 'radioSet').length).toBe(3);           // accepted: not sent again while nothing changed
  });
});

describe('G4 editionMirror', () => {
  test('returns the refusal and carries the generation it checked', async () => {
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    const dev = nativeDevice()!;
    const g = accountGeneration();
    f.s.refuse = 1;
    expect(await dev.mirror(['space'], g)).toBe(false);
    expect(await dev.mirror(['space'], g)).not.toBe(false);
    expect(f.s.calls.filter((c) => c.op === 'editionMirror').every((c) => c.gen === g)).toBe(true);
  });
});

describe('G4b a refused copy is sent again at the next open', () => {
  test('mirror refused once: the picks page asks again next time and the second copy is the one native keeps', async () => {
    const sent: { topics: string[]; gen: number }[] = []; let refuse = true;
    const dev: Device = { mirror: async (topics, gen) => { sent.push({ topics, gen }); if (refuse) { refuse = false; return false; } return true; }, forget: async () => undefined, heard: async () => [], said: async () => undefined };
    const env = { device: dev, live: async () => [] as string[] };
    await picksPage(undefined, {}, env);                                           // opens: first copy, refused
    await picksPage(undefined, {}, env);                                           // opens again: sent again (once per open, no storm)
    await picksPage(undefined, {}, env);                                           // accepted: not sent again
    expect(sent.length).toBe(2); expect(sent.every((c) => c.gen === accountGeneration())).toBe(true);
  });
});

describe('G5 the stamp is the generation the radio was made under (seat 2, item 1)', () => {
  test('a pulse begun after the sign-out began carries the old radio\'s generation and is refused; if the sign-out is given up the radio is still this account\'s and is taken again', async () => {
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    const r = await getRadio(undefined, f.native as never);
    const made = accountGeneration();
    await beginSignOut();                                                          // the generation moves; native was told (the signOut call announces it)
    f.s.newest = accountGeneration();
    expect(accountGeneration()).toBeGreaterThan(made);
    await radioPulse(undefined, f.native as never);                                // begun AFTER the move: it reads the new generation, but this radio is the old account's
    expect(f.s.calls.filter((c) => c.op === 'radioSet').map((c) => c.gen)).toEqual([made]);
    expect(f.s.pushed.length).toBe(0);
    abortSignOut();                                                                // given up: nothing was wiped, the radio is the same account's
    await radioPulse(undefined, f.native as never);
    expect(f.s.pushed.filter((c) => c.op === 'radioSet').map((c) => c.gen)).toEqual([accountGeneration()]);
    expect(r.gen).toBe(accountGeneration());
  });
  test('a refused push is said in the log', async () => {
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    await getRadio(undefined, f.native as never); f.s.refuse = 1;
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    try { await radioPulse(undefined, f.native as never); } finally { console.log = log; }
    expect(logs.some((l) => l.includes('native refused') && l.includes('radioSet'))).toBe(true);
  });
});

// The third review round (gen-review): the windows that remained.
//  G6 the block is set before beginSignOut's awaits: while the kept session is being read the radio already keeps its own stamp.
//  G7 a radio is never restored or restamped across a wipe: the sheet file is removed even when the wipe throws, a removal that failed is tried again and nothing is restored from it, and a sign-out that
//     was given up after a THROW (unknown whether the key and the native stores went) does not hand the old radio the new generation.
//  G8 the wipe's announcement has a deadline and says so when it fails; the sign-out still completes.
describe('G6 the window while the kept session is read', () => {
  test('sealingBlocked is true and the radio keeps its stamp while beginSignOut is still reading the key', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const f = fakeNative(0);
    const held = { available: true, later: async (r: Call) => { if (r.op === 'session') { await gate; return { found: false }; } return f.native.later(r); } };
    adoptNative(held as never, 'test');
    const r = await getRadio(undefined, held as never); const made = r.gen;
    const begun = beginSignOut();                                                  // not awaited: it is inside keptSession
    await new Promise((x) => setTimeout(x, 20));
    expect(sealingBlocked()).toBe(true);
    await radioPulse(undefined, held as never);
    expect(f.s.calls.filter((c) => c.op === 'radioSet').map((c) => c.gen)).toEqual([made]);      // not the new generation
    release(); await begun;
  });
});

describe('G7 never restored or restamped across a wipe', () => {
  const store = (files: Map<string, Uint8Array>, rmFails: { n: number }) => ({
    fs: {
      readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
      atomicWriteFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
      rm: async (p: string) => { if (rmFails.n > 0) { rmFails.n -= 1; throw new Error('EIO'); } files.delete(p); },
      stat: async (p: string) => { if (!files.has(p)) throw new Error('ENOENT'); return {}; },
    },
  });
  const oldSheet = JSON.stringify({ sheet: { version: 3, position_seconds: 0, entries: [{ id: 'old-1', kind: 'story', placed_by: 'desk', state: 'ahead', title: 'Old account', text: 'W.', est_seconds: 5, revision: 0 }], applied_ops: [], master_version: 0, sheet_epoch: 0 }, pending: [], epoch: 0 });
  test('the sheet file is removed even when the wipe throws', async () => {
    const files = new Map<string, Uint8Array>([[SHEET_FILE, new TextEncoder().encode(oldSheet)]]);
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    const s = store(files, { n: 0 }) as never;
    const r = await getRadio(s, f.native as never);
    expect(r.rt.store.entry('old-1')).toBeDefined();
    r.rt.signOut = async () => { throw new Error('the wipe threw'); };
    await wipeRadio(s).catch(() => undefined);
    expect(files.has(SHEET_FILE)).toBe(false);
  });
  test('a removal that failed is tried again and the next radio starts empty', async () => {
    const files = new Map<string, Uint8Array>([[SHEET_FILE, new TextEncoder().encode(oldSheet)]]);
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    const s = store(files, { n: 2 }) as never;      // the store's own removal and the bridge's both fail
    await getRadio(s, f.native as never);
    await wipeRadio(s);                                                             // the removal fails once: the file is still there
    expect(files.has(SHEET_FILE)).toBe(true);
    const next = await getRadio(s, f.native as never);
    expect(next.rt.store.entry('old-1')).toBeUndefined();                           // not restored
    expect(files.has(SHEET_FILE)).toBe(false);                                      // and removed again
  });
  test('a sign-out given up after a throw does not give the old radio the new generation', async () => {
    const f = fakeNative(0); adoptNative(f.native as never, 'test');
    const r = await getRadio(undefined, f.native as never); const made = r.gen;
    await beginSignOut(); f.s.newest = accountGeneration();
    abortSignOut(false);                                                            // native threw: the key may be gone and its stores wiped
    await radioPulse(undefined, f.native as never);
    expect(r.gen).toBe(made); expect(f.s.pushed.length).toBe(0);
  });
});

describe('G8 the wipe\'s announcement', () => {
  test('a native that never answers it does not hang the wipe, and the failure is in the log', async () => {
    const never = { available: true, later: async (r: Call) => (r.op === 'generation' ? new Promise(() => undefined) : {}) };
    adoptNative(never as never, 'test'); setAnnounceDeadlineForTest(40);
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    try { const t0 = Date.now(); await wipeAll(undefined, undefined, []); expect(Date.now() - t0).toBeLessThan(1500); } finally { console.log = log; setAnnounceDeadlineForTest(undefined); }
    expect(logs.some((l) => l.includes('generation floor') && l.includes('not announced'))).toBe(true);
  });
  test('a native that throws on it is said too', async () => {
    const bad = { available: true, later: async (r: Call) => { if (r.op === 'generation' && r.boot !== true) throw new Error('no'); return {}; } };
    adoptNative(bad as never, 'test');
    const logs: string[] = []; const log = console.log; console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
    try { await wipeAll(undefined, undefined, []); } finally { console.log = log; }
    expect(logs.some((l) => l.includes('not announced'))).toBe(true);
  });
});
