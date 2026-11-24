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
export async function getJSON(path: string, query: Query = {}): Promise<Reply> {
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

export type Authed = { method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'; query?: Query; body?: unknown; headers?: Record<string, string> };

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

/** The app module's `later`, for the native calls shared code makes; null when there is no module. */
export function nativeLater(): ((request: { op: string; account?: Account; b64?: string; aad?: string; [key: string]: unknown }) => Promise<unknown>) | null {
  if (!reader || reader.available === false) return null;
  const native = reader;
  // The first native call of a JS context announces this context's generation, as a BOOT (the native module instance lives per session, a reload of the bundle does not restart it: docs/reference/exact/native-modules.md "Threading and lifecycle"), before anything else
  // goes over: native adopts a boot even when it is below the newest generation it saw in an earlier context, so a restart at 0 is not refused for the life of the process (AutoradioKit `SharedGeneration.boot`).
  // Sent first, not waited for: native takes its calls in the order they arrive, and a native that never answers the announcement must not stop every call behind it.
  return (r) => {
    if (r.op !== 'generation' && !bootSent) bootSent = Promise.resolve(native.later({ op: 'generation', gen: generation, boot: true })).then(() => undefined, (e: unknown) => { console.log(`api: the boot announcement failed (${e instanceof Error ? e.message : 'threw'}): native keeps the generation it last saw`); });
    return native.later(r);
  };
}
let bootSent: Promise<void> | null = null;
/** For tests: a fresh JS context (the boot is announced again). */
export function resetBootForTest(): void { bootSent = null; }
/** Tell native the generation shared code is at NOW (a floor: native refuses what is stamped below it from here on). The wipe's last act, so nothing computed before it can land after it. */
/** A deadline, not a limit: a native that does not answer must not hold the sign-out; the failure is said in the log (the proof reads it) and native keeps refusing by the generation it last knew. */
let announceDeadlineMs = 5000;
export function setAnnounceDeadlineForTest(ms: number | undefined): void { announceDeadlineMs = ms ?? 5000; }
export async function announceGeneration(): Promise<void> {
  const later = nativeLater();
  if (!later) return;
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    const out = await Promise.race([later({ op: 'generation', gen: generation }).then(() => 'ok', (e: unknown) => `threw: ${e instanceof Error ? e.message : String(e)}`), new Promise<string>((res) => { timer = setTimeout(() => res(`no answer in ${announceDeadlineMs} ms`), announceDeadlineMs); })]);
    if (out !== 'ok') console.log(`account: the generation floor ${generation} was not announced to native (${out}); native refuses by the generation it last knew`);
  } catch (e) { console.log(`account: the generation floor ${generation} was not announced to native (${e instanceof Error ? e.message : 'threw'})`); }
  finally { if (timer !== undefined) clearTimeout(timer); }
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


// ACCOUNT GENERATION (finding 5). Bumped by sign-out before the wipes. A reply that started under an older generation is dropped as signed out,
// so a request in flight at sign-out can never write the old account's data back into a store the wipe just emptied.
let generation = 0;
export const accountGeneration = (): number => generation;
export function nextGeneration(): number { generation += 1; return generation; }

// A SIGN-OUT UNDER WAY (review 2026-11-19, mail seat 1, finding 2). `beginSignOut` runs BEFORE the native sign-out: the generation moves at once (a store write
// that began under the old one refuses itself, mail-store.ts), and mail writes are off until a NEW session is established (a key whose digest is not the one that
// was signed out) or the sign-out is given up (`abortSignOut`).
let blockedDigest: string | null = null;      // null: not signing out. '' or a digest: a sign-out began; mail writes wait for a session whose key is another one.
export const signOutBlocked = (): boolean => blockedDigest !== null;
export async function beginSignOut(): Promise<void> {
  nextGeneration(); resetSession();
  blockedDigest = '';                                    // the block first, before the awaits below: while the key is being read a pulse must not see a sign-out that is not blocked
  let d = '';
  try { const s = await keptSession(); if (s) d = await digestOf(s.key); } catch { /* no key to name */ }
  blockedDigest = d;
}
/** `certain` false: native threw, so whether the key and its stores went is unknown: the radio is NOT handed the new generation (it keeps the old stamp and native refuses it) until the account is settled. */
export function abortSignOut(certain = true): void { blockedDigest = null; uncertainAbort = !certain; }
let uncertainAbort = false;
export const signOutUncertain = (): boolean => uncertainAbort;
/** A new session clears the block explicitly: called before a write is judged, so the first sync of the next session is not answered idle. */
export async function refreshSignOutBlock(): Promise<void> {
  if (blockedDigest === null) return;
  try { const sess = await keptSession(); if (sess && (await digestOf(sess.key)) !== blockedDigest) blockedDigest = null; } catch { /* still signed out */ }
}

/** ONE rule for every moved store's file write (review 2026-11-19): `gen` is the account generation the SOURCE started under (`accountGeneration()` before its request). The write is
 * refused if a sign-out has moved the generation, and a file that lands after the generation moved is removed again. True when the file is there. */
export async function writeCurrent(gen: number, storage: Storage | undefined, path: string, bytes: Uint8Array): Promise<boolean> {
  if (!storage || gen !== generation) return false;
  try { await storage.fs.atomicWriteFile(path, bytes); } catch { return false; }
  if (gen !== generation) { try { await storage.fs.rm(path); } catch { /* the wipe will */ } return false; }
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

async function send(path: string, opts: Authed, accept: string, extra: Record<string, unknown> = {}): Promise<{ r: Response; key: string }> {
  if (!plainPath(path)) throw new Error('refused: a signed-in path is a plain absolute path on the server');
  const sess = await keptSession();
  if (!sess) throw new SignedOutError('not signed in', await noSessionReason());
  const key = sess.key;
  if (blockedDigest !== null && (await digestOf(key)) !== blockedDigest) blockedDigest = null;      // another account signed in: a new session
  const headers: Record<string, string> = { ...(opts.headers ?? {}), Accept: accept, Authorization: `Bearer ${key}` };
  const method = opts.method ?? 'GET';
  let body: string | undefined;
  if (opts.body instanceof Uint8Array) {
    // Raw bytes go as they are (a file added to the listener's computer: PUT /api/files/{path}); the caller names the type.
    body = opts.body as unknown as string; headers['Content-Type'] = headers['content-type'] ?? headers['Content-Type'] ?? 'application/octet-stream'; delete headers['content-type'];
  } else if (opts.body !== undefined) { body = JSON.stringify(opts.body); headers['Content-Type'] = 'application/json'; }
  if (!path.startsWith('/auth/') && path !== STATUS_PATH) {
    try { headers['X-Choir-Computer'] = await computerId(key, sess.computerId); } catch (e) { console.log(`api: computer id failed: ${e instanceof Error ? e.name : 'error'}`); throw e; }
  }
  try {
    return { r: await fetch(authedURL(path, opts.query), { method, headers, body, exactTimeout: AUTHED_TIMEOUT_MS, redirect: 'manual', ...extra } as RequestInit), key };
  } catch (e) {
    const timed = (e as { kind?: string } | null)?.kind === 'Timeout' || (e as { name?: string } | null)?.name === 'TimeoutError';
    throw new Error(scrub(`${timed ? 'timeout' : 'network'}: ${e instanceof Error ? e.message : 'request failed'}`, key));
  }
}

async function computerId(key: string, kept: string): Promise<string> {
  if (kept) return kept;
  const digest = await digestOf(key);
  if (discovered && discovered.digest === digest) return discovered.id;
  if (discovering && discovering.digest !== digest) discovering = null;
  if (!discovering) {
    const gen = generation;
    let mine!: Promise<string>;
    mine = (async () => {
      try {
        const { r, key: used } = await send(STATUS_PATH, {}, 'application/json');
        if (used !== key) throw new SignedOutError();       // the account changed while this was asked
        await check(r, STATUS_PATH, key);
        const o = (await r.json()) as { computers?: { computer_id?: string; kind?: string; current?: boolean }[]; warnings?: string[] };
        const list = (o.computers ?? []).filter((c) => (c.kind ?? 'interactive') === 'interactive' && !!c.computer_id);
        const pick = list.find((c) => c.current === true) ?? list[0];
        if (!pick || !pick.computer_id) {
          const warn = (o.warnings ?? [])[0] ?? '';
          throw new Error('the account has no computer yet' + (warn ? ` (${warn.slice(0, 120)})` : '') + '; try again in a moment');
        }
        if (gen !== generation) throw new SignedOutError();
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
export async function authedJSON(path: string, opts: Authed = {}): Promise<Reply> {
  const gen = generation;
  const { r, key } = await send(path, opts, 'application/json');
  await check(r, path, key);
  const raw = await r.text();
  if (gen !== generation) throw new SignedOutError('signed out while the request was in flight');
  let body: unknown = null;
  if (raw !== '') { try { body = JSON.parse(raw); } catch { throw new Error(`${path} answered a body that is not JSON`); } }
  const date = r.headers.get('date');
  const parsed = date ? Date.parse(date) : NaN;
  return { body, serverTime: Number.isFinite(parsed) ? parsed : null, raw };
}

/** The whole answer as text, whatever its type (the programme route's event stream, read when it has ended: Exact's `exactStream` is not shown to take a POST, docs/reference/exact/gaps.md 36). */
export async function authedText(path: string, opts: Authed = {}, accept = 'text/event-stream'): Promise<string> {
  const gen = generation;
  const { r, key } = await send(path, opts, accept);
  await check(r, path, key);
  const raw = await r.text();
  if (gen !== generation) throw new SignedOutError('signed out while the request was in flight');
  return raw;
}

/** The same, for bytes (a file, a PDF, audio). */
export async function authedBytes(path: string, opts: Authed = {}): Promise<{ bytes: Uint8Array; contentType: string }> {
  const gen = generation;
  const { r, key } = await send(path, opts, '*/*');
  await check(r, path, key);
  const bytes = new Uint8Array(await r.arrayBuffer());
  if (gen !== generation) throw new SignedOutError('signed out while the request was in flight');
  return { bytes, contentType: r.headers.get('content-type') ?? '' };
}

/** A signed-in GET whose body the HOST writes to an app file as it arrives (`exactBodyTo`, reference.md, fetch row): the bytes never enter JavaScript, so
 * a 300 MB recording costs neither the engine's 64 MiB heap nor the turn's 100 ms. The file appears whole or not at all (the host stages it beside the
 * target); a reply that is not 2xx writes nothing and is an error as in `authedBytes`. `dest` is under `app:/cache/`, plain. A sign-out in flight
 * removes the file and drops the reply; a host that wrote nothing (it read the body into memory) is an error and the file is removed. */
export async function authedToFile(path: string, dest: string, storage: Storage | undefined, opts: Authed = {}): Promise<{ bytes: number; contentType: string }> {
  if (!/^app:\/cache\/[^\s?#\\]+$/.test(dest) || dest.split('/').some((s) => s === '.' || s === '..')) throw new Error('refused: a download goes to a plain file under app:/cache/');
  const gen = generation;
  const drop = async () => { try { await storage?.fs.rm(dest); } catch { /* absent */ } };
  const { r, key } = await send(path, opts, '*/*', { exactBodyTo: dest });
  try { await check(r, path, key); } catch (e) { await drop(); throw e; }
  const wrote = (r as unknown as { exactBodyTo?: { bytes?: unknown } | null }).exactBodyTo;
  if (gen !== generation) { await drop(); throw new SignedOutError('signed out while the request was in flight'); }
  if (!wrote || typeof wrote.bytes !== 'number') { await drop(); throw new Error('the host did not write the file'); }
  return { bytes: wrote.bytes, contentType: r.headers.get('content-type') ?? '' };
}

export type KeyState = 'present' | 'absent' | 'unreadable';
/** The stored sign-in key as THREE answers (review 2026-11-19, auth round 2): `present` (with the SHA-256 of the key), `absent` (the Keychain answered item-not-found) or
 * `unreadable` (locked before the first unlock, any other status, no native module, a bridge error). Read only. `unreadable` is never "signed out". */
export async function keyState(): Promise<{ state: KeyState; digest: string; error?: boolean }> {
  const none = { state: 'unreadable' as KeyState, digest: '' };
  if (!reader || reader.available === false) return none;
  try {
    const r = (await reader.later({ op: 'keystate' })) as { state?: unknown; found?: unknown } | null;
    if (r && typeof r.state === 'string') nativeAnswered = true;
    const st = r && (r.state === 'present' || r.state === 'absent' || r.state === 'unreadable') ? r.state : r && r.found === true ? 'present' : 'unreadable';
    if (st !== 'present') return { state: st, digest: '' };
    const sess = await keptSession();
    return sess ? { state: 'present', digest: await digestOf(sess.key) } : none;
  } catch { return { ...none, error: true }; }
}

/** For tests: forget what was discovered. */
export function resetSession(): void { discovered = null; discovering = null; }
/** For tests: the generation and the rest as a fresh process has them. */
export function resetSeam(): void { resetSession(); bootSent = null; uncertainAbort = false; generation = 0; reader = undefined; blockedDigest = null; lastNative = ''; nativeAnswered = false; rejected = ''; }

/** The computer id today's sign-in kept in the Keychain, or null (the Account screen names it when the key answers none). */
export async function keptComputerId(): Promise<string | null> { return keptItem('computer-id'); }
