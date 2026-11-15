# Carry by reference: three review rounds without convergence

Written 2026-11-15T15:34Z. Design: `docs/design/carry-by-reference-2026-11-15.md` on
`lane/t1-carry-design` (tips d21c5cab7, 9d56fbc0e, 5411f0d5c). No code was
written. Station T1 of
`docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md`.

## Problem

A new run copies its source run's conversation into fresh run-memory
entries. The design replaced the copies with one reference row naming the
source run and entry range. Three rounds with two seats (DeepSeek v4.1
flash, Sonnet; the Gemini seat answered 429 each time):

| Round | DeepSeek | Sonnet | What the blocking findings were about |
| --- | --- | --- | --- |
| 1 | REPAIR (6) | REPAIR (4) | retirement deleting a named origin; restore and fork without the origin; old binaries; a fallback that hid failure; one dangling reference blocking every wake |
| 2 | REPAIR (4) | REPAIR (2) | where resolved rows sit; sequence numbers; the copy writer stated three ways; owner and agent unchecked; the new op never selected |
| 3 | REPAIR (2) | ACCEPT | the store floor is lost when a checkpoint restore replaces the workspace, so an older binary is admitted and wedges; the retention stop gate names a census field that has no bytes |

Every earlier finding was confirmed closed by the next round. Each round
found new ones of the same severity on a surface the last round had not
reached. The round 3 blocker is concrete and cites code
(`internal/store/event_index_trim.go:85-98`,
`internal/agentcore/rematerialize.go:243-261`), so by the rule that a
reproducible minority blocker outweighs a majority pass, the design is not
accepted, and by the three-round rule it is not patched again.

## Evidence

The paper's own counters since its 10:31Z boot, read at 2026-11-15T15:34Z
(`telemetry_batching`, read-only):

- tape bytes: 182.7 MB
- carried message bytes: 23.5 MB in 201 carries, which is **12.8% of tape
  bytes**. The 55.9% in the goal file is the carried share of *message*
  bytes, and message bytes are a little over a quarter of the tape.
- `choir.run` object bytes: 39.0 MB by object kind (42.0 MB as a family),
  **21 to 23% of tape bytes**
- `choir.event` object bytes: 31.6 MB, 17%
- `choir.texture_document` family: 24.9 MB, 14%

## Belief

The findings did not converge because a reference makes one run's rows
depend on another run's lifetime, and run lifetime is touched by five
separate mechanisms that each had to learn about it: retirement, the
tape op format with its old-binary floor, restore and fork, residue
import, and every reader of run memory. The design grew to six pieces on
red surfaces with a one-way store floor, and each review reached one more
interaction. That is a property of the frame, not of the drafting.

The saving does not justify that surface. Carry is 12.8% of the tape, and
retention of whole origin runs (unmeasured) is subtracted from it. The run
record is nearly twice as large and its weight sits in one growing list
(`texture_available_source_entities`, 42 to 81 KB in a completed Texture
run on d@e), which is a local change to one object.

## Changed approach

1. T1 is parked. `lane/t1-carry-design` stays as the record; nothing is
   built from it.
2. T2 goes first. The run-write counter (`lane/t2-counter-land`) says
   which writes and fields carry the run record's bytes on the paper; the
   mechanism is chosen from that reading (claim C101).
3. If carry is taken up again, the frame to try is different: the
   conversation belongs to the agent's thread, not to the run, so a new
   run appends to the same thread and nothing is carried or referenced.
   That removes the cross-run lifetime instead of managing it. It is a
   redefinition of what run memory is keyed by and needs its own design;
   it is not started.
4. `choir.event` at 17% of tape bytes has no station. It gets a read
   (which events, how large, written how often) before anything is
   proposed.

## Remaining error

The 12.8% is five hours of one day on one computer. The retention offset
was never measured. The thread frame is untested against the retirement
rule that keeps the newest two runs of an agent. The round 3 floor finding
may also apply to the existing retire-op floor after a checkpoint restore;
that was not checked.
