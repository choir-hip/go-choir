# Two race-shard CI flakes on main: a 5 s pace in the Texture owner-recovery test, and a lost update in vmctl recovery admission (2026-10-26)

Found 04:12 and 04:26 UTC in GitHub CI on main (race shards), one failure
each, both passed on re-run. Investigation only: read-only on the code,
plus throwaway probes on the agents VM (nothing committed but this record).
Mutation class of this record: green. The two fixes would be yellow (test
deadline) and red (vmctl recovery admission: a protected surface, but the
change is small, additive and one-commit revertible).

## Summary

| | Flake 1: actorruntime | Flake 2: vmctl |
|---|---|---|
| Test | `TestAdapterStartSerializesTextureOwnerRecoveryBeforeActorDelivery` | `TestRecoveryAdmissionGuestRefusalStopsRetriesAcrossRestart` |
| CI | job 121392242981, run 40267236832, commit d58d194cf, race non-runtime shard 3 | job 121397242240 (attempt 1), run 40268891859, commit b4b4e551b, race non-runtime shard 7 |
| Failure | `adapter_test.go:707: Texture activation did not start after owner recovery` | `recovery_admission_test.go:290: duplicate job POSTs while job is non-terminal: 2` |
| Cause | a fixed 5 s wait on a path with no 5 s bound (about 1.4 s idle, more under load) | a lost update: `Admit` stores a stale snapshot over the live condition |
| Defect | test (the deadline is a pace, not a bound) | product (a real race) that the test exposes |
| Repro (agents VM, `-race`) | 18 of 40 under load; 0 of 5 idle | 7 of 1440 under load (plus 5 in the sibling test); 10 of 10 with one injected delay; about 1 in 1600 idle |
| Tonight's train | not the cause | not the cause (code untouched since 2026-10-14) |

## Flake 2: vmctl recovery admission loses an update

### Evidence

- Job 121397242240, attempt 1. Fetch it with `gh api --allow-escape-sequences
  repos/yusefmosiah/go-choir/actions/jobs/121397242240/logs`; `gh run view
  --job` returns the latest attempt, which passed (attempt 2, job
  121400296841). Log, UTC:
  - 04:26:44.756 test starts
  - 04:26:44.758 first resolve, `recovery refusal recorded`
  - 04:26:44.760 second resolve, 2 ms later
  - 04:26:44.810 `recovery_admission_test.go:290: duplicate job POSTs while
    job is non-terminal: 2`
- The second resolve began 2 ms after the first, so this was not a slow
  machine.
- Neither `internal/vmctl/recovery_admission.go` nor its test has changed
  since 2026-10-14 (44656ef90, an unrelated hostauth edit). The race is from
  0f7c58ba6 (2026-10-08). Tonight's commits cannot be the cause; it is
  latent.

### Cause (read in code, then reproduced)

`RecoveryAdmission.Admit` (recovery_admission.go) does this for a durable
condition:

1. Lock, clone `a.conditions[computerID]` (a snapshot), unlock.
2. Read the evidence over HTTP (head, watermark, escrow: up to three round
   trips).
3. `reevaluateRecoveryCondition` clones the snapshot again into `updated`.
4. `a.storeCondition(updated)` overwrites the stored condition with that
   copy, then `a.considerJob(updated)` decides about a job POST from it.

Meanwhile the first resolve's background `enqueueJob` goroutine finishes its
POST, takes the lock, writes `Job`, `JobRequestedAt`, `JobSeedWatermark` and
`JobSeedBaseRef` onto the stored condition and clears `enqueueInFlight`. If
that lands between step 1 and step 4, step 4 erases it. `considerJob(updated)`
then sees `Job == nil`, a zero `JobRequestedAt` (so the 30 s re-enqueue
backoff does not apply) and `enqueueInFlight` already false, so it posts a
second job.

The test makes the window likely: `waitForRecoveryTest` waits until the
fake has recorded one POST, which happens before the response reaches
`enqueueJob`. The second resolve's snapshot is therefore taken while the
first enqueue is still unfinished.

The same lost update applies in production to any two callers of `Admit` for
one computer, and to `Admit` against a finishing `enqueueJob`: every start
attempt of a blocked computer calls `Admit`. Effects: a duplicate POST (the
platform deduplicates jobs by computer, so low severity today), the
re-enqueue backoff bypassed, and `Job`, `JobSeed*` and `JobError` rolled
back to the older snapshot. Only the duplicate POST was observed; the others
are read from code.

### Reproduction (agents VM, `-race`, commit b4b4e551b)

- Idle: `-count=100 -cpu 1,2` run 8 times (1600 runs): 1 failing batch, the
  first (message not captured). The other 7 passed. About 1 in 1600.
- Load: 12 test processes of `-run TestRecoveryAdmission -count=60 -cpu 1,2`
  plus 8 CPU spinners on 8 cores. 7 failures of
  `TestRecoveryAdmissionGuestRefusalStopsRetriesAcrossRestart` (all line
  290, "posts: 2") in 1440 runs, about 0.5%. Same runs: 5 failures of
  `TestRecoveryAdmissionFreshInstallRefusesOvercapBeforeBoot` at line 458
  (`head growth alone must not requeue the repair job, posts=2`), the same
  race through a second test.
- Mechanism proof: a throwaway test (not committed, deleted from the VM)
  delays the job POST response by 60 ms and the evidence endpoints by
  200 ms after the first resolve. 10 of 10 runs fail with `duplicate job
  POSTs: 2`. That is the lost update: the snapshot predates `enqueueJob`'s
  write and the store follows it.

### Proposed fix (smallest)

In `Admit`, replace `a.storeCondition(updated)` in the re-evaluation branch
with a merge under the lock: re-read `a.conditions[computerID]`, copy only
the re-evaluated fields (`Kind`, `Reason`, `Witness`, `LastEvaluatedAt`)
onto it, keep `Job`, `JobRequestedAt`, `JobSeedWatermark`, `JobSeedBaseRef`
and `JobError`, and pass the merged clone to `considerJob`. If the
condition was cleared in between, store nothing. The other stores need no
change: the fresh-install path stores a brand-new condition that has no job
yet, and `refreshJobAsync` already mutates `current` under the lock.

The test needs no change. A test-only mitigation (wait for `Job` to be set,
or lengthen the sleep) would hide a real defect.

Class: red (vmctl is a protected surface), small and additive, one-commit
revert. Review tier 1 (one seat, Devin) by the repo rule: it is not tape,
auth, schema, boot path or a delete. Tests first (yellow): add a variant of
the existing test that delays the jobs response, as in the probe above, so
it fails deterministically before the fix.

## Flake 1: the Texture owner-recovery test waits 5 s for a path that has no 5 s bound

### Evidence

- Job 121392242981 (race non-runtime shard 3). The test ran 14.66 s and
  failed at adapter_test.go:707, the `time.After(5 * time.Second)` that
  follows `adapter.Start`. Log timeline, UTC (go test JSON timestamps, so
  approximate):
  - 04:12:34 test starts; store open and DDL alone took about 2.6 s
    (`apply-ddl duration=794ms` and others)
  - 04:12:39 `boot follows a crash or stop`; 04:12:41 `runtime: started`
  - 04:12:42 `actor wake latency kind=owner_revision target=texture:doc-startup-recovery waited=2.32s dispatch=9ms`
  - 04:12:42 `... kind=lifecycle_work_assigned target=research:... waited=2.754s`
  - 04:12:45 a second `lifecycle_work_assigned ... waited=2.18s`
  - 04:12:46 the failure line (5 s after start)
  - 04:12:47 `actor wake outbox disposed dead wake update-startup-recovery`
  - 04:12:48 `Texture activation terminated without disposing exact trigger
    (run=875a09e8... state=completed)` and `runtime: stopped`
- The last two lines are cleanup noise, not the cause. Cleanup closes
  `blocking.release` first (cleanups run last-in first-out), so the
  activation that had not yet executed runs and completes unblocked, and the
  dispatcher logs that it completed without disposing its trigger because
  the test is stopping. `state=completed` shows the activation did start,
  after the deadline. Both lines, and `consumed without a turn: invalid ...
  no lifecycle Texture-control fingerprint`, also appear in a passing run on
  the VM, so they do not discriminate.

### Cause (measured; the mechanism behind the 1.4 s is a hypothesis)

The test is a timing assumption. Measured on the agents VM, `-race`, commit
b4b4e551b, with a throwaway copy of the test that waits up to 90 s and logs
elapsed time (not committed, deleted):

- Idle: `adapter.Start` returns in about 50 ms. The activation reaches
  `provider.Execute` 1.43 s and 1.46 s after Start. Margin to the 5 s
  deadline: 3.5 s, with a floor of about 1.4 s.
- Under load (10 processes of the test plus 8 spinners on 8 cores): Start
  returns in 0.1 to 0.3 s. The 20 activations started at 3.86, 3.91, 4.12,
  4.26, 4.52, 4.55, 4.73, 4.74, 4.84, 4.93, 4.93, **5.05, 5.16, 5.27, 5.33,
  5.47, 5.47, 6.28, 6.34, 6.51** s. 9 of 20 are past 5 s. The activation
  always starts; it is late.
- The unmodified test under the same load: **18 failures in 40 runs**
  (45%), all with the CI message. Idle: 0 of 5 here.

Hypothesis for the 1.4 s floor and its growth under load (not read from a
run ledger; the CI log carries no turn-commit trace): the first dispatch of
the owner-revision or research wake is refused or deferred until the
Texture-recovery base occurrence is processed
(`errTextureRecoveryBaseNotProcessed`, the serialization this test pins).
The wake projector then retries with `wakeRetryBaseDelay = 1s`
(`internal/agentcore/actor_wake_projector.go`) and a growing delay per
failed attempt. One backoff step is about 1 s, two are about 3 s, and the
dispatch and activation work, 5 to 10 times slower in `-race` on a loaded
shard, rides on top. The CI log shows `waited` of 2.2 to 2.75 s per wake,
consistent with one to two backoff steps plus slow dispatch.

The same pattern has been fixed twice: b49f8aedb (2026-10-04) widened a
restart-backlog deadline 5 s to 30 s, and 954b89768 (2026-10-23) did the
same for the sibling `TestAdapterSQLitePreBindResearchRecoveryBindsAndExecutesWithoutSnapshot`
("under -race on a loaded CI shard the owner-bind deferrals alone passed
5 s"; two CI failures). This test kept its 5 s.

### Test defect or product defect

Test defect. In every loaded run the failure was the deadline, and the 20
timed samples all went on to start the activation. The assertions the test
exists for (exactly one Texture run, `texture_agent_revision` metadata,
canonical revision authority) come after the wait and were not the thing
that failed.

Not a production defect. One note for the owner of the wake path: a 1 s
retry floor on first-boot owner recovery means a restarted computer's
Texture waits at least about 1.4 s to start, longer under load. That is the
existing documented backoff, not a regression.

### Could tonight's train have changed the timing?

No, by code diff. `git diff dde4bf411^..b4b4e551b` (the train: owner-act wake
lane SD-08, F1 continue, the related-story edge) over the non-test files in
`agentcore`, `store`, `textureowner` and `actorruntime` leaves
`Adapter.Start`, `textureOwner.Start`, `flushBootDispatches`, the wake
projector and the dispatcher untouched. The edits that touch this fixture
at all: (a) a lineage check in `QueueLifecycleUpdate`, skipped when the
trajectory already matches, which it does here; (b) the new
`selfdev_owner_act_wake.go`, behind `texture.owner_act_wakes`, off by
default; (c) a `relationMu` field on the Runtime; (d) `texture_agent_revision.go`
changes on the owner revise path, which this test does not use. I did not
A/B a pre-train binary under load; the code diff is the argument, and the
5 s deadline against a 3.5 s idle margin accounts for the failure rate by
itself. The test dates from 2026-07-14 (1e9fd7063); the file was last edited
by 954b89768 for a different test.

### Proposed fix (smallest)

Make the wait a bound, not a pace: `time.After(5 * time.Second)` to
`30 * time.Second` at adapter_test.go:706, the change 954b89768 made for the
sibling test. The select returns as soon as the provider starts, so a
passing run costs nothing extra. Class: yellow (test only), tier 0, no model
review. Optional follow-up: this package has 4 more `5 * time.Second`
waits; sweep them with the same rule when next touched.

## Why neither was caught earlier

Both are rate effects. Flake 1 passes at 3.5 s idle and a CI race shard has
several times the latency. Flake 2 needs the first enqueue to finish inside
a few-millisecond window and happens about once in a thousand idle runs. Each
failed once in CI and passed on re-run.

## Belief state and remaining error

- Flake 2: confirmed product race (lost update), low severity today because
  the platform deduplicates jobs by computer. Not checked: whether Node B can
  hit it (two `Admit` calls for one blocked computer inside one evidence
  read), and whether the rolled-back `Job` status matters to any reader.
- Flake 1: confirmed test-deadline defect. The retry-backoff explanation of
  the 1.4 s floor is a hypothesis; a read of the wake outbox attempts and
  `wakeRetries` for this fixture would settle it and was not done.
- Heresy delta: discovered 1 (the vmctl lost update), introduced 0,
  repaired 0.
- Cleanup: the VM's `~/lanes/test-fp0-train` is on detached b4b4e551b with
  no tracked changes; throwaway test files were removed; the compiled test
  binaries `/tmp/vmctl.test`, `/tmp/actorrt.test` and `/tmp/actorrt2.test`
  remain on the VM.
