# Mission 3c_2: Actor Runtime Migration (Real)

**Status:** ready for execution  
**Date:** 2026-06-27  
**Umbrella:** `docs/mission-3-universal-wire-ingestion-rebuild-v0.md`  
**Predecessor:** `docs/mission-3c-actor-runtime-migration-v0.md` (Part 1 + States 1-3 done, States 4-7 failed)  
**Plan doc:** `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md`  
**Successor:** `docs/mission-3d-distribution-v0.md` (to be drafted after 3c_2)

## What Went Wrong in 3c

The first attempt at States 4-7 (commit `73a86943`, stashed) took a wrong
turn. Instead of replacing the old runtime's concurrency substrate, it:

1. **Wired the actor runtime as a wake/dispatch layer on top of the old runtime.**
   The `actorruntime.Adapter` embedded `*runtime.Runtime` and delegated all
   business logic to it. The actor handler called `ReconcileActorWake`, which
   called the old `startRunAsync` → `executeActivation` → `executeWithToolLoop`
   path. The actor goroutine blocked on `ReconcileActorWake` returning, which
   returned immediately (async goroutine). The actor passivated while the run
   continued outside the actor model.

2. **Replaced channel-based wake signals with 200ms store polling.**
   The old `waitForAgentSignal` (instant, channel-based) was replaced with a
   200ms polling loop in `coagentParkWaiter`. This is a latency regression and
   a step backward from proper wake semantics.

3. **Did not delete the old runtime.** `runtime.go` was still 3750 lines with
   15 mutexes. `channels.go` (434 lines) still existed. `tools_coagent.go`
   still existed. All wire files still existed. State 7 was not done.

4. **The execution substrate was still the old runtime.** The 15 mutexes
   (`coagentSpawnMu`, `workerRequestMu`, `superRequestMu`, `textureEditMu`,
   `conductorRouteMu`, `browserOpMu`, `browserCDPMu`, `modelPolicyMu`,
   `objectGraphMu`, `qdrantPipelineMu`, etc.) were all still in play. The
   actor runtime only controlled wake/steer/passivate, not execution.

**Net result:** lost-wake bug class was fixed, but the borked concurrency
substrate was still the execution engine. The migration was not done.

## What 3c Got Right (Keep These)

- **Part 1: AGENTS.md revision** — committed (`55ef75bb`). Split into 3 files,
  4 new rules, deletion-first heuristic, simplified mutation ceremony. This is
  settled and correct.
- **States 1-3: Interface extraction** — committed (`b98531cd`).
  `internal/provideriface`, `internal/agentprofile`, `internal/toolregistry`
  extracted. Providers rewired. All via type aliases. Build passes.
- **The `AgentSubstrate` interface concept** — breaking the circular
  dependency between runtime and actorruntime is correct. The interface
  design was right; the implementation was wrong.

## Objective

Complete the actor runtime migration. The actor runtime
(`internal/actor/actor.go`, 326 lines, 1 mutex) must become the execution
substrate, not just a wake layer. The old runtime's concurrency code must be
deleted. The wire pipeline must run through actor handlers.

## The Core Insight

The actor runtime's `Handler.HandleUpdate` is the execution boundary. When an
actor activates, the handler processes updates (coagent dispatches) and runs
the tool loop. The actor goroutine stays resident for the entire duration of
execution — it does not passivate until the handler returns.

The old runtime's `startRunAsync` spawns a goroutine and returns immediately.
The actor handler must NOT call `startRunAsync`. Instead, the handler must
call the execution logic synchronously — `executeActivation` must run inside
the actor goroutine, not in a separate goroutine spawned by the handler.

This means:
- `startRunAsync` is deleted. Runs start when an actor activates.
- `executeActivation` / `executeWithToolLoop` become the handler's body (or
  are called by the handler).
- The actor's `pending` mailbox replaces `agentWaiters`. No polling.
- The actor's passivation (idle check in `loop()`) replaces the park-waiter
  mechanism. No 200ms polling.
- The 15 mutexes in `*runtime.Runtime` are either removed (actor runtime
  handles concurrency) or kept only for non-actor state (e.g., objectgraph
  lazy init, which is not actor-managed).

## Migration Plan

### Phase 1: Actor handler as execution boundary (replaces States 4-5)

Create `internal/actorruntime/handler.go` that implements `actor.Handler`:

```
HandleUpdate(ctx, agentID, update, memory):
    1. Decode agentID → ownerID, agentName
    2. Load or create the RunRecord for this agent
    3. Call executeActivation-equivalent logic SYNCHRONOUSLY
    4. The tool loop runs inside this goroutine
    5. When the tool loop parks (waiting for coagent updates), return from
       HandleUpdate — the actor passivates
    6. When a new update arrives, the actor re-activates and the handler
       resumes the run
```

The key change: `executeActivation` runs inside `HandleUpdate`, not in a
goroutine spawned by `startRunAsync`. The actor goroutine IS the run goroutine.

**Postcondition:** `cmd/sandbox/main.go` uses `actorruntime.New()`. Runs
execute inside actor goroutines. No `startRunAsync`. Build passes, tests pass.

### Phase 2: Delete old concurrency (replaces States 6-7)

Delete from `internal/runtime/`:
- `startRunAsync` — replaced by actor activation
- `notifyAgentSignal` / `waitForAgentSignal` — already deleted in 3c attempt
- `residentAgents` / `agentWaiters` — already deleted in 3c attempt
- `channels.go` — the old channel system, replaced by actor mailbox
- The 15 mutexes that managed concurrency (keep only those for non-actor
  state like lazy-init guards)

Move wire pipeline files (`wire_synthesis.go`, `wire_publication.go`,
`wire_reconciler_debounce.go`, `wire_platform_publish.go`,
`tools_wire_processor.go`, `tools_coagent.go`) to run as actor handlers or
business logic called by actor handlers. The wire logic stays — the
concurrency wrapper around it changes.

**Postcondition:** `internal/runtime/` contains only business logic (run
creation, tool loop execution, wire synthesis, texture reconciliation). No
concurrency management. `go build ./...` passes, `go test -race ./...` passes.

### Phase 3: Staging E2E (State 8)

Push to main, monitor CI, verify staging:
- sourcecycled dispatches
- sandbox accepts runs via actor runtime
- processor creates VText article revisions
- autonomous publish to platformd
- `/api/universal-wire/stories` returns non-empty
- Article cards visible on staging

**Postcondition:** wire works end-to-end on actor runtime.

## What NOT to Touch

- `internal/actor/actor.go` — the correct runtime, do not modify its protocol
- `specs/actor_protocol.tla` — TLA+ spec
- `internal/objectgraph/` — O1 settled code
- `internal/qdrant/` — O2 settled code
- `internal/sourcegraph/` — O3 settled code
- `internal/sources/` — source poller implementations
- `internal/cycle/` — cycle engine
- `cmd/sourcecycled/` — sourcecycled daemon
- AGENTS.md and the 3 split files from Part 1 — already settled

## Checklist

- [ ] Phase 1: Actor handler runs executeActivation synchronously
- [ ] Phase 1: No startRunAsync — runs start on actor activation
- [ ] Phase 1: cmd/sandbox/main.go uses actorruntime.New()
- [ ] Phase 1: go build ./... passes
- [ ] Phase 1: go test -race ./internal/actor/... passes
- [ ] Phase 1: go test ./internal/runtime/... passes (sharded)
- [ ] Phase 2: Delete startRunAsync, channels.go, old concurrency mutexes
- [ ] Phase 2: Wire pipeline runs through actor handlers
- [ ] Phase 2: go build ./... passes
- [ ] Phase 2: go test -race ./... passes
- [ ] Phase 3: Push to main, monitor CI
- [ ] Phase 3: Staging E2E — article cards visible
- [ ] Update this document with evidence

## Acceptance

- `internal/actor/` is the execution substrate (not just wake layer)
- `startRunAsync` is deleted — runs execute inside actor goroutines
- `channels.go` is deleted
- Old concurrency mutexes removed (only lazy-init guards remain)
- `go build ./...` passes
- `go test -race ./...` passes
- Staging: sourcecycled → processor → VText → publish → article cards visible
- No 200ms polling — actor mailbox delivers updates instantly

## Parallax State

status: ready

mission conjecture: if the actor handler becomes the execution boundary
(running executeActivation synchronously inside HandleUpdate), the actor
runtime replaces the old runtime's concurrency substrate entirely. Runs
execute inside actor goroutines, the mailbox replaces polling, and the 15
mutexes are removed. The wire pipeline runs on the correct substrate.

deeper goal (G): a production system where the only concurrency primitive is
the actor runtime's single mutex, with all business logic executing inside
actor goroutines.

witness/spec (A/S): actor handler running executeActivation, deleted
startRunAsync, deleted channels.go, go test -race passing, staging E2E.

invariants / qualities / domain ramp (I/Q/D):
- I: Do not modify `internal/actor/actor.go` protocol, TLA+ spec, O1-O3,
  source pollers, cycle engine, sourcecycled daemon, AGENTS.md (Part 1
  settled)
- Q: Actor handler is synchronous (blocks until run completes or parks). No
  startRunAsync. No polling. `go test -race` required. Staging proof required.
- D: Local build + test → staging deploy → staging verification.

variant (conjecture descent) V: count uncompleted phases. V = 3 (Phases 1-3).
Each completed phase decreases V by 1. Phase 3 (staging) is the settlement
gate.

budget: 3-4 passes. Phase 1 is 1-2 passes (high reasoning, execution boundary
redesign). Phase 2 is 1 pass (deletion + wire migration). Phase 3 is 1 pass
(staging). Allow extra for staging iteration.

authority / bounds: may modify `internal/runtime/` (business logic extraction
+ concurrency deletion), `internal/actorruntime/` (adapter + handler + log),
`cmd/sandbox/main.go` (rewire). May push to main. May not touch
`internal/actor/actor.go`, TLA+ spec, O1-O3, source pollers, cycle engine,
sourcecycled, AGENTS.md.

mutation class / protected surfaces: Red — replacing production execution
substrate. Protected: `internal/actor/actor.go`, TLA+ spec, O1-O3, source
pollers, cycle engine, sourcecycled, AGENTS.md.

rollback path: each phase is a commit. Phase 1 is the critical change — if it
fails, revert to pre-Phase-1 commit. Phase 2 deletion is irreversible but
only done after Phase 1 verifies. Phase 3 is staging verification.

conjecture delta / heresy delta:
- `discovered`: 3c's attempt revealed that the adapter pattern (embedding
  *runtime.Runtime and delegating) preserves the old concurrency instead of
  replacing it. The handler must be the execution boundary, not a dispatcher.
- `introduced`: none expected (wiring existing verified runtime)
- `repaired`: the substrate class of bugs (lost wakes, check-then-act races,
  no backpressure, 15 mutexes) is repaired by making the actor runtime the
  execution substrate

position / live conjectures / open edges:
- 3c Part 1 (AGENTS.md revision) and States 1-3 (interface extraction) are
  committed and correct. Build on top of them.
- 3c's States 4-7 attempt is stashed (`git stash`). Do NOT unstash. Start
  fresh from the current HEAD.
- The `AgentSubstrate` interface concept from 3c is correct — keep it. The
  implementation (adapter embedding runtime, handler calling
  ReconcileActorWake → startRunAsync) is wrong — replace it.
- Open edge: the handler must handle park-wait. When a run's tool loop parks
  (waiting for coagent updates), the handler should return from HandleUpdate
  (actor passivates). When a new update arrives, the actor re-activates and
  the handler resumes the run. This requires the handler to persist run state
  across activations.
- Open edge: `executeActivation` currently uses `rt.wg.Add(1)` and
  `defer rt.wg.Done()`. Inside an actor handler, the actor runtime manages
  the goroutine lifecycle — `rt.wg` should not be used for actor-managed runs.
- Open edge: the 15 mutexes in `*runtime.Runtime` — some guard non-actor
  state (e.g., `objectGraphMu` for lazy init, `qdrantPipelineMu` for lazy
  init). These can stay. Others guard concurrency that the actor runtime now
  manages (e.g., `coagentSpawnMu`, `workerRequestMu`). These must be removed
  or the actor runtime doesn't actually replace them.

next move: read `internal/actor/actor.go` (the actor runtime), `internal/runtime/runtime.go`
(lines 483-640: startRunAsync, executeActivation, executeWithToolLoop), and
`internal/runtime/super_controller.go` (reconcileUpdatedCoagentActor,
coagentParkWaiter). Then implement Phase 1: make the actor handler the
execution boundary.

ledger file: docs/mission-3c_2-actor-runtime-migration-real-v0.ledger.md
version / lineage: v0, successor to mission-3c (Part 1 + States 1-3 done,
States 4-7 failed)
learning state: retained here / promoted outward / successor links
settlement: open until Phase 3 staging verification produces article cards
on the actor runtime as execution substrate.

## Suggested Goal String

```text
Use Parallax on docs/mission-3c_2-actor-runtime-migration-real-v0.md. Mission:
complete the actor runtime migration that 3c failed to do. 3c wired the actor
runtime as a wake/dispatch layer on top of the old runtime — the old runtime
(3750 lines, 15 mutexes) was still the execution substrate. 3c replaced
channel-based wake signals with 200ms polling. The migration was not done.

What 3c got right (keep): Part 1 (AGENTS.md revision, commit 55ef75bb) and
States 1-3 (interface extraction, commit b98531cd) are committed and correct.
The AgentSubstrate interface concept is correct. Build on top of them.

What to do: make the actor handler the execution boundary. HandleUpdate must
run executeActivation SYNCHRONOUSLY — no startRunAsync, no separate goroutine.
The actor goroutine IS the run goroutine. When the tool loop parks (waiting
for coagent updates), return from HandleUpdate (actor passivates). When a new
update arrives, the actor re-activates and resumes the run. The actor mailbox
replaces polling — updates are delivered instantly via actor.Send.

Phase 1: Actor handler runs executeActivation synchronously. Delete
startRunAsync. Rewire cmd/sandbox/main.go to actorruntime.New(). Build + race
tests pass.
Phase 2: Delete channels.go, old concurrency mutexes. Wire pipeline runs
through actor handlers. Build + race tests pass.
Phase 3: Push to main, staging E2E — article cards visible.

DO NOT TOUCH: internal/actor/actor.go protocol, specs/actor_protocol.tla,
O1-O3 (objectgraph, qdrant, sourcegraph), source pollers, cycle engine,
sourcecycled daemon, AGENTS.md (Part 1 settled). Do NOT unstash the 3c
States 4-7 attempt (git stash). Start fresh from current HEAD.

Verify: go build ./..., go test -race ./internal/actor/... after Phase 1,
go test -race ./... after Phase 2, staging acceptance after Phase 3. Budget:
3-4 passes. Exit: settled when V=0 (all 3 phases done, staging produces
article cards on actor runtime as execution substrate).
```
