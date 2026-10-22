---
definition_version: 4
readiness: executable
# Authority to execute: the launch first pass metamission
# (docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md), FP0's
# disk work after deploy R was refused by the disk floor (2026-10-22).
---

# FP0 lane: VM-state retention (test computers and quarantined data images)

Mutation class: red (VM lifecycle: vmctl deletes VM state). Lane branch
`lane/fp0-vm-retention`. Problem record:
[docs/problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md](../problems/vm-state-retention-skips-test-computers-and-quarantines-2026-10-22.md).

## Value criterion

Node B's VM state stops holding disk that nothing needs: about 40 GiB of
test-computer state and 32.4 GiB of quarantined images (FP0's 10-22
reading). This must never touch the paper, the owner's computer, a real
account's computer, or a computer a staged feature selects. Before anything
is deleted, FP0 can read what would go and why the rest stays.

## What landed on the lane

| Commit | Class | What |
| --- | --- | --- |
| 16de7f6f7 | green | problem record |
| 03e212674 | yellow | tests T1 to T10 (internal/vmctl/retention_test_class_test.go, computertiers) |
| de3f616e6 | red | the two rules, the report, the manifest, the env wiring, node-b.nix shadow observe |
| fc7bdb950 | yellow | scripts/proofs/fp0-vm-retention.sh and its selftest |

The design:

- **test_class_idle.** Switch `VMCTL_RETENTION_TEST_IDLE_MODE` (off,
  dry-run or active; default off) and `_MIN_AGE` (default 168h). The rule
  takes a computer only if all of these hold:
  - its warmness class is test, read from the live tiers. A computer is
    test class when its account is a test account (the ephemeral
    domains/prefixes) or when computer-tiers.json's new optional
    `test_computers` list names it. Platform and paid computers keep their
    own class even if listed;
  - it is stopped, hibernated or failed;
  - it is not held;
  - it has been idle for at least N;
  - no `VM_*_ONLY` selector names it by VM id or computer id.

  Removal is `destroyRetentionCandidate`'s path: `DestroyVMState`, then
  the ownership is dropped. It runs under a new lifecycle claim (a visit
  is refused while the state goes), and the rule is re-checked at destroy
  time.
- **quarantine_age_out.** Switch `VMCTL_RETENTION_QUARANTINE_MODE`
  (default off) and `_MIN_AGE` (default 168h). The rule removes a
  `data.img.quarantine-*` only if all of these hold:
  - its recovery journal is `done` or its status is `active`;
  - its age, read from the rename's ctime, is at least N;
  - its computer is not protected, not held and not selector-named.

  It keeps pre-swap, failed and journal-less quarantines: each may be the
  only copy of the computer's data.
- **Removal gates and bounds.** Active removal needs both the retention
  mode and the rule's switch set to active. Removals share the pass's
  `MAX_DELETES` and `MAX_BYTES`.
- **Manifest.** `<state dir>/retention-manifest.jsonl` gets a `removing`
  line before each unlink or destroy, then a `removed` (or `failed`) line.
  Each line records what was removed, when it was quarantined or last
  active, why, and its size. If the intent line cannot be written, the
  removal does not happen.
- **Report.** While a rule is not off, the plan carries `test_class[]` and
  `quarantines[]` (every entry, with verdict `keep`, `would_remove` or
  `will_remove`, plus reasons and bytes) and inventory totals. The shadow
  plan forces both rules to dry-run.

Deploy shape: **inert**. The active switches stay unset, and
`test_computers` is empty. node-b.nix sets the two `VMCTL_RETENTION_SHADOW_*`
rule modes to dry-run, which only changes the GET-only shadow plan. With
both switches off, the active plan is unchanged (T10).

Tests: `go test ./internal/vmctl ./internal/computertiers ./cmd/vmctl`
passes, and the new tests pass under `-race`. `scripts/proofs/selftest.sh`
reports 128 passed.

## Deployed proof (FP0)

1. Inert deploy (rides with other inert batches): run
   `scripts/proofs/fp0-vm-retention.sh --commit <sha> --evidence <file>`
   (steps 0 to 2, all read-only). Expect:
   - shadow `test_idle_mode` and `quarantine_mode` are dry-run;
   - every test-class computer and every quarantine is listed with its
     verdict and reasons;
   - the paper, the owner's computer and the delegate are never
     `would_remove`;
   - the active plan's switches are off, it has no `test_class_idle`
     candidate, and there is no manifest.

   The no-effect proof is the usual one: the paper is healthy and polling,
   and vmctl's boot log is clean.
2. Record in the problem record the kept reasons of the five test
   computers and of the two 2026-10-07 quarantines. This step confirms or
   refutes hypotheses H1 to H3.
3. Enforce is a later deploy, a reviewed node-b.nix change to
   `VMCTL_RETENTION_QUARANTINE_MODE=active` and/or
   `VMCTL_RETENTION_TEST_IDLE_MODE=active` (`_MIN_AGE=168h`). If a
   registered account's computer should go, it first needs a
   computer-tiers.json `test_computers` entry with a note. After that
   deploy, run `--step 3`, which reads the manifest and journal lines, and
   take `du` of vm-state before and after.

## Owner and FP0 questions

- **"Never an account's primary computer"** is read here as: never the
  primary computer of a non-test account. Every test computer is its test
  account's primary computer, so a literal reading would exclude all five.
  Confirm this reading.
- **Listing FP0's registered test accounts' computers** in `test_computers`
  changes their warmness class to test. That makes them first under
  pressure reclaim, matches `VM_MEMORY_HOTPLUG_ONLY=class:test`, and gives
  them the visual capture grant. It is a reviewed commit, the owner's or
  FP0's call.
- **The two 2026-10-07 quarantines** stay `keep` if their journals are
  missing or pre-swap. Deleting them then is a manual, owner-approved act
  (black-adjacent: they may be the only copy).

## Residuals

- `PruneRecoveryQuarantines` (vmmanager) returns at the first VM dir whose
  prune errors, so one stuck dir stops the count prune for every later dir.
  It is recorded and not repaired here.
- The plan's sizes are allocated blocks. On btrfs, shared extents may free
  less (as the 10-22 backup deletion showed).
- `LastActiveAt` is reset by every lifecycle transition, so "idle" means
  "no transition for N days" and not "no owner use for N days".
- An active test computer that is stuck busy is never taken. That is
  pressure reclaim's job.

```yaml
now:
  status: waiting_on_fp0
  slice: lane complete; the inert deploy and its read-only proof are FP0's
  next_action: FP0 rebases, reviews and lands de3f616e6 with fc7bdb950 as an inert red batch, then runs scripts/proofs/fp0-vm-retention.sh
  blocker_or_risk: none for the inert deploy; enforce waits on the shadow report and the owner questions above
```
