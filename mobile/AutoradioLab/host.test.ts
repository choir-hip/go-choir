// Is there a native module? (host.ts). Exact's own fact, `native.available`, read in the answer: no native call and no timer (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md;
// it replaced a probe that sent the module a call under three timers). The deck's page set and the notebook's shared twins stay idle on iOS. Failure list, written first:
//  H1 a handle whose `available` is true is "native"; one whose `available` is false, or no handle (Android, the web), is "none".
//  H2 deciding asks the handle nothing: no call, no `later`.
//  H4 the contracts read only "none" as no native (an answer not in yet, "", behaves as iOS does).
import { describe, expect, test } from 'bun:test';
import { hostCaps } from './host.ts';

describe('hostCaps', () => {
  test('H1 available is native; not available or no handle is none', () => {
    expect(hostCaps({ available: true })).toEqual({ mode: 'native' });
    expect(hostCaps({ available: false })).toEqual({ mode: 'none' });
    expect(hostCaps(null)).toEqual({ mode: 'none' });
    expect(hostCaps(undefined)).toEqual({ mode: 'none' });
  });
  test('H2 deciding asks the handle nothing', () => {
    let asked = 0;
    const h = { get available() { return true; }, later: () => { asked += 1; return Promise.resolve({}); }, call: () => { asked += 1; return {}; } };
    expect(hostCaps(h).mode).toBe('native');
    expect(asked).toBe(0);
  });
  test('H4 the contracts read only "none" as no native (unknown is native)', async () => {
    const spine = await Bun.file(new URL('./spine.contract', import.meta.url)).text();
    expect(spine).toContain('derive noNative = hc.mode == "none"');
    expect(spine).toContain('resource hc = hostCaps() as shape HostCaps');
    for (const f of ['radio.contract']) expect(await Bun.file(new URL(`./${f}`, import.meta.url)).text()).toContain('noNative');
    expect(spine).not.toContain('derive pg = pgn.top');          // the SET is never chosen by the native set's content
  });
});

describe('the notebook page is idle when it is not on top', () => {
  test('H5 with the page not on top (active false) the notebook source does not fetch', async () => {
    let fetches = 0; globalThis.fetch = (async () => { fetches++; return new Response('{}', { status: 200 }); }) as never;
    const { answer } = await import('./app.ts');
    const storage = { fs: { readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async () => undefined } } as never;
    const n = await answer('notebookPageShared' as never, ['', 0, false, ''] as never, undefined as never, storage, undefined as never) as { docState: string };
    expect(n.docState).toBe('loading'); expect(fetches).toBe(0);
  });
});
