---
definition_version: 4

# Consolidate and scale: computers that start without a person making room,
# then every started thing finished or deleted, then the residuals.
# Supersedes the ordering of
# docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md, which
# settles checkpoint_incomplete: its first real loop (SR5) is complete, and its
# open stations carry over here by name.
# Owner 2026-10-26, in session (ledger 11:3x to 12:0x): "once we land
# everything we're working on currently, [today] is going to be about
# retracing our steps, cleaning up residuals, and completing all the stuff that
# we started"; "this scalability work is all a priority, above other residuals
# even"; "we should consider hibernating before 30 minutes. If we can restore
# fast we can hibernate idle computers at will"; "We shouldn't need to be doing
# that manual memory management"; "I don't like the policy of building stuff
# and not switching it on"; "some of these ideas were experiments that didn't
# work, so we shouldn't just turn everything on"; "we should plan all this and
# basically make a new artifact ... think about kind of the big picture".
# Authority to execute: the owner ratified this spine at 12:1x ("New
# metamission looks good"), with rulings on PR-12 and SD-01 in the same
# message (ledger 12:1x).
readiness: executable

review:
  reviewer: owner (ratified the spine in session, 2026-10-26 12:1x)
  frozen_ref: this file as first staged, before the 12:1x ruling edits
  verdict: accept
  evidence_ref: docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.ledger.md, entry 12:1x

metamission:
  stations:
    - id: L0
      path: this file (the orchestrator's landing queue)
      readiness: executable
      status: working
      depends_on: []
      note: >-
        Land what is in flight, then stop starting. As of 22:5x: the
        bearer-wake fix (review 339 accepted, one fix round), the rollback
        CLI verb, the delete-dead lane (five dead switched-off features),
        the progressive delivery design (design only), train 27 (built,
        unpushed; D2's next-deploy proof). Trains 13 to 26b landed; see
        the ledger.
    - id: D
      path: docs/definitions/choir-density-2026-10-26.md
      readiness: executable
      status: working
      depends_on: []
      note: >-
        Density first (owner, 12:0x). Evidence:
        scratchpad memory-density-why.md, to land as
        docs/problems/start-refused-for-memory-no-reclaim-2026-10-26.md.
        Ordered by impact: D1 a refused start hibernates the lowest-priority
        idle computer and then admits (admission and reclaim coupled; red;
        the fourth recurrence: 10-13, 10-22, twice 10-26);
        D2 snapshots survive a deploy (red): today every deploy voids every
        snapshot, so with a deploy about every 45 minutes 0 of 8 snapshots
        resumed in 58 hours and every wake was a 10 s cold boot; without D2,
        "hibernate at will" is "cold boot at will";
        D3 idle hibernation reaches primary computers (keepalive
        under-capacity off) and the idle timer drops from 30 min to minutes,
        never a computer with a live run, capsule, apply (red; the paper
        joins under D8);
        D4 admission charges the wake cost (0.3 to 1.4 GiB), not the last
        peak (7.8 GiB for b@c; page cache never returns) (red);
        D5 diff snapshots on (built, RES-13 L3a, off): the resident pages only,
        about 1.4 GiB written in about 1.5 s instead of 8 GiB in 6 to 10 s
        (estimate; on disk btrfs compression already holds a full snapshot to
        0.7 to 1.5 GB); L3b, diff after a restored guest, is unbuilt (red);
        D6 balloon inflation returns guest page cache (unbuilt; red);
        D7 the density bench, run (yellow).
        D8 (owner 2026-10-27 19:1x: "We need snapshotting and hibernation
        everywhere automatically"): automatic checkpoints, snapshots and
        hibernation for every computer, the paper included; first find why
        the host checkpoint service has no job for the paper (its base
        watermark is 34660 against a tape of 4.39M) and measure on d@e how
        long a self-checkpoint holds a computer's appends. Acceptance: on staging, five test computers started in a
        row with no hand stop, each wake under one second, host memory and
        snapshot bytes recorded.
    - id: V
      path: docs/definitions/choir-v-boot-vocab-pass-removal-2026-10-27.md
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        Owner 2026-10-27 19:1x: "when you bring the paper back, skip the
        rescan. Fix it and get rid of that dead code." The boot vocabulary
        full pass held the paper down 17:48 to 19:19 (OOM loop;
        docs/problems/paper-down-on-full-vocabulary-rescan-2026-10-27.md).
        Delete it from boot; rebuilds keep the migration; guests back to
        8 GiB in the same train.
    - id: X
      path: docs/problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md
      readiness: drafted
      status: working
      depends_on: []
      note: >-
        Self-development substrate breaks found today. X1 the apply's
        checkpoint replays the whole tape into /tmp and killed b@c
        (lane apply-replay-bound; red, tier 2). X2 R10-5, a Complete states
        toolchain facts its own output contradicts (design first, red).
        X3 SD-01 stack rebase U4, to build (owner 12:1x: "we need to
        rebase not reject security updates. Security comes first"; red,
        tier 2; design in docs/design/computer-update-architecture-rnd-2026-10-24.md,
        "U4 design"); until it lands FP0 keeps landing applied changes
        before deploys by hand.
        X4 the old metamission's open acceptance leg: restore b@c to a
        checkpoint before an apply and build on the restored stack.
    - id: W
      path: docs/evidence/switched-off-inventory-2026-10-26.md
      readiness: drafted
      status: pending
      depends_on: []
      note: >-
        Switches: built features are on, failed experiments are deleted
        (owner). W1 a missing setting row reads the registered default, not
        off (one read-path change, red), so defaults can flip in code. W2
        flip the defaults of proved features. W3 delete the failed
        experiments, list to the owner first (intake.event_wakes,
        store.trace_log, others named by the inventory). Done: the two
        research economy switches deleted (train 13).
    - id: F
      path: this file (the started work, each with its own file)
      readiness: drafted
      status: pending
      depends_on: [D]
      note: >-
        Finish what started. F1 Library (SR9): the Reader published as the
        first real release, shared across owners (needs a third test
        account; the owner will make it soon), withdrawn; then S4 to S14 in order. F2 related stories by
        shared entities (lane related-review, slices a to e, with the
        standing review schedule, a backfill and a coverage floor in the
        deployed spec). F3 the four desktop themes on b@c (T1c). F4 Slides,
        Files (Open with menu), Calendar, Podcasts (T1a). F5 the window
        manager's phone restore and the Reader's snippet space. F6 CI and
        test co-optimization (CI0). F7 the ingestion gate: every source
        categorized by System 1 and stripped to safe structured text before
        any desk reads it (owner 11:2x); email comes after it.
    - id: R
      path: docs/evidence/residual-register-2026-10-26.md
      readiness: drafted
      status: pending
      depends_on: [D]
      note: >-
        Residuals: the six blocking-real first, then the stale rows
        (built, off or deleted since), then the rest by severity.
    - id: Z
      path: none (deferred by the owner or by order)
      readiness: intent
      status: pending
      depends_on: [F, R]
      note: >-
        Deferred: email (numbered addresses and the autopaper@ inbox,
        docs/design/email-numbered-addresses-and-autopaper-inbox-review-2026-10-26.md;
        owner: tomorrow or later), SR7 recoverable changes, SR8 staged
        auto-approve, T1b Choir in Choir, T2, T3 (U2, U3, U5 to U7);
        PR-12 step-up (owner 12:1x: "tentatively passkeys when needed for
        owner approvals, but let's not build this yet. Currently it's
        fine"; approvals will later reach the owner through Autoradio app
        notifications; "I want to push the autonomy actually (fewer
        approval prompts is better)"; the cookie lifetime is not important
        prelaunch). PR-12 leaves the blocking-real rows as deferred by the
        owner.

start:
  observed_at: 2026-10-26 12:10 UTC
  canonical_ref: origin/main 618727c97 (train 13 in CI); deployed 99c4d9a9a (train 12)
  computers:
    - b@c computer-6c50d6e5…, active since 11:50 (restarted after the apply crash; Reader round 7 operation reads materializing)
    - d@e computer-59c902c5…, stopped 11:50 by FP0 to make room (engineering now Synthetic DeepSeek)
    - the paper computer-1355b83b…, research.related_story on since 11:25
  dirty_worktrees: main checkout M go.sum and untracked scratchpad/ (owner's; untouched)
  open_lanes: related-review (entity layer), apply-replay-bound (X1)
  known_failures:
    - a start can be refused for memory with no reclaim (b@c, 11:49)
    - an apply on a long-tape computer kills its runtime (X1)

finish:
  deliver: >-
    Computers start and wake without anyone making room for them, every
    started piece of work is landed and switched on or deleted, and the
    residual register's blocking rows are closed.
  artifact: >-
    On main and deployed: D1 to D6 with D7's numbers; X1 to X4; W1 to W3;
    F1 to F7 each to its own finish; the residual register with no
    blocking-real row open.
  acceptance:
    - action: >-
        On staging, start five test computers in a row on a host that cannot
        hold them all resident; none is refused, idle ones hibernate in
        minutes, every wake is under one second, and nobody stops a computer
        by hand.
      proves: the density stations remove manual memory management.
      evidence_class: deployed proof
    - action: >-
        The switched-off inventory, re-read: every built switch is on where
        it belongs or deleted, and a missing row reads its default.
      proves: nothing built sits off by accident.
      evidence_class: deployed proof
  rollback: >-
    Each station names its own; density changes are vmctl config and code
    with a per-host switch and git revert.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the hand steps in running computers (memory, switches,
    restarts) and the count of started-but-unfinished work, while keeping
    security first, availability second, customizations third, and the
    restart rules.
  goodharting_would_be: >-
    Raising the host's memory floor or adding RAM instead of reclaiming;
    turning every switch on including failed experiments; closing residual
    rows by rewording them; hibernating a computer that is working.

homotopy:
  realism_axis: >-
    From two test computers on one host, through every test computer at
    once, to owner computers and then the paper under the same policy.

boundaries:
  mutation_class: red
  authority_sources:
    - the owner's statements in this file's header and the ledger of 2026-10-26
    - docs/choir-doctrine.md, CLAUDE.md
  must_preserve:
    - a computer stays on while it is working; crash restarts close work, planned restarts resume
    - the paper and the owner's computers are never used for adversarial or proof steps
    - security first, availability second, customizations third
    - guests do not swap (owner ruling 2026-10-09)
    - Texture is the sole agent writer of documents; one home and one writer per kind of data
  excluded:
    - email (deferred by the owner)
    - node-a changes
  protected_surfaces:
    - vmctl admission, reclaim and hibernate (D)
    - self-development apply and checkpoint (X)
    - settings read path (W1)

now:
  status: superseded
  slice: >-
    Data architecture selection
    (docs/definitions/choir-data-architecture-selection-2026-10-28.md,
    drafted): census, owner budgets, candidates, tape-replay bake-off,
    panel, shadow migration. Dolt versioning is load-bearing nowhere found.
  bridge_conjecture: >-
    If immutable content leaves the per-computer Dolt stores (tape as truth,
    content-addressed host store, small authority store), then memory,
    disk, contention and checkpoint cost fall by structure, and the host
    serves more computers per byte. It fails if the census shows the
    paper's bulk is authority state, not duplicated immutable content.
  belief:
    believed_state: >-
      Updated 2026-10-28 12:2x UTC. Landed and deployed (staging
      f7e215e54): S1, S2 (the paper's first base live_captured seq 4526210),
      the witness spill, the capture inode pin, S3, S4, S5, S6, V, the
      switch registry; b@c 8 GiB start observed. The paper's /health
      reports store_stalled (online GC on the write connection, then
      engineMu); the Store A squash OOM'd at 10G and was restored; the old
      seed is retired (reversible).
    main_uncertainty: >-
      What dominates the paper's 26.5 GB and Store A's 51 GB (the
      tape-index event_json hypothesis is unmeasured), and which squash
      phase holds more than 10G.
    next_observation: >-
      Per-table sizes of the paper's captured base and Store A, read-only.
  open_decision: >-
    Owner questions in docs/design/data-rearchitecture-2026-10-28.md: where
    Dolt's versioning is load-bearing; dedup scope; Autoradio after step 4.
  blocker: >-
    Token budget (owner, 2026-10-28: tokens return tomorrow). b@c citation
    redo waits for self-development with the owner's Claude Code delegate.
  next_action: >-
    Run the kill -9 drill for store.debounced_checkpoint on a throwaway
    test computer and walk it from R0; take the per-table size census;
    measure the Store A squash per phase on a reflink copy under a 24G cap
    and quote SQL_CHECK_CMD; then finish R7 (trace out of the store).

## Terminal receipt (2026-10-28 ~12:50 UTC): dropped, superseded

The owner dropped this goal. Landed and deployed on its watch (staging
0bf89ed84): S1, S2 (the paper's first base, live_captured seq 4526210),
the witness spill, the capture inode pin, S3, S4, S5, S6, V, the switch
registry, the squash quoting fix; the b@c 8 GiB start observed. Carried
forward, not done: the paper's store_stalled and the Store A squash memory
go to docs/definitions/choir-data-architecture-selection-2026-10-28.md
(its parallel track: the kill -9 drill for store.debounced_checkpoint, the
squash per-phase measurement, R7); the b@c citation redo waits for
self-development (docs/problems/cite-promote-expands-all-citations-2026-10-28.md);
stations D7, X4, W walks, F and R remain as recorded above for a later goal.
