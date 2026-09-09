---
definition_version: 2
definition_id: choir-rlm-settlement-gate-2026-09-09
execution_mode: draft_non_executable

start:
  captured_at: "2026-09-09T00:00:00Z"
  source:
    canonical_ref: "main@24be54a2"
    deploy_identity: "unknown at draft; re-observe staging build/computer/guest/epoch/effects/fence before any red mutation."
  worktree_inventory:
    status: reconciled
    evidence_ref: "Draft-time clean tree; re-observe before charter and before any red mutation."
    preservation_rule: "Preserve every subsequently discovered dirty path, non-primary worktree, and unrelated WIP. This mission will own only named settlement surfaces at charter."
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: read_only
      paths_or_digest: "clean at draft receipt"
      recovery: leave_in_place
  candidates:
    - id: none
      ref: none
      base: none
      scope: []
      disposition: none
  observed_artifact:
    - claim: "The static import check returns without poisoning, yet cell serving returns the error and both session loops exit the worker on any evaluation error; the broker drops the session on any non-empty error string."
      evidence_ref: "internal/yaegikernel/session.go:90; internal/yaegikernel/session_loop.go:82,161; cmd/capsule-broker/session_worker.go:391,407"
    - claim: "The session finish path poisons on every error including parse and type failures; the session comment declares poisoning on any failed eval."
      evidence_ref: "internal/yaegikernel/session.go:136-137; internal/yaegikernel/session.go:19-23"
    - claim: "The one-shot fallback runs only on session spawn failure with the session error attached and the result marked Fallback; the one-shot path overwrites the exact diagnostic with a generic wait error."
      evidence_ref: "cmd/capsule-broker/session_worker.go:386-428; panel evidence for handleGoEvalOneShot diagnostic loss"
    - claim: "The assignment report identifier is derived over the provider tool call identifier and the terminal fingerprint over the report identifier; same-command changed-payload retries conflict."
      evidence_ref: "internal/agentcore/cosuper_assignment_fate.go:568-592,601-609"
    - claim: "The tool loop closes only on an exactly-one recognized terminal call while the batch executor names no evaluation or terminal tool in its sequential list, so evaluation plus terminal report run concurrently."
      evidence_ref: "Panel evidence: internal/toolregistry/toolloop.go:622-650; internal/toolregistry/batch_executor.go sequential list"
    - claim: "The researcher fallback binds delegated terminal outcomes into the parent under guards (root runs return, lifecycle-authoritative runs abstain, explicit identity matching, duplicate bindings rejected)."
      evidence_ref: "internal/agentcore/researcher_checkpoint_fallback.go:23-122"
    - claim: "The cutover's run acceptance is withheld for this remainder; mission 0 executes the restore predecessor in parallel with this draft."
      evidence_ref: "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md remainder disposition; docs/definitions/choir-rlm-restore-zero-2026-09-08.md"
  unknowns:
    - "Live staging/base identities at charter; fresh reconciliation required."
    - "Whether Yaegi exposes a genuinely non-executing compile/type boundary (isolation experiment outstanding)."
    - "Full sequential-tool and terminal-recognition inventory beyond panel-cited sites."
    - "Measured terminal-conflict and batch-shape distributions."

finish:
  deliver: "Terminal settlement is one durable truth per assignment attempt: provider metadata excluded from semantic identity, compile feedback preserving the heap, runtime failure poisoning to the same snapshot, a single execution path with exact diagnostics, a crash-resumable fate saga, batches that serialize or refuse, rejections that never settle, and a fallback that explains rather than commits. The cutover's withheld run acceptance closes under this mission."
  artifact: "A deployed staging settlement contract across the Yaegi session, broker, batch executor, tool loop, assignment fate store, researcher fallback, and capsule revoke paths, with the epoch-888-class sealed proof re-executed to clean freeze, terminal pass, and revocation."
  entrypoints:
    implementation:
      - "internal/yaegikernel/session.go and session_loop.go"
      - "cmd/capsule-broker/session_worker.go and one-shot worker"
      - "internal/toolregistry/batch_executor.go and toolloop.go"
      - "internal/agentcore/cosuper_assignment_fate.go and store/cosuper_assignments.go"
      - "internal/agentcore/researcher_checkpoint_fallback.go"
      - "capsule revoke/fate reconciliation paths"
  acceptance:
    - action: "Demonstrate a genuinely non-executing compile/type boundary (or record its absence): rejected cells return structured diagnostics, preserve prior heap/imports/definitions, drop only the failed tray, hold the cursor, and the following valid cell observes the exact prior heap. Runtime panic, timeout, overflow, worker death, transport corruption, or any doubt poisons, drops the tray, holds the cursor, and respawns over the same snapshot."
      proves: "Compile feedback never destroys working context; runtime failure never reuses unsafe state; string matching carries no classification."
      evidence_class: local_test
    - action: "Unify execution onto the persistent session with disposal as its single-cell case; delete the one-shot diversion so the exact diagnostic surfaces as runtime-class through one error contract."
      proves: "One execution path, one diagnostic contract, no silent fallback."
      evidence_class: local_test
    - action: "Scrub provider call identifiers, retry ordinals, batch positions, delivery receipts, and model metadata from report identity and fingerprint; replay the same semantic packet under fresh provider metadata and changed content."
      proves: "Same semantics return the original receipt; changed content conflicts deterministically; no retry mints or collides."
      evidence_class: local_test
    - action: "Exercise singleton, mixed consequential, double-terminal, and rejection batch shapes; an accepted terminal disposition of any shape closes the loop, a rejected packet settles nothing and explains itself."
      proves: "The singleton special case is gone; settlement is a reducer property, not a batch-shape accident."
      evidence_class: local_test
    - action: "Fault-inject before and during the reducer transition, after the fate request, and after physical effect before acknowledgement: zero partial durable effects before commit, one recovered fate command after, provider-free replay reproducing receipt and disposition, changed-payload conflict, and cancellation beating a racing pass."
      proves: "Fate is a resumable saga with deterministic identity, not an assumed atomic transaction."
      evidence_class: local_test
    - action: "Close the fallback write port: it explains structured rejections and repackages corrections through the sole reducer and can mint no update, report, wake, terminal state, freeze, or revoke."
      proves: "Terminal truth has one author; delegation closes deterministically without a second committer."
      evidence_class: local_test
    - action: "Re-execute the sealed staging proof class to clean freeze, terminal pass, and capsule revocation with effects OFF, then record freeze/pass/revocation receipts."
      proves: "The withheld cutover run acceptance closes under this mission on the physical staging computer."
      evidence_class: deployed_proof
    - action: "Run the affected existing suites with no regression."
      proves: "Settlement repair preserves adjacent behavior."
      evidence_class: local_test
  rollback: "Revert the source commits and redeploy the prior accepted source; immutable events, reports, and recovery evidence remain. A retained flawed settlement path is a completion blocker; no live legacy detector fallback is added. Product restore remains a forward event-chain transaction."
  landing:
    required: true
    environment: "staging https://choir.news"
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "Any provider or transport token enters semantic identity or digest input."
    - "Any compile rejection loses heap/imports, or any runtime failure reuses the interpreter."
    - "Two execution paths exist, or any diagnostic is replaced by a generic wait error."
    - "Two accepted terminal packets can coexist for one attempt, or a rejection advances state."
    - "A fate interruption can partially commit, duplicate, or lose the terminal outcome."
    - "The fallback can commit anything, or terminal truth has two authors."
    - "The withheld run acceptance is reported closed without the sealed re-execution receipts."
    - "Only panel agreement, timing, local tests, or a deployment SHA exists in place of deployed proof."

boundaries:
  mutation_class: red
  drafting_mutation_class: green
  authority_sources:
    - "Owner-settled mission order; docs/reports/choir-rlm-mission-state-2026-09-08.md"
    - "docs/choir-doctrine.md; docs/computer-ontology.md"
    - "AGENTS.md; docs/standing-questions.md"
    - "Remainder transfer: docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md"
  must_preserve:
    - "The reducer is the sole committer of terminal truth; the researcher fallback explains and repackages only."
    - "Compile preservation requires the isolation experiment; without it the failure stays poison."
    - "Problem-documentation-first: a code-free Define receipt naming the observed batch/digest/classification defects precedes every repair-code commit."
    - "Simplification over addition: each receipt names the surface extended, the flawed path deleted or made unreachable with citers, and every new module with evidence no surface carries its contract."
    - "Mission 1 may rehearse beside mission 0, but commits on shared files, deployment, route, and registry authority serialize."
    - "Pre-A checkpoint 99949fe2 remains untouched as the self-development fence."
  excluded:
    - "Mission-0 restore implementation (parallel predecessor)."
    - "Mission-2 desk rename and V1 inventory beyond the vocabulary seam already established."
    - "Engineering, Texture, Research, Management, prompt-bar, continuation deletion, shadow evaluations, provider/hill-climbing work."
    - "Conductor-agentic behavior; styleguide control; native goals machinery."
    - "Candidate-A authoring, promotion, World Wire, tape deletion."
  protected_surfaces:
    - "internal/yaegikernel/session.go and session_loop.go"
    - "cmd/capsule-broker/session_worker.go and one-shot worker"
    - "internal/toolregistry/batch_executor.go and toolloop.go"
    - "internal/agentcore/cosuper_assignment_fate.go and store/cosuper_assignments.go"
    - "internal/agentcore/researcher_checkpoint_fallback.go"
    - "capsule revoke/fate reconciliation, run acceptance, and staging deployment routing"
  completion_evidence_floor: [local_test, deployed_proof]
  conjecture_delta:
    discovered:
      - "One reducer-side terminal contract can retire the JSON batch race without a second settlement implementation."
      - "A typed pre-execution error class can preserve compile feedback without risking silent heap corruption."
    falsifiers:
      - "Yaegi cannot demonstrate a non-executing boundary; then compile preservation is refused and all failures stay poison."
      - "Fate effects cannot be fenced into intent/ack; then physical finality needs re-scoping before completion."
  heresy_delta:
    discovered:
      - "Same-turn eval+terminal batch escaping the singleton exit with provider-contaminated identity."
      - "Compile failures discarded working heaps through loop-level worker exit."
      - "One-shot diversion replacing exact diagnostics with generic wait errors."
    introduced: []
    repaired: "none; mark repaired only after terminal deployed receipts"

measures:
  - name: terminal_conflict_rate
    kind: gate
    baseline: "Withheld acceptance on one epoch-888 retry; distributions unmeasured."
    desired: "Zero same-semantic conflicts; deterministic changed-content conflicts; provider-fresh replays return original receipts."
    decision_use: "Blocks completion while identity contamination persists."
    cannot_prove: "Cannot prove fate crash-safety or diagnostic exactness."
  - name: compile_heap_preservation
    kind: gate
    baseline: "Compile failures exit the worker today."
    desired: "Rejected cells preserve heap/imports; valid successors observe exact prior state."
    decision_use: "Blocks the classification cutover without the isolation experiment."
    cannot_prove: "Cannot prove Yaegi boundary purity beyond the experiment."
  - name: batch_shape_coverage
    kind: telemetry
    baseline: "Singleton-only recognition."
    desired: "All consequential shapes serialize, refuse, or close deterministically."
    decision_use: "Exposes unhandled batch geometries."
    cannot_prove: "Cannot prove reducer atomicity or fate resumability."
  - name: panel_agreement
    kind: weak_signal
    baseline: "Draft consensus pending."
    desired: "No additional agreement threshold."
    decision_use: "Explains draft review scope only."
    cannot_prove: "Cannot authorize promotion, prove code, or advance completion."

now:
  status: blocked_incomplete
  slice: "full draft under consensus review; executable only after mission 0 completes and owner ratifies charter"
  question: none
  reconciliation:
    observed_at: "2026-09-09T00:00:00Z"
    source_ref: "main@24be54a2"
    deploy_identity: "unreconciled; re-observe before charter"
    authority_identities:
      - "docs/reports/choir-rlm-mission-state-2026-09-08.md"
      - "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md"
      - "AGENTS.md; docs/standing-questions.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "draft-time clean; re-observe before charter"
    status: reconciling
  candidate:
    id: none
    state: none
    ref: none
    owner: none
    base: none
    digest: none
    scope: []
  decision:
    selected: "Mission 1 charters the settlement gate as drafted; executes only after mission 0 completes; closes the withheld cutover run acceptance."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: "Owner-settled mission order"
    owner_ratification_ref: "charter ratification still required before execution"
    recorded_at: "2026-09-09T00:00:00Z"
    consequence: "Draft and review only. No settlement repair executes under this file before charter ratification and mission 0 completion."
  evidence_refs:
    - "internal/yaegikernel/session.go"
    - "internal/yaegikernel/session_loop.go"
    - "cmd/capsule-broker/session_worker.go"
    - "internal/agentcore/cosuper_assignment_fate.go"
    - "internal/agentcore/researcher_checkpoint_fallback.go"
  blocker_or_risk: "Mission 0 incomplete; Yaegi isolation experiment unproven; sequential/terminal inventories partially panel-sourced and need mechanical verification at charter."
  next_action: "Iterate consensus on this draft until verdicts accept; then await mission 0 completion and owner charter ratification."

receipts: []
