---
definition_version: 3

start:
  captured_at: 2026-09-18T22:30:00Z
  source:
    canonical_ref: main@de9ed92d
    deploy_identity: c592331b1f9318de013a54faaa162b6735e40342
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_owned
      owner: orchestrator
      touch: goal_owned
      recovery: main

finish:
  deliver: >-
    The platform-dolt store stops re-growing toward disk exhaustion: per-mutation
    DOLT_COMMIT is replaced by batched commits, the canonical event log is
    isolated from world-wire/corpus churn into its own store, and fat payloads
    are content-addressed into platform-artifacts — while the immutable
    auditable event log and autoputer recovery are preserved throughout.
  artifact: >-
    A running platform where (a) dolt_log stays flat under load (batched
    commits), (b) the canonical event store and the world-wire/corpus store run
    as separate sql-server processes with independent retention, and (c)
    items/og_objects fat payloads resolve through platform-artifacts CAS
    digests — verified by a restore drill proving an autoputer rebuilds from
    checkpoint + event tail.
  acceptance:
    - action: dolt_log count on Node B after a load window post-Move-1
      proves: commit batching stopped per-mutation commit growth
      evidence_class: deployed observation
    - action: kill-9 crash drill on staging — COMMIT without DOLT_COMMIT, restart, verify rows
      proves: Dolt 2.1.9 working-set durability (Move 1 safety gate)
      evidence_class: staging proof
    - action: two sql-server processes serving platform (Store A) and corpus (Store B); event queries hit A only
      proves: authority split landed with process isolation
      evidence_class: deployed observation
    - action: restore drill — install base_ref snapshot, replay (watermark, head], compare state commitment
      proves: autoputer recovery invariant preserved across the split
      evidence_class: staging/deployed proof
    - action: items.body / og_objects.body resolve via content_hash into platform-artifacts
      proves: fat payloads externalized to CAS
      evidence_class: deployed observation
  rollback: >-
    dump-20260918 (20G SQL, pre-split full dump) is the rollback ref; per-move
    rollback is git revert + redeploy for code, and restore-from-dump for the
    store. Store A bulk tables retained read-only until Store B verified.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize dolt_log commit growth and noms journal/oldgen bytes while
    preserving the immutable event log (computer_event_append_receipts +
    platform-artifacts pins) and the watermark+snapshot recovery path.
  goodharting_would_be: >-
    Squashing the store once and declaring victory — the 08-26 and 09-18
    squashes both "fixed" disk while the per-mutation commit leak kept
    re-growing. A flat dolt_log under load, not a small store at one instant,
    is the proof.

homotopy:
  realism_axis: >-
    staging rehearsal (disposable store copy, kill-9 drill, dump-split dry run)
    -> production cutover on the live platform store. Each rehearsal must
    deform continuously into the real operation — same dump tooling, same
    commit-batching code path, same restore drill — not a mock.

boundaries:
  mutation_class: red
  authority_sources:
    - owner direction 2026-09-18 (durable fix for the storage leak)
    - docs/designs/platform-dolt-storage-normalization-2026-09-18.md
    - docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md
  must_preserve:
    - immutable auditable event log (computer_event_append_receipts rows + CAS pins)
    - autoputer recovery via watermark base_ref snapshot + event tail replay
    - computer-scoped og_* kinds readable during restore (Store A authority)
    - world-wire/corpus as its own source of truth (Store B backup identity)
  excluded:
    - event-log truncation (separate explicit protocol, not this mission)
    - Store B engine swap to Postgres (gated on post-split query audit)
    - guest embedded-Dolt GC (already wired; different store)
  protected_surfaces:
    - canonical event log + computer_replay_watermarks + computer_file_roots
    - platform-artifacts CAS (event payloads, checkpoint artifacts, file roots)
    - recovery watermark/base_ref replay path
    - Texture canonical writes (platform_texture_revisions)

now:
  status: working
  slice: >-
    Move 1 — commit batching: coalesce ~52 per-mutation DOLT_COMMIT sites
    behind one debounced snapshot committer. Move 0 gates passed 2026-09-19.
  source_ref: main@de9ed92d
  deploy_identity: c592331b1f9318de013a54faaa162b6735e40342
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: c-durability-gate
    claim: >-
      SQL COMMIT without DOLT_COMMIT persists the Dolt 2.1.9 working set across
      kill-9/restart, so batched commits lose no acknowledged writes.
    test: staging kill-9 drill — write + COMMIT (no DOLT_COMMIT), kill -9, restart, verify rows
    edge: missing_oracle
    delta_o: run the drill on the deployed 2.1.9 binary, not docs
    scope_if_supported: batched commits are safe; the unaddressable window <= replay window
    status: supported
    evidence_refs:
      - >-
        Node B scratch dolt sql-server 2.1.9 drill 2026-09-19 — COMMIT'd and
        autocommitted rows survived kill -9 + restart; in-flight uncommitted
        row was lost (expected); dolt_log stayed at 1 (init commit only).
  decision:
    what: three-move sequence — Move 1 commit batching, Move 2 authority split, Move 3 CAS payloads
    kind: architecture
    status: settled
    evidence_ref: docs/designs/platform-dolt-storage-normalization-2026-09-18.md (consensus-adjudicated)
    owner_ratification_ref: owner direction 2026-09-18
  belief:
    believed_state: >-
      Root cause is per-mutation DOLT_COMMIT (52 sites) accumulating reachable
      history; dolt_log 66,793 on 2026-09-19 (~27K/day growth, up from 39,540
      baseline). Move 0 inventory: platform og_objects has ZERO computer-scoped
      rows (computer_id='' on all 6.07M rows; all kinds publication-domain).
      Design doc's "computer-scoped kinds → Store A" concern is moot for the
      platform store — guest og_* lives in the guest embedded Dolt, rebuilt
      from the event log on restore. Dump splitter still filters by
      computer_id/kind as a safety net since the HTTPStore path can carry
      computer_id. Correction to design doc: verifier_attestations,
      consent_records, rollback_refs are publication-domain (publication_version
      / public_route targets only) → Store B, not Store A.
    main_uncertainty: >-
      None blocking Move 1. Store B engine gate (Dolt vs Postgres) deferred to
      post-split 2-week query audit per design.
    next_observation: >-
      dolt_log growth rate after Move 1 deploy — must go flat under load.
  blocker_or_risk: >-
    Move 0 gates passed. Move 1 risk: a mutation path that relied on
    DOLT_COMMIT for cross-connection visibility (unlikely — SQL COMMIT
    suffices) or for AS OF reads (none exist on platform tables).
  next_action: >-
    Map all 52 DOLT_COMMIT/commitDolt call sites; implement the debounced
    snapshot committer (checkpoint/watermark event-driven + 30-60s fallback);
    migrate call sites.

receipts:
  - id: move0-og-inventory
    at: 2026-09-19
    kind: deployed_observation
    what: >-
      og_objects object_kind census on Node B platform store: 6,067,518 rows,
      100% computer_id=''. Kinds: choir.source_entity 3,032,123;
      choir.web_capture 3,032,117; ~209 each of publication/provenance/
      consent/attestation/route/artifact kinds; choir.provenance_agent 1;
      choir.publication_policy 1; choir.publication_transclusion 1;
      choir.subject 1. og_edges: all kinds publication-domain
      (captured_from 3,032,112 dominates). No choir.run/choir.agent/choir.event
      rows — those live in guest embedded Dolt.
    proves: dump-split filter can be table-level for og_* with a
      computer_id/kind safety net; no computer-scoped rows exist to protect.
  - id: move0-durability-drill
    at: 2026-09-19
    kind: staging_proof
    what: >-
      Scratch dolt sql-server 2.1.9 on Node B (/tmp/dolt-durability-drill):
      INSERT+COMMIT and autocommit INSERT without DOLT_COMMIT, kill -9,
      restart — both rows present; in-flight uncommitted INSERT lost;
      dolt_log=1 (init commit only, no working-set commits created).
    proves: Dolt 2.1.9 working-set durability — batched commits lose no
      acknowledged writes; unaddressable window is the uncommitted
      transaction, <= replay window.
  - id: move0-table-classification
    at: 2026-09-19
    kind: deployed_observation
    what: >-
      Store A (canonical event/control): computer_event_append_receipts,
      computer_event_heads, computer_lifecycle_operations,
      computer_lifecycle_receipts, computer_checkpoints,
      computer_replay_watermarks, computer_file_roots, computer_key_escrows,
      computer_key_escrow_transparency, computer_key_unwrap_approvals,
      computer_key_unwrap_requests, computer_route_projection_certificates,
      computer_self_development_modes, computer_version_artifact_programs,
      computer_version_code_closures, computer_version_route_authority_config,
      computer_version_route_authority_modes,
      computer_version_route_authorization_evidence,
      computer_version_route_slots, computer_version_route_transition_receipts,
      control_key_history. Store B (world-wire/corpus): everything else
      including og_objects, og_edges, items, fetches, ingestion_events,
      cycle_events, cycles, sources, issues, processor_requests,
      reconciler_requests, proposal_delivery_records, artifact_blobs,
      artifact_manifests, citation_edges, consent_records, public_routes,
      publication_*, platform_subjects, platform_texture_*,
      platform_vtext_*, provenance_*, retrieval_*, review_records,
      rollback_refs, verifier_attestations.
    proves: dump-split table filter enumerated; corrects design doc's
      Store-A placement of verifier_attestations/consent_records/rollback_refs
      (all publication-domain by live target_kind census).
  - id: move1-batching-deployed
    at: 2026-09-19
    kind: staging_proof
    what: >-
      doltbatch.Committer (45s debounce, CommitNow on checkpoint/watermark/
      file-root boundaries) deployed at 0be14e7f; corpusd + sourcecycled
      migrated. Store A dolt_log: 25,244 commits 00:00-04:00 (pre-deploy
      per-mutation tail) -> 1 commit 04:00-04:20 post-flip. Store B: 29
      commits over import + backfill load.
    proves: per-mutation DOLT_COMMIT eliminated; dolt_log flat under load.
  - id: move2-authority-split
    at: 2026-09-19
    kind: staging_proof
    what: >-
      Two sql-servers live: platform (13306, Store A) + corpus (13307,
      Store B). corpusd carries CORPUSD_CORPUS_DOLT_DSN=13307/corpus;
      sourcecycled SOURCECYCLED_DOLT_DSN=13307/corpus. Event/checkpoint/
      replay paths verified on s.db (Store A); og_*/items on corpus().
      Cutover: fenced dump 20.8G -> split (21 A tables / 39 B tables) ->
      offline import -> DSN flip via /var/lib/go-choir/corpus-dsn.env.
      Deploy mid-cutover (1cf4080b, 03:01) restarted writers before the
      flip, landing ~53min of world-wire writes in Store A; delta-synced
      51,835 rows A->B via scripts/og-delta-sync; B now ahead of A
      (converged). corpus-dolt WorkingDirectory fix c40f2355 (chdir
      precedes ExecStartPre).
    proves: authority split live; blast-radius isolation achieved; the
      deploy-during-cutover hazard is documented + repaired.
  - id: move3-cas-externalization
    at: 2026-09-19
    kind: staging_proof
    what: >-
      og_objects.body externalized to platform-artifacts CAS at >=1KiB:
      body_ref/body_size columns, PutObject/PutBatch write sha256/og/
      <hash>.bin, reads hydrate. Live: 668 body_ref rows, CAS file hash
      == ref name == sha256(body), size matches body_size. Backfill
      (scripts/og-body-backfill) externalizing existing rows, guarded on
      body_ref='' AND content_hash so concurrent writers win.
      DEVIATION from design: items.body/raw_json/reader_snapshot stay
      inline — raw_json is empty and reader_snapshot is a TINYINT flag on
      the live store (design premise stale), and items.body feeds the live
      lower(i.body) LIKE search in cycle.Storage.SearchItems; externalizing
      breaks search recall without a search-path redesign. og_objects.body
      (6.0G, the dominant fat payload) is externalized; items.body (1.34G)
      deferred to a search-path redesign.
    proves: dominant fat payload off the chunk store; CAS refs resolve;
      deviation documented with live evidence.
  - id: restore-drill-post-split
    at: 2026-09-19
    kind: staging_proof
    what: >-
      Store A event chain for computer-03335285269bdba4f94377e56879f9e6:
      161,464 contiguous events (seq 1..161464, no gaps), digests valid,
      watermark at 148431 -> replay range (148431,161464] intact on Store A.
      Event artifact ref resolves through CAS
      (sha256/computer-event/<digest>). computer_checkpoints +
      computer_replay_watermarks on Store A.
    proves: replay-from-checkpoint recovery invariant preserved post-split;
      autoputer rebuild path intact.


weak_measures:
  - name: dolt_log_commits
    kind: weak_signal
    baseline: 39540 (2026-09-18, ~1 day post-squash)
    desired: flat under load after Move 1
    decision_use: confirms commit batching stopped the leak
    cannot_prove: that the event log is intact or recovery works (needs the restore drill)
  - name: noms_oldgen_bytes
    kind: telemetry
    baseline: 9.1G (post-squash)
    desired: bounded, not re-growing toward 200G
    decision_use: early leak-recurrence detector
    cannot_prove: correctness — only size
---

# Platform-Dolt Storage Normalization

Mission doc for the durable storage-leak fix. Design + consensus:
`docs/designs/platform-dolt-storage-normalization-2026-09-18.md`. Root-cause
receipt: `docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md`.

## The three moves (sequenced, each gated)

- **Move 0 (blocking):** og_* per-kind consumer inventory + Dolt 2.1.9
  working-set durability drill. Produces the dump-split filter and the Move 1
  safety proof.
- **Move 1:** coalesce ~52 per-mutation `DOLT_COMMIT`/`commitDolt` sites behind
  one debounced snapshot committer (event-driven on checkpoint/watermark +
  30–60s fallback). Gate: durability drill + flat `dolt_log` under load.
- **Move 2:** split by authority — Store A (canonical event/control, own
  sql-server) vs Store B (world-wire/corpus, own sql-server). `og_*` split by
  `object_kind` (computer-scoped → A, corpus → B). Offline dump-and-split,
  fenced cutover, never dual-write. Gate: restore drill + Store B verified.
- **Move 3:** content-address `items.body`/`og_objects.body` into
  `platform-artifacts` CAS; keep `content_hash` + size in SQL. Gate: refs
  resolve + restore drill still passes.

## Suggested goal string

```text
/goal docs/goal-platform-dolt-normalization-2026-09-18.md — execute the
platform-dolt storage normalization: Move 0 (og_* per-kind inventory + Dolt
2.1.9 durability drill), Move 1 (commit batching), Move 2 (authority split,
separate sql-servers, og_* by kind), Move 3 (CAS payloads). Preserve the
immutable event log + watermark recovery invariant throughout. Red class;
landing required.
```
