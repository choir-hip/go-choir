---
definition_version: 4
definition_id: choir-appdev-s6-commit-gate-full-release-2026-10-01
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
      Self-dev control plane is live: M11 ran a desk-authored operation through
      frozen, awaiting approval and applied under qualified consensus, rendered
      evidence in Texture, and restored a pinned head; its probe did not assert
      a runtime code effect
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:94-100).
    - >-
      Self-dev effect reaches only the served release: StageGrantedRelease writes
      under var/lib/artifact/release, the updater swaps current, and the computer
      surface is the current/frontend reader; the runtime itself is executed from
      the image Nix store
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:102-109).
    - >-
      Semantic snapshots already provide ProjectionBase blobs plus replay
      watermark W and PlanRecovery, while the current owner blob is about 16 GB
      (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:136-140).
    - >-
      Existing release staging requires a frozen engineering capsule and emits
      its staged release from its merged capsule tree
      (internal/capsule/executor.go:1344-1376).
    - >-
      M7 already rejects a decision when its expected desired/effective heads,
      pending transition, or state commitments no longer match
      (internal/agentcore/api_self_development.go:906-918).

finish:
  deliver: >-
    The owner's approved, tested capsule change becomes one complete app-layer
    release whose guest backend behavior is observable after reboot and the
    existing cold-resume path, and whose pinned restore removes that behavior
    without exposing a mixed release.
  artifact: >-
    A deployed per-computer app-layer release and M7 decision receipt binding
    immutable source, pinned inputs, test evidence, closure, release manifest,
    S5 rendered-content digest attested by the trusted serving path, executable/
    frontend/state-compatibility record, release digest, and expected computer
    head; activation/recovery records on the canonical event chain, checkpoint,
    and route projection.
  acceptance:
    - action: >-
        On staging, build a capsule change that alters a guest backend endpoint,
        freeze source, inputs, tests, closure, S5 serving-path-attested rendered
        content digest and state-compatibility record, pass its tests, obtain the
        owner decision in the desktop, then fetch the endpoint after activation.
      proves: >-
        The approved release changes actual guest backend behavior rather than
        only served frontend bytes or a capsule preview.
      evidence_class: deployed proof
    - action: >-
        Reboot the guest, exercise its existing cold-resume product lifecycle
        path, and fetch the changed backend endpoint after each transition; then
        restore the pinned pre-release head and fetch the endpoint again.
      proves: >-
        The full release survives reboot and existing cold resume, while pinned
        restore removes the release's backend effect.
      evidence_class: deployed proof
    - action: >-
        Run an intentionally failing declared test against the frozen candidate
        and attempt activation; verify no release activation or backend change
        occurs.
      proves: A failing test blocks activation before owner approval can promote bytes.
      evidence_class: deployed proof
    - action: >-
        With passing tests, exercise activation with no owner decision and with
        an explicit negative owner decision; verify both leave the release
        inactive and the backend unchanged.
      proves: Absent and negative owner decisions block activation.
      evidence_class: deployed proof
    - action: >-
        Attempt owner approval after changing frozen source, pinned input, test
        evidence, closure, preview digest, or state-compatibility record, and
        after advancing the expected event head; at every activation boundary
        kill the updater and reconcile from the canonical event head before
        comparing runtime/frontend/backend versions.
      proves: >-
        Post-test mutation and stale-head promotion refuse, and every interrupted
        activation recovers derivably with no mixed-version observation.
      evidence_class: deployed proof
  rollback: >-
    Refuse an unbound or stale decision before mutation; for an applied release,
    restore the pinned pre-release head through the checkpoint/tape path and
    reconcile the updater, checkpoint and route projection from canonical events.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between an owner-approved tested capsule change and one
    durable, recoverable guest app-layer effect, while preserving a single
    canonical event authority, immutable approval bindings, and pinned restore.
  goodharting_would_be: >-
    Approving a digest while later bytes are activated, proving only frontend
    current/ swapping, or recovering a killed updater by silently serving a
    mixture of old runtime, backend, frontend, and route state.

homotopy:
  realism_axis: >-
    Release-transition completeness, from one frozen backend change observed
    after a clean activation to the same change across every updater crash
    boundary, reboot, resume, and pinned-head restore.

boundaries:
  mutation_class: red
  authority_sources:
    - owner-ratified metamission 2026-10-01 (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:153-173, 373-379)
    - docs/choir-doctrine.md
    - docs/computer-ontology.md
    - existing M7 decision and materialization machinery (internal/agentcore/api_self_development.go:902-1032; internal/agentcore/self_development_materializer.go:39-158)
  must_preserve:
    - Capsules own no semantic state; only an accepted event changes desired code.
    - Owner approval (or a stronger declared consensus policy) gates every commit.
    - Every committed change is restorable through the pinned-head path.
    - No guest-side writable Nix store or daemon outside a capsule boundary.
    - The non-forkable base (kernel, capsule broker, updater, signers, network policy) stays platform-owned and identical across computers on the same image.
    - Provider credentials never enter a capsule.
  excluded:
    - S2-layering-runtime-from-release's base/app-layer closure substrate and builder choice.
    - S3-fast-resume's machine-snapshot implementation, snapshot-resume proof, and latency target.
    - S5-live-preview-supervision's preview bridge and Texture work view.
    - S7-app-packages's dynamic package manifest and package boundaries.
    - S8-source-publication's cross-computer publication and adoption flow.
  protected_surfaces:
    - updater trust boundary and release manifest
    - canonical event commit path
    - checkpoint / route projection

heresy_delta:
  discovered:
    - >-
      M11 proved a control-plane decision and pinned restore without asserting a
      runtime code effect, so an approval receipt alone is not evidence of a
      complete guest release (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:94-109).
  introduced: []
  repaired:
    - none; this station remains checkpoint_incomplete pending S2 and S5.

now:
  status: checkpoint_incomplete
  slice: >-
    Freeze-to-approved-full-release binding and canonical-event recovery,
    awaiting its layering and live-preview prerequisites.
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
    id: immutable-approval-yields-one-recoverable-release
    claim: >-
      Extending M7's accepted-decision materializer to bind frozen source,
      inputs, tests, closure, S5 serving-path-attested preview digest and
      executable/frontend/state-compatibility record to the decision and expected
      head will make one full app-layer release recoverable from the canonical
      event chain without permitting stale or post-test bytes.
    test: >-
      Deployed backend-change proof across passing and intentionally failing
      tests, absent and negative owner decisions, every updater kill boundary,
      reboot, existing cold resume and pinned restore, plus negative
      post-test-mutation and stale-head attempts.
    edge: missing_oracle
    delta_o: >-
      A release identity observer that joins frozen-input/test/closure, S5
      serving-path-attested preview, and state-compatibility digests with the
      decision receipt, canonical head, updater journal, checkpoint, route and
      live runtime/frontend/backend identities at each fault boundary.
    scope_if_supported: >-
      One staging computer's M7-governed app-layer release transition; not
      publication, fleet propagation, base-image rollout, or machine-snapshot
      resume.
    status: proposed
    evidence_refs:
      - internal/agentcore/selfdev_derivable_materialize_test.go:999-1096
      - internal/agentcore/self_development_materializer.go:405-655
  decision:
    what: >-
      Reuse M7's owner-decision boundary and derivable reconciler; propose that
      its approval bind immutable tested release inputs, S5 serving-path-attested
      preview bytes and state compatibility, then promote only through the
      canonical event, checkpoint and route-projection chain.
    kind: architecture
    status: proposal
    evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:578-580
    owner_ratification_ref: none — the specific M7 full-release extension is not separately owner-ratified.
  belief:
    believed_state: >-
      M7 already has a canonical, idempotent materialization chain through
      applied event, checkpoint and route projection, but its current release is
      insufficiently bound to the complete app-layer inputs, preview content and
      state compatibility this station needs.
    main_uncertainty: >-
      Whether the complete release identity can remain stable across all updater
      interruption boundaries and lifecycle transitions without a mixed runtime,
      frontend, backend or route observation.
    next_observation: >-
      S2 and S5 completion evidence followed by a fault-injected staging release
      transition whose identity observer spans canonical head through live guest
      backend effect; machine-snapshot resume remains S3's proof.
  blocker_or_risk: >-
    A release identity that excludes tests, closure, S5 rendered bytes, state
    compatibility, inputs, or the expected head could let owner approval
    authorize bytes different from those activated.
  next_action: >-
    Promotes to working when S2-layering-runtime-from-release and
    S5-live-preview-supervision complete; then document the release-identity
    contract and fault boundaries before mutating protected surfaces.

receipts: []
---

# S6 — Commit Gate, Full Release

## Mechanism sketch

- The approval boundary receives only a frozen release witness: source patch,
  pinned inputs, test command/results, closure identity, release manifest, S5
  rendered-content digest attested by the trusted serving path,
  executable/frontend/state-compatibility record, and expected
  desired/effective/canonical head.
- The M7 decision event must join that witness before it can enter Accepted;
  a failing test, absent or negative owner decision, digest mismatch, post-test
  mutation, or changed expected head is a refusal, not a re-test or a
  best-effort promotion.
- Current M7 decision creation already CAS-checks desired/effective heads,
  pending transition and state commitments before appending its event
  (`internal/agentcore/api_self_development.go:906-981`).
- Its verifier already ties the decision to the operation, capsule, bundle,
  verifier reference, owner or qualified-consensus authority, and event-head
  receipt (`internal/agentcore/self_development_decision_binding.go:27-125`).
- The full-release extension must add the missing immutable inputs to that same
  witness rather than create a second approval authority.

## Transaction and recovery

- Activation is one recoverable canonical-event transaction: materialization
  started, updater apply, materialization applied, checkpoint published, then
  route projection updated.
- Existing M7 proof expects exactly one of each of those event kinds and one
  updater restart on a normal derivable materialization
  (`internal/agentcore/selfdev_derivable_materialize_test.go:999-1055`).
- The reconciler is driven by committed events and returns from durable operation
  state; failures retain a retry wake rather than requiring an HTTP caller
  (`internal/agentcore/self_development_materializer.go:39-127`).
- Checkpoint publication binds the post-append canonical head and retries only
  on head CAS conflict; route projection re-reads the live canonical head before
  its authority request (`internal/agentcore/self_development_materializer.go:413-515`,
  `internal/agentcore/self_development_materializer.go:546-655`).
- The updater-kill sweep deliberately strengthens the spine's S6 bullet with
  fault-boundary proof of its one-transaction invariant
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:578-580).
- At every transaction boundary, kill the updater, restart/reconcile normally,
  and observe one joined identity: release digest, canonical head, checkpoint,
  route, runtime, frontend and changed backend response.
- A mixed-version observation is a failure even if the final retry appears
  healthy; preserve the pre-fault identity and fault boundary as evidence.

## Key risk and handoff

- The release manifest is the updater trust boundary: the existing updater reads
  a pinned manifest only when its content digest matches, and swaps `current`
  only to that pinned release (`internal/updater/updater.go:512-537`).
- Pinned restore must restage the prior SPA release only after the tape witness
  matches; the existing helper deliberately avoids Apply and service restart
  (`internal/updater/updater.go:524-537`).
- In this station, “resume” means the existing cold-resume lifecycle path;
  machine-snapshot resume and its proof remain exclusively S3 work
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:130-134,
  544-559).
- S2 hands over a realizable, base-compatible app-layer closure. S5 hands over
  the owner-visible tested preview, supervision evidence, and serving-path-
  attested rendered-content digest. S6 hands S7/S8 a single approved, complete
  release record—not a capsule directory or binary
  (docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md:523-543,
  575-587).
- The existing crash proof recovers a parked Materializing operation through the
  same reconciler with exactly one event chain; S6 expands that proof to every
  updater boundary and the deployed lifecycle observations
  (`internal/agentcore/selfdev_derivable_materialize_test.go:1058-1096`).
