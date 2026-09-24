---
definition_version: 3
definition_id: choir-durable-vocabulary-migration-draft-2026-09-23
execution_mode: mission_orchestrator
draft: true

start:
  captured_at: '2026-09-23T21:00:00Z'
  source:
    canonical_ref: main@b0adf6f7
    deploy_identity: staging https://choir.news build.commit=4de7fdf9
  worktrees:
    - path: /Users/wiz/go-choir
      status: unknown
      class: unknown
      owner: unknown
      touch: read_only
      recovery: reconcile at charter
  predecessor:
    mission: choir-desk-vocabulary-docs-cutover-draft-2026-09-23
    disposition: >-
      independent of R2/R3/R4 — durable vocabulary migration may run any
      time after R1, or be deferred. It is red-class: it touches canonical
      event kinds, OG kinds, SQL tables, identity seeds, and replay goldens.
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
  observed_artifact:
    - claim: >-
        Durable vocabulary still carries the old names:
        co_super_assignment_* event kinds (types/lifecycle.go:60-64),
        choir.co_super_assignment OG kinds (store/cosuper_assignments.go:25-39),
        the co_super_slots SQL table (store/store.go:402), the assignment
        identity seed choir:co-super-assignment:v3
        (cosuper_assignment_runtime.go:104-108), CLI evidence schema
        choir.co_super_capsule_evidence/v1 (cmd/choir/main.go:830), the
        lifecycle JSON field co_super_assignments, and replay goldens
        content-addressed over operation identity.
      claim_scope: current
      evidence_ref: internal/types/lifecycle.go
    - claim: >-
        The frozen computerevent decoder covers only profile tokens
        (decode.go:38-50) — nothing decodes historic lifecycle event kinds,
        OG kinds, or command kinds. A V1→V2 profile normalization point is
        also missing: the frozen decoder maps historic tokens to V1 values
        while live constants are V2.
      claim_scope: current
      evidence_ref: internal/computerevent/decode.go

finish:
  deliver: >-
    The durable vocabulary matches the live vocabulary: stored event
    kinds, OG kinds, schema strings, and table names say
    engineering/management/research — or a frozen decoder per family makes
    the old names permanently readable without live use.
  artifact: >-
    Per-family disposition, each either migrated or frozen-decoded:
    (a) lifecycle event kinds (co_super_assignment_* → engineering_*);
    (b) OG kinds (choir.co_super_assignment → choir.engineering_assignment);
    (c) SQL table co_super_slots; (d) the assignment identity seed —
    either a v4 seed with a replay story or an explicit decision to keep
    v3 forever (identity seeds are content; renaming breaks replay);
    (e) CLI/API schema strings; (f) lifecycle JSON field names;
    (g) replay goldens regenerated; (h) the V1→V2 profile normalization
    point for historic ActorProfile comparisons.
  acceptance:
    - action: >-
        Replay a pre-migration tape: every historic co_super_* event
        decodes and folds identically to pre-migration behavior (frozen
        decoders or migrated data — either is acceptable, mixing is not).
      proves: history is preserved
      evidence_class: local test
    - action: >-
        On staging, post-migration: a new assignment writes engineering_*
        event kinds; old co_super_* events in the same store still read
        correctly; replay goldens pass.
      proves: the migration is live-safe
      evidence_class: deployed proof
  rollback: >-
    Frozen decoders are additive and trivially reversible; migrated data
    requires the pre-migration snapshot. The identity seed decision is
    one-way — a renamed seed cannot be un-renamed for already-minted IDs.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the divergence between durable vocabulary and live vocabulary
    while preserving every historic record's readability.
  goodharting_would_be: >-
    Renaming identity seeds or event kinds in place — a clean grep over a
    broken tape. The acceptance is replay-equivalence, not string absence.

homotopy:
  realism_axis: >-
    Durable-name coverage: from mixed old/new (current) through
    decoded-or-migrated per family (this mission) to a fully uniform tape
    (optional end state — frozen decoders may be the permanent answer for
    identity seeds).

boundaries:
  mutation_class: red
  authority_sources:
    - docs/desk-rlm-rectification-plan-2026-09-23.md
    - AGENTS.md
  must_preserve:
    - replay equivalence: every historic event decodes and folds
      identically
    - assignment identity continuity: already-minted IDs stay valid
    - the frozen computerevent decoder's existing behavior
  excluded:
    - live vocabulary (R1 — already done)
    - runtime behavior changes
    - the desk rebuild (R2/R3)
  protected_surfaces:
    - canonical event kinds and the tape
    - the object graph's stored kinds
    - Dolt tables and migration integrity
    - replay goldens

now:
  status: blocked_incomplete
  slice: durable vocabulary migration
  source_ref: main@b0adf6f7
  deploy_identity: staging https://choir.news build.commit=4de7fdf9
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: r5-durable-vocabulary
    claim: >-
      Each durable vocabulary family can be either migrated (with verified
      replay equivalence) or permanently frozen-decoded, and the per-family
      choice is independent — a mixed strategy is correct, not a
      compromise.
    test: >-
      The replay-equivalence acceptance: pre-migration tapes fold
      identically post-migration.
    edge: resource — a full OG/Dolt migration of historic rows may exceed
      a reasonable mission budget; frozen decoders are the fallback per
      family.
    delta_o: >-
      Per-family census first: row counts, reader surfaces, and whether a
      frozen decoder is cheaper than migration for that family.
    scope_if_supported: >-
      Durable vocabulary is uniform or permanently decoded; the naming
      confusion is fully closed.
    status: active
    evidence_refs:
      - internal/types/lifecycle.go
      - internal/computerevent/decode.go
  decision:
    what: >-
      Per-family disposition: migrate where cheap and safe, frozen-decode
      where history is content (identity seeds likely stay v3 forever).
      The V1→V2 profile normalization point is required regardless.
    kind: operational
    status: proposal
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
    owner_ratification_ref: pending — plan under owner review
  belief:
    believed_state: >-
      The durable surface is fully enumerated (event kinds, OG kinds, SQL,
      seeds, schemas, JSON fields, goldens); the per-family strategy is the
      open design work.
    main_uncertainty: >-
      Whether the identity seed can be versioned without breaking
      in-flight assignment replay — likely it cannot, making v3 permanent.
    next_observation: >-
      The per-family census: row counts and reader surfaces per family.
  blocker_or_risk: none beyond its red-class ceremony; deferrable
  next_action: promote when the naming confusion becomes load-bearing
    (e.g., when R2's new event kinds would compound the mix)

receipts: []
---

## Per-family inventory (from the consensus census)

| Family | Location | Likely disposition |
|---|---|---|
| Lifecycle event kinds `co_super_assignment_*` | types/lifecycle.go:60-64 | frozen decoder or migrate |
| OG kinds `choir.co_super_assignment*` | store/cosuper_assignments.go:25-39 | frozen decoder or migrate |
| SQL table `co_super_slots` | store/store.go:402 | migrate (schema change) |
| Identity seed `choir:co-super-assignment:v3` | cosuper_assignment_runtime.go:106 | **keep forever** — renaming breaks replay |
| CLI schema `choir.co_super_capsule_evidence/v1` | cmd/choir/main.go:830 | version bump + decoder |
| Lifecycle JSON `co_super_assignments` | types/lifecycle.go:487 | frozen decoder |
| Replay goldens | rlm_replay_linux_test.go | regenerate post-migration |
| V1→V2 profile normalization | computerevent/decode.go | required — one explicit point |
| Frontend `role: 'cosuper'` fixture | responsive-layout.spec.js | R1 scope (live label) |
| `super-console` app name | frontend/src/lib/apps/registry.ts | product decision — the console may keep its name or rename to management-console |
