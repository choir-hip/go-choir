# Desk worker count follows agents, not desk classes

Found 2026-10-21 while reading the desk worker pool for SDC ARCH-06
(docs/security-density-architecture-checklist-2026-10-21.md; station
docs/definitions/choir-launch-fp10-arch-06-2026-10-21.md). Evidence class:
code reading at main d1c60ad87, plus RES-17's measured idle worker RSS
(lane/fp10-res17-reaper 6d04b4651). Not measured on a live computer.

## What happens

`desk_go_eval` (internal/agentcore/tools_desk.go) gives each agent its own
worker process: `deskSessionKey` keys a desk run's session by `agent:<id>`,
and a cell with no run record (a `choir.Research` child) by its activation.
The session (the REPL's values) and the slot (the process) are one object,
so a process can only be given back by ending the agent's session.

1. **No bound on busy workers.** `acquire` spawns a worker for every key it
   has not seen. Only idle sessions are bounded (`deskSessionMaxIdle`, 12).
   Twenty agents with a cell in flight are twenty processes.
2. **Research children spawn and kill a worker each.** `runResearchChild`
   builds a fresh pool per child (`newResearchChildDeskWorkers(childKey)`)
   and releases it when the child returns. A tree of N children spawns N
   processes, up to `researchChildConcurrency` (4) at once, and reuses none.
3. **The idle bound is global, across desk classes.** `reclaimLocked` evicts
   the least recently used idle session computer-wide, so a burst of
   research runs can close management's or Texture's session.
4. **A slot cannot be reclaimed without losing the session.** Because the
   two are one object, density and session continuity trade directly.

## Why it matters

The checklist's goal is many agents per user, app-agent desks, and RLM
trees that scale. Each app desk adds an agent and each agent adds a worker
(about 20 MiB idle after RES-17, 81 MiB before). Worker count should be
bounded by (tenant, desk class, taint class, tree), with agents as records
whose state lives on the tape, not in a process.

What makes a cheaper slot affordable: PD2's deployed receipt (S1d-PD2,
2026-10-11) found desks did not reuse earlier REPL values; each run re-read
its records and searched again. Losing a REPL cache on eviction costs little
today; losing the agent's records would cost a lot, and nothing here touches
them.

## Belief and test

Belief: on a busy computer the peak live worker count is set by agent count
and research fan-out, not by desk class count. Test: the observe slice in
the station file records per-class live, busy and idle counts and the peak,
on staging, before any cap is enforced.

## Fix direction

The station file's design: a slot-class key read from FP2's desk manifest,
observe first, then per-class caps with a bounded wait and a retryable
refusal, then sibling slot reuse within a research tree. It builds on FP2
(the manifest, per-desk uid, slot-bound frames) and RES-17 (the reaper),
which both edit the pool, so it waits for them to land.
