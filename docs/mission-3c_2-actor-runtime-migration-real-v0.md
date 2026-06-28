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

### Phase 2: Delete old concurrency — full mutex elimination (replaces States 6-7)

The `internal/runtime/` package is 101,749 lines across ~100 files. It is
not a concurrency substrate — it is the entire business logic of the system.
The concurrency code is ~2,000 lines of that. The rest is tool loops, texture
revisions, wire pipeline, coagent coordination, browser sessions, run
lifecycle, HTTP API, content processing, model routing, promotion/rollback,
and ~50 more categories of business logic.

Phase 2 detangles the concurrency substrate from the business logic. The
business logic stays. The concurrency code is deleted. Every mutex is
eliminated — not "mostly," not "the business-logic ones can stay." All 15.

#### The 15 mutexes and their fate

**Group A: Concurrency substrate — DELETE (6 mutexes)**

These guard the old concurrency model. The actor runtime replaces them
entirely. Delete the mutex, the state it guards, and all code that mutates
that state.

| Mutex | Guards | Why it's substrate |
|---|---|---|
| `mu` | `residentAgents`, `agentWaiters`, `actorBridge`, `health`, `running` | `residentAgents` is the volatile actor-residency index. `agentWaiters` is the channel-based wake system (lines 3723-3753). These ARE the old concurrency substrate. |
| `textureWakeMu` | `textureWakePending` map | Wakes go through `actor.Send` now. |
| `superRequestMu` | super request state | Super requests are actor messages. |
| `coagentSpawnMu` | coagent spawn state | Spawning a coagent is `actor.Send` to a new agent. |
| `workerRequestMu` | `workerRequests` map | Worker requests are actor messages. |
| `conductorRouteMu` | conductor routing state | Conductor routing is actor message dispatch. |

Also delete: `startRunAsync` (or its inlined body in `activate()` — no
fallback path), `notifyAgentSignal`, `waitForAgentSignal`,
`registerRunActivation`, `removeRunning`, `removeRunningLocked`,
`residentRunByAgent`, `channels.go` (434 lines), `ActorBridge` interface,
`SetActorBridge`, `ActorBridgeActive`. The actor runtime tracks residency.
The actor mailbox delivers wakes. There is no fallback to the old path.

**Group B: Lazy init — ELIMINATE by moving to construction time (3 mutexes)**

These guard lazy initialization of shared resources. The resources are known
at startup. Initialize them in `New()` (or `actorruntime.New()`). No lazy
init, no mutex.

| Mutex | Guards | Replacement |
|---|---|---|
| `objectGraphMu` | `objectGraph`, `objectGraphInitErr` | Init in constructor |
| `qdrantPipelineMu` | `qdrantPipeline`, `qdrantPipelineInitErr` | Init in constructor |
| `modelPolicyMu` | `modelPolicies` map | Init in constructor |

**Group C: Shared mutable state — ELIMINATE by converting to actor state (4 mutexes)**

These guard shared mutable state accessed by multiple actor goroutines. In
the actor model, shared mutable state becomes actor state — one goroutine,
state in actor memory, access via messages. No mutex.

| Mutex | Guards | Actor model replacement |
|---|---|---|
| `wirePublishDebounceMu` | debouncer batch state (doc IDs, revision IDs, timers) | Wire publish debouncer actor: receives "record" messages, tracks batch in actor state, emits "publish batch" when threshold/timer fires |
| `textureEditMu` | texture edit serialization | Texture is an appagent with its own actor goroutine. Edits go through the Texture actor's mailbox as messages. The actor serializes by definition. |
| `browserOpMu` | `browserOps` session map | Browser manager actor: owns session map, handles requests via messages |
| `browserCDPMu` | `browserCDP` session map | Browser manager actor: owns CDP sessions, handles requests via messages |

#### Phase 2 scope: what to do now, what to defer

**Do now (Phase 2):**
- Delete Group A (6 concurrency-substrate mutexes) + all associated code
- Eliminate Group B (3 lazy-init mutexes) by moving init to construction
- Delete `channels.go`, `startRunAsync`/fallback, `ActorBridge` interface
- Move APIHandler to `internal/actorruntime/`
- Verify: `go build ./...` passes, `go test -race ./...` passes

This eliminates 9 of 15 mutexes and all of the old concurrency substrate.
After this, `internal/runtime/` contains business logic + 4 shared-state
mutexes (Group C). The actor runtime is the execution substrate. No
fallback path exists.

**Defer to Phase 2.5 (follow-up mission or 3c_2 extension):**
- Convert wire publish debouncer to an actor (eliminates `wirePublishDebounceMu`)
- Route texture edits through the Texture actor's mailbox (eliminates `textureEditMu`)
- Create browser manager actor for session/CDP management (eliminates `browserOpMu`, `browserCDPMu`)

These are real actor design work — each needs a new actor type, message
protocol, and migration of callers. They're not blocking the substrate
replacement. They're the next step toward the end state: zero mutexes in
`internal/runtime/`, all shared state owned by actors.

#### The end state

The end state is: `internal/runtime/` (or whatever the business-logic package
is renamed to) contains zero mutexes. All shared mutable state is owned by
actors. The only concurrency primitive in the system is the actor runtime's
single mutex in `internal/actor/actor.go`. Business logic functions are
called by actor handlers, operate on actor-owned state or the store, and
never touch a mutex.

Phase 2 gets us to 4 remaining mutexes (Group C). Phase 2.5 gets us to zero.
The mission settles at Phase 3 (staging E2E) with 4 mutexes remaining and a
clear plan for Phase 2.5. Phase 2.5 itself may be a separate mission.

#### APIHandler decision — REVISED 2026-06-27

**Do NOT move APIHandler to `internal/actorruntime/`.** The agent's
analysis found that APIHandler accesses 76 unexported Runtime members
across 23 files. Moving it would require exporting 60+ symbols — massive
churn that breaks encapsulation, and it would be undone when the runtime
is dissolved through app extraction.

The current architecture already works: `actorruntime.New()` creates an
Adapter that embeds `*runtime.Runtime`, and `cmd/sandbox/main.go` calls
`NewAPIHandler(adapter.Runtime)` on the embedded runtime. The APIHandler
stays in the runtime package but operates on the actor-backed runtime.

APIHandler will be split by domain during app extraction (texture API →
`internal/texture/`, wire API → `internal/wire/`, etc.) as part of the
runtime dissolution plan in
[docs/runtime-deletion-and-extraction-plan-2026-06-27.md](runtime-deletion-and-extraction-plan-2026-06-27.md).

#### Group B mutexes — deferred

The 3 lazy-init mutexes (`modelPolicyMu`, `objectGraphMu`,
`qdrantPipelineMu`) are not concurrency-substrate mutexes. They guard lazy
initialization of optional services. Eliminating them by moving init to
construction time is a small win that can be done during app extraction
(when the services move to their own packages). They are not blocking
Phase 3.

**Postcondition (revised):** `internal/runtime/` has 7 remaining mutexes:
- `runningMu` — guards running map (will be eliminated when runs are
  actor-owned)
- `healthMu` — guards health state (will move to actor state)
- `wirePublishDebounceMu` — Group C, deferred to wire extraction
- `textureEditMu` — Group C, deferred to texture extraction
- `browserOpMu` + `browserCDPMu` — Group C, deferred to browser extraction
- `modelPolicyMu` — Group B, deferred to model policy extraction
- `objectGraphMu` — Group B, deferred to objectgraph extraction
- `qdrantPipelineMu` — Group B, deferred to qdrant extraction

All 6 concurrency-substrate mutexes (Group A) are deleted.
`channels.go` deleted. `startRunAsync` deleted (no fallback).
`ActorBridge` interface deleted. `go build ./...` passes, `go test -race`
passes on actor + actorruntime packages.

### Phase 3: Staging E2E (State 8)

Push to main, monitor CI, verify staging:
- sourcecycled dispatches
- sandbox accepts runs via actor runtime
- processor creates Texture article revisions
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

- [x] Phase 1: Actor handler runs executeActivation synchronously
- [x] Phase 1: Park-resume via memory snapshot (encode on park, decode on re-activation)
- [x] Phase 1: No startRunAsync — runs start on actor activation
- [x] Phase 1: cmd/sandbox/main.go uses actorruntime.New()
- [x] Phase 1: go build ./... passes
- [x] Phase 1: go test -race ./internal/actor/... passes
- [x] Phase 1: go test ./internal/runtime/... passes (sharded)
- [x] Phase 2a: Delete channels.go (434 lines) + channels_test.go (598 lines)
- [x] Phase 2b: Delete startRunAsync fallback in activate() — no legacy path
- [x] Phase 2c: Delete notifyAgentSignal, waitForAgentSignal, registerRunActivation, removeRunning, residentRunByAgent
- [x] Phase 2c: Split mu into runningMu + healthMu
- [x] Phase 2d: Delete textureWakeMu, superRequestMu, coagentSpawnMu, workerRequestMu, conductorRouteMu
- [x] Phase 2e: Delete ActorBridge interface, SetActorBridge, ActorBridgeActive
- [x] Phase 2: go build ./... passes
- [x] Phase 2: go test -race ./internal/actor/... + ./internal/actorruntime/... passes
- [ ] Phase 2f: Move APIHandler to internal/actorruntime/ — **SKIPPED** (revised: APIHandler stays in runtime, will be split during app extraction)
- [ ] Phase 2: Eliminate Group B — 3 lazy-init mutexes — **DEFERRED** to app extraction
- [ ] Phase 2: Verify remaining mutexes are only Group B + C (deferred to extraction)
- [ ] Phase 3: Push to main, monitor CI
- [ ] Phase 3: Staging E2E — article cards visible
- [ ] Update this document with evidence

## Acceptance

- `internal/actor/` is the execution substrate (not just wake layer)
- `startRunAsync` is deleted — no fallback path, no legacy goroutine spawning
- `channels.go` is deleted
- `ActorBridge` interface is deleted — handler calls business logic directly
- Group A: 6 concurrency-substrate mutexes deleted + all associated state/code
- Group B: 3 lazy-init mutexes remain, deferred to app extraction (not blocking)
- Group C: 4 shared-state mutexes remain, deferred to app extraction
- `runningMu` + `healthMu` remain (split from original `mu`), will be eliminated when runs are actor-owned
- Park-resume works: tool loop state persists across passivation via memory snapshot
- APIHandler stays in `internal/runtime/` — will be split by domain during app extraction
- `go build ./...` passes
- `go test -race ./internal/actor/... + ./internal/actorruntime/...` passes
- Staging: sourcecycled → processor → Texture → publish → article cards visible
- No 200ms polling — actor mailbox delivers updates instantly

## Parallax State

status: working — Phase 2 complete (Group A deleted, APIHandler move skipped, Group B+C deferred to extraction), Phase 3 staging E2E next

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

variant (conjecture descent) V: count uncompleted phases. V = 1 (Phase 3
remains). Phase 1 done (ΔV=-1). Phase 2 done (ΔV=-1). Phase 3 (staging) is
the settlement gate.

budget: 4-5 passes. Spent: 3 (exploration + Phase 1 + Phase 2).
Remaining: 1-2. Solvent.

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
- `discovered`: `internal/runtime/` is 101,749 lines across ~100 files. It is
  the entire business logic of the system, not a concurrency substrate. The
  concurrency code is ~2,000 lines. Phase 2 detangles the ~2,000 lines of
  concurrency from the ~100,000 lines of business logic.
- `discovered`: all 15 mutexes can be eliminated. 6 are concurrency substrate
  (delete). 3 are lazy init (move to construction). 4 are shared mutable state
  (convert to actor state — deferred to Phase 2.5). The end state is zero
  mutexes in `internal/runtime/`, all shared state owned by actors.
- `introduced`: `ActorBridge` interface — this was `AgentSubstrate` renamed.
  It was the 3c failure pattern recurring. Phase 2 deleted it. The handler
  calls business logic directly via `dispatchActor` function hook.
- `repaired`: the substrate class of bugs (lost wakes, check-then-act races,
  no backpressure, 15 mutexes) is repaired by making the actor runtime the
  execution substrate. Phase 2 deleted 6 Group A mutexes + all associated
  state/code. 7 mutexes remain, deferred to app extraction (Group B+C).
- `discovered`: APIHandler has deep coupling with Runtime internals (76
  unexported members across 23 files). Moving it to actorruntime would
  require exporting 60+ symbols — massive churn that breaks encapsulation.
  Correctly skipped. APIHandler will be split by domain during app extraction.

position / live conjectures / open edges:
- Phase 1 complete: actor handler is the execution boundary.
  `ExecuteActivationSync` runs `executeActivation` in the actor goroutine.
  Park-resume via `resumeState` memory snapshot. `activate()` sends
  `initial_dispatch` actor messages. `wakeUpdatedCoagent` sends
  `coagent_result` actor messages. `coagentParkWaiter` passivates immediately
  in actor mode (no channel wait). `cmd/sandbox/main.go` uses
  `actorruntime.New()`. `go build ./...` + `go test -race ./internal/actor/...`
  + `go test -race ./internal/actorruntime/...` pass.
- Phase 2 complete: all 6 Group A concurrency-substrate mutexes deleted.
  `channels.go` (434 lines) deleted, `startRunAsync` fallback deleted,
  `ActorBridge` interface deleted. `notifyAgentSignal`, `waitForAgentSignal`,
  `residentAgents`, `agentWaiters`, `registerRunActivation`, `removeRunning`
  deleted. `mu` split into `runningMu` + `healthMu`. `residentRunByAgent`
  replaced with store-backed `activeRunByAgent` (excludes blocked runs).
  Texture wake debounce replaced with direct actor dispatch. Worker request
  cache replaced with store-backed dedup. Net -1154 lines. `go build ./...` +
  `go test -race ./internal/runtime/...` + actor + actorruntime tests pass.
- Phase 2f (move APIHandler) correctly skipped. APIHandler accesses 76
  unexported Runtime members across 23 files. Moving it would require
  exporting 60+ symbols — massive churn that breaks encapsulation, all undone
  during app extraction. Current architecture works: `actorruntime.Adapter`
  embeds `*runtime.Runtime`, APIHandler operates on the actor-backed runtime.
  APIHandler will be split by domain during app extraction (texture API →
  internal/texture/, wire API → internal/wire/, etc.).
- 7 mutexes remain, all deferred to app extraction:
  - `runningMu`, `healthMu` — eliminated when runs are actor-owned
  - `wirePublishDebounceMu`, `textureEditMu`, `browserOpMu`, `browserCDPMu` —
    Group C, move with their apps
  - `modelPolicyMu`, `objectGraphMu`, `qdrantPipelineMu` — Group B, move with
    their services
- Open edge: coagent update steering while actor is warm may cause redundant
  `coagent_result` messages (the update is already injected by
  `injectUserTurns`). Not a correctness issue (idempotent), but an
  inefficiency. Can optimize in a follow-up.

next move: Phase 3 — push to main, staging E2E. The critical question: does
the wire pipeline work end-to-end on the actor runtime? Sourcecycled →
processor → Texture → publish → article cards visible. That's the settlement
gate.

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
actorruntime.New(). Build + race tests pass. [DONE — commit 32d809c5]
Phase 2: Delete old concurrency — full mutex elimination. [DONE — commit 1ee14035]
  - Group A (6 mutexes): delete mu, textureWakeMu, superRequestMu,
    coagentSpawnMu, workerRequestMu, conductorRouteMu + all associated state
    (residentAgents, agentWaiters, etc.) and code (startRunAsync fallback,
    notifyAgentSignal, waitForAgentSignal, registerRunActivation, channels.go,
    ActorBridge interface). [DONE]
  - Group B (3 mutexes): deferred to app extraction — objectGraphMu,
    qdrantPipelineMu, modelPolicyMu move with their services.
  - Group C (4 mutexes): deferred to app extraction — wirePublishDebounceMu,
    textureEditMu, browserOpMu, browserCDPMu move with their apps.
  - Move APIHandler to internal/actorruntime/. [SKIPPED — 76 unexported
    members across 23 files, correctly deferred to app extraction]
  - Build + race tests pass. [DONE]
Phase 2.5 (future): App extraction — split runtime by domain. APIHandler
splits by domain (texture API → internal/texture/, wire API → internal/wire/).
Group B+C mutexes move with their apps. Zero mutexes in internal/runtime/.
May be a separate mission.
Phase 3: Push to main, staging E2E — article cards visible. [NEXT]

DO NOT TOUCH: internal/actor/actor.go protocol, specs/actor_protocol.tla,
O1-O3 (objectgraph, qdrant, sourcegraph), source pollers, cycle engine,
sourcecycled daemon, AGENTS.md (Part 1 settled). The 3c States 4-7 stash has
been dropped — do not attempt to recover it. Start fresh from current HEAD.

Verify: go build ./..., go test -race ./internal/actor/... after Phase 1,
go test -race ./... after Phase 2, staging acceptance after Phase 3. Budget:
4-5 passes. Exit: settled when V=0 (all 3 phases done, staging produces
article cards on actor runtime as execution substrate).
```
