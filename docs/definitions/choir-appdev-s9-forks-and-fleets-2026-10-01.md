---
definition_version: 4
definition_id: choir-appdev-s9-forks-and-fleets-2026-10-01
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
      Semantic snapshots already exist: ProjectionBase blobs + replay
      watermark W with PlanRecovery (rebase/resume/refuse, tail bound
      10,000) landed 9341b5d1; the owner computer's blob is about 16 GB
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:136-140).
    - >-
      VM state lives on btrfs, so reflink copies of data.img are available;
      Firecracker snapshot create/load is unused
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:142-143).
    - >-
      Host-side data-image accounting distinguishes allocated file bytes,
      virtual capacity, and total VM state-directory bytes
      (internal/vmctl/data_image.go:10-18,27-57).
    - >-
      The current host networking setup enables global IP forwarding and adds
      unconditional FORWARD ACCEPT rules for every tap device
      (internal/vmmanager/manager.go:2686-2697,2707-2708,2726-2733).

finish:
  deliver: >-
    An owner can create resource-admitted ephemeral and persistent sibling
    computers that inherit only their authorized data classes, run parallel
    experiments, see their work in the parent Texture document, and bring a
    winning source change back through publication and adoption.
  artifact: >-
    A deployed staging fork-and-fleet product path with a two-path fork
    constructor, fresh child identity material, class-labelled lineage and
    fleet admission/quotas under the 32 GiB host budget, plus Texture
    transclusion of the fork sources.
  acceptance:
    - action: >-
        On staging, fork one long-running parent into three ephemeral children
        and one persistent child with recorded chosen data-class sets. The
        exercise includes the same-owner all-class reflinked-disk/cold-boot
        path and a selective-class or cross-owner semantic-snapshot allowlist
        reconstruction. Before any child runtime or network access, inspect
        that every child has a distinct ComputerID, signer keys, privacy key,
        and gateway token.
      proves: >-
        Both legally distinct construction paths make real sibling computers
        without inheriting parent identity material.
      evidence_class: deployed proof
    - action: >-
        Seed an excluded-class canary and a deleted-data remnant in the parent,
        create a selective-class child, then scan the child disk, Dolt history,
        logs, and artifact graph offline for both markers.
      proves: >-
        The semantic allowlist is evidence of exclusion; post-copy deletion is
        not being treated as a privacy boundary.
      evidence_class: deployed proof
    - action: >-
        Run a different experiment in each fork and inspect their sources from
        the parent's Texture document. Publish the winning experiment and have
        the parent adopt it through the S8 source-publication flow; reclaim all
        ephemeral forks afterward.
      proves: >-
        Fleet work remains separately owned and returns only as source through
        publication/adoption, while ephemeral capacity is recoverable.
      evidence_class: deployed proof
    - action: >-
        Request one further fork whose admission would exceed the 32 GiB host
        memory budget and observe refusal before child construction.
      proves: Fleet admission enforces the host resource envelope.
      evidence_class: deployed proof
  rollback: >-
    Revert and redeploy the station source change. Delete ephemeral children;
    stop a persistent child and treat it through the ordinary computer
    lifecycle. Revoke or refuse any incomplete child identity before it can
    route, and retain the parent unchanged; no direct state merge is a rollback
    path.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the difference between parallel experimentation and safe sibling
    ownership: each child carries exactly its authorized state, identity and
    resource share while a useful winning change can return as source.
  goodharting_would_be: >-
    Calling a copied VM a fork while it shares parent keys, scrubbing excluded
    rows after a disk copy, rendering a static fleet list instead of live
    Texture sources, or admitting children that only fit by ignoring memory.

homotopy:
  realism_axis: >-
    Inherited-parent-state coverage, continuously increased from a fresh
    zero-class sibling through class-labelled semantic exports to a same-owner
    all-class disk inheritance exercised across a four-child fleet.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission, 2026-10-01
    - docs/choir-doctrine.md
    - docs/computer-ontology.md
    - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md
  must_preserve:
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
    - Publication is source plus pinned inputs; no cross-tenant binary execution.
    - A fork never holds its parent's identity material.
  excluded:
    - S1-security-floor isolation and egress-policy implementation
    - S3-fast-resume machine-snapshot lifecycle and resume semantics
    - S8-source-publication implementation beyond the fork merge-back consumer
    - S10-org-templates export/import and template policy
    - Forking a computer mid-run
    - A bespoke fleet dashboard; the fleet view is Texture transclusion
  protected_surfaces:
    - identity material minting
    - VM networking / tap forwarding

now:
  status: checkpoint_incomplete
  slice: >-
    S9 fork construction and fleet admission are pending their security,
    fast-resume, and source-publication gates.
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
    id: class-bound-sibling-forks
    claim: >-
      Two construction paths classified before copying state, with child keys
      minted before execution and admission before construction, can provide
      useful parallel sibling computers without expanding parent authority or
      exceeding the single host's resource envelope.
    test: >-
      The deployed four-fork proof exercises both paths, scans a selective
      child offline for excluded and deleted parent data, verifies fresh child
      identity before runtime/network access, follows winner publication and
      parent adoption, reclaims ephemerals, and refuses an over-budget request.
    edge: missing_oracle
    delta_o: >-
      A deployed construction receipt ordered before child boot/network access,
      paired with offline disk/Dolt/log/artifact-graph scans and host admission
      accounting for the tested fleet.
    scope_if_supported: >-
      Single-host Choir Community Cloud staging on the 32 GiB Node B budget,
      for quiescent parent computers and the two owner/class construction
      categories stated here.
    status: proposed
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:588-601
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:270-285
  decision:
    what: >-
      Reflinked disk plus cold boot, host-side re-encryption, and new genesis
      is whole-disk same-owner only; selective-class and cross-owner children
      are fresh-disk semantic-snapshot reconstructions from an allowlisted
      export. Both paths mint child identity material before runtime or network
      access.
    kind: architecture
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:588-601,642-646
    owner_ratification_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:147-151,425-437
  belief:
    believed_state: >-
      Semantic recovery supplies a portable reconstruction substrate, while
      btrfs can supply same-owner whole-disk reflinks; neither is yet a deployed
      sibling-computer product path.
    main_uncertainty: >-
      Whether an admissible implementation can prove its per-class exclusion,
      pre-access re-keying, and fleet-resource accounting under the present
      host networking and memory constraints.
    next_observation: >-
      S1, S3, and S8 completion receipts followed by the first deployed
      two-path four-child proof and offline exclusion scan.
  blocker_or_risk: >-
    S1-security-floor, S3-fast-resume, and S8-source-publication are incomplete;
    current permissive tap forwarding makes any child networking change a
    protected-surface risk.
  next_action: >-
    Promotes to working when S1-security-floor, S3-fast-resume, and
    S8-source-publication complete; then reconcile their landed interfaces
    before designing the fork construction receipt.

receipts: []
---

# Fork construction

A fork is a sibling computer, never a parent candidate or a direct state-merge
partner (AGENTS.md:280-287). Its lineage names the parent checkpoint, the
construction path, carried classes, and the child identity; it does not make a
parent key or route transferable.

The same-owner all-class path starts with a quiescent parent. It reflinks the
whole `data.img`, cold-boots the child, re-encrypts host-side state, and records
a new genesis plus parent lineage. `data.img` is the per-VM disk image under its
state directory, and its allocated bytes differ from its virtual capacity
(internal/vmctl/data_image.go:10-18,27-57).

Selective-class and cross-owner requests never begin with that disk copy. They
export only named classes from a semantic snapshot into a fresh child disk;
the export receipt must make every carried class explicit. `PlanRecovery`
already distinguishes genesis, install, rebase, resume, and refusal, and caps
the replay tail at 10,000 events (internal/projectionbase/recovery_plan.go:7-30,69-77).

For both paths, allocation and registration are ordered: classify request;
admit capacity; create child identity material; bind child lineage and class
receipt; then permit cold boot, runtime, or networking. The current credential
envelope verifier binds both ComputerID and realization ID before exchange
(internal/platform/credential_envelope.go:72-90), so the new minting path must
retain that binding rather than cloning a parent bootstrap credential.

# Fleet and return path

Admission accounts for the requested fleet before a child exists and refuses a
request that would exceed the 32 GiB host memory budget. Ephemeral children are
reclaimed after their source is published or discarded; persistent children
remain independent computers, not retained copies available for parent merge.

Texture transcludes each fork source into the parent's work document. This is
the fleet view, not a new dashboard; rich transclusion remains an external
cutover dependency and must be recorded as a plain-ledger gap until available
(docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:357-362).
The only return path is S8 publication and parent adoption of source plus pinned
inputs, never a disk, closure, event-chain, or key transfer
(docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:584-601).

# Risk boundary

A scrub-after-copy claim is rejected: deleted rows can remain in Dolt history
or on disk, so only fresh-disk semantic reconstruction can prove selective
exclusion. The negative proof scans disk, history, logs, and artifact graph as
the metamission requires
(docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:270-285).

S9 consumes S1 rather than widening networking itself. Today's tap setup turns
on forwarding and accepts forwarding in both tap directions
(internal/vmmanager/manager.go:2707-2708,2726-2733); fork traffic cannot be
admitted across that surface until the S1 security floor has landed.
