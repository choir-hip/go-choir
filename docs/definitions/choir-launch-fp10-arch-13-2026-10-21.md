---
definition_version: 4

# Station FP10, item ARCH-13 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): store fan-in for wide
# trees. Authority: the metamission's FP10 entry ("the P2 items ... each
# decided item by item") and SDC ARCH-13, whose plan is option (c) of
# docs/reports/projection-event-volume-design-2026-10-19.md and SA slices 4
# and 6.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the red commit to the bunch panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: main e4b792b7f
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /home/agent/lanes/fp10-arch13-fanin
      status: clean
      class: goal_candidate
      owner: FP10
      touch: goal_owned
      recovery: branch lane/fp10-arch13-fanin

finish:
  deliver: >-
    A guest write never waits on a Dolt history commit when the owner turns
    the debounced checkpoint on, and every computer reports how often it
    commits, how long a commit holds the write path, and how many events
    and ops each commit carries.
  artifact: >-
    Commits on lane/fp10-arch13-fanin: the problem record and this file
    (green); the failing tests (yellow); the observe counters, the
    lock-free dirty mark and the debounced checkpoint behind the computer
    setting store.debounced_checkpoint, default off (red).
  acceptance:
    - action: >-
        go test ./internal/store -race -run 'Checkpoint|TestFinalizeBatchesDoltCheckpoints';
        go test ./internal/agentcore -race -run 'Checkpoint|ComputerSetting|Observability|TelemetryBatching|BootConfigures';
        go vet of both
      proves: the mode switch, the bounds, the background commit and the counters behave as the tests below state
      evidence_class: local test
    - action: >-
        After the deploy, GET /api/runtime/observability on a staging test
        computer (owner session): read guest_checkpoint
      proves: the counters are live in count mode (mode "count", inline commits, a duration histogram)
      evidence_class: deployed proof
    - action: >-
        On the same test computer, POST /api/computer-settings
        {"key":"store.debounced_checkpoint","value":true}, run one research
        activation, read guest_checkpoint again over a few minutes, then
        POST the value false and read it once more
      proves: inline_commits and inline_ns stop growing, background_commits grows, last_commit_age_ms stays under debounce_ms plus a commit, failures stays 0; after false, mode is count and inline commits resume
      evidence_class: deployed proof
    - action: the paper's schedules and polls read before and after the deploy (setting left off on the paper)
      proves: the deploy did not take the paper down
      evidence_class: deployed proof
  rollback: >-
    The setting off (the next mark goes back to counted inline commits);
    then git revert of the red commit. Nothing persistent changes shape:
    no schema, no witness, no tape format.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the time a guest writer waits on work it does not need (a
    database-wide history commit), while keeping every acknowledged write
    durable and history addressable within a bounded age.
  goodharting_would_be: >-
    Fewer commits by never committing; a background commit that still
    blocks writers through the dirty mark; counters that report the mode
    but not the time writers lost.

homotopy:
  realism_axis: >-
    Store tests with -race against a real embedded Dolt, then a staging test
    computer with the setting on under one research activation, then (after
    a soak) the paper.

boundaries:
  mutation_class: green (docs); yellow (tests); red (guest store write path)
  authority_sources:
    - metamission FP10 entry; SDC ARCH-13
    - docs/reports/projection-event-volume-design-2026-10-19.md §4(c), §5 stage 1
    - docs/choir-doctrine.md and CLAUDE.md
  must_preserve:
    - Default behavior unchanged in kind - count mode commits inline every 100 events as today.
    - Durability never depends on a Dolt commit; nothing acknowledged is lost on a crash in either mode.
    - Replay keeps its own cadence (CommitReplay); GetHistory and DiscardPrepared still commit synchronously.
    - No schema, witness, tape-format or boot-path change; no boot-time resumer.
    - vmctl, vmmanager and the VM image are not touched (FP8's lane).
  excluded:
    - SA slice 4 (read cost) and slice 5 (store split, group commit of acts), and slice 6 (VM shapes, which are vmctl and FP8's).
    - Turning the setting on for the paper; that is FP0's or the owner's call after a soak.
  protected_surfaces: [guest store write path]

now:
  status: checkpoint_incomplete
  slice: >-
    Built and tested on the lane; nothing deployed. The red commit d1ec806e2 (was 9b8cc35f8)
    goes to FP0's bunch panel on its frozen diff, then lands alone (or with
    green) and FP0 runs the deployed acceptance above on a test computer,
    never the paper. Local evidence: internal/store full package green
    (374 s); the new store tests 3x under -race; the agentcore seam tests and
    every agentcore test file that touches settings or observability green
    under -race.
  source_ref: >-
    lane/fp10-arch13-on-main, rebased onto main 5efe46672 on 2026-10-22
    (old -> new): 3939454fe -> 64fded347, 4372cbbdc -> c8a1d5c00,
    9b8cc35f8 -> d1ec806e2, 6d059b885 -> f8966523d, 3bab3b8d1 -> 62dd84558,
    ced58e401 -> 31c6b2e5b, 410c6d35d -> f298f7484, 9196f87d9 -> 08bb5e4d6.
    The red commit FP0 reviews is now d1ec806e2, the review fix f298f7484.
  deploy_identity: unknown to this lane
  candidate:
    id: fp10-arch13-debounced-checkpoint
    state: ready
    ref: lane/fp10-arch13-on-main
    base: 5efe46672
    digest: none
    scope:
      - internal/store/checkpoint_cadence.go
      - internal/store/checkpoint_cadence_test.go
      - internal/store/store.go
      - internal/store/computer_events.go
      - internal/store/computer_setting.go
      - internal/agentcore/guest_checkpoint.go
      - internal/agentcore/api.go
      - internal/agentcore/computer_settings.go
      - internal/agentcore/runtime.go
      - internal/agentcore/telemetry_batching_test.go
  rebase:
    - >-
      2026-10-22 onto main 5efe46672. One conflict, Store.Close in
      internal/store/store.go: main added CancelDerivedColumnMigration, this
      lane closeCheckpoint. Both kept, cancel first: the cancel only stops
      the migration before its next step, so no migration step starts after
      the closing checkpoint commit (which would leave its writes
      uncommitted and race that commit); the checkpoint commit writes
      through s.db, so it runs before the pools close. Reopen needed no
      change (main's cancel stays first, the lane's cadence reopen stays
      last). Re-run after rebase (GOMAXPROCS=2 GOFLAGS=-p=2, -race -count=1,
      a shared loaded VM): go vet internal/store and internal/agentcore
      clean; agentcore -run 'Checkpoint|ComputerSetting|Observability|
      TelemetryBatching|BootConfigures|Rematerialize' ok (141 s). The whole
      internal/store package in one run hit the 10-minute cap under load
      with no failure or race before it, so it was run as four -run groups
      covering all 451 top-level tests: three ok first time; group two
      (109 tests, alphabetical second quarter) panicked once, details lost
      to an output filter, then passed twice in full (294 s). Residual: an
      unidentified intermittent panic in that group; not shown to come from
      this lane's code.
  review_fixes:
    - >-
      2026-10-22, FP0 first-pass reviewer (blocking): Reopen after Close left
      the cadence closed. Problem recorded, failing test, fixed: Reopen
      reopens the cadence and arms it (TestCheckpointReopenAfterCloseStaysDebounced).
  residuals:
    - >-
      Behavior change in count mode too, by design: the dirty mark no longer
      waits on a commit in flight, a "nothing to commit" result counts as a
      commit in the stats, and Reopen marks the adopted workspace dirty (at
      most one extra no-op commit). Reviewers should check these three.
    - >-
      Concurrent DOLT_COMMIT('-Am') with in-flight finalize transactions is
      exercised by a -race test on embedded Dolt and has precedent
      (GetHistory today, doltbatch on the platform), but not under the
      paper's load. Soak on a test computer before the paper.
    - >-
      ARCH-13's larger parts stay open: SA slice 4 (read cost), slice 5
      (store split, group commit of acts); slice 6 is VM shapes (vmctl,
      FP8's lane). The checklist line should say eventMu plus the appender
      mutex, not engineMu (FP0 edits the checklist).
  conjecture:
    id: C-checkpoint-off-write-path
    claim: >-
      The guest's inline Dolt checkpoint is a write-path stall that every
      concurrent writer shares; moving it to a debounced background commit
      with a lock-free dirty mark removes that stall without losing
      durability or history addressability beyond the debounce window.
    test: >-
      Count mode on staging shows non-zero write-path checkpoint time; with
      the setting on, write-path checkpoint time is zero, background
      commits occur within the window, and the computer behaves as before.
    edge: missing_oracle
    delta_o: the observe counters (this slice) are the oracle
    scope_if_supported: guest store at the deployed SHA
    status: testing
    evidence_refs: [docs/problems/guest-checkpoint-on-the-write-path-2026-10-21.md, 9b8cc35f8]
  decision:
    what: take option (c) plus a lock-free mark, behind a computer setting, default off
    kind: safety
    status: settled
    evidence_ref: the cited design's stage 1 names this piece; red ships off
    owner_ratification_ref: not_applicable
---

# FP10 · ARCH-13 · the guest checkpoint off the write path

Problem: [guest-checkpoint-on-the-write-path-2026-10-21](../problems/guest-checkpoint-on-the-write-path-2026-10-21.md).

## The slice

1. **Observe, always on.** `Store.CheckpointStats()` and
   `/api/runtime/observability` `guest_checkpoint`: mode; inline and background commit counts;
   write-path checkpoint nanoseconds (inline commits only) and the max;
   background commit nanoseconds; failures and the last error; events and
   ops since the last commit; age of the last commit.
2. **Lock-free mark.** The dirty flag becomes a generation counter. A mark
   is an atomic increment; a commit records the generation it read before
   the SQL call and is "clean" only if no mark came after. A write never
   waits on a commit in flight. Same commit set as today in count mode.
3. **Debounced mode** (setting `store.debounced_checkpoint`, or env
   `CHOIR_GUEST_CHECKPOINT=debounced`; default off). `finalizeBatch` never
   commits; it arms a timer (default 60 s, env
   `CHOIR_GUEST_CHECKPOINT_DEBOUNCE`) or, past an ops bound (default
   10,000 ops since the last commit, env
   `CHOIR_GUEST_CHECKPOINT_MAX_OPS`), kicks the background committer at
   once. The commit runs outside `eventMu` and the appender mutex. Close
   stops the timer and commits once. Turning the setting off stops the
   timer and returns to counted inline commits.

## Failure modes, written before the code

- A mark during a commit in flight is lost (the store reports clean while
  rows sit uncommitted). Test: the generation guard.
- The background commit races a finalize transaction (Dolt). Test: -race
  with concurrent writers and a 1 ms debounce against real embedded Dolt.
- The timer never fires again after a failure. Test: a failed commit
  re-arms.
- A quiet computer never commits. The timer fires on the first mark after
  a commit, so the age is bounded by the window.
- Mode switch mid-flight leaves a timer that commits after Close. Test:
  Close stops it; a commit after Close is not attempted.
- Count mode drifts from today's cadence. Test: the existing
  `doltCheckpointEvery` test still passes, and the inline counter counts
  exactly one commit per `every` events.
- Replay starts committing per event. Replay path untouched; test it
  stays at zero inline commits.

## FP0 steps

Land after the bunch review; deploy alone or with green; run the
acceptance above on a test computer, not the paper.
