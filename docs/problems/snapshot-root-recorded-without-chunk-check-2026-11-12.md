# A snapshot root can be recorded for chunks the collector already deleted

Date: 2026-11-12. Status: open, not yet reproduced; the hibernate path (a capture spanning a pause) is closed by `lane/restart-cancels-capture`, which cancels an in-flight capture before the pause and keeps new ones from starting while the fence is closed (unmerged); a missing chunk check at registration is not addressed. Found by review (seat 1,
Gemini 4 Hydrogen) of `lane/snapshot-loop`, read from the code.

## Problem

`HandleSnapshotRoot` and `RecordSnapshotRoot`
(`internal/platform/snapshot_roots.go:67-152`) accept a snapshot root without
checking that the chunks it names exist. The file-sync path does check
(`HandleFileCASRoot`, `internal/platform/file_cas_http.go:129-135`, refuses
with 409 through `missingFileChunk`). Recording a root also prunes the
computer's older roots down to two.

A capture uploads its chunks first and records the root last. Unrooted chunks
are collected once their last claim is older than `fileCASChunkUploadGrace`
(24 hours). If the gap between upload and root exceeds the grace, the root is
recorded for chunks that are gone, an older good root is pruned, and
`snapshot_recorded` goes on the tape. The newest recovery base is then not
restorable, and nothing says so.

## How the gap can exceed the grace

- A computer hibernated mid capture. `prepareHibernate`
  (`internal/agentcore/hibernate.go:354-366`) reads `SnapshotInFlight` but
  does not refuse on it, and the guest's monotonic clock stops while
  hibernated, so the capture's budget does not expire. On wake the capture
  finishes and records its root.
- A budget set at or above the grace.

## Belief

With the grace at 24 hours this needs a hibernation of more than a day in the
middle of a capture, so it is unlikely to have happened. It is why the grace
was not shortened to 3 hours in the snapshot loop fix: at 3 hours an ordinary
overnight hibernation would be enough.

## Remaining error

Not reproduced. Whether any recorded root on Node B names a missing chunk is
unchecked; a read-only audit of the live roots against the chunk store would
settle it.

## Fix direction

Before the grace is shortened (storage mission, station S4): refuse a root
whose chunks are missing, refresh the chunks' claim time under the collector's
lock when a root is recorded, and either refuse hibernation while a capture
is in flight or abort the capture on wake.
