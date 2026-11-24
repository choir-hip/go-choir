// Stale, then fresh, for the pages that keep a saved copy (first-open lane, fix 2 beyond the paper). Failure list, written before the code:
//  S1 podcasts: with a library and a feed kept from an earlier run and the network held, `podcastHomeSaved` answers the shows at once; the fresh answer is still out.
//  S2 podcasts: no library kept, or a query typed (a search is the network's): nothing is answered from the saved copy, and nothing is invented.
//  S3 podcasts: a show page answers its episodes from the kept feed at once; no kept feed: nothing.
//  S4 notebook: the notebook kept from the last read is the page at once, with the scorecard still loading; the page that is not on top, or no kept text, answers nothing.
//  S5 an account change while a saved copy is being read: it is not shown (podcasts, notebook).
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import * as P from './podcast.ts';
import * as M from './podcast-model.ts';
import { notebookSaved } from './notebook-run.ts';
import { notebookShared } from './notebook-run.ts';
import { CACHE_FILE as NB_FILE } from './notebook.ts';
import { bindNative, resetSession, nextGeneration, resetSeam } from './api.ts';

const fx = (n: string): string => readFileSync(new URL(`../fixtures/podcasts/${n}`, import.meta.url), 'utf8');
const FEED = fx('feed-synthetic-hour.xml');
const FEED_URL = 'https://feeds.example.com/show.xml';
const NOW = Date.parse('2026-11-16T12:00:00Z');
const files = new Map<string, Uint8Array>();
let readGate: Promise<void> | null = null; let netGate: Promise<void> | null = null; let calls = 0;
const storage = { fs: {
  readFile: async (p: string) => { if (readGate) await readGate; const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, mkdir: async () => {}, rm: async (p: string) => { for (const k of [...files.keys()]) if (k === p || k.startsWith(p + '/')) files.delete(k); },
} } as never;
const reply = (body: string, status = 200): Response => new Response(body, { status, headers: { date: new Date(NOW).toUTCString() } });
beforeEach(() => {
  P.forget(); resetSeam(); files.clear(); readGate = null; netGate = null; calls = 0; resetSession(); bindNative(undefined);
  P.search.pause = 0; P.zone.offset = 0;
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    calls++; if (netGate) await netGate;
    const u = new URL(String(input));
    if (u.host === 'feeds.example.com') return reply(FEED);
    return reply('{}', 404);
  }) as typeof fetch;
});
/** A library and a feed kept by an earlier run: subscribe, open the home (it fetches the feed and keeps it), then forget what the process held. */
async function keptRun(): Promise<void> {
  await P.podcastSubscribe(FEED_URL, storage);
  await P.podcastHome({ q: '', address: M.HOME, tick: 0 }, storage);
  P.forget(); calls = 0;
}

describe('podcasts', () => {
  test('S1 the home shows the kept shows while the network is held', async () => {
    await keptRun();
    let release: () => void = () => undefined; netGate = new Promise<void>((r) => { release = r; });
    const fresh = P.podcastHome({ q: '', address: M.HOME, tick: 0 }, storage);        // out on the gate (a stale feed is fetched again)
    const saved = await P.podcastHomeSaved({ q: '', address: M.HOME, tick: 0 }, storage);
    expect(saved.status).toBe('ready'); expect(saved.shows.length).toBe(1); expect(saved.shows[0].feedURL).toBe(FEED_URL); expect(saved.shows[0].title).not.toBe('');
    release(); await fresh;
  });
  test('S2 no kept library, or a search typed: nothing from the saved copy', async () => {
    expect((await P.podcastHomeSaved({ q: '', address: M.HOME, tick: 0 }, storage)).status).toBe('');
    await keptRun();
    expect((await P.podcastHomeSaved({ q: 'chess', address: M.HOME, tick: 0 }, storage)).status).toBe('');
    expect((await P.podcastHomeSaved({ q: '', address: 'paper:front', tick: 0 }, storage)).status).toBe('');
  });
  test('S3 a show page answers its episodes from the kept feed at once', async () => {
    await keptRun();
    const address = M.showAddress(FEED_URL);
    let release: () => void = () => undefined; netGate = new Promise<void>((r) => { release = r; });
    const fresh = P.podcastShow({ address, tick: 0, nowId: '', rev: 0 }, storage);
    const saved = await P.podcastShowSaved({ address, tick: 0, nowId: '', rev: 0 }, storage);
    expect(saved.status).toBe('ready'); expect(saved.episodes.length).toBeGreaterThan(0); expect(saved.title).not.toBe('');
    release(); await fresh;
    files.clear(); P.forget();
    expect((await P.podcastShowSaved({ address, tick: 0, nowId: '', rev: 0 }, storage)).status).toBe('');
  });
  test('S5a an account change while the kept copy is read: not shown', async () => {
    await keptRun();
    let release: () => void = () => undefined; readGate = new Promise<void>((r) => { release = r; });
    const p = P.podcastHomeSaved({ q: '', address: M.HOME, tick: 0 }, storage); await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); release();                                                          // a sign-out or an account change moves the generation
    expect((await p).status).toBe('');
  });
});

describe('notebook', () => {
  const TEXT = '# My notebook\n\n## What you asked for\n- Space\n- Chess\n\n## What Autoradio has learned\n- A thing. [L2 · heard 1 of 1]\n';
  const keep = (): void => { files.set(NB_FILE, new TextEncoder().encode(TEXT)); };
  test('S4 the kept notebook is the page at once, the scorecard still loading', async () => {
    keep();
    const saved = await notebookSaved({ selected: '', active: true }, storage);
    expect(saved.docState).toBe('ready'); expect(saved.sections.length).toBeGreaterThan(0); expect(saved.scoreState).toBe('loading');
    expect((await notebookSaved({ selected: '', active: false }, storage)).docState).toBe('loading');
    files.clear();
    expect((await notebookSaved({ selected: '', active: true }, storage)).docState).toBe('loading');
  });
  test('S4b the fresh page does not wait for the saved one, and the saved one does not touch the network', async () => {
    keep(); let gets = 0;
    const env = { get: (async () => { gets++; return await new Promise(() => undefined); }) as never, device: null };
    const fresh = notebookShared({ selected: '', tick: 0, active: true, signedOutTick: '' }, storage, env);   // hangs on the computer
    const saved = await notebookSaved({ selected: '', active: true }, storage);
    expect(saved.docState).toBe('ready');
    await new Promise((r) => setTimeout(r, 20)); expect(gets).toBeGreaterThan(0);          // the fresh page is out on the computer and the saved one did not wait for it
    void fresh;
  });
  test('S5b an account change while the kept notebook is read: not shown', async () => {
    keep();
    let release: () => void = () => undefined; readGate = new Promise<void>((r) => { release = r; });
    const p = notebookSaved({ selected: '', active: true }, storage); await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); release();
    expect((await p).docState).toBe('loading');
  });
});
