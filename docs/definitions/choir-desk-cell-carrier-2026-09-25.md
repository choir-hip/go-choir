---
definition_version: 3
definition_id: choir-desk-cell-carrier-2026-09-25
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T02:50:00Z'
  source:
    canonical_ref: main@4cf82057
    deploy_identity: staging https://choir.news build.commit=4cf82057
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-texture-ledger-consumer-2026-09-25
    disposition: satisfied — R3a landed 2026-09-25 (dual-read over
      commitment_record; Addressee+EvidenceRefs on the record; typed
      divergence surfacing). Deployed live-record acceptance deferred —
      handed to this station's doc desk report-cast (see its receipts).
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md §11 (R3b)
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: R3b

finish:
  outcome: >-
    Non-capsule desks (management, texture, research) run model-authored yaegi
    cells inside a killable host subprocess, not the host process. The
    capsule-broker sessionWorker pattern is lifted host-side: each activation
    spawns a persistent framed-eval worker in its own process group with a
    restricted stdlib export set; InCellCarrier fans per profile; choir.Cast
    validates target desk + kind.
  artifact: >-
    A host-side session-worker spawn (socketpair + FramedConn + process-group
    kill + post-prebind ready handshake) reachable from the non-capsule desk
    activation path, replacing in-process desk-cell evaluation; the profile
    allowlist bounds each desk's staged choir verbs.
  acceptance:
    - action: >-
        A management-profiled activation runs a yaegi eval in a dedicated
        host subprocess (its own process group, separate PID, restricted
        exports) and the cell's staged intents reduce through the canonical
        ledger.
      proves: the host desk-cell carrier executes desk cells out-of-process.
      evidence_class: local test + code inspection
    - action: >-
        Killing a desk's session worker mid-cell re-fires a derivable wake:
        the interrupted cell does not silently drop its staged intents, and a
        replacement worker serves the next eval.
      proves: kill containment + derivable-wake recovery; no orphaned cell.
      evidence_class: local test
    - action: >-
        Restricted stdlib is enforced per profile: a management cell cannot
        import os/exec or mutate the filesystem; a desk's staged verbs are
        bounded by its role allowlist, and choir.Cast rejects an invalid
        target desk or kind.
      proves: the carrier carries the correct per-desk boundary.
      evidence_class: local test
    - action: >-
        On staging: a non-capsule desk turn runs through the host session
        worker — observed via a process/listing or run receipt that names the
        worker pid/profile — not in the host process.
      proves: the carrier is live on the deployed computer.
      evidence_class: deployed proof
  rollback: >-
    git revert + redeploy. InCellCarrier stays gated on the actuator flag, so
    revert restores in-process desk cells for non-capsule desks; capsule desk
    carrier (capsule_go_eval) is unaffected.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Desk cells run out-of-process so a misbehaving cell (runaway, OOM,
    poisoned interpreter) is contained and killable, and the host daemon is
    never at risk from untrusted model-authored Go — the same containment the
    capsule guest already has.
  goodharting_would_be: >-
    A desk that "runs in a subprocess" by name only: a worker spawn that
    still shares the host address space, or a kill that orphans the cell's
    staged intents. The acceptance insists on a separate PID boundary AND a
    derivable wake on kill — not a cosmetic wrapper.
  falsifiers:
    - 'Falsified: a desk cell still evals inside the host process — the host
      session worker spawn is unwired or bypassed.'
    - 'Falsified: killing the worker mid-cell loses the cell''s staged intents
      or blocks the next eval — the derivable wake is fake.'
    - 'Falsified: a desk cell escapes its allowlist (imports os/exec, mutates
      files outside its root) — the restricted stdlib is nominal.'
    - 'Falsified: only Engineering reaches the carrier — the InCellCarrier fan
      never reaches management/texture/research.'

now:
  status: working
  slice: >-
    station R3b — host desk-cell carrier. R3a landed: ledger evidence
    dual-read; deployed live-record acceptance deferred to a doc desk
    report-cast (which this station's carrier enables once it can drive a
    desk report). R3b spawns a host sessionWorker for non-capsule desks.
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: host-cell-carrier-contains-desks
    claim: >-
      Lifting the capsule-broker sessionWorker pattern to the host (socketpair
      + FramedConn + process-group kill) runs every desk's model-authored
      cells in a killable subprocess with a per-profile restricted stdlib,
      so desk containment matches the guest — kill mid-cell re-fires a
      derivable wake, never loses staged intents.
    test: >-
      A management activation evals yaegi in a separate-PID host worker;
      killing it mid-cell re-fires the wake and the next eval respawns a
      worker; allowlist denial is enforced per profile.
    delta_o: >-
      If containment is proven host-side, R3c (management live + cast) can
      drive real cross-desk casts; texture desk (R3d) reuses the same
      carrier.
    scope_if_supported: non-capsule desk cell execution host-side
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (R3b)
  belief:
    believed_state: >-
      The capsule-broker sessionWorker (cmd/capsule-broker/session_worker.go,
      unix socketpair + FramedConn + Setpgid/Pdeathsig + ready handshake) is
      the proven killable-subprocess pattern, today reachable only inside the
      guest for Engineering+actuator=rlm. Non-capsule desks run cells in the
      host process (no subprocess boundary). yaegikernel already exposes
      ExecuteWorkerSessionConn/SessionWorkerConfig/FramedConn; the desk
      module set (choir.go deskModuleSet) already bounds per-role verbs.
    main_uncertainty: >-
      Whether host-side spawn is a lift of the existing pattern or a build:
      the worker binary entrypoint (--isolation-stage exec-go-session) is a
      capsule-broker cmd; a host equivalent must be reachable. The derivable
      wake on kill is the second unknown — the R2x deadline-wake machinery
      may already cover it.
    next_observation: >-
      Where the non-capsule desk activation actually evals its cell today,
      and the smallest seam to route it through a host sessionWorker.
  blocker_or_risk: >-
    Sharpest execution risk in the spine (per panel): host spawn for
    non-capsule desks. Decompose before patching — do not special-case a
    single desk. No Blocking Asks applies: fork decisions go to
    agentic-consensus or a conservative default documented here.
  next_action: >-
    Locate the non-capsule desk cell-eval entrypoint; decide lift-vs-build
    for the host sessionWorker spawn; wire a host worker binary + spawn.

receipts: []
---

# R3b — Host Desk-Cell Carrier (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11:
host-side spawn of the sessionWorker pattern for non-capsule desks;
restricted stdlib; `InCellCarrier` fanned per profile; `Cast` validates
target desk + kind. Kill mid-cell → derivable wake re-fires.
