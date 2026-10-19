# The guest event appender keeps every projection payload in memory forever (2026-10-19)

Status: found by code review (Opus design review, docs/reports/tape-volume-
forward-fix-and-surgery-2026-10-19.md, A0), confirmed by reading the code.
The size per day is an estimate until measured.
Mutation class of the fix: orange (runtime memory; no change to the tape,
event digests, or projection results).

## Evidence

- `ComputerEventAppender.AppendNewNonPrivatePayload` stores a copy of every
  payload it appends: `a.livePayloads[payloadDigest] = append([]byte(nil), payload...)`
  (`internal/computerevent/appender.go:391-394`).
- The map is read in one place, `resolveProjectionBatch`
  (`appender.go:1012`), so `finalizeProjection` can decode the batch without
  reading it back from the host. If the entry is missing it resolves the
  payload through the artifact reader (`appender.go:1013-1035`).
- Nothing deletes from the map (`git grep livePayloads`: four lines, no
  `delete`). Every projection batch the process ever appended stays on the
  heap until the runtime restarts.
- `projection_batch_recorded` is 99.6% of the paper's events, one per
  durable guest write (docs/reports/projection-event-volume-design-
  2026-10-19.md). At the tape's measured byte rate the review estimates about
  0.7 GB a day of retained payloads on the paper (hypothesis: not measured
  from a heap profile).

## Why it matters

The paper's guest has 8 GiB. Its Firecracker process sat at 7.9 GB on Node
B at 21:45. A runtime that is up for days grows by the payloads it wrote;
planned restarts hide it. This may be part of the guest memory pressure
recorded on 2026-10-01 and 2026-10-09 (embedded Dolt memory-starved; guests
raised to 16 GiB, then 8 GiB): a hypothesis to check with the counter below.

## Fix

Drop the entry once the append has returned without error: by then
`finalizeProjection` has run (`appender.go:690`). After a failed append the
entry stays, because the prepared-event recovery paths (`appender.go:738`,
`appender.go:816`) may finalize again and the cache saves them a read.
Failure modes to pin in a test: the entry survives a successful append;
the batch is not applied when the cache is gone; a second append of the same
payload bytes is not finalized.
