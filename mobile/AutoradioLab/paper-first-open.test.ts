// The paper asks once its arguments settle (first-open lane, fix 3). Failure list, written before the code:
//  D1 the page asks its data once its arguments settle: the asks a first open used to make (width 0, then the width, then the focus moving to the paper, then the picks) make one fetch and ONE compose.
//  D2 a real width that differs from the default is composed once, and not again for the focus.   D3 My edition does compose again for picks and focus (it is built from them).   D4 a pull to refresh asks again and composes again.
import { beforeEach, describe, expect, test } from 'bun:test';
import { paperFront, composeCount, forget, type PaperArgs } from './paper.ts';
import { resetSeam } from './api.ts';
import real from '../fixtures/paper/front-live-2026-11-18.json';

const args = (o: Partial<PaperArgs> = {}): PaperArgs => ({ view: 'front', width: 393, tick: 0, focus: 'paper', picks: '', ...o });
const NOW = Date.parse('2026-11-18T13:00:00Z');
let calls = 0; let gate: Promise<void> | null = null;
const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
} } as never;
beforeEach(() => {
  forget(); resetSeam(); calls = 0; gate = null; files.clear();
  globalThis.fetch = (async () => { calls++; if (gate) await gate; return new Response(JSON.stringify(real), { status: 200, headers: { date: new Date(NOW).toUTCString() } }); }) as typeof fetch;
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
