---
definition_version: 3
definition_id: choir-sub-rlm-document-channel-draft-2026-09-22
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
    mission: choir-texture-owner-input-cutover-2026-09-22
    disposition: required — M1's document channel must exist before this
      mission's cast can run on it. Do not start until M1 lands.
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (M2)
  observed_artifact:
    - claim: 'The engineering-desk carrier proved its mechanism (A9 first
        overlay-served arm; A14 executed task cells and staged choir.Complete)
        but the roster receipt is quarantined: it was driven by texture tell,
        not the document channel.'
      claim_scope: current
      evidence_ref: docs/current-situation-2026-09-22.md (carrier decision)
    - claim: 'The carrier substrate exists: capsule_go_eval envelope, staged
        choir.* intents, in-cell session worker. What is missing is the
        document-channel cast path and the R7/R9 deletions.'
      claim_scope: current
      evidence_ref: cmd/capsule-broker/session_worker.go;
        internal/agentcore/rlm_reduce.go

finish:
  deliver: >-
    An external harness (e.g. this OMP session) drives one real Choir
    development task end-to-end through the `choir` CLI: a document edit
    produces a cast-only sub-RLM call on the document channel, the task
    completes, and acceptance reads canonical evidence. The engineering desk
    runs entirely on the in-cell carrier.
  artifact: >-
    A deployed staging proof: one real development task driven by `choir` CLI
    document edit → sub-RLM cast → result on canonical evidence; engineering
    desk on in-cell carrier only; five overlay tools + four legacy capsule ops
    deleted (R7); provider-heresy path deleted (R9); run acceptance on
    canonical evidence, not tool names.
  acceptance:
    - action: >-
        On staging, drive one real engineering task via `choir` CLI document
        edit; observe the sub-RLM cast on the document channel and the task
        result on canonical evidence.
      proves: the carrier lands on the real input channel
      evidence_class: deployed proof
    - action: >-
        `grep -rn "update_coagent\|record_assignment_result\|assign_co_super"
        internal/runtimeprompts/ internal/promptstore/` returns no live
        references; the five overlay tools and four legacy capsule ops are
        absent from the engineering desk's tool surface.
      proves: R7 deletions landed
      evidence_class: code inspection + deployed proof
    - action: >-
        Run acceptance for the driven task reads canonical evidence (event
        receipts, revision refs), not tool-name checkpoints.
      proves: acceptance is off tool names
      evidence_class: deployed proof
    - action: >-
        Agentic consensus panel reviews the landed candidate (frozen diff
        identity + deployed evidence) before `goal.complete`; a SEND BACK
        verdict blocks completion until the named gap is closed.
      proves: independent review gates acceptance, not just self-report
      evidence_class: consensus review
  rollback: >-
    git revert + redeploy. The carrier path is additive to the desk; revert
    restores the prior tool surface.
  landing:
    required: true
    environment: staging
    required_receipts:
      - pushed_commit
      - ci
      - deploy
      - environment_identity
      - deployed_acceptance
      - consensus_review

value:
  better_means: >-
    Minimize the gap between "a harness can drive Choir" and "Choir drives
    itself" — the first self-development capability — while preserving the
    invariant that the cast is on the canonical document channel.
  goodharting_would_be: >-
    A harness that drives Choir through a reintroduced side channel (a new
    tell-shaped CLI verb, a direct HTTP post to an internal endpoint) and
    calls it "the carrier." Or a proof that passes on a live process but
    strands on restart — restart durability is M3's claim, not this one's.

homotopy:
  realism_axis: >-
    Carrier realism: from "sub-RLM cast on the document channel, live-process
    only" (this mission) to "the same cast with derivable restart-safe
    continuation" (M3). The live-process proof is a valid projection — same
    interface, same event semantics — as long as it does not claim restart
    durability it cannot deliver.

boundaries:
  mutation_class: red
  authority_sources:
    - owner direction 2026-09-22 (self-dev phase 1 = choir CLI from a harness;
      roster is a sub-RLM call)
    - ordered mission list (docs/world-wire-mission-stack-2026-09-22.md, M2)
    - superseded carrier definition (choir-rlm-engineering-carrier-2026-09-11,
      retained scope)
  must_preserve:
    - The cast is on the document channel M1 lands — no new side channel.
    - Acceptance does not claim restart durability (M3 owns that).
    - The five overlay tools and four legacy capsule ops are deleted only
      after the in-cell replacements are proven.
  excluded:
    - "Ontology cutover internals (M3): fenced atomic commit, dispatcher,
      due-index, sweeps deletion."
    - Desk crossings beyond engineering (M4).
    - Records mechanism (M5+).
    - Updating system (M9).
  protected_surfaces:
    - Run acceptance semantics.
    - Capsule/RLM tool surface (R7 deletions behind replay proofs).
    - Provider routing (R9 deletion).

now:
  status: blocked_incomplete
  slice: draft — awaiting M1 completion
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
    id: m2-cast-on-document-channel
    claim: >-
      A document edit on the canonical path can carry a sub-RLM cast that the
      engineering desk consumes — the carrier works on the real input channel
      without any tell-shaped side path.
    test: >-
      Drive one real task via `choir` CLI document edit on staging; observe
      the cast and the result on canonical evidence.
    edge: missing_oracle — the cast may need a continuation the pre-cutover
      runtime cannot derive; if so, M2's proof is live-process only and M3
      becomes the hard prerequisite.
    delta_o: >-
      Kill the process mid-task: does the continuation survive? If not, the
      claim is live-process-scoped and M3 owns durability.
    scope_if_supported: >-
      Self-dev capability phase 1 is real: a harness drives Choir development
      through the product path.
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      M2 is the carrier landing and self-dev capability phase 1 as one
      mission (1c ≡ 3a). Not two missions.
    kind: architecture
    status: settled
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md
    owner_ratification_ref: owner direction 2026-09-22
  belief:
    believed_state: >-
      The carrier mechanism is proven; the input channel is the blocker. Once
      M1 lands the document channel, the cast path is the remaining work.
    main_uncertainty: >-
      Whether the pre-cutover runtime can carry the cast's continuation far
      enough for a live-process proof, or whether M2 collapses into M3.
    next_observation: >-
      M1's deployed acceptance: does the document revision event reach the
      desk cleanly enough to build a cast on?
  blocker_or_risk: >-
    Blocked on M1. Fake-island risk: a live-process proof that dies on
    restart — acceptance must not claim durability M3 hasn't delivered.
  next_action: >-
    Wait for M1 terminal receipt; then reconcile start state and charter.

receipts: []
---

## What this mission is

M2 of the ordered mission list — the carrier landing and self-development
capability phase 1 as one mission (1c ≡ 3a). An external harness drives a real
Choir development task through the `choir` CLI as a sub-RLM cast on the
document channel. This is the first moment self-development is real.

## Inherits from the superseded carrier definition

`choir-rlm-engineering-carrier-2026-09-11` (superseded by M1) carried: the
engineering desk fully on the in-cell carrier, five overlay tools + four
legacy capsule ops deleted (R7), run acceptance on canonical evidence, R9
provider-heresy deletion. That scope lands here, on the document channel M1
builds.

## Draft status

This is a draft successor — blocked on M1, not executable. It becomes the
working entrypoint only on promotion after M1's terminal receipt.
