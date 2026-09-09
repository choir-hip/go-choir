# Settlement Gate Item 4 — Code-Free Define Receipt (2026-09-09)

Mutation class: green (docs only). No source changed in this receipt.
Mission: `docs/definitions/choir-rlm-settlement-gate-2026-09-09.md`, acceptance item 4.
Prior: item-3 repair `965e26a7` (terminal identity contract, proposition digest, slot gate, supersede tuple, and strict admission).
Worktree: 4 untracked unrelated-WIP paths preserved (mission-0 completion report,
report generator script, pycache, tmp/).

## Problem (observed, not inferred)

1. **Parallel execution omission on the active settlement route.**
   In `internal/toolregistry/batch_executor.go:123-130`, `toolRequiresSequentialTurnExecution`
   lists sequential tools (`bash`, `write_file`, `patch_texture`, etc.), but omits
   `capsule_go_eval` and `record_assignment_result`.
   Consequently, any turn containing both `capsule_go_eval` and `record_assignment_result`
   falls through to the goroutine-based `sync.WaitGroup` parallel dispatcher (`:39-47`),
   racing evaluation execution against settlement report recording in parallel threads.
2. **Missing batch admission grammar (Stage 1 pre-dispatch).**
   An assigned CoSuper batch can currently submit:
   - Two or more explicit terminal calls (`record_assignment_result` with completed/failed/blocked) in a single turn.
   - Two or more `capsule_go_eval` calls in a single turn.
   - Reversed companion order (`record_assignment_result` preceding `capsule_go_eval`).
   - Forbidden companions (e.g. bash, write_file, spawn_agent) alongside an explicit terminal.
   Currently, none of these invalid shapes are statically refused before dispatch; they partially execute or race concurrently.
3. **Missing post-eval tray validation (Stage 2 post-eval).**
   When `capsule_go_eval` executes, an eval cell may stage tray intents. If a turn produces
   both a tray terminal intent and a subsequent explicit JSON `record_assignment_result`,
   the two settlement paths collide. Furthermore, if an eval fails or is poisoned, subsequent
   terminal calls in the same turn must be prevented from committing contradictory settlement.
4. **Tool loop singleton predicate lock-in.**
   In `internal/toolregistry/toolloop.go:622`, terminal closure is guarded by `len(resp.ToolCalls) == 1`.
   This singleton check prevents admitted serialized multi-tool shapes (e.g. `[capsule_go_eval, record_assignment_result]`)
   from completing under the detached terminal timeout, forcing models to split eval and settlement across separate turns.

## Authorized repair boundary (item 4 only; red, next commit)

1. **Sequential execution classification (`internal/toolregistry/batch_executor.go`):**
   Add `"capsule_go_eval"` and `"record_assignment_result"` to `toolRequiresSequentialTurnExecution`.
   Every turn containing either tool executes sequentially in provider call order.
2. **Stage 1 pre-dispatch admission validation (`internal/toolregistry/batch_executor.go`):**
   In `ExecuteToolBatch`, before executing any tool in the batch, if the batch contains `capsule_go_eval` or `record_assignment_result`:
   - Refuse if explicit terminal calls (`record_assignment_result` with terminal result) $\ge 2$.
   - Refuse if evaluation calls (`capsule_go_eval`) $> 1$.
   - Refuse if `record_assignment_result` precedes `capsule_go_eval`.
   - Refuse if any companion tool is present alongside an explicit terminal (only `[capsule_go_eval, record_assignment_result]` is admitted).
   - "With none run": when refused, every tool in the batch receives a typed `tool_error: admission_grammar_refusal: <reason>`, and zero tools execute.
   Admitted shapes:
   - Shape (a): later-turn singleton `record_assignment_result`.
   - Shape (b): at most one `capsule_go_eval` then at most one JSON `record_assignment_result`, executed strictly sequentially.
3. **Stage 2 post-eval tray validation (`internal/agentcore/tools_capsule.go`):**
   In `newCapsuleGoEvalTool`, after `Executor.GoEval` returns:
   - Check `result.Intents`. If the tray contains contradictory terminal intents or if a subsequent sibling `record_assignment_result` is present in the turn, fail closed and mark the turn context to prevent double-minting or multi-terminal collisions.
   - Preserve mailbox `choir.Complete` / `IntentComplete` as mailbox-only completion, distinct from assignment-fate settlement.
4. **Tool loop decoupling (`internal/toolregistry/toolloop.go`):**
   Update `toolloop.go:622` to recognize admitted batch terminal shapes (singleton terminal or eval+terminal) rather than strictly requiring `len(resp.ToolCalls) == 1`.
5. **Tests:**
   Unit and contract tests in `internal/toolregistry` and `internal/agentcore` covering all admitted shapes and refusal variants (2+ terminals, 2+ evals, reversed order, forbidden companions, none-run guarantees).

## Explicitly not in this boundary

Fate saga reordering and pending proposal durability (item 5), orphan close (item 6),
physical staging proof (item 7).

## Evidence floor / rollback

Floor: local tests for batch executor admission, sequential execution, and eval dispatch.
Rollback: revert the repair commit; this Define receipt retained.

## Standing-questions answers for this boundary

1. Settled by: owner-chartered definition item 4 (narrow consequential subset, owner direction 2026-09-09).
2. Topology: conforms to charter entrypoints (`batch_executor.go`, `tools_capsule.go`, `toolloop.go`).
3. Deletion citers: singleton check in `toolloop.go:622` updated to admitted shape predicate.
4. Consumers: tool batch dispatcher, agentcore capsule eval and settlement tool loops.
5. Single authority: reducer-owned settlement; admission grammar protects reducer from contradictory turns.
6. Artifact: local_test admission matrix (refusals, shape a, shape b).
7. Fate-sharing: pre-dispatch refusal runs in-process before any capsule or store action.
8. Restart: stateless batch validation; no persistent state added.
9. No SSH: local unit and integration tests.
