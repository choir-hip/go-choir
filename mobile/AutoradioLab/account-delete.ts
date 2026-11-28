// Delete the account, from inside the app (Apple 5.1.1(v); owner, 2026-11-23: "takes are removed; published stories are kept with the author detached;
// access is revoked at once; the irreversible purge waits 72 hours and signing in during that time cancels it").
//
// One press is one run of this machine, owned here:
//   1. POST /auth/account/delete/begin (the app's key)    -> the server's assertion options for THIS account, good for five minutes
//   2. the platform's passkey screen (`@exact/passkeys`)   -> a fresh assertion; the key alone starts nothing
//   3. POST /auth/account/delete/finish (the key + the assertion) -> the server revokes every session and key of the account in one step, and says when the purge is
//   4. only after the server said yes: the same sign-out the Account page's "Sign out" runs (account.ts): the native half deletes the key and its own stores
//      (the voice's turn history and kept turn audio, mail, reader holds, the run sheet), then every shared store is wiped (WIPES in account.ts).
// Nothing local is touched before step 3 answers 200: a cancelled passkey screen, a refusal, a lost connection all leave the app exactly as it was.
//
// Failure list (each line has a test in account-delete.test.ts):
//  D1 the passkey screen is cancelled, or begin or finish refuses, and something local is signed out or wiped.
//  D2 finish is sent with anything but the assertion the passkey screen answered; or begin fails and an assertion is asked for anyway.
//  D3 a second press while a run is working starts a second ceremony.
//  D4 the key, the assertion or the server's options appear in a view, a message or a log line.
//  D5 the sign-out fails (or throws) and the person is told the account was not deleted, though the server accepted; or the sign-out runs before the server accepted.
//  D6 a phone with no key starts a run (there is nothing to prove the account with); it says to sign in.
//  D7 a refusal (403 protected account) or an outage (503, offline) is worded as the same thing; each says what is true.
//  D8 the words say "deleted" while the account is only scheduled, or give no date for the purge, or promise a cancel that is not there.
import { WORDS } from './words.ts';
import { CeremonyError, platformError, type Passkeys } from './passkey-ceremony.ts';

export const BEGIN_PATH = '/auth/account/delete/begin';
export const FINISH_PATH = '/auth/account/delete/finish';

export type Phase = 'idle' | 'working' | 'done' | 'failed' | 'cancelled';
export type DeleteView = {
  phase: Phase; pending: boolean;
  /** Plain words of how it ended; empty while idle or working. */
  message: string;
  /** The purge date as the person reads it ("26 November 2026"), once the server accepted. */
  purgeOn: string;
  /** True once this phone has been signed out and wiped for the deletion. */
  signedOut: boolean;
  rev: number;
};

export type Reply = { status: number; body: unknown };
export type SignOutAnswer = { done: boolean; failed: string; message: string };

export type DeleteEnv = {
  /** The platform's passkey screen. */
  passkeys: Passkeys;
  /** A POST with the app's key. `status` 0 is "no answer"; 401 is "the key is not accepted". Never throws. */
  post(path: string, body: unknown): Promise<Reply>;
  /** Is there a key to ask with? */
  hasKey(): boolean | 'locked' | 'unknown' | Promise<boolean | 'locked' | 'unknown'>;
  /** The Account page's sign-out (account.ts): the native half, then every shared store. */
  signOut(): Promise<SignOutAnswer>;
  log(line: string): void;
};

let phase: Phase = 'idle';
let message = '';
let purgeOn = '';
let signedOut = false;
let rev = 0;
let running = false;

export const resetDelete = (): void => { phase = 'idle'; message = ''; purgeOn = ''; signedOut = false; rev = 0; running = false; };
export const deleteView = (): DeleteView => ({ phase, pending: phase === 'working', message, purgeOn, signedOut, rev });
const set = (p: Phase, m = ''): void => { phase = p; message = m; rev += 1; };

const field = (o: unknown, k: string): unknown => (o && typeof o === 'object' ? (o as Record<string, unknown>)[k] : undefined);
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
/** "26 November 2026" from an RFC 3339 time, in UTC; '' when the text is not a time. */
export function plainDate(iso: unknown): string {
  const t = typeof iso === 'string' ? Date.parse(iso) : NaN;
  if (!Number.isFinite(t)) return '';
  const d = new Date(t);
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]} ${d.getUTCFullYear()}`;
}

export const SCHEDULED = (on: string): string =>
  `Your account is scheduled for deletion. Your access ended now. The removal begins after ${on || '72 hours'}; until it begins, ` +
  'nothing is removed, and you can change your mind: sign in with your passkey and choose Cancel deletion.';

/** The Delete page of the App Review account (whoami `reviewer`), in place of the confirmation: the server refuses its deletion (owner, 2026-11-25). */
export const REVIEWER_DELETE_WORDS =
  'This account is kept for App Review and cannot be deleted from the app. To try account deletion, sign out, create a new account with a passkey ' +
  '(Create account, your email, the 8-digit code we send, then the passkey), and delete that account here.';

/** The words for a refusal, an outage or a failure of the first two steps (D7). */
/** The key state as the delete run is told it: present (true), absent (false), or could not be read (`locked`, or `unknown` for a read that threw or was refused for want of a store). */
export function keyAnswer(k: { state: string; error?: boolean; noStore?: boolean }): boolean | 'locked' | 'unknown' {
  return k.state === 'present' ? true : k.state === 'unreadable' ? (k.error || k.noStore ? 'unknown' : 'locked') : false;
}

export function failureWords(status: number, step: 'begin' | 'finish'): string {
  if (status === 0) return 'No connection. Nothing was deleted. Check the network and try again.';
  if (status === 401) return 'This phone is not signed in, so it cannot delete the account. Sign in first. Nothing was deleted.';
  if (status === 403) return 'This account cannot be deleted from the app. Nothing was deleted.';
  if (status === 409) return 'This account has no passkey to confirm with. Nothing was deleted.';
  if (status === 429 || status === 503) return 'Choir could not do this just now. Nothing was deleted. Try again in a minute.';
  if (status === 400 && step === 'finish') return 'The confirmation did not go through, or took too long. Nothing was deleted. Try again.';
  return 'Something went wrong. Nothing was deleted. Try again.';
}

/** One run of the deletion. Returns the view it ended in. */
export async function deleteRun(env: DeleteEnv): Promise<DeleteView> {
  if (running) return deleteView();                                           // D3
  running = true;
  signedOut = false; purgeOn = '';
  set('working');
  try {
    const key = await env.hasKey();
    if (key === 'locked' || key === 'unknown') { set('failed', `${key === 'locked' ? WORDS.locked : WORDS.unknown} Nothing was deleted.`); return deleteView(); }      // could not read the key: not "sign in", and nothing is sent
    if (!key) { set('failed', failureWords(401, 'begin')); return deleteView(); }          // D6
    const begin = await env.post(BEGIN_PATH, undefined);
    if (begin.status !== 200) { set('failed', failureWords(begin.status, 'begin')); env.log(`delete: begin ${begin.status}`); return deleteView(); }
    const publicKey = field(begin.body, 'publicKey');
    if (!publicKey || typeof publicKey !== 'object') { set('failed', failureWords(500, 'begin')); env.log('delete: begin sent no options'); return deleteView(); }
    let assertion: unknown;
    try { assertion = await env.passkeys.get({ publicKey }); }
    catch (e) {
      const k = platformError(e).kind;
      env.log(`delete: passkey screen ended: ${k}`);
      if (k === 'cancelled') set('cancelled', 'Nothing was deleted.');                                  // D1
      else if (k === 'unsupported') set('failed', 'This phone cannot use a passkey here. Nothing was deleted.');
      else set('failed', 'The passkey could not be checked. Nothing was deleted.');
      return deleteView();
    }
    const fin = await env.post(FINISH_PATH, assertion);                                                  // D2: exactly what the screen answered
    if (fin.status !== 200) { set('failed', failureWords(fin.status, 'finish')); env.log(`delete: finish ${fin.status}`); return deleteView(); }
    // The server has accepted: the key is already revoked there. From here nothing is "not deleted" (D5).
    purgeOn = plainDate(field(fin.body, 'purge_after'));
    env.log('delete: the server accepted; signing this phone out');
    let out: SignOutAnswer | null = null;
    try { out = await env.signOut(); } catch { out = null; }
    signedOut = !!out && out.done;
    let words = SCHEDULED(purgeOn);
    if (!out || !out.done) words += ' This phone could not finish signing out; it will finish by itself the next time the app starts. Its data stays until then.';
    else if (out.failed !== '') words += ` ${out.message}`;
    set('done', words);
    return deleteView();
  } catch (e) {
    // post() does not throw; anything that does is a bug, said plainly, and it never carries a key or a body.
    env.log(`delete: stopped: ${e instanceof CeremonyError ? e.kind : 'error'}`);
    if (phase === 'working') set('failed', failureWords(500, 'begin'));
    return deleteView();
  } finally {
    running = false;
  }
}

/** Opening the page again after an end starts clean: an old "failed" or "cancelled" is not shown to the next visit. A run still working is left alone. */
export function deleteReset(): DeleteView {
  if (phase === 'working') return deleteView();
  if (phase !== 'idle' && phase !== 'done') { phase = 'idle'; message = ''; rev += 1; }
  return deleteView();
}
