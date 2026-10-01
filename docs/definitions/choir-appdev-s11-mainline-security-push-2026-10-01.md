---
definition_version: 4
definition_id: choir-appdev-s11-mainline-security-push-2026-10-01
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-supervised-app-development-metamission-2026-10-01

review: {reviewer: none, frozen_ref: none, verdict: none, evidence_ref: none}

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
      M9a platform-to-computer signed push exists and was proven on staging
      with a 1-file payload; nothing in CI calls it. No computer-to-computer
      publication surface exists (M9b unbuilt).
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:120-123)
    - >-
      The updater is a root-owned guest boundary: it stages a private release,
      swaps current, restarts, health-probes, and restores the prior release
      on failure. (internal/updater/updater.go:198-285)
    - >-
      Existing platform updates are tracking-only, platform-control-signed
      offers whose signature, structure, expiry, lineage policy, and canonical
      head binding all gate mutation. (internal/agentcore/platform_update.go:25-34,
      59-90, 118-148; internal/selfdevprotocol/platform_update.go:18-21, 42-46)
    - >-
      A successful tracking update publishes a checkpoint and then promotes the
      route under a distinct platform-follow evidence class.
      (internal/agentcore/platform_update.go:303-309, 341-369, 404-429)

finish:
  deliver: >-
    A host can select a published source change into main and safely push a
    platform security fix across tracking and divergent computers without a
    silent skip, including computers that are asleep at the deadline.
  artifact: >-
    A deployed host mainline-selection and security-offer path with a signed
    offer record (severity, deadline, exploit-test witness, and per-computer
    disposition), ephemeral-fork compatibility evaluation, host-side
    fail-closed enforcement, and independently verified capability removal.
  acceptance:
    - action: >-
        On staging, select a published source change into host main, issue a
        signed security offer, and exercise it against four computers: tracking,
        clean divergent-compatible, divergent with structural component absence,
        and divergent-conflicting.
      proves: >-
        The host records and executes respectively auto-apply, rebase-and-apply,
        exempt, and proposal dispositions without treating divergence as a
        silent skip.
      evidence_class: deployed proof
    - action: >-
        Run the offer's exploit regression witness against the pre-fix and
        post-fix builds on each required ephemeral-fork evaluation; submit a
        passing-test-only exemption request.
      proves: >-
        The witness fails before the fix and passes after it before an offer is
        valid, and a passing witness alone cannot establish exemption.
      evidence_class: deployed proof
    - action: >-
        Leave a vulnerable divergent-conflicting computer asleep through the
        offer deadline, inspect the host's recorded disposition, then attempt
        the vulnerable capability and independently verify its removal on the
        actual computer.
      proves: >-
        Deadline enforcement is host-side, fail-closed does not depend on a
        sleeping computer rebasing, and capability removal is real rather than
        an unverified host record.
      evidence_class: deployed proof
  rollback: >-
    Git-revert and redeploy the host selection/offer implementation; retract an
    unexecuted offer; for an executed misclassification restore the affected
    computer through its pinned-head path before reissuing a corrected offer.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between a known platform vulnerability and an accounted,
    safe per-computer outcome while preserving source authority, divergent work,
    checkpoint/route integrity, and host control at the deadline.
  goodharting_would_be: >-
    Reporting fleet success after skipping divergent or sleeping computers, or
    calling a passing exploit test an exemption without proving that the
    vulnerable component is absent.

homotopy:
  realism_axis: >-
    The same signed-offer and disposition protocol from one tracking computer,
    through a clean divergent ephemeral-fork rebase, to a four-computer fleet
    including structural absence, conflict, and a sleeping deadline case.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission spine (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:373-379, 605-617)
    - docs/choir-doctrine.md
    - docs/computer-ontology.md
    - AGENTS.md:119-131
  must_preserve:
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - Publication is source plus pinned inputs; no cross-tenant binary execution.
    - A fork never holds its parent's identity material.
  excluded:
    - S8-source-publication source publication, review, adoption, and cross-computer source transfer
    - S9-forks-and-fleets fork construction, re-keying, data-class selection, and fleet admission
    - S10-org-templates export/import of scrubbed organization templates
    - bespoke fleet dashboard; fleet view remains Texture transclusion
  protected_surfaces:
    - platform-control signing domain
    - checkpoint / route projection

now:
  status: checkpoint_incomplete
  slice: >-
    Pending S8 source-publication and S9 forks-and-fleets: define the host
    mainline-selection and security-offer implementation from their published
    source record and ephemeral-fork contract.
  source_ref: main@8aa1dce9
  deploy_identity: 'staging https://choir.news deployed_commit=a3cfaa00'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: host-security-disposition
    claim: >-
      A host-owned signed offer plus an ephemeral-fork compatibility witness can
      preserve divergent work while still making every affected computer reach
      an auditable, safe disposition by deadline; structural absence, not an
      exploit-test pass, can justify exemption.
    test: >-
      Deployed four-computer proof exercises tracking auto-apply, clean
      divergent rebase-and-apply, structurally absent exempt, and conflicting
      proposal then sleeping-computer fail-closed; each valid offer's witness
      fails pre-fix and passes post-fix.
    edge: missing_oracle
    delta_o: >-
      Host disposition receipts joined to ephemeral-fork evidence and an
      independent capability-denial observation on the real affected computer.
    scope_if_supported: >-
      Staging's single-host fleet for platform app-layer security offers after
      S8 publication and S9 ephemeral-fork facilities have landed.
    status: proposed
    evidence_refs:
      - docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:605-617
      - internal/agentcore/platform_update.go:59-90
  decision:
    what: >-
      Use host selection into main and signed security offers with the four
      defined classifications; fail close host-side at deadline, and allow
      exemption only on structural component-absence proof.
    kind: architecture
    status: settled
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:605-617
    owner_ratification_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:373-379
  belief:
    believed_state: >-
      The existing tracking-only signed update path preserves its own
      checkpoint/route evidence, but has no stated divergent-fleet disposition
      or host-side deadline enforcement.
    main_uncertainty: >-
      Whether S8's source record and S9's ephemeral forks expose sufficient
      evidence to prove structural absence and independently verify removal on
      a real computer without widening signing authority.
    next_observation: >-
      The landed S8 publication contract and S9 fork lifecycle, followed by a
      staging offer rehearsal across the four required divergence states.
  blocker_or_risk: >-
    Implementing before S8 and S9 would invent the publication or ephemeral-fork
    contracts and could bypass platform-control signing or checkpoint/route
    projection safeguards.
  next_action: >-
    Promotes to working when S8-source-publication and S9-forks-and-fleets
    complete; then reconcile their landed contracts with the current signed
    platform-update path and document any newly observed security problem before
    fixing it.

receipts: []
---

# Mainline and Security Push

## Mechanism sketch

- Host selection consumes S8's source record; it does not import a publisher's binary.
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:584-587)
- The selected record enters the platform base only through the host's normal mainline
  authority and the platform-control signing domain.
- A security offer binds severity, deadline, target component, signed content, and its
  regression witness to one canonical interpretation of the fix.
- The witness is valid only when it fails on the pre-fix build and passes on the post-fix
  build. It demonstrates regression coverage; it never proves exemption.
- For every divergent computer, the host evaluates compatibility on an S9 ephemeral fork.
  A clean result can rebase and apply; a conflict or inconclusive result remains a proposal.
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:605-617)
- Tracking computers auto-apply through the signed update path. A divergent computer may
  be exempt only with structural proof that the affected component is absent.
- At deadline the host records one disposition for every computer. If the computer cannot
  rebase or is asleep, the host denies the vulnerable capability, notifies the owner, and
  later independently verifies removal on the real computer.
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:605-617)

## Safety case

Existing tracking update processing verifies the offer before mutation and binds acceptance
onto the event chain (internal/agentcore/platform_update.go:59-63, 75-90, 163-193). Its
materialization receipt and health receipt are verified before the applied event is recorded
(internal/agentcore/platform_update.go:264-291). S11 extends neither condition by treating
an exploit test as an authority source: platform-control signature, source provenance,
canonical-head constraints, and checkpoint/route joins remain separate protections.

The critical risk is declaring a vulnerable divergent computer exempt or successful while it
remains exposed. The deployed sleeping-computer proof is therefore a terminal safety proof,
not an operations simulation: the host record, denied capability, and real-computer
verification must agree.

## Changes of hand

S8 supplies the reviewable source record and S9 supplies isolated, re-keyed ephemeral forks
(docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:584-600).
S11 returns host-mainline provenance, the signed offer, per-computer dispositions, and
fail-closed evidence. S10 template work and any fleet presentation remain outside this station.
