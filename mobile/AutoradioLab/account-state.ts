// What the app says about the account, from the facts: the stored key as three answers (api.ts keyState), whether the server refused it (a 401), demo mode, and whether this host has a native
// package at all. One list of words (words.ts). "Unreadable" is NOT signed out: a key that cannot be read yet (locked before the first unlock) says "Unlock your phone to continue", offers no
// sign-in and wipes nothing (account.ts reconcile never wipes on unreadable); the contract re-asks it when the app comes back to the front and every few seconds while locked, so it cannot stick.
// A bridge error, or no answer from a native package yet, is "unknown": nothing is shown. A host with no native package (Android, web) cannot sign in yet: "noSignIn", no button.
import { keyState, hasNativeHandle, rejectedDigest, isDemo, setDemo } from './api.ts';
import { WORDS } from './words.ts';

export { WORDS };
export type AccountView = { state: string; message: string; action: string };
const view = (state: string, message: string, action: string): AccountView => ({ state, message, action });

/** `_tick`: changes when something that can change the answer happened (a sign-in or sign-out, a page change, the app returning to the front, a request the server refused). `host`: the host fact
 * the contract already holds (`hc.mode`, decided in host.ts after its bounded wait): "none" (no native package: no sign-in here), "native", or "" (not decided yet). This source never waits itself
 * (a wait in a data source the bake cannot see through fails the build: "the answer is pending on nothing"): with no handle and no decision it answers unknown, which renders nothing. */
export async function accountState(_tick: string, top = '', demo = false, host = ''): Promise<AccountView> {
  if (demo) setDemo(true);
  if (isDemo()) return view('signedIn', '', '');
  if (!hasNativeHandle()) return host === 'none' ? view('noSignIn', WORDS.noSignIn, '') : view('unknown', '', '');
  const k = await keyState();
  if (k.error) return view('unknown', '', '');
  if (k.state === 'absent') return view('signedOut', top === 'radio:' ? WORDS.radioSignedOut : WORDS.signedOut, 'signIn');
  if (k.state === 'unreadable') return view('locked', WORDS.locked, '');
  if (rejectedDigest() !== '' && rejectedDigest() === k.digest) return view('rejected', WORDS.rejected, 'signIn');
  return view('signedIn', '', '');
}
