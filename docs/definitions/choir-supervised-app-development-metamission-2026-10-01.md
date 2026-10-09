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
# v5 2026-10-05 (owner-approved outline): governing principle "density before
# distribution" — scale agents horizontally inside a 2-4 GiB guest before
# scaling guests per host, before hosts. New stations SR (desk-surface cleanup;
# absorbs Jev M0a + processor/reconciler deletion), SA (agent density: memory +
# Dolt engine bottleneck + store split; absorbs SO's memory slice), SM (model
# policy rewrite + evals; absorbs Jev M2), SC (desk capability surface).
# S3 now depends on SA, S5 on SC. World Wire: consuming lens; phase 1 can start
# after S5; the 10-01 WW metamission is superseded pending a rewrite. See
# "v5 plan".
# v5.1 2026-10-06 (director review): S2, SR closed; SMG landed with its
# behavioral legs blocked. The Management storm (4th recurrence; survives
# restart because the wake-outbox migration re-arms open August obligations
# on every boot) moves from SA slice 4 to SA slice 1, ahead of the baseline.
# Registration genesis fix becomes SA slice 0. SMG legs run on a disposable
# with the bootstrap-chain preamble, never by hand-draining the owner guest.
# CAS durability + 161k lost corpus bodies recorded under SO. See
# "v5.1 revision".
# v5.2 2026-10-07 (addendum only, not a revision): SA slice 1 drain fixes
# landed (91e9c03b rewarm, 29817fda+5eb63161 gateway decode retry, 7a36713c
# drain-carrier validator relax). Management-open race fixed e9cd9fed —
# lifecycle_work_assigned wake fires reconcileAgentWakeLocked before
# EnsureTextureHandoff's own submit; loser now recovers the already-committed
# run instead of 500ing. See "v5.2 addendum" in now.slice.
# v6 2026-10-09 (owner-ratified roadmap): three gates with exit tests replace
# the long station chain as the spine. Gate 1 stable computer (SH O21, SL O1,
# O7/O12 enforceable checks, O8/O9, Texture contract + acceptance); Gate 2
# self-development with live Texture supervision (S1 remainder -> S4 -> S5 ->
# S6; check how much of SC S5 truly needs); then World Wire design + build.
# Optimizations deferred (S3, SA beyond the storm). Post-goal stations moved
# out of the spine (S7-S11). World Wire interface design pulled forward as
# station SW (design only) so it cannot force late changes on Gates 1-2.
# Owner decisions 2026-10-09: World Wire runs on the four core desks only
# (processor/reconciler deleted); shared data between computers uses the
# object graph (published objects adopted onto the subscriber's tape, never
# Dolt branch sync); no architectural ceiling on ingest — budget is the
# constraint, cost per item is a management dial; corpus (Store B) torn down.
# Registry drift fixed: S2 and SMG closed per their now cards. See "v6 plan".
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
      status: complete  # closed 2026-10-05 (now card v5.1)
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
    - id: SR-desk-surface-cleanup
      # v5. Completes Jev M0a (research RLM cutover: registry -> {desk_go_eval},
      # prompt rewrite) and deletes processor/reconciler end to end. Small.
      # CLOSED 2026-10-06 (panel r7 7-0; deployed 12d3adc0).
      path: docs/definitions/choir-appdev-sr-desk-surface-cleanup-2026-10-05.md
      readiness: reviewed
      status: closed
      depends_on: [S2-layering-runtime-from-release]
    - id: SMG-management-rlm-cutover
      # v5 addendum 2026-10-06 (owner directive): SR closed the research desk;
      # the persistent-management desk still carries typed JSON tools
      # (report_to_texture, cancel_co_super_assignment) beside desk_go_eval.
      # "Get rid of all adhoc JSON tool calls everywhere; RLM-only prompts."
      # Ordered before SM/SC: SM evals and SC desk capability assume the
      # sealed-cell carrier on all desks. Problem doc:
      # docs/problems/management-typed-json-tools-remain-2026-10-06.md
      path: docs/definitions/choir-appdev-smg-management-rlm-cutover-2026-10-06.md
      # Cutover DEPLOYED 2026-10-06 (1b1d9b7e + 4bedf999): management
      # registry = {desk_go_eval} incl. product_api_request -> choir.ProductAPI;
      # deployed schema leg PASSED. Behavioral legs blocked by the
      # management occurrence storm on the owner guest — see station now card.
      readiness: reviewed
      status: complete  # closed 2026-10-06 (now card v5.1)
      depends_on: [SR-desk-surface-cleanup]
    - id: SA-agent-density
      # v5. "Scale agents horizontally in a 2-4 GiB VM before scaling VMs."
      # Memory budget (moved from SO) + embedded-Dolt engine bottleneck +
      # operational/versioned store split. Gates S3 and SC.
      path: docs/definitions/choir-appdev-sa-agent-density-2026-10-05.md
      readiness: drafted
      status: pending
      depends_on: [SR-desk-surface-cleanup, SO-ops-substrate]
    - id: SL-obligation-terminality
      # 2026-10-08 owner direction ("proceed with mission next"): enforce
      # operational invariant O1 (docs/operational-invariants-register-2026-10-08.md).
      # Absorbs SA slice 1 (Management storm convergence is an O1 instance).
      path: docs/definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md
      readiness: drafted
      status: working
      depends_on: [SMG-management-rlm-cutover]
    - id: SH-state-homes
      # 2026-10-08 owner direction ("good policy and good ideas. Let's do it"):
      # enforce operational invariant O21 (a realization holds no unique
      # state). Escrow-to-realization privacy key delivery owner-ratified.
      # Prerequisite for the desktop app's hosted<->local move.
      path: docs/definitions/choir-appdev-sh-state-homes-2026-10-08.md
      readiness: drafted
      status: working
      depends_on: []
    - id: SW-world-wire-interface
      # v6 2026-10-09: World Wire interface design, pulled forward (design
      # only). Output: constraints on Gates 1-2 — schedule/timer obligation
      # kinds for SL, retention class + adopted-foreign-object homes for SH,
      # cross-computer object adoption (authority, privacy class,
      # revocation, merge) on the object graph + tape, editorial publish as
      # an owner-supervised Texture action for S5/S6, four-desk mapping.
      path: unauthored
      readiness: intent
      status: pending
      depends_on: []
    - id: SP-production-infrastructure
      # v6 2026-10-09: DEFERRED until Gates 1-2 pass human QA (owner). Second
      # host as off-host durable homes, self-hosted logs/metrics/alerts over a
      # WireGuard mesh, zero-downtime deploys, failover, cold audit archive
      # (O22). Documented so the design is not lost; not executable.
      path: docs/definitions/choir-appdev-sp-production-infrastructure-2026-10-09.md
      readiness: intent
      status: deferred
      depends_on: [SH-state-homes]
    - id: SM-model-policy-and-evals
      # v5. Absorbs Jev M2 (choir-signal-model-policy-rlm-module-2026-09-29.md).
      # Jev M3 (research hill-climb) is its first consumer.
      path: unauthored (scope in "v5 plan" -> SM)
      readiness: intent
      status: pending
      depends_on: [SMG-management-rlm-cutover]
    - id: SC-desk-capability-surface
      # v5. Per-desk package sets + Graph/Ledger/Similar/Source read verbs +
      # management scorer fan-out via SM per-activation selection.
      path: unauthored (scope in "v5 plan" -> SC)
      status: pending
      depends_on: [SA-agent-density, SM-model-policy-and-evals, SMG-management-rlm-cutover]
    - id: S3-fast-resume
      path: docs/definitions/choir-appdev-s3-fast-resume-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S1-security-floor, S2-layering-runtime-from-release, SO-ops-substrate, SA-agent-density]
    - id: S4-capsule-open-world
      path: docs/definitions/choir-appdev-s4-capsule-open-world-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S1-security-floor]
    - id: S5-live-preview-supervision
      path: docs/definitions/choir-appdev-s5-live-preview-supervision-2026-10-01.md
      readiness: reviewed
      status: pending
      depends_on: [S4-capsule-open-world, SC-desk-capability-surface]
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
    - Bulk World Wire scale-out across VMs (v5; belongs to the World Wire metamission after S9).
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
    v5.1 (2026-10-06 director review): Closed since v5: S2 (10-05), SR
    (10-06, panel r7 7-0). **SMG CLOSED 2026-10-06**: legs 1-3 passed on
    a fresh disposable per the director path (computer-0ca7656f, build
    475902d7, docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json)
    — management_open via POST /api/texture/management-open (canonical
    IssueLifecycleControl), control_queued signal,
    co_super_capsule_disposition_set + co_super_assignment_cancelled,
    unbound refusal evidence, bound producer report
    (update_delivered result:sha256:6f0d599a →
    mgmt-work-smg-rlm-open-smg-dispo5-1791328145); probe exit 0. Deployed
    schema leg passed on computer-03335285 (tools exactly [desk_go_eval]).
    Named edge into SA: re-run legs on the owner computer after slice 1
    (owner-guest storm is SA's domain). Two NEW substrate defects
    discovered en route and handed to SA slice 1's defect field:
    sa-delegated-report-poisons-management-listing-2026-10-06.md (a
    delegated-cast producer report fails the delivered-page ProducerReport
    validation arm → every subsequent management activation dies ~30s
    post-bind in a burn loop, 4 observed failures +34/+34/+28/+35s,
    cycle stalled when the bound report discharged the work item) and
    sa-management-mint-no-start-slot-deadlock-2026-10-06.md (fixed
    01199fb1).
    SA SLICE 0 DEPLOYED 2026-10-06 (9f6f369c, CI green): fresh computers
    mint genesis_imported in-guest before the replay gate opens;
    bootstrap-chain is repair-path-only; pre-genesis writes get a clean
    503. Deployed proof: fresh registration accepted first prompt 202
    (computer-07b582d5). Residual exposed behind it: first ~2 submits
    post-boot race the outbox/replay burst → 500 'replace durable
    activation: lifecycle invalid transition' (transient, CAS class) —
    feeds slice 1.
    SA SLICE 0(a) FIXED 2026-10-07 (ca8c8c18): delegated-cast producer
    reports no longer poison the consuming run's delivered-page listing
    — the binding check accepts the consumer-side control binding OR the
    delegated work-item lineage join. Test fails-before/passes-after;
    store suite green. Deployed-verified on vm-48bc0981.
    APPLY-FENCE FOLLOW-ON FIXED 2026-10-07 (19d7913e): the owner canary's
    apply of ca8c8c18 rolled back on updater health-probe 503 during the
    546k-event replay — the 30-attempt fence was shorter than
    boot-to-healthy on a large store. HTTPHealthProber now treats an
    advancing replaying 503 (committed_sequence/progress) as liveness and
    resets the stall budget; MaxDuration=15m is the absolute bound. The
    commit-bound push gate (68397ae7) also landed: a rolled-back computer
    no longer reports "healthy". Owner runs 19d7913e via guest-image
    deploy + active-VM refresh; an app-layer acceptance apply is still
    owed (19d7913e itself is base-image-only).
    SA SLICE 1 STORM CONVERGENCE (2026-10-07): wake mint ceased on
    19d7913e owner boot (01:20:27); Management dd52c39d in serial drain
    of 14-run delivered backlog; management-open control defers FIFO.
    Fixes landed: drain-carrier rewarm (91e9c03b, drop request_source
    gate), gatewayruntime Call JSON-decode retry (29817fda + host
    5eb63161), drain-carrier report validator relaxation (7a36713c).
    Drain-carrier kill residuals: transient gateway decode + host-side
    health-check kill reset mid-drain — reattach fix pending.
    SMG MANAGEMENT-OPEN RACE FIXED 2026-10-07 (e9cd9fed): the
    lifecycle_work_assigned wake (minted by StartLifecycle in the same
    batch) fires reconcileAgentWakeLocked → submitTextureAgentRevisionRun
    → ReplaceLifecycleActivation BEFORE EnsureTextureHandoff's own submit
    lands. The wake's run wins; the handoff's run-B hits
    previous-active-run gate → ErrLifecycleInvalidTransition → 500.
    Fix: submitTextureAgentRevisionRun calls recoverRacedTextureActivation
    on ErrLifecycleInvalidTransition; if agent.ActiveRunID is set and
    live, returns that run idempotently. Problem doc:
    smg-management-open-invalid-transition-2026-10-07.md. Deployed proof
    pending CI on e9cd9fed.
    Critical path: SA slice 1 (storm convergence), then the SA baseline.
    See "v5.1 revision".
    before distribution. Closed: S0, S0m, S1a, **S2** (2026-10-05, terminal
    receipt s2-station-terminal-2026-10-05; consensus round 2 6 approve /
    1 send-back, sole send-back receipt-completion only and discharged).
    S2 leaves the (B,U,R) transition contract, builder-produced closures,
    CI-driven app-layer pushes, and the pending-transition discharge path
    hardened for every downstream station. In flight: S1 remainder
    (non-root runtime), SO (storage lifecycle + og wrong-store GC fix,
    observability, CI gating, World Wire containment). Next substrate: SR
    desk-surface cleanup, then SA agent density in a 2-4 GiB guest, with
    SM model policy + evals after SR, and SC desk capability surface after
    SA + SM. See "v5 plan".
  source_ref: main@e9cd9fed
  deploy_identity: 'staging https://choir.news deployed_commit=e9cd9fed (SMG probe verified on computer-e472d237; owner + vm-48bc0981 on 19d7913e base image)'
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
      builder substrate = host service. v5 (owner-approved 2026-10-05):
      density before distribution (agents per guest, then guests per host,
      then hosts); SR/SA/SM/SC inserted; Jev M0a and M2 absorbed; World
      Wire is the consuming lens and its 10-01 metamission is superseded
      pending a rewrite on the rearchitecture analysis.
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
    0. RESOLVED — SA slice 0's apply-fence blocker: ca8c8c18's apply
       rolled back on updater health-probe 503 during the 546k-event
       replay (~35s of 503s vs a 30-attempt fence). Fixed 19d7913e —
       HTTPHealthProber treats an advancing replaying 503 as liveness
       (resets stall budget, MaxDuration=15m absolute). Owner runs
       19d7913e via guest-image deploy + refresh. Gate hardened 68397ae7
       (commit-bound; rolled-back pushed computer fails the deploy).
       Residual: a real app-layer apply on the owner under the tolerant
       probe is still owed — land on the next deploy carrying an
       app-layer delta. Fresh disposables still resolve the PREVIOUS
       release (storedisk.erofs baked at guest-image build) — S2
       per-mint release pin is the durable fix.
    1. SA slice 1: storm convergence (red). Problem-doc first: record the
       boot re-arm mechanism (v5.1 revision, finding 2) and confirm it from
       the guest trace (re-armed wake ids vs their source obligations'
       trajectory state). Then: terminal fate for stale obligations as a
       recorded act on the tape; wake-outbox migration one-shot (versioned
       marker) or deleted; O(1) occurrence resolve; per-desk dispatch gate
       with a paced drain. Acceptance: owner guest restart converges
       pending to a bounded floor within a stated window and stays there
       for 24 h, and the SMG probe passes on the owner computer.
    2. SA slice 0 residual — RESOLVED + DEPLOYED-VERIFIED 2026-10-07
       (e9cd9fed): first-activation CAS race (500 'replace durable
       activation: lifecycle invalid transition' on submits 1-2 post-boot)
       was the lifecycle_work_assigned wake racing EnsureTextureHandoff —
       the wake's reconcileAgentWakeLocked committed run-A before the
       handoff's own submit landed, so run-B hit the previous-active-run
       gate. Fix: submitTextureAgentRevisionRun recovers the already-
       committed run on ErrLifecycleInvalidTransition
       (recoverRacedTextureActivation). Deployed-verified on fresh
       disposable computer-e472d237 — management_open 202, all legs green
       (smg-rlm-acceptance-fix-verify-2026-10-07.json).
    3. SMG named edge (not blocking SA): after slice 1 lands, re-run
       scripts/smg_rlm_acceptance_probe.mjs on the owner computer AND a
       fresh disposable; if the texture desk still will not emit
       open_persistent_super on demand, escalate — the edge is agency,
       not transport (three failures across two prompts on a clean
       disposable 2026-10-06; evidence
       smg-rlm-acceptance-disposable-2026-10-06.json). Director constraint
       stands: do NOT drain/park the owner storm to pass the legs.
    4. SA slices 2-7 (baseline, memory, engine lock, read cost, store
       split, shapes) in order.
    5. SO remainder: CAS durability (fsync file+dir before the og row
       commits), recorded loss disposition for the 161,185 missing corpus
       body_refs, check artifact-GC history for the ~115k pre-10-05 loss,
       delete platform.og_objects residue, corpus-dolt CPU sample. Not on
       the critical path (corpus frozen).
    6. S1: non-root runtime (gates S4).
    7. Author SM and SC station files (SR has landed; SM can start in
       parallel with SA slice 2+ since it does not touch the store).
    DONE since v5: S2 CLOSED 2026-10-05 (s2-station-terminal-2026-10-05);
    SR CLOSED 2026-10-06; og wrong-store GC fix 2fa30b17; sourcecycled
    durably off d42c47b/0564246; CI deploy-gate verifier f15fb7ea; SMG
    commits 1b1d9b7e, 4bedf999 deployed; SA slice 0 9f6f369c deployed
    (in-guest genesis mint; pre-genesis -> 503; deployed-verified on
    disposable computer-07b582d5).

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
  - id: smg-to-sa-transition-2026-10-06
    kind: station_transition
    status: closed
    landed: SMG-management-rlm-cutover (choir-appdev-smg-management-rlm-cutover-2026-10-06)
    next: SA-agent-density (choir-appdev-sa-agent-density-2026-10-05)
    closed_at: '2026-10-06T23:15:00Z'
    landed_receipts: >-
      Probe legs 1-3 PASSED on fresh disposable computer-0ca7656f
      (build 475902d7, marker smg-dispo5):
      docs/evidence/smg-rlm-acceptance-disposable-2026-10-06.json —
      management_open via POST /api/texture/management-open (canonical
      IssueLifecycleControl reducer), persistent_management_signal
      (control_queued), assignment_cancel_verb
      (co_super_capsule_disposition_set + co_super_assignment_cancelled),
      unbound_refusal_evidence, bound_producer_report (update_delivered
      result:sha256:6f0d599a… →
      mgmt-work-smg-rlm-open-smg-dispo5-1791328145); probe exit 0.
      Deployed schema leg passed on computer-03335285 (tools exactly
      [desk_go_eval]). Slices 1-4 landed+deployed (1b1d9b7e, 4bedf999;
      CI 37420355583 green incl. Node B).
    panel: >-
      Convergent boundary panel 2026-10-06
      (.agentic-consensus/agentic-consensus-20261006-191327, 9/10 reporting):
      close affirmed + promote SA (not S3 — the DAG gates S3 on SA+SO);
      conditions discharged: SA now-card updated with the defect field,
      stale deploy-freshness residual removed, weak legs named as edges.
    report: docs/reports/smg-management-rlm-cutover-station-close-2026-10-06.md
    named_edges: >-
      Re-run legs 1-3 on the owner computer after SA slice 1 (director
      2026-10-06; owner-guest storm is SA's domain; do NOT drain/park it).
      The re-run must carry two legs that stayed weak on the disposable:
      (a) cell-level unbound refusal — the probe matcher is substring-only
      and the objective text contains the match strings, so the leg is
      unproven beyond host-side unit tests; no packet-body API exists to
      tighten it; (b) work_disposition=completed settlement — the
      delivered report was never incorporated (no surviving management
      run on the disposable). Both need a post-fix owner run.
      Texture-desk agency edge stands for organic opens: desk does not
      author open_persistent_super on demand (S0m-documented); the
      /api/texture/management-open endpoint is the deterministic
      acceptance surface, not the product path.
    defect_field_handed_to_sa:
      - docs/problems/sa-delegated-report-poisons-management-listing-2026-10-06.md
      - docs/problems/sa-management-mint-no-start-slot-deadlock-2026-10-06.md
      - docs/problems/sa1-wake-outbox-rearm-storm-2026-10-06.md
      - docs/problems/sa2-appendevent-unbounded-scan-2026-10-06.md
    residuals: >-
      Host-pressure reclaim hibernates a VM with a live probe run
      (recorded under S2 hibernate contract; vmctl journal 2026-10-06).
      Probe-hardening residuals from midcourse panel folded into
      smg_rlm_acceptance_probe.mjs fixes this session (work_item_id only
      materializes on update_delivered; bound_report must not wait on a
      live-gated trajectory).

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

## v5.1 revision — convergence before density (2026-10-06)

Director review after S2, SR and SMG. Verdict: continue the metamission;
the station order changes inside SA only, plus one SO addition.

**Progress since v5.** S2 closed 10-05. SR closed 10-06 (panel r7 7-0):
research is a full RLM and processor/reconciler are gone. SO: the og
wrong-store GC fix (`2fa30b17`, fail-closed on live-set errors),
sourcecycled durably off (`d42c47b`, `0564246`), and the deploy-gate
verifier for pointer-following guests (`f15fb7ea`). SMG, inserted on the
owner's 10-06 directive, landed: management, research and texture all
expose exactly `{desk_go_eval}`, and the `product_api_request`,
`report_to_texture` and `cancel_co_super_assignment` tools are gone. A
latent broker defect (evidence actions validated but never dispatched) was
fixed in passing. SMG's schema leg is deployed-proven; its three
behavioral legs are blocked.

**Finding 1 — the storm is the critical path.** The Management
live-occurrence storm is the fourth recurrence of one substrate defect
(live-locks `886e5ce1`, `3b0a1ed2`/`b7f59cc9`, the 10-03 storm, the 10-06
restart regrowth). It starves every fresh activation on the owner computer.
That blocks SMG's legs, would make SA's baseline a measurement of the storm,
and makes the owner computer unreliable for ordinary work. Root Cause
Clustering applies. It moves from SA slice 4 to SA slice 1, ahead of the
baseline.

**Finding 2 — why it survives restart (source-traced; confirm in the
trace).** `migrateActorWakeOutboxAsync` (`internal/agentcore/runtime.go:2588`)
runs `MigrateActorWakeOutbox` (`internal/store/lifecycle.go:965`) on every
kernel-mode boot, not once at cutover. It lists every object of six kinds,
including all runs and Texture revisions. For each wake row that is already
projected but whose source obligation still derives as open, it re-arms the
wake. Nothing gives stale obligations (August trajectories) a terminal fate,
so each restart re-feeds the herd. The 10-06 07:20Z boot logged "minted 2124
pending wakes"; the wake keys are deterministic and prior boots already
minted them, so that count is almost entirely re-arms. The migration is also
an O(all objects) boot scan under `engineMu`, which is SA cost in its own
right. The fix is the convergence invariant already written in
`s0m-management-live-occurrence-storm-2026-10-03.md` plus two additions:
- **Terminal fate as a recorded act.** Obligations whose trajectory is
  closed, or that exceed a recovery budget, get a durable disposition on the
  tape (`delivery_attempts_exhausted` / `expired`). Never a table delete:
  the tape stays the single state authority.
- **One-shot migration.** Gate it behind a versioned marker, or delete it if
  the cutover is complete everywhere (deletion-citers grep first).

**Finding 3 — the disposable path is not blocked.** The genesis residual has
a documented manual route: `POST /api/computers/{id}/lifecycle/bootstrap-chain`
with the owner's key mints genesis, and writes then succeed. SMG's legs can
run now on a disposable with that preamble. Hand-draining or parking the
owner guest's storm to pass the legs is not admissible. It is the S2
hand-staged-nar trap: the probe passes, nothing is proven, and the substrate
stays broken. The proper fix (provisioning mints genesis or refuses `active`;
pre-genesis writes get a clean 503) is small and red. It becomes SA slice 0,
because every SA acceptance runs on disposables.

**Finding 4 — CAS durability (SO, off the critical path).** 161,185 live
corpus `body_ref`s are missing from disk: at most 45,829 from the 10-05
sweep, about 115k earlier. Externalized rows store `body = NULL`, so the CAS
file was the only copy. `externalizeBody`
(`internal/platform/objectgraph_store.go:42`) does tmp + rename with no file
or directory fsync, and the Dolt row commits regardless. The leading
hypothesis for the earlier ~115k is earlier artifact-GC runs with the same
wrong-store live set, since that bug dates from the Store A/B split. Check
the GC run history before blaming fsync. SO items:
- fsync file and directory before the row commits;
- a recorded loss disposition for the missing refs, so readers degrade
  cleanly instead of erroring;
- delete the `platform.og_objects` residue (6.09M rows, a future decoy).

Guest stores keep og bodies inline (`DoltStore`), so this loss is
corpus-only. The corpus is frozen, so nothing user-facing regresses now. It
must be repaired before World Wire ingests again.

**Revised SA order:** 0 registration genesis → 1 storm convergence → 2
baseline + offline GC re-measure → 3 memory → 4 engine lock (race test
first) → 5 read cost → 6 store split → 7 shapes and elasticity. SM can start
in parallel once SA slice 1 lands, because SM does not touch the store.

## v5 plan — density before distribution (2026-10-05)

**Principle (owner):** "we must first scale agents horizontally in a 2-4 GiB
VM before scaling VMs horizontally." Scale in order:
1. concurrent desk activations inside one guest (SA);
2. guests per host (S3 residency tiers);
3. hosts (out of scope).

Bulk World Wire across many computers comes after S9, in the World Wire
metamission.

### Critical path

```
S1 (non-root) ─┐
S2 (accept) ───┼─> SR ─> SA ─> S3
SO ────────────┘     └─> SM ─> SC ─> S5 ─> S6 ─> S7/S8 ─> S9 ─> S10/S11
                                     └ S4 (after S1) ┘
World Wire phase 1 (the newspaper) can start after S5.
```

### SR — desk-surface cleanup (small; absorbs Jev M0a)

Finding (director review 2026-10-05): the research RLM cutover is half-done.
- R3r (settled 09-26) put research on the in-cell carrier but kept 13 typed
  tools (`internal/agentcore/tool_profiles.go:394-408`).
- M0a phase 1 landed the 13 in-cell `choir.*` verbs. The deletion commit,
  the deployed verify and the prompt rewrite never happened, and M0a's file
  has been frozen since the 10-01 spine change.
- The base prompt `internal/promptstore/defaults/research.yaml` still
  teaches `update_coagent` and JSON-tool cadence. That tool is absent from
  the research desk registry, and S0m retired raw messaging.
- `rlm_research_runtime.yaml` advertises both surfaces.

Scope:
- Research registry becomes `{desk_go_eval}`. Delete the legacy
  `ResearchRuntimeOverlay` branch if the carrier is always live.
- Delete processor and reconciler end to end: roles in modelpolicy,
  `processor_runtime.yaml`/`reconciler_runtime.yaml`, the
  `processor_requests`/`reconciler_requests` dispatch path, and
  sourcecycled's processing path. Run a deletion-citers grep first.
- Rewrite `research.yaml` for the record-native in-cell world. The overlay
  describes one surface.
- Check whether S0m discharged Jev M1 ("typed commitments"). Close or
  re-scope M1 accordingly.

Acceptance: M0a's four deployed actions (multi-search loop in one cell with
evidence reaching Texture mid-loop; capability-parity checklist; egress
budget refusal returned into the cell; deployed tool schema is exactly
`{desk_go_eval}`).

### SA — agent density in a 2-4 GiB guest (red)

Finding: `engineMu` (`internal/objectgraph/dolt_store.go:62-66`) serializes
every query on the guest's embedded Dolt. The cause is a race, not design:
two pools on one embedded engine "race on shared internal buffers
(unescapeHTMLCodepoints mutating a JSON slice in place)". Occupancy was
~97%, read-dominated, top holders unindexed scans (latency mission
`cf0ef207`).

One store holds versioned content and high-churn operational rows (runs,
events, channel messages, inbox deliveries, work items, wake outbox, pending
mutations). That plausibly explains three symptoms at once:
- lock contention;
- the ~22 GB journal against ~5 GB live;
- the 2 -> 16 GiB memory ratchet.

**Target** (fixed from slice 1's baseline): about N concurrent desk
activations in a 4 GiB guest with p95 act-commit under 1 s and an idle
footprint under 2 GiB. The owner computer comes down to a 4 GiB ceiling.

Slices, in order:
1. **Baseline.** A fan-out load probe (N research + M scorer activations)
   plus a memory receipt: `EngineMutexOpStats` wait/hold per caller, guest
   meminfo, Go runtime metrics, Dolt cache and journal sizes, and host
   committed RSS per Firecracker process. Then a host offline GC of the owner
   store, and measure again.
2. **Memory.**
   - GC ordering fix (`internal/store/dolt_maintenance.go:290` guard
     precedes the `:309` journal trigger).
   - Automatic host offline GC above the in-guest safety threshold.
   - `GOMEMLIMIT`; bounded Dolt caches.
3. **Engine lock.**
   - Reproduce the race under `-race` with concurrent JSON queries on one
     engine.
   - Upgrade or patch Dolt / go-mysql-server, or avoid the path.
   - Then make `engineMu` a read/write lock.
   - No RW lock without a passing race test.
4. **Read cost.**
   - Scans become indexes or cursors; polling becomes notification.
   - Hot reads (lifecycle snapshot, pending mutations, inbox, `Pack()`)
     come from write-maintained projections.
   - (v5.1: the Management storm convergence invariant moved to SA slice 1;
     see "v5.1 revision".)
5. **Store split.**
   - High-churn operational tables move to SQLite WAL (precedent: the actor
     recovery log, `internal/actorruntime`).
   - Versioned content (Texture revisions, og effective state, event index)
     stays in Dolt.
   - Group-commit acts.
   - The replay manifest and restore set are updated. State authority is
     unchanged: the tape stays canonical.
6. **Declared shapes and elasticity.**
   - Remove the `vmctl-priority.env` 16 GiB override; shapes live in
     tracked config.
   - Per-VM cgroups (`MemoryHigh`/`MemoryMax`), balloon (verify v1.15.1
     free-page reporting), host zswap, and a declared oversubscription ratio.

### SM — model policy rewrite and evals (absorbs Jev M2)

- **Policy as layered, versioned data:** platform -> computer -> desk
  (desk-owned, cell-editable) -> per-cast -> eval assignment.
  - Keys: desk plus purpose (for example research/imputation,
    management/scoring, texture/drafting), task tags, budget class.
  - Values: candidate sets with weights and constraints (tools/modalities,
    context, cost ceiling, provider health), replacing hard-coded
    fallbacks.
- **Per-activation selection for every desk** (today only engineering
  assignments pass `ModelPolicyOverlayID`).
- **Every resolution stamped on the activation and its records:** policy
  digest, matched rule, selection.
- **Policy changes are precommitments,** resolved by evals or production
  records. No more unrecorded edits like the 10-02 live data.img swap.
- **Evals are N-arm runs** varying model, effort, prompt version (prompts
  and styles are versioned Textures), context pack and tool surface.
  1. Fixture evals first, with the rubric frozen before runs (Jev M3 rule).
  2. Then shadow evals on sampled live traffic, with read-only arms (no
     canonical acts, no external sends), after S1 is complete.
  3. Replay evals only once tool-call results are on the tape.
- Arms precommit; heterogeneous management scorer activations score.
- Retire dead roles (`processor`, `reconciler`, and any other unused).

### SC — desk capability surface

- **Per-desk Go package sets** (`deskPackageSets` beside `deskModuleSets`).
  - Pure computation only (`slices`, `maps`, `cmp`, `container/heap`,
    `crypto/sha256`, `hash/fnv`, `net/url`, `encoding/xml`, `encoding/csv`,
    …).
  - Production must run workers with `--session-harden` before widening
    any set.
- **Read verbs:**
  - `Graph.Get/Versions/Neighbors/Query` — `Neighbors` with backlinks is
    autocitation;
  - `Ledger.Query/TrackRecord/Due`;
  - `Similar` (host-mediated vector search);
  - `Source.Versions/Diff/Pin` (pinned span as a transclusion ref);
  - `Export(query)` into an analysis capsule for engineering.
- **Texture transcludes object-graph object versions** via `ApplyTexture`.
- **Management scorer fan-out** uses SM's per-activation selection. A desk
  is many heterogeneous activations: independence comes from model-family
  diversity and context isolation, not from a separate scorer service.
- **Read verbs are computer-scoped.** Cross-computer reads come later
  through the claim feed, never as a back door into Store B.

### Changes to existing stations

- **S3:** depends on SA (small guests mean small snapshots). It opens with
  the snapshot measurement S0b could not take.
- **S5:** depends on SC (Texture reads), and still consumes the external
  Texture transclusion work.
- **S6:** narrower. S2-f already joined self-dev to the builder, so S6 is the
  owner-gated full-release path from a supervised capsule.
- **S8:** gains a design-only slice for the claim-feed protocol
  (cross-computer data subscription: signed append-only claim log, base +
  watermark + tail on the Restore-Zero pattern, subscription authority,
  privacy class, revocation). It is built in the World Wire metamission.

### Tracked open decisions

1. M1 vs S0m — settled in SR.
2. Claim-feed protocol shape — S8 design slice.
3. corpus-dolt CPU burn diagnosis and its 18-20 GiB cap (host density).
4. Capsule egress default: open-with-recording vs allowlist (S4).
5. Snapshot encryption at rest (S3/S10).
6. Trusted host build cache (deferred).
7. Legal and licensing for scoring named public figures and republishing
   excerpts (owner/legal, not engineering).

## World Wire as the consuming application (2026-10-05)

Owner framing: World Wire / autopaper is "the true application of Choir,
the purpose of all this infra". This is not one autopaper but a platform
for others' autopapers, with white-label copies built from the primary.
The rearchitecture analysis is `docs/world-wire-rearchitecture-2026-10-05.md`
(read it with its director notes); the evidence is
`docs/problems/world-wire-corpus-resource-burn-2026-10-05.md`.

How this metamission treats it:
- **v5 status:** the World Wire metamission
  (`choir-world-wire-metamission-2026-10-01.md`) and its W0 station are
  superseded. They predate the rearchitecture (processor/reconciler, host
  ingestion). A rewrite is pending, built on the rearchitecture analysis:
  - speaker -> statement -> imputed commitment -> resolution;
  - four-desk factorization (no processor/reconciler);
  - the claim feed;
  - a platform observer computer plus tenant autopaper computers.

  The 10-01 owner decision on verticals ordering (general news, then AI,
  then Taiwan/geopolitics/semis/internal democracy, then the region ladder)
  carries forward. **Phase 1 (the newspaper)** can start after S1, S2, SO,
  SR, SA, SC, S3 and S5 land; it does not need S6-S11.
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
  at all"; target 2-4 GiB, oversubscribed).** *v5: moved to SA. It shares
  a root cause with the Dolt engine bottleneck. The analysis below stands;
  SA owns execution.*
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
`docs/problems/s2-app-layer-offer-bind-gaps-2026-10-05.md`.

### Handoff 2026-10-05 — S2 STATION CLOSED

S2 closed under frozen transition contract `s2-layered-transition-v1`
(terminal receipt `s2-station-terminal-2026-10-05` in the station file).
All six acceptance criteria discharged at deployed-proof class on
disposable computer-6450a253 plus the owner computer; criterion 2's
CI-origin proof landed by CI run 37378327268 (`app-layer push: 2/2
healthy, time-to-healthy=131s`, commit `4ef44901`, served release marker
`app-layer-4ef4490123ee` on the guest's `current/`). Consensus: round 1
4 approve / 3 send-back (criterion-2 CI-origin + discharge-append wedge,
both then closed); round 2 **6 approve / 1 send-back** (sole send-back
named receipt completion only, discharged by continuity pins — single
Firecracker process and one boot id across the push). Deployed identity:
`x-choir-build-commit=7c0897c2` (4ef44901 is docs-only).

Carried residuals (open problem docs, not station blockers):
`s2-postswap-restart-loop-kills-vm` (health-failing release can kill FC
mid-restore; S2-e repair remains partially open),
canonical-head bootstrap authority for never-committed computers
(owner-authority decision), `s2-m9a-route-projection-owner-binding`
(close the record with the route-promotion receipt),
platform-artifacts GC cadence (S0 — first sweep reclaimed 35.4GB).

Next station: **S3 fast resume** per the v5 ordering; S2 hands S3/S6 the
(B,U,R) transition tuple and the builder-produced closure contract.



## v6 plan (2026-10-09, owner-ratified)

The goal is three outcomes, in order; stations serve them, not the reverse.

**Gate 1 — the computer is stable.** Exit test: lose-the-disk proof on a
disposable (SH), SL fault matrix on a disposable, owner computer 72 h
unattended with no new liveness problem docs, Texture acceptance suite
green. Stations: SH (O21, in flight), SL (O1; absorbs the Management storm),
O7/O12 as enforceable checks (boot-cost budget test, lock-scope rule),
O8/O9 remainder, Texture contract and acceptance (Texture is input, output,
supervision, control plane, multi-agent member and an RLM desk — it gets
its own invariants in the register).

**Gate 2 — self-development with live Texture supervision.** Exit test: the
owner asks Texture for a change; engineering develops it in a capsule; the
owner watches a live preview, approves, and the release applies and can be
rolled back. Stations: S1 remainder -> S4 -> S5 -> S6. SC/SM only as far as
S5 needs them (to be checked when S5 is re-read).

**Gate 3 — World Wire operational.** Designed on SW's constraints after
Gate 2; four core desks (research reads and corroborates, management sets
attention and spend, engineering writes source adapters in capsules,
Texture writes, revises and publishes articles); deterministic steps are
desk tool modules, never separate actors; shared claims are published
objects that subscriber computers adopt onto their own tape; no
architectural ceiling on ingest — budget is the constraint.

**Deferred (after the goals):** S3 fast resume, SA density beyond the storm
fix, S7 app packages, S8 source publication, S9 forks and fleets, S10 org
templates, S11 mainline push, and SP production infrastructure (second host
as off-host durable homes, self-hosted remote logs/metrics/alerting over a
WireGuard mesh, zero-downtime deploys, multi-host placement, O22;
design recorded in
[SP](choir-appdev-sp-production-infrastructure-2026-10-09.md)). Owner
2026-10-09: "prove the system for human QA before I invest more money into
it, and before we make the infrastructure more complex." Gate 1 runs on the
single host with local logs.

**In flight now:** SH (slice 3 deployed; file-hydration fix landing),
SW design, corpus teardown, then SL.
