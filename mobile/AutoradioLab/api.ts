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
  let body: unknown;
  try { body = JSON.parse(raw); } catch { throw new Error(`${path} answered a body that is not JSON`); }
  const date = response.headers.get('date');
  const parsed = date ? Date.parse(date) : NaN;
  return { body, serverTime: Number.isFinite(parsed) ? parsed : null, raw };
}

// ---------------------------------------------------------------------------------------------------------------------
// Signed-in requests: the listener's own computer. The one seam every page calls (the lane `exact-signin` owns it).
//
// What today's sign-in (Swift, Choir/ChoirSignIn.swift and ChoirClient.swift) keeps, and what this reads in place:
//   Keychain, service "news.choir.autoradio.choir", generic passwords, accounts "api-key" (the bearer key, a secret),
//   "key-id" (not a secret; revokes the key at sign-out) and "computer-id" (not a secret; the listener's computer).
// Exact's own `store` keeps items under the app's id ("news.choir.autoradio.lab"), so it cannot see these: the read goes
// through the one native call in modules/apple/ModelsEntry.swift, and nothing here writes the Keychain. The key is read for
// each request, held in no state, and is in no log, no error and no page. A key another account signs in with is simply read next time.
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

/** `world`: the world the request BELONGS to, for work held outside a chain (a queue of batches to post, a retry): captured with `currentWorld()` when the work was made. A closed world's request is refused (a rejection
 * the holder can drop the work on), and one of an open world is signed only with that world's key. Without it the request belongs to the world that is current when it is made. */
export type Authed = { method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'; query?: Query; body?: unknown; headers?: Record<string, string>; world?: World };
const ended = (): Promise<never> => Promise.reject(new SignedOutError('the account this request belongs to has ended', 'none'));

/** The three accounts today's sign-in keeps; the native reader answers no other name. */
export const KEYCHAIN_ACCOUNTS = ['api-key', 'key-id', 'computer-id'] as const;
type Account = (typeof KEYCHAIN_ACCOUNTS)[number];

/** The data module's one native module, as far as this file uses it. */
export type NativeReader = { available?: boolean; later(request: { op: string; account?: Account; b64?: string; aad?: string }): Promise<unknown> };
/** app.ts hands over the module's native handle at every answer (adoptNative). */
let reader: NativeReader | undefined;
/** True once ANY app-module op has answered (a key read, a key state): the native module is there, and the host decision (host.ts) can no longer say "no native". */
let nativeAnswered = false;
export const nativeHasAnswered = (): boolean => nativeAnswered;
export function bindNative(native: NativeReader | null | undefined): void { reader = native ?? undefined; }
let lastNative = '';
/** What app.ts calls at every answer: a handle that is missing or not available NEVER replaces a usable one (the first real run showed Settings, Files and Mail signed out for a whole
 * launch although the key answered: a source answered without a usable native handle, and the next source's call found none). Logged on change, with the source's name. */
export function adoptNative(native: NativeReader | null | undefined, source = ''): void {
  const usable = !!native && native.available !== false;
  const state = usable ? 'usable' : native ? 'unavailable' : 'missing';
  if (state !== lastNative) { lastNative = state; console.log(`api: native handle ${state} (answering ${source})`); }
  if (usable) reader = native!; else if (!reader) reader = native ?? undefined;
}

/** The app module's `later`, for the native pair that seals mail (mail-seal.ts); null when there is no module. */
export function nativeLater(): ((request: { op: string; account?: Account; b64?: string; aad?: string }) => Promise<unknown>) | null {
  return reader && reader.available !== false ? (r) => reader!.later(r) : null;
}

async function keptItem(account: Account): Promise<string | null> {
  if (!reader || reader.available === false) return null;
  const r = (await reader.later({ op: 'get', account })) as { found?: boolean; value?: unknown } | null;
  return r && r.found === true && typeof r.value === 'string' && r.value !== '' ? r.value : null;
}

/** No session: is the key absent, or only unreadable (locked)? One read-only native question. */
async function noSessionReason(): Promise<SignedOutReason> {
  if (!reader) return 'none';                                // no handle at all: the host has no key store; the page says there is nothing signed in (the card says whether signing in exists)
  try { const r = (await reader?.later({ op: 'keystate' })) as { state?: unknown } | null | undefined; return r && r.state === 'unreadable' ? 'locked' : r && r.state === 'absent' ? 'none' : 'unknown'; } catch { return 'unknown'; }       // a bridge that throws or says nothing: say so, never "sign in"
}

/** The key and the computer id in ONE native read: a sign-out or a new sign-in between two reads cannot pair an old key with a new computer. */
let lastFlags = '';
async function keptSession(): Promise<{ key: string; computerId: string } | null> {
  if (!reader || reader.available === false) return null;
  const r = (await reader.later({ op: 'session' })) as { found?: boolean; value?: unknown; computerId?: unknown } | null;
  if (r && typeof r === 'object' && typeof r.found === 'boolean') nativeAnswered = true;
  const flags = `found=${String(r?.found)} key=${typeof r?.value === 'string' && r.value !== ''} computer=${typeof r?.computerId === 'string' && r.computerId !== ''}`;   // flags only, never a value
  if (flags !== lastFlags) { lastFlags = flags; console.log(`api: session ${flags}`); }              // on change only: one line per request is volume
  if (!r || r.found !== true || typeof r.value !== 'string' || r.value === '') return null;
  return { key: r.value, computerId: typeof r.computerId === 'string' ? r.computerId : '' };
}


// THE WORLD (docs/design/account-boundary-2026-11-21.md, section 3). One World per stretch of the boundary with no account change in it. The boundary moving (a sign-out beginning, a Clear beginning, an Adopt)
// CLOSES it and opens the next; the generation is its number. Every I/O choke point is INERT for a closed world: a chain of work that began in world A and waits on I/O never resumes once A is closed
// (its promise never settles), so after the move nothing of A's is inserted, written or sent, whatever it would have done next. Guarding awaits one by one did not converge (five review rounds).
// The choke points: the storage the gate hands a source (`bindStorage`), `getJSON`, `authedJSON`/`authedBytes`/`authedToFile`, the podcast feed fetch, and a chain's own timers (`inertTimeout`).
// A request is signed only with the key of the world that began the chain (`world.digest`, set by the gate from the facts). The Clear drains A's STARTED writes (`drainClosed`) before it wipes.
// Not covered, said plainly: a chain resumed from a non-I/O await after the close (a microtask, a shared promise another chain resolves) that then starts NEW I/O begins it in the new world.
export class World {
  id: number; closed = false;
  /** The SHA-256 of the key of the account this world is for ('' for none); undefined where no gate set it (tests). A request signed with another key is refused. */
  digest: string | undefined = undefined;
  /** The boundary moved and no gate has confirmed it open for this world yet: what belongs to an account (a Texture theme) is not shown. */
  owed = false;
  pending = new Set<Promise<unknown>>();
  whenClosed: Promise<void>; private resolveClosed: () => void = () => undefined;
  constructor(id: number) { this.id = id; this.whenClosed = new Promise<void>((r) => { this.resolveClosed = r; }); }
  close(): void { this.closed = true; this.resolveClosed(); }
}
let world = new World(0);
const closedWorlds: World[] = [];
export const currentWorld = (): World => world;
export const accountGeneration = (): number => world.id;
/** The boundary moved: the world ends, the next begins. Returns the one that ended. */
export function closeWorld(owed = false): World { const old = world; old.close(); world = new World(old.id + 1); world.owed = owed; closedWorlds.push(old); return old; }
/** A sign-out or a Clear begins: the world ends and the next is OWED until a gate confirms the boundary open. */
export function nextGeneration(): number { closeWorld(true); return world.id; }
const NEVER = new Promise<never>(() => undefined);
/** `p` for a world that is still open when it answers; for a closed one, a promise that never settles (the chain waiting on it stops). */
export function inert<T>(w: World, p: Promise<T>): Promise<T> {
  return p.then((v) => (w.closed ? NEVER : v), (e) => (w.closed ? NEVER : Promise.reject(e)));
}
/** A chain's own timer: it fires for an open world only. */
export function inertTimeout(ms: number, w: World = world): Promise<void> { return inert(w, new Promise<void>((r) => { setTimeout(r, ms); })); }
const READS = new Set(['readFile', 'stat', 'readdir', 'exists', 'readText', 'open']);
/** The storage a source is handed: every call is inert for a closed world (not even started; an answer that comes after the close never arrives), and a write, a removal or a folder started while the world
 * is open is tracked, so the Clear can wait for it (`drainClosed`) and no write lands after the wipe. Reads of a world still open pass through. */
export function bindStorage<S extends { fs: object }>(raw: S, w: World = world): S {
  if (!raw || !raw.fs) return raw;
  const fs = new Proxy(raw.fs as Record<string, unknown>, { get(t, k) {
    const v = t[k as string];
    if (typeof v !== 'function') return v;
    return (...a: unknown[]) => {
      if (w.closed) return NEVER;
      const p = Promise.resolve((v as (...x: unknown[]) => unknown).apply(t, a));
      if (!READS.has(String(k))) { w.pending.add(p); void p.then(() => w.pending.delete(p), () => w.pending.delete(p)); }
      return inert(w, p);
    };
  } });
  return Object.create(raw, { fs: { value: fs, enumerable: true } }) as S;      // defineProperty, not assignment: the host's storage object has a read-only `fs`
}
/** Wait (up to `ms`) for every write that a closed world started to finish. True when none is left. */
export async function drainClosed(ms: number): Promise<boolean> {
  const all = closedWorlds.flatMap((w) => [...w.pending]);
  const ok = all.length === 0 || (await withDeadline(Promise.allSettled(all).then(() => true), false, ms));
  if (ok) closedWorlds.length = 0;
  return ok;
}

// A SIGN-OUT UNDER WAY (review 2026-11-19, mail seat 1, finding 2). `beginSignOut` runs BEFORE the native sign-out: the generation moves at once (a store write
// that began under the old one refuses itself, mail-store.ts), and sealing is off until a NEW session is established (a key whose digest is not the one that
// was signed out) or the sign-out is given up (`abortSignOut`). The native side also makes no mail key once the sign-in key is gone (MailSeal.swift).
let blockedDigest: string | null = null;      // null: not signing out. '' or a digest: a sign-out began; sealing waits for a session whose key is another one.
export const sealingBlocked = (): boolean => blockedDigest !== null;
export async function beginSignOut(): Promise<void> {
  nextGeneration(); resetSession(); rejected = '';          // a refused key is forgotten with the session (the same key signed in again is not 'no longer accepted' until the server says so)
  let d = '';
  try { const s = await withDeadline(keptSession().then((x) => x, () => null), null, keyTiming.ms); if (s) d = await digestOf(s.key); } catch { /* no key to name */ }      // a read that hangs names no digest: the block still stands
  blockedDigest = d;
}
export function abortSignOut(): void { blockedDigest = null; }
/** A new session clears the block explicitly: called before sealing is judged, so the first sync of the next session is not answered idle. */
export async function refreshSealBlock(): Promise<void> {
  if (blockedDigest === null) return;
  try { const sess = await keptSession(); if (sess && (await digestOf(sess.key)) !== blockedDigest) blockedDigest = null; } catch { /* still signed out */ }
}

/** ONE rule for every moved store's file write (review 2026-11-19): `gen` is the account generation the SOURCE started under (`accountGeneration()` before its request). The write is
 * refused if a sign-out has moved the generation, and a file that lands after the generation moved is removed again. True when the file is there. */
export async function writeCurrent(gen: number, storage: Storage | undefined, path: string, bytes: Uint8Array): Promise<boolean> {
  if (!storage || gen !== world.id) return false;
  try { await storage.fs.atomicWriteFile(path, bytes); } catch { return false; }
  if (gen !== world.id) { try { await storage.fs.rm(path); } catch { /* the wipe will */ } return false; }
  return true;
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

// The computer discovered for a key (memory only; today's sign-in keeps the id in the Keychain and that is read first). Only a SHA-256 of the
// key is kept, never the key (finding 6); the discovery in flight is keyed by that digest too, so another account's key never receives it.
let discovered: { digest: string; id: string } | null = null;
let discovering: { digest: string; p: Promise<string> } | null = null;
async function digestOf(key: string): Promise<string> {
  const d = new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(key) as unknown as BufferSource));
  return Array.from(d, (x) => x.toString(16).padStart(2, '0')).join('');
}

function scrub(text: string, key: string): string { return key ? text.split(key).join('[key]') : text; }

async function send(w: World, path: string, opts: Authed, accept: string, extra: Record<string, unknown> = {}): Promise<{ r: Response; key: string }> {
  if (!plainPath(path)) throw new Error('refused: a signed-in path is a plain absolute path on the server');
  const sess = await keptSession();
  if (w.closed) await NEVER;                                 // the boundary moved while the key was read: this chain is over, nothing is signed
  if (!sess) throw new SignedOutError('not signed in', await noSessionReason());
  const key = sess.key;
  if (w.digest !== undefined && (await digestOf(key)) !== w.digest) throw new SignedOutError('the stored key is not the key of the account this request belongs to');       // a request is signed only with the key of its world
  if (blockedDigest !== null && (await digestOf(key)) !== blockedDigest) blockedDigest = null;      // another account signed in: a new session
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
  if (w.closed) await NEVER;
  try {
    return { r: await fetch(authedURL(path, opts.query), { method, headers, body, exactTimeout: AUTHED_TIMEOUT_MS, redirect: 'manual', ...extra } as RequestInit), key };
  } catch (e) {
    const timed = (e as { kind?: string } | null)?.kind === 'Timeout' || (e as { name?: string } | null)?.name === 'TimeoutError';
    throw new Error(scrub(`${timed ? 'timeout' : 'network'}: ${e instanceof Error ? e.message : 'request failed'}`, key));
  }
}

async function computerId(key: string, kept: string, w: World): Promise<string> {
  if (kept) return kept;
  const digest = await digestOf(key);
  if (discovered && discovered.digest === digest) return discovered.id;
  if (discovering && discovering.digest !== digest) discovering = null;
  if (!discovering) {
    const gen = world.id;
    let mine!: Promise<string>;
    mine = (async () => {
      try {
        const { r, key: used } = await send(w, STATUS_PATH, {}, 'application/json');
        if (used !== key) throw new SignedOutError();       // the account changed while this was asked
        await check(r, STATUS_PATH, key);
        const o = (await r.json()) as { computers?: { computer_id?: string; kind?: string; current?: boolean }[]; warnings?: string[] };
        const list = (o.computers ?? []).filter((c) => (c.kind ?? 'interactive') === 'interactive' && !!c.computer_id);
        const pick = list.find((c) => c.current === true) ?? list[0];
        if (!pick || !pick.computer_id) {
          const warn = (o.warnings ?? [])[0] ?? '';
          throw new Error('the account has no computer yet' + (warn ? ` (${warn.slice(0, 120)})` : '') + '; try again in a moment');
        }
        if (gen !== world.id) throw new SignedOutError();
        discovered = { digest, id: pick.computer_id };
        return pick.computer_id;
      } finally { const cur = discovering as { p: Promise<string> } | null; if (cur && cur.p === mine) discovering = null; }
    })();
    discovering = { digest, p: mine };
  }
  return discovering.p;
}

/** The digest of the key the server last refused (a 401): the account state says "rejected" while that key is the stored one, and another key clears it. */
let rejected = '';
export const rejectedDigest = (): string => rejected;
export const hasNativeHandle = (): boolean => !!reader && reader.available !== false;
async function check(r: Response, path: string, key: string): Promise<void> {
  if (r.ok) { rejected = ''; return; }                                // the server took the key again: it is no longer refused
  if (r.status === 401) { try { rejected = await digestOf(key); } catch { /* the digest is a convenience */ } throw new SignedOutError('the key was refused; sign in again', 'refused'); }
  let msg = '';
  try { const o = JSON.parse(await r.text()); if (o && typeof o.error === 'string') msg = scrub(o.error, key).slice(0, 200); } catch { /* the server's string only; never the body */ }
  const retry = Number(r.headers.get('retry-after'));
  if (r.status >= 300 && r.status < 400) throw new HTTPError(r.status, 'redirect refused: the key goes only to the server it was made for');
  const reason = r.status === 403 ? `403: the key lacks the scope for ${path}` : r.status === 429 ? '429: rate limited' : `HTTP ${r.status}`;
  throw new HTTPError(r.status, scrub(reason + (msg ? `: ${msg}` : ''), key), r.status === 429 && Number.isFinite(retry) && r.headers.get('retry-after') ? retry : null);
}

/** A signed-in JSON request to the listener's own computer. Throws SignedOutError when there is no session. */
export function authedJSON(path: string, opts: Authed = {}): Promise<Reply> { const w = opts.world ?? world; return w.closed ? ended() : inert(w, authedJSONImpl(w, path, opts)); }
async function authedJSONImpl(w: World, path: string, opts: Authed): Promise<Reply> {
  const gen = world.id;
  const { r, key } = await send(w, path, opts, 'application/json');
  await check(r, path, key);
  const raw = await r.text();
  if (gen !== world.id) throw new SignedOutError('signed out while the request was in flight');
  let body: unknown = null;
  if (raw !== '') { try { body = JSON.parse(raw); } catch { throw new Error(`${path} answered a body that is not JSON`); } }
  const date = r.headers.get('date');
  const parsed = date ? Date.parse(date) : NaN;
  return { body, serverTime: Number.isFinite(parsed) ? parsed : null, raw };
}

/** The same, for bytes (a file, a PDF, audio). */
export function authedBytes(path: string, opts: Authed = {}): Promise<{ bytes: Uint8Array; contentType: string }> { const w = opts.world ?? world; return w.closed ? ended() : inert(w, authedBytesImpl(w, path, opts)); }
async function authedBytesImpl(w: World, path: string, opts: Authed): Promise<{ bytes: Uint8Array; contentType: string }> {
  const gen = world.id;
  const { r, key } = await send(w, path, opts, '*/*');
  await check(r, path, key);
  const bytes = new Uint8Array(await r.arrayBuffer());
  if (gen !== world.id) throw new SignedOutError('signed out while the request was in flight');
  return { bytes, contentType: r.headers.get('content-type') ?? '' };
}

/** A signed-in GET whose body the HOST writes to an app file as it arrives (`exactBodyTo`, reference.md, fetch row): the bytes never enter JavaScript, so
 * a 300 MB recording costs neither the engine's 64 MiB heap nor the turn's 100 ms. The file appears whole or not at all (the host stages it beside the
 * target); a reply that is not 2xx writes nothing and is an error as in `authedBytes`. `dest` is under `app:/cache/`, plain. A sign-out in flight
 * removes the file and drops the reply; a host that wrote nothing (it read the body into memory) is an error and the file is removed. */
export function authedToFile(path: string, dest: string, storage: Storage | undefined, opts: Authed = {}): Promise<{ bytes: number; contentType: string }> { const w = opts.world ?? world; return w.closed ? ended() : inert(w, authedToFileImpl(w, path, dest, storage, opts)); }
async function authedToFileImpl(w: World, path: string, dest: string, storage: Storage | undefined, opts: Authed): Promise<{ bytes: number; contentType: string }> {
  if (!/^app:\/cache\/[^\s?#\\]+$/.test(dest) || dest.split('/').some((s) => s === '.' || s === '..')) throw new Error('refused: a download goes to a plain file under app:/cache/');
  const gen = world.id;
  const drop = async () => { try { await storage?.fs.rm(dest); } catch { /* absent */ } };
  const { r, key } = await send(w, path, opts, '*/*', { exactBodyTo: dest });
  try { await check(r, path, key); } catch (e) { await drop(); throw e; }
  const wrote = (r as unknown as { exactBodyTo?: { bytes?: unknown } | null }).exactBodyTo;
  if (gen !== world.id) { await drop(); throw new SignedOutError('signed out while the request was in flight'); }
  if (!wrote || typeof wrote.bytes !== 'number') { await drop(); throw new Error('the host did not write the file'); }
  return { bytes: wrote.bytes, contentType: r.headers.get('content-type') ?? '' };
}

export type KeyState = 'present' | 'absent' | 'unreadable';
/** A deadline, not a limit, on one read of the Keychain through the bridge: a native call that never answers must not hold a page answer, a sign-out or the boundary. */
export const keyTiming = { ms: 5000 };
/** `p`, or `late` when it has not answered within `ms`. (The timer is cleared as soon as `p` answers; a source's own timers end with its answer.) */
export async function withDeadline<T>(p: Promise<T>, late: T, ms: number): Promise<T> {
  let t: ReturnType<typeof setTimeout> | undefined;
  try { return await Promise.race([p, new Promise<T>((r) => { t = setTimeout(() => r(late), ms); })]); } finally { if (t !== undefined) clearTimeout(t); }
}

/** The stored sign-in key as the boundary needs it (docs/design/account-boundary-2026-11-21.md section 2): `absent` ONLY when the Keychain answered item-not-found; `present` with the SHA-256 once the
 * key was read whole; `unreadable` when locked, on a bridge error, past the deadline, when `keystate` says present and the session read then finds nothing, or when a module that answered before is
 * gone; `nomodule` when this host has no native module at all (Android, web: no key exists). An absent key leads to a wipe, so only the Keychain's own answer may say it. Read only. */
export type KeyFact = { state: KeyState | 'nomodule'; digest: string; error?: boolean };
let keyReads = 0;
export async function keyFact(): Promise<KeyFact> {
  keyReads += 1; if (keyReads % 100 === 0) console.log(`api: key reads ${keyReads}`);      // a count, so a spike is seen
  if (!reader || reader.available === false) return nativeAnswered ? { state: 'unreadable', digest: '' } : { state: 'nomodule', digest: '' };
  const read = async (): Promise<KeyFact> => {
    try {
      const r = (await reader!.later({ op: 'keystate' })) as { state?: unknown; found?: unknown } | null;
      if (r && typeof r.state === 'string') nativeAnswered = true;
      const st = r && (r.state === 'present' || r.state === 'absent' || r.state === 'unreadable') ? r.state : r && r.found === true ? 'present' : 'unreadable';
      if (st !== 'present') return { state: st, digest: '' };
      const sess = await keptSession();
      return sess ? { state: 'present', digest: await digestOf(sess.key) } : { state: 'unreadable', digest: '' };
    } catch { return { state: 'unreadable', digest: '', error: true }; }
  };
  return withDeadline(read(), { state: 'unreadable', digest: '' }, keyTiming.ms);
}
/** `keyFact` for the callers that have only three answers (no module reads as unreadable, as before). */
export async function keyState(): Promise<{ state: KeyState; digest: string; error?: boolean }> {
  const k = await keyFact();
  return { ...k, state: k.state === 'nomodule' ? 'unreadable' : k.state };
}

/** For tests: forget what was discovered. */
export function resetSession(): void { discovered = null; discovering = null; }
/** For tests: the generation and the rest as a fresh process has them. */
export function resetSeam(): void { resetSession(); keyTiming.ms = 5000; keyReads = 0; world.close(); world = new World(0); closedWorlds.length = 0; reader = undefined; blockedDigest = null; lastNative = ''; nativeAnswered = false; rejected = ''; }

/** The computer id today's sign-in kept in the Keychain, or null (the Account screen names it when the key answers none). */
export async function keptComputerId(): Promise<string | null> { return keptItem('computer-id'); }
