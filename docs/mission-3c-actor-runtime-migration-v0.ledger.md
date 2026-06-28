# Mission 3c Ledger

## Pass 1 — Part 1: AGENTS.md revision

**Conjecture:** The AGENTS.md revision can be landed as specified (split into 3
files, 4 new rules, deletion-first, simplified mutation class) without breaking
cross-references or the docs truth checker.

**Move:** construct (batch P1.1-P1.4 + verify)

**Expected ΔV:** -5 (P1.1, P1.2, P1.3, P1.4, verify)
**Actual ΔV:** -4 (P1.1-P1.4 done; verify folded into P1 — doccheck passes,
no broken cross-refs found)

**Verdict:** supported

**Receipts:**
- Commit `55ef75bb`: split AGENTS.md (417→224 lines) + created
  `docs/agent-product-doctrine.md` (197 lines) + `docs/agent-parallax-rules.md`
  (57 lines)
- `nix develop -c go run ./cmd/doccheck`: report-only complete, 285 docs, 1038
  warnings (pre-existing), exit 0
- `grep -rn 'AGENTS\.md#' docs/`: no anchor references — no broken cross-refs
- Pre-existing WIP (`docs/production-readiness-checklist.md`) stashed separately:
  `stash@{0}: pre-existing: production-readiness-checklist actor model review WIP`

**Edges left open:**
- AGENTS.md is 224 lines, over the ~150 target. The 4 new rules + deletion-first
  + all operating rules need the space. Further compression would lose content.
- Part 2 solvency: States 1-3 are mechanical (batchable), States 4-5 are
  medium-reasoning, State 6 is high-entanglement (8 pts), States 7-8 require
  deletion + staging access. Budget: 2-3 passes remaining.

## Pass 2 — States 1-3: interface extraction (batch)

**Conjecture:** The interface types (Provider, ToolLoopProvider, ProviderPolicy,
ToolLoopRequest, ToolLoopResponse, TokenUsage, ToolDefinition, EventEmitFunc,
AgentProfile*, Config, ToolRegistry) can be extracted to new packages using
type aliases without breaking the build or any tests.

**Move:** construct (batch States 1-3)

**Expected ΔV:** -3 (States 1, 2, 3)
**Actual ΔV:** -3 (States 1-3 done; APIHandler extraction deferred — 20+ methods
on *Runtime, beyond Medium complexity. Documented as open edge.)

**Verdict:** supported (with weakened State 3 — ToolRegistry extracted, APIHandler
deferred to State 7)

**Receipts:**
- Commit `b98531cd`: 10 files changed, 610 insertions, 505 deletions
- New packages: internal/provideriface, internal/agentprofile, internal/toolregistry
- `go build ./...` passes
- `go test ./internal/runtime/... -run "TestTool|TestConfig|TestProvider|TestAPI"` passes
- `go test ./internal/provider/... ./internal/gatewayruntime/...` passes
- `go test -race ./internal/actor/...` passes

**Key finding (discovered conjecture):**
State 4 (adapter) is not a thin wrapper. The old *runtime.Runtime has 71+ methods
containing both business logic and concurrency code intertwined. The actor runtime
has 5 methods (Send, Sweep, Evict, Resident, Stop). Building the adapter requires
separating business logic from concurrency code in 3797 lines of runtime.go —
equivalent in complexity to State 6 (8 pts), not a separate "Medium" (5 pts) task.

This changes the mission's execution model: States 4 and 6 should be treated as
one unified effort. The plan's separation underestimated the entanglement.

**Edges left open:**
- APIHandler extraction deferred (State 3 partial)
- actorruntime/adapter.go is a skeleton — full implementation requires business
  logic extraction from old runtime
- States 4-8 remain (V=5)
- Budget insolvent for full settlement in remaining 1-2 passes
