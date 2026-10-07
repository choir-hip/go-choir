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
    v5.1 (director 2026-10-06): SR has landed. SMG CLOSED 2026-10-06
    (spine receipt smg-to-sa-transition-2026-10-06): legs 1-3 passed on
    disposable computer-0ca7656f at build 475902d7 via the deterministic
    POST /api/texture/management-open acceptance surface
    (docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json). Named
    edge into this station: re-run legs on the owner computer after
    slice 1 — including the two weak legs recorded below (cell-level
    unbound refusal; work_disposition=completed settlement).
    Order changed: slice 0 registration genesis (fresh computers accept
    first prompt without manual bootstrap-chain; every SA acceptance runs
    on disposables), slice 1 Management storm convergence (moved up from
    slice 4: it starves every activation on the owner computer and would
    make any baseline a measurement of the storm), then the baseline and
    the remaining slices.
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
    OPENING DEFECT FIELD (slice 1 starts here):
    (NEW 2026-10-07 — owner VM boot loop, blocks drain carrier):
    docs/problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md
    — projection base watermark never re-advertised; recovery tail
    423720 > MaxRecoveryTailEvents 10000; VM cannot boot until a fresh
    base is published via choir-rebuild-base --source http (DiskEventSource
    additionally broken for effect_accepted events — TargetStateCommitment
    fabricated from ResultingEffectiveCommitment which is empty).
    (a) sa-delegated-report-poisons-management-listing-2026-10-06.md
    (red, protected surface): a delegated-cast producer report fails the
    ListLifecycleControlsDeliveredToRunPage ProducerReport validation arm
    (~:794, likely producerWork.TrajectoryID) — the listing throws on the
    whole page rather than quarantining one packet, so the first
    delegated report kills every subsequent management activation in a
    ~30s bind→fail→re-mint burn loop (4 failures observed on
    computer-0ca7656f, +34/+34/+28/+35s; loop stalls only when the
    delivered report discharges the work item). d94ce9ef's quarantine
    covers pending reconcile paths, NOT this delivered-page listing —
    extend the invariant or validate delegated lineage explicitly.
    FIXED ca8c8c18 (landed 2026-10-07): delivered-page ProducerReport
    arm fetches producer run+work first, then accepts the consumer-side
    binding OR the delegated work-item lineage join
    (parent_control_id/parent_work_item_id/parent_loop_id).
    TestDelegatedCastReportDoesNotPoisonConsumerDeliveredListing
    fails-before/passes-after; full ./internal/store suite green.
    DEPLOYED ACCEPTANCE RESIDUAL: ca8c8c18 pushed 2/2 "healthy" but the
    owner canary's apply failed with updater health-probe 503 and
    rolled back — the push gate is not commit-bound
    (docs/problems/app-layer-push-health-gate-not-commit-bound-2026-10-07.md,
    deterministic on retry). Behavioral proof on the owner computer is
    blocked by that platform-update fence defect; vm-48bc0981 runs the
    fix but has no API key for driving a delegated cast.
    Regression gate: one delegated-cast producer report → subsequent
    persistent-Management activations survive.
    (b) sa-management-mint-no-start-slot-deadlock-2026-10-06.md — fixed
    01199fb1 (zero-bound stranded fresh-mint runs fail-release the slot);
    deployed-verified on disposable 0ca7656f (run bound+drove legs).
    (c) sa2-appendevent-unbounded-scan-2026-10-06.md — fixed 475902d7
    (AppendEvent seeded counter replaces unbounded objectgraph scan);
    deployed on owner computer (verified 475902d7 via
    /api/runtime/observability).
    (d) sa1-wake-outbox-rearm-storm-2026-10-06.md — the boot re-arm storm
    record; d94ce9ef candidate addresses.
    WEAK SMG LEGS CARRIED FORWARD: (1) unbound_refusal_evidence — probe
    matcher is substring ('unbound'|'requires'|…) over event blobs and
    the objective text itself contains those strings; no packet-body API
    exists to tighten it; re-run on owner must assert refusal via the
    management run's own report content. (2) work_disposition=completed
    settlement — the delivered report is durable but no surviving
    management run incorporated it on the disposable (burn loop); a
    post-fix owner run must show the work item settle.
    SECOND DEFECT 2026-10-06 (mint-no-start): fresh disposable
    computer-03335285 minted persistent Management run 462d30ea at 17:24Z;
    initial_dispatch never delivered; run stranded pending holding the
    slot; every later wake deferred. Fix 01199fb1 deployed-verified on
    disposable 0ca7656f (recorded at (b) above); the earlier
    deploy-freshness residual is resolved — 475902d7 ships 01199fb1 and
    the disposable confirmed the bound-run path.
    DISPOSABLE-RELEASE RESIDUAL (S2 line, not slice 1's fix): a fresh
    disposable resolves the shared storedisk.erofs at boot time, so a VM
    minted before a deploy but booted after it can run a stale runtime
    (observed 2026-10-06: disposable served build 4dff031c while main
    carried 475902d7; probe --expect-commit gate refused and caught it).
    Not a pin defect — a deploy-race on a shared store image; an S2
    per-mint release pin or boot-time freshness assertion would close it.
    APPLY-FENCE RESIDUAL RESOLVED (a)'s follow-on: the owner canary's
    apply of ca8c8c18 rolled back on updater health-probe 503 during the
    546k-event replay — fixed 19d7913e (liveness-tolerant HTTPHealthProber
    + MaxDuration=15m) and landed on the owner via the guest-image deploy
    + active-VM refresh. The commit-bound push gate is fixed 68397ae7.
    Slice-1 blocker lifted: the owner can now receive app-layer releases.
  source_ref: main@29817fda
  deploy_identity: 'choir.news deployed_commit=5eb63161 (run 37560283352; guest runtime unchanged — gateway is host-side); pending 29817fda (guest gatewayruntime decode-EOF retry) through CI + deploy + owner refresh'
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
    Slice 1 (storm convergence; red) — mechanism landed: stale-packet
    terminal fate `41822a4c`, wake-mint marker `d94ce9ef`, occurrence
    resolve O(1) `e3e96067`, paced drain `d94ce9ef`. Deployed observation
    2026-10-07 on `19d7913e` (owner `computer-03335285`): wake mint
    ceased 01:20:27 — the stale supply drained; the storm is now a
    bounded 14-run delivered backlog, not a perpetual loop. Carrier-kill
    residual fixed `29817fda` (guest-side `gatewayruntime.Provider.call`
    retries decode-EOF/read/5xx 3x; host-side companion `5eb63161`).
    Drain carriers `dd52c39d` → `ed435162` (killed i=94 transient,
    pre-fix) → `6f2119ea` (refresh-killed) → `ceff999b` (vmctl
    reattach-kill — `update_coagent` carriers were invisible to rewarm;
    fixed `91e9c03b`) → `5f311268` (wake-minted post-03:15 boot) →
    `1a9ca12e` (04:03 rewarm-minted on `[696]` image post-03:47 NixOS
    switch) → `3aad068e` (05:05). `delivered-pending-runs` stuck at 14
    across `1a9ca12e`'s ~1hr run — carrier consumes but bound packets'
    disposition never leaves `pending`. **Residual resolved
    2026-10-07**: drain carriers cannot report — the
    `QueueLifecycleUpdate` producer-run check demanded a
    single-trajectory binding (`assignment_trajectory_id` +
    `lifecycle_control_bindings`) that backlog-minted
    `update_coagent` carriers never carry, and the runtime
    `persistentManagementBoundReport` required `trajectory_id` from the
    run record (mint-defaulted to runID). Fixed `7a36713c`: the store
    validator relaxes for `update_coagent` carriers
    (`control.DeliveredToRunID == req.SourceRunID` is authority); the
    runtime resolves the report trajectory from the delivered set and
    scopes consumption to it. Queued `management-open` control defers
    FIFO behind drain (correct). Remaining acceptance is measurement:
    drain settles at a bounded floor in a stated window, holds 24h,
    then SMG probe passes on the owner (legs 1-3 incl. the two weak
    legs). Deploy gate: `29817fda` + `91e9c03b` + `7a36713c` through
    CI + deploy + owner app-layer push (push currently refused on
    head-churn fence while drain runs — land post-drain). -> slice 2
    (baseline + offline GC re-measure) -> 3 (memory) -> 4 (race test,
    then read/write lock) -> 5 (read cost) -> 6 (store split + group
    commit) -> 7 (declared shapes + elasticity) -> acceptance.
receipts: []
---

# SA — Agent Density in a 2-4 GiB Guest

Owner principle: scale agents horizontally inside a small VM before scaling
VMs. A desk is many heterogeneous activations; this station makes the guest
able to host many of them at once. That requires fixing the store they all
share, not raising the memory ceiling again. The slices and their rationale
are in the metamission's "v5 plan" -> SA.
