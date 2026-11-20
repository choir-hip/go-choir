// One rule for every moved store's file write (api.ts `writeCurrent`): a source that began before a sign-out writes nothing after it, and a file that lands after the
// generation moved is removed. Failure list, written before the code (a test per store; each suspends the source across the sign-out):
//  W1 the helper itself: before, after.   W2 paper (front list cache).   W3 notebook (text cache).   W4 files (recent list, memory and file).   W5 files (cache download).
//  Why the others cannot write after the wipe (no test possible or needed):
//   takes: `saveKept` is called by take-session.ts (`keep`, `carryOver`), the takesAct send, never by a source; the file is app:/data/takes.json.   podcasts: the feed file goes through `writeCurrent` (W6); `saveLibrary` writes
//   `lib`, which `forget()` sets to null (podcast.ts:76) AND goes through `writeCurrent` too (W7), so a write already inside the atomic write when the wipe's rm runs is removed again.
//   content: every put goes through `put()` in syncAll, which checks `c.live()` (the store's generation, bumped by `wipe`) immediately before the write (content.ts `const put`).
//   mail: mail-store.ts `write` (tests R3 and R9 in mail.test.ts).   vectors: search.ts persist runs from the live store that sign-out empties first (account.ts wipes `vectors`).
import { beforeEach, describe, expect, test } from 'bun:test';
import { accountGeneration, bindNative, nextGeneration, resetSeam, writeCurrent } from './api.ts';
import * as paper from './paper.ts';
import * as F from './files.ts';
import { loadDoc, CACHE_FILE as NB_CACHE, type Get } from './notebook.ts';
import * as P from './podcast.ts';
import * as PM from './podcast-model.ts';

const written = new Map<string, Uint8Array>();
let gate: Promise<void> | null = null;
const storage = {
  fs: {
    atomicWriteFile: async (p: string, d: Uint8Array) => { written.set(p, d); },
    writeFile: async (p: string, d: Uint8Array) => { written.set(p, d); },
    readFile: async (p: string) => { const b = written.get(p); if (!b) throw new Error('ENOENT'); return b; },
    rm: async (p: string) => { written.delete(p); },
    stat: async (p: string) => { const b = written.get(p); if (!b) throw new Error('ENOENT'); return { size: b.length }; },
    mkdir: async () => undefined,
  },
} as never;
const gated = async (): Promise<void> => { if (gate) await gate; };
beforeEach(() => { written.clear(); gate = null; resetSeam(); bindNative(undefined); paper.forget(); F.forget(); });
const hold = (): (() => void) => { let r: () => void = () => undefined; gate = new Promise<void>((res) => { r = res; }); return r; };

describe('W1 the helper', () => {
  test('refuses after the generation moved; removes a file that landed after it moved', async () => {
    const g = accountGeneration();
    expect(await writeCurrent(g, storage, 'app:/data/x', new Uint8Array([1]))).toBe(true); expect(written.has('app:/data/x')).toBe(true);
    nextGeneration();
    expect(await writeCurrent(g, storage, 'app:/data/y', new Uint8Array([1]))).toBe(false); expect(written.has('app:/data/y')).toBe(false);
    const g2 = accountGeneration();
    const slow = { fs: { atomicWriteFile: async (p: string, d: Uint8Array) => { await gated(); written.set(p, d); }, rm: async (p: string) => { written.delete(p); } } } as never;
    const release = hold(); const p = writeCurrent(g2, slow, 'app:/data/z', new Uint8Array([1])); nextGeneration(); release();
    expect(await p).toBe(false); expect(written.has('app:/data/z')).toBe(false);
  });
});

describe('W6 podcasts', () => {
  test('a feed that arrives after the sign-out is not kept on disk', async () => {
    P.forget(); const release = hold();
    const rss = '<?xml version="1.0"?><rss><channel><title>Show</title><item><title>E</title><guid>g</guid><enclosure url="https://cdn.example.com/e.mp3" type="audio/mpeg"/></item></channel></rss>';
    globalThis.fetch = (async () => { await gated(); return new Response(rss, { status: 200, headers: { 'content-type': 'application/rss+xml' } }); }) as never;
    const p = P.podcastShow({ address: PM.showAddress('https://feeds.example.com/s.xml'), tick: 0, nowId: '', rev: 0 }, storage);
    await new Promise((r) => setTimeout(r, 10)); nextGeneration(); release(); await p; await new Promise((r) => setTimeout(r, 30));
    expect([...written.keys()].filter((k) => k.startsWith('app:/data/podcast-feeds') || k.includes('feed'))).toEqual([]);
  });
});

describe('W7 podcasts: the library file', () => {
  test('a library write in flight when the sign-out begins is removed again', async () => {
    P.forget(); written.clear();
    let release: () => void = () => undefined; const g = new Promise<void>((r) => { release = r; });
    const slow = { fs: { atomicWriteFile: async (p: string, d: Uint8Array) => { await g; written.set(p, d); }, readFile: async () => { throw new Error('ENOENT'); }, rm: async (p: string) => { written.delete(p); }, mkdir: async () => undefined } } as never;
    globalThis.fetch = (async () => new Response('{}', { status: 404 })) as never;
    const p = P.podcastSubscribe('https://feeds.example.com/s.xml', slow);
    await new Promise((r) => setTimeout(r, 15)); nextGeneration(); release(); await p; await new Promise((r) => setTimeout(r, 30));
    expect(written.has(P.LIBRARY_FILE)).toBe(false);
  });
});

describe('W2 to W5 the stores', () => {
  test('W2 paper: a front list that arrives after the sign-out is not cached', async () => {
    const release = hold();
    globalThis.fetch = (async () => { await gated(); return new Response(JSON.stringify({ stories: [{ route: { bare: 'a' }, title: 'A', updated_at: '2026-11-19T00:00:00Z' }] }), { status: 200 }); }) as never;
    const p = paper.listing('', storage); nextGeneration(); release(); await p;
    expect(written.has(paper.CACHE_FILE)).toBe(false);
  });
  test('W3 notebook: a text read after the sign-out is not cached', async () => {
    const release = hold();
    const get: Get = async (path) => {
      if (path === '/api/texture/edition') return { found: true, doc_id: 'nb1' };
      if (path === '/api/texture/documents/nb1') return { document: { current_revision_id: 'r1' } };
      await gated(); return { revision: { content: '# My notebook\n' } };
    };
    const p = loadDoc(get, storage); await new Promise((r) => setTimeout(r, 10)); nextGeneration(); release(); await p;
    expect(written.has(NB_CACHE)).toBe(false);
  });
  test('W4 files: a file opened across the sign-out is not put in the recent list, in memory or on disk', async () => {
    const release = hold();
    const net: F.Net = {
      json: (async (path: string) => { if (path === '/api/files') return { body: [{ name: 'a.txt', type: 'file', size: 3, modified: '2026-11-18T10:00:00Z' }], serverTime: 1, raw: '' }; await gated(); return { body: { doc_id: 'd1' }, serverTime: 1, raw: '' }; }) as never,
      bytes: (async () => ({ bytes: new Uint8Array(), contentType: '' })) as never,
      toFile: async () => ({ bytes: 3, contentType: '' }),
    };
    const p = F.fileResolve({ address: 'file:/a.txt', title: '' }, storage, net); await new Promise((r) => setTimeout(r, 10)); nextGeneration(); release(); await p;
    expect(written.has(F.RECENT_FILE)).toBe(false);
    const list = await F.fileList({ path: '/', mode: 'recent', sort: 'name', desc: false, query: '' }, storage, net);
    expect(list.rows.length).toBe(0);
  });
  test('W5 files: a download that finishes after the sign-out is removed by the seam', async () => {
    const api = await import('./api.ts');
    const g = api.accountGeneration(); expect(g).toBeGreaterThanOrEqual(0);
    const rmd: string[] = [];
    const reader = { available: true, later: async (r: { op: string }) => (r.op === 'session' ? { found: true, value: 'k-not-real-0001', computerId: 'c' } : { ok: true }) };
    bindNative(reader as never);
    globalThis.fetch = (async () => { nextGeneration(); const r = new Response('', { status: 200 }); Object.defineProperty(r, 'exactBodyTo', { value: { path: 'app:/cache/files/h/a.mp3', bytes: 5 } }); return r; }) as never;
    await expect(api.authedToFile('/api/files/a.mp3', 'app:/cache/files/h/a.mp3', { fs: { rm: async (p: string) => { rmd.push(p); } } } as never)).rejects.toBeInstanceOf(api.SignedOutError);
    expect(rmd).toEqual(['app:/cache/files/h/a.mp3']);
  });
});
