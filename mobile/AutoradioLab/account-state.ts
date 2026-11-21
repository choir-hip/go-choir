// What the app says about the account, from the account boundary (account.ts `peek`: the key and the record, read again, never remembered) and whether the server refused the key (a 401).
// One list of words (words.ts). The card is EXEMPT from the gate (app.ts): it never waits on a wipe; asking CLEARING starts one if none runs, and asking ADOPT retries its one write.
// Per state (docs/design/account-boundary-2026-11-21.md section 3): OPEN says nothing (after a 401: "no longer accepted", Clear this phone); EMPTY "signed out", Sign in (the ONLY state that offers it:
// no second account signs in while the first one's data is on the phone); ADOPT "could not record the account", Try again; CLEARING "clearing", and after five tries Try again and Clear this phone;
// LOCKED "Unlock your phone", and after five tries the same two buttons. A bridge error, or no answer from a native package yet, is "unknown": nothing is shown. A host with no native package
// (Android, web) cannot sign in yet: "noSignIn", no button. `epoch` moves when the boundary closes or a wipe completes: every gated resource takes it (the contracts' acctTick) so a page that
// refused is asked again when the boundary heals.
import { hasNativeHandle, rejectedDigest } from './api.ts';
import { peek } from './account.ts';
import { WORDS } from './words.ts';
import type { Storage, Store } from './app.contract.d.ts';

export { WORDS };
export type AccountView = { state: string; message: string; action: string; alsoClear: boolean; epoch: string };
const view = (state: string, message: string, action: string, epoch: string, alsoClear = false): AccountView => ({ state, message, action, alsoClear, epoch });
/** After this many re-reads of a refusing state the buttons appear (the contract's rereadBoundary stops at the same number). */
export const TRIES = 5;

/** `_tick`: changes when something that can change the answer happened (a sign-in or sign-out, a page change, the app returning to the front, a request the server refused, a re-read). `host`: the host
 * fact the contract already holds (`hc.mode`): "none" (no native package: no sign-in here), "native", or "" (not decided yet). `tries`: how many times the contract re-read a refusing state. This source
 * never waits itself (a wait in a data source the bake cannot see through fails the build): with no handle and no decision it answers unknown, which renders nothing. */
export async function accountState(_tick: string, top = '', host = '', tries = 0, storage?: Storage, store?: Store): Promise<AccountView> {
  const p = await peek(storage, store);
  if (!hasNativeHandle() && p.state === 'empty') return host === 'none' ? view('noSignIn', WORDS.noSignIn, '', p.epoch) : view('unknown', '', '', p.epoch);
  if (p.key.error) return view('unknown', '', '', p.epoch);
  const stuck = tries >= TRIES;
  const more = stuck ? ` ${WORDS.stuck}` : '';
  switch (p.state) {
    case 'empty': return view('signedOut', top === 'radio:' ? WORDS.radioSignedOut : WORDS.signedOut, 'signIn', p.epoch);
    case 'adopt': return view('adopt', WORDS.adopt + more, 'retry', p.epoch, stuck);
    case 'clearing': return view('clearing', WORDS.clearing + (stuck && p.failed.length ? ` Could not clear: ${p.failed.join(', ')}.` : '') + more, stuck ? 'retry' : '', p.epoch, stuck);
    case 'locked': return view('locked', WORDS.locked + more, stuck ? 'retry' : '', p.epoch, stuck);
    default:
      if (rejectedDigest() !== '' && rejectedDigest() === p.key.digest) return view('rejected', WORDS.rejected, 'clear', p.epoch);
      return view('signedIn', '', '', p.epoch);
  }
}
