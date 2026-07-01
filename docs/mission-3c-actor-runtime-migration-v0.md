# Mission 3c: Actor Runtime Migration

**Status:** ready for execution  
**Date:** 2026-06-27  
**Umbrella:** `docs/mission-3-universal-wire-ingestion-rebuild-v0.md`  
**Predecessor:** `docs/mission-3b-ingestion-path-v0.md` (settled)  
**Plan doc:** `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md`  
**Successor:** `docs/mission-3d-distribution-v0.md` (to be drafted after 3c)

## Objective

Wire the durable actor runtime (`internal/actor/`, 326 lines, 1 mutex) into
production, extract and delete the borked old runtime (`internal/runtime/`,
3797 lines, 15 mutexes), and revise AGENTS.md to prevent the failure class
that caused two weeks of misdirected debugging.

The actor runtime was built and TLA+ verified on 2026-06-11. It was never
wired in. The wire pipeline runs on the old runtime — the exact substrate
bugs (lost wakes, check-then-act races, no backpressure, 10+ mutexes with no
lock ordering) that caused the universal wire failures.

## Why This Is the Key Priority

3b proved the ingestion path works end-to-end: real articles on staging. But
the pipeline runs on a broken concurrency substrate. Scaling to multiple VMs
(3d) on top of a broken substrate would multiply the failure modes. The
actor runtime is the correct substrate. It exists. It just needs wiring.

The two-week debugging loop happened because no agent connected the wire bugs
to the substrate. The AGENTS.md revision prevents this by adding rules for
checking existing fixes, root cause clustering, and dead-end escalation.

## Part 1: AGENTS.md Revision

Land first. Any agent working on Part 2 operates under the revised rules.

### P1.1: Split AGENTS.md into 3 files

- **`AGENTS.md` (~150 lines)** — operating rules loaded every session
- **`docs/agent-product-doctrine.md`** — product architecture rules, loaded on demand
- **`docs/agent-parallax-rules.md`** — long-running mission rules, loaded for Parallax work

### P1.2: Add four new rules to AGENTS.md

- **Check for Existing Fixes** — before debugging subsystem X, search for replacement implementations
- **Root Cause Clustering** — 3+ bugs in same subsystem in one week → stop patching, write clustering assessment
- **Substrate vs Symptom Classification** — classify every problem; 3+ symptoms from same substrate → substrate-level action
- **Dead-End Escalation** — 3+ iterations or 2+ days without convergence → stop, write structural assessment, escalate

### P1.3: Add deletion-first heuristic

Before adding code to fix a bug, ask: is the code being patched already superseded? Would deleting and connecting the replacement be safer?

### P1.4: Simplify mutation class ceremony

- Green/yellow: name the class, proceed
- Orange: name class + rollback path (full ceremony optional unless touching provider routing or VM lifecycle)
- Red/black: full ceremony required

## Part 2: Runtime Migration (8-State Machine)

Each state has preconditions and postconditions. Do not enter a state until
the previous state's postcondition is verified.

### State 1: Extract interfaces (3 pts, Low)

Move `runtime.Provider`, `runtime.ToolLoopProvider`, `runtime.ProviderPolicy`,
`runtime.ToolLoopRequest`, `runtime.ToolLoopResponse`, `runtime.ToolDefinition`,
`runtime.TokenUsage`, `runtime.EventEmitFunc`, `runtime.AgentProfile*`,
`runtime.Config` into new packages (`internal/provideriface`,
`internal/agentprofile`, etc.).

**Postcondition:** `internal/runtime/` still compiles, re-exports from new packages. `go build ./...` passes.

### State 2: Rewire providers (2 pts, Low)

Update `internal/provider/bridge.go` and `internal/gatewayruntime/provider.go`
to import from new packages instead of `internal/runtime`.

**Postcondition:** providers no longer depend on `internal/runtime` for interfaces. `go build ./...` passes, `go test ./internal/provider/...` passes.

### State 3: Extract tool registry and API handlers (3 pts, Medium)

Move `runtime.ToolRegistry`, `runtime.NewAPIHandler`, `runtime.RegisterRoutes`
into `internal/toolregistry` and `internal/apihandler`.

**Postcondition:** HTTP handlers and tool registry no longer in `internal/runtime`. `go build ./...` passes, API tests pass.

### State 4: Build actor-based runtime adapter (5 pts, Medium)

Create `internal/actorruntime` package that adapts `internal/actor.Runtime`
to the same surface that `cmd/sandbox/main.go` expects (`New`, `LoadConfig`,
`RuntimeOption`, etc.).

**Postcondition:** actorruntime compiles, provides same surface as old `runtime.New`. Unit tests pass, `go build ./cmd/sandbox` passes with actorruntime.

### State 5: Rewire cmd/sandbox/main.go (2 pts, Low)

Replace `runtime.New()` with `actorruntime.New()`. Keep old runtime import
temporarily for config loading if needed.

**Postcondition:** sandbox binary uses actor runtime for concurrency. Sandbox starts, health endpoint responds, basic run submission works.

### State 6: Migrate wire pipeline (8 pts, High)

Move `wire_*.go` files and `tools_wire_processor.go`, `tools_coagent.go` to
run on top of actor runtime. These files contain wire logic (not concurrency
logic) but are entangled with old runtime types. Extract the logic, adapt to
actor runtime's `Handler` interface and `Update` message type.

**Note:** 3b added `qdrant_dedup.go`, `sourcecycled_web_captures.go` changes,
and `wire_synthesis.go` changes. These must be accounted for during migration.

**Postcondition:** wire pipeline runs through actor runtime. `sourcecycled → processor → VText → publish → edition → /api/universal-wire/stories` returns non-empty after a cycle.

### State 7: Delete old concurrency code (3 pts, Medium)

Delete `runtime.go`, `channels.go`, `tools_coagent.go`, and remaining
concurrency code from `internal/runtime/`. Keep extracted interfaces/types
in their new packages. Delete `internal/runtime/` entirely if nothing remains.

**Postcondition:** `internal/runtime/` does not exist or contains only non-concurrency re-exports. `go build ./...` passes, `go test -race ./...` passes.

### State 8: End-to-end wire verification (3 pts, Medium)

Run the deployed acceptance test:
- sourcecycled dispatches
- platform sandbox accepts runs via actor runtime
- processor creates VText article revisions
- autonomous publish to corpusd
- edition transclusion
- `/api/universal-wire/stories` returns non-empty
- Universal Wire app renders article cards

**Postcondition:** wire works end-to-end on actor runtime. Staging acceptance proof with article cards visible.

## Execution Model

Hybrid (per plan doc §3.1):
- **Agent-driven:** States 1-3 (mechanical extraction), State 5 (single file), States 7-8 (cleanup + verify)
- **Human+agent:** State 4 (adapter design), State 6 (wire pipeline migration — highest entanglement)

## What NOT to Touch

- `internal/actor/actor.go` — the correct runtime, do not modify its protocol
- `specs/actor_protocol.tla` — TLA+ spec, update only if protocol shape changes
- `internal/objectgraph/` — O1 settled code
- `internal/qdrant/` — O2 settled code (schema, pipeline, client)
- `internal/sourcegraph/` — O3 settled code
- `internal/sources/` — source poller implementations
- `internal/cycle/` — cycle engine (3b changes)
- `cmd/sourcecycled/` — sourcecycled daemon (3b changes)

## Checklist

- [x] P1.1: Split AGENTS.md into 3 files
- [x] P1.2: Add 4 new rules (check-for-existing-fixes, root cause clustering, substrate-vs-symptom, dead-end escalation)
- [x] P1.3: Add deletion-first heuristic
- [x] P1.4: Simplify mutation class ceremony
- [x] Verify AGENTS.md revision: docs build passes, no broken cross-refs
- [x] State 1: Extract interfaces to new packages
- [x] State 2: Rewire providers to new packages
- [x] State 3: Extract tool registry (APIHandler deferred — 20+ methods on *Runtime)
- [ ] State 4: Build actor-based runtime adapter (skeleton created, full impl remains)
- [ ] State 5: Rewire cmd/sandbox/main.go
- [ ] State 6: Migrate wire pipeline to actor runtime
- [ ] State 7: Delete old concurrency code
- [ ] State 8: Staging E2E verification
- [ ] Update this document with evidence

## Acceptance

- AGENTS.md split into 3 files, 4 new rules active
- `internal/runtime/` concurrency code deleted
- `internal/actor/` is the production runtime
- Wire pipeline runs through actor runtime
- `go build ./...` passes
- `go test -race ./...` passes
- Staging: sourcecycled → processor → VText → publish → article cards visible

## Parallax State

status: open_handoff

mission conjecture: if the actor runtime is wired into production and the old
borked runtime is deleted, the wire pipeline runs on a correct concurrency
substrate, eliminating the class of bugs (lost wakes, check-then-act races,
no backpressure) that caused two weeks of misdirected debugging, and the
AGENTS.md revision prevents the next substrate-level failure from producing
the same patch loop.

deeper goal (G): a production system running on a verified concurrency
substrate, with agent operating rules that recognize substrate-vs-symptom
distinctions and prevent incremental patch loops on broken substrates.

witness/spec (A/S): 8-state migration machine with verification gates,
AGENTS.md split + 4 new rules, staging E2E with article cards.

invariants / qualities / domain ramp (I/Q/D):
- I: Do not modify `internal/actor/actor.go` protocol, TLA+ spec, O1-O3
  settled code, source poller implementations, cycle engine, sourcecycled
  daemon
- Q: Each state verified before proceeding. No half-migrated system. `go test
  -race` required after States 4-7. Staging proof required for settlement.
- D: Local build + test → staging deploy → staging verification. Must reach
  staging.

variant (conjecture descent) V: count uncompleted states + Part 1 items.
V = 5 (States 4-8 remaining). Part 1 complete (commit 55ef75bb). States 1-3
complete (commit b98531cd). Last ΔV: -3 (States 1-3). Conjecture decided:
interface extraction via type aliases is clean and preserves backward compat;
ToolRegistry extraction is self-contained; APIHandler extraction deferred (20+
methods on *Runtime, beyond Medium complexity).

budget: 3-4 passes granted. 2 spent (Part 1, States 1-3). 1-2 remaining.
Solvency verdict: INSOLVENT for full settlement. States 4-8 require separating
3797 lines of intertwined business logic and concurrency code. State 4 alone
is equivalent in complexity to State 6 (8 pts). State 8 requires staging deploy
access. Open handoff with clear next-move.

authority / bounds: may modify `AGENTS.md`, `docs/agent-*.md` (new files),
`internal/runtime/` (extraction + eventual deletion), `internal/provideriface/`
(new), `internal/agentprofile/` (new), `internal/toolregistry/` (new),
`internal/apihandler/` (new), `internal/actorruntime/` (new),
`internal/provider/bridge.go` (import changes),
`internal/gatewayruntime/provider.go` (import changes),
`cmd/sandbox/main.go` (rewire to actorruntime). May push to main (triggers CI
+ staging deploy). May not touch `internal/actor/actor.go` protocol, TLA+
spec, O1-O3, source pollers, cycle engine, sourcecycled daemon.

mutation class / protected surfaces: Orange/Red — interface extraction is
orange (refactor, no behavior change), sandbox rewire is red (production
entry point change), wire pipeline migration is red (production behavior
change on new substrate), old runtime deletion is red (removing production
code path). Protected: `internal/actor/actor.go`, TLA+ spec, O1-O3, source
pollers, cycle engine, sourcecycled.

rollback path: each state is a commit. States 1-3 are pure refactors
(revertible independently, commits 55ef75bb and b98531cd). States 4-6 change
behavior (revert as group if State 6 fails). State 7 deletion is irreversible
but only done after State 6 verifies no remaining imports.

conjecture delta / heresy delta:
- `discovered`: the adapter (State 4) is not a thin wrapper — it requires
  separating business logic from concurrency code in 3797 lines of runtime.go.
  The old *runtime.Runtime has 71+ methods; the actor runtime has 5. Building
  the adapter is equivalent in complexity to State 6, not a separate "Medium"
  task. This is a major finding that changes the mission's execution model.
- `introduced`: none expected (wiring existing verified runtime)
- `repaired`: the substrate class of bugs (lost wakes, check-then-act races,
  no backpressure) will be repaired by running on the correct runtime — but
  only after States 4-7 are complete

position / live conjectures / open edges:
- Part 1 done. States 1-3 done. Interface types extracted to provideriface +
  agentprofile. ToolRegistry extracted to toolregistry. Providers rewired.
  All via type aliases preserving backward compat. go build ./... passes.
  go test -race ./internal/actor/... passes. Provider tests pass.
- `internal/actorruntime/adapter.go` skeleton created — provides New()
  signature matching old runtime.New, creates actor.Runtime placeholder.
  Full implementation requires business logic extraction from old runtime.
- KEY FINDING: State 4 (adapter) and State 6 (wire migration) are the same
  class of work — both require separating business logic from concurrency
  code. The plan's separation of States 4 and 6 underestimated the
  entanglement. The old runtime's `startRunAsync` → `executeActivation` →
  `executeWithToolLoop` flow is the core execution path that must be
  reimplemented on the actor model.
- Open edge: APIHandler extraction (State 3 partial) — APIHandler calls 20+
  methods on *Runtime. Must be addressed in State 7 when old runtime is
  deleted. Options: (a) define a RuntimeAPI interface, (b) move APIHandler
  to actorruntime package, (c) keep APIHandler in a residual runtime package.
- Open edge: State 8 requires staging deploy access and E2E verification.

next move (for resuming agent):
1. Read this paradoc, the plan doc
   (docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md), and
   internal/actorruntime/adapter.go
2. Study the old runtime's executeActivation flow (runtime.go:1521+) and
   executeWithToolLoop (runtime.go:1581+) to understand the business logic
   that must be preserved
3. Implement the adapter by:
   a. Creating an actor.Log implementation backed by the store
   b. Creating an actor.Handler that runs the tool loop
   c. Wiring StartRun → actor.Send, CancelRun → actor.Evict
   d. Delegating read methods (GetRun, ListRunsByOwner) to the store directly
4. Rewire cmd/sandbox/main.go to use actorruntime.New() (State 5)
5. Migrate wire_*.go to actor runtime (State 6) — the wire pipeline's
   processor/reconciler logic should become actor handlers
6. Delete old concurrency code (State 7)
7. Staging E2E (State 8)

ledger file: docs/mission-3c-actor-runtime-migration-v0.ledger.md
version / lineage: v0, successor to mission-3b (settled)
learning state: retained here / promoted outward / successor links
settlement: open_handoff — Part 1 + States 1-3 complete, States 4-8 remain.
The key finding (adapter complexity = State 6 complexity) changes the
execution model: States 4 and 6 should be treated as one unified effort,
not separate passes.

## Suggested Goal String

```text
Use Parallax on docs/mission-3c-actor-runtime-migration-v0.md. Status:
open_handoff. Part 1 (AGENTS.md revision) and States 1-3 (interface extraction)
are complete. V=5 (States 4-8 remaining). Budget: 1-2 passes remaining.

KEY FINDING: State 4 (adapter) and State 6 (wire migration) are the same class
of work — both require separating business logic from concurrency code in 3797
lines of runtime.go. The old *runtime.Runtime has 71+ methods; the actor runtime
has 5. Treat States 4 and 6 as one unified effort.

Next move: implement internal/actorruntime/adapter.go by (a) creating an
actor.Log backed by the store, (b) creating an actor.Handler that runs the
tool loop, (c) wiring StartRun → actor.Send, CancelRun → actor.Evict, (d)
delegating read methods to the store. Then rewire cmd/sandbox/main.go (State 5),
migrate wire_*.go (State 6), delete old concurrency code (State 7), staging E2E
(State 8).

DO NOT TOUCH: internal/actor/actor.go protocol, specs/actor_protocol.tla,
O1-O3 (objectgraph, qdrant, sourcegraph), source pollers, cycle engine,
sourcecycled daemon. Verify: go build ./..., go test -race ./internal/actor/...
after States 4-7, staging acceptance after State 8. Exit: settled when V=0
(all 12 items done, staging produces article cards on actor runtime).

Ledger: docs/mission-3c-actor-runtime-migration-v0.ledger.md
```
