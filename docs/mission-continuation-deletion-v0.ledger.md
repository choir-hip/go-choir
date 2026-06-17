# M4 Continuation Deletion — Ledger

Append-only Parallax mission ledger. State lives in
`docs/mission-continuation-deletion-v0.md`; this file is written, not re-read.

## 2026-06-17 - Paradoc compiled (Path A design)

Claim/scope: M4 is the spine deletion between M3 lifecycle cutover and M5 wire
on settlement; it had no paradoc (`m4-continuation-deletion` was `planned` with
empty path in `docs/mission-graph.yaml`). Compiled the paradoc as the Path A
design artifact. No runtime behavior change in this pass.

Move: construct (design only). Authored
`docs/mission-continuation-deletion-v0.md` with full Parallax State and a
Suggested Goal String, grounded in the current continuation surface inventory
(`internal/runtime/continuation.go` ~293 lines, `internal/store/continuations.go`,
`/api/continuations/*` in `internal/runtime/api.go`, `types.RunContinuationRecord`,
`continuation-level` acceptance, trace/compaction/product-api/run-acceptance
references). Recorded the route insight: deleting the dual-model continuation
control surface is conjectured to remove the cause of the recurring Texture
product-loop regressions (M3.x treadmill), not just to tidy up.

Expected ΔV: 0 at execution level (no code deleted yet); the value is removing
a route-planning gap (M4 had no paradoc). Status set to `planned`, explicitly
gated on M3 settlement.

Receipts:
- `docs/mission-continuation-deletion-v0.md`
- `docs/mission-continuation-deletion-v0.ledger.md`
- `docs/mission-graph.yaml` node `m4-continuation-deletion` (path + ledger).

Open edge: do not begin deletion until M3 settles or names its blocker, because
M3 settlement names which run rows / parent-run fields remain audit-only and M4
deletes the layer above them. First execution move is the caller map.
