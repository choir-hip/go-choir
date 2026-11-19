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

// ---- PROVISIONAL api seam (lane exact-content; the lead drops this commit: lane exact-signin owns the real one) ----
export class SignedOutError extends Error {}
export type Authed = { method?: 'GET' | 'POST' | 'PUT' | 'DELETE'; query?: Query; body?: unknown; headers?: Record<string, string> };

/** PROVISIONAL: the session the real seam reads from the secret store. */
let provisionalSession: { token: string; computer: string } | null = null;
export function provisionalSetSession(s: { token: string; computer: string } | null): void { provisionalSession = s; }

function authHeaders(opts: Authed): Record<string, string> {
  if (!provisionalSession) throw new SignedOutError('not signed in');
  return { authorization: `Bearer ${provisionalSession.token}`, 'x-choir-computer': provisionalSession.computer, ...(opts.headers ?? {}) };
}

/** A signed-in JSON request to the listener's own computer. Throws SignedOutError when there is no session. */
export async function authedJSON(path: string, opts: Authed = {}): Promise<Reply> {
  const headers: Record<string, string> = { accept: 'application/json', ...authHeaders(opts) };
  let body: string | undefined;
  if (opts.body !== undefined) { body = JSON.stringify(opts.body); headers['content-type'] = 'application/json'; }
  const response = await fetch(url(path, opts.query ?? {}), { method: opts.method ?? 'GET', headers, body, exactTimeout: TIMEOUT_MS } as RequestInit);
  if (response.status === 401) throw new SignedOutError('sign-in refused');
  if (!response.ok) throw new Error(`${path} answered ${response.status}`);
  const raw = await response.text();
  let parsed: unknown;
  try { parsed = JSON.parse(raw); } catch { throw new Error(`${path} answered a body that is not JSON`); }
  const date = response.headers.get('date');
  const t = date ? Date.parse(date) : NaN;
  return { body: parsed, serverTime: Number.isFinite(t) ? t : null, raw };
}

/** The same, for bytes (a file, a PDF, audio). */
export async function authedBytes(path: string, opts: Authed = {}): Promise<{ bytes: Uint8Array; contentType: string }> {
  const headers: Record<string, string> = { ...authHeaders(opts) };
  let body: BodyInit | undefined;
  if (opts.body instanceof Uint8Array) body = opts.body as unknown as BodyInit;
  else if (opts.body !== undefined) { body = JSON.stringify(opts.body); headers['content-type'] = 'application/json'; }
  const response = await fetch(url(path, opts.query ?? {}), { method: opts.method ?? 'GET', headers, body, exactTimeout: TIMEOUT_MS } as RequestInit);
  if (response.status === 401) throw new SignedOutError('sign-in refused');
  if (!response.ok) throw new Error(`${path} answered ${response.status}`);
  return { bytes: new Uint8Array(await response.arrayBuffer()), contentType: response.headers.get('content-type') ?? '' };
}
