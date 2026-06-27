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

## Pass 2 — 2026-06-27 (W5 staging verification)

### Problem discovered: Ollama not deployed on node-b

**Evidence (staging, 2026-06-27T23:45Z):**

- `ssh node-b "systemctl is-active ollama"` → `inactive`
- `ssh node-b "systemctl status ollama"` → `Unit ollama.service could not be found`
- `ssh node-b "curl -s http://localhost:11434/api/tags"` → connection refused
  (port 11434 not listening)
- `grep -r ollama /etc/nixos/` on node-b → no results (not in NixOS config)
- `grep ollama nix/node-b.nix` in repo → no matches (not in flake config)

**Impact on mission 3b:**

The semantic dedup pass (`dedupSourceItemsSemantically`) requires Ollama to
embed item text before searching Qdrant. Without Ollama, the dedup pass
falls back to pass-through (best-effort design): all items proceed to
objectgraph projection and processor dispatch. This means:

1. Ingestion still works — items are captured and projected into the
   objectgraph, processor dispatch proceeds.
2. Semantic dedup does NOT work — near-duplicate captures are not filtered.
   Content-hash dedup in the cycle engine still prevents exact duplicates.
3. The `QdrantDedupThreshold` config (W3) and dedup code (W2) are deployed
   but inert until Ollama is available.

**Root cause:**

Ollama was never added to the node-b NixOS configuration
(`nix/node-b.nix`). The Qdrant service was added (lines 211-226) but the
Ollama embedding service was not. The production readiness checklist
(line 50) notes "Ollama failure mode" as an open item, and line 63 lists
"Ollama model update" runbook as unimplemented.

**Belief state:**

- W1 (per-source-type polling): will work on staging once deployed — no
  Ollama dependency.
- W2 (semantic dedup): code deployed but inert — Ollama missing.
- W3 (threshold config): code deployed but inert — Ollama missing.
- W4 (objectgraph writes): will work on staging — no Ollama dependency.
- W5 (real article production): should work — ingestion + processor dispatch
  do not require Ollama. Articles can still be produced from real captures
  without semantic dedup.

**Decision:**

Do NOT block W5 on Ollama. The mission conjecture is "real source captures
produce source-grounded articles" — that does not require semantic dedup.
Semantic dedup is a quality improvement, not a gating dependency for article
production. Document the Ollama gap as a follow-up work item (W6: deploy
Ollama on node-b) and proceed with W5 verification using the best-effort
pass-through path.

The Ollama deployment fix (adding `services.ollama` to `nix/node-b.nix`) is
a separate orange mutation that should be its own commit after this
documentation step, per Problem Documentation First.
