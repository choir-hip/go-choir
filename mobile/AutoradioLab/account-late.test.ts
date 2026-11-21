// A late answer must not plant the old account's data in a module cache after the wipe (seat 2, review of fe027e0fc, finding 1). The gate drops the ANSWER of a source that began before a wipe;
// it cannot drop the source's own side effects. THE CLASS: every module-level cache insert and every write after an await captures the account generation before the await and drops the
// insert when it moved (api.ts `sameAccount`, the `writeCurrent` pattern). Each test: A's read is in flight (held after it took the bytes), a clean sign-out completes, A's read resolves,
// B adopts, and B's gated source must not return A's value.
//  C8.files     files.ts loadRecents (`recents`)            C8.podcast  podcast.ts loadLibrary (`lib`) and cachedFeed (`feeds`)
//  C8.mail      mail.ts alias and ensureKept/savedDrafts     C8.clips    clips.ts merge (`byID`)          C8.demo  demo-leftovers.ts rewrite (a write after a late read)
//  C8.helper    sameAccount() is false once the generation moved, true otherwise
import { afterEach, describe, expect, test } from 'bun:test';
import * as account from './account.ts';
import * as api from './api.ts';
import * as P from './pages.ts';
import * as F from './files.ts';
import * as PC from './podcast.ts';
import * as ML from './mail.ts';
import * as CL from './clips.ts';
import { clearDemoLeftovers, resetDemoLeftovers } from './demo-leftovers.ts';
import { answer } from './app.ts';
import { dig, events, KEY_A, KEY_B, memFs, newWorld, phone, STAMP, type Mem, type World } from './account-phone.ts';

let W: World; let n: ReturnType<typeof phone>; let m: Mem;
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
function boot(files: Record<string, string>): void {
  events.length = 0;
  W = newWorld('present', KEY_A); n = phone(() => W);
  m = memFs({ 'app:/data/demo-leftovers-cleared': '1', ...files, [STAMP]: dig(KEY_A) });      // the demo pass is done: its own read is not the one held
  account.resetBoundary(); api.resetSeam(); P.forget(); F.forget(); PC.forget(); ML.forget(); CL.forget(); resetDemoLeftovers(); api.bindNative(n as never);
  globalThis.fetch = (async () => new Response('{}', { status: 404 })) as never;
}
afterEach(() => { account.resetBoundary(); api.resetSeam(); });
const call = (source: string, args: unknown[]) => answer(source as never, args as never, undefined as never, m.storage, n as never);
const signOut = () => call('accountSignOut', []) as Promise<{ done: boolean }>;
/** A's read in flight; a clean sign-out; A's read resolves; B signs in. Returns A's own answer's fate. */
async function lateThenB(file: string, startA: () => Promise<unknown>): Promise<string> {
  let release: () => void = () => undefined; m.faults.hold.set(file, new Promise<void>((r) => { release = r; }));
  const a = startA().then(() => 'answered', (e) => (e as Error).name);
  await wait(15);
  expect((await signOut()).done).toBe(true);
  m.faults.hold.delete(file); release();
  const fate = await a; await wait(15);
  W.key = 'present'; W.value = KEY_B;
  return fate;
}

describe('C8 a late read plants nothing after the wipe', () => {
  test('C8.files A\'s recent files, read late, are not B\'s', async () => {
    boot({ 'app:/data/recent-files.json': JSON.stringify([{ path: '/A-private-notes.txt', size: 1, modified: null, openedAt: 1 }]) });
    const fate = await lateThenB('app:/data/recent-files.json', () => call('fileList', ['/', 'recent', 'name', false, '', 'x']));
    expect(fate).toBe('AccountClosedError');
    const b = await call('fileList', ['/', 'recent', 'name', false, '', 'x']);
    expect(JSON.stringify(b)).not.toContain('A-private-notes');
  });
  test('C8.podcast A\'s library, read late, is not B\'s', async () => {
    const lib = { subscriptions: [{ show: { feedURL: 'https://a.example/feed.xml', title: 'A-PRIVATE-SHOW', author: '', artworkURL: '', summary: '' }, at: 1 }], removed: [], synced: [], positions: {}, clock: 0 };
    boot({ 'app:/data/podcasts-library.json': JSON.stringify(lib) });
    const fate = await lateThenB('app:/data/podcasts-library.json', () => call('podcastHome', ['', 'podcasts:', 0, 'x']));
    expect(fate).toBe('AccountClosedError');
    const b = await call('podcastHome', ['', 'podcasts:', 0, 'x']);
    expect(JSON.stringify(b)).not.toContain('A-PRIVATE-SHOW');
  });
  test('C8.podcast a cached feed file of A, read late, is not B\'s', async () => {
    boot({});
    const feedUrl = 'https://a.example/feed.xml';
    const file = `${PC.FEED_DIR}/x.json`; void file;
    // the module names the file by a hash of the feed key: let it write the file itself, then read it late
    const feed = { show: { feedURL: feedUrl, title: 'A-FEED-TITLE', author: '', artworkURL: '', summary: '' }, episodes: [], fetchedAt: 1 };
    const names: string[] = [];
    const real = (m.storage as unknown as { fs: { readFile: (p: string) => Promise<Uint8Array> } }).fs;
    const read = real.readFile; real.readFile = async (p: string) => { names.push(p); if (p.startsWith(PC.FEED_DIR)) return new TextEncoder().encode(JSON.stringify(feed)) as never; return read(p); };
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    real.readFile = async (p: string) => { if (p.startsWith(PC.FEED_DIR)) { await gate; return new TextEncoder().encode(JSON.stringify(feed)) as never; } return read(p); };
    const late = PC.cachedFeed(feedUrl, m.storage); await wait(10);
    api.nextGeneration(); await PC.wipe(m.storage); release(); await late;
    real.readFile = async () => { throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); };
    expect(await PC.cachedFeed(feedUrl, m.storage)).toBeNull();                                      // B asks: nothing of A's feed was kept in memory
  });
  test('C8.mail the alias a late reply names is not kept across the wipe', async () => {
    boot({});
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async (u: string) => {
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 });
      if (String(u).includes('/api/email/aliases')) { await gate; return new Response(JSON.stringify({ aliases: [{ address: 'a-alias@example.test' }] }), { status: 200 }); }
      return new Response(JSON.stringify({ id: 'd1', from_address: 'x', to_addresses: [], subject: 's', text_body: 'b' }), { status: 200 });
    }) as never;
    const late = call('mailSaveDraft', ['mail:new', 'x@example.test', 's', 'b']).then(() => 'answered', (e) => (e as Error).name);
    await wait(15); expect((await signOut()).done).toBe(true); release(); await late; await wait(15);
    W.key = 'present'; W.value = KEY_B;
    const sent: string[] = [];
    globalThis.fetch = (async (u: string, init: RequestInit) => {
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c2', current: true }] }), { status: 200 });
      if (String(u).includes('/api/email/aliases')) return new Response(JSON.stringify({ aliases: [{ address: 'b-alias@example.test' }] }), { status: 200 });
      sent.push(String(init?.body ?? '')); return new Response(JSON.stringify({ id: 'd2', from_address: 'b', to_addresses: [], subject: 's', text_body: 'b' }), { status: 200 });
    }) as never;
    await call('mailSaveDraft', ['mail:new', 'y@example.test', 's2', 'b2']);
    expect(sent.join('|')).not.toContain('a-alias@example.test');
  });
  test('C8.clips a clips reply that lands after the generation moved is not merged', async () => {
    CL.forget();
    const body = JSON.parse(await Bun.file(new URL('../fixtures/podcasts/clips.json', import.meta.url)).text());
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async () => { await gate; return new Response(JSON.stringify(body), { status: 200 }); }) as never;
    const late = CL.showClips('podcast:show/' + encodeURIComponent('https://feeds.example.com/show.xml'), 0);
    await wait(5); api.nextGeneration(); CL.forget(); release(); await late;
    expect(CL.remembered()).toBe(0);
  });
  test('C8.demo a late read of the recents does not write the old account\'s file back after the wipe', async () => {
    boot({ 'app:/data/recent-files.json': JSON.stringify([{ path: '/Readme.txt', size: 55, modified: 1_700_000_000_000 - 30 * 86_400_000, openedAt: 1 }, { path: '/A-mine.txt', size: 1, modified: null, openedAt: 2 }]) });
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/recent-files.json', new Promise<void>((r) => { release = r; }));
    m.files.delete('app:/data/demo-leftovers-cleared'); const run = clearDemoLeftovers(m.storage); await wait(15);
    api.nextGeneration(); m.files.delete('app:/data/recent-files.json'); m.faults.hold.clear(); release(); await run; await wait(15);
    expect(m.files.has('app:/data/recent-files.json')).toBe(false);                                // the wipe's removal stands
  });
  test('C8.helper sameAccount is true until the generation moves', () => {
    api.resetSeam(); const same = (api as unknown as { sameAccount: () => () => boolean }).sameAccount();
    expect(same()).toBe(true); api.nextGeneration(); expect(same()).toBe(false);
  });
});
