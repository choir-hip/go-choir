# Root-Cause Cluster: The Wrong-Path Class

Date: 2026-09-22
Status: clustering assessment — documented before fix, per the
problem-documentation-first invariant. Triggered by the owner's `texture tell`
correction and the four-scout wrong-path sweep.
Mutation class of this document: green (analysis; no runtime change).

## The root cause

Choir has **two ways to cause an effect**:

1. **The canonical path** — append an event to the tape; a reducer projects
   state; a dispatcher derives the continuation from `pending = addressed
   events − incorporated events`. Durable, derivable, survives restart.
2. **The wrong path** — an out-of-band side channel: a direct call, a detached
   goroutine, a process-local timer/wake, a separate input queue, a sweep that
   enumerates state, or a write that bypasses the event append.

Every wrong-path instance is the same defect: **its continuation is not
derivable from the tape**, so it strands on restart and lies by omission
(run-terminal ≠ fate-terminal). `texture tell` is one instance — an owner
directive admitted through a side table and a handler-created wake instead of
a document-edit event. The roster driver is another — an external CLI driving
work through that side channel instead of an in-cell sub-RLM cast.

The ratified ontology cutover
(`docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md`) is the
substrate fix: it makes the canonical path the *only* path. This document
enumerates the wrong-path instances so the cutover deletes the **class**, not
one symptom.

## The cluster, by sub-class

### (a) Out-of-band input channels — input that isn't a document/tape event

| Instance | Path | Status vs cutover |
|---|---|---|
| `/tell` + `/correct` + `LifecycleOwnerInstruction` | `internal/textureowner/texture_owner_instruction.go`, `internal/types/owner_instruction.go`, `internal/store/lifecycle_owner_instruction.go` | **Subsumed** — but the tell payload carries arbitrary directive text `/revisions` doesn't preserve; needs a typed owner-intent payload on the canonical doc event before deletion |
| `/revise` lifecycle alias → forwards to `/tell` | `internal/textureowner/texture_agent_revision.go:55-121` | Subsumed; delete after callers migrate |
| `/revise` unbound legacy worker-mailbox | `texture_agent_revision.go:234-309` | Subsumed; strands unbound revisions until migrated |
| `cmd/choir/roster.go` + `texture tell\|correct` CLI | `cmd/choir/roster.go`, `cmd/choir/main.go:1109-1136` | Subsumed; migrate roster to in-cell casts first |
| Self-dev terminal-Super re-wake (synthetic tell) | `internal/agentcore/selfdev_texture_join.go:236-271` | Subsumed |
| Internal channel casts | `internal/agentcore/api.go:725-774`, `channel_store.go` | Subsumed (durable but non-tape) |
| `run start` prompt-bar | `cmd/choir/main.go:1688-1718`, `prompt_bar.go` | **NOT a defect** — canonical durable start; keep |

### (b) Process-local continuations & wakes — progress owned by a live process

| Instance | Path | Strands |
|---|---|---|
| Detached `Complete`/fate commit (`context.WithoutCancel`) | `rlm_reduce.go:400-425` | assignment saga mid-fate (`revoke_requested`) |
| CoSuper fate watchdog (`time.AfterFunc` 5m) | `cosuper_assignment_fate.go:855-893` | non-terminal fate sagas |
| Persistent-Super fresh-mint + reactivation watchdogs | `super_controller.go:646-853` | pending Super runs |
| Activation-budget `context.AfterFunc` terminalize | `runtime.go:249-283` | stalled activations |
| Actor Go-channel mailbox + drainer + idle/retry timers | `actor/actor.go:200-605` | unprocessed actor occurrences |
| In-memory child-completion coalescer | `actor/coalesce.go:46-171` | parent `WakeBatch` continuation |
| Synchronous child/revision polling (500ms loop) | `tools_coagent.go:99-183` | parent activation (violates cast-only) |
| Self-dev materializer `go reconcile...` | `api_self_development.go:1012,1368` | accepted/rollback-pending ops |
| sourcecycled ticker queue-drain | `cmd/sourcecycled/main.go:167-223` | queued source handoffs |
| vmctl idle/pressure/retention sweeper | `vmctl/ownership.go:739-798` | VM lifecycle actions — **possible authority-boundary exception** (host control plane, not Choir tape) |

All subsumed by the cutover's derivable-continuation + `not_before` due-index
model. The vmctl sweeper is the one judgment call: if host resource lifecycle
is inside the canonical domain it's subsumed; if it's a separate host control
plane it's an explicit boundary exception, not a defect.

### (c) Dual-path duplicates — old path live beside the in-cell carrier

| Instance | Keeper | Wrong path | Blocker |
|---|---|---|---|
| `actuator=tools` (R8) | RLM `capsule_go_eval` + staged `choir.*` intents | legacy JSON capsule ops (`tools_capsule.go`) | management/research desks not yet crossed |
| `update_coagent` JSON | staged `choir.Message` → `commitMessageIntent` | direct `newUpdateCoagentTool` (`tools_worker_update.go`) | Super/Researcher/Processor/Reconciler still registered |
| `report_to_texture` JSON | typed staged outcome | `tools_cosuper_assignment.go:167-337` | persistent-Super registry still installs it |
| Super substrate (R10) | common event-driven desk kernel | `super_controller.go` separate controller | still authoritative for self-dev/assignment admission |
| `install_frontend_pointer` host SPA | per-computer guest `ComputerSurface` | `ci.yml:1162-1186` host-global pointer | platform-shell deploy contract (separate migration) |
| Unbound `ChoirScope.Message` fallback | bound-cell staged intent | `yaegikernel/choir.go:204-226` direct `BrokerRequest` | one-shot/tools compatibility |
| Lifecycle vs legacy worker-update queue | `QueueLifecycleUpdate` | `DispatchWorkerUpdate` (`store.go:2847-2968`) | persisted rows + unmigrated profiles |

### (d) Sweep / enumerate-state recovery — recovery that scans instead of consuming minted continuations

~14 sweeps, all subsumed by the dispatcher's pending-projection (pending =
tape events − actor state head). The largest:

- Boot passivation / lifecycle rewarm / terminal-outcome repair / mailbox +
  work-item + spawned-work sweeps — `internal/agentcore/runtime.go:2025-2745`
- CoSuper trajectory/deadline/stranded-fate reconcilers —
  `cosuper_assignment_fate.go:293-520,959-993`
- Texture subject/channel/run + pending-instruction + passivated-run sweeps —
  `texture_controller.go:48-881`
- Parked-mailbox snapshot recovery — `actorruntime/adapter.go:500-608`
- Self-dev rewake + materialization state scans — `selfdev_texture_join.go`,
  `self_development_materializer.go`, `selfdev/operations.go`
- Open wire-publication recovery — `wire_publication.go:21-38`
- Actor `Sweep`/`AgentsWithBacklog` — `actor/actor.go:327-343`

**Transform, don't just delete:** the dispatcher's pending projection *is* a
scan — but it's the one canonical consumer, derived from the tape, not an
independent repair authority. Event-backed reducer commands
(`QueueLifecycle*`, `SetCoSuperCapsuleDisposition`, `CancelCoSuperAssignment`)
are **keepers**; only their scan/timer *callers* are the defect.

### (e) Non-event mutations — writes that bypass the append

Direct `UpdateRun`/`UpdateWorkItem`/`UpdateTrajectory`/`PutBatchConditional`
calls that mutate durable state without a canonical event — ~13 sites across
`runtime.go`, `super_controller.go`, `actorruntime/handler.go`,
`texture_controller.go`, `texture.go` (document title), `store/texture.go`
(legacy archive), `texture_handoff.go` (route metadata), `api_self_development.go`,
`selfdev/operations.go` (SQL fallback), `wire_publication.go`. All subsumed:
the transition becomes an event payload; the reducer projects it.

## What is NOT subsumed (needs a real decision, not just deletion)

1. **Tell payload semantics.** `/tell` carries arbitrary directive text;
   `/revisions` doesn't preserve it. Need a typed owner-intent payload on the
   canonical document event before the side channel can be deleted.
2. **Roster migration.** `roster.go` must move to in-cell sub-RLM casts before
   the tell path dies, or live roster input is stranded.
3. **`actuator=tools` / Super substrate.** Deletion is blocked until the
   management and research desks cross — these are the cutover's migration
   targets, not pre-cutover deletions.
4. **`install_frontend_pointer`.** The host-global SPA is the platform-shell
   deploy contract, not the computer surface — a separate migration (immutable
   platform-shell artifact), not part of the tape cutover.
5. **vmctl sweeper / sourcecycled ticker.** Possible authority-boundary
   exceptions — host control plane and source daemon may legitimately live
   outside the Choir tape. Decide explicitly rather than silently treating
   them as defects.

## The deletion rule

> Delete every `List*ByState` / `List*ByChannel` / `ListOpen*` /
> `ForEach*ByState` scan used to *cause* progress, every `time.AfterFunc` /
> `context.AfterFunc` / detached `go` continuation, every mailbox /
> owner-instruction / channel wake, and every direct `Update*` that mutates
> lifecycle-owned state. Retain only: event append, reducer projection, and
> the dispatcher's tape-derived pending/due-index. A scan may survive only as
> the dispatcher's internal projection — never as an independent trigger.

## Consequence for the stack

This cluster is the evidence that the ontology cutover (Phase 1d, ratified) is
the right substrate fix and that the standalone wake repair was correctly
folded into it. The cutover's scope is now concrete: it deletes sub-classes
(a)–(e) above, with the five exceptions called out. The carrier's roster gate
is re-scoped to the real harness (in-cell casts on the document channel) as
recorded in `current-situation-2026-09-22.md`.
