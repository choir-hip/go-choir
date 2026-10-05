---
definition_version: 4
definition_id: choir-appdev-so-ops-substrate-2026-10-05
execution_mode: mission_orchestrator
member_of: choir-supervised-app-development-metamission-2026-10-01
readiness: reviewed
review:
  reviewer: >-
    Metamission v4 director review ("Orientation 2026-10-05") — SO scope
    authored there and ratified as a new parallel station gating S3.
  frozen_ref: 'main@c069eb42'
  verdict: accept
  evidence_ref: >-
    docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md
    ("SO — ops substrate (new parallel station; gates S3)")


start:
  captured_at: '2026-10-05T00:00:00Z'
  source:
    canonical_ref: main@e527c169
    deploy_identity: >-
      staging https://choir.news deployed_commit=3c1cbaf6; S2 contract slices
      e527c169 queued behind S2-e deploy
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed_baseline:
    - >-
      Five disk-headroom incidents since 2026-10-01; guest stderr unreachable
      after cold boot; VM shape drift lives in mutable vmctl-priority.env;
      CI gating strands runtime code behind docs-only heads
      (metamission v4 Orientation 2026-10-05, "SO — ops substrate").
    - >-
      `platform-artifacts` accumulates every layered nar (~146MB each) with
      no GC; `.corrupt`/`.pre-*`/quarantine dirs and dead vm-state dirs
      persist unbounded on /var/lib/go-choir.
    - >-
      Firecracker serial console isn't journaled on the host; the layered-apply
      journal marker is an observability gap, not evidence (S2 station
      `now.slice`, 2026-10-04).
    - >-
      `store.MaybeRunDoltGC` returns at the live-size guard
      (`internal/store/dolt_maintenance.go:290`) before the journal trigger
      (`:309`): once the guest's live store exceeds ~5 GiB the guest never
      GCs again, and the 10-03 measurement showed ~22 GiB of journal garbage
      (`guest-dolt-journal-and-host-image-leak`).
    - >-
      Guest memory grew 2 -> 4 -> 8 -> 16 GiB each time justified by "the
      store outgrew the guest"; owner direction 2026-10-05: "shouldn't need
      16gb at all" — target 2-4 GiB oversubscribed. The shape is untracked
      drift in `vmctl-priority.env` (VM_INTERACTIVE_MEM_MIB=16384).
    - >-
      Deploy need is computed against the push delta, not the deployed
      identity; a docs-only head has already stranded runtime code
      (`ci-docs-only-head-strands-runtime-deploy-2026-10-04`).

finish:
  deliver: >-
    The platform has a storage lifecycle, durable host-side guest
    observability, declared VM shapes, and CI deploy gating that reflects
    the deployed identity. Guest memory runs at a declared 2-4 GiB ceiling
    with elastic oversubscription, and the corpus-dolt host footprint is
    handed a number through the capacity mission.
  artifact: >-
    A Node B ops substrate: a retention/refcount GC over platform-artifacts
    keyed by live route slots + retained predecessors; a dead-dir reaper for
    vm-state/.corrupt/.pre-*/quarantine remnants; a per-VM rotated serial/
    journal sink wired through vmctl; declared guest shapes in tracked
    config (mutable priority env holds IDs only); a memory receipt
    (/proc/meminfo + Go metrics + Dolt sizes + host RSS per Firecracker
    process) that lands the 16->4 GiB question empirically; deploy-impact
    classified against deployed identity.
  acceptance:
    - action: >-
        Fill Node B /var/lib/go-choir past the prior disk-headroom tripwire
        with a real layered-release + snapshot workload; observe the
        retention GC reclaim dead dirs and stale artifacts before the
        deploy stalls, and the headroom budget project future snapshot
        files.
      proves: The platform can run layered releases + snapshots for a week
        of deploys without an ops fire.
      evidence_class: deployed proof
    - action: >-
        Trigger an exec/boot/apply failure on a disposable guest; read the
        guest's own serial console + journal from the host without any
        bespoke diag file, then confirm the layered-apply journal marker
        ("go-choir-autoputer: layering release") now lands in the host-side
        stream.
      proves: Guest observability is durable, not a special case.
      evidence_class: deployed proof
    - action: >-
        Record the memory receipt before and after a host-side offline GC
        of the owner store; observe whether guest memory falls with the
        journal, then land the declared 4 GiB ceiling on the owner computer
        if it fits, oversubscribed via MemoryHigh/MemoryMax + balloon.
      proves: The 16 GiB shape was paying for garbage, and the declared
        2-4 GiB ceiling is sustainable.
      evidence_class: deployed proof
    - action: >-
        Push a docs-only main commit followed by a runtime change in the
        same window; observe deploy-impact classify against the deployed
        identity (not the push delta) and the runtime change still deploy.
      proves: CI gating never strands runtime code behind a docs-only head.
      evidence_class: deployed proof
    - action: >-
        Remove VM_INTERACTIVE_MEM_MIB=16384 from mutable vmctl-priority.env
        and restart vmctl; observe guest shapes read from tracked config
        and no drift.
      proves: Declared shapes live in tracked config, not untracked drift.
      evidence_class: deployed proof
  rollback: >-
    Revert and redeploy the platform change. Storage lifecycle is
    additive/capacitative: retain the prior /var/lib/go-choir layout and
    restore mutable vmctl-priority.env from the deploy's -previous
    rollback ref; the guest observability sink is read-only and can be
    disabled without affecting the serving computer.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the number of unbounded growth surfaces, invisible guest
    failures, and untracked shape/gating drift on Node B while preserving
    the serving computer; a deploy should keep working for a week, not
    until the next disk or observability fire.
  goodharting_would_be: >-
    Deleting live platform-artifacts or retained predecessors under the
    label of GC, claiming observability from a one-off diag file, or
    lowering the guest ceiling without the memory receipt that proves it
    fits.

homotopy:
  realism_axis: >-
    Workload + data volume: one computer, one layered release, one snapshot
    -> the full tracking fleet + a week of deploys + hibernate churn, with
    the same GC/observability/shape guarantees holding at the larger scale.

boundaries:
  mutation_class: orange
  authority_sources:
    - metamission v4 director review ("Orientation 2026-10-05", SO scope)
    - owner direction 2026-10-05: "shouldn't need 16gb at all"
    - docs/computer-ontology.md (persistent user computer)
    - AGENTS.md (Landing Loop, Problem Documentation First)
  must_preserve:
    - Serving computers never lose their live route slot or retained
      predecessor releases to GC.
    - No guest-visible Nix store or daemon is introduced for observability;
      the sink is host-side.
    - Declared shapes are tracked config; mutable vmctl-priority.env holds
      IDs only.
    - Deploy-impact classification stays the sole deploy gate; docs-only
      pushes still never deploy.
  excluded:
    - S3 hibernate/snapshot mechanics (opens with its own snapshot
      measurement slice)
    - S2 layered release path (landed in parallel; SO only owns the
      substrate its artifacts land on)
    - S1 security floor remainder (parallel work)
    - corpus-dolt capacity sizing (handed a number through the capacity
      mission, not fixed here)
  protected_surfaces:
    - /var/lib/go-choir storage lifecycle + platform-artifacts
    - guest serial/journal sink (host-side)
    - guest shape declaration + vmctl admission
    - deploy-impact classification + deploy gating
heresy_delta:
  discovered: >-
    The guest GC ordering defect (live-size guard before journal trigger)
    was the real reason memory scaled with the store; 16 GiB was paying
    for ~22 GiB of journal garbage.
  introduced: >-
    None — SO only adds a lifecycle/observability/shape/gating substrate;
    it must not introduce a new release authority, a writable guest store,
    or a deploy gate outside deploy-impact-classify.
  repaired: pending — the five named substrate defects are repaired only
    when the deployed proofs above land.

now:
  status: working
  slice: >-
    Station file authored from the v4 Orientation 2026-10-05 SO scope;
    slices are ordered storage lifecycle -> guest observability -> declared
    shapes + memory -> CI deploy gating, matching the fire order
    (disk-headroom -> invisible guest failure -> untracked drift ->
    stranded deploy).
  source_ref: main@e527c169
  deploy_identity: >-
    staging https://choir.news deployed_commit=3c1cbaf6; S2 contract slices
    queued behind S2-e deploy (e527c169)
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: platform-ops-substrate
    claim: >-
      If the platform's unbounded growth surfaces (platform-artifacts, dead
      vm-state dirs, guest journals, untracked shapes) are given a declared
      lifecycle + durable host-side observability + tracked shapes, then
      layered release/snapshot workloads run for a week of deploys without
      ops fires, and guest memory drops to a declared 2-4 GiB ceiling.
    test: >-
      The five deployed proofs above: retention GC under real layered +
      snapshot workload, durable guest serial/journal on a failure, memory
      receipt before/after offline GC showing journal-driven memory,
      deploy-impact vs deployed identity on a docs-only head, and shapes
      read from tracked config after the mutable drift is removed.
    edge: missing_oracle
    delta_o: >-
      The memory receipt itself is the first-order delta: if it shows
      guest memory does not fall with the journal, the 2-4 GiB target is
      refuted and the capacity mission re-opens the shape question.
    scope_if_supported: >-
      Node B + the tracking fleet: storage lifecycle, guest observability,
      declared shapes, CI deploy gating.
    status: testing
    evidence_refs:
      - docs/problems/guest-dolt-journal-and-host-image-leak-2026-10-03.md
      - docs/problems/ci-docs-only-head-strands-runtime-deploy-2026-10-04.md
      - docs/problems/guest-stderr-unreachable-after-cold-boot-2026-10-04.md
      - internal/store/dolt_maintenance.go:290-320
      - internal/vmctl/ownership.go:1220-1224
  decision:
    what: >-
      SO is a new parallel station gating S3: storage lifecycle, durable
      guest observability, declared shapes + memory budget, and CI deploy
      gating. Slices fire in order of the evidence field
      (disk -> observability -> shapes -> gating).
    kind: architecture
    status: settled
    evidence_ref: >-
      docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md
      ("SO — ops substrate (new parallel station; gates S3)", v4 Orientation 2026-10-05)
    owner_ratification_ref: >-
      owner direction 2026-10-05 ("shouldn't need 16gb at all") and the v4
      director review's recorded SO station.
  belief:
    believed_state: >-
      One substrate explains the five incidents: no storage lifecycle and
      no durable guest observability. The memory question is empirical —
      the receipt decides whether 16 GiB was paying for garbage or real
      working set; the GC ordering defect is already identified.
    main_uncertainty: >-
      Whether the journal-GC fix + declared ceiling + balloon/zswap hold
      under hibernate churn and the full tracking fleet, and whether the
      platform-artifacts GC can distinguish retained predecessors from
      dead weight without route-slot bookkeeping.
    next_observation: >-
      The memory receipt before/after the host-side offline GC on the
      owner computer; the first retention GC pass on platform-artifacts
      under the layered-release workload.
  blocker_or_risk: >-
    The memory receipt must land before the 4 GiB ceiling; the owner
    computer cannot go below its proven working set. corpus-dolt's cap is
    handed to the capacity mission, not fixed here — guests cannot be
    dense while it takes over half the host RAM.
  next_action: >-
    Slice order: (1) storage lifecycle — retention/refcount GC for
    platform-artifacts keyed by live route slots + dead-dir reaper;
    (2) guest observability — per-VM rotated serial/journal sink wired
    through vmctl; (3) declared shapes + memory receipt + the GC ordering
    fix + the 4 GiB ceiling on the owner computer once it fits;
    (4) CI deploy gating against deployed identity.
  slice_1_progress_2026_10_05: >-
    Storage lifecycle landed + deployed. Dead-dir reaper: orphan-auth bug
    fixed (orphans had empty user/desktop so authorizeLifecycleRoute always
    refused) + candidate-* dir match added (8a322e57, deployed). Platform-
    artifacts GC: internal/platform/artifact_gc.go — reachability sweep over
    file-cas-chunks/roots, projection-base, platform-update, og with
    DB-derived live sets + dry-run + grace + bounded deletes; periodic
    corpusd GCRunner + on-demand /internal/platform/artifact-gc endpoint
    (f20f3a51, DEPLOYED 2026-10-05 — corpusd commit f20f3a51). Deployed
    dry-run proof in flight against the live store; the report lands in
    docs/evidence/. The event-tape namespaces are a chain-retention slice.
    candidate-fleet-e15cb89f is the LIVE owner computer (health 200,
    firecracker running) — not a zombie; the stale appearance was absent
    serial capture (fixed in slice 2) + a stale-active reconciliation gap
    where the deploy refresh boots the canary and marks it active.
  slice_2_progress_2026_10_05: >-
    Guest observability landed (e8137d03). Root cause: buildFirecrackerConfig
    emits no logger/log_fifo/serial_out_path; guest console=ttyS0 output went
    to FC's stdout, wired to vmctl's shared os.Stdout (all VMs mixed into
    vmctl's journald, unindexed). FC 1.15.1 accepts serial_out_path only via
    its API socket; --no-api skips it. Fix: cmd.Stdout is now a bounded
    rotating writer at <StateDir>/<vmID>/console.log (1 MiB active + 4
    generations, 5 MiB cap) in the shared launchFirecracker path — covers
    cold boot, resume, recover, refresh. Unit-tested. Deployed proof pending
    the next VM launch on the new vmctl. Residual: ReattachVM (vmctl restart)
    orphans the serial fd until next launch seam.

receipts:
  - id: s0-1-storage-lifecycle-2026-10-05
    kind: station_slice
    status: deployed_pending_proof
    commits: ['8a322e57', 'f20f3a51']
    summary: >-
      Dead-dir reaper correctness (orphan-auth + candidate-* match) and a
      platform-artifacts reachability GC (dry-run default, on-demand
      endpoint) — deployed, corpusd at f20f3a51. Deployed dry-run proof in
      flight against the live store.
  - id: s0-2-guest-observability-2026-10-05
    kind: station_slice
    status: landed
    commits: ['e8137d03']
    summary: >-
      Per-VM rotated guest serial/console sink at <StateDir>/<vmID>/console.log
      (1 MiB active + 4 generations) via child stdout capture in the shared
      launchFirecracker path. Deployed proof pending next VM launch.
---

# SO — Ops Substrate

This station owns the platform's operational substrate: storage lifecycle,
guest observability, declared VM shapes + memory budget, and CI deploy
gating. It is the fire order — every incident since 2026-10-01 traces to
one of those four surfaces, not to a release or security defect.

The full slice ordering, evidence requirements, and the memory-budget
diagnosis/hypothesis/measure/shrink/elasticity plan live in
docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md
("SO — ops substrate (new parallel station; gates S3)", v4 Orientation
2026-10-05). This file is the executable contract for the station.
