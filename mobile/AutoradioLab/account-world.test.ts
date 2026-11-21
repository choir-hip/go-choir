// THE MECHANISM (docs/design/account-boundary-2026-11-21.md, section 3, "The world"). Guarding awaits one by one did not converge (five rounds): every review found another await. So the guard is
// not at the await: the boundary moving closes the account's WORLD (api.ts `closeWorld`), and every I/O choke point is inert for a closed world. A chain that started in world A and is waiting on I/O
// never resumes once A is closed (the promise never settles); so after the boundary moves nothing of A's is inserted, written or sent, whatever the chain would have done next. The choke points: the
// storage the gate hands a source (`bindStorage`), `getJSON`, `authedJSON`/`authedBytes`/`authedToFile`, the podcast feed fetch, and the timers a chain waits on. A request is signed only with the key
// of the world that began it (`world.digest`). The Clear drains A's started writes before it wipes, so none lands after the wipe.
//  M1  one chain in world A (a read, a cache insert, a write, a send): the boundary moves, B adopts, the read is released: nothing inserted, written or sent; B's sources return nothing of A's.
//  M1b the same with the write already in flight when the world closes: the Clear waits for it (drain), and its file is wiped, not left.
//  M2  a request is never signed with a key other than the one of the world that began the chain (the Keychain changed under a live world).
//  M3  inventory: module-level mutable state is listed, file by file, with a reason; a new one fails here until someone lists it. Raw fetch, raw timers and the raw Keychain read stay in the choke points.
//  M4  a queued Adopt or Clear whose boundary moved before it ran is skipped; a hung native wipe is a failed Clear by its deadline (never a second Clear), and the card says to close and reopen the app.
//  C9.podcast-send  seat 1, finding (i): A's sync is in flight; the sign-out completes; A's list answers late: no request is made under B's key, A's subscriptions are not sent to B's computer.
//  C9.take-session  seat 1, finding (ii): A's kept takes are read late; B's takes page holds none of A's ids, handle or muted list, and the native mirror is not pushed A's.
import { afterEach, describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import * as account from './account.ts';
import * as api from './api.ts';
import * as P from './pages.ts';
import * as F from './files.ts';
import * as PC from './podcast.ts';
import * as ML from './mail.ts';
import * as CL from './clips.ts';
import { answer } from './app.ts';
const PAGES_FILE = 'app:/data/open-pages-shared.json';
import { dig, events, KEY_A, KEY_B, memFs, newWorld, phone, STAMP, type Mem, type World as PhoneWorld } from './account-phone.ts';

let W: PhoneWorld; let n: ReturnType<typeof phone>; let m: Mem;
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
function boot(files: Record<string, string>, rec = true): void {
  events.length = 0;
  W = newWorld('present', KEY_A); n = phone(() => W);
  m = memFs({ 'app:/data/demo-leftovers-cleared': '1', ...files, ...(rec ? { [STAMP]: dig(KEY_A) } : {}) });
  account.resetBoundary(); api.resetSeam(); P.forget(); F.forget(); PC.forget(); ML.forget(); CL.forget(); api.bindNative(n as never);
  globalThis.fetch = (async () => new Response('{}', { status: 404 })) as never;
}
afterEach(() => { account.resetBoundary(); api.resetSeam(); });
const call = (source: string, args: unknown[]) => answer(source as never, args as never, undefined as never, m.storage, n as never);
const signOut = () => call('accountSignOut', []) as Promise<{ done: boolean }>;
const idle = () => account.idle();

describe('M1 a chain of a closed world does nothing more', () => {
  test('a read, an insert, a write and a send in world A; the boundary moves; B adopts; the read is released: nothing inserted, written or sent', async () => {
    boot({ 'app:/data/a-private.json': 'A-SECRET' });
    const requests: string[] = [];
    globalThis.fetch = (async (u: string, init: RequestInit) => {
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 });
      requests.push(String(u) + ' ' + JSON.stringify((init?.headers as Record<string, string>)?.Authorization ?? '') + ' ' + String(init?.body ?? '')); return new Response('{}', { status: 200 });
    }) as never;
    const cache = new Map<string, string>();
    const storage = api.bindStorage(m.storage);                                                        // what the gate hands a source
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/a-private.json', new Promise<void>((r) => { release = r; }));
    const chain = (async () => {
      const bytes = await storage!.fs.readFile('app:/data/a-private.json');                            // I/O 1: held
      cache.set('after-read', new TextDecoder().decode(bytes));                                         // an insert
      await storage!.fs.atomicWriteFile('app:/data/planted-by-a.json', new TextEncoder().encode('A-SECRET'));      // a write
      cache.set('after-write', 'x');
      await api.authedJSON('/api/platform/send', { method: 'POST', body: { secret: 'A-SECRET' } });   // a send
      cache.set('after-send', 'x');
    })();
    void chain.catch(() => undefined);
    await wait(15);
    expect((await signOut()).done).toBe(true);                                                         // the boundary moves (world A closes), the phone is wiped
    W.key = 'present'; W.value = KEY_B;                                                                // B signs in
    expect((await call('pagesView', [0]).then(() => 'ok', (e) => (e as Error).name))).toBe('ok');      // B adopts and reads
    m.faults.hold.clear(); release(); await wait(30); await idle();
    expect([...cache.keys()]).toEqual([]);                                                             // nothing inserted
    expect(m.files.has('app:/data/planted-by-a.json')).toBe(false);                                    // nothing written
    expect(requests.filter((r) => r.includes('A-SECRET'))).toEqual([]);                                // nothing sent
  });
  test('M1b a write already in flight when the world closes: the Clear waits for it, and its file does not survive the wipe', async () => {
    boot({});
    const storage = api.bindStorage(m.storage);
    let finish: () => void = () => undefined; const gate = new Promise<void>((r) => { finish = r; });
    const real = (m.storage as unknown as { fs: { atomicWriteFile: (p: string, b: Uint8Array) => Promise<void> } }).fs;
    const w = real.atomicWriteFile; real.atomicWriteFile = async (p, b) => { await gate; return w(p, b); };
    void storage!.fs.atomicWriteFile('app:/data/podcasts-library.json', new TextEncoder().encode('A-LIB')).catch(() => undefined);
    await wait(5);
    const out = signOut(); await wait(40);
    let done = false; void out.then(() => { done = true; }); await wait(20);
    expect(done).toBe(false);                                                                          // the Clear is waiting for A's started write
    finish(); await out; await idle();
    expect(m.files.has('app:/data/podcasts-library.json')).toBe(false);                                // the write landed BEFORE the wipe, and the wipe took it
  });
});

describe('M2 a request is signed only with the key of its world', () => {
  test('the Keychain changed under a live world: the chain\'s next request is refused, nothing is sent', async () => {
    boot({});
    const sent: string[] = [];
    globalThis.fetch = (async (u: string, init: RequestInit) => {
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 });
      sent.push(JSON.stringify(init?.headers ?? {})); return new Response('{}', { status: 200 });
    }) as never;
    await call('pagesView', [0]);                                                                      // the gate opens world A for key A
    await api.authedJSON('/api/one');
    expect(sent.length).toBe(1); expect(sent[0]).toContain(KEY_A);
    W.value = KEY_B;                                                                                    // the Keychain item is replaced: no gate has run yet
    const r = await Promise.race([api.authedJSON('/api/two').then(() => 'sent', (e) => (e as Error).name), wait(50).then(() => 'refused-or-hung')]);
    expect(r).not.toBe('sent'); expect(sent.length).toBe(1); expect(sent.join('')).not.toContain(KEY_B);
  });
});

describe('M3 the inventory of module-level mutable state', () => {
  const dir = new URL('.', import.meta.url).pathname;
  const files = readdirSync(dir).filter((f) => f.endsWith('.ts') && !/\.test\.ts$|\.d\.ts$|^account-phone\.ts$|^build-info\.gen\.ts$/.test(f));
  const found: Record<string, string[]> = {};
  for (const f of files) {
    const names: string[] = [];
    for (const line of readFileSync(dir + f, 'utf8').split('\n')) {
      const mm = /^(?:export )?(?:let|var) ([A-Za-z_]\w*)/.exec(line) ?? /^(?:export )?const ([A-Za-z_]\w*)(?:: [^=]+)? = new (?:Map|Set|WeakMap|WeakSet)\b/.exec(line);
      if (mm && !/^[A-Z][A-Z0-9_]*$/.test(mm[1])) names.push(mm[1]);
    }
    if (names.length) found[f] = names;
  }
  // WHY each may stay (the mechanism makes a closed world's chains inert, and a wipe clears what an account's chains put here): `wipe` = cleared by forget()/the wipe list; `device` = the phone's own
  // facts, not an account's; `public` = public data keyed by public addresses; `world` = the seam's own; `tests` = reset by tests only.
  const INVENTORY: Record<string, string[]> = {
    'account.ts': ['adopting', 'chain', 'clearing', 'wipes', 'boundaryId', 'lastFailed', 'active'],                  // the boundary's own machinery
    'api.ts': ['reader', 'nativeAnswered', 'lastNative', 'lastFlags', 'world', 'generation', 'blockedDigest', 'discovered', 'discovering', 'rejected', 'keyReads', 'closedWorlds'],   // the seam: world, key, session
    'app.ts': ['takesOverlay'],                                                                           // the takes overlay: its own generation guard drops it whole (take-session.ts sync)
    'bar.ts': ['seen'], 'clips.ts': ['byID', 'askedFeeds', 'askedAll', 'lastTick'],                       // seen: keyed by generation; clips: public, wipe
    'deck-ops.ts': ['posting'], 'demo-leftovers.ts': ['attempt'], 'files.ts': ['recents', 'lastNow', 'resolved'],   // posting: a flag; attempt: once per process; files: wipe
    'host.ts': ['decided', 'inflight'], 'mail-sanitize.ts': ['lastTarget'], 'mail-seal.ts': ['sealable'],       // device facts
    'mail.ts': ['kept', 'autoRead', 'drafts', 'savedDrafts', 'alias', 'clock', 'started'],                // wipe
    'pages.ts': ['pages', 'recency', 'lastClosed', 'loaded', 'loading', 'epoch', 'saving'],               // wipe (its own epoch)
    'paper-model.ts': ['KEYS', 'PADDED'], 'paper.ts': ['kept', 'lastTick'],                               // public
    'podcast.ts': ['generation', 'lib', 'feeds', 'known', 'failed', 'attempted', 'inflight', 'transcripts', 'transcriptAsked', 'searches', 'latestQuery', 'suggestions', 'suggestionsAt', 'syncAt', 'syncNote', 'lastTick', 'rev', 'reportedFeeds', 'reportedNow', 'played', 'nowSeconds', 'libGen'],   // wipe
    'runsheet-entries.ts': ['longURLs'], 'search.ts': ['liveVectors'], 'content.ts': ['liveStores', 'running'], 'contract-rules.ts': ['roleFaces'],       // keyed by entry id (run sheet lane); live stores wipe themselves
    'runsheet-store.ts': [], 'textures.ts': ['last'], 'themes.ts': ['loading', 'current', 'status', 'saving', 'dirty', 'migrated', 'hadChoiceFile', 'choiceTouched', 'migrating', 'migrateTries', 'rev'],   // device settings; Texture themes dropped by the wipe
  };
  test('every module-level mutable name is in the inventory, with its file (a new one fails until someone lists it and says why)', () => {
    const unlisted: string[] = [];
    for (const [f, names] of Object.entries(found)) for (const nm of names) if (!(INVENTORY[f] ?? []).includes(nm)) unlisted.push(`${f}: ${nm}`);
    expect(unlisted).toEqual([]);
  });
  test('raw fetch, raw timers on an account chain, and the Keychain session read stay in their choke points', () => {
    const bad: string[] = [];
    for (const f of files) {
      const src = readFileSync(dir + f, 'utf8');
      if (f !== 'api.ts' && /[^.\w]fetch\(/.test(src.replace(/\/\/.*$/gm, '')) && !/^(files|podcast)\.ts$/.test(f)) bad.push(`${f}: fetch( outside api.ts`);
      if (f !== 'api.ts' && f !== 'account.ts' && /keptSession\(|op: 'session'/.test(src)) bad.push(`${f}: reads the session key itself`);
    }
    expect(bad).toEqual([]);
  });
});

describe('M4 one writer: a moved boundary skips what was queued, a hung wipe is a failed Clear', () => {
  test('a queued Adopt or Clear whose boundary moved before it ran does not run', async () => {
    boot('none' === 'none' ? { 'app:/data/takes.json': 'x' } : {}, false);
    W = newWorld('present', KEY_A); W.signOut = 'hang';
    const out = signOut(); await wait(15);                                                              // a sign-out hangs in native: the chain is busy
    const ran: string[] = [];
    const q = (account as unknown as { job: (f: () => Promise<void>, o?: { skipIfMoved: boolean }) => Promise<void> }).job;
    expect(typeof q).toBe('function');
    const queued = q(async () => { ran.push('adopt'); }, { skipIfMoved: true });
    api.nextGeneration();                                                                               // the boundary moves while it waits
    W.signOut = 'ok'; W.release?.(); await out.catch(() => undefined); await queued; await idle();
    expect(ran).toEqual([]);
  });
  test('a native wipe that never returns is a failed Clear at its deadline: the record stays, no second Clear, the card says to close and reopen the app', async () => {
    boot({ 'app:/data/takes.json': 'x' }); W = newWorld('absent'); W.wipe = 'hang'; account.timing.nativeMs = 40;
    await call('pagesView', [0]).catch(() => undefined); await wait(10); await call('pagesView', [0]).catch(() => undefined);
    expect(W.log).toEqual(['wipe']);                                                                    // inside the deadline: one Clear, not two
    await wait(80); await idle();
    expect(m.files.has(STAMP)).toBe(true);                                                              // nothing marked clean
    const { accountState } = await import('./account-state.ts');
    const c = await accountState('t', '', 'native', 5, m.storage, undefined);
    expect(c.message).toContain('close and reopen');
    expect((account as Record<string, unknown>).abandonClear).toBeUndefined();
  });
});

describe('C9 named regressions', () => {
  test('C9.podcast-send A\'s sync is in flight when the sign-out completes: no request is made under B\'s key', async () => {
    const lib = { subscriptions: [{ show: { feedURL: 'https://a.example/feed.xml', title: 'A-PRIVATE-SHOW', author: '', artworkURL: '', summary: '' }, at: 1 }], removed: [], synced: [], positions: {}, clock: 0 };
    boot({ 'app:/data/podcasts-library.json': JSON.stringify(lib) });
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const sent: string[] = []; let phase = 'A';
    globalThis.fetch = (async (u: string, init: RequestInit) => {
      const url = String(u);
      if (url.endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c-' + phase, current: true }] }), { status: 200 });
      if (url.includes('/api/podcast/subscriptions') && (init?.method ?? 'GET') === 'GET' && phase === 'A') { await gate; return new Response(JSON.stringify({ subscriptions: [] }), { status: 200, headers: { date: 'Sat, 21 Nov 2026 12:00:00 GMT' } }); }
      if (url.startsWith('https://choir.news')) sent.push(`${phase} ${url} ${JSON.stringify(init?.headers ?? {})} ${String(init?.body ?? '')}`);
      return new Response('{}', { status: 404 });
    }) as never;
    const a = call('podcastHome', ['', 'podcasts:', 0, 'x']).then(() => 'answered', (e) => (e as Error).name);
    await wait(25);
    expect((await signOut()).done).toBe(true);
    W.key = 'present'; W.value = KEY_B; phase = 'B';
    await call('pagesView', [0]);                                                                       // B adopts
    release(); expect(await a).toBe('AccountClosedError'); await wait(40);
    expect(sent.filter((s) => s.startsWith('B ') && (s.includes('A-PRIVATE-SHOW') || s.includes('a.example')))).toEqual([]);
    expect(sent.filter((s) => s.includes(KEY_B) && s.includes('a.example'))).toEqual([]);
  });
  test('C9.take-session A\'s kept takes, read late, are not B\'s: no ids, handle or muted list, and nothing mirrored to native', async () => {
    boot({ 'app:/data/takes.json': JSON.stringify({ ids: ['take-of-a'], handle: 'a-handle-0001', muted: ['voice-muted-a1'] }) });
    const mirrored: string[] = [];
    const rec = { phase: 'idle', ended: '', seconds: 0, level: 0, transcript: '', canRecord: true, reason: '' };
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/takes.json', new Promise<void>((r) => { release = r; }));
    const a = call('takesPage', [rec, 'x']).then(() => 'answered', (e) => (e as Error).name);
    await wait(20);
    expect((await signOut()).done).toBe(true);
    W.key = 'present'; W.value = KEY_B; m.faults.hold.clear(); release();
    expect(await a).toBe('AccountClosedError'); await wait(30);
    const b = JSON.stringify(await call('takesPage', [rec, 'y']));
    expect(b).not.toContain('take-of-a'); expect(b).not.toContain('a-handle-0001'); expect(b).not.toContain('voice-muted-a1');
    expect(mirrored).toEqual([]); expect(m.files.has('app:/data/takes.json')).toBe(false);
  });
});


describe('C9b seat 2\'s second round, under the world mechanism', () => {
  test('C9.queued-signout a Clear this phone tapped while another job runs does not delete the key of an account that arrived meanwhile', async () => {
    boot({ 'app:/data/takes.json': 'x' }); W = newWorld('absent'); W.wipe = 'hang'; account.timing.nativeMs = 50;
    await call('pagesView', [0]).catch(() => undefined); await wait(10);                                // A's Clear is running (native hangs)
    const out = signOut();                                                                               // the person taps Clear this phone: it queues behind the Clear
    await wait(5);
    W.key = 'present'; W.value = KEY_B; W.wipe = 'ok';                                                   // B's key arrives; the Clear ends at its deadline
    const r = await out;
    expect(r.done).toBe(false);                                                                          // nothing was signed out: the key it saw at its turn is not the one it saw at the tap
    expect(W.key).toBe('present'); expect(W.log.filter((x) => x === 'signOut')).toEqual([]);
  });
  test('C9.gate-adopt a hung sign-out does not leave gated pages hanging: the page is refused at once (ADOPT with another job running)', async () => {
    boot({}, false); W.signOut = 'hang';
    const out = signOut(); await wait(15);
    const r = await Promise.race([call('pagesView', [0]).then(() => 'answered', (e) => (e as Error).name), wait(400).then(() => 'hung')]);
    expect(r).toBe('AccountClosedError');
    W.signOut = 'ok'; W.release?.(); await out;
  });
  test('C9.stale-wipe a shared wipe that timed out is detached, not cancelled: B cannot adopt (the Clear cannot finish) while it may still land, and nothing of B\'s is removed', async () => {
    boot({ 'app:/data/takes.json': 'x', 'app:/data/podcasts-library.json': 'x' }); W = newWorld('absent'); account.timing.sharedMs = 40;
    m.faults.hangRm.add('app:/data/takes.json');
    await call('pagesView', [0]).catch(() => undefined); await wait(80); await idle();                  // attempt 1: the shared half hangs and times out
    expect(m.files.has(STAMP)).toBe(true);
    W.key = 'present'; W.value = KEY_B;
    for (let i = 0; i < 3; i++) { await call('pagesView', [0]).catch(() => undefined); await wait(60); await idle(); }
    expect(m.files.has(STAMP)).toBe(true);                                                               // still A's record: B is not adopted while A's removal may land
    expect((await call('pagesOpen', ['mail:b', 'B']).then(() => 'wrote', (e) => (e as Error).name))).toBe('AccountClosedError');   // and B wrote nothing
    m.faults.hangRm.clear(); m.faults.releaseRm(); await wait(20);                                      // the host lets go: the removal lands (on A's data, before any B data exists)
    await call('pagesView', [0]).catch(() => undefined); await idle();
    expect(m.text(STAMP)).toBe(dig(KEY_B));
    expect((await call('pagesOpen', ['mail:b', 'B']).then(() => 'wrote', (e) => (e as Error).name))).toBe('wrote');
    await wait(50); await idle(); expect(m.text(PAGES_FILE)).toContain('mail:b');                       // B's data survives
  });
  test('C9.texture-theme A\'s Texture theme is not shown while the boundary is owed, and a load held across the Clear does not bring it back', async () => {
    const T = await import('./themes.ts');
    const ember = JSON.stringify(T.parseTextureTheme(JSON.parse(readFileSync(new URL('./fixtures/texture/theme-ember.json', import.meta.url), 'utf8'))).theme);
    boot({ 'app:/data/themes/texture-ember.json': ember }); T.resetThemes();
    expect(JSON.stringify(await call('themeList', ['light', false, 'x']))).toContain('ember');          // an open boundary shows it
    T.resetThemes();
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/themes/texture-ember.json', new Promise<void>((r) => { release = r; }));
    const held = call('themeList', ['light', false, 'x']).catch(() => undefined); await wait(15);       // a load in flight
    m.faults.hangRm.add('app:/data/themes/texture-ember.json');                                          // the Clear cannot remove it yet: the phone stays CLEARING
    const out = signOut(); await wait(30);
    m.faults.hold.clear(); release(); await held; await wait(20);
    expect(JSON.stringify(await call('themeList', ['light', false, 'x']))).not.toContain('ember');      // not shown while owed
    m.faults.hangRm.clear(); m.faults.releaseRm(); await out; await idle();
  });
  test('C9.poster-batch work held outside a chain (a queue of posts) carries its world: once that world is closed the request is refused, and nothing is sent', async () => {
    boot({});
    const sent: string[] = [];
    globalThis.fetch = (async (u: string, init: RequestInit) => {
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 });
      sent.push(JSON.stringify(init?.headers ?? {}) + String(init?.body ?? '')); return new Response('{}', { status: 200 });
    }) as never;
    await call('pagesView', [0]);
    const wA = api.currentWorld();                                                                       // the batch is captured in world A
    await api.authedJSON('/api/queue', { method: 'POST', body: { batch: 1 }, world: wA });
    expect(sent.length).toBe(1);
    expect((await signOut()).done).toBe(true);
    W.key = 'present'; W.value = KEY_B; await call('pagesView', [0]);                                    // B adopts: the Keychain now holds B's key
    const r = await api.authedJSON('/api/queue', { method: 'POST', body: { batch: 2 }, world: wA }).then(() => 'sent', (e) => (e as Error).name);
    expect(r).toBe('SignedOutError'); expect(sent.length).toBe(1);                                      // refused, and never signed with B's key
    expect(sent.join('')).not.toContain(KEY_B);
  });
});

describe('C8 a late read plants nothing after the wipe (now by the mechanism, with no per-site guard)', () => {
  test('C8.files A\'s recent files, read late, are not B\'s', async () => {
    boot({ 'app:/data/recent-files.json': JSON.stringify([{ path: '/A-private-notes.txt', size: 1, modified: null, openedAt: 1 }]) });
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/recent-files.json', new Promise<void>((r) => { release = r; }));
    const a = call('fileList', ['/', 'recent', 'name', false, '', 'x']).then(() => 'answered', (e) => (e as Error).name);
    await wait(15); expect((await signOut()).done).toBe(true); m.faults.hold.clear(); release();
    expect(await a).toBe('AccountClosedError'); await wait(15);
    W.key = 'present'; W.value = KEY_B;
    expect(JSON.stringify(await call('fileList', ['/', 'recent', 'name', false, '', 'x']))).not.toContain('A-private-notes');
  });
  test('C8.podcast A\'s library, read late, is not B\'s', async () => {
    const lib = { subscriptions: [{ show: { feedURL: 'https://a.example/feed.xml', title: 'A-PRIVATE-SHOW', author: '', artworkURL: '', summary: '' }, at: 1 }], removed: [], synced: [], positions: {}, clock: 0 };
    boot({ 'app:/data/podcasts-library.json': JSON.stringify(lib) });
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/podcasts-library.json', new Promise<void>((r) => { release = r; }));
    const a = call('podcastHome', ['', 'podcasts:', 0, 'x']).then(() => 'answered', (e) => (e as Error).name);
    await wait(15); expect((await signOut()).done).toBe(true); m.faults.hold.clear(); release();
    expect(await a).toBe('AccountClosedError'); await wait(15);
    W.key = 'present'; W.value = KEY_B;
    expect(JSON.stringify(await call('podcastHome', ['', 'podcasts:', 0, 'x']))).not.toContain('A-PRIVATE-SHOW');
  });
  test('C8.clips a public reply that lands after the boundary moved is not merged', async () => {
    boot({}); const body = JSON.parse(readFileSync(new URL('../fixtures/podcasts/clips.json', import.meta.url), 'utf8'));
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async () => { await gate; return new Response(JSON.stringify(body), { status: 200 }); }) as never;
    void CL.showClips('podcast:show/' + encodeURIComponent('https://feeds.example.com/show.xml'), 0);
    await wait(5); api.nextGeneration(); CL.forget(); release(); await wait(20);
    expect(CL.remembered()).toBe(0);
  });
  test('C8.demo the demo pass of a closed world writes nothing back', async () => {
    boot({ 'app:/data/recent-files.json': JSON.stringify([{ path: '/Readme.txt', size: 55, modified: 1_700_000_000_000 - 30 * 86_400_000, openedAt: 1 }, { path: '/A-mine.txt', size: 1, modified: null, openedAt: 2 }]) });
    m.files.delete('app:/data/demo-leftovers-cleared');
    const { clearDemoLeftovers, resetDemoLeftovers } = await import('./demo-leftovers.ts'); resetDemoLeftovers();
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/recent-files.json', new Promise<void>((r) => { release = r; }));
    void clearDemoLeftovers(api.bindStorage(m.storage)); await wait(15);
    api.nextGeneration(); m.files.delete('app:/data/recent-files.json'); m.faults.hold.clear(); release(); await wait(20);
    expect(m.files.has('app:/data/recent-files.json')).toBe(false);
  });
});
