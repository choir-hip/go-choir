---
definition_version: 3
definition_id: choir-carrier-substrate-integrity-2026-09-25
execution_mode: mission_orchestrator
draft: false

start:
  captured_at: '2026-09-25T00:00:00Z'
  source:
    canonical_ref: main@173ba26a
    deploy_identity: staging https://choir.news build.commit=537fce04
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  predecessor:
    mission: choir-commitment-ledger-desk-carrier-draft-2026-09-23
    disposition: >-
      R2 closed on substrate 2026-09-25 (landed+deployed 537fce04, resolve
      5404d2c7, now.status=complete). It deliberately deferred two substrate
      repairs it discovered; this mission is that tail, chartered by the
      restructure (desk-rlm-rectification-plan-2026-09-23.md §11).
  observed_artifact:
    - claim: 'commit() is sequential, not atomic: record -> envelope ->
      CommitInboxCursor (rlm_reduce.go ~560-612). A crash between the record
      append and cursor commit leaves a half-staged cast on the durable
      tape — the strand-2 shape R2 was chartered to close.'
      evidence_ref: internal/agentcore/rlm_reduce.go
    - claim: 'The computer-wide engineering-assignment deadline cancel and
      stranded-freeze resume run only inside persistent-management
      selection (management_controller.go:301-302 ->
      enforceEngineeringAssignmentDeadlines /
      resumeStrandedFrozenAssignmentCommits). The kernel derivable wake
      (armAssignedEngineeringFateWatchdog -> assigned_engineering_fate_deadline,
      engineering_assignment_fate.go:720,851,892-908, dispatched via
      actorruntime/handler.go:93) re-drives already-armed per-assignment
      transitions only; it does not scan for un-armed or expired
      assignments. Management parked => fate stops.'
      evidence_ref: internal/agentcore/management_controller.go
    - claim: 'choir.Resolve (5404d2c7) is a post-deploy main commit; staging
      runs 537fce04 and cannot resolve records until that SHA deploys.'
      evidence_ref: internal/agentcore/rlm_reduce.go

finish:
  deliver: >-
    Engineering-assignment fate no longer depends on the management desk
    being selected, and a desk cell's staged tray commits all-or-nothing —
    the two substrate seams R2 left behind are closed so the live-desks
    mission (R3b/R3c) builds on a sound carrier.
  artifact: >-
    (a) act-commit made transactional, or a named partial-commit recovery
    that the reducer exercises; (b) assignment-deadline expiry and
    stranded-freeze resume armed as derivable wakes on the dispatcher
    due-index, with the two selection-path sweeps deleted from
    management_controller.go.
  acceptance:
    - action: >-
        On staging: create an engineering assignment with a short deadline,
        then park the management desk (or never select it) and observe the
        assignment cancel on canonical evidence — no management reconcile
        ran.
      proves: >-
        deadline fate is kernel-scheduled, not selection-gated; the I26
        fail-closed backstop survives a parked management desk.
      evidence_class: deployed proof
    - action: >-
        A reducer test that injects a failure between the commitment-record
        append and CommitInboxCursor observes either a rolled-back commit or
        a deterministic partial-commit recovery; a re-reduced cell does not
        double-mint the record.
      proves: >-
        act commit is all-or-nothing (or its recovery is replay-safe); no
        half-staged cast can persist.
      evidence_class: local test + code inspection
    - action: >-
        grep shows resumeStrandedFrozenAssignmentCommits and
        enforceEngineeringAssignmentDeadlines no longer called from
        management_controller.go:301-302, and the sweep functions exist
        only behind the derivable-wake path.
      proves: selection-path sweeps are deleted, not duplicated.
      evidence_class: static analysis
  rollback: git revert + redeploy; both changes are additive guards on
    existing machinery.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the number of desk-fate and commit-correctness behaviors that
    still depend on the tool-loop selection path, while preserving the
    fail-closed I26 contract and replay idempotency.
  goodharting_would_be: >-
    A new ticker or sweep that fires under the same selection path (the
    wedge persists, renamed), or a "transaction" that wraps the appends but
    leaves the cursor commit outside it — both look done and fail the
    parked-management probe.

homotopy:
  realism_axis: >-
    Wake coverage: low = a single armed wake per assignment transition
    (current); high = every fate transition derivable from durable state
    regardless of which actor runs. This mission moves management-gated
    scans onto derivable wakes without inventing a new periodic-timer
    concept.

boundaries:
  mutation_class: red
  authority_sources: [owner, doctrine, desk-rlm-rectification-plan-2026-09-23 §11]
  must_preserve:
    - I26 fail-closed contract — expired assignments still release their slot
    - replay idempotency — a re-reduced cell re-derives the same record id
    - the derivable-wake (NotBefore/due-index) mechanism — no new ticker
    - the assignment/fate saga as host machinery
  excluded:
    - desk-cell liveness for non-engineering desks (R3b)
    - texture ledger consumer (R3a) / management cast (R3c) / authoring (R3d)
    - scores, materiality, context packs (R4)
    - durable vocabulary migration (R5)
    - deleting the fate saga itself
  protected_surfaces:
    - canonical event/commit path (rlm_reduce.go commit + CommitInboxCursor)
    - assignment fate saga + dispatcher due-index
    - management reconcile path (sweep removal)

now:
  status: landed
  slice: >-
    landed — repairs merged (ebdaef45, 53035642): deadline wake armed at
    open+bind, both selection sweeps deleted, commit self-heals via
    recoverPartialActCommit. Deployed-cancel proof deferred (see
    blocker_or_risk).
  source_ref: main@53035642
  deploy_identity: staging https://choir.news build.commit=53035642
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: r2x-fate-and-commit-integrity
    claim: >-
      Moving the deadline-cancel and stranded-freeze scans onto the existing
      derivable-wake path, and making act-commit atomic (or replay-safe on
      partial commit), closes the two substrate seams without new machinery
      — the spine then builds on a carrier whose correctness does not depend
      on which desk is selected.
    test: >-
      The parked-management staging probe above: a deadline expiry lands
      with management unselected; a mid-commit reducer failure leaves no
      half-staged record.
    edge: missing_oracle — pre-K assignments with no armed wake, and
      stranded-freeze states the outbox rules miss, are coverage the audit
      must enumerate; an unlisted state survives falsely.
    delta_o: >-
      A coverage audit listing every assignment state that can be created
      without an armed wake (pre-K rows, hand-opened, replayed), so the
      deletion is proven against the full state set, not the happy path.
    scope_if_supported: all desk assignments, including management's own
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11
  decision:
    what: >-
      Fold both repairs into one substrate-integrity mission on the spine;
      fate sweeps become derivable wakes (not a new timer), commit becomes
      atomic-or-recovering.
    kind: operational
    status: settled
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md §11.2/§11.4
    owner_ratification_ref: owner 2026-09-25 — plan §11 + this goal ratified
  belief:
    believed_state: >-
      Audit result: resumeStrandedFrozenAssignmentCommits is fully covered
      by the fate watchdog (armed at every pending-fate transition,
      fate.go:720/:851) + boot reconcile (runtime.go:618) — safe to delete.
      enforceEngineeringAssignmentDeadlines is NOT covered: a healthy
      bound+active assignment never arms the watchdog, so a new wake armed
      at bind (fires at CreatedAt+engineeringAssignmentDeadline=6h) is
      required before deletion. commit() non-atomicity still open.
    main_uncertainty: >-
      Whether a store-level transaction can wrap record+envelope+cursor,
      or the mission lands a named partial-commit recovery; and whether a
      deadline wake handler must cancel non-pending bound assignments
      (extending resumeStrandedFateAssignmentIfPending's contract).
    next_observation: >-
      A store transaction primitive for the commit triple, else the
      partial-commit recovery design.
  blocker_or_risk: >-
    Deployed proof deferred: no env passthrough to the guest (kernel-cmdline
    allowlist in autoputer-vm.nix) and no public wake-append route, so a
    "short deadline" on staging needs a choir.assignment_deadline cmdline
    param + VM package deploy. CHOIR_ASSIGNMENT_DEADLINE env override is
    committed as the lever; local test
    (TestDeadlineWakeCancelsExpiredBoundAssignmentWithoutManagementSelection)
    proves wake->cancel with no management selection. Follow-up: plumb the
    cmdline param, run the deployed probe, or fold into a later station.
    Per owner directive (No Blocking Asks): defer, do not ask.
  next_action: hand station pointer to R3a (texture ledger consumer)

receipts: []
---

# Carrier Substrate Integrity (R2x)

The seam between "R2 closed on substrate" and "desks go live" (R3b/R3c).
Two repairs R2 discovered but deferred — both are the strand-2 failure
shape, both block live desks from being trustworthy:

1. **Act commit is not atomic** — `commit()` appends the commitment record,
   mails the envelope, then commits the inbox cursor as three sequential
   steps. A crash mid-commit can leave a staged cast half-durable.
2. **Fate is selection-gated** — the computer-wide deadline cancel and
   stranded-freeze resume run only when management reconcile is selected.
   Park management and expired assignments never release their slot.

Neither invents new machinery: the derivable wake (`assigned_engineering_fate_deadline`,
`delegated_assignment_spawn_deadline`) and the reducer commit path already
exist. The work is coverage + deletion, not a build.
