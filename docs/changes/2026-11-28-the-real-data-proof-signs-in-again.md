---
date: 2026-11-28
title: the real-data proof signs in again, with a disposable key
class: yellow
---
What: `mobile/scripts/proof-key.sh` is new, and `proof-real.sh` uses it. The proof mints a disposable key with the QA key (`POST /auth/api-keys`, three hours), writes it as `choir.key` and its id as `choir.key-id` in Exact's scratch secret store in the simulator's container (files 0600, directory 0700), and removes the stamp, the sign-out marker, the pending-revoke list and any computer id so the next launch does not wipe or sign out. Right after the warm-up drive it asserts the account page says Signed in and the console says `api: session found=true key=true`, else `PROOF FAILED at sign-in: the app is signed out`. A refused mint stops with `PROOF FAILED at sign-in: the test key may not mint keys (<status>)`. The signed-out drives clear the seed and assert they are signed out. An exit trap revokes the minted key by id and says loudly, with the id only, if it cannot. The key is never on a command line (curl reads headers and body from files), in a variable, in a log or in an `sh -x` trace. The `proof-key` copy into Documents is gone. `sim-device.sh` now picks the newest runtime that supports the iPhone 18 Pro from `simctl list -j runtimes` and fails naming the device type when none does.

Why: since 5721a63cd the key lives in shared code, which in an agent drive reads Exact's scratch store and not the Keychain, so the proof ran signed out for five days and nothing noticed. The QA key must not be seeded itself: the sign-out drill revokes the key the app holds. Plan: docs/design/proof-sign-in-after-the-key-moved-2026-11-28.md, option A.

Proof: not proved: the lead runs it. `sh mobile/scripts/test-proof-helpers.sh` passes over fake `curl` and `simctl` (written first and seen failing): the seed holds exactly the two files with the right bytes and modes, a refused mint seeds nothing, no key in output, argv or a trace, the trap revokes by id, a failed revoke names the id only, the assertions fail on the wrong tree, `sim-device.sh` picks 27.0 and fails clearly. Not shown by anything but a real run: that Exact reads the placed files, that the QA key holds `manage:keys`, and the scratch path.

Replaced: the `proof-key` file in Documents and the native import it fed (`ProofKey.swift` stays, unused by a drive, until owner decision D6).

Undo: revert the commit. The QA key is untouched; the proof only ever holds a disposable one.
