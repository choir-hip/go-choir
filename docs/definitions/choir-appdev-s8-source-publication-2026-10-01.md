---
definition_version: 4
definition_id: choir-appdev-s8-source-publication-2026-10-01
execution_mode: mission_orchestrator
readiness: reviewed
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
    receiving or executing the publisher's executable closure bytes.
  artifact: >-
    A staging host registry record containing the approved patch, base
    revision, fixed-output hashes whose payloads are retained in the
    CAS-backed S4 fetch record, build recipe, tests, and, for an S7 package,
    its manifest plus app-data compatibility and restore contract; plus B's
    distinct, owner-approved adopted app-layer release and canonical commit
    receipt.
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
        payloads. Before adoption, attempt an altered patch or input hash, a
        missing CAS payload, and a publication signature replayed as a
        platform offer; then attempt an adoption authorization bound to another
        recipient or B canonical head. Verify B receives no executable closure
        bytes from A; identical Nix path names are permitted only when B
        independently builds them or the authorized base supplies them.
      proves: >-
        Clean replay comes only from declared source inputs; publication
        signature/base binding cannot substitute for B's recipient-and-head
        adoption authority; invalid records refuse before B's canonical state
        or release changes; and B obtains no executable closure bytes from A.
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
    original origins remain online; executable closure bytes transferred from
    A, a review without a rebuild, or a forged record rejected after B has
    already mutated.

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
  heresy_delta:
    discovered: >-
      M9a's platform-control-signed update is an adjacent, verification-first
      transport, but it is not a computer-to-computer source publication or a
      recipient-owned adoption authority
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:120-123).
    introduced: >-
      No runtime heresy is introduced by this drafted goal; implementation risk
      is treating a discoverable publication signature as authority to mutate B
      or smuggling A's executable closure bytes through an apparent source
      record.
    repaired: >-
      Bind the generally discoverable publication signature to its source
      record and base, then require a separate B recipient-and-current-head
      adoption authorization before B's canonical event or release mutates;
      rebuild solely from declared inputs and B or authorized-base bytes.

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S6's full-release gate and S4's retained-payload receipt: bind the
    accepted change record to a source-only host registry and recipient-owned
    reconstruction/adoption.
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
      fixed-output inputs, recipe, tests, and any S7 package compatibility and
      restore contract, a differently owned computer can rebuild and adopt a
      customized result without receiving executable closure bytes from A.
    test: >-
      The deployed A-to-B publication proof rebuilds B with original origins
      unavailable, records only declared CAS input resolution, and shows
      B's tests and owner approval create B's own accepted release. Altered
      patch/input hashes, missing CAS payloads, and a platform-offer replay
      refuse before mutation; B adoption authority is bound to B's recipient
      and current head, separately from the discoverable publication record.
    edge: missing_oracle
    delta_o: >-
      A clean B capsule replay with A's executable closure bytes unavailable,
      original origins disabled, and an adoption receipt joining B's resolved
      declared inputs, recipient, and current head to B's accepted event and
      release while distinguishing independently built or authorized-base path
      names.
    scope_if_supported: >-
      Cross-owner publication of S6-approved app-layer changes on staging;
      package publications consume S7's manifest and app-data compatibility/
      restore contract, but this does not assert fork construction or
      platform-base updates.
    status: proposed
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:584-587
      - docs/definitions/choir-appdev-s7-app-packages-2026-10-01.md:55-60
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
      every input and binding needed for a clean B reconstruction without
      receiving A's executable closure bytes, and whether an observer can
      distinguish authorized-base or independently built path reuse.
    next_observation: >-
      The first S6-approved release joined to a completed S4 retained-payload
      receipt, then a clean B rebuild with all original origins and A's
      executable closure bytes unavailable.
  blocker_or_risk: >-
    S6-commit-gate-full-release is incomplete, and S4 must produce a
    retained-payload receipt for every fixed-output input before a publication
    can honestly promise origin-independent replay.
  next_action: >-
    Promotes to working when S6-commit-gate-full-release completes and S4's
    retained-payload receipt proves every fixed-output input retrievable from
    CAS; then map the accepted change and receipts onto the host registry and
    prove B's clean, customized source rebuild.

receipts: []
---

# S8 — Source Publication

## Mechanism

The cross-computer unit is the shared record, not an updater payload:
**base revision + patch stack + pinned inputs + build recipe + tests +
receipts** (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:470-477).

When S6 completes, it supplies the approved, restorable app-layer change. S8
serializes its source identity into a host-registry record and binds each
fetched dependency to S4's fixed-output hash and retained content-addressed
payload. S8 does not turn the record into an A-to-B executable release.

For an S7 package, the record also carries the package manifest and its
app-data compatibility and restore requirements; the package's own artifact
defines those as release-bound data
(docs/definitions/choir-appdev-s7-app-packages-2026-10-01.md:55-60).

B obtains the record through the registry, so review is of the patch, base,
recipe, tests, every pinned input, and package contract before any build or
adoption. B may modify that source in B's capsule. The resulting release must
enter B's own S6-shaped test and owner-approval gate, producing B's event and
pinned restore history rather than extending A's authority.

A discoverable publication signature binds its source record and base. It is
not B's adoption authority. A separate adoption transaction must bind B's
recipient identity and then-current canonical head before B's gate can commit
the customized result.

The release staging primitive already insists on both engineering authority and
a frozen capsule (internal/capsule/executor.go:1344-1360). That constraint
keeps S8 downstream of the accepted-release boundary instead of treating a
live workspace as publishable state.

## Verification boundary

The proof removes the original origins after A publishes. B resolves only the
declared fixed-output hashes from the captured payloads, then builds and tests
from source with a local change. The observation is stronger than a matching
digest: B receives no executable closure bytes from A. Identical Nix path
names are allowed only when B independently builds them or the authorized base
supplies them.

Before any B mutation, the registry/adoption path must reject an altered patch
or input hash, a missing captured payload, an invalid publication signature,
and a publication signature replayed as a platform offer. It must also reject
an adoption authorization bound to a different B recipient or canonical head.
This follows the existing verification-first precedent, whose platform update
checks binding and signature before mutation
(internal/agentcore/platform_update.go:59-90), but S8 establishes separate
source-publication and B-adoption semantics rather than reusing M9a's
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

A source-looking record can still smuggle executable closure bytes from A or
let a generally discoverable publication signature authorize B's adoption. A
fixed-output hash can also name content the registry cannot serve after its
origin disappears. The clean replay and pre-mutation refusal are therefore
release/adoption gates, not post-hoc audit claims.
