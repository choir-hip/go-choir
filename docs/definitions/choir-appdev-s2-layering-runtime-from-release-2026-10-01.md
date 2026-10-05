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
  repaired: >-
    Still pending (panel send-back upheld): the discovered baseline-fallback
    heresy plus three in-run finds — pre-mutation refusal wedging the pending
    transition (fixed 6fcb05e5 + regression test), stranded accepted events
    bound to a retired realization wedging the computer permanently (fixed
    0b9d5186 + regression test, fails-before/passes-after), updater/client
    discarding the computed refusal reason (fixed 297eedf1/63865ede) — and one
    new open wedge: a post-swap guest restart loop can kill Firecracker and
    cold-boot the disposable
    (docs/problems/s2-postswap-restart-loop-kills-vm-2026-10-05.md).
    Require before completion: the executable/frontend/state/head serving
    join by digest, its deployed negative proofs, a clean rollback with
    unchanged boot id, and a CI-driven M9a push with request-to-healthy.

now:
  status: checkpoint_incomplete
  slice: >-
    Gap-closure run 2026-10-05 ~11:30-13:30 on choir.news (deploy fae12950):
    criterion 6 CLOSED by a quiet-window clean rollback (rb-push 13:23:23,
    tape seq 112-114 accepted/started/applied, same fc pid 3123628 + boot
    id 7f725ae1, serving 2811c779 healthy); schema-window refusal CLOSED
    as a leg and fixed as a defect (daemon journals pre-mutation refusals
    as terminal Outcome=refused; agent discharges on refused); CI push
    CLOSED by mechanism (run 37305290512 green, phase ran, t2h recorded).
    Second consensus panel 6/6 send_back: two gaps remain, both specified
    ~10-minute legs — (a) frontend-by-digest proof via a marker inside the
    built SPA (the 2d0c16c0 marker attempt failed at the Nix cleanSourceWith
    filter: marker in repo-root index.html never enters frontend/dist;
    retry with the marker in built bytes), (b) reboot-then-push of a
    base-absent entrypoint. Station NOT closed.
  source_ref: main@14d2ac7d
  deploy_identity: 'staging https://choir.news deployed_commit=fae12950; guest base fae12950 coherent'
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
    status: testing
    evidence_refs:
      - docs/problems/s2-runtime-exec-still-baseline-2026-10-04.md
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:512-518
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:523-543
      - internal/updater/updater.go:125-285
  deploy_identity: 'staging https://choir.news deployed_commit=63865ede; guest base 63865ede coherent; deploy-receipt matches'
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
    status: testing
    evidence_refs:
      - docs/problems/s2-runtime-exec-still-baseline-2026-10-04.md
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
      Mechanism proven on staging: signed layered offer, CAS-ref
      transport, nar replay into the private store, overlay exec in the
      guest (layering-diag unshare_ok/mount_ok/exec_path_resolves), route
      promotion, base-mismatch refusal. The vm-3dc68688 crash-loop was a
      hand-staged stale binary (August 43310064 copied into /tmp/closure.nar
      on Node B, synthetic code_commit), not builder output. Contract
      defects (source-traced 2026-10-05, director review):
      (1) layering-entrypoint is global at the updater root, so
      restorePrior does not revert exec (internal/updater/closure.go:418,
      457-462; wrapper nix/autoputer-vm.nix:153);
      (2) no pre-mutation state-compat refusal beyond the base digest;
      (3) builder code_commit is caller-supplied and nothing binds the
      binary's buildinfo to the manifest;
      (4) self-dev still emits file releases, not builder input.
    main_uncertainty: >-
      Whether builder-produced runtime closures stay small as deltas
      against the base, and whether CI can drive builder -> mint -> push
      inside the deploy window.
    next_observation: >-
      A builder-produced release from a real commit, applied to a
      disposable computer alongside one deliberately incompatible release
      (refused pre-mutation) and one health-failing release (restored,
      exec included).
  blocker_or_risk: >-
    Hand-staged artifacts may answer mechanism questions only; station
    acceptance requires builder-produced releases (metamission evidence
    quality rule, v4). Each layered release adds a ~146MB nar to
    platform-artifacts with no GC; coordinate with SO before CI wiring
    multiplies it.
  next_action: >-
    Slices landed 2026-10-05: S2-e rollback atomicity (9f5aa8a0), S2-d
    state-compat gate (c8fb7834), S2-c provenance (8f06b3d9), S2-f
    builder join (e1c3924b), S2-g CI wiring (e527c169) — mechanism live
    in deploy, bind gaps fixed (a3da83c4) and proven end-to-end on
    computer-6450a253. Next: close the S2-g residual gates (canonical-head
    bootstrap authorization, realization staleness on hibernate) and run
    station acceptance per the conjecture test with builder-produced
    releases only.
    believed_state_update_2026_10_05: >-
      App-layer land proven: release 36743b1f applied on
      computer-6450a253b8b6ebc0866471973694f5be via signed offer +
      verifier_refs; build.commit 2f0e2cac served healthy;
      layering-entrypoint -> release bin/autoputer; current/ swapped
      atomically; code_commit 2f0e2cac + base_commit eb9c5b19 +
      builder_receipt_digest bound (S2-c). See
      docs/problems/s2-app-layer-offer-bind-gaps-2026-10-05.md.


receipts:
  - id: s2-acceptance-run-2026-10-05
    kind: station_evidence
    status: checkpoint
    commits: [6fcb05e5, 34041932, 297eedf1, 0b9d5186, 63865ede, 7d78b182, dac021cd]
    deployed: 'staging https://choir.news deployed_commit=63865ede'
    summary: >-
      Builder release 6fcb05e5 applied (tape seq 72 materialization_applied,
      serving 6fcb05e5 healthy, fc pid 2686228 + boot id d541d292 unchanged);
      six incompatible offers refused pre-mutation with discharge; panic
      release restored predecessor incl. exec. Consensus panel 7 send_back /
      1 approve — station NOT closed; gaps receipted in now.slice.
    evidence_refs:
      - /var/lib/go-choir/deploy-failures/s2-acceptance-20261005T*.jsonl (node B)
      - /tmp/s2-panel/manifest.tsv (panel run, 8 ok / 3 failed-or-timeout)
  - id: s2-postswap-restart-loop-wedge-2026-10-05
    kind: problem
    status: open
    summary: >-
      Rollback leg swap was correct but the post-swap guest restart loop
      killed Firecracker and cold-booted the disposable; criterion 6 needs
      one clean rollback leg with unchanged boot id.
    evidence_ref: docs/problems/s2-postswap-restart-loop-kills-vm-2026-10-05.md
  - id: s2-e-rollback-atomicity-2026-10-05
    kind: station_slice
    status: landed
    commits: [9f5aa8a0]
    summary: >-
      layering-entrypoint moved inside the release dir; current/ swap now
      moves served frontend and exec atomically and restorePrior reverts
      exec with the same pointer. Boot-loop guard added with host-readable
      tripped receipt.
  - id: s2-d-state-compat-gate-2026-10-05
    kind: station_slice
    status: landed
    commits: [c8fb7834]
    summary: >-
      Manifest declares store_schema_version + base_commit; Apply refuses
      before mutation on mismatch.
  - id: s2-c-provenance-2026-10-05
    kind: station_slice
    status: landed
    commits: [8f06b3d9]
    summary: >-
      Builder derives code_commit; manifest carries builder_receipt_digest;
      updater checks the entrypoint's embedded buildinfo commit.
  - id: s2-f-builder-join-2026-10-05
    kind: station_slice
    status: landed
    commits: [e1c3924b]
    summary: >-
      Self-dev freeze emits source.patch for runtime changes; builder
      consumes patches, not file releases.
  - id: s2-g-ci-app-layer-2026-10-05
    kind: station_slice
    status: landed
    commits: [e527c169, a3da83c4]
    summary: >-
      CI app-layer release push for tracking computers — no VM reboot for
      autoputer-only changes. Three sequential bind gates surfaced and
      fixed/proven: canonical head bootstrap, verifier_refs requirement,
      realization staleness on hibernate. End-to-end land proven on
      computer-6450a253b8b6ebc0866471973694f5be (release 36743b1f6ddb,
      build.commit 2f0e2cac).

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
