// Sign-in in the system sign-in sheet. The device flow is unchanged and stays native where the key is: Swift (ChoirSignIn) begins it, keeps the device code,
// polls /auth/device/token and writes the Keychain. That is the one reason this is not all here: the device code and the key never pass through shared code.
// Shared code does the one thing that used to leave the app: it OPENS THE APPROVAL PAGE in Exact's `openAuthSession` (ASWebAuthenticationSession on iPhone,
// Custom Tabs / Auth Tab on Android; docs/reference/exact/auth-session.md), a sheet over the app that shares the browser's passkeys and session and closes
// by itself when the page navigates to the app's callback.
//
// The callback is a HINT. It carries `state` (our own nonce, which Exact checks) and nothing else: no key, no code, no account. This module never reads it:
// the answer is one of three words, and whether the phone is signed in is whatever the Keychain says (account-state.ts), which the native poll updates.
// A callback with no approval behind it therefore changes nothing; the poll goes on asking the server.
//
// If the sheet cannot open (501 no browser, 502, a throw), the card offers the same link in the system browser (account.contract / onboarding.contract: Exact's `href`), and the
// native poll goes on as before: the sign-in has no path that is only the sheet.
//
// Which scheme: `authCallback()` is this build's granted `auth.callback` (app.ts: `<app id>:/signed-in`; each variant has its own id, so its own scheme). The
// page is told the scheme's NAME (not a URL); the server builds the callback from a list of the apps that exist (internal/auth/device_login.go).

export const RETURN_PATH = '/signed-in';

/** How the sheet ended. Which sign-in it belongs to is the attempt's business (signin-attempt.ts): the attempt that opened it names its id when it hears this. */
export type SheetResult = { outcome: 'returned' | 'cancelled' | 'failed'; message: string };
export type SheetEnv = {
  /** The one server origin the grant `auth.session` names. */
  origin: string;
  authCallback(): string;
  openAuthSession(url: string, options: { callback: string; state: string; ephemeral?: boolean; logout?: boolean }): Promise<string>;
  log(line: string): void;
};

/** Exact's two globals, as a host with the grants has them. */
export function liveSheetEnv(origin: string): SheetEnv {
  const g = globalThis as unknown as { authCallback?: () => string; openAuthSession?: SheetEnv['openAuthSession'] };
  return {
    origin,
    authCallback: () => { if (typeof g.authCallback !== 'function') throw new Error('no auth callback'); return g.authCallback(); },
    openAuthSession: (u, o) => { if (typeof g.openAuthSession !== 'function') return Promise.reject(new Error('no auth session')); return g.openAuthSession(u, o); },
    log: (l) => console.log(l),
  };
}

const CALLBACK = /^([a-z][a-z0-9.+-]*):\/signed-in$/;
const NOT_AN_APP = new Set(['http', 'https', 'javascript', 'data', 'file', 'blob', 'about', 'vbscript']);

/** The approval page's link with this app's callback scheme and this sign-in's nonce added; null when the link is not the granted origin over https or the callback is not ours. */
export function sheetUrl(link: string, callback: string, state: string, origin: string): string | null {
  const m = CALLBACK.exec(callback);
  if (!m || NOT_AN_APP.has(m[1])) return null;
  let u: URL;
  try { u = new URL(link); } catch { return null; }
  if (u.protocol !== 'https:' || u.origin !== origin || u.username !== '' || u.password !== '') return null;
  u.searchParams.set('return', m[1]);
  u.searchParams.set('state', state);
  return u.toString();
}

const nonce = (): string => Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) => b.toString(16).padStart(2, '0')).join('');   // 32 hex characters

/** What the card says when the sheet could not open. */
export const sheetOutcomeNote = 'The sign-in window could not open. Open it in your browser instead; this screen finishes the sign-in once you approve.';

/** Open the approval page in the sheet and say how the sheet ended. Never throws; never carries the callback's URL. */
export async function signInSheet(link: string, env: SheetEnv): Promise<SheetResult> {
  const failed = (why: string): SheetResult => { env.log(`signin: sheet failed${why}`); return { outcome: 'failed', message: sheetOutcomeNote }; };
  let callback: string;
  try { callback = env.authCallback(); } catch { return failed(' (no callback)'); }
  let state: string;
  try { state = nonce(); } catch { return failed(' (no nonce)'); }
  const url = sheetUrl(link, callback, state, env.origin);
  if (url === null) return failed(' (not our link)');
  if (typeof env.openAuthSession !== 'function') return failed(' (no session)');
  env.log('signin: sheet opened');
  try {
    await env.openAuthSession(url, { callback, state, ephemeral: false });      // not ephemeral: the sheet must share the browser's passkeys and session
    env.log('signin: sheet returned');
    return { outcome: 'returned', message: '' };
  } catch (e) {
    const status = (e as { status?: unknown } | null)?.status;
    if (status === 499) { env.log('signin: sheet cancelled'); return { outcome: 'cancelled', message: '' }; }
    return failed(typeof status === 'number' ? ` status=${status}` : '');
  }
}
