import { beforeEach, describe, expect, test } from 'bun:test';
import { begin, keepKey, KEY_ID_NAME, KEY_NAME, COMPUTER_ID_NAME, passkeySupport, resetAttempt, signInCancel, signInReset, signInRun, signInView, step, type SignInEnv, type View } from './app-signin.ts';

const ORIGIN = 'https://choir.news';
const KEY_A = 'choir_sk_test-key-a-not-real', KEY_B = 'choir_sk_test-key-b-not-real';
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

type Deferred<T> = { promise: Promise<T>; resolve(v: T): void; reject(e: unknown): void };
const deferred = <T>(): Deferred<T> => { let resolve!: (v: T) => void, reject!: (e: unknown) => void; const promise = new Promise<T>((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; };

function harness(opts: { create?: () => Promise<unknown>; get?: () => Promise<unknown>; finish?: (path: string, body: any) => Promise<Response> | Response; keepThrows?: boolean } = {}) {
  const kept: { key: string; keyId: string }[] = [], revoked: { key: string; keyId: string }[] = [], logs: string[] = [], views: View[] = [], posts: { path: string; body: any }[] = [];
  let n = 0;
  const fetcher = (async (url: string, init: { body: string }) => {
    const path = url.slice(ORIGIN.length), body = JSON.parse(init.body);
    posts.push({ path, body });
    if (path === '/auth/register/begin' || path === '/auth/app/login/begin') return json(200, { publicKey: { challenge: 'c' } });
    return opts.finish ? opts.finish(path, body) : json(200, { api_key: KEY_A, key_id: 'ak_1' });
  }) as unknown as typeof fetch;
  const env: SignInEnv = {
    ceremony: { origin: ORIGIN, fetch: fetcher, passkeys: { create: opts.create ?? (async () => ({ id: 'made' })), get: opts.get ?? (async () => ({ id: 'got' })) } },
    appId: 'news.choir.autoradio.lab',
    keep: (key, keyId) => { if (opts.keepThrows) throw new Error('keychain refused'); kept.push({ key, keyId }); },
    revoke: async (key, keyId) => { revoked.push({ key, keyId }); },
    newId: () => `attempt-${++n}`,
    log: (l) => logs.push(l),
    publish: (v) => views.push(v),
  };
  return { env, kept, revoked, logs, views, posts };
}

beforeEach(() => resetAttempt());

describe('the table', () => {
  test('an event of another attempt, or after an end, changes nothing', () => {
    const a = begin('x', 'signin');
    expect(step(a, { t: 'done', id: 'y' })).toBe(a);
    const done = step(a, { t: 'done', id: 'x' });
    expect(done.phase).toBe('done');
    expect(step(done, { t: 'failed', id: 'x', message: 'm' })).toBe(done);
    expect(step(done, { t: 'cancel', id: 'x' })).toBe(done);
    expect(step(a, { t: 'cancel', id: 'x' }).phase).toBe('cancelled');
    expect(step(a, { t: 'failed', id: 'x', message: 'm' })).toMatchObject({ phase: 'failed', message: 'm' });
  });
});

describe('create account', () => {
  test('the email goes to the server, the credential to the create finish, and the key is kept once', async () => {
    const h = harness();
    const r = await signInRun(h.env, 'create', ' a@b.example ');
    expect(r.phase).toBe('done');
    expect(h.posts.map((p) => p.path)).toEqual(['/auth/register/begin', '/auth/app/register/finish']);
    expect(h.posts[0].body).toEqual({ email: 'a@b.example' });
    expect(h.posts[1].body).toEqual({ app: 'news.choir.autoradio.lab', credential: { id: 'made' } });
    expect(h.kept).toEqual([{ key: KEY_A, keyId: 'ak_1' }]);
    expect(signInView()).toMatchObject({ phase: 'done', pending: false, message: '' });
  });
  test('no email types nothing to the server and opens no screen', async () => {
    let opened = 0;
    const h = harness({ create: async () => { opened++; return {}; } });
    const r = await signInRun(h.env, 'create', '  ');
    expect(r.phase).toBe('failed');
    expect(h.posts).toEqual([]);
    expect(opened).toBe(0);
    expect(signInView().message).toContain('valid email');
  });
  test('sign-up closed says the waiting list and keeps nothing', async () => {
    const h = harness({ finish: () => json(403, { code: 'registration_waitlist', error: 'Choir is full' }) });
    await signInRun(h.env, 'create', 'a@b.example');
    expect(signInView()).toMatchObject({ phase: 'failed' });
    expect(signInView().message).toContain('full for now');
    expect(h.kept).toEqual([]);
    expect(h.posts.filter((p) => p.path === '/auth/app/register/finish').length).toBe(1);     // final: nothing is sent again
    expect(h.revoked).toEqual([]);
  });
  test('sign-up closed at the begin is final too: the passkey screen never opens', async () => {
    let opened = 0;
    const base = harness({ create: async () => { opened++; return {}; } });
    const env = { ...base.env, ceremony: { ...base.env.ceremony, fetch: (async () => json(403, { code: 'registration_waitlist', error: 'Choir is full' })) as unknown as typeof fetch } };
    await signInRun(env, 'create', 'a@b.example');
    expect(opened).toBe(0);
    expect(signInView().message).toContain('full for now');
  });
});

describe('sign in', () => {
  test('no email: the discoverable begin, the sign-in finish, one key', async () => {
    const h = harness();
    const r = await signInRun(h.env, 'signin');
    expect(r.phase).toBe('done');
    expect(h.posts.map((p) => p.path)).toEqual(['/auth/app/login/begin', '/auth/app/login/finish']);
    expect(h.posts[0].body).toEqual({});
    expect(h.kept.length).toBe(1);
  });
  test('cancelled, no passkey and offline each end in plain words and keep nothing', async () => {
    const cancel = harness({ get: async () => { throw Object.assign(new Error('x'), { name: 'NotAllowedError' }); } });
    await signInRun(cancel.env, 'signin');
    expect(signInView().message).toContain('No passkey was used');
    resetAttempt();
    const offline = harness({ finish: () => { throw new TypeError('network'); } });
    await signInRun(offline.env, 'signin');
    expect(signInView().message).toContain('No connection');
    expect(cancel.kept.length + offline.kept.length).toBe(0);
  });
  test('an answer with no key is a failure, not a signed-in app', async () => {
    const h = harness({ finish: () => json(200, { ok: true }) });
    await signInRun(h.env, 'signin');
    expect(signInView().phase).toBe('failed');
    expect(h.kept).toEqual([]);
  });
  test('a store that refuses the key revokes it at once', async () => {
    const h = harness({ keepThrows: true });
    const r = await signInRun(h.env, 'signin');
    expect(r.phase).toBe('failed');
    expect(h.revoked).toEqual([{ key: KEY_A, keyId: 'ak_1' }]);
  });
});

describe('one owner, the newest wins', () => {
  test('a double tap: the first attempt is superseded while its passkey screen is up; its credential is never sent', async () => {
    const first = deferred<unknown>();
    let calls = 0;
    const h = harness({ get: () => (++calls === 1 ? first.promise : Promise.resolve({ id: 'second' })) });
    const p1 = signInRun(h.env, 'signin');
    await Promise.resolve();
    const p2 = signInRun(h.env, 'signin');
    const r2 = await p2;
    expect(r2.phase).toBe('done');
    first.resolve({ id: 'first-late' });
    const r1 = await p1;
    expect(r1.phase).toBe('cancelled');
    const finishes = h.posts.filter((p) => p.path.endsWith('/finish'));
    expect(finishes.map((f) => f.body.credential.id)).toEqual(['second']);
    expect(h.kept.length).toBe(1);
  });
  test('an old attempt whose finish answers late never writes a key, and its key is revoked', async () => {
    const lateFinish = deferred<Response>();
    let n = 0;
    const h = harness({ finish: () => (++n === 1 ? lateFinish.promise : json(200, { api_key: KEY_B, key_id: 'ak_2' })) });
    const p1 = signInRun(h.env, 'signin');
    for (let i = 0; i < 20; i++) await Promise.resolve();
    const r2 = await signInRun(h.env, 'signin');
    expect(r2.phase).toBe('done');
    expect(h.kept).toEqual([{ key: KEY_B, keyId: 'ak_2' }]);
    lateFinish.resolve(json(200, { api_key: KEY_A, key_id: 'ak_1' }));
    expect((await p1).phase).toBe('cancelled');
    expect(h.kept).toEqual([{ key: KEY_B, keyId: 'ak_2' }]);
    expect(h.revoked).toEqual([{ key: KEY_A, keyId: 'ak_1' }]);
  });
  test('Cancel while the finish is out: the late key is revoked and never written', async () => {
    const lateFinish = deferred<Response>();
    const h = harness({ finish: () => lateFinish.promise });
    const p = signInRun(h.env, 'signin');
    for (let i = 0; i < 20; i++) await Promise.resolve();
    expect(signInView().pending).toBe(true);
    expect(signInCancel(h.env).phase).toBe('cancelled');
    lateFinish.resolve(json(200, { api_key: KEY_A, key_id: 'ak_1' }));
    expect((await p).phase).toBe('cancelled');
    expect(h.kept).toEqual([]);
    expect(h.revoked).toEqual([{ key: KEY_A, keyId: 'ak_1' }]);
  });
  test('Cancel with nothing running is a no-op; reset after an end shows a clean screen', async () => {
    const h = harness({ finish: () => json(403, { code: 'registration_waitlist', error: 'x' }) });
    expect(signInCancel(h.env).phase).toBe('idle');
    await signInRun(h.env, 'signin');
    expect(signInView().phase).toBe('failed');
    expect(signInReset(h.env).phase).toBe('idle');
    expect(signInView()).toMatchObject({ phase: 'idle', message: '', pending: false });
  });
  test('reset does not interrupt a running attempt', async () => {
    const gate = deferred<unknown>();
    const h = harness({ get: () => gate.promise });
    const p = signInRun(h.env, 'signin');
    await Promise.resolve();
    expect(signInReset(h.env).phase).toBe('working');
    gate.resolve({ id: 'x' });
    expect((await p).phase).toBe('done');
  });
});

describe('what is published and logged', () => {
  test('every change publishes a view with a rising rev; the last one is the end; no key anywhere', async () => {
    const h = harness();
    await signInRun(h.env, 'create', 'a@b.example');
    expect(h.views.map((v) => v.phase)).toEqual(['working', 'done']);
    expect(h.views[1].rev).toBeGreaterThan(h.views[0].rev);
    const everything = JSON.stringify([h.views, h.logs, signInView()]);
    expect(everything).not.toContain('choir_sk_');
    expect(everything).not.toContain('ak_1');
    expect(everything).not.toContain('a@b.example');
  });
});

describe('keeping the key', () => {
  const fake = (over: { forgetThrows?: string[]; setThrows?: string } = {}) => {
    const items = new Map<string, string>();
    return { items, store: {
      get: (n: string) => items.get(n) ?? null,
      set: (n: string, v: string) => { if (over.setThrows === n) throw new Error('keychain refused'); items.set(n, v); },
      forget: (n: string) => { if (over.forgetThrows?.includes(n)) throw new Error('item not found'); items.delete(n); },
    } };
  };
  test('a forget of the absent computer id that throws does not undo the key', () => {
    const f = fake({ forgetThrows: [COMPUTER_ID_NAME] });
    keepKey(f.store, KEY_A, 'ak_1');
    expect(f.items.get(KEY_NAME)).toBe(KEY_A);
    expect(f.items.get(KEY_ID_NAME)).toBe('ak_1');
  });
  test('the old computer id is forgotten when there is one', () => {
    const f = fake();
    f.items.set(COMPUTER_ID_NAME, 'old');
    keepKey(f.store, KEY_A, 'ak_1');
    expect(f.items.has(COMPUTER_ID_NAME)).toBe(false);
  });
  test('a store that refuses the key id leaves neither item, and throws', () => {
    const f = fake({ setThrows: KEY_ID_NAME });
    expect(() => keepKey(f.store, KEY_A, 'ak_1')).toThrow();
    expect(f.items.has(KEY_NAME)).toBe(false);
    expect(f.items.has(KEY_ID_NAME)).toBe(false);
  });
});

describe('where passkeys can work', () => {
  test('the paid-team apps on Apple can; the free-team lab app and Android cannot, each for its own reason', () => {
    expect(passkeySupport('news.choir.autoradio', 'ios')).toBe('');
    expect(passkeySupport('news.choir.autoradio.lab.one', 'ios')).toBe('');
    expect(passkeySupport('news.choir.autoradio.lab', 'ios')).toBe('app');
    expect(passkeySupport('news.choir.autoradio.lab', 'macos')).toBe('app');
    expect(passkeySupport('news.choir.autoradio', 'android')).toBe('android');
  });
  test('the web build and an undecided host are not gated by the app id', () => {
    expect(passkeySupport('news.choir.autoradio.lab', 'web')).toBe('');
    expect(passkeySupport('news.choir.autoradio.lab', '')).toBe('');
  });
});
