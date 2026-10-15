# Retention prune would destroy the paper's disk after a 24-hour stop

Found 2026-10-16 about 00:05 UTC, during the overnight disk survey that
followed owner §24 ("free as much disk as possible, build mechanistic
pruning").

## Evidence

The evidence was read on Node B with vmctl at 4592c7b63.

- vmctl runs retention prune in `active` mode:
  - `VMCTL_RETENTION_PRUNE_MODE=active`;
  - ephemeral email domains `example.com,example.test`;
  - `EPHEMERAL_MIN_AGE=24h`;
  - `MAX_BYTES_MIB=122880`.
- The journal at 23:20:20 shows it working: "retention prune deleted 1 VM
  state directorie(s), reclaimed 3753.5 MiB".
- The paper computer's ownership (vm-1669020a…) belongs to an
  **example.com** account. The retention policy therefore classifies the
  paper as ephemeral. The classification was recomputed read-only from
  auth.db, with domains only and no addresses printed.
- vmctl `/health` warmness shows the paper as `critical_protected` (the
  one platform computer, from internal/computertiers).
- `retentionOwnershipReclaimable` (internal/vmctl/retention_prune.go)
  checks hold, ephemeral account, primary desktop, state
  (stopped/hibernated/failed) and age. It **never consults the warmness
  class.** Pressure reclaim does (pressure_reclaim.go `critical_protected`),
  and so does deploy refresh. Retention does not.

## Consequence

If the paper is stopped, hibernated or failed for 24 hours, the next prune
sweep calls `DestroyVMState` on it and removes its ownership. That 24 hours
could come from a crash with no recovery, an owner stop, or a failed state
during an outage like the disk-full outage of 2026-10-15. The loss would
include the paper's tape, its texture DB and every published edition's
local state. Nothing is lost today, because the paper is active. The
hazard is latent and armed.

## Secondary findings

- `retentionOrphanCandidates` never increments `stateDirCount`. The plan
  therefore reports `state_dirs: 0` while 32 directories exist. That is a
  reporting defect only.
- Most of vm-state (153 GB across 32 dirs) is **not** a leak. 17 of the
  stopped vm-* dirs are example.com test or tracer computers less than 24
  hours old, and the existing pruner will reclaim them as they age. The
  long-lived non-ephemeral dirs are:
  - the paper (48 GB);
  - an active non-test candidate-fleet computer (40 GB);
  - the owner's stopped computer (3 GB);
  - two tiny July candidates with no account (under 0.3 GB).

## Fix direction

Retention must refuse any ownership whose warmness class is protected
(`warmnessClassProtected`: critical_protected, public_platform,
premium_always_on). The check belongs in the planner and again at destroy
time, the same double check the ephemeral rule already gets. Retention
deletes; it is never the authority on whether a platform computer is
disposable. Also count state dirs truthfully.

Failure modes the test must cover:

- **R1:** a protected computer on an ephemeral account, stopped longer
  than the minimum age, is planned or destroyed.
- **R2:** the protection is checked in the plan but not at destroy time,
  so a class change between plan and destroy slips through.
- **R3:** an unprotected ephemeral computer is no longer pruned (the guard
  over-reaches).

## Mutation class

Red (vmctl lifecycle, destructive path). The change only **removes**
deletions; it adds none. Rollback is a git revert of the fix commit.
