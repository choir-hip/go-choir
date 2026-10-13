# Cut 0 breakage log (2026-10-12)

One log per cut window (docs/re-engineering-decisions-2026-10-12.md §12.4):
each problem found during Cut 0 and fixed in the same commit is appended
here in that commit. A problem not fixed in the same commit gets its own
problem doc first. Reviewed once at the cut's close.

Cut: docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md.

## B1. Stored run progress events lost their phase; cost extraction read nothing

- **Found:** while mapping the trace bundle's read surfaces (C0 unit 1).
- **Evidence:** the runtime's emit closure took `phase` but never wrote it
  (internal/agentcore/runtime.go, the `emit :=` closure before this fix);
  `EventRecord.Phase` is not persisted by the store; the tool loop's
  per-turn payload has no `phase` key (internal/toolregistry/toolloop.go,
  the progress payload). `llmcost.ExtractCostEntries` keys on
  `payload.phase` == "tool_loop" / "execution", so on real rows it found
  no model turns, while its tests injected the phase by hand
  (internal/apihandler/costs_test.go). The Cut 0 per-turn token baseline
  reads the same rows.
- **Fix:** `Runtime.runEventEmitter` writes the phase into a progress
  payload that is a JSON object without one. Payloads that already name a
  phase, non-object payloads and tool events are unchanged.
- **Test:** TestRunEventEmitterPersistsPhase (stored rows carry the phase;
  cost extraction reads both model turns).
- **Class:** red (Trace/evidence rows; additive payload key). Rows written
  before the fix stay without a phase; nothing is backfilled (§13.1).
