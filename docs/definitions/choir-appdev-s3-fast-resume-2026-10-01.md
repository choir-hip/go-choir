---
definition_version: 4
definition_id: choir-appdev-s3-fast-resume-2026-10-01
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-supervised-app-development-metamission-2026-10-01

review: {reviewer: none, frozen_ref: none, verdict: none, evidence_ref: none}

start:
  captured_at: '2026-10-01T00:00:00Z'
  source:
    canonical_ref: main@8aa1dce9
    deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00;
      owner guest computer-03335285269bdba4f94377e56879f9e6 on a3cfaa00'
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed_baseline:
    - >-
      Boot latency (owner-reported 2026-10-01): cold boot about 15-30 s,
      warm boot about 5 s; both too slow. No per-phase boot timeline is
      recorded. Hibernate is stop (vmmanager HibernateVM); resume is a cold
      boot plus recovery planning. vmctl polls readiness every 250 ms.
      Every guest-image deploy reboots active computers
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:130-134).
    - >-
      VM state lives on btrfs, so reflink copies of data.img are available;
      Firecracker snapshot create/load is unused
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:142-143).
    - >-
      The guest health surface exposes cumulative engine-lock wait and hold
      counters for the latency drain probe, not a durable zero-active-work or
      quiescence oracle (internal/objectgraph/engine_mutex.go:102-117).

finish:
  deliver: >-
    An owner wakes an owner-sized hibernated computer in about a second without
    restoring memory against moved, stale, or consumed computer state.
  artifact: >-
    Deployed machine-snapshot hibernate/resume for a computer: a durable
    admission/quiescence fence held through drain, sync, fsfreeze, and capture;
    root-only Firecracker snapshot files in its state directory; paired
    reflinked disks with recorded generation; durable vmctl snapshot metadata
    and one-shot consumption; exact pre-load validation; cold-boot fallback;
    post-resume repair; and hibernate/resume space-and-time timeline records.
  acceptance:
    - action: >-
        On staging, hibernate and owner-wake a computer with at least 10 GB
        persistent state and 8 GiB RAM, an open Texture document, and a
        completed run. Build and exercise a durable admission/quiescence fence:
        drain active work, reject or hold racing new work, and retain that fence
        from the no-active-work check through sync, fsfreeze, and snapshot
        capture. Record hibernate write time, btrfs memory-file space, and
        wake-to-healthy timeline across a representative run set. Verify the
        same state is served and the post-resume hook steps the clock, refreshes
        the gateway token, reseeds RNG, regenerates process-local keys,
        reconnects clients, and handles the epoch. Verify Firecracker exits and
        guest RAM is freed on the host, snapshot files are unreadable to
        non-root users, and snapshot space participates in a pressure-reclaim
        decision.
      proves: >-
        Owner-sized machine snapshots quiesce rather than race writes, free
        host RAM, and resume safely with p50 wake-to-healthy under 1 s and p95
        under 3 s.
      evidence_class: deployed proof
    - action: >-
        Independently alter each bound restore-set field—paired disk generation,
        image/kernel/Firecracker identity, effective release, and event
        head—then request resume on staging. For every variant, observe refusal
        before Firecracker load, snapshot discard, and a cold boot that serves
        the consistent disk.
      proves: >-
        Each stale or mismatched identity fails closed before unsafe memory can
        load.
      evidence_class: deployed proof
    - action: >-
        Resume a valid snapshot once, then issue both a second sequential resume
        request and concurrent resume requests; interrupt snapshot capture and
        verify its incomplete receipt cannot be resumed.
      proves: >-
        Consumption is durable in vmctl before load, concurrent resumes admit at
        most one loader, and interrupted capture cannot create a resumable state.
      evidence_class: deployed proof
    - action: >-
        Use the S0 per-phase cold-boot timeline to identify the largest
        attributable critical-path delay, deploy a trim for that measured delay,
        and record before/after timeline evidence while preserving cold-boot
        readiness and the snapshot fallback.
      proves: Cold-path optimization follows observed attribution rather than a speculative shortcut.
      evidence_class: deployed proof
  rollback: >-
    Discard the machine snapshot and cold boot from its paired consistent disk;
    for a landed defect, git revert and redeploy the prior artifact. Never retry
    a refused, moved, stale, or consumed snapshot.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize wake-to-healthy latency for a long-lived owner computer while
    preserving disk consistency, exact restore identity, single consumption,
    secret renewal, and a usable cold-boot refusal path.
  goodharting_would_be: >-
    Reporting sub-second resume from a tiny fresh VM, or loading memory before
    checking disk generation, release, event head, and consumption state.

homotopy:
  realism_axis: >-
    Snapshot fidelity on the same Firecracker computer lifecycle: from a
    minimally stateful disposable computer through an owner-sized, long-lived
    computer with its real disk, release, event head, and pressure reclaim.

boundaries:
  mutation_class: red
  authority_sources:
    - owner direction in session 2026-10-01 (machine snapshots, latency targets)
    - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:425-437
    - docs/computer-ontology.md
    - AGENTS.md:109-131
  must_preserve:
    - Every committed change is restorable through the pinned-head path.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
    - A machine snapshot is restored at most once, only against its paired disk and its exact image/hypervisor versions.
  excluded:
    - S0-reality-and-boot-timeline ownership of initial timeline instrumentation and cold-path attribution.
    - S1-security-floor network, runtime-confinement, and credential-scrubbing work.
    - S2-layering-runtime-from-release release materialization and updater trust-boundary work.
    - S4-capsule-open-world and S9-forks-and-fleets; machine snapshots never become portable fork artifacts.
  protected_surfaces: [Firecracker lifecycle, snapshot files, vmctl state machine]
  heresy_delta:
    discovered: >-
      EngineMutexStats are cumulative latency telemetry, not a durable
      quiescence fence; the existing hibernate transition records hibernated
      after delegating without this admission boundary.
    introduced: none at authored baseline
    repaired: >-
      None yet; S3 must repair the missing fence and pre-load invariant checks
      before its deployed acceptance can settle.

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S3 machine-snapshot hibernate/resume: establish the exact paired
    restore set, quiesce and one-shot protocol, fallback, post-resume repair,
    pressure reclaim, and evidence-driven cold-path trim after its gates close.
  source_ref: main@8aa1dce9
  deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00; owner guest computer-03335285269bdba4f94377e56879f9e6 on a3cfaa00'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: exact-machine-snapshot-resume
    claim: >-
      If a durable admission/quiescence fence drains active work and remains
      held through sync, fsfreeze, and capture, then an exact, paired, one-shot
      Firecracker snapshot with post-resume repair can meet the wake target
      without widening restore risk.
    test: >-
      Staging hibernate/resume acceptance exercises the fence under racing
      work, interrupted capture, and concurrent resume; records timing, space,
      healthy response and post-resume effects; and refuses every invalidation
      before Firecracker load.
    edge: missing_oracle
    delta_o: >-
      S0 timeline evidence plus durable fence and snapshot receipts that bind
      the disk generation, image, kernel, Firecracker version, effective
      release, event head, capture completion, consumption transition, and
      pre-load refusal reason.
    scope_if_supported: >-
      Same-host Firecracker hibernate/resume for a single owner computer on
      staging; not cross-host restore, export, or fork construction.
    status: proposed
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:544-559
  decision:
    what: >-
      Use machine snapshots only for exact, same-computer hibernate/resume;
      semantic snapshots remain the portable fork and distribution mechanism.
    kind: architecture
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:425-437
    owner_ratification_ref: >-
      owner 2026-10-01: "I ratify #1"; the owner-ratified metamission records
      machine snapshots for resume and semantic snapshots for distribution.
  belief:
    believed_state: >-
      Current hibernate preserves persistent data but kills Firecracker, and
      current resume launches Firecracker again rather than loading memory
      (internal/vmmanager/manager.go:841-942).
    main_uncertainty: >-
      Whether the pinned Firecracker/kernel with owner-sized memory can create
      and lazily restore a safe snapshot inside the target budget.
    next_observation: >-
      S0's owner-sized snapshot create/resume and UFFD lazy-loading measurement
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:503-511),
      followed by the S1 and S2 completion receipts.
  blocker_or_risk: >-
    A stale disk, advanced effective release/event head, or a second memory
    load can corrupt filesystem state or duplicate live secrets; omission of a
    post-resume repair is an identity and availability risk
    (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:455-459).
  next_action: >-
    Promotes to working when S1-security-floor and
    S2-layering-runtime-from-release complete; then consume the S0 timeline and
    implement the invariant protocol before any latency trim.

receipts: []
---

# S3 — Fast Resume

## Mechanism sketch

The current manager implements hibernate as process termination with persistent
state retained, and resumes by launching Firecracker then waiting for guest
readiness (internal/vmmanager/manager.go:841-867,
internal/vmmanager/manager.go:870-942). EngineMutexStats are cumulative
latency counters, so they cannot prove that no work is active
(internal/objectgraph/engine_mutex.go:102-117). S3 therefore builds a durable
admission/quiescence fence: it drains admitted work, rejects or holds racing
work, and remains held from the no-active-work check through `sync`,
`fsfreeze`, and snapshot capture.

The snapshot receipt binds one computer identity to the Firecracker memory and
VM-state files, reflinked disk, disk content generation, exact image/kernel/
Firecracker versions, effective release, and event head. State storage already
places `data.img` under each VM state directory
(internal/vmmanager/manager.go:731-748), while the host root and `/data`
filesystems are btrfs (nix/disks.nix:6-16). S3 records the generation rather
than treating a path name or reflink alone as proof of the disk pairing.

Before any memory load, vmctl durably marks the receipt consumed and verifies
the disk generation, image, kernel, Firecracker identity, effective release,
and event head. A mismatch, incomplete capture, or loser of a concurrent
resume race discards the snapshot and uses the normal cold-boot path; it never
attempts partial restore. The current vmctl hibernate transition delegates to
the manager, records `hibernated`, and persists ownership state
(internal/vmctl/ownership.go:2020-2061), so the one-shot transition belongs in
that protected state machine rather than a separate cache.

## Risks and handoff

Resume repairs all process-lifetime material before the computer is healthy:
clock step, gateway-token refresh, RNG reseed, process-local key regeneration,
reconnections, and epoch handling. Snapshot files remain root-only within the
owning computer state directory. Hibernation must prove Firecracker exit and
host-RAM release, while pressure reclaim accounts for snapshot disk usage rather
than retaining unbounded memory images. Existing pressure reclaim already
selects bounded eligible computers and hibernates only under active host
pressure (internal/vmctl/pressure_reclaim.go:493-532;
internal/vmctl/ownership.go:2072-2086); S3 extends its accounting to snapshot
space without changing its authority policy.

S0 hands S3 the per-phase timeline and owner-sized snapshot measurements
(docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:503-511).
S3 adds hibernate-write and btrfs-space observations beside resume time, then
deploys a cold-path trim selected from S0's measured attribution, not a guessed
optimization. S2 hands S3 the effective-release identity that invalidates a
snapshot after an app-layer change; S1 supplies the security floor required
before preserved memory and refreshed credentials are operated
(docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:519-559).
