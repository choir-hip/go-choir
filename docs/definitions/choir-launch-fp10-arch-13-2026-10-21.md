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
    When the owner turns the debounced checkpoint on, the Dolt history
    commit leaves finalize, so a writer stalls on a commit at most once per
    debounce window or ops bound instead of every 100 events. The stall is
    bounded, not zero: the background commit still holds the store's one
    connection, and a finalize that starts during it waits at BeginTx
    (removing that is SA slice 5). Every computer reports how often it
    commits, how long a commit holds the write path, how often and how long
    a finalize waited on a commit, and how many events and ops each commit
    carries.
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
      proves: inline_commits and inline_ns stop growing, background_commits grows, last_commit_age_ms stays under debounce_ms plus a commit whenever events_since_commit is above 0 (with nothing pending the age may grow without bound, since a no-op commit, counted in noop_commits, does not reset it), failures stays 0 (a planned restart does not move it, since cancelled_at_close, closing_commits_skipped and closes_left_open count the close apart); finalize_commit_waits grows by at most about one per background commit and finalize_commit_wait_max_ns stays near background_max_ns (the bounded stall, reported, not hidden); after false, mode is count and inline commits resume
      evidence_class: deployed proof
    - action: >-
        Round 5 is not inert (see the landing note): with the setting off,
        on a staging test computer, a planned restart while idle - the
        guest journal has no "close left the store open" line and the
        restart takes no longer than before; and, if FP0 can trigger it, a
        planned restart during an online DOLT_GC - the journal shows "close
        left the store open: a Dolt commit holds the commit lock", Close
        returned after at most about 10.25 s past the telemetry drain, the
        process exited, and the next boot opened the store with no lock
        error and served
      proves: the default-mode Close fence on a real computer, idle and behind a GC
      evidence_class: deployed proof
    - action: >-
        Rematerialize (round-6 review F5; FP0's): on a staging test computer
        with the setting off, run a rematerialize while idle - it completes
        as before (Close returns nil, the flip happens, the store reopens on
        the staged workspace and serves); and, if FP0 can arrange it, one
        while an online DOLT_GC or a long statement is running - the owner
        act returns an error naming ErrCheckpointCommitLive within about
        10.25 s past the telemetry drain, the journal shows "close left the
        store open", nothing is quarantined or renamed, the computer keeps
        serving on its original store in debounced or count cadence as
        before (mode is not "closed"), and a later rematerialize, once idle,
        succeeds
      proves: rematerialize fails closed and leaves the computer serving, and still works when idle
      evidence_class: deployed proof
    - action: the paper's schedules and polls read before and after the deploy (setting left off on the paper)
      proves: the deploy did not take the paper down
      evidence_class: deployed proof
    - action: >-
        Before the setting is turned on for the paper (not part of this
        landing): on a test computer with the setting on, finalize events
        (one research activation), read guest_checkpoint so that
        events_since_commit > 0, kill -9 the guest runtime process, let the
        computer restart, then read the tape head and the projected rows
        for every event acknowledged before the kill
      proves: >-
        the durability assumption - with the checkpoint off the write path,
        an acknowledged write rests on SQL COMMIT being durable in the
        embedded Dolt working set; every acknowledged event is still
        projected after the kill, with no Dolt commit covering it
      evidence_class: deployed proof (FP0; a soak alone does not count)
  rollback: >-
    The setting off (the next mark goes back to counted inline commits);
    then git revert, newest first, of each round's record, fix and tests
    together: the round-2 and round-4 tests do not compile without their
    fix, and the round-3 tests compile but fail on behavior, so a fix
    reverted alone leaves red tests (the round-5 tests compile on round 4
    and five of six fail there on behavior; TestCheckpointCloseBoundIsOneDeadline
    passes there, it pins a bound round 4 already had; the round-6 test
    fails on round 5 on behavior; four of the five round-7 tests fail on
    round 6 on behavior, the fence-restore test pins round-6 behavior).
    Round 8 - its green record, 7078b0561, 949fb1467 (949fb1467 does
    not compile without 7078b0561); round 7 - eb8185144, 6dbdf815d, 1aaa5e7c9 (this alone reverts
    the pinning; the commit-lock fence stays); round 6 - 3b220fa02,
    7ed2ce76a, c10afb2d3, d36230506,
    9d2523727; round 5 - 83f7bc1f1, 4d42ba082, d529f6a94; round 4 - 75f3a9a46, cf2dc9d90, ed56fdc0e; round 3 - 40f1422a7, b0ea5d1c0, 1a0fb4c13; round 2 -
    2b964182e, a95e8ff16, ba875fab5; then f298f7484 and d1ec806e2. Nothing
    persistent changes shape: no schema, no witness, no tape format.
  landing_note: >-
    ARCH-13 is a live red with its own deploy, not an inert ride-along.
    The debounced checkpoint stays behind its setting (default off), but
    round 5's Close changes are unconditional: closeMu, the commit-lock
    fence and the Reopen refusal apply in the default count mode on every
    computer, the paper included. Online DOLT_GC holds the commit lock
    for minutes, so a default-mode Close during a GC now waits up to
    about 10.25 s (after the telemetry drain's own up-to-10 s) and returns
    ErrCheckpointCommitLive instead of tearing the engine down under the
    GC. The planned restart ignores that error and exits, so its restart
    is up to about 10.25 s slower; rematerialize returns the error where
    it used to proceed. Gating the fence on the setting would reopen the
    round-4 fail-open door in default mode, so it stays unconditional.
    Round 7 (its own landing after deploy R) widens the fence to the
    pools: a Close behind any statement in flight or open transaction on
    the guest store, not only a commit, now waits up to the same hard
    deadline and fails closed.
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
    - Turning the setting on for the paper; that is FP0's or the owner's call after a soak and the kill -9 drill in acceptance.
  assumptions:
    - >-
      Durability without the checkpoint rests on SQL COMMIT being durable in
      the embedded Dolt working set (the Dolt commit only adds history
      addressability). Not yet tested by a crash; the kill -9 drill in
      acceptance tests it before the paper.
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
    Review round bunch 45 on f60daaa09: 4495cd076 (problem record),
    ba875fab5 (tests), a95e8ff16 (fix, red), then this card.
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
  review_round_bunch_45: >-
    2026-10-22, DeepSeek (accept with edits; Gemini flash and Devin had
    accepted). Problem record 4495cd076, tests ba875fab5, fix a95e8ff16.
    (1) the claim is narrowed: the stall is bounded, not zero; the
    finalize commit-wait counters make it visible; the acceptance reads
    them. (2) the closing commit is capped at 10 s. (3) residual, below.
    (4) no-op commits counted apart; mode reports closed. (5) the
    durability assumption is stated and a kill -9 drill is required before
    the paper.
  review_round_3_bunch_48: >-
    2026-10-22, Gemini 3.8 flash accept; DeepSeek v4.1 flash accept with
    edits (review of round 2). Tests 1a0fb4c13 (fail on 2b964182e by
    behavior), fix b0ea5d1c0 (red), this record. (1) fixed, not narrowed:
    Close cancels the in-flight background commit and takes the commit
    lock only until the close deadline, else skips the closing commit and
    counts a failure. (2) fixed: commits pin the connection; the wait
    counters are exact. (3) the cap is a context deadline; stated in the
    failure modes with the data-safety argument. (4) Close comment fixed;
    a background commit after Close is not attempted (tested). (5) the age
    check applies only while events are pending. (6) rollback reverts
    yellow and red together.
  review_round_5_bunch_52: >-
    2026-10-22, DeepSeek v4.1 flash send back (reproduced High), Gemini
    flash accept with edits. Tests d529f6a94, fix 4d42ba082 (red), this
    record. (1) fixed - Close and Reopen serialized on closeMu; every
    Close ends with the commit-lock fence by the hard deadline; the nil
    shortcut on a retried Close is gone. (2) fixed - count mode takes the
    same fence. (3) one hard deadline bounds the checkpoint waits and the
    fence (round 6 narrowed this from "the whole Close"); the code at
    75f3a9a46 already shared one context, the test pins the bound. (4)
    one count per cut commit and per failing Close. (5) test and record
    claims narrowed; the callers residual corrected. (6) Reopen refuses a
    store that is not closed. Re-run - go test ./internal/store -run
    'Checkpoint|Finalize|Close|Reopen' ok; the store package in four -run
    groups (466 tests) ok; autoputer ok; under ~/heavy-test.lock with
    GOMAXPROCS=2 GOFLAGS=-p=1, -race store Checkpoint|Finalize|Close|Reopen
    ok (108 s) and the agentcore seam tests ok (134 s); gates hang-guards,
    decode-roots, heresies, v1-writer-purity ok; guest-data-fstrim not
    runnable here (no nix binary; the lane touches no nix file).
  review_round_6: >-
    2026-10-22, after round 5's three accepts (Gemini flash; DeepSeek v4.1
    flash and Sonnet 5.5 accept with edits). No patch round on the commit
    class (the structural assessment stands). Test 9d2523727, fix
    d36230506 (red), this record. (1) docs - round 5 is not inert; the
    landing note and a deployed acceptance for a planned restart idle and
    behind an online GC. (2) fixed - a Close that fails closed restores
    the cadence and the telemetry sealer; close cuts are judged by what
    cut the commit. (3) DeepSeek - the bound sentence now covers only the
    checkpoint waits and the fence (fixed); the rollback note says five
    of six round-5 tests fail on round 4 (fixed); the committer-path skip
    counts only in debounced mode (fixed in code). (4) non-commit
    statements recorded as an owner decision (pin the pools, or never
    close the engine in-process), with Sonnet's F3 and F5 as residuals.
    Re-run - go test ./internal/store -run 'Checkpoint|Finalize|Close|Reopen'
    ok; under ~/heavy-test.lock with GOMAXPROCS=2 GOFLAGS=-p=1, -race store
    Checkpoint|Finalize|Close|Reopen|Telemetry ok (130 s) and the agentcore
    seam tests ok (136 s); gates hang-guards, decode-roots, heresies,
    v1-writer-purity ok; guest-data-fstrim not runnable here (no nix
    binary; the lane touches no nix file).
  owner_decisions:
    - what: >-
        Fence non-commit statements by pinning the pools - before teardown,
        Close checks out each pool's single connection (s.db, s.readDB)
        under the hard deadline, else fails closed - option (a); not
        option (b), never closing the engine in-process
      kind: safety
      decided_by: owner
      decided_at: 2026-10-22
      relayed_by: FP0
      status: built in round 7 (tests 1aaa5e7c9, fix 6dbdf815d), on top of 3b220fa02; FP0 lands it after deploy R
  round_8_mirror_panic: >-
    2026-10-22, the arch13j Linux -race mirror of eb8185144 (rebased on
    main 0bf8a097d) panicked in internal/store. Root cause - the test's
    workspace was removed under its open engine (store test paths were
    os.TempDir()/go-choir-m3-store-test/<TestName>, shared by every test
    process of the package on the VM; not reproduced in one process, the
    whole package passed under -race here in 1033 s); Open failed
    ("database not found: texture") and its Close closed the engine; Dolt
    registers SQL-engine databases with SetCrashOnFatalError, so
    NomsBlockStore.Close, unable to flush the journal root into the
    missing .dolt/noms, crashed the process. Pre-existing - the same test
    panics identically on 3b220fa02 (overlay), and FP0's 01:16 record
    (agentcore-store-open-tables-changed-under-load) shows the same crash;
    round 7's teardown order is not the cause (the engine closes once,
    last, after every pinned connection is released, on every path).
    Tests 949fb1467 (Close over a removed workspace and an Open failing
    after the engine is up, both in child processes; test paths per
    process), fix 7078b0561 (Close checks the workspace's .dolt/noms is
    on disk before closing the engine, else leaves it and returns
    errWorkspaceGone). Re-run - under ~/heavy-test.lock with
    GOMAXPROCS=2 GOFLAGS=-p=1, the whole internal/store package under
    -race on this Linux VM ok (1261 s) and the agentcore seam tests ok
    (161 s); go test ./internal/store -run
    'Checkpoint|Finalize|Close|Reopen|Telemetry' ok; gates hang-guards,
    decode-roots, heresies, v1-writer-purity ok; guest-data-fstrim not
    runnable here (no nix binary).
  round_7_pin_the_pools: >-
    2026-10-22, the owner's decision (a). Tests 1aaa5e7c9, fix 6dbdf815d
    (red), this record. Close pins each pool's connection after the
    commit-lock fence, under the same hard deadline, and fails closed with
    the round-6 restore when it cannot; the pools are opened on a
    connector wrapper so DB.Close does not close the engine, and the
    engine closes last with no connection open. Round-6 review F4 (the
    fence-failure restore) has a test; F5 (rematerialize) has a deployed
    acceptance step for FP0. Re-run - go test ./internal/store -run
    'Checkpoint|Finalize|Close|Reopen|Telemetry' ok; the store package in
    four -run groups (472 tests) ok; under ~/heavy-test.lock with
    GOMAXPROCS=2 GOFLAGS=-p=1, -race store
    Checkpoint|Finalize|Close|Reopen|Telemetry ok (173 s) and the agentcore
    seam tests ok (136 s); gates hang-guards, decode-roots, heresies,
    v1-writer-purity ok; guest-data-fstrim not runnable here (no nix
    binary; the lane touches no nix file).
  review_round_7: >-
    2026-10-22, DeepSeek v4.1 flash accept with edits on c10afb2d3 (the
    behavior is right; three stale statements). Fixed - the left-open
    residual now says a failed Close restores the cadence and the sealer;
    the Store.Close code comment and the failure-mode bullet say the hard
    deadline bounds the checkpoint waits and the fence, not the whole
    Close. Not done in this round (FP0 scoped it to the three statements)
    - DeepSeek F4 (no test of the undo on the fence-failure path) and F5
    (no deployed acceptance for rematerialize).
  structural_assessment: >-
    Round 5 on Close (CLAUDE.md escalation rule - 3+ rounds without
    convergence). The rounds found doors one at a time because Close had
    no single lifecycle authority - the cadence's closed flag, the
    closeLive channel and the unlocked pool teardown each guarded part of
    it. Round 5 makes it one - closeMu over Close and Reopen, and the
    commit lock as the only fence before teardown - so the claim reduces
    to one invariant - no pool closes while doltCommitMu is held by
    anyone else. If a further door is found, FP0 should escalate to the
    owner rather than patch - the alternative is to drop the debounced
    closing commit and leave Close to the fence alone.
  review_round_4_bunch_50: >-
    2026-10-22, DeepSeek v4.1 flash accept with edits on 40f1422a7. Tests
    ed56fdc0e, fix cf2dc9d90 (red), this record. (1) fixed: Close waits for
    the cancelled committer and the closing commit within the deadline and
    otherwise fails closed with ErrCheckpointCommitLive, leaving the engine
    open; Reopen refuses; the next-open check is split (same process -
    Dolt's lock refuses, the engine is intact; next process - the pre-close
    head, no lock error), because with the engine left open an in-process
    open cannot be lock-free. (2) wording - exact for the connection hold,
    and the lost ErrBadConn retry. (3) cancels, skips and left-open closes
    count apart from failures. (4) rollback reason and list fixed. (5) the
    finalize-wait test syncs on the pool's WaitCount, not a sleep.
    Re-run - go test ./internal/store -run 'Checkpoint|Finalize|Close|Reopen'
    ok; under ~/heavy-test.lock with GOMAXPROCS=2 GOFLAGS=-p=1, -race
    store Checkpoint|Finalize|Close|Reopen ok (96 s) and agentcore
    Checkpoint|ComputerSetting|Observability|TelemetryBatching|
    BootConfigures|Rematerialize ok (155 s); gates check-test-hang-guards,
    check-decode-roots, check-heresies, check-v1-writer-purity ok;
    check-guest-data-fstrim not runnable here (no nix binary on the VM;
    the lane touches no nix file).
  previous_rerun: >-
    Re-run (GOMAXPROCS=2, ulimit -n 65536):
    store TestCheckpoint* -race ok (17 tests); the four timing tests -race
    -count=5 ok; store Checkpoint|Finalize|Close -race ok; store package
    without -race ok as four -run groups covering all 457 top-level tests
    (in one run it hit the 10-minute cap on the loaded VM with no
    failure); agentcore seam tests -race ok; go vet and the hang guard ok.
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
      A store whose workspace is removed under its open engine leaks the
      engine until process exit (round 8): Close returns errWorkspaceGone
      instead of crashing. The check and the engine close are not atomic,
      so a removal between them still crashes; and Dolt can still crash
      on a removed workspace on any write-path flush, which no Close
      change covers. Removing a live store's workspace is not a product
      path; it came from test paths shared across processes.
    - >-
      REPAIRED in round 7 (6dbdf815d) by the owner's decision of
      2026-10-22, option (a), pin the pools - Close now checks out each
      pool's connection and fails closed when it cannot. Kept here as the
      record of the class as it stood before round 7 - non-commit
      statements were not fenced (Sonnet r5 F2,
      the same class as the commit fence, one rung wider). The fence is
      the commit lock; a statement that is not a commit (a SELECT SLEEP
      in flight, an open write transaction, a finalize transaction, a
      migration step) holds the pool's one connection without it, and
      Close closes the engine under it. Probes on 83f7bc1f1 - a SLEEP in
      flight failed "context canceled" and Close returned nil in 29 ms;
      an open write transaction's Commit failed "database is read only"
      and its rows were not durable. Loud failures, not silent ones.
      Two designs end it. (a) Pin the pools - before teardown, check out
      each pool's single connection (s.db.Conn and s.readDB.Conn, both
      MaxOpenConns 1) under the hard deadline; a strict superset of the
      commit fence, small. (b) Never close the engine in-process -
      rematerialize and the projection-base rebase go through a planned
      restart with the durable marker; larger, deletes the fence,
      closeMu, ErrCheckpointCommitLive and the left-open state (DeepSeek
      notes rematerialize renames the workspace in-process today, so (b)
      moves that flip to boot). FP0 put this to the owner, who chose (a)
      on 2026-10-22; (b) is not pursued.
    - >-
      A commit queued behind the Close fence (an inline finalize commit,
      the online GC, a texture history read) takes the commit lock once
      Close returns and reads the store's handles while a Reopen swaps
      them (Sonnet r5 F5, pre-existing): a data race whose commit lands
      on the adopted engine or fails on the closed pool. Closed by design
      (b) above, or by taking the commit lock around Reopen's adoption.
    - >-
      Once the fence is held, a wedged pool teardown keeps the commit
      lock and closeMu, so every later commit and every Close or Reopen
      waits behind it (Sonnet r5 F3); only process exit recovers it.
    - >-
      The background commit still holds the store's one connection
      (MaxOpenConns 1): a finalize that starts during it waits at BeginTx
      holding eventMu, and other writers queue behind it. Bounded to about
      once per window or ops bound and now counted
      (finalize_commit_waits); removing it needs a second connection or
      the store split, SA slice 5 (excluded).
    - >-
      After a Close that failed closed (a commit still running at the
      deadline) the store stays open and serving, with the checkpoint
      cadence and the telemetry sealer restored to what they were before
      the Close (round 6); Reopen refuses until a later Close succeeds,
      and a retried Close is fenced like any other. Only a derived-column
      migration the Close cancelled stays cancelled. The callers differ. The planned restart ignores Close's
      error and exits, and process exit reclaims the store. The
      projection-base rebase returns the error and boot fails.
      Rematerialize returns the error and the guest stays up on the open
      store, so a later Close may be retried while an inline commit runs;
      that retried Close is fenced like any other (round 5).
    - >-
      A commit that holds the commit lock past the deadline forever keeps
      every Close failing closed and Reopen refusing; nothing in the
      process recovers it (no watchdog). Process exit does.
    - >-
      Close is not a write fence (review item 3, kept as a residual, not
      fixed): CancelDerivedColumnMigration only signals, closeCheckpoint
      does not wait for writers in flight, and database/sql Close does not
      wait for a connection in use, so a migration step already queued on
      the connection, or any other writer racing shutdown, can land after
      the closing commit or fail on the closed pool. (The checkpoint's own
      background committer is fenced: after Close it does not commit.) Effect: history addressability at a planned
      shutdown only, not durability (the write is in the SQL working set,
      and the next DOLT_COMMIT -Am after boot takes it). Not fenced because
      waiting on the migration alone would not make the closing commit a
      fence for the other writers, and a real fence (stop all writers
      first) belongs to the shutdown ordering, not the cadence.
    - >-
      Behavior change in count mode too, by design: the dirty mark no longer
      waits on a commit in flight, and Reopen marks the adopted workspace
      dirty (at most one extra commit, counted in noop_commits if Dolt has
      nothing to commit). No-op commits no longer count as commits.
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
      with a lock-free dirty mark bounds that stall to about once per
      window (it does not remove it while the commit shares the store's one
      connection) without losing durability or history addressability
      beyond the debounce window.
    test: >-
      Count mode on staging shows non-zero write-path checkpoint time; with
      the setting on, inline write-path checkpoint time is zero, the
      finalize waits on a commit are bounded to about one per background
      commit, background commits occur within the window, and the computer
      behaves as before.
    edge: missing_oracle
    delta_o: the observe counters (this slice) are the oracle, including finalize_commit_waits for the bounded stall
    scope_if_supported: guest store at the deployed SHA
    status: testing
    evidence_refs: [docs/problems/guest-checkpoint-on-the-write-path-2026-10-21.md, d1ec806e2, a95e8ff16]
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
   background commit nanoseconds; no-op commits ("nothing to commit",
   counted apart); finalizes whose BeginTx waited on a commit holding the
   store's one connection, with their wait time and max; failures and the
   last error; events and ops since the last commit; age of the last
   commit. Mode is count, debounced, or closed (between Close and Reopen).
2. **Lock-free mark.** The dirty flag becomes a generation counter. A mark
   is an atomic increment; a commit records the generation it read before
   the SQL call and is "clean" only if no mark came after. A mark never
   waits on a commit in flight (the transaction can; see 3). Same commit set as today in count mode.
3. **Debounced mode** (setting `store.debounced_checkpoint`, or env
   `CHOIR_GUEST_CHECKPOINT=debounced`; default off). `finalizeBatch` never
   commits; it arms a timer (default 60 s, env
   `CHOIR_GUEST_CHECKPOINT_DEBOUNCE`) or, past an ops bound (default
   10,000 ops since the last commit, env
   `CHOIR_GUEST_CHECKPOINT_MAX_OPS`), kicks the background committer at
   once. The commit runs outside `eventMu` and the appender mutex, but on
   the store's one connection, so a finalize that starts during it waits
   (counted, above; the commit pins the connection, so the count is
   exact for the connection hold). Close and Reopen are serialized on
   the store (a second Close waits for the first). Close stops the timer,
   cancels a background commit in flight and waits for it, in debounced
   mode commits once (again on a retried Close), and then, in every
   mode, takes the commit lock under which every Dolt commit runs and
   holds it while the pools and the connector close; then (round 7) it
   checks out each pool's single connection, so no statement or open
   transaction is using the engine, closes the pools, releases those
   connections into the closed pools and closes the engine last (the
   pools no longer close the engine themselves). One hard deadline,
   a 10 s close deadline plus one 250 ms grace, bounds the checkpoint
   waits, the fence and the pin. It does not bound the telemetry drain before them
   (its own 10 s), the wait for a concurrent Close or Reopen on closeMu,
   or the pool teardown after the fence (only process exit recovers a
   wedged teardown), so a caller's worst case is about 10 s + 10.25 s
   plus a concurrent Close's own. A Close that fails closed restores the
   cadence and the telemetry sealer it changed. If a
   commit is still running or holds the lock then, Close fails closed:
   it returns ErrCheckpointCommitLive and leaves the pools, connector and
   engine open (process exit reclaims them), and Reopen refuses until a
   later Close succeeds. Reopen also refuses a store that is not closed. Turning the setting off stops the
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
  Close stops it; a background commit that reaches the commit lock after
  Close is not attempted (TestCheckpointCommitAfterCloseIsNotAttempted).
- Close waits out a background commit already in flight. Test: with a
  live background commit ignoring its cancellation, Close returns within
  the close deadline plus the grace with ErrCheckpointCommitLive, the
  connector still answers, Reopen refuses, and a Close after the commit
  finishes closes the store
  (TestCheckpointCloseIsBoundedBehindABackgroundCommit).
- Close tears the embedded engine down under a running commit. database/sql
  Close does not wait for a connection in use, and the connector's Close
  shuts the engine, DoltDB and NomsBlockStore; that ordering is no driver
  contract, could panic, and NomsBlockStore.Close can block on nbs.mu.
  So Close never closes the pools while a Dolt commit holds the commit
  lock, in either mode and on any Close: it fails closed instead. (A
  finalize transaction or migration step that is not a commit is not
  fenced; that is the write-fence residual.) Tests: in this process an
  open while the commit is held is refused by Dolt's lock (the engine is
  intact), and after release a retried Close commits the pending rows
  (TestCheckpointNextOpenWhileCommitterHeld; the held stand-in returns
  before DOLT_COMMIT with its context cancelled, so this does not show
  what a real DOLT_COMMIT already inside Dolt would do); a child process
  that fails closed and exits with the commit held leaves a store the
  next process opens with no lock error at the pre-close head
  (TestCheckpointNextProcessOpenAfterFailedClose; this shows process
  exit reclaims the store, not that an in-Dolt commit does not land).
- A second Close, a retried Close, or a count-mode Close closes the pools
  under a live commit. Tests: a concurrent Close waits for the first and
  both fail closed; a retried Close under an inline FinalizeBatch commit
  fails closed; a count-mode Close under an inline commit fails closed;
  readDB answers a ping after each
  (TestCheckpointConcurrentCloseFailsClosed,
  TestCheckpointRetriedCloseUnderInlineCommitFailsClosed,
  TestCheckpointCountModeCloseFailsClosedUnderInlineCommit).
- Close waits a grace per wait. Test: one hard deadline bounds the
  checkpoint waits and the fence (TestCheckpointCloseBoundIsOneDeadline);
  it does not bound the telemetry drain, the closeMu wait or the teardown.
- Reopen over an open store orphans its handles. Test:
  TestCheckpointReopenRequiresAClosedStore.
- Close crashes the process over a removed workspace: Dolt's
  crash-on-fatal when the journal cannot flush into a missing .dolt/noms
  (the arch13j mirror panic). Tests, in child processes:
  TestCloseAfterTheWorkspaceIsRemovedReturnsAnError,
  TestOpenFailingAfterTheEngineIsUpReturnsAnError.
- Close closes the engine under a statement that is not a commit (a
  statement in flight on either pool, an open write transaction). Tests:
  Close fails closed, the statement or transaction completes, and its
  rows are durable after a later Close and reopen
  (TestCloseFailsClosedUnderAStatementInFlight,
  TestCloseFailsClosedUnderAReadInFlight,
  TestCloseFailsClosedUnderAnOpenWriteTransaction); a Close failed at
  the fence or at the pin restores the cadence and the sealer
  (TestCloseFailedAtTheFenceRestoresCadence,
  TestCloseFailedAtThePinRestoresCadence).
- The close deadline is a context deadline, not a wall-clock bound on the
  commit itself: DOLT_COMMIT does not check its context mid-write, so a
  commit already running inside Dolt runs to its end. Close does not wait
  past the deadline and the grace; it fails closed and leaves that commit
  its engine. If the process then exits mid-commit, data-safety rests on
  Dolt writing content-addressed chunks and moving the working set and
  HEAD atomically: at worst an unadvanced HEAD and unreferenced chunks,
  never a partly visible commit; the rows stay in the SQL working set and
  the next commit after boot takes them. Not exercised against a real
  wedged DOLT_COMMIT (read from dolt/go, per the reviewer).
- A planned restart reads as failures. A background commit cancelled by
  Close, a closing commit skipped or cut, and a Close that failed closed
  count apart (TestCheckpointPlannedCloseCountsNoFailure), each once per
  commit or per failing Close (TestCheckpointOverrunClosingCommitCountsOnce).
- Count mode drifts from today's cadence. Test: the existing
  `doltCheckpointEvery` test still passes, and the inline counter counts
  exactly one commit per `every` events.
- Replay starts committing per event. Replay path untouched; test it
  stays at zero inline commits.

## FP0 steps

Land after the bunch review as a live red with its own deploy (round 5's
Close fence is unconditional; see `landing_note`), not as an inert
ride-along. Run the acceptance above on a test computer, not the paper,
including the planned restart idle and, if it can be triggered, during
an online DOLT_GC. Put the non-commit fence decision (pin the pools, or
never close the engine in-process) to the owner.
