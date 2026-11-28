# The sign-in sheet is offered while the key cannot be read

Standing: governing | kind: happened | settled-by: a reading of spine.contract, members-takes.contract, members-mail.contract and onboarding.contract on origin/main 954102fc7 by two review seats, 2026-11-28 | note: not seen on a phone; no test drives a locked key

## Problem

The app's contracts decide "offer sign-in" from one boolean, `au.signedIn`.
It is false when there is no key. It is also false when the key is there and
could not be read: a phone restarted and not yet unlocked, or a read Exact
refused. In that state the ask bar, the microphone button, Record and Mail's
ask open the sign-in sheet, and the first-run card says to sign in.

If the listener signs in there, a new key is written over an account that is
still signed in. The account-change check may then wipe the account's stores
on the phone.

Two more places carry the same boolean:

- `members-settings.contract` and `members-notebook.contract` pass it to their
  sources, so on a locked key the Computer buttons and the notebook skip their
  work as if signed out;
- `bar.ts` turns an engine error containing "not signed in" into "Sign in to
  talk to your computer". Whether a locked-key error can reach it through the
  native voice engine is not traced.

## Evidence

- `spine.contract` `askGate = au.signedIn ? "" : "signin"`, and `sendAsk`,
  `micTap`; `members-takes.contract:17`; `members-mail.contract:119`;
  `onboarding.contract:369-373`.
- `account-state.ts` `choirAuthState`: `signedIn: k.state === 'present'`.
- The account's own answer, `acs` (`accountState`), already tells the states
  apart and has an `action` that is `signIn` only for signed out and for a key
  the server refused.

## What was tried

Lane `session-unreadable` moved the sheet's sites to `acs.action`. The fourth
review sent it back: for a key the server refused, `acs.action` is `signIn`
and `au.signedIn` is true, so the sheet opened and the ask was sent as well,
and the typed text was cleared. The contract part was taken out; the record is
[three rounds without convergence](../evidence/key-read-refused-three-rounds-2026-11-28.md).

## Belief

The fix is small but has to be written as one rule for all five states
(signed in, signed out, locked, unknown, refused), with each site doing exactly
one thing in each state, and tested as a table. For the sheet:
open it only when `acs.action == "signIn"` and `au.signedIn` is false; a
refused key keeps main's behaviour. That has not been built or reviewed.

## Remaining error

- How often a listener meets a locked key in practice is not known. iPhone
  keeps the key readable after the first unlock, so the window is from a
  restart to the first unlock, when the app is rarely on screen.
- The web test host cannot fake a locked account, so the rule can be tested
  only as a table over the contract's expressions until a simulator drive can
  lock the Keychain.

## For the owner

Nothing here needs a decision to stay safe today: main's behaviour is
unchanged. The next step is a fresh, small change with the five-state table
written first. Say if the locked-key case should be left as it is until the
key's home is decided (D6), since that decision changes who reads the key.
