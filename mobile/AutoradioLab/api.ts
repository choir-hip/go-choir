// (Named api.ts, not server.ts: Exact reads a server.ts beside app.ts as the test backend.)
// The one place that knows where the paper's server is and how a request to it is made.
// Staging and production are the same host today (https://choir.news); a second host is a second constant, never a
// literal elsewhere. The constant lives in app.ts because the build reads the grant `net.fetch ${ORIGIN}` there without running modules.
//
// The routes the shared pages call are public: no sign-in, no token, so nothing here reads the secret store.
// Logs written from here carry counts and ids only, never a title.

import { ORIGIN } from './app.ts';

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
export class SignedOutError extends Error {
  constructor(message = 'not signed in') { super(message); this.name = 'SignedOutError'; }
}

/** The server answered a status outside 200 to 299 (other than 401). `message` is only the server's own short `error` string. */
export class HTTPError extends Error {
  status: number;
  retryAfter: number | null;
  constructor(status: number, message: string, retryAfter: number | null = null) { super(message); this.name = 'HTTPError'; this.status = status; this.retryAfter = retryAfter; }
}

export type Authed = { method?: 'GET' | 'POST' | 'PUT' | 'DELETE'; query?: Query; body?: unknown; headers?: Record<string, string> };

/** The three accounts today's sign-in keeps; the native reader answers no other name. */
export const KEYCHAIN_ACCOUNTS = ['api-key', 'key-id', 'computer-id'] as const;
type Account = (typeof KEYCHAIN_ACCOUNTS)[number];

/** The data module's one native module, as far as this file uses it. */
export type NativeReader = { available?: boolean; later(request: { op: 'get'; account: Account }): Promise<unknown> };
let reader: NativeReader | undefined;
/** app.ts hands over the module's native handle at every answer. */
export function bindNative(native: NativeReader | null | undefined): void { reader = native ?? undefined; }

async function keptItem(account: Account): Promise<string | null> {
  if (!reader || reader.available === false) return null;
  const r = (await reader.later({ op: 'get', account })) as { found?: boolean; value?: unknown } | null;
  return r && r.found === true && typeof r.value === 'string' && r.value !== '' ? r.value : null;
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

// The computer discovered for a key (memory only; today's sign-in keeps the id in the Keychain and that is read first).
let discovered: { key: string; id: string } | null = null;
let discovering: Promise<string> | null = null;

function scrub(text: string, key: string): string { return key ? text.split(key).join('[key]') : text; }

async function send(path: string, opts: Authed, accept: string): Promise<{ r: Response; key: string }> {
  if (!plainPath(path)) throw new Error('refused: a signed-in path is a plain absolute path on the server');
  const key = await keptItem('api-key');
  if (!key) throw new SignedOutError();
  const headers: Record<string, string> = { ...(opts.headers ?? {}), Accept: accept, Authorization: `Bearer ${key}` };
  const method = opts.method ?? 'GET';
  let body: string | undefined;
  if (opts.body !== undefined) { body = JSON.stringify(opts.body); headers['Content-Type'] = 'application/json'; }
  if (!path.startsWith('/auth/') && path !== STATUS_PATH) headers['X-Choir-Computer'] = await computerId(key);
  try {
    return { r: await fetch(authedURL(path, opts.query), { method, headers, body, exactTimeout: AUTHED_TIMEOUT_MS } as RequestInit), key };
  } catch (e) {
    throw new Error(scrub(`network: ${e instanceof Error ? e.message : 'request failed'}`, key));
  }
}

async function computerId(key: string): Promise<string> {
  const kept = await keptItem('computer-id');
  if (kept) return kept;
  if (discovered && discovered.key === key) return discovered.id;
  if (!discovering) {
    discovering = (async () => {
      try {
        const { r } = await send(STATUS_PATH, {}, 'application/json');
        await check(r, STATUS_PATH, key);
        const o = (await r.json()) as { computers?: { computer_id?: string; kind?: string; current?: boolean }[]; warnings?: string[] };
        const list = (o.computers ?? []).filter((c) => (c.kind ?? 'interactive') === 'interactive' && !!c.computer_id);
        const pick = list.find((c) => c.current === true) ?? list[0];
        if (!pick || !pick.computer_id) {
          const warn = (o.warnings ?? [])[0] ?? '';
          throw new Error('the account has no computer yet' + (warn ? ` (${warn.slice(0, 120)})` : '') + '; try again in a moment');
        }
        discovered = { key, id: pick.computer_id };
        return pick.computer_id;
      } finally { discovering = null; }
    })();
  }
  return discovering;
}

async function check(r: Response, path: string, key: string): Promise<void> {
  if (r.ok) return;
  if (r.status === 401) throw new SignedOutError('the key was refused; sign in again');
  let msg = '';
  try { const o = JSON.parse(await r.text()); if (o && typeof o.error === 'string') msg = o.error.slice(0, 200); } catch { /* the server's string only; never the body */ }
  const retry = Number(r.headers.get('retry-after'));
  const reason = r.status === 403 ? `403: the key lacks the scope for ${path}` : r.status === 429 ? '429: rate limited' : `HTTP ${r.status}`;
  throw new HTTPError(r.status, scrub(reason + (msg ? `: ${msg}` : ''), key), r.status === 429 && Number.isFinite(retry) && r.headers.get('retry-after') ? retry : null);
}

/** A signed-in JSON request to the listener's own computer. Throws SignedOutError when there is no session. */
export async function authedJSON(path: string, opts: Authed = {}): Promise<Reply> {
  const { r, key } = await send(path, opts, 'application/json');
  await check(r, path, key);
  const raw = await r.text();
  let body: unknown = null;
  if (raw !== '') { try { body = JSON.parse(raw); } catch { throw new Error(`${path} answered a body that is not JSON`); } }
  const date = r.headers.get('date');
  const parsed = date ? Date.parse(date) : NaN;
  return { body, serverTime: Number.isFinite(parsed) ? parsed : null, raw };
}

/** The same, for bytes (a file, a PDF, audio). */
export async function authedBytes(path: string, opts: Authed = {}): Promise<{ bytes: Uint8Array; contentType: string }> {
  const { r, key } = await send(path, opts, '*/*');
  await check(r, path, key);
  return { bytes: new Uint8Array(await r.arrayBuffer()), contentType: r.headers.get('content-type') ?? '' };
}

/** For tests: forget what was discovered. */
export function resetSession(): void { discovered = null; discovering = null; }

/** The computer id today's sign-in kept in the Keychain, or null (the Account screen names it when the key answers none). */
export async function keptComputerId(): Promise<string | null> { return keptItem('computer-id'); }
