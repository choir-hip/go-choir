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
| 9c4e28e38 | green | review round 2 (bunch76) findings R2-1 to R2-7 in the problem record |
| c8f129da5 | yellow | round-2 tests (failing on 7430e1f31, or killing the surviving mutants) |
| 7f375704c | green | R2-8: the live orphan rule would destroy recovery state |
| cdb7521d5 | yellow | R2-8 test |
| 77af30099 | red | round-2 fixes R2-1 to R2-5, R2-8, F8, F9 |
| 98f1c074b | green | review round 3 (bunch86) findings R3-1 to R3-7 |
| 35e44c806 | yellow | round-3 tests |
| b55cf98f3 | red | round-3 fixes: the live ephemeral rule's recovery-state keep and lifecycle claim, the kept-dir report, the young-quarantine keep |
| f93535204 | yellow | proof step 4 (the live rules, before and after the deploy) |

The design:

- **test_class_idle.** Switch `VMCTL_RETENTION_TEST_IDLE_MODE` (off,
  dry-run or active; default off) and `_MIN_AGE` (default 168h). The rule
  takes a computer only if all of these hold:
  - its warmness class is test, read from the live tiers. A computer is
    test class when its account is a test account (the ephemeral
    domains/prefixes) or when computer-tiers.json's new optional
    `test_computers` list names it. Platform and paid computers keep their
    own class even if listed;
  - it is the interactive computer on the account's primary desktop;
  - it is stopped, hibernated or failed, and not stopped by
    `recover_current`;
  - it is not held;
  - it has been idle for at least N (a zero `LastActiveAt` is not idle);
  - its dir holds no unfinished, unreadable or journal-less quarantine,
    no staging image and no unfinished recovery journal;
  - no per-computer selector names it by VM id or computer id: any
    `VM_*_ONLY` or `VMCTL_*_ONLY` (`VMCTL_JAILER_ONLY`), `CHOIR_*_COMPUTERS`
    (`CHOIR_CLAUDE_DELEGATE_COMPUTERS`, `CHOIR_VISUAL_CAPTURE_COMPUTERS`,
    `CHOIR_CAPSULE_MIRROR_COMPUTERS`) or `CHOIR_*_ONLY`
    (`CHOIR_CAPSULE_MIRRORS_ONLY`).

  Removal is `destroyRetentionCandidate`'s path: `DestroyVMState`, then
  the ownership is dropped. It runs under a new lifecycle claim (a visit
  is refused while the state goes), and the rule is re-checked at destroy
  time.
- **quarantine_age_out.** Switch `VMCTL_RETENTION_QUARANTINE_MODE`
  (default off) and `_MIN_AGE` (default 168h). The rule removes a
  `data.img.quarantine-*` only if all of these hold:
  - it is in a `vm-*` or `candidate-*` dir that has an ownership record
    (ownerless: `no_ownership`; other dirs are never scanned);
  - the owner is test class (a real account's `done` quarantine may still
    be the only copy: the 10-09 484-path record);
  - its recovery journal is finished: phase `done` or status `active`, not
    `failed` (`swapped` and `booted` are kept);
  - the journal names the owner's own computer, and that computer is not
    the paper or any platform computer, not a paid account's computer, and
    not any protected ownership's computer;
  - its age, read from the rename's ctime on Linux, is at least N (off
    Linux the age is zero, so nothing ages out);
  - its owner is not held, not recovering and not selector-named.

  The candidate-fleet 10-07 pair is kept by these checks whether or not
  its dir still resolves to the owner.
- **The live orphan rule (R2-8).** `orphan_state_dir`, active on Node B
  today, no longer plans or destroys a dir holding a quarantine, a staging
  image or a recovery journal.
- **Removal gates and bounds.** Active removal needs both the retention
  mode and the rule's switch set to active. Removals share the pass's
  `MAX_DELETES` and `MAX_BYTES`. Both minimum ages have a 24 h floor.
- **Manifest.** `<state dir>/retention-manifest.jsonl` gets a `removing`
  line before each unlink or destroy, then a `removed` (or `failed`) line.
  Each line records what was removed, when it was quarantined or last
  active, why, and its size. If the intent line cannot be written, the
  removal does not happen.
- **The live ephemeral rule (R3-3).** `ephemeral_test_primary`, active on
  Node B, keeps a dir that holds unfinished, unreadable, failed or
  journal-less recovery state, a staging image, an unfinished journal, or
  a finished quarantine younger than the quarantine minimum age. At
  destroy time it re-reads everything under the registry lock and claims
  the computer's lifecycle.
- **Kept dirs (R3-2, R3-5).** The active plan's `kept_state_dirs` lists
  every dir the orphan or ephemeral rule keeps, with its rule, reasons,
  bytes and release. An orphan dir is released only by an owner act: read
  it, then delete it or restore its ownership. A `rec-*.journal` is never
  deleted, so an orphan dir that ever cold-recovered stays until then. An
  ephemeral dir is released when its recovery finishes and the quarantine
  ages past the minimum, or by an owner act.
- **test_class_idle (R3-6)** also keeps a dir with a finished quarantine
  younger than the quarantine minimum age.
- **Report.** While a rule is not off, the plan carries `test_class[]` and
  `quarantines[]` (every entry, with verdict `keep`, `would_remove` or
  `will_remove`, plus reasons and bytes) and inventory totals. The shadow
  plan forces both rules to dry-run.

Deploy shape: **not inert since round 2, but it only removes deletions.**
The two new rules are as before: the active switches stay unset,
`test_computers` is empty, and node-b.nix sets only the GET-only shadow
plan's rule modes to dry-run. The plan's config block gains the new keys
(functionally, not byte-, identical). The running-path changes are in two live rules, and both only delete
less:
- `orphan_state_dir` (R2-8) keeps a dir that holds recovery state;
- `ephemeral_test_primary` (R3-3) keeps one too, and its removal now also
  refuses a concurrent visit, resume or refresh.

The batch is a live red batch, with its own deployed proof: step 4,
below.

Tests:
- On Linux, `go test ./internal/vmctl ./internal/computertiers ./cmd/vmctl`
  passes, and the retention tests pass under `-race`.
- Off Linux, the tests that need a quarantine to age out skip with the
  reason (no ctime there: age zero, fail closed). Everything else runs.
  The round-3 reviewers built and ran the package on darwin/arm64; this
  host cannot (Dolt cgo does not cross-compile). A source test forbids
  Linux-only stat fields outside `_linux.go` files.
- Mutants each compile and are killed:
  - round 2: M07, M08, M09, M10, M12, M29 and M33;
  - round 3: the orphan destroy-time recheck deleted, the ephemeral
    destroy-time dir recheck deleted, the ephemeral claim not taken, and
    its conflict check skipped.
- `scripts/proofs/selftest.sh` reports 149 passed and 1 failed; the
  failure is the pre-existing `fp1 step 1 computes a busy share`, not
  this lane's (round 3 said 132 passed; it was 131 and the same failure).

## Deployed proof (FP0)

0. **Before the deploy**, run
   `scripts/proofs/fp0-vm-retention.sh --step 4 --evidence before.txt`. It
   lists every `vm-*` or `candidate-*` dir on Node B holding a quarantine,
   a staging image or a recovery journal (`ls`, `du`), and checks that
   none is a candidate in the active plan.
   Reading step 4 (review round 4, R4-1, R4-4):
   - a FAIL that says the state dir or the active plan was not read, or
     the plan does not parse, compared nothing: fix the read and run the
     step again before deploying;
   - a MANUAL "nothing to check" line is not a pass: no dir holds
     recovery state, or the plan has no candidates. Record it as such;
     on Node B the candidate fleet's 10-07 quarantines should make the
     first one impossible, so treat it as a wrong state dir first;
   - a FAIL naming candidates before the deploy means today's binary will
     destroy a dir holding recovery state at its next sweep (the old
     ephemeral rule has no keep). Record the dirs in the problem record
     and deploy this batch at once; if it cannot go at once, the owner
     decides whether to copy the named dirs aside first (a Node B
     mutation, not a step of this script).
1. Deploy it as a live red batch (its own deploy, or with inert ones). Run
   `--step 4 --evidence after.txt`. Expect:
   - the same recovery-dir lines as before (none gone, the candidate-fleet
     10-07 quarantines included);
   - every ownerless one is a `kept ... rule=orphan_state_dir` line;
   - no recovery dir is a candidate.

   Then run
   `scripts/proofs/fp0-vm-retention.sh --commit <sha> --evidence <file>`
   (steps 0 to 2, all read-only). Expect:
   - shadow `test_idle_mode` and `quarantine_mode` are dry-run;
   - every test-class computer and every quarantine is listed with its
     verdict and reasons;
   - the paper, the owner's computer and the delegate are never
     `would_remove`;
   - the active plan's switches are off, it has no `test_class_idle`
     candidate, and there is no manifest.

   Also check the paper is healthy and polling, vmctl's boot log is clean,
   and vmctl's journal shows no retention prune of a recovery dir.
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
- **The two 2026-10-07 quarantines** are now always `keep`: the owner's
  account is paid, so they never age out, and if their dir loses its
  ownership they are `no_ownership`. Deleting them is a manual,
  owner-approved act (black-adjacent: they may be the only copy, and the
  484-path check of 10-09 never closed).
- **Ownerless dirs holding only a plain `data.img`** (a computer that lost
  its ownership and never cold-recovered) are still removed by the live
  orphan rule after 6 h. That has been the rule's purpose since before
  this lane, but a registry reset or a moved realization (R2-8's premise)
  would reach them too. Keep the rule as is, or keep every ownerless dir
  holding a `data.img` until an owner act? Owner's call (DeepSeek round 3).
- **Releasing a kept orphan dir** is an owner act (read it, then delete it
  or restore its ownership); vmctl never releases one.
- **Real accounts' quarantines** never age out under this rule (R2-3).
  Reclaiming them needs either a positive completeness signal (a verified
  file-root hydration) or an owner act.

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
- These round-2 findings are kept as residuals:
  - the ephemeral rule ignores selectors and runs first (pre-existing);
  - the report's verdicts are not bounded by `MAX_DELETES` or `MAX_BYTES`;
  - the manifest lives on the filesystem it frees, and its new directory
    entry is not fsynced;
  - concurrent passes can write a false `failed` line for an
    already-removed quarantine, and a quarantine unlink can race a cold
    recovery's own count prune on the same VM (Sonnet F7);
  - the newest rollback quarantine of a test computer can age out
    (DeepSeek 5).

```yaml
now:
  status: waiting_on_fp0
  slice: review round 4 fixed (R4-1, step 4 can no longer pass vacuously); the deploy and its proof (step 4 before and after, then steps 0 to 2) are FP0's
  next_action: FP0 re-reviews the round-4 delta (2ffb6c3ad..head), runs step 4 on Node B, lands it as a live red batch (orphan and ephemeral rules delete less), runs step 4 again and steps 0 to 2
  blocker_or_risk: enforce waits on the shadow report and the owner questions above
```
