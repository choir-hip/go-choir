# The updater prune did not converge in three review rounds: structural assessment

Recorded 2026-11-14T04:20Z. Candidate: `lane/updater-prune` at `03c956e31`.
It does not land (owner ruling N1 in the part 3 goal file). Problem it was
solving: [updater-directory-fills-the-data-volume-2026-11-13.md](updater-directory-fills-the-data-volume-2026-11-13.md).

## What happened

| Round | Gemini 4 Hydrogen | DeepSeek v4.1 flash | Blocking defects |
| --- | --- | --- | --- |
| 1 | REPAIR | REPAIR | the restore check was a probe, not a hold; bundles of kept releases deleted; a failed hold proceeded; an unstaged payload never pruned |
| 2 | REPAIR | REPAIR | a swap-back's journal shadowed the real predecessor; the hold test passed on a probe; boot waited on the lock with no deadline |
| 3 | REPAIR | ACCEPT | the bounded hold polls every 25 ms and starves against the chunk loop, which frees the lock for microseconds; a long prune makes boot or a restore refuse |

Seven of the eight blocking findings are about one thing: two processes
(the updater daemon that prunes, the runtime that restores and swaps
`current`) share one directory, and the prune runs while the other is live.
Each round's repair to that coordination (probe, hold, chunks, deadline)
created the next round's defect. The eighth (which releases to keep) was
closed in round 3 and both seats verified it: at most three releases,
bounded under repeated restage and apply.

## Belief

The keep rule is sound. The concurrency is the defect, and it is in the
frame, not in the patch: a prune that runs beside a live runtime needs a
cross-process protocol, and an advisory lock with chunks is a weak one. A
fourth patch (a pause between chunks, a wait-intent file) would be a fourth
protocol revision on the same frame.

## Changed approach

Prune when nothing else is running. The guest already has boot units ordered
before both `go-choir-updater.service` and `go-choir-autoputer.service`
(`nix/autoputer-vm.nix`). The prune becomes one more: a oneshot that runs to
completion, or to a time bound, before either starts.

- One process touches the updater root while it runs. No restore hold, no
  chunks, no deadline, no lock order. `restore_hold.go`, the chunk loop and
  the `lockCurrentRelease` changes are deleted, not repaired.
- Every release reaches a computer by a planned refresh, which is a boot.
  So the prune runs once per release, which is the rate at which the
  directory grows.
- It keeps what the candidate got right: the union keep rule, rename then
  delete, skip entirely while an apply is unsettled, the ten-minute window,
  the runtime's bundle collection failing closed.
- Its bound is time: stop at a fixed boot budget and finish at the next
  boot. A unit that fails or times out must not block the boot.

What it gives up: space is reclaimed at the next boot, not minutes after an
apply. A computer that never restarts never prunes; every computer restarts
at each deploy.

Remaining error: whether anything else can write the updater root that
early (a host-side agent, the recovery path units), whether the staged
source deletion at apply (inside the updater, under its own mutex) is
independent of the hold code, and how long deleting d@e's 20 GiB backlog
takes against a boot budget. The builder verifies these before writing code.

Residuals both seats named that the new candidate must also answer: a dead
journal from a crash mid-apply pins two releases for good; a leading slash
in the layering entrypoint silently disables store collection.
