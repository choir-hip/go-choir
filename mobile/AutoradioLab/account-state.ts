// What the app says about the account, from the facts: the stored key as three answers (api.ts keyState), whether the server refused it (a 401), demo mode, and whether this host has a native
// package at all. One list of words; the pages and the banner use it. "Unreadable" is NOT signed out: a key that cannot be read yet (locked before the first unlock) says "Unlock your phone to
// continue", offers no sign-in, wipes nothing (account.ts reconcile never wipes on unreadable), and is asked again when the phone unlocks (the native key-store notifier touches `choirAuth`, which
// changes the first argument). A bridge error is "unknown": nothing is shown that could be wrong.
import { keyState, hasNativeHandle, rejectedDigest, isDemo } from './api.ts';

export const WORDS = {
  signedOut: 'You are signed out. Sign in to use your own computer: your mail, your files, your paper.',
  radioSignedOut: 'Reading the public paper. Sign in to hear your own programme.',
  locked: 'Unlock your phone to continue.',
  rejected: "This phone's sign-in is no longer accepted. Sign in again.",
};

export type AccountView = { state: string; message: string; action: string };
const view = (state: string, message: string, action: string): AccountView => ({ state, message, action });

/** `_tick`: changes when something that can change the answer happened (a sign-in, a sign-out, a page change, the phone unlocking); the source is asked again. */
export async function accountState(_tick: string, top = ''): Promise<AccountView> {
  const out = await decide();
  return out.state === 'signedOut' && top === 'radio:' ? { ...out, message: WORDS.radioSignedOut } : out;
}

async function decide(): Promise<AccountView> {
  if (isDemo()) return view('signedIn', '', '');
  if (!hasNativeHandle()) return view('signedOut', WORDS.signedOut, 'signIn');
  const k = await keyState();
  if (k.error) return view('unknown', '', '');
  if (k.state === 'absent') return view('signedOut', WORDS.signedOut, 'signIn');
  if (k.state === 'unreadable') return view('locked', WORDS.locked, '');
  if (rejectedDigest() !== '' && rejectedDigest() === k.digest) return view('rejected', WORDS.rejected, 'signIn');
  return view('signedIn', '', '');
}
