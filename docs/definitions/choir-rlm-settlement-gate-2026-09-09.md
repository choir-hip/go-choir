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
    - claim: "The static import check returns without poisoning, yet cell serving returns the error and both session loops exit the worker on any evaluation error; the broker drops the session on any non-empty error string. Mechanically verified at main@d3e253b9."
      evidence_ref: "internal/yaegikernel/session.go:90; internal/yaegikernel/session_loop.go:82,161; cmd/capsule-broker/session_worker.go:391,407"
    - claim: "Errors reaching the session finish path poison, and the session comment declares poisoning on any failed eval; the static import check returns before finish and does not itself poison, although loop and broker behavior still drop the session on the resulting error. Mechanically verified at main@d3e253b9."
      evidence_ref: "internal/yaegikernel/session.go:136-137; internal/yaegikernel/session.go:19-23"
    - claim: "The RLM session-spawn fallback is the path to remove: on session spawn failure the broker runs the cell on the one-shot worker with the session error attached and the result marked Fallback. The child diagnostic remains in raw encoded SidecarResponse bytes copied to GoEvalResult.Stdout but is never decoded into typed Error/Stderr, whose error is the process wait error (verified against dissent by code trace 2026-09-09). The one-shot worker itself remains as the explicit actuator=tools route unless separately retired. Mechanically verified at main@d3e253b9."
      evidence_ref: "cmd/capsule-broker/session_worker.go:386-432; internal/yaegikernel/sidecar.go:140-163; cmd/capsule-broker/main.go:689-804"
    - claim: "Fresh provider-call minting starts a new report/intent while the same provider identifier with changed content conflicts; the terminal fingerprint derives over the report identifier, and summary text currently feeds the fingerprint. This documents the contaminated present behavior the identity repair removes. Mechanically verified at main@d3e253b9."
      evidence_ref: "internal/agentcore/cosuper_assignment_fate.go:568-592,601-609"
    - claim: "The loop's detached-terminal singleton predicate closes only single recognized-terminal turns; the sequential-execution inventory omits the capsule_go_eval and record_assignment_result pair at its dispatch anchor, so evaluation plus terminal report fall through to WaitGroup parallel dispatch. Mechanically verified at main@d3e253b9; full inventory freezes at charter (residue R5)."
      evidence_ref: "internal/toolregistry/toolloop.go:622; internal/toolregistry/batch_executor.go:114-129 (inventory omission); internal/toolregistry/batch_executor.go:39-47 (parallel dispatch)"
    - claim: "The researcher fallback binds delegated terminal outcomes into the parent under guards (root runs return, lifecycle-authoritative runs abstain, explicit identity matching, duplicate bindings rejected) and synthesizes reference updates with channel emission and wake on the bound path. Mechanically verified at main@d3e253b9."
      evidence_ref: "internal/agentcore/researcher_checkpoint_fallback.go:23-122,134-167"
    - claim: "The cutover's run acceptance is withheld for this remainder; mission 0 executes the restore predecessor in parallel with this draft. (Start-time observation; mission-0 completion recorded as dated start_correction below, not edited into this receipt.)"
      evidence_ref: "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md remainder disposition; docs/definitions/choir-rlm-restore-zero-2026-09-08.md"
  start_correction:
    - date: "2026-09-09"
      note: "Mechanical review at main@d3e253b9: claims 1-6 are source-trace observations anchored at cited lines; claim 7 is documentary (cutover remainder disposition). Mission 0 completed with deployed proof; its blocked live-drill debt stays mission-0-owned (residue R1). All claims require charter-head re-pinning. Original start claims above unchanged."
  unknowns:
    - "Live staging/base identities at charter; fresh reconciliation required."
    - "Whether Yaegi exposes a genuinely non-executing compile/type boundary (isolation experiment outstanding)."
    - "Full sequential-tool and terminal-recognition inventory beyond panel-cited sites."
    - "Measured terminal-conflict and batch-shape distributions."
  deliver: "Terminal settlement is one durable truth per assignment attempt: provider metadata excluded from semantic identity under the full canonicalization contract, diagnostic feedback preserving the heap only for proven non-mutating pre-execution classes (host import preflight today) with every interpreter-entered or unclassified failure typed unsafe-to-reuse while retaining its original diagnostic kind, a single execution path with exact diagnostics, a fenced crash-resumable fate saga with no terminal finality visible until revoke acknowledgement, batches that refuse multi-terminal and serialize admitted shapes, rejections that never settle, reducer-owned orphan close over the named slot family, and a fallback that explains rather than commits. The cutover's withheld run acceptance closes under this mission through a demonstrated successful settlement path."

finish:
  deliver: "Terminal settlement is one durable truth per assignment attempt: provider metadata excluded from semantic identity under the full canonicalization contract, diagnostic feedback preserving the heap only for proven non-mutating pre-execution classes (host import preflight today) with every interpreter-entered or unclassified failure typed unsafe-to-reuse while retaining its original diagnostic kind, a single execution path with exact diagnostics, a fenced crash-resumable fate saga with no terminal finality visible until revoke acknowledgement, batches that refuse multi-terminal and serialize admitted shapes, rejections that never settle, reducer-owned orphan close over the named slot family, and a fallback that explains rather than commits. The cutover's withheld run acceptance closes under this mission through a demonstrated successful settlement path."
  artifact: "A deployed staging settlement contract across the Yaegi session, broker, batch executor, tool loop, assignment fate store, researcher fallback, and capsule revoke paths, with the epoch-888-class sealed proof re-executed to clean freeze, terminal pass, and revocation."
  entrypoints:
    implementation:
      - "internal/yaegikernel/session.go and session_loop.go (evaluator, cell serving, both loops, sidecar boundary)"
      - "cmd/capsule-broker/session_worker.go and one-shot worker"
      - "internal/toolregistry/batch_executor.go and toolloop.go"
      - "internal/agentcore/cosuper_assignment_fate.go and internal/store/cosuper_assignments.go (report/fingerprint identity, CAS, correction-as-new-attempt)"
      - "internal/agentcore/researcher_checkpoint_fallback.go"
      - "capsule revoke/fate reconciliation paths"
      - "internal/agentcore/rlm_reduce.go, internal/agentcore/tools_capsule.go, and the runtime/store transaction seam carrying the atomic promotion"
  acceptance:
    - action: "Run the isolation matrix against the exact Yaegi/session construction: only host CheckImports/imports-only rejection proved to occur before interpreter entry may preserve the live interpreter, and the loop/broker must carry that typed class without worker disposal. Body syntax, type, declaration, and every other EvalWithContext error remain unsafe-to-reuse unless the matrix proves an exact non-mutating class, with the original syntax, type, runtime, timeout, panic, overflow, worker, or transport diagnostic kind preserved separately; a valid successor observes exactly the prior heap only for a proven class. Every runtime error, panic, timeout, overflow, worker death, transport failure, or doubt poisons and respawns from the durable snapshot; string matching is forbidden. If no further class passes, the host-preflight class alone preserves the interpreter; every other failure remains typed unsafe-to-reuse."
      proves: "Only proven non-mutating rejection classes preserve working context; reuse disposition is typed separately from diagnostic kind; string matching carries no classification."
      evidence_class: local_test
    - action: "Define the terminal identity contract: the terminal uniqueness/CAS slot is keyed solely by the immutable assignment-attempt identity (or delegated-run identity where applicable); the charter freezes the slot tuple as owner, canonical computer, assignment, and attempt. The versioned canonical typed proposition and digest are immutable values reserved in that slot: a same-slot/same-digest submission replays the original pending/final receipt, while a same-slot/different-digest submission conflicts before report, fate, outbox, wake, or physical effect. Report/command, fate, outbox, and receipt identities are domain-separated derivations from the accepted slot and canonical proposition as applicable; they never create an independent terminal slot. Freeze scalar normalization, absent/null handling, map ordering, ordered sequences, and key-sorted sets; exclude provider identifiers, retry/batch/transport metadata, summaries, and prose from every key. Partial reports use an explicitly separate nonterminal sequence and never occupy the terminal slot. Cancellation-wins and late-fate outcomes are reducer dispositions recorded on the receipt, never state-dependent rewrites of the submitted packet before identity. Corrections land as new attempts (owner-settled) carrying the frozen superseding tuple (supersedes_assignment_id, supersedes_attempt, prior_receipt_ref, supersede_kind in {correction, retry_after_block, owner_reopen}, reason_enum, delta_digest over structured fields only, no summary or prose). Rejection classes: (a) pre-reserve admission reject (schema/auth/nonterminal-misuse) occupies no terminal slot, so the same attempt may submit a well-formed packet; (b) reserved same-digest replays the original pending/rejected/final receipt and never becomes a second accept; (c) reserved different-digest conflicts, and correction proceeds only as a new attempt with the tuple."
      proves: "An identical canonical typed proposition replays the original receipt despite changed provider-call, retry, batch, transport, model, summary, or other explicitly excluded metadata; any difference in included canonical proposition content conflicts and cannot reserve a second terminal row; rejections never brick an attempt nor smuggle a correction."
      evidence_class: local_test
    - action: "Freeze a profile-scoped terminal/consequential recognition inventory before execution for assigned-CoSuper settlement. Preflight-refuse every batch with two or more explicit terminal calls before any member runs (owner-settled); independent reducer uniqueness holds regardless. capsule_go_eval classifies as nonterminal at preflight (owner-settled 2026-09-09: admit eval, inspect tray after): admitted eval-then-terminal shapes execute strictly sequentially in declared order with ExecuteToolBatch parallel dispatch forbidden; after evaluation, inspect the complete returned tray before applying any staged effect. Only assignment-fate-class intents are terminal for this admission rule: mailbox choir.Complete/IntentComplete remains non-authoritative for assignment fate, occupies no terminal slot, and does not abort an admitted JSON record_assignment_result. Frozen assigned-CoSuper admission (R5): preflight-terminal is record_assignment_result with result other than partial; preflight-nonterminal is capsule_go_eval, mailbox choir.Complete/IntentComplete, and partial reports. Two or more preflight terminals refuse with none run. The admitted shape is at most one capsule_go_eval then at most one assignment-fate JSON terminal, sequential, tray inspected after eval. After an accepted or replayed-accepted assignment-fate disposition, abort every later member as unexecuted and stop batch and loop. After a rejected staged-terminal disposition, abort all remaining members as unexecuted, stop batch and loop immediately with the structured rejection, settle nothing, and permit no later terminal. An errored terminal call with no disposition (transport failure on the terminal call itself) aborts all later members as unexecuted, stops batch and loop, and settles nothing; the pending fate proposal holds the slot per the saga rule. Terminal-first/middle, eval+eval, extra members, and every other unnamed shape fail closed. Exercise two-terminal preflight refusal, eval-then-terminal clean, tray-terminal aborting later members, tray-terminal-reject then JSON terminal, JSON terminal rejection, errored terminal, duplicate/replay, and the Complete-plus-JSON successful path."
      proves: "The singleton special case is gone; contradictory turns never execute; no staged tray smuggles a second terminal past preflight; settlement is a reducer property, not a batch-shape accident."
      evidence_class: local_test
    - action: "Durably commit the reducer-owned pending proposal (immutable slot/digest, staged effects, operation identities, fence) before issuing any physical action. The present implementation order is the defect this saga replaces: the terminal report commits at frozen-capsule acknowledgement and revocation follows (internal/agentcore/cosuper_assignment_fate.go:733-748), so a crash in that window leaves assignment-final truth beside a live capsule. Freeze and revoke are sequential fenced operations of the pending proposal instead; frozen is observable capsule state but never terminal assignment finality, delivery, or wake. After the final required physical acknowledgement (revoke on the demonstrated pass path), one reducer transaction promotes disposition/receipt, staged durable effects, parent update/outbox/wake, and inbox-cursor advancement; before that ack, no terminal finality is visible (owner-settled 2026-09-09). A dead or orphaned child first resumes reconciliation; timeout alone never releases, replaces, or expires the reservation, and expiry requires authoritative evidence that every issued operation completed or can no longer execute under its fence, or acknowledged compensation — otherwise a visible blocked pending state persists with neither terminal finality nor orphan disposition published. The actuator durably deduplicates by fate identity and current fence, or exposes authoritative operation resolution proving the original cannot still execute before any reissue; query-then-reissue against a possibly live operation is forbidden. Fault-inject every intent/effect/ack/finalization boundary, including stale freeze-vs-revoke fencing and cancellation racing pass. Same slot plus identical packet returns the original receipt across restart/retry storms; changed semantic content conflicts before new report, fate, outbox, or wake effects. This is a fenced intent/ack saga, not a capsule distributed transaction."
      proves: "Fate is a resumable saga with one atomic final boundary; revocation races and dead children resolve to one visible finality."
      evidence_class: local_test
    - action: "Close the fallback canonical-author port: for a delegated child with no accepted terminal packet (child death, worker death, cancellation, restart-discovered orphan, or liveness-timeout observation of a dead child), a lifecycle/reconciliation producer submits one authenticated immutable orphan observation only (scope, child/run identity, liveness/terminal observation, reason enum, timestamps, evidence) and the reducer alone validates it, deriving disposition, terminal proposition, parent update, receipt, outbox, and wake. A fate-operation timeout never expires a pending freeze/revoke reservation and never routes through this orphan path; this path opens only on authoritative child-death or dead-child observation. The slot family is explicit: the assignment-attempt terminal slot for assignment children, a delegated-run slot where no assignment exists. Identical orphan retries replay the original outcome with no duplicate wake; a semantically different late child report returns a structured late/conflict rejection referencing the original receipt, never accepted-replay semantics. Admission atomically rechecks lifecycle state and the absence of any accepted or pending terminal proposal. The fallback only explains structured rejections or submits non-authoritative correction proposals; it performs no canonical dispatch, terminal bind, wake-policy, report, fate, freeze, or revoke write. Exercise explicit child, terminal-without-packet, dead, timeout-observed-dead, cancel, restart, duplicate, and late-different-packet cases."
      proves: "Terminal truth has one author; delegation closes deterministically without a second committer; late difference can never masquerade as replay."
      evidence_class: local_test
    - action: "Consume mission-0's accepted restore/computer identity read-only; mission-0's blocked live-drill debt stays mission-0-owned and is neither implemented nor waived here (owner-settled 2026-09-09; docs/mission-residues.md R1). Using scoped owner CLI/product APIs without SSH on the freshly reconciled staging computer/realization/build/route/fence with effects OFF (effects OFF forbids guest self-development mutation, not assignment-capsule freeze/revoke lifecycle fate), demonstrate both the repaired admitted serialized eval-then-terminal shape and its refusal variants, and on a separate admitted attempt a successful evaluation, freeze, accepted terminal pass, and revocation with no terminal finality visible until the revoke acknowledgement. Then re-submit the identical canonical terminal packet under fresh provider-call metadata and observe replay of the original receipt with no second report, fate, outbox, or wake. Run the cancellation race on another identified attempt with no second accepted packet/wake/fate effect; bind execution, intent/ack, parent/outbox, freeze/revoke, environment, deploy, and verifier receipts to one attempt/computer/capsule/deployment identity. In one atomic source commit, write mission-1's terminal receipt, mark the target-cutover Definition superseded by this settlement gate under the recorded owner decision (owner-settled 2026-09-09: the cutover was decomposed and its program continues in future missions), and update docs/ACTIVE.md, docs/mission-graph.yaml, and docs/doc-authority-manifest.yaml as projections in that same commit, recording registry-conformance verification in the terminal receipt. Historical epoch-888 receipts are provenance only, never a requirement to reuse the old epoch or SHA."
      proves: "The withheld cutover run acceptance closes under this mission on the physical staging computer through a demonstrated successful settlement path plus provider-fresh replay, not refusal alone; the decomposed cutover retires as superseded by owner decision."
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
    - "Any rejection in a proven non-mutating class loses heap/imports, or any unproven interpreter failure or runtime failure reuses the interpreter."
    - "Two execution paths exist on the active RLM route, or any diagnostic on it is replaced by a generic wait error."
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
    - "Compile preservation requires the isolation experiment; without it the failure stays unsafe-to-reuse with its diagnostic kind preserved."
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
    - "internal/agentcore/cosuper_assignment_fate.go and internal/store/cosuper_assignments.go (report/fingerprint identity, CAS, correction-as-new-attempt)"
    - "internal/agentcore/researcher_checkpoint_fallback.go"
    - "internal/agentcore/rlm_reduce.go, internal/agentcore/tools_capsule.go, capsule revoke/fate reconciliation, run acceptance, and staging deployment routing"
  completion_evidence_floor: [local_test, deployed_proof]
  conjecture_delta:
    discovered:
      - "One reducer-side terminal contract can retire the JSON batch race without a second settlement implementation."
      - "A typed pre-execution error class can preserve compile feedback without risking silent heap corruption."
    falsifiers:
      - "Yaegi cannot demonstrate a non-executing boundary; then compile preservation is refused and all failures stay unsafe-to-reuse."
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
  slice: "round-5 repaired draft under focused verification; mission 0 complete; charter ratification pending"
  question: "Is the reconciled round-5 repaired charter with frozen R4/R5 ready for owner ratification?"
  reconciliation:
    observed_at: "2026-09-09T00:00:00Z"
    source_ref: "main@35810aad (mechanical review head; round-5 draft edits committed alongside)"
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
  next_action: "Focused verification consensus on round-5 repairs; reconcile the repaired draft with fresh source, deploy, worktree, and authority identities; freeze R4 correction fields and R5 recognition inventory; then present the reconciled charter for owner ratification."

receipts: []
