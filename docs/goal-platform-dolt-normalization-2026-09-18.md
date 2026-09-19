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
    Move 0 — blocking pre-migration: og_* per-kind consumer inventory +
    Dolt 2.1.9 working-set durability drill. Both gate Move 1/2.
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
    status: proposed
    evidence_refs: []
  decision:
    what: three-move sequence — Move 1 commit batching, Move 2 authority split, Move 3 CAS payloads
    kind: architecture
    status: settled
    evidence_ref: docs/designs/platform-dolt-storage-normalization-2026-09-18.md (consensus-adjudicated)
    owner_ratification_ref: owner direction 2026-09-18
  belief:
    believed_state: >-
      Root cause is per-mutation DOLT_COMMIT (52 sites) accumulating reachable
      history; dolt_log already 39.5K ~1 day post-squash and climbing.
    main_uncertainty: >-
      Which og_objects/og_edges object_kinds are computer-scoped (Store A) vs
      corpus (Store B) — decides the dump-split filter.
    next_observation: >-
      The og_* per-kind consumer inventory result + the kill-9 durability drill
      result.
  blocker_or_risk: >-
    Move 1 safety unproven until the durability drill; Move 2 correctness
    unproven until the og_* classification. Both are Move 0 gates.
  next_action: >-
    Enumerate og_objects/og_edges object_kind values and classify each
    computer-scoped vs corpus; run the staging kill-9 working-set durability
    drill.

receipts: []

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
