// SIGN-IN IN SHARED CODE (docs/design/account-boundary-2026-11-21.md, "Round 12"): the device flow against the server (`/auth/device/begin`, then `/auth/device/token` until approved), the key and its id kept with `store.set`
// under `secret.keep`, and the marker that says an account was signed in here. The approval page is opened by the contract's own `openURL` host command (recipes/functions.md), never by native. The device code stays in this module and
// is never returned; the key goes only to the store and to `holdSession`. A sign-in is refused unless the process has no key and has not ended its session.
import type { Storage, Store } from './app.contract.d.ts';
import { unauthedPost, holdSession, heldSession, adoptStore, sessionEnded, currentWorld, SECRET_KEY, SECRET_KEY_ID } from './api.ts';
import { handKey, writeSigned } from './account.ts';

export type SignInView = { signedIn: boolean; baseURL: string; status: string; userCode: string; verificationURL: string; message: string };
type Flow = { device: string; wait: number; ticks: number; expiresMs: number; spentMs: number; token: number };
let status = 'idle', userCode = '', verificationURL = '', message = '';
let flow: Flow | null = null;
let serial = 0;                                   // a cancel, a sign-out or a second sign-in moves it: a poll that began under an older number stores nothing
const POLL_TICK_MS = 3000;                         // the contract's task cadence (account.contract signInTick)

export function resetSignIn(): void { status = 'idle'; userCode = ''; verificationURL = ''; message = ''; flow = null; serial += 1; }
export function signInView(store?: Store): SignInView { adoptStore(store); return { signedIn: heldSession() !== null, baseURL: '', status, userCode, verificationURL, message }; }
const fail = (m: string): void => { status = 'failed'; userCode = ''; verificationURL = ''; message = m; flow = null; };

/** Begin: ask the server for the codes. Refused when the session has ended; a held key means already signed in. */
export async function signInStart(_storage: Storage | undefined, store?: Store): Promise<SignInView> {
  adoptStore(store);
  if (sessionEnded()) { fail('Close Autoradio and open it again to sign in.'); return signInView(store); }
  if (heldSession()) return signInView(store);
  const mine = (serial += 1); status = 'pending'; userCode = ''; verificationURL = ''; message = 'contacting the server'; flow = null;
  try {
    const r = await unauthedPost('/auth/device/begin', { label: 'Autoradio' });
    if (mine !== serial) return signInView(store);
    if (!r.ok) { fail(`The server answered ${r.status}. Try again.`); return signInView(store); }
    const o = (await r.json()) as { device_code?: string; user_code?: string; verification_uri_complete?: string; verification_uri?: string; interval?: number; expires_in?: number };
    if (mine !== serial) return signInView(store);
    if (!o.device_code || !o.user_code) { fail('The server did not give a sign-in code. Try again.'); return signInView(store); }
    const url = o.verification_uri_complete ?? o.verification_uri ?? '';
    userCode = o.user_code; verificationURL = /^https?:\/\//.test(url) ? url : ''; message = 'approve this sign-in in the browser';
    flow = { device: o.device_code, wait: Math.max(1, Number(o.interval) || 5), ticks: 0, expiresMs: (Number(o.expires_in) || 600) * 1000, spentMs: 0, token: mine };
  } catch { if (mine === serial) fail('Could not reach the server. Try again.'); }
  return signInView(store);
}

/** One poll, every contract tick (3 s) while pending; the server's interval is kept by counting ticks (`force`: a test). Stores the key only while this flow is the current one, no key is held and the session has not ended. */
export async function signInPoll(storage: Storage | undefined, store?: Store, force = false): Promise<SignInView> {
  const f = flow;
  if (!f || status !== 'pending' || f.token !== serial) return signInView(store);
  f.ticks += 1; f.spentMs += POLL_TICK_MS;
  if (f.spentMs > f.expiresMs + 5000) { fail('The code expired; sign in again.'); return signInView(store); }
  if (!force && f.ticks * POLL_TICK_MS < f.wait * 1000) return signInView(store);
  f.ticks = 0;
  try {
    const r = await unauthedPost('/auth/device/token', { device_code: f.device });
    if (f.token !== serial || sessionEnded()) return signInView(store);
    const o = (await r.json()) as { error?: string; api_key?: string; key_id?: string };
    if (r.status === 400) {
      switch (o.error ?? '') {
        case 'authorization_pending': return signInView(store);
        case 'slow_down': f.wait += 5; return signInView(store);
        case 'access_denied': fail('The sign-in was denied.'); return signInView(store);
        case 'expired_token': fail('The code expired; sign in again.'); return signInView(store);
        default: fail('The code is no longer valid.'); return signInView(store);
      }
    }
    if (!r.ok || !o.api_key) { fail('The server did not give a key. Try again.'); return signInView(store); }
    if (f.token !== serial || sessionEnded() || heldSession() || currentWorld().kind === 'account') return signInView(store);
    if (!storage || !store) { fail('Could not store the key; close Autoradio and open it again.'); return signInView(store); }
    // THE STORE IS HERE ONLY IN THIS TURN (the continuation of an effectful request): the key is stored before any other await (account.ts `turn`).
    try { store.set(SECRET_KEY, o.api_key); if (o.key_id) store.set(SECRET_KEY_ID, o.key_id); } catch { fail('Could not store the key; close Autoradio and open it again.'); return signInView(store); }
    holdSession(o.api_key, o.key_id ?? '');
    if (!(await writeSigned(storage))) console.log('account: sign-in: the signed-in marker could not be written');      // the key is stored; the marker only tells "absent" from "not readable yet" before the first unlock
    await handKey();
    status = 'idle'; userCode = ''; verificationURL = ''; message = 'signed in'; flow = null;
  } catch { if (f.token === serial && status === 'pending') message = 'waiting; the network did not answer'; }
  return signInView(store);
}

/** Stop waiting for an approval; keeps any key already stored. */
export function signInCancel(): SignInView { serial += 1; flow = null; status = 'idle'; userCode = ''; verificationURL = ''; message = ''; return signInView(); }
