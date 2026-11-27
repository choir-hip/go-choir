# Node B refuses deploys again: root disk below the 60 GiB floor (2026-11-27)

Standing: governing | kind: happened | settled-by: CI run 45988510694 and a metadata read of Node B at 2026-11-27T11:30Z | note: third record of this refusal; the earlier ones are node-b-deploy-refused-disk-headroom-2026-11-11.md and node-b-storage-clustering-assessment-2026-11-11.md

## Problem

The staging deploy of `cc3fe4db4` (the batched landing queue, scripts only)
was refused by `scripts/node-b-deploy-disk-preflight`: "root filesystem still
below required headroom (62914560 KiB)". CI run 45988510694; every test job
passed and only "Deploy to Staging (Node B)" failed, so the run's conclusion
is failure. Staging stays on `4ac73d992`. No deploy can land until free space
is back above the floor, including a fix the paper or the app might need.

The refusal is correct. The problem is the disk, and it is the same problem
as on 2026-11-11, with the same two owners of the space, both larger.

## Evidence (read 2026-11-27 about 11:30Z, sizes only, nothing changed)

- `/` is 476 GiB, 427 GiB used, 42 GiB free (92%). The preflight read 45.6
  GiB free, ran its reclaim (vmctl reclaim: nothing eligible; nix retention
  and `nix store gc`: 290 MiB; journal vacuum: 0) and stopped at 45.5 GiB.
- `/var/lib/go-choir`: `platform-artifacts` 221 GiB (`tape-seg` 120,
  `sha256` 54, `capture-bodies` 45), `vm-state` 155 GiB, `guest-releases` 19.
  `/nix/store` is 36 GiB.
- One computer holds 216 GiB of it: `tape-seg/computer-1355b83b…` is 97 GiB
  (83 GiB for all computers on 2026-11-11) and `vm-state/vm-1669020a…/data.img`
  is 119 GiB (45 GiB for the paper's computer on 2026-11-11). Whether these
  two are the same computer was not checked; the sizes say the paper.
- `capture-bodies` went from 19 to 45 GiB in sixteen days.
- Two `candidate-fleet-*` directories under `vm-state` hold 10.8 GiB.

## Belief

The paper's tape and data image grow without a bound that the disk can
hold, and the reclaim the preflight runs cannot touch either. The floor was
crossed on 2026-11-11 and again now; between them the disk was not made
larger and the growth was not stopped, so it will be crossed again after any
one-time cleanup. This is three records in one subsystem; the clustering
assessment of 2026-11-11 already exists and its remedies are the storage
mission's stations (M3), several of which are stalled lanes
(`snapshot-capture-s3`, `restore-boot-install`; see
docs/evidence/half-started-work-2026-11-27.md).

## What it blocks

Every staging deploy. A landing whose CI tests pass still ends with a failed
run, and `scripts/land-queue` (correctly) will not push on top of a main
whose newest code push has a red run, so the batched queue cannot be proved
on a real batch (ledger claim C257) until a deploy succeeds.

## Not done, and why

Nothing was deleted. Freeing space here means removing production data
(tape segments, a computer's data image, capture bodies) or lowering a
safety floor, and both are the owner's to decide.

## Options for the owner

1. Free at least 20 GiB once: name what may go (the two candidate-fleet
   directories if they are dead, old guest releases, capture bodies past
   an age). Gets deploys back today; does not stop the growth.
2. Lower the deploy floor from 60 GiB for a stated time. Gets deploys back
   at once; spends the margin the floor exists to keep.
3. Move the paper's data image or sealed tape segments off Node B (Node A
   has the backup target). The durable fix; it is the storage mission's work.
4. Finish the stalled storage stations first and let them reclaim.

## Remaining error

Which files inside the 97 GiB tape directory and the 119 GiB image are live
was not read. The growth rate per day was not measured, only two points.
