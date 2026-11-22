// THE WORLD, THE ROOT AND THE INVENTORY (docs/design/account-boundary-2026-11-21.md, "Accounts are partitions"). Station 1. Failure list, written first:
//  R1  the handle re-roots: every path maps under `app:/<space>/accounts/<d>/`, and `..`, `accounts/`, the device paths, a path outside the three spaces and a function that is not a plain one are refused (I1).
//  R2  `sqlite.open` and the `dest` of `authedToFile` are re-rooted too; the picker's `app:/tmp/picked` passes read-only and unprefixed.
//  R3  a world with no account (none, or after a sign-out) binds nothing; a closed world's calls are inert; the bag of one world is not the next's.
//  R4  the device handle reaches the device paths and nothing else, and takes no world (I4); `openAccountStorage` is read-only unless asked, and only while an account world is open.
//  R5  a request is signed only with the key of its world, and an anonymous or closed world signs nothing (I2).
//  M3  module state belongs to the world: no module-level mutable binding outside the allow-list, each with its reason (I3). `new World(` only in api.ts; raw storage only in api.ts and account.ts (I1); raw fetch and the key read stay
//      in their choke points (I2); the raw native `later` is listed per module.
//  L2  every `app:/` literal in a module is a device path, the picker's, or has its top-level form in LEFTOVERS (so a store that is not re-rooted fails here).
//  I7  every gated resource of the contracts takes the account handle, or is `transient`; the device sources that read Texture themes take it too.
import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import * as api from './api.ts';
import { LEFTOVERS, LEFTOVER_PREFIXES } from './account.ts';
import { sources, DEVICE } from './app.ts';
import { bindings, isTable, mutations, provablyImmutable, type Binding } from './module-scan.ts';
import { dig, KEY_A, KEY_B, memFs } from './account-phone.ts';

const D = dig(KEY_A); const ROOT = `accounts/${D}`;
const settle = (p: Promise<unknown>): Promise<unknown> => Promise.race([p.then(() => undefined, () => undefined), new Promise((r) => setTimeout(r, 30))]);
const denies = async (p: Promise<unknown>): Promise<boolean> => p.then(() => false, (e: { code?: string }) => e?.code === 'denied');
afterEach(() => { api.resetSeam(); });
beforeEach(() => { api.resetSeam(); });

describe('R re-rooting and the handle', () => {
  test('R1 every path maps under the account\'s root; the refusals', async () => {
    const m = memFs(); Object.assign(m.storage.fs, { watch: async () => undefined }); const w = api.openWorld(D); const s = api.bindStorage(m.storage as never, w);
    await s.fs.atomicWriteFile('app:/data/takes.json', new Uint8Array([1]));
    await s.fs.atomicWriteFile('app:/cache/files/a/b.bin', new Uint8Array([2]));
    await s.fs.atomicWriteFile('app:/tmp/scratch', new Uint8Array([3]));
    await s.fs.mkdir('app:/data/mail');
    expect([...m.files.keys()].sort()).toEqual([`app:/cache/${ROOT}/files/a/b.bin`, `app:/data/${ROOT}/takes.json`, `app:/tmp/${ROOT}/scratch`]);
    expect(new TextDecoder().decode(await s.fs.readFile('app:/data/takes.json'))).toBe('\u0001');
    expect(await s.fs.readdir('app:/cache/files')).toEqual(['a']);
    for (const bad of ['app:/data/../cache/x', 'app:/data/./x', 'app:/data/%2e%2e/x', `app:/data/accounts/${dig(KEY_B)}/takes.json`, 'app:/data/accounts', 'app:/cache/accounts/x', 'app:/data/theme-choice.json', 'app:/data/themes/mine.json', 'app:/data/current-account', 'app:/elsewhere/x', 'file:///etc/passwd', 'app:/data x', 'app:/datax/y']) {
      expect(await denies(s.fs.readFile(bad))).toBe(true); expect(await denies(s.fs.atomicWriteFile(bad, new Uint8Array([9])))).toBe(true);
    }
    expect([...m.files.keys()].length).toBe(3);
    expect(await denies((s.fs as unknown as { watch: (p: string) => Promise<unknown> }).watch('app:/data/x'))).toBe(true);          // not a plain one-path call
  });
  test('R1b a module cannot name another account\'s root: whatever it names gets the prefix', async () => {
    const m = memFs({ [`app:/data/accounts/${dig(KEY_B)}/takes.json`]: 'B' }); const s = api.bindStorage(m.storage as never, api.openWorld(D));
    expect(await denies(s.fs.readFile(`app:/data/accounts/${dig(KEY_B)}/takes.json`))).toBe(true);
    await expect(s.fs.readFile('app:/data/takes.json')).rejects.toMatchObject({ code: 'ENOENT' });
  });
  test('R1c two-path calls re-root both; a copy from the picker keeps its source', async () => {
    const m = memFs({ 'app:/tmp/picked/p.pdf': 'pdf' }); const calls: string[][] = [];
    const raw = { fs: { copyFile: async (a: string, b: string) => { calls.push([a, b]); }, rename: async (a: string, b: string) => { calls.push([a, b]); } }, sqlite: {} } as never;
    const s = api.bindStorage(raw, api.openWorld(D)) as unknown as { fs: { copyFile(a: string, b: string): Promise<void>; rename(a: string, b: string): Promise<void> } };
    await s.fs.copyFile('app:/tmp/picked/p.pdf', 'app:/data/x.pdf'); await s.fs.rename('app:/data/a', 'app:/data/b');
    expect(calls).toEqual([['app:/tmp/picked/p.pdf', `app:/data/${ROOT}/x.pdf`], [`app:/data/${ROOT}/a`, `app:/data/${ROOT}/b`]]); void m;
  });
  test('R2 sqlite.open is re-rooted; the picker passes read-only and unprefixed; authedToFile\'s dest is re-rooted', async () => {
    const opened: string[] = []; const m = memFs({ 'app:/tmp/picked/p.pdf': 'pdf' });
    const raw = { fs: m.storage.fs, sqlite: { open: async (p: string) => { opened.push(p); return {}; }, drop: async () => undefined } } as never;
    const w = api.openWorld(D); const s = api.bindStorage(raw, w);
    await s.sqlite.open('app:/data/content.db'); expect(opened).toEqual([`app:/data/${ROOT}/content.db`]);
    await expect((s.sqlite as unknown as { open(p: string): Promise<unknown> }).open('app:/data/../x.db')).rejects.toMatchObject({ code: 'denied' });
    expect(await denies((s.sqlite as unknown as { drop(): Promise<unknown> }).drop())).toBe(true);
    expect(new TextDecoder().decode(await s.fs.readFile('app:/tmp/picked/p.pdf'))).toBe('pdf');              // read, unprefixed
    await s.fs.atomicWriteFile('app:/tmp/picked/q.pdf', new Uint8Array([1]));                              // a write is never the host's input: it is re-rooted like any other
    expect(m.files.has(`app:/tmp/${ROOT}/picked/q.pdf`)).toBe(true); expect(m.files.has('app:/tmp/picked/q.pdf')).toBe(false);
    await s.fs.rm('app:/tmp/picked/p.pdf'); expect(m.files.has('app:/tmp/picked/p.pdf')).toBe(false);       // removed after the upload
    expect(api.rootPath(w, 'app:/cache/files/x/y.mp3')).toBe(`app:/cache/${ROOT}/files/x/y.mp3`);
    expect(api.appPath('app:/cache/files/x/y.mp3')).toBe(`app:/cache/${ROOT}/files/x/y.mp3`);
  });
  test('R2b authedToFile hands the host the re-rooted dest, and removes that file on a failure', async () => {
    const seen: { dest?: string }[] = []; const removed: string[] = [];
    api.holdSession(KEY_A, '', 'c1');
    globalThis.fetch = (async (_u: unknown, init: { exactBodyTo?: string }) => { seen.push({ dest: init.exactBodyTo }); return new Response('', { status: 500 }); }) as never;
    const w = api.openWorld(D);
    const storage = { fs: { rm: async (p: string) => { removed.push(p); } } } as never;
    await expect(api.authedToFile('/api/files/a', 'app:/cache/files/k/a.mp3', storage, { world: w })).rejects.toBeDefined();
    expect(seen[0].dest).toBe(`app:/cache/${ROOT}/files/k/a.mp3`); expect(removed).toEqual([`app:/cache/${ROOT}/files/k/a.mp3`]);
    await expect(api.authedToFile('/api/files/a', 'app:/data/x', storage, { world: w })).rejects.toThrow(/under app:\/cache/);
  });
});

describe('R3 worlds', () => {
  test('R3 a process with no world binds nothing; the anonymous world has its own root; after a sign-out no world opens', async () => {
    const m = memFs(); const none = api.bindStorage(m.storage as never);
    expect(await denies(none.fs.readFile('app:/data/x'))).toBe(true); expect(await denies(none.fs.atomicWriteFile('app:/data/x', new Uint8Array([1])))).toBe(true);
    const anon = api.openWorld('anonymous'); await api.bindStorage(m.storage as never, anon).fs.atomicWriteFile('app:/data/paper-front.json', new Uint8Array([1]));
    expect([...m.files.keys()]).toEqual(['app:/data/accounts/anonymous/paper-front.json']);
    expect(api.openWorld('anonymous')).toBe(anon);                                      // asking again is the same world
    const acct = api.openWorld(D); expect(anon.closed).toBe(true); expect(acct.closed).toBe(false);
    expect(() => api.openWorld(dig(KEY_B))).toThrow(/one account world/);
    expect(() => api.openWorld('not-a-digest')).toThrow(/64 lowercase hex/);
    api.endWorld(); expect(acct.closed).toBe(true); expect(() => api.openWorld('anonymous')).toThrow(/ended/);
  });
  test('M1 a closed world\'s storage, requests, timers and native calls never settle; its bag is not the next world\'s', async () => {
    const m = memFs({ [`app:/data/${ROOT}/a`]: 'a' }); const w = api.openWorld(D); const s = api.bindStorage(m.storage as never, w);
    api.bindNative({ available: true, later: async () => ({ ok: true }) } as never);
    const S = api.scoped(() => ({ n: 0 })); S().n = 5; expect(S.of(w).n).toBe(5);
    const writes = m.writes.length; const calls: string[] = []; globalThis.fetch = (async (u: unknown) => { calls.push(String(u)); return new Response('{}'); }) as never;
    const nat = api.nativeInert(); const timer = api.inertTimeout(5, w); const get = api.getJSON('/api/public');
    api.endWorld();
    const after = [s.fs.atomicWriteFile('app:/data/b', new Uint8Array([1])), s.fs.readFile('app:/data/a'), api.authedJSON('/api/x', { world: w }), api.authedText('/api/x', { world: w }), api.authedBytes('/api/x', { world: w }), nat?.({ op: 'x' }), timer, get].map((p) => settle(p ?? Promise.resolve()));
    await Promise.all(after);
    expect(m.writes.length).toBe(writes); expect(calls.filter((u) => u.includes('/api/x'))).toEqual([]);
    api.resetSeam(); const w2 = api.openWorld(D); expect(S.of(w2).n).toBe(0); expect(S().n).toBe(0);
  });
});

describe('R4 the device handle', () => {
  test('R4 reaches the device paths and nothing else, with no world; sees nothing under accounts/ and no root', async () => {
    const m0 = memFs(); void m0;
    const m = memFs({ [`app:/data/${ROOT}/takes.json`]: 'SECRET', 'app:/data/theme-choice.json': '{"theme":"x"}', 'app:/data/themes/a.json': '{}' });
    Object.assign(m.storage.fs, { rename: async () => undefined }); const d = api.deviceStorage(m.storage as never);
    expect(api.isDeviceStorage(d)).toBe(true); expect(api.isDeviceStorage(m.storage)).toBe(false);
    expect(new TextDecoder().decode(await d.fs.readFile('app:/data/theme-choice.json'))).toContain('theme'); expect(await d.fs.readdir('app:/data/themes')).toEqual(['a.json']);
    await d.fs.atomicWriteFile('app:/data/theme-choice.json', new TextEncoder().encode('{}'));
    for (const bad of [`app:/data/${ROOT}/takes.json`, 'app:/data/accounts', 'app:/data/takes.json', 'app:/cache/files/x', 'app:/data/themes/../x', 'app:/data/theme-choice.json.bak', 'app:/data/themesx']) expect(await denies(d.fs.readFile(bad))).toBe(true);
    expect(await denies((d.fs as unknown as { rename(a: string, b: string): Promise<void> }).rename('app:/data/themes/a.json', 'app:/data/themes/b.json'))).toBe(true);
    expect(api.openAccountStorage(d)).toBeUndefined();                                // no account world open: no Texture theme
  });
  test('R4c the device handle opens no database (P8): the content store is the account\'s', async () => {
    const opened: string[] = []; const m = memFs();
    const raw = { fs: m.storage.fs, sqlite: { open: async (p: string) => { opened.push(p); return {}; } } } as never;
    const d = api.deviceStorage(raw) as unknown as { sqlite: { open(p: string): Promise<unknown> } };
    expect(await denies(d.sqlite.open(`app:/data/accounts/${dig(KEY_B)}/content.db`))).toBe(true); expect(await denies(d.sqlite.open('app:/data/theme-choice.json'))).toBe(true); expect(opened).toEqual([]);
  });
  test('R4d an already-bound handle is unwrapped, not bound twice, and no message carries a path or a digest (P5)', async () => {
    const m = memFs(); const w = api.openWorld(D); const bound = api.bindStorage(m.storage as never, w);
    const again = api.openAccountStorage(bound, true)!; await again.fs.atomicWriteFile('app:/data/x', new Uint8Array([1])); expect(m.files.has(`app:/data/${ROOT}/x`)).toBe(true);
    const e = await bound.fs.readFile(`app:/data/${ROOT}/x`).catch((x: Error) => x); expect(String((e as Error).message)).not.toMatch(/accounts|[0-9a-f]{64}/);
  });
  test('R4b the Texture themes are read from the OPEN account world, read-only unless asked, and not after the world closes', async () => {
    const m = memFs({ [`app:/data/${ROOT}/theme-textures/texture-1.json`]: 'T' }); const d = api.deviceStorage(m.storage as never);
    const w = api.openWorld(D); const ro = api.openAccountStorage(d)!;
    expect(await ro.fs.readdir('app:/data/theme-textures')).toEqual(['texture-1.json']); expect(await denies(ro.fs.atomicWriteFile('app:/data/theme-textures/x.json', new Uint8Array([1])))).toBe(true); expect(await denies(ro.fs.rm('app:/data/theme-textures/texture-1.json'))).toBe(true);
    await api.openAccountStorage(d, true)!.fs.atomicWriteFile('app:/data/theme-textures/texture-2.json', new Uint8Array([1])); expect(m.files.has(`app:/data/${ROOT}/theme-textures/texture-2.json`)).toBe(true);
    api.endWorld(); expect(w.closed).toBe(true); expect(api.openAccountStorage(d)).toBeUndefined();
  });
});

describe('R5 a request is signed only with the key of its world', () => {
  const key = { v: KEY_A };
  beforeEach(() => { key.v = KEY_A; api.holdSession(KEY_A, '', 'c1'); });
  test('R5 the key of the account world signs; another key, an anonymous world and a world with no account sign nothing', async () => {
    const sent: string[] = []; globalThis.fetch = (async (_u: unknown, init: { headers: Record<string, string> }) => { sent.push(init.headers.Authorization); return new Response('{}', { status: 200 }); }) as never;
    const none = api.currentWorld(); await expect(api.authedJSON('/api/x', { world: none })).rejects.toMatchObject({ name: 'SignedOutError' });
    const anon = api.openWorld('anonymous'); await expect(api.authedJSON('/api/x', { world: anon })).rejects.toMatchObject({ name: 'SignedOutError' });
    const w = api.openWorld(D); await api.authedJSON('/api/x', { world: w }); expect(sent).toEqual([`Bearer ${KEY_A}`]);
    key.v = KEY_B; api.holdSession(KEY_B, '', 'c1'); await expect(api.authedJSON('/api/x', { world: w })).rejects.toMatchObject({ name: 'SignedOutError' }); expect(sent.length).toBe(1);      // the Keychain changed under a live world
    await expect(api.authedJSON('/api/x', {} as never)).rejects.toThrow(/must state its world/);
  });
  test('R5b the computer id is the world\'s: another world never meets the first one\'s lookup', async () => {
    const hdr: string[] = []; globalThis.fetch = (async (u: unknown, init: { headers: Record<string, string> }) => { if (String(u).endsWith('/api/compute/status')) return new Response(JSON.stringify({ computers: [{ computer_id: 'disc-1' }] })); hdr.push(init.headers['X-Choir-Computer']); return new Response('{}'); }) as never;
    api.holdSession(KEY_A, '', '');
    const w = api.openWorld(D); await api.authedJSON('/api/x', { world: w }); expect(hdr).toEqual(['disc-1']);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- the inventory (M3)
const dir = new URL('.', import.meta.url).pathname;
const files = readdirSync(dir).filter((f) => f.endsWith('.ts') && !/\.test\.ts$|\.d\.ts$|^account-phone\.ts$|^module-scan\.ts$|^gen-build-info-preload\.ts$|^build-info\.gen\.ts$/.test(f));
const text = (f: string): string => readFileSync(dir + f, 'utf8');
const RAWFETCH = /(?<![.\w$])fetch\b/;      // any `fetch` identifier: a call, `globalThis.fetch` is caught by the `.`-less match below, a reference (`void fetch(`, `c ? fetch(`, `&& fetch(`)
const RAWGLOBAL = /\bglobalThis\s*\.\s*fetch\b|\bwindow\s*\.\s*fetch\b|\bself\s*\.\s*fetch\b/;
const stripped = (f: string): string => text(f).replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '');

describe('M3 module state belongs to the world: no module-level mutable state outside a short allow-list', () => {
  // Every top-level binding of every non-test module (module-scan.ts: a small lexer) is (a) provably immutable by its text (a primitive, a regular expression, a function, a symbol, `scoped(...)`: the world's own state),
  // (b) a literal table that nothing changes, or (c) on the list below with the reason it cannot hold a string, id, promise or object derived from account data. Anything else fails until someone lists it.
  const STATEFUL: Record<string, Record<string, string>> = {
    'account.ts': { lastWhy: 'a log line (fixed words, on change only)', lost: 'whether the card\'s re-reads ran out with the marker set: a boolean', pendingRevoke: 'the key id and key of a sign-out in flight, held ONLY until the revoke call that sends it (the key is the one being revoked; it goes to ORIGIN only)', timing: 'deadlines (tests shorten them)', swept: 'whether the launch sweep ran: a boolean', lockedWhy: 'fixed words naming why the launch is locked (starting)', markerWritten: 'whether this process wrote the ending mark: a boolean', finishP: 'the one native sign-out in flight: a promise of the native answer `done` (a boolean), no account data', endedWhy: 'fixed words naming why the session ended (signedout, unfinished, blocked): no account data', launching: 'the one launch of this process: a promise that settles ready or locked, holds no value', chain: 'the one job chain: a sign-out runs alone; resolves to nothing' },
    'api.ts': {
      held: 'THE KEY IS EXACT\'S: what the answer\'s store gave (adoptStore) for the chains of this process, dropped at a sign-out and never adopted after the session ended; the key is signed only for its world (send)',
      keyTiming: 'a deadline (tests shorten it)',
      reader: 'the seam: the native handle (the module), no account data', nativeAnswered: 'the seam: whether a native op ever answered', lastNative: 'a log line',
      handleSeen: 'whether a native handle object has existed in this process: a boolean',
      world: 'THE WORLD: the process\'s one world slot (api.ts openWorld is the only writer)', generation: 'a counter that moves when a world opens or closes', sessionOver: 'whether the session ended', NEVER: 'a promise that never settles: holds no value',
    },
    'signin.ts': { status: 'fixed words of the sign-in flow', userCode: 'the code the person types on the approval page (shown on screen, no account data)', verificationURL: 'the approval page address (no account data)', message: 'a sentence', flow: 'the device flow in progress: the device code (a credential for this one approval, never returned) and counters', serial: 'a counter that moves when a flow is cancelled or replaced' },
    'radio-bridge.ts': { lastPulse: 'a log-dedupe string (the pulse state word), no account data', holding: 'a boolean: one radioHold at a time, cleared in a finally' },
    'radio-runtime.ts': { HOLD_AHEAD: 'a number computed from the deck bound at load' },
    'onboarding.ts': { launchGen: 'the account generation of the first memory this process made: a number' },
    'clock.ts': { clock: 'the device clock the native facts set (a fact of the phone, never a string from an account)' },
    'host.ts': { decided: 'the host fact (is there a native package): the device\'s', inflight: 'the host probe: a promise of that fact' },
    'mail-seal.ts': { MAGIC: 'the three-byte file magic: read only', sealable: 'whether the platform can seal: the device\'s' },
    'notebook.ts': { K: 'the SHA-256 round constants: a read-only typed-array table' },
    'paper-model.ts': { KEYS: 'a memo of a pure function of its argument', PADDED: 'a memo of a pure function of its argument' },
    'search.ts': { BUDGET_CHARS: 'a number computed from the budget table at load', u32: 'with f32: a scratch float converter, set and read inside one synchronous function; holds one number, never data', f32: 'a scratch float converter' },
    'themes.ts': {
      loggedScheme: 'the last system scheme logged (a log-dedupe word)', TYPE_SCALES: 'a literal table of type scales', CHAN: 'a regular expression source template',
      loading: 'the DEVICE theme store (bundled themes, the person\'s own theme files, the choice): it holds no Texture theme (add refuses one, the loader skips texture- names)', current: 'the device theme store', saving: 'a write chain of the device choice file', dirty: 'a flag',
      migrated: 'a flag of the old-build carry-over', hadChoiceFile: 'a flag', choiceTouched: 'a flag', migrating: 'the old-build carry-over single-flight (the device handle)', migrateTries: 'a counter', rev: 'a counter',
    },
    'retrieval-run.ts': { 'Retrieval.COLD_WAIT_MS': 'a number (a deadline the tests shorten)', providers: 'a registry of local-search functions: it holds code, never data; the data a provider reads is the world\'s (retrieval-live.ts)', latest: 'a counter that drops a stale launcher query: one number' },
    'retrieval-live.ts': { "registerSearchProvider('________', (q) =": 'registers a function once at load; the copy it reads is the world\'s (`R()`)' },
    'reader.ts': { decoding: 'the serial queue of story decodes: a promise that settles to nothing, holds no value', handOver: 'a test seam: the hand-over function (holds code, never data)' },
    'imported.ts': { UPLOAD_MAX: 'a number', say: 'a function', entriesOf: 'an alias of a function' },
    'podcast-model.ts': { decode: 'an alias of a function' },
    'mail-store.ts': { STATE_FILE: 'a path template' },
    'app.ts': { grants: 'the grant text: a template of the origin constant' },
    'contract-rules.ts': { lab: 'the folder of the contract rules (a path)' },
    'paper-layout.ts': { DEFAULT_DOC: 'the bundled layout document (JSON import, cast): read only', DISTANT_PAST: 'a number' },
  };
  const seen = (): { found: Record<string, Binding[]>; unbalanced: string[] } => { const found: Record<string, Binding[]> = {}; const unbalanced: string[] = []; for (const f of files) { const r = bindings(text(f)); if (!r.balanced) unbalanced.push(f); found[f] = r.found; } return { found, unbalanced }; };
  test('the scanner sees the shapes of a hidden binding: a call, an initialiser on the next line, an IIFE-held Map, a class static, a destructuring, a mutated export', () => {
    const names = (src: string): string[] => bindings(src).found.filter((b) => !(b.kind === 'const' && provablyImmutable(b.init))).map((b) => b.name);
    expect(names('const x = factory();')).toEqual(['x']); expect(names('const cache =\n  new Map<string, string>();')).toEqual(['cache']);
    expect(names('const held = (() => { const m = new Map(); return m; })();')).toEqual(['held']); expect(names('class Box {\n  static seen = new Set<string>();\n  static make() { return 1; }\n}')).toEqual(['Box.seen']);
    expect(names('const { a, b: c } = load();\nlet p, q = 1;')).toEqual(['a', 'c', 'p', 'q']); expect(names('export const N = 5;\nconst RE = /a`b/g;\nconst f = (v: unknown): v is X => v === 1;\nconst s = scoped(() => ({}));\nconst t = `a ${1}`;')).toEqual(['t']);
    expect(mutations("import { TABLE } from './t.ts';\nTABLE.set('k', 1);", 'TABLE')).not.toEqual([]); expect(isTable("new Set(['a'])")).toBe(true); expect(isTable('new Set()')).toBe(false);
  });
  test('every top-level binding is provably immutable, an unchanged literal table, or allowed with its reason; the allow-list has no stale entry', () => {
    const { found, unbalanced } = seen(); expect(unbalanced).toEqual([]);
    const unlisted: string[] = []; const stale: string[] = []; const changed: string[] = [];
    for (const [f, bs] of Object.entries(found)) for (const b of bs) {
      if (b.kind === 'const' && provablyImmutable(b.init)) continue;
      if (STATEFUL[f]?.[b.name] !== undefined) continue;
      if (b.kind === 'const' && isTable(b.init)) {
        const mod = f.replace(/\.ts$/, '');
        for (const g of files) {
          const src = text(g); let hits: string[] = [];
          if (g === f) hits = mutations(src, b.name);
          else {
            const named = new RegExp(`import\\s*(?:type\\s*)?\\{[^}]*\\b${b.name}\\b[^}]*\\}\\s*from\\s*['"]\\./${mod}(?:\\.ts)?['"]`).test(src); const ns = new RegExp(`import\\s*\\*\\s*as\\s+(\\w+)\\s+from\\s*['"]\\./${mod}(?:\\.ts)?['"]`).exec(src);
            if (named) hits = mutations(src, b.name); if (ns) hits = hits.concat(mutations(src, b.name, ns[1]));
          }
          if (hits.length) changed.push(`${f}: ${b.name} is changed in ${g} (${hits[0]})`);
        }
        continue;
      }
      unlisted.push(`${f}: ${b.kind} ${b.name}`);
    }
    for (const [f, mm] of Object.entries(STATEFUL)) for (const nm of Object.keys(mm)) if (!(found[f] ?? []).some((b) => b.name === nm)) stale.push(`${f}: ${nm}`);
    expect(unlisted).toEqual([]); expect(changed).toEqual([]); expect(stale).toEqual([]);
  });
  test('I5 `new World(` appears only in api.ts, and only in the one-world function and the test reset', () => {
    const users = files.filter((f) => /new World\(/.test(stripped(f))); expect(users).toEqual(['api.ts']);
    expect((stripped('api.ts').match(/new World\(/g) ?? []).length).toBe(3);               // the slot at load, openWorld, resetWorld (tests)
  });
  test('I1 raw storage handles are made and used only in api.ts and account.ts', () => {
    const bad: string[] = [];
    for (const f of files) {
      const s = stripped(f);
      if (!/^(api|account)\.ts$/.test(f) && /\bunbound\b/.test(s)) bad.push(`${f}: unbound() (the raw storage)`);
      if (!/^(api|account)\.ts$/.test(f) && /\brmTree\b/.test(s)) bad.push(`${f}: rmTree`);
      if (!/^(api|themes|app|account)\.ts$/.test(f) && /\b(deviceStorage|openAccountStorage)\b/.test(s)) bad.push(`${f}: a device handle outside the theme sources and the gate`);
      if (!/^(api|app)\.ts$/.test(f) && /\bbindStorage\b/.test(s)) bad.push(`${f}: bindStorage outside the gate`);
    }
    expect(bad).toEqual([]);
  });
  test('I2 raw fetch, the Keychain session read and the raw native later stay in their choke points', () => {
    const NATIVE_RAW: Record<string, string> = {
      'api.ts': 'the Keychain: keystate, session, get; every request then checks the key against its world\'s digest', 'account.ts': 'the native sign-out: the answer is done or not, no account content',
      'host.ts': 'the host probe (is there a native package): a device fact', 'themes.ts': 'themeLegacy: the old build\'s device theme files (the device handle; a texture-* file is never written)',
      'content-run.ts': 'contentBusy: whether remote audio is on air, a device fact; no account data crosses',
    };
    const using = files.filter((f) => /\bnativeLater\b/.test(stripped(f).replace(/import \{[^}]*\} from '.\/api.ts';/g, ''))).sort(); expect(using).toEqual(Object.keys(NATIVE_RAW).sort());
    const bad: string[] = [];
    for (const f of files) {
      const s = stripped(f);
      if (f !== 'api.ts' && (RAWFETCH.test(s) || RAWGLOBAL.test(s))) bad.push(`${f}: fetch outside api.ts`);
      if (f !== 'api.ts' && f !== 'account.ts' && /keptSession\(|op: 'session'/.test(s)) bad.push(`${f}: reads the session key itself`);
      if (f !== 'api.ts' && /Authorization/.test(s)) bad.push(`${f}: names the Authorization header`);
    }
    expect(bad).toEqual([]);
  });
  test('the search-provider registration is one named statement (retrieval-live.ts, `imported`), not a prefix the allow-list would match twice', () => {
    const regs = files.filter((f) => /^registerSearchProvider\(/m.test(stripped(f)));
    expect(regs).toEqual(['retrieval-live.ts']); expect((stripped('retrieval-live.ts').match(/^registerSearchProvider\(/gm) ?? []).length).toBe(1); expect(text('retrieval-live.ts')).toContain("registerSearchProvider('imported'");
  });
  test('the fetch scan sees every shape of a raw fetch', () => {
    for (const bad of ['await fetch(u)', 'globalThis.fetch(u)', 'const f = c ? fetch(u) : 0', 'ok && fetch(u)', 'void fetch(u)', 'return fetch(u)', 'x(fetch)']) expect(RAWFETCH.test(bad) || RAWGLOBAL.test(bad)).toBe(true);
    for (const ok of ['backend.fetchBytes(p)', 'net.fetch ${ORIGIN}', 'refetch(x)', 'prefetch(x)']) expect(RAWFETCH.test(ok) || RAWGLOBAL.test(ok)).toBe(false);
  });
  test('the world\'s timers: a chain of an account waits on inertTimeout; the raw setTimeout users are the deadlines listed here', () => {
    const RAW_TIMER: Record<string, string> = {
      'api.ts': 'withDeadline and inertTimeout themselves', 'host.ts': 'the host probe\'s own wait (a device fact)', 'themes.ts': 'the device handle\'s waits', 'edition-writer.ts': 'a deadline that settles a race and is cleared with it',
      'runsheet-store.ts': 'the retry timer: its flush posts through the world\'s request, which a closed world refuses', 'retrieval-run.ts': 'a pause between slices (the module cannot import api.ts: an import cycle); its I/O is inert',
      'search.ts': 'a zero-length yield', 'content-run.ts': 'the BUSY deadline: a device fact',
    };
    const using = files.filter((f) => /\bsetTimeout\(/.test(stripped(f))).sort(); expect(using).toEqual(Object.keys(RAW_TIMER).sort());
  });
});

describe('L2 the leftovers cover every old top-level path', () => {
  test('every `app:/` literal of a module is a device path, the picker\'s, or its top-level form is in LEFTOVERS (so no store is forgotten when it is re-rooted)', () => {
    const device = [...api.DEVICE_PATHS.files, ...api.DEVICE_PATHS.dirs]; const found = new Set<string>();
    for (const f of files) for (const mt of text(f).replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '').matchAll(/['"`](app:\/(?:data|cache|tmp)(?:\/[^'"`$\s]*)?)/g)) found.add(mt[1]);
    const missing: string[] = [];
    for (const p of found) {
      if (p === 'app:/tmp/picked' || p.startsWith('app:/tmp/picked/')) continue;
      if (device.some((d) => p === d || p.startsWith(d + '/'))) continue;
      if (LEFTOVERS.some((l) => p === l || p.startsWith(l + '/'))) continue;
      if (LEFTOVER_PREFIXES.some(([d, pre]) => p === d || p.startsWith(`${d}/${pre}`))) continue;
      if (/^app:\/(data|cache|tmp)$/.test(p) || p === 'app:/cache/accounts' ) continue;
      if (f_isAccountsRoot(p)) continue;
      missing.push(p);
    }
    expect(missing).toEqual([]);
    for (const l of LEFTOVERS) expect(l).toMatch(/^app:\/(data|cache)\/[^/]+$/); for (const p of LEFTOVERS) expect(p.includes('accounts')).toBe(false);
  });
  test('the literal scan reads single, double and template quotes', () => {
    const re = /['"`](app:\/(?:data|cache|tmp)(?:\/[^'"`$\s]*)?)/g; const got = (t: string): string[] => [...t.matchAll(re)].map((x) => x[1]);
    expect(got(`a('app:/data/x.json')`)).toEqual(['app:/data/x.json']); expect(got('a("app:/cache/y")')).toEqual(['app:/cache/y']); expect(got('a(`app:/tmp/z-${n}.bin`)')).toEqual(['app:/tmp/z-']);
  });
  test('a module that builds a path from the cache folder and hands it out routes it through appPath (the host and the page see the root); the others only read', () => {
    const READ_ONLY = new Set(['imported-search.ts', 'files.ts']);      // files.ts defines CACHE_DIR and hands `local`/`src` out through appPath (checked below)
    const bad: string[] = [];
    for (const f of files) { const t = stripped(f); if (!/\$\{(?:F\.)?CACHE_DIR\}|\bCACHE_DIR\}/.test(t)) continue; if (READ_ONLY.has(f)) continue; if (!/\bappPath\b/.test(t)) bad.push(f); }
    expect(bad).toEqual([]); expect(stripped('files.ts')).toMatch(/appPath\(dest\)/);
  });
  test('LEFTOVERS holds no device path', () => { for (const l of LEFTOVERS) for (const d of [...api.DEVICE_PATHS.files, ...api.DEVICE_PATHS.dirs]) expect(l === d || l.startsWith(d + '/')).toBe(false); });
});
function f_isAccountsRoot(p: string): boolean { return /\/accounts(\/|$)/.test(p); }

// ---------------------------------------------------------------------------------------------------------------------------------------------- I7
// EVERY resource of EVERY contract (ours and the native package's) either has the account handle as a real argument (parsed, not "the line contains the word"), or ends its line with `transient` (Exact never keeps its answer:
// contract/lower/src/lib.rs `set_resource_transient`; runner/src/runner/kept_wait.rs, docs/reference/exact/internals.md "Kept answers"), or is on the short list below with the reason it holds no account data.
// The card `acs` and the resource its tick reads (`au`) are TRANSIENT, or a kept answer of the card would hand the previous launch's handle to every gated resource at the first frame (the derive reads the kept answer).
describe('I7 every resource takes the account handle, or is transient, or holds no account data', () => {
  const contracts = readdirSync(dir).filter((f) => f.endsWith('.contract') && !/test\.contract$/.test(f));
  type R = { file: string; name: string; src: string; args: string[]; transient: boolean; line: string };
  /** The top-level comma-separated arguments of a call that starts after `(` at `from` (template literals and brackets are balanced). */
  const argsOf = (l: string, from: number): string[] => {
    const out: string[] = []; let d = 1; let q = false; let at = from;
    for (let i = from; i < l.length; i++) {
      const c = l[i];
      if (c === '`') q = !q; else if (q) continue;
      else if ('([{'.includes(c)) d++; else if (')]}'.includes(c)) { d--; if (d === 0) { const last = l.slice(at, i).trim(); if (last !== '') out.push(last); return out; } }
      else if (c === ',' && d === 1) { out.push(l.slice(at, i).trim()); at = i + 1; }
    }
    throw new Error('unbalanced call: ' + l.slice(0, 80));
  };
  const all: R[] = []; let declared = 0;
  for (const f of contracts) for (const l of text(f).split('\n')) {
    if (/^\s*resource\s/.test(l)) declared += 1;
    const mt = /^\s*resource (\w+) = (\w+)\(/.exec(l); if (!mt) continue;
    all.push({ file: f, name: mt[1], src: mt[2], args: argsOf(l, mt[0].length), transient: /\btransient\s*$/.test(l), line: l });
  }
  const isHandle = (a: string): boolean => /^(?:\w+\s*=\s*)?acctHandle$/.test(a) || /^`[^`]*\$\{acctHandle\}[^`]*`$/.test(a);
  // No account content in these, each with its reason (the card, the host fact and the build rows are asked by the app before any account):
  const NO_ACCOUNT_DATA: Record<string, string> = {
    hpg: 'exactPage: whether the page is visible and focused, a fact of the screen', spVp: 'exactViewport: the display\'s size and keyboard', qVp: 'exactViewport: the display\'s size', vp: 'exactViewport: the display\'s size and the system scheme',
    hc: 'hostCaps: whether there is a native package, decided from the handle; the same for every account', bld: 'aboutBuild: the app\'s commit and the server\'s public /health',
  };
  test('the lister sees every resource declaration (so this test cannot pass on a form it cannot read)', () => { expect(declared).toBe(all.length); expect(all.length).toBeGreaterThan(70); expect(all.some((r) => !Object.hasOwn(sources, r.src))).toBe(true); expect(all.some((r) => r.src === 'fileList')).toBe(true); });
  test('each resource has the handle as an argument, is transient, or is listed with its reason; every list entry exists', () => {
    const bad = all.filter((r) => !r.args.some(isHandle) && !r.transient && NO_ACCOUNT_DATA[r.name] === undefined).map((r) => `${r.file}: ${r.name} = ${r.src}(${r.args.join(', ').slice(0, 60)})`);
    expect(bad).toEqual([]);
    for (const k of Object.keys(NO_ACCOUNT_DATA)) expect(all.some((r) => r.name === k)).toBe(true);
  });
  test('the card and what its tick reads are transient: the handle is derived from the card', () => {
    const acs = all.find((r) => r.name === 'acs')!; expect(acs.transient).toBe(true);
    expect(all.find((r) => r.name === 'au')!.transient).toBe(true);
    expect(text('account.contract')).toMatch(/derive acctHandle = acs\.handle/); expect(text('account.contract')).not.toMatch(/acctTick|signedOutTick/);
    expect(acs.line).not.toMatch(/\bpg\./);                                                         // a page argument would make the handle depend on the page that takes it: a cycle
  });
  test('the sources of ours that read Texture themes take the handle; the mail resources are transient too (D8)', () => {
    for (const s of ['theme', 'themeList', 'themeCards', 'themeExtras']) { const l = all.filter((x) => x.src === s); expect(l.length).toBeGreaterThan(0); for (const x of l) expect(x.args.some(isHandle)).toBe(true); }
    for (const s of ['mailBox', 'mailFrame', 'mailSync', 'mailMessage', 'mailCompose']) for (const x of all.filter((y) => y.src === s)) expect(x.transient).toBe(true);
  });
  test('the argument parser reads the forms the contracts use, and a resource without the handle is caught', () => {
    expect(argsOf('resource a = f(x, `${y}|${acctHandle}`, z=acctHandle, g(1, 2)) as shape S', 'resource a = f('.length)).toEqual(['x', '`${y}|${acctHandle}`', 'z=acctHandle', 'g(1, 2)']);
    expect(isHandle('acctHandle')).toBe(true); expect(isHandle('acct=acctHandle')).toBe(true); expect(isHandle('`${n}|${acctHandle}`')).toBe(true);
    expect(isHandle('acctHandleX')).toBe(false); expect(isHandle('`acctHandle`')).toBe(false); expect(isHandle('f(acctHandle)')).toBe(false);
  });
  test('the Sign out offered from a locked phone is two steps: the words say it removes this phone\'s copy, with Remove and Cancel', () => {
    const c = text('signedout.contract');
    expect(c).toContain("This removes this account's data from this phone. What is saved in your Choir computer stays.");
    expect(c).toMatch(/testId="signedout-remove"/); expect(c).toMatch(/testId="signedout-cancel"/);
    expect(c).toMatch(/acs\.alsoSignOut and confirmRemove == false\n\s+button press=askRemove/);                                               // the first tap asks, it does not sign out
  });
  test('the device sources are exactly the listed ones, each with a reason', () => { for (const [k, why] of Object.entries(DEVICE)) { expect(Object.hasOwn(sources, k)).toBe(true); expect((why ?? '').length).toBeGreaterThan(10); } });
});

describe('the launch has one owner: the card', () => {
  test('the card is asked again while it has no answer (a launch attempt that died with its answer is restarted by it), and only then', () => {
    const c = text('account.contract');
    expect(c).toMatch(/task cardUnanswered when acs\.state == ""/); expect(c).toMatch(/task cardUnanswered[^\n]*\n\s+after\(3000, bumpLock\)/);
  });
});
