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
    - claim: "Errors reaching the session finish path poison, and the session comment declares poisoning on any failed eval; the static import check returns before finish and does not itself poison, although loop and broker behavior still drop the session on the resulting error."
      evidence_ref: "internal/yaegikernel/session.go:136-137; internal/yaegikernel/session.go:19-23"
    - claim: "The one-shot fallback runs only on session spawn failure with the session error attached and the result marked Fallback; the one-shot worker overwrites the result Error with the process wait error, so the exact child diagnostic is not decoded into the result contract."
      evidence_ref: "cmd/capsule-broker/session_worker.go:386-428; cmd/capsule-broker/main.go:141-145,787-802"
    - claim: "The assignment report identifier is derived over the provider tool call identifier and the terminal fingerprint over the report identifier; same-command changed-payload retries conflict."
      evidence_ref: "internal/agentcore/cosuper_assignment_fate.go:568-592,601-609"
    - claim: "The tool loop closes only when the batch holds exactly one recognized terminal call while the sequential-execution list names no evaluation or terminal tool, so evaluation plus terminal report run concurrently."
      evidence_ref: "internal/toolregistry/toolloop.go:622; internal/toolregistry/batch_executor.go:114-129"
    - claim: "The researcher fallback binds delegated terminal outcomes into the parent under guards (root runs return, lifecycle-authoritative runs abstain, explicit identity matching, duplicate bindings rejected) and synthesizes reference updates with channel emission and wake on the bound path."
      evidence_ref: "internal/agentcore/researcher_checkpoint_fallback.go:23-122,134-167"
    - claim: "The cutover's run acceptance is withheld for this remainder; mission 0 executes the restore predecessor in parallel with this draft."
      evidence_ref: "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md remainder disposition; docs/definitions/choir-rlm-restore-zero-2026-09-08.md"
  unknowns:
    - "Live staging/base identities at charter; fresh reconciliation required."
    - "Whether Yaegi exposes a genuinely non-executing compile/type boundary (isolation experiment outstanding)."
    - "Full sequential-tool and terminal-recognition inventory beyond panel-cited sites."
    - "Measured terminal-conflict and batch-shape distributions."
    - "Correction-landing mechanism choice (new attempt vs structured supersession event vs append-only presentation amendment) and field-inventory audit proving every authoritative claim has a structured representation."

finish:
  deliver: "Terminal settlement is one durable truth per assignment attempt: provider metadata excluded from semantic identity, compile feedback preserving the heap where the isolation experiment proves a non-executing boundary (else documented all-poison), runtime failure poisoning to the same snapshot, a single execution path with exact diagnostics, a crash-resumable fate saga, batches that refuse multi-terminal and order the admitted shapes, rejections that never settle, reducer-owned orphan close, and a fallback that explains rather than commits. The cutover's withheld run acceptance closes under this mission."
  artifact: "A deployed staging settlement contract across the Yaegi session, broker, batch executor, tool loop, assignment fate store, researcher fallback, and capsule revoke paths, with the epoch-888-class sealed proof re-executed to clean freeze, terminal pass, and revocation."
  entrypoints:
    implementation:
      - "internal/yaegikernel/session.go and session_loop.go (evaluator, cell serving, both loops, sidecar boundary)"
      - "cmd/capsule-broker/session_worker.go and one-shot worker"
      - "internal/toolregistry/batch_executor.go and toolloop.go"
      - "internal/agentcore/cosuper_assignment_fate.go and store/cosuper_assignments.go"
      - "internal/agentcore/researcher_checkpoint_fallback.go"
      - "capsule revoke/fate reconciliation paths"
  acceptance:
    - action: "Run the isolation matrix against the exact Yaegi/session construction: rejected parse/import/type/declaration cells must prove no user execution and no observable mutation of heap, imports, definitions, or interpreter bookkeeping, with a valid successor observing exactly the prior heap. Only the proven class preserves the interpreter through evaluator, cell serving, both loops, sidecar, and broker; static import/parser rejection is separately non-executing and must survive the loop/broker. Every unproven Eval-time error, runtime error, panic, timeout, overflow, worker death, transport failure, or doubt poisons and respawns from the durable snapshot; string matching is forbidden. If the experiment fails, all failures remain typed runtime-poison by documented revision, which is a valid safe endpoint (owner-settled)."
      proves: "Compile feedback never destroys working context; runtime failure never reuses unsafe state; string matching carries no classification."
      evidence_class: local_test
    - action: "Unify execution onto the persistent session with disposal as its single-cell case; delete the one-shot diversion so the exact diagnostic surfaces as runtime-class through one error contract."
      proves: "One execution path, one diagnostic contract, no silent fallback."
      evidence_class: local_test
    - action: "Define a stable terminal-slot key over immutable assignment scope only; a canonical length-framed digest of the submitted typed terminal proposition carrying result/verdict/payload; and the first committed reducer receipt. Exclude provider call identifiers, retry ordinals, batch positions, delivery receipts, model/transport metadata, summary, and prose from all three. Inventory and classify every authoritative request/report/fate/outbox/receipt field. Cancellation-wins and late-fate outcomes are reducer dispositions recorded on the receipt, never state-dependent rewrites of the submitted packet before identity. Stored pass with a later cancellation intent and identical retry returns original receipt and replay semantics with the reducer disposition; never a fresh conflict. Corrections land as new attempts with explicit superseding relation (owner-settled); legacy provider-keyed stored reports conflict or are explicitly reconciled, never re-minted. Owner-panel settled 11-0: presentation-only."
      proves: "Retries and crash recovery succeed on paraphrase; corrections require structured deltas; no retry mints or collides."
      evidence_class: local_test
    - action: "Freeze the typed terminal/consequential recognition inventory before execution and fail closed on unknown terminal-like calls. Preflight-refuse every batch with more than one terminal before any member runs. Admit only explicitly ordered nonterminal-before-terminal shapes; terminal-before-eval and unknown shapes fail closed (owner-settled). Accepted or replayed-accepted terminal disposition immediately stops the batch and loop with remaining members recorded unexecuted; a structured rejection settles nothing and enables no later terminal in that turn; runtime-poison evaluation aborts later members. Tool transport/JSON success is not terminal acceptance. Exercise terminal-first/middle/last, terminal+eval, eval+terminal, first-reject/two-terminal, duplicate/replay, and poison-before-terminal. Owner-settled: simple refusal now; full RLM dissolves the batch race structurally."
      proves: "The singleton special case is gone; contradictory turns never execute; settlement is a reducer property, not a batch-shape accident."
      evidence_class: local_test
    - action: "Atomically persist the validated proposal plus freeze/revoke intent before physical action; intent is legitimate durable pending state carrying the stable idempotency/fencing key, and the actuator is idempotent or queryable by fate identity so recovery reissues to one logical effect and one accepted typed acknowledgement. No parent wake, accepted settlement, or externally final frozen/revoked claim precedes the required store acknowledgement. Fault-inject before durable intent, after intent/before effect, after effect/before ack, after ack/projection, and during cancellation racing pass, including stale freeze-vs-revoke fencing. Same slot plus identical packet returns the original receipt across restart/retry storms; changed semantic content conflicts before new report, fate, outbox, or wake effects."
      proves: "Fate is a resumable saga with deterministic identity, not an assumed atomic transaction."
      evidence_class: local_test
    - action: "Close the fallback canonical-author port: for a delegated child with no accepted terminal packet (dead, timeout, cancellation, worker death, restart-discovered orphan), a lifecycle/reconciliation producer submits one authenticated structured orphan outcome and the reducer commits exactly one parent-visible disposition/update/wake under the existing slot; replays, duplicates, and late child reports return the original outcome with no duplicate wake. The fallback only explains structured rejections or submits non-authoritative correction proposals; it performs no canonical dispatch, terminal bind, wake-policy, report, fate, freeze, or revoke write. Exercise explicit child, terminal-without-packet, dead, timeout, cancel, restart, duplicate, and late-packet cases."
      proves: "Terminal truth has one author; delegation closes deterministically without a second committer."
      evidence_class: local_test
    - action: "After mission-0 deployed acceptance, consume its accepted restore/computer identity read-only. On the freshly reconciled staging computer/realization/build/route/fence with effects OFF, execute the original same-turn eval+terminal class in its repaired serialization/refusal form, a same-semantic retry with fresh provider metadata returning the original receipt, a changed-semantic-payload conflict before effects, and cancel racing pass with no second accepted packet/wake/fate effect; retrieve execution, terminal, intent, ack, parent/outbox, freeze/revoke, environment, deploy, and verifier receipts bound to one attempt/computer/capsule/deployment identity. Record the remainder-paid receipt and update the cutover registry disposition atomically. Historical epoch-888 receipts are provenance only, never a requirement to reuse the old epoch or SHA."
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
    - "internal/agentcore/cosuper_assignment_fate.go and store/cosuper_assignments.go (report/fingerprint identity, CAS, correction-as-new-attempt)"
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
  slice: "repaired draft under verification round 2; mission 0 complete; charter ratification pending"
  question: "Terminal/consequential recognition inventory and correction-as-new-attempt superseding relation still to be mechanically closed at charter."
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
    status: proposal
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
  blocker_or_risk: "Mission 0 complete; charter ratification pending. Yaegi isolation experiment unproven; sequential/terminal inventories mechanically anchored at cited lines with full-inventory closure at charter; correction-as-new-attempt relation chosen, superseding mechanics open."
  next_action: "Verification consensus round on repaired draft; then owner charter ratification with fresh reconciliation."

receipts: []
