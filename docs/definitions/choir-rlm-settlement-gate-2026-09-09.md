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
    - claim: "The loop's detached-terminal singleton predicate closes only single recognized-terminal turns; the sequential-execution inventory omits the capsule_go_eval and record_assignment_result pair at its dispatch anchor, so evaluation plus terminal report fall through to WaitGroup parallel dispatch. Mechanically verified at main@d3e253b9; inventory re-pins at charter and is input to the R5 narrow grammar."
      evidence_ref: "internal/toolregistry/toolloop.go:622; internal/toolregistry/batch_executor.go:114-129 (inventory omission); internal/toolregistry/batch_executor.go:39-47 (parallel dispatch)"
    - claim: "The researcher fallback binds delegated terminal outcomes into the parent under guards (root runs return, lifecycle-authoritative runs abstain, explicit identity matching, duplicate bindings rejected) and synthesizes reference updates with channel emission and wake on the bound path. Mechanically verified at main@d3e253b9."
      evidence_ref: "internal/agentcore/researcher_checkpoint_fallback.go:23-122,134-167"
    - claim: "The cutover's run acceptance is withheld for this remainder; mission 0 executes the restore predecessor in parallel with this draft. (Start-time observation; mission-0 completion recorded as dated start_correction below, not edited into this receipt.)"
      evidence_ref: "docs/definitions/choir-rlm-target-architecture-cutover-2026-09-04.md remainder disposition; docs/definitions/choir-rlm-restore-zero-2026-09-08.md"
  start_correction:
    - date: "2026-09-09"
      note: "Mechanical review at main@d3e253b9 with source-trace convergence across verification rounds: claims 1-6 are source-trace observations anchored at cited lines (the panel-sourced diagnostic-loss and inventory-omission claims are code-verified, not folklore); claim 7 is documentary (cutover remainder disposition). Refinement history: the claim texts and evidence references above were revised in place across repair rounds (commits e7d1f613, d3e253b9, 35810aad, ed2ad8fb, 81e2d589, 3dc2f3c0, 65e4c797, 0314ec70); the original draft-time wordings, including panel-evidence labels, are recoverable from git history, not reproduced here. d3e253b9 and later review checkpoints are never charter HEAD. Mission 0 completed with deployed proof; its blocked live-drill debt stays mission-0-owned (residue R1). All claims require charter-head re-pinning. Charter replaces receipts: [] below with the real code-free Define receipt."
  unknowns:
    - "Live staging/base identities at charter; fresh reconciliation required."
    - "Whether Yaegi exposes a genuinely non-executing compile/type boundary (isolation experiment outstanding)."
    - "Full sequential-tool and terminal-recognition inventory beyond panel-cited sites."
    - "Measured terminal-conflict and batch-shape distributions."
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
      - "capsule revoke/fate reconciliation paths, and the capsule error/sidecar type seam carrying the typed reuse disposition plus independent diagnostic kind"
      - "internal/agentcore/rlm_reduce.go, internal/agentcore/tools_capsule.go, and the runtime/store transaction seam carrying the atomic promotion"
  acceptance:
    - action: "Run the isolation matrix against the exact Yaegi/session construction: only host CheckImports/imports-only rejection proved to occur before interpreter entry may preserve the live interpreter, and the loop/broker must carry that typed class without worker disposal. Body syntax, type, declaration, and every other EvalWithContext error remain unsafe-to-reuse unless the matrix proves an exact non-mutating class, with the original syntax, type, runtime, timeout, panic, overflow, worker, or transport diagnostic kind preserved separately; a valid successor observes exactly the prior heap only for a proven class. Every runtime error, panic, timeout, overflow, worker death, transport failure, or doubt poisons and respawns from the durable snapshot; string matching is forbidden. If no further class passes, the host-preflight class alone preserves the interpreter; every other failure remains typed unsafe-to-reuse."
      proves: "Only proven non-mutating rejection classes preserve working context; reuse disposition is typed separately from diagnostic kind; string matching carries no classification."
      evidence_class: local_test
    - action: "Unify RLM execution onto the persistent session with disposal as its single-cell case. Under actuator=rlm, remove fallbackGoEval: a session worker start failure returns a typed session diagnostic and never executes the cell through the one-shot worker. The one-shot worker remains only for explicit actuator=tools and is unreachable from the RLM route. One typed diagnostic contract governs the active RLM route."
      proves: "One execution path and one diagnostic contract on the active RLM route; the tools rollback route is preserved, not silently deleted."
      evidence_class: local_test
    - action: "Define the terminal identity contract with the immutable versioned field-classification receipt (v1, frozen at charter): slot identity = owner, canonical computer, assignment, attempt; the ordinary terminal path and the delegated-run/orphan path compete for the same obligation's terminal truth and never hold a parallel orphan slot (a delegated-run slot exists only where no assignment obligation exists). Every submitted request/report/tray/orphan/reference field classifies as exactly slot, canonical, reducer_derived, or excluded, with unknown fields and extensions failing closed; the table covers outputs, mutations, execution attestations and receipt references, command/execution references, exit codes, candidate/certification facts, ordering, and nested reference content. Canonical input is only submitted claims plus authenticated receipt facts available before reservation: result, verdict, subject/candidate/certification/effect declarations, the ordered command/digest sequence, and canonically normalized evidence references (nested receipts contribute only their classified canonical fields, never provider/transport/retry/batch/model/timestamp/summary/prose envelopes). Actual freeze/revoke observations, candidate/artifact discovery, dispositions, identifiers, and all other post-reservation facts are reducer-derived: a mismatch finalizes a rejection against the original slot/digest and never rewrites the digest. Normalized sequence/set input = ordered sequences and key-sorted sets per the frozen normalization; reducer-derived output = report/command/fate/outbox/receipt identifiers, freeze/revoke overlay fields, and cancellation/late-fate dispositions; excluded metadata = provider-supplied identifiers, transport/retry/batch/model metadata, timestamps, late flags, summary and prose. The versioned canonical-proposition digest is computed before any report, command, fate, outbox, or receipt identifier is derived and covers exactly canonical submitted proposition content plus content of referenced receipts where explicitly classified. Report, command, fate, outbox, and receipt identifiers are domain-separated derivations from (reserved slot, proposition digest) and are never digest inputs. Freeze nil/empty handling, ordering, normalization, and domain separation. The terminal uniqueness/CAS slot is keyed solely by the immutable slot identity: a same-slot/same-digest submission replays the original pending/final receipt, while a same-slot/different-digest submission conflicts before report, fate, outbox, wake, or physical effect. Partial reports use an explicitly separate nonterminal sequence and never occupy the terminal slot. Cancellation-wins and late-fate outcomes are reducer dispositions recorded on the receipt, never state-dependent rewrites of the submitted packet before identity. Corrections land as new attempts (owner-settled) carrying the frozen superseding tuple (supersedes_assignment_id, supersedes_attempt, prior_receipt_ref, supersede_kind in {correction, retry_after_block, owner_reopen}, reason_enum, delta_digest over structured fields only, no summary or prose). Rejection classes: (a) pre-reserve admission reject (schema/auth/nonterminal-misuse) occupies no terminal slot, so the same attempt may submit a well-formed packet; (b) reserved same-digest replays the original pending/rejected/final receipt and never becomes a second accept; (c) reserved different-digest conflicts, and correction proceeds only as a new attempt with the tuple. Behavior cases at charter: each canonical-field mutation (including an Outputs-only change) conflicts; excluded metadata never does; provider-fresh same semantics replays; same slot/different digest yields no second report, fate, outbox, wake, or effect."
      proves: "An identical canonical typed proposition replays the original receipt despite changed provider-call, retry, batch, transport, model, summary, or other explicitly excluded metadata; any difference in included canonical proposition content conflicts and cannot reserve a second terminal row; rejections never brick an attempt nor smuggle a correction."
      evidence_class: local_test
    - action: "Enforce the narrow assigned-CoSuper admission grammar (owner-settled 2026-09-09: narrow consequential subset). Stage 1, before dispatch: apply this grammar to assigned-CoSuper batches containing capsule_go_eval or an explicit assignment terminal, because evaluation can create assignment-fate intents; batches with neither fall under existing policy. Statically refuse two or more explicit terminal calls and every statically forbidden companion with none run. Stage 2, after evaluation: an eval-staged tray terminal is unknowable until capsule_go_eval has run, and the cell plus any capsule-local mutation may already have occurred — the contract never claims the eval did not run. Execute at most one eval, then inspect and validate the complete tray before assignment-fate reduction, terminal reservation, or any later JSON terminal; a tray/JSON terminal collision, a tray with two or more assignment-fate terminal intents, or an unknown tray kind discards the tray, leaves the assignment slot untouched, skips later members, and stops the model turn. The exhaustive assignment-fate skeleton: (a) later-turn singleton record_assignment_result; (b) at most one capsule_go_eval then at most one JSON terminal, strictly sequential with ExecuteToolBatch parallel dispatch forbidden. capsule_go_eval, mailbox choir.Complete/IntentComplete, and partial reports may accompany one admitted skeleton but never occupy the terminal slot or create a second shape; eval-only and eval-plus-Complete with zero JSON terminals are admitted as (b). Mailbox Complete/IntentComplete tray or cursor effects, if any, are governed by the mailbox contract only and never constitute assignment settlement; Complete never creates a terminal and never aborts an admitted JSON terminal. Partial reports are shape-ignored as a separate nonterminal sequence (charter confirms). Assignment-fate terminals count together across explicit JSON members and eval-staged tray intents: two or more, or any other assignment-fate-class member, refuse; unlisted consequential or JSON companions, including overlay tools, fail closed and never fall through to parallel dispatch. A failed or poisoned eval aborts later members unexecuted and leaves the slot untouched. A validation failure is class (a) only when authoritative evidence proves the reservation request never reached the reducer and no in-flight request can still reserve; a timeout, disconnect, worker death, lost response, or other outcome-unknown failure stops batch and model loop, reconciles the original slot and digest, and never authorizes a corrected packet until absence of a reservation is proved — an existing same digest resumes or replays and a different digest conflicts. A class-(a) reject or any rejection stops the current batch and never licenses a later terminal in that turn; only class-(a) pre-reservation rejection leaves the provider/model turn loop able to resubmit on the same attempt. Accepted, same-digest replay, conflict, or unresolved-pending disposition cannot permit a later terminal in that batch. Post-durable-reservation interruption holds the pending slot and follows the fate saga; the loop stops. Independent reducer uniqueness holds regardless. Exercise two-terminal preflight refusal, later-turn singleton terminal, eval-then-terminal clean, failed-eval abort, eval-staged tray terminal plus JSON terminal, eval-plus-Complete, eval-plus-Complete-plus-JSON, eval-plus-unknown-tray-plus-JSON, two explicit terminals where the first would reject, mixed reject-then-terminal, errored terminal with no disposition, duplicate/replay, and the Complete-plus-JSON successful path."
      proves: "The singleton special case is gone; contradictory turns never execute; the reservation split never bricks an attempt nor double-mints; settlement is a reducer property, not a batch-shape accident."
      evidence_class: local_test
    - action: "Durably commit the reducer-owned pending proposal (immutable slot/digest, staged effects, operation identities, fence) as computer-durable assignment state before issuing any physical action — never broker or capsule process memory, so rematerialization resumes that row through the saga, never through the orphan path. The present implementation order is the defect this saga replaces: the terminal report commits at frozen-capsule acknowledgement and revocation follows (internal/agentcore/cosuper_assignment_fate.go:733-748), so a crash in that window leaves assignment-final truth beside a frozen-but-unrevoked capsule — finality published before the last physical revoke acknowledgement. Freeze and revoke are sequential fenced operations of the pending proposal instead; frozen is observable capsule state but never terminal assignment finality, delivery, or wake. After the final required physical acknowledgement (revoke on the demonstrated pass path; for every disposition requiring physical revoke, finality waits for every required acknowledgement), one reducer transaction records disposition/receipt, staged durable effects, parent update, outbox entry, and inbox-cursor advancement; before that ack, no terminal finality is visible (owner-settled 2026-09-09). Only after commit does an idempotent outbox consumer emit the channel event and wake: wake is a replayable projection, never part of the atomic transition and never a second author. A dead or orphaned child first resumes reconciliation; timeout alone never releases, replaces, or expires the reservation, and expiry requires authoritative evidence that every issued operation completed or can no longer execute under its fence, or acknowledged compensation — otherwise a visible blocked pending state persists with neither terminal finality nor orphan disposition published. Dead-actuator liveness: authoritatively observed capsule/VM death or fence supersession resumes the pending saga along the dotted path — re-resolve every issued operation under the new fence, reissue only what the authoritative resolution proves cannot still execute, or acknowledge compensation; where no authoritative observation is obtainable, the reservation remains visibly blocked pending indefinitely rather than timing out into a second author. The actuator durably deduplicates by fate identity and current fence, or exposes authoritative operation resolution proving the original cannot still execute before any reissue; query-then-reissue against a possibly live operation is forbidden. Fault-inject every intent/effect/ack/finalization boundary, including stale freeze-vs-revoke fencing and cancellation racing pass. Same slot plus identical packet returns the original receipt across restart/retry storms; changed semantic content conflicts before new report, fate, outbox, or wake effects. This is a fenced intent/ack saga, not a capsule distributed transaction."
      proves: "Fate is a resumable saga with one atomic final boundary; revocation races and dead children resolve to one visible finality."
      evidence_class: local_test
    - action: "Close the fallback canonical-author port: this path opens on authoritative durable child termination without an accepted terminal packet — including successful completion without a packet, cancellation, or confirmed death — and a lifecycle/reconciliation producer submits one authenticated immutable orphan observation only (scope, child/run identity, liveness/terminal observation, reason enum, timestamps, evidence) with the reducer alone validating it and deriving disposition, terminal proposition, parent update, receipt, outbox, and wake. Liveness timeout alone never opens this path and never expires a pending freeze/revoke reservation. A pending terminal proposal always reconciles through the fate saga, never through orphan disposition. Orphan admission atomically rechecks lifecycle state and the terminal slot: only an unreserved slot may reserve an orphan proposition, pending slots reconcile through the fate saga, and accepted or rejected reservations retain their proposition with replay/conflict rules intact. On authoritative child termination after a reserved rejection, the reducer closes the parent obligation using that rejection receipt without replacing the proposition or accepting another packet; correction is a new attempt. The fallback only explains structured rejections or submits non-authoritative correction proposals; only the reducer authors terminal outcomes and canonical updates, the fallback has neither canonical-write nor wake authority, and the reducer-committed outbox consumer alone emits channel events and wakes. Prove by caller-map that no fallback or non-reducer path outside that consumer can bind a terminal outcome, emit a canonical update, or wake. Exercise durably-completed-without-packet, cancelled, dead, timeout-observed-dead, restart, duplicate, and late-different-packet cases."
      proves: "Terminal truth has one author; delegation closes deterministically without a second committer; late difference can never masquerade as replay."
      evidence_class: local_test
    - action: "Consume mission-0's accepted restore/computer identity read-only; mission-0's blocked live-drill debt stays mission-0-owned and is neither implemented nor waived here (owner-settled 2026-09-09; docs/mission-residues.md R1). Land in this order: implementation commit, then push with CI green, staging deploy, and fresh environment observation, then the deployed proof on the freshly reconciled staging computer/realization/build/route/fence with effects OFF (effects OFF forbids guest self-development mutation, not assignment-capsule freeze/revoke lifecycle fate) using scoped owner CLI/product APIs without SSH. For each scenario — repaired admitted serialized eval-then-terminal shape and refusal variants; separate admitted attempt with successful evaluation, freeze, accepted terminal pass, and revocation with no terminal finality visible until the revoke acknowledgement; re-submitted identical canonical terminal packet under fresh provider-call metadata replaying the original receipt with no second report, fate, outbox, or wake; cancellation race on another identified attempt with no second accepted packet/wake/fate effect — bind execution, intent/ack, parent/outbox, freeze/revoke, environment, deploy, and verifier receipts to that scenario's own attempt/computer/capsule/deployment identity, and link the distinct scenarios through one immutable acceptance manifest. Finally, in one atomic source commit, write mission-1's terminal receipt recording only already-observed immutable receipts, apply a dated correction to the target-cutover Definition's now card recording closure of the withheld settlement acceptance while retaining its R6 remainder-holder status, and update docs/ACTIVE.md, docs/mission-graph.yaml, and docs/doc-authority-manifest.yaml as projections in that same commit, recording registry-conformance verification in the terminal receipt. This item is the withheld-run-acceptance closure; the focused-contracts item below is adjacent regression coverage only. Historical epoch-888 receipts are provenance only, never a requirement to reuse the old epoch or SHA."
      proves: "The withheld cutover run acceptance closes under this mission on the physical staging computer through a demonstrated successful settlement path plus provider-fresh replay, not refusal alone; the cutover retires only by a named successor, never by implication."
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
    - "Two execution paths exist on the active RLM route, including any session-spawn fallback diversion, or any diagnostic on it is replaced by a generic wait error."
    - "Two accepted terminal packets can coexist for one attempt, or a pre-reservation rejection occupies a slot or commits submitted effects, or a reserved rejection is treated as successful settlement or licenses a replacement proposition."
    - "A fate interruption can partially commit, duplicate, or lose the terminal outcome, or a pending reservation is released or expired without authoritative evidence that every issued operation completed or can no longer execute under its fence."
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
    - "internal/agentcore/rlm_reduce.go, internal/agentcore/tools_capsule.go, capsule revoke/fate reconciliation, the capsule error/sidecar type seam, run acceptance, and staging deployment routing"
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
  - name: terminal_replay_predicate
    kind: gate
    baseline: "Withheld acceptance on one epoch-888 retry; distributions unmeasured."
    desired: "Same slot plus same digest replays the original receipt under any excluded metadata; same slot plus different digest conflicts before any report, fate, outbox, wake, or effect. Retry/conflict distributions remain telemetry only."
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
    desired: "The frozen narrow grammar admits exactly its enumerated shapes; every unlisted consequential shape fails closed."
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
  slice: "round-10 repaired draft under focused verification; mission 0 complete; charter ratification pending"
  question: "Is the reconciled round-10 repaired charter with two-stage admission, total classification, and liveness policy ready for owner ratification?"
  reconciliation:
    source_ref: "main@9f95e590 (round-10 repaired draft; re-observe HEAD at charter ratification)"
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
    source: orchestrator
    evidence_ref: "Owner-settled mission order and owner-settled narrow-grammar, revoke-visibility, and cutover-holder choices; the complete settlement architecture remains a proposal until the ratification receipt exists"
    owner_ratification_ref: "charter ratification still required before execution"
    recorded_at: "2026-09-09T00:00:00Z"
    consequence: "Draft and review only. No settlement repair executes under this file before charter ratification with fresh reconciliation. Mission-0 drill debt stays mission-0-owned (residue R1)."
  evidence_refs:
    - "internal/yaegikernel/session.go"
    - "internal/yaegikernel/session_loop.go"
    - "cmd/capsule-broker/session_worker.go"
    - "internal/agentcore/cosuper_assignment_fate.go"
  blocker_or_risk: "Charter ratification pending. Yaegi isolation experiment unproven (host-preflight class only until the matrix proves more). Classification receipt v1, two-stage narrow grammar, dead-actuator liveness policy, and R6 cutover-holder retention are now in the draft; charter re-pins and ratifies them."
  next_action: "Focused verification consensus on round-10 repairs; reconcile the repaired draft with fresh source, deploy, worktree, and authority identities; then present the reconciled charter for owner ratification."

receipts: []
