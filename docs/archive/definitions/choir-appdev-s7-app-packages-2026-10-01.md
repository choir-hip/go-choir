---
definition_version: 4
definition_id: choir-appdev-s7-app-packages-2026-10-01
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
      Apps are compiled into one SPA bundle via a static registry and app
      backends into the one autoputer binary; a new app currently requires
      rebuilding both monoliths
      (docs/archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md:116-118).
    - >-
      The frontend registry is a static APP_REGISTRY whose entries use source
      imports and whose launcher projections and app lookup derive from that
      array (frontend/src/lib/apps/registry.ts:55-65,266-283).
    - >-
      Capsule release staging currently requires both an executable
      bin/autoputer and frontend artifacts, so package installation must extend
      the S6 full-release artifact rather than create an alternate release
      path (internal/capsule/executor.go:1543-1559).
    - >-
      A release manifest already binds event-schema and reducer versions with
      its file digest; checkpoint binding first proves replay completeness and
      then derives frontend identity from the release files
      (internal/updater/updater.go:44-65;
      internal/agentcore/checkpoint_restore_bindings.go:11-31).

finish:
  deliver: >-
    An owner can install, run, update, disable, and purge a dynamically
    declared app package—frontend module with an optional backend service—on
    one computer without rebuilding the Choir monolith, while the package's
    data remains compatible and recoverable across its release history.
  artifact: >-
    A deployed app-package manifest and installed-package runtime: each package
    declares its frontend module, optional backend service, build recipe, and
    app-data schema compatibility and restore requirements; the desktop loads
    enabled packages dynamically and the S6 release binds their bytes,
    compatibility, and effective event head as one committed release.
  acceptance:
    - action: >-
        On staging, install through the owner-approved S6 release path a new
        package containing a frontend window and backend effect, then open the
        window and exercise the backend without rebuilding the guest monolith.
      proves: >-
        A full-stack package is dynamically installed and served from the
        committed app release, not compiled into the baseline SPA or binary.
      evidence_class: deployed proof
    - action: >-
        Reboot and resume the computer after installation, reopen the package,
        and read the data it wrote. Then restore first to a checkpoint before
        installation and confirm the package is absent, and separately to a
        package-bearing checkpoint and reopen it with its matching data.
      proves: >-
        Reboot and resume preserve the installed package, while pinned-head
        restore selects the corresponding package presence and compatible data
        state rather than claiming every restore retains the package.
      evidence_class: deployed proof
    - action: >-
        Deliberately interrupt the package's app-data migration during a staged
        update, then recover by the declared release procedure and inspect the
        package data and effective release.
      proves: >-
        Migration interruption cannot leave an unclassified half-migrated
        package; recovery is either an idempotent completion or a return to the
        prior compatible release and data state.
      evidence_class: deployed proof
    - action: >-
        Disable the installed package, confirm its desktop module and optional
        backend effect are unavailable, then purge it through the committed
        package lifecycle and verify it no longer appears in the installed
        package set.
      proves: Package disable and purge are real lifecycle transitions.
      evidence_class: deployed proof
    - action: >-
        Attempt staging activation from a mutable manifest URL or a direct
        registry injection that is absent from the accepted package release.
      proves: >-
        Dynamic package activation refuses bytes and declarations not bound to
        the accepted, content-addressed release manifest.
      evidence_class: deployed negative proof
  rollback: >-
    Refuse an incompatible manifest before activation; use the S6-bound
    pinned-head restore to return to the prior package release and its declared
    compatible data state. Revert and redeploy a faulty package-runtime change;
    preserve the failed migration receipt for diagnosis rather than deleting it.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between an independently releasable app package and a
    monolithic frontend/backend rebuild while preserving one committed,
    restorable event-and-data state for every enabled package.
  goodharting_would_be: >-
    A registry entry dynamically added only at build time, a frontend-only
    screenshot, or a manifest that loads a module while its backend and data
    migration still bypass the committed release and restore path.

homotopy:
  realism_axis: >-
    Package completeness: (owner 2026-10-14) every existing first-party app
    converted to a package first, then a manifest-listed frontend-only module, then the same
    module dynamically loaded from its manifest, then an optional backend
    service, then persistent app data with a compatible migration, then
    reboot/resume/restore and interrupted-migration recovery, then
    disable/purge through the committed lifecycle.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission, S7 app-packages intent (docs/archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md:581-583)
    - owner-ratified two-layer app-release architecture (docs/archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md:425-437,479-489)
    - docs/choir-doctrine.md
    - docs/computer-ontology.md
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
  excluded:
    - S2-layering-runtime-from-release guest base/app-layer materialization and updater implementation.
    - S3-fast-resume machine-snapshot lifecycle and timing work.
    - S4-capsule-open-world egress and capsule-private Nix-store work.
    - S5-live-preview-supervision desktop-to-capsule preview bridge.
    - S6-commit-gate-full-release owner approval, canonical event commit, and full-release materialization machinery.
    - S8-source-publication publication transport, cross-computer adoption, and registry operations.
    - S9-forks-and-fleets package inheritance across sibling computers.
  protected_surfaces:
    - canonical event commit path
  station_added_scope_extensions:
    - >-
      Package disable and purge are deliberately added lifecycle scope: they
      make package divergence reversible rather than leaving enabled code
      outside the pinned-head contract
      (docs/archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md:341-343).
    - >-
      Package app-data migration compatibility and interruption recovery are
      deliberately added scope: release restore must cover package state, and
      S10's curated seed data makes unversioned package schemas non-portable
      (docs/archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md:341-343,602-604).
  heresy_delta:
    discovered: >-
      Static registry entries and monolithic release contents cannot represent
      package-level divergence, schema compatibility, or package lifecycle as
      release-bound evidence.
    introduced: >-
      No runtime heresy is introduced by this drafted goal; the implementation
      risk is a second mutable package authority beside the canonical release.
    repaired: >-
      Require every installed, disabled, purged, migrated, and restored package
      state to be selected by the accepted release manifest and its canonical
      event head.

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S6-commit-gate-full-release: specify the dynamic package manifest
    and its release-bound data evolution contract without creating a second
    commit or activation path.
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
    id: package-manifest-is-one-release
    claim: >-
      A manifest-derived dynamic frontend module can be introduced first and
      can remain the same package object when an optional backend service and
      app-data migration are added, because S6 binds the whole package release
      to the canonical event head and S2 provides its app-layer materialization.
    test: >-
      First prove a manifest-derived frontend-only module on staging, then use
      the same manifest shape for the full-stack install, reboot/resume/restore,
      interrupted migration recovery, disable, and purge acceptance path.
    edge: missing_oracle
    delta_o: >-
      A deployed package inventory and release receipt that jointly show the
      loaded frontend module, optional service, app-data schema state, release
      digest, and effective event head before and after each lifecycle action.
    scope_if_supported: >-
      Staging computers running the S2 app layer and S6 owner-approved releases;
      it does not assert cross-computer publication or fork compatibility.
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Treat the package manifest, its frontend module, optional backend service,
      recipe, and app-data evolution contract as one release-bound package
      object; evaluate the frontend-only manifest-derived import-map path before
      full-stack packages use S2/S6 machinery.
    kind: architecture
    status: proposal
    evidence_ref: >-
      S7 intent and two-layer design
      (docs/archive/definitions/choir-supervised-app-development-metamission-2026-10-01.md:479-489,581-583).
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      Current static registry and release staging are monolithic, but the
      existing manifest, updater, replay, and checkpoint bindings are the
      correct substrate for one package lifecycle rather than a parallel loader.
    main_uncertainty: >-
      Whether a manifest-derived frontend-only module can load under the staged
      SPA serving and integrity model without loosening the release, event, or
      restore authority—and what exact package-data compatibility evidence is
      sufficient after an interrupted migration.
    next_observation: >-
      S6's deployed full-release proof and a staging frontend-only dynamic
      module experiment that records the package inventory and release binding.
  blocker_or_risk: >-
    Dynamic loading can accidentally make enabled code or app data diverge from
    the canonical accepted release; a partial data migration can make a
    pinned-head restore claim false unless compatibility and recovery attach to
    the package release itself.
  next_action: >-
    Promotes to working when S6-commit-gate-full-release completes; then prove
    the manifest-derived frontend-only module path before extending the same
    package object to its optional backend service and app-data evolution.

receipts: []
---

# S7 — App Packages

## Mechanism sketch

- The package is a release member, not a new deployment channel.
- Its manifest names one frontend module, an optional backend service, its recipe,
  and the data compatibility and restore obligations for that release.
- Start with the manifest-derived import-map route for a frontend-only module;
  retain that same manifest identity when adding the optional service.
- Package bytes remain inside the S6 full-release closure rather than a mutable
  module directory. Current staging copies only paths beneath
  `var/lib/artifact/release/` and refuses unsafe or secret-bearing files
  (internal/capsule/executor.go:1371-1451).
- The existing surface serves `current/frontend` and falls back only to the
  immutable baseline when that staged frontend is unavailable
  (internal/autoputer/computer_surface.go:11-26,69-87).
- Therefore dynamic package discovery must be derived from the accepted,
  content-addressed release manifest, not from a separately writable registry.
- The existing release manifest already carries its accepted event head,
  code/artifact refs, schema and reducer versions, and hashes every file
  (internal/updater/updater.go:38-65).

## Data evolution and recovery

- A package data migration is part of package activation, never an untracked
  backend-side side effect.
- The package declares what schema state it reads and writes, which neighboring
  release data states it can read, and how a restore reconstructs that state.
- Activation must make the package module, service, data compatibility record,
  and effective event head agree or refuse before exposing the package.
- Interrupted migration recovery must record whether it resumed idempotently or
  restored the preceding compatible package release; it must not guess from a
  partially changed database.
- Existing rematerialization refuses an invalid target, reconstructs a staged
  store through the canonical target head, and bounds recovery replay
  (internal/agentcore/rematerialize.go:98-180).
- Existing checkpoint binding already couples replay completeness with the
  release digest and frontend file identity
  (internal/agentcore/checkpoint_restore_bindings.go:11-31); S7 extends that
  release contract to the package's data compatibility evidence.

## Commit, lifecycle, and handoff

- S6 remains the sole authority that turns a frozen capsule result into an
  owner-approved full release. S7 supplies the package object S6 commits.
- The platform update path demonstrates the required posture: verification and
  canonical-head binding gate mutation, and its receipt is the event chain
  rather than the HTTP body (internal/agentcore/platform_update.go:25-34,59-63).
- On failed materialization, the current updater restores the prior release and
  appends an auditable failed transition
  (internal/agentcore/platform_update.go:604-635); package migration failure
  must preserve an equally legible prior-compatible disposition.
- Disable and purge are committed package lifecycle states. They must remove the
  package from dynamic discovery and prevent its optional service from running;
  purge additionally removes its declared installed state only through the same
  release-bound authority.
- S8 consumes the shared S6 change record for source publication and adoption;
  package-specific publication integrates after S7 supplies a manifest, recipe,
  tests, and data-evolution contract that an adopter can build and validate.

## Key risk

A dynamic module loader that trusts a mutable URL or a database row would split
what users see from the bytes, event head, and recovery record that the updater
actually committed. The first frontend-only proof is a discriminator, not a
demo: it must prove that the future full-stack package has no second authority
path.

## Appagent egress grants (addendum 2026-10-09, owner direction)

Appagents (Texture first; email next) request durable network grants in
their package manifest. The grant object, the effect classes and the
data-flow rule are defined once in S4 ("Egress capability model"). S7 adds:

- manifest-declared grants;
- owner approval at install;
- re-approval with a visible diff when an upgrade widens a grant;
- renewal;
- operation contracts for `visible-write` endpoints.

The manifest's `egress` and `ingress` blocks, the install and upgrade
approval rules, and the rule that ingress always lands on a host service and
reaches the desk as records are in
[app-agent-network-policy-2026-10-21.md](../../design/app-agent-network-policy-2026-10-21.md) (NET-10).

## Owner direction, 2026-10-14: convert the existing apps first

"We need to do the app packages upgrade even before we build anything new.
We need to apply that to all the existing apps that we have." So S7's first
slice (S7-0) converts Choir's first-party apps, today compiled into one SPA
through the static registry (`frontend/src/lib/apps/registry.ts`), into
packages under the same manifest, release and data-compatibility contract
this file defines: Texture, Mail, Calendar, Files, Autopaper, Web Lens and
every other registry entry. New apps (the rebuild candidates in
`docs/research/puter-everything-2026-10-14/README.md`) come only after
S7-0. Each package's app-data migration should target the shared
object-graph kinds named in that review (one generic app-document kind,
`choir.party`, `choir.calendar_event`, ...), and package bytes are fetched
on demand from a host content-addressed cache (the same pattern as the
capsule Go module proxy) and distributed by station LU (guest updates).
Sequencing: in the ledger metamission, S6 and S7 come after the Autopaper
scaling work (owner priority), not before. (Superseded by owner notes §28:
S6, then S7-0.)

## App packages carry app-agent desks (addendum 2026-10-21, owner direction)

Owner notes §5 and §36: each app agent is its own RLM desk, app agents get
their own desks, and many more app desks are coming (Mail, Calendar,
Slides, a video editor). The manifest above has no desk component, and the
email desk design
([agentic-email-desk-2026-10-14.md](../../reports/agentic-email-desk-2026-10-14.md))
never mentions packages, so the two plans are related but not yet joined.
When S7-0 is designed:

- An app package may carry its app desk as a desk manifest (owned object
  kinds, verbs, role table, sandbox profile and taint class, egress verbs,
  compute and memory class, model policy, addressing, wake and dormancy,
  restart contract); install registers the desk, uninstall retires it.
- The manifest-declared grants in the addendum above attach to that desk.
- Per-app sole writers: the desk is the one writer of its app's object
  kinds, as Texture is for documents.

Design of the join, with the fields it needs from the desk manifest and the
failure modes of the install:
[app-package-carries-app-desk-2026-10-21.md](../../design/app-package-carries-app-desk-2026-10-21.md).

Items ARCH-01 to ARCH-05 of the
[security, density and architecture checklist](../../security-density-architecture-checklist-2026-10-21.md) track
this. Still owed in this file: owner ruling D1 (whether a package is
installed is a tape fact, not part of the release closure; owner notes §23)
and the edits listed in `choir-lp-s6-commit-gate-2026-10-20.md:391-397`.
