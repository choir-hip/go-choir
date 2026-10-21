---
definition_version: 4

# Station FP1 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md). Batch B2: L1-01,
# SDC RES-18, L3-05. The orchestrator (FP0) lands; this lane builds.
readiness: drafted

review:
  reviewer: none yet; the orchestrator sends the frozen red candidate to a panel before it lands
  frozen_ref: none
  verdict: none
  evidence_ref: none

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
  status: working
  slice: problem record and this file (commit 1), then tests that fail, then the side table
  source_ref: f399a2e8c
  deploy_identity: f2c4a3ec3 serving
  candidate:
    id: none
    state: none
    ref: lane/b2-store
    base: f399a2e8c
    digest: none
    scope: []
  conjecture:
    id: C-FP1-sidetable
    claim: >-
      A side table maintained by every writer in its transaction and
      repaired online by a hash-comparing validator answers the three reads
      exactly as the JSON scans do, and takes the engine mutex from 70% to
      about half.
    test: >-
      The differential and rollback-simulation tests locally; the engine-mutex
      counters on the paper before and after (FP0).
    edge: missing_oracle
    delta_o: the validator logs its pass and the tripwire counts hash mismatches, so a missed writer shows as a number
    scope_if_supported: the guest object store on a computer with the same writers
    status: active
    evidence_refs: []
  decision:
    what: >-
      Persisted side table, in-memory validated flag, read-time tripwire; the
      extractors skip the table; replay does not maintain it.
    kind: architecture
    status: proposal
    evidence_ref: docs/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md
    owner_ratification_ref: the orchestrator's panel on the frozen candidate
  belief:
    believed_state: >-
      The three scans held the engine 40.1 s of the 122 s window (33% of
      wall time, 47% of the 85 s it was busy); GetObject's 150 s of waiting
      is queueing behind them.
    main_uncertainty: >-
      Whether the validator pass on the paper's real row counts is short
      enough, and whether a join-free two-step read keeps its plan on the
      embedded engine.
    next_observation: plan-shape and 20,000-row timings in the worktree
  blocker_or_risk: >-
    The panel's findings on the first design are not in the committed
    problem doc ("panel pending"); the design here follows the brief. A new
    table makes an older binary replay-ineligible after a rollback.
  next_action: write the failing tests

receipts: []
---

# FP1: store and daemon throughput

The work and its reasons are in the problem record named in `now`. Order:
(1) the three reads on a rollback-safe side table; (2) progress logs for
every boot-path store step; (3) the engine-concurrency design, implemented
only if the design is clear and testable in this batch, otherwise named as
the next batch; (4) L3-05 if time remains. The orchestrator's measurement of
17:58 adds one cheap item: label `GetObject` and `ListObjectsPage` by kind.
