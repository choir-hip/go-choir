---
definition_version: 4

# v2 2026-10-01: adds layering via guest Nix closures, fast resume /
# machine-snapshot hibernate, forks and fleets (owner-ratified sibling-computer
# model), and org templates. Stations reordered for development ease: the
# substrate that shortens every later dev loop (security floor, layering,
# fast resume) comes first.

readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  stations:
    - id: S0-reality-and-boot-timeline
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: []
    - id: S1-security-floor
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S0-reality-and-boot-timeline]
    - id: S2-layering-runtime-from-release
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S0-reality-and-boot-timeline]
    - id: S3-fast-resume
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S1-security-floor, S2-layering-runtime-from-release]
    - id: S4-capsule-open-world
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S1-security-floor]
    - id: S5-live-preview-supervision
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S4-capsule-open-world]
    - id: S6-commit-gate-full-release
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S2-layering-runtime-from-release, S5-live-preview-supervision]
    - id: S7-app-packages
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S6-commit-gate-full-release]
    - id: S8-source-publication
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S6-commit-gate-full-release]
    - id: S9-forks-and-fleets
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S1-security-floor, S3-fast-resume, S8-source-publication]
    - id: S10-org-templates
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S9-forks-and-fleets]
    - id: S11-mainline-and-security-push
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S8-source-publication, S9-forks-and-fleets]

start:
  captured_at: '2026-10-01T00:00:00Z'
  source:
    canonical_ref: main@81bbdd82014501a9a73004a09aefed899d92e002
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
        Hibernate/resume. A computer with an open Texture doc and a
        completed run is hibernated (quiesce, snapshot, process exits,
        RAM freed on host) and woken by an owner request.
        Recorded timeline: wake request to first healthy response within
        the S3 target (proposed: p50 under 1 s, p95 under 3 s). The
        resumed computer serves the same state and refreshes its clock,
        gateway token and RNG. A deliberately invalidated snapshot (image
        mismatch) falls back to a cold boot with the disk consistent. A
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
        a non-conflicting divergent computer, is classified exempt where
        the computer's own code passes the exploit test, and fails closed
        (component reverted or capability cut, owner notified) on a
        conflicting vulnerable computer.
      proves: Fleet security push without silent breakage or silent skip.
      evidence_class: deployed proof
    - action: >-
        Negative proofs. A capsule or a fork cannot reach another
        tenant's guest or the host's private services. A fork cannot
        sign as, or read undeclared data classes of, its parent.
        Capsules cannot read the gateway token. Uncommitted capsule
        changes do not survive disposal. Snapshot files are unreadable
        outside the owning computer's state directory. Forged or
        misbound publications, offers, or snapshot restores are refused
        before any mutation.
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
    Authoring v2. Stations are intent only. Next is S0: read-only reality
    probes plus a boot timeline instrument, so every later latency and
    layering claim has a measured baseline.
  source_ref: main@81bbdd82014501a9a73004a09aefed899d92e002
  deploy_identity: unknown
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
      M9a transport, capsule effect bundles and Restore-Zero semantic
      snapshots carry it. What is missing is effect reach, openness and
      speed:
      (a) the guest executes a per-computer app-layer closure, not the
      image baseline;
      (b) capsules get recorded egress and a private Nix store;
      (c) a preview bridge;
      (d) the change record is base rev + patch stack + pinned inputs;
      (e) machine snapshots for resume, semantic snapshots for
      distribution.
      Layering (a) is also the largest latency lever, because most
      updates stop rebooting VMs.
    test: >-
      S0 confirms (a) is the blocker on staging (a self-dev Go change
      applies but the endpoint does not change) and produces a boot
      timeline that attributes the 5-30 s. Each later station is
      falsified if it needs a new authority path rather than a new effect
      path.
    edge: missing_oracle
    delta_o: >-
      A per-boot timeline receipt (host spawn, kernel, initrd, systemd
      units, runtime boot phases, first healthy) and a staging probe
      asserting a runtime-observable effect after self-dev apply.
    scope_if_supported: >-
      Single-host Choir Community Cloud staging (32 GiB Node B), Go
      runtime + Svelte frontend + app backends, Firecracker.
    status: proposed
    evidence_refs:
      - docs/evidence/m11-probe-run9-satisfied-2026-09-29.json
      - docs/reports/nixos-agent-platform-redhat-deepseek-audit-2026-10-01.md
      - docs/problems/guest-release-propagation-manual-2026-10-01.md
      - docs/reports/choir-rlm-restore-zero-snapshotting-correction-2026-09-09.md
  decision:
    what: >-
      Two-layer guest (shared non-forkable NixOS base; per-computer
      app-layer Nix closure built against it). Source-only publication.
      Machine snapshots for resume, semantic snapshots for fork and
      distribution. Forks are sibling computers.
    kind: architecture
    status: settled
    evidence_ref: owner statements in session 2026-10-01
    owner_ratification_ref: >-
      owner 2026-10-01: "I like your suggestion for layering, and using
      guest NixOS closures appropriately"; "permission granted" (forks);
      "I ratify #1"; source only
  belief:
    believed_state: >-
      The control plane works. The effect plane stops at the frontend.
      Capsules are closed to the network. Apps are monoliths. Publication
      does not exist. The VM network may be too open between tenants.
      Hibernate is a cold boot. Every image deploy reboots every active
      computer.
    main_uncertainty: >-
      Where the 5-30 s actually goes (kernel/initrd vs the serialized
      systemd chain vs runtime boot phases such as Dolt open and recovery
      planning). And whether an owner-sized (8 GiB guest, about 11 GiB
      store) machine snapshot hibernates and resumes within budget on
      btrfs with lazy memory loading.
    next_observation: >-
      S0 boot timeline on staging for one fresh and one owner-sized
      computer, plus the reality probes (self-dev Go effect, tap->tap
      reachability, gateway token visibility, M9a full-bundle payload).
  blocker_or_risk: >-
    S4 (open-world capsule) and S9 (forks) must not land before S1. A
    machine snapshot restored against a moved disk corrupts the
    filesystem, and a double resume duplicates RNG and key state. S3's
    invariants are the safety case, not optional polish.
  next_action: >-
    Author S0 as a throughline station file (read-only probes + boot
    timeline instrument, green/yellow) and run it. Problem-document each
    confirmed finding before any fix.

receipts: []
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

- **S0 reality + boot timeline.** Read-only probes: self-dev Go effect,
  tap-to-tap reachability, gateway token visibility, M9a full-bundle
  payload, reflink on the VM state path, Firecracker snapshot support in
  the pinned version (including VMGenID/RNG reseed on restore). Add a
  per-boot timeline receipt: host spawn, kernel, initrd, each systemd
  unit, each runtime boot phase, first healthy. Measure a fresh computer
  and an owner-sized one.
- **S1 security floor.** Per-tap FORWARD isolation, default-deny VM
  egress, a confined non-root runtime unit, gateway token scrubbed from
  child environments, and the Yaegi worker kernel floor. Prerequisite
  for S4 and S9.
- **S2 layering.** Split the guest into base image + app-layer closure.
  The guest executes the runtime from its committed release (image
  baseline as fallback). Each release records the base image it was
  built against. Add a per-computer guest image reference with GC-rooted
  retention (the bootc switch equivalent). The updater refuses a release
  whose closure does not resolve in the booted base. Platform app-layer
  pushes (M9a) apply without reboot. Wire the M9a push into CI for
  tracking computers.
- **S3 fast resume.** Machine-snapshot hibernate/resume with these
  invariants: guest quiesce (sync + fsfreeze) before snapshot so the disk
  alone is consistent; paired reflinked disks; one-shot consumption
  recorded in vmctl before resume; exact image/kernel/Firecracker
  pairing, otherwise discard and cold boot; a post-resume hook (clock
  step, gateway token refresh, RNG reseed, process-local key
  regeneration, reconnects, epoch handling); snapshot files root-only in
  the computer's state directory; snapshot disk usage counted by
  pressure reclaim. Then trim the cold boot critical path using the S0
  timeline.
- **S4 capsule open world.** A recording egress proxy bridged into each
  capsule netns (URL + content hash of every fetch logged to the
  trajectory; never private or tap ranges). A capsule-private Nix store:
  either in the capsule upper, or the Nix local-overlay store over the
  read-only base (experimental; verify). Substitution goes through the
  proxy.
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
- **S9 forks and fleets.** A fork is a new sibling computer built from a
  semantic snapshot, or for speed from a reflinked disk with a cold boot
  under new identity. It is never a resumed machine snapshot of the
  parent unless every in-memory identity is re-keyed. It mints a new
  ComputerID, signer keys, privacy key and gateway token, and declares
  which data classes it carries. Forks are ephemeral or persistent, with
  fleet admission and quotas under the 32 GiB budget. Merge-back happens
  via S8. The fleet view is Texture transclusion of fork sources.
- **S10 org templates.** Export a scrubbed semantic template (base rev +
  patch stack + curated seed data + policy, encrypted at rest). Install
  it as a fresh computer that builds from source.
- **S11 mainline + security push.** Host selection into main. Security
  offers carry severity, a deadline and an exploit test. Each divergent
  computer is evaluated on an ephemeral fork: auto-apply, exempt,
  proposal (agent rebase in a capsule), or after the deadline fail-closed.

## Latency plan (S0, S2, S3)

1. **Measure before cutting.** No optimization lands without the S0
   timeline attributing the time.
2. **Stop rebooting for updates.** Layering (S2) turns most deploys into
   updater swap + runtime restart. That also keeps machine snapshots
   valid across app-layer updates, because the base image does not
   change.
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

1. Fork construction speed: semantic snapshot (slow for a 16 GB
   ProjectionBase) vs reflinked disk + cold boot + re-key (fast). Leaning
   reflink + re-key, with a scrub step for data classes not carried.
2. Default capsule egress: open-with-recording + never-private-ranges
   (leaning) vs allowlist.
3. Trusted host build cache keyed by derivation hash (deferred).
4. Whether hibernate snapshots are retained after resume for a fast
   re-hibernate via diff snapshots, or always discarded (start with
   discard).
5. Snapshot encryption at rest on the host (needed before any export;
   for on-host hibernate, root-only permissions match data.img today).
