---
definition_version: 4
definition_id: choir-jev-supervision-metamission-2026-09-29
execution_mode: mission_orchestrator
readiness: executable

# review binds THIS file at its fixed commit (stamped at promotion).
# executable: now.status=working + named authority (owner-ratified spine
# 2026-09-29 + authoring-panel accept). Stations promote per their own
# readiness as dependencies clear.
review:
  reviewer: 'agentic-consensus authoring panel (codex, claude, devin,
    gpt6-sol, gemini38) — send_back round resolved'
  frozen_ref: 'main@63be04e3'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20260929-215420/'

metamission:
  stations:
    - id: m0-debug-stabilize
      path: docs/definitions/choir-signal-m0-debug-stabilize-2026-09-29.md
      readiness: drafted
      status: complete
      depends_on: []
    - id: m-sub-signal-plane
      path: docs/definitions/choir-signal-async-signal-plane-2026-09-29.md
      readiness: drafted
      status: working
      depends_on: [m0-debug-stabilize]
    - id: m0a-research-rlm-cutover
      path: docs/definitions/choir-signal-research-rlm-cutover-2026-09-29.md
      readiness: drafted
      status: working
      depends_on: [m-sub-signal-plane]
    - id: m1-typed-commitments
      path: docs/definitions/choir-signal-typed-commitments-2026-09-29.md
      readiness: drafted
      status: working
      depends_on: []
    - id: m2-model-policy-rlm-module
      path: docs/definitions/choir-signal-model-policy-rlm-module-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m0a-research-rlm-cutover, m1-typed-commitments]
    - id: m3-research-hillclimb
      path: docs/definitions/choir-signal-research-hillclimb-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m2-model-policy-rlm-module]
    - id: m4-jev-transport
      path: docs/definitions/choir-signal-jev-transport-2026-09-29.md
      readiness: drafted
      status: complete
      depends_on: []
    - id: m5-management-scorer
      path: docs/definitions/choir-signal-management-scorer-jev-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m1-typed-commitments, m4-jev-transport]
    # world-wire is the NEXT stack — not a station of this metamission.
    # Keeping it out of metamission.stations is load-bearing: a spine
    # settles complete only when every station is complete or superseded.

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: 'staging https://choir.news build.commit=b85af274
      (node B health, 2026-09-29)'
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
    - path: docs/desk-rlm-rectification-plan-2026-09-23.md
      status: dirty
      class: other_agent_wip
      owner: other agent
      touch: forbidden
      recovery: 'leave in place; never include in station commits'
  predecessor:
    mission: choir-rectification-spine-2026-09-25 — settled 2026-09-29 on
      M11's landed receipt (probe run 9 satisfied, staging e886f576)
    disposition: satisfied — the substrate the prior spine built is the
      foundation this metamission extends (desks on the in-cell carrier,
      commitment ledger, typed acts, one-tool doctrine on 3/4 desks)
  observed_artifact:
    - 'QA repro on owner computer stalled: prompt -> v1 -> research leg
      123,490 input tokens ~470s -> "remains open", no v2 committed'
    - 'owner computer runs b85af274, one build behind wake-repair commits;
      desk_pending_mutations=7; runs b05f42a6/43ad448a/e98f8f3f pending
      since 9/28'
    - 'ReduceCellIntents(failed) persists nothing — a cell that fails
      silently leaves the reducer waiting (the stall terminator gap)'
    - 'rlm_inbox_cursor is run-memory scoped to runID — respawn replays
      the channel from zero'
    - 'injectUserTurns seam (toolloop.go:672) exists between model calls;
      boundary-drain injection lands there'

finish:
  deliver: 'The four desks run on the full-RLM doctrine with a durable
    async signal plane between them; commitments are typed and scoreable;
    management IS the commitment scorer (Jev default, RLM escalation);
    research iterates fast-and-deep inside Go cells instead of a serial
    tool-call loop; the owner-visible stall class is closed end-to-end.'
  artifact: 'All eight station files settled complete or superseded with
    deployed acceptance receipts on the tape; spine now.slice advanced
    through each station.'
  acceptance:
    - action: 'read each station''s goal file now.status and fetch + verify
        its deployed acceptance receipt and environment identity (not
        merely a reported status)'
      proves: 'every station delivered its promised artifact with verified
        staging evidence'
      evidence_class: deployed proof
    - action: 'owner prompt-bar QA repro runs on the final build:
        research evidence reaches texture mid-run, texture revises the
        doc, management scores the resulting commitments'
      proves: 'the supervision loop closes end-to-end on the product
        surface'
      evidence_class: deployed proof
  rollback: 'per-station rollback paths apply station-by-station; the
    spine itself is reverted by returning docs/ACTIVE.md +
    mission-graph.yaml to the prior entrypoint and marking the spine
    superseded'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize the distance between the owner''s question and
    a supervised, evidence-backed answer — while preserving the epistemic
    boundary that keeps scores out of ActingPack and the tray atomicity
    that keeps writes all-or-nothing'
  goodharting_would_be: 'stations land structurally but the QA repro
    still stalls, or scores exist but nothing routes on them —
    supervision theater'

homotopy:
  realism_axis: 'delivery immediacy + scoring depth — from sequential
    blocking wake to durable async signal to scored commitments driving
    routing; each station is a continuous deepening, never a different
    object'

boundaries:
  mutation_class: red
  # red: this spine''s stations touch canonical event authority, kernel
  # broker surface, desk tool surfaces, model/provider routing, and
  # credential provisioning. Full ceremony applies per station.
  authority_sources:
    - 'owner corrections + direction 2026-09-29 (scorer=management,
      conductor=policy routing, model=RLM module, notice-injection,
      one-tool doctrine)'
    - 'orientation doc adjudicated design (two consensus panels,
      .agentic-consensus/agentic-consensus-20260928-* and 20260929-*)'
    - 'docs/choir-doctrine.md + AGENTS.md mutation classes'
  must_preserve:
    - 'tray atomicity — semantic acts commit all-or-nothing at cell end'
    - 'epistemic boundary — scores/distributions/disagreement never enter
      ActingPack'
    - 'channelID:seq wake dedup'
    - 'actors park; activations are not retained for delivery'
    - 'single state authority — commitments on the tape, not parallel
      stores'
  excluded:
    - 'World Wire fanout substrate (successor stack)'
    - 'conductor model routing (conductor is policy routing only)'
    - 'Box score / production wire (out of scope per owner 9/22)'
  protected_surfaces:
    - 'internal/yaegikernel broker + session loop'
    - 'internal/agentcore rlm_reduce + channel_store + run-memory'
    - 'internal/toolloop injectUserTurns seam'
    - 'gateway provider route surface + credential provisioning'
    - 'desk cell registries + prompt overlays'

now:
  status: working
  slice: 'M0 + M4 settled complete. M-SUB residuals landed (emit boundary-
    drain, cursor rekey, advisory piggyback); M0a phase-1 verbs landed +
    cell-proven (b83ea5db, a09fd215, 007b64df, 9e3d6948 — research choir.*
    egress verbs + handle auth + research messaging-authority repair). Live
    front: deployed acceptance on all three — M-SUB Emit proofs, M1
    typed-commitment tape, M0a controlled-comparison verify before the
    deletion commit.'
  source_ref: main@9e3d6948
  deploy_identity: 'staging deployed_commit=2404e7d2 (vmctl flapping;
    007b64df/9e3d6948 deploys propagating behind a wedged Node B job)'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: signal-plane-closes-supervision
    claim: 'If the signal plane + typed commitments + management-scorer
      land in this order, the desk-to-desk supervision path the QA stall
      exposed closes end-to-end on staging'
    test: 'the finish acceptance: QA repro delivers evidence mid-run,
      texture revises, management scores'
    edge: frame_lock
    # frame_lock risk: closing THIS QA path could still leave the general
    # supervision claim unproven; scope_if_supported stays bounded to the
    # exercised path until further proof
    delta_o: 'post-M5 QA run on staging + commitment_score records'
    scope_if_supported: 'the desk-to-desk path exercised on staging —
      texture<-research emission + management scoring; wider claims need
      further stations'
    status: testing
    evidence_refs: []
  decision:
    what: 'ordered stations M0->M-SUB->M0a->M2->M3; M1+M4 parallel-safe;
      M5 after M1+M4; spine settles when all eight settle'
    kind: architecture
    status: settled
    evidence_ref: '.agentic-consensus/agentic-consensus-20260929-212240/
      (convergent verdicts)'
    owner_ratification_ref: 'conversation 2026-09-29: owner corrections
      ratified (scorer=management; conductor=policy; model=RLM module;
      notice-injection)'
  belief:
    believed_state: 'the QA stall is the desk-consumption hole
      (ReduceCellIntents(failed) persists nothing) plus research running a
      serial tool loop; both are substrate problems, not tuning problems'
    main_uncertainty: 'whether the 7 pending mutations + 3 stale runs on
      the owner computer are the same wedge class or a second defect'
    next_observation: 'first M1 desk-cell commitment records + M4 live
      round-trip distribution on the staging tape; first M-SUB cell_fate
      record on a timed-out/killed cell'
  blocker_or_risk: 'staging env root-caused on Node B 2026-09-30 to a HOST
    CAPACITY fault, not code: platform-dolt (go-choir-platform-dolt) balloons
    to anon-rss 28GB (total-vm 48GB) on a 31GB box -> kernel OOM-killer kills
    it AND the retained-computer firecracker
    (vmmanager: "exited with error: signal: killed", 12:31:26). Each kill
    reboots the guest (~20-40min cycle) -> boot passivation mass-parks
    in-flight desk cells -> desk legs (incl. our QA research legs) die
    mid-leg -> v2 cannot form -> all deployed_proof items are unreachable.
    The stale-route/vmctl-unavailable flapping is a SYMPTOM of these re-mints;
    POST /internal/vmctl/refresh on the vmctl unix socket recovers the route
    non-destructively between reboots; build.commit=9e3d6948 is deployed and
    the guest itself is READY/working when up. Also REPAIRED
    this session: a second independent defect — texture audit events were
    hardcoded EventKind=lifecycle_observed (texture_audit.go), which lands
    them in the computer_lifecycle_receipts CAS join that a non-lifecycle
    action can never satisfy -> every texture audit append aborted
    "lifecycle receipt join unavailable"; the tape silently dropped the
    whole texture audit trail since ~Aug 20. Fix 0a03783b maps
    entry.Action->causal kind. THIRD fault surfaced 13:05 (the prompt-bar
    submit 500): "computer event projection repair required" — an OOM-kill
    mid-append committed the platform CAS but killed the embedded finalize,
    so platformHead != embeddedHead and the appender refuses all appends.
    UPDATE 13:20Z: a `choir computer refresh` rebooted the guest and the
    autoputer ran the replay phase — "projection recovery resume for
    computer-03335285 (local=174099 H=174100 tail=1)" then "computer event
    authority reconstructed". THE PROJECTION WEDGE IS REPAIRED. Remaining
    fault is the OOM itself + the stale vmctl route it re-mints (502
    "resolve user autoputer"); audit fix 0a03783b is merged but its deploy
    was concurrency-cancelled twice — build still 9e3d6948.

    ROOT-CAUSE CLUSTER (three live-locks, one day, one class): the actor
    dispatcher''s `defer` is the silent default — any reconcile error that
    isn''t explicitly typed as durable-invalid wraps `ErrDeferUnprocessed`,
    which rolls back the attempt count, so a *decided-invalid* condition
    bypasses MaxAttempts/poison and redelivers forever, saturating the
    guest CPU and starving every other desk''s reconcile:
      (1) exhausted restart recasts transient-error live-lock — 886e5ce1;
      (2) stored-grant-attestation ErrEngineeringAssignmentInvalid wrapped
          transient — 382 defers/3min on texture:f939b0f9, fix b7f59cc9
          (map Invalid -> invalid authority -> terminalize), deployed;
      (3) reactivated persistent-Management resident `b05f42a6` — carried
          actor_reactivated_from_passivated so the fresh-mint watchdog
          skipped it, and the reactivation watchdog is a one-shot timer
          lost on restart; resident stayed pending, slot occupied, every
          later Management wake deferred — fix 3b0a1ed2 (call
          failExpiredReactivatedManagementResume on the live-wake
          resident gate).
    Per AGENTS.md Root-Cause-Clustering the next move is substrate-level,
    not another per-symptom patch. A 5-panelist convergent consensus
    (.agentic-consensus/live-lock-20260930/) unanimously returned D —
    change the dispatcher contract, not just the error vocabulary: make
    defer explicit + bounded, let unclassified errors fall to
    poison/terminalize, and introduce a durable-invalid category the
    occurrence records its fate and releases the slot.
    LANDED (pending deploy): ErrDurableInvalid consumes the occurrence and
    emits a delivery_invalid fate to the error sink; defer_count column +
    MaxDeferrals=64 bound ErrDeferUnprocessed so a wait that outlives the
    bound poisons instead of live-locking; texture pending-postcondition
    classified by run state (terminal run -> durable-invalid, parked run ->
    actorruntime/{adapter,handler}. Tests prove the bound + the consume.

    18:11Z — cc1b5af4 deployed; guest refresh forced onto it bricked the
    retained computer: unclean unmount left data.img (vdb) corrupt ->
    fsck /dev/vdb failed -> /mnt/persistent failed -> emergency mode, and
    the in-flight vmctl refresh goroutine hung holding refreshing[key]
    (blocks /recover). Recovery: kill the emergency-mode firecracker, copy
    + e2fsck -y the data.img copy (journal recovered, free counts fixed),
    swap repaired image in, restart vmctl to clear the stuck flag, refresh.
    Guest came up clean on cc1b5af4 at 19:18Z — runtime: started, boot
    passivation sweep ran, ZERO defer/poison/durable-invalid flood
    post-boot (all four live-lock signatures dead). Wedged
    persistent-Management resident f2e0446f passivated at 19:16Z and the
    slot freed — management no longer defer-storms. research:cfa90b87 run
    362febb2 is passivated awaiting its cell wake. engineering desk
    reconcile logs a non-repeating ''co-super assignment invalid
    transition'' — a fourth invalid-wrap surfaced, needs a typed
    ErrDurableInvalid mapping.

    SIXTH DEFECT (delivered-not-consumed wake strand): cfa90b87 M0a cell
    armed as control 4158e48b (texture:ce3e0e77 -> research:cfa90b87,
    disposition=pending, web-search question) with delivered_to_loop_id=
    362febb2 delivered_at 17:12:39 — the run consumed nothing (store
    append failed 17:37), then restart-passivated. The wake-mint guard in
    actorWakeOutboxFromWorkerUpdate (lifecycle.go:553-557) skips any
    update with DeliveredToRunID set, so MigrateActorWakeOutbox never
    re-mints its wake and sweepActorWakeOutbox has nothing to project:
    a crash AFTER delivery-binding but BEFORE consumption strands the
    pending control with no wake path. FIXED a2b87d69: suppress only when
    the bound run is still Active; passivated/terminal/unresolvable bound
    run re-mints the wake (actor_wake_strand_test.go proves both legs).
    Awaiting deploy; cfa90b87 unstrands on the next guest boot migration.
    DEPLOY-OUTCOME UPDATE (08a76896): a7e31232's migration minted 3 pending
    wakes at 21:14Z yet cfa90b87 stayed passivated — a divergent panel plus
    code audit proved the re-arm replayed the consumed tape row
    (actorDispatchUpdateID is content-derived; ON CONFLICT(update_id) DO
    NOTHING swallows the resend). The sweep and the canonical re-enqueue
    now dispatch through redrive hooks that mint <base>#redrive-N when the
    identity family is fully consumed; pending members suppress salting so
    replay dedup is unchanged. cfa90b87 unstrands on the 08a76896 boot.
    DEPLOY-OUTCOME UPDATE (37882e1d, deployed 22:39Z, guest epoch 979):
    wakeUpdatedCoagent (the live commit trigger) also routed through the
    redrive hook — all three re-drive authorities (outbox sweep, canonical
    re-enqueue, persistent-Management recovery, live commit trigger) now
    salt on a consumed family. Post-boot evidence: obligation CONFIRMED
    still open (work item 7be3d1de open, bound run 362febb2 passivated,
    predicate actorWakeOutboxFromWorkerUpdate yields the wake). The salted
    dispatch is queued — the serial outbox drain is working a 128+ dead-wake
    backlog (discharged obligations re-armed by the same migration; each
    disposes on ErrNoPendingActorOccurrence). Separate environment fault:
    provider chatgpt circuit open (upstream unhealthy) — research cell
    calls fail on the provider until upstream recovers; not the seam.
  '
  '
  next_action: 'STATUS 23:00Z — redrive chain (a2b87d69 + a7e31232 + 08a76896
    + 766b3a53 + 37882e1d) DEPLOYED to staging, guest epoch 979 live on
    37882e1d. cfa90b87 obligation confirmed open (work item 7be3d1de,
    bound run 362febb2 passivated). Salted dispatch is queued behind a
    279+ dead-wake serial drain (discharged obligations re-armed by the
    migration, each disposing on ErrNoPendingActorOccurrence). BLOCKED:
    unstrand confirmation pending drain + provider chatgpt circuit-open
    recovery. M-SUB/M1/M0a deployed proofs all share this gate.

    and fixed: exhausted restart recasts live-locked the guest. "recast
    attempts exhausted" returned a transient error, so the actor
    dispatcher re-delivered the reconcile occurrence forever (every ~2-4s
    x2 dead assignments), saturating CPU and starving every other desk
    reconcile — why the M1 texture work item never got a slot and
    texture create/revise timed out. The exhausted branch already fails
    the bound selfdev operation durably, so it now returns clean
    (occurrence incorporated, loop stops). Fix 886e5ce1 (impl+verify
    branches), agentcore+actor+textureowner green. DEPLOYED 14:36Z as
    part of 14faf3d5 — guest rebooted onto it, recast spam went 83/90s
    -> 0/30s, runtime: started clean.

    M1 residual finding (14:38Z): with env now stable (audit fix live +
    projection repaired + live-lock dead + vmctl ok + no OOM kill in the
    window), a fresh trajectory 5fc5b712 bound a texture work item
    (doc df78cc76) that STILL sits open/unexecuted — the prompt-bar
    texture work item is not a self-driving RLM cell; the
    precommit/resolve/disagreement path needs the document-channel
    sub-RLM cast to an agent desk — the missions own deliverable,
    not yet self-driving end-to-end.

    15:35Z — the "dual desk-target residue" diagnosis was WRONG: the second
    open texture:f939b0f9 item is the BY-DESIGN supervision surface minted
    by 19e2c256 beside every document cast, not corrupt residue. The real
    defect was four sites built on ''at most one desk agent per document'':
    the revise guard counted supervision as a desk target (409), the
    wake resolver probed texture: first (cast wakes went to the
    supervision agent, never the engineering occurrence consumer),
    lifecycleDocDeskProfile was order-nondeterministic, and
    reconcileAgentWakeLocked would have armed apply_owner_revision cells
    on the supervision agent. Fixed in c58ed60a (receipt
    docs/problems/owner-revision-supervision-desk-target-409-2026-09-30.md),
    deployed to staging, computer refreshed to epoch 972: texture revise
    409 -> 202 (revision 87baf761), artifact_head_advanced ->
    co_super_assignment_opened -> co_super_assignment_bound -> run
    run:assignment-dee9215f-2e4b-5b05-bbb3-55cf0f540c35 executing the M1
    commitment-cell prompt. The document-channel cast is proven live.

    17:10Z — M1 commitment cell COMPLETED on the deployed build. Revision
    43e3b14b on doc f939b0f9 (prompt names the live choir verbs so the cell
    calls them, not greps its stale Sep-4 source snapshot — the deployed
    capsule-broker binary exports Precommit/Resolve/Disagreement) fired
    run:assignment-062f03d9-0e02-5ccf-b1c8-7933561387de to terminal
    `completed`, report:sha256:b64c933f. The cell staged choir.Precommit
    over "a"+"b"=="ab" (outcomes {match,other}), observed, resolved it via
    choir.Resolve, correctly skipped choir.Disagreement (no contradiction),
    and completed — one honest predict->observe->resolve cycle. The reduce
    appends each act via AppendCommitmentRecord to ogKindCommitmentRecord
    objects on the same commit; a completed reduce means the records
    landed on the guest OG ledger. Gate (b) is CLOSED.

    Prior-cell forensics (17:10Z): the earlier blocked reports blamed a
    missing choir.Precommit symbol — FALSE. The capsule SOURCE TREE is a
    frozen Sep-4 snapshot (7574d899, predates typed commitments
    5205cb5f/7dda0a26/2f6f490c); the cell read intent.go from it and saw
    only {message,spawn,complete}. The deployed WORKER BINARY exports all
    three verbs (verified on /opt/go-choir/capsule-broker). Residual lesson:
    cells must be told the live surface, not trusted to grep a stale
    frozen source snapshot.

    Remaining gate: (a) platform-dolt OOM capacity — 17:15Z measured
    corpus :13307 at 9.1GB RSS, :13306 at 1.5GB, 14GB free on 31GB
    (headroom now, but the corpus store grows unboundedly and re-mints
    the VM every ~20-40min when it crosses ~28GB); the refresh-cycle
    workaround is exercised but a memory cap is the durable fix —
    owner/ops, not code. Gate (b) is CLOSED by the 17:10Z
    completion above.

    19:20Z — dispatcher substrate fix DEPLOYED and live (cc1b5af4):
    ErrDurableInvalid consumes decided-invalid occurrences + records
    delivery_invalid fate; defer_count/MaxDeferrals=64 bound every defer so
    a wait that outlives the bound poisons instead of live-locking. All four
    live-lock signatures dead (0 defer/poison/invalid post-boot); wedged
    residents f2e0446f (Management) and 362febb2 (research:cfa90b87)
    passivated, slots freed. The forced refresh bricked the guest (unclean
    vdb unmount -> fsck fail -> emergency mode + hung vmctl refreshing
    flag): recovered via copy+e2fsck repair + vmctl restart. NEXT: re-drive
    the armed research cell (cfa90b87 M0a verify) and run the M-SUB/M1
    deployed proofs on a now-stable dispatcher.'
  receipts:
  - id: m1-commit-proof-attempt-2026-09-30
    boundary: execute
    commit_or_artifact: 'trajectories b9f7f256-07ad + 0c228ea4-6fd5 +
      5fc5b712-825d on computer-03335285; engineering-bound f939b0f9 cast
      fired but all attempts restart-cancelled; texture revise 409s on
      dual desk-target residue.'
    proof_refs: [prompt-bar run start accepted 2026-09-30T13:32Z+13:42Z;
      traj 84868c2d seq30 open engineering+texture work items]
    cannot_prove: 'a desk cell staging precommit/resolve/disagreement
      intent -> choir.commitment_record objects on the OG ledger —
      CLOSED 17:10Z: run:assignment-062f03d9 completed on staging,
      cell staged Precommit+Resolve and completed; the reduce lands
      each act on ogKindCommitmentRecord objects.'
  - id: m1-commit-proof-landed-2026-09-30
    boundary: execute
    commit_or_artifact: 'deployed staging cell run:assignment-062f03d9
      on computer-03335285 -> completed, report:sha256:b64c933f; cell
      staged choir.Precommit + choir.Resolve and completed.'
    proof_refs: [run status completed 2026-09-30T17:10Z; trajectory
      84868c2d co_super_assignment_reported event]
    cannot_prove: 'direct guest-OG ledger read of the commitment_record
      objects — the guest dolt is inside the Firecracker VM, not
      externally queryable; existence is inferred from the completed
      reduce (a failed AppendCommitmentRecord would have failed it).'

receipts: []

weak_measures:
    kind: weak_signal
    baseline: '38e094fc (post-M0-repair; staging tip)'
    desired: 'tracks staging tip within one deploy cycle'
    decision_use: 'a computer more than one build behind can mask repaired
      substrate defects — treat its symptoms as suspect'
    cannot_prove: 'that the stall is a build artifact vs a real defect'
---
