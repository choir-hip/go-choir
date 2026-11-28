# The refused key read: three review rounds without convergence, and the change of approach

Standing: governing | kind: happened | settled-by: three reviews each by seat 1 (Gemini 4 Hydrogen, diff only) and a fresh Sonnet whole-tree reviewer of lane session-unreadable, the last on da1dd8523, 2026-11-28 | note: nothing in the lane has landed or run on a device

The lane fixes the weakness under
[the changelog request storm](../problems/a-signed-in-app-asks-the-server-for-the-changelog-about-forty-times-a-second-2026-11-28.md):
a key read that Exact refuses was answered as "no key". The contract says
three rounds without convergence end the patching. This is the assessment.

## What happened

| Round | Head | What it built | Why it was sent back |
| --- | --- | --- | --- |
| 1 | f4ec42e84 | A refused session read is "unreadable"; `keptSession` throws `locked` or `unknown`, never `none` | Eight pages ignore the reason and show sign-in words; malformed answers still read as "no key"; the real store path was untested |
| 2 | 948d1a3ee | One shared wording function; each named page uses it | One more page (delete account) still said "sign in"; a let-go answer lost its marker and would be retried; the reason came from a second read |
| 3 | da1dd8523 | Delete account fixed; a static guard lists every `.ts` file that touches sign-in state; a flag for a between-answers refusal | The contracts open the sign-in sheet on `au.signedIn == false`, which is false for an unreadable key (ask bar, takes, mail, first run); `choirAuthState` answers the same for "no key" and "could not read"; seven files still test the error by `instanceof` |

The change grew from 4 files and 188 lines to 19 files and 582.

## What the three rounds have in common

Every blocking finding after round 1 is one defect in different places:
**"is the listener signed in" has three answers (yes, no, could not read) and
is carried through channels that hold two.** A boolean (`au.signedIn`), an
error class without its reason, a status code, a state string. Each round
widened one channel and the next review found another. The guard added in
round 3 lists the `.ts` channels; the contracts are a channel it does not see.

The second cause is the lead's. The first brief said "every reader tells no
key from could not read", which made the change a sweep of every page rather
than a fix at one source, and each send-back added sites to the sweep.

A correction to the lead's own premise: before this lane, a refused read
already reached the pages as reason `locked`, not `none`, because the second
question (`keystate`) was refused too. What was wrong on main was narrower
than the first brief said: the log line, the words "Unlock your phone" for a
phone that is not locked, pages that ignore the reason, and the sign-in sheet
offered on a boolean.

## What is not in doubt

Reviewed and not faulted in round 3:

- `keptSession` answers null only for a clean "no key"; a refused or malformed
  answer throws `locked` or `unknown`, with the reason taken from the failing
  read;
- `keyState` carries the between-answers flag; `reconcile`, sign-out and
  `credentialsAbsent` do nothing on an unreadable key;
- delete account tells present, absent and unreadable apart and posts nothing
  on the third;
- the Texture editor keeps an unsaved edit in all three cases (offline, locked,
  between answers);
- the guard's list of `.ts` files is complete and its claims are true.

## The change of approach

The sweep stops. There is already one place that decides, with every fact in
hand, whether a sign-in may be offered: `accountState` in `account-state.ts`,
whose answer `acs` has a state (signed in, signed out, locked, unknown,
rejected, no sign-in here) and an `action` that is `signIn` only when a
sign-in is the right thing. So:

- every contract that opens the sign-in sheet or says "sign in" asks `acs`,
  through one derive, and not `au.signedIn`;
- `choirAuthState` gives `unknown` its own status, so "could not read" is not
  the same answer as "no key";
- a pin test lists every contract line that reads `au.signedIn` or sets the
  sign-in gate, so a new one is reviewed (the same kind of pin as the clock
  lines in the request-storm fix);
- the error is recognised one way (by name) everywhere.

No page is reworded beyond what round 3 already holds. No requirement is added.

## What happens next

One build on this approach, then both seats. If either sends it back for a
defect of the same kind, the lane is cut down to the parts listed under "not
in doubt" (they are safe alone and can land), and the sign-in offer on an
unreadable key goes to the owner as an open problem with this record.

## Not settled by any round

- Whether errors are ever copied across a boundary that breaks `instanceof`.
  No reviewer showed one; recognising by name costs nothing.
- If Exact rewords "no store here", a between-answers refusal reads as
  `locked`: safe, with slightly wrong words.
- With no store and not between answers, `store.get` returns null, which
  reads as "no key" (`door.rs:53`). Not closed here.
- The larger fix: read the key inside the answer that starts a piece of work
  and hand it on as a value, so nothing reads the store between answers.
