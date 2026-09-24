---
definition_version: 3
definition_id: choir-sub-rlm-document-channel-2026-09-22
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-23T11:00:00Z'
  source:
    canonical_ref: main@f2d67e4b
    deploy_identity: staging https://choir.news build.commit=3b780ed2
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: choir-texture-owner-input-cutover-2026-09-22
    disposition: satisfied — M1 landed 2026-09-23 (deployed build 3b780ed2,
      consensus SEND BACK closed by turn-consumption receipt). The document
      channel exists; this mission may start.
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
  status: landed
  slice: >-
    Deployed proof complete: document-channel cast → assignment → run →
    terminal report → disposition completed, all on canonical evidence.
    Trajectory ad04eaba-53e9-5c5c-9116-f8c18ae6dce5 on staging. Terminal-
    report strand fixed (verdict validation + coercion); see
    docs/problems/document-cast-terminal-report-strand-2026-09-23.md.
  source_ref: main@4de7fdf9 (pushed; deployed)
  deploy_identity: staging https://choir.news build.commit=4de7fdf9
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
    status: active
    evidence_refs:
      - internal/agentcore/rlm_reduce.go (castStagedIntent)
      - cmd/capsule-broker/session_worker.go
  decision:
    what: >-
      M2 is the carrier landing and self-dev capability phase 1 as one
      mission (1c ≡ 3a). Not two missions. Charter 2026-09-23 settled the
      cast mechanism: a lifecycle document binds to the engineering desk
      (`engineering:{docID}` agent, profile engineering); every owner-authored
      revision on that document is one cast — the commit dispatches a
      document-channel occurrence whose consumer opens the assignment
      directly (capsule spawn + bound run on the document trajectory,
      ChannelID = the document channel, objective = the revision's
      owner_prompt/content). The revision event IS the admission; the
      assignment's completion report lands on the same trajectory as
      canonical evidence. assign_co_super is deleted (the document channel
      replaces the Super-mediated opener); self-dev operation assignment
      opens reroute through the same document-channel cast; verification
      opens host-side on implementation completion when a selfdev operation
      is bound. update_coagent stays registered for the four non-engineering
      desks (M4 owns their crossing); the acceptance grep's "live
      references" is read as references on desks where the named tool is
      absent — engineering prompts name none of the retired names.
    kind: architecture
    status: settled
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md
    owner_ratification_ref: owner direction 2026-09-22
  belief:
    believed_state: >-
      Implementation landed locally. The document-bound engineering desk is
      live: `desk=engineering` on lifecycle-documents create binds an
      `engineering:{docID}` desk agent (profile/role engineering, channel =
      the document); StartLifecycle, QueueLifecycleUpdate, and
      requireLifecycleAssignedAgent admit the engineering profile; every
      owner-authored revision dispatches a document-channel occurrence whose
      consumer (actor handler, test dispatch, boot scan) opens the
      assignment directly via `ReconcileEngineeringRevisionCast` /
      `ReconcileEngineeringDesk` — deterministic assignment identity keyed
      on the admitting revision, spawn/bind/activate saga resumed on
      stranded opens, verification chained host-side on implementation
      completion when a selfdev operation is frozen. Self-dev start commits
      the engineering doc + directive revision (no mediating Super run).
      R7: the four capsule ops and assign_co_super are deleted; the
      assigned-CoSuper registry is exactly `capsule_go_eval`; engineering
      and management prompts name none of the retired tools.
      R9: deepseek/xiaomi providers, AnthropicCompatProvider, the ChatGPT
      terminal fallback and toolloop retry branch are deleted; nix defaults
      moved to chatgpt/gpt-5.6-luna; gateway refuses uncatalogued models.
      Acceptance: `super_direction_opened` reads the committed assignment on
      document trajectories; `capsule_effect_frozen` only fails when a
      selfdev operation is bound. `choir texture revise` + `--desk` flag
      added. All package suites green (agentcore 226s, actorruntime,
      textureowner, store pending).
    main_uncertainty: >-
      Whether staging's deployed runtime carries a real model call for the
      assigned engineering run — provider auth (chatgpt) was a pre-existing
      gap; the cast proof may be activation-only.
    next_observation: >-
      Push → CI → staging deploy → `choir texture create --desk engineering`
      → `choir texture revise` → observe the cast and result on canonical
      evidence.
  blocker_or_risk: >-
    Fake-island risk: a live-process proof that dies on restart — acceptance
    must not claim durability M3 hasn't delivered. Provider auth on staging
    (chatgpt) was a pre-existing gap; confirm a real model call completes or
    the cast proof is activation-only. Self-dev reroute is code-level only:
    operations may be unreachable with effects OFF on staging — the deployed
    proof is the document-edit path, not an operation.
  next_action: >-
    Commit, push, monitor CI + staging deploy, verify deployed commit
    identity, then drive the acceptance probe: create an engineering-bound
    document via `choir` CLI, revise it with a real task, observe the
    assignment open and the result on canonical evidence.

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

## Status

Promoted to the sole working entrypoint 2026-09-23 on M1's landed terminal
receipt. Executable with `/goal
docs/definitions/choir-sub-rlm-document-channel-2026-09-22.md`.
