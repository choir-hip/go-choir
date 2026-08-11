---
definition_version: 2
definition_id: choir-supervised-self-development-effects-2026-08-11
execution_mode: mission_orchestrator

start:
  captured_at: 2026-08-11T02:10:00Z
  source:
    canonical_ref: main@2379616d
    deploy_identity: "staging https://choir.news deployed_commit 26c53692494aed1a2ea337550990d70c7cd16735 via CI 31445846546 (2026-08-11T00:54:41Z); /health ok"
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
    - claim: Accepted/rejected decision binding joins operation identity, verifier refs, mode receipt, and both heads.
      evidence_ref: internal/agentcore/self_development_decision_binding.go
    - claim: Frozen CapsuleEffectBundle ladder exists (content digest, verifier-signed, classifier-classified; BuildBundleFromDiff).
      evidence_ref: internal/capsule/transaction/builder.go
    - claim: Updater ReleaseManifest with probe/apply surface exists.
      evidence_ref: internal/updater/updater.go
    - claim: Model policy is computer-scoped durable state read on activation, with apply/rollback helpers prepared for CTS acceptance but never executed (effects OFF).
      evidence_ref: internal/modelpolicy/model_policy.go; /tmp/cts-acceptance-model-policy-apply.py (prepared, unexecuted)
    - claim: No usable owner bearer remains; retained computer failed epoch 8253; CTS blocked on exact owner presence in headed Chrome with Touch ID; effects remain OFF.
      evidence_ref: docs/choir-crashed-prime-session-review-2026-08-09.md; docs/definitions/choir-continuous-texture-supervision-2026-08-07.md now.next_action
  unknowns:
    - Whether rival-proposal/supersession semantics needed for E2 are expressible on the existing event/decision/selfdev operation graph without a new settlement subsystem. Pre-flight must document this before coding (problem-documentation-first); do not silently downgrade to E1.
    - Whether the owner issues a durable computer-bound key (Mission 0) and ratifies CTS supersession (Mission 1) before this Definition can execute.
    - Whether retained computer epoch 8253 is recovered or explicitly retired.

finish:
  deliver: "One supervised self-development candidate accepted on staging: the computer makes one real change to its own working state (computer-scoped model policy) under granted rules, legible on the canonical tape, durable across a restart, through the correction spine: rival proposal A is accepted, admissible evidence falsifies A, correction B supersedes A, and restart proves B effective."
  artifact: "An authenticated staging computer trajectory carrying: mode receipt, accepted decision A binding operation identity + verifier refs + both heads, materialization receipt with checkpoint/route TransitionPromote, applied model-policy A digest, falsifying admissible evidence, superseding decision B with TransitionRollback-equivalent forward transition, and a post-restart proof that B is the effective policy head — all joined to the canonical computer event chain."
  acceptance:
    - action: "Product-path rehearsal on staging: propose -> accept_once -> materialize -> rollback (and restart-durable read of applied state) with the live flip gated on rehearsal PASS."
      proves: "Landing machinery (frozen bundle, mode CAS, materializer, checkpoint/route, forward rollback) works end to end before any live accept_once."
      evidence_class: deployed proof
    - action: "Live E2 proof: accept A (model-policy bundle) -> admissible evidence falsifies A -> correction B supersedes A -> restart -> B effective."
      proves: "The vision correction spine lands on the canonical tape and survives restart."
      evidence_class: deployed proof
    - action: "Replay the accepted trajectory from canonical events."
      proves: "Single semantic authority reconstruction; no second state authority was introduced."
      evidence_class: deployed proof
  rollback: "Mode CAS -> off; forward rollback transaction restores the prior ComputerVersion checkpoint and original model-policy bytes; revert the behavior commits through origin/main and CI to the last accepted runtime. Direct policy-PUT shortcuts are not the proof and are not a rollback path."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - Effects remain OFF (no accepted + materialized + restart-proven policy change on staging).
    - Only E1 evidence exists (single apply without correction supersession) at registry close.
    - The E2 rival/supersession gap was downgraded instead of documented and fixed.
    - CTS remains active in all three registries without a successor pointer.

boundaries:
  mutation_class: red
  authority_sources: [owner-ratified decisions, docs/choir-vision.md, docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/standing-questions.md, docs/choir-self-development-roadmap-2026-08-11.md, AGENTS.md]
  must_preserve:
    - Canonical computer event chain is the single semantic state authority.
    - Effects stay OFF until the staging rehearsal gate passes.
    - No headless/CDP/virtual-authenticator substitute for exact owner presence; no SSH-shaped operations; product path only.
    - Problem-documentation-first: a discovered platform problem is documented in a code-free Define receipt before any repair-code commit.
  excluded:
    - RLM/Yaegi actor authoring (successor capability mission; Phase 1 of the RLM memo preserves effects-OFF).
    - Source/code effects via ReleaseManifest (D3).
    - Production environment.
    - CTS full supervision choreography (finishing CTS cannot deliver the vision proof; its evidence folds into this Definition).
    - In-choir computer-control draft (separate successor authority).
  protected_surfaces: [self-development mode CAS, canonical computer event chain, materializer + decision binding, checkpoint/route projection, model-policy authority, updater root, vmctl lifecycle, auth/session renewal, gateway/provider calls, deployment routing]
  completion_evidence_floor: [deployed proof, independent review of frozen bundle + decision binding]

measures:
  - name: rehearsal pass
    kind: gate
    baseline: 0
    desired: accept->materialize->rollback rehearsal PASS on staging before live accept_once
    decision_use: unlocks the live flip
    cannot_prove: the vision correction spine
  - name: tape segment count
    kind: weak_signal
    baseline: 0
    desired: >= 6 (mode receipt, accept A, materialize A, falsify, supersede B, restart proof)
    decision_use: inspect the next transition; never advances complete alone
    cannot_prove: acceptance
  - name: consensus agreement
    kind: weak_signal
    baseline: roadmap convergent panel unanimous on sequence shape (3/3 verdicts)
    desired: independent review acceptance of frozen E2 candidate
    decision_use: schedule review; never settles authority
    cannot_prove: the product works

now:
  status: blocked_incomplete
  slice: "Author the effects Definition (Mission 2); CTS supersession and key issuance remain owner gates."
  question: "E2 rival/supersession expressibility on the existing decision-binding graph; owner ratification of CTS supersession and Mission 0."
  reconciliation:
    observed_at: 2026-08-11T02:10:00Z
    source_ref: main@2379616d
    deploy_identity: "staging deployed_commit 26c53692494aed1a2ea337550990d70c7cd16735"
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/choir-self-development-roadmap-2026-08-11.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: 2026-08-11 read-only git status (clean)
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Effects-first on existing substrate; D1 model-policy content x D2 frozen-bundle envelope; E2 correction-spine acceptance; RLM strictly after; CTS superseded as blocked_incomplete."
    kind: architecture
    status: proposal
    source: orchestrator
    evidence_ref: docs/choir-self-development-roadmap-2026-08-11.md; .agentic-consensus/self-dev-roadmap/{divergent,lateral,convergent}/
    owner_ratification_ref: none
    recorded_at: 2026-08-11T02:10:00Z
    consequence: "Nothing executes until the owner ratifies this architecture decision, the CTS supersession, and Mission 0; ratification must be recorded here before the rehearsal slice starts."
  evidence_refs: [docs/choir-self-development-roadmap-2026-08-11.md, docs/choir-crashed-prime-session-review-2026-08-09.md, docs/memo-persistent-rlm-actors-2026-08-09.md]
  blocker_or_risk: "Owner ratification of CTS supersession (C1) and Mission 0 durable key; E2 expressibility pre-flight; retained computer epoch 8253 disposition."
  next_action: "Owner reviews this Definition and the roadmap; on ratification, record the decision here, execute Mission 1 (registry flip: CTS superseded_incomplete + this Definition registered across ACTIVE.md/mission-graph.yaml/doc-authority-manifest.yaml), then run the E2-expressibility pre-flight."

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
    registry_conformance_ref: not_applicable (this Definition not yet registered)

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

The CTS Definition stays owner-ratified until the owner ratifies this
successor; at that point all three registries flip in one commit (registry
hygiene contract) and CTS's evidence — mailbox gaps, policy apply/rollback
helpers, requirement audit, implementation inventory — folds into this
Definition's evidence refs. CTS's own `not_done_when` forbids effects while
OFF, so finishing CTS cannot deliver the vision proof; that is the
supersession justification.

## Route map from the converged roadmap

1. **Mission 0 (owner, red):** durable, narrow, computer-bound key from exact
   owner presence; wrong-target 403 and effects-OFF proofs; recover or
   explicitly retire retained computer epoch 8253.
2. **Mission 1 (green):** CTS superseded as `blocked_incomplete`; this
   Definition registered in ACTIVE.md, mission-graph.yaml,
   doc-authority-manifest.yaml.
3. **Mission 3 (orange->red):** product-path plumbing + staging rehearsal
   `propose -> accept_once -> materialize -> rollback`, restart-durable read;
   live flip gated on rehearsal PASS.
4. **Mission 4 (red):** live E2 proof — accept A (model-policy bundle) ->
   falsify -> B supersedes -> restart proves B.
5. **Mission 5 (orange/red):** RLM/Yaegi authoring upgrade — explicitly NOT
   part of this Definition.

## E2 expressibility pre-flight (blocking question)

The roadmap assumption: rival-proposal/supersession semantics are expressible
on the existing event/decision/selfdev operation graph without inventing a new
settlement subsystem. Before any rehearsal code, document one concrete way to
represent "B supersedes A" as an ordinary write on the existing graph, or
document the gap problem-first. Do not silently downgrade to E1.
