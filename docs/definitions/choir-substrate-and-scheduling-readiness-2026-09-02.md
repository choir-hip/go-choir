---
definition_version: 2

start:
  captured_at: "2026-09-02T21:00:00Z"
  source:
    canonical_ref: "main@a52ef06d"
    deploy_identity: "staging https://choir.news deployed_commit 42d476044ec80efe8a31d043af577ad77ba7572b; retained computer computer-03335285269bdba4f94377e56879f9e6 active epoch 831; guest boot stable; pre-A checkpoint 99949fe2 published and verified as immutable restore fence; effects OFF"
  worktree_inventory:
    status: reconciled
    evidence_ref: "2026-09-02 read-only git status; clean single worktree /Users/wiz/go-choir"
    preservation_rule: "Preserve every non-primary worktree and all unrelated WIP; this Definition owns itself, its receipts, and the scheduling and vmctl boot surfaces."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: none
  observed_artifact:
    - claim: "Pre-A checkpoint 99949fe2 remains the published, verified, immutable restore fence on computer-03335285269bdba4f94377e56879f9e6."
      evidence_ref: "docs/evidence/effects-red-pre-a-checkpoint-published-2026-08-19.md; docs/ACTIVE.md"
    - claim: "ArrivalOrdinal ordering and store-level FIFO selection shipped in commit cc2cb702 and deployed."
      evidence_ref: "internal/store/lifecycle.go:1306-1379; internal/store/texture_turn_scheduler_test.go"
    - claim: "Held-computer boot outage closed 2026-08-28 (commit 42d47604); guest boot stable at epoch 831, but residual body scans (ogListAllByMetadata, mailbox backlog paging, sweepPassivatedSpawnedCoagentWork) remain unclustered and could re-enter boot death on corpus growth."
      evidence_ref: "docs/reports/choir-held-computer-boot-outage-postmortem-2026-08-28.md; docs/evidence/root-cause-clustering-objectgraph-body-scan-2026-08-28.md"
    - claim: "Compete-and-cancel supersession loop documented 2026-08-21 requires deployed proof under real competing request load to certify structure cannot ping-pong assignments."
      evidence_ref: "docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md; internal/agentcore/super_controller.go:377-435"
  unknowns:
    - "Whether nine undelivered CoSuper cancel producer reports from the 08-19 storm require explicit tombstoning before fresh assignment selection can proceed cleanly."

finish:
  deliver: "The persistent staging computer (computer-03335285269bdba4f94377e56879f9e6) boots cleanly through the product path without maintenance holds and processes competing execution requests in strict FIFO arrival-ordinal order without assignment supersession."
  artifact: "Deployed staging proof: FIFO scheduler selects exactly one work item per Super cycle from competing requests, restart mid-run preserves selection, stale duplicates settle as late evidence without new assignments, and product-path normal-boot reaches guest health 200 without RUNTIME_MAINTENANCE_HOLD."
  acceptance:
    - action: "Queue >= 2 competing Texture execution requests on computer-03335285269bdba4f94377e56879f9e6 via CLI/API and observe persistent Super across >= 3 cycles."
      proves: "Exactly one live CoSuper assignment executes; other requests remain pending untouched; zero ping-pong supersession."
      evidence_class: deployed_proof
    - action: "Simulate service restart while one of the competing requests is in-flight."
      proves: "Durable computer-scoped ArrivalOrdinal selection is preserved across restart without re-supersession (I26 durability)."
      evidence_class: deployed_proof
    - action: "Settle stale duplicate requests whose operations have terminal attempts."
      proves: "Stale duplicates settle as late evidence without spawning new CoSuper assignments."
      evidence_class: deployed_proof
    - action: "Execute product-path boot probe (>= 5 consecutive normal boots without hold parameters) and poll guest /health with hard timeout."
      proves: "Guest reaches health 200 within deadline without RUNTIME_MAINTENANCE_HOLD; residual scans do not hang normal boot."
      evidence_class: deployed_proof
  rollback: "Git revert of scheduling and vmctl commits; checkpoint 99949fe2 remains the immutable pre-A fence."
  landing:
    required: true
    environment: staging
    required_receipts: [source_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "Competing Texture execution requests trigger cancellation of an in-flight assignment."
    - "Normal boot hangs or requires RUNTIME_MAINTENANCE_HOLD to reach health 200."
    - "More than one CoSuper assignment is live simultaneously."
    - "Candidate A authoring or RLM session interpreter coding is attempted in this Definition."

boundaries:
  mutation_class: red
  authority_sources:
    - "Choir Doctrine I26 (Super singleton coherence, scheduling contract)"
    - "Owner direction 2026-09-02 (Option B sequence: substrate first, no parallel coding)"
    - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
  must_preserve:
    - "Two writer classes: AuthorAppAgent for Texture; CAS for owner; texture_turn_committed is not a revision"
    - "Pre-A checkpoint 99949fe2 untouched"
    - "Effects remain OFF; no live mail/trading/mutations"
  excluded:
    - "RLM session interpreter code (owned by Def 2)"
    - "Candidate A authoring and solitaire implementation (owned by Def 3)"
    - "World Wire redesign, prompt caching, or naming cutover"
    - "Cold-recover B14 image rebuilding (residual only if required for live computer recovery)"
  protected_surfaces:
    - "internal/agentcore/super_controller.go"
    - "internal/store/lifecycle.go"
    - "internal/vmctl/handlers.go"
  completion_evidence_floor:
    - deployed_proof

conjectures:
  - id: fifo-scheduler-suffices
    conjecture: "A durable computer-scoped arrival ordinal plus FIFO among non-expired requests eliminates the assignment supersession loop without priority or preemption machinery."
    falsifier: "An active CoSuper assignment is cancelled or superseded by a newly arrived Texture execution request."
  - id: residual-scan-clustering-prevents-hang
    conjecture: "Characterizing and bounding residual objectgraph body scans prevents the normal-boot 3/3 hang from recurring without requiring full cold recovery."
    falsifier: "Product-path normal boot exceeds health deadline or hangs on database scan during consecutive boot probe."

heresies:
  discovered:
    - "Competing pending Texture execution requests ping-pong fresh assignments, cancelling every CoSuper before author/build/freeze (discovered 2026-08-21)."
    - "vmctl normal boots hung 3/3 while hold-param boots succeeded 4/4 due to unindexed body scans (discovered 2026-08-25)."
  introduced: []
  repaired:
    - "Store-level ArrivalOrdinal and super_controller fail-closed deadlines implemented."

measures:
  - name: supersession_count
    kind: gate
    baseline: "repeated ping-pong cancellations observed 2026-08-21"
    desired: "0"
    decision_use: "certifies FIFO scheduling contract"
    cannot_prove: "cannot prove RLM interpreter readiness"
  - name: normal_boot_success_rate
    kind: gate
    baseline: "epoch 831 stable on 42d47604"
    desired: "5/5 consecutive normal boots reaching health 200"
    decision_use: "certifies substrate unblocked for Def 2 sealed proof"
    cannot_prove: "cannot prove guest code correctness"

now:
  status: working
  slice: "Substrate & Scheduling Readiness definition baseline"
  question: none
  reconciliation:
    observed_at: "2026-09-02T21:00:00Z"
    source_ref: "main@a52ef06d"
    deploy_identity: "staging https://choir.news deployed_commit 42d476044ec80efe8a31d043af577ad77ba7572b; computer-03335285269bdba4f94377e56879f9e6 active epoch 831"
    authority_identities:
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
      - "docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "git status --short clean on main@a52ef06d"
    status: reconciled
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Extract substrate and scheduling obligations from choir-scheduling-and-candidate-proof-2026-08-21.md into standalone Definition 1; target computer-03335285269bdba4f94377e56879f9e6 at epoch 831+ with fence 99949fe2."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
    owner_ratification_ref: "owner direction 2026-09-02"
    recorded_at: "2026-09-02T21:00:00Z"
    consequence: "Supervised self-development candidate A is tabled to Def 3; execution focuses strictly on FIFO contract and boot stability."
  evidence_refs:
    - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
    - "docs/evidence/effects-red-assignment-supersession-loop-2026-08-21.md"
    - "docs/reports/choir-held-computer-boot-outage-postmortem-2026-08-28.md"
  blocker_or_risk: "Residual same-substrate scans outside the 42d47604 repair could re-enter boot death loop on corpus growth. Probe must certify 5/5 boots pass within timeout."
  next_action: "Execute 5x normal-boot probe on computer-03335285269bdba4f94377e56879f9e6, then run 2-request competing FIFO selection test on staging."

receipts:
  - id: substrate-scheduling-baseline-2026-09-02
    boundary: define
    commit_or_artifact: "main@a52ef06d"
    proof_refs:
      - "docs/reports/choir-harness-state-and-rlm-cutover-plan-2026-09-02.md"
      - "docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md"
    rollback_ref: "checkpoint 99949fe2 remains the immutable pre-A fence"
    disposition: "accepted as Definition 1 establishing strict substrate-first serialization; supersedes substrate scope of choir-scheduling-and-candidate-proof-2026-08-21.md"
    problem_ref: assignment-supersession-loop-2026-08-21
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

# Definition 1: Substrate & Scheduling Readiness

This Definition executes Phase 1 of the 3-Definition autonomous engineering sequence.

## Core Deliverables
1. **FIFO Scheduling Contract**: Competing execution requests are scheduled strictly by computer-scoped `ArrivalOrdinal`. Stale duplicate requests are settled as late evidence. In-flight assignments are protected from supersession cancellations across restarts.
2. **vmctl Normal-Boot Stability**: Prove product-path normal boots reach `/health` 200 within deadline without relying on `RUNTIME_MAINTENANCE_HOLD`.
3. **Producer Report Tombstoning**: Cleanly drain or settle the 9 undelivered cancel producer reports from the 08-19 storm.

Candidate A and RLM session interpreter implementations are strictly excluded from this Definition.
