# Node B refuses deploys: root disk below the 60 GiB floor (2026-11-11)

## Problem

The staging deploy of `29e01354f` (the Autoradio app, files under `mobile/`
only) was refused by `scripts/node-b-deploy-disk-preflight`: "root filesystem
still below required headroom (62914560 KiB)". CI run 43006686777; every test
job passed and only "Deploy to Staging (Node B)" failed. Staging stays on
`3df4e78c7`. Until free space is back above the floor, no deploy can land,
including a fix the paper might need.

The refusal is correct behaviour. The problem is the disk.

## Evidence (read 2026-11-11 about 11:30Z, metadata only)

- `/` is 476 GiB, 381 GiB used, 50 to 52 GiB free (89%). btrfs unallocated
  20 GiB.
- The preflight ran its bounded reclaim (vmctl reclaim, journal vacuum, nix
  retention, `nix store gc`) and was still below the floor.
- `go-choir-host-disk-alarm` has warned "free below 60 GiB (the deploy floor)"
  since about 05:00Z. Samples over the day before: 62 (23:30Z), 69, 67, 56
  (about 05:00Z), 51, 56, 50 (11:15Z). It moves by several GiB in both
  directions within hours.
- `/var/lib/go-choir` holds 350 GiB: `platform-artifacts` 173 (`tape-seg` 83,
  `sha256` 70, `capture-bodies` 19), `vm-state` 153 (the paper's computer 45,
  d@e 26, `candidate-fleet-bf04…` 20, then 11, 9, 8), `guest-releases` 13,
  `drill-scratch` 5. `/nix/store` is 36.
- The artifact collector is running and deleting (`artifact-gc-log`, last
  entries 11:22Z).
- The last successful deploy was 04:31Z (`3df4e78c7`), the tenth of the night.

## Belief

The host sits within about 10 GiB of the floor in normal operation, so a
normal swing crosses it. I have not found one new large consumer. Likely
contributors, none measured: the night's ten deploys (prebuilt artifacts and
guest releases), the paper's own growth (its computer is the largest at
45 GiB and the tape segments are 83), and d@e's state after the night's
real-data proofs.

## Remaining error

- No growth rate per directory: I have one reading, not two.
- Whether `candidate-fleet-bf04…` (20 GiB) and `drill-scratch` (5 GiB) are
  live or leftovers is unknown.
- Whether tape segment and capture-body retention is working as designed is
  unchecked.

## Not done, on purpose

Nothing was deleted or changed on Node B. Removing state there is a
destructive act in production and belongs to the owner. The existing policy is
`docs/problems/node-b-nix-store-and-host-disk-policy-2026-10-22.md`.

## Next

1. Measure growth per directory over a few hours (read-only).
2. Owner decides what may go: stale candidate fleets, drill scratch, old
   dumps and backups under `/var/lib/go-choir`, or a larger disk.
3. Re-run the deploy job for `29e01354f` (or let the next landing carry it).
