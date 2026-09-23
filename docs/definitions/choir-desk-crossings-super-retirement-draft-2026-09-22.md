---
definition_version: 3
definition_id: choir-desk-crossings-super-retirement-draft-2026-09-22
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
    disposition: required — the desks cross onto the kernel M3 lands; the
      dispatcher must exist before a desk can run on it.
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md (M4)
  observed_artifact:
    - claim: 'Three desks remain off the carrier: Texture (partially — owner
        input is fixed by M1 but the desk still runs on the old substrate),
        research, and management. The Super substrate (super_controller.go) is
        a second supervision path; actuator=tools is the old JSON capsule
        surface.'
      claim_scope: current
      evidence_ref: internal/agentcore/super_controller.go;
        cmd/capsule-broker/main.go:53-79;
        docs/problems/root-cause-wrong-path-cluster-2026-09-22.md

finish:
  deliver: >-
    Texture, research, and management desks run on the common event-driven
    carrier. `actuator=tools` and the Super substrate are deleted — there is
    one desk runtime, one continuation authority, one input model.
  artifact: >-
    Deployed staging state where all three desks execute on the in-cell
    carrier with product-path evidence; `actuator=tools` route and
    `super_controller.go` substrate removed; R8 and R10 closed.
  acceptance:
    - action: >-
        On staging, drive one task on each of Texture, research, and
        management desks; observe each completes on the in-cell carrier with
        canonical evidence.
      proves: all desks crossed
      evidence_class: deployed proof
    - action: >-
        `grep -rn "actuator.*tools\|tools_capsule\|super_controller"
        internal/ cmd/` returns no live route; the tools actuator and Super
        substrate are absent.
      proves: R8 and R10 deleted
      evidence_class: code inspection + deployed proof
    - action: >-
        No desk falls back to the old capsule/tools path; the broker serves
        only the RLM/session route.
      proves: one runtime, no dual path
      evidence_class: deployed proof
    - action: >-
        Agentic consensus panel reviews the landed candidate (frozen diff
        identity + deployed evidence) before `goal.complete`; a SEND BACK
        verdict blocks completion until the named gap is closed.
      proves: independent review gates acceptance, not just self-report
      evidence_class: consensus review
  rollback: >-
    git revert + redeploy. Desk crossings are per-desk; a failed crossing
    reverts that desk, not the kernel.
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
    Minimize the number of desk runtimes and supervision substrates to one
    each while preserving each desk's product behavior.
  goodharting_would_be: >-
    Crossing a desk's prompt/registration while leaving its work driven by the
    old substrate — the desk "runs on the carrier" in name but its
    continuations still come from Super or the tools actuator.

homotopy:
  realism_axis: >-
    Desk substrate: from "each desk on its own runtime + a separate Super
    controller" (current) to "all desks are actors on the common kernel"
    (target). Per-desk crossings are valid rungs only if each crossed desk
    fully leaves the old substrate — a desk half-on both is a dual path, not
    progress.

boundaries:
  mutation_class: red
  authority_sources:
    - ratified ontology cutover (docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md)
    - ordered mission list (docs/world-wire-mission-stack-2026-09-22.md, M4)
    - owner correction 2026-09-22 (actuator=tools/Super blocked on crossings)
  must_preserve:
    - Each desk's product behavior through the crossing.
    - actuator=tools and Super are deleted only after every desk that used
      them has crossed — not before.
    - Self-dev admission becomes a tape event (the Super substrate was the
      admission authority).
  excluded:
    - The kernel itself (M3).
    - Owner-input path (M1).
    - Records mechanism (M5+).
    - Updating system (M9).
  protected_surfaces:
    - Desk runtimes and prompt/tool surfaces.
    - Self-dev operation admission (was Super's authority).
    - Run acceptance.

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
    id: m4-desks-cross-cleanly
    claim: >-
      Texture, research, and management can each cross to the common carrier
      without behavior loss, and once all three are across, actuator=tools and
      Super have no remaining consumers.
    test: >-
      Per-desk deployed proof on the carrier; then a caller census on
      actuator=tools and super_controller before deletion.
    edge: missing_oracle — a desk may depend on a Super-only capability
      (persistent supervision, fate watchdogs) that has no carrier equivalent
      until M3's derivable continuations exist.
    delta_o: >-
      Enumerate each desk's Super/tools dependencies before crossing; any
      dependency with no kernel equivalent is a named gap, not a silent
      carry-over.
    scope_if_supported: >-
      One desk runtime; R8/R10 closed; self-dev admission is a tape event.
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      M4 is the desk crossings plus R8/R10 retirement — the migration targets
      of the ontology cutover, run after the kernel lands.
    kind: architecture
    status: settled
    evidence_ref: docs/world-wire-mission-stack-2026-09-22.md
    owner_ratification_ref: ratified ontology cutover 2026-09-22
  belief:
    believed_state: >-
      Three desks to cross; Super and actuator=tools are the deletions that
      become safe only after the crossings.
    main_uncertainty: >-
      Whether any desk has a Super-only dependency with no kernel equivalent —
      the persistent-Super supervision path is the named risk.
    next_observation: >-
      M3's kernel acceptance: do derivable continuations cover the supervision
      cases Super currently owns?
  blocker_or_risk: >-
    Blocked on M3. Deleting Super before self-dev admission is a tape event
    strands the self-dev substrate.
  next_action: >-
    Wait for M3 terminal receipt; then reconcile start state and charter.

receipts: []
---

## What this mission is

M4 of the ordered mission list — the remaining desk crossings plus the R8/R10
retirements. Texture, research, and management desks cross to the common
carrier; `actuator=tools` and the Super substrate are deleted once nothing
consumes them.

## Draft status

Draft successor — blocked on M3, not executable. Becomes the working
entrypoint only on promotion.
