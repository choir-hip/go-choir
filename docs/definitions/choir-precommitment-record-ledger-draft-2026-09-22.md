---
definition_version: 3
definition_id: choir-precommitment-record-ledger-draft-2026-09-22
execution_mode: mission_orchestrator
draft: true

start:
  captured_at: '2026-09-22T23:45:00Z'
  source:
    canonical_ref: main@a4cef7cf
    deploy_identity: unknown — capture at execution time
  worktrees:
    - path: /Users/wiz/go-choir
      status: unknown
      class: unknown
      owner: unknown
      touch: read_only
      recovery: reconcile at charter
  predecessor:
    mission: choir-ontology-kernel-draft-2026-09-22
    disposition: required — an unresolved precommitment is a durable wake
      obligation; the record's resolution continuation needs M3's derivable
      continuations. Schema drafting may overlap M3; the runtime cannot land
      before it.
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (M5)
  observed_artifact:
    - claim: 'No precommitment record type exists. The Markdown conjecture
        ledger and doccheck regex are the current (to-be-retired) mechanism.'
      claim_scope: current
      evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (2a)

finish:
  deliver: >-
    A precommitment record is an object-graph object with provenance edges:
    an agent commits a typed prediction, the system pairs it with the
    observed result, and the record survives restart. The Markdown conjecture
    ledger and doccheck regex retire.
  artifact: >-
    The record type + ledger live on the OG: `expected` linkage +
    `resolves`/`prediction_ref`, epistemic-boundary invariant, deterministic
    pairing projection, async Curator desk minting `learning_record_minted`,
    result-wake policy, REPL-queryable records with auto-inject OFF,
    actor/model scoping. Never a third store.
  acceptance:
    - action: >-
        On staging, an agent commits a record; the system observes the result
        and pairs them deterministically; the record is queryable via REPL and
        survives a restart.
      proves: the record type and ledger work on the real tape
      evidence_class: deployed proof
    - action: >-
        The Markdown conjecture ledger and doccheck regex are absent; records
        are OG objects with provenance edges.
      proves: the old mechanism retired, the record is OG-native
      evidence_class: code inspection
  rollback: >-
    git revert + redeploy. Records are additive OG objects; revert removes
    the type without disturbing existing state.
  landing:
    required: true
    environment: staging
    required_receipts:
      - pushed_commit
      - ci
      - deploy
      - environment_identity
      - deployed_acceptance

value:
  better_means: >-
    Minimize the gap between "an agent made a claim" and "the claim is a
    durable, scored, provenance-linked record" while preserving the
    epistemic-boundary invariant (records never feed back as authority).
  goodharting_would_be: >-
    A record type that exists but is never written by a real run, or a ledger
    that is a parallel store rather than an OG projection — the schema lands
    but no commitment is ever actually recorded.

homotopy:
  realism_axis: >-
    Record realism: from "schema exists, no live writes" to "every material
    action on a real run carries a committed record." The schema-only rung is
    valid only if the write path is the same one production runs will use.

boundaries:
  mutation_class: orange
  authority_sources:
    - ordered mission list (docs/world-wire-mission-stack-2026-09-22.md, M5)
    - precommitment records engineering memo (docs/Precommitment Records —
      Engineering Memo.md)
    - precommitment records theory (docs/Precommitment Records Theory and
      Implications.md)
  must_preserve:
    - The record is an OG object with provenance edges — never a third store.
    - Epistemic-boundary invariant: records are evidence, not authority.
    - Auto-inject OFF by default; retrieval is M8's gated decision.
    - Result-wake uses M3's derivable continuations, not a new timer.
  excluded:
    - The agent-facing `precommit` Yaegi surface (M6).
    - Context packs and the learning-claims gate (M8).
    - The self-dev proof (M11).
  protected_surfaces:
    - OG write path (the record is an OG object).
    - Event append (learning_record_minted is an event).

now:
  status: blocked_incomplete
  slice: draft — awaiting M3
  source_ref: main@a4cef7cf
  deploy_identity: unknown
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: m5-record-is-og-object
    claim: >-
      A precommitment record can be an OG object with provenance edges —
      paired deterministically to its result — without a third store, and the
      resolution continuation rides M3's derivable wakes.
    test: >-
      Commit a record on staging, observe the deterministic pairing, restart,
      confirm the record and its resolution obligation survive.
    edge: resource — the eight schema decisions are open; if the pairing
      projection or Curator minting is more expensive than budgeted, the
      claim shrinks to a smaller record scope.
    delta_o: >-
      Close the eight schema decisions (engineering memo) before building;
      each undecided default is a place the record can drift from the OG
      model.
    scope_if_supported: >-
      The commitment ledger exists; every later mechanism (scoring, packs,
      supervision) builds on it.
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      M5 is the record type + ledger (2a). The agent-facing surface (2b) is
      M6; context packs (2c) are M8. Schema drafting may overlap M3; runtime
      lands after it.
    kind: architecture
    status: settled
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md
    owner_ratification_ref: not_applicable — within owner-ratified stack
  belief:
    believed_state: >-
      The record is the mechanism the whole product thesis rests on; it needs
      M3's continuations for resolution wakes but its schema can be drafted
      in parallel.
    main_uncertainty: >-
      The eight open schema decisions in the engineering memo — each is a
      default the first implementation must fix.
    next_observation: >-
      M3's kernel acceptance: do derivable continuations cover the
      result-wake obligation a record creates?
  blocker_or_risk: >-
    Blocked on M3 for runtime. The eight schema decisions are the real
    blocker for authoring — close them before the first construct.
  next_action: >-
    Close the eight schema decisions from the engineering memo; wait for M3.

receipts: []
---

## What this mission is

M5 of the ordered mission list — the precommitment record type and OG ledger
(stack 2a). The mechanism the product thesis rests on: an unresolved
precommitment is a durable wake obligation, the same object as the carrier's
stranded-continuation bug.

## Draft status

Draft successor — blocked on M3 for runtime, not executable. Schema decisions
can be drafted in parallel. Becomes the working entrypoint only on promotion.
