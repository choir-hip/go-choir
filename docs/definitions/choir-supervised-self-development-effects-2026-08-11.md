---
definition_version: 2
definition_id: choir-supervised-self-development-effects-2026-08-11
execution_mode: mission_orchestrator

start:
  captured_at: 2026-08-11T18:40:00Z
  source:
    canonical_ref: main@6ff6b7d0
    deploy_identity: "staging https://choir.news deployed_commit 26c53692494aed1a2ea337550990d70c7cd16735 via CI 31445846546 (2026-08-11T00:54:41Z); /health ok; deploy identity predates 367265f8 and 6ff6b7d0 and must be re-reconciled before the rehearsal slice"
  worktree_inventory:
    status: reconciled
    evidence_ref: 2026-08-11 read-only git status; clean single worktree /Users/wiz/go-choir
    preservation_rule: Preserve every non-primary worktree and all unrelated WIP; this Definition owns only itself, its review evidence, and the three registry entries at supersession.
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: revert the docs-only commit
  candidates:
    - id: none
  observed_artifact:
    - claim: Self-development mode CAS exists, missing row resolves to OFF, accept_once requires exact operation/bundle/heads/pending transition/commitments plus a future canonical UTC expiry.
      evidence_ref: internal/platform/self_development_modes.go NewSelfDevelopmentModeCAS/Get/Set/validateSelfDevelopmentModeTransition
    - claim: Propose/decision/genesis/rollback API routes exist with verifyStartModeReceipt and accept_once expiry reconciliation conflict fail-close.
      evidence_ref: internal/agentcore/api_self_development.go
    - claim: Materializer reconciles, recovers decisions, and materializes/rolls back with signed receipts, checkpoint/route projection, TransitionPromote/TransitionRollback, and EventMaterializationApplied/EventRollbackApplied joins.
      evidence_ref: internal/agentcore/self_development_materializer.go reconcileSelfDevelopmentMaterialization/recoverSelfDevelopmentDecision/materializeSelfDevelopmentOperation/rollbackSelfDevelopmentOperation
    - claim: Accepted/rejected decision binding joins operation identity, verifier refs, mode receipt, and both heads; it admits exactly one InputArtifactRef (the mode receipt) and exactly one VerifierRef.
      evidence_ref: internal/agentcore/self_development_decision_binding.go:45,53
    - claim: The frozen CapsuleEffectBundle envelope is source-code shaped by construction — Validate refuses any bundle lacking SourceTreeRef, a capsule-exec BuildRecipeRef, RuntimeArtifactRef, TestReceipts, and DependencyToolchainRefs.
      evidence_ref: internal/capsule/transaction/builder.go:95-111
    - claim: The complete CoSuper capsule authoring surface exists — spawn_capsule, capsule_write_file, capsule_exec (signed execution receipts), commit_transaction (freezes the classified diff into updaterRoot/incoming), inspect_self_development_bundle, record_self_development_verification.
      evidence_ref: internal/agentcore/tools_capsule.go:153,251,319,398,477,713,765; internal/capsule/executor.go Exec/PersistGrantedFreezeReceipt
    - claim: Updater applies a release by staging manifest files, swapping the current pointer, restarting the service, health-probing, and restoring the prior release on probe failure.
      evidence_ref: internal/updater/updater.go:145-238
    - claim: The frontend is embedded into the Go binary via go:embed, so a UI change and an API change ship as one built runtime artifact.
      evidence_ref: cmd/desktop/main.go:24
    - claim: There is no schema migration framework; stores create tables with CREATE TABLE IF NOT EXISTS at startup, so an additive table is created by a new binary and ignored by the prior binary after rollback.
      evidence_ref: internal/platform/computer_events.go:18,39,63; no migration runner in internal/platform
    - claim: No usable owner bearer was available at the prior capture; API key issuance has since been repaired and the one-click bootstrap mints an owner-wide admin key (agentic consensus Family 1: one key controls every owned interactive computer via the use-time vmctl ownership join; bound keys remain as attenuation), so Mission 0's residual is disposition, not absence of a path.
      evidence_ref: 367265f8; 6ff6b7d0; .agentic-consensus/key-model-2026-08-11-convergent/; docs/choir-crashed-prime-session-review-2026-08-09.md
  problems_documented:
    - id: model-policy-effect-target-mismatch-2026-08-11
      problem: "The prior content axis (D1, computer-scoped model policy) has no path to its target. The materializer's only apply surface stages bundle RuntimeFiles into the updater release tree and swaps the current pointer, while model policy is read from a separate files root on every activation. A model-policy bundle would stage, promote, restart, health-probe, and sign an applied receipt while changing nothing about resolution — a fully green proof of a change that did not happen."
      evidence_ref: internal/agentcore/self_development_materializer.go:199; internal/updater/updater.go:145-188; internal/modelpolicy/model_policy.go:89-118; internal/provideriface/config.go:95-101
      consequence: "Content axis changed from D1 (model policy) to D3 (source code). The mismatch is not repaired by repointing the policy path; it is dissolved by choosing content the substrate was built to carry."
    - id: model-policy-is-system-owned-2026-08-11
      problem: "Making model policy the first thing the computer edits about itself contradicts promoted architecture direction: model selection, fallback, and cycling are system-owned processes rather than agent-edited configuration, and internal/modelpolicy is slated for generalization into broker-mediated multi-call selection, with live overlays to be retired once the eval substrate lands."
      evidence_ref: docs/memo-persistent-rlm-actors-2026-08-09.md (integration table, internal/modelpolicy row; Multi-Model Execution); docs/memo-live-retrospective-evals-2026-08-09.md (overlays are not counterfactual isolation)
      consequence: "The first supervised self-development effect must not teach the computer to own a surface doctrine assigns to the system and has scheduled for replacement."
  unknowns:
    - Whether the CoSuper capsule image carries a Go toolchain and a provisioned source tree sufficient to build and test the runtime artifact. capsule.Executor.PreflightSourceSnapshot suggests source provisioning exists; the toolchain must be confirmed at the first slice, problem-first if absent.
    - How staging serves the web frontend. If it is served outside the updater-controlled release, the UI half of the candidate lands outside the effect boundary and the candidate must be API-only until that is resolved.
    - Whether rival-proposal/supersession semantics for E2 are expressible on the existing event/decision/selfdev operation graph without a new settlement subsystem. Pre-flight must document this before coding; do not silently downgrade to E1.
    - Retained computer epoch 8253 disposition; ak_45ce1796 row and root-only auth rollback cleanup.

finish:
  deliver: "One supervised self-development candidate accepted on staging: a CoSuper capsule authors, builds, and tests a new solitaire capability for the computer — headless play API, durable game persistence, play history, and embedded UI — and the computer adds that capability to its own running program under granted rules, through the correction spine. Defective version A is accepted and applied; admissible headless-play evidence falsifies it; corrected version B supersedes A; restart proves B effective. A final owner-directed rollback then revokes the capability."
  artifact: "An authenticated staging computer trajectory carrying: mode receipt; frozen bundle A with real SourceTreeRef, capsule-exec BuildRecipeRef, DependencyToolchainRefs, TestReceipts, RuntimeArtifactRef, and an independent verifier receipt; accepted decision A binding operation identity + verifier ref + mode receipt + both heads; materialization receipt with checkpoint and route TransitionPromote; a headless API play transcript driving the engine into the illegal state A accepted; superseding decision B carrying its own frozen bundle and forward transition; a post-restart replay of the same sequence showing B refuses the move; and a final rollback receipt showing the capability absent with game rows retained by design — all joined to the canonical computer event chain."
  acceptance:
    - action: "Product-path rehearsal on staging with a trivial no-op source change: propose -> accept_once -> materialize -> rollback, with a restart-durable read of the applied artifact. Live flip gated on rehearsal PASS."
      proves: "Landing machinery (capsule build, frozen bundle, mode CAS, materializer, checkpoint/route, forward rollback) works end to end before any live accept_once on a real capability."
      evidence_class: deployed proof
    - action: "Capsule authorship proof: a CoSuper capsule spawns, writes the solitaire source, runs build and test via capsule_exec, and freezes the bundle via commit_transaction, with every required bundle ref bound to a real execution receipt."
      proves: "The candidate is authored by the computer under supervision, not hand-authored and pushed through the pipeline."
      evidence_class: deployed proof
    - action: "Live E2 proof: accept A (defective solitaire) -> headless play evidence falsifies A -> correction B supersedes A -> restart -> B effective."
      proves: "The vision correction spine lands on the canonical tape and survives restart."
      evidence_class: deployed proof
    - action: "Owner-directed revocation: rollback removes the solitaire capability; the deployed API returns absent and the retained game rows are shown to persist."
      proves: "Granted capability is revocable through the product path, with rollback asymmetry stated rather than concealed."
      evidence_class: deployed proof
    - action: "Replay the accepted trajectory from canonical events."
      proves: "Single semantic authority reconstruction; no second state authority was introduced."
      evidence_class: deployed proof
  rollback: "Mode CAS -> off; forward rollback transaction restores the prior ComputerVersion checkpoint and the prior release pointer; revert the behavior commits through origin/main and CI to the last accepted runtime. Direct file edits on the deployed node are not the proof and are not a rollback path. Rollback restores the code head only: additive solitaire tables and their rows persist and must be reported as retained, not as removed."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - Effects remain OFF (no accepted + materialized + restart-proven source change on staging).
    - Only E1 evidence exists (single apply without correction supersession) at registry close.
    - The candidate diff was hand-authored while the report implies capsule authorship.
    - A's defect was caught by A's own verification (that proves the gate, not the correction spine).
    - The E2 rival/supersession gap was downgraded instead of documented and fixed.
    - Rollback is claimed as total while solitaire rows persisted.
    - CTS remains active in all three registries without a successor pointer.

boundaries:
  mutation_class: red
  authority_sources: [owner-ratified decisions, docs/choir-vision.md, docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/standing-questions.md, docs/choir-self-development-roadmap-2026-08-11.md, AGENTS.md]
  must_preserve:
    - Canonical computer event chain is the single semantic state authority.
    - Effects stay OFF until the staging rehearsal gate passes.
    - The candidate is authored inside a CoSuper capsule; every required bundle ref binds a real execution receipt from that capsule.
    - A's defect is pre-declared in a Define receipt before A is proposed, so the falsification cannot later be reported as discovered.
    - Self-development schema changes are additive only — new tables via CREATE TABLE IF NOT EXISTS, never ALTER or DROP of existing tables — because there is no migration framework and rollback does not reverse schema.
    - Solitaire touches no protected surface: no Texture writes, no auth/session, no event/decision path, no provider routing.
    - No headless/CDP/virtual-authenticator substitute for exact owner presence; no SSH-shaped operations; product path only.
    - Problem-documentation-first: a discovered platform problem is documented in a code-free Define receipt before any repair-code commit.
  excluded:
    - RLM/Yaegi actor authoring (successor capability mission; Phase 1 of the RLM memo preserves effects-OFF).
    - Model-policy content as a self-development effect (see problems_documented; the surface is system-owned and slated for replacement).
    - Production environment.
    - CTS full supervision choreography (finishing CTS cannot deliver the vision proof; its evidence folds into this Definition).
    - In-choir computer-control draft (separate successor authority).
  protected_surfaces: [self-development mode CAS, canonical computer event chain, materializer + decision binding, checkpoint/route projection, updater root, vmctl lifecycle, auth/session renewal, gateway/provider calls, deployment routing]
  completion_evidence_floor: [deployed proof, independent review of frozen bundle + decision binding]

measures:
  - name: rehearsal pass
    kind: gate
    baseline: 0
    desired: accept->materialize->rollback rehearsal PASS on staging before live accept_once
    decision_use: unlocks the live flip
    cannot_prove: the vision correction spine
  - name: capsule-bound bundle refs
    kind: gate
    baseline: 0
    desired: all five required bundle refs (source tree, build recipe, toolchain, tests, runtime artifact) bound to receipts from the authoring capsule
    decision_use: distinguishes capsule authorship from hand authorship
    cannot_prove: that the authored capability is correct
  - name: tape segment count
    kind: weak_signal
    baseline: 0
    desired: ">= 7 (mode receipt, accept A, materialize A, falsify, supersede B, restart proof, revoke)"
    decision_use: inspect the next transition; never advances complete alone
    cannot_prove: acceptance
  - name: consensus agreement
    kind: weak_signal
    baseline: roadmap convergent panel unanimous on sequence shape (3/3 verdicts)
    desired: independent review acceptance of frozen E2 candidate
    decision_use: schedule review; never settles authority
    cannot_prove: the product works

now:
  status: active
  slice: "Content axis pivoted from model policy to source code; the solitaire candidate and capsule authorship are settled. Next is the toolchain/frontend feasibility check and the E2-expressibility pre-flight."
  question: "Does the CoSuper capsule carry a Go toolchain and source tree? Does staging serve the frontend from the updater-controlled release? Is 'B supersedes A' writable on the existing decision graph given the one-input-artifact constraint?"
  reconciliation:
    observed_at: 2026-08-11T18:40:00Z
    source_ref: main@6ff6b7d0
    deploy_identity: "staging deployed_commit 26c53692494aed1a2ea337550990d70c7cd16735 (stale relative to main; re-reconcile before the rehearsal slice)"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/choir-self-development-roadmap-2026-08-11.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-11 read-only git status (clean)
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Effects-first on existing substrate; D3 source-code content (capsule-authored solitaire: headless play API, persistence, history, embedded UI) x D2 frozen-bundle envelope; E2 correction-spine acceptance followed by owner-directed revocation; RLM strictly after; CTS superseded as superseded_incomplete."
    kind: architecture
    status: ratified
    source: orchestrator
    evidence_ref: docs/choir-self-development-roadmap-2026-08-11.md; .agentic-consensus/self-dev-roadmap/{divergent,lateral,convergent}/; .agentic-consensus/readiness-key-2026-08-11/
    owner_ratification_ref: "owner direction 2026-08-11: model policy rejected as the content axis ('changing model policy doesnt actually prove self-development works ... we should be changing source code'); solitaire candidate proposed by owner; capsule authorship directed by owner ('definitely written by a cosuper in a capsule')"
    recorded_at: 2026-08-11T18:40:00Z
    consequence: "D1 is removed from the content axis and D3 is no longer excluded. The rehearsal slice does not start until the toolchain/frontend feasibility check and the E2-expressibility pre-flight are documented."
  evidence_refs: [docs/choir-self-development-roadmap-2026-08-11.md, docs/choir-crashed-prime-session-review-2026-08-09.md, docs/memo-persistent-rlm-actors-2026-08-09.md, docs/memo-live-retrospective-evals-2026-08-09.md]
  blocker_or_risk: "Capsule Go toolchain and source-tree availability; staging frontend serving location; E2 expressibility pre-flight; Mission 0 residual (epoch 8253 disposition, ak_45ce1796 row, root-only auth rollback cleanup); staging deploy identity stale relative to main."
  next_action: "Run the feasibility check (capsule toolchain + source tree; staging frontend serving), then the E2-expressibility pre-flight, then re-reconcile staging deploy identity before the rehearsal slice."

receipts:
  - id: roadmap-consensus-2026-08-11
    boundary: define
    commit_or_artifact: 2379616d
    proof_refs: [.agentic-consensus/self-dev-roadmap/divergent/manifest.tsv, .agentic-consensus/self-dev-roadmap/lateral/manifest.tsv, .agentic-consensus/self-dev-roadmap/convergent/manifest.tsv]
    rollback_ref: revert 2379616d
    disposition: accepted (unanimous shape; E1/E2 resolved E2-for-vision, E1-as-rehearsal-gate)
    problem_ref: not_applicable
    authorization_ref: not_applicable
    candidate_or_evidence_refs: [docs/choir-self-development-roadmap-2026-08-11.md]
    landing:
      source_commit: 2379616d
      ci_ref: 31450288548 (Docs Truth Check success)
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "registered 2026-08-11 across docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml (entrypoint true; supersedes choir-continuous-texture-supervision-2026-08-07)"
  - id: content-axis-pivot-2026-08-11
    boundary: define
    commit_or_artifact: pending
    proof_refs: [internal/capsule/transaction/builder.go:95-111, internal/updater/updater.go:145-238, internal/modelpolicy/model_policy.go:89-118, internal/agentcore/tools_capsule.go:251,319,713]
    rollback_ref: revert the docs-only pivot commit
    disposition: accepted (owner-directed; D1 rejected, D3 adopted, capsule authorship required)
    problem_ref: [model-policy-effect-target-mismatch-2026-08-11, model-policy-is-system-owned-2026-08-11]
    authorization_ref: owner direction 2026-08-11
    candidate_or_evidence_refs: [docs/definitions/choir-supervised-self-development-effects-2026-08-11.md]
    landing:
      source_commit: pending
      ci_ref: pending (Docs Truth Check)
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "definition_id unchanged; no registry flip required — this is a content-axis revision to an already-registered active Definition"

view:
  path: none
  generator: none
---

# Supervised Self-Development with Effects — Successor Definition

This Definition supersedes `choir-continuous-texture-supervision-2026-08-07`
as the executable path to the vision proof target (see
[docs/choir-vision.md](../choir-vision.md)): a self-development candidate
accepted on staging — one real change to the computer's own working state under
granted rules, legible on the tape, durable across a restart.

CTS's own `not_done_when` forbids effects while OFF, so finishing CTS cannot
deliver the vision proof; that is the supersession justification. CTS's
evidence — mailbox gaps, policy apply/rollback helpers, requirement audit,
implementation inventory — folds into this Definition's evidence refs.

## Why the content axis is source code

The first version of this Definition proposed computer-scoped model policy as
the content of the first effect, on the theory that it was the smallest real
change to the computer's working state. Two findings retired that choice; both
are recorded in `start.problems_documented` and neither is repaired by
adjusting the model-policy plumbing.

The first is mechanical. The materializer's only apply surface stages the
bundle's runtime files into the updater's release tree, swaps the `current`
pointer, restarts, and health-probes. Model policy is read from a separate
files root on every activation. A model-policy bundle would have produced a
complete green trajectory — staged, promoted, restarted, health-probed, signed —
while changing nothing about resolution. That is the precise failure this
Definition exists to make impossible.

The second is architectural. The frozen `CapsuleEffectBundle` cannot validate
without a source tree ref, a capsule-executed build recipe, a runtime artifact
ref, test receipts, and dependency toolchain refs. The envelope was designed
around one story: a capsule took a source tree, ran a recorded build with a
recorded toolchain, ran tests, and produced a runtime artifact. Pushing a
configuration file through it would have required stubbing five fields whose
entire purpose is to attest to a build that never happened. The updater's
restart-and-health-probe loop, which restores the prior release on failure, is
likewise a binary-deployment safety loop; a configuration file exercises none
of it.

Promoted direction agrees. Model selection, fallback, and cycling are
system-owned processes rather than agent-edited configuration, and
`internal/modelpolicy` is already scheduled for generalization into
broker-mediated multi-call selection. The first thing the computer learns to
change about itself should not be a surface the system owns and has planned to
replace.

## The candidate: capsule-authored solitaire

A CoSuper capsule authors a solitaire capability for the computer: a rules
engine, a headless play API sufficient for automated play without a browser,
durable game persistence, play history, and the embedded UI. The capsule writes
the source, runs the build and the tests through `capsule_exec`, and freezes the
classified diff into a complete effect bundle whose five required refs are all
bound to that capsule's own execution receipts.

Solitaire is the right candidate for four reasons. It is unmistakably source
code rather than configuration. It touches no protected surface. Its
correctness is crisp, so a defect is a genuine defect rather than a staged
marker. And because the frontend is embedded into the Go binary, the UI and the
API ship as a single built runtime artifact inside the release the updater
already controls.

## The correction spine

Version A ships with a pre-declared rule defect that its own test suite does not
catch — for example a foundation that accepts a same-color build, or win
detection that fires on an incomplete tableau. A is verified, accepted,
materialized, and restarts healthy.

Admissible evidence then falsifies it: a headless API play sequence drives the
engine into the illegal state A accepted, producing a verifier receipt bound to
the operation. Version B corrects the rule and supersedes A as a forward
transition. After restart, the same replayed sequence is refused, and B is the
effective head.

The defect must be genuine and pre-declared, and it must be invisible to A's own
verification. If A's tests catch it, the bundle is refused and the mission has
proven the gate rather than the correction spine — a different and lesser
result. Recording the defect in a Define receipt before A is proposed is what
keeps the falsification from being reported later as a discovery.

Revocation closes the mission. An owner-directed rollback removes the
capability; the deployed API reports it absent. The additive solitaire tables
and their rows persist, because there is no migration framework and rollback
restores the code head, not the schema. That asymmetry is reported as a
property, never concealed.

## Route map

1. **Feasibility check (green):** confirm the CoSuper capsule carries a Go
   toolchain and a provisioned source tree, and determine whether staging serves
   the frontend from the updater-controlled release. Document either gap
   problem-first; if the frontend is served outside the release, the candidate
   becomes API-only until that is resolved.
2. **E2 expressibility pre-flight (green):** document one concrete way to write
   "B supersedes A" as an ordinary write on the existing event/decision/selfdev
   graph, or document the gap problem-first. Do not silently downgrade to E1.
   Note the binding constraint: the decision verifier admits exactly one input
   artifact ref (the mode receipt) and exactly one verifier ref, so the
   supersession citation must ride B's proposal event rather than its decision
   event.
3. **Mission 0 residual (red):** confirm the durable computer-bound owner key
   end to end after the recent issuance repair; recover or explicitly retire
   retained computer epoch 8253; clean up the `ak_45ce1796` row and the
   root-only auth rollback.
4. **Rehearsal (orange->red):** product-path `propose -> accept_once ->
   materialize -> rollback` on a trivial no-op source change, with a
   restart-durable read. The live flip is gated on rehearsal PASS.
5. **Live E2 (red):** capsule authors A -> accept -> falsify -> B supersedes ->
   restart proves B -> owner-directed revocation.
6. **RLM/Yaegi authoring upgrade:** explicitly NOT part of this Definition.
