// The run sheet's restore under Exact's answer ownership (docs/problems/radio-restore-empty-after-let-go-2026-11-25.md). Failure list, written first:
//  R1 the answer that first asks for the radio (a pulse) is let go while its reads are out: a later hold still restores the sheet from its file (6 ahead), never an empty sheet kept for the process.
//  R2 a refused sheet read is not an empty sheet: the radio stays not restored, the pulse says `hold`, and the next hold restores it.
//  R3 an absent sheet file IS an empty sheet: restored, and nothing more is due for it.
//  R4 the pulse makes no deck call of its own: it reads the deck state the contract hands it (`ds`), and lines in it the pump has not read make the hold due.
//  R5 no resource answer arms a timer (the restore's post and its retry timer ran as the first pulse, and Exact refused the timer).
//  R6 the run sheet store arms no retry timer after a post the route did not take; a post Exact refused (the answer let go) is no failure: no backoff.
//  R7 a sheet act sent before any hold restores first: the act is added to the sheet from the file, not to an empty one the file then replaces.
//  R8 the desk is not asked to write onto a sheet not yet read: no `write` while not restored.
//
// The model of Exact (pinned prelude.js), as settle-once.test.ts has it: AsyncLocalStorage stands for the answer each step runs as; an answer let go has its native calls rejected and its storage
// refused ("called outside an answer: it was never run", js/src/storage.js:127-137); a timer armed by a let-go answer throws.
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { AsyncLocalStorage } from 'node:async_hooks';
import { adoptNative, resetSeam } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { getRadio, queueAdd, radioHold, radioPulse, resetRadio, SHEET_FILE } from './radio-bridge.ts';
import { paperStory } from './runsheet-entries.ts';
import { RunSheetStore, routePoster, type Poster, type PostOutcome, type Storage as RunStorage } from './runsheet-store.ts';

class FakePoster implements Poster {
  answer: PostOutcome = { kind: 'notYet' };
  async post(): Promise<PostOutcome> { return this.answer; }
  async fetchSheet() { return undefined; }
}

type Call = { id: string; letGo: boolean; replied: boolean; waiting: Set<(e: Error) => void> };
const calls = new AsyncLocalStorage<Call>();
const realSet = globalThis.setTimeout;
let armedBy: string[] = [];
let n = 0;
const newCall = (id = 'c' + ++n): Call => ({ id, letGo: false, replied: false, waiting: new Set() });
const letGo = (c: Call): void => { c.letGo = true; for (const w of c.waiting) w(new Error('the answer was let go before this fetch')); c.waiting.clear(); };
const asCall = async <T>(c: Call, f: () => Promise<T>): Promise<T> => { try { return await calls.run(c, f); } finally { c.replied = true; } };
const tick = async (k = 20) => { for (let i = 0; i < k; i++) await new Promise((r) => realSet(r, 1)); };

const EMPTY_DS = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };

/** The app module: `deck` reads are counted, and gated while `gate` is set; a call made by (or in flight for) an answer let go is rejected. */
function phone(world: { gate?: Promise<void>; ops: string[]; deck?: unknown }) {
  return { available: true, later: (r: { op: string; name?: string }) => {
    const c = calls.getStore();
    if (c?.letGo) return Promise.reject(new Error('the answer was let go before this fetch'));
    return new Promise((res, rej) => {
      c?.waiting.add(rej);
      void (async () => {
        if (r.op === 'deck' && world.gate) await world.gate;
        if (c?.letGo) return;
        c?.waiting.delete(rej);
        world.ops.push(r.op === 'deck' ? 'deck:' + r.name : r.op);
        if (r.op === 'radioFacts') return res({ at: 1_700_000_000_000, ready: false, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [] });
        if (r.op === 'deck') return res(world.deck ?? EMPTY_DS);
        return res({ ok: true });
      })();
    });
  } };
}

/** app:/data as Exact gives it to an answer: an answer let go, or one that has replied, is refused. `refuse` refuses the next n sheet reads the same way (the answer's storage wait failed). */
function disk(seed: Record<string, string> = {}) {
  const files = new Map<string, Uint8Array>(Object.entries(seed).map(([k, v]) => [k, new TextEncoder().encode(v)]));
  const st = { refuse: 0, reads: 0 };
  const guard = (): void => {
    const c = calls.getStore();
    if (c && (c.letGo || c.replied)) throw Object.assign(new Error('storage.readFile() called outside an answer: it was never run. Start storage inside an answer'), { kind: 'Unavailable', code: 'failed' });
  };
  const fs = {
    readFile: async (p: string) => {
      guard();
      if (p === SHEET_FILE) { st.reads += 1; if (st.refuse > 0) { st.refuse -= 1; throw Object.assign(new Error("the answer's storage wait failed"), { kind: 'Unavailable', code: 'failed' }); } }
      const b = files.get(p); if (!b) throw Object.assign(new Error('No such file or directory (os error 2)'), { kind: 'Unavailable', code: 'ENOENT' }); return b;
    },
    atomicWriteFile: async (p: string, b: Uint8Array) => { guard(); files.set(p, b); },
    rm: async (p: string) => { guard(); files.delete(p); },
  };
  return { files, st, storage: { fs } as never };
}

/** A sheet file with six entries ahead and their six place ops pending (never posted). */
async function sixAhead(): Promise<string> {
  let text = '';
  const mem: RunStorage = { read: async () => text || undefined, write: async (t) => { text = t; }, remove: async () => { text = ''; } };
  const s = new RunSheetStore({ storage: mem });
  for (let i = 1; i <= 6; i++) s.place(paperStory('world/story-' + i, 'Story ' + i), 'end', { by: 'listener' });
  await s.save();
  return text;
}

beforeEach(() => {
  armedBy = [];
  globalThis.setTimeout = ((fn: (...a: unknown[]) => void, ms?: number, ...a: unknown[]) => {
    const c = calls.getStore();
    if (c) armedBy.push(c.id);
    if (c?.letGo || c?.replied) throw new Error('setTimeout() is unavailable here: a timer belongs to the answer that arms it, and none is running');
    return realSet(fn, ms, ...a);
  }) as typeof setTimeout;
  resetSeam(); resetRadio(); forgetOnboarding();
});
afterEach(() => { globalThis.setTimeout = realSet; });

const ahead = async (storage: never, native: never): Promise<number> => (await asCall(newCall(), () => getRadio(storage, native, false))).rt.store.ahead().length;

describe('the run sheet is restored by a mutation, and kept as a value', () => {
  test('R1 the first pulse is let go while its reads are out: the hold still restores the six entries', async () => {
    let open!: () => void; const world = { gate: new Promise<void>((r) => { open = r; }), ops: [] as string[] };
    const native = phone(world); adoptNative(native as never, 'test');
    const d = disk({ [SHEET_FILE]: await sixAhead() });
    const a = newCall('pulse-A');
    const first = asCall(a, () => radioPulse(d.storage, native as never, EMPTY_DS)).catch(() => undefined);
    await tick(5);
    letGo(a); open(); await first;                                                         // A is asked again (a native touch) while its reads are out
    await asCall(newCall('hold-B'), () => radioHold(d.storage, native as never));
    expect(await ahead(d.storage, native as never)).toBe(6);
  });

  test('R2 a refused sheet read leaves the radio not restored, the pulse says hold, and the next hold restores', async () => {
    const native = phone({ ops: [] }); adoptNative(native as never, 'test');
    const d = disk({ [SHEET_FILE]: await sixAhead() }); d.st.refuse = 1;
    await asCall(newCall(), () => radioHold(d.storage, native as never));
    expect(await ahead(d.storage, native as never)).toBe(0);
    const p = await asCall(newCall(), () => radioPulse(d.storage, native as never, EMPTY_DS));
    expect(p.hold).toBe(true);
    await asCall(newCall(), () => radioHold(d.storage, native as never));
    expect(await ahead(d.storage, native as never)).toBe(6);
    expect(d.st.reads).toBe(2);
  });

  test('R3 an absent sheet file is an empty sheet: restored once, and the hold is no longer due for it', async () => {
    const native = phone({ ops: [] }); adoptNative(native as never, 'test');
    const d = disk();
    expect((await asCall(newCall(), () => radioPulse(d.storage, native as never, EMPTY_DS))).hold).toBe(true);
    await asCall(newCall(), () => radioHold(d.storage, native as never));
    expect((await asCall(newCall(), () => radioPulse(d.storage, native as never, EMPTY_DS))).hold).toBe(false);
    await asCall(newCall(), () => radioHold(d.storage, native as never));
    expect(d.st.reads).toBe(1);
  });
});

describe('the pulse reads the deck the contract read', () => {
  test('R4 no deck call in the pulse; a journal line in `ds` the pump has not read makes the hold due', async () => {
    const world = { ops: [] as string[] }; const native = phone(world); adoptNative(native as never, 'test');
    const d = disk();
    await asCall(newCall(), () => radioHold(d.storage, native as never));
    world.ops.length = 0;
    const quiet = await asCall(newCall(), () => radioPulse(d.storage, native as never, EMPTY_DS));
    expect(quiet.hold).toBe(false);
    const ds = { ...EMPTY_DS, seq: 1, journal: [{ seq: 1, type: 'remote', item: '', at: '2026-11-25T00:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: 'next', reason: '' }] };
    const due = await asCall(newCall(), () => radioPulse(d.storage, native as never, ds));
    expect(due.hold).toBe(true);
    expect(world.ops.filter((o) => o.startsWith('deck:'))).toEqual([]);
  });
});

describe('no timer from an answer that may be let go', () => {
  test('R5 the pulses and the screens\' reads arm no timer, with ops pending in the restored sheet', async () => {
    const native = phone({ ops: [] }); adoptNative(native as never, 'test');
    const d = disk({ [SHEET_FILE]: await sixAhead() });
    await asCall(newCall('pulse-1'), () => radioPulse(d.storage, native as never, EMPTY_DS));
    await asCall(newCall('hold'), () => radioHold(d.storage, native as never));
    await tick(5);
    await asCall(newCall('pulse-2'), () => radioPulse(d.storage, native as never, EMPTY_DS));
    await tick(5);
    expect(armedBy.filter((id) => id.startsWith('pulse'))).toEqual([]);
    expect(armedBy.filter((id) => id === 'hold')).toEqual([]);
  });

  test('R6 a post the route did not take arms no retry timer; a post Exact refused (let go) is not a failure', async () => {
    const at = { t: 1_700_000_000_000 };
    const p = new FakePoster(); p.answer = { kind: 'offline' };
    const s = new RunSheetStore({ poster: p, now: () => new Date(at.t) });
    const before = armedBy.length;
    const c = newCall('w');
    await asCall(c, async () => { s.place(paperStory('a', 'A'), 'end', { by: 'listener' }); await s.flushTask; });
    expect(armedBy.length).toBe(before);                                                   // no setTimeout: the pulse calls for the next post
    expect(s.canPost).toBe(false);                                                         // offline: it backs off
    const refused = routePoster(async () => { throw new Error('network: the answer was let go before this fetch'); });
    const s2 = new RunSheetStore({ poster: refused, now: () => new Date(at.t) });
    s2.place(paperStory('b', 'B'), 'end', { by: 'listener' }); await s2.flushTask;
    expect(s2.pending.length).toBe(1);
    expect(s2.canPost).toBe(true);                                                         // no backoff: the next pulse calls for the post at once
  });
});

describe('the sheet is read before it is changed or written to', () => {
  test('R7 a sheet act sent before any hold lands on the sheet from the file', async () => {
    const native = phone({ ops: [] }); adoptNative(native as never, 'test');
    const d = disk({ [SHEET_FILE]: await sixAhead() });
    await asCall(newCall(), () => queueAdd(d.storage, native as never, 'end', 'story', 'world/story-7', 'Story 7', ''));
    expect(await ahead(d.storage, native as never)).toBe(7);
  });

  test('R8 no write is due while the sheet is not restored', async () => {
    const native = phone({ ops: [] }); adoptNative(native as never, 'test');
    const d = disk({ [SHEET_FILE]: await sixAhead() }); d.st.refuse = 1;
    await asCall(newCall(), () => radioHold(d.storage, native as never));
    const p = await asCall(newCall(), () => radioPulse(d.storage, native as never, EMPTY_DS));
    expect(p.write).toBe(false);
  });
});
