// THE KEY IS EXACT'S (secret.keep): docs/design/account-boundary-2026-11-21.md, "Round 12". Failure list, written before the code:
//  K1  a key in the store at launch opens that account's world (digest of the key); native is handed the key in memory (`holdKey`), never asked to read or decide anything.
//  K2  no key and no marker: the anonymous world.
//  K3  no key but the marker says an account was signed in here: STARTING (locked), asked again; the key appearing (Exact's retry re-asks the answer) opens the account; after a bound the launch is anonymous and Sign in is offered.
//  K4  an unreadable marker is STARTING too.
//  K5  sign-in: begin asks the server, the poll stores the key and key id with `store.set`, writes the marker, hands native the key, and the anonymous world moves to the account's once.
//  K6  sign-out: native is told to drop the key FIRST (before the ending mark is written), the key is revoked on the server then forgotten, the session ends.
//  K7  a launch that finds the ending mark finishes it: revoke, forget the three secrets, sweep every root, ask native to forget its files, remove the markers (the ending mark last); a failure of any is blocked.
//  K8  the sign-in poll never stores a key after a sign-out or a cancel, and `denied` and `expired` end the flow with a sentence.
import { afterEach, describe, expect, test } from 'bun:test';
import * as account from './account.ts';
import * as api from './api.ts';
import * as signin from './signin.ts';
import { dig, events, fakeStore, KEY_A, memFs, type Mem } from './account-phone.ts';

const SIGNED = 'app:/data/signed-in';
const ROOT = (k: string): string => `app:/data/accounts/${dig(k)}`;
type Calls = { op: string; [k: string]: unknown }[];
let m: Mem; let calls: Calls; let secrets: Map<string, string>; let fetches: { url: string; method: string; auth: string }[]; let respond: (url: string, method: string) => Response;
let fs: ReturnType<typeof fakeStore>;      // Exact's store as the real host gives it: there in an answer's first turn only (account-phone.ts)
const store = (): never => fs as never;
function boot(files: Record<string, string> = {}, seed: Record<string, string> = {}): void {
  events.length = 0; calls = []; fetches = []; fs = fakeStore(seed); secrets = fs.kept;
  m = memFs(files); account.resetBoundary(); api.resetSeam(); signin.resetSignIn();
  api.bindNative({ available: true, later: async (r: { op: string }) => { calls.push(r as never); events.push(`native:${r.op}`); return r.op === 'forgetAccountFiles' ? { done: true } : r.op === 'hello' ? { state: 'native' } : { ok: true }; } } as never);
  respond = () => new Response('{}', { status: 200 });
  globalThis.fetch = (async (u: string, init: { method?: string; headers?: Record<string, string> }) => { fetches.push({ url: String(u), method: init?.method ?? 'GET', auth: init?.headers?.Authorization ?? '' }); const r = respond(String(u), init?.method ?? 'GET'); return (init?.method ?? 'GET') === 'GET' ? r : fs.post(r); }) as never;
}
afterEach(() => { account.resetBoundary(); api.resetSeam(); signin.resetSignIn(); delete (globalThis as { exactHost?: unknown }).exactHost; });
const has = (p: string): boolean => [...m.files.keys()].some((k) => k === p || k.startsWith(p + '/'));
const enter = (): Promise<{ readable: boolean; state: string; world: api.World }> => fs.turn(() => account.enter(m.storage as never, store()));
const launched = async (): Promise<{ readable: boolean; state: string; world: api.World }> => { let r = await enter(); for (let i = 0; i < 6 && r.state === 'locked'; i++) r = await enter(); return r; };
const SEED = { 'api-key': KEY_A, 'key-id': 'kid-1' };

describe('K the key is Exact\'s', () => {
  test('K1 a key in the store opens that account; native is handed the key in memory and asked nothing else', async () => {
    boot({}, SEED);
    const b = await enter();
    expect(b.state).toBe('open'); expect(b.world.digest).toBe(dig(KEY_A));
    const hold = calls.find((c) => c.op === 'holdKey'); expect(hold?.key).toBe(KEY_A);
    expect(calls.map((c) => c.op).filter((o) => ['verdict', 'session', 'get', 'signIn', 'keystate'].includes(o))).toEqual([]);
  });
  test('K2 no key and no marker: anonymous, native is told the launch is clean and is handed no key', async () => {
    boot({ [`${ROOT(KEY_A)}/takes.json`]: 'orphan' });
    const b = await enter();
    expect(b.state).toBe('anonymous'); expect(has(ROOT(KEY_A))).toBe(false); expect(calls.find((c) => c.op === 'holdKey')).toBeUndefined(); expect(calls.some((c) => c.op === 'ready')).toBe(true);
  });
  test('K3 the marker says signed in and the secret does not read: starting, nothing swept; when it reads the account opens; after the bound it is anonymous', async () => {
    boot({ [SIGNED]: '1', [`${ROOT(KEY_A)}/takes.json`]: 'mine' }, SEED); fs.hide(true);
    const a = await enter();
    expect(a.state).toBe('locked'); expect(has(`${ROOT(KEY_A)}/takes.json`)).toBe(true); expect(api.currentWorld().kind).toBe('none');
    fs.hide(false);
    const b = await enter(); expect(b.state).toBe('open'); expect(b.world.digest).toBe(dig(KEY_A));
    boot({ [SIGNED]: '1' }, SEED); fs.hide(true);
    for (let i = 0; i < 20; i++) expect((await enter()).state).toBe('locked');                    // however many answers ask: only the card's re-reads bound the wait
    expect((await fs.turn(() => account.peek(m.storage as never, store(), 5))).state).toBe('anonymous');
    fs.hide(false); const c = await fs.turn(() => account.peek(m.storage as never, store())); expect(c.state).toBe('open');      // a late retry that brings the key back moves the anonymous world to the account once
  });
  test('K4 an unreadable marker is starting too', async () => {
    boot({}, SEED); (m.storage.fs as { readdir: unknown }).readdir = async () => { const e = new Error('EACCES') as Error & { code: string }; e.code = 'EACCES'; throw e; }; fs.hide(true);
    expect((await enter()).state).toBe('locked');
  });
});

describe('K5 sign-in in shared code', () => {
  test('begin, poll pending, then approved: the key and key id are stored, the marker written, native handed the key, and the world moves once', async () => {
    boot({});
    expect((await enter()).state).toBe('anonymous');
    respond = (u) => u.endsWith('/auth/device/begin') ? new Response(JSON.stringify({ device_code: 'dev1', user_code: 'ABCD-EFGH', verification_uri_complete: 'https://choir.news/device?c=ABCD-EFGH', interval: 5, expires_in: 600 }), { status: 200 }) : new Response('{}', { status: 400 });
    const v = await fs.turn(() => signin.signInStart(m.storage as never, store()));
    expect(v.status).toBe('pending'); expect(v.userCode).toBe('ABCD-EFGH'); expect(v.verificationURL).toBe('https://choir.news/device?c=ABCD-EFGH');
    respond = () => new Response(JSON.stringify({ error: 'authorization_pending' }), { status: 400 });
    expect((await fs.turn(() => signin.signInPoll(m.storage as never, store(), true))).status).toBe('pending'); expect(secrets.size).toBe(0);
    respond = () => new Response(JSON.stringify({ api_key: KEY_A, key_id: 'kid-9' }), { status: 200 });
    const d = await fs.turn(() => signin.signInPoll(m.storage as never, store(), true));
    expect(d.message).toBe('signed in'); expect(d.status).toBe('idle'); expect(d.signedIn).toBe(true); expect(secrets.get('api-key')).toBe(KEY_A); expect(secrets.get('key-id')).toBe('kid-9'); expect(has(SIGNED)).toBe(true);
    expect(calls.find((c) => c.op === 'holdKey')?.key).toBe(KEY_A);
    const b = await enter(); expect(b.state).toBe('open'); expect(b.world.digest).toBe(dig(KEY_A));
  });
  test('K8 denied and expired end the flow with a sentence and store nothing; a sign-out or a cancel stops a poll that is in flight from storing', async () => {
    boot({}); await enter();
    respond = (u) => u.endsWith('/begin') ? new Response(JSON.stringify({ device_code: 'd', user_code: 'U', verification_uri: 'https://choir.news/device', interval: 5, expires_in: 600 }), { status: 200 }) : new Response(JSON.stringify({ error: 'access_denied' }), { status: 400 });
    await fs.turn(() => signin.signInStart(m.storage as never, store()));
    const d = await fs.turn(() => signin.signInPoll(m.storage as never, store(), true)); expect(d.status).toBe('failed'); expect(d.message).toContain('denied'); expect(secrets.size).toBe(0);
    respond = (u) => u.endsWith('/begin') ? new Response(JSON.stringify({ device_code: 'd2', user_code: 'U', verification_uri: 'https://choir.news/device', interval: 5, expires_in: 600 }), { status: 200 }) : new Response(JSON.stringify({ api_key: KEY_A, key_id: 'k' }), { status: 200 });
    await fs.turn(() => signin.signInStart(m.storage as never, store())); signin.signInCancel();
    const e = await fs.turn(() => signin.signInPoll(m.storage as never, store(), true)); expect(e.status).toBe('idle'); expect(secrets.size).toBe(0);
    await fs.turn(() => signin.signInStart(m.storage as never, store())); await fs.turn(() => account.accountEnding(m.storage as never, store()));
    const f = await fs.turn(() => signin.signInPoll(m.storage as never, store(), true)); expect(secrets.size).toBe(0); expect(f.signedIn).toBe(false);
  });
});

describe('K6 and K7 sign-out and the launch that finishes it', () => {
  test('K6 native drops the key BEFORE the ending mark is written; the key is revoked then forgotten; the session ends', async () => {
    boot({ [SIGNED]: '1', [`${ROOT(KEY_A)}/takes.json`]: 'mine' }, SEED); await enter();
    const order: string[] = []; const w = m.storage.fs.atomicWriteFile; (m.storage.fs as { atomicWriteFile: unknown }).atomicWriteFile = async (p: string, b: Uint8Array) => { order.push(`write:${p}`); return w(p, b); };
    const nl = (api.nativeLater() as unknown) as (r: { op: string }) => Promise<unknown>; api.bindNative({ available: true, later: async (r: { op: string; key?: string }) => { order.push(`native:${r.op}:${r.key ?? ''}`); return nl(r); } } as never);
    const e = await fs.turn(() => account.accountEnding(m.storage as never, store())); expect(e.done).toBe(true);
    const r = await fs.turn(() => account.accountSignOut(m.storage as never, store())); expect(r.done).toBe(true);
    await fs.turn(() => account.peek(m.storage as never, store()));      // the next answer that has the store forgets the secrets (the store is gone after the revoke's awaits)
    expect(order.indexOf('native:holdKey:')).toBeGreaterThanOrEqual(0); expect(order.indexOf('native:holdKey:')).toBeLessThan(order.indexOf(`write:${account.ENDING}`));
    expect(fetches.find((f) => f.method === 'DELETE')?.url).toContain('/auth/api-keys/kid-1'); expect(fetches.find((f) => f.method === 'DELETE')?.auth).toBe(`Bearer ${KEY_A}`);
    expect(secrets.size).toBe(0); expect(api.sessionEnded()).toBe(true); expect(has(account.ENDING)).toBe(true); expect(has(`${ROOT(KEY_A)}/takes.json`)).toBe(true);      // nothing is wiped in this process
  });
  test('K7 a launch that finds the mark with the key still there revokes, forgets, sweeps everything, has native forget its files and removes the marks (the ending mark last)', async () => {
    boot({ [account.ENDING]: 'ending', [SIGNED]: '1', [`${ROOT(KEY_A)}/takes.json`]: 'mine', 'app:/data/accounts/anonymous/p.json': '{}' }, SEED);
    const b = await launched();
    expect(b.state).toBe('anonymous'); expect(secrets.size).toBe(0); expect(has(ROOT(KEY_A))).toBe(false); expect(has('app:/data/accounts/anonymous')).toBe(true);
    expect(has(SIGNED)).toBe(false); expect(has(account.ENDING)).toBe(false); expect(calls.some((c) => c.op === 'forgetAccountFiles')).toBe(true);
    expect(events.indexOf(`rm:${SIGNED}`)).toBeLessThan(events.indexOf(`rm:${account.ENDING}`)); expect(fetches.some((f) => f.method === 'DELETE')).toBe(true);
  });
  test('K7b a root that cannot be removed, or native that cannot forget, is blocked (native is never told `ready`) and the mark stays', async () => {
    boot({ [account.ENDING]: 'ending', [`${ROOT(KEY_A)}/takes.json`]: 'mine' }, SEED); m.faults.rm.add(ROOT(KEY_A));
    const a = await launched(); expect(a.state).toBe('ended'); expect(has(account.ENDING)).toBe(true); expect(calls.some((c) => c.op === 'ready')).toBe(false);
    boot({ [account.ENDING]: 'ending' }, SEED);
    api.bindNative({ available: true, later: async (r: { op: string }) => (r.op === 'forgetAccountFiles' ? { done: false } : { ok: true }) } as never);
    const b = await launched(); expect(b.state).toBe('ended'); expect(has(account.ENDING)).toBe(true);
  });
  test('K10 the host\'s own rules, pinned: the store is gone after an await (a forget after storage I/O throws), forgetting an ABSENT name is not an error, and a write after a request with effects stands', async () => {
    boot({}, { 'api-key': KEY_A });
    expect(() => fs.forget('api-key')).toThrow(/no store here/); expect(() => fs.get('api-key')).toThrow(/no store here/);      // outside an answer
    await fs.turn(async () => { fs.forget('key-id'); fs.forget('computer-id'); await m.storage.fs.readdir('app:/data'); expect(() => fs.forget('api-key')).toThrow(/no store here/); });      // an absent name, then the same answer after storage I/O
    expect(secrets.has('api-key')).toBe(true);
    await fs.turn(async () => { await fs.post('x'); fs.set('api-key', 'new'); });      // the continuation of a POST keeps it
    expect(secrets.get('api-key')).toBe('new');
  });
});

describe('K9 an attempt whose answer is gone never finishes: the card starts another', () => {
  test('the first launch attempt waits on a storage call that never lands (the host dropped its answer); gated answers wait on the shared result; the next card ask restarts the attempt with its own storage and every waiter is released', async () => {
    boot({}, SEED);
    const dead = memFs({}).storage as never as { fs: { readdir: unknown } }; (dead.fs as { readdir: unknown }).readdir = () => new Promise(() => undefined);
    const first = fs.turn(() => account.peek(dead as never, store()));             // the card's first answer: its storage never replies
    const gate = fs.turn(() => account.enter(m.storage as never, store()));          // a gated answer waits for the shared result
    await new Promise((r) => setTimeout(r, 20));
    expect(await Promise.race([gate.then(() => 'released'), new Promise((r) => setTimeout(() => r('waiting'), 20))])).toBe('waiting');
    const again = await fs.turn(() => account.peek(m.storage as never, store()));      // the card is asked again (its tick moved): a fresh attempt with live storage
    expect(again.state).toBe('open'); expect((await gate).state).toBe('open'); expect(api.currentWorld().kind).toBe('account');
    void first;
  });
});
