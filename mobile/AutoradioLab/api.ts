// (Named api.ts, not server.ts: Exact reads a server.ts beside app.ts as the test backend.)
// The one place that knows where the paper's server is and how a request to it is made.
// Staging and production are the same host today (https://choir.news); a second host is a second constant, never a
// literal elsewhere. The constant lives in app.ts because the build reads the grant `net.fetch ${ORIGIN}` there without running modules.
//
// The routes the shared pages call are public: no sign-in, no token, so nothing here reads the secret store.
// Logs written from here carry counts and ids only, never a title.

import { ORIGIN } from './app.ts';
import type { Storage } from './app.contract.d.ts';

export { ORIGIN };

/** How long a request may take before it fails (milliseconds). A deadline, not a rate limit. */
export const TIMEOUT_MS = 15000;

export type Query = Record<string, string | number>;

export function url(path: string, query: Query = {}): string {
  const keys = Object.keys(query);
  if (keys.length === 0) return ORIGIN + path;
  return ORIGIN + path + '?' + keys.map((k) => `${encodeURIComponent(k)}=${encodeURIComponent(String(query[k]))}`).join('&');
}

/** A JSON reply and the server's own clock (the `Date` header, as epoch milliseconds; null when it sent none). */
export type Reply = { body: unknown; serverTime: number | null; raw: string };

/** GET a public route. Throws on a transport error, a status outside 200 to 299 or a body that is not JSON. */
export function getJSON(path: string, query: Query = {}): Promise<Reply> { return inert(world, getJSONImpl(path, query)); }
async function getJSONImpl(path: string, query: Query): Promise<Reply> {
  const response = await fetch(url(path, query), { method: 'GET', headers: { accept: 'application/json' }, exactTimeout: TIMEOUT_MS } as RequestInit);
  if (!response.ok) throw new Error(`${path} answered ${response.status}`);
  const raw = await response.text();
  // A large reply (a story bundle was 1.6 MB): its text and its JSON are not decoded in one turn (Exact gives a turn about 100 ms of CPU; Bun has no scheduler).
  if (raw.length > 200_000) { const sc = (globalThis as { scheduler?: { yield?: () => Promise<void> } }).scheduler; if (sc?.yield) await sc.yield(); }
  let body: unknown;
  try { body = JSON.parse(raw); } catch { throw new Error(`${path} answered a body that is not JSON`); }
  const date = response.headers.get('date');
  const parsed = date ? Date.parse(date) : NaN;
  return { body, serverTime: Number.isFinite(parsed) ? parsed : null, raw };
}

// ---------------------------------------------------------------------------------------------------------------------
// Signed-in requests: the listener's own computer. The one seam every page calls (the lane `exact-signin` owns it).
//
// THE KEY IS EXACT'S (`secret.keep api-key`, `key-id`, `computer-id`: the Keychain on Apple, Keystore-wrapped files on Android); signin.ts stores it, `adoptStore` reads it at the start of an answer (see below). The key is held for the chains of that answer, in no log, no error and no page.
// Where the key goes: only to ORIGIN (a path that is not a plain absolute path on it is refused before the key is read).
// There is no refresh: the key is long-lived and the server answers 401 when it is revoked (the Swift client did the same).

/** The listener has no usable session: nothing was kept, or the server refused the key (401). Pages answer their sign-in state. */
/** Why there is no session: `none` (no key), `locked` (the key exists but cannot be read yet: before the first unlock), `refused` (the server refused the key, a 401). Pages say a different thing for each. */
export type SignedOutReason = 'none' | 'locked' | 'refused' | 'unknown';
export class SignedOutError extends Error {
  reason: SignedOutReason;
  constructor(message = 'not signed in', reason: SignedOutReason = 'none') { super(message); this.name = 'SignedOutError'; this.reason = reason; }
}

/** The server answered a status outside 200 to 299 (other than 401). `message` is only the server's own short `error` string. */
export class HTTPError extends Error {
  status: number;
  retryAfter: number | null;
  constructor(status: number, message: string, retryAfter: number | null = null) { super(message); this.name = 'HTTPError'; this.status = status; this.retryAfter = retryAfter; }
}

/** `world`: the world the request BELONGS to, REQUIRED so the type checker lists every caller: a chain that captured account data before an await passes the world it began in (`const w = currentWorld()` at its
 * start); a caller with nothing captured passes `currentWorld()`. A closed world's request is refused (a rejection the holder can drop the work on), and one of an open world is signed only with that world's key. */
export type Authed = { method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'; query?: Query; body?: unknown; headers?: Record<string, string>; world: World };
/** A signed-in request names the world it belongs to (the type requires it; this is the same rule at run time): no default, because the world current when a request is MADE is not always the one its data was captured in. */
const noWorld = (): Promise<never> => Promise.reject(new Error('a signed-in request must state its world (options.world: currentWorld(), or the world the work began in)'));
const ended = (): Promise<never> => Promise.reject(new SignedOutError('the account this request belongs to has ended', 'none'));

/** The three accounts today's sign-in keeps; the native reader answers no other name. */
export const KEYCHAIN_ACCOUNTS = ['api-key', 'key-id', 'computer-id'] as const;
type Account = (typeof KEYCHAIN_ACCOUNTS)[number];

/** The data module's one native module, as far as this file uses it. */
export type NativeReader = { available?: boolean; later(request: { op: string; account?: Account; b64?: string; aad?: string; [key: string]: unknown }): Promise<unknown> };
/** app.ts hands over the module's native handle at every answer (adoptNative). */
let reader: NativeReader | undefined;
/** True once ANY app-module op has answered (a key read, a key state): the native module is there, and the host decision (host.ts) can no longer say "no native". */
let nativeAnswered = false;
export const nativeHasAnswered = (): boolean => nativeAnswered;
export const noteNativeAnswered = (): void => { nativeAnswered = true; };
export function bindNative(native: NativeReader | null | undefined): void { reader = native ?? undefined; if (native) handleSeen = true; }
let lastNative = '';
let handleSeen = false;                       // a native handle object has existed in this process (usable or not): the host has the module
/** Has a native handle ever been given to the seam (even one that was not usable yet)? A host that has the module never decides "no module" by a timer. */
export const nativeHandleSeen = (): boolean => handleSeen;
/** What app.ts calls at every answer: a handle that is missing or not available NEVER replaces a usable one (the first real run showed Settings, Files and Mail signed out for a whole
 * launch although the key answered: a source answered without a usable native handle, and the next source's call found none). Logged on change, with the source's name. */
export function adoptNative(native: NativeReader | null | undefined, source = ''): void {
  if (native) handleSeen = true;
  const usable = !!native && native.available !== false;
  const state = usable ? 'usable' : native ? 'unavailable' : 'missing';
  if (state !== lastNative) { lastNative = state; console.log(`api: native handle ${state} (answering ${source})`); }
  if (usable) reader = native!; else if (!reader) reader = native ?? undefined;
}

/** The app module's `later`, for the native pair that seals mail (mail-seal.ts); null when there is no module. */
export function nativeLater(): ((request: { op: string; account?: Account; b64?: string; aad?: string; [key: string]: unknown }) => Promise<unknown>) | null {
  return reader && reader.available !== false ? (r) => reader!.later(r) : null;
}

// THE KEY IS EXACT'S (docs/design/account-boundary-2026-11-21.md, "Round 12"): `secret.keep api-key`, `key-id`, `computer-id` in Exact's store (the Keychain on Apple, Keystore-wrapped files on Android). A read is synchronous from the snapshot the host takes
// before boot, and a read the host could not make (before the first unlock) answers null exactly as an absent key does: the host retries and, when the snapshot completes, asks again every answer that read the store (runner/src/runner/kept.rs
// `store_changed`, host/apple/src/activation.rs:203-211). So `adoptStore` runs at the start of every answer that is handed the store, and what it finds is held here for the chains of that answer. A held key is dropped by a sign-out and never adopted
// after the session has ended. Nothing here writes the store but `holdSession` (a sign-in) and the sign-out's `forgetSession`.
export const SECRET_KEY = 'api-key', SECRET_KEY_ID = 'key-id', SECRET_COMPUTER = 'computer-id';
export type SecretStore = { get(name: string): string | null; set(name: string, value: string): void; forget(name: string): void };
type Held = { key: string; keyId: string; computerId: string };
let held: Held | null = null;
/** Read the three secrets of the answer's store. A key found is held; a key not found leaves what is held (a stale snapshot must not lose a key a sign-in just stored); an ended session holds nothing. Returns the held key's presence. */
export function adoptStore(store: SecretStore | undefined): boolean {
  if (sessionOver) { held = null; return false; }
  if (!store) return held !== null;
  try {
    const key = store.get(SECRET_KEY);
    if (key) held = { key, keyId: store.get(SECRET_KEY_ID) ?? held?.keyId ?? '', computerId: store.get(SECRET_COMPUTER) ?? held?.computerId ?? '' };
  } catch { /* a refused read is an absent key until the host reads again */ }
  return held !== null;
}
export const heldSession = (): Held | null => held;
/** A sign-in stored this key: the store write is the caller's; this holds it for the chains. */
export function holdSession(key: string, keyId: string, computerId = ''): void { if (!sessionOver) held = { key, keyId, computerId }; }
/** The sign-out: forget the three secrets (the store's own write) and hold nothing. Throws when the store refuses. */
export function forgetSession(store: SecretStore): void {
  held = null; const bad: string[] = [];
  for (const n of [SECRET_KEY, SECRET_KEY_ID, SECRET_COMPUTER]) { try { store.forget(n); } catch (e) { bad.push(`${n}: ${String((e as { message?: unknown } | null)?.message ?? e).slice(0, 100)}`); } }      // every name is tried; the host's own refusal words are logged (they name the secret, never its value)
  if (bad.length) { console.log(`account: the store refused to forget (${bad.join('; ')})`); throw new Error('the store refused to forget'); }
}
export const dropHeld = (): void => { held = null; };
export async function keyDigest(key: string): Promise<string> { return digestOf(key); }
/** REVOKE a key on the server with itself (a second place a request is signed: the key is the one being revoked and goes to ORIGIN only). Bounded; already revoked (401) or unknown (404) is revoked. Answers whether nothing live is left. */
export async function revokeKey(keyId: string, key: string, ms = 8000): Promise<boolean> {
  if (!keyId || !key) return false;
  try {
    const r = await withDeadline(fetch(authedURL(`/auth/api-keys/${encodeURIComponent(keyId)}`), { method: 'DELETE', headers: { Authorization: `Bearer ${key}`, Accept: 'application/json' }, exactTimeout: ms, redirect: 'manual' } as RequestInit), null as Response | null, ms);
    return !!r && (r.ok || r.status === 401 || r.status === 404);
  } catch { return false; }
}

/** A POST of JSON to ORIGIN with NO credentials (the device flow's begin and token: the sign-in has no key yet). The only unsigned request of a signed-out phone. */
export function unauthedPost(path: string, body: unknown, ms = 15000): Promise<Response> {
  if (!plainPath(path)) return Promise.reject(new Error('refused: a path on the server'));
  return fetch(authedURL(path), { method: 'POST', headers: { 'Content-Type': 'application/json', Accept: 'application/json' }, body: JSON.stringify(body), exactTimeout: ms, redirect: 'manual' } as RequestInit);
}

/** No session: there is no key (the page says sign in; the card says why). */
async function noSessionReason(): Promise<SignedOutReason> { return 'none'; }

async function keptItem(_account: Account): Promise<string | null> { return held?.computerId || null; }
/** The key and the computer id, from what the answer's store gave. */
async function keptSession(): Promise<{ key: string; computerId: string } | null> { return held ? { key: held.key, computerId: held.computerId } : null; }


// THE WORLD AND THE ROOT (docs/design/account-boundary-2026-11-21.md, "Accounts are partitions"). An account's files live under ONE root per account (`app:/data/accounts/<d>/`, and the same under
// `app:/cache` and `app:/tmp`), where `<d>` is the SHA-256 of the account's key (64 lowercase hex), so a module that writes `app:/data/takes.json` writes inside the open account's root and cannot name another
// account's. A process opens AT MOST ONE account world: it may first open the anonymous world (public data, no key), then exactly one account world. A sign-out ends the session (`endWorld`): no world opens
// again until the app is relaunched, so a late chain of the account that left can reach only its own bag, its own root and its own key; there is no other account in the process to reach.
// Every I/O choke point is inert for a closed world: a chain that began in it and waits on I/O never resumes (its promise never settles).
// The choke points: the storage the gate hands a source (`bindStorage`), `getJSON`, `authedJSON`/`authedText`/`authedBytes`/`authedToFile`, the podcast feed fetch, native calls of an account chain (`nativeInert`) and a chain's own timers (`inertTimeout`).
export const ACCOUNTS = 'accounts';
export type WorldKind = 'none' | 'anonymous' | 'account';
export class World {
  id: number; closed = false;
  /** `none`: a process that has opened no world yet (nothing is bound to it); `anonymous`: public data, no key; `account`: the account whose key has `digest`. */
  kind: WorldKind = 'none';
  /** The SHA-256 of the account's key (an account world); undefined for the others. A request is signed only with the key whose digest this is. */
  digest: string | undefined = undefined;
  /** The module state of this world (`scoped`): a world begins with none, and a closed world's is unreachable. */
  bag = new Map<symbol, unknown>();
  whenClosed: Promise<void>; private resolveClosed: () => void = () => undefined;
  constructor(id: number) { this.id = id; this.whenClosed = new Promise<void>((r) => { this.resolveClosed = r; }); }
  /** Callbacks to run when this world closes: a gated answer in flight registers one and removes it when it settles (no reaction is left behind per answer), `close` drains the rest. */
  private watchers = new Set<() => void>();
  get watching(): number { return this.watchers.size; }
  watch(fn: () => void): () => void { if (this.closed) { fn(); return () => undefined; } this.watchers.add(fn); return () => { this.watchers.delete(fn); }; }
  close(): void { this.closed = true; this.resolveClosed(); const all = [...this.watchers]; this.watchers.clear(); for (const f of all) f(); }
  /** The folder name of this world under `accounts/`: the digest, or `anonymous`; '' for none. */
  get root(): string { return this.kind === 'account' ? this.digest ?? '' : this.kind === 'anonymous' ? 'anonymous' : ''; }
  /** The account handle: the first 16 hex of the digest (invariant I7: every resource that reads account data takes it as an identifying argument), `anonymous`, or '' when no world is open. */
  get handle(): string { return this.kind === 'account' ? (this.digest ?? '').slice(0, 16) : this.kind === 'anonymous' ? 'anonymous' : ''; }
}
let world = new World(0);
let generation = 0;                                   // moves when a world opens or closes: what a chain compares to know the world it began in is still the process's
let sessionOver = false;                                  // the session ended (a sign-out): no world opens again
export const currentWorld = (): World => world;
export const accountGeneration = (): number => generation;
export const sessionEnded = (): boolean => sessionOver;
const HEX64 = /^[0-9a-f]{64}$/;
/** THE ONE-WORLD RULE, the only `new World(` in the app. `digest`: an account's key digest, or `anonymous`. A process opens the anonymous world and then at most one account world; after a sign-out it opens none. */
export function openWorld(digest: string): World {
  if (sessionOver) throw new Error('the session has ended: no world opens again until the app is relaunched');
  const anon = digest === 'anonymous';
  if (!anon && !HEX64.test(digest)) throw new Error('a world is opened for the digest of a key (64 lowercase hex) or `anonymous`');
  if (world.kind === 'account') throw new Error('one account world per process');
  if (world.kind === 'anonymous') { if (anon) return world; world.close(); }
  generation += 1;
  const w = new World(generation); w.kind = anon ? 'anonymous' : 'account'; w.digest = anon ? undefined : digest;
  world = w;
  return w;
}
/** The session ends (a sign-out): the open world closes and no world opens again. Returns the world that ended. */
export function endWorld(): World { const old = world; sessionOver = true; if (!old.closed) { old.close(); generation += 1; } return old; }
/** MODULE STATE BELONGS TO THE WORLD. A module that keeps anything account-scoped (a cache, a loaded flag, a single-flight promise, a mutex chain) declares it once as
 *     const S = scoped(() => ({ items: new Map<string, Item>(), loading: null as Promise<void> | null }));
 * and reads `S().items`. Every world starts with a fresh one and a closed world's is unreachable, so nothing a chain of the old world left (above all a promise that never settles) can be joined by the
 * next world. `S.reset()` drops the current world's (tests, and a module's own `forget`). The M3 test allows no other module-level mutable state. */
export function scoped<T>(init: () => T): { (): T; reset(): void; of(w: World): T } {
  const key = Symbol('scoped');
  const read = (w: World): T => { let v = w.bag.get(key); if (v === undefined) { v = init(); w.bag.set(key, v); } return v as T; };
  const get = (): T => read(world);
  return Object.assign(get, { reset: (): void => { world.bag.delete(key); }, of: read });
}
const NEVER = new Promise<never>(() => undefined);
/** `p` for a world that is still open when it answers; for a closed one, a promise that never settles (the chain waiting on it stops). */
export function inert<T>(w: World, p: Promise<T>): Promise<T> {
  return p.then((v) => (w.closed ? NEVER : v), (e) => (w.closed ? NEVER : Promise.reject(e)));
}
/** A chain's own timer: it fires for an open world only. */
export function inertTimeout(ms: number, w: World = world): Promise<void> { return inert(w, new Promise<void>((r) => { setTimeout(r, ms); })); }

const RAW = Symbol('raw-storage');
const DEVICE = Symbol('device-storage');
const DEVRAW = Symbol('device-raw');          // what a device handle was made from: only `openAccountStorage` (below) reads it
const denied = (_what: string): Promise<never> => Promise.reject(Object.assign(new Error('refused: not a path the storage allows'), { code: 'denied' }));
const fnNames = (src: object): Set<string> => { const names = new Set<string>(); for (let o: object | null = src; o && o !== Object.prototype; o = Object.getPrototypeOf(o)) for (const k of Object.getOwnPropertyNames(o)) if (k !== 'constructor') names.add(k); return names; };
const plain = (p: unknown): p is string => typeof p === 'string' && p.startsWith('app:/') && !p.split('/').some((s) => s === '.' || s === '..' || /^%2e/i.test(s)) && !/[\\\s]/.test(p);
const SPACES = ['data', 'cache', 'tmp'] as const;
/** The paths that belong to the DEVICE, outside every root: the theme choice and the person's own theme files, `current-account` (the pointer, from station 3) and `ending` (the durable mark of a sign-out under way, account.ts). Bundled themes are in the binary. */
export const DEVICE_PATHS = { files: ['app:/data/theme-choice.json', 'app:/data/current-account', 'app:/data/ending', 'app:/data/signed-in'], dirs: ['app:/data/themes'] };
const isDevicePath = (p: string): boolean => DEVICE_PATHS.files.includes(p) || DEVICE_PATHS.dirs.some((d) => p === d || p.startsWith(d + '/'));
/** The host's input that is not the account's: the picker's documents (`app:/tmp/picked`). Read, and removed after the upload; never written, never re-rooted. */
const PICKED = 'app:/tmp/picked';
const isPicked = (p: string): boolean => p === PICKED || p.startsWith(PICKED + '/');
/** Where a module's path goes for world `w`: `app:/data/<rest>` becomes `app:/data/accounts/<d>/<rest>` (and the same under `app:/cache` and `app:/tmp`). Throws (code `denied`) for a world with no root, a path that
 * is not plain, one already under `accounts/` and the device paths. The only function that makes an account path. */
export function rootPath(w: World, p: unknown): string {
  const root = w.root;
  const bad = (_why: string): never => { throw Object.assign(new Error('refused: not a path the account\'s storage allows'), { code: 'denied' }); };      // the reason is not in the message: a message reaches a page, and a path may carry a root
  if (root === '') bad('no account is open');
  if (!plain(p)) return bad(`${String(p).slice(0, 60)} is not a plain app path`);
  if (isDevicePath(p)) bad(`${p.slice(0, 60)} is a device path`);
  for (const space of SPACES) {
    const base = `app:/${space}`;
    if (p === base || p.startsWith(base + '/')) {
      const rest = p.slice(base.length);                                            // '' or '/...'
      if (rest === `/${ACCOUNTS}` || rest.startsWith(`/${ACCOUNTS}/`)) bad(`${p.slice(0, 60)} is already under ${ACCOUNTS}/`);
      return `${base}/${ACCOUNTS}/${root}${rest}`;
    }
  }
  return bad(`${p.slice(0, 60)} is not under app:/data, app:/cache or app:/tmp`);
}
const ONE_PATH = new Set(['readFile', 'readdir', 'stat', 'exists', 'readText', 'atomicWriteFile', 'writeFile', 'rm', 'mkdir']);
const TWO_PATHS = new Set(['rename', 'copyFile']);
const READS = new Set(['readFile', 'stat', 'readdir', 'exists', 'readText']);
/** THE STORAGE OF A SOURCE THAT DRAWS BEFORE SIGN-IN (the theme sources, the host fact, the build rows, the first run): not the host's storage but a handle that reaches the device paths and nothing else
 * (invariant I4). Every other path, and every function that is not a plain one-path one, is refused. It takes no world: a device path is the same for every account. */
export function deviceStorage<S extends { fs: object }>(raw: S): S {
  if (!raw || !raw.fs) return raw;
  const src = raw.fs as Record<string, unknown>; const fs: Record<string, unknown> = {};
  for (const k of fnNames(src)) {
    const v = src[k];
    if (typeof v !== 'function') { fs[k] = v; continue; }
    fs[k] = (...a: unknown[]) => {
      if (!ONE_PATH.has(k)) return denied(`${k}() is not a device call`);
      const p = a[0];
      if (!plain(p) || !isDevicePath(p)) return denied(`${String(p).slice(0, 60)} is not a device path`);
      return Promise.resolve((v as (...x: unknown[]) => unknown).apply(src, a));
    };
  }
  const props: PropertyDescriptorMap = { fs: { value: fs, enumerable: true }, [DEVICE]: { value: true }, [DEVRAW]: { value: raw } };
  const sq = (raw as unknown as { sqlite?: object }).sqlite;
  if (sq && typeof sq === 'object') { const dn: Record<string, unknown> = {}; for (const k of fnNames(sq)) dn[k] = () => denied(`sqlite.${k}()`); props.sqlite = { value: dn, enumerable: true }; }      // a device source opens no database: the content store is the account's
  return Object.create(raw, props) as S;
}
export const isDeviceStorage = (s: unknown): boolean => !!s && (s as Record<symbol, unknown>)[DEVICE] === true;
/** The storage a bound one was made from. Only api.ts and account.ts may hold it (the sweep and the root deletion). */
export const unbound = <S,>(s: S): S => ((s as unknown as Record<symbol, S> | undefined)?.[RAW] ?? s);
/** THE STORAGE A GATED SOURCE IS HANDED, for world `w`: every path is re-rooted under the world's root (`rootPath`), `sqlite.open` too; the picker's documents pass read-only and unprefixed; a path of the device, `..`, anything under `accounts/`
 * and any function that is not a plain one is refused. Every call is inert for a closed world (not even started; an answer that comes after the close never arrives). */
export function bindStorage<S extends { fs: object }>(raw: S, w: World = world): S {
  if (!raw || !raw.fs) return raw;
  // A copy with every function wrapped (no Proxy: the host's `fs` may be frozen, and a Proxy may not return a different value for a frozen property).
  const src = raw.fs as Record<string, unknown>; const fs: Record<string, unknown> = {};
  for (const k of fnNames(src)) {
    const v = src[k];
    if (typeof v !== 'function') { fs[k] = v; continue; }
    fs[k] = (...a: unknown[]) => {
      if (w.closed) return NEVER;
      let args = a;
      try {
        if (ONE_PATH.has(k)) {
          const p = a[0];
          if (plain(p) && isPicked(p) && (READS.has(k) || k === 'rm')) args = a;
          else args = [rootPath(w, p), ...a.slice(1)];
        } else if (TWO_PATHS.has(k)) {
          const from = a[0]; const keepFrom = plain(from) && isPicked(from) && k === 'copyFile';
          args = [keepFrom ? from : rootPath(w, from), rootPath(w, a[1]), ...a.slice(2)];
        } else return denied(`${k}() is not an account call`);
      } catch (e) { return Promise.reject(e); }
      return inert(w, Promise.resolve((v as (...x: unknown[]) => unknown).apply(src, args)));
    };
  }
  const props: PropertyDescriptorMap = { fs: { value: fs, enumerable: true }, [RAW]: { value: raw } };
  const sq = (raw as unknown as { sqlite?: Record<string, unknown> }).sqlite;
  if (sq && typeof sq === 'object') {
    const sqlite: Record<string, unknown> = {};
    for (const k of fnNames(sq)) {
      const v = sq[k];
      sqlite[k] = typeof v !== 'function' ? v : k === 'open'
        ? (...a: unknown[]) => { if (w.closed) return NEVER; try { return inert(w, Promise.resolve((v as (...x: unknown[]) => unknown).apply(sq, [rootPath(w, a[0]), ...a.slice(1)]))); } catch (e) { return Promise.reject(e); } }
        : () => denied(`sqlite.${k}() is not an account call`);
    }
    props.sqlite = { value: sqlite, enumerable: true };
  }
  return Object.create(raw, props) as S;      // defineProperty, not assignment: the host's storage object has a read-only `fs`
}
/** The Texture themes are account data, and the theme sources draw before sign-in and so hold only the device handle: this gives them the OPEN account world's storage (re-rooted like any gated source's), read-only unless
 * `write` is asked for. `s` is a device handle (or the host's storage). undefined when no account world is open: none before sign-in, none after a sign-out. */
export function openAccountStorage<S extends { fs: object }>(s: S, write = false): S | undefined {
  const w = world;
  if (w.kind !== 'account' || w.closed) return undefined;
  const h = s as unknown as Record<symbol, S> | undefined;
  const raw = (h?.[DEVRAW] ?? h?.[RAW] ?? s) as S;      // a device handle or an already-bound one is unwrapped to the host's storage: binding a bound handle again would refuse every path
  const b = bindStorage(raw, w);
  if (write) return b;
  const fs: Record<string, unknown> = {};
  for (const k of fnNames(b.fs)) { const v = (b.fs as Record<string, unknown>)[k]; fs[k] = typeof v !== 'function' ? v : READS.has(k) ? v : () => denied(`${k}() is a write`); }
  return Object.create(b, { fs: { value: fs, enumerable: true } }) as S;
}
/** The app module's `later` for a chain that works on an account's data (the mail seal, the voice's takes, the run sheet): inert for a closed world like every other choke point, so a native answer that arrives after the
 * session ended never resumes the chain. Null when there is no module. (`nativeLater` stays raw for the boundary's own machinery, the host fact and the device-level sources.) */
export function nativeInert(): ((request: { op: string; account?: Account; b64?: string; aad?: string; [key: string]: unknown }) => Promise<unknown>) | null {
  const w = world;
  return reader && reader.available !== false ? (r) => (w.closed ? NEVER : inert(w, reader!.later(r))) : null;      // a closed world's call is NOT DELIVERED (the deck's play after a sign-out must not restart the runtime), and an answer that arrives after it closed never resumes the chain
}
/** Remove a file or a folder and everything under it, children first: a host that refuses to remove a non-empty folder (`ENOTEMPTY`) must not keep a sign-out from finishing. (Exact's Apple and Unix host removes a folder
 * with its contents itself, `app_fs_unix.rs:236-260`; the other hosts are not verified, so every folder removal goes through this.) A missing path is fine; any other failure throws. */
export async function rmTree(storage: Storage, path: string): Promise<void> {
  // A child that appears while the folder is being removed (a late write) makes the folder refuse (ENOTEMPTY): the tree is walked again, a few times, before the failure counts.
  for (let attempt = 0; ; attempt++) {
    let names: string[] = [];
    try { names = await storage.fs.readdir(path); } catch { names = []; }                // a file, or nothing there
    for (const n of names) await rmTree(storage, `${path}/${n}`);
    try { await storage.fs.rm(path); return; } catch (e) {
      const code = (e as { code?: unknown } | null)?.code;
      if (code === 'ENOENT') return;
      if (code !== 'ENOTEMPTY' || attempt >= 3) throw e;
    }
  }
}

/** A module's path as the HOST and the native side see it: `app:/cache/files/x` is `app:/cache/accounts/<d>/files/x` there. A path that leaves JavaScript (an image's `src`, an iframe's, a url the deck plays, a share) goes through this;
 * every path JavaScript hands to `storage` is the module's own and is re-rooted by the handle. With no account or anonymous world open the path is returned as it is. */
export function appPath(p: string): string { return world.root === '' || world.closed ? p : rootPath(world, p); }

/** A public GET of a page that is not the server (a web page the radio reads aloud, a feed): the one raw `fetch` of the app outside the signed seam, inert for a closed world; it carries no key. */
export function fetchPublic(url: string, init: object): Promise<Response> { return inert(world, fetch(url, init as RequestInit)); }

/** A file write that reports whether the file is there: a full disk or a refusal loses only the copy. (The storage a source holds is its world's: a closed world's write never starts.) */
export async function putFile(storage: Storage | undefined, path: string, bytes: Uint8Array): Promise<boolean> {
  if (!storage) return false;
  try { await storage.fs.atomicWriteFile(path, bytes); return true; } catch { return false; }
}

/** An id as one path segment (Swift PathSegment.encode): everything but unreserved characters is escaped; "." and ".." too. */
export function segment(id: string): string {
  if (id === '.') return '%2E';
  if (id === '..') return '%2E%2E';
  return encodeURIComponent(id).replace(/[!'()*]/g, (c) => '%' + c.charCodeAt(0).toString(16).toUpperCase());
}

/** The URL of a signed-in route: the query is sorted by name and escaped as the Swift client did (`/ : @ ,` stay). */
export function authedURL(path: string, query: Query = {}): string {
  const keys = Object.keys(query).sort();
  if (keys.length === 0) return ORIGIN + path;
  const enc = (s: string) => encodeURIComponent(s).replace(/[!'()*]/g, (c) => '%' + c.charCodeAt(0).toString(16).toUpperCase()).replace(/%2F/g, '/').replace(/%3A/g, ':').replace(/%40/g, '@').replace(/%2C/g, ',');
  return ORIGIN + path + '?' + keys.map((k) => `${enc(k)}=${enc(String(query[k]))}`).join('&');
}

/** A deadline for a signed-in call, not a rate limit: the Swift client had only a 60 s idle timeout, and a model turn may be slow. */
export const AUTHED_TIMEOUT_MS = 120000;
const STATUS_PATH = '/api/compute/status';

function plainPath(path: string): boolean {
  return path.startsWith('/') && !path.startsWith('//') && !/[\\\s#]/.test(path) && !path.split('?')[0].split('/').some((s) => s === '.' || s === '..') && !path.includes('?');
}

// What the seam keeps for ITS world (memory only): the computer discovered for the key (today's sign-in keeps the id in the Keychain and that is read first), the discovery in flight, and the digest the server last
// refused. They are in the world's bag, so another world never meets them. Only a SHA-256 of the key is kept, never the key (finding 6).
const seamState = scoped(() => ({ discovered: null as string | null, discovering: null as Promise<string> | null, rejected: '' }));
async function digestOf(key: string): Promise<string> {
  const d = new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(key) as unknown as BufferSource));
  return Array.from(d, (x) => x.toString(16).padStart(2, '0')).join('');
}
export { digestOf };

function scrub(text: string, key: string): string { return key ? text.split(key).join('[key]') : text; }

/** THE ONLY PLACE A REQUEST IS SIGNED (invariant I2). The key is read from its source and used only when its digest is the digest of the account world `w` that began the request (a world with no account signs nothing),
 * and never after `w` closed. */
async function send(w: World, path: string, opts: Authed, accept: string, extra: Record<string, unknown> = {}): Promise<{ r: Response; key: string }> {
  if (!plainPath(path)) throw new Error('refused: a signed-in path is a plain absolute path on the server');
  if (w.closed) throw new SignedOutError('the account this request belongs to has ended');
  if (w.kind !== 'account' || !w.digest) throw new SignedOutError('no account is open for this request', 'none');
  const sess = await keptSession();
  if (w.closed) throw new SignedOutError('the account this request belongs to has ended');      // the session ended while the key was read: nothing is signed (the chain above is inert)
  if (!sess) throw new SignedOutError('not signed in', await noSessionReason());
  const key = sess.key;
  if ((await digestOf(key)) !== w.digest) throw new SignedOutError('the stored key is not the key of the account this request belongs to');       // a request is signed only with the key of its world
  const headers: Record<string, string> = { ...(opts.headers ?? {}), Accept: accept, Authorization: `Bearer ${key}` };
  const method = opts.method ?? 'GET';
  let body: string | undefined;
  if (opts.body instanceof Uint8Array) {
    // Raw bytes go as they are (a file added to the listener's computer: PUT /api/files/{path}); the caller names the type.
    body = opts.body as unknown as string; headers['Content-Type'] = headers['content-type'] ?? headers['Content-Type'] ?? 'application/octet-stream'; delete headers['content-type'];
  } else if (opts.body !== undefined) { body = JSON.stringify(opts.body); headers['Content-Type'] = 'application/json'; }
  if (!path.startsWith('/auth/') && path !== STATUS_PATH) {
    try { headers['X-Choir-Computer'] = await computerId(key, sess.computerId, w); } catch (e) { console.log(`api: computer id failed: ${e instanceof Error ? e.name : 'error'}`); throw e; }
  }
  if (w.closed) throw new SignedOutError('the account this request belongs to has ended');
  try {
    return { r: await fetch(authedURL(path, opts.query), { method, headers, body, exactTimeout: AUTHED_TIMEOUT_MS, redirect: 'manual', ...extra } as RequestInit), key };
  } catch (e) {
    const timed = (e as { kind?: string } | null)?.kind === 'Timeout' || (e as { name?: string } | null)?.name === 'TimeoutError';
    throw new Error(scrub(`${timed ? 'timeout' : 'network'}: ${e instanceof Error ? e.message : 'request failed'}`, key));
  }
}

async function computerId(key: string, kept: string, w: World): Promise<string> {
  if (kept) return kept;
  const S = seamState.of(w);
  if (S.discovered) return S.discovered;
  if (!S.discovering) {
    let mine!: Promise<string>;
    mine = (async () => {
      try {
        const { r, key: used } = await send(w, STATUS_PATH, { world: w }, 'application/json');
        if (used !== key) throw new SignedOutError();
        await check(w, r, STATUS_PATH, key);
        const o = (await r.json()) as { computers?: { computer_id?: string; kind?: string; current?: boolean }[]; warnings?: string[] };
        const list = (o.computers ?? []).filter((c) => (c.kind ?? 'interactive') === 'interactive' && !!c.computer_id);
        const pick = list.find((c) => c.current === true) ?? list[0];
        if (!pick || !pick.computer_id) {
          const warn = (o.warnings ?? [])[0] ?? '';
          throw new Error('the account has no computer yet' + (warn ? ` (${warn.slice(0, 120)})` : '') + '; try again in a moment');
        }
        if (w.closed) throw new SignedOutError();
        S.discovered = pick.computer_id;
        return pick.computer_id;
      } finally { if (S.discovering === mine) S.discovering = null; }
    })();
    S.discovering = mine;
  }
  return S.discovering;
}

/** The digest of the key the server last refused (a 401), for the open world: the account state says "rejected" while that key is the stored one, and a 200 clears it. */
export const rejectedDigest = (): string => seamState().rejected;
export const hasNativeHandle = (): boolean => !!reader && reader.available !== false;
async function check(w: World, r: Response, path: string, key: string): Promise<void> {
  const S = seamState.of(w);
  if (r.ok) { S.rejected = ''; return; }                                // the server took the key again: it is no longer refused
  if (r.status === 401) { try { S.rejected = await digestOf(key); } catch { /* the digest is a convenience */ } throw new SignedOutError('the key was refused; sign in again', 'refused'); }
  let msg = '';
  try { const o = JSON.parse(await r.text()); if (o && typeof o.error === 'string') msg = scrub(o.error, key).slice(0, 200); } catch { /* the server's string only; never the body */ }
  const retry = Number(r.headers.get('retry-after'));
  if (r.status >= 300 && r.status < 400) throw new HTTPError(r.status, 'redirect refused: the key goes only to the server it was made for');
  const reason = r.status === 403 ? `403: the key lacks the scope for ${path}` : r.status === 429 ? '429: rate limited' : `HTTP ${r.status}`;
  throw new HTTPError(r.status, scrub(reason + (msg ? `: ${msg}` : ''), key), r.status === 429 && Number.isFinite(retry) && r.headers.get('retry-after') ? retry : null);
}

/** A signed-in JSON request to the listener's own computer. Throws SignedOutError when there is no session. */
export function authedJSON(path: string, opts: Authed): Promise<Reply> { const o = opts ?? ({} as Authed); if (!o.world) return noWorld(); const w = o.world; return w.closed ? ended() : inert(w, authedJSONImpl(w, path, o)); }
async function authedJSONImpl(w: World, path: string, opts: Authed): Promise<Reply> {
  const { r, key } = await send(w, path, opts, 'application/json');
  await check(w, r, path, key);
  const raw = await r.text();
  let body: unknown = null;
  if (raw !== '') { try { body = JSON.parse(raw); } catch { throw new Error(`${path} answered a body that is not JSON`); } }
  const date = r.headers.get('date');
  const parsed = date ? Date.parse(date) : NaN;
  return { body, serverTime: Number.isFinite(parsed) ? parsed : null, raw };
}

/** The whole answer as text, whatever its type (the programme route's event stream, read when it has ended: Exact's `exactStream` is not shown to take a POST, docs/reference/exact/gaps.md 36). */
export function authedText(path: string, opts: Authed, accept = 'text/event-stream'): Promise<string> { const o = opts ?? ({} as Authed); if (!o.world) return noWorld(); const w = o.world; return w.closed ? ended() : inert(w, authedTextImpl(w, path, o, accept)); }
async function authedTextImpl(w: World, path: string, opts: Authed, accept: string): Promise<string> {
  const { r, key } = await send(w, path, opts, accept);
  await check(w, r, path, key);
  return r.text();
}

/** The same, for bytes (a file, a PDF, audio). */
export function authedBytes(path: string, opts: Authed): Promise<{ bytes: Uint8Array; contentType: string }> { const o = opts ?? ({} as Authed); if (!o.world) return noWorld(); const w = o.world; return w.closed ? ended() : inert(w, authedBytesImpl(w, path, o)); }
async function authedBytesImpl(w: World, path: string, opts: Authed): Promise<{ bytes: Uint8Array; contentType: string }> {
  const { r, key } = await send(w, path, opts, '*/*');
  await check(w, r, path, key);
  const bytes = new Uint8Array(await r.arrayBuffer());
  return { bytes, contentType: r.headers.get('content-type') ?? '' };
}

/** A signed-in GET whose body the HOST writes to an app file as it arrives (`exactBodyTo`, reference.md, fetch row): the bytes never enter JavaScript, so a 300 MB recording costs neither the engine's 64 MiB heap nor the
 * turn's 100 ms. The file appears whole or not at all (the host stages it beside the target); a reply that is not 2xx writes nothing and is an error as in `authedBytes`. `dest` is a plain path under `app:/cache/`; it is
 * RE-ROOTED under the world's root here (the host writes it, not `storage`), so a late download lands in the closed world's own folder, which no key names and launch deletes. A host that wrote nothing (it read the
 * body into memory) is an error and the file is removed. */
export function authedToFile(path: string, dest: string, storage: Storage | undefined, opts: Authed): Promise<{ bytes: number; contentType: string }> {
  const o = opts ?? ({} as Authed); if (!o.world) return noWorld(); const w = o.world;
  if (w.closed) return ended();
  return inert(w, authedToFileImpl(w, path, dest, storage, o));
}
async function authedToFileImpl(w: World, path: string, dest: string, storage: Storage | undefined, opts: Authed): Promise<{ bytes: number; contentType: string }> {
  if (!/^app:\/cache\/[^\s?#\\]+$/.test(dest) || dest.split('/').some((s) => s === '.' || s === '..')) throw new Error('refused: a download goes to a plain file under app:/cache/');
  const real = rootPath(w, dest);
  const drop = async () => { try { await unbound(storage)?.fs.rm(real); } catch { /* absent */ } };
  const { r, key } = await send(w, path, opts, '*/*', { exactBodyTo: real });
  try { await check(w, r, path, key); } catch (e) { await drop(); throw e; }
  const wrote = (r as unknown as { exactBodyTo?: { bytes?: unknown } | null }).exactBodyTo;
  if (!wrote || typeof wrote.bytes !== 'number') { await drop(); throw new Error('the host did not write the file'); }
  return { bytes: wrote.bytes, contentType: r.headers.get('content-type') ?? '' };
}

/** A deadline, not a limit, on one read of the Keychain through the bridge: a native call that never answers must not hold a page answer, a sign-out or the launch. */
export const keyTiming = { ms: 5000 };
/** `p`, or `late` when it has not answered within `ms`. (The timer is cleared as soon as `p` answers; a source's own timers end with its answer.) */
export async function withDeadline<T>(p: Promise<T>, late: T, ms: number): Promise<T> {
  let t: ReturnType<typeof setTimeout> | undefined;
  try { return await Promise.race([p, new Promise<T>((r) => { t = setTimeout(() => r(late), ms); })]); } finally { if (t !== undefined) clearTimeout(t); }
}

/** READS EXACT'S HOST FACT AND LETS IT THROW. At the bake `exactHost()` throws ("unavailable during bake") and the bake leaves the resource that asked unbaked (reference.md: "the bake leaves its answers for the host"; js/tests/it/inputs.rs asserts `initial.len == 0`); in a test or a plain run there is no such function and nothing happens. It must never be wrapped in a try: swallowing that throw let a gated answer be evaluated at bake and fail it.
 * (`hostKind` below is the tolerant reader, for logic that must not throw.) */
export function markNotBaked(): void { const f = (globalThis as { exactHost?: () => unknown }).exactHost; if (typeof f === 'function') f(); }
/** What this runtime says it is: Exact's `exactHost()` (`native` on a phone, `web` in a browser worker; no grant). Absent where Exact does not provide it (a test, a plain bun run): `unknown`. This, not a timeout, is how a phone is told from a host with no native module. */
export function hostKind(): 'native' | 'web' | 'unknown' {
  const f = (globalThis as { exactHost?: () => unknown }).exactHost;
  if (typeof f !== 'function') return 'unknown';
  try { const h = f(); return h === 'native' || h === 'web' ? h : 'unknown'; } catch { return 'unknown'; }
}
/** For tests: forget what was discovered, in the open world. */
export function resetSession(): void { seamState.reset(); }
/** For tests: a process that has opened no world yet (the native handle stays). */
export function resetWorld(): void { world.close(); world = new World(0); generation += 1; sessionOver = false; }
/** For tests: the seam as a fresh process has it. */
export function resetSeam(): void { keyTiming.ms = 5000; held = null; resetWorld(); generation = 0; reader = undefined; lastNative = ''; handleSeen = false; nativeAnswered = false; }

/** The computer id today's sign-in kept in the Keychain, or null (the Account screen names it when the key answers none). */
export async function keptComputerId(): Promise<string | null> { return keptItem('computer-id'); }
