---
definition_version: 4
definition_id: choir-appdev-s0-reality-boot-timeline-2026-10-01
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-supervised-app-development-metamission-2026-10-01

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

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
      The current runtime wrapper reasserts CHOIR_BASELINE_RELEASE_ROOT to
      the image package and execs that package's autoputer binary
      (nix/autoputer-vm.nix:93-138); release staging admits only changed
      files below var/lib/artifact/release/ (internal/capsule/executor.go:1344-1375).
    - >-
      Capsule creation unshares mount, PID, network, UTS, IPC, user and cgroup
      namespaces; its declared network namespace has no interfaces
      (internal/capsule/namespace.go:25-45). This is source evidence, not a
      staging proof of its effective mount or Nix-build behavior.
    - >-
      Host setup enables IP forwarding and appends FORWARD ACCEPT rules both
      into and out of each VM tap (internal/vmmanager/manager.go:2686-2732),
      so tap-to-tap reachability must be measured rather than assumed.
    - >-
      The Node B root and /data filesystems are btrfs
      (nix/disks.nix:1-16), while Firecracker snapshot create/load and UFFD
      lazy loading have no staging measurement. The current manual data.img
      helper describes itself as a recovery snapshot tool, not machine
      hibernate (scripts/node-b-data-img-snapshot:19-44).
    - >-
      Owner-reported cold boot is about 15-30 s and warm boot about 5 s, with
      no per-phase timeline; hibernate is stop and resume is currently a cold
      boot plus recovery planning (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:130-134).

finish:
  deliver: >-
    Staging has an inspectable boot-and-resume timeline and a reality-probe
    evidence set that attributes the current fresh and owner-sized computer
    paths before S1-S4 select security, layering, capsule-builder, or resume
    changes.
  artifact: >-
    A deployed staging boot-timeline instrument plus
    docs/evidence/s0-probe-index-2026-10-01.json, whose named entries retain
    the per-phase receipts and the pre/post state of each disposable-computer
    experiment; every confirmed defect has a preceding docs/problems record.
  acceptance:
    - action: >-
        On staging, boot one fresh and one owner-sized computer and fetch the
        timeline receipts named s0a-boot-timeline-fresh-2026-10-01.json and
        s0a-boot-timeline-owner-sized-2026-10-01.json.
      proves: >-
        Each receipt attributes host spawn, kernel, initrd, every systemd unit,
        runtime phases, and first healthy response for the real boot path.
      evidence_class: deployed proof
    - action: >-
        On the owner-sized staging computer, create and resume a Firecracker
        snapshot and capture s0a-snapshot-create-resume-2026-10-01.json and
        s0a-uffd-lazy-load-2026-10-01.json, including wall times and the pinned
        Firecracker/kernel identities.
      proves: >-
        Snapshot create/resume and UFFD lazy-load behavior are measured on the
        pinned platform rather than inferred from feature presence.
      evidence_class: deployed proof
    - action: >-
        Capture the named S0a receipts s0a-guest-layout-2026-10-01.json,
        s0a-runtime-closure-2026-10-01.json, s0a-tap-reachability-2026-10-01.json,
        s0a-gateway-token-visibility-2026-10-01.json, and
        s0a-vm-state-reflink-2026-10-01.json on staging.
      proves: >-
        Store/mount layout, runtime closure dependency resolution, tenant-tap
        reachability, token visibility, and the actual VM-state reflink path
        have observed, separately attributable results.
      evidence_class: deployed proof
    - action: >-
        Run every S0b probe only on a disposable staging computer; record its
        computer identity and pre/post state in s0b-selfdev-go-effect-2026-10-01.json,
        s0b-m9a-full-bundle-2026-10-01.json, s0b-capsule-store-overlay-2026-10-01.json,
        s0b-erofs-nix-db-2026-10-01.json, s0b-nixpkgs-build-2026-10-01.json, and
        s0b-runtime-dependency-lifecycle-2026-10-01.json.
      proves: >-
        The Go effect, full bundle transport, namespace/mount feasibility,
        EROFS Nix DB, sandboxed one-package build, and a base-absent runtime
        dependency survive only the required dispose, activate, reboot, and
        restore transitions.
      evidence_class: deployed proof
    - action: >-
        Inspect the probe index before any repair commit; each confirmed
        finding links a docs/problems/s0-*-2026-10-01.md record dated before
        its fix.
      proves: Problem documentation precedes repair, so observations remain
        independently reviewable.
      evidence_class: deployed proof + problem documentation
  rollback: >-
    Revert and redeploy only the timeline instrumentation; discard every S0b
    disposable computer and its snapshot artifacts. Never operate the owner
    computer as an experiment or retain a snapshot after a failed probe.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize uncertainty about where boot/resume time and app-layer effects
    actually fail while preserving the existing computer's authority,
    persistent state, and tenant boundaries.
  goodharting_would_be: >-
    Reporting Firecracker capabilities, a tiny fresh-VM timing, or source
    layout as if it were an owner-sized staging measurement; declaring an
    experiment safe without its disposable-computer pre/post receipt.

homotopy:
  realism_axis: >-
    Increasing fidelity of the same guest lifecycle: source inspection, fresh
    staging boot, owner-sized staging boot, then disposable-computer capsule
    build and runtime-dependency lifecycle across dispose, activate, reboot,
    and restore.

boundaries:
  mutation_class: orange
  phase_mutation_classes:
    S0a: green/yellow instrumentation and read-only staging observation
    S0b: orange disposable-computer experiments only
  authority_sources:
    - owner-ratified choir-supervised-app-development-metamission-2026-10-01
    - AGENTS.md
    - docs/computer-ontology.md
  must_preserve:
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
    - Provider credentials never enter a capsule.
    - A machine snapshot is restored at most once, only against its paired disk and its exact image/hypervisor versions.
  excluded:
    - S1 security-floor remediation, egress policy, runtime confinement, and tap isolation
    - S2 app-layer implementation or updater/runtime-exec cutover
    - S3 hibernate/resume implementation or cold-boot optimization
    - S4 capsule open-world implementation or selected writable-store mechanism
    - S6 commit-gate release materialization
  protected_surfaces: []

now:
  status: working
  slice: >-
    S0a: deploy the boot-timeline instrument and collect the read-only staging
    reality receipts before opening the separately bounded S0b disposable-computer experiments.
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
    id: s0-observed-boot-and-builder-boundary
    claim: >-
      A per-phase staging timeline and bounded disposable-computer probes will
      distinguish the boot/resume bottleneck and establish whether the current
      capsule/base substrate can carry the S2-S4 builder and effect path without
      widening authority.
    test: >-
      The named S0a and S0b deployed receipts reproduce their declared lifecycle
      transitions, retain the required identities and pre/post state, and link
      every confirmed defect to a problem document before repair.
    edge: missing_oracle
    delta_o: >-
      Timeline instrumentation through first healthy response plus direct
      snapshot/UFFD and disposable-capsule lifecycle observation on staging.
    scope_if_supported: >-
      The current pinned staging image, Firecracker/kernel pair, and disposable
      computers on the single-host Choir staging deployment.
    status: active
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:503-518
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:619-638
  decision:
    what: >-
      Run S0a as instrumentation/read-only evidence first; admit S0b only as
      orange experiments on disposable computers, and problem-document every
      confirmed finding before any fix.
    kind: operational
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:460-463
    owner_ratification_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:375-379
  belief:
    believed_state: >-
      The served release and the image-baseline runtime differ, the boot path
      lacks phase attribution, and source facts alone cannot establish the
      current security, snapshot, or builder boundaries.
    main_uncertainty: >-
      Whether the owner-sized pinned platform can create/resume a lazily loaded
      snapshot and whether a capsule can build a closure that remains usable
      after the required lifecycle transitions.
    next_observation: >-
      The first fresh and owner-sized deployed timeline receipts, followed by
      the disposable computer's scoped S0b pre/post evidence.
  blocker_or_risk: >-
    A probe can expose a runtime-mutating or security defect; document it as a
    problem before any repair and do not widen S0 into a sibling's remediation.
  next_action: >-
    Begin S0a by landing and deploying the per-boot timeline instrument, then
    capture the fresh-computer staging receipt before measuring the owner-sized
    computer.

receipts: []
---

# S0 mechanism

The timeline is an observer, not a boot-path rewrite. It records one monotonic
receipt for the actual staging lifecycle so the same boundaries remain visible
as the station moves from cold boot to snapshot resume.

## S0a — timeline and read-only reality

- Start one receipt at the host spawn boundary and preserve phase boundaries for
  kernel, initrd, each systemd unit, runtime initialization, and first health.
- Use one fresh computer and one owner-sized computer; report identities,
  monotonic timestamps, wall durations, and any missing observer boundary.
- Inspect guest mounts, store layout, runtime closure dependencies and image
  identity without changing them. Source shows the runtime service follows the
  updater and signer services (nix/autoputer-vm.nix:654-764); timing must show
  the effective ordering on staging.
- Treat tap-to-tap reachability, gateway-token visibility, reflink behavior,
  snapshot create/load, and UFFD lazy loading as experiments with a negative
  result as valuable as a positive one.

## S0b — disposable-computer boundary

- Create the disposable computer before the first orange probe and retain its
  identity, baseline, probe input, result, and terminal disposition in the
  matching named receipt.
- Probe the self-dev Go effect and M9a full bundle independently: a transport
  acceptance is not evidence that the runtime executes the intended payload.
- Mount only the proposed `/nix/store`-prefixed overlay inside the capsule
  namespace; observe EROFS Nix DB and a sandboxed one-package `nix build`
  before selecting an S4 writable-store mechanism.
- Build one runtime dependency absent from the base, then prove its behavior
  through dispose → activate → reboot → restore. Do not generalize from a
  capsule-local success.

## Handoff

The probe index is the single S0 evidence map. A confirmed finding first becomes
`docs/problems/s0-<finding>-2026-10-01.md`; only the next repair boundary may
name that problem. S1 receives reachability/token findings, S2 receives effect
and closure facts, S3 receives timeline/snapshot facts, and S4 receives mount,
Nix-DB, sandbox, and lifecycle facts.
