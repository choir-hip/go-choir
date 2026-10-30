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
  evidence_ref: docs/archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md (panel findings P1, P2)

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
    engine-concurrency design, docs/archive/design/store-engine-concurrency-2026-10-21.md;
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
    (docs/archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md,
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
    - docs/archive/problems/stored-column-add-blocks-paper-boot-2026-10-21.md
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
    Follow-up (2026-10-22, lane/fp1-event-table-d2): F-c deploy 2, the
    writer, on deploy 1 (c4508ef82), tests first. With store.event_table
    on, the writer stamps ProjectionOp.Table "choir_events" on the ten
    batched kinds, both on the acknowledged write and in the sealer. The
    switch needs two keys. The host names the computer
    (CHOIR_EVENT_TABLE_COMPUTERS, ComputerIDs only, empty as deployed; a
    grant on the credential disk at cold boot). The owner sets the setting
    (POST /api/computer-settings; applied at once and at boot; refused to
    ProductAPI, in-guest callers and unauthenticated callers). Off is
    deploy 1 byte for byte. F17 (local): b_ce/b_og 0.21 per op (bar 0.5,
    pass); per event with batching off, 0.52 (the per-tape-event overhead
    is unchanged). Round 2 (rebased on deploy 1's round 1b fbb6d74a8):
    the store floor (store-floor.json, written before the first stamp,
    never lowered; Open, Apply and the updater's recovery refuse a binary
    below it), decode outside the engine lock, cursor reads that cannot
    skip between their halves, and a CI guard against lowering the epoch
    (script in the lane; the workflow step is FP0's). Deploy 1 lands
    first; earlier slices: deploy 1, the boot-window repair (deploy E), B2.
    Round 2b (2026-10-23, bunch 148, Devin and DeepSeek both ACCEPT),
    rebased on stage/deployD10 (main plus D9 plus deploy 1 round 1e;
    keep-both conflicts in computer_setting.go, api.go, runtime.go,
    store.go): the CI guard now fails closed on an unreadable non-zero
    base (fetch retry, then exit 1; selftest-store-schema-floor; the
    evidence patch is regenerated against the current ci.yml and runs the
    selftest); the below-deploy-1 rollback commit carries
    Store-Floor-Override and its pre-rollback reading (valid only while
    every computer reports event_table.store_floor == 0), and rehearse
    runs the guard over it; RestagePinnedRelease and ImportBaseline run
    the floor check before swapping current (a baseline that declares no
    epoch still passes there: genesis baselines stage SPA bytes); a
    same-value POST re-applies store.event_table and telemetry.batch; the
    status reads the store's live switch; the writer refuses an unbound
    store and a floor above the build epoch; Reopen floors the adopted
    workspace before adopting it; ReadGrant requires basis computer.
    Round 2c (bunch 152, Devin ACCEPT with one fix): the restage floor
    check now covers the live workspace as well as the staged one (on
    the genesis-replay path the staged floor file does not exist at
    check time); the Reopen ordering test observes the floor before the
    handle swap; the CI patch notes it lands with or after deploy 2's
    scripts. Residuals: undeclared-epoch baselines pass the restage and import
    paths; the CI override trailer is push-only; platform-follow offers
    still declare no epoch.
    Round 3 (2026-10-23, lane/fp1-event-table-d2-r3, on main 5669e22fc
    with D24's stream_seq lease): the 32 deploy-2 commits ported by
    cherry-pick (deploy 1's 25 are on main through D10 and were skipped);
    conflicts in store.go only (the lease's allocateEventSeqLocked
    replaces the inline seeds; Open's closeFailedOpen; Reopen keeps the
    lease resume beside the floor). The lease gate the lease design owes
    this lane (section 4 "When"): the writer stamps only once the lease
    state is ready with its latest lease row written through a bound
    tape (lease.onTape, recorded at the seed and refill writes), and POST
    store.event_table=true answers 409 (stream_seq_lease_not_ready) and
    stores nothing before that. A stored value arms the switch at boot;
    the writer begins when the eager seed lands, and the status says it
    waits. The proof's step 2 reads the lease ready before the switch;
    step 5 asserts seeded_from=lease after the restart, with no
    exact-seed read (design section 6 step 3).
  source_ref: lane/fp1-event-table-d2-r3
  deploy_identity: this lane deploys nothing; FP0 lands and proves
  candidate:
    id: fp1-event-table-deploy2
    state: built, not landed; waits for deploy 1 to land and hold (owner Q5)
    ref: lane/fp1-event-table-d2-r3
    base: origin/main with D24 (the stream_seq lease); was stage/deployD10 (010cb1a29), fbb6d74a8
    digest: none
    scope:
      - internal/store/event_table.go
      - internal/store/store.go
      - internal/store/computer_setting.go
      - internal/store/telemetry_sealer.go
      - internal/store/telemetry_stats.go
      - internal/eventtablegrant/grant.go
      - internal/vmmanager/event_table_grant.go
      - internal/vmmanager/config.go
      - internal/vmmanager/manager.go
      - nix/node-b.nix
      - internal/agentcore/event_table.go
      - internal/agentcore/runtime.go
      - internal/agentcore/computer_settings.go
      - internal/agentcore/api.go
      - internal/storeschema/schema.go
      - internal/storeschema/floor.go
      - internal/updater/updater.go
      - scripts/check-store-schema-floor.sh
      - scripts/selftest-store-schema-floor
      - scripts/rollback/fc-deploy1-rollback.sh
      - docs/evidence/fc-deploy2-ci-schema-floor-step.patch
      - scripts/proofs/fp1-event-table-deploy2.sh
  conjecture:
    id: C-FP1-event-table
    claim: >-
      Moving batched telemetry from the 13-tree og_objects to the 4-tree
      choir_events cuts its journal per op by more than half, and the
      paper's whole journal by 20 to 35%.
    test: >-
      Locally (F17, directional): per op 87.6 to 18.5 KiB (0.21); per event
      unbatched 142.5 to 74.4 KiB (0.52). On staging the fit b_ce/b_og <=
      0.5 (FP0, proof steps 3 and 7); on the paper, readings 1 to 6 before
      and after (proof step 8).
    edge: missing_oracle
    delta_o: the observability counter tape_ops_by_table and the store-bytes snapshots make the fit possible per computer
    scope_if_supported: the guest store of any computer the host names
    status: testing
    evidence_refs: [docs/evidence/fp1-event-table-journal-probe-2026-10-22.json]
  decision:
    what: >-
      Two keys for the writer: the host's per-computer grant (ComputerIDs
      only) and the owner's setting. This is the owner's "observe
      everywhere, enforce on one test computer" pattern. Each computer that
      stamps makes deploy 1 its floor, so that is a platform decision.
    kind: architecture
    status: proposal
    evidence_ref: docs/archive/design/choir-event-table-f-c-2026-10-22.md (Status, deploy 2)
    owner_ratification_ref: the owner's section 8 defaults (2026-10-22) and FP0's brief naming the selector pattern
  belief:
    believed_state: >-
      The writer is correct and inert as deployed. Locally the per-op
      saving is larger than predicted (sequential keys share leaves within
      a batch). Unbatched, the saving is about 48% per batched event.
    main_uncertainty: the paper's batched share of ops and its row share of an event's cost (design section 9)
    next_observation: FP0, the test computer's fit (proof steps 3 and 7)
  blocker_or_risk: >-
    Deploy 1 must land and hold first (one staging day and one paper boot).
    Enabling on the paper needs it named in the selector and a cold boot (a
    planned refresh) before its setting can act. Once any op is stamped on
    a computer, deploy 1 is that computer's rollback floor. Read as journal
    per event (on/off) with batching off, deploy 2 sits at the bar (0.52),
    not under it; the design's gate is per op (0.21). A floored computer
    refuses platform-follow offers until offers declare a store schema
    epoch; decide before the paper is floored. A floored computer whose new
    release fails its health probe stays on it (no recovery below the
    floor) until a forward fix.
  next_action: >-
    FP0: after deploy 1 has landed and held, review deploy 2 in a bunch
    (red: AppendEvent and the sealer, vmctl credential-disk material, the
    runtime boot step and settings route), mirror under -race, and land it
    as its own deploy. Before it, land the CI store-floor step (the
    design's deploy 2 status has the YAML) in a commit of its own. Then
    run scripts/proofs/fp1-event-table-deploy2.sh
    step 0 on the paper (no effect). Mint a test computer, name it in
    CHOIR_EVENT_TABLE_COMPUTERS (a nix commit, deployed), cold boot it, and
    run steps 1 to 7. The paper comes only after that and the panel, under
    step 8's conditions. Deploy 3 (the multi-row insert) is next for this
    lane once deploy 2 has staging numbers.
  followup: >-
    Journal growth (lane/fp1-journal-growth, 2026-10-22): problem record
    docs/archive/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md.
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
  - id: fp1-event-table-deploy2
    boundary: implement
    identity: >-
      lane/fp1-event-table-d2, rebased on fbb6d74a8: cafbc9e29 and
      d810f6b2f (tests, yellow), 9cd9abb8b (store writer, red), cfe13553c
      (host grant, red), 931a6f8f2 (runtime switch, red), 4c83665cc (proof,
      yellow), e3134069f (docs, green); round 2: 9b9434cfe (findings
      recorded, green), 967cf6595 (tests, yellow), 9f3895f95 (floor,
      red), 3af82d7d5 (store floor, decode, cursor, red), 0183d3a64
      (updater, red), 31a5d25b5 (runtime, red), 6680db093 (CI script,
      orange), the proof update (yellow), then the round 2 docs (green)
    proof_refs:
      - internal/store/event_table_writer_test.go
      - internal/store/event_table_guard_test.go (TestEventTableOnlyTheWriterStamps)
      - internal/store/event_table_journal_probe_test.go (F17)
      - internal/eventtablegrant/grant_test.go
      - internal/vmmanager/event_table_grant_test.go
      - internal/agentcore/event_table_setting_test.go (F18)
      - internal/store/event_table_floor_test.go, internal/storeschema/floor_test.go, internal/updater/store_floor_test.go (finding A)
      - internal/store/event_table_reads_test.go (findings B/F, E, G)
      - docs/evidence/fp1-event-table-journal-probe-2026-10-22.json
      - scripts/proofs/fp1-event-table-deploy2.sh
    rollback_ref: the setting off (live); git revert of the three red commits returns to deploy 1, which projects and reads every stamped op
    disposition: built and tested locally, not landed; see the design's deploy 2 status for the tests run
  - id: fp1-event-table-deploy1
    boundary: implement
    identity: >-
      lane/fp1-event-table on 6c60685cc: 2b30186a9 (tests, yellow);
      e31a14260 (DDL, storage class, epoch 2), 3a75441f7 (projector route),
      23d1a4cae (union readers and seeds), 220698ce1 (witness, manifest,
      fingerprints, fences), e287b6e7e (deletions), 21718f62f (plan-shape
      fix), all red; 04d055014 and 17736c304 (test repairs, yellow);
      3900eed5c (proof script, yellow); c4508ef82 (docs, green). Round 1b:
      2ea0613a9 and 0e8f72cad (F14 lock pin, stamp guard, F6 hook),
      3fe5e810f (rollback recipe rehearsal, proof step 6), fbb6d74a8
      (round 1b docs). Round 1c: the type-based stamp guard and the
      test-only hook guard (d5cc7dcde), the rollback artifact and script
      (e07613ae5), bf56262e7 (round 1c docs). Round 1d: 03ec9e0f5 (guard:
      container conversions, type errors, pinned package set), 051bc0af2
      (rollback survives its revert: apply --from, rehearse, exact table
      name), then the round 1d docs commit
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
    proof_refs: [docs/archive/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md]
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
    proof_refs: [docs/archive/design/store-engine-concurrency-2026-10-21.md, internal/objectgraph/engine_concurrency_probe_test.go]
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
