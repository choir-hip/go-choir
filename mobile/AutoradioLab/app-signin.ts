// Sign-in and account creation with the platform's passkey screen, inside the app (owner, 2026-11-23: "native passkey account creation and sign in. no 'code'").
// One attempt is one small state machine with one owner (this file). Every event names its attempt; an event of another attempt, or after an end, changes nothing; a new
// press while one is running makes the newest win; and a late answer of an old attempt never writes a key (a key it minted is revoked).
//
//   create:  POST /auth/register/begin {email} -> the passkey screen (passkey-ceremony.ts) -> POST /auth/app/register/finish {app, credential} -> a key
//   sign in: POST /auth/app/login/begin        -> the passkey screen, no email typed    -> POST /auth/app/login/finish    {app, credential} -> a key
//
// The key: the finish route answers a bearer key with the grant a device approval gives. It is kept as the Exact secrets `choir.key` and `choir.key-id` (grant `secret.keep`,
// the Keychain on Apple, AfterFirstUnlockThisDeviceOnly). It never leaves this file's data layer: not a log line, not an answer, not a contract resource. The answer of a press
// is `{phase, rev}` and the view is `{phase, pending, mode, message, rev}`; no word of either can hold a key. The native voice code reads the same Keychain items (they are
// the one place the key lives) and writes none of them at sign-in.
import { CeremonyError, ceremonyMessage, createCeremony, getCeremony, postPublic, refusal, type CeremonyEnv } from './passkey-ceremony.ts';

export type Mode = 'create' | 'signin';
export type Phase = 'idle' | 'working' | 'done' | 'failed' | 'cancelled';
export type Attempt = { id: string; mode: Mode; phase: Phase; message: string };
export type Event = { t: 'done'; id: string } | { t: 'failed'; id: string; message: string } | { t: 'cancel'; id: string };

/** The names the app's key is kept under (grants `secret.keep <name>` in app.ts). The computer id is not a secret, kept beside the key so the voice code and shared code read one place. */
export const KEY_NAME = 'choir.key';
export const KEY_ID_NAME = 'choir.key-id';
export const COMPUTER_ID_NAME = 'choir.computer-id';

export const isTerminal = (p: Phase): boolean => p === 'done' || p === 'failed' || p === 'cancelled';
export const begin = (id: string, mode: Mode): Attempt => ({ id, mode, phase: 'working', message: '' });

/** The whole table. Pure. */
export function step(a: Attempt, e: Event): Attempt {
  if (e.id !== a.id || a.phase !== 'working') return a;
  switch (e.t) {
    case 'done': return { ...a, phase: 'done' };
    case 'failed': return { ...a, phase: 'failed', message: e.message };
    case 'cancel': return { ...a, phase: 'cancelled' };
  }
}

export type View = { phase: Phase; pending: boolean; mode: string; message: string; rev: number };
export function view(a: Attempt | null, rev = 0): View {
  if (!a) return { phase: 'idle', pending: false, mode: '', message: '', rev };
  return { phase: a.phase, pending: a.phase === 'working', mode: a.mode, message: a.phase === 'failed' ? a.message : '', rev };
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

let cur: Attempt | null = null;
let rev = 0;
export const resetAttempt = (): void => { cur = null; rev = 0; };
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

async function finish(env: SignInEnv, mode: Mode, credential: unknown): Promise<Minted> {
  const path = mode === 'create' ? '/auth/app/register/finish' : '/auth/app/login/finish';
  const r = await postPublic(env.ceremony, path, { app: env.appId, credential });
  if (r.status !== 200) throw refusal(r);
  const key = str(r.body, 'api_key'), keyId = str(r.body, 'key_id');
  if (!key.startsWith('choir_sk_') || keyId === '') throw new CeremonyError('failed', 'the server sent no key');
  return { key, keyId };
}

/** Start an attempt and run it to its end. A running attempt is superseded (the newest wins; its late answer is ignored, and a key it mints is revoked, never kept). Returns the phase it ended in. */
export async function signInRun(env: SignInEnv, mode: Mode, email = ''): Promise<{ phase: Phase; rev: number }> {
  if (cur && cur.phase === 'working') dispatch(env, { t: 'cancel', id: cur.id });
  const id = env.newId();
  adopt(env, begin(id, mode));
  let minted: Minted | null = null;
  try {
    if (mode === 'create' && email.trim() === '') throw new CeremonyError('email');
    const credential = mode === 'create' ? await createCeremony(env.ceremony, email.trim()) : await getCeremony(env.ceremony);
    if (!live(id)) return left(id);                          // replaced or cancelled while the passkey screen was up: nothing is sent
    minted = await finish(env, mode, credential);
    if (!live(id)) { await env.revoke(minted.key, minted.keyId).catch(() => undefined); return left(id); }     // a late answer of an old attempt: never written, revoked
    env.keep(minted.key, minted.keyId);                       // no await between the check above and this write
    minted = null;
    dispatch(env, { t: 'done', id });
  } catch (e) {
    if (minted) await env.revoke(minted.key, minted.keyId).catch(() => undefined);       // minted and not kept (the store refused): revoked
    const message = ceremonyMessage(e, mode);
    env.log(`signin: attempt ${id.slice(0, 6)} ${mode} ended: ${e instanceof CeremonyError ? e.kind : 'error'}`);
    dispatch(env, { t: 'failed', id, message });
  }
  return left(id);
}

/** The screen's Cancel: the machine moves first, so any later answer is ignored (and a key that answer minted is revoked). */
export function signInCancel(env: SignInEnv): { phase: Phase; rev: number } {
  if (cur && cur.phase === 'working') dispatch(env, { t: 'cancel', id: cur.id });
  return { phase: cur ? cur.phase : 'idle', rev };
}

/** Opening the screen again after an end starts clean: an old "failed" or "done" is not shown to the next person. */
export function signInReset(env: SignInEnv): { phase: Phase; rev: number } {
  if (cur && cur.phase === 'working') return { phase: cur.phase, rev };
  if (cur) { cur = null; rev += 1; env.publish?.(view(null, rev)); }
  return { phase: 'idle', rev };
}
