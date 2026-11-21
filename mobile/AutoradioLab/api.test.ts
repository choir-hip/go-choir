// The signed-in seam (api.ts: authedJSON, authedBytes, SignedOutError) under bun test, `fetch` and the native reader stubbed.
// Every value here is made up. Failure list, written before the code (each line has a test below):
//  A1 no native reader, or no key kept: SignedOutError, and nothing is sent.
//  A2 a key kept: the request goes to ORIGIN with `Authorization: Bearer <key>`, `Accept: application/json` and `X-Choir-Computer: <id>`.
//  A3 the computer id kept by today's sign-in is used as it is, with no discovery call.
//  A4 no computer id kept: one GET /api/compute/status (no X-Choir-Computer) picks the current interactive computer; a second request reuses it.
//  A5 /auth/ routes and /api/compute/status carry no X-Choir-Computer (Swift: ChoirClient.request).
//  A6 a body is JSON with Content-Type; a query is sorted by name and escaped as Swift does (& + = ; escaped, / : @ , kept).
//  A7 401 is a SignedOutError (the key was refused; the key is not deleted: sign-in is the user's act); 403, 429 (Retry-After), 5xx are HTTPError
//     carrying the server's own short `error` string and never the body.
//  A8 a path that could carry the key elsewhere (absolute URL, "//host", no leading slash) is refused before anything is sent.
//  A9 the key never appears in an error message or in what the seam logs.
//  A10 authedBytes returns the bytes and the content type, and asks for any type.
//  A11 a changed key (another account signed in) does not reuse the old account's computer id.
//  A12 a transport failure is an Error, not a SignedOutError.
//  A13 authedToFile: the body goes to the named app file by the host (`exactBodyTo`), the reply is never read, a path outside app:/cache is refused before the key is read,
//      a refusal is an error with no file, a sign-out in flight removes the file and drops the reply, a host that wrote nothing is an error.
import { beforeEach, describe, expect, test } from 'bun:test';
import { dig } from './account-phone.ts';
import { currentWorld as theWorld } from './api.ts';
import { SignedOutError, HTTPError, bindNative, resetSession, resetSeam, nextGeneration, ORIGIN, authedURL, KEYCHAIN_ACCOUNTS } from './api.ts';
// A signed-in request states its world (api.ts: a missing one throws): the tests state the one current when they ask.
import * as apiReal from './api.ts';
const authedJSON = (p: string, o: object = {}) => apiReal.authedJSON(p, { ...o, world: apiReal.currentWorld() } as never);
const authedBytes = (p: string, o: object = {}) => apiReal.authedBytes(p, { ...o, world: apiReal.currentWorld() } as never);
const authedToFile = (p: string, d: string, s: never, o: object = {}) => apiReal.authedToFile(p, d, s, { ...o, world: apiReal.currentWorld() } as never);


const KEY = 'test-key-0001-not-real';
const KEY2 = 'test-key-0002-not-real';
let kept: Record<string, string> = {};
let reads: string[] = [];
const native = { available: true, later: async (r: { op?: string; account?: string }) => { if (r.op === 'session') { reads.push('session'); const k = kept['api-key']; return k === undefined ? { found: false } : { found: true, value: k, computerId: kept['computer-id'] ?? '' }; } reads.push(r.account as string); const v = kept[r.account as string]; return v === undefined ? { found: false } : { found: true, value: v }; } };

type Call = { url: string; init: RequestInit };
let calls: Call[] = [];
let handler: (c: Call) => Response;
const header = (c: Call, n: string) => (c.init.headers as Record<string, string>)[n];

beforeEach(() => {
  kept = { 'api-key': KEY, 'computer-id': 'comp-1' };
  reads = []; calls = []; resetSeam();
  bindNative(native as never); theWorld().digest = dig(KEY);      // the boundary stamps a world with the key it read; a test does it by hand
  handler = () => new Response('{"ok":true}', { status: 200, headers: { 'content-type': 'application/json' } });
  globalThis.fetch = (async (u: string, init: RequestInit) => { const c = { url: String(u), init }; calls.push(c); return handler(c); }) as never;
});

describe('signed-in seam', () => {
  test('A1 no reader, or no key: SignedOutError and nothing sent', async () => {
    kept = {};
    await expect(authedJSON('/api/x')).rejects.toBeInstanceOf(SignedOutError);
    bindNative(undefined);
    kept = { 'api-key': KEY };
    await expect(authedJSON('/api/x')).rejects.toBeInstanceOf(SignedOutError);
    expect(calls.length).toBe(0);
  });
  test('A2 headers', async () => {
    const r = await authedJSON('/api/x', { query: { a: 1 } });
    expect(r.body).toEqual({ ok: true });
    expect(calls.length).toBe(1);
    expect(calls[0].url).toBe(`${ORIGIN}/api/x?a=1`);
    expect(calls[0].init.method).toBe('GET');
    expect(header(calls[0], 'Authorization')).toBe(`Bearer ${KEY}`);
    expect(header(calls[0], 'Accept')).toBe('application/json');
    expect(header(calls[0], 'X-Choir-Computer')).toBe('comp-1');
  });
  test('A3 the kept computer id: no discovery', async () => {
    await authedJSON('/api/x'); await authedJSON('/api/y');
    expect(calls.map((c) => c.url)).toEqual([`${ORIGIN}/api/x`, `${ORIGIN}/api/y`]);
  });
  test('A4 discovery picks the current interactive computer, once', async () => {
    kept = { 'api-key': KEY };
    handler = (c) => c.url.endsWith('/api/compute/status')
      ? new Response(JSON.stringify({ computers: [{ computer_id: 'c-other', kind: 'interactive' }, { computer_id: 'c-batch', kind: 'batch', current: true }, { computer_id: 'c-cur', current: true }] }), { status: 200 })
      : new Response('{}', { status: 200 });
    await authedJSON('/api/x'); await authedJSON('/api/y');
    expect(calls.map((c) => c.url.replace(ORIGIN, ''))).toEqual(['/api/compute/status', '/api/x', '/api/y']);
    expect(header(calls[0], 'X-Choir-Computer')).toBeUndefined();
    expect(header(calls[1], 'X-Choir-Computer')).toBe('c-cur');
    expect(header(calls[2], 'X-Choir-Computer')).toBe('c-cur');
  });
  test('A4b no computer yet is an Error saying so, not a sign-out', async () => {
    kept = { 'api-key': KEY };
    handler = () => new Response('{"computers":[],"warnings":["still starting"]}', { status: 200 });
    const e = await authedJSON('/api/x').catch((x) => x);
    expect(e).toBeInstanceOf(Error); expect(e).not.toBeInstanceOf(SignedOutError); expect(String(e.message)).toContain('no computer yet');
  });
  test('A5 auth routes and status carry no computer header', async () => {
    await authedJSON('/auth/api-keys/abc', { method: 'DELETE' });
    await authedJSON('/api/compute/status');
    expect(header(calls[0], 'X-Choir-Computer')).toBeUndefined();
    expect(header(calls[1], 'X-Choir-Computer')).toBeUndefined();
    expect(header(calls[0], 'Authorization')).toBe(`Bearer ${KEY}`);
  });
  test('A6 body and query', async () => {
    await authedJSON('/api/x', { method: 'POST', body: { q: 'AT&T' }, query: { z: 'C++', a: 'x/y:z@w,v', m: 'a b=c;d' } });
    expect(calls[0].url).toBe(`${ORIGIN}/api/x?a=x/y:z@w,v&m=a%20b%3Dc%3Bd&z=C%2B%2B`);
    expect(calls[0].init.body).toBe('{"q":"AT&T"}');
    expect(header(calls[0], 'Content-Type')).toBe('application/json');
    await authedJSON('/api/x');
    expect(header(calls[1], 'Content-Type')).toBeUndefined();
  });
  test('A7 statuses', async () => {
    handler = () => new Response('{"error":"nope"}', { status: 401 });
    await expect(authedJSON('/api/x')).rejects.toBeInstanceOf(SignedOutError);
    handler = () => new Response('{"error":"nope"}', { status: 403 });
    const f = await authedJSON('/api/x').catch((x) => x);
    expect(f).toBeInstanceOf(HTTPError); expect(f.status).toBe(403); expect(f.message).toContain('nope');
    handler = () => new Response('{}', { status: 429, headers: { 'retry-after': '7' } });
    const r = await authedJSON('/api/x').catch((x) => x);
    expect(r.status).toBe(429); expect(r.retryAfter).toBe(7);
    handler = () => new Response(`{"error":"${'e'.repeat(500)}","echo":"secret-body"}`, { status: 500 });
    const s = await authedJSON('/api/x').catch((x) => x);
    expect(s.status).toBe(500); expect(s.message).not.toContain('secret-body'); expect(s.message.length).toBeLessThan(260);
    expect(kept['api-key']).toBe(KEY); // a refused key is not deleted
  });
  test('A8 a path that could aim the key elsewhere is refused', async () => {
    for (const p of ['https://evil.example.com/x', '//evil.example.com/x', 'api/x', '/a/../../x', '/x\\y', '']) {
      await expect(authedJSON(p)).rejects.toThrow();
    }
    expect(calls.length).toBe(0);
    expect(reads.length).toBe(0);
  });
  test('A9 the key is in no message', async () => {
    handler = () => new Response(`{"error":"${KEY}"}`, { status: 500 });
    const e = await authedJSON('/api/x').catch((x) => x);
    expect(String(e.message)).not.toContain(KEY);
    handler = () => { throw new Error(`boom ${KEY}`); };
    const t = await authedJSON('/api/x').catch((x) => x);
    expect(String(t.message)).not.toContain(KEY);
  });
  test('A9b a key straddling the 200-character cut is scrubbed before the cut, not after', async () => {
    handler = () => new Response(JSON.stringify({ error: 'x'.repeat(190) + KEY }), { status: 500 });
    const e = await authedJSON('/api/x').catch((x) => x);
    expect(String(e.message)).not.toContain(KEY.slice(0, 10));
  });
  test('A10 bytes', async () => {
    handler = () => new Response(new Uint8Array([1, 2, 3]), { status: 200, headers: { 'content-type': 'application/pdf' } });
    const r = await authedBytes('/api/file/1');
    expect(Array.from(r.bytes)).toEqual([1, 2, 3]); expect(r.contentType).toBe('application/pdf');
    expect(header(calls[0], 'Accept')).toBe('*/*');
    expect(header(calls[0], 'Authorization')).toBe(`Bearer ${KEY}`);
    handler = () => new Response('', { status: 401 });
    await expect(authedBytes('/api/file/1')).rejects.toBeInstanceOf(SignedOutError);
  });
  test('A11 another account does not reuse the old computer', async () => {
    kept = { 'api-key': KEY };
    handler = (c) => c.url.endsWith('/api/compute/status') ? new Response(JSON.stringify({ computers: [{ computer_id: header(c, 'Authorization').includes(KEY2) ? 'c-two' : 'c-one' }] }), { status: 200 }) : new Response('{}', { status: 200 });
    await authedJSON('/api/x');
    expect(header(calls[1], 'X-Choir-Computer')).toBe('c-one');
    kept = { 'api-key': KEY2 }; apiReal.closeWorld(); theWorld().digest = dig(KEY2);      // another account is another world, stamped with its own key
    await authedJSON('/api/x');
    expect(header(calls[calls.length - 1], 'X-Choir-Computer')).toBe('c-two');
  });
  test('A12 transport failure', async () => {
    handler = () => { throw new TypeError('network down'); };
    const e = await authedJSON('/api/x').catch((x) => x);
    expect(e).toBeInstanceOf(Error); expect(e).not.toBeInstanceOf(SignedOutError);
  });
  test('only the three accounts today\'s sign-in keeps are readable', async () => {
    expect(KEYCHAIN_ACCOUNTS).toEqual(['api-key', 'key-id', 'computer-id']);
    expect(authedURL('/api/x', {})).toBe(`${ORIGIN}/api/x`);
  });
});

describe('review 2026-11-19: generation, redirect, one read', () => {
  test('N1 a source answered WITHOUT a usable native handle does not unbind the one that works (the signed-out screens of the first real run)', async () => {
    const { adoptNative } = await import('./api.ts');
    resetSeam(); theWorld().digest = dig(KEY); adoptNative(native as never, 'paperFront');
    adoptNative(undefined, 'mailBox'); adoptNative({ available: false, later: native.later } as never, 'filesList');
    const r = await authedJSON('/api/x');
    expect(r.body).toEqual({ ok: true }); expect(header(calls[0], 'Authorization')).toBe(`Bearer ${KEY}`);
  });
  test('N1 with no handle ever given, the seam is signed out (nothing invented)', async () => {
    const { adoptNative } = await import('./api.ts');
    resetSeam(); adoptNative(undefined, 'x');
    await expect(authedJSON('/api/x')).rejects.toBeInstanceOf(SignedOutError);
  });
  test('G1 a reply that finished after a sign-out is dropped as signed out, JSON and bytes', async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async (u: string, init: RequestInit) => { calls.push({ url: String(u), init }); await gate; return new Response('{"old":"account"}', { status: 200 }); }) as never;
    const j = authedJSON('/api/x'); const b = authedBytes('/api/y');
    await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); release();
    // The world mechanism (api.ts): a reply that arrives after the boundary moved never reaches the chain that asked: its promise does not settle (it is not "rejected": a rejection would run the caller's catch branch).
    const settled = await Promise.race([Promise.allSettled([j, b]).then(() => 'settled'), new Promise((r) => setTimeout(() => r('never'), 60))]);
    expect(settled).toBe('never');
  });
  test('R1 the key reaches the server only: redirects are not followed, a 3xx is an error', async () => {
    handler = () => new Response('', { status: 302, headers: { location: 'https://evil.example.com/' } });
    await expect(authedJSON('/api/x')).rejects.toBeInstanceOf(HTTPError);
    expect((calls[0].init as { redirect?: string }).redirect).toBe('manual');
  });
  test('O1 the key and the computer id come from ONE native read', async () => {
    await authedJSON('/api/x');
    expect(reads).toEqual(['session']);
  });
  test('K1 what is kept after discovery is a digest: the key text is in no field of the module state', async () => {
    kept = { 'api-key': KEY };
    handler = (c) => c.url.endsWith('/api/compute/status') ? new Response(JSON.stringify({ computers: [{ computer_id: 'c9', kind: 'interactive', current: true }] }), { status: 200 }) : new Response('{}', { status: 200 });
    await authedJSON('/api/x');
    const api = await import('./api.ts');
    expect(JSON.stringify(Object.values(api).map((v) => typeof v === 'function' ? '' : v))).not.toContain(KEY);
  });
});

describe('A13 the signed-in download to a file', () => {
  const DEST = 'app:/cache/files/abc/talk.mp4';
  const storage = (removed: string[]) => ({ fs: { rm: async (p: string) => { removed.push(p); } } }) as never;
  const wrote = (bytes: number) => { const r = new Response('', { status: 200 }) as Response & { exactBodyTo?: unknown }; Object.defineProperty(r, 'exactBodyTo', { value: { path: DEST, bytes } }); return r; };
  test('the host is asked to write the file; the key goes only in the header; the reply body is never read', async () => {
    let read = false;
    handler = () => { const r = wrote(60_000_000); r.arrayBuffer = async () => { read = true; return new ArrayBuffer(0); }; r.text = async () => { read = true; return ''; }; return r; };
    const got = await authedToFile('/api/files/talk.mp4', DEST, storage([]), { query: { disposition: 'inline' } });
    expect(got.bytes).toBe(60_000_000); expect(read).toBe(false);
    expect((calls[0].init as { exactBodyTo?: string }).exactBodyTo).toBe(DEST);
    expect(header(calls[0], 'Authorization')).toBe(`Bearer ${KEY}`); expect(header(calls[0], 'Accept')).toBe('*/*');
    expect(calls[0].url).toBe(`${ORIGIN}/api/files/talk.mp4?disposition=inline`); expect((calls[0].init as { redirect?: string }).redirect).toBe('manual');
  });
  test('a destination outside app:/cache, with a .. or in a text of its own is refused before the key is read', async () => {
    for (const d of ['app:/data/x', 'app:/cache/../data/x', 'https://evil.example/x', '/etc/passwd', 'app:/cache']) await expect(authedToFile('/api/files/a', d, storage([]))).rejects.toThrow();
    expect(calls.length).toBe(0); expect(reads.length).toBe(0);
  });
  test('401 is a sign-out, 404 and 500 are errors; none keeps a file', async () => {
    const removed: string[] = [];
    handler = () => new Response('', { status: 401 }); await expect(authedToFile('/api/files/a', DEST, storage(removed))).rejects.toBeInstanceOf(SignedOutError);
    handler = () => new Response('{"error":"gone"}', { status: 404 }); const e = await authedToFile('/api/files/a', DEST, storage(removed)).catch((x) => x);
    expect(e).toBeInstanceOf(HTTPError); expect(e.status).toBe(404);
  });
  test('a host that wrote nothing (an older runtime read the body into memory) is an error, and nothing is kept', async () => {
    const removed: string[] = [];
    handler = () => new Response('hello', { status: 200 });
    await expect(authedToFile('/api/files/a', DEST, storage(removed))).rejects.toThrow(/did not write/);
    expect(removed).toContain(DEST);
  });
  test('a sign-out while the file arrives: the reply is dropped, and the file is left to the Clear (it removes the cache by path; a remove here could take the next account\'s file at the same path)', async () => {
    const removed: string[] = [];
    handler = () => { nextGeneration(); return wrote(10); };
    await Promise.race([authedToFile('/api/files/a', DEST, storage(removed)).then(() => undefined, () => undefined), new Promise((r) => setTimeout(r, 40))]);      // the reply never reaches the caller (the world is closed)
    expect(removed).not.toContain(DEST);
  });
});
