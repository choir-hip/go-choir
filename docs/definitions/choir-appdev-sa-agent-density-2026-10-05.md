---
definition_version: 4
definition_id: choir-appdev-sa-agent-density-2026-10-05
execution_mode: mission_orchestrator
member_of: choir-supervised-app-development-metamission-2026-10-01
readiness: drafted
review:
  reviewer: none (owner approved the v5 outline 2026-10-05; file-level panel pending)
  frozen_ref: none
  verdict: none
  evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md ("v5 plan" -> SA)

start:
  captured_at: '2026-10-05T00:00:00Z'
  source:
    canonical_ref: main@9e0fe4d
    deploy_identity: unknown (verify /health before slice 1)
  worktrees:
    - path: repo root (orchestrator worktree)
      status: unknown
      class: unknown
      owner: orchestrator
      touch: read_only
      recovery: classify before any edit
  observed_baseline:
    - >-
      engineMu (internal/objectgraph/dolt_store.go:62-66) serializes every
      query on the guest's embedded Dolt because two pools on one engine
      "race on shared internal buffers (unescapeHTMLCodepoints mutating a JSON
      slice in place)". Latency mission attribution cf0ef207: ~97% occupancy,
      read-dominated, top holders unindexed scans; cut 3 indexes deployed,
      load re-measurement pending.
    - >-
      One embedded Dolt holds versioned content (Texture revisions, og
      effective state, event index) and high-churn operational rows (runs,
      events, channel messages, inbox deliveries, work items, wake outbox,
      pending mutations); the replay manifest already classes most of the
      latter empty_until_supported.
    - >-
      Owner guest 10-03: live store ~5 GiB vs ~22 GiB journal garbage; in-guest
      GC unreachable above 5 GiB live (internal/store/dolt_maintenance.go:290
      guard precedes the :309 journal trigger). No GOMEMLIMIT; no Dolt cache
      bound. Guest shape ratcheted 2 -> 16 GiB; 16 GiB is untracked drift in
      vmctl-priority.env.
    - >-
      The Management live-occurrence storm's live-locks are repaired; its
      convergence invariant (O(1) occurrence resolve + durable dispatch gate)
      is unbuilt (s0m-management-live-occurrence-storm-2026-10-03).

finish:
  deliver: >-
    Many desk activations run concurrently inside one 2-4 GiB guest. The
    owner computer runs at a 4 GiB ceiling, with an idle footprint under
    2 GiB, and fan-out does not starve commits.
  artifact: >-
    A declared density target with baseline and result receipts; memory
    fixes (GC ordering, automatic host offline GC, GOMEMLIMIT, bounded
    caches); a race-tested read/write engine lock; read-cost cuts incl. the
    storm convergence invariant; the operational/versioned store split
    (SQLite WAL for operational tables) with group commit; declared shapes
    and per-VM elasticity (cgroup, balloon, zswap, oversubscription ratio).
  acceptance:
    - action: >-
        Fan-out load probe on a disposable 4 GiB computer: N concurrent
        research activations + M scorer activations (N and M fixed in slice 1
        from the baseline) meet the target: p95 act-commit under 1 s, no
        activation starved past a stated bound, guest memory within the
        ceiling, idle under 2 GiB after the burst.
      proves: Horizontal agent scaling inside one small guest.
      evidence_class: deployed proof
    - action: >-
        The owner computer reboots at a 4 GiB ceiling from tracked config and
        serves normal use plus one fan-out burst; journal size stays bounded
        across a week of use.
      proves: The real long-lived computer fits, not only a fresh one.
      evidence_class: deployed proof
    - action: >-
        go test -race over concurrent reads on one embedded engine passes on
        the shipped Dolt version before the read/write lock lands.
      proves: Read concurrency is safe, not assumed.
      evidence_class: local test (correctness gate)
    - action: >-
        After the store split, replay completeness and pinned-head restore
        still pass on a disposable computer.
      proves: Authority and restore are unchanged by the engine change.
      evidence_class: deployed proof
  rollback: >-
    Each slice reverts independently. The store split ships behind a
    migration with a retained pre-split copy until the restore check passes.
    Memory ceilings are config.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize concurrent useful desk activations per GiB of guest memory while
    preserving the event tape as the single state authority and keeping
    restore and replay green.
  goodharting_would_be: >-
    A density number measured on an empty fresh computer; a read/write lock
    without the race test; raising the ceiling again; moving tables to SQLite
    without updating the replay manifest and restore set.

homotopy:
  realism_axis: >-
    Fresh disposable computer -> owner-sized store copy -> the owner
    computer itself; one activation -> N concurrent -> N plus a scorer
    fan-out.

boundaries:
  mutation_class: red
  authority_sources:
    - owner 2026-10-05 ("scale agents horizontally in a 2-4gb vm before scaling vms"; "shouldn't need 16gb at all")
    - docs/computer-ontology.md (ledger split, restore-set boundary)
  must_preserve:
    - The event tape is canonical; operational tables stay derived or recovery state.
    - Replay completeness and pinned-head restore keep passing.
    - No read/write engine lock without a passing race test.
  excluded:
    - Scaling across VMs or hosts (S3 and later; World Wire metamission).
    - New desk capabilities (SC).
  protected_surfaces:
    - guest persistent store layout and the replay manifest
    - VM shape and lifecycle config (vmctl)
    - desk dispatch and wake delivery

now:
  status: working
  slice: >-
    v5.1 (director 2026-10-06): SR has landed. Not started. Order changed:
    slice 0 registration genesis (fresh computers accept first prompt
    without manual bootstrap-chain; every SA acceptance runs on
    disposables), slice 1 Management storm convergence (moved up from
    slice 4: it starves every activation on the owner computer, blocks SMG
    behavioral legs, and would make any baseline a measurement of the
    storm), then the baseline and the remaining slices.
    SLICE 0 DEPLOYED 2026-10-06 (9f6f369c, CI 37465278828 green, Node B
    verified running 9f6f369c). Deployed acceptance: fresh registration
    (computer-07b582d5, no bootstrap-chain preamble) — genesis minted
    in-guest at boot (bootstrap-chain reports already_bootstrapped,
    head seq 57). First write accepted (202 on third submit).
    RESIDUAL EXPOSED: first ~2 prompt-bar submits during the first
    minutes after boot fail 500 `replace durable activation: lifecycle
    invalid transition` — a transient CAS race between the initial
    activation commit and the post-boot outbox/replay burst, not the
    pre-genesis class (trajectory + work item commit, run left
    passivated, then settles). Feeds slice 1 storm/race work.
    SLICE 1 CANDIDATE PUSHED 2026-10-06 (d94ce9ef): wake-outbox storm
    convergence — marker-gated one-shot migration, recorded-act expiry
    of undeliverable pending packets (ExpireStaleLifecyclePacket /
    update_expired), index-scan occurrence resolve, paced drain
    (4/desk, 64/tick), poison-packet quarantine, and the empty-trajectory
    directive bind fix. Deployed acceptance still owed: owner-guest
    restart with migration minted ~=0 and storm absent, then SMG legs.
    Separate defect found: fresh-disposable SMG probe failed on Texture
    runs exhausting the 1.2M token budget (evidence
    docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json) — not
    the wake storm; candidate for its own problem record.
    SECOND DEFECT 2026-10-06 (mint-no-start): fresh disposable
    computer-03335285 minted persistent Management run 462d30ea at 17:24Z;
    initial_dispatch never delivered (emission-buffer discard on
    ErrEpochConflict or deferred-activation drop, per dispatcher.go:382-449);
    run stranded pending holding the slot; every later wake deferred.
    Watchdog fired at deadline but zero bound packets → no-op (escalate
    packets bind nothing). Fix committed 01199fb1: zero-bound stranded
    fresh-mint runs fail-release the slot via terminalizeRunCanonical.
    Problem doc sa-management-mint-no-start-slot-deadlock-2026-10-06.md.
    DEPLOY-FRESHNESS RESIDUAL: guest app layer pins the runtime closure
    at mint; 01199fb1 not present on any deployed Node B runtime —
    observable only after app-layer update ships (S2/S4 gate) or a
    future disposable mints on a bumped flake pin.
  source_ref: main@d94ce9ef
  deploy_identity: 'choir.news deployed_commit=9f6f369c (d94ce9ef deploying)'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: one-store-three-symptoms
    claim: >-
      Lock contention, journal growth and the memory ratchet share one root
      cause: high-churn operational rows in a versioned chunk store,
      amplified by unindexed scans and polling. Fixing that cause, rather
      than raising ceilings, yields many concurrent activations in a 4 GiB
      guest.
    test: >-
      Slice 1 baseline vs post-GC measurement (does memory fall with the
      journal?), then the density acceptance after slices 2-5.
    edge: missing_oracle
    delta_o: The memory receipt and per-caller engineMu stats under fan-out load.
    scope_if_supported: Single-guest density on the current base image and Dolt version.
    status: proposed
    evidence_refs:
      - internal/objectgraph/dolt_store.go:62-66
      - internal/store/dolt_maintenance.go:290-310
      - docs/problems/guest-dolt-journal-and-host-image-leak-2026-10-03.md
      - docs/problems/guest-vm-embedded-dolt-memory-starved-2026-10-01.md
      - docs/problems/s0m-management-live-occurrence-storm-2026-10-03.md
      - internal/store/lifecycle.go:965 (MigrateActorWakeOutbox re-arms on every boot)
      - internal/agentcore/runtime.go:2588
  decision:
    what: Density before distribution; fix the store, not the ceiling.
    kind: architecture
    status: settled
    evidence_ref: owner 2026-10-05
    owner_ratification_ref: owner 2026-10-05 in the director session
  belief:
    believed_state: >-
      The guest is store-bound, not model-bound: model latency dominates an
      activation, but every context read and act commit queues on one engine
      lock.
    main_uncertainty: >-
      Whether the upstream Dolt race is fixed in a current release (cheap win)
      or needs a patch; and how much memory is journal garbage vs a real
      working set.
    next_observation: Slice 1 baseline numbers.
  blocker_or_risk: >-
    Slice 1 is the fourth attempt at the storm family; prior fixes each
    returned a terminal verdict for one trigger class. Land the invariant
    (every pending obligation binds, discharges, or scores a bounded
    terminal fate; nothing scans history to find it), not another
    per-trigger fix. The store split is the largest red change in the metamission. Sequence it
    after slices 2-4 show what remains, and only with the restore check in
    hand.
  next_action: >-
    Slice 0 (registration genesis; red, small) -> slice 1 (storm
    convergence: problem-doc the boot re-arm mechanism and confirm it from
    the guest trace; terminal fate for stale obligations as a recorded act;
    wake-outbox migration one-shot or deleted; O(1) occurrence resolve;
    per-desk dispatch gate + paced drain; acceptance = owner guest restart
    converges pending to a bounded floor within a stated window and holds
    24 h, and the SMG probe passes on the owner computer) -> slice 2
    (baseline + offline GC re-measure) -> 3 (memory) -> 4 (race test, then
    read/write lock) -> 5 (read cost) -> 6 (store split + group commit) ->
    7 (declared shapes + elasticity) -> acceptance.
receipts: []
---

# SA — Agent Density in a 2-4 GiB Guest

Owner principle: scale agents horizontally inside a small VM before scaling
VMs. A desk is many heterogeneous activations; this station makes the guest
able to host many of them at once. That requires fixing the store they all
share, not raising the memory ceiling again. The slices and their rationale
are in the metamission's "v5 plan" -> SA.
