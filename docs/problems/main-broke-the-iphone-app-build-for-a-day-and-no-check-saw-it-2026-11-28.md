# Main did not compile as an iPhone app for 31 hours and no check saw it

Standing: governing | kind: happened | settled-by: two simulator builds of origin/main 00aedd0bc in the build tree on 2026-11-28, the first failing, the second with a one-line fix passing | note: only the simulator build was run; no phone build and nothing was run on a device

## Problem

The Android deck landing `ce79438a8` (2026-11-26) added one line to
`mobile/exact-package/apple/AutoradioModule+Deck.swift`: a call to
`Deck.shared.boot()` from a function that is not on the main actor. Swift
refuses it. From that commit until the fix, no iPhone app could be built from
main.

Nothing noticed. The Swift package tests do not compile `mobile/exact-package/apple`
or `mobile/AutoradioLab/modules/apple`; they compile only in the app build. CI
has no Mac and builds neither. The app build runs only in the one build tree,
by hand, and between 2026-11-26 and 2026-11-28 the tree was used for Android
builds only.

## Evidence

- `build-lab.sh ios` (simulator, `AR_VARIANT=lab-one`) at `00aedd0bc`:
  `AutoradioModule+Deck.swift:48:21: error: call to main actor-isolated
  instance method 'boot()' in a synchronous nonisolated context`.
- `git blame` gives line 48 to `ce79438a8`.
- The same build with the call wrapped in `onMainThread`, as the next line
  already is: `Build complete`.
- Between the two commits, seven landings changed Swift in those two
  directories (the Android deck and passkey landings, and five of the move to
  Exact on 2026-11-27). All were read by reviewers. None was compiled.

## Belief

One cause: the app-side Swift has no compile check that runs without a person.
A reviewer reading a diff does not find an actor-isolation error reliably, and
two did not here.

## Remaining error

- The second build passed, so the five landings of 2026-11-27 in those
  directories compile. Whether they behave is not known: nothing ran.
- Built 2026-11-28: the second option, a landing step (`scripts/check-app-compiles`, run by `scripts/land`
  after the gates; [change](../changes/2026-11-28-landings-compile-the-iphone-app.md)). It compiles the
  simulator app in the build tree when a train touches app-side Swift or what it compiles against, and refuses
  when that tree is dirty. It has never run against a real build.
- Still open: CI does not build the app (no Mac runner), so a push that bypasses `scripts/land` is not
  checked. The step needs the build tree clean and free, and about ten minutes under the one Mac build lock,
  so landings on this Mac queue behind it. The Swift package tests still do not compile the two directories.
- Android is not compiled by that step. The same method fits: `exact.sh android --no-install` builds an APK
  without adb (Exact's reference, `bun exact.mjs android --release --no-install --json`), and exact-guard.sh
  allows it for a development app in the build tree. How long it takes is not known: no log of an Android
  build was found, and none was run.
- The Android build was not re-run after the fix.
