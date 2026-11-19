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

// PROVISIONAL api seam (the lane exact-signin owns the real one; the lead drops this commit at merge). The signatures are the agreed ones.
// A non-2xx answer throws an HttpError carrying `status`; the files page reads only that and `SignedOutError`.
export class SignedOutError extends Error {}
export class HttpError extends Error { constructor(public status: number, message: string) { super(message); } }
export type Authed = { method?: 'GET' | 'POST' | 'PUT' | 'DELETE'; query?: Query; body?: unknown; headers?: Record<string, string> };
/** A signed-in JSON request to the listener's own computer. Throws SignedOutError when there is no session. */
export async function authedJSON(_path: string, _opts: Authed = {}): Promise<Reply> { throw new SignedOutError('no session (provisional seam)'); }
/** The same, for bytes (a file, a PDF, audio). */
export async function authedBytes(_path: string, _opts: Authed = {}): Promise<{ bytes: Uint8Array; contentType: string }> { throw new SignedOutError('no session (provisional seam)'); }
