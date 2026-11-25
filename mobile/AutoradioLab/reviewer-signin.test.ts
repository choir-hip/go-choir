import { beforeEach, describe, expect, test } from 'bun:test';
import { resetAttempt, reviewerRun, signInCancel, signInRun, signInView, type SignInEnv, type View } from './app-signin.ts';

// The reviewer sign-in (docs/ship/demo-account.md): a user name and password, the same attempt machine, the same keep.
const ORIGIN = 'https://choir.news';
const KEY = 'choir_sk_test-reviewer-key-not-real';
const PASS = 'test-only-password-not-real-0123456789';
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

function harness(reply: () => Response | Promise<Response>) {
  const kept: { key: string; keyId: string }[] = [], revoked: string[] = [], logs: string[] = [], views: View[] = [], posts: { path: string; body: any }[] = [];
  const fetcher = (async (url: string, init: { body: string }) => { posts.push({ path: url.slice(ORIGIN.length), body: JSON.parse(init.body) }); return reply(); }) as unknown as typeof fetch;
  const env: SignInEnv = {
    ceremony: { origin: ORIGIN, fetch: fetcher, passkeys: { create: async () => ({}), get: async () => ({}) } },
    appId: 'news.choir.autoradio.lab',
    keep: (key, keyId) => { kept.push({ key, keyId }); },
    revoke: async (key) => { revoked.push(key); },
    newId: () => 'attempt-' + (posts.length + 1),
    log: (l) => logs.push(l),
    publish: (v) => views.push(v),
  };
  return { env, kept, revoked, logs, views, posts };
}
beforeEach(() => resetAttempt());

describe('reviewer sign-in', () => {
  test('posts the user name and password once to the demo route and keeps the key the way the passkey flow does', async () => {
    const h = harness(() => json(200, { api_key: KEY, key_id: 'ak_9', reviewer: true }));
    const r = await reviewerRun(h.env, ' reviewer ', PASS);
    expect(r.phase).toBe('done');
    expect(h.posts).toEqual([{ path: '/auth/app/demo/login', body: { app: 'news.choir.autoradio.lab', username: 'reviewer', password: PASS } }]);
    expect(h.kept).toEqual([{ key: KEY, keyId: 'ak_9' }]);
    expect(signInView()).toMatchObject({ phase: 'done', pending: false, mode: 'reviewer', message: '' });
  });
  test('neither the password nor the key is in a log line or a published view', async () => {
    const h = harness(() => json(200, { api_key: KEY, key_id: 'ak_9' }));
    await reviewerRun(h.env, 'jane-doe-name', PASS);
    const all = JSON.stringify([h.logs, h.views, signInView()]);
    expect(all).not.toContain(PASS);
    expect(all).not.toContain('choir_sk_');
    expect(all).not.toContain('jane-doe-name');
  });
  test('empty fields send nothing', async () => {
    const h = harness(() => json(200, {}));
    expect((await reviewerRun(h.env, '', PASS)).phase).toBe('failed');
    expect((await reviewerRun(h.env, 'reviewer', '')).phase).toBe('failed');
    expect(h.posts).toEqual([]);
  });
  test('a wrong password, a busy server, and "not available" each say so in plain words and keep nothing', async () => {
    for (const [status, word] of [[401, 'not right'], [429, 'Try again'], [404, 'not available'], [500, 'Something went wrong']] as const) {
      const h = harness(() => json(status, { error: 'x' }));
      expect((await reviewerRun(h.env, 'reviewer', PASS)).phase).toBe('failed');
      expect(signInView().message).toContain(word);
      expect(h.kept).toEqual([]);
    }
  });
  test('a reply with no key keeps nothing', async () => {
    const h = harness(() => json(200, { api_key: 'not-a-key', key_id: 'ak' }));
    expect((await reviewerRun(h.env, 'reviewer', PASS)).phase).toBe('failed');
    expect(h.kept).toEqual([]);
  });
  test('cancelled while the answer is out: the late key is revoked, never kept', async () => {
    let release!: (r: Response) => void;
    const h = harness(() => new Promise<Response>((r) => { release = r; }));
    const p = reviewerRun(h.env, 'reviewer', PASS);
    for (let i = 0; i < 20; i++) await Promise.resolve();
    signInCancel(h.env);
    release(json(200, { api_key: KEY, key_id: 'ak_9' }));
    expect((await p).phase).toBe('cancelled');
    expect(h.kept).toEqual([]);
    expect(h.revoked).toEqual([KEY]);
  });
  test('a newer attempt wins over a running one', async () => {
    let release!: (r: Response) => void;
    const h = harness(() => new Promise<Response>((r) => { release = r; }));
    const p = reviewerRun(h.env, 'reviewer', PASS);
    for (let i = 0; i < 20; i++) await Promise.resolve();
    const q = signInRun(h.env, 'create', '');       // supersedes (an empty email fails at once)
    expect((await q).phase).toBe('failed');
    release(json(200, { api_key: KEY, key_id: 'ak_9' }));
    expect((await p).phase).toBe('cancelled');
    expect(h.kept).toEqual([]);
  });
});
