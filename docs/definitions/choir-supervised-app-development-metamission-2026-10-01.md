---
definition_version: 4

readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  stations:
    - id: S0-reality
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: []
    - id: S1-security-floor
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S0-reality]
    - id: S2-runtime-from-release
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S0-reality]
    - id: S3-capsule-open-world
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S1-security-floor]
    - id: S4-live-preview-supervision
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S3-capsule-open-world]
    - id: S5-commit-gate-full-release
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S2-runtime-from-release, S4-live-preview-supervision]
    - id: S6-app-packages
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S5-commit-gate-full-release]
    - id: S7-source-publication
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S5-commit-gate-full-release]
    - id: S8-platform-mainline-and-security-push
      path: unauthored (intent)
      readiness: intent
      status: pending
      depends_on: [S7-source-publication]

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
    survives reboot and restore. The owner can then publish the change
    to the host as source. Another computer's owner can review it, take
    the source, customize it, build it locally in a capsule, and adopt
    it through their own gate. The host can select a published change,
    mainline it into the platform base, and push security fixes across
    a divergent fleet without silently breaking computers whose
    divergence conflicts with the fix.
  artifact: >-
    A deployed product path on choir.news with these pieces:
    (1) `choir run start` from an API-key client yields a supervised
    trajectory.
    (2) A capsule with recorded, policy-mediated egress and a capsule
    Nix store.
    (3) A preview route from the desktop into the capsule's frontend
    and backend.
    (4) A commit gate (tests + owner approval) that materializes a full
    release (runtime + frontend + app backends) that the guest actually
    executes.
    (5) A source publication record (patch + base revision + pinned
    inputs + build recipe + tests) on the host.
    (6) An adopt flow on a second computer that builds from source.
    (7) A platform security offer with per-computer rebase, exploit-test
    classification, and a fail-closed fallback.
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
        The single-computer deliverable works end to end on the product
        path, including backend effect (not frontend only) and
        reversibility.
      evidence_class: deployed proof
    - action: >-
        Same proof, modifying an existing app (for example a Mail or
        Texture behavior change spanning backend and frontend).
      proves: >-
        Modification of existing app code, not only additive new apps.
      evidence_class: deployed proof
    - action: >-
        A publishes. Computer B (a different owner) lists A's
        publication, reviews the diff and the pinned inputs, adopts, and
        builds from source in its own capsule with a local customization.
        B's tests and B's owner approval land it. B serves the customized
        app. No binary from A is executed on B.
      proves: >-
        Source-only publication and divergent adoption.
      evidence_class: deployed proof
    - action: >-
        The host mainlines A's change into main and ships a security fix
        touching an app-layer boundary. The fix lands automatically on a
        tracking computer, lands after rebase on a non-conflicting
        divergent computer, is classified exempt on a computer whose own
        code already passes the exploit test, and fails closed (component
        reverted or capability cut, owner notified) on a conflicting
        vulnerable computer.
      proves: >-
        Fleet security push without silent breakage or silent skip.
      evidence_class: deployed proof
    - action: >-
        Negative proofs. A capsule cannot reach another tenant's guest or
        the host's private services. A capsule cannot read the gateway
        token. Uncommitted capsule changes do not survive capsule
        disposal. A forged or misbound publication or offer is refused
        before any mutation.
      proves: >-
        The open-world capsule did not widen authority.
      evidence_class: deployed proof
  rollback: >-
    Per station: git revert + redeploy. For a computer: pinned-head
    restore through the existing tape/checkpoint path (M9a/M11 edge).
    For publication: retract the record; adopters keep their own
    committed copies because source-only means no remote kill switch.
    A security offer that misclassifies a computer is reverted by
    restoring that computer to its pre-offer head.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between "an agent with an API key asked for an app"
    and "the owner is running that app, backend and frontend, having
    watched it built and having approved it, and other owners can adopt
    it from source". Preserve the invariants: no authority widening, no
    unreviewed code executing outside a capsule, every committed change
    reversible, and every fleet push accounted for per computer.
  goodharting_would_be: >-
    A demo that only touches frontend files (the M11 shape). A preview
    that is a screenshot or a static file listing rather than the running
    app. A "publish" that ships a binary or an opaque tarball. A
    "security push" that skips divergent computers and reports success.
    Network egress enabled by opening the VM wider instead of through a
    recorded capsule proxy.

homotopy:
  realism_axis: >-
    The same pipeline (capsule -> preview -> gate -> release -> publish ->
    adopt -> mainline/push) at increasing resolution:
    one string change in an existing Go handler, then
    a new frontend-only app, then
    a new app with backend endpoint and persistent state, then
    a build that fetches from the network and nixpkgs, then
    a second computer adopting with customization, then
    a fleet security push across tracking, divergent-compatible,
    divergent-exempt, and divergent-conflicting computers.

boundaries:
  mutation_class: red
  authority_sources:
    - owner direction in session 2026-10-01 (deliverable statement)
    - docs/choir-doctrine.md
    - docs/computer-ontology.md (capsule, effect bundle, restore-set rules)
    - docs/reports/choir-platform-update-system-consensus-2026-09-08.md (stratified layers)
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned.
    - Provider credentials never enter a capsule.
    - Publication is source plus pinned inputs; no cross-tenant binary execution.
  excluded:
    - Binary distribution between computers (owner decision 2026-10-01; may be revisited as a trusted host build cache, never as tenant-to-tenant binaries).
    - Per-guest nixos-rebuild or per-tenant system closures for the base OS.
    - Marketplace, payments, or ranking of publications.
    - Multi-host fleet / distributed store.
  protected_surfaces:
    - guest boot path and runtime exec (S2)
    - VM networking / tap forwarding / capsule egress (S1, S3)
    - updater trust boundary and release manifest (S2, S5)
    - canonical event commit path (S5, S7, S8)
    - checkpoint / route projection (S5, S8)
    - platform-control signing domain (S7, S8)

now:
  status: working
  slice: >-
    Authoring. This file is a drafted metamission. Stations are intent
    only. Next is S0, a read-only reality probe that turns this file's
    source inferences into staging observations before any station is
    authored to the schema.
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
      The deliverable needs no new governance machinery. The M7/M11
      self-dev control plane, the M9a offer transport, and capsule effect
      bundles already carry it. What is missing is effect reach and
      openness:
      (a) the guest executes its own release, not the image baseline;
      (b) capsules get recorded egress and a private Nix store;
      (c) a preview bridge exists;
      (d) the release unit becomes base revision + patch stack + pinned
      inputs, which makes source publication and per-computer rebase the
      same object.
      If (a) through (d) land under a two-layer model (shared
      non-forkable base, per-computer forkable app layer), the whole
      deliverable follows.
    test: >-
      S0 confirms (a) is the actual blocker on staging: a Go-string
      change via self-dev applies but /health does not reflect it. Each
      later station is falsified if it needs a new authority path rather
      than a new effect path.
    edge: missing_oracle
    delta_o: >-
      A staging probe that asserts a runtime-observable effect (Go
      handler output) after self-dev apply, not just events and release
      digest.
    scope_if_supported: >-
      Single-host Choir Community Cloud staging; Go runtime + Svelte
      frontend + app backends.
    status: proposed
    evidence_refs:
      - docs/evidence/m11-probe-run9-satisfied-2026-09-29.json
      - docs/reports/nixos-agent-platform-redhat-deepseek-audit-2026-10-01.md
  decision:
    what: >-
      Source-only publication. Per-computer app layer expressed as
      (platform base rev + patch stack + pinned inputs), built locally in
      a capsule. Shared non-forkable base image.
    kind: architecture
    status: proposal
    evidence_ref: owner direction in session 2026-10-01
    owner_ratification_ref: pending (owner stated "source only" and the deliverable; layer split not yet ratified)
  belief:
    believed_state: >-
      The control plane works. The effect plane stops at the frontend.
      Capsules are closed to the network. Apps are monoliths. Publication
      does not exist. The VM network may be too open in exactly the
      wrong direction (between tenants) while the capsule is closed in
      the direction the deliverable needs (the internet, recorded).
    main_uncertainty: >-
      Whether the runtime-from-release cutover (S2) can keep the
      restore-set and boot-path invariants without per-computer Nix
      builds in the guest. Equivalently: can the capsule's go build of
      the runtime (cgo, ICU) produce an artifact that stays valid across
      base image updates, or must the per-computer app layer be a Nix
      closure built against the base?
    next_observation: >-
      S0 probe on staging: (1) self-dev apply of a Go handler change, then
      check /health or an endpoint; (2) tap->tap reachability between two
      test computers; (3) whether the runtime unit's children can read
      RUNTIME_GATEWAY_TOKEN.
  blocker_or_risk: >-
    S3 (open-world capsule) must not land before S1 (security floor).
    curl | bash inside a guest whose VM network reaches other tenants and
    whose runtime is root would turn a feature into a cross-tenant
    exploit path.
  next_action: >-
    Author S0 as a throughline station file (read-only probes, green/
    yellow) and run it. Problem-document any confirmed finding before
    any fix (CLAUDE.md Problem Documentation First).

receipts: []
---

# Supervised App Development and Source Publication — Metamission

## Why this is one metamission

Each station is a separable landing, but they share one object: **the
per-computer change**. It starts as a capsule experiment. It becomes a
committed release on one computer. It becomes a publication other
computers adopt. It becomes a candidate for the platform base. If the
stations each invented their own representation of "a change", the
pieces would not compose. The design commitment is that the same record
(base revision + patch stack + pinned inputs + build recipe + tests +
receipts) flows through every station.

## Two layers

- **Non-forkable base.** Kernel, systemd, capsule broker, updater,
  signers, kernel probe and network policy. One shared NixOS guest image,
  platform-owned. Security fixes here are image pushes and cannot
  conflict with anything, because no computer may diverge this layer.
  Policy: put security boundaries here wherever possible.
- **Forkable app layer.** The runtime, frontend, app modules and skills.
  Per computer, defined as base revision + patch stack. This is what
  self-dev edits, what gets published, and what security offers must
  rebase onto.

## Stations (intent)

- **S0 reality.** Read-only staging probes. Does a self-dev Go change take
  effect? Is tap-to-tap forwarding open? Can runtime children read the
  gateway token? Does the M9a push handle a real frontend-sized payload?
  Problem-document each confirmed finding first.
- **S1 security floor.** Per-tap FORWARD isolation and default-deny egress
  at the VM. Confine the guest runtime unit (non-root, ProtectSystem,
  delegated cgroup). Scrub the gateway token from child environments.
  Prerequisite for S3.
- **S2 runtime from release.** The guest runs the runtime from its own
  committed release, falling back to the image baseline. Each release
  records the base image it was built against. The updater refuses a
  release whose dynamic links do not resolve in the booted base. Decide
  here between a cgo build in the capsule and a capsule-built Nix
  closure.
- **S3 capsule open world.** Per-capsule egress through a recording proxy
  bridged into the capsule netns, so every fetch is logged with URL and
  content hash to the trajectory. Policy: open-with-recording by default,
  never reaching tap or host-private addresses. Add a capsule-private Nix
  store, either a store in the capsule upper or the Nix local-overlay
  store over the read-only base (experimental feature; verify) with
  substitution through the proxy. Result: curl | bash and nixpkgs work,
  and nothing leaves the capsule uncommitted.
- **S4 live preview + supervision.** A broker-bridged unix socket from the
  capsule's dev server to a desktop preview route. Texture work doc shows
  commands, fetches, tests and the preview link live while the capsule
  runs.
- **S5 commit gate, full release.** Tests plus owner approval (existing M7
  decision) materialize a full release: runtime + frontend + app backends.
  Prove backend effect after reboot and restore.
- **S6 app packages.** Dynamic app manifest (the 09-08 FrontendManifest
  idea), so a new app is a package (frontend module + optional backend
  service + manifest + recipe), not a monolith rebuild. Packages become
  the unit of divergence (`DivergedComponents`) and of publication.
  May run in parallel with S7.
- **S7 source publication (M9b).** Publish = patch + base revision +
  pinned inputs (every fetch from S3's log converted to a fixed-output
  hash) + recipe + tests. Host-side registry. Adopters review the diff
  and the inputs, then fork, customize, build in their own capsule, and
  land through their own gate.
- **S8 mainline + security push.** Host selects a publication, PRs it into
  main, and ships a new base. Security offers carry severity, deadline and
  an exploit regression test. Per computer: rebase the patch stack and
  run the exploit test. The outcome is one of auto-apply, exempt
  (already safe), proposal (agent rebase in capsule), or after the
  deadline fail-closed (revert the component or cut the capability, and
  notify the owner). Wire the M9a push into CI for tracking computers.

## Open decisions (record, do not block)

1. Capsule build of the runtime: plain toolchain (fast, fragile across
   base updates) or a Nix closure built in the capsule store (reproducible,
   slower first build). Leaning Nix for promotion, plain toolchain for
   iteration.
2. Default capsule egress policy: open-with-recording, or an allowlist.
   Leaning open-with-recording plus never-private-ranges, because the
   deliverable explicitly includes curl | bash.
3. Whether a trusted host build cache keyed by derivation hash is allowed
   later. It does not violate "source only" at the trust level (the
   builder is trusted, not the publishing tenant), but it is deferred.
4. Where S6 sits relative to S5. Additive new apps may be easier as
   packages first; modifying existing in-runtime apps needs S2 regardless.
