---
definition_version: 4

# Station L1 of the ledger metamission
# (docs/definitions/choir-ledger-metamission-2026-10-13.md): no crash loops.
# Owner 2026-10-13, in session: "we never get into a crash loop". The
# specification is the restart design v3
# (docs/restart-subsystem-design-2026-10-13.md), reviewed by a lateral
# round (7 of 7) and a convergent round (5 ready with changes, 1 not ready)
# adjudicated in its §9. This file is the executable station over that
# design; the design stays the specification.
# Review: convergent round 2 on d46d3d08 (opencode, gemini, deepseek, glm
# accept; claude send_back for four documentation repairs, adopted after the
# round: receipt landing blocks, the two recorded deviations, red ceremony,
# the post-commit trigger named in L1a-3).
readiness: reviewed

review:
  reviewer: agentic-consensus convergent round 2 on d46d3d08 (4 accept, 1 send_back adopted)
  frozen_ref: git blob 761687463de8e9c3636915b1f7b2148d12249457 (this file at d46d3d08)
  verdict: accept
  evidence_ref: .agentic-consensus/ledger-metamission-convergent-r2-2026-10-13/

start:
  captured_at: 2026-10-13T20:40:00Z
  source:
    canonical_ref: main fdc70329
    deploy_identity: staging 8a1477c3
  worktrees:
    - path: /Users/wiz/go-choir (untracked `sourcecycled` at the repo root)
      status: dirty
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Before L1a-1, guest units (runtime, updater, both signers) restarted 1 s
      after any failure with no start limit, and host units had none either
      (nix/autoputer-vm.nix; design §9 finding 5).
    - >-
      Before L1a-2, a guest wrote its own planned marker before each
      platform-update, self-development apply or rollback, so an apply that
      crashed the runtime booted as planned and applied again without bound
      (finding 5).
    - >-
      Boot runs platform-update resume, engineering capsule reconcile and
      self-development materialization reconcile on every boot, crash or
      planned (internal/agentcore/runtime.go Start); only run closure,
      round closure, pre-decision self-development closure and source
      schedule closure gate on the boot kind.
    - >-
      ResumeVM calls launchFirecracker without ReserveBootEpoch; resolve,
      page meta-refresh and proxied requests can each boot a failed
      computer; warm sweeps warm wedged computers (findings 1, 3, 8).

finish:
  deliver: >-
    Computers that cannot crash-loop: after any external event a computer
    gets at most (1 + allowance) spawns times each unit's start-limit burst,
    then a visible hold that only an explicit owner act or a different build
    releases; a crash boot never re-drives work.
  artifact: the boot ledger and holds visible in `vmctl list` and the owner's computer page; the fault-injection matrix receipt from a minted test computer
  acceptance:
    - action: the design §5 fault-injection matrix (with §9's added cells) on a minted test computer on staging
      proves: every cell ends in a bounded number of spawns and a visible hold or a clean run
      evidence_class: deployed proof
    - action: crash boot of a minted test computer with a pending platform update, a materializing self-development op and an open engineering assignment
      proves: no re-driver runs; each transition settles to a terminal fate and later updates are not refused
      evidence_class: deployed proof
  rollback: revert per slice; tag the last-good commit before L1b
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize automatic boots and re-drives per external event, while every
    computer the owner expects running stays reachable without an operator.
  goodharting_would_be: >-
    No loops because nothing restarts automatically; holds that never
    release; a bound counted in host spawns that misses in-guest unit
    restarts.

homotopy:
  realism_axis: >-
    unit tests -> nix eval of rendered units -> a minted test computer on
    staging with injected faults -> the paper computer -> the owner's
    computer

boundaries:
  mutation_class: red
  authority_sources:
    - owner 2026-10-13 in session ("we never get into a crash loop")
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)"
    - owner wedge ruling 2026-10-10 (internal/vmctl/wedge.go)
    - docs/restart-subsystem-design-2026-10-13.md v3 §6 and §9
  must_preserve:
    - a crash boot never resumes or re-drives work
    - a planned boot resumes once; target invariant (repaired by L1c) — only when the host attests it
    - a wedged computer the owner expects running gets one automatic retry before a hold
    - the hold is the existing SetHold/ClearHold state, one authority
  excluded:
    - management-owned reopen of work after a crash
    - schedules option B before the owner ratifies it
    - A/B image slots
  protected_surfaces: [vmctl boot actuator and ownership, guest boot and systemd units, run acceptance, self-development materialization, platform update]
  admissible_evidence: deployed proof on a minted test computer on staging; unit tests and nix eval shape a slice but never prove vmctl, boot or restart behavior
  deviations_from_design:
    - id: D1
      design: §6 item 1, "a markerless or guest-self-marked boot runs no re-driver"
      instead: L1a covers markerless (crash) boots; a guest-self-marked boot still resumes its own apply, bounded at 3 attempts per apply (L1a-2), because the post-restart re-push is the second half of every apply
      repaired_by: L1c (the host asserts planned boots; the guest writes no marker)
      consequence: until L1c, the matrix cell "an apply that crashes the runtime" shows up to 3 replays per apply, by design, not a regression
    - id: D2
      design: §6 item 1, "start limits and OnFailure on every restarting unit"
      instead: L1a-1 added start limits only; a guest unit that latches failed is silent while the VM stays up
      repaired_by: L1c visibility (the ledger tail in vmctl list and an alert on any hold)
  heresy_delta:
    discovered: [guest self-written planned markers, unbounded guest and host units, ResumeVM launching without a grant, warm sweeps booting wedged computers, crash boots re-driving platform updates and self-development applies, a leaked planned marker after a replayed apply, a lost apply response read as a refusal]
    introduced: [up to 3 replays per apply on guest-self-marked boots until L1c (D1)]
    repaired: [unbounded units (L1a-1), unbounded self-applies and crash-boot re-drive of pre-boot work wakes (L1a-2)]

now:
  status: working
  slice: L1b-0, the warm-sweep bound, pulled ahead of the harness (2026-10-14 00:20 UTC). Computer tiers (188f98c3) put the paper computer in the always-on sweep, and the sweep warms a computer stopped as wedged or recovery_failed on every 2-minute pass, so a computer that wedges after each boot loops about every 7 minutes with no bound (the design's section-2 row "Always-on warm ... including stopped_by=wedged: no"). Zero retries is not the fix: the 2026-10-10 owner ruling (wedge.go header) says the always-on policy boots a wedged computer and the owner never has to, and nothing else opens the paper computer. L1b-0 is the design's "one automatic retry, then hold" as an interim: one automatic warm per failure stop per 24 h, written to the ownership before the boot (FailureWarmedAt, deleted when the ledger lands). L1a landed (L1a-1, L1a-2, L1a-3); after L1b-0 the fault-injection harness for the matrix cells, then L1b proper. Was L1a-3 — crash boots run no re-driver and settle from the updater journal (docs/problems/crash-boots-redrive-applies-2026-10-13.md); 3a platform-update resume and the leaked marker; 3b self-development reconcile, both the boot trigger and the post-commit observer, for every op decided before the boot (engineering capsule reconcile only closes work; the retry wake finds nothing once 3b settles)
  source_ref: 18d01cc0
  deploy_identity: staging 18d01cc0 (CI 38463609655 success)
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: L1-1
    claim: >-
      Two bounds suffice: unit start limits bound in-guest restarts, and
      boot-kind gating removes the re-drive that turns one crash into a
      loop. With both, a crashing apply costs at most its attempt budget
      times the start-limit burst before the unit latches failed.
    test: the matrix cell "apply crashes the runtime on every boot" on a minted test computer
    edge: missing_oracle
    delta_o: the boot ledger (L1b) counting spawns per external event
    scope_if_supported: guest-side loops on staging
    status: testing
    evidence_refs: [dc184db2, c08ead07]
  decision: {what: "L1a in three slices before C1e; L1b beside C1e; L1c after", kind: operational, status: proposal, evidence_ref: docs/restart-subsystem-design-2026-10-13.md, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      L1a-1 (start limits, reset-failed on deploy) and L1a-2 (3 attempts per
      self-apply across boots; crash boots mark pre-boot work wakes
      interrupted) are pushed in fdc70329's batch. L1a-2 bounds
      guest-self-marked replays at 3 attempts rather than refusing every
      re-drive on a guest-self-marked boot, so planned updates still resume
      until L1c moves planned-ness to the host.
    main_uncertainty: whether gating self-development materialization reconcile on crash boots strands post-decision ops that tests expect to recover (TestSelfDevReconcileRecoversMaterializingOperation)
    next_observation: which boot re-drivers already settle on crash and which re-drive
  blocker_or_risk: L1a's code is landed, so C1e may start; L1a's deployed matrix cells are still unproven. A loop slower than the start limit (one crash every 2 minutes or more, such as a poisoned record re-read after each boot) escapes the unit bound; the boot ledger (L1b) is the bound for it
  next_action: land L1b-0 with the resource mission's test tier (one vmctl push); then build the matrix fault-injection harness (a minted test computer with a pending platform update or a decided self-development op, then a crash restart) and run L1a's cells; then L1b

receipts:
  - id: L1a-1
    boundary: implement
    identity: dc184db2
    proof_refs: [nix eval of the rendered units in session (output not kept), .github/workflows/ci.yml reset-failed]
    rollback_ref: revert dc184db2
    disposition: start limits on every restarting guest (5 in 10 min) and host (10 in 10 min) unit; deploys reset-failed before restarting host units (not the Dolt units, which deploys do not restart); warm filter moved to L1b
    landing: {source_commit: fdc70329 (batch), ci_ref: 38456685282 success, deploy_ref: CI deploy job success, environment_identity: staging fdc70329, deployed_acceptance: not yet run (L1's matrix cells)}
  - id: L1a-2
    boundary: implement
    identity: c08ead07
    proof_refs: [internal/agentcore/restart_bound_test.go]
    rollback_ref: revert c08ead07
    disposition: durable per-apply attempt budget (3); crash boots mark pre-boot work wakes interrupted_by_restart
    landing: {source_commit: fdc70329 (batch), ci_ref: 38456685282 success, deploy_ref: CI deploy job success, environment_identity: staging fdc70329, deployed_acceptance: not yet run (L1's matrix cells)}
  - id: L1a-3
    boundary: implement
    identity: 18d01cc0
    proof_refs: [docs/problems/crash-boots-redrive-applies-2026-10-13.md, internal/agentcore/crash_settle_test.go, docs/evidence/l1a3-landing-2026-10-13.json]
    rollback_ref: revert 18d01cc0
    disposition: crash boots settle platform updates and decided self-development ops from the updater journal and never call the updater; the planned marker clears whenever an apply returns
    landing: {source_commit: 18d01cc0, ci_ref: 38463609655 success, deploy_ref: CI deploy job success, environment_identity: staging 18d01cc0, deployed_acceptance: crash scenario l1a3-crash-2 pass (regression); matrix cells not yet run}
---

# L1: no crash loops

The design is the specification; this file tracks its execution.

- **L1a** (guest and Nix, before C1e): start limits (L1a-1), bounded
  self-applies and wake gating (L1a-2), crash boots run no re-driver
  (L1a-3). Two departures from the design are recorded as D1 and D2 in
  `boundaries.deviations_from_design`.
- **L1b** (host, may run beside C1e): grants at `launchFirecracker` keyed by
  spawn sequence; exhaustion sets the existing hold with a
  `recovery-ledger` class; a wedged or failed computer gets the one
  automatic retry and then holds; owner acts are explicit mutations only;
  the held page does not refresh; deploy refresh releases a recovery hold
  only for a different build.
- **L1c** (after C1e): the host asserts planned boots; the guest writes no
  marker of its own; crash closure becomes a barrier. Then visibility (the
  ledger tail in `vmctl list` and on the owner's page, an alert on any
  hold).
