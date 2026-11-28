# The app's real-data proof has not been able to sign in since the key moved to shared code

Standing: governing | kind: happened | settled-by: a run of mobile/scripts/proof-real.sh on origin/main bb10f2d21 in a slim simulator against staging, 2026-11-28, and a reading of 5721a63cd | note: whether anyone ran the proof between 2026-11-23 and this run is not known

## Problem

`mobile/scripts/proof-real.sh` is the app's deployed proof: the iPhone app in
a simulator, signed in to the test computer on staging. It signs in by copying
a key file into the app's Documents. `ProofKey.swift` (simulator only) reads
that file into the native Keychain at launch.

Since `5721a63cd` (2026-11-23) the account key lives in Exact's secret store
and shared code is its only reader (`api.ts`, `app-signin.ts`). Nothing reads
the native Keychain item any more. So the proof's key is imported and never
used. The app stays signed out for the whole proof.

## Evidence

- The run on 2026-11-28: every drive's tree holds "You are signed out"; the
  account page reads "Signed in: No"; the console says
  `api: session found=false key=false computer=false` at each launch.
- The proof still got through eleven drives (warm, survey, system dark,
  migrate, takes, imported, pick and read, reader three times, tour) because
  the paper and the radio work signed out. It failed at the edition picks
  drive: the signed-out banner covers the "Culture" row, so the tap is refused.
- `ProofKey.importIfPresent` writes `KeyAccount.apiKey` in the native Keychain
  (`AutoradioModule.swift:129`, `ModelsEntry.swift:38`). `api.ts:117-118` reads
  the key from the secret store only.

## Belief

Every signed-in check in the proof has been unreachable for five days: Files,
Mail, the account page, sign-out and its wipe, the notebook, takes for a
signed-in listener. Landings since then that touched those were proved by
tests and review only. That includes the sign-out order (`4f5e66020`) and the
one native reader of "signed in" (`48dde9e32`).

What the run did show, signed out, on main `bb10f2d21`: the app builds,
installs and launches; the paper loads real stories; a story opens in the
reader; the radio plays and the bar shows the entry on air.

## Correction (2026-11-28)

The Keychain item is the same item in a normal launch, but an agent drive does not read the Keychain: its secret store is memory or files in the drive's scratch tree (`EXACT_STORE=real` is needed for the Keychain, and `exact-env.sh` drops it). See [the plan](../design/proof-sign-in-after-the-key-moved-2026-11-28.md). Also: shared sign-out now revokes the held key on the server, so a signed-in drill would revoke the QA key.

## Remaining error

Built 2026-11-28 (scripts only, not yet run against staging; see the change entry `2026-11-28-the-real-data-proof-signs-in-again`):

- The proof signs in by seeding a disposable key into Exact's scratch secret store (`proof-key.sh`). Not proved until a run: that Exact reads the hand-placed files, that the QA key may mint keys (a refusal stops the proof with `PROOF FAILED at sign-in: the test key may not mint keys`), and the scratch path.
- Blindness: the proof now asserts the app is signed in right after the warm-up drive, and that the signed-out drives are signed out. Other checks that pass vacuously signed out (the drill's "no rows after sign-out" when no rows were ever shown) are not yet given a positive twin.
- The signed-out banner covering the "Culture" row: the edition drive now runs signed in, with no banner, so the proof no longer meets it. Whether it matters to a signed-out person is still not checked (the plan, 7a, finds it by design).
- `sim-device.sh` now picks the newest runtime that supports the device type and fails naming the type; tested over fake `simctl`.
- `ProofKey.swift` and its two call sites remain (Swift; owner decision D6).
