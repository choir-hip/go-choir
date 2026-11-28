// What the app says about the account, from the facts: the stored key as three answers (api.ts keyState), whether the server refused it (a 401), and whether there is anywhere to read a key from (the secret store of an answer). One list of words (words.ts). "Unreadable" is NOT signed out: a key that cannot be read yet (locked before the first unlock) says "Unlock your phone to continue", offers no
// sign-in and wipes nothing (account.ts reconcile never wipes on unreadable); the contract re-asks it when the app comes back to the front and every few seconds while locked, so it cannot stick.
// A bridge error, or no answer from a native package yet, is "unknown": nothing is shown. A host with no secret store at all cannot sign in: "noSignIn", no button.
import { keyState, hasCredentialSource, rejectedDigest, ORIGIN } from './api.ts';
import { WORDS } from './words.ts';
import { APP_ID } from './app-id.ts';
import { passkeySupport } from './app-signin.ts';

export { WORDS };
export type AccountView = { state: string; message: string; action: string };
export type ChoirAuthState = { signedIn: boolean; baseURL: string; status: string; message: string };
const view = (state: string, message: string, action: string): AccountView => ({ state, message, action });

/** The former Apple package source, now shared: read the Exact secret store through api.ts and keep the same four-field shape. An unreadable key is not signed in here; accountState below says "locked" and wipes nothing. */
export async function choirAuthState(): Promise<ChoirAuthState> {
  const k = await keyState();
  return { signedIn: k.state === 'present', baseURL: ORIGIN, status: k.state !== 'unreadable' ? 'idle' : k.noStore || k.error ? 'unknown' : 'locked', message: '' };
}

/** `_tick`: changes when something that can change the answer happened (a sign-in or sign-out, a page change, the app returning to the front, a request the server refused). `host`: the host fact
 * the contract already holds (`hc.mode`, decided in host.ts after its bounded wait): "none" (no native package: no sign-in here), "native", or "" (not decided yet). This source never waits itself
 * (a wait in a data source the bake cannot see through fails the build: "the answer is pending on nothing"): with no handle and no decision it answers unknown, which renders nothing. */
export async function accountState(_tick: string, top = '', host = '', platform = '', appId = APP_ID): Promise<AccountView> {
  if (!hasCredentialSource()) return host === 'none' ? view('noSignIn', WORDS.noSignIn, '') : view('unknown', '', '');
  const k = await keyState();
  if (k.error || (k.state === 'unreadable' && k.noStore)) return view('unknown', '', '');      // a refusal for want of a store is not a locked phone
  // A build that cannot make a passkey says so, plainly, instead of offering a sign-in that fails: `platform` is exactPage().platform, `appId` this build's id (app-id.ts).
  const support = passkeySupport(appId, platform);
  const noPasskey = (): AccountView => view('noPasskey', support === 'android' ? WORDS.noPasskeyAndroid : WORDS.noPasskeyApp, '');
  if (k.state === 'absent') return support ? noPasskey() : view('signedOut', top === 'radio:' ? WORDS.radioSignedOut : WORDS.signedOut, 'signIn');
  if (k.state === 'unreadable') return view('locked', WORDS.locked, '');
  if (rejectedDigest() !== '' && rejectedDigest() === k.digest) return support ? noPasskey() : view('rejected', WORDS.rejected, 'signIn');
  return view('signedIn', '', '');
}
