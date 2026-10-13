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
readiness: drafted

review:
  reviewer: none (the design it executes is reviewed; this file is not)
  frozen_ref: none
  verdict: none
  evidence_ref: docs/restart-subsystem-design-2026-10-13.md

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
      A guest writes its own planned marker before each platform-update,
      self-development apply or rollback, so an apply that crashes the
      runtime boots as planned and applies again (finding 5).
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

now:
  status: working
  slice: L1a-3 — crash boots run no re-driver (platform-update resume, engineering capsule reconcile, self-development materialization reconcile, the selfdev_materialization_retry exemption) and settle those transitions to terminal fates
  source_ref: fdc70329
  deploy_identity: staging 8a1477c3 (fdc70329 deploying)
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
  blocker_or_risk: L1a-3 touches the same boot re-drivers C1e deletes; C1e waits for it
  next_action: document the crash-boot re-drivers that still run (problem doc), then land L1a-3

receipts:
  - id: L1a-1
    boundary: implement
    identity: dc184db2
    proof_refs: [nix eval of rendered guest and host units, .github/workflows/ci.yml reset-failed]
    rollback_ref: revert dc184db2
    disposition: start limits on every restarting guest (5 in 10 min) and host (10 in 10 min) unit; deploys reset-failed before restart; warm filter moved to L1b
  - id: L1a-2
    boundary: implement
    identity: c08ead07
    proof_refs: [internal/agentcore/restart_bound_test.go]
    rollback_ref: revert c08ead07
    disposition: durable per-apply attempt budget (3); crash boots mark pre-boot work wakes interrupted_by_restart
---

# L1: no crash loops

The design is the specification; this file tracks its execution.

- **L1a** (guest and Nix, before C1e): start limits (L1a-1), bounded
  self-applies and wake gating (L1a-2), crash boots run no re-driver
  (L1a-3).
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
