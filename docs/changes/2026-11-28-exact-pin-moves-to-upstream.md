---
date: 2026-11-28
title: The Exact pin moves to upstream 8552d20f9
class: yellow
---

What: `mobile/exact.pin` moves from f53347503 to 8552d20f9 (upstream main, 2026-11-26). Patches 0002 and 0003 are re-made for it and 0001 is unchanged. One spelling changed in the contracts (`swipeleft`). The Android Gradle lock file and verification file are rewritten for the new dependency graph (`androidx.window` 1.5.0). `setup-exact.sh` now installs the std sources, llvm-tools and wasm target of the dated nightly that Exact's web build names.

Why: the owner, 2026-11-27: "we should plan to update to upstream before the cutover finishes".

Proof: in the build tree on 2026-11-28 at the new pin: `setup-exact.sh` and `check-exact-pin.sh` pass with all three patches applied; `contract build app.contract` passes; `bun test` 1,841 pass, 0 fail; `test web` 35 pass, 2 fail, the same two voice-list tests that fail on main; the iOS app compiles ("Build complete"); the Android lab-one app builds. Not proved: nothing was driven on a simulator or a phone. The iOS command ended with an error after compiling, because at this pin `ios` also installs into a booted simulator and the one booted refused the install. The Android build installed the lab-one development app on the connected phone, which was not asked for.

Replaced: the f53347503 pin and its patches 0002 and 0003.

Undo: revert this commit, then in the build tree take the patches off `.exact-src` by hand (`git -C mobile/.exact-src apply -R` of each of this commit's patches) and run `setup-exact.sh`. The script takes off only patches that match the tree it is run from, so a pin move that also re-makes a patch needs that step in both directions.
