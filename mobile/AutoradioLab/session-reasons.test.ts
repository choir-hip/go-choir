// One table: every SignedOutError reason gives the right words and state at every site that shows or maps one (a new site that ignores the reason is caught here by adding it to SITES).
// Rule: `locked` and `unknown` (a key that could not be READ) never say sign in or sign out, and never become a 401 or the signed-out state; `none` and `refused` keep their words.
import { describe, expect, test } from 'bun:test';
import { SignedOutError, unreadableWords, signedOutStatus, resetSeam, bindNative, type SignedOutReason } from './api.ts';
import { WORDS } from './words.ts';
import { plainAccountError } from './settings.ts';
import { plain as computerPlain } from './computer.ts';
import { plain as activityPlain } from './activity.ts';
import { sentence as saveSentence, isOffline } from './texture-edit.ts';
import { signInNotice } from './take-session.ts';
import { programmeFailKind } from './radio-bridge.ts';
import { plainWords } from './programme-model.ts';
import { sendOne } from './steer.ts';
import { keyAnswer } from './account-delete.ts';
import { syncContent, forgetContent } from './content-run.ts';
import { noFetchers } from './content.ts';
import { Database } from 'bun:sqlite';

const REASONS: SignedOutReason[] = ['none', 'locked', 'refused', 'unknown'];
const err = (reason: SignedOutReason): SignedOutError => new SignedOutError('x', reason);
const SIGN = /sign(ed)? (in|out)|not signed|no longer accepted/i;      // advice to sign in or out ("your sign-in could not be checked" is not advice)
const sync = async (reason: SignedOutReason): Promise<{ state: string; line: string }> => {
  resetSeam(); bindNative(undefined); await forgetContent();
  const f = { ...noFetchers(), stories: async () => { throw err(reason); }, documentPage: async () => { throw err(reason); } };
  const raw = new Database(':memory:');
  const db = { execute: async (sql: string, p: unknown[] = []) => { raw.run(sql, p as never); return {}; }, query: async (sql: string, p: unknown[] = []) => ({ rows: raw.query(sql).values(...(p as never[])) as unknown[][] }), close: async () => undefined };
  const storage = { sqlite: { open: async () => db } } as never;
  return syncContent(`k-${reason}`, storage, { fetchers: f as never, now: async () => 1, busy: async () => false });
};

const WORDS_SITES: [string, (e: unknown) => string][] = [
  ['settings plainAccountError', plainAccountError], ['computer plain', computerPlain], ['activity plain', activityPlain], ['texture-edit sentence', saveSentence],
  ['take publish', (e) => signInNotice(e as SignedOutError, 'publish a take')], ['take delete', (e) => signInNotice(e as SignedOutError, 'delete a take')],
];

describe('every word site, every reason', () => {
  for (const [site, f] of WORDS_SITES) {
    test(`${site}: locked says unlock, unknown says could not check, neither says sign in or out; none and refused keep a sign-in line`, () => {
      expect(f(err('locked'))).toBe(WORDS.locked);
      expect(f(err('unknown'))).toBe(WORDS.unknown);
      expect(SIGN.test(f(err('locked')) + f(err('unknown')))).toBe(false);
      expect(SIGN.test(f(err('none')))).toBe(true);
      expect(SIGN.test(f(err('refused')))).toBe(true);
    });
  }
  test('the shared function agrees', () => {
    for (const r of REASONS) expect(unreadableWords(err(r))).toBe(r === 'locked' ? WORDS.locked : r === 'unknown' ? WORDS.unknown : null);
    expect(unreadableWords(new Error('x'))).toBeNull();
  });
});

describe('every state site, every reason', () => {
  test('request status: 401 only for none and refused; 503 for a key that could not be read', () => {
    for (const r of REASONS) expect(signedOutStatus(err(r))).toBe(r === 'locked' || r === 'unknown' ? 503 : 401);
    expect(signedOutStatus(new Error('x'))).toBe(401);
  });
  test('radio programme post: signedOut only for none and refused; transport (try again) otherwise', () => {
    for (const r of REASONS) expect(programmeFailKind(err(r))).toBe(r === 'locked' || r === 'unknown' ? 'transport' : 'signedOut');
    expect(plainWords({ kind: programmeFailKind(err('unknown')) })).not.toMatch(SIGN);
  });
  test('a cloned error (same name and reason, not an instance) is classed the same way', () => {
    const clone = (reason: string) => Object.assign(new Error('x'), { name: 'SignedOutError', reason });
    expect(programmeFailKind(clone('unknown'))).toBe('transport'); expect(programmeFailKind(clone('none'))).toBe('signedOut');
    expect(signedOutStatus(clone('locked'))).toBe(503); expect(unreadableWords(clone('locked'))).toBe(WORDS.locked);
  });
  test('steer: every reason keeps the tap queued (offline), says nothing', async () => {
    for (const r of REASONS) expect(await sendOne({ route: '/p/a' } as never, async () => { throw err(r); })).toBe('offline');
  });
  test('delete-account key answer: unreadable words, never 401 advice', () => {
    expect(keyAnswer({ state: 'unreadable', noStore: true })).toBe('unknown');
  });
  test('texture-edit: an unreadable key is offline (kept, tried again); a refused one is a failure', () => {
    for (const r of REASONS) expect(isOffline(err(r))).toBe(r === 'locked' || r === 'unknown');
  });
  test('content sync: an unreadable key is offline with the unreadable words; none and refused are signed out', async () => {
    for (const r of REASONS) {
      const out = await sync(r);
      if (r === 'locked' || r === 'unknown') { expect(out.state).toBe('offline'); expect(out.line).toBe(r === 'locked' ? WORDS.locked : WORDS.unknown); }
      else expect(out.state).toBe('signedout');
    }
  });
});
