// Is there a native package? (host.ts). Decided from the native handle, not from what the native set happens to hold: a cold iPhone launch has an empty native set until the first
// native call answers, and the shared twins (deck set, notebook) must stay idle there. Failure list, written first:
//  H1 a usable native handle that answers the probe is "native", at once, and stays "native" (a later failed probe does not flip it back).
//  H2 no handle at all (Android) is "none" only after the wait; "native" if a handle becomes usable inside the wait (a module that loads late).
//  H3 a handle that is there but never answers the probe is "none" after the wait; a handle that throws NotSupported is "none".
//  H4 before the decision the contract reads the answer as unknown ("" is neither): the contracts treat only "none" as no native, so unknown behaves as iOS does.
import { beforeEach, describe, expect, test } from 'bun:test';
import { hostMode, resetHost } from './host.ts';
import { adoptNative, resetSeam } from './api.ts';

beforeEach(() => { resetSeam(); resetHost(); });
const handle = (later: (r: { op: string }) => Promise<unknown>) => ({ available: true, later }) as never;

describe('hostMode', () => {
  test('H1 a handle that answers the probe is native, and stays native', async () => {
    adoptNative(handle(async () => ({ state: 'present' })), 'x');
    expect(await hostMode(20)).toBe('native');
    adoptNative(handle(async () => { throw new Error('NotSupported'); }), 'y');                     // never replaces a usable one; and the decision is kept
    expect(await hostMode(20)).toBe('native');
  });
  test('H2 no handle is "none" after the wait; a handle that turns up inside it is native', async () => {
    expect(await hostMode(30)).toBe('none');
    resetHost(); resetSeam();
    const p = hostMode(200);
    setTimeout(() => adoptNative(handle(async () => ({ state: 'absent' })), 'late'), 30);
    expect(await p).toBe('native');
  });
  test('H3 a handle that never answers, or throws NotSupported, is none', async () => {
    adoptNative(handle(() => new Promise(() => undefined)), 'x');
    expect(await hostMode(40, 20)).toBe('none');
    resetHost(); resetSeam(); adoptNative(handle(async () => { throw new Error('NotSupportedError'); }), 'y');
    expect(await hostMode(40, 20)).toBe('none');
  });
  test('H4 the contracts read only "none" as no native (unknown is native)', async () => {
    const spine = await Bun.file(new URL('./spine.contract', import.meta.url)).text();
    expect(spine).toContain('derive noNative = hc.mode == "none"');
    for (const f of ['radio.contract']) expect(await Bun.file(new URL(`./${f}`, import.meta.url)).text()).toContain('noNative');
    expect(spine).not.toContain('derive pg = pgn.top');          // the SET is never chosen by the native set's content; the native set's top page only tells the host fact that the package answered (hostCaps(pgn.top != ""))
  });
});

describe('round 6: recovery and the time bound', () => {
  test('R1 once the package has answered (the native set has a top page) the decision is native and cannot be downgraded; a "none" recovers on that signal', async () => {
    expect(await hostMode(30, 10)).toBe('none');                                  // a bad window: no usable handle in time
    expect(await hostMode(30, 10, true)).toBe('native');                         // the contract re-asks with "the package answered": recovered
    expect(await hostMode(30, 10, false)).toBe('native');                        // and never downgraded again
  });
  test('R1b an app-module op that answered anywhere (api.ts) counts as native too', async () => {
    const api = await import('./api.ts');
    resetSeam(); resetHost();
    api.adoptNative(handle(async (r) => (r.op === 'session' ? { found: false } : { state: 'absent' })), 'x');
    await api.keyState();                                                         // any answer from the app module
    expect((await api.keyState()).state).toBe('absent');
    expect(api.nativeHasAnswered()).toBe(true);
    expect(await hostMode(5, 5)).toBe('native');
  });
  test('R2 the wait is bounded by time: a probe that hangs does not stretch it', async () => {
    adoptNative(handle(() => new Promise(() => undefined)), 'x');
    const t = performance.now(); const m = await hostMode(120, 1000);          // a 1 s probe timeout must not extend a 120 ms wait
    expect(m).toBe('none'); expect(performance.now() - t).toBeLessThan(600);
  });
});

describe('round 7', () => {
  test('N1 a probe in flight when the native signal arrives and then times out does not overwrite native with none', async () => {
    adoptNative(handle(() => new Promise(() => undefined)), 'x');
    const first = hostMode(80, 20);                                              // the probe hangs; the wait will end in "none"
    await new Promise((r) => setTimeout(r, 10));
    expect(await hostMode(80, 20, true)).toBe('native');                         // the signal arrives while it is in flight
    expect(await first).toBe('native');                                          // the in-flight decision must not say none now
    expect(await hostMode(80, 20, false)).toBe('native');                        // and the fact stays native
  });
});

describe('the notebook page is idle when it is not on top', () => {
  test('H5 with the page not on top (active false) the notebook source does not fetch', async () => {
    let fetches = 0; globalThis.fetch = (async () => { fetches++; return new Response('{}', { status: 200 }); }) as never;
    const { answer } = await import('./app.ts');
    const storage = { fs: { readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async () => undefined } } as never;
    const n = await answer('notebookPageShared' as never, ['', 0, false, ''] as never, undefined as never, storage, undefined as never) as { docState: string };
    expect(n.docState).toBe(''); expect(fetches).toBe(0);
  });
});
