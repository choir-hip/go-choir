---
definition_version: 3
definition_id: choir-strand2-freeze-order-patch-2026-09-23
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-23T22:30:00Z'
  source:
    canonical_ref: main@6f064940
    deploy_identity: staging https://choir.news build.commit=c8811ff8
  worktrees:
    - path: /Users/wiz/go-choir
      status: unknown
      class: unknown
      owner: unknown
      touch: read_only
      recovery: reconcile at charter
  predecessor:
    mission: choir-sub-rlm-document-channel-2026-09-22
    disposition: >-
      independent patch — the strand-2 diagnosis is adjudicated
      (9-agent consensus, .agentic-consensus/agentic-consensus-20260923-191530)
      and the fix does not depend on the desk-RLM rebuild. Promotable
      immediately; do not wait for the rectification sequence.
    evidence_ref: docs/problems/document-cast-stray-complete-settlement-2026-09-23.md
  observed_artifact:
    - claim: >-
        freezeCapsuleEffectBundle (internal/agentcore/tools_capsule.go:235)
        calls ExtractGranted — the only production caller of Capsule.Quiesce
        outside the Complete saga — as its first effect, before validating
        receipts, before GetByTrajectory, before the head check. A
        choir.Freeze intent on a document trajectory (no selfdev operation)
        freezes the executor then fails post-freeze: executor Frozen or
        wedged Quiescing, store active, zero events.
      claim_scope: current
      evidence_ref: docs/problems/document-cast-stray-complete-settlement-2026-09-23.md
    - claim: >-
        The prompt commands the bug: rlm_engineering_runtime.yaml:61
        instructs every engineering-desk assignment — including
        document-channel casts — to call choir.Freeze then choir.Complete.
        The overlay was written for the selfdev flow; document casts carry
        no selfdev operation.
      claim_scope: current
      evidence_ref: internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml

finish:
  deliver: >-
    A stray or prompt-mandated choir.Freeze on a non-selfdev trajectory can
    no longer freeze the executor capsule. The freeze intent validates its
    selfdev authority before any executor effect; the prompt no longer
    mandates Freeze on document casts; the failure modes that wedge
    Quiescing or skip the watchdog are closed.
  artifact: >-
    Deployed staging state where: (a) freezeCapsuleEffectBundle runs all
    selfdev authority checks (GetByTrajectory, StateExecuting, head check,
    authority presence) before ExtractGranted; (b) the engineering overlay
    gates the Freeze instruction behind selfdev-operation presence;
    (c) commitFreezeIntent refuses before any executor effect when no
    resolvable selfdev operation exists; (d) Capsule.Quiesce restores
    StateActive on cgroup-freeze failure; (e) the Freeze reduce path uses
    WithoutCancel like Complete; (f) every terminal-saga failure inside the
    freeze_requested block arms the fate watchdog.
  acceptance:
    - action: >-
        On staging, drive a document-channel cast and have the assigned cell
        stage a choir.Freeze intent on the document trajectory (no selfdev
        operation). Observe: the intent is refused before any executor
        effect; the capsule stays Active; the assignment remains bound and
        completable; a subsequent choir.Complete reduces normally.
      proves: freeze-before-validate is closed; the strand-2 wedge class
        cannot recur via this path
      evidence_class: deployed proof
    - action: >-
        Unit-level: a staged Freeze on a trajectory with no selfdev
        operation returns an error with zero executor effects (executor
        state unchanged, no Quiesce call observed).
      proves: validation precedes effect
      evidence_class: local test
  rollback: git revert of the patch commits + redeploy.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the set of intent paths that can mutate executor state before
    their authority is validated, while preserving the legit selfdev
    freeze→verify→complete flow.
  goodharting_would_be: >-
    Refusing Freeze on document trajectories by trajectory kind or prompt
    text — a special-case that leaves the effect-before-validation order
    intact for the next caller.

homotopy:
  realism_axis: >-
    Validation-order strictness: from "effect first, validate later"
    (current) through "validate authority before effect" (this mission) to
    "all executor effects gated on committed intent" (the kernel's fenced
    commit, K). This mission is the middle rung — same code path, reordered.

boundaries:
  mutation_class: orange
  authority_sources:
    - docs/problems/document-cast-stray-complete-settlement-2026-09-23.md
    - .agentic-consensus/agentic-consensus-20260923-191530 (adjudicated diagnosis)
    - AGENTS.md
  must_preserve:
    - the selfdev freeze→verify→complete flow on selfdev-bound trajectories
    - ExtractGranted's receipt resolution requiring StateFrozen (stays post-freeze)
    - the Complete saga's store-write-before-executor ordering
  excluded:
    - the desk-RLM rebuild (R2/R3)
    - the commitment ledger (R2)
    - durable vocabulary rename (R5)
    - the store/executor reconcile sweep (K's periodic timer owns it)
  protected_surfaces:
    - capsule executor state machine (internal/capsule)
    - assignment fate saga (internal/agentcore/cosuper_assignment_fate.go)
    - prompt overlay assembly (internal/runtimeprompts)

now:
  status: working
  slice: freeze-order patch
  source_ref: main@6f064940
  deploy_identity: staging https://choir.news build.commit=c8811ff8
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: r0-validate-before-effect
    claim: >-
      Reordering freezeCapsuleEffectBundle so all selfdev authority checks
      precede ExtractGranted, plus gating the prompt's Freeze mandate behind
      selfdev-operation presence, closes the zero-event wedge class without
      breaking the selfdev freeze flow.
    test: >-
      Staging reproduction: a document-cast cell staging Freeze is refused
      with zero executor effects; a selfdev-bound Freeze still freezes and
      completes.
    edge: missing_oracle — a second post-freeze failure step
      (classifier-reject returning Rejected with err=nil) may strand the
      same way; the acceptance must cover both error and reject paths.
    delta_o: >-
      Assert on executor state (InspectCapsuleRaw) after the refused intent,
      not only on the error return.
    scope_if_supported: >-
      The freeze intent path is safe for arbitrary trajectories; the wedge
      class narrows to kernel-owned reconcile for non-intent paths.
    status: active
    evidence_refs:
      - docs/problems/document-cast-stray-complete-settlement-2026-09-23.md
  decision:
    what: >-
      Fix in the shared freezeCapsuleEffectBundle body (covers intent path
      and any future caller); prompt gate in the overlay; Quiesce restores
      Active on failure; Freeze reduce gets WithoutCancel; watchdog arms on
      every terminal-saga failure in the freeze_requested block.
    kind: operational
    status: settled
    evidence_ref: docs/problems/document-cast-stray-complete-settlement-2026-09-23.md
    owner_ratification_ref: not_applicable — adjudicated bug fix, not architecture
  belief:
    believed_state: >-
      Diagnosis adjudicated by 9-agent consensus; fix direction agreed;
      the patch is small and independent of the desk rebuild.
    main_uncertainty: >-
      Whether the classifier-reject path (BuildBundleFromDiff returning
      Rejected with err=nil post-freeze) needs its own guard beyond the
      reorder — the reorder prevents the freeze, but a reject-shaped success
      may still mislead the model.
    next_observation: >-
      The staging reproduction: does the refused Freeze leave the capsule
      Active and the assignment completable.
  blocker_or_risk: none — promotable immediately
  next_action: promote and execute; the fix is adjudicated and independent

receipts: []
---

## Scope

Six changes, one commit set:

1. `freezeCapsuleEffectBundle` (`internal/agentcore/tools_capsule.go:235`):
   move `GetByTrajectory`, `StateExecuting`, head check, and authority
   presence *before* `ExtractGranted`. Receipt resolution stays after —
   `ResolveGrantedExecutionReceipts` requires `StateFrozen`.
2. `commitFreezeIntent` (`internal/agentcore/rlm_reduce.go:485-497`):
   refuse before any executor effect when no resolvable selfdev operation
   exists on the trajectory.
3. `rlm_engineering_runtime.yaml:61`: the Freeze mandate gates behind
   selfdev-operation presence — document casts must not be told to Freeze.
4. `Capsule.Quiesce` (`internal/capsule/capsule.go:126-127`): restore
   `StateActive` on cgroup-freeze failure (the ctx-cancel path already
   does).
5. Freeze reduce path (`rlm_reduce.go:436`): `WithoutCancel` + timeout,
   matching Complete (`:424`).
6. Watchdog coverage (`cosuper_assignment_fate.go:718-810`): arm on every
   terminal-saga failure inside the `freeze_requested` block, not only the
   tail commit.

## Non-goals

The store/executor reconcile sweep (kernel timer, K), the desk-RLM rebuild
(R2/R3), the commitment ledger (R2), durable vocabulary (R5). The stranded
assignment `e77f7940` on staging heals on restart or the next Super-selection
deadline sweep — this mission does not repair it.
