// The class guard. Every non-test .ts file here that catches or tests a SignedOutError, reads the key state (`keyState(`, a `.state === 'present'` or `'absent'` test, `hasKey`) or reads a
// SignedOutError's `.reason` must be in ALLOWED below, with one line saying how it treats a key that could not be read (`locked` / `unknown`). A new file that touches sign-in state fails until
// someone reads it and decides. The behaviour of the pure ones is in session-reasons.test.ts.
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';

const ALLOWED: Record<string, string> = {
  'account-delete.ts': 'the run takes the key as present / absent / locked / unknown (keyAnswer) and words the unreadable two with the shared words; nothing is posted',
  'account-state.ts': 'choirAuthState: signedIn is false for a key that could not be read, status is `locked` (the store failed) or `unknown` (no store here, or a read that threw), `idle` otherwise; accountState gives the locked and unknown views with NO action, and the contracts open the sign-in sheet only under `offerSignIn` (acs.action), pinned by contract-signin-pin.test.ts; nothing reads au.status',
  'account.ts': 'reconcile, sign-out and the pending revoke act only on present or absent; unreadable never wipes, adopts or reports a sign-out done (decide table, session-unreadable.test.ts U4 to U6)',
  'activity.ts': 'plain() asks the shared unreadableWords first',
  'api.ts': 'defines SignedOutError and the shared helpers (isUnreadable, unreadableWords, signedOutStatus); keptSession throws unknown or locked, never none, for a failed read',
  'app.ts': 'the delete post maps through signedOutStatus (503 for unreadable) and hasKey through keyAnswer',
  'computer.ts': 'plain() asks unreadableWords first; the phase-failed branch uses plain()',
  'consent.ts': 'deskMode only chooses a mode and says no words: present is the desk, a read that threw is the desk, absent or an unreadable key (no error) is listening only (the paper plays, nothing goes to a model)',
  'content-run.ts': 'a SignedOutError that is unreadable makes the sync offline with the unreadable words (asked again); only none and refused are signedout',
  'content.ts': 'a comment and a re-export of SignedOutError only; no catch',
  'files.ts': 'failure words read e.reason: locked and unknown get WORDS.locked and WORDS.unknown',
  'mail-model.ts': 'the SignedOutError branch reads the reason: locked and unknown get WORDS.locked and WORDS.unknown',
  'mail.ts': 'only THROWS a SignedOutError (a sign-out while reading); it catches none',
  'notebook.ts': '`signedout` is the notebook page\'s own state string (not the account state); it carries e.reason and the page words map locked and unknown to WORDS.locked and WORDS.unknown',
  'onboarding.ts': 'acts only when the key state is absent (a read that failed is not absent), and a throw leaves the first run to decide',
  'podcast.ts': 'a SignedOutError only clears the sync note (no words, no state); the next sync asks again',
  'radio-bridge.ts': 'programmeFailKind: an unreadable key is transport ("try again"), only none and refused are signedOut',
  'refused.ts': 'a comment names SignedOutError; refusedHere reads its noStore flag, so a refused key read from a let-go answer is a quiet stop for every caller of refusedHere and for texture-edit isLetGo',
  'reader.ts': 'failDoc words read e.reason (locked and unknown get the unreadable words); a held page is not served for a SignedOutError, the error page says why',
  'settings.ts': 'plainAccountError asks unreadableWords first; a signed-out account line asks the key state and uses the unreadable words when the key could not be read',
  'steer.ts': 'a SignedOutError (any reason) keeps the tap queued as offline and says nothing',
  'take-session.ts': 'signInNotice asks unreadableWords first',
  'texture-edit.ts': 'sentence asks unreadableWords first; isOffline treats an unreadable key as offline (kept and retried)',
  'textures.ts': 'failure words read e.reason: locked and unknown get WORDS.locked and WORDS.unknown',
  'themes.ts': 'failure words read e.reason: locked and unknown get WORDS.locked and WORDS.unknown',
};

const PATTERNS: [string, RegExp][] = [
  ['SignedOutError', /SignedOutError/],
  ['keyState(', /\bkeyState\(/],
  ["state === 'present'/'absent'", /\.state\s*[!=]==\s*'(present|absent)'/],
  ['hasKey', /\bhasKey\b/],
  ['isSignedOut(', /\bisSignedOut\(/],
  ['.reason of an error', /\b(e|err|error)\.reason\b|\{\s*reason\?:\s*string/],
];

const files = readdirSync(import.meta.dir).filter((f) => f.endsWith('.ts') && !f.endsWith('.test.ts') && !f.endsWith('.d.ts'));
const touching = (src: string): string[] => PATTERNS.filter(([, re]) => re.test(src)).map(([n]) => n);

describe('the error is recognised one way', () => {
  test('no non-test file tests `instanceof SignedOutError`: use isSignedOut (by name)', () => {
    const bad = files.filter((f) => /instanceof\s+SignedOutError/.test(readFileSync(`${import.meta.dir}/${f}`, 'utf8')));
    expect(bad).toEqual([]);
  });
});

describe('files that touch sign-in state are decided', () => {
  test('every such file is in ALLOWED, with a reason', () => {
    const missing: string[] = [];
    for (const f of files) {
      const hit = touching(readFileSync(`${import.meta.dir}/${f}`, 'utf8'));
      if (hit.length > 0 && !ALLOWED[f]) missing.push(`${f} (${hit.join(', ')})`);
    }
    expect(missing).toEqual([]);      // read the file, make it treat locked and unknown as "could not check" (api.ts isUnreadable, unreadableWords), then list it above with one line saying how
  });
  test('no entry is stale: each listed file exists and still touches sign-in state', () => {
    const stale = Object.keys(ALLOWED).filter((f) => !files.includes(f) || touching(readFileSync(`${import.meta.dir}/${f}`, 'utf8')).length === 0);
    expect(stale).toEqual([]);
  });
  test('every entry says how, in a line', () => {
    for (const [f, why] of Object.entries(ALLOWED)) expect([f, why.length > 20]).toEqual([f, true]);
  });
});
