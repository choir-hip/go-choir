// What demo mode left in the shared files of a phone that was in it (demo mode is gone), removed once. Failure list, written before the code:
//  L1 recent-files.json: an entry goes only on the demo tree's exact path AND size AND modified time (the demo backend's fixed clock); a real /Readme.txt of 55 bytes, a real 37-byte
//     Added note.txt, a demo name with another size or time all stay; the marker is written after.
//  L6 one attempt per process: two callers share it; a failed first write leaves the marker unset and, end to end with pages.ts, the next launch cleans what the failed one left.
//  L2 open-pages-shared.json: a demo address of each kind goes (pub, texture, mailbox, mail, doc of the sample book, a demo mail attachment); a real address of a similar name stays.
//  L3 once: a second call does nothing; the marker is set only after the removals (a failed write leaves it unset, and the next launch retries).
//  L4 an unreadable file is never rewritten and the marker is still set (nothing to remove is not a failure).   L5 the radio page stays in the deck.
import { beforeEach, describe, expect, test } from 'bun:test';
import { clearDemoLeftovers, resetDemoLeftovers, isDemoAddress, DEMO_MARKER } from './demo-leftovers.ts';
import { RECENT_FILE } from './files.ts';
import * as pagesMod from './pages.ts';
import { PAGES_FILE } from './pages.ts';

const mem = new Map<string, Uint8Array>();
let failWrites = false;
const storage = {
  fs: {
    readFile: async (p: string) => { const b = mem.get(p); if (!b) throw new Error('ENOENT'); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { if (failWrites) throw new Error('disk full'); mem.set(p, d); },
    writeFile: async (p: string, d: Uint8Array) => { if (failWrites) throw new Error('disk full'); mem.set(p, d); },
    rm: async (p: string) => { mem.delete(p); },
    stat: async (p: string) => { const b = mem.get(p); if (!b) throw new Error('ENOENT'); return { size: b.length }; },
    mkdir: async () => undefined,
  },
} as never;
const put = (p: string, v: unknown): void => { mem.set(p, new TextEncoder().encode(typeof v === 'string' ? v : JSON.stringify(v))); };
const get = (p: string): any => JSON.parse(new TextDecoder().decode(mem.get(p)!));
beforeEach(() => { mem.clear(); failWrites = false; resetDemoLeftovers(); pagesMod.forget(); });
const NOW = 1_700_000_000_000, DAY = 86_400_000;

const rec = (path: string, size: number, modified: number | null = 1) => ({ path, size, modified, openedAt: 2 });
const page = (address: string, pinned = false) => ({ address, kind: 'x', title: 't', opener: '', pinned });

describe('L1 recent files', () => {
  test('a demo entry (path, size and modified time) goes; every real look-alike stays', async () => {
    put(RECENT_FILE, [
      rec('/Documents/Council minutes.pdf', 2553, NOW - 1 * DAY), rec('/Readme.txt', 55, NOW - 30 * DAY), rec('/Documents/Added note.txt', 37, NOW), rec('/Pictures/Harbor test card.png', 2844, NOW - 5 * DAY),
      rec('/Documents/Council minutes.pdf', 9999, NOW - 1 * DAY), rec('/Documents/Council minutes.pdf', 2553, 1_763_000_000_000), rec('/Readme.txt', 55, 1_763_000_000_000), rec('/Readme.txt', 55, null),
      rec('/Notes/Added note.txt', 37, 1_763_000_000_000), rec('/Notes/Added note.txt', 37, NOW - DAY), rec('/Taxes/2025.pdf', 2553, NOW - 1 * DAY)]);
    await clearDemoLeftovers(storage);
    expect(get(RECENT_FILE).map((r: any) => `${r.path}:${r.size}:${r.modified}`)).toEqual([
      '/Documents/Council minutes.pdf:9999:' + (NOW - DAY), '/Documents/Council minutes.pdf:2553:1763000000000', '/Readme.txt:55:1763000000000', '/Readme.txt:55:null',
      '/Notes/Added note.txt:37:1763000000000', '/Notes/Added note.txt:37:' + (NOW - DAY), '/Taxes/2025.pdf:2553:' + (NOW - DAY)]);
    expect(mem.has(DEMO_MARKER)).toBe(true);
  });
});

describe('L2 open pages (the shared deck file)', () => {
  test('every kind of demo address goes, nothing real does', async () => {
    const demo = ['pub:demo-harbor-barrier', 'pub:/demo-river-festival#b2', 'texture:demo-notes', 'texture:demo-edition@ed-r4', 'texture:demo-draft', 'texture:demo-letter', 'texture:demo-notebook',
      'mailbox:demo-empty', 'mailbox:demo-loading', 'mailbox:demo-offline', 'mailbox:demo-error', 'mail:m1', 'mail:q1', 'mail:demo-draft-1', 'doc:6916a7c31cd3', 'file:/Mail/m6/site-plan.pdf'];
    const real = ['radio:', 'pub:demo-not-a-fixture', 'pub:/real-story', 'texture:demo-notes-2', 'texture:abc123', 'mailbox:inbox', 'mailbox:demo-spam', 'mail:em_01hz08', 'mail:m10', 'doc:aaaaaaaaaaaa', 'file:/Mail/em_01/x.pdf', 'file:/Documents/Council minutes.pdf'];
    put(PAGES_FILE, { pages: [...demo, ...real].map((a) => page(a)), recency: [...demo, ...real] });
    await clearDemoLeftovers(storage);
    const out = get(PAGES_FILE);
    expect(out.pages.map((p: any) => p.address)).toEqual(real); expect(out.recency).toEqual(real);
    for (const a of demo) expect(isDemoAddress(a), a).toBe(true);
    for (const a of real) expect(isDemoAddress(a), a).toBe(false);
  });
  test('L5 the radio stays', async () => {
    put(PAGES_FILE, { pages: [page('radio:'), page('pub:demo-grid-storage')], recency: ['pub:demo-grid-storage', 'radio:'] });
    await clearDemoLeftovers(storage);
    expect(get(PAGES_FILE).pages.map((p: any) => p.address)).toEqual(['radio:']);
  });
});

describe('L3 and L4 once, the marker, unreadable files', () => {
  test('a second call does nothing', async () => {
    put(RECENT_FILE, [rec('/Readme.txt', 55, NOW - 30 * DAY)]);
    await clearDemoLeftovers(storage); expect(get(RECENT_FILE)).toEqual([]);
    resetDemoLeftovers();                                  // a new process: the marker is what stops it
    put(RECENT_FILE, [rec('/Readme.txt', 55, NOW - 30 * DAY)]);
    await clearDemoLeftovers(storage);
    expect(get(RECENT_FILE).length).toBe(1);
  });
  test('a failed write leaves the marker unset; the next launch retries', async () => {
    put(RECENT_FILE, [rec('/Readme.txt', 55, NOW - 30 * DAY)]);
    failWrites = true; await clearDemoLeftovers(storage);
    expect(mem.has(DEMO_MARKER)).toBe(false);
    failWrites = false; resetDemoLeftovers(); await clearDemoLeftovers(storage);
    expect(get(RECENT_FILE)).toEqual([]); expect(mem.has(DEMO_MARKER)).toBe(true);
  });
  test('an unreadable file is never rewritten, and the marker is set', async () => {
    put(RECENT_FILE, 'not json {'); put(PAGES_FILE, '[1,2');
    await clearDemoLeftovers(storage);
    expect(new TextDecoder().decode(mem.get(RECENT_FILE)!)).toBe('not json {'); expect(new TextDecoder().decode(mem.get(PAGES_FILE)!)).toBe('[1,2');
    expect(mem.has(DEMO_MARKER)).toBe(true);
  });
  test('no files at all: the marker is set, nothing is created', async () => {
    await clearDemoLeftovers(storage);
    expect(mem.has(RECENT_FILE)).toBe(false); expect(mem.has(PAGES_FILE)).toBe(false); expect(mem.has(DEMO_MARKER)).toBe(true);
  });
  test('no storage: nothing happens', async () => { await clearDemoLeftovers(undefined); });
});

describe('L6 one attempt per process, and a failed first write end to end', () => {
  test('two callers share one attempt', async () => {
    put(RECENT_FILE, [rec('/Readme.txt', 55, NOW - 30 * DAY)]);
    let writes = 0;
    const counting = { fs: { ...(storage as any).fs, atomicWriteFile: async (p: string, d: Uint8Array) => { writes++; mem.set(p, d); } } } as never;
    await Promise.all([clearDemoLeftovers(counting), clearDemoLeftovers(counting), clearDemoLeftovers(counting)]);
    expect(writes).toBe(2);                                   // the recent file and the marker, once each
  });
  test('the first write fails: pages loaded after it keep what is on disk, the marker stays unset, and the next launch cleans the file the process saved', async () => {
    put(PAGES_FILE, { pages: [page('radio:'), page('pub:demo-grid-storage')], recency: ['pub:demo-grid-storage', 'radio:'] });
    failWrites = true; await clearDemoLeftovers(storage); failWrites = false;
    expect(mem.has(DEMO_MARKER)).toBe(false);
    await clearDemoLeftovers(storage);                        // the same process asks again (a later answer): no second attempt, nothing is rewritten under the loaded state
    expect(mem.has(DEMO_MARKER)).toBe(false);
    const view = await pagesMod.pagesView(storage);           // pages.ts loads after the attempt
    expect(view.rows.map((r) => r.address)).toContain('pub:demo-grid-storage');
    await pagesMod.pagesOpen(storage, 'paper:front', 'Paper');   // the process saves its memory: the demo page is still in it, as it was loaded
    expect(JSON.stringify(get(PAGES_FILE))).toContain('demo-grid-storage');
    pagesMod.forget(); resetDemoLeftovers();                  // the next launch
    await clearDemoLeftovers(storage);
    expect(mem.has(DEMO_MARKER)).toBe(true);
    expect(JSON.stringify(get(PAGES_FILE))).not.toContain('demo-grid-storage');
    const again = await pagesMod.pagesView(storage);
    expect(again.rows.map((r) => r.address).sort()).toEqual(['paper:front', 'radio:']);
    expect(again.rows.some((r) => r.address === 'pub:demo-grid-storage')).toBe(false);
  });
  test('the cleaned file is what pages.ts loads when the attempt succeeds first', async () => {
    put(PAGES_FILE, { pages: [page('radio:'), page('pub:demo-grid-storage')], recency: ['pub:demo-grid-storage', 'radio:'] });
    await clearDemoLeftovers(storage);
    expect((await pagesMod.pagesView(storage)).rows.map((r) => r.address)).toEqual(['radio:']);
  });
});
