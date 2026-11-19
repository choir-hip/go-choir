// The deck's open pages in shared code (pages.ts). Failure list, written before the code was trusted (each line has a test):
//  P1 a launch lands on the radio, alone.   P2 opening puts a page on top and a second open of it moves it, no duplicate; the opener is the page that was on top.
//  P3 the radio cannot be closed; closing the front page brings its opener forward, else the next most recent.   P4 reopen brings back the last closed page once.
//  P5 the set survives a relaunch (addresses, titles, pins, order) and a damaged file is the radio alone.   P6 the pile keeps 30 unpinned pages, pinned ones stay.
//  P7 kinds and titles are the Swift ones (a file's name, a section, "texture:my-edition" opens the edition page).   P8 sign-out leaves the radio and no file.
//  P9 a write that lands after a sign-out began is removed.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as P from './pages.ts';
import { nextGeneration, resetSeam } from './api.ts';

const files = new Map<string, Uint8Array>();
const storage = {
  fs: {
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    rm: async (p: string) => { files.delete(p); },
  },
} as never;
beforeEach(() => { files.clear(); P.forget(); resetSeam(); });
const addrs = (l: P.PageList): string[] => l.rows.map((r) => r.address);

describe('pages', () => {
  test('P1 a launch lands on the radio, alone', async () => {
    const v = await P.pagesView(storage);
    expect(addrs(v)).toEqual(['radio:']); expect(v.top).toBe('radio:'); expect(v.topKind).toBe('radio'); expect(v.topTitle).toBe('Autoradio'); expect(v.count).toBe(1);
  });
  test('P2 open puts a page on top; a second open moves it; no duplicate; the opener is the previous top', async () => {
    await P.pagesOpen(storage, 'paper:front', ''); await P.pagesOpen(storage, 'textures:', '');
    let v = await P.pagesOpen(storage, 'paper:front', '');
    expect(addrs(v)).toEqual(['paper:front', 'textures:', 'radio:']); expect(v.top).toBe('paper:front'); expect(v.topKind).toBe('paper'); expect(v.topTitle).toBe('Paper');
    v = await P.pagesOpen(storage, 'pub:/pub/texture/a', 'A story');
    expect(v.rows[0]).toMatchObject({ address: 'pub:/pub/texture/a', kind: 'reader', title: 'A story', opener: 'paper:front' });
  });
  test('P3 the radio stays; closing the front page brings its opener forward', async () => {
    await P.pagesOpen(storage, 'paper:front', ''); await P.pagesOpen(storage, 'textures:', ''); await P.pagesOpen(storage, 'pub:/x', 'X');   // opener of pub:/x is textures:
    let v = await P.pagesClose(storage, 'radio:'); expect(addrs(v)).toContain('radio:');
    v = await P.pagesClose(storage, 'pub:/x'); expect(v.top).toBe('textures:');
    v = await P.pagesClose(storage, 'textures:'); expect(v.top).toBe('paper:front');
    expect(addrs(await P.pagesClose(storage, 'nothing:here'))).toEqual(['paper:front', 'radio:']);
  });
  test('P4 reopen brings back the last closed page once', async () => {
    await P.pagesOpen(storage, 'textures:', 'My texts'); await P.pagesClose(storage, 'textures:');
    let v = await P.pagesReopen(storage); expect(v.top).toBe('textures:'); expect(v.topTitle).toBe('My texts');
    v = await P.pagesReopen(storage); expect(v.top).toBe('textures:'); expect(v.count).toBe(2);
  });
  test('P5 the set survives a relaunch; a damaged file is the radio alone', async () => {
    await P.pagesOpen(storage, 'paper:front', ''); await P.pagesOpen(storage, 'file:/Docs/a.md', ''); await P.pagesPin(storage, 'paper:front');
    P.forget();
    const v = await P.pagesView(storage);
    expect(addrs(v)).toEqual(['radio:', 'file:/Docs/a.md', 'paper:front']);       // a launch lands on the radio, the rest keep their order
    expect(v.rows[1].title).toBe('a.md'); expect(v.rows[2].pinned).toBe(true);
    files.set(P.PAGES_FILE, new TextEncoder().encode('not json')); P.forget();
    expect(addrs(await P.pagesView(storage))).toEqual(['radio:']);
  });
  test('P6 the pile keeps 30 unpinned pages; a pinned one stays', async () => {
    await P.pagesOpen(storage, 'paper:front', ''); await P.pagesPin(storage, 'paper:front');
    for (let i = 0; i < 40; i++) await P.pagesOpen(storage, `pub:/s${i}`, `S${i}`);
    const v = await P.pagesView(storage);
    expect(v.rows.filter((r) => !r.pinned).length).toBe(30); expect(addrs(v)).toContain('paper:front'); expect(addrs(v)).toContain('pub:/s39'); expect(addrs(v)).not.toContain('pub:/s0');
  });
  test('P7 kinds and titles are the Swift ones', async () => {
    expect(P.kindOf('mailbox:inbox')).toBe('mailbox'); expect(P.kindOf('podcast:show/x')).toBe('podcast'); expect(P.kindOf('weird')).toBe('unknown');
    expect(P.defaultTitle('paper:section/science')).toBe('Paper · science'); expect(P.defaultTitle('mailbox:sent')).toBe('Mail · sent'); expect(P.defaultTitle('podcast:episode/x')).toBe('Episode');
    expect(P.canonical(' Texture:My-Edition ')).toBe('onboarding:edit'); expect(P.canonical('doc:abc@12')).toBe('doc:abc');
    expect((await P.pagesOpen(storage, 'texture:my-edition', '')).top).toBe('onboarding:edit');
  });
  test('P8 sign-out leaves the radio and no file', async () => {
    await P.pagesOpen(storage, 'paper:front', ''); expect(files.has(P.PAGES_FILE)).toBe(true);
    await P.wipePages(storage);
    expect(files.has(P.PAGES_FILE)).toBe(false); expect(addrs(await P.pagesView(undefined))).toEqual(['radio:']);
  });
  test('P9 a write that lands after a sign-out began is removed', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const slow = { fs: { readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async (p: string, d: Uint8Array) => { await gate; files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
    const p = P.pagesOpen(slow, 'paper:front', ''); await new Promise((r) => setTimeout(r, 5)); nextGeneration(); release(); await p;
    expect(files.has(P.PAGES_FILE)).toBe(false);
  });
});
