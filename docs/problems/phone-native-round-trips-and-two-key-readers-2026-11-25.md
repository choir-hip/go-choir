# The phone asks native code for facts shared code already holds, and "signed in" has two readers

Recorded 2026-11-25 by a builder in lane `move-b`, from the read-only study of the
native boundary (moves B, C and H of that study) and the phone run it cites
(8 launches of the build `f7e16f502`, the phone-int train). Fix in the same lane.

## Problem

1. **Two readers of "signed in".** Shared code reads the key from Exact's secret
   store (`api.ts` `storeSource`), and Swift reads the same Keychain item for
   the package source `choirAuth` (`ChoirSignIn.current`), for `choirPlayer`
   and for `barFacts.signedIn`. `au.signedIn` (from Swift) is part of the key of
   eight resources (`acs`, `cons`, `micperm`, `nb`, `acctInfo`, `cmpt`, `act`,
   `about`); at launch its native answer arrives and re-asks all eight, which
   lets go of answers in flight in the first second.
2. **The key is read from a remembered store after an `await`.** `bindStore`
   keeps "the store of the last answer"; every reader (`keyState`,
   `keptSession`, `keptItem`, `heldKey`, `retryPendingRevoke`) calls `get` on it
   later, from whatever turn is running. The phone's log shows
   `api: session found=true` and `found=false` alternating within 5 ms
   (20:43:26.831-.859). Exact's door op 2 answers `null` (not an error) when no
   store is bound and the call is not between answers
   (`$EXACT2/js/src/door.rs:45-53`), and `storeSource` maps a `null` key read
   to `absent`. `reconcile` (`account.ts`) turns `absent` plus an account stamp
   into `finish`: a full sign-out. Not observed, but one store-less read at
   launch would sign the owner out.
3. **The host probe asks native "are you there?" with a call and three timers**
   (`host.ts`: a `keystate` op raced against `setTimeout`, a 25 ms loop, a
   `waitMs` timer). Every timer and native call belongs to the answer that made
   it and is refused once that answer is let go (seat 2's finding A1 on the
   settle-once lane: the loop hangs). Exact answers the same question with no
   call: `native.available` (`$EXACT2/js/src/prelude.js:833-841`;
   `door.rs:112-120`: true when the app links a module).
4. **Pure round trips through Swift** (shared code sends a value to Swift and
   reads it back): `mailOpenReview` (a URL checked, then sent back as the
   `openWeb` verb into radio-bridge's inbox), `readerAim`/`readerAimSet` (no
   native code reads `ReaderAim`), `editionMirror`/`editionMirrorReset` (only
   read by `retrievalAsk`'s `facts`, which go back to shared code), the
   `facts` of `retrievalAsk` (the run sheet, the open pages, the topics and the
   page text: all shared code's), `importedFollow` (dead: nothing sets it), and
   the four package sends `deckPlay`, `deckEnqueue`, `deckAct`, `deckAck` (no
   contract calls them; shared code uses the app-module `deck` op).
   `pageRequest`/`openPage` and `obMirror`/`radioFacts.ready` are round trips
   too; both ends are in `radio-bridge.ts`.
5. **Clock and version from native.** `settingsFacts.now` is native's
   `Date()` and `appVersion` the bundle's `CFBundleShortVersionString`, while
   the build already writes `build-info.gen.ts`, and Exact gives the clock to
   a contract (`exactTime`).

## Evidence

- The study: `…/opencode/native-boundary/report.md`, sections 1a, 1b, 2.1, 2.3.
- The run: `…/opencode/phone-settle/unified-app.log` (the flap lines),
  `console-N.log` (the native op ledger: `choirAuth`, `choirPlayer`,
  `barFacts` 3-4 per launch each).
- `scripts/check-autoradio-native-boundary` on this tree before the change:
  35 files, 3,485 lines, 18 names, above its own ceiling (34, 3,403, 18): the
  ceiling file was not regenerated after the train's merges.

## Belief

- The flap is reads from a turn with no store bound (unproved which turn); one
  read per answer, made while the answer's own store is bound and kept as a
  value, removes it whatever the cause.
- Deriving "signed in" from the shared account answer removes the launch
  re-key and the native reader together.

## Remaining error

- Whether `native.available` is true in the first answer of a launch on the
  phone is read from source (the module is configured with the JS context,
  `js/src/lib.rs:770-791`), not seen on a device.
- The voice turn still reads the key natively (`KeychainKeyStore`, through
  `ChoirEnvironment.keys`): the voice calls the server from native code. That
  is a reader of the key, not of "signed in", and stays with its reason.
- `pageRequest`/`openPage` and `obMirror`/`radioFacts.ready` are not removed
  here: their shared ends are in `radio-bridge.ts`, which the parallel Move A
  rewrites.
