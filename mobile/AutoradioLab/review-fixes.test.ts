// The review of the first-open lane (seat 2, 93a6df1ba) sent it back. Failure list, written before the fixes:
//  R1 the kept copy of the podcast home, the podcast show and the notebook is what the page gets while the request is out. Exact keeps a resource's last answer when its arguments change
//     (contract-for-agents.md: "A resource keeps its answer and asks again when its arguments change"), so the fresh source's answer from a moment when the page was not on top (the launch lands on the radio)
//     must be the EMPTY status, or the derive `Now.status != "" ? Now : Kept` never reaches the kept copy. Off the page the fresh source answers "".
//  R2 a saved category page never says "empty": only fresh data may say a section has nothing. (A saved category answers nothing; the front page is what the saved list can say alone.)
//  R3 the unawaited saves issue no storage call after the answer: writeCurrent's post-write removal is not part of them.
//  R4 the saved page does not claim to be fresh: with no device time its "Updated ..." line is empty.
//  R6 a show page shows an answer only for the address it was asked for: going straight from show A to B, A's answer is kept by Exact until B's lands and must not be drawn on B's page (members-podcasts.contract).
//  R5 a sign-out after an open: against a storage that runs its operations in the order issued (Exact's one queue), the open set's late write is followed by the wipe's removal, and the file is gone.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as P from './pages.ts';
import * as R from './recent-pages.ts';
import * as PC from './podcast.ts';
import * as M from './podcast-model.ts';
import { notebookShared, notebookSaved } from './notebook-run.ts';
import { paperSaved, paperFront, forget as forgetPaper, CACHE_FILE } from './paper.ts';
import { clock } from './clock.ts';
import { writeCurrent, nextGeneration, resetSeam, bindNative } from './api.ts';
import real from '../fixtures/paper/front-live-2026-11-18.json';

const files = new Map<string, Uint8Array>();
const ops: string[] = [];
let tail: Promise<void> = Promise.resolve(); let writeGate: Promise<void> | null = null;
/** A storage that, like Exact's, runs one operation at a time in the order issued. */
const queued = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: (p: string, d: Uint8Array) => { ops.push('issue write ' + p); const run = tail.then(async () => { if (writeGate) await writeGate; files.set(p, d); ops.push('land write ' + p); }); tail = run.catch(() => undefined); return run; },
  rm: (p: string) => { ops.push('issue rm ' + p); const run = tail.then(() => { files.delete(p); ops.push('land rm ' + p); }); tail = run.catch(() => undefined); return run; },
  mkdir: async () => undefined,
} } as never;
beforeEach(() => { files.clear(); ops.length = 0; tail = Promise.resolve(); writeGate = null; P.forget(); R.forget(); PC.forget(); forgetPaper(); resetSeam(); bindNative(undefined); });

describe('R1 the fresh sources answer "" when the page is not on top', () => {
  test('the podcast home, the podcast show and the notebook', async () => {
    expect((await PC.podcastHome({ q: '', address: 'radio:', tick: 0 }, queued)).status).toBe('');
    expect((await PC.podcastShow({ address: 'radio:', tick: 0, nowId: '', rev: 0 }, queued)).status).toBe('');
    const off = await notebookShared({ selected: '', tick: 0, active: false, signedOutTick: '' }, queued);
    expect(off.docState).toBe('');
  });
  test('so a launch on the radio, then the page opened, gives the kept copy while the request is out', async () => {
    // What Exact does: the resource keeps its last answer (the radio's) while the next request for the page is out; the page's derive takes the kept copy while that answer is empty.
    const pick = <T extends { status?: string; docState?: string }>(now: T, kept: T): T => ((now.status ?? now.docState) !== '' ? now : kept);
    files.set(PC.LIBRARY_FILE, new TextEncoder().encode(JSON.stringify({ subscriptions: [{ show: { feedURL: 'https://feeds.example.com/show.xml', title: 'A show', author: '', artworkURL: '', summary: '' }, at: 1 }], positions: {}, removed: [], synced: [], clock: 0 })));
    const retained = await PC.podcastHome({ q: '', address: 'radio:', tick: 0 }, queued);
    const kept = await PC.podcastHomeSaved({ q: '', address: M.HOME, tick: 0 }, queued);
    expect(kept.shows.length).toBe(1);
    expect(pick(retained, kept).shows.length).toBe(1);
    files.set('app:/data/notebook-cache.md', new TextEncoder().encode('# My notebook\n\n## What you asked for\n- Space\n'));
    const nbOff = await notebookShared({ selected: '', tick: 0, active: false, signedOutTick: '' }, queued);
    const nbKept = await notebookSaved({ selected: '', active: true }, queued);
    expect(pick(nbOff, nbKept).docState).toBe('ready');
  });
});

describe('the paper', () => {
  beforeEach(() => { files.set(CACHE_FILE, new TextEncoder().encode(JSON.stringify(real))); });
  test('R2 a saved category page answers nothing (never "empty")', async () => {
    const s = await paperSaved({ view: 'sports-nothing-on-the-front', width: 393, tick: 0, focus: '', picks: '' }, queued);
    expect(s.status).toBe(''); expect(s.bands).toEqual([]);
    expect((await paperSaved({ view: 'front', width: 393, tick: 0, focus: '', picks: '' }, queued)).status).toBe('ready');
  });
  test('R4 with no device time the saved page does not say how fresh it is', async () => {
    clock.ms = 0;
    const s = await paperSaved({ view: 'front', width: 393, tick: 0, focus: '', picks: '' }, queued);
    expect(s.updated).toBe('');
    void paperFront;
  });
});

describe('the writes behind a page open', () => {
  test('R3 an unawaited save makes no storage call after its write: a sign-out moving the generation adds no removal', async () => {
    await P.pagesView(queued);
    let release: () => void = () => undefined; writeGate = new Promise<void>((r) => { release = r; });
    await P.pagesOpen(queued, 'paper:front', '');
    nextGeneration();
    release(); await P.pagesSettled(); await R.recentSettled(); await tail;
    expect(ops.filter((o) => o.startsWith('issue rm'))).toEqual([]);
  });
  test('R3b a write its caller waits for keeps the removal: a sign-out that moves the generation during the write removes what landed', async () => {
    let release: () => void = () => undefined; writeGate = new Promise<void>((r) => { release = r; });
    const gen = 0;
    const done = writeCurrent(gen, queued, 'app:/data/x', new Uint8Array([1]));
    nextGeneration(); release();
    expect(await done).toBe(false);
    await tail;
    expect(ops).toContain('issue rm app:/data/x'); expect(files.has('app:/data/x')).toBe(false);
  });
  test('R5 a sign-out after an open: the late write lands, then the wipe removes it, in that order, and the file is gone', async () => {
    await P.pagesView(queued);
    let release: () => void = () => undefined; writeGate = new Promise<void>((r) => { release = r; });
    await P.pagesOpen(queued, 'paper:front', '');
    expect(files.has(P.PAGES_FILE)).toBe(false);                    // the write is issued and has not landed
    nextGeneration();
    const wipe = P.wipePages(queued);                                // the sign-out's wipe: its removal is queued behind the write
    release(); await wipe; await tail;
    const w = ops.indexOf('land write ' + P.PAGES_FILE), r = ops.indexOf('land rm ' + P.PAGES_FILE);
    expect(w).toBeGreaterThanOrEqual(0); expect(r).toBeGreaterThan(w);
    expect(files.has(P.PAGES_FILE)).toBe(false);
  });
  test('R5b the same for the recent pages', async () => {
    await P.pagesView(queued);
    let release: () => void = () => undefined; writeGate = new Promise<void>((r) => { release = r; });
    await P.pagesOpen(queued, 'paper:front', '');
    nextGeneration();
    const wipe = R.wipeRecent(queued);
    release(); await wipe; await tail;
    expect(files.has(R.RECENT_FILE)).toBe(false);
  });
});

describe('R6 the show page, from show A straight to show B', () => {
  const FEED_A = 'https://feeds.example.com/a.xml', FEED_B = 'https://feeds.example.com/b.xml';
  const addrA = M.showAddress(FEED_A), addrB = M.showAddress(FEED_B);
  // The contract's own rule (members-podcasts.contract `derive podShow`), here as a function over the three resources' answers.
  type V = { status: string; address: string; title: string };
  const pick = (top: string, now: V, kept: V, none: V): V => ((now.status !== '' && now.address === top) ? now : ((kept.status !== '' && kept.address === top) ? kept : none));
  test('every answer says which address it was asked for', async () => {
    const a = await PC.podcastShow({ address: addrA, tick: 0, nowId: '', rev: 0 }, queued);
    const off = await PC.podcastShow({ address: 'radio:', tick: 0, nowId: '', rev: 0 }, queued);
    expect(a.address).toBe(addrA); expect(off.address).toBe('radio:');
    expect((await PC.podcastShowSaved({ address: addrB, tick: 0, nowId: '', rev: 0 }, queued)).address).toBe(addrB);
    expect(PC.podcastShowNone().status).toBe(''); expect(PC.podcastShowNone().address).toBe('');
  });
  test('A\'s answer is not drawn on B\'s page: B\'s kept copy, else nothing', async () => {
    const A: V = { status: 'ready', address: addrA, title: 'Show A' };
    const none = PC.podcastShowNone() as V;
    const keptB: V = { status: 'ready', address: addrB, title: 'Show B (kept)' };
    const noKeptB: V = { status: '', address: addrB, title: '' };
    expect(pick(addrB, A, keptB, none).title).toBe('Show B (kept)');     // A is what Exact still holds for the fresh resource; B's kept copy is the page
    expect(pick(addrB, A, noKeptB, none)).toBe(none);                    // nothing kept for B: nothing, never A
    expect(pick(addrA, A, noKeptB, none).title).toBe('Show A');          // and A's own page still gets A
  });
});
