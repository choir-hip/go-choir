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
      path: docs/definitions/choir-appdev-s1-security-floor-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S0-reality-and-boot-timeline, S1a-host-boundary-hotfix]
    - id: S2-layering-runtime-from-release
      path: docs/definitions/choir-appdev-s2-layering-runtime-from-release-2026-10-01.md
      readiness: reviewed
      status: working
      depends_on: [S0-reality-and-boot-timeline]
    - id: S3-fast-resume
      path: docs/definitions/choir-appdev-s3-fast-resume-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S1-security-floor, S2-layering-runtime-from-release]
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
    S0 COMPLETE 2026-10-04 (s0b-boundary-close receipt): boot attribution
    and disposable-computer probes both closed with named edges into S2 —
    (1) selfdev executing->frozen wedge must be repaired before any Go
    effect can execute, (2) capsule-namespace probe is the precondition
    for the privileged-builder-capsule substrate branch, (3) snapshot/
    UFFD surface is host-level and belongs to S3, not guest probes.
    S1a host-boundary hotfix closed before it (deployed refusal matrix
    PASSED, 5/5 refusals + 5/5 legitimate flows). Next: S2
    builder-substrate selection + landing.
  source_ref: main@ea4b35cd
  deploy_identity: 'staging https://choir.news deployed_commit=e87f3294 (S1a boundary + POST legs + prebind-flake fix live)'
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
      snapshots and (since S0m) record-native desk couplings carry it.
      What is missing is effect reach, openness, speed, and a host
      boundary that actually holds:
      (a) the guest executes a per-computer app-layer closure, not the
      image baseline;
      (b) capsules get recorded egress and a private Nix store;
      (c) a preview bridge;
      (d) the change record is base rev + patch stack + pinned inputs;
      (e) machine snapshots for resume, semantic snapshots for
      distribution;
      (f) guest traffic carries no host-internal authority (new
      2026-10-04).
      Layering (a) is also the largest latency lever, because most
      updates stop rebooting VMs.
    test: >-
      S0b confirms (a) on a disposable computer (a self-dev Go change
      applies but the endpoint does not change), selects or falsifies the
      S2 builder substrate, and measures snapshot create/resume on an
      owner-sized fixture. S1a's deployed refusals confirm (f). Each later
      station is falsified if it needs a new authority path rather than
      a new effect path.
    edge: missing_oracle
    delta_o: >-
      Landed in S0a: a per-boot timeline receipt (host marks + guest
      systemd + runtime phases + first healthy). Still needed: S0b's
      runtime-effect probe, snapshot/UFFD measurement, and S1a's
      guest-origin refusal receipts.
    scope_if_supported: >-
      Single-host Choir Community Cloud staging (32 GiB Node B), Go
      runtime + Svelte frontend + app backends, Firecracker v1.15.1.
    status: testing
    evidence_refs:
      - docs/evidence/m11-probe-run9-satisfied-2026-09-29.json
      - docs/reports/nixos-agent-platform-redhat-deepseek-audit-2026-10-01.md
      - docs/problems/guest-release-propagation-manual-2026-10-01.md
      - docs/reports/choir-rlm-restore-zero-snapshotting-correction-2026-09-09.md
      - docs/reports/s0a-boot-timeline-landing-2026-10-01.md
      - docs/evidence/s0a-boot-timeline-fresh-2026-10-01.json
      - docs/evidence/s0a-boot-timeline-owner-sized-post-refresh-2026-10-01.json
      - docs/evidence/s0a-tap-reachability-2026-10-01.json
      - docs/evidence/s0a-gateway-token-visibility-2026-10-01.json
      - docs/evidence/s0m-ask-acceptance-2026-10-04.json
      - docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md
      - docs/problems/s0m-guest-runtime-deploy-gap-2026-10-02.md
  decision:
    what: >-
      Two-layer guest (shared non-forkable NixOS base; per-computer
      app-layer Nix closure built against it). Source-only publication.
      Machine snapshots for resume, semantic snapshots for fork and
      distribution. Forks are sibling computers. 2026-10-04: S1a
      host-boundary hotfix runs before S0b (operational ordering under
      continuous authority; security precedes further probing on a
      deployment with open registration).
    kind: architecture
    status: settled
    evidence_ref: owner statements in session 2026-10-01; owner 2026-10-04 "update the metamission ... don't exclude the security relevant info"
    owner_ratification_ref: >-
      owner 2026-10-01: "I like your suggestion for layering, and using
      guest NixOS closures appropriately"; "permission granted" (forks);
      "I ratify #1"; source only. 2026-10-04: prerelease; considerable
      hardening work follows this metamission.
  belief:
    believed_state: >-
      CONTROL PLANE: self-dev governance works (M11). Desk couplings are
      record-native (S0m): every addressed act is a commitment record whose
      Addressee is the delivery instruction; ask->report->resolve is
      deployed-verified (trajectory 684ddcb1).
      EFFECT PLANE: still stops at the frontend. The guest runtime execs
      from the image's Nix store. Deploys skip constructed-computer-version
      guests, so their runtime lags main until a manual refresh
      (s0m-guest-runtime-deploy-gap). The Texture desk model was changed by
      a manual live data.img edit (docs/evidence/s0m-texture-model-swap-2026-10-02.md):
      untracked state drift that S2 must turn into a release.
      BOOT (S0a, measured): fresh cold boot 8.6 s to first healthy. That is
      host data-image creation ~1.1 s, guest kernel+initrd+systemd ~5.9 s
      (initrd 2.8 s, userspace 3.0 s; network-online is immediate because
      wait-online is masked), and runtime ~1.1 s. The owner-sized refresh
      is 26.6 s: the same ~6 s OS, store open 3.2 s, reconstruct 15.6 s with
      applied_rows=0, vocab fence 0.5 s. Reconstruct-with-nothing-to-apply
      is the largest cold-path target; initrd is next. The 662.7 s
      vocab-rescan boot is repaired.
      SHAPES: the fresh computer booted with 16384 MiB while the owner
      computer runs 4096 MiB; node-b.nix declares 4096 default / 8192
      interactive. The source of 16384 is unexplained and matters for S3
      snapshot size and the 32 GiB budget.
      SECURITY (prerelease, recorded in full by owner direction):
      - guest->other guest, guest->host internal ports, and guest->internet
        are all open (confirmed).
      - Guest->host traffic arrives as loopback. vmctl, corpusd, maild, the
        gateway and guest /internal/runtime/* accept caller-controlled
        X-Internal-Caller / X-Authenticated-User / Host headers
        (source-traced).
      - Hence any account holder can, from source, control any computer's
        lifecycle, read other tenants' mail and guest routes, and inject a
        frontend into tracking computers via a genuinely platform-signed
        offer. The owner computer is canary and refuses offers.
      - The gateway token rides the guest kernel cmdline.
      - The guest runtime is unconfined root.
      - Yaegi workers have no kernel floor.
      - vmctl's lock substrate has a blocks-all class (10 siblings, S0a
        panel).
      OPS: a docs push cancels an in-flight CI deploy (ci.yml concurrency
      group ci-${{ github.ref }}, cancel-in-progress: true; confirmed
      2026-10-02, fix reached staging only by manual force deploy).
      Node B deploy headroom is 103G vs a ~90G floor, reached by deleting
      the 09-18 rollback ref. Guest Dolt journal leak and host dead-image
      accumulation remain open (capacity mission). The Management
      live-occurrence storm's live-locks are repaired, but its convergence
      invariant (O(1) resolve + durable dispatch gate) is unbuilt.
    main_uncertainty: >-
      (1) Whether the S1a fix can refuse guest-origin authority without
      breaking the legitimate guest->host flows (gateway inference, maild
      drafts, corpusd event CAS, wire publish, source service), which today
      ride the same header/loopback path. (2) Whether an owner-sized
      machine snapshot creates and resumes within budget with UFFD lazy
      loading on Firecracker v1.15.1. (3) Which builder substrate
      (host-side service, privileged builder capsule, or scoped guest
      service) S2 uses.
    next_observation: >-
      Two disposable accounts on staging: log RemoteAddr on one host
      service for a guest-originated request (confirms the loopback leg),
      then the S1a refusal matrix pre/post fix. Then the S0b probe suite.
  blocker_or_risk: >-
    SECURITY: the cross-tenant authority chain is live on a deployment with
    open registration until S1a lands. Treat S1a as the next code change.
    No S0b probe or later station may assume tenant isolation before it.
    PROCESS: the S0m boundary protocol is incomplete. CI cancels deploys on
    any docs push, which collides with Problem-Documentation-First on every
    red fix. S4 (open-world capsule) and S9 (forks) must not land before
    S1. A machine snapshot restored against a moved disk corrupts the
    filesystem, and a double resume duplicates RNG and key state. S3's
    invariants are the safety case, not optional polish.
  next_action: >-
    1. S2 builder-substrate: select between the scoped-guest-service
       (weakened by S0b), host-service, and privileged-builder-capsule
       branches; land the selected substrate; then the layering slice
       (per-computer app-layer Nix closure) begins. The S0 wedge
       (docs/problems/s0-selfdev-executing-wedge-2026-10-04.md) and the
       capsule-namespace probe are the first named obligations.
    2. S1 remainder (runtime identity, token scrubbing, capsule/yaegi
       floor) runs in parallel or before S2's layering per the S1 station
       file.
    3. The 'zot' PATH-shadowing heresy and the M9a route-projection
       owner-binding defect are carried named residuals on the S1/S0
       records.
receipts:
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
