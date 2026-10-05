---
definition_version: 4

# v2 2026-10-01: adds layering via guest Nix closures, fast resume /
# machine-snapshot hibernate, forks and fleets (owner-ratified sibling-computer
# model), and org templates. Stations reordered for development ease: the
# substrate that shortens every later dev loop (security floor, layering,
# fast resume) comes first.
# v3.1 2026-10-04: post-S0a/S0m review. S0a evidence folded in (boot
# attribution, confirmed open tap/egress, gateway token exposure), S0m
# recorded complete with an INCOMPLETE boundary protocol, a source-traced
# cross-tenant authority chain recorded in full (owner: prerelease, include
# security detail), S1a host-boundary hotfix pulled ahead of S0b, CI
# deploy-cancellation and ops hazards named.
# v3.2 2026-10-04 (addendum only, not a revision): S2 layering exec
# stabilization in flight. Overlay exec confirmed guest-only-failing on
# staging; a durable stderr/observability hole blocked root-causing it.
# Four substrate issues recorded for the next revision; a full metamission
# revision is deferred until S2 is stable. See "Addendum 2026-10-04".
# v4 2026-10-05 (director review): S0/S0m/S1a closed; S2 re-scoped around
# the real residuals (provenance, state-compat gate, rollback atomicity of the
# exec pointer, builder->release join, CI wiring). New parallel station SO
# (ops substrate: storage lifecycle, guest observability, declared VM shapes,
# CI deploy gating) gates S3. S3 opens with the snapshot measurement S0b could
# not take. Addendum folded into "Orientation 2026-10-05".

readiness: executable

review:
  reviewer: agentic-consensus panel (convergent 5/5 + divergent 7/7), 2026-10-01
  frozen_ref: main@523f6b45532aba99683575108bc9f1ba5fc9ed9c
  verdict: accept
  evidence_ref: .agentic-consensus/agentic-consensus-20261001-133535 (convergent), agentic-consensus-20261001-133637 (divergent)

metamission:
  stations:
    - id: S0-reality-and-boot-timeline
      path: docs/definitions/choir-appdev-s0-reality-boot-timeline-2026-10-01.md
      readiness: reviewed
      status: complete
      depends_on: []
    - id: S0m-record-native-messaging
      path: docs/definitions/choir-appdev-s0m-record-native-messaging-2026-10-01.md
      readiness: reviewed
      status: complete
      depends_on: []
    - id: S1a-host-boundary-hotfix
      # First slice of S1, pulled ahead of S0b on 2026-10-04: it needs no
      # S0b evidence (reachability is already confirmed by S0a) and closes
      # a source-traced cross-tenant authority chain on a deployment with
      # open registration. Authored as the S1 file's first slice.
      path: docs/definitions/choir-appdev-s1-security-floor-2026-10-01.md
      readiness: reviewed
      status: complete
      depends_on: [S0m-record-native-messaging]
    - id: S1-security-floor
      # S1a complete; the remainder (non-root runtime, gateway token off the
      # cmdline and out of child env, Yaegi kernel floor, zot PATH-shadowing,
      # S0a diag surfaces) runs in parallel with S2 and must close before S4.
      path: docs/definitions/choir-appdev-s1-security-floor-2026-10-01.md
      readiness: reviewed
      status: working
      depends_on: [S0-reality-and-boot-timeline, S1a-host-boundary-hotfix]
    - id: S2-layering-runtime-from-release
      path: docs/definitions/choir-appdev-s2-layering-runtime-from-release-2026-10-01.md
      readiness: reviewed
      status: working
      depends_on: [S0-reality-and-boot-timeline]
    - id: SO-ops-substrate
      # New in v4. Five disk-headroom hits in four days (Root Cause
      # Clustering threshold), guest stderr unreachable, an undeclared
      # 16 GiB VM shape, and CI deploy gating that strands runtime code are
      # one substrate: the platform has no storage lifecycle and no durable
      # guest observability. Runs in parallel with S2; S3 cannot start
      # without it (memory snapshots add GiBs per hibernated computer).
      path: docs/definitions/choir-appdev-so-ops-substrate-2026-10-05.md
      readiness: reviewed
      status: working
      depends_on: []
    - id: S3-fast-resume
      path: docs/definitions/choir-appdev-s3-fast-resume-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S1-security-floor, S2-layering-runtime-from-release, SO-ops-substrate]
    - id: S4-capsule-open-world
      path: docs/definitions/choir-appdev-s4-capsule-open-world-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S1-security-floor]
    - id: S5-live-preview-supervision
      path: docs/definitions/choir-appdev-s5-live-preview-supervision-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S4-capsule-open-world]
    - id: S6-commit-gate-full-release
      path: docs/definitions/choir-appdev-s6-commit-gate-full-release-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S2-layering-runtime-from-release, S5-live-preview-supervision]
    - id: S7-app-packages
      path: docs/definitions/choir-appdev-s7-app-packages-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S6-commit-gate-full-release]
    - id: S8-source-publication
      path: docs/definitions/choir-appdev-s8-source-publication-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S6-commit-gate-full-release]
    - id: S9-forks-and-fleets
      path: docs/definitions/choir-appdev-s9-forks-and-fleets-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S1-security-floor, S3-fast-resume, S8-source-publication]
    - id: S10-org-templates
      path: docs/definitions/choir-appdev-s10-org-templates-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S9-forks-and-fleets]
    - id: S11-mainline-and-security-push
      path: docs/definitions/choir-appdev-s11-mainline-security-push-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S8-source-publication, S9-forks-and-fleets]

start:
  captured_at: '2026-10-01T00:00:00Z'
  source:
    canonical_ref: main@0bddb11aa2d27694cb760f8aebda52c95ba4e44d
    deploy_identity: unknown (not observed from this session; no staging access)
  worktrees:
    - path: /home/user/go-choir (cloud session clone)
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed_baseline:
    - >-
      Self-dev control plane is live: M11 (2026-09-29, staging e886f576)
      ran a desk-authored op frozen -> awaiting_approval -> applied under
      qualified consensus, rejected candidate B, rendered evidence in the
      Texture doc, and restored a pinned head. The probe's change was a
      "minimal reversible evidence change"; no runtime code change was
      asserted to take effect.
    - >-
      Self-dev effect plane reaches only the served release. A capsule's
      release is whatever it writes under var/lib/artifact/release/
      (internal/capsule/executor.go StageGrantedRelease); the updater swaps
      CHOIR_UPDATER_ROOT/current; the only reader of current/ is the
      computer-surface handler serving current/frontend
      (internal/autoputer/computer_surface.go). The runtime process is
      always exec'd from the image's Nix store
      (nix/autoputer-vm.nix autoputerRuntimeExec). Inference from source,
      not yet observed on staging.
    - >-
      Capsules are air-gapped (empty netns, internal/capsule/namespace.go);
      the guest Nix store is read-only EROFS with no nix on the runtime
      PATH. curl | bash and nixpkgs are impossible inside a capsule today.
    - >-
      Apps are compiled into one SPA bundle via a static registry
      (frontend/src/lib/apps/registry.ts) and app backends into the one
      autoputer binary. A new app means rebuilding both monoliths.
    - >-
      M9a platform->computer signed push exists and was proven on staging
      with a 1-file payload; nothing in CI calls it. No computer->computer
      publication surface exists (M9b unbuilt).
    - >-
      Guest VM egress is open and tap->tap forwarding appears permitted
      (internal/vmmanager/manager.go setupHostNetworking; host firewall
      does not filter FORWARD). The guest runtime unit runs as unconfined
      root. Unverified on staging. See
      docs/reports/nixos-agent-platform-redhat-deepseek-audit-2026-10-01.md.
    - >-
      Boot latency (owner-reported 2026-10-01): cold boot about 15-30 s,
      warm boot about 5 s; both too slow. No per-phase boot timeline is
      recorded. Hibernate is stop (vmmanager HibernateVM); resume is a cold
      boot plus recovery planning. vmctl polls readiness every 250 ms.
      Every guest-image deploy reboots active computers.
    - >-
      Semantic snapshots already exist: ProjectionBase blobs + replay
      watermark W with PlanRecovery (rebase/resume/refuse, tail bound
      10,000) landed 9341b5d1
      (docs/reports/choir-rlm-restore-zero-snapshotting-correction-2026-09-09.md).
      The owner computer's blob is about 16 GB.
    - >-
      VM state lives on btrfs (nix/disks.nix), so reflink copies of
      data.img are available. Firecracker snapshot create/load is unused.
  start_corrections:
    - date: '2026-10-01'
      correction: >-
        Owner ratified forked computers as sibling computers (not
        self-dev candidates). The prior "no candidate/worker VM" rule
        predates capsules. AGENTS.md (CLAUDE.md) Safety section and
        docs/computer-ontology.md naming rules were amended in the same
        commit as this v2.
    - date: '2026-10-04'
      correction: >-
        The baseline line "Guest VM egress is open and tap->tap forwarding
        appears permitted ... Unverified on staging" is now CONFIRMED on
        staging by S0a (docs/evidence/s0a-tap-reachability-2026-10-01.json:
        guest->other guest :8085, guest->host vmctl :8083, guest->1.1.1.1:443
        all ok). Its consequence is larger than egress: guest traffic to
        host services is DNATed to 127.0.0.1 and MASQUERADEd to loopback, and
        host-internal authority checks accept caller-controlled headers, so
        any guest inherits host-internal and cross-tenant authority
        (docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md).
        The baseline line "Boot latency ... cold 15-30 s, warm ~5 s; no
        per-phase timeline" is superseded by measured S0a receipts: fresh
        cold 8.6 s, owner-sized refresh 26.6 s (was 662.7 s before the
        vocab-rescan repair). See now.belief.

finish:
  deliver: >-
    An external agent holding only a Choir API key prompts a user's
    computer to build a new app, or modify an existing one. The owner
    watches the development live in the logged-in web desktop, through
    the Texture work document and a live preview of the app running
    inside the capsule. The agent may run arbitrary shell, including
    curl | bash and nixpkgs, and all of it stays inside a capsule. The
    change lands only when automated tests pass and the owner approves.
    It then works on both the guest backend and the guest frontend, and
    survives reboot and restore. The owner can publish the change to the
    host as source. Another owner can review it, take the source,
    customize it, build it locally in a capsule, and adopt it through
    their own gate. A long-running computer can be snapshotted and forked
    into fleets of ephemeral or persistent sibling computers for parallel
    experiments, and exported as a scrubbed org template. The host can
    mainline a published change into the platform base and push security
    fixes across a divergent fleet without silent breakage, testing
    compatibility on ephemeral forks first. Computers resume from
    hibernation in about a second, and app-layer updates apply without
    rebooting the VM.
  artifact: >-
    A deployed product path on choir.news with these pieces:
    (1) A two-layer guest: a shared non-forkable NixOS base image and a
    per-computer app layer (runtime + frontend + app backends) built as
    a Nix closure against that base. The guest executes the app layer
    from its committed release.
    (2) Machine-snapshot hibernate/resume, with the pairing, one-shot and
    fallback invariants, plus a recorded boot timeline per wake.
    (3) Capsules with recorded, policy-mediated egress and a
    capsule-private Nix store.
    (4) A preview bridge from the desktop into the capsule.
    (5) A commit gate (tests + owner approval) materializing a full
    app-layer release.
    (6) App packages.
    (7) Source publication (patch + base rev + pinned inputs + recipe +
    tests) and an adopt flow.
    (8) Forks as sibling computers with re-keying and data-class
    selection, plus fleet admission.
    (9) Org template export/import.
    (10) Platform security offers with per-computer rebase, exploit-test
    classification on ephemeral forks, and a fail-closed fallback.
  acceptance:
    - action: >-
        Deployed Playwright + API proof. Computer A is driven only by an
        API-key client. It builds a new app with a backend endpoint and a
        frontend window, using curl | bash or a nixpkgs package during
        the build. The owner session sees live Texture updates and a
        working capsule preview before commit. Tests pass, the owner
        approves in the desktop, the app serves from A's guest after a
        guest reboot, and a pinned restore removes it.
      proves: >-
        The single-computer deliverable works end to end, including
        backend effect and reversibility.
      evidence_class: deployed proof
    - action: >-
        Same proof, modifying an existing app across backend and
        frontend.
      proves: Modification of existing app code, not only additive apps.
      evidence_class: deployed proof
    - action: >-
        App-layer update without reboot. A committed app-layer release
        (and a platform app-layer push) applies by updater swap + runtime
        restart only. The Firecracker process and guest boot id are
        unchanged. Time to healthy is recorded.
      proves: Layering removes VM reboots from routine updates.
      evidence_class: deployed proof
    - action: >-
        Hibernate/resume. An owner-sized computer (>=10 GB persistent
        state, 8 GiB RAM) with an open Texture doc and a completed run
        is hibernated (quiesce, snapshot, process exits, RAM freed on
        host) and woken by an owner request. Recorded timeline: wake
        request to first healthy response within the S3 target
        (proposed: p50 under 1 s, p95 under 3 s). The resumed computer
        serves the same state and refreshes its clock, gateway token
        and RNG. A deliberately invalidated snapshot (image mismatch
        or moved paired-disk generation) is refused before Firecracker
        load and falls back to a cold boot with the disk consistent. A
        second resume of a consumed snapshot is refused.
      proves: Fast resume without unsafe restores.
      evidence_class: deployed proof
    - action: >-
        A publishes. Computer B (different owner) lists A's publication,
        reviews the diff and pinned inputs, adopts it, and builds from
        source in its own capsule with a local customization. B's tests
        and B's owner approval land it. No binary from A executes on B.
      proves: Source-only publication and divergent adoption.
      evidence_class: deployed proof
    - action: >-
        Fork and fleet. A long-running computer is forked into three
        ephemeral forks and one persistent fork, each with a chosen data
        class set. Each fork has a new ComputerID, signer keys, privacy
        key and gateway token. Different experiments run in each, and
        outcomes are visible from the parent's Texture doc. The winner
        publishes and the parent adopts. Ephemeral forks are reclaimed.
        Admission refuses a fork that would breach the host memory
        budget.
      proves: Forks are real sibling computers and fleets fit the host.
      evidence_class: deployed proof
    - action: >-
        Org template. The parent is exported as a scrubbed semantic
        template (base rev + patch stack + curated seed data + policy),
        then installed as a fresh computer that builds from source and
        carries none of the parent's private data classes or keys.
      proves: Distributable organizational images.
      evidence_class: deployed proof
    - action: >-
        Security push. The host mainlines A's change and ships an
        app-layer security fix with an exploit test. Compatibility is
        evaluated on an ephemeral fork of each divergent computer. The
        fix auto-applies on a tracking computer, applies after rebase on
        a non-conflicting divergent computer, is classified exempt only
        where structural proof shows the vulnerable component absent
        (a passing exploit test alone never exempts), and fails closed
        (component reverted or capability cut, owner notified) on a
        conflicting vulnerable computer.
      proves: Fleet security push without silent breakage or silent skip.
      evidence_class: deployed proof
    - action: >-
        Negative proofs. A capsule or a fork cannot reach another
        tenant's guest or the host's private services. A fork cannot
        sign as, or read undeclared data classes of, its parent — and a
        seeded excluded-class canary plus a deleted-data remnant are
        unrecoverable from the fork's disk, Dolt history, logs and
        artifact graph under offline scan. Capsules cannot read the
        gateway token. Uncommitted capsule changes do not survive
        disposal. Snapshot files are unreadable outside the owning
        computer's state directory. A snapshot whose paired disk
        generation moved is refused before Firecracker load. A preview
        digest mismatch (owner approved a stale preview) refuses the
        commit. No store path from publisher A's closure is substituted
        on adopter B. Forged or misbound publications, offers, or
        snapshot restores are refused before any mutation.
      proves: No authority widening across capsule, fork, snapshot, or publication.
      evidence_class: deployed proof
  rollback: >-
    Per station: git revert + redeploy. For a computer: pinned-head
    restore through the existing tape/checkpoint path. For hibernate:
    discard the snapshot and cold boot from the paired consistent disk.
    For a fork: delete it (ephemeral) or treat it as any other computer
    (persistent). For publication: retract the record; adopters keep
    their own committed copies. A misclassified security offer is
    reverted by restoring that computer to its pre-offer head.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between "an agent with an API key asked for an app"
    and "the owner is running that app, backend and frontend, having
    watched it built and approved it; other owners and forks can adopt
    it from source; and all of it is reached in seconds, not reboots".
    Preserve the invariants: no authority widening, no unreviewed code
    outside a capsule, every committed change reversible, every fork
    re-keyed, and every fleet push accounted for per computer.
  goodharting_would_be: >-
    A demo that only touches frontend files (the M11 shape). A preview
    that is a screenshot. A "publish" that ships a binary. A "security
    push" that skips divergent computers and reports success. Egress
    enabled by opening the VM wider instead of through a recorded capsule
    proxy. Resume latency measured on a tiny fresh computer instead of a
    long-lived one, or achieved by restoring memory against a disk that
    moved on. A "fork" that copies the parent's keys.

homotopy:
  realism_axis: >-
    The same pipeline at increasing resolution:
    a Go string change in an existing handler, then
    a new frontend-only app, then
    a new app with a backend endpoint and persistent state, then
    a build that fetches from the network and nixpkgs, then
    hibernate and resume of a long-lived owner-sized computer, then
    a second computer adopting with customization, then
    a fleet of forks of a months-old computer, then
    an org template install, then
    a fleet security push across tracking, divergent-compatible,
    divergent-exempt and divergent-conflicting computers.

boundaries:
  mutation_class: red
  authority_sources:
    - owner direction in session 2026-10-01 (deliverable, source only, fork ratification, latency targets)
    - docs/choir-doctrine.md
    - docs/computer-ontology.md (capsule, effect bundle, restore-set, forked computer, snapshot naming)
    - docs/reports/choir-platform-update-system-consensus-2026-09-08.md (stratified layers)
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
    - Provider credentials never enter a capsule.
    - Publication is source plus pinned inputs; no cross-tenant binary execution.
    - A machine snapshot is restored at most once, only against its paired disk and its exact image/hypervisor versions.
    - A fork never holds its parent's identity material.
    - >-
      Guest-originated traffic never carries host-internal authority or
      another tenant's identity. TARGET invariant; currently VIOLATED
      (docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md);
      S1a restores it before S4 opens capsule egress or S9 creates forks.
  excluded:
    - Binary distribution between computers (owner decision 2026-10-01; a trusted host build cache is a later, separate decision).
    - Per-guest nixos-rebuild of the base OS.
    - cloud-hypervisor, CPU/RAM hotplug (owner 2026-10-01; keep a seam - snapshot format behind a vmmanager interface).
    - Multi-host, cross-host snapshot migration, horizontal/vertical host scaling, zero-downtime host deploys (next frontier after 32 GiB utilization is efficient).
    - Forking a computer mid-run (pending tool-call replay from the tape, audit pattern 2.3).
    - Building a bespoke fleet dashboard; fleet view is Texture transclusion.
    - Marketplace, payments, or ranking of publications.
  dependencies_external:
    - >-
      Texture transclusion and rich formatting in the RLM tool-call
      cutover (owner 2026-10-01: not yet integrated). The S5 live work
      view and S9 fleet view consume it. Until it lands, those stations
      render plain ledger entries and record the gap.
  protected_surfaces:
    - guest boot path and runtime exec (S2, S3)
    - VM networking / tap forwarding / capsule egress (S1, S4, S9)
    - Firecracker lifecycle, snapshot files and vmctl state machine (S3, S9)
    - identity material minting (S9, S10)
    - updater trust boundary and release manifest (S2, S6)
    - canonical event commit path (S6, S8, S11)
    - checkpoint / route projection (S6, S11)
    - platform-control signing domain (S8, S11)

now:
  status: working
  slice: >-
    v4.1 director review 2026-10-05 (after ee013c96). S2: all contract
    slices landed (9f5aa8a0 S2-e, c8fb7834 S2-d, 8f06b3d9 S2-c, e1c3924b
    S2-f, e527c169 + a3da83c4 S2-g); the app-layer push is live in CI and
    proven on computer-6450a253 (release 36743b1f6ddb, build.commit
    2f0e2cac). S2 is at station acceptance against builder-produced
    releases. S1 remainder mostly landed: token off the cmdline (619d645e),
    child-env scrub (6b54522c), zot shadowing (a80d2145), diag restricted
    (98cf387b), Yaegi worker landlock/capdrop/seccomp behind
    --session-harden (a578bbc0, eb9c5b1e); non-root runtime remains (problem
    doc 94342f9d). SO: slice 1 (dead-dir reaper, platform-artifacts
    reachability GC in dry-run) and slice 2 (per-VM serial sink e8137d03)
    landed. NEW: World Wire corpus burn triaged. Containment rides SO; the
    WW rearchitecture is recorded as this metamission's consuming
    application (section below).
  source_ref: main@4c76730b13cda645665c6c95b5e89a7e2e805e78
  deploy_identity: 'staging https://choir.news; last observed deployed runtime e605cdde/3c1cbaf6 per S2 layering-diag receipts (verify /health before the next probe)'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: one-pipeline-two-layers
    claim: >-
      The deliverable needs no new governance machinery. M7/M11 self-dev,
      M9a transport, capsule effect bundles, Restore-Zero semantic
      snapshots and record-native desk couplings carry it. What is missing
      is effect reach, openness, speed, a host boundary that holds, and an
      operable substrate:
      (a) the guest executes a per-computer app-layer closure — mechanism
      PROVEN 2026-10-04; contract (provenance, compat, atomic rollback)
      open;
      (b) capsules get recorded egress and a private Nix store;
      (c) a preview bridge;
      (d) the change record is base rev + patch stack + pinned inputs;
      (e) machine snapshots for resume, semantic snapshots for
      distribution;
      (f) guest traffic carries no host-internal authority — S1a
      deployed-verified;
      (g) the host has a storage lifecycle and durable guest observability
      (new v4; SO).
    test: >-
      S2: a real code change built by the host builder from a commit applies
      without reboot and with recorded time-to-healthy; an incompatible
      release refuses before mutation; a release that passes pre-checks but
      fails health restores the predecessor INCLUDING its exec entrypoint.
      SO: a release + snapshot workload runs for a week of deploys without a
      manual disk reclaim, and every guest exec failure leaves a host-visible
      record. Each later station is falsified if it needs a new authority
      path rather than a new effect path.
    edge: missing_oracle
    delta_o: >-
      Durable guest stderr/journal on the host (SO) — today a guest-side
      failure after cold boot is invisible without bespoke diag files.
    scope_if_supported: >-
      Single-host Choir Community Cloud staging (32 GiB Node B), Go runtime
      + Svelte frontend + app backends, Firecracker v1.15.1.
    status: testing
    evidence_refs:
      - docs/reports/s0b-disposable-probe-station-close-2026-10-04.md
      - docs/reports/s1a-host-boundary-station-close-2026-10-04.md
      - docs/reports/s0m-record-native-station-close-2026-10-04.md
      - docs/reports/s2-layering-runtime-checkpoint-2026-10-04.md
      - docs/evidence/s1a-refusal-matrix-2026-10-04.json
      - docs/evidence/s0b-go-effect-2026-10-04.json
      - docs/evidence/s0b-snapshot-resume-2026-10-04.json
      - docs/problems/s2-runtime-exec-still-baseline-2026-10-04.md
      - docs/problems/s2-builder-substrate-2026-10-04.md
      - docs/problems/s2-host-builder-landed-2026-10-04.md
      - docs/problems/node-b-deploy-disk-headroom-2026-10-04.md
      - docs/problems/guest-stderr-unreachable-after-cold-boot-2026-10-04.md
      - docs/problems/ci-docs-only-head-strands-runtime-deploy-2026-10-04.md
      - docs/problems/guest-vm-embedded-dolt-memory-starved-2026-10-01.md
  decision:
    what: >-
      Two-layer guest; source-only publication; machine snapshots for
      resume, semantic for fork/distribution; forks are sibling computers;
      builder substrate = host service (S0b). v4 operational decisions
      (director, under continuous authority, reversible): S2 is re-scoped
      to S2-c..g; SO is added as a parallel station gating S3; S3 opens
      with its own snapshot measurement; acceptance releases must come out
      of the builder (no hand-staged binaries).
    kind: architecture
    status: settled
    evidence_ref: owner statements 2026-10-01 and 2026-10-04; director review 2026-10-05 (this card)
    owner_ratification_ref: >-
      owner 2026-10-01 (layering, Nix closures, forks, source only);
      2026-10-04 (prerelease, full security detail); 2026-10-05
      ("reorient and revise the metamission").
  belief:
    believed_state: >-
      CLOSED: S0 (boot attribution; S0b probe suite, builder decision,
      self-dev ops wedge found + fixed f61de45b with a Go-effect bundle
      frozen), S0m (record-native couplings, boundary closed with panel and
      report), S1a (network isolation + transport-bound internal authority;
      refusal matrix 5/5 refused, 5/5 legitimate flows green).
      CORRECTION: the v3.1 claim that guest->host traffic arrives as
      loopback was wrong; the real guest source IP is preserved (bffad45,
      s1a guest-to-host-addr-preservation evidence). The authority defect
      was the caller-controlled headers, which S1a removed.
      S2 MECHANISM: works in the guest. The crash-loop on vm-3dc68688 was a
      stale-binary release (August build 43310064 vs an October store) that
      the probe hand-staged as /tmp/closure.nar on Node B ("a copied
      autoputer" with a marker byte, synthetic code_commit). Builder output
      was never used for acceptance.
      S2 CONTRACT DEFECTS (source-traced 2026-10-05):
      (1) Provenance unbound: builder Request.CodeCommit is caller-supplied;
      mint accepts arbitrary files + code_commit; nothing checks that the
      binary's embedded buildinfo matches the manifest.
      (2) No state-compat gate: Apply checks the base digest only; schema,
      reducer and store compatibility are probed after restart (health),
      not refused before mutation.
      (3) Rollback is not atomic: the exec pointer
      $CHOIR_UPDATER_ROOT/layering-entrypoint is a global file outside the
      release dir (internal/updater/closure.go:418,457-462). restorePrior
      swaps current/ but leaves the entrypoint pointing at the failed
      release, so a recovery restart re-execs the bad binary. This is the
      likely cause of the silent crash-loop, and a single-state-authority
      violation.
      (4) The self-dev materializer still emits file releases from capsule
      var/lib/artifact/release; nothing joins a capsule change to the host
      builder.
      OPS: five Node B disk-headroom hits since 10-01 (dead vm-state dirs,
      quarantine remnants, nix store; corpus-dolt 97G + platform-artifacts
      77G structural, and every layered release adds a ~146MB nar with no
      GC). Guest stderr is unreachable after cold boot. CI never-cancel
      landed (520a998), but a docs-only head after a failed runtime deploy
      still strands runtime undeployed. VM_INTERACTIVE_MEM_MIB=16384 is set
      in the mutable vmctl-priority.env, outside its declared purpose and
      outside the repo: untracked drift that sizes every interactive guest
      at half the host. MEMORY ROOT CAUSE (2026-10-05): the 2->16 GiB
      ratchet followed store and journal growth, not hot-set growth. The
      live store is about 5 GiB against a 22 GiB garbage journal, because
      the guest GC guard (dolt_maintenance.go:290) precedes the journal
      trigger (:309). There is no GOMEMLIMIT or Dolt cache bound. Owner
      target: 2-4 GiB ceilings, oversubscribed, with residency tiers making
      computers feel always on.
      S3 INPUT GAP: S0b took no snapshot measurement (no vmctl snapshot
      surface; sealed guest). S3 must measure first.
      OPEN RESIDUALS CARRIED: registration-computer-missing-genesis; M9a
      route owner-binding (probably resolved by 69983b0e per S2 route
      promotion; record not closed); zot PATH-shadowing; Management storm
      convergence invariant; guest Dolt journal leak.
    main_uncertainty: >-
      (1) Whether a builder-produced runtime closure stays small enough to
      ship per release once it carries real deps (the probe nar was 146MB),
      i.e. whether the delta-vs-base export keeps app-layer releases
      cheap. (2) Snapshot create/resume cost for a 16 GiB vs 8 GiB guest on
      this host and disk. (3) Whether CI can drive builder -> mint -> push
      for tracking computers within the deploy window.
    next_observation: >-
      S2-e and S2-d land first (small, high-leverage), then one
      builder-produced release from a real commit applied to a disposable
      computer with a deliberate incompatible release and a deliberate
      health-failing release.
  blocker_or_risk: >-
    Disk headroom will block deploys again within days unless SO's storage
    lifecycle starts now; S2 makes it worse (unreclaimed release nars).
    Acceptance probes that stage binaries by hand can pass mechanism checks
    while proving nothing about the release contract. Treat builder-produced
    releases as the only admissible S2 evidence. S4 still requires the
    full S1 floor. S3 must not start before SO's storage lifecycle and the
    VM shape decision.
  next_action: >-
    1. S2 station acceptance: run the conjecture test with builder-produced
       releases (compatible apply without reboot + time-to-healthy;
       incompatible refused pre-mutation; health-failing restored with exec
       reverted). Close the S2-g bind-gap record. Then the station boundary
       protocol (panel, report, transition receipt).
    2. SO containment (World Wire, docs/problems/world-wire-corpus-resource-burn-2026-10-05.md
       "Director review"):
       (a) fix the og GC wrong-store liveness bug (artifact_gc.go:202 reads
       Store A; og lives in Store B) with a split-pool test; og stays
       dry-run, WW bodies excluded;
       (b) make the sourcecycled disable durable via a default-off option
       in nix/node-b.nix;
       (c) take one corpus-dolt CPU sample + processlist with sourcecycled
       stopped.
    3. SO continues: memory receipt + host offline GC of the owner store
       (before/after), the GC-ordering fix, then declared shapes.
    4. S1: non-root runtime (the last S1 item; required before S4).

receipts:
  - id: s0-to-s2-transition-2026-10-04
    kind: station_transition
    status: closed
    landed: S0-reality-and-boot-timeline (S0a + S0b)
    next: S2-layering-runtime-from-release
    terminal_receipt: docs/definitions/choir-appdev-s0-reality-boot-timeline-2026-10-01.md#s0b-boundary-close-2026-10-04
    panel: .agentic-consensus/s0b-boundary-panel-20261004 (8 accept_with_edge / 3 send_back; send-backs closed)
    report: docs/reports/s0b-disposable-probe-station-close-2026-10-04.md
    note: >-
      Recorded on the spine by the 2026-10-05 director review (the
      transition was executed but only receipted on the station file).
      Named edges into S2: wedge fix (landed f61de45b), capsule-namespace
      probe (closed: builder = host service), host snapshot measurement
      (not taken; moved to S3's first slice).
  - id: s0a-slice-landed-2026-10-02
    kind: slice_transition
    station: S0-reality-and-boot-timeline
    status: closed
    boundary: S0a landed -> S0b (S0b then deferred behind S0m)
    identity: main@7e412ca7 (instrument + deadlock hotfix), evidence refresh 4708a034, deployed fd8b2973
    proof_refs:
      - docs/reports/s0a-boot-timeline-landing-2026-10-01.md
      - docs/evidence/s0a-boot-timeline-fresh-2026-10-01.json
      - docs/evidence/s0a-boot-timeline-owner-sized-2026-10-01.json
      - docs/evidence/s0a-boot-timeline-owner-sized-post-refresh-2026-10-01.json
      - docs/evidence/s0a-guest-layout-2026-10-01.json
      - docs/evidence/s0a-runtime-closure-2026-10-01.json
      - docs/evidence/s0a-tap-reachability-2026-10-01.json
      - docs/evidence/s0a-gateway-token-visibility-2026-10-01.json
    panel: >-
      S0a boundary panel recorded in the S0 station file (S1-sufficient,
      S3-insufficient-as-archived); forced 21bbabff, the forgeable-caller
      problem doc, and the 7e412ca7 deadlock hotfix.
    heresy_delta: >-
      discovered: open tap/egress, gateway token on the kernel cmdline,
      forgeable internal caller, deploy-refresh skipping autoputer
      internals, vocab rescan on any replay, vmctl fetch-under-lock
      deadlock (plus 10 sibling lock sites).
      introduced: the 21bbabff fetch-polling deadlock (~40 min serialized
      boots on staging).
      repaired: the deadlock (7e412ca7) and the vocab rescan (owner boot
      662.7 s -> 26.6 s).
    rollback_ref: git revert of the S0a instrument commits (90af3b2..7e412ca)
  - id: s0m-station-terminal-2026-10-04
    kind: station_terminal
    station: S0m-record-native-messaging
    status: closed
    terminal_receipt: docs/definitions/choir-appdev-s0m-record-native-messaging-2026-10-01.md#s0m-ask-acceptance-2026-10-04
    identity: main@a4fcdb8d deployed 2026-10-04T02:01:45Z
    landing:
      source_commit: a4fcdb8d
      ci_ref: run 37165170516 (code green; first deploy failed on Node B disk headroom, redeployed after reclaim)
      deploy_ref: Node B deploy 2026-10-04T02:01:45Z
      environment_identity: https://choir.news deployed_commit=a4fcdb8d
      deployed_acceptance: ask->report->resolve on trajectory 684ddcb1 (docs/evidence/s0m-ask-acceptance-2026-10-04.json)
    heresy_delta: >-
      repaired: research->texture return path, desk-run dispatch stall,
      startup-refused crash loop, prompt-bar submit cancel, stranded-bound
      control deadlock + rebind, consume-marking, idle-trigger mask,
      channel-mail dead letter, actor-wake created_at drift.
      discovered: Management live-occurrence storm (live-locks repaired;
      convergence invariant unbuilt), guest Dolt journal leak + host
      dead-image accumulation, Node B deploy disk headroom, guest runtime
      deploy gap for constructed computers, CI deploy cancellation by docs
      push.
      introduced: an untracked live data.img edit for the Texture model swap.
  - id: s0m-to-next-transition-2026-10-04
    kind: station_transition
    status: closed
    landed: S0m-record-native-messaging
    next: S1a-host-boundary-hotfix, then S0b (S0 resumes)
    closed_at: '2026-10-04T03:30:00Z'
    panel: >-
      Convergent agentic-consensus panel run 2026-10-03/04
      (.agentic-consensus/agentic-consensus-20261003-224612). Verdicts:
      mixed — 1 land, 4 send_back, 1 narrow-send_back — converging on real
      gaps only: action-2 needed a DISPOSABLE computer (done:
      computer-ca3a2cf9), a4fcdb8d's deploy run ID was missing (resolved:
      37165170516), channel-mail problem doc was stale (fix b18f3baf had
      landed — deployed refusal verified live, trajectory aef9a197), S1a
      ordering/design gaps (slice authored + adopted: wildcard/pair-drop
      rules, tap reconcile, RemoteAddr leg, self-dev mint note,
      stale-deploy guard). Refuted findings: space-bunny's
      MASQUERADE-blackhole claim (0-packet counters + live SRC trace)
      and gemini's paths-ignore proposal (docs pushes must not run any
      deploy steps at all).
    report: >-
      docs/reports/s0m-record-native-station-close-2026-10-04.md
      (reporter, iCloud PDF same title).
    note: >-
      Action-1's precommit->resolve leg: record->packet->wake->consume->
      settle deployed-verified (684ddcb1); mechanical resolve minted by
      system:reducer is unit-covered and ledger-internal — the resolve
      record itself has no owner-visible API surface, so deployed evidence
      shows the resolve's effect (report consumed, work settled) rather
      than the record row. The desk was twice prompted to choir.Ask cold
      and completed with no authoring act (model agency, recorded as an
      accepted edge: the mechanism is proven by the unit suite + the
      ask-acceptance chain; desk-agency at authoring an ask on demand is
      not a transport defect).
  - id: s1a-to-s0b-transition-2026-10-04
    kind: station_transition
    status: closed
    landed: S1a-host-boundary-hotfix (slice of station S1, executed as its
      own boundary per the v3.1 first-slice rule)
    next: S0b-disposable-computer-probes (station S0 resumes)
    closed_at: '2026-10-04T05:55:00Z'
    landed_receipts: >-
      Deployed refusal matrix PASSED —
      docs/evidence/s1a-refusal-matrix-2026-10-04.json (builds a3f0d48e
      boundary + b15f012a oracle + POST legs): R1 tap->tap FORWARD
      timeout, R2 vmctl internal 403, R3 maild forged owner 403, R4
      corpusd mint POST 403, R5 proxy publish POST 403; L1 gateway dial,
      L2 bound-owner CV resolve 200, L3 bound-owner maild 200, L4 egress
      dial, L5 product page 200, L6 bound-guest source-service search.
      Post-fix iptables ruleset + per-tap DROP counters recorded in the
      same evidence file.
    panel: >-
      Convergent agentic-consensus panel run 2026-10-04
      (.agentic-consensus/agentic-consensus-20261004-013636). Verdict:
      send_back (5/10, 4 accept, 1 silent) — resolved in place: the R4/R5
      GET-405 legs carried no evidence (POST is the real exploit path),
      so the oracle gained a bounded method=post gated to a path
      allowlist of exactly the two refusal targets and the legs were
      rerun as POST with forged headers -> 403. Second findings adopted:
      bound-guest source-service leg (:8787) added green, post-fix
      iptables ruleset recorded. Refuted/declined: widening the oracle
      beyond the two POST targets (would violate its no-authority
      invariant on open registration).
    report: >-
      docs/reports/s1a-host-boundary-station-close-2026-10-04.md
      (reporter, iCloud PDF same title).
    residuals: >-
      Egress remains open by design (default-deny internet is deferred to
      the S1/S4 recording-proxy work; the metamission security posture
      keeps open-with-recording). Source-service /internal/* stays
      bound-guest-open by design (researchtools consumes it inside the
      guest). The 'zot' PATH-shadowing heresy (management console spawns
      a third-party TUI; no in-guest exec surface) is carried as
      discovered-unrepaired with repair owner TBD (S1 or hardening).
      The S0b registration-genesis defect
      (docs/problems/s0b-registration-computer-missing-genesis-2026-10-04.md)
      is S0b's first named blocker.
---

# Supervised App Development, Fast Resume, Forks, and Source Publication — Metamission (v2)

## The one object

Every station handles the same record: **base revision + patch stack +
pinned inputs + build recipe + tests + receipts**. It starts as a capsule
experiment. It becomes a committed app-layer release on one computer. It
becomes a publication, a fork's merge-back, an org template's content,
and a candidate for the platform base. Stations must not invent their own
idea of what a change is.

## Two layers

- **Non-forkable base.** One shared NixOS guest image: kernel, systemd,
  capsule broker, updater, signers, kernel probe, network policy.
  Platform-owned. Security boundaries live here wherever possible, so
  fixes here are image pushes that cannot conflict with anything.
- **Forkable app layer.** The runtime, frontend, app modules and skills,
  as a per-computer Nix closure built against the base: base revision
  plus patch stack. Self-dev edits it, publication carries it, forks
  inherit it, and security offers rebase onto it. Updating it is an
  updater swap plus a service restart, not a VM reboot.

## Two kinds of snapshot

- **Machine snapshot**: paired Firecracker memory + VM state + reflinked
  disks. Fast, exact, and tied to one image and hypervisor version.
  Contains live secrets. Restored at most once, by the same computer.
  Used for hibernate/resume.
- **Semantic snapshot**: the existing ComputerVersion checkpoint +
  ProjectionBase. Portable, rebaseable. Used for forks, org templates,
  and anything that leaves the host.

## Stations (intent)

- **S0 reality + boot timeline.** Two phases, classified separately.
  **S0a (read-only):** per-boot timeline receipt (host spawn, kernel,
  initrd, each systemd unit, each runtime boot phase, first healthy) for
  one fresh and one owner-sized computer; guest store/mount layout,
  runtime closure deps, image/runtime identity; tap→tap reachability,
  gateway token visibility, reflink on the VM state path, Firecracker
  snapshot support in the pinned version — **measured snapshot create
  and resume wall time on an owner-sized computer and UFFD lazy-loading
  on the pinned Firecracker/kernel, not feature presence alone**.
  **S0b (scoped disposable-computer experiments, orange):** self-dev Go
  effect, M9a full-bundle payload, capsule netns/userns mount of a
  `/nix/store`-prefixed overlay, whether the base EROFS carries a valid
  Nix DB, a sandboxed `nix build` of one nixpkgs package inside a
  capsule, and a capsule-built runtime with a dep absent from the base
  (dispose → activate → reboot → restore). Every S0b probe runs on a
  disposable computer with recorded pre/post state.
- **S1 security floor.** Per-tap FORWARD isolation, default-deny VM
  egress, a confined non-root runtime unit, gateway token scrubbed from
  child environments, and the Yaegi worker kernel floor. Prerequisite
  for S4 and S9.
- **S2 layering.** Split the guest into base image + app-layer closure.
  The guest executes the runtime from its committed release (image
  baseline as fallback). **Materialization mechanism (panel-required):
  the shared EROFS store cannot hold per-computer paths — the release
  carries its Nix closure unpacked at a GC-rooted path on the guest data
  disk, and the runtime execs from it with the same store-path layout;
  no writable global store or nix daemon is introduced.** Each release
  records the base image it was built against. Add a per-computer guest
  image reference with GC-rooted retention (the bootc switch
  equivalent). The updater refuses a release whose closure does not
  resolve in the booted base, and activation binds executable +
  frontend + state compatibility + effective event head as one
  transaction (stale-head or post-test mutation refuses). Platform
  app-layer pushes (M9a) apply without reboot. Wire the M9a push into CI
  for tracking computers.
  **Builder substrate (panel-found gap):** four downstream stations
  (S6 commit, S8 adopt, S9 fork, S10 install) assume something can
  evaluate a Nix closure; nothing names it. S0b must resolve whether
  the builder is a host-side service, a privileged builder-capsule
  class, or a scoped guest service — and it lands as a dependency of
  S2 itself.
- **S3 fast resume.** Machine-snapshot hibernate/resume with these
  invariants: guest quiesce (sync + fsfreeze after guest health shows no
  active engineMu work — quiesce polls guest state, not just fsfreeze)
  before snapshot so the disk alone is consistent; paired reflinked
  disks whose content generation (btrfs generation or content digest)
  is recorded at snapshot time; one-shot consumption recorded in vmctl
  before resume; exact image/kernel/Firecracker pairing **plus matching
  disk generation, effective release and event head** — a moved disk or
  advanced head invalidates unless explicitly reconciled; otherwise
  discard and cold boot; a post-resume hook (clock step, gateway token
  refresh, RNG reseed, process-local key regeneration, reconnects,
  epoch handling); snapshot files root-only in the computer's state
  directory; snapshot disk usage counted by pressure reclaim. Hibernate
  write time and btrfs space of the memory file join the timeline next
  to resume time. Then trim the cold boot critical path using the S0
  timeline.
- **S4 capsule open world.** A recording egress proxy bridged into each
  capsule netns (URL + content hash + redirect chain + connected address
  of every fetch logged to the trajectory; deny private, loopback,
  link-local, host and tap ranges at connect time and on every redirect;
  supported protocols declared, opaque bypasses denied). Fetched
  payloads captured into content-addressed storage so S8 can rebuild
  with origins unavailable. A capsule-private Nix store: the broker
  owns the netns and drops capabilities before capsule exec; **done
  requires exactly one proven mechanism — a capsule-private writable
  upper or the Nix local-overlay store — selected by the S0b probe.**
  Named falsifiers: the EROFS lower may ship no Nix DB; the capsule
  userns may not mount overlay over `/nix/store`; Nix sandbox nesting
  may fail; closure paths may not resolve after capsule disposal.
  Substitution goes through the proxy.
- **S5 live preview + supervision.** A broker-bridged unix socket from
  the capsule's dev server to a desktop preview route. Capsule commands,
  fetches and tests stream into the Texture work doc. Consumes the
  Texture transclusion work (external dependency).
- **S6 commit gate, full release.** Tests + owner approval (existing M7
  decision) materialize a full app-layer release. Prove backend effect
  after reboot, after resume, and after restore.
- **S7 app packages.** Dynamic app manifest (frontend module + optional
  backend service + manifest + recipe). Packages are the unit of
  divergence and publication.
- **S8 source publication (M9b).** Publish = patch + base revision +
  pinned inputs (S4's fetch log turned into fixed-output hashes) +
  recipe + tests. Host registry. Review, fork, customize, build in own
  capsule, adopt through own gate.
- **S9 forks and fleets.** A fork is a new sibling computer. Two
  construction paths, classified by what it may carry: **(a) reflinked
  disk + cold boot + host-side re-encryption + new genesis with parent
  lineage** — permitted only for a fork authorized to inherit the
  parent's *entire* disk state (same owner, all data classes);
  post-copy scrub is not evidence of exclusion — Dolt history and
  unallocated blocks retain deleted rows. **(b) selective-class or
  cross-owner forks are built from a semantic snapshot via an
  allowlisted export** into a fresh disk — the export contract names
  each carried data class. Both mint a new ComputerID, signer keys,
  privacy key and gateway token before any child runtime or network
  access. Forks are ephemeral or persistent, with fleet admission and
  quotas under the 32 GiB budget. Merge-back happens via S8. The fleet
  view is Texture transclusion of fork sources.
- **S10 org templates.** Export a scrubbed semantic template (base rev +
  patch stack + curated seed data + policy, encrypted at rest). Install
  it as a fresh computer that builds from source.
- **S11 mainline + security push.** Host selection into main. Security
  offers carry severity, a deadline and an exploit test. Each divergent
  computer is evaluated on an ephemeral fork. **The exploit test is a
  regression witness, not an exemption oracle: it must fail on the
  pre-fix build and pass on the post-fix build before the offer is
  valid.** Classification: auto-apply (tracking), rebase-and-apply
  (clean divergent), proposal (divergent), fail-closed at deadline.
  **Exempt requires structural proof of component absence — never a
  passing exploit test alone.** Inconclusive or timeout remains
  proposal. Deadline enforcement is host-side: the host records the
  per-computer disposition and can deny the vulnerable capability even
  when the computer is asleep or cannot rebase; capability removal is
  independently verified on the actual computer.

## Security posture (prerelease, 2026-10-04)

**Update 2026-10-05:**
- S1a landed and was deployed-verified: network isolation plus
  transport-bound internal authority; refusal matrix 5/5 refused, 5/5
  legitimate flows green (`docs/evidence/s1a-refusal-matrix-2026-10-04.json`).
- **Correction:** guest→host traffic keeps its real guest source IP. It
  does not arrive as loopback (`bffad45`). The exposure was the
  caller-controlled headers.
- Still open (S1 remainder; required before S4):
  - non-root runtime;
  - gateway token off the kernel cmdline and out of child environments;
  - Yaegi kernel floor;
  - zot PATH-shadowing;
  - credentialing or removal of the S0a diag surfaces.

The text below is the 2026-10-04 snapshot as written.

Owner direction: record security findings in full. Choir is prerelease,
and considerable hardening follows this metamission. This section lists
what is known now. It is the input to that hardening pass.

**Confirmed on staging (S0a):**
- A guest can open TCP to other guests' runtime port, every
  host-internal service port, and the internet.
- The gateway token is on the guest kernel cmdline and readable inside the
  guest.

**Source-traced, not exercised against another tenant:**
- Guest->host traffic is DNATed to 127.0.0.1 and MASQUERADEd to loopback.
- vmctl (`isInternalCaller`), corpusd (platform-update mint), maild, the
  gateway's internal check, and guest `/internal/runtime/*` all accept
  caller-controlled `X-Internal-Caller`, `X-Authenticated-User` or `Host`
  headers.
- From any account's own guest, that yields:
  - lifecycle control over every computer;
  - cross-tenant reads/writes on mail and guest user routes;
  - frontend injection into tracking computers through a genuinely
    platform-signed update offer.
- Full chain and fix shape:
  `docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md`.

**Known weaker boundaries (audit and S0a):**
- The guest runtime runs as unconfined root.
- Non-capsule shells (terminal, zot) inherit its environment, including
  the gateway token.
- Yaegi session workers have only a package allowlist, with no Landlock,
  seccomp or netns.
- Capsule isolation is per capsule, not per call.
- The S0a instrument's `/internal/diag/tcp-dial` and boot-timeline
  surfaces are header-gated.
- vmctl has a lock-substrate blocks-all class.

**In-metamission repairs:**
- **S1a:** host boundary — network isolation + authority binding.
- **S1:** runtime confinement, token off the cmdline and out of child
  environments, Yaegi kernel floor, removal or credentialing of the S0a
  diag surfaces.
- **S3:** snapshot secrecy and one-shot invariants.
- **S4:** recorded egress.
- **S9:** fork re-keying.

**Deferred to the post-metamission hardening pass (named so it is not
lost):**
- per-tool-call Landlock;
- encryption at rest for snapshots and exports;
- a full review of every host-internal endpoint's authority model;
- signer and credential lifecycle review;
- supply-chain review of the S4 pinned-input capture;
- an external review before public launch.

## World Wire as the consuming application (2026-10-05)

Owner framing: World Wire / autopaper is "the true application of Choir,
the purpose of all this infra". This is not one autopaper but a platform
for others' autopapers, with white-label copies built from the primary.
The rearchitecture analysis is `docs/world-wire-rearchitecture-2026-10-05.md`
(read it with its director notes); the evidence is
`docs/problems/world-wire-corpus-resource-burn-2026-10-05.md`.

How this metamission treats it:
- **Not a station, a validation lens.** At each station boundary, the panel
  also asks whether an autopaper computer could run on what landed. The
  target sketch is a platform "wire-observer" computer plus per-tenant
  autopaper computers plus S10 white-label, and it consumes S2-S11 almost
  one-for-one.
- **Gap named:** cross-computer *data* subscription. Tenant autopapers
  reading a shared public claims graph owned by a platform computer has no
  producing station; S8 publishes code. This is a candidate successor
  station after S8 (intent only; not added to the station list until the
  owner promotes it).
- **Deferred:**
  - processor/reconciler factorization (evidence can come from the frozen
    corpus, read-only);
  - shared-graph placement;
  - corpus migration or archive;
  - the editorial multisupervision location.
- **Containment now, in SO:**
  - sourcecycled durably disabled;
  - og GC wrong-store fix;
  - WW corpus frozen and retained (no deletion, migration or
    compaction);
  - one CPU observation.
- **Corrections recorded:**
  - The event-head CAS does not share corpus-dolt's process: Store A on
    :13306 and Store B on :13307 are split. `docs/computer-ontology.md` was
    corrected.
  - The host already has `dolt` and `jq`.

## Orchestration contract (station boundaries)

This file is the executable `/goal`. Station files are authority-bearing
contracts consumed by the orchestrator; none is slashed separately. On
every station boundary — terminal receipt landed, `now.slice` about to
advance — the orchestrator MUST run, in order:

1. **agentic-consensus** (`skills/agentic-consensus/SKILL.md`,
   `agentic-consensus-runner.sh`): convergent panel on the landing
   station's receipts + the next station's readiness gate ("is the
   landed evidence sufficient, and does the next station's file still
   match reality?"). Send-back findings resolve before promotion.
2. **reporter** (`skills/reporter/SKILL.md`): publish the station's
   human-readable narrative report + evidence refs to the repo.
3. **Station transition receipt** on the spine's `receipts:` block:
   landed station id + terminal receipt ref, next station id +
   promoted `now.slice`, panel verdict digest, report ref. Only then
   may the next station's `now.status` move to `working`.

Between-boundary continuous authority: within a station, the station
file's own `now` card governs; the spine does not re-review mid-station.
A station whose evidence falsifies its conjecture stops at
`blocked_incomplete` and returns to this boundary protocol rather than
self-promoting a successor.

## Latency plan (S0, S2, S3)

1. **Measure before cutting.** No optimization lands without the S0
   timeline attributing the time.
2. **Stop rebooting for updates.** Layering (S2) turns most deploys into
   updater swap + runtime restart. Machine snapshots survive an
   app-layer update only when the effective release and paired disk
   generation are unchanged — an app-layer swap that mutates runtime
   state invalidates the snapshot unless explicitly reconciled; the
   base image staying put is necessary but not sufficient.
3. **Resume instead of boot.** Machine snapshots with lazy memory loading
   target sub-second to low-second wakes, under the S3 invariants.
   Shrink snapshots by releasing guest page cache only if S0 shows the
   write cost matters; warm caches are part of what makes a resumed
   computer fast.
4. **Trim the cold path** with evidence. Likely candidates to check:
   network-online waits, the serialized signer, probe, updater and
   runtime chain, runtime boot phases that are not needed to serve, and
   initrd modules. Cold boot remains the fallback after image changes
   and failures.

## Open decisions (record, do not block)

1. Fork construction: resolved by panel 2026-10-01 — two paths, not a
   choice. Reflink + cold boot + host-side re-encryption + new genesis
   is whole-disk same-owner only; selective-class or cross-owner forks
   are semantic-snapshot reconstructions via an allowlisted export.
   See S9.
2. Default capsule egress: open-with-recording + never-private-ranges
   (leaning) vs allowlist.
3. Trusted host build cache keyed by derivation hash (deferred).
4. Whether hibernate snapshots are retained after resume for a fast
   re-hibernate via diff snapshots, or always discarded (start with
   discard).
5. Snapshot encryption at rest on the host (needed before any export;
   for on-host hibernate, root-only permissions match data.img today).

## Orientation 2026-10-05 (director review, v4)

This folds in the 2026-10-04 addendum. Pattern of work: the owner runs
`/goal` on this file in a local agent. A remote director session reviews
the commits between runs and revises this file. Station files govern
inside a station; this section and the `now` card govern ordering.

### Where we are

- **Closed:** S0 (S0a boot attribution + S0b probe suite), S0m, S1a.
- **Live:** S2, with the S1 remainder and SO in parallel.
- **S2 mechanism is proven on staging:**
  - signed layered offer;
  - CAS-ref transport for large payloads;
  - nar replay into a GC-rooted private store;
  - overlay exec over the read-only base store;
  - route promotion;
  - base-mismatch refusal.

  The mid-S2 scare (the guest "falls back to base" or crash-loops) resolved
  into one probe artifact plus two real contract gaps.

### S2 re-scope (remaining slices, in order)

1. **S2-e Rollback atomicity.**
   - Defect: `layering-entrypoint` is a global file at the updater root
     (`internal/updater/closure.go:418,457-462`), outside the release dir.
     `restorePrior` swaps `current` but not the entrypoint, so recovery
     re-execs the failed binary.
   - Fix: put the entrypoint inside the release (for example
     `current/layering-entrypoint`) and have the wrapper in
     `nix/autoputer-vm.nix` read it through `current`. One pointer swap
     then moves frontend and exec together. That is the single-state-
     authority answer.
   - Add a boot-loop guard: after an apply, N failed starts route to the
     recovery path and leave a receipt.
2. **S2-d State-compat gate.**
   - The manifest already carries `EventSchemaVersion`/`ReducerVersion`.
   - Add the minimum persistent-store schema the release accepts, and the
     base commit it was built against. Refuse in `Apply` before mutation.
   - The health probe stays as the post-restart backstop, not the gate.
3. **S2-c Provenance.**
   - The builder derives `code_commit` from the flake ref it evaluates,
     instead of accepting it from the caller (`internal/builder/request.go`).
   - The manifest carries the builder receipt digest.
   - The updater verifies that the release binary's embedded `buildinfo`
     commit equals the manifest.
   - Acceptance releases must be builder output. No hand-staged nars.
4. **S2-f Self-dev → builder join.**
   - Self-dev freeze currently yields a file release from the capsule's
     `var/lib/artifact/release`.
   - For runtime changes it must yield a source patch (base revision +
     diff) that the host builder builds. That is source-only, and it is
     the shape S6 and S8 consume.
   - Frontend-only file releases may remain as the narrow path.
5. **S2-g CI wiring.**
   - On deploy: builder → mint → push for tracking computers; app-layer-only
     changes do not reboot VMs; record time-to-healthy.
   - This closes `s0m-guest-runtime-deploy-gap` and
     `guest-release-propagation-manual`. Canary and constructed computers
     follow their declared policy.

### SO — ops substrate (new parallel station; gates S3)

Root Cause Clustering applies: five disk-headroom incidents since 10-01,
guest stderr unreachable, untracked VM shape drift, and CI gating that
strands runtime code. One substrate: no storage lifecycle and no durable
guest observability.

- **Storage lifecycle:**
  - reclaim dead `vm-state` dirs (no live unit), `.corrupt`/`.pre-*` and
    quarantine remnants;
  - a retention/refcount GC for `platform-artifacts` (layered nars, offers,
    publications) keyed by live route slots and retained predecessors;
  - corpus-dolt growth handed to the capacity mission with a number;
  - a headroom budget that includes future snapshot files.
- **Guest observability:** a durable host-side sink for guest console and
  journal (Firecracker serial to a per-VM rotated log, or journal
  forwarding) so exec, boot and apply failures are host-visible without
  bespoke diag files. `layering-diag.log` becomes a special case.
- **Guest memory budget (owner direction 2026-10-05: "shouldn't need 16gb
  at all"; target 2-4 GiB, oversubscribed):**
  - **Diagnosis.** The owner guest went 2 → 4 → 8 → 16 GiB, each step
    justified by "the store outgrew the guest"
    (`internal/vmctl/ownership.go:1220-1224`,
    `guest-vm-embedded-dolt-memory-starved`). On 10-03 the live store was
    about 5 GiB and the noms journal held about 22 GiB of garbage
    (`guest-dolt-journal-and-host-image-leak`), because
    `store.MaybeRunDoltGC` returns at the live-size guard
    (`internal/store/dolt_maintenance.go:290`) before the journal trigger
    (`:309`). Once live exceeds 5 GiB, the guest never GCs again.
  - **Hypotheses to measure, not assume.** Guest memory demand tracks
    journal and store size rather than the hot working set, because of:
    (a) Dolt journal/index state proportional to the journal;
    (b) O(store) scans at boot (reconstruct takes 15.6 s with
    `applied_rows=0`) and in request paths (`scan object`);
    (c) guest page cache filling whatever ceiling it is given;
    (d) an unbounded Go heap (no `GOMEMLIMIT`).
  - **Measure.** Add a memory receipt (guest `/proc/meminfo`, Go runtime
    metrics, Dolt cache/journal sizes) and the host-side committed RSS per
    Firecracker process. Record before and after a host offline GC of the
    owner store. This is the decisive observation: if memory falls with
    the journal, the 16 GiB was paying for garbage.
  - **Shrink the working set (root cause).**
    - Fix the GC ordering: routine journal GC must be reachable. Above the
      in-guest safety threshold, schedule host-side offline GC
      automatically rather than via a manual runbook.
    - Bound Dolt caches and set `GOMEMLIMIT`.
    - Remove O(store) scans from boot and request paths with indexes, or
      cursors from the last applied point.
    - Longer-term: move cold Texture revision history out of the guest's
      embedded store into content-addressed blobs.
    - S2's host builder already removes compile spikes from guests.
  - **Elastic memory and oversubscription.**
    - The guest shape becomes a ceiling, not a reservation (target 4 GiB
      for interactive, 2 GiB for fresh/disposable).
    - Each Firecracker VM runs in its own cgroup: `MemoryHigh` as the soft
      target, `MemoryMax` as the ceiling.
    - Use the Firecracker balloon (deflate-on-OOM; free-page reporting if
      v1.15.1 supports it — verify) so idle guests return pages to the host.
    - Use host zswap over the existing swap device so cold guest pages
      compress instead of OOMing.
    - Admission: the sum of ceilings may exceed host RAM by a declared
      oversubscription ratio, with pressure reclaim choosing hibernation
      (S3) over OOM.
  - **Declared shapes.** Remove `VM_INTERACTIVE_MEM_MIB=16384` from the
    mutable `vmctl-priority.env`. That file is declared for priority IDs
    only, so the value is untracked drift. Shapes live in tracked config,
    and the owner computer comes down to the 4 GiB ceiling once the GC fix
    and measurement show it fits.
  - **Host side.** corpus-dolt is capped at 18-20 GiB on a 32 GiB host and
    is the largest single consumer. Hand it a number through the capacity
    mission. Guests cannot be dense while one host database takes over
    half the RAM.
- **CI deploy gating:** compute deploy need against the deployed identity,
  not the push delta, so a docs-only head never strands runtime code
  (`ci-docs-only-head-strands-runtime-deploy`).

### S3 adjustment — residency tiers ("feels always on")

S3 is framed as residency tiers rather than hibernate alone. The goal is
that every computer behaves as always on without being resident:
- **Hot:** running, serving, with runs active.
- **Warm-idle:** running and ballooned down to a small floor (~0.5-1 GiB),
  for instant response at small cost.
- **Cold:** a machine snapshot on disk, resumed in about 1 s.

Waking is driven by triggers, not residency:
- the proxy holds an HTTP request while the computer resumes;
- a host-side scheduler (the DSec Watcher pattern) wakes computers for due
  timers, mail arrival and agent continuations.

Smaller, ballooned guests make snapshots smaller and resumes faster. SO's
memory work and S3 are one lever pulled from both ends.


S0b could not measure snapshots: there is no vmctl snapshot surface and the
guest is sealed. S3's first slice is therefore the measurement:
- a disposable-only vmctl snapshot create/load path;
- wall time for create and for resume-to-healthy, at 8 and 16 GiB;
- UFFD versus file-backed loading;
- snapshot file size after btrfs compression.

The S3 invariants stand. S3 waits on SO's storage lifecycle and the shape
decision.

### Evidence quality rule (new)

Mechanism probes (does exec work in the overlay?) and contract probes (does
a real release from a real commit apply, refuse when incompatible, and roll
back atomically?) are different evidence classes. Hand-staged artifacts
may answer mechanism questions only. Station acceptance requires contract
probes.

### Carried residuals

- `s0b-registration-computer-missing-genesis`.
- `s0-m9a-route-projection-owner-binding`: likely resolved by `69983b0e`;
  close the record with the S2 route-promotion receipt.
- zot PATH-shadowing (S1 remainder).
- Management storm convergence invariant.
- Guest Dolt journal leak (capacity mission).
- `s2-layered-offer-transport-cap`: CAS-ref landed; close or re-scope the
  record.

### Superseded addendum (2026-10-04) — summary

The overlay-exec question is settled: it works. Its four substrate issues
are now owned:
- guest stderr → SO;
- disk recurrence → SO;
- the S4 overlay falsifier → S4 keeps the private-store choice, informed
  by the guest result that overlay works in the guest service context;
- the silent apply-restart wedge → S2-e.

### Handoff 2026-10-05 — S2 contract slices

State for the next session: S2 mechanism is proven on staging; the work is
the release *contract*, not the mount machinery. The layering scare was
resolved into two non-bugs and one real defect:

- `unshare`/`mount -t overlay`/`exec` all succeed in the guest's systemd
  private mount namespace (`layering-diag.log`: `unshare_ok` → `mount_ok`
  → `exec_path_resolves`). The mechanism is sound.
- The `vm-3dc68688` crash-loop was the probe's own hand-staged binary —
  an August autoputer (`43310064`) copied into `/tmp/closure.nar` on
  Node B — running against an October persistent store. Builder output was
  never exercised. Per the v4 evidence-quality rule, hand-staged artifacts
  answer mechanism questions only; station acceptance requires
  builder-produced releases.
- The real defect: **`layering-entrypoint` is a global file** at
  `$CHOIR_UPDATER_ROOT/layering-entrypoint`
  (`internal/updater/closure.go:418,457-462`), outside the release dir and
  outside the `current/` swap. `restorePrior` reverts `current` but not the
  entrypoint, so a recovery restart re-execs the failed release. This is the
  single-state-authority violation S2-e closes.

Deployed state (`main@90d93a82`, staging `choir.news`):
- `e605cdde` — `autoputerRuntimeExec` falls back to direct-exec when the
  overlay path fails, and points `CHOIR_BASELINE_RELEASE_ROOT`/skills at the
  applied layerdir. Keep or revert once `current/layering-entrypoint`
  becomes the single pointer; it is a compatibility shim, not the contract.
- `3c1cbaf6` — the `layering-diag.log` stage recorder that produced the
  `unshare_ok`/`mount_ok`/`exec_path_resolves` evidence. Leave in place;
  SO's durable-stderr work generalizes it.

S2 slice order (v4): S2-e rollback atomicity → S2-d state-compat gate →
S2-c provenance → S2-f builder join → S2-g CI wiring. Parallel: SO ops
substrate (storage lifecycle, guest stderr, declared VM shapes, CI gating)
gates S3; the S1 remainder continues.

S2 contract slices **all landed 2026-10-05** (`9f5aa8a0` `c8fb7834`
`8f06b3d9` `e1c3924b` `e527c169`, + `a3da83c4` verifier bind). The S2-g
app-layer push is live in `ci.yml` (`deploy_app_layer=true` when autoputer
is the only runtime dep) — three sequential bind gates surfaced on the
first runs and were fixed/proven end-to-end on
`computer-6450a253b8b6ebc0866471973694f5be` (release `36743b1f6ddb`,
`build.commit 2f0e2cac`, atomic `current/` swap, entrypoint → release
binary). Remaining S2-g residual gates are in
`docs/problems/s2-app-layer-offer-bind-gaps-2026-10-05.md`. The S2 station
is now at acceptance-test stage (conjecture test vs builder-produced
releases).

