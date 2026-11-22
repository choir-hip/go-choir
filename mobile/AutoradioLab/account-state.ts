// What the app says about the account, from the account boundary (account.ts `peek`: the launch verdict made in shared code from the store's key and the markers, held for the process) and whether the server refused the key (a 401). One list of words (words.ts).
// The card is a DEVICE source (app.ts): it never waits on a native wipe (a mark the launch wipe could not clear ends the session: the card says close and reopen) and never reads a page. States: `signedIn` (an account world is open), `signedOut` (the anonymous world: public data, one way in), `locked` (the key cannot be read yet: "Unlock your
// phone"), `rejected` (the server refused the key: Sign out), `ended` (a sign-out, or the Keychain changing under an open account, ended the session: a full-screen card says to close and reopen the app), `unknown` (no native
// handle answered yet: nothing is shown) and `noSignIn` (a host with no native package cannot sign in). `handle` is the account's first 16 hex (or `anonymous`, or ''): the identifying argument of every gated resource
// (invariant I7), so Exact's kept answers of one account are never shown for another. It moves when the account opens, so a page that refused is asked again.
import { peek } from './account.ts';
import { WORDS } from './words.ts';
import type { Storage, Store } from './app.contract.d.ts';

export { WORDS };
export type AccountView = { state: string; message: string; radioMessage: string; action: string; alsoSignOut: boolean; handle: string };
const view = (state: string, message: string, action: string, handle: string, radioMessage = '', alsoSignOut = false): AccountView => ({ state, message, radioMessage, action, alsoSignOut, handle });
/** After this many re-reads of a locked state the buttons appear (the contract's rereadBoundary stops at the same number). */
export const TRIES = 5;

/** `_tick`: changes when something that can change the answer happened (a sign-in or sign-out, a page change, the app returning to the front, a request the server refused, a re-read). `host`: the host fact the contract
 * already holds (`hc.mode`): "none" (no native package: no sign-in here), "native", or "" (not decided yet). This source never waits itself: with no handle and no decision it answers unknown, which renders nothing. */
export async function accountState(_tick: string, _host = '', tries = 0, storage?: Storage, store?: Store): Promise<AccountView> {
  const p = await peek(storage, store, tries);      // reading the store here makes this answer one Exact asks again when the host's retry brings a key it could not read
  if (p.state === 'ended') return view('ended', p.ended === 'unfinished' ? WORDS.endedUnfinished : p.ended === 'blocked' ? WORDS.endedBlocked : WORDS.endedSignedOut, '', '');
  switch (p.state) {
    case 'anonymous': return view('signedOut', WORDS.signedOut, 'signIn', p.handle, WORDS.radioSignedOut);
    case 'locked': {
      const stuck = tries >= TRIES;      // a state that cannot be left is not allowed: after five re-reads, Try again and Sign out (native removes the three Keychain accounts even when no module was built, and says whether it did)
      return view('locked', (p.why === 'starting' ? WORDS.starting : WORDS.locked) + (stuck ? ` ${WORDS.stuck}` : ''), stuck ? 'retry' : '', '', '', stuck);
    }
    default:
      if (p.rejected) return view('rejected', WORDS.rejected, 'signOut', p.handle);
      return view('signedIn', '', '', p.handle);
  }
}
