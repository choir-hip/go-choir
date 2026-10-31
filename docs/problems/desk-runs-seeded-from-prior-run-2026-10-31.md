# Every new desk run is seeded from the previous run's memory

Date: 2026-10-31. Status: recorded, parked until after the Mac app (owner).

## Problem

Owner, 2026-10-31: "this is wrong behavior. Each desk should start with
system prompt and repl only. This seed can be in repl but often isn't
relevant. Completed runs shouldn't be so relevant to the next run. But we
shouldn't fix it yet. Let's note this for after the Mac app."

## Evidence (code reading, main at 3d9039aa0)

- `internal/agentcore/run_memory.go` `initialize`: when a run has no memory
  entries, it calls `seedActorMemorySnapshot`.
- `seedActorMemorySnapshot` calls `store.LatestActorRunMemoryEntries` for the
  same owner, computer and agent id. That is the newest run in state
  completed or passivated. It builds a deterministic summary of that run's
  latest checkpoint plus the messages after it, and writes it into the new
  run as a compaction entry, reason `actor_rewarm` ("Actor memory snapshot
  from prior activation <run>").
- Texture runs with `actor_rewarm_carry_conversation` go through
  `carryActorConversation` instead: the earlier run's checkpoint and its
  messages are copied verbatim into the new run.
- This applies to every agent with a stable agent id, so every persistent
  desk. It is not a crash-recovery path: a resumed run (planned restart,
  passivation) continues its own log and is not seeded; a crash-closed run is
  never a seed source.

## Why it matters

- Context: each activation starts with the last run's conversation as its
  frame. The owner's design is a desk starting from the system prompt and
  the REPL, with earlier work available in the REPL to look up when it is
  relevant, not pushed into context.
- Storage: each seed writes a copy of the prior checkpoint text into the new
  run's memory, once per activation. Unmeasured.
- Correctness coupling: a damaged run that completes clean would propagate
  into the next run (the group-commit gap-2 defect, fixed in
  lane/tape-group-commit 0ae655674).

## Direction (not started)

A desk run starts with the system prompt and the REPL only. Prior runs'
memory becomes something the REPL can read on demand.

Texture is the exception, keyed by document (owner, 2026-10-31: "Texture
should resume prior runs if it's continuing the same doc"). A Texture run
that continues the same document resumes that document's prior run
conversation. A Texture run on a different document starts fresh. Today's
carry is keyed by agent (the newest run of the Texture agent, or the run
named by actor_rewarm_source_loop_id), not by document, so a Texture wake on
document B can carry document A's conversation. The fix keys the carry
source by document id.

Measure the seed bytes per activation before changing it.
