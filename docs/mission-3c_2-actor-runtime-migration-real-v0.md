# Mission 3c_2: Actor Runtime Migration (Real)

**Status:** ready for execution  
**Date:** 2026-06-27  
**Umbrella:** `docs/mission-3-universal-wire-ingestion-rebuild-v0.md`  
**Predecessor:** `docs/mission-3c-actor-runtime-migration-v0.md` (Part 1 + States 1-3 done, States 4-8 failed)  
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
- **The `AgentSubstrate` interface concept** — NOT kept. The interface was
  defined in the stashed (failed) 3c code, not in committed code. It does not
  exist at current HEAD. It was designed for the adapter-on-top-of-old-runtime
  pattern, which 3c_2 rejects. The handler-as-execution-boundary pattern does
  not need a substrate interface between actor runtime and old runtime — the
  old runtime's concurrency code is being deleted, not interfaced with. The
  adapter calls business logic functions directly.

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

## Actor Handler Semantics (Read Before Implementing)

The previous attempt (3c) failed because it did not understand how the tool
loop's park-wait maps to the actor's activate-passivate cycle. Read this
section carefully. If you do not understand it, STOP and ask the operator
before writing code. Implementing the wrong semantics will produce a
deadlock or a polling workaround, both of which are failures.

### The tension

The old runtime and the actor model have different assumptions about handler
lifetime:

- **Old runtime:** `startRunAsync` spawns a goroutine that runs the entire
  tool loop — potentially for hours. When the tool loop needs a coagent
  result, it parks (blocks on a channel). The goroutine stays alive, holding
  the LLM conversation context in memory. When the coagent responds, the
  channel is signaled, the goroutine wakes, the tool loop continues.

- **Actor model:** `HandleUpdate` is called once per update. It processes
  the update and returns. The actor loop then moves to the next update. When
  there are no more updates, the actor passivates. The handler is expected to
  be short-lived — process one message, return, process the next.

These collide at the park point. The tool loop wants to block and wait. The
actor model wants the handler to return.

### What NOT to do (deadlock)

If the handler blocks inside HandleUpdate waiting for a coagent response:

```
1. Actor loop calls HandleUpdate(initial_dispatch)
2. Handler starts tool loop
3. Tool loop dispatches to coagent via rt.Send()
4. Coagent processes and sends response via rt.Send()
5. Response lands in THIS actor's pending mailbox + durable log
6. Tool loop parks — handler blocks, waiting for coagent response
7. DEADLOCK: handler is waiting for a response that is in the pending
   mailbox, but the actor loop can't deliver it because HandleUpdate
   hasn't returned
```

The handler is waiting for a message it can only receive by returning. This
is a structural deadlock. The 3c attempt hit this, didn't understand why,
and worked around it with `startRunAsync` + 200ms store polling. That
workaround is a failure — it preserves the old runtime as the execution
substrate.

### What to do (save and resume)

```
1. Actor loop calls HandleUpdate(initial_dispatch, memory=nil)
2. Handler decodes memory (nil = fresh start)
3. Handler loads/creates RunRecord from store
4. Handler starts tool loop
5. Tool loop makes LLM calls, executes tools
6. Tool loop dispatches to coagent via rt.Send()
7. Tool loop needs to park (wait for coagent response)
8. Handler encodes resume state into memory:
   - run ID
   - tool loop phase ("parked_coagent")
   - park reason (which coagent, what request)
9. Handler RETURNS from HandleUpdate
10. Actor loop checks for more backlog — nothing yet
11. Actor passivates (saves memory snapshot)

...time passes, coagent works...

12. Coagent sends response via rt.Send()
13. Response lands in durable log + actor activates
14. Actor loop calls HandleUpdate(coagent_response, memory=saved_snapshot)
15. Handler decodes memory → resume state
16. Handler loads RunRecord + conversation history from store
17. Handler checks: does this update match the park reason?
18. Yes — inject the coagent response as the tool result
19. Tool loop resumes from where it parked
20. If tool loop parks again → goto step 8
21. If tool loop completes → handler returns, memory cleared
```

### Memory is the resume pointer, not the full state

The store already holds the conversation history, tool call results, and run
metadata. The actor's `memory` snapshot does NOT need to hold the entire
conversation. It only needs a tiny resume pointer:

```go
type resumeState struct {
    RunID       string
    Phase       string    // "parked_coagent" | "executing" | "llm_call"
    ParkReason  string    // which coagent update we're waiting for
    ParkRequest string    // the request we sent
}
```

Maybe 200 bytes. The handler serializes this to JSON, returns it as
`memory`. On re-activation, deserializes it, loads the full state from the
store, and resumes.

### The handler contract, stated plainly

1. **HandleUpdate is called once per incoming update, not once per run.** A
   single run may involve many HandleUpdate calls — one for the initial
   dispatch, one for each coagent response, one for each external trigger.

2. **The handler returns when it can't make further progress without a new
   update.** When the tool loop parks, the handler returns. When the tool
   loop completes, the handler returns. When the tool loop is waiting for an
   LLM response, the handler blocks (LLM calls are synchronous I/O, not
   actor messages — the handler holds the goroutine during the API call).

3. **Memory is the resume pointer, not the full state.** The store holds the
   conversation, tool results, and run metadata. Memory holds just enough to
   know where to resume.

4. **The handler distinguishes update kinds.** `Update.Kind = "initial_dispatch"`
   starts a run. `Update.Kind = "coagent_result"` resumes a parked tool loop.
   `Update.Kind = "cancel"` aborts. The handler switches on kind.

5. **The actor goroutine is the run goroutine.** There is no separate
   `startRunAsync` goroutine. The actor's goroutine runs the tool loop inside
   HandleUpdate. When HandleUpdate returns, the goroutine goes back to the
   actor loop (checks for more updates, passivates if none).

6. **Coagent updates arrive as actor messages, not channel signals.** When a
   coagent sends a result via `rt.Send()`, it lands in the durable log. The
   actor loop queries `Unprocessed()` and finds it. The handler receives it
   as the next HandleUpdate call. No polling. No channels. No 200ms.

### Circuit breaker

If you (the executing agent) reach a point where:
- the handler deadlocks (blocks waiting for a message it can't receive), OR
- you are considering a polling workaround (checking the store on a timer), OR
- you are considering spawning a separate goroutine for the tool loop, OR
- you are considering keeping `startRunAsync` in any form, OR
- you do not understand how the tool loop resumes after passivation,

**STOP. Do not write code. Do not attempt a workaround.** Write a paragraph
explaining what you are trying to do and why the semantics above don't seem
to cover it. Ask the operator for guidance. Implementing the wrong semantics
will produce a system that looks like it works but preserves the old runtime
as the execution substrate — which is the exact failure mode of 3c.

## Migration Plan

### Phase 1: Actor handler as execution boundary (replaces States 4-5)

Create `internal/actorruntime/handler.go` that implements `actor.Handler`:

```
HandleUpdate(ctx, agentID, update, memory):
    1. Decode agentID → ownerID, agentName
    2. Decode memory → run state (tool loop context, LLM conversation,
       tool call stack, pending park state)
    3. Load or create the RunRecord for this agent
    4. Call executeActivation-equivalent logic SYNCHRONOUSLY
    5. The tool loop runs inside this goroutine
    6. When the tool loop parks (waiting for coagent updates):
       a. Encode run state into memory (the []byte snapshot)
       b. Return from HandleUpdate — the actor passivates
    7. When a new update arrives, the actor re-activates:
       a. Handler receives the new update + saved memory
       b. Decode memory → reconstructed run state
       c. Resume the tool loop from the park point, not from scratch
```

The key change: `executeActivation` runs inside `HandleUpdate`, not in a
goroutine spawned by `startRunAsync`. The actor goroutine IS the run goroutine.

**Critical: park-resume via memory snapshot.** The handler must persist run
state in the actor's `memory` (`[]byte`) before returning on park, and
reconstruct it on re-activation. Without this, the tool loop restarts from
scratch on every coagent update, losing the LLM conversation context and
tool call history. The `memory` parameter in `HandleUpdate` is the mechanism
for this — encode the tool loop state (conversation history, pending tool
calls, park reason) into memory before passivation, decode it on re-activation.
This is not optional; the wire pipeline depends on multi-turn tool loops that
park for coagent updates and resume with the results.

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

**APIHandler decision: move to `internal/actorruntime/`.** The 3c mission
noted that APIHandler calls 20+ methods on `*runtime.Runtime` and deferred
its extraction. Phase 2 must resolve this. The APIHandler is a thin HTTP
layer over runtime methods — move it to `internal/actorruntime/` and rewire
its method calls to the adapter. This is the cleanest cut: no new interface
abstraction, no residual package. The adapter owns the methods that APIHandler
calls. If some methods are pure business logic (no concurrency), they can
stay in a residual `internal/runtime/` package as free functions or a
business-logic struct with no mutexes.

**Postcondition:** `internal/runtime/` contains only business logic (run
creation, tool loop execution, wire synthesis, texture reconciliation). No
concurrency management. APIHandler lives in `internal/actorruntime/`.
`go build ./...` passes, `go test -race ./...` passes.

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
- [ ] Phase 1: Park-resume via memory snapshot (encode on park, decode on re-activation)
- [ ] Phase 1: No startRunAsync — runs start on actor activation
- [ ] Phase 1: cmd/sandbox/main.go uses actorruntime.New()
- [ ] Phase 1: go build ./... passes
- [ ] Phase 1: go test -race ./internal/actor/... passes
- [ ] Phase 1: go test ./internal/runtime/... passes (sharded)
- [ ] Phase 2: Delete startRunAsync, channels.go, old concurrency mutexes
- [ ] Phase 2: Wire pipeline runs through actor handlers
- [ ] Phase 2: APIHandler moved to internal/actorruntime/
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
- Park-resume works: tool loop state persists across passivation via memory snapshot
- APIHandler lives in `internal/actorruntime/`, not `internal/runtime/`
- `go build ./...` passes
- `go test -race ./...` passes
- Staging: sourcecycled → processor → VText → publish → article cards visible
- No 200ms polling — actor mailbox delivers updates instantly

## Parallax State

status: working — Phase 1 complete, Phase 2 next

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
  startRunAsync in production. No polling. `go test -race` required. Staging
  proof required.
- D: Local build + test → staging deploy → staging verification.

variant (conjecture descent) V: count uncompleted phases. V = 2 (Phases 2-3
remain). Phase 1 done (ΔV=-1). Phase 3 (staging) is the settlement gate.

budget: 4-5 passes. Spent: 2 (exploration + Phase 1 implementation).
Remaining: 2-3. Solvent.

authority / bounds: may modify `internal/runtime/` (business logic extraction
+ concurrency deletion), `internal/actorruntime/` (adapter + handler + log),
`cmd/sandbox/main.go` (rewire). May push to main. May not touch
`internal/actor/actor.go`, TLA+ spec, O1-O3, source pollers, cycle engine,
sourcecycled, AGENTS.md.

mutation class / protected surfaces: Red — replacing production execution
substrate. Protected: `internal/actor/actor.go`, TLA+ spec, O1-O3, source
pollers, cycle engine, sourcecycled, AGENTS.md.

rollback path: Phase 1 commit is the critical change — revert to revert to
pre-Phase-1 commit (a453d299) if it fails. Phase 2 deletion is irreversible
but only done after Phase 1 verifies. Phase 3 is staging verification.

conjecture delta / heresy delta:
- `discovered`: 3c's attempt revealed that the adapter pattern (embedding
  *runtime.Runtime and delegating) preserves the old concurrency instead of
  replacing it. The handler must be the execution boundary, not a dispatcher.
- `discovered`: the old runtime already has a park-resume mechanism for
  Texture actors (`passivateIdleToolLoopRun` + `reactivatePassivatedTextureRun`).
  The conversation history lives in the store (`runMemoryManager`), not in
  process memory. The actor's `memory` parameter only needs a tiny resume
  pointer (`{runID, phase}`). This generalized cleanly to the actor handler.
- `introduced`: `ActorBridge` interface — the seam between `internal/runtime`
  and `internal/actorruntime`. Avoids circular imports. The adapter implements
  it; the runtime calls it for activation and wake.
- `repaired`: the substrate class of bugs (lost wakes, check-then-act races,
  no backpressure, 15 mutexes) is repaired by making the actor runtime the
  execution substrate (Phase 2 completes this)

position / live conjectures / open edges:
- Phase 1 complete: actor handler is the execution boundary.
  `ExecuteActivationSync` runs `executeActivation` in the actor goroutine.
  Park-resume via `resumeState` memory snapshot. `activate()` sends
  `initial_dispatch` actor messages. `wakeUpdatedCoagent` sends
  `coagent_result` actor messages. `coagentParkWaiter` passivates immediately
  in actor mode (no channel wait). `cmd/sandbox/main.go` uses
  `actorruntime.New()`. `go build ./...` + `go test -race ./internal/actor/...`
  + `go test -race ./internal/actorruntime/...` pass. Legacy runtime tests
  still pass (startRunAsync fallback when ActorBridge is nil).
- Open edge: `startRunAsync` kept as legacy fallback for tests. Phase 2
  deletes it and fixes tests to use the actor path.
- Open edge: the 15 mutexes in `*runtime.Runtime` — some guard non-actor
  state (e.g., `objectGraphMu` for lazy init, `qdrantPipelineMu` for lazy
  init). These can stay. Others guard concurrency that the actor runtime now
  manages (e.g., `coagentSpawnMu`, `workerRequestMu`). These must be removed
  in Phase 2.
- Open edge: APIHandler calls 20+ methods on `*runtime.Runtime`. Phase 2
  moves APIHandler to `internal/actorruntime/` and rewires to the adapter.
- Open edge: `channels.go` (434 lines) still exists. Phase 2 deletes it.
- Open edge: coagent update steering while actor is warm may cause redundant
  `coagent_result` messages (the update is already injected by
  `injectUserTurns`). Not a correctness issue (idempotent), but an
  inefficiency. Phase 2 can optimize.

next move: Phase 2 — delete `startRunAsync`, `channels.go`, old concurrency
mutexes. Wire pipeline runs through actor handlers. Move APIHandler to
`internal/actorruntime/`. `go build ./...` + `go test -race ./...` pass.

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
(3797 lines, 15 mutexes) was still the execution substrate. 3c replaced
channel-based wake signals with 200ms polling. The migration was not done.

CRITICAL: Read the "Actor Handler Semantics" section of the mission doc
before writing any code. 3c failed because it did not understand how the tool
loop's park-wait maps to the actor's activate-passivate cycle. If you reach a
point where the handler deadlocks, or you are considering polling, or you
are considering spawning a separate goroutine, or keeping startRunAsync —
STOP and ask the operator. Do not attempt workarounds.

What 3c got right (keep): Part 1 (AGENTS.md revision, commit 55ef75bb) and
States 1-3 (interface extraction, commit b98531cd) are committed and correct.
Build on top of them.

What 3c got wrong (abandon): the AgentSubstrate interface concept. It was
designed for the adapter-on-top-of-old-runtime pattern. The
handler-as-execution-boundary pattern does not need it. The adapter calls
business logic directly.

What to do: make the actor handler the execution boundary. HandleUpdate must
run executeActivation SYNCHRONOUSLY — no startRunAsync, no separate goroutine.
The actor goroutine IS the run goroutine. When the tool loop parks (waiting
for coagent updates), encode a small resume pointer (run ID, phase, park
reason) into the memory parameter and return from HandleUpdate — the actor
passivates. When a new update arrives, the actor re-activates, the handler
decodes memory to get the resume pointer, loads full state from the store,
and resumes the tool loop from the park point. The actor mailbox replaces
polling — updates are delivered instantly via actor.Send.

Phase 1: Actor handler runs executeActivation synchronously with park-resume
via memory snapshot. Delete startRunAsync. Rewire cmd/sandbox/main.go to
actorruntime.New(). Build + race tests pass.
Phase 2: Delete channels.go, old concurrency mutexes. Wire pipeline runs
through actor handlers. Move APIHandler to internal/actorruntime/. Build +
race tests pass.
Phase 3: Push to main, staging E2E — article cards visible.

DO NOT TOUCH: internal/actor/actor.go protocol, specs/actor_protocol.tla,
O1-O3 (objectgraph, qdrant, sourcegraph), source pollers, cycle engine,
sourcecycled daemon, AGENTS.md (Part 1 settled). The 3c States 4-7 stash has
been dropped — do not attempt to recover it. Start fresh from current HEAD.

Verify: go build ./..., go test -race ./internal/actor/... after Phase 1,
go test -race ./... after Phase 2, staging acceptance after Phase 3. Budget:
4-5 passes. Exit: settled when V=0 (all 3 phases done, staging produces
article cards on actor runtime as execution substrate).
```
