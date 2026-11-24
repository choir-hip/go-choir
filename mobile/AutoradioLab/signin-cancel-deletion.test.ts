// Signing in to an account whose deletion is pending (server: 403 account_deletion_pending with a one-use cancel token) offers "Cancel deletion",
// and the cancel answers with the app's key, which is kept like any sign-in's key.
//
// Failure list (written before the code):
//  C1 a pending account's sign-in is worded as a generic failure, or keeps a key, or shows no offer.
//  C2 the offer shows without a server answer that asked for it, or survives a new attempt or the screen being opened again.
//  C3 the cancel is sent without the token, twice, or with anything but the app's id; the token or key appears in a view, a log or an answer.
//  C4 a 409 (the purge began) or an expired token is worded as a retryable failure, or leaves the offer up.
//  C5 a late cancel answer of a replaced attempt keeps a key.
//  C6 the date the removal would have happened is missing from the words.
import { beforeEach, describe, expect, test } from 'bun:test';
import { resetAttempt, signInCancelDeletion, signInReset, signInRun, signInView, type SignInEnv } from './app-signin.ts';

const ORIGIN = 'https://choir.news';
const KEY = 'choir_sk_test-key-cancel-not-real';
const TOKEN = 'tok-cancel-not-real';
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });
const PENDING = { error: 'account_deletion_pending', cancel_token: TOKEN, purge_after: '2026-11-27T10:00:00Z', requested_at: '2026-11-24T10:00:00Z' };

function harness(answers: { login?: () => Response; cancel?: () => Response | Promise<Response> } = {}) {
  const kept: { key: string; keyId: string }[] = [], revoked: string[] = [], logs: string[] = [], posts: { path: string; body: any }[] = [];
  let n = 0;
  const fetcher = (async (url: string, init: { body: string }) => {
    const path = url.slice(ORIGIN.length), body = JSON.parse(init.body);
    posts.push({ path, body });
    if (path === '/auth/app/login/begin') return json(200, { publicKey: { challenge: 'c' } });
    if (path === '/auth/app/login/finish') return answers.login ? answers.login() : json(403, PENDING);
    if (path === '/auth/account/delete/cancel') return answers.cancel ? answers.cancel() : json(200, { api_key: KEY, key_id: 'ak_9' });
    return json(404, {});
  }) as unknown as typeof fetch;
  const env: SignInEnv = {
    ceremony: { origin: ORIGIN, fetch: fetcher, passkeys: { create: async () => ({}), get: async () => ({ id: 'got' }) } },
    appId: 'news.choir.autoradio.lab',
    keep: (key, keyId) => { kept.push({ key, keyId }); },
    revoke: async (key) => { revoked.push(key); },
    newId: () => `attempt-${++n}`,
    log: (l) => logs.push(l),
  };
  return { env, kept, revoked, logs, posts };
}
beforeEach(() => resetAttempt());

describe('a pending account signs in', () => {
  test('C1 C6: no key, an offer, and the date in the words', async () => {
    const h = harness();
    const r = await signInRun(h.env, 'signin');
    expect(r.phase).toBe('failed');
    expect(h.kept).toEqual([]);
    const v = signInView();
    expect(v.offer).toBe(true);
    expect(v.message).toContain('27 November 2026');
    expect(v.message.toLowerCase()).toContain('deletion');
  });
  test('C2: a plain failure offers nothing; a new attempt or reopening clears the offer', async () => {
    const h = harness({ login: () => json(400, { error: 'x' }) });
    await signInRun(h.env, 'signin');
    expect(signInView().offer).toBe(false);
    const p = harness();
    await signInRun(p.env, 'signin');
    expect(signInView().offer).toBe(true);
    signInReset(p.env);
    expect(signInView().offer).toBe(false);
    await signInRun(p.env, 'signin');
    expect(signInView().offer).toBe(true);
    const ok = harness({ login: () => json(200, { api_key: KEY, key_id: 'ak_1' }) });
    await signInRun(ok.env, 'signin');
    expect(signInView().offer).toBe(false);
  });
  test('a 403 for any other reason is not an offer', async () => {
    const h = harness({ login: () => json(403, { error: 'forbidden', cancel_token: TOKEN }) });
    await signInRun(h.env, 'signin');
    expect(signInView().offer).toBe(false);
  });
});

describe('cancel deletion', () => {
  test('C3: the token and the app id go once; the key is kept; the offer is gone; nothing secret in view or log', async () => {
    const h = harness();
    await signInRun(h.env, 'signin');
    const r = await signInCancelDeletion(h.env);
    expect(r.phase).toBe('done');
    const sent = h.posts.filter((p) => p.path === '/auth/account/delete/cancel');
    expect(sent).toEqual([{ path: '/auth/account/delete/cancel', body: { cancel_token: TOKEN, app: 'news.choir.autoradio.lab' } }]);
    expect(h.kept).toEqual([{ key: KEY, keyId: 'ak_9' }]);
    expect(signInView().offer).toBe(false);
    expect(JSON.stringify(signInView()) + JSON.stringify(r) + h.logs.join('\n')).not.toContain(TOKEN);
    expect(JSON.stringify(signInView()) + JSON.stringify(r) + h.logs.join('\n')).not.toContain(KEY);
    // The token is spent: a second press sends nothing.
    await signInCancelDeletion(h.env);
    expect(h.posts.filter((p) => p.path === '/auth/account/delete/cancel').length).toBe(1);
  });
  test('with no offer a press sends nothing', async () => {
    const h = harness();
    await signInCancelDeletion(h.env);
    expect(h.posts).toEqual([]);
  });
  test('C4: 409 says it cannot be undone and drops the offer; an expired token says to sign in again', async () => {
    const a = harness({ cancel: () => json(409, { error: 'begun' }) });
    await signInRun(a.env, 'signin');
    await signInCancelDeletion(a.env);
    expect(signInView()).toMatchObject({ phase: 'failed', offer: false });
    expect(signInView().message).toContain('cannot be undone');
    expect(a.kept).toEqual([]);
    resetAttempt();
    const b = harness({ cancel: () => json(400, { error: 'expired' }) });
    await signInRun(b.env, 'signin');
    await signInCancelDeletion(b.env);
    expect(signInView()).toMatchObject({ phase: 'failed', offer: false });
    expect(signInView().message).toContain('sign in again');
  });
  test('an outage keeps the offer so the person can press again', async () => {
    const h = harness({ cancel: () => json(503, { error: 'busy' }) });
    await signInRun(h.env, 'signin');
    await signInCancelDeletion(h.env);
    expect(signInView().phase).toBe('failed');
    expect(signInView().offer).toBe(true);
  });
  test('C5: a late answer of a replaced attempt keeps no key and revokes the minted one', async () => {
    let release!: (r: Response) => void;
    const gate = new Promise<Response>((res) => { release = res; });
    const h = harness({ cancel: () => gate });
    await signInRun(h.env, 'signin');
    const pending = signInCancelDeletion(h.env);
    signInReset(h.env);               // a running attempt is left alone: it must still be live
    expect(signInView().pending).toBe(true);
    const { signInCancel } = await import('./app-signin.ts');
    signInCancel(h.env);
    release(json(200, { api_key: KEY, key_id: 'ak_9' }));
    await pending;
    expect(h.kept).toEqual([]);
    expect(h.revoked).toEqual([KEY]);
  });
});
