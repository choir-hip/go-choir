// Is there a native package? (host.ts). Decided from the native handle, not from what the native set happens to hold: a cold iPhone launch has an empty native set until the first
// native call answers, and the shared twins (deck set, reader, notebook) must stay idle there. Failure list, written first:
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
    for (const f of ['members-reader.contract', 'members-notebook.contract']) expect(await Bun.file(new URL(`./${f}`, import.meta.url)).text()).toContain('noNative');
    expect(spine).not.toContain('pgn.top != ""');
  });
});

describe('the twins are idle where there is a native package', () => {
  test('H5 with the flag false (iOS) neither the reader twin nor the notebook twin fetches', async () => {
    let fetches = 0; globalThis.fetch = (async () => { fetches++; return new Response('{}', { status: 200 }); }) as never;
    const { answer } = await import('./app.ts');
    const storage = { fs: { readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async () => undefined } } as never;
    const r = await answer('readerDocShared' as never, ['pub:/pub/texture/x', false] as never, undefined as never, storage, undefined as never) as { status: string };
    const n = await answer('notebookPageShared' as never, ['', 0, false, ''] as never, undefined as never, storage, undefined as never) as { docState: string };
    expect(r.status).toBe('loading'); expect(n.docState).toBe('loading'); expect(fetches).toBe(0);
  });
});
