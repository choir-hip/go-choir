// The paper's first open (first-open lane, fixes 2 and 3). Failure list, written before the code:
//  C1 the saved front list is shown at once: `paperSaved` answers from the file while the fetch is still out, as a ready page with no offline line.
//  C2 no saved list: it answers nothing (status "", the page's own loading), it does not invent one.
//  C3 the account generation: a saved list read across a sign-out or account change is not shown.
//  C4 unchanged content: the saved page and the fresh one have the same bands (ids, routes, titles), so the page swaps nothing and the scroll stays; changed content differs.
//  C5 My edition and an unsized page (width not known yet) answer nothing from the saved list (picks arrive a moment later; the saved page is composed once the width is known).
//  D1 the page asks its data once its arguments settle: the asks a first open used to make (width 0, then the width, then the focus moving to the paper, then the picks) make one fetch and ONE compose.
//  D2 a real width that differs from the default is composed once, and not again for the focus.   D3 My edition does compose again for picks and focus (it is built from them).   D4 a pull to refresh asks again and composes again.
import { beforeEach, describe, expect, test } from 'bun:test';
import { paperFront, paperSaved, composeCount, forget, CACHE_FILE, type PaperArgs } from './paper.ts';
import { nextGeneration, resetSeam } from './api.ts';
import real from '../fixtures/paper/front-live-2026-11-18.json';

const args = (o: Partial<PaperArgs> = {}): PaperArgs => ({ view: 'front', width: 393, tick: 0, focus: 'paper', picks: '', ...o });
const NOW = Date.parse('2026-11-18T13:00:00Z');
let calls = 0; let gate: Promise<void> | null = null;
const files = new Map<string, Uint8Array>();
let readGate: Promise<void> | null = null;
const storage = { fs: {
  readFile: async (p: string) => { if (readGate) await readGate; const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
} } as never;
beforeEach(() => {
  forget(); resetSeam(); calls = 0; gate = null; readGate = null; files.clear();
  globalThis.fetch = (async () => { calls++; if (gate) await gate; return new Response(JSON.stringify(real), { status: 200, headers: { date: new Date(NOW).toUTCString() } }); }) as typeof fetch;
});
const save = (): void => { files.set(CACHE_FILE, new TextEncoder().encode(JSON.stringify(real))); };

describe('stale then fresh', () => {
  test('C1 the saved list is shown while the fetch is still out', async () => {
    save();
    let release: () => void = () => undefined; gate = new Promise<void>((r) => { release = r; });
    const fresh = paperFront(args(), storage);                         // out, waiting on the gate
    const saved = await paperSaved(args(), storage);                  // answers without it
    expect(saved.status).toBe('ready'); expect(saved.message).toBe(''); expect(saved.count).toBeGreaterThan(10); expect(saved.bands.length).toBeGreaterThan(0);
    release(); expect((await fresh).status).toBe('ready');
  });
  test('C2 no saved list: nothing is shown, nothing invented', async () => {
    const saved = await paperSaved(args(), storage);
    expect(saved.status).toBe(''); expect(saved.count).toBe(0); expect(saved.bands).toEqual([]);
  });
  test('C3 a sign-out or account change while the file was being read: the saved list is not shown', async () => {
    save();
    let release: () => void = () => undefined; readGate = new Promise<void>((r) => { release = r; });
    const p = paperSaved(args(), storage); await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); release();
    const saved = await p;
    expect(saved.status).toBe(''); expect(saved.bands).toEqual([]);
  });
  test('C4 saved and fresh of an unchanged list have the same bands, routes and titles', async () => {
    save();
    const saved = await paperSaved(args(), storage); const fresh = await paperFront(args(), storage);
    expect(saved.bands.map((b) => [b.id, b.items.map((i) => i.route)])).toEqual(fresh.bands.map((b) => [b.id, b.items.map((i) => i.route)]));
    expect(saved.routes).toEqual(fresh.routes); expect(saved.titles).toEqual(fresh.titles); expect(saved.viewId).toBe(fresh.viewId);
    // a changed list differs: the page does swap then
    const other = JSON.parse(JSON.stringify(real)); for (const st of other.stories) st.title = 'Changed: ' + st.title;
    files.set(CACHE_FILE, new TextEncoder().encode(JSON.stringify(other)));
    expect((await paperSaved(args(), storage)).titles).not.toEqual(fresh.titles);
  });
  test('C5 My edition and an unsized page answer nothing from the saved list', async () => {
    save();
    expect((await paperSaved(args({ view: 'mine', picks: 'Energy' }), storage)).status).toBe('');
    expect((await paperSaved(args({ width: 0 }), storage)).status).toBe('');
  });
});

describe('the page asks once its arguments settle', () => {
  test('D1 the asks of a first open make one fetch and one compose', async () => {
    const before = composeCount();
    await paperFront(args({ width: 0, focus: 'away' }), storage);              // the page's first ask: width not reported yet (the default width is composed), focus still on the radio
    await paperFront(args({ width: 393, focus: 'away' }), storage);            // the width arrives: the same one the default stands for
    await paperFront(args({ width: 393, focus: 'paper' }), storage);           // pg.top catches up
    const last = await paperFront(args({ width: 393, focus: 'paper', picks: 'Energy' }), storage);   // the picks arrive
    expect(calls).toBe(1); expect(composeCount() - before).toBe(1); expect(last.status).toBe('ready');
  });
  test('D2 a real width that differs is composed (the layout depends on it), once', async () => {
    await paperFront(args({ width: 0 }), storage); const n = composeCount();
    await paperFront(args({ width: 430 }), storage); await paperFront(args({ width: 430, focus: 'away' }), storage);
    expect(composeCount() - n).toBe(1); expect(calls).toBe(1);
  });
  test('D3 My edition is built from the picks and composes again when they change', async () => {
    await paperFront(args({ view: 'mine', picks: '' }), storage);
    const n = composeCount();
    await paperFront(args({ view: 'mine', picks: 'Energy' }), storage);
    expect(composeCount() - n).toBe(1);
    await paperFront(args({ view: 'mine', picks: 'Energy', focus: 'away' }), storage);
    await paperFront(args({ view: 'mine', picks: 'Energy', focus: 'paper' }), storage);
    expect(composeCount() - n).toBeGreaterThanOrEqual(2);                  // coming back to the paper asks again (the topic screen above all)
  });
  test('D4 a pull to refresh asks again and composes again', async () => {
    await paperFront(args(), storage); const n = composeCount();
    await paperFront(args({ tick: 1 }), storage);
    expect(calls).toBe(2); expect(composeCount() - n).toBe(1);
  });
});
