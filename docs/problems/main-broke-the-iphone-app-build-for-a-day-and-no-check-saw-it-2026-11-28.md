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
- The gap is still open. Until a check compiles the app-side Swift on every
  landing that touches it, this will recur. Options, none chosen: a Mac CI
  runner; a gate in the landing queue that runs the simulator build in the
  build tree when those paths change (about ten minutes, and it needs the
  build tree free); moving the app-side Swift into the Swift package so the
  package tests compile it.
- The Android build was not re-run after the fix.
