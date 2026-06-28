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

---

## Post-Phase 1 Repair: H030 — Actor Runtime Database Polling (2026-06-27)

**Discovery:** During worktree cleanup audit, found that the actor runtime
implementation (`internal/actor/actor.go`) was database-polling instead of
using Go-channel mailboxes. The loop called `log.Unprocessed` every iteration
with zero `chan` declarations. A vestigial `pending []Update` slice existed
but was cleared and ignored. This contradicted the design doc ("The database
remembers. Go delivers.") and was the third occurrence of this heresy (H030).

**Impact on Phase 1:** Phase 1's "done" status was conditional on this
repair. The handler (`internal/actorruntime/handler.go`) was correct — it
checks run state before acting (idempotent) — but the delivery substrate
underneath it was wrong. The handler worked despite the broken substrate
because it is idempotent and the database-polling eventually delivered every
message. But the performance characteristics were wrong (DB query per loop
iteration instead of instant channel delivery) and the architecture was wrong
(database as delivery mechanism, not Go channels).

**Repair applied:**
- `residentActor.pending []Update` → `residentActor.mailbox chan Update`
  (buffered Go channel, default capacity 256)
- `Send` does non-blocking channel send when warm (select with default for
  overflow)
- `loop` restructured: cold-start log replay → channel drain → warm select
  with idle timer → post-drain overflow catch → passivation
- Skip set prevents double processing (messages in both channel and log)
- `processOne` checks skip before processing (found by critical review)
- Skip set persists for activation lifetime (found by critical review)
- Snapshot saved under lock during passivation
- Added `MailboxCapacity` and `IdleTimeout` to `Options`
- Adapter sets `MailboxCapacity: 256`, `IdleTimeout: 30s`

**Verification:**
- All 8 actor tests pass (`go test ./internal/actor/ -v -count=1`)
- Both adapter tests pass (`go test ./internal/actorruntime/ -v -count=1`)
- Full build compiles (`go build ./...`)
- Two independent reviews: first found skip-set race (fixed), second found
  processOne missing skip check (fixed), third verified SHIP

**Heresy delta:** discovered 2026-06-27, introduced original 3c_2
implementation, repaired 2026-06-27.
**Evidence:** [docs/memo-actor-runtime-database-polling-heresy-2026-06-27.md](./memo-actor-runtime-database-polling-heresy-2026-06-27.md),
H030 in [docs/choir-doctrine.md](./choir-doctrine.md).

---

## Post-H030 Comprehensive Test Repair (2026-06-28)

**Discovery:** Deep review subagent found that the H030 repair and the broader
Phase 1/2 deletions (old timer-based wake system, in-memory ChannelManager)
left three tests in `internal/runtime/` referencing deleted symbols. These
tests are behind the `//go:build comprehensive` tag and were not caught by
default `go test` runs.

**Broken tests found:**
1. `TestScheduleTextureWorkerWakeLeadingCoalesce` (texture_test.go:1652) —
   referenced deleted `textureWakeKey`, `rt.textureWakeMu`,
   `rt.textureWakePending[key].timer`. The old timer-based debounce system
   was replaced by `scheduleTextureWorkerWake` sending an actor message via
   `dispatchActor`. Coalescing is now natural via actor mailbox + park-resume.
2. `TestHandleTopologyReportsOrchestrationShape` (api_test.go:4375) —
   referenced deleted `rt.ChannelManager().Channel()`. The topology handler
   already returns `ChannelCount: 0` with a comment noting in-memory channels
   are deleted.
3. `TestConcurrentWorkers_IndependentChannels` (concurrent_workers_test.go:242)
   — referenced deleted `rt.ChannelManager().Channel(id)` for channel existence
   check. The core assertion (message independence via `ChannelPost`/
   `ChannelRead`) is still valid and store-backed.

**Repair applied:**
- Deleted `TestScheduleTextureWorkerWakeLeadingCoalesce` (replaced with a
  comment explaining why the test no longer exists).
- Removed `ChannelManager().Channel()` calls from
  `TestHandleTopologyReportsOrchestrationShape`; updated expected
  `ChannelCount` from 2 to 0.
- Removed `ChannelManager().Channel()` existence check from
  `TestConcurrentWorkers_IndependentChannels`; kept the `ChannelPost`/
  `ChannelRead` verification that proves message independence.

**Verification:**
- `go test -tags comprehensive ./internal/runtime/ -run "^$" -count=1` —
  compiles clean (no undefined symbols).
- `go test -tags comprehensive ./internal/runtime/ -run "TestHandleTopologyReportsOrchestrationShape|TestConcurrentWorkers_IndependentChannels" -v` —
  both PASS.
- `go test ./internal/actor/ ./internal/actorruntime/ -v` — all 10 tests PASS.
- `go vet ./internal/runtime/` — clean.

**Mutation class:** yellow (test-only changes, no runtime behavior change).
**Edge class:** closed (independent review found the issue, fix verified).
