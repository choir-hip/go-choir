// The Settings page's account source. `settingsAccount` answers who is signed in (GET /auth/whoami) as the Account screen draws it.
// It replaces the Swift AccountModel, AccountClient and WhoAmI. Sign-in is shared code (app-signin.ts); the key, an Exact secret, is read in
// place by the signed-in seam in api.ts. The key itself is never read here: the view carries the handle, key id, key label and scopes.
//
// Failure list (each line has a test in settings.test.ts):
//  A1 the key or an email is shown or logged: only handle, key id, key label and scopes are copied; a failure logs nothing.
//  A2 whoami answers 401, 404, garbage or nothing: the view says which in plain words and the page still renders.
//  A3 signed out: the view says "Not signed in." and no request is sent.
//  A5 a body with no handle is "not what was expected", never a half-filled view.
//  A6 an owner-wide key answers `computer_id: ""`: the Keychain's stored computer id fills it (the computer the phone found); a bound id wins.
// Dropped: the "loading" status and the generation guard (A3 in Swift): a source answers once per ask, so a late answer cannot land.

import { WORDS } from './words.ts';
import { authedJSON, HTTPError, keyState, keptComputerId, ORIGIN, SignedOutError, unreadableWords } from './api.ts';

export type SettingsAccount = {
  status: string; signedIn: boolean; handle: string; keyID: string; keyLabel: string; scopes: string; computerID: string; message: string; baseURL: string;
  /** The key's account is the App Review account (whoami `reviewer`): it cannot be deleted in the app. */
  reviewer: boolean;
};
export type AccountArgs = { signedIn: boolean; tick: number };

export const WHOAMI_PATH = '/auth/whoami';

const empty = (): SettingsAccount => ({ status: 'idle', signedIn: false, handle: '', keyID: '', keyLabel: '', scopes: '', computerID: '', message: '', baseURL: ORIGIN, reviewer: false });

const str = (v: unknown): string => (typeof v === 'string' ? v : '');

/** The Account screen's view from a whoami body; null when the body has no handle (A5). */
export function parseWhoami(body: unknown): Omit<SettingsAccount, 'signedIn' | 'baseURL' | 'message' | 'status'> | null {
  if (!body || typeof body !== 'object') return null;
  const o = body as Record<string, unknown>;
  const handle = str(o.handle);
  if (handle === '') return null;
  const scopes = Array.isArray(o.scopes) ? o.scopes.filter((s): s is string => typeof s === 'string') : [];
  return { handle, keyID: str(o.key_id), keyLabel: str(o.key_label), scopes: scopes.join(', '), computerID: str(o.computer_id), reviewer: o.reviewer === true };
}

/** What the page says for a failure, in plain words (A2). */
export function plainAccountError(e: unknown): string {
  const unread = unreadableWords(e); if (unread) return unread;
  if (e instanceof SignedOutError) return "This phone's sign-in is no longer accepted. Sign out and sign in again.";
  if (e instanceof HTTPError && e.status === 404) return 'This server cannot say which account a key is yet.';
  if (e instanceof Error && /not JSON/.test(e.message)) return "The server's answer was not what was expected.";
  return 'Could not reach choir.news.';
}

export async function settingsAccount(args: AccountArgs): Promise<SettingsAccount> {
  const base = empty();
  if (!args.signedIn) {
    // `signedIn` is a bool: false is also what a key that could not be read gives (account-state.ts). Ask which, and say so.
    const k = await keyState();
    const words = k.state === 'unreadable' ? (k.error || k.noStore ? WORDS.unknown : WORDS.locked) : null;
    return { ...base, message: words ?? 'Not signed in.' };
  }
  try {
    const r = await authedJSON(WHOAMI_PATH);
    const w = parseWhoami(r.body);
    if (!w) return { ...base, signedIn: true, status: 'failed', message: "The server's answer was not what was expected." };
    const kept = w.computerID === '' ? await keptComputerId().catch(() => null) : null;
    return { ...base, ...w, computerID: w.computerID || kept || '', signedIn: true, status: 'ok' };
  } catch (e) {
    return { ...base, signedIn: true, status: 'failed', message: plainAccountError(e) };
  }
}

/** The About page's app and server rows: the app's version is a platform fact (the bundle), the server is this build's one origin. */
export const aboutPage = (f: { appVersion: string }, signedIn: boolean): { app: string; base: string; signedIn: boolean } =>
  ({ app: f.appVersion === '' ? 'dev' : f.appVersion, base: ORIGIN, signedIn });
