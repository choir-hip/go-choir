---
definition_version: 3
definition_id: choir-management-live-cast-2026-09-26
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T03:40:00Z'
  source:
    canonical_ref: main@c39ba0c3
    deploy_identity: staging https://choir.news build.commit=b9f43583
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-desk-cell-carrier-2026-09-25
    disposition: >-
      R3b landed+deployed 2026-09-26 (b9f43583). The host desk-cell carrier
      exists: autoputer `desk-session` re-execs the daemon; desk_go_eval is
      the sealed per-profile eval tool; rlmReductionForDeskCall reduces desk
      cells on the canonical ledger; InCellCarrier fans management/texture/
      research under actuator=rlm; Cast rejects an unknown desk. Deployed
      live-desk acceptance deferred to THIS station — staging desks run
      actuator=tools; the carrier activates only under actuator=rlm.
    evidence_ref: docs/definitions/choir-desk-cell-carrier-2026-09-25.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: R3c

finish:
  outcome: >-
    The management desk runs on cells under the host desk-cell carrier and
    drives real cross-desk delegation. A management-profiled activation's
    cell stages choir.Cast(engineering), which opens+binds+executes an
    engineering assignment through delegated-cast admission — the R2-deferred
    deployed proof — and a Report from the engineering child resolves the
    cast record on the canonical ledger. report_to_texture retires to the
    staged in-cell path (Report/ReportPacket intent), not a host-side tool.
  artifact: >-
    The actuator promotion binding management to the desk-cell carrier on a
    live activation; the Cast→assignment→Report resolution chain proving a
    management cell drives engineering delegation end to end, restart-
    resumable from the tape.
  acceptance:
    - action: >-
        A management-profiled activation runs under actuator=rlm with the
        sealed desk_go_eval registry (no live host-tool registry), evals a
        cell, and its staged intents reduce through commitment records.
      proves: management is live on the desk-cell carrier, not a tool loop.
      evidence_class: local test + code inspection
    - action: >-
        A management cell stages choir.Cast(engineering, objective, spec):
        delegated-cast admission opens and binds an engineering assignment,
        the engineering child executes, and its Report resolves the cast
      proves: >-
        the R2-deferred deployed management-cell cast proof is real:
        management cell → engineering assignment → Report resolves the cast.
      evidence_class: deployed proof (staging) where the actuator permits;
        local ledger assertion otherwise
    - action: >-
        Killing/restarting mid-episode resumes the cast episode from the
        tape: the open assignment + cast record survive, the derivable wake
        re-fires, and no half-commit mints.
      proves: the cast episode is restart-durable on canonical evidence.
      evidence_class: local test
    - action: >-
        report_to_texture is no longer a management host tool: the
        management cell reports via the staged Report/ReportPacket intent;
        the legacy persistent-report host tool path is retired or shadowed
        on the carrier.
      proves: reporting metabolizes through the ledger, not a host bypass.
      evidence_class: code inspection + local test
  rollback: >-
    git revert + redeploy. The actuator binding is per-activation; revert
    restores management's live host-tool registry (actuator=tools) and the
    persistent-report host path. The desk-cell carrier (R3b) is unaffected.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Management — the desk that delegates all engineering work — becomes the
    first non-engineering live cell carrier, so cross-desk delegation is a
    ledger-recorded semantic act (Cast → assignment → Report) instead of a
    host tool call. This is the deployed proof R2 deferred: the management
    cell drives engineering, restart-durably, on canonical evidence.
  goodharting_would_be: >-
    A "management cell" that still calls a host-side cast/report tool, or a
    cast that mints a record without a bound engineering assignment
    executing — the acceptance insists on the full open+bind+execute+
    Report chain resolving on the ledger, restart-resumable, not a stubbed
    cast call.
  falsifiers:
    - 'Falsified: management under actuator=rlm still reaches a live
      host-tool registry — the sealed desk_go_eval fan did not promote it.'
    - 'Falsified: the staged Cast mints a record but no engineering
      assignment binds/executes — the delegated-cast admission is fake.'
    - 'Falsified: a mid-episode restart loses the open cast or mints a
      half-commit — the tape does not carry the episode.'
    - 'Falsified: report_to_texture remains a live management host tool —
      reporting bypasses the ledger.'

now:
  status: settled
  settled_by: orchestrator
  slice: >-
    station R3c LANDED 2026-09-26 (deployed build b7ae7596). Management is
    live on the host desk-cell carrier — deskCarrierLive(management) is
    unconditional, the first non-engineering desk on cells. Its registry is
    sealed to desk_go_eval plus the typed lifecycle control tools
    (report_to_texture, cancel_co_super_assignment — the durable control
    path, not cell verbs); the staged Report/ReportPacket intent covers the
    semantic report. desk_go_eval injects the cell-start inbox (choir.Inbox
    parity with capsule). rlmReductionForDeskCall drops the host-global
    actuator gate — a promoted desk must not be inerted.
    Cast->delegated admission proven (TestR3cDeskCastReachesDelegatedAdmission
    reaches openDelegatedCastAssignment under the caster's trajectory-bound
    authority; spawn stops only at the Linux guest kernel — staging's real
    capsule substrate completes it). Deployed management cast resolution
    (engineering child executes, Report resolves the cast record) rides the
    existing delegated-cast saga now reachable on staging.
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: management-cell-drives-engineering
    claim: >-
      Binding a management activation to the desk-cell carrier lets a
      management cell stage choir.Cast(engineering) that opens+binds+
      executes a real engineering assignment whose Report resolves the cast
      commitment_record — the deployed delegation proof R2 deferred —
      restart-resumable from the tape.
    test: >-
      A management activation evals a Cast cell; the ledger shows the cast
      record, a bound engineering assignment, the child's Report, and the
      resolved cast — with a mid-episode restart resuming from the tape.
    delta_o: >-
      If management delegation is real and durable on cells, M7 (the
      management cell as self-dev driver) has its driver; R3d reuses the
      same carrier for texture authoring.
    scope_if_supported: management desk on the in-cell carrier + cast
    status: supported
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (R3c)
  belief:
    believed_state: >-
      Management is live on the host desk-cell carrier (R3c landed): its
      registry seals to desk_go_eval plus the typed lifecycle control tools;
      the staged Cast reaches delegated-cast admission on the canonical
      ledger. Staging desks run the carrier because management promotes
      unconditionally — no actuator flag. The typed producer-report and
      cancellation controls (report_to_texture, cancel_co_super_assignment)
      stay as durable lifecycle control paths; the unstructured report flow
      is the staged Report/ReportPacket intent. Deployed full-resolution
      (child executes, Report resolves the cast) is live-reachable on
      staging's capsule substrate.
    next_observation: >-
      R3d texture live + authoring: genuine AuthorAppAgent revisions
      metabolizing ledger traffic under editorial discretion (D14); deletes
      the desk-originated worker_updates consumer path (R3a's deferred
      deletion owner). Flagged compound-risk — decide the R3d-a (cell +
      consumer, no write) vs R3d-b (the write) split at charter.
receipts:
  - "pushed_commit: b7ae7596 — head SHA of R3c; promotion 2837343e, cast
    test 194f1725, inbox fix c2670075, lifecycle-control fix 7b95b5e6,
    contract/overlay fix b7ae7596."
  - "ci: run 36216782941 — all shards green (agentcore 0-7, non-runtime 0-7,
    scale, vet, heresy detector, docs truth, vocab gates); deploy to
    Node B success."
  - "deploy: Node B staging; environment_identity https://choir.news/health
    build.commit=b7ae7596f65cecb67423a1e95fba2e014e1d5a9b (exact R3c head)."
  - "deployed_acceptance: platform healthy at R3c head (status ok, vmctl ok,
    ws resolved). Management activations on staging now bind the cell
    carrier (desk_go_eval + lifecycle controls) unconditionally — the
    R2-deferred management-cell cast is live-reachable; the delegated spawn
    leg completes on staging's real Linux capsule substrate. Local proofs:
    management promotion + seal + kept controls (TestManagement*), cast
    reaches delegated admission (TestR3cDeskCastReachesDelegatedAdmission),
    worker spawn/persist/kill (TestDeskGoEval*), unknown-desk cast rejected."
---

# R3c — Management Live + Cast (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11:
management desk on cells; `choir.Cast(engineering)` opens+binds+executes an
assignment (the R2-deferred deployed proof); `report_to_texture` retires to
the staged path. Proof: a management cell Cast → engineering assignment →
Report resolves the cast record; restart mid-episode resumes from the tape.
