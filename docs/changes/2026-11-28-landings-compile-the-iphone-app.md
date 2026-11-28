---
date: 2026-11-28
title: Landings compile the phone app on both platforms
class: yellow
---

What: `scripts/check-app-compiles` is a landing step. When the train changes native code the phone app compiles, or what it compiles against, it builds that platform in the build tree: under the Mac build lock it checks the train out detached, runs `AR_VARIANT=lab-one mobile/scripts/build-lab.sh ios --no-install` and/or `... android --no-install` (2400 s limit each), and checks the tree's own commit back out once, always.

- iPhone is needed for `mobile/exact-package/apple/`, `mobile/AutoradioLab/modules/apple/`, `mobile/AutoradioKit/Sources/`, and the shared paths below. It passes only on Swift's `Build complete!` with no `error:` line.
- Android is needed for `mobile/exact-package/android/`, `mobile/AutoradioLab/modules/android/`, `mobile/AutoradioLab/gradle.lockfile`, `gradle-verification.xml`, `mobile/android-model-runtime.json`, the Android model scripts (`check-android-model-runtime.py`, `build_android_model_runtime.py`, `gen_android_model_catalog.py`), `gen-exact-interface.py`, `exact-env.sh`, `exact-guard.sh`, `exact-args.sh`, `check-exact-pin.sh` and `app-identity.sh`, and the shared paths. It passes only on Gradle's `BUILD SUCCESSFUL` with no `BUILD FAILED` and no line starting `e: ` (a Kotlin error). The JDK is found by `exact-env.sh` (`JAVA_HOME`, else Homebrew openjdk@21); with no java the step fails with a message.
- Shared: `mobile/exact-package/exact.json` and `interface/`, `mobile/exact.pin`, `mobile/exact-patches/`, and `build-lab.sh`, `stage-app.sh`, `sync-exact-package.sh`, `exact.sh`.
- When both are needed, iPhone runs first, then Android even if iPhone failed, so each verdict is reported; either failure fails the step and names the platform.
- The verdict is the log's, never the exit code. It refuses, naming the dirty paths, when the build tree holds uncommitted work or is on a branch, reports (never cleans) tracked files a build leaves changed, and fails when this Mac names no build tree. `scripts/land` runs it after the gates and before the suites and the push, and its plan line names the platforms; `scripts/land-queue` stages it with `land`; `scripts/gates` runs its selftest (fake tree, fake builds). Any other train prints one line and passes. There is no way to skip it.

Why: a landing broke the iPhone build for 31 hours and no check saw it ([the record](../problems/main-broke-the-iphone-app-build-for-a-day-and-no-check-saw-it-2026-11-28.md)). Android was in the same position.

Proof: the selftest (50 checks) and the land and queue selftests pass. Not proved: the real builds have not been run through this step (builders do not run app builds). The `android --no-install` build was run by hand by the lead on 2026-11-28 (about 80 s warm, `BUILD SUCCESSFUL`); that it works under this step's locked child, and what it leaves in tracked files, is read from the scripts and not seen.

Replaced: the lead running the simulator and Android builds by hand.

Undo: revert this commit; landings go back to not compiling the app.
