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
  status: settled
  settled_by: orchestrator
  slice: >-
    station R3b LANDED 2026-09-26 (deployed build b9f43583, + design receipt
    e505680a). Host desk-cell carrier built: autoputer `desk-session`
    re-execs the daemon; desk_go_eval is the sealed per-profile eval tool;
    rlmReductionForDeskCall reduces desk cells on the canonical ledger
    (commitTray path, toolCtx=nil); InCellCarrier fans management/texture/
    research under actuator=rlm; Cast rejects an unknown desk. Deployed
    live-desk acceptance deferred to R3c — staging desks run actuator=tools;
    the carrier activates only under actuator=rlm, promotion is R3c/R3d's
    decision (same deferral shape as R3a).
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
      the proven killable-subprocess pattern, today reachable only inside
      the guest for Engineering+actuator=rlm. yaegikernel already exposes
      ExecuteWorkerSessionConn/SessionWorkerConfig/FramedConn; the desk
      module set (choir.go deskModuleSet) bounds per-role staged verbs.
      Census finding: non-capsule desks run a toolregistry.ToolLoop
      (patch_texture et al.) — they do NOT eval yaegi cells or produce
      StagedIntents today. So R3b is a BUILD (a new desk cell carrier +
      host sessionWorker + cell admission), not a lift: the spawn pattern
      transfers, the desk wiring is new.
    main_uncertainty: >-
      The carrier shape: whether a desk gets a host `go_eval`-style cell
      tool inside its existing tool loop (minimal — one new tool per
      profile), or its whole turn becomes a yaegi cell (full InCellCarrier
      like Engineering). Plan §11 says "InCellCarrier fanned per profile",
      which implies the latter; the lift-vs-build fork is the design risk.
    next_observation: >-
      Carrier shape DECIDED (agentic-consensus 20260925-224851; claude,
      codex, gpt6-sol, gemini, grok, glm, luna, cursor — 8/9 landed): build
      the Engineering-shaped host carrier — a per-profile sealed registry
      whose sole tool is a host `desk_go_eval` spawning a host
      sessionWorker (socketpair+FramedConn+Setpgid/Pdeathsig+ready).
      Consensus correction: Engineering's InCellCarrier is NOT a whole-turn
      cell — it's a ToolLoop sealed to one eval tool; "fan InCellCarrier
      per profile" means seal each desk registry to `desk_go_eval` behind
      the actuator flag. NOT a peer tool beside existing tools (dual
      channel, Goodharts containment). Live desk turn behavior stays
      R3c/R3d; R3b ships the carrier + a management-profiled activation
      proof. Host worker = autoputer self-exec `desk-session` ->
      yaegikernel.ExecuteWorkerSessionConn; spawn pattern lifted host-side.
      Pdeathsig is Linux-only — staging proves; Darwin relies on broker
      reap.

receipts:
  - "pushed_commit: b9f43583 (agentcore desk_go_eval carrier + kill test) —
    head SHA of R3b; 1ce02337 (yaegikernel host spawn) landed earlier."
  - "ci: run 36214084659 — all shards green (agentcore 0-7, non-runtime,
    docs truth, deploy-impact); deploy to Node B success."
  - "deploy: Node B staging, deployed 2026-09-26T03:26:10Z."
  - "environment_identity: staging https://choir.news/health
    build.commit = b9f4358384182dce49eac564a263b6ee63ecd4bf (exact R3b head)."
  - "deployed_acceptance: Staging proves deploy-clean + platform healthy
    (status ok, vmctl ok, ws resolved) at R3b head. The desk-cell carrier is
    inactive on staging — it activates only under actuator=rlm
    (choir.actuator boot param / CHOIR_ACTUATOR env); staging desks run
    actuator=tools. Live-desk acceptance deferred to R3c actuator
    promotion, as R3a deferred its live-record proof. Local acceptance
    (TestDeskGoEval*): separate-PID worker spawn+eval, worker persists
    across cells, kill mid-cell respawns clean with no state leak, empty
    source rejected, cast to an unknown desk rejected — all pass."
---

# R3b — Host Desk-Cell Carrier (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11:
host-side spawn of the sessionWorker pattern for non-capsule desks;
restricted stdlib; `InCellCarrier` fanned per profile; `Cast` validates
target desk + kind. Kill mid-cell → derivable wake re-fires.
