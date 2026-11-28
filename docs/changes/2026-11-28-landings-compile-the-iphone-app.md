---
date: 2026-11-28
title: Landings compile the iPhone app
class: yellow
---

What: `scripts/check-app-compiles` is a landing step. When the train changes `mobile/exact-package/apple/`, `mobile/AutoradioLab/modules/apple/`, `mobile/AutoradioKit/Sources/`, `mobile/exact-package/exact.json` or `interface/`, `mobile/exact.pin`, `mobile/exact-patches/`, or `build-lab.sh`, `stage-app.sh`, `sync-exact-package.sh` or `exact.sh`, it compiles the app in the build tree: under the Mac build lock it checks the train out detached, runs `AR_VARIANT=lab-one mobile/scripts/build-lab.sh ios --no-install` (2400 s limit), and always checks the tree's own commit back out. It passes only on Swift's `Build complete!` with no `error:` line in the log; the exit code is not read. It refuses, naming the dirty paths, when the build tree holds uncommitted work or is on a branch, and it fails when this Mac names no build tree. `scripts/land` runs it after the gates and before the suites and the push; `scripts/land-queue` stages it with `land`; `scripts/gates` runs its selftest (fake tree, fake build: 28 checks). Any other train prints one line and passes. There is no way to skip it.

Why: a landing broke the iPhone build for 31 hours and no check saw it ([the record](../problems/main-broke-the-iphone-app-build-for-a-day-and-no-check-saw-it-2026-11-28.md)).

Proof: the selftest and the land and queue selftests pass. Not proved: the real build has not been run through this step (builders do not run app builds), so `--no-install` on the `ios` verb, the build's byproducts in tracked files and the time it takes are read from Exact's source and not seen.

Replaced: the lead running the simulator build by hand.

Undo: revert this commit; landings go back to not compiling the app.
