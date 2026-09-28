# M11 blocker: Texture required-write loop starves the engineering desk op

**Status:** documented 2026-09-27; fix pending.
**Evidence:** staging, computer `computer-9d8257559c5761cf07502d2bfb54f184`,
op `selfdev-3c27040a8900fa53ebd58831c13887d2` (probe run r5, timed out after
60 min in `executing`); Node B journal 22:45–22:56 UTC.

## Observation

The selfdev probe's engineering desk operation never reached
`awaiting_approval`. The tape (canonical events) halts at seq 80
(`projection_batch_recorded`, 22:07:21) while the guest runtime kept working.
Between 22:45 and 22:56 the autoputer log shows an unbroken ~45-second loop:

```
runtime: run <id> → failed: tool loop: required write tool did not succeed after 2 retries
texture:24ac3db1…/1c468ab9… until <+45s> cause=actor: defer unprocessed
  occurrence: actorruntime: Texture activation returned without disposing
  exact trigger
```

A `request_source=update_coagent` Texture continuation wakes, fails the
required-write check twice (`runtime.go:3294-3299` demands a
`desk_go_eval` result containing `rlm:texture_apply:`), the run terminates
failed *without disposing the occurrence*, and the dispatcher redelivers it
~45 s later — indefinitely. This is the desk receiving a worker-update
continuation it never completes: the model either isn't calling
`choir.ApplyTexture` in its cell or the receipt isn't carrying the marker.

## Structural failure (substrate, not prompt)

Two bugs compound:

1. **No disposal/backoff on required-write failure.** An occurrence whose
   activation fails the write contract is deferred and redelivered forever
   at fixed cadence. There is no max-attempts, exponential backoff, or
   terminal disposition — a broken trigger becomes a permanent spin.
2. **Starvation under a stuck trigger.** The retried Texture activation
   consumes the desk cell budget every cycle; the engineering assignment
   (separate desk, separate capsule) sits `executing` for an hour because
   the operation's next transition never schedules.

The probe's `awaiting_approval` timeout is the *symptom*; the loop is the
substrate defect. This class of bug (wake → fail → no disposition → redeliver)
is exactly what `actor_wake_outbox` + `MarkActorWakeProjected` was built to
fence — the `request_source=update_coagent` continuation path evidently
doesn't ride it.

## Root cause — CONFIRMED 2026-09-28 via retained-computer run memory

Pulled the texture desk's run memory out of the retained computer's Dolt store
(guest `data.img` → `state.texture/texture`, table `run_memory_entries`,
loop `0a098ce1-0110-4cc6-a009-1798cfc46e9f`). The model **does** call
`choir.ApplyTexture` — every turn contains `desk_go_eval` tool calls with
well-formed `{"op":"apply","base_revision_id":...}` bodies.

The cells never execute. Every result is:

```
{"diag":"compile","error":"2:8: choir/_.go redeclared in this block"}
```

The desk cell is a persistent yaegi `Session` where the model is prompted to
write full `package main` + `import "choir"` programs. The second cell's
`import "choir"` collides with the first cell's import — yaegi compiles each
cell as `_.go` in the same package and re-importing a package path is a hard
redeclaration error. `Reuse: preserve` keeps the worker alive, so the model
retries the same program shape every redelivery → infinite compile-fail loop.
Locally reproduced with `yaegikernel.NewSession`: cell2 re-importing `fmt`
fails `fmt/_.go redeclared`.

Second defect in the same session semantics: a `func main` defined in any
cell re-executes on every subsequent cell (yaegi treats `main` as the
entrypoint per cell). So even a fragment-only model can trigger repeated
side effects if a prior cell left `main` behind.

Third defect, the RLM-design violation: `update_coagent` payloads are inlined
into chat (`buildCoagentUpdateUserMessages` marshals the full packet JSON
into a user turn) while the REPL — where the model must commit —
has no variable holding them. `choir.Inbox()` reads the Dolt channel log
only; update_coagent records live in the lifecycle-control object store.
Context is a chat message, not a REPL variable — inverted from the RLM
substrate (prompt-as-variable in the REPL, pointer-only in the context
window).

Fourth defect: the desk system prompts never state the cell contract.
`texture.yaml`/`run_system.yaml` describe authoring philosophy but never say
"cells are fragments on a persistent session; `import` each package at most
once; no `func main`; the activation's terminal write is
`choir.ApplyTexture`". The management overlay says "fragments building on
persisted state" but doesn't name the redeclaration trap, so models keep
sending full programs.

## Fix direction (decided)

Upstream, per owner direction 2026-09-28:

1. **Normalize cell source before eval** in `Session.Eval`/`serveCell`:
   strip the `package` clause, drop already-imported import declarations,
   and rename `func main` → unique `__cell_main_N()` + emit one explicit call
   (preserves `return` semantics, kills the auto-rerun). Accepts both the
   fragment shape tests use and the full-program shape models actually write.
2. **Bind update_coagent records as a cell variable** (`choir.Updates()` /
   `SessionFrame.Updates`), populated from `pendingCoagentUpdatesForRun` at
   cell admission — the REPL becomes the single source of truth for the
   payload, matching RLM prompt-as-variable.
3. **Slim the injected wake turn** to `update_ids` + phase + the desk's
   terminal-write verb name — no payload JSON in chat. Keep the injected
   turn (dedupe still works on the injection-append receipt).
4. **Write the desk cell contract into each profile's system prompt**:
   fragments not programs, imports-once, no `func main`, named terminal verb.

The required-write check and redelivery/backoff pressure stay as-is for now:
with the compile trap gone and the payload reachable, `ApplyTexture` should
commit and the loop terminates naturally. If it still spins, dead-letter
disposal is the follow-up — but don't design it until the honest path works.
## Residual question for the trace drill-down

Whether the stuck engineering op is starved *by* this loop (resource

contention) or has its own defect — the next probe should run against a VM
without a live Texture continuation to isolate.
