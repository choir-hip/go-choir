// Sign-in and account creation with the platform's passkey screen, inside the app (owner, 2026-11-23: "native passkey account creation and sign in"; a new account first proves its email address with a mailed code, docs/problems/passkey-claims-an-unproved-address-2026-11-24.md).
// One attempt is one small state machine with one owner (this file). Every event names its attempt; an event of another attempt, or after an end, changes nothing; a new
// press while one is running makes the newest win; and a late answer of an old attempt never writes a key (a key it minted is revoked).
//
//   create:  POST /auth/register/mail {email, binding} -> (the person types the 8-digit code from the email) -> POST /auth/register/begin {email, attempt_id, code, binding}
//            -> the passkey screen (passkey-ceremony.ts) -> POST /auth/app/register/finish {app, credential} -> an account, its passkey and a key, made together
//   sign in: POST /auth/app/login/begin        -> the passkey screen, no email typed    -> POST /auth/app/login/finish    {app, credential} -> a key
//
// The key: the finish route answers a bearer key with the grant a device approval gives. It is kept as the Exact secrets `choir.key` and `choir.key-id` (grant `secret.keep`,
// the Keychain on Apple, AfterFirstUnlockThisDeviceOnly). It never leaves this file's data layer: not a log line, not an answer, not a contract resource. The answer of a press
// is `{phase, rev}` and the view is `{phase, pending, mode, message, rev}`; no word of either can hold a key. The native voice code reads the same Keychain items (they are
// the one place the key lives) and writes none of them at sign-in.
import passkeyApps from './passkey-apps.json';
import { CeremonyError, ceremonyMessage, createCeremony, getCeremony, postPublic, refusal, sendRegisterCode, type CeremonyEnv, type Proof } from './passkey-ceremony.ts';
import { plainDate } from './account-delete.ts';

/** The server answers a passkey sign-in of an account whose deletion is pending with 403 `account_deletion_pending` and a one-use cancel token (internal/auth/deletion_handlers.go); no key is made. */
export const CANCEL_DELETION_PATH = '/auth/account/delete/cancel';
class DeletionPending extends Error { constructor(readonly token: string, readonly purgeOn: string) { super('account_deletion_pending'); } }
class DeletionPurging extends Error { constructor() { super('account_deletion_purging'); } }
let offer: { token: string; purgeOn: string } | null = null;

export type Mode = 'create' | 'signin' | 'reviewer';
type CeremonyMode = Exclude<Mode, 'reviewer'>;
/** `code`: the mail was sent and the screen waits for the code the person types (a resting phase, not an end). */
export type Phase = 'idle' | 'working' | 'code' | 'done' | 'failed' | 'cancelled';
export type Attempt = { id: string; mode: Mode; phase: Phase; message: string };
export type Event = { t: 'code'; id: string } | { t: 'done'; id: string } | { t: 'failed'; id: string; message: string } | { t: 'cancel'; id: string };

/** The names the app's key is kept under (grants `secret.keep <name>` in app.ts). The computer id is not a secret, kept beside the key so the voice code and shared code read one place. */
export const KEY_NAME = 'choir.key';
export const KEY_ID_NAME = 'choir.key-id';
export const COMPUTER_ID_NAME = 'choir.computer-id';

/** The apps that can make passkeys for choir.news: the ones the server accepts and its association file names (passkey-apps.json; internal/appids must equal it, tested in Go). The first lab app, `news.choir.autoradio.lab`, is signed by a
 * free developer team, which cannot carry Associated Domains, so no passkey can be made in it. */
export const PASSKEY_APP_IDS: string[] = (passkeyApps.apps as { bundle: string }[]).map((a) => a.bundle);
/** '' when this app on this host can sign in with a passkey; 'app' for a build that cannot (its id is not on the list; judged only on Apple hosts, where the id is signed in); 'android' where the server
 * accepts no origin yet. `platform` is `exactPage().platform`; the web build and an undecided host are not gated here. */
export function passkeySupport(appId: string, platform: string): '' | 'app' | 'android' {
  if (platform === 'android') return 'android';
  if ((platform === 'ios' || platform === 'macos') && !PASSKEY_APP_IDS.includes(appId)) return 'app';
  return '';
}

/** Write the key and its id to the secret store; a refusal leaves neither. Forgetting the old computer id is bookkeeping outside that scope: a store that throws on an absent item must not undo the key. */
export function keepKey(store: { set(n: string, v: string): void; forget(n: string): void }, key: string, keyId: string): void {
  try { store.set(KEY_NAME, key); store.set(KEY_ID_NAME, keyId); }
  catch (e) { for (const n of [KEY_NAME, KEY_ID_NAME]) { try { store.forget(n); } catch { /* absent */ } } throw e; }
  try { store.forget(COMPUTER_ID_NAME); } catch { /* absent: nothing to forget */ }
}

export const isTerminal = (p: Phase): boolean => p === 'done' || p === 'failed' || p === 'cancelled';
export const begin = (id: string, mode: Mode): Attempt => ({ id, mode, phase: 'working', message: '' });

/** The whole table. Pure. */
export function step(a: Attempt, e: Event): Attempt {
  if (e.id !== a.id || a.phase !== 'working') return a;
  switch (e.t) {
    case 'code': return { ...a, phase: 'code' };
    case 'done': return { ...a, phase: 'done' };
    case 'failed': return { ...a, phase: 'failed', message: e.message };
    case 'cancel': return { ...a, phase: 'cancelled' };
  }
}

export type View = { phase: Phase; pending: boolean; mode: string; message: string; /** Cancel deletion is on offer (the account was found pending). Never the token. */ offer: boolean; rev: number };
export function view(a: Attempt | null, rev = 0): View {
  if (!a) return { phase: 'idle', pending: false, mode: '', message: '', offer: false, rev };
  return { phase: a.phase, pending: a.phase === 'working', mode: a.mode, message: a.phase === 'failed' ? a.message : '', offer: offer !== null && a.phase !== 'working', rev };
}

export type SignInEnv = {
  ceremony: CeremonyEnv;
  /** The app's id: one of the server's fixed list (internal/appids), never typed by anyone. */
  appId: string;
  /** Write the key and its id to the secret store; throws when the store refuses. Called only for the live attempt, with no await before it. */
  keep(key: string, keyId: string): void;
  /** Revoke a key this attempt minted but may not keep (best effort; the key is unusable by anyone without the text this call is handed). */
  revoke(key: string, keyId: string): Promise<void>;
  newId(): string;
  log(line: string): void;
  /** Told the view after every accepted change; the screen reads `signInView`. */
  publish?(view: View): void;
};

/** The code asked for an address, and the secret this client made when asking: memory only, gone with the process. */
let pendingProof: { email: string; proof: Proof; } | null = null;
let cur: Attempt | null = null;
let rev = 0;
export const resetAttempt = (): void => { cur = null; rev = 0; pendingProof = null; offer = null; };
export const signInView = (): View => view(cur, rev);
const left = (id: string): { phase: Phase; rev: number } => ({ phase: cur && cur.id === id ? cur.phase : 'cancelled', rev });
const live = (id: string): boolean => cur !== null && cur.id === id && cur.phase === 'working';

function adopt(env: SignInEnv, next: Attempt): void {
  const was = cur?.phase;
  cur = next;
  rev += 1;
  if (next.phase !== was) env.log(`signin: attempt ${next.id.slice(0, 6)} ${next.mode} ${next.phase}`);
  env.publish?.(view(cur, rev));
}
function dispatch(env: SignInEnv, e: Event): void {
  if (!cur) return;
  const next = step(cur, e);
  if (next !== cur) adopt(env, next);
}

type Minted = { key: string; keyId: string };
const str = (o: unknown, k: string): string => (o && typeof o === 'object' && typeof (o as Record<string, unknown>)[k] === 'string' ? ((o as Record<string, unknown>)[k] as string) : '');

async function finish(env: SignInEnv, mode: CeremonyMode, credential: unknown): Promise<Minted> {
  const path = mode === 'create' ? '/auth/app/register/finish' : '/auth/app/login/finish';
  const r = await postPublic(env.ceremony, path, { app: env.appId, credential });
  if (r.status === 403 && str(r.body, 'error') === 'account_deletion_pending' && str(r.body, 'cancel_token') !== '') {
    throw new DeletionPending(str(r.body, 'cancel_token'), plainDate(str(r.body, 'purge_after')));
  }
  // The 72 hours are over and the removal has begun: no key, no cancel, and the words say so.
  if (r.status === 403 && str(r.body, 'error') === 'account_deletion_purging') throw new DeletionPurging();
  if (r.status !== 200) throw refusal(r);
  const key = str(r.body, 'api_key'), keyId = str(r.body, 'key_id');
  if (!key.startsWith('choir_sk_') || keyId === '') throw new CeremonyError('failed', 'the server sent no key');
  return { key, keyId };
}

/** Start an attempt and run it to its end. A running attempt is superseded (the newest wins; its late answer is ignored, and a key it mints is revoked, never kept). Returns the phase it ended in. */
export async function signInRun(env: SignInEnv, mode: CeremonyMode, email = '', code = ''): Promise<{ phase: Phase; rev: number }> {
  if (cur && cur.phase === 'working') dispatch(env, { t: 'cancel', id: cur.id });
  const id = env.newId();
  offer = null;
  adopt(env, begin(id, mode));
  let minted: Minted | null = null;
  try {
    if (mode === 'create' && email.trim() === '') throw new CeremonyError('email');
    let credential: unknown;
    if (mode === 'create') {
      const address = email.trim();
      const held = pendingProof && pendingProof.email === address ? pendingProof : null;
      if (!held || code.trim() === '') {
        // No code asked for this address yet, or none typed: mail one, and wait for it. No account is made, and nothing is stored on the server for the address.
        const binding = env.newId();
        const attemptId = await sendRegisterCode(env.ceremony, address, binding);
        pendingProof = { email: address, proof: { attemptId, code: '', binding } };
        if (!live(id)) return left(id);
        dispatch(env, { t: 'code', id });
        return left(id);
      }
      // The code is spent by this call whether or not the passkey screen then finishes: a second try asks for a new one.
      const proof: Proof = { ...held.proof, code: code.trim() };
      pendingProof = null;
      credential = await createCeremony(env.ceremony, address, proof);
    } else {
      credential = await getCeremony(env.ceremony);
    }
    if (!live(id)) return left(id);                          // replaced or cancelled while the passkey screen was up: nothing is sent
    minted = await finish(env, mode, credential);
    if (!live(id)) { await env.revoke(minted.key, minted.keyId).catch(() => undefined); return left(id); }     // a late answer of an old attempt: never written, revoked
    env.keep(minted.key, minted.keyId);                       // no await between the check above and this write
    minted = null;
    dispatch(env, { t: 'done', id });
  } catch (e) {
    if (minted) await env.revoke(minted.key, minted.keyId).catch(() => undefined);       // minted and not kept (the store refused): revoked
    let message = ceremonyMessage(e, mode);
    if (e instanceof DeletionPurging) message = 'This account is being removed now. The removal has begun and cannot be cancelled.';
    if (e instanceof DeletionPending) {
      if (live(id)) offer = { token: e.token, purgeOn: e.purgeOn };
      message = `This account is scheduled for deletion${e.purgeOn ? `; the removal begins after ${e.purgeOn}` : ''}. It has not begun. Cancel the deletion to keep it.`;
    }
    env.log(`signin: attempt ${id.slice(0, 6)} ${mode} ended: ${e instanceof DeletionPending ? 'deletion-pending' : e instanceof DeletionPurging ? 'deletion-purging' : e instanceof CeremonyError ? e.kind : 'error'}`);
    dispatch(env, { t: 'failed', id, message });
  }
  return left(id);
}

/** "Cancel deletion", pressed on the offer a sign-in of a pending account left. The token is one-use and goes once; the answer is the app's key, kept like a sign-in's. */
export async function signInCancelDeletion(env: SignInEnv): Promise<{ phase: Phase; rev: number }> {
  if (!offer || (cur && cur.phase === 'working')) return { phase: cur ? cur.phase : 'idle', rev };
  const { token, purgeOn } = offer;
  offer = null;                                                     // spent on this press, whatever the answer
  const id = env.newId();
  adopt(env, begin(id, 'signin'));
  let minted: Minted | null = null;
  try {
    const r = await postPublic(env.ceremony, CANCEL_DELETION_PATH, { cancel_token: token, app: env.appId });
    if (r.status === 409) throw new CeremonyError('failed', 'begun');
    if (r.status === 503 || r.status === 0) { offer = { token, purgeOn }; throw refusal(r); }       // nothing happened server-side: the offer stays
    if (r.status !== 200) throw new CeremonyError('failed', 'expired');
    const key = str(r.body, 'api_key'), keyId = str(r.body, 'key_id');
    if (!key.startsWith('choir_sk_') || keyId === '') throw new CeremonyError('failed', 'the server sent no key');
    minted = { key, keyId };
    if (!live(id)) { await env.revoke(key, keyId).catch(() => undefined); return left(id); }
    env.keep(key, keyId);
    minted = null;
    dispatch(env, { t: 'done', id });
  } catch (e) {
    if (minted) await env.revoke(minted.key, minted.keyId).catch(() => undefined);
    if (e instanceof CeremonyError && e.kind === 'offline' && !offer) offer = { token, purgeOn };   // no answer came: the token may be unspent
    const why = e instanceof CeremonyError ? e.message : '';
    const message = why === 'begun' ? 'The removal has begun and cannot be undone.'
      : why === 'expired' ? 'The offer to cancel ran out. Please sign in again.'
      : ceremonyMessage(e, 'signin');
    env.log(`signin: cancel deletion ended: ${why === 'begun' ? 'begun' : why === 'expired' ? 'expired' : e instanceof CeremonyError ? e.kind : 'error'}`);
    dispatch(env, { t: 'failed', id, message });
  }
  return left(id);
}

/** The screen's Cancel: the machine moves first, so any later answer is ignored (and a key that answer minted is revoked). */
export function signInCancel(env: SignInEnv): { phase: Phase; rev: number } {
  if (cur && cur.phase === 'working') dispatch(env, { t: 'cancel', id: cur.id });
  return { phase: cur ? cur.phase : 'idle', rev };
}

export const REVIEWER_PATH = '/auth/app/demo/login';
const REVIEWER_WORDS: Record<number, string> = {
  401: 'That name or secret is not right.',
  404: 'Signing in with a name is not available.',
  429: 'Too many tries. Try again in a few minutes.',
  503: 'Choir is busy. Try again in a moment.',
};

/** The reviewer sign-in (docs/ship/demo-account.md): a user name and a password, for Apple's reviewers, who cannot hold a passkey for this site. The same attempt machine and the same `keep` as the
 * passkey flow; the server answers a key for one account with a narrow grant. Nothing here logs the user name or the password, and neither is kept: they live in this call's arguments only. */
export async function reviewerRun(env: SignInEnv, username: string, password: string): Promise<{ phase: Phase; rev: number }> {
  if (cur && cur.phase === 'working') dispatch(env, { t: 'cancel', id: cur.id });
  const id = env.newId();
  adopt(env, begin(id, 'reviewer'));
  let minted: Minted | null = null;
  try {
    if (username.trim() === '' || password === '') { dispatch(env, { t: 'failed', id, message: REVIEWER_WORDS[401] }); return left(id); }
    const r = await postPublic(env.ceremony, REVIEWER_PATH, { app: env.appId, username: username.trim(), password });
    if (r.status !== 200) { dispatch(env, { t: 'failed', id, message: REVIEWER_WORDS[r.status] ?? 'Something went wrong. Try again.' }); return left(id); }
    const key = str(r.body, 'api_key'), keyId = str(r.body, 'key_id');
    if (!key.startsWith('choir_sk_') || keyId === '') throw new CeremonyError('failed', 'the server sent no key');
    minted = { key, keyId };
    if (!live(id)) { await env.revoke(key, keyId).catch(() => undefined); return left(id); }
    env.keep(key, keyId);                                     // no await between the check above and this write
    minted = null;
    dispatch(env, { t: 'done', id });
  } catch (e) {
    if (minted) await env.revoke(minted.key, minted.keyId).catch(() => undefined);
    env.log(`signin: attempt ${id.slice(0, 6)} reviewer ended: ${e instanceof CeremonyError ? e.kind : 'error'}`);
    dispatch(env, { t: 'failed', id, message: e instanceof CeremonyError && e.kind === 'offline' ? ceremonyMessage(e, 'signin') : 'Something went wrong. Try again.' });
  }
  return left(id);
}

/** Opening the screen again after an end starts clean: an old "failed" or "done" is not shown to the next person. */
export function signInReset(env: SignInEnv): { phase: Phase; rev: number } {
  if (cur && cur.phase === 'working') return { phase: cur.phase, rev };
  if (cur) { cur = null; offer = null; rev += 1; env.publish?.(view(null, rev)); }
  return { phase: 'idle', rev };
}
