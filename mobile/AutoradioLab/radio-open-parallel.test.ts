// The radio's open on the shared-code side (first-open lane, fix 4). Failure list, written before the code:
//  E1 the front list is asked at once, beside the work that is independent of it: with the run sheet file slow to read, the request is already out (it used to start after the sheet was restored, the pump and the first-run read).
//  E2 with the deck's state slow (the pump), the request is already out as well.
//  E4 a hold that is not about the start asks nothing.
//  E3 the opening still comes from that list, once: one request, the opening placed and the deck handed it; a request that fails still opens the radio (as before: from no front page, after 2 s).
import { beforeEach, describe, expect, test } from 'bun:test';
import { adoptNative } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { radioHold, radioPulse, wipeRadio, SHEET_FILE } from './radio-bridge.ts';
import real from '../fixtures/paper/front-live-2026-11-18.json';

const calls: { op: string; name?: string }[] = [];
let at = 1_700_000_000_000;
let isReady = true;
let deckGate: Promise<void> | null = null; let sheetGate: Promise<void> | null = null; let fronts = 0; let failFront = false;
const native = {
  available: true,
  later: async (r: { op: string; name?: string; [k: string]: unknown }) => {
    calls.push({ op: r.op, name: r.name });
    if (r.op === 'radioFacts') return { at, ready: isReady, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, inbox: [] };
    if (r.op === 'deck') { if (deckGate && r.name === 'deckState') await deckGate; return { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] }; }
    return {};
  },
};
const storage = { fs: {
  readFile: async (p: string) => { if (p === SHEET_FILE && sheetGate) await sheetGate; throw Object.assign(new Error('no'), { code: 'ENOENT' }); },
  atomicWriteFile: async () => undefined, rm: async () => undefined,
} } as never;
const tick = (ms = 30): Promise<void> => new Promise((r) => setTimeout(r, ms));
beforeEach(async () => {
  forgetOnboarding(); await wipeRadio(undefined); calls.length = 0; deckGate = null; sheetGate = null; isReady = true; fronts = 0; failFront = false; at = 1_700_000_000_000;
  adoptNative(native as never, 'test');
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    if (String(input).includes('publications/front')) { fronts++; if (failFront) throw new Error('offline'); return new Response(JSON.stringify(real), { status: 200 }); }
    return new Response('{}', { status: 404 });
  }) as typeof fetch;
});

describe('the front list is asked beside the work that does not need it', () => {
  test('E1 with the sheet file slow to read the request is already out', async () => {
    let release: () => void = () => undefined; sheetGate = new Promise<void>((r) => { release = r; });
    const hold = radioHold(storage, native as never);
    await tick();
    expect(fronts).toBe(1);                                   // out while the sheet file is still being read
    release(); await hold;
  });
  test('E2 with the deck slow to answer (the pump) the request is already out', async () => {
    await radioPulse(storage, native as never);                // the radio is made and restored
    let release: () => void = () => undefined; deckGate = new Promise<void>((r) => { release = r; });
    const hold = radioHold(storage, native as never);
    await tick();
    expect(fronts).toBe(1);
    release(); await hold;
  });
  test('E4 a hold that is not about the start (the first run is not over) asks nothing', async () => {
    isReady = false;
    await radioPulse(storage, native as never);
    await radioHold(storage, native as never);
    expect(fronts).toBe(0);
  });
  test('E3 one request, and the opening is handed to the deck from it; a failed request still opens the radio', async () => {
    await radioPulse(storage, native as never);
    at += 3000;
    await radioHold(storage, native as never);
    expect(fronts).toBe(1); expect(calls.some((c) => c.op === 'deck' && c.name === 'deckPlay')).toBe(true);
    await wipeRadio(undefined); calls.length = 0; fronts = 0; failFront = true; at = 1_700_000_000_000;
    await radioPulse(storage, native as never);
    await radioHold(storage, native as never); at += 3000; await radioHold(storage, native as never);
    expect(calls.some((c) => c.op === 'deck' && c.name === 'deckPlay')).toBe(true);
  });
});
