---
definition_version: 4
definition_id: choir-appdev-s8-source-publication-2026-10-01
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
      "M9a platform->computer signed push exists and was proven on staging
      with a 1-file payload; nothing in CI calls it. No computer->computer
      publication surface exists (M9b unbuilt)."
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:120-123)
    - >-
      "Capsules are air-gapped (empty netns, internal/capsule/namespace.go);
      the guest Nix store is read-only EROFS with no nix on the runtime PATH.
      curl | bash and nixpkgs are impossible inside a capsule today."
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:112-115)
    - >-
      StageGrantedRelease permits an engineering capability to stage only from
      a frozen capsule, so S8 must use S6's accepted release rather than a
      mutable capsule filesystem (internal/capsule/executor.go:1344-1360).
    - >-
      The existing platform-update path verifies signature, structure, expiry,
      lineage policy, and head binding before mutation; its platform-control
      evidence cannot be relaxed for publication or adoption
      (internal/agentcore/platform_update.go:59-90).

finish:
  deliver: >-
    A computer owner can publish an approved app-layer change as source and a
    different owner can inspect, customize, rebuild, and adopt it without
    executing the publisher's closure.
  artifact: >-
    A staging host registry record containing the approved patch, base
    revision, fixed-output hashes whose payloads are retained in the
    CAS-backed S4 fetch record, build recipe, and tests; plus B's distinct,
    owner-approved adopted app-layer release and canonical commit receipt.
  acceptance:
    - action: >-
        On staging, computer A publishes its approved change. Computer B,
        owned by a different owner, lists the host-registry entry, reviews the
        patch and every pinned input, adds a local customization, builds from
        the declared source in B's own capsule, passes B's tests, and B's
        owner approves the adoption.
      proves: >-
        The product path transfers inspectable source inputs rather than an
        executable closure and admits a divergent, recipient-owned result
        through B's normal gate.
      evidence_class: deployed proof
    - action: >-
        Repeat B's adoption rebuild after disabling every original fetch
        origin; resolve declared fixed-output hashes from the CAS-captured
        payloads. Attempt a forged or wrong-recipient/wrong-base publication
        before adoption, and inspect B's materialization record for any path
        from A's closure.
      proves: >-
        Clean replay comes only from declared source inputs, no A closure path
        substitutes on B, and invalid publication authority or binding refuses
        before B's canonical state or release changes.
      evidence_class: deployed proof
  rollback: >-
    Retract the host-registry record and git-revert/redeploy the publication
    service if needed; B retains only its separately committed adoption and
    restores it through B's pinned-head path. Refused records never create an
    adoption or mutate B.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between a reviewed change on A and a differently owned B
    independently rebuilding, understanding, customizing, and approving that
    change, while preserving source provenance, recipient authority, and
    reversibility.
  goodharting_would_be: >-
    A registry entry that merely links A's release or succeeds only while the
    original origins remain online; a copied A store path, a review without a
    rebuild, or a forged record rejected after B has already mutated.

homotopy:
  realism_axis: >-
    The proportion of each identical publish/adopt record independently
    reconstructed from its declared patch, base, recipe, tests, and
    content-addressed inputs: from all inputs locally available through the
    same registry flow to every original origin unavailable and every input
    resolved from the retained CAS payloads on a differently owned computer.

boundaries:
  mutation_class: red
  authority_sources:
    - owner direction in session 2026-10-01 (source only)
    - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:425-437
    - docs/computer-ontology.md
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - Provider credentials never enter a capsule.
    - Publication is source plus pinned inputs; no cross-tenant binary execution.
  excluded:
    - S4-capsule-open-world egress proxy, fetch capture, and capsule-private store construction
    - S6-commit-gate-full-release release materialization and owner-approval gate construction
    - S7-app-packages package manifest design
    - S9-forks-and-fleets sibling-computer creation and fleet admission
    - S11-mainline-and-security-push platform-base selection and security offers
  protected_surfaces:
    - canonical event commit path
    - platform-control signing domain

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S6's full-release gate: bind its accepted change record to a
    source-only host registry and recipient-owned reconstruction/adoption.
  source_ref: main@8aa1dce9
  deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00;
    owner guest computer-03335285269bdba4f94377e56879f9e6 on a3cfaa00'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: source-only-replayable-adoption
    claim: >-
      If a publication binds the S6-approved patch and base to S4-captured,
      fixed-output inputs, recipe, and tests, a differently owned computer can
      rebuild and adopt a customized result without executing or substituting
      any publisher closure.
    test: >-
      The deployed A-to-B publication proof rebuilds B with original origins
      unavailable, records only declared CAS input resolution, and shows
      B's tests and owner approval create B's own accepted release; forged or
      misbound records refuse before any B mutation.
    edge: independence
    delta_o: >-
      A clean B capsule replay with A's closure unavailable, original origins
      disabled, and a materialization receipt that joins B's resolved declared
      inputs to B's accepted event and release.
    scope_if_supported: >-
      Cross-owner publication of S6-approved app-layer changes on staging;
      not binary distribution, fork construction, or platform-base updates.
    status: proposed
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:584-587
      - internal/agentcore/platform_update.go:59-90
  decision:
    what: >-
      Publication is source only: patch plus base revision, pinned inputs,
      recipe, and tests; adoption rebuilds and commits on the recipient's own
      computer through its own gate.
    kind: architecture
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:425-433
    owner_ratification_ref: >-
      docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:434-437
      (owner 2026-10-01: "source only")
  belief:
    believed_state: >-
      M9a proves an adjacent verification-first signed update path, but M9b
      lacks a computer-to-computer source publication and adoption surface.
    main_uncertainty: >-
      Whether S4's captured payloads and S6's accepted-release record provide
      every input and binding needed for a clean B reconstruction without a
      hidden dependence on A's closure.
    next_observation: >-
      The first S6-approved release joined to S4 fetch receipts, then a clean
      B rebuild with all original origins and A's closure made unavailable.
  blocker_or_risk: >-
    S6-commit-gate-full-release is incomplete, and S4 must retain fetched
    payloads behind the fixed-output hashes before a publication can honestly
    promise origin-independent replay.
  next_action: >-
    Promotes to working when S6-commit-gate-full-release completes; then map
    its accepted change record and S4 fetch receipts onto the host registry
    contract and prove B's clean, customized source rebuild.

receipts: []
---

# S8 — Source Publication

## Mechanism

The cross-computer unit is the shared record, not an updater payload:
**base revision + patch stack + pinned inputs + build recipe + tests +
receipts** (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:470-477).

S6 supplies the approved, restorable app-layer change. S8 serializes its
source identity into a host-registry record and binds each fetched dependency
to S4's fixed-output hash and retained content-addressed payload. S8 does not
turn the payload into an A-to-B executable release.

B obtains the record through the registry, so review is of the patch, base,
recipe, tests, and every pinned input before any build or adoption. B may
modify that source in B's capsule. The resulting release must enter B's own
S6-shaped test and owner-approval gate, producing B's event and pinned restore
history rather than extending A's authority.

The release staging primitive already insists on both engineering authority and
a frozen capsule (internal/capsule/executor.go:1344-1360). That constraint
keeps S8 downstream of the accepted-release boundary instead of treating a
live workspace as publishable state.

## Verification boundary

The proof removes the original origins after A publishes. B resolves only the
declared fixed-output hashes from the captured payloads, then builds and tests
from source with a local change. The observation is stronger than a matching
digest: B's adopted executable and release path are B-owned, and no store path
from A's closure appears in B's materialization.

Before any B mutation, the registry/adoption path must reject an invalid
signature, a record bound to another recipient or base, an altered patch or
input hash, and a missing captured payload. This follows the existing
verification-first precedent, whose platform update checks binding and
signature before mutation (internal/agentcore/platform_update.go:59-90), but
S8 must establish its own source-publication semantics rather than reuse M9a's
platform-follow authority.

## What changes hands

A sends only a declarative source record through the host registry. The record
contains no executable closure and grants no authority on B. B contributes a
review decision, local customization, local build, test evidence, and owner
approval; B's gate is the sole admission route to B's desired code.

S4's capsule egress recorder and CAS are a prerequisite because its declared
purpose is to capture fetched payloads so S8 can rebuild while origins are
unavailable (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:560-573).
S8 deliberately leaves transport policy, package shape, fork construction, and
platform mainlining to their named stations.

## Key risk

A source-looking record can still smuggle binary dependence if B substitutes an
A closure or if a fixed-output hash names content that the registry cannot
serve after its origin disappears. The clean replay and pre-mutation refusal
are therefore release/adoption gates, not post-hoc audit claims.
