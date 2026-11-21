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
    const wChain = api.currentWorld();
    const chain = (async () => {
      const bytes = await storage!.fs.readFile('app:/data/a-private.json');                            // I/O 1: held
      cache.set('after-read', new TextDecoder().decode(bytes));                                         // an insert
      await storage!.fs.atomicWriteFile('app:/data/planted-by-a.json', new TextEncoder().encode('A-SECRET'));      // a write
      cache.set('after-write', 'x');
      await api.authedJSON('/api/platform/send', { method: 'POST', body: { secret: 'A-SECRET' }, world: wChain });   // a send
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
    let finish: () => void = () => undefined; const gate = new Promise<void>((r) => { finish = r; });
    const real = (m.storage as unknown as { fs: { atomicWriteFile: (p: string, b: Uint8Array) => Promise<void> } }).fs;
    const w = real.atomicWriteFile; real.atomicWriteFile = async (p, b) => { await gate; return w(p, b); };
    const storage = api.bindStorage(m.storage);                                                        // bound after the patch: the copy holds the functions it was made from
    void storage!.fs.atomicWriteFile('app:/data/podcasts-library.json', new TextEncoder().encode('A-LIB')).catch(() => undefined);
    await wait(5);
    const out = signOut(); await wait(40);
    let done = false; void out.then(() => { done = true; }); await wait(20);
    expect(done).toBe(false);                                                                          // the Clear is waiting for A's started write
    finish(); await out; await idle();
    expect(m.files.has('app:/data/podcasts-library.json')).toBe(false);                                // the write landed BEFORE the wipe, and the wipe took it
  });
});

describe('M1c bindStorage on the host\'s storage', () => {
  test('a storage object whose `fs` is read-only (the host\'s, found by `test web`) is bound without assignment', async () => {
    const raw = Object.freeze({ fs: Object.freeze({ readFile: async () => new Uint8Array([1]), atomicWriteFile: async () => undefined }) });
    const bound = api.bindStorage(raw as never) as typeof raw;
    expect((await bound.fs.readFile()).length).toBe(1);
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
    await api.authedJSON('/api/one', { world: api.currentWorld() });
    expect(sent.length).toBe(1); expect(sent[0]).toContain(KEY_A);
    W.value = KEY_B;                                                                                    // the Keychain item is replaced: no gate has run yet
    const r = await Promise.race([api.authedJSON('/api/two', { world: api.currentWorld() }).then(() => 'sent', (e) => (e as Error).name), wait(50).then(() => 'refused-or-hung')]);
    expect(r).not.toBe('sent'); expect(sent.length).toBe(1); expect(sent.join('')).not.toContain(KEY_B);
  });
});

describe('M3 module state belongs to the world: no module-level mutable state outside a short allow-list', () => {
  const dir = new URL('.', import.meta.url).pathname;
  const files = readdirSync(dir).filter((f) => f.endsWith('.ts') && !/\.test\.ts$|\.d\.ts$|^account-phone\.ts$|^build-info\.gen\.ts$/.test(f));
  // What the test SEES: every top-level `let`/`var`, and every top-level `const` whose initializer builds something that can change (new X(...), [], {}, an object or array literal), ALL-CAPS names included.
  const seen = (): Record<string, string[]> => {
    const found: Record<string, string[]> = {};
    for (const f of files) for (const line of readFileSync(dir + f, 'utf8').split('\n')) {
      const mm = /^(?:export )?(?:let|var) ([A-Za-z_]\w*)/.exec(line) ?? /^(?:export )?const ([A-Za-z_]\w*)(?:: [^=]+)? = (?:new [A-Z]\w*(?:<[^(]*>)?\(|\[\]|\{\}|\{|\[)/.exec(line);
      if (mm) (found[f] ??= []).push(mm[1]);
    }
    return found;
  };
  // STATEFUL: allowed, each with the reason it is not the world's. A module that keeps account-scoped state uses `scoped()` (api.ts) instead and shows up nowhere here.
  const STATEFUL: Record<string, Record<string, string>> = {
    'account.ts': { chain: 'the boundary\'s own machinery: the one writer', clearing: 'the one writer', wipes: 'the one writer', boundaryId: 'the one writer', lastFailed: 'the one writer', adopting: 'the one writer', active: 'the one writer', timing: 'deadlines (tests shorten them)' },
    'api.ts': { reader: 'the seam: the native handle', nativeAnswered: 'the seam', lastNative: 'the seam (a log line)', lastFlags: 'the seam (a log line)', world: 'THE WORLD', closedWorlds: 'the worlds whose started writes a Clear waits for', blockedDigest: 'the seam: sealing block', discovered: 'digest-keyed computer id', discovering: 'digest-keyed computer id', rejected: 'the refused key digest (cleared at sign-out)', keyTiming: 'deadline (tests shorten it)', keyReads: 'a counter for the log' },
    'demo-leftovers.ts': { attempt: 'keyed by world: an attempt of a closed world is replaced' },
    'host.ts': { decided: 'the host fact (is there a native package): the device\'s', inflight: 'the host probe' }, 'mail-sanitize.ts': { lastTarget: 'the last link target the sanitizer allowed: not read across answers' },
    'mail-seal.ts': { sealable: 'whether the platform can seal: the device\'s' },
    'paper-model.ts': { KEYS: 'a memo of a pure function of its argument', PADDED: 'a memo of a pure function of its argument' },
    'podcast.ts': { search: 'a test knob (the pause before a search)' },
    'search.ts': { liveVectors: 'live stores wipe themselves (wipeLiveVectors)', embedPrompt: 'a function', f32: 'a function' },
    'content.ts': { liveStores: 'live stores wipe themselves (wipeLiveContent)' },
    'themes.ts': { loading: 'device settings: the phone\'s theme store, Texture themes hidden while the boundary is owed', current: 'device settings', status: 'device settings', saving: 'device settings', dirty: 'device settings', migrated: 'device settings', hadChoiceFile: 'device settings', choiceTouched: 'device settings', migrating: 'device settings', migrateTries: 'device settings', rev: 'device settings' },
  };
  // CONSTANTS: read-only tables and objects. The test checks that nothing in their own file changes them.
  const CONSTANTS: Record<string, string[]> = {"account.ts": ["WIPES"], "activity.ts": ["LABELS", "MONTHS"], "api.ts": ["KEYCHAIN_ACCOUNTS", "NEVER", "READS"], "app.ts": ["sources", "EXEMPT"], "clips.ts": ["NONE"], "content.ts": ["KINDS", "KIND_LABEL", "STANDARD_BUDGET", "TEXT_EXTENSIONS"], "contract-rules.ts": ["roleFaces"], "deck.ts": ["DECK_NAMES", "EMPTY_ITEM"], "demo-leftovers.ts": ["STORIES", "TEXTURES", "MAILBOXES", "MESSAGES", "FILES"], "edition-model.ts": ["HOUSE", "TREE", "ALIASES", "TRIM", "FILLERS", "ADD_VERBS", "REMOVE_VERBS", "FINISH", "QUESTION_LEAD", "NOTHING", "NEGATIVE_LEAD"], "files-model.ts": ["IMAGE", "VIDEO", "AUDIO", "MARKDOWN", "TEXT", "CSV", "CODE", "LABELS", "SYMBOLS", "MIME", "MONTHS"], "files.ts": ["realNet"], "mail-model.ts": ["FOLDERS", "ENTITIES", "DROPPED", "BLOCK_END", "DAYS", "MONTHS", "OFFLINE"], "mail-sanitize.ts": ["ALLOWED_TAGS", "VOID_TAGS", "RAW_TEXT", "DROP_WITH_CONTENT", "GLOBAL_ATTRS", "NAMED"], "mail-seal.ts": ["MAGIC"], "notebook.ts": ["HELD"], "paper-layout.ts": ["FALLBACK_STEP"], "paper-model.ts": ["DAYS", "MONTHS", "HOUSE", "FINER", "ALIASES", "TERMS", "STOP"], "podcast-feed.ts": ["INLINE"], "podcast-model.ts": ["LINEBREAK", "MONTHS", "MONTH_NAMES", "ZONES"], "podcast.ts": ["enc", "NO_PLAY", "zone"], "programme-model.ts": ["SEGMENT_LABELS", "STRIPPED"], "radio-director.ts": ["SESSION_RETRY_DELAYS"], "radio-model.ts": ["SMALL", "TENS", "DAYS", "MONTHS"], "radio-page.ts": ["EMPTY_FACTS"], "runsheet-entries.ts": ["enc"], "runsheet-model.ts": ["KINDS", "BYS", "STATES", "OUTCOMES", "ROLES", "OPS", "ATS", "Dropped", "Bounds", "enc"], "runsheet-store.ts": ["NO_REPORT"], "search.ts": ["BUDGET", "STOP", "PRONOUNS", "JOB_BUDGET"], "take-session.ts": ["IDLE", "PHASES", "ENDED", "RECORDER_OPS"], "takes.ts": ["BOUNDS", "STRIPPED", "enc", "NEW"], "textures.ts": ["MONTHS", "real"], "themes-bundled.ts": ["BUNDLED"], "themes.ts": ["FACES", "GENERIC", "TYPE_SCALES", "UNSAFE", "RGB_COMMA", "RGB_SPACE", "MODES", "realNet"], "words.ts": ["WORDS"], "xml.ts": ["NAMED"]};
  test('every module-level mutable-looking name is allowed, and the allow-list has no stale entry', () => {
    const found = seen(); const unlisted: string[] = []; const stale: string[] = [];
    for (const [f, names] of Object.entries(found)) for (const nm of names) if (!(nm in (STATEFUL[f] ?? {})) && !(CONSTANTS[f] ?? []).includes(nm)) unlisted.push(`${f}: ${nm}`);
    for (const [f, m] of Object.entries(STATEFUL)) for (const nm of Object.keys(m)) if (!(found[f] ?? []).includes(nm)) stale.push(`${f}: ${nm}`);
    for (const [f, a] of Object.entries(CONSTANTS)) for (const nm of a) if (!(found[f] ?? []).includes(nm)) stale.push(`${f}: ${nm}`);
    expect(unlisted).toEqual([]); expect(stale).toEqual([]);
  });
  test('a name allowed as a constant is never changed in its own file', () => {
    const changed: string[] = [];
    for (const [f, names] of Object.entries(CONSTANTS)) {
      const src = readFileSync(dir + f, 'utf8');
      for (const nm of names) if (new RegExp(`\\b${nm}\\.(set|add|push|pop|shift|splice|delete|clear|sort|reverse)\\(|\\b${nm}\\s*(=|\\+=)[^=>]|\\b${nm}\\[[^\\]]+\\]\\s*=[^=]|\\b${nm}\\.\\w+\\s*=[^=>]`).test(src.replace(/^(?:export )?const [^\n]*\n/gm, (l) => (l.includes(' ' + nm + ' ') || l.includes(' ' + nm + ':') ? '' : l)))) changed.push(`${f}: ${nm}`);
    }
    expect(changed).toEqual([]);
  });
  test('the scoped state of a module is what a new world starts without: a world\'s bag is fresh and a closed one is unreachable', () => {
    api.resetSeam(); const S = api.scoped(() => ({ n: 0 }));
    S().n = 5; const w1 = api.currentWorld(); api.nextGeneration();
    expect(S().n).toBe(0); expect(api.currentWorld()).not.toBe(w1); S().n = 2; S.reset(); expect(S().n).toBe(0);
  });
  // The native ops that are NOT inert for a closed world, each with why a late answer cannot carry account content into the next world (everything an account chain asks native goes through nativeInert).
  const NATIVE_RAW: Record<string, string> = {
    'api.ts': 'the Keychain: keystate, session, get. An answer is the key or a digest of it; every request then checks the key against its world\'s digest, and a late one is for a world that is closed',
    'account.ts': 'the boundary\'s own wipe and signOut: the answer is done or failed, no account content',
    'host.ts': 'the host probe (is there a native package): a device fact',
    'themes.ts': 'themeLegacy: the old build\'s device theme files; a texture-* file is not written once the world is closed or owed',
  };
  test('every module that calls the raw native `later` is listed with its reason, and no other', () => {
    const using = files.filter((f) => /\bnativeLater\(\)/.test(readFileSync(dir + f, 'utf8').replace(/\/\/.*$/gm, ''))).sort();
    expect(using).toEqual(Object.keys(NATIVE_RAW).sort());
  });
  test('raw fetch, raw native calls and the Keychain session read stay in their choke points', () => {
    const bad: string[] = [];
    for (const f of files) {
      const src = readFileSync(dir + f, 'utf8').replace(/\/\/.*$/gm, '');
      if (f !== 'api.ts' && /[^.\w]fetch\(/.test(src) && !/^(files|podcast)\.ts$/.test(f)) bad.push(`${f}: fetch( outside api.ts`);
      if (f !== 'api.ts' && f !== 'account.ts' && /keptSession\(|op: 'session'/.test(src)) bad.push(`${f}: reads the session key itself`);
      if (!/^(api|account|host|themes)\.ts$/.test(f) && /\bnativeLater\(\)/.test(src)) bad.push(`${f}: nativeLater() on an account chain (use nativeInert)`);
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
    const rec = { phase: 'idle', ended: '', seconds: 0, level: 0, transcript: '', canRecord: true, reason: '' };
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/takes.json', new Promise<void>((r) => { release = r; }));
    const a = call('takesPage', [rec, 'x']).then(() => 'answered', (e) => (e as Error).name);
    await wait(20);
    expect((await signOut()).done).toBe(true);
    W.key = 'present'; W.value = KEY_B; m.faults.hold.clear(); release();
    expect(await a).toBe('AccountClosedError'); await wait(30);
    const b = JSON.stringify(await call('takesPage', [rec, 'y']));
    expect(b).not.toContain('take-of-a'); expect(b).not.toContain('a-handle-0001'); expect(b).not.toContain('voice-muted-a1');
    expect(W.mirrors.join('')).not.toContain('take-of-a'); expect(W.mirrors.join('')).not.toContain('a-handle-0001'); expect(W.mirrors.join('')).not.toContain('voice-muted-a1'); expect(m.files.has('app:/data/takes.json')).toBe(false);
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
  test('C9.texture-theme-loaded A\'s Texture theme already in memory is not shown the moment a sign-out begins (before any wipe step runs)', async () => {
    const T = await import('./themes.ts');
    const ember = JSON.stringify(T.parseTextureTheme(JSON.parse(readFileSync(new URL('./fixtures/texture/theme-ember.json', import.meta.url), 'utf8'))).theme);
    boot({ 'app:/data/themes/texture-ember.json': ember }); T.resetThemes();
    expect(JSON.stringify(await call('themeList', ['light', false, 'x']))).toContain('ember');
    W.signOut = 'hang'; const out = signOut(); await wait(20);                                           // the world is closed and owed; native has not answered; no wipe has run
    expect(JSON.stringify(await call('themeList', ['light', false, 'x']))).not.toContain('ember');
    W.signOut = 'ok'; W.release?.(); await out;
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

describe('C10 seat 1\'s round on c222cbf80: module state belongs to the world, the gate captures its world first, native is a choke point', () => {
  test('C10.pages-save a sign-out during a page write: B\'s first page change completes (the old save chain does not outlive its world)', async () => {
    boot({ 'app:/data/open-pages-shared.json': JSON.stringify({ pages: [], recency: [] }) }); account.timing.sharedMs = 40;
    let release: () => void = () => undefined; m.faults.writeHold.set(PAGES_FILE, new Promise<void>((r) => { release = r; }));
    void call('pagesOpen', ['mail:a', 'A']).catch(() => undefined); await wait(15);                      // A's save is in flight
    await signOut(); m.faults.writeHold.clear(); release(); await wait(20);                              // the sign-out ends (its Clear waited for the write, then failed or finished)
    W.key = 'present'; W.value = KEY_B;
    let r = 'refused';
    for (let i = 0; i < 6 && r !== 'wrote'; i++) { r = await Promise.race([call('pagesOpen', ['mail:b', 'B']).then(() => 'wrote', (e) => (e as Error).name), wait(400).then(() => 'hung')]); await idle(); }
    expect(r).toBe('wrote');
    expect(m.text(PAGES_FILE)).toContain('mail:b');
  });
  test('C10.adopt-mid-read an Adopt while A\'s first read of the open pages is in flight: B\'s first answer completes (a single-flight load of the closed world is not joined)', async () => {
    boot({ 'app:/data/open-pages-shared.json': JSON.stringify({ pages: [], recency: [] }) }, false); W = newWorld('absent'); n = phone(() => W); api.bindNative(n as never);
    let release: () => void = () => undefined; m.faults.hold.set(PAGES_FILE, new Promise<void>((r) => { release = r; }));
    void call('pagesView', [0]).catch(() => undefined); await wait(15);                                 // EMPTY world: the read is held
    W.key = 'present'; W.value = KEY_B; m.faults.hold.clear();
    const r = await Promise.race([call('pagesView', [0]).then(() => 'answered', (e) => (e as Error).name), wait(500).then(() => 'hung')]);
    release();
    expect(r).toBe('answered');
  });
  test('C10.gate-world the boundary moving during the facts read: the answer is refused and the NEW world keeps no digest and stays owed', async () => {
    boot({}); let go: () => void = () => undefined; W.keystateGate = new Promise<void>((r) => { go = r; });
    const a = call('pagesView', [0]).then(() => 'answered', (e) => (e as Error).name); await wait(15);
    api.nextGeneration();                                                                                // the boundary moves while the facts are read
    const w1 = api.currentWorld(); go();
    expect(await a).toBe('AccountClosedError');
    expect(w1.digest).toBeUndefined(); expect(w1.owed).toBe(true);
  });
  test('C10.native a native answer that arrives after the boundary moved never reaches the chain that asked (nativeInert)', async () => {
    boot({}); let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    api.bindNative({ available: true, later: async () => { await gate; return { ok: true }; } } as never);
    const later = (api as unknown as { nativeInert: () => ((r: object) => Promise<unknown>) | null }).nativeInert();
    expect(typeof later).toBe('function');
    const p = later!({ op: 'x' }).then(() => 'resumed', () => 'rejected');
    await wait(5); api.nextGeneration(); release();
    expect(await Promise.race([p, wait(60).then(() => 'never')])).toBe('never');
  });
  test('C10.slow-wipe a timed-out attempt\'s detached wipe is progressing slowly: after attempt 2 closed its world and B adopted, it removes nothing of B\'s', async () => {
    boot({ 'app:/data/podcasts-library.json': 'x', 'app:/data/takes.json': 'x' }); W = newWorld('absent'); n = phone(() => W); api.bindNative(n as never);
    account.timing.sharedMs = 40; let go: () => void = () => undefined; m.faults.rmGate.set('app:/data/podcasts-library.json', new Promise<void>((r) => { go = r; }));
    await call('pagesView', [0]).catch(() => undefined); await wait(90); await idle();                  // attempt 1: its first removal is slow; it times out and is detached
    expect(m.files.has(STAMP)).toBe(true);
    account.timing.sharedMs = 2000; W.key = 'present'; W.value = KEY_B;
    void call('pagesView', [0]).catch(() => undefined);                                                  // attempt 2 starts (and closes attempt 1's world)
    await wait(100);
    const early = await call('pagesOpen', ['mail:b1', 'B1']).then(() => 'wrote', (e) => (e as Error).name);   // B may already be adopted here (only if nothing waits for the slow removal)
    go();                                                                                                // the slow removal lands now
    await wait(120); await idle();
    if (early === 'wrote') expect(m.text(PAGES_FILE) ?? '').toContain('mail:b1');                      // B's data written before it landed survives the detached wipe
    let r = 'refused';
    for (let i = 0; i < 20 && r !== 'wrote'; i++) { r = await call('pagesOpen', ['mail:b2', 'B2']).then(() => 'wrote', (e) => (e as Error).name); await wait(25); await idle(); }
    expect(r).toBe('wrote'); await wait(150); await idle();
    expect(m.text(PAGES_FILE)).toContain('mail:b2');
  });
});

describe('C11 seat 2\'s third round', () => {
  test('C11.migrate-texture the old-theme carry-over (exempt, raw storage) writes no Texture theme once the boundary moved: A\'s Texture theme does not reach B', async () => {
    const T = await import('./themes.ts');
    const ember = JSON.stringify(T.parseTextureTheme(JSON.parse(readFileSync(new URL('./fixtures/texture/theme-ember.json', import.meta.url), 'utf8'))).theme);
    boot({}); T.resetThemes(); W.legacy = { 'texture-ember.json': ember }; let go: () => void = () => undefined; W.legacyGate = new Promise<void>((r) => { go = r; });
    const held = call('themeMigrate', ['native', 0]).catch(() => undefined); await wait(20);
    expect((await signOut()).done).toBe(true);
    go(); await held; await wait(30);
    expect(m.files.has('app:/data/themes/texture-ember.json')).toBe(false);
  });
  test('C11.download a download in flight when the world closes: the Clear waits for it and the file does not stay (a host download is not a storage write)', async () => {
    boot({}); account.timing.sharedMs = 2000;
    const dest = 'app:/cache/files/x/a.bin'; let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async (u: string) => {
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 });
      await gate; m.files.set(dest, new TextEncoder().encode('A-BYTES'));                                 // the host writes the file as the body arrives
      const r = new Response('', { status: 200 }); Object.defineProperty(r, 'exactBodyTo', { value: { path: dest, bytes: 7 } }); return r;
    }) as never;
    await call('pagesView', [0]);
    const bound = api.bindStorage(m.storage);
    void api.authedToFile('/api/files/a.bin', dest, bound, { world: api.currentWorld() }).catch(() => undefined); await wait(20);
    const out = signOut(); await wait(30); release();
    expect((await out).done).toBe(true); await idle();
    expect(m.files.has(dest)).toBe(false);
  });
  test('C11.record-write the record write and the record removal have deadlines: a hung one is a failed job, never a wedged writer', async () => {
    boot({ 'app:/data/takes.json': 'x' }, false); account.timing.recordMs = 40;
    let rel: () => void = () => undefined; m.faults.writeHold.set(STAMP, new Promise<void>((r) => { rel = r; }));
    const a = await Promise.race([call('pagesView', [0]).then(() => 'answered', (e) => (e as Error).name), wait(500).then(() => 'hung')]);
    expect(a).toBe('AccountClosedError');                                                                // the Adopt timed out: refused, not wedged
    const card = await Promise.race([account.peek(m.storage, undefined).then((p) => p.state), wait(500).then(() => 'hung')]); expect(card).not.toBe('hung');
    const so = await Promise.race([signOut().then((r) => r.done), wait(1500).then(() => 'hung')]); expect(so).not.toBe('hung');   // and the person's way out still runs
    rel();
    boot({ 'app:/data/takes.json': 'x' }); W = newWorld('absent'); n = phone(() => W); api.bindNative(n as never); account.timing.recordMs = 40; account.timing.sharedMs = 200; m.faults.hangRm.add(STAMP);
    await call('pagesView', [0]).catch(() => undefined); await wait(400); await idle();
    expect(m.files.has(STAMP)).toBe(true);                                                               // a hung removal: the record stays, the state still refuses
    const so2 = await Promise.race([signOut().then((r) => r.done), wait(1500).then(() => 'hung')]); expect(so2).not.toBe('hung');
    m.faults.hangRm.clear(); m.faults.releaseRm();
  });
  test('C11.epoch a world close with no state change (a sign-out that did not finish) moves the card\'s epoch, so refused pages are asked again', async () => {
    boot({}); const e0 = (await account.peek(m.storage, undefined)).epoch;
    api.nextGeneration(); const e1 = (await account.peek(m.storage, undefined)).epoch;
    expect(e1).not.toBe(e0);
  });
});

describe('C12 seat 1\'s residuals on 0ced3a4ee', () => {
  test('C12.adopt-drain an Adopt write that timed out and lands after A signed out and B adopted does not wipe B: the Adopt waits for the closed world\'s writes, and does not stack a second', async () => {
    boot({}, false); account.timing.recordMs = 40; account.timing.sharedMs = 60;
    let release: () => void = () => undefined; m.faults.writeHold.set(STAMP, new Promise<void>((r) => { release = r; }));
    expect(await call('pagesView', [0]).then(() => 'answered', (e) => (e as Error).name)).toBe('AccountClosedError');   // A's Adopt write is hung: refused
    await call('pagesView', [0]).catch(() => undefined);                                                 // a retry in the same world
    expect(events.filter((e) => e === 'writecall:' + STAMP).length).toBe(1);                             // it did not stack a second write
    expect((await signOut()).done).toBe(true);                                                           // A signs out: record none, key absent (EMPTY), no Clear
    W.key = 'present'; W.value = KEY_B;
    expect(await call('pagesOpen', ['mail:b', 'B']).then(() => 'wrote', (e) => (e as Error).name)).toBe('AccountClosedError');   // B's Adopt waits: A's write may still land
    m.faults.writeHold.clear(); release(); await wait(30);                                               // A's late write lands: the record names A, the key is B
    let r = 'refused';
    for (let i = 0; i < 12 && r !== 'wrote'; i++) { r = await call('pagesOpen', ['mail:b', 'B']).then(() => 'wrote', (e) => (e as Error).name); await wait(30); await idle(); }
    expect(r).toBe('wrote'); await wait(100); await idle();
    expect(m.text(STAMP)).toBe(dig(KEY_B)); expect(m.text(PAGES_FILE)).toContain('mail:b');             // B's data survives
  });
  test('C12.rm-tree every folder wipe works on a host that refuses to remove a non-empty folder', async () => {
    boot({ 'app:/data/podcast-feeds/f1.json': 'x', 'app:/cache/files/h/a.bin': 'x', 'app:/tmp/picked/p.bin': 'x', 'app:/data/vectors/v1.bin': 'x', 'app:/data/mail/msg-1.bin': 'x', 'app:/cache/mail-1.html': 'x' });
    W = newWorld('absent'); n = phone(() => W); api.bindNative(n as never); m.faults.strictDirs = true;
    await call('pagesView', [0]).catch(() => undefined); await idle();
    expect([...m.files.keys()].filter((k) => k !== STAMP)).toEqual([]); expect(m.files.has(STAMP)).toBe(false); expect((await account.peek(m.storage, undefined)).state).toBe('empty');
  });
  test('C12.theme-wedge a gated Texture install whose first theme-store load is cut by an Adopt (no wipe runs) does not wedge the device theme store', async () => {
    const T = await import('./themes.ts'); boot({ 'app:/data/theme-choice.json': '{}' }, false); W = newWorld('absent'); n = phone(() => W); api.bindNative(n as never); T.resetThemes();
    let release: () => void = () => undefined; m.faults.hold.set('app:/data/theme-choice.json', new Promise<void>((r) => { release = r; }));
    void call('themeFromTexture', ['doc-1']).catch(() => undefined); await wait(20);                  // an EMPTY-world chain holds the first load
    W.key = 'present'; W.value = KEY_A; await call('pagesView', [0]).catch(() => undefined);            // the Adopt ends that world; nothing is wiped
    m.faults.hold.clear(); release(); await wait(20);
    expect(await Promise.race([call('themeList', ['light', false, 'x']).then(() => 'answered', () => 'rejected'), wait(500).then(() => 'hung')])).not.toBe('hung');
  });
  test('C12.world-required a signed-in request that names no world is refused', async () => {
    boot({});
    await expect((api.authedJSON as unknown as (p: string) => Promise<unknown>)('/api/x')).rejects.toThrow(/world/);
  });
  test('C12.download-closed a download that ends after its world closed does not remove the file itself (the Clear removes the cache by path)', async () => {
    boot({});
    const dest = 'app:/cache/files/x/a.bin'; let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async (u: string) => {
      if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'c', current: true }] }), { status: 200 });
      await gate; m.files.set(dest, new TextEncoder().encode('B-OWN'));
      const r = new Response('', { status: 200 }); Object.defineProperty(r, 'exactBodyTo', { value: { path: dest, bytes: 5 } }); return r;
    }) as never;
    await call('pagesView', [0]);
    void api.authedToFile('/api/files/a.bin', dest, api.bindStorage(m.storage), { world: api.currentWorld() }).catch(() => undefined); await wait(15);
    api.nextGeneration(); release(); await wait(30);
    expect(events.filter((e) => e === 'rm:' + dest)).toEqual([]);
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
