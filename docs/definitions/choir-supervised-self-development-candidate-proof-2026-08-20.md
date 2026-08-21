---
definition_version: 2

start:
  captured_at: "2026-08-20T16:00:00Z"
  source:
    canonical_ref: "main@bc119278"
    deploy_identity: "staging https://choir.news proxy e6ee8c68; guest autoputer ab756117 on retained computer computer-03335285269bdba4f94377e56879f9e6 active epoch 333 (propose_only generation 1, effects OFF); pre-A checkpoint 99949fe2 published; operation selfdev-ccf0f1ec0e851750f253fe5f5ed97974 executing"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-08-20 read-only git status; clean single worktree /Users/wiz/go-choir"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns itself, its receipts, and the three navigation registries."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  candidates:
    - id: candidate-a-solitaire
      ref: none
      base: "ab756117315719be669692a9e5ed741411ca13f4"
      scope: [internal/solitaire]
      disposition: active
      evidence_ref: "docs/choir-effects-solitaire-candidate-spec-2026-08-19.md"
  observed_artifact:
    - claim: "Pre-A checkpoint 99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7 published and verified at epoch 324 as immutable restore fence."
      evidence_ref: "docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md"
    - claim: "Substrate repaired and verified live on staging computer 03335285... at epoch 333 on commit ab756117; Texture execution_request rewake deployed; pre-promotion safeguards F1-F4 applied."
      evidence_ref: "docs/evidence/effects-red-super-texture-rewake-2026-08-20.md; docs/reports/choir-24-hour-architecture-and-effects-progress-report-2026-08-20.md"
    - claim: "CI pipeline optimized to 8.0m across 12 shards; total landing loop under 13 minutes."
      evidence_ref: "https://github.com/choir-hip/go-choir/actions/runs/32378407753"
    - claim: "Conductor intake and Texture live supervision architecture specified and aligned; hardcoded example routes cleanly removed from platform codebase."
      evidence_ref: "docs/texture-live-supervision-architecture.md; docs/reports/choir-autonomous-supervision-and-self-development-master-blueprint-2026-08-20.md"
  unknowns:
    - "Exact runtime duration for CoSuper capsule compilation and test execution inside guest VM."

finish:
  deliver: "The automatic computer executes autonomous in-VM self-development under effect-specific multiagent consensus on staging: CoSuper capsule authors, builds, tests, freezes, and proposes candidate change A; qualified consensus approves exact subject; promotes to live realization; play API verified; falsified with B; and whole-computer state restored back to pre-A checkpoint 99949fe2."
  artifact: "An authenticated staging computer trajectory carrying: frozen candidate A bundle with 5 capsule-bound refs (SourceTreeRef, BuildRecipeRef, DependencyToolchainRefs, TestReceipts, RuntimeArtifactRef); qualified consensus receipt under reversible-selfdev-v1; promotion event; live play verification receipt; falsification and supersession receipt; and acceptance-fenced restore receipt back to checkpoint 99949fe2."
  acceptance:
    - action: "CoSuper authors and tests candidate change inside disposable guest capsule on staging computer-03335285269bdba4f94377e56879f9e6."
      proves: "Self-development authorship runs inside guest capsules via the choir CLI, not external harness git commits."
      evidence_class: deployed proof
    - action: "Freeze candidate bundle A with 5 capsule-bound artifact references."
      proves: "Speculative candidate changes remain inert and cryptographically bound prior to acceptance."
      evidence_class: deployed proof
    - action: "Evaluate frozen candidate A under qualified consensus policy reversible-selfdev-v1."
      proves: "Policy-governed multiagent consensus authorizes promotion without per-candidate human approval."
      evidence_class: deployed proof
    - action: "Promote candidate A -> verify live play -> falsify A -> supersede with B."
      proves: "The autonomous self-development and correction spine operates end-to-end on a live persistent computer."
      evidence_class: deployed proof
    - action: "Execute acceptance-fenced restore back to pre-A checkpoint 99949fe2."
      proves: "Complete whole-computer restore back to baseline state with full event tape causality preserved."
      evidence_class: deployed proof
    - action: "Texture living supervision document updates monotonically on choir.news with transcluded citations."
      proves: "Real-time human-readable supervision is observable without raw log inspection."
      evidence_class: deployed proof
  rollback: "Restore to published pre-A checkpoint 99949fe2; git revert of platform commits remains independent repo fallback."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "Candidate bundle lacks any of the 5 required capsule-bound artifact references."
    - "Effects are enabled prior to qualified consensus authorization."
    - "Promotion occurs without qualified consensus receipt under reversible-selfdev-v1."
    - "Restore fails to return computer state to exact pre-A checkpoint 99949fe2."
    - "Texture supervision exposes only post-hoc summary without live revisions committed during open work."

boundaries:
  mutation_class: red
  authority_sources: [docs/choir-doctrine.md, docs/agent-product-doctrine.md, docs/computer-ontology.md, AGENTS.md]
  must_preserve:
    - "Single ComputerID (computer-03335285269bdba4f94377e56879f9e6) plus canonical event chain is the evolving computer."
    - "Reversibility is a recovery property; multiagent consensus policy is the autonomy boundary."
    - "Effects remain OFF until qualified consensus passes."
    - "Texture owns canonical document versions; never promotion, checkpoint, or route authority."
    - "Pre-A checkpoint 99949fe2 remains the immutable restore baseline."
  excluded:
    - "Yaegi RLM private Go kernel cutover (owned by successor definition choir-private-go-actor-kernel-2026-08-12)."
    - "Model policy editing as self-development effect."
    - "Production environment mutations."
  protected_surfaces: [self-development mode CAS, canonical computer event chain, materializer + decision binding, checkpoint/route projection, Texture canonical writes, vmctl lifecycle, auth/session renewal]
  completion_evidence_floor: [deployed proof, independent review of frozen bundle + decision binding]

measures:
  - name: pre-A restore fence
    kind: gate
    baseline: "checkpoint 99949fe2 published at epoch 324"
    desired: "verified immutable restore baseline"
    decision_use: "authorizes promotion and restore testing"
    cannot_prove: "that candidate A authors cleanly"
  - name: capsule-bound bundle refs
    kind: gate
    baseline: 0
    desired: 5
    decision_use: "authorizes consensus evaluation"
    cannot_prove: "that candidate code passes verification"
  - name: qualified consensus quorum
    kind: gate
    baseline: "policy reversible-selfdev-v1 defined"
    desired: "unanimous quorum without unresolved safety dissent"
    decision_use: "authorizes event appender promotion"
    cannot_prove: "that the promoted feature is defect-free"
  - name: living texture revisions
    kind: weak_signal
    baseline: 1
    desired: ">= 6 monotonically incrementing revisions with transcluded citations"
    decision_use: "inspects live supervision progress"
    cannot_prove: "semantic correctness of candidate"

now:
  status: superseded
  superseded_by: "docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md (carries forward candidate A proof on the ratified scheduling contract; this Definition's finish.acceptance is inherited unchanged)"
  slice: "Superseded 2026-08-21 before scheduler implementation landed. See successor."
  reconciliation:
    observed_at: "2026-08-21 (see docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md)"
    source_ref: "staging deployed e42c65c0; retained computer epoch 361; capsule memory reclaim works; competing pending Texture execution requests for distinct operations ping-pong fresh assignments, cancelling every CoSuper before author/build/freeze; no candidate artifact exists."
    deploy_identity: "Staging /health reports deployed proxy commit e42c65c04f0681e4dd695853ed1396fed736a467; retained computer computer-03335285269bdba4f94377e56879f9e6 reports active epoch 361; effects remain OFF; pre-A checkpoint 99949fe2 remains published and untouched."
    authority_identities: [docs/choir-vision.md, docs/choir-doctrine.md, docs/standing-questions.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "clean single worktree /Users/wiz/go-choir"
    status: reconciled
  candidate:
    id: candidate-a-solitaire
    state: ready
    ref: "capsule-isolated guest workspace"
    owner: cosuper
    base: "ab756117315719be669692a9e5ed741411ca13f4"
    digest: pending_freeze
    scope: [internal/solitaire]
  decision:
    selected: "Effect-specific multiagent consensus is the autonomy boundary; reversibility is a recovery property. Restore of an effect excursion consumes the choir-tape-recovery-2026-08-13 substrate and is addressed by event head and acceptance-fenced. Checkpoints bind event head, CodeRef, ArtifactProgramRef, VM-local content witness, and frontend identity. CoSuper authors and proposes inside guest capsules. A policy-selected qualified panel authorizes each exact effect subject under frozen eligibility, independence, quorum, dissent, expiry, and consequence rules. Reversible and irreversible effects are both admissible."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "docs/problems/irreversible-effects-human-gate-drift-2026-08-13.md; docs/choir-vision.md; docs/choir-doctrine.md"
    owner_ratification_ref: "owner correction 2026-08-13: irreversible effects are not outside the autonomy window; effect-specific multiagent consensus is the governing boundary."
    recorded_at: "2026-08-13T14:18:16Z"
    consequence: "Execute reversible self-development candidate authorship, qualified consensus under reversible-selfdev-v1, promotion, live play verification, falsification, and acceptance-fenced restore back to checkpoint 99949fe2."
  evidence_refs:
    - "docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md"
    - "docs/evidence/effects-red-capsule-memory-budget-exhaustion-2026-08-21.md"
    - "docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md"
    - "docs/evidence/effects-red-super-model-policy-fallback-2026-08-21.md"
    - "docs/texture-live-supervision-architecture.md"
    - "docs/reviews/architecture-review-texture-cosuper-memory-2026-08-21.md"
    - "docs/reports/choir-autonomous-supervision-and-self-development-master-blueprint-2026-08-20.md"
  blocker_or_risk: "Competing pending Texture execution requests for distinct self-development operations cause persistent Super cycles to open fresh assignments that supersede and cancel their predecessors (memory reclaim works; scheduling does not yet). No candidate artifact, bundle, proposal, promotion, or live state write exists; effects remain OFF."
  next_action: "Under the ratified scheduling contract, select exactly one non-expired request by computer arrival ordinal/FIFO, settle stale duplicates whose operations already have terminal attempts as late evidence, and keep retryable refusal pending. Only after the loop is stopped, resume CoSuper candidate A authoring, five-ref freeze, qualified consensus, promotion, live-play verification, falsification with B, and restore. Effects remain OFF."

receipts:
  - id: effects-candidate-proof-baseline-2026-08-20
    boundary: define
    commit_or_artifact: "main@bc119278"
    proof_refs: [docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md, docs/evidence/effects-red-super-texture-rewake-2026-08-20.md, docs/reports/choir-24-hour-architecture-and-effects-progress-report-2026-08-20.md]
    rollback_ref: "checkpoint 99949fe2 remains the pre-A fence"
    disposition: "accepted as compact successor Definition v2 establishing clean baseline on staging epoch 333 (ab756117) with pre-A checkpoint 99949fe2 verified; supersedes choir-supervised-self-development-effects-2026-08-11"
    problem_ref: not_applicable
    authorization_ref: "Choir Doctrine C14/C15; skills/definition/SKILL.md Definition v2 migration"
    candidate_or_evidence_refs: [docs/choir-effects-solitaire-candidate-spec-2026-08-19.md]
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: staging /health deployed_commit e6ee8c68
      environment_identity: retained computer-03335285269bdba4f94377e56879f9e6 active epoch 333; guest autoputer ab756117; propose_only generation 1; live op selfdev-ccf0f1ec executing
      deployed_acceptance: "clean baseline verified; candidate A authorship ready"
    registry_conformance_ref: "docs/ACTIVE.md; docs/mission-graph.yaml; docs/doc-authority-manifest.yaml"
---
