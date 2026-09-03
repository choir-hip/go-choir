---
definition_version: 2

start:
  captured_at: "2026-09-02T21:10:00Z"
  source:
    canonical_ref: "main@a52ef06d"
    deploy_identity: "staging https://choir.news deployed_commit 42d47604; retained computer computer-03335285269bdba4f94377e56879f9e6 active epoch 831; Def 1 and Def 2 pending; effects OFF; pre-A checkpoint 99949fe2 published restore fence intact"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-02 read-only git status; clean single worktree /Users/wiz/go-choir"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns candidate A solitaire implementation and self-development promotion/restore verification."
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
      base: none
      scope: [internal/solitaire]
      disposition: paused
      evidence_ref: "docs/choir-effects-solitaire-candidate-spec-2026-08-19.md"
  observed_artifact:
    - claim: "Pre-A checkpoint 99949fe2 remains the published, verified, immutable restore fence on computer-03335285269bdba4f94377e56879f9e6."
      evidence_ref: "docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md; docs/ACTIVE.md"
    - claim: "Substrate and RLM prerequisites are formally serialized into Def 1 and Def 2; candidate A execution is paused until both definitions pass deployed acceptance."
      evidence_ref: "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
  unknowns:
    - "Exact runtime duration for CoSuper capsule compilation and test execution inside guest VM under RLM session cells."

finish:
  deliver: "The automatic computer executes autonomous in-VM self-development under effect-specific multiagent consensus on staging, authored exclusively through model-written Go on the RLM session interpreter: CoSuper capsule authors, builds, tests, freezes, and proposes candidate change A with five bundle refs bound to capsule receipts; qualified consensus under reversible-selfdev-v1 approves exact subject; promotes to live realization; play API verified; falsified with B; and whole-computer state restored back to pre-A checkpoint 99949fe2."
  artifact: "Authenticated staging trajectory carrying: frozen bundle A with the five required refs bound to capsule-exec receipts (candidate-spec, package-manifest, build-log, test-log, falsification-spec); qualified consensus/quorum receipts; promotion event + epoch receipt; play API evidence; B falsification evidence; restore-to-99949fe2 witness; Texture revisions wherever semantic state changed during the arc."
  acceptance:
    - action: "CoSuper authors, compiles, and tests internal/solitaire inside guest capsule exclusively via model-written Go cells in the session interpreter."
      proves: "RLM session interpreter fully supports autonomous software engineering workflows without fallback to ambient JSON tools."
      evidence_class: deployed_proof
    - action: "Freeze 5 bundle refs (candidate-spec, package-manifest, build-log, test-log, falsification-spec) bound to capsule execution receipts and submit proposal under reversible-selfdev-v1."
      proves: "Candidate A is computer-authored, not hand-authored, and satisfies artifact bundle completeness."
      evidence_class: deployed_proof
    - action: "Execute qualified consensus panel approval on exact subject with independent-reviewer seat running on gpt-5.6-terra per owner direction."
      proves: "Autonomy boundary holds under ratified decision policy."
      evidence_class: deployed_proof
    - action: "Live E2 verification: A promotes -> solitaire plays via headless API writing rows -> evidence falsifies A -> B supersedes -> restart shows B effective -> restore to 99949fe2."
      proves: "Live platform promotion path and whole-computer restore return the machine to the exact pre-A state."
      evidence_class: deployed_proof
    - action: "Supervision legibility: Texture revisions authored by Texture agent appear wherever semantic state changed; turn-outcome vs revision-outcome census confirms semantic changes produce exactly one version each."
      proves: "Living document protocol and Texture live supervision hold under autonomous RLM execution."
      evidence_class: deployed_proof
  rollback: "Immediate acceptance-fenced restore to pre-A checkpoint 99949fe2."
  landing:
    required: true
    environment: staging
    required_receipts: [source_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "Any candidate A authoring step falls back to legacy JSON tools."
    - "Candidate bundle lacks any of the 5 required capsule-bound artifact references."
    - "Promotion occurs without qualified consensus approval."
    - "Restore fails to return computer state to exact pre-A checkpoint 99949fe2."
    - "Texture supervision exposes only post-hoc summary without live revisions during open work."

boundaries:
  mutation_class: red
  authority_sources:
    - "Choir Doctrine C14/C15 (Effect-specific multiagent consensus, reversible self-development)"
    - "Owner direction 2026-09-02 (Candidate A as the empirical proof of RLM cutover)"
    - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
  must_preserve:
    - "Pre-A checkpoint 99949fe2 untouched until restore verification"
    - "Effects OFF until promotion within this Definition's own consensus gates"
    - "Single actuator surface: model-written Go only"
    - "Two writer classes: AuthorAppAgent for Texture; CAS for owner"
  excluded:
    - "World Wire redesign, prompt caching, or naming cutover"
  protected_surfaces:
    - "internal/solitaire"
    - "internal/agentcore/selfdev_texture_join.go"
    - "internal/store/computer_events.go"
    - "internal/store/texture.go"
  completion_evidence_floor:
    - deployed_proof

conjectures:
  - id: candidate-a-proves-rlm-cutover
    conjecture: "Executing the complete candidate A self-development arc via model-written Go conclusively proves that the RLM session interpreter provides ambient tool parity and sufficient expressivity."
    falsifier: "CoSuper encounters an execution or expressivity barrier requiring reintroduction of legacy ambient JSON tools."
  - id: panel-diversity-terra-independent
    conjecture: "A two-model verification panel (luna authoring + terra reviewing) is adequate for candidate A authorization under reversible-selfdev-v1."
    falsifier: "Candidate A promotion passes while the two models share a correlated blind spot that restore/falsification exposes."

heresies:
  discovered: []
  introduced: []
  repaired: []

measures:
  - name: rlm_authoring_purity
    kind: gate
    baseline: "0% (prior runs used ambient JSON tools)"
    desired: "100% Go cells"
    decision_use: "proves RLM cutover validity"
    cannot_prove: "cannot prove absence of semantic bugs in solitaire"
  - name: restore_fidelity
    kind: gate
    baseline: "checkpoint 99949fe2 published"
    desired: "exact pre-A state verified"
    decision_use: "certifies safety and reversibility of autonomous engineering"
    cannot_prove: "cannot prove forward performance of candidate A"

now:
  status: blocked_incomplete
  slice: "Supervised Self-Development on RLM definition baseline"
  question: none
  reconciliation:
    observed_at: "2026-09-02T21:10:00Z"
    source_ref: "main@a52ef06d"
    deploy_identity: "staging https://choir.news; computer-03335285269bdba4f94377e56879f9e6 active epoch 831"
    authority_identities:
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
      - "docs/definitions/choir-supervised-self-development-candidate-proof-2026-08-20.md"
      - "docs/definitions/choir-substrate-and-scheduling-readiness-2026-09-02.md"
      - "docs/definitions/choir-rlm-session-interpreter-cutover-2026-09-02.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "git status --short clean on main@a52ef06d"
    status: reconciled
  candidate:
    id: candidate-a-solitaire
    state: paused
    ref: none
    owner: CoSuper
    base: "main@a52ef06d"
    digest: none
    scope: [internal/solitaire]
  decision:
    selected: "Inherit candidate A solitaire acceptance criteria from choir-supervised-self-development-candidate-proof-2026-08-20.md, updated to execute strictly via RLM session cells on computer-03335285269bdba4f94377e56879f9e6 with pre-A checkpoint 99949fe2."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
    owner_ratification_ref: "owner direction 2026-09-02"
    recorded_at: "2026-09-02T21:10:00Z"
    consequence: "Candidate A execution is strictly gated behind Definition 1 and Definition 2."
  evidence_refs:
    - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
    - "docs/definitions/choir-supervised-self-development-candidate-proof-2026-08-20.md"
  blocker_or_risk: "Strict serialization: Gated behind successful deployed acceptance of Definition 1 (substrate) and Definition 2 (RLM cutover)."
  next_action: "Remain paused until Definition 1 and Definition 2 achieve complete deployed acceptance on staging."

receipts:
  - id: selfdev-rlm-baseline-2026-09-02
    boundary: define
    commit_or_artifact: "main@a52ef06d"
    proof_refs:
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
      - "docs/definitions/choir-supervised-self-development-candidate-proof-2026-08-20.md"
    rollback_ref: "checkpoint 99949fe2 remains the immutable pre-A fence"
    disposition: "accepted as Definition 3 governing candidate A self-development on RLM; successor to candidate-proof-2026-08-20; paused pending Def 1 and Def 2"
    problem_ref: not_applicable
    authorization_ref: "owner direction 2026-09-02; agentic consensus unanimous Option B"
    candidate_or_evidence_refs: []
    landing:
      source_commit: not_applicable
      ci_ref: not_applicable
      deploy_ref: not_applicable
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: "docs/ACTIVE.md; docs/mission-graph.yaml; docs/doc-authority-manifest.yaml"
---

# Definition 3: Supervised Self-Development on RLM

This Definition executes Phase 3 of the 3-Definition autonomous engineering sequence.

## Core Deliverables
1. **RLM-Authored Candidate A**: CoSuper inside guest capsule authors, builds, and tests `internal/solitaire` exclusively via model-written Go cells in the persistent session interpreter.
2. **Bundle Freeze & Qualified Consensus**: Freeze 5 bundle refs bound to capsule receipts (candidate-spec, package-manifest, build-log, test-log, falsification-spec); qualified consensus panel evaluates and approves exact subject under `reversible-selfdev-v1` with gpt-5.6-terra independent reviewer.
3. **Promotion & Live Play Verification**: Promote candidate A to live realization and verify play API response.
4. **Falsification & Full Restore**: Author candidate B to falsify A, promote, then execute acceptance-fenced restore back to pre-A checkpoint `99949fe2`.

Execution of this Definition is strictly gated behind successful completion and deployed acceptance of Definition 1 and Definition 2.
