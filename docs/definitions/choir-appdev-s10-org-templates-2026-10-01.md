---
definition_version: 4
definition_id: choir-appdev-s10-org-templates-2026-10-01
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
      Semantic snapshots already exist: ProjectionBase blobs plus replay
      watermark W with PlanRecovery; the owner computer blob is about 16 GB
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:135-140).
    - >-
      Publication remains source-only in the intended cross-computer path:
      an adopter builds in its own capsule and no binary from the publisher
      executes on the adopter (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:235-239).
    - >-
      Encryption at rest is an open prerequisite before any export; current
      on-host hibernate protection is root-only permissions, not export
      protection (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:653-654).

finish:
  deliver: >-
    An organization can export a deliberately selected, encrypted semantic
    template from one computer and install it as a new computer that builds
    the selected source without inheriting the exporting computer's private
    data classes or identity material.
  artifact: >-
    A staging-accessible encrypted org-template record containing a base
    revision, patch stack, curated seed-data selection, policy, source-build
    inputs, and encryption-key-binding plus integrity-binding receipts; its
    installation creates a fresh computer with independently minted identity
    material.
  acceptance:
    - action: >-
        On staging, export a parent computer as an encrypted template, install
        it through the product path as a fresh computer, build the template's
        source there, and run the resulting computer.
      proves: >-
        The template is portable semantic content that produces a usable fresh
        computer by source build rather than execution of the parent's binary
        or machine image.
      evidence_class: deployed proof
    - action: >-
        Attempt to read the stored export without its authorized decrypt
        capability, then attempt install of a tampered or recipient-misbound
        encrypted template; capture the encryption and integrity-binding
        receipts and verify both attempts refuse before any fresh-computer
        mutation.
      proves: >-
        Stored template confidentiality and ciphertext-to-intended-install
        binding fail closed instead of being discovered after a child computer
        exists.
      evidence_class: deployed negative proof
    - action: >-
        Seed excluded-class canaries and a deleted-data remnant on the parent;
        after authorized decryption, scan the template plaintext at that
        boundary and the fresh installed computer disk, Dolt history, logs,
        and artifact graph, while checking distinct ComputerID and fresh
        signer, privacy, and gateway identity material.
      proves: >-
        Neither parent private data classes nor keys crossed the template
        boundary, including recoverable deleted remnants.
      evidence_class: deployed proof
  rollback: >-
    Retract the template record to prevent future installs and retain the
    parent unchanged; retraction does not revoke or delete independently
    installed computers. Disable the template's future decrypt/import
    authority if an export or scan receipt fails. Revert and redeploy any
    platform implementation change through the normal landing loop; an
    installed computer is reclaimed only through its own lifecycle authority.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between an organization's explicitly selected reusable
    computer setup and a fresh independently identified computer that can build
    it from source, while preserving complete exclusion of parent-private data
    classes and identity material.
  goodharting_would_be: >-
    Calling a reflinked disk, encrypted VM snapshot, or metadata flag a
    template; it may install quickly while retaining parent keys, deleted
    records, private blobs, or a runnable parent closure.

homotopy:
  realism_axis: >-
    Template fidelity — from a small allowlisted seed with one patch through
    the same encrypted semantic record carrying the selected base revision,
    patch stack, curated seed data, policy, and source-build inputs for a
    running fresh computer.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission, docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:153-294
    - org-template station intent, docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:602-604
    - docs/computer-ontology.md:54-61
  must_preserve:
    - 'Publication is source plus pinned inputs; no cross-tenant binary execution.'
    - 'A fork never holds its parent''s identity material.'
    - 'Every committed change is restorable through the pinned-head path.'
    - 'The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.'
  excluded:
    - whole-disk same-owner fork construction and fleet admission (S9-forks-and-fleets)
    - source publication review and adopter commit gate (S8-source-publication, S6-commit-gate-full-release)
    - machine-snapshot hibernate and resume (S3-fast-resume)
    - trusted host build-cache design (deferred open decision)
  transitive_install_prerequisites:
    - >-
      S2 builder-substrate receipt: before S10 becomes executable, the S2
      landing receipt must identify the working Nix-closure evaluation
      substrate required by S10 install
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:538-543).
  protected_surfaces: [identity material minting]
  heresy_delta:
    discovered: >-
      An encrypted stored export or a post-copy scrub alone does not establish
      private-data exclusion or ciphertext-to-install binding; deleted data
      and misbound template bytes can survive those proxies.
    introduced: >-
      No runtime heresy is introduced by this drafted goal; implementation
      risks creating a second computer-creation or identity authority outside
      S9's sibling-computer path.
    repaired: >-
      Require allowlisted semantic construction, encryption and integrity
      binding receipts, pre-mutation refusal, authorized-boundary plaintext
      scanning, and fresh identity minting through the existing path.

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S9's sibling-computer construction and re-keying contract and the
    transitive S2 builder-substrate receipt; S10 will define and implement an
    encrypted allowlisted semantic export and fresh-computer install without
    treating machine state as a template.
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
    id: semantic-template-is-sufficient
    claim: >-
      An encrypted allowlisted semantic record of base revision, patch stack,
      curated seed data, policy, and source-build inputs is sufficient to
      recreate a useful organization setup as a fresh computer without copying
      parent private data classes, keys, or executable closures.
    test: >-
      Export a parent on staging; prove unauthorized decrypt and tampered or
      misbound import refuse before fresh-computer mutation; then authorizedly
      decrypt, source-build, install, and independently scan template
      plaintext and installed state for excluded-class canaries, deleted
      remnants, parent keys, and parent closure substitution.
    edge: missing_oracle
    scope_if_supported: >-
      Choir Community Cloud staging templates whose selected source can be
      built against the receiving computer's declared base and pinned inputs.
    status: proposed
    delta_o: >-
      Recorded encryption-key and integrity-binding receipts, refusal evidence
      before child mutation, and an offline scanner covering authorized-boundary
      template plaintext, installed disk, Dolt history, logs, and artifact
      graph, with seeded excluded-class and deleted-data witnesses.
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:253-258
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:271-285
  decision:
    what: >-
      Sequence org-template work after S9 supplies sibling-computer identity
      minting and selective semantic-export boundaries.
    kind: operational
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:70-74
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      Semantic snapshots are the portable form; machine snapshots contain live
      secrets and are tied to one computer, so an org template must be a
      selected semantic reconstruction rather than a copied disk.
    main_uncertainty: >-
      Whether the selected semantic record contains every reproducible build
      input and useful seed while encryption, integrity binding, and the
      authorized-boundary offline scan expose no retained private class or
      identity material.
    next_observation: >-
      S9's landed allowlisted export and fresh-identity receipts plus S2's
      builder-substrate receipt, followed by an encrypted template
      export/install refusal and scan proof on staging.
  blocker_or_risk: >-
    A post-copy scrub is not evidence of exclusion: deleted rows and
    unallocated blocks can retain data, so template construction must start
    from an allowlisted semantic export rather than a parent disk copy.
  next_action: 'Promotes to working when S9-forks-and-fleets complete.'

receipts: []
---

# S10 — Org Templates

## Mechanism

An org template is a portable semantic reconstruction record, not a machine
snapshot. The latter is exact but computer-bound and contains live secrets;
the former is the stated distribution form (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:490-498).

The export boundary starts from an explicit allowlist: base revision, patch
stack, source-build inputs, curated seed-data classes, and policy. It records
what is selected and rejects every unselected class before encryption. It does
not copy a parent disk and then attempt a deletion pass.

The installed computer resolves the selected base and pinned inputs, builds the
selected source locally, and activates only its own resulting release. Parent
closures are never a substitute; the existing cross-computer acceptance shape
requires that no publisher binary execute on an adopter (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:235-239).

Fresh identity minting precedes child runtime or network access, following the
S9 construction invariant for ComputerID, signer keys, privacy key, and gateway
token (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:587-599). The template contains no identity material.

Encryption protects the exported semantic package at rest. Root-only host file
permissions are insufficient for this boundary; the metamission explicitly
requires at-rest encryption before export (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:653-654).

## Evidence and risk

The installer proof includes a real source build and running fresh computer,
not a successful decrypt or a copied-image boot. It first records refusal of
unauthorized decrypt and tampered or misbound ciphertext before child mutation,
with encryption and integrity bindings identified. The authorized decrypt
boundary then scans template plaintext and fresh state for an excluded-class
canary, deleted-data remnant, parent keys, and parent closure paths across disk,
Dolt history, logs, and the artifact graph
(docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:271-285).

Current publication export metadata sets `private_material_omitted`, but that
metadata assertion is not a scrub proof and does not define an org template
(`internal/platform/export_formats.go:61-82`). S10 must add artifact-level
selection and scan evidence rather than promoting that presentation-export
field into a security claim.

The handoff from S9 is its allowlisted semantic export discipline and fresh
identity receipts. The handoff to S11 is a reusable, encrypted, independently
rebuildable organizational baseline; S10 does not decide fleet security offers
or mainline policy.
