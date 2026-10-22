---
definition_version: 4

# Station FP1 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md). Batch B2: L1-01,
# SDC RES-18, L3-05. The orchestrator (FP0) lands; this lane builds.
readiness: drafted

review:
  reviewer: FP0's panel, GPT-6.1-sol and Gemini 4 Argon (2026-10-21)
  frozen_ref: 8cadecc7e
  verdict: >-
    sol send back (A1 boot ALTER, A2 withdrawal races), Argon accept (A2 as
    non-blocking). FP0: fix A2 and the malformed-family recovery on
    lane/b2-store-b1fix; A1 is pre-existing, recorded, not fixed here.
  evidence_ref: docs/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md (panel findings P1, P2)

start:
  captured_at: 2026-10-21T18:10:00Z
  source:
    canonical_ref: origin/main f399a2e8c (lane/b2-store)
    deploy_identity: >-
      staging serves f2c4a3ec3 (the revert of the stored columns), c893a956a
      deploying. The paper (vm-1669020a) booted 17:10:07 UTC; engine mutex
      held 70% over 122 s from 17:58:57 with 247.6 s of waiting.
  worktrees:
    - path: scratchpad/lanes/b2-store
      status: clean
      class: goal_candidate
      owner: FP1
      touch: goal_owned
      recovery: branch lane/b2-store from f399a2e8c; nothing outside it is touched
    - path: /Users/wiz/go-choir
      status: dirty
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      recovery: leave in place

finish:
  deliver: >-
    The Autopaper computer's store spends well under half its time on JSON
    scans, a schema addition can never again hold its boot, and the way to
    more desk slots is written down with its evidence.
  artifact: >-
    On lane/b2-store: (1) a rollback-safe side table og_key_index with
    writer maintenance in the same transaction, an online validator, and the
    three reads (run by agent_id, records by addressee, source_ref by doc_id)
    on it only when validated, otherwise on the unchanged JSON path;
    (2) start/end/duration logs for every store step before the listener;
    (3) GetObject and ListObjectsPage labelled by object kind; (4) the
    engine-concurrency design, docs/design/store-engine-concurrency-2026-10-21.md;
    (5) optionally L3-05.
  acceptance:
    - action: go test of the new internal/objectgraph and internal/store tests (differential reads, writer maintenance, rollback simulation, writer storm, plan shape, open-with-many-rows, no-scan op counts), then internal/store, internal/objectgraph, internal/computerversion and internal/projectionbase once each through the heavy wrapper
      proves: the side reads equal the JSON reads, every writer maintains the table, a stale table is repaired online, and boot does no work that scales with og_objects
      evidence_class: local test
    - action: after the deploy, the paper boots in under two minutes; its engine-mutex counters over 120 s show the three scans gone and the validator's pages short
      proves: the batch moves the measured cost and does not hold the boot
      evidence_class: deployed proof (FP0)
  rollback: >-
    git revert of the batch commits. The side table stays on the paper's
    database, additive; an older binary lists it as an unknown table and
    reports replay-ineligible until it is dropped by hand
    (docs/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md,
    finding 3). Forward again repairs it online.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the engine mutex's busy share and the desk wake wait on the
    paper while preserving every read's result, the boot's bounded time, and
    rollback to the previous binary.
  goodharting_would_be: >-
    Reads that are fast because they return less (a missed writer, a stale
    table trusted), a backfill that finishes by holding the engine for
    minutes, a flag that says complete without a comparison, or a boot that
    is fast because the work moved to the first read.

homotopy:
  realism_axis: >-
    A focused test on an embedded Dolt store in the worktree, then the same
    store with tens of thousands of rows, then the frozen candidate on
    staging's paper under its real load, then a day of the paper's polls.

boundaries:
  mutation_class: red (object store schema and every og_objects writer; boot path); the labels, logs and design are orange or green
  authority_sources:
    - the metamission and the orchestrator's brief (2026-10-21)
    - CLAUDE.md, docs/standing-questions.md
    - docs/problems/stored-column-add-blocks-paper-boot-2026-10-21.md
    - docs/problems/research-more-slow-and-hidden-2026-10-21.md (E6)
  must_preserve:
    - Nothing on the boot path scans or rewrites og_objects; the validator starts after the listener is up and holds the engine only per page.
    - Today's read semantics: LEFT(..., 255) with the 255-character bypass, binary NO PAD collation, an error on a malformed body, tombstones, the JSON path for every case the side table cannot answer.
    - A failed derivation or side-table error never fails a write.
    - The four names og_meta_agent_id, og_meta_capture_id, og_meta_doc_id, og_rec_addressee are never used, dropped or rewritten.
    - Restarts end work; no boot-time resumer without a marker (the validator is idempotent derived-data repair, not work).
    - No push, no merge, no other branch, no secrets, no staging.
  excluded:
    - Raising the dispatcher slot count or removing engineMu before the design is measured.
    - Changing the replay manifest's classes for existing tables.
  protected_surfaces: [object store schema, og_objects writers including the production projector, replay and witness extraction, boot path]

now:
  status: checkpoint_incomplete
  slice: >-
    Follow-up (2026-10-22, lane/fp1-event-table): F-c deploy 1, run
    telemetry in its own table, inert. Built tests first from the design's
    section 5 on origin/main 6c60685cc after the owner approved the
    section 8 defaults ("Defaults are fine"): choir_events (4 trees), the
    projector route on ProjectionOp.Table, the union readers and both seeds,
    the witness rule (an empty choir_events is not observed) and manifest
    class (version 4), both vocabulary fences, store schema epoch 2, the
    three caller-less methods deleted. No writer stamps an op: deploy 2 (the
    writer behind store.event_table) is not built. Earlier slices: the
    boot-window repair (deploy E), B2 (bed24b526).
  source_ref: lane/fp1-event-table
  deploy_identity: this lane deploys nothing; FP0 lands and proves
  candidate:
    id: fp1-event-table-deploy1
    state: built, not landed; awaiting FP0's review bunch and mirror
    ref: lane/fp1-event-table
    base: 6c60685cc
    digest: none
    scope:
      - internal/store/event_table.go
      - internal/store/store.go
      - internal/store/project.go
      - internal/store/computer_events.go
      - internal/store/graph_store.go
      - internal/store/object_graph_fingerprints.go
      - internal/store/telemetry_sealer.go
      - internal/store/vocab_migrate_og.go
      - internal/store/vocab_replay_fence.go
      - internal/objectgraph/dolt_store.go
      - internal/types/event_storage.go
      - internal/storeschema/schema.go
      - internal/computerversion/dolt_state_extractor.go
      - internal/agentcore/replay_eligibility.go
      - internal/agentcore/system1_verb.go
      - scripts/proofs/fp1-event-table-deploy1.sh
  conjecture:
    id: C-FP1-sidetable
    claim: >-
      A side table maintained by every writer in its transaction and
      repaired online by a hash-comparing validator answers the three reads
      exactly as the JSON scans do, and takes the engine mutex from 70% to
      about half.
    test: >-
      The differential and rollback-simulation tests locally (passing);
      the engine-mutex counters on the paper before and after (FP0).
    edge: missing_oracle
    delta_o: the validator logs its pass and the tripwire counts hash mismatches, so a missed writer shows as a number
    scope_if_supported: the guest object store on a computer with the same writers
    status: testing
    evidence_refs: [receipts]
  decision:
    what: >-
      Persisted side table, in-memory validated flag, read-time tripwire; the
      extractors skip the table; replay does not maintain it and withdraws the
      families. L3-05: T1 was already landed (1db03dacf), T2 is left as is.
    kind: architecture
    status: proposal
    evidence_ref: docs/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md
    owner_ratification_ref: the orchestrator's panel on the frozen candidate
  belief:
    believed_state: >-
      The boot-window hold was the newest-first walk in
      LatestComputerEventByIdempotencyPrefix after ready; the range read
      costs the family's size (plan test).
    main_uncertainty: whether another long s.db holder remains after this read is gone
    next_observation: >-
      FP0, next paper boot window: /health 200 within seconds of "runtime:
      started"; if not, the probe's goroutine dump.
  blocker_or_risk: >-
    Lands only after the host lane's vmctl rollback-reachability fix. A
    rollback to a binary without the extractor change leaves og_key_index as an
    unknown table to that binary (replay-ineligible until dropped).
    Residual, pre-existing and not fixed here: boot can ALTER og_objects
    STORED on a populated store missing an older generated column
    (docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md);
    FP0 can check the paper with the query in that record before the deploy.
    Residual: the validator's COUNT and stray-id lists are not keyset-paged.
  next_action: >-
    FP0: review the deploy-1 set in a bunch (red: store schema, epoch,
    projector, replay, witness, manifest, fences, event readers), mirror it
    under -race, land it as its own deploy (owner Q5; the metamission's
    one-store-batch-per-deploy rule), then run
    scripts/proofs/fp1-event-table-deploy1.sh (step 2 --label before ahead
    of the deploy). Keep deploy 1 at least one staging day and one paper
    boot before deploy 2. Next lane: deploy 2 (the writer and
    store.event_table, tests F17 and F18 first; F17's local ratio at most
    0.5 gates it). Rollback below deploy 1: git revert is safe while no op
    is stamped, but the updater refuses the epoch-1 release on any store
    this build opened; once deploy 2 stamps, deploy 1 is the floor (design
    4.5 and its status section).
  followup: >-
    Journal growth (lane/fp1-journal-growth, 2026-10-22): problem record
    docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md.
    FP0 asked for the dry-run fix; the tests-first measurement on the real
    tape harness showed the dry run costs about 5% (133/141/136 KiB per
    event without/with/without), because finalize rewrites the same chunks
    and the journal stores a chunk once. Not built; the record carries the
    correction. Measured instead: telemetry batching 141 -> 97 KiB per event
    at 6 ops per batch, floor 91 KiB (the og_objects row, 13 trees, one
    statement per op); a multi-row statement cuts a row 100 -> 63 KiB.
    Next: F-a (telemetry.batch on a test computer), then F-e (multi-row
    og_objects upserts in projectBatch, red, small), then F-c as a design;
    FP0's six readings in the record.

receipts:
  - id: fp1-event-table-deploy1
    boundary: implement
    identity: >-
      lane/fp1-event-table on 6c60685cc: 2b30186a9 (tests, yellow);
      e31a14260 (DDL, storage class, epoch 2), 3a75441f7 (projector route),
      23d1a4cae (union readers and seeds), 220698ce1 (witness, manifest,
      fingerprints, fences), e287b6e7e (deletions), 21718f62f (plan-shape
      fix), all red; 04d055014 and 17736c304 (test repairs, yellow);
      3900eed5c (proof script, yellow); then this docs commit (green)
    proof_refs:
      - internal/store/event_table_test.go
      - internal/store/event_table_guard_test.go
      - internal/store/event_table_vocab_test.go
      - internal/store/event_table_old_binary_test.go
      - internal/computerversion/dolt_state_extractor_test.go (TestDoltExtractorOmitsAnEmptyOptionalTable)
      - internal/agentcore/replay_eligibility_test.go (TestReplayEligibilityOptionalEventTable)
      - internal/updater/state_compat_test.go (TestApplyRefusesPreEventTableReleaseOnEventTableStore)
      - scripts/proofs/fp1-event-table-deploy1.sh
    rollback_ref: git revert of the lane's red commits (safe while no op is stamped)
    disposition: >-
      built and tested locally, not landed. internal/store whole package
      passes (one repair found by it, 17736c304); the F-c, fence, telemetry
      and call-site tests pass under -race; projectionbase, selfdevprotocol,
      apihandler, computerversion, updater, types, storeschema, objectgraph
      and computerevent pass; agentcore and textureowner pass on the
      replay, restore, checkpoint, acceptance, WS, run-events, System1 and
      diagnosis filters. scripts/check-*.sh pass except
      check-guest-data-fstrim.sh, which needs the nix binary this VM lacks
      (environmental, unrelated).
  - id: fp1-boot-window-repair
    boundary: implement
    identity: b6523c170, 37cab03d4 (tests), 40cdc19fc (fix), 0084139d6 (record)
    proof_refs: [internal/store/computer_event_prefix_test.go]
    rollback_ref: git revert 40cdc19fc
    disposition: landed and deployed in deploy E (d991907e6); boot to healthy about 30 s
  - id: fp1-boot-window-reading
    boundary: define
    identity: 29c175738 (problem record append), 601e90c2a (probe)
    proof_refs: [docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md, scripts/test_paper_boot_window_probe.py]
    rollback_ref: git revert 601e90c2a 29c175738
    disposition: docs and read-only tooling; no runtime change
  - id: fp1-journal-growth
    boundary: define
    identity: lane/fp1-journal-growth (problem record commits)
    proof_refs: [docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md]
    rollback_ref: git revert of the record commits (docs only)
    disposition: problem documented, causes ranked, fixes proposed; nothing implemented
  - id: fp1-side-table
    boundary: implement
    identity: aa1e2a4f4 (objectgraph), 14c1a62db (store wiring), 3aa4b543e (extractors)
    proof_refs:
      - internal/objectgraph/derived_keys_test.go
      - internal/store/derived_keys_test.go
      - internal/computerversion/dolt_state_extractor_test.go
    rollback_ref: git revert 3aa4b543e 14c1a62db aa1e2a4f4
    disposition: built and tested locally; not landed
  - id: fp1-boot-logs
    boundary: implement
    identity: 17778b6e6, 8829596b3
    proof_refs: [internal/store/boot_steps_test.go]
    rollback_ref: git revert 8829596b3 17778b6e6
    disposition: built and tested locally; not landed
  - id: fp1-engine-concurrency-design
    boundary: define
    identity: 636612d3e, b630883d0
    proof_refs: [docs/design/store-engine-concurrency-2026-10-21.md, internal/objectgraph/engine_concurrency_probe_test.go]
    rollback_ref: git revert 636612d3e
    disposition: design landed in the lane, nothing implemented; next batch named
  - id: fp1-panel-fixes
    boundary: implement
    identity: d3921df4a (problem record), 655262e1b (failing tests), e1814ff86 (fix); adae3e779 (A1 record)
    proof_refs:
      - internal/objectgraph/derived_keys_test.go (TestPassPublication*, TestReadSees*)
      - internal/store/derived_keys_test.go (TestRepairedMalformedRowRestoresTheSideRead)
      - docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md
    rollback_ref: git revert e1814ff86 655262e1b
    disposition: >-
      built and tested locally: the five new cases failed before e1814ff86
      and pass after; objectgraph and store derived-key tests pass under
      -race; internal/objectgraph and internal/store whole pass; not landed
  - id: fp1-health-bounded
    boundary: implement
    identity: 4b4656f44 (problem record F1, green), d375134b6 (failing tests, yellow), 8550227a4 (fix, orange), e870e721f (bunch 18 tests, yellow), 579362834 (updater, orange), 2722cf27d (ci.yml, orange); branch lane/fp1-health-bounded rebased on d23dfaa2d
    proof_refs:
      - internal/agentcore/api_health_bounded_test.go (TestHandleHealthAnswers503NamingHolderWhenStoreStalled, TestHandleHealthUnheldStoreAnswers200WithUnchangedBody, TestHandleHealthStalledStoreLeaksNoGoroutines)
      - docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md (F1, the caller table)
      - internal/updater/runtime_probe_store_stall_test.go (TestHTTPHealthProberWaitsOutAStoreStallAsLongAsTimeoutsDid, TestHTTPHealthProberStoreStallFailsAtTheTimeoutBudget)
    rollback_ref: git revert 2722cf27d 579362834 8550227a4
    disposition: >-
      built and tested locally: the two stall tests failed at their bound
      before fda627de8 and pass after (-race, count 3); the 200 baseline
      passes on both. Not landed. FP0: land (orange, independent of B2),
      then sample /health during the next boot window or online GC pass: it
      answers in about 1 s, 200 or 503 store_stalled with the holder, never
      000. Bunch 18 (Gemini flash, Devin accept; DeepSeek accept with
      edits): its High is repaired. The updater charges store_stalled by wall
      time (579362834), and CI sleeps out a fast store_stalled (2722cf27d).
      Its leak-test flake risk is repaired by counting only readHealthStore
      goroutines (e870e721f). Its other Low (an in-flight statement holds
      the connection past the deadline) is documented and left as is.
---

# FP1: store and daemon throughput

The work and its reasons are in the problem record named in `now`. Order:
(1) the three reads on a rollback-safe side table; (2) progress logs for
every boot-path store step; (3) the engine-concurrency design, implemented
only if the design is clear and testable in this batch, otherwise named as
the next batch; (4) L3-05 if time remains. The orchestrator's measurement of
17:58 adds one cheap item: label `GetObject` and `ListObjectsPage` by kind.
