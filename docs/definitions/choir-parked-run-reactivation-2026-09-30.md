---
definition_version: 4
definition_id: choir-parked-run-reactivation-2026-09-30
readiness: reviewed
execution_mode: mission_orchestrator

review:
  reviewer: 'agentic-consensus panel — claude/opus, codex, cursor, devin,
    omp-gpt6-sol, omp-gpt6-luna, omp-gemini38, omp-glm53-flash (8 ok; opencode
    + 3 free-tier OMP models failed on a provider 403, environmental)'
  frozen_ref: 'git-blob 578ea765 (final file; verdict binds to the decision
    content the panel reviewed — the DeliveredToRunID recovery seam)'
  evidence_ref: .agentic-consensus/agentic-consensus-20260930-213725/manifest.tsv

start:
  captured_at: '2026-09-30T23:55:00Z'
  source:
    canonical_ref: main@ad362c22
    deploy_identity: 'staging https://choir.news build.commit=37882e1d (guest epoch 979)'
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: choir-jev-supervision-metamission-2026-09-29
    disposition: 'in flight — M0a + M-SUB + M1-live proofs blocked on this repair'
    evidence_ref: docs/problems/coagent-result-parked-run-not-reactivated-2026-09-30.md

finish:
  deliver: 'A parked lifecycle Research run whose coagent_result obligation is
    re-driven reactivates and consumes the packet — the deliverable reaches the
    run it was bound to even when the actor resume snapshot lost its pointer.'
  artifact: 'handleCoagentResult resolves the exact canonical packet by its
    occurrence content digest on the rs.RunID=="" arm and, when the packet''s
    DeliveredToRunID names a passivated/blocked lifecycle research run, feeds
    that run id into the existing parked-run branch (ReconcileParkedLifecycle
    CoagentWake -> ExecuteActivationSyncChecked -> memoryFromRunState, healing
    the snapshot). Run 362febb2 reactivates on staging.'
  acceptance:
    - action: 'on staging, refresh/trigger the parked-run wake path for a
        research run with an open delivered coagent_result; observe the run
        transition passivated -> pending -> running and consume the packet'
      proves: 'the parked-run reactivation defect is closed; a deliverable is
        not lost when the resume snapshot is missing'
      evidence_class: deployed proof
    - action: 'run status on a previously stranded run shows non-passivated
        state with an advanced updated_at after the wake drains'
      proves: 'the obligation discharged into the run it was bound to'
      evidence_class: deployed proof
    - action: 'go test ./internal/actorruntime ./internal/agentcore ./internal/store
        passes including a regression test covering DeliveredToRunID-driven
        reactivation on the rs.RunID=="" arm'
      proves: 'the repair is pinned against the no-resume-snapshot case'
      evidence_class: local test
  rollback: 'git revert of the handler change; the prior arm falls back to
    reconcileCoagentWake. The defect is latent (no new write path), so revert
    restores prior behavior with no state migration.'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: 'minimize the number of durable obligations that discharge
    without reaching their bound run — a coagent_result must reach the run it
    named, or fail loudly; never strand silently on a missing resume pointer.'
  goodharting_would_be: 'the wake dispatches and the occurrence incorporates
    (no deferral, no poison), but the parked run never reactivates — a
    green-looking delivery that loses the deliverable.'

homotopy:
  realism_axis: 'resume authority — from actor-memory-snapshot-only (fragile)
    to obligation-authoritative DeliveredToRunID fallback (durable). Same
    reactivation transaction (ReconcileParkedLifecycleCoagentWake), same
    guards; only the run-id source widens.'

boundaries:
  mutation_class: red
  authority_sources:
    - AGENTS.md red-class ceremony
    - docs/problems/coagent-result-parked-run-not-reactivated-2026-09-30.md
    - panel verdict on this file (authoring gate)
  must_preserve:
    - 'actor memory remains the primary resume authority (snapshot first)'
    - 'ReconcileParkedLifecycleCoagentWake''s guards unchanged — no minting a
      competing run for a parked actor-memory run'
    - 'generic reconcile never guesses a run id — only the packet-bound
      DeliveredToRunID may supply it'
    - 'dedup/idempotency: redrive salt semantics unchanged'
  excluded:
    - 'platform-dolt OOM / ballooning (separate substrate mission)'
    - 'the 1778-wake serial-drain throughput (separate substrate mission)'
    - 'vmctl restart/rebind behavior'
    - 'the M-SUB emit proof cells themselves (this mission unblocks them;
      the proofs are the station''s own acceptance)'
  protected_surfaces:
    - 'actor wake/handler seam (handleCoagentResult, resume-state decode)'
    - 'lifecycle reactivation transaction (ReconcileParkedLifecycleCoagentWake)'
    - 'obligation/wake mint+re-arm (MigrateActorWakeOutbox)'
    - 'dispatcher defer/poison contract'

conjecture:
  id: parked-run-recovers-from-delivered-to-run-id
  claim: 'If handleCoagentResult resolves the bound run from the obligation''s
    DeliveredToRunID when the actor snapshot''s resume.RunID is empty, then a
    parked lifecycle run reactivates on its own deliverable without relying on
    a snapshot pointer that can be lost — closing the deliverable-strand class.'
  test: 'unit: seed a passivated research run + an open UpdatePending
    coagent_result with DeliveredToRunID set, invoke the handler with empty
    resume memory, assert ReconcileParkedLifecycleCoagentWake path fires and
    the run reactivates. deployed: observe 362febb2-class transition on
    staging after the wake drains.'
  edge: missing_oracle — guest-internal actor-mailbox/resume-snapshot reads are
    not host-reachable, so the failure was diagnosed from absence-of-log
    evidence; the fix''s correctness is confirmed by the unit + deployed
    transition, not by reading the lost snapshot.
  delta_o: 'guest-internal actor-log read surface (would confirm the exact
    resume-state loss rather than infer it).'
  scope_if_supported: 'all coagent_result deliveries to parked lifecycle
    research runs — the strand class generalizes to any snapshot loss.'
  status: active
  evidence_refs:
    - docs/problems/coagent-result-parked-run-not-reactivated-2026-09-30.md

now:
  status: working
  slice: 'drafted — awaiting consensus verdict on the repair seam
    (DeliveredToRunID-driven reactivation) before promotion to executable.'
  source_ref: main@ad362c22
  deploy_identity: 'staging build.commit=37882e1d (guest epoch 979)'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: parked-run-recovers-from-delivered-to-run-id
    claim: 'DeliveredToRunID fallback reactivates the parked run the obligation
      named, closing the deliverable-strand class.'
    test: 'unit regression + deployed passivated->running transition.'
    edge: missing_oracle
    delta_o: 'guest-internal actor-log read surface.'
    scope_if_supported: 'parked lifecycle research runs with open delivered
      coagent_result obligations.'
    status: active
    evidence_refs:
      - docs/problems/coagent-result-parked-run-not-reactivated-2026-09-30.md
  decision:
    what: 'resolve the canonical packet by occurrence-content digest on the
      rs.RunID=="" arm; feed packet.DeliveredToRunID into the existing
      parked-run branch (handler.go:620+) so it reconciles via
      ReconcileParkedLifecycleCoagentWake, executes via
      ExecuteActivationSyncChecked, and returns memoryFromRunState — re-healing
      the snapshot. Keep management_controller.go:1673 hard-error unchanged
      (generic reconcile never selects a run). Resolve via
      ListActionablePendingLifecycleUpdates (includes bound rows);
      ListAllPendingLifecycleUpdates misses bound-delivered packets.'
    kind: architecture
    status: settled
    evidence_ref: .agentic-consensus/agentic-consensus-20260930-213725/manifest.tsv
    owner_ratification_ref: pending
  belief:
    believed_state: 'the defect is a resume-authority gap: the obligation is
      bound (DeliveredToRunID set) but the run-id source is snapshot-only.
      Panel confirmed DeliveredToRunID is same-authority exact binding — the
      fix is obligation-authoritative recovery into the parked-run branch.'
    main_uncertainty: 'whether 362febb2''s resume.RunID is actually empty —
      guest-internal read unavailable; the fix is correct for both the
      silent-incorporate and 1673 sub-paths, but acceptance must confirm the
      passivated->running transition, not just the code path.'
    next_observation: 'the first staged-repro or guest boot where a bound
      coagent_result reaches a passivated run with empty resume state.'
  blocker_or_risk: 'guest OOM-cycles every ~20-40min; the deployed acceptance
    window is per-uptime-window. Sibling residual: an UNBOUND pending control
    (DeliveredToRunID=="") on the same arm still strands — named, out of
    scope (no bound run to recover; fail-closed is correct there).'
  next_action: 'author the handler-arm fix: add a resolveDeliveredPacket helper
    (content-digest match over actionable pending controls) feeding
    rs.RunID=DeliveredToRunID, then land red-class and prove on staging.'

receipts: []
---

## Context

This is a surgical red-class repair mission, scoped to one defect documented in
`docs/problems/coagent-result-parked-run-not-reactivated-2026-09-30.md`. It
unblocks the Jev-supervision metamission's remaining stations (M0a verify,
M-SUB emit proofs, end-to-end QA repro) which all gate on a parked research
run reactivating on its own deliverable.

The full defect receipt — ruled-out theories, the leading candidate
(`handleCoagentResult` `rs.RunID==""` arm → `reconcileCoagentWake` →
`parkedLifecycleControlCandidate` hard-error), and the needed-confirmation
notes — lives in the problem doc; this file carries only the goal.

## Scope discipline

Two adjacent substrate issues are explicitly OUT of scope and named as
`excluded`: the platform-dolt OOM reboot cycle and the serial-wake drain
throughput. They are real, but they are environment-capacity problems, not the
deliverable-loss defect. Folding them in would turn a one-seam repair into a
substrate overhaul and delay the unblock the metamission needs.
