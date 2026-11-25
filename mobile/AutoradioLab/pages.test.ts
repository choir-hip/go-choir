// The deck's open pages in shared code (pages.ts). Failure list, written before the code was trusted (each line has a test):
//  P1 a launch lands on the radio, alone.   P2 opening puts a page on top and a second open of it moves it, no duplicate; the opener is the page that was on top.
//  P3 the radio cannot be closed; closing the front page brings its opener forward, else the next most recent.   P4 reopen brings back the last closed page once.
//  P5 the set survives a relaunch (addresses, titles, pins, order) and a damaged file is the radio alone.   P6 the pile keeps 30 unpinned pages, pinned ones stay.
//  P7 kinds and titles are the Swift ones (a file's name, a section, "texture:my-edition" opens the edition page).   P8 sign-out leaves the radio and no file.
//  P9 a write that lands after a sign-out began is removed.
//  P10 a stored file with a malformed entry (a non-string in recency, a page with no address) drops THAT entry and keeps the rest: nothing is half-applied.
//  P11 two callers at the first read share ONE load (no duplicates, one read).   P12 a wipe during a load: the old account's file does not refill the set.
//  P13 a close then reopen keeps the opener (as the Swift reopen does).
import { beforeEach, describe, expect, test } from 'bun:test';
import * as P from './pages.ts';
import { bindNative, nextGeneration, resetSeam } from './api.ts';

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
    // the opener is NOT the next most recent: open C from A while B was opened later and focused; closing C must bring A forward, not B
    P.forget(); files.clear();
    await P.pagesOpen(storage, 'paper:front', ''); await P.pagesOpen(storage, 'textures:', ''); await P.pagesFocus(storage, 'paper:front');          // front: paper
    await P.pagesOpen(storage, 'pub:/c', 'C');                                                                                                          // opener: paper:front
    await P.pagesFocus(storage, 'textures:'); await P.pagesFocus(storage, 'pub:/c');                                                                    // recency now: c, textures, paper
    v = await P.pagesClose(storage, 'pub:/c'); expect(v.top).toBe('paper:front');                                                                       // the opener, not textures:
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
    expect(v.rows.filter((r) => !r.pinned && r.address !== 'radio:').length).toBe(30); expect(addrs(v)).toContain('paper:front'); expect(addrs(v)).toContain('pub:/s39'); expect(addrs(v)).not.toContain('pub:/s0'); expect(addrs(v)).toContain('radio:');   // the radio is never trimmed
  });
  test('P7 kinds and titles are the Swift ones', async () => {
    expect(P.kindOf('mailbox:inbox')).toBe('mailbox'); expect(P.kindOf('podcast:show/x')).toBe('podcast'); expect(P.kindOf('weird')).toBe('unknown');
    expect(P.defaultTitle('paper:section/science')).toBe('Paper · science'); expect(P.defaultTitle('mailbox:sent')).toBe('Mail · sent'); expect(P.defaultTitle('podcast:episode/x')).toBe('Episode');
    expect(P.canonical('texture:abc123')).toBe('texture:abc123'); expect(P.kindOf('onboarding:edit')).toBe('onboarding'); expect(P.defaultTitle('onboarding:edit')).toBe('My edition');
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

describe('load', () => {
  test('P10 malformed entries are dropped one by one; the rest is kept', async () => {
    files.set(P.PAGES_FILE, new TextEncoder().encode(JSON.stringify({
      pages: [{ address: 'paper:front', title: 'Paper', opener: '', pinned: false }, { title: 'no address' }, 7, null, { address: 'textures:', title: 'T', opener: 5, pinned: 'yes' }],
      recency: ['textures:', 42, null, 'paper:front', 'gone:'],
    })));
    const v = await P.pagesView(storage);
    expect(addrs(v).sort()).toEqual(['paper:front', 'radio:', 'textures:']);
    expect(v.rows.find((r) => r.address === 'textures:')).toMatchObject({ opener: '', pinned: false });          // wrong types become the defaults
    await P.pagesOpen(storage, 'file:/', '');                                                                  // and the set is usable and saved
    expect(JSON.parse(new TextDecoder().decode(files.get(P.PAGES_FILE)!)).pages.length).toBe(4);
  });
  test('P11 two callers at the first read share one load', async () => {
    await P.pagesOpen(storage, 'paper:front', ''); P.forget();
    let reads = 0; let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const slow = { fs: { readFile: async (p: string) => { reads++; await gate; return files.get(p)!; }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
    const a = P.pagesView(slow), b = P.pagesOpen(slow, 'textures:', '');
    await new Promise((r) => setTimeout(r, 10)); release(); await Promise.all([a, b]);
    const v = await P.pagesView(slow);
    expect(reads).toBe(1); expect(addrs(v)).toEqual(['textures:', 'radio:', 'paper:front']); expect(new Set(addrs(v)).size).toBe(v.rows.length);
  });
  test('P12 a wipe during a load: the old account\'s file does not refill the set (fails without the epoch guard)', async () => {
    await P.pagesOpen(storage, 'paper:front', 'Old account page'); P.forget();
    const snapshot = files.get(P.PAGES_FILE)!;                                 // the bytes the read in flight will return, taken BEFORE the wipe deletes the file
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    let reads = 0;
    const slow = { fs: { readFile: async () => { reads++; if (reads === 1) { await gate; return snapshot; } throw new Error('ENOENT'); }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
    const load = P.pagesView(slow); await new Promise((r) => setTimeout(r, 10));
    await P.wipePages(slow);                                                   // sign-out while the read is out
    release(); const v = await load;
    expect(addrs(v)).toEqual(['radio:']); expect(addrs(await P.pagesView(slow))).toEqual(['radio:']);
  });
  test('P13 close then reopen keeps the opener', async () => {
    await P.pagesOpen(storage, 'paper:front', ''); await P.pagesOpen(storage, 'pub:/x', 'X'); await P.pagesClose(storage, 'pub:/x');
    let v = await P.pagesReopen(storage); expect(v.rows[0]).toMatchObject({ address: 'pub:/x', opener: 'paper:front' });
    v = await P.pagesClose(storage, 'pub:/x'); expect(v.top).toBe('paper:front');                                    // the opener comes forward again
  });
});

describe('round 6', () => {
  test('S1 saves are serialized: the last state wins the file even if an older write is slower', async () => {
    const delays = [60, 0, 0]; let n = 0;
    const slowFirst = { fs: { readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async (p: string, d: Uint8Array) => { const w = delays[n++] ?? 0; await new Promise((r) => setTimeout(r, w)); files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
    const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));
    await P.pagesView(slowFirst);
    const p1 = P.pagesOpen(slowFirst, 'paper:front', ''); await sleep(5); const p2 = P.pagesOpen(slowFirst, 'textures:', ''); await sleep(5); const p3 = P.pagesOpen(slowFirst, 'file:/', '');
    await Promise.all([p1, p2, p3]);
    const saved = JSON.parse(new TextDecoder().decode(files.get(P.PAGES_FILE)!)) as { recency: string[] };
    expect(saved.recency[0]).toBe('file:/'); expect(saved.recency.length).toBe(4);
  });
  test('C1 close and pin canonicalise the address like open does', async () => {
    await P.pagesOpen(storage, 'doc:abc@12', '');
    let v = await P.pagesPin(storage, 'doc:abc@12'); expect(v.rows.find((r) => r.address === 'doc:abc')?.pinned).toBe(true);
    v = await P.pagesClose(storage, 'doc:abc@12'); expect(addrs(v)).not.toContain('doc:abc');
  });
  test('F1 focus canonicalises the address like open, close and pin', async () => {
    await P.pagesOpen(storage, 'doc:abc@12', ''); await P.pagesOpen(storage, 'paper:front', '');
    const v = await P.pagesFocus(storage, 'doc:abc@12'); expect(v.top).toBe('doc:abc');
  });
  test('O1 an open that finished loading after a sign-out began changes nothing', async () => {
    const api = await import('./api.ts');
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const slow = { fs: { readFile: async () => { await gate; throw new Error('ENOENT'); }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
    const p = P.pagesOpen(slow, 'paper:front', ''); await new Promise((r) => setTimeout(r, 10)); api.nextGeneration(); release(); const v = await p;
    expect(addrs(v)).not.toContain('paper:front'); expect(files.has(P.PAGES_FILE)).toBe(false);
  });
});

describe('the voice\'s facts and a native request (P14, P15)', () => {
  const calls: Record<string, unknown>[] = [];
  const native = { available: true, later: async (r: Record<string, unknown>) => { calls.push(r); return { ok: true }; } };
  beforeEach(() => { calls.length = 0; bindNative(native as never); });
  test('P14 every change hands the voice the pages as facts, with the generation the change began under; the first read does too, once', async () => {
    await P.pagesView(storage); await P.pagesView(storage);
    expect(calls.filter((c) => c.op === 'pagesMirror').length).toBe(1);
    await P.pagesOpen(storage, 'pub:a', 'A');
    const last = calls[calls.length - 1] as { op: string; gen: number; pages: { address: string; title: string; kind: string }[] };
    expect(last.op).toBe('pagesMirror'); expect(last.pages.map((x) => x.address)).toEqual(['pub:a', 'radio:']); expect(last.pages[0]).toEqual({ address: 'pub:a', title: 'A', kind: 'reader' });
    nextGeneration();
    await P.pagesClose(storage, 'pub:a');
    expect((calls[calls.length - 1] as { gen: number }).gen).toBe(1);
  });
  test('P14 no native module: the set still works and nothing is sent', async () => {
    bindNative(undefined);
    expect(addrs(await P.pagesOpen(storage, 'pub:a', 'A'))).toEqual(['pub:a', 'radio:']);
    expect(calls.length).toBe(0);
  });
  test('P15 a native request opens its page once per id; an older or empty request asks nothing', async () => {
    expect(addrs(await P.pageRequested(storage, 0, '', ''))).toEqual(['radio:']);
    expect(addrs(await P.pageRequested(storage, 1, 'podcast:episode/x', 'Episode'))).toEqual(['podcast:episode/x', 'radio:']);
    await P.pagesClose(storage, 'podcast:episode/x');
    expect(addrs(await P.pageRequested(storage, 1, 'podcast:episode/x', 'Episode'))).toEqual(['radio:']);       // the same id again: not reopened
    expect(addrs(await P.pageRequested(storage, 2, 'podcast:episode/y', 'Episode'))).toEqual(['podcast:episode/y', 'radio:']);
  });
});
