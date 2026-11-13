# A snapshot root can be recorded for chunks the collector already deleted

Date: 2026-11-12. Status: root-time verification fixed on
`lane/snapshot-root-verifies-chunks`, not yet landed; hibernation mid capture
and the shorter grace stay open (see "What the fix closes" below). Found by review (seat 1,
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

## What the fix closes (lane/snapshot-root-verifies-chunks)

- Registration (`RecordVerifiedSnapshotRoot`) stats every named chunk under
  the shared CAS reuse lock and answers 409 naming the first missing one,
  recording and pruning nothing. A computer holding a deletion tombstone is
  refused. The verified chunks enter an in-memory pending set before the lock
  is released; the database commit runs after, bounded by 30 seconds, so a
  slow commit does not queue event appends behind a waiting collector.
- The collector's unlink (`unlinkIfStillUnreachable`, exclusive lock) keeps a
  chunk that is pending, and asks the computer's live snapshot roots when a
  root committed since it classified (an in-process generation counter,
  bumped after each commit and before the pending set clears). The
  destroyed-computer purge skips the recheck: it deletes the roots with the
  chunks, behind a tombstone and an archive. The first version of this fix
  wedged that purge forever (review round 1); the purge fixture now names
  real chunks.
- `Store.recordSnapshotRoot` is unexported; production cannot record an
  unverified root.
- The guest turns the 409 into `snapshot.ErrSnapshotChunkMissing` with the
  host's reason; the failed attempt appends no `snapshot_recorded` and takes
  the existing backoff.

Measured (build VM, local Postgres, warm disk): registering a root of 10,500
chunks takes about 30 to 95 ms. One live-root query costs 0.7 ms against one
10,500-chunk root and 2.3 ms against two; 2,000 unlinks cost 4.6 s with the
query on every candidate and 35 ms with the generation shortcut.

Still open: a capture that spans hibernation (being closed by
`lane/restart-cancels-capture`); shortening `fileCASChunkUploadGrace`, a later
step that depends on this one being proved on staging; a read-only audit of
the roots on Node B; a root registered in the instant before a purge's
tombstone commits could strand one row. The generation counter is per process;
a second process that wrote snapshot roots (none does) would not bump it.
