# Mission 3b Ingestion Path — Parallax Ledger

## Pass 1 — 2026-06-27

### Conjectures decided

- **C1 (supported)**: `PollBySourceType(ctx, sourceType)` filters sources by
  type without changing `PollAll` semantics. `PollAll` now delegates to
  `PollBySourceType(ctx, "")`. Receipt: `internal/cycle/cycle.go`,
  `internal/cycle` tests green.
- **C2 (supported)**: `QdrantDedupThreshold` is configurable via
  `QDRANT_DEDUP_THRESHOLD` (default 0.7862 in `LoadConfig`). A zero value
  disables semantic dedup — test configs that do not opt in pass through.
  Receipt: `internal/runtime/config.go`, `internal/runtime/qdrant_dedup.go`.
- **C3 (supported)**: semantic dedup runs in
  `HandleInternalSourcecycledWebCaptures` before objectgraph projection. It
  embeds each item, searches the production Qdrant collection, drops items
  whose top match score >= threshold, and upserts passing items. Best-effort:
  if Qdrant/Ollama are unavailable, all items pass through with a skip
  reason. Receipt: `internal/runtime/sourcecycled_web_captures.go`,
  `internal/runtime/qdrant_dedup.go`, web-captures test green with dedup
  disabled (threshold 0).
- **C4 (supported)**: sourcecycled writes web captures to the runtime's
  Dolt-backed objectgraph via the runtime web-captures HTTP endpoint when
  `VMCTL_SANDBOX_PROXY_SOCK` is set (staging node-b config). The local SQLite
  fallback in `sourcecycledObjectGraphServiceFromEnv` only runs when no
  runtime dispatcher is configured (local dev). Receipt:
  `cmd/sourcecycled/main.go:projectSourceItemsToObjectGraph`,
  `nix/node-b.nix` sets `VMCTL_SANDBOX_PROXY_SOCK`.

### Move

construct (batched): W1 + W3 + W2 + W4 implemented in one pass because the
route was unambiguous — each work item was a bounded construct whose shape
was decided by the paradoc. Tripwire: the web-captures test failed on the
first run because `QdrantDedupThreshold == 0` was treated as "use default
0.7862", causing the test to hit real Qdrant and drop its only item. Fixed
by making 0 mean "disabled" — production gets 0.7862 via `LoadConfig` env
default, test configs pass through. Re-ran the test: green.

### Expected vs actual ΔV

- Expected: ΔV = -4 (W1, W2, W3, W4 decided)
- Actual: ΔV = -4 (C1, C2, C3, C4 supported)
- V: 5 → 1

### Receipts

- `nix develop -c go build ./...` — exit 0
- `nix develop -c go test ./internal/cycle/... ./internal/qdrant/... ./internal/objectgraph/...` — all ok
- `nix develop -c go test ./internal/runtime/... -run 'TestHandleInternalSourcecycledWebCaptures'` — PASS
- `nix develop -c scripts/go-test-runtime-shards` — all 4 shards green (319 tests)

### Edges left open

- Staging Qdrant/Ollama availability unverified — dedup is best-effort, so
  ingestion proceeds if Qdrant is down, but no semantic dedup happens.
- Per-source-type tickers increase cycle frequency; backpressure logic
  unchanged. Staging must confirm no overload.
- The bridge to G (staging produces real articles) is still unproven — W5.

### Next move

commit W1-W4, push to main, monitor CI, verify staging E2E (W5).
