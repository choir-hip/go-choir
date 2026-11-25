import { beforeEach, describe, expect, test } from 'bun:test';
import { BEGIN_PATH, FINISH_PATH, SCHEDULED, deleteReset, deleteRun, deleteView, failureWords, plainDate, resetDelete, type DeleteEnv, type Reply, type SignOutAnswer } from './account-delete.ts';

// The failure list is the header of account-delete.ts (D1 to D8).

const KEY = 'choir_sk_THE_SECRET_KEY';
const OPTIONS = { publicKey: { challenge: 'CHALLENGE', rpId: 'choir.news', allowCredentials: [{ id: 'cred', type: 'public-key' }] } };
const ASSERTION = { id: 'cred', rawId: 'cred', type: 'public-key', response: { clientDataJSON: 'cd', authenticatorData: 'ad', signature: 'sig' } };
const ACCEPT = { ok: true, job_id: 'del-1', requested_at: '2026-11-23T20:00:00Z', purge_after: '2026-11-26T20:00:00Z' };

type Script = {
  begin?: Reply; finish?: Reply; passkey?: () => Promise<unknown>; hasKey?: boolean; signOut?: () => Promise<SignOutAnswer>;
};
function env(s: Script = {}) {
  const posts: { path: string; body: unknown }[] = [];
  const logs: string[] = [];
  const order: string[] = [];
  let screens = 0, signOuts = 0;
  const e: DeleteEnv = {
    passkeys: {
      create: async () => { throw new Error('not used'); },
      get: async (o) => { screens++; order.push('screen'); expect(o).toEqual({ publicKey: OPTIONS.publicKey }); return s.passkey ? s.passkey() : ASSERTION; },
    },
    post: async (path, body) => {
      posts.push({ path, body }); order.push(path);
      if (path === BEGIN_PATH) return s.begin ?? { status: 200, body: OPTIONS };
      return s.finish ?? { status: 200, body: ACCEPT };
    },
    hasKey: () => s.hasKey ?? true,
    signOut: async () => { signOuts++; order.push('signout'); return s.signOut ? s.signOut() : { done: true, failed: '', message: '' }; },
    log: (l) => logs.push(l),
  };
  return { e, posts, logs, order, counts: () => ({ screens, signOuts }) };
}
const text = (v: unknown) => JSON.stringify(v);

beforeEach(resetDelete);

describe('the whole run', () => {
  test('begin, passkey screen, finish with that assertion, then the sign-out; the words say scheduled with the date', async () => {
    const t = env();
    const v = await deleteRun(t.e);
    expect(t.order).toEqual([BEGIN_PATH, 'screen', FINISH_PATH, 'signout']);
    expect(t.posts[1].body).toEqual(ASSERTION);                                                    // D2: exactly what the screen answered
    expect(v.phase).toBe('done');
    expect(v.signedOut).toBe(true);
    expect(v.purgeOn).toBe('26 November 2026');
    expect(v.message).toBe(SCHEDULED('26 November 2026'));
    expect(v.message).toContain('scheduled');
    expect(v.message).toContain('26 November 2026');
    expect(v.message).toContain('Cancel deletion');
    expect(v.message.toLowerCase()).not.toContain('is deleted');                                    // D8
  });
});

describe('D1 and D2: nothing local changes until the server accepts', () => {
  test('a cancelled passkey screen signs nothing out and sends no finish', async () => {
    const t = env({ passkey: async () => { throw Object.assign(new Error('x'), { name: 'NotAllowedError' }); } });
    const v = await deleteRun(t.e);
    expect(v.phase).toBe('cancelled');
    expect(v.message).toBe('Nothing was deleted.');
    expect(t.counts().signOuts).toBe(0);
    expect(t.posts.map((p) => p.path)).toEqual([BEGIN_PATH]);
  });
  test('a refused or failed begin never opens the passkey screen', async () => {
    for (const status of [0, 401, 403, 409, 429, 500, 503]) {
      resetDelete();
      const t = env({ begin: { status, body: null } });
      const v = await deleteRun(t.e);
      expect(v.phase).toBe('failed');
      expect(t.counts()).toEqual({ screens: 0, signOuts: 0 });
      expect(v.message).toContain('Nothing was deleted');
    }
  });
  test('a begin with no options is a failure, and the screen stays closed', async () => {
    const t = env({ begin: { status: 200, body: { publicKey: null } } });
    expect((await deleteRun(t.e)).phase).toBe('failed');
    expect(t.counts().screens).toBe(0);
  });
  test('a refused finish signs nothing out', async () => {
    for (const status of [0, 400, 403, 429, 500, 503]) {
      resetDelete();
      const t = env({ finish: { status, body: { error: 'no' } } });
      const v = await deleteRun(t.e);
      expect(v.phase).toBe('failed');
      expect(t.counts().signOuts).toBe(0);
      expect(v.message).toContain('Nothing was deleted');
      expect(v.signedOut).toBe(false);
    }
  });
  test('a passkey screen that fails otherwise, or is unsupported, says so and changes nothing', async () => {
    for (const [name, want] of [['NotSupportedError', 'cannot use a passkey'], ['SecurityError', 'could not be checked'], ['Weird', 'could not be checked']] as const) {
      resetDelete();
      const t = env({ passkey: async () => { throw Object.assign(new Error('x'), { name }); } });
      const v = await deleteRun(t.e);
      expect(v.phase).toBe('failed');
      expect(v.message).toContain(want);
      expect(t.counts().signOuts).toBe(0);
    }
  });
});

describe('D3: one run at a time', () => {
  test('a second press while working starts nothing', async () => {
    let release: (v: unknown) => void = () => undefined;
    const t = env({ passkey: () => new Promise((r) => { release = r; }) });
    const first = deleteRun(t.e);
    await new Promise((r) => setTimeout(r, 5));
    expect(deleteView().pending).toBe(true);
    const second = await deleteRun(t.e);
    expect(second.pending).toBe(true);
    expect(t.posts.filter((p) => p.path === BEGIN_PATH).length).toBe(1);
    release(ASSERTION);
    expect((await first).phase).toBe('done');
    expect(t.counts().signOuts).toBe(1);
  });
});

describe('D4: no secret anywhere', () => {
  test('the view, the messages and the log never hold the key, the options or the assertion', async () => {
    for (const s of [{}, { finish: { status: 500, body: { error: KEY } } }, { passkey: async () => { throw Object.assign(new Error(KEY), { name: 'Weird' }); } }] as Script[]) {
      resetDelete();
      const t = env(s);
      const v = await deleteRun(t.e);
      const all = text(v) + t.logs.join('\n');
      for (const bad of [KEY, 'CHALLENGE', '"signature"', 'authenticatorData', 'choir_sk_']) expect(all).not.toContain(bad);
    }
  });
});

describe('D5: the server said yes, so it is scheduled whatever the sign-out does', () => {
  test('a sign-out that reports a partial wipe, fails, or throws still ends done, with the truth about this phone', async () => {
    const t1 = env({ signOut: async () => ({ done: true, failed: 'mail', message: 'Signed out, but these could not be cleared yet: mail.' }) });
    const v1 = await deleteRun(t1.e);
    expect(v1.phase).toBe('done');
    expect(v1.signedOut).toBe(true);
    expect(v1.message).toContain('could not be cleared yet: mail');
    resetDelete();
    const t2 = env({ signOut: async () => ({ done: false, failed: '', message: 'Sign-out did not finish.' }) });
    const v2 = await deleteRun(t2.e);
    expect(v2.phase).toBe('done');
    expect(v2.signedOut).toBe(false);
    expect(v2.message).toContain('scheduled for deletion');
    expect(v2.message).toContain('could not finish signing out');
    resetDelete();
    const t3 = env({ signOut: async () => { throw new Error('boom'); } });
    const v3 = await deleteRun(t3.e);
    expect(v3.phase).toBe('done');
    expect(v3.message).toContain('scheduled for deletion');
  });
  test('the sign-out runs after the finish and never before', async () => {
    const t = env();
    await deleteRun(t.e);
    expect(t.order.indexOf('signout')).toBeGreaterThan(t.order.indexOf(FINISH_PATH));
  });
});

describe('D6: no key, no run', () => {
  test('says to sign in and sends nothing', async () => {
    const t = env({ hasKey: false });
    const v = await deleteRun(t.e);
    expect(v.phase).toBe('failed');
    expect(v.message).toContain('Sign in first');
    expect(t.posts.length).toBe(0);
    expect(t.counts()).toEqual({ screens: 0, signOuts: 0 });
  });
});

describe('D7: each failure says what is true', () => {
  test('a refusal, an outage and offline are three different sentences', () => {
    const w = [failureWords(403, 'begin'), failureWords(503, 'begin'), failureWords(0, 'begin')];
    expect(new Set(w).size).toBe(3);
    expect(w[0]).toContain('cannot be deleted from the app');
    expect(w[1]).toContain('Try again in a minute');
    expect(w[2]).toContain('No connection');
  });
});

describe('D8 and the page reopening', () => {
  test('plainDate reads RFC 3339 in UTC and refuses junk', () => {
    expect(plainDate('2026-11-26T20:00:00Z')).toBe('26 November 2026');
    expect(plainDate('2026-12-01T23:59:59-05:00')).toBe('2 December 2026');
    expect(plainDate('')).toBe('');
    expect(plainDate('soon')).toBe('');
    expect(plainDate(null)).toBe('');
  });
  test('a missing date still says a time, never an empty gap', () => {
    expect(SCHEDULED('')).toContain('after 72 hours');
    expect(SCHEDULED('')).not.toContain('after ;');
  });
  test('reopening the page clears an old failure but not a finished deletion', async () => {
    const t = env({ begin: { status: 503, body: null } });
    await deleteRun(t.e);
    expect(deleteView().phase).toBe('failed');
    expect(deleteReset().phase).toBe('idle');
    expect(deleteReset().message).toBe('');
    resetDelete();
    await deleteRun(env().e);
    expect(deleteReset().phase).toBe('done');
  });
});

import { REVIEWER_DELETE_WORDS } from './account-delete.ts';
describe('the App Review account', () => {
  test('its page says it cannot be deleted here and how to try deletion with a new passkey account', () => {
    expect(REVIEWER_DELETE_WORDS).toContain('kept for App Review and cannot be deleted from the app');
    expect(REVIEWER_DELETE_WORDS).toContain('create a new account with a passkey');
    expect(REVIEWER_DELETE_WORDS).toContain('8-digit code');
    expect(REVIEWER_DELETE_WORDS).toContain('delete that account here');
  });
});

import { readFileSync } from 'node:fs';
describe('the App Review account page', () => {
  test('the Delete page shows the reviewer words verbatim, and its confirmation only for anyone else', () => {
    const page = readFileSync(new URL('./settings.contract', import.meta.url), 'utf8');
    expect(page).toContain(`text "${REVIEWER_DELETE_WORDS}" testId="settings-delete-reviewer"`);
    expect(page).toContain('if acctInfo.reviewer');
    expect(page).toContain('if delv.phase != "done" and not acctInfo.reviewer');
  });
});
