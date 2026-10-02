---
definition_version: 4
definition_id: choir-appdev-s0-reality-boot-timeline-2026-10-01
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-supervised-app-development-metamission-2026-10-01

review:
  reviewer: 'agentic-consensus authoring panel (devin, codex, claude,
    omp-gpt6-sol, omp-gpt6-luna, omp-gemini38, omp-glm53-flash) - send-back
    round resolved'
  frozen_ref: 'main@653d975c'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20261001-135404/'

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
    the per-phase receipts, the selected-or-falsified S2 builder substrate, and
    the pre/post state of each disposable-computer experiment; every confirmed
    defect has a preceding docs/problems record.
  acceptance:
    - action: >-
        On staging, boot one fresh and one owner-sized computer and fetch the
        timeline receipts named s0a-boot-timeline-fresh-2026-10-01.json and
        s0a-boot-timeline-owner-sized-2026-10-01.json.
      proves: >-
        Each receipt attributes host spawn, kernel, initrd, every systemd unit,
        runtime phases, first healthy response, and the booted image/runtime
        identities for the real boot path.
      evidence_class: deployed proof
    - action: >-
        Capture the named S0a receipts s0a-guest-layout-2026-10-01.json,
        s0a-runtime-closure-2026-10-01.json, s0a-tap-reachability-2026-10-01.json,
        and s0a-gateway-token-visibility-2026-10-01.json on staging.
      proves: >-
        Store/mount layout, runtime closure dependency resolution, tenant-tap
        reachability, and token visibility have observed, separately
        attributable results.
      evidence_class: deployed proof
    - action: >-
        Use a disposable owner-sized fixture, never the owner computer, to
        create and resume a Firecracker snapshot and exercise the VM-state
        reflink path; capture s0b-snapshot-create-resume-2026-10-01.json,
        s0b-uffd-lazy-load-2026-10-01.json, and
        s0b-vm-state-reflink-2026-10-01.json with wall times and pinned
        Firecracker/kernel identities.
      proves: >-
        Snapshot create/resume, UFFD lazy-load, and reflink behavior are
        measured on the pinned platform without mutating the owner computer.
      evidence_class: deployed proof
    - action: >-
        Run every remaining S0b probe only on a disposable staging computer;
        record its computer identity and pre/post state in
        s0b-selfdev-go-effect-2026-10-01.json,
        s0b-m9a-full-bundle-2026-10-01.json,
        s0b-capsule-store-overlay-2026-10-01.json,
        s0b-erofs-nix-db-2026-10-01.json,
        s0b-nixpkgs-build-2026-10-01.json,
        s0b-runtime-dependency-lifecycle-2026-10-01.json, and
        s0b-builder-substrate-decision-2026-10-01.json.
      proves: >-
        The Go effect, full bundle transport, namespace/mount feasibility,
        EROFS Nix DB, sandboxed one-package build, and a base-absent runtime
        dependency are observed through the declared lifecycle; the final
        receipt selects host service, privileged builder capsule, or scoped
        guest service as S2's builder substrate, or falsifies all three with
        evidence. An unsupported transition is a valid S0 finding, not an
        obligation to implement S2 or S4 inside this station.
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
  protected_surfaces: [guest boot path, Firecracker lifecycle]
  heresy_delta:
    discovered: prospective — S0 probes may reveal boot, snapshot, token, or capsule-boundary heresies; each confirmed instance is first recorded as a problem.
    introduced: none — S0 does not intentionally widen a runtime authority boundary.
    repaired: none — this station observes and documents; remediation belongs to the receiving sibling station.

now:
  status: working
  slice: >-
    S0a complete: instrument deployed, deadlock hotfix landed and verified
    on staging, seven receipts captured (fresh + owner-sized + post-refresh),
    three new problem docs filed. S0b is the live slice: bounded
    disposable-computer probes (capsule health map, M9a bundle, Go effect,
    snapshot/resume).
  source_ref: main@4708a034
  deploy_identity: 'staging https://choir.news deployed_commit=fd8b2973 (deadlock fix live); owner guest computer-03335285269bdba4f94377e56879f9e6 epoch=1001'
  candidate:
    id: s0a-boot-timeline-instrument
    state: landed
    ref: main@7e412ca7
    base: main@5e789bef
    digest: deployed_and_probed_post_fix
    scope:
      - internal/autoputer/boot_timeline.go (guest collector + /internal/boot/timeline + /internal/diag/tcp-dial)
      - internal/autoputer/run.go (phase marks, SetOnListen, replay volume, applied_rows predicate)
      - internal/server/server.go (SetOnListen hook)
      - internal/vmmanager/boot_timeline.go + manager.go (host marks, merged receipt, persistence, fetch outside m.mu, t.mu deadlock fix, locked MarshalJSON)
      - internal/vmctl/handlers.go + ownership.go + cmd/vmctl/main.go (boot-timeline endpoint + BootKind)
  conjecture:
    id: s0-observed-boot-and-builder-boundary
    claim: >-
      A per-phase staging timeline and bounded disposable-computer probes will
      distinguish the boot/resume bottleneck and establish whether the current
      capsule/base substrate can carry the S2-S4 builder and effect path without
      widening authority.
    test: >-
      The named S0a and S0b deployed receipts reproduce their declared lifecycle
      transitions, retain the required identities and pre/post state, record an
      evidence-backed S2 builder selection or falsify all candidate substrates,
      and link every confirmed defect to a problem document before repair.
    edge: missing_oracle
    delta_o: >-
      Timeline instrumentation through first healthy response plus direct
      snapshot/UFFD and disposable-capsule lifecycle observation on staging.
    scope_if_supported: >-
      The current pinned staging image, Firecracker/kernel pair, and disposable
      computers on the single-host Choir staging deployment.
    status: testing
    evidence_refs:
      - docs/evidence/s0a-boot-timeline-fresh-2026-10-01.json
      - docs/evidence/s0a-boot-timeline-owner-sized-2026-10-01.json
      - docs/evidence/s0a-boot-timeline-owner-sized-post-refresh-2026-10-01.json
      - docs/evidence/s0a-guest-layout-2026-10-01.json
      - docs/evidence/s0a-runtime-closure-2026-10-01.json
      - docs/evidence/s0a-tap-reachability-2026-10-01.json
      - docs/evidence/s0a-gateway-token-visibility-2026-10-01.json
      - docs/problems/s0-gateway-token-on-kernel-cmdline-2026-10-01.md
      - docs/problems/s0-tap-egress-unfiltered-2026-10-01.md
      - docs/problems/s0-deploy-refresh-skips-autoputer-internals-2026-10-01.md
      - docs/problems/s0-vocab-rescan-fires-on-any-replay-2026-10-01.md
      - docs/problems/s0-internal-surface-forgeable-caller-2026-10-01.md
      - docs/problems/s0-fetch-guest-timeline-deadlock-2026-10-01.md
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
      Guest tap/egress is fully open (guest->guest :8085, guest->host vmctl
      :8083, guest->external :443 all reachable); the gateway token rides the
      kernel cmdline (world-readable in the guest) and host fc-config; the
      runtime closure is 11 requisites under the guest's Nix store path.
      Deploy's active-VM refresh skips autoputer-internal pushes, leaving
      stale guests until manual refresh. The vocab-rescan-on-any-replay
      substrate defect is repaired: owner-sized refresh (epoch 1001) reaches
      healthy at 26.6s with applied_rows=0; fresh boot 8.6s. The fetch-guest-
      timeline deadlock (t.mu reentry + fetch under m.mu) is repaired and
      verified live; the lock-substrate class remains (bootVM pre-launch
      exec under m.mu, r.mu held across manager calls) — problem-documented
      for S0b/S1.
    main_uncertainty: >-
      Whether the disposable owner-sized fixture can create/resume a lazily
      loaded snapshot and whether a capsule can build a closure that remains
      usable after the required lifecycle transitions; S0b must select or
      falsify the S2 builder substrate rather than assume one.
    next_observation: >-
      S0b disposable-computer receipts: capsule health map, M9a bundle
      install/activate/reboot/restore, one Go effect, one runtime dep absent
      from the base.
  blocker_or_risk: >-
    The 300s deploy-refresh timeout stands (a refresh that runs >300s is
    killed mid-boot — acceptable now that real boots land ~27s, but a
    large-tape growth bound belongs to S3). Push-cancellation cancels in-
    flight CI deploys — confirmed 2026-10-02: a docs push (734ce69b) between
    the e4780c1a fix push and its deploy cancelled the fix's in-flight run
    (37026635153), and the docs run then classified docs-only and skipped the
    host deploy — the fix only reached staging via a manual
    force_staging_deploy. A docs push between a fix push and its deploy is
    unsafe until CI concurrency groups are scoped per-ref.
  next_action: >-
    S0b deferred behind S0m (owner direction 2026-10-01: record-native
    messaging station inserted first). On S0m landing, resume here: run the
    disposable-computer probe suite (capsule health map, M9a bundle
    lifecycle, one Go effect, one absent runtime dep, snapshot/resume)
    against a fresh registration computer on staging; then the boundary
    panel on the frozen S0a+S0b evidence and the transition receipt.

receipts:
  - id: s0a-boundary-close-2026-10-02
    kind: slice_transition
    status: closed
    closed_at: '2026-10-02T00:45:00Z'
    boundary: S0a landed → S0b live
    panel: >-
      Agentic-consensus boundary panel (authoring + divergent): S1-sufficient,
      S3-insufficient-as-archived; the divergence forced the replayed-predicate
      fix (21bbabff), the forgeable-caller problem doc, and the deadlock
      hotfix (7e412ca7).
    incident: >-
      21bbabff's fetch-polling tail-out re-entered t.mu → self-deadlock while
      holding m.mu → all boots serialized for ~40min on staging; problem-
      documented, hotfixed, verified live (owner refresh healthy at 26.6s
      post-fix vs 662.7s pre-fix).
    evidence:
      - docs/evidence/s0a-boot-timeline-fresh-2026-10-01.json
      - docs/evidence/s0a-boot-timeline-owner-sized-post-refresh-2026-10-01.json
      - docs/problems/s0-vocab-rescan-fires-on-any-replay-2026-10-01.md
      - docs/problems/s0-internal-surface-forgeable-caller-2026-10-01.md
      - docs/problems/s0-fetch-guest-timeline-deadlock-2026-10-01.md
      - docs/reports/s0a-boot-timeline-landing-2026-10-01.md
      - docs/choir-appdev-s0-reality-boot-timeline-ledger-2026-10-01.md
---

# S0 mechanism

The timeline is an observer, not a boot-path rewrite. It records one monotonic
receipt for the actual staging lifecycle so the same boundaries remain visible
as the station moves from cold boot to snapshot resume.

## S0a — timeline and read-only reality

- Start one receipt at the host spawn boundary and preserve phase boundaries for
  kernel, initrd, each systemd unit, runtime initialization, and first health.
- Use one fresh computer and one owner-sized computer; report booted
  image/runtime identities, monotonic timestamps, wall durations, and any
  missing observer boundary.
- Inspect guest mounts, store layout, runtime closure dependencies and image
  identity without changing them. Source shows the runtime service follows the
  updater and signer services (nix/autoputer-vm.nix:654-764); timing must show
  the effective ordering on staging.
- Treat tap-to-tap reachability and gateway-token visibility as observations
  with a negative result as valuable as a positive one.

## S0b — disposable-computer boundary

- Create the disposable computer before the first orange probe and retain its
  identity, baseline, probe input, result, and terminal disposition in the
  matching named receipt.
- Use a disposable owner-sized fixture, never the owner computer, for snapshot
  create/load, UFFD lazy-load, and VM-state reflink experiments.
- Probe the self-dev Go effect and M9a full bundle independently: a transport
  acceptance is not evidence that the runtime executes the intended payload.
- Mount only the proposed `/nix/store`-prefixed overlay inside the capsule
  namespace; observe EROFS Nix DB and a sandboxed one-package `nix build`
  before selecting the S2 builder substrate: host service, privileged builder
  capsule, or scoped guest service.
- Build one runtime dependency absent from the base, then observe it through
  dispose → activate → reboot → restore. An unsupported transition is a valid
  S0 finding; it must be documented, not repaired in S0.

## Handoff

The probe index is the single S0 evidence map. A confirmed finding first becomes
`docs/problems/s0-<finding>-2026-10-01.md`; only the next repair boundary may
name that problem. S1 receives reachability/token findings, S2 receives effect
and closure facts, S3 receives timeline/snapshot facts, and S4 receives mount,
Nix-DB, sandbox, and lifecycle facts.
