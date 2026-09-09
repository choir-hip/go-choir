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
    - claim: "The one-shot fallback runs only on session spawn failure with the session error attached and the result marked Fallback; the child encodes SidecarResponse.Error to stdout which the parent never decodes, so the parent records the generic process wait error in GoEvalResult.Error and the exact child diagnostic never enters the result contract."
      evidence_ref: "cmd/capsule-broker/session_worker.go:386-428; internal/yaegikernel/sidecar.go:140-163; cmd/capsule-broker/main.go:141-145,787-802"
    - claim: "Fresh provider-call minting starts a new report/intent while the same provider identifier with changed content conflicts; the terminal fingerprint derives over the report identifier, and summary text currently feeds the fingerprint. Mechanically verified at the reviewed identity."
      evidence_ref: "internal/agentcore/cosuper_assignment_fate.go:568-592,601-609"
    - claim: "The tool loop closes only when the batch holds exactly one recognized terminal call; the sequential-execution inventory omits the capsule_go_eval and record_assignment_result pair at its dispatch anchor, so evaluation plus terminal report dispatch concurrently. Mechanically verified at the reviewed identity."
      evidence_ref: "internal/toolregistry/toolloop.go:622; internal/toolregistry/batch_executor.go:114-129"
    - claim: "The researcher fallback binds delegated terminal outcomes into the parent under guards (root runs return, lifecycle-authoritative runs abstain, explicit identity matching, duplicate bindings rejected) and synthesizes reference updates with channel emission and wake on the bound path."
      evidence_ref: "internal/agentcore/researcher_checkpoint_fallback.go:23-122,134-167"
    - claim: "The cutover's run acceptance was withheld for this remainder at draft time; mission 0 has since completed (2026-09-09) and this claim is dated/superseded, retained as document provenance rather than fresh staging proof."
      evidence_ref: "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md remainder disposition; docs/definitions/choir-rlm-restore-zero-2026-09-08.md"
  unknowns:
    - "Live staging/base identities at charter; fresh reconciliation required."
    - "Whether Yaegi exposes a genuinely non-executing compile/type boundary (isolation experiment outstanding)."
    - "Full sequential-tool and terminal-recognition inventory beyond panel-cited sites."
    - "Measured terminal-conflict and batch-shape distributions."
    - "Correction-as-new-attempt is owner-settled (2026-09-09); the charter freezes the mandatory superseding-relation fields. Residue R4."

finish:
  deliver: "Terminal settlement is one durable truth per assignment attempt: provider metadata excluded from semantic identity under the full canonicalization contract, compile feedback preserving the heap only for proven non-mutating classes (host preflight today; all else typed runtime-poison), runtime failure poisoning to the same snapshot, a single execution path with exact diagnostics, a fenced crash-resumable fate saga with no terminal finality visible until revoke acknowledgement, batches that refuse multi-terminal and serialize admitted shapes, rejections that never settle, reducer-owned orphan close over the named slot family, and a fallback that explains rather than commits. The cutover's withheld run acceptance closes under this mission through a demonstrated successful settlement path."
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
    - action: "Run the isolation matrix against the exact Yaegi/session construction: only host CheckImports/imports-only rejection proved to occur before interpreter entry may preserve the live interpreter, and the loop/broker must carry that typed class without worker disposal. Body syntax, type, declaration, and every other EvalWithContext error remain poison unless the matrix proves an exact non-mutating class; a valid successor observes exactly the prior heap only for a proven class. Every runtime error, panic, timeout, overflow, worker death, transport failure, or doubt poisons and respawns from the durable snapshot; string matching is forbidden. If no further class passes, the host-preflight class alone preserves; all other failures remain typed runtime-poison."
      proves: "Only proven non-mutating rejection classes preserve working context; runtime failure never reuses unsafe state; string matching carries no classification."
      evidence_class: local_test
    - action: "Unify execution onto the persistent session with disposal as its single-cell case; delete the one-shot diversion so the exact diagnostic surfaces as runtime-class through one error contract."
      proves: "One execution path, one diagnostic contract, no silent fallback."
      evidence_class: local_test
    - action: "Define the terminal identity contract: slot key, report/command identity, proposition digest, CAS key, fate key, outbox key, and receipt all derive from immutable slot plus canonical typed proposition only; classify every field and every collection as ordered semantic sequence or key-sorted semantic set. Exclude provider call identifiers, retry ordinals, batch positions, delivery receipts, model/transport metadata, summary, and all prose from every key. Partial reports take their own nonterminal identity/sequence or are excluded from the terminal surface. Cancellation-wins and late-fate outcomes are reducer dispositions recorded on the receipt, never state-dependent rewrites of the submitted packet before identity. Fresh provider-call minting starts a new report/intent; same provider identifier with changed content conflicts; identical semantic packets under fresh provider metadata return the original receipt with its first-committed presentation. Corrections land as new attempts (owner-settled); the charter freezes the mandatory superseding-relation fields."
      proves: "Retries and crash recovery succeed on paraphrase; corrections require structured deltas; no retry mints or collides."
      evidence_class: local_test
    - action: "Freeze a profile-scoped terminal/consequential recognition inventory before execution for assigned-CoSuper settlement; count every record_assignment_result/terminal-like call regardless of result kind, and an evaluation cell staging terminal intent counts for preflight. Preflight-refuse every batch with two or more terminal/consequential calls before any member runs (owner-settled). Admit only (a) all-nonterminal known batches, (b) one recognized terminal alone, and (c) known nonterminals before one terminal; terminal-first/middle and every unknown shape fail closed. Any admitted batch containing evaluation or consequential calls executes strictly sequentially in declared order; ExecuteToolBatch parallel dispatch is forbidden for these shapes. Any non-success evaluation or nonterminal rejection aborts all later members as unexecuted; an accepted or replayed-accepted terminal disposition immediately stops batch and loop with remaining members recorded unexecuted; a terminal rejection settles nothing and permits no later terminal in that turn. Tool transport/JSON success is not terminal acceptance. This scope preserves unrelated Texture handling and the explicit actuator=tools rollback path unless separately authorized. Exercise terminal-first/middle/last, terminal+eval, eval+terminal, first-reject/two-terminal, duplicate/replay, and in-cell-terminal shapes."
      proves: "The singleton special case is gone; contradictory turns never execute; admitted shapes serialize deterministically; settlement is a reducer property, not a batch-shape accident."
      evidence_class: local_test
    - action: "Atomically persist the validated proposal plus freeze/revoke intent before physical action; intent is legitimate durable pending state carrying the stable idempotency/fencing key. The actuator durably deduplicates by fate identity and current fence, or exposes authoritative operation resolution proving the original cannot still execute before any reissue; query-then-reissue against a possibly live operation is forbidden. No parent wake, accepted settlement, or externally final frozen/revoked claim precedes the required store acknowledgement, and no terminal finality is visible until the revoke acknowledgement lands (owner-settled 2026-09-09). Fault-inject before durable intent, after intent/before effect, after effect/before ack, after ack/projection, and during cancellation racing pass, including stale freeze-vs-revoke fencing. Same slot plus identical packet returns the original receipt across restart/retry storms; changed semantic content conflicts before new report, fate, outbox, or wake effects."
      proves: "Fate is a resumable saga with deterministic identity, not an assumed atomic transaction; revocation races resolve to one visible finality."
      evidence_class: local_test
    - action: "Close the fallback canonical-author port: for a delegated child with no accepted terminal packet (dead, timeout, cancellation, worker death, restart-discovered orphan), a lifecycle/reconciliation producer submits one authenticated immutable orphan observation only (scope, child/run identity, liveness/terminal observation, reason enum, timestamps, evidence) and the reducer alone validates it, deriving disposition, terminal proposition, parent update, receipt, outbox, and wake. The slot family is explicit: the assignment-attempt terminal slot for assignment children, a delegated-run slot where no assignment exists. Identical orphan retries replay the original outcome with no duplicate wake; a semantically different late child report returns a structured late/conflict rejection referencing the original receipt, never accepted-replay semantics. Admission atomically rechecks lifecycle state and the absence of any accepted or pending terminal proposal. The fallback only explains structured rejections or submits non-authoritative correction proposals; it performs no canonical dispatch, terminal bind, wake-policy, report, fate, freeze, or revoke write. Exercise explicit child, terminal-without-packet, dead, timeout, cancel, restart, duplicate, and late-different-packet cases."
      proves: "Terminal truth has one author; delegation closes deterministically without a second committer; late difference can never masquerade as replay."
      evidence_class: local_test
    - action: "Consume mission-0's accepted restore/computer identity read-only; mission-0's blocked live-drill debt stays mission-0-owned and is neither implemented nor waived here (owner-settled 2026-09-09; docs/mission-residues.md R1). Using scoped owner CLI/product APIs without SSH on the freshly reconciled staging computer/realization/build/route/fence with effects OFF, demonstrate both the repaired refused/serialized original same-turn shape and, on a separate admitted attempt, successful evaluation, freeze, accepted terminal pass, and revocation with no terminal finality visible until the revoke acknowledgement. Run the cancellation race on another identified attempt with no second accepted packet/wake/fate effect; bind execution, intent/ack, parent/outbox, freeze/revoke, environment, deploy, and verifier receipts to one attempt/computer/capsule/deployment identity. First write mission-1's terminal receipt and the target-cutover Definition's remainder-paid receipt and now disposition; then update ACTIVE, mission graph, and authority manifest as discovery projections in the same document transaction. Historical epoch-888 receipts are provenance only, never a requirement to reuse the old epoch or SHA."
      proves: "The withheld cutover run acceptance closes under this mission on the physical staging computer through a demonstrated successful settlement path, not refusal alone."
      evidence_class: deployed_proof
    - action: "Run the focused contracts with no regression: session persistence/poisoning, broker containment, batch admission/order, identity replay/conflict, fate intent/ack recovery, and fallback non-authority."
      proves: "Settlement repair preserves adjacent behavior on the contracts it touches."
      evidence_class: local_test
  rollback: "Revert the source commits and redeploy the prior accepted source; immutable events, reports, and recovery evidence remain. A retained flawed settlement path is a completion blocker; no live legacy detector fallback is added. Product restore remains a forward event-chain transaction."
  landing:
    required: true
    environment: "staging https://choir.news"
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
  not_done_when:
    - "Any provider or transport token enters semantic identity or digest input."
    - "Any compile rejection outside a proven non-mutating class loses heap/imports, or any runtime failure reuses the interpreter."
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
    desired: "Only proven non-mutating rejection classes preserve heap/imports; valid successors observe exact prior state."
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
  slice: "round-3 repaired draft under focused verification; mission 0 complete; charter ratification pending"
  question: "Focused verification verdicts on the round-3 repairs; then charter ratification with fresh reconciliation."
  reconciliation:
    observed_at: "2026-09-09T00:00:00Z"
    source_ref: "main@aaa55d0f"
    deploy_identity: "unreconciled; mission-0 accepted identity consumed read-only at charter; re-observe before any red mutation"
    authority_identities:
      - "docs/reports/choir-rlm-mission-state-2026-09-08.md"
      - "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md"
      - "AGENTS.md; docs/standing-questions.md"
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "draft-observed clean tree superseded; current untracked leftovers (mission-0 completion report, scripts, tmp/) preserved as unrelated WIP; re-observe before charter"
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
    selected: "Mission 1 charters the settlement gate as repaired; executes only after owner charter ratification with fresh reconciliation; closes the withheld cutover run acceptance through a demonstrated successful settlement path."
    kind: architecture
    status: proposal
    source: owner
    evidence_ref: "Owner-settled mission order"
    owner_ratification_ref: "charter ratification still required before execution"
    recorded_at: "2026-09-09T00:00:00Z"
    consequence: "Draft and review only. No settlement repair executes under this file before charter ratification with fresh reconciliation. Mission-0 drill debt stays mission-0-owned (residue R1)."
  evidence_refs:
    - "internal/yaegikernel/session.go"
    - "internal/yaegikernel/session_loop.go"
    - "cmd/capsule-broker/session_worker.go"
    - "internal/agentcore/cosuper_assignment_fate.go"
    - "internal/agentcore/researcher_checkpoint_fallback.go"
  blocker_or_risk: "Charter ratification pending. Yaegi isolation experiment unproven (host-preflight class only until the matrix proves more); full terminal inventory freezes at charter (residue R5); correction superseding fields freeze at charter (residue R4)."
  next_action: "Focused verification consensus on round-3 repairs; then owner charter ratification with fresh reconciliation."

receipts: []
---
