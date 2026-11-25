// The launch's account settle and the host probe, under Exact's answer ownership (docs/problems/shared-settle-promise-fails-every-request-2026-11-25.md). Failure list, written first:
//  F1 the answer that starts the launch settle is let go before `reconcile` answers: a later answer still answers.
//  F2 after F1 the launch's account work still happens: a sign-out the last launch left unfinished is finished by a live answer (the let-go run's refused `keystate` is not taken as the answer).
//  F3 a settle that rejects is not kept: the next call reconciles again and answers.
//  F4 no timer is armed in the settle chain at launch (the pending-revoke retry is bounded by each request's own `exactTimeout`).
//  F5 hostMode: the answer that starts the probe is let go; another answer waiting on it gets "native" or "none", not the let-go answer's error.
//
// The model of Exact (pinned prelude.js): every step of an async function resumes under the answer that called it (`__exact_async_context`, :230-241), which AsyncLocalStorage reproduces in Bun;
// a timer armed by an answer that was let go throws (`timerOwner`, :1352-1366); letting an answer go clears its timers and rejects the fetches (here: native calls) it waits on (`__exact_forget`, :997-1002).
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { AsyncLocalStorage } from 'node:async_hooks';
import { answer } from './app.ts';
import { resetSettled, settleAccount, PENDING_FILE, STAMP_FILE } from './account.ts';
import { resetDemoLeftovers } from './demo-leftovers.ts';
import { hostMode, resetHost } from './host.ts';
import { adoptNative, resetSeam } from './api.ts';

type Call = { id: string; letGo: boolean; timers: Set<ReturnType<typeof setTimeout>>; waiting: Set<(e: Error) => void> };
const calls = new AsyncLocalStorage<Call>();
const realSet = globalThis.setTimeout; const realClear = globalThis.clearTimeout;
let armedBy: string[] = [];
let made: Call[] = [];
const newCall = (id: string): Call => { const c: Call = { id, letGo: false, timers: new Set(), waiting: new Set() }; made.push(c); return c; };
const letGo = (c: Call): void => { c.letGo = true; for (const t of c.timers) realClear(t); c.timers.clear(); for (const w of c.waiting) w(new Error('the answer was let go before this fetch')); c.waiting.clear(); };
const asCall = <T>(c: Call, f: () => Promise<T>): Promise<T> => calls.run(c, f);
beforeEach(() => {
  armedBy = [];
  globalThis.setTimeout = ((fn: (...a: unknown[]) => void, ms?: number, ...a: unknown[]) => {
    const c = calls.getStore();
    if (c) armedBy.push(c.id);
    if (c?.letGo) { throw new Error('setTimeout() is unavailable here: a timer belongs to the answer that arms it, and none is running'); }
    const t = realSet(() => { c?.timers.delete(t); fn(...a); }, ms);
    c?.timers.add(t);
    return t;
  }) as typeof setTimeout;
  resetSeam(); resetSettled(); resetHost(); resetDemoLeftovers();
});
afterEach(() => { for (const c of made) { for (const t of c.timers) realClear(t); c.timers.clear(); } made = []; globalThis.setTimeout = realSet; });      // an answer's timers end with it

/** The app module as the phone has it: a native call made by an answer that was let go is refused; one in flight when it is let go is rejected. `gate` holds `keystate` until released. */
function phone(world: { key: 'present' | 'absent'; gate?: Promise<void>; ops: string[] }) {
  return { available: true, later: (r: { op: string }) => {
    const c = calls.getStore();
    if (c?.letGo) return Promise.reject(new Error('the answer was let go before this fetch'));
    return new Promise((res, rej) => {
      c?.waiting.add(rej);
      void (async () => {
        if (r.op === 'keystate' && world.gate) await world.gate;
        if (c?.letGo) return;
        c?.waiting.delete(rej);
        world.ops.push(r.op);
        if (r.op === 'keystate') return res({ state: world.key });
        if (r.op === 'session') return res(world.key === 'present' ? { found: true, value: 'test-key-A-not-real', computerId: 'c' } : { found: false });
        if (r.op === 'signOut') { world.key = 'absent'; return res({ done: true }); }
        if (r.op === 'wipe') return res({ done: true });
        return res({ ok: true });
      })();
    });
  } };
}

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
    readdir: async (p: string) => { const k = kids(p); if (!k.length) throw new Error('ENOENT'); return [...new Set(k.map((x) => x.slice(p.length + 1).split('/')[0]))]; },
  };
  return { files, storage: { fs, sqlite: {} } as never };
}
const tick = async (n = 20) => { for (let i = 0; i < n; i++) await new Promise((r) => realSet(r, 1)); };

describe('the launch settle under answer ownership', () => {
  test('F1 the answer that starts the settle is let go before reconcile answers: later answers still answer', async () => {
    let open!: () => void; const world = { key: 'present' as const, gate: new Promise<void>((r) => { open = r; }), ops: [] as string[] };
    const native = phone(world); const m = memFs();
    const a = newCall('A');
    const first = asCall(a, () => answer('answerDismissShared' as never, ['x'] as never, undefined as never, m.storage, native as never)).then(() => 'answered', (e: Error) => e.message);
    await tick(5);
    letGo(a); open(); await first;                                                  // A is asked again with new arguments while its `keystate` is out
    const b = newCall('B');
    const r = await asCall(b, () => answer('answerDismissShared' as never, ['y'] as never, undefined as never, m.storage, native as never)) as { ok: boolean };
    expect(r.ok).toBe(true);
    const c = newCall('C');
    const pages = await asCall(c, () => answer('pagesView' as never, [0, ''] as never, undefined as never, m.storage, native as never)) as { top?: string };
    expect(typeof pages).toBe('object');
  });

  test('F2 after F1 the unfinished sign-out is still finished at this launch, by a live answer', async () => {
    let open!: () => void; const world = { key: 'absent' as 'present' | 'absent', gate: new Promise<void>((r) => { open = r; }), ops: [] as string[] };
    const native = phone(world); const m = memFs(['app:/data/takes.json', PENDING_FILE, STAMP_FILE]);
    const a = newCall('A');
    const first = asCall(a, () => answer('answerDismissShared' as never, ['x'] as never, undefined as never, m.storage, native as never)).catch(() => undefined);
    await tick(5);
    letGo(a); open(); await first;
    const b = newCall('B');
    await asCall(b, () => answer('answerDismissShared' as never, ['y'] as never, undefined as never, m.storage, native as never));
    expect(world.ops).toContain('signOut');
    expect(m.files.has('app:/data/takes.json')).toBe(false); expect(m.files.has(PENDING_FILE)).toBe(false); expect(m.files.has(STAMP_FILE)).toBe(false);
  });

  test('F3 a settle that rejects is not kept: the next call reconciles again', async () => {
    const world = { key: 'absent' as 'present' | 'absent', ops: [] as string[] };
    adoptNative(phone(world) as never, 't');
    const m = memFs(['app:/data/takes.json', PENDING_FILE]);
    let throws = true;
    const flaky = { signOut: async () => { if (throws) { throws = false; throw new Error('a bridge that threw once'); } return true; }, wipe: async () => true };
    const a = newCall('A');
    await expect(asCall(a, () => settleAccount(m.storage, undefined, flaky))).rejects.toThrow('threw once');
    const b = newCall('B');
    const r = await asCall(b, () => settleAccount(m.storage, undefined, flaky));
    expect(r.action).toBe('finish'); expect(m.files.has(PENDING_FILE)).toBe(false);
  });

  test('F4 the launch settle arms no timer of its own: the pending-revoke retry is bounded by its requests', async () => {
    const world = { key: 'present' as 'present' | 'absent', ops: [] as string[] };
    const m = memFs(); m.files.set('app:/data/pending-revoke', new TextEncoder().encode(JSON.stringify([{ id: 'ak_old', tries: 0 }])));
    const seen: { timeout?: number }[] = []; const real = globalThis.fetch;
    globalThis.fetch = (async (_u: string, init: { exactTimeout?: number }) => { seen.push({ timeout: init.exactTimeout }); return new Response('{}', { status: 200 }); }) as never;
    try {
      const a = newCall('A');
      const store = { get: (n: string) => (n === 'choir.key' ? 'test-key-A-not-real' : null), set: () => undefined, forget: () => undefined };      // the key as the app keeps it (an Exact secret)
      await asCall(a, () => answer('answerDismissShared' as never, ['x'] as never, store as never, m.storage, phone(world) as never));
    } finally { globalThis.fetch = real; }
    expect(armedBy).toEqual([]);
    expect(seen.length).toBe(1); expect(seen[0].timeout).toBeLessThanOrEqual(3000);
    expect(m.files.has('app:/data/pending-revoke')).toBe(false);
  });
});

describe('the host probe under answer ownership', () => {
  test('F5 the answer that starts the probe is let go: another answer waiting on it still gets a decision', async () => {
    const hang = { available: true, later: () => { const c = calls.getStore(); return new Promise((_res, rej) => { c?.waiting.add(rej); }); } };
    adoptNative(hang as never, 'x');
    const a = newCall('A'); const b = newCall('B');
    const first = asCall(a, () => hostMode(120, 20)).catch((e: Error) => e.message);
    await tick(2);
    const second = asCall(b, () => hostMode(120, 20)).then((m) => m, (e: Error) => `error: ${e.message}`);
    letGo(a);
    await first;
    expect(await second).toBe('none');
  });
});

describe('the settle with a live owner is unchanged', () => {
  test('two answers at launch share one reconcile (the native wipe runs once)', async () => {
    const world = { key: 'absent' as 'present' | 'absent', ops: [] as string[] };
    const native = phone(world); const m = memFs(['app:/data/takes.json', PENDING_FILE]);
    const [x, y] = [newCall('X'), newCall('Y')];
    await Promise.all([
      asCall(x, () => answer('answerDismissShared' as never, ['x'] as never, undefined as never, m.storage, native as never)),
      asCall(y, () => answer('answerDismissShared' as never, ['y'] as never, undefined as never, m.storage, native as never)),
    ]);
    expect(world.ops.filter((o) => o === 'signOut').length).toBe(1);
    expect(m.files.has(PENDING_FILE)).toBe(false);
  });
});
