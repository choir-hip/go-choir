---
definition_version: 4
definition_id: choir-appdev-s2-layering-runtime-from-release-2026-10-01
execution_mode: mission_orchestrator
member_of: choir-supervised-app-development-metamission-2026-10-01
readiness: reviewed
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
      "Self-dev effect plane reaches only the served release." A capsule's
      release is what it writes beneath var/lib/artifact/release/, the updater
      swaps CHOIR_UPDATER_ROOT/current, and the runtime is exec'd from the
      image's Nix store
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:102-110).
    - >-
      "M9a platform->computer signed push exists and was proven on staging
      with a 1-file payload; nothing in CI calls it"
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:120-122).
    - >-
      "Every guest-image deploy reboots active computers"
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:130-134).
    - >-
      The guest runtime wrapper sets CHOIR_UPDATER_ROOT on persistent storage
      and execs the image-built autoputer binary
      (nix/autoputer-vm.nix:136-138).
    - >-
      ComputerSurface uses current/frontend below the updater root when it has
      an index.html, otherwise it may serve the immutable baseline frontend;
      only absence of both roots refuses service
      (internal/autoputer/computer_surface.go:13-25,69-87).
    - >-
      The updater already verifies a manifest, stages an immutable release,
      atomically changes current, restarts the service, probes health, and
      restores the prior release after a failed apply
      (internal/updater/updater.go:171-263,320-393,654-719).
    - >-
      M9a has a staging proof that a tracking computer accepted a signed push
      and restored through its pinned head
      (docs/evidence/choir-platform-update-push-restore-deployed-2026-09-26.md:5-8,17-41).
    - >-
      The owner-ratified spine makes the builder substrate an S2 dependency:
      S0b must choose host-side service, privileged builder-capsule, or scoped
      guest service before downstream closure-consuming stations proceed
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:538-543).

finish:
  deliver: >-
    A committed app change becomes a per-computer guest release that applies
    without rebooting that computer: the guest runs its committed runtime and
    frontend closure, while the platform-owned NixOS base remains shared.
  artifact: >-
    A deployed two-layer guest release path: a non-forkable NixOS base image;
    a per-computer app-layer Nix closure unpacked and GC-rooted on the guest
    data disk under the same store-path layout; a release record bound to its
    base-image identity and a GC-rooted retained per-computer guest-image
    reference; and an updater transaction joining executable, frontend, state
    compatibility, and effective event head. S2 lands the S0b-selected builder
    substrate before any closure-consuming acceptance. The route has no
    writable global Nix store or Nix daemon, rejects a closure unresolved by
    the booted base, restarts only the runtime for an app-layer swap, and makes
    M9a tracking pushes CI-driven.
  acceptance:
    - action: >-
        On staging, commit an app-layer release against the booted base and
        apply it through the updater; observe the new backend and frontend
        after the runtime becomes healthy, with the Firecracker process
        identity and guest boot id recorded unchanged across the swap.
      proves: >-
        A committed app-layer release applies by updater swap plus runtime
        restart, not VM reboot.
      evidence_class: deployed proof
    - action: >-
        From CI, drive a signed M9a app-layer offer to a staging tracking
        computer; record request-to-healthy time and observe the same
        updater-only apply while the Firecracker process and guest boot id are
        unchanged.
      proves: >-
        Tracking-computer platform pushes use the no-reboot release path and
        expose its time-to-healthy.
      evidence_class: deployed proof
    - action: >-
        With a predecessor serving on staging, present releases whose closure
        does not resolve against the booted base, whose accepted event head is
        stale, and whose tested content changes before activation; observe
        each refusal before pointer swap, runtime restart, or effective-head
        mutation, with the predecessor backend and frontend still serving.
      proves: >-
        Base resolution, stale-head, and post-test-mutation fences fail closed
        before a release can change the serving computer.
      evidence_class: deployed negative proof
    - action: >-
        After disposal of the selected builder's build environment and a guest
        reboot, present a release requiring a dependency absent from the
        booted base; observe refusal before mutation and the retained
        predecessor still serving.
      proves: >-
        A closure cannot rely on a disposed builder or silently substitute a
        base-absent dependency.
      evidence_class: deployed negative proof
    - action: >-
        On staging, inspect the guest after a successful apply: observe the
        closure at its per-computer GC-rooted store layout and establish that
        no writable guest-global Nix store or Nix daemon was introduced.
      proves: >-
        Materialization uses the required data-disk closure mechanism rather
        than weakening the guest store boundary.
      evidence_class: deployed proof
    - action: >-
        Apply a prior retained app-layer release through the same release
        authority after the successful swap; observe its backend/frontend and
        effective release identity restored while the guest boot id remains
        unchanged.
      proves: The release-retention and rollback path restores the prior release.
      evidence_class: deployed proof
  rollback: >-
    Revert and redeploy the platform change; for an affected computer, retain
    and select the prior GC-rooted release through the updater/accepted-head
    restore path, restart the runtime, health-check it, and refuse any release
    whose base, closure, event head, or compatibility join does not verify.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between a committed app change and a running guest while
    preserving base immutability, release provenance, event-head/state joins,
    and rollback; routine app updates must no longer require a VM reboot.
  goodharting_would_be: >-
    Restarting or recreating the VM while calling it a runtime restart, serving
    a host-global frontend, copying an unrooted closure outside the real store
    layout, or accepting a closure that only happens to work on one base.

homotopy:
  realism_axis: >-
    App-layer closure breadth, from the existing runtime plus one frontend
    change through a backend/frontend/state-compatible app release, while the
    same base-bound closure, updater transaction, retention, and no-reboot
    evidence remain in place.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified two-layer decision (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:425-437)
    - S2 station contract (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:523-543)
    - docs/computer-ontology.md:100-121
    - AGENTS.md:109-131
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
  excluded:
    - S0 reality/boot measurement and builder-substrate selection evidence; S2 lands the S0b-selected substrate
    - S1 networking, confinement, and credential-boundary work
    - S3 Firecracker snapshot/hibernate work
    - S4 capsule-private Nix-store and egress work
    - S6 owner commit gate and release materialization policy
    - S8 source publication, S9 fork/fleet construction, and S10 template installation
    - S11 divergent-fleet security offer classification
  protected_surfaces:
    - guest boot path and runtime exec
    - updater trust boundary and release manifest
heresy_delta:
  discovered: >-
    The current computer surface can fall back to the immutable baseline when
    a staged frontend is absent; a successful runtime restart alone therefore
    does not prove the committed release served its frontend
    (internal/autoputer/computer_surface.go:69-87).
  introduced: none — this station must not add a writable guest-global store, daemon, or parallel release authority.
  repaired: pending — require the executable/frontend/state/head serving join and its deployed negative proofs before completion.

now:
  status: working
  slice: >-
    Live. Full layering mechanism + transport landed end-to-end 2026-10-04:
    (1) base-join manifest (20880d75); (2) pure-Go narchive reader +
    GC-rooted store materializer (b68357a1), wired into Apply (a7052d2a);
    (3) guest runtime exec via recorded store-path entrypoint + mount-ns
    overlay (c7bb4a12 + 1be8bd72 stale-entrypoint fix); (4) producer layering
    join (f5460bdc); (5) CAS-ref transport for the ~146MB closure.nar —
    PlatformUpdateFile.ref + corpusd PUT/GET blob endpoint + guest
    FetchBlobRaw streaming (ef2e607d, convergent-panel adjudicated). Deployed
    image = 3142979b (exec) + ef2e607d (transport). Remaining: the deployed
    layering acceptance — PUT the nar blob, mint by ref, apply to a
    disposable, observe the guest exec the release store-path binary in the
    overlay + a base-mismatched offer fail closed.
    source_ref: main@ef2e607d
    deploy_identity: 'staging https://choir.news deployed_commit=ef2e607d; layering exec + CAS-ref transport live'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: base-bound-closure-runtime-swap
    claim: >-
      If S0b identifies one evaluable builder substrate and a per-computer
      closure can be rooted on the data disk with every base dependency
      resolving in the booted image, then an updater transaction can replace
      the runtime/frontend layer without rebooting or weakening release and
      event-head authority.
    test: >-
      On staging, CI drives an M9a release whose closure contains the committed
      backend and frontend change and records time-to-healthy; the updater
      refuses unresolved-base, stale-head, and post-test-mutated releases
      before mutation, refuses a base-absent dependency after builder disposal
      and reboot, accepts the compatible release with only a runtime restart,
      and restores the retained predecessor.
    edge: missing_oracle
    delta_o: >-
      S0b's disposable-computer probe records the selected evaluator and a
      closure containing a dependency absent from the base, plus its base Nix
      database/store-layout result.
    scope_if_supported: >-
      Tracking single-host staging computers running the shared NixOS base and
      a per-computer app-layer runtime/frontend closure.
    status: proposed
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:512-518
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:523-543
      - internal/updater/updater.go:125-285
  decision:
    what: >-
      Adopt the owner-ratified two-layer guest: shared non-forkable NixOS base
      plus per-computer app-layer closure. S0b selects the concrete builder
      substrate from its probes; S2 includes landing that selection before
      closure-consuming acceptance, rather than guessing or deferring it.
    kind: architecture
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:425-437
    owner_ratification_ref: >-
      owner 2026-10-01: "I like your suggestion for layering, and using guest
      NixOS closures appropriately"; "I ratify #1"
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:434-437)
  belief:
    believed_state: >-
      The existing updater already has manifest-verified release staging,
      pointer swap, runtime restart, health evidence, and prior-release
      recovery; the image runtime is still its execution baseline.
    main_uncertainty: >-
      Which S0b-validated evaluator can build the required closure without a
      writable guest-global store or daemon, and whether its dependency graph
      can resolve strictly against the booted base.
    next_observation: >-
      S0's recorded base/store layout and S0b evaluator selection, then S2's
      landed builder and a staging no-reboot app-layer apply with refusal and
      retained-predecessor evidence.
  blocker_or_risk: >-
    A closure that appears to start but bypasses the booted base, leaves the
    computer surface on immutable-baseline fallback, omits the
    frontend/state/event-head atomic join, or leaks a writable global store
    violates the updater trust boundary rather than providing a valid speedup.
  next_action: >-
    Full layering mechanism landed + exec image deployed (df23219c, staging).
    Next is the deployed layering acceptance (docs/problems/s2-runtime-exec-still-baseline):
    on a disposable staging computer, mint a layered platform-update
    (closure.nar carrying an app-layer binary + base_image_manifest_digest),
    transport it in, apply, and observe the guest process exec the release
    binary (store paths resolve through the overlay) — then a
    deliberately-broken release rolls back to the prior/base binary.

receipts: []
---

# S2 — Layering Runtime From Release

## Mechanism

- The base remains the platform-owned NixOS image: kernel, systemd, capsule
  broker, updater, signers, and network policy. The app layer is the runtime,
  frontend, app modules, and skills, matching the ratified split
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:479-489).
- An accepted release carries the base-image identity used to build it and a
  complete Nix closure. Materialization unpacks that closure on the guest data
  disk, roots it for GC, and executes it at its original store paths; it does
  not make the shared EROFS store writable or introduce a guest-wide daemon
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:523-533).
- Retain the computer's selected base-image reference as a GC-rooted guest
  image generation. A release refuses before mutation when that reference or
  any closure dependency cannot resolve against the actually booted base.
- The release activation is one updater authority transaction: executable,
  served frontend, state compatibility, and effective event head either join
  the same healthy release or leave the serving predecessor intact. This
  extends the updater's existing swap/restart/probe/recovery sequence rather
  than creating a parallel release switch
  (internal/updater/updater.go:198-285,674-719).
- M9a remains the signed transport and accepted-head authority; S2 changes its
  payload to an app-layer release and makes CI dispatch it for tracking
  computers. The deployed M9a record is evidence that signed push plus
  pinned-head restoration is a real product route, not evidence that it can
  already carry an app closure
  (docs/evidence/choir-platform-update-push-restore-deployed-2026-09-26.md:17-41).

## Builder dependency and handoff

- S0b selects, from disposable-computer evidence, one of the named host-side
  service, privileged builder-capsule, or scoped guest-service substrates;
  **S2 includes landing that S0b-selected substrate before
  closure-consuming acceptance**. It MUST NOT choose a substitute by
  convenience (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:538-543).
- The landed builder hands S2 a closure, its derivation/input evidence, and
  the base identity it resolves against. S2 hands S6 a materialization contract
  for full releases; it does not absorb S6's commit gate.
- The dangerous failure is a nominally fast swap that crosses the base or
  updater-trust boundary. Refusal and retained-predecessor recovery are the
  safety case; reboot avoidance is valuable only after that case holds.
