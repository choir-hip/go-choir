# Mission 3c_2 Ledger

## Pass 1 — exploration (V=3→3, no ΔV, observer evidence gained)

**Conjecture:** The old runtime already contains a park-resume mechanism
(`passivateIdleToolLoopRun` + `reactivatePassivatedTextureRun`) for Texture
actors that persists conversation state to the store and re-enters
`executeWithToolLoop` with `actor_reactivate_existing_memory=true`. If this
mechanism generalizes to the actor handler, then Phase 1 is a wiring task
(handler calls `executeActivation` synchronously + memory = resume pointer),
not a redesign of the tool loop.

**Verdict:** supported. The existing park-resume mechanism is exactly the
actor handler's resume path. The conversation history lives in the store
(`runMemoryManager`), not in process memory. The actor's `memory` parameter
only needs a tiny resume pointer (`{runID, phase}`). The tool loop re-enters
via `executeWithToolLoop` with the reactivate flag set, loads the persisted
conversation, and injects new coagent updates via `injectUserTurns`.

**Move:** probe (read actor.go, runtime.go, super_controller.go,
texture_controller.go, adapter.go, cmd/sandbox/main.go, log_sqlite.go).

**Findings:**
- `actor.HandleUpdate(ctx, agentID, u, memory) ([]byte, error)` is the
  execution boundary. `loop()` drains backlog, calls handler per update,
  passivates on idle.
- `startRunAsync` spawns a goroutine; `executeActivation` has
  `defer rt.wg.Done()` + `removeRunning`. For sync execution, need
  `rt.wg.Add(1)` to match, or a new entry point.
- `coagentParkWaiter` blocks on `waitForAgentSignal` (channel). In actor
  mode: return `Passivate: true` immediately when no pending updates.
- `wakeUpdatedCoagent` calls `notifyAgentSignal` (channel) +
  `reconcileUpdatedCoagentActor` (creates new run via startRunAsync). In
  actor mode: send actor message to target agent.
- `actor.SQLiteLog` exists, uses `modernc.org/sqlite`. Store uses Dolt
  (MySQL-compatible) — must use separate SQLite file for actor log.
- `runtime.Config = provideriface.Config`, `runtime.Provider =
  provideriface.Provider` (type aliases from States 1-3).
- `cmd/sandbox/main.go` calls `runtime.New()`, `rt.Start`,
  `rt.InstallDefaultAgentTools`, `rt.ToolRegistryForProfile`,
  `rt.EmitProductEvent`, `runtime.NewAPIHandler(rt)`.
- Uncommitted doc changes: `docs/mission-3c_2-...md` (this paradoc) and
  `docs/production-readiness-checklist.md` (unrelated observability
  philosophy refactor — preserve, do not mix into mission commits).

**Phase 1 approach (decided):**
1. Add `ActorBridge` interface to `internal/runtime` —
   `Send(ctx, agentID, kind, content, trajectoryID, fromAgentID) error`.
   When set on `*runtime.Runtime`, replaces `startRunAsync` (→
   `actorBridge.Send(..., "initial_dispatch", runID, ...)`) and
   `wakeUpdatedCoagent` (→ `actorBridge.Send(..., "coagent_result", ...)`)
   and `coagentParkWaiter` (→ immediate passivate).
2. Add `ExecuteActivationSync(ctx, rec)` on `*runtime.Runtime` —
   calls `executeActivation` in the caller's goroutine (the actor goroutine).
3. Implement `actorHandler` in `internal/actorruntime/handler.go` —
   decodes memory → resumeState, loads run, calls `ExecuteActivationSync`,
   checks `rec.State` (Passivated → save memory; Completed/Failed → clear).
4. Implement full `Adapter` — embeds `*runtime.Runtime`, creates actor
   runtime + SQLiteLog + handler, sets `ActorBridge`.
5. Rewire `cmd/sandbox/main.go` to `actorruntime.New()`.
6. Keep `startRunAsync` as legacy fallback (tests); delete in Phase 2.

**Edge class:** missing_oracle (no independent review yet).
**Open edges:** runtime tests may break if `startRunAsync` callers change;
will verify `go build ./...` + `go test -race ./internal/actor/...`.
**Receipt:** file reads above; no code changes yet.

## Pass 2 — Phase 1 construct (V=3→2, ΔV=-1, conjecture decided)

**Conjecture:** The actor handler can be the execution boundary by (a)
adding an `ActorBridge` interface to `internal/runtime` that the adapter
implements, (b) adding `ExecuteActivationSync` that runs `executeActivation`
in the caller's goroutine, (c) implementing `actorHandler.HandleUpdate` that
decodes a `resumeState` memory pointer, loads the run, and calls
`ExecuteActivationSync` synchronously, (d) modifying `coagentParkWaiter` to
passivate immediately in actor mode, and (e) rewiring `cmd/sandbox/main.go`
to `actorruntime.New()`.

**Verdict:** supported. All Phase 1 verification passes:
- `go build ./...` — passes
- `go test -race ./internal/actor/...` — passes
- `go test -race ./internal/actorruntime/...` — passes (2 tests:
  StartRunExecutesViaActorHandler, ActorBridgeActive)
- `go test -race ./internal/runtime/ -run TestRuntime` — passes (legacy
  fallback with ActorBridge nil)

**Move:** construct (implement Phase 1).

**Files changed:**
- `internal/runtime/runtime.go`: added `ActorBridge` interface,
  `actorBridge` field, `SetActorBridge`, `ActorBridgeActive`, `activate`,
  `ExecuteActivationSync`. Replaced 5 `startRunAsync` callers with
  `activate()`.
- `internal/runtime/super_controller.go`: modified `coagentParkWaiter` to
  passivate immediately in actor mode. Modified `wakeUpdatedCoagent` to use
  `actorBridge.Send` in actor mode. Added `ReconcileCoagentWake` exported
  method.
- `internal/actorruntime/handler.go`: new file implementing `actor.Handler`.
  Handles `initial_dispatch`, `coagent_result`, `cancel`. Park-resume via
  `resumeState` memory snapshot. Falls back to `ReconcileCoagentWake` when
  no parked run exists.
- `internal/actorruntime/adapter.go`: full implementation. Embeds
  `*runtime.Runtime`, creates actor runtime + SQLiteLog + handler, wires
  `ActorBridge`. Implements `Send`, `Start`, `Stop`.
- `internal/actorruntime/adapter_test.go`: integration tests.
- `cmd/sandbox/main.go`: rewired to `actorruntime.New()`.

**Key design decisions:**
- `ActorBridge` interface avoids circular imports (`internal/runtime` →
  interface, `internal/actorruntime` → implementation).
- `startRunAsync` kept as legacy fallback (tests use `runtime.New` directly
  without the bridge). Phase 2 deletes it.
- `ExecuteActivationSync` calls `registerRunActivation` + `rt.wg.Add(1)` to
  match `executeActivation`'s `defer rt.wg.Done()` + `removeRunning`. This
  is a concession to the existing code; Phase 2 removes it.
- Actor log uses separate SQLite file (`<storePath>-actor.db`) because the
  store uses Dolt (MySQL-compatible), not SQLite.
- `handleCoagentResult` falls back to `ReconcileCoagentWake` when no parked
  run exists (cold start / process restart).

**Edge class:** missing_oracle (no independent review yet).
**Open edges:** startRunAsync legacy fallback, 15 mutexes, channels.go,
APIHandler move — all Phase 2.
**Receipt:** `go build ./...`, `go test -race` outputs above.
