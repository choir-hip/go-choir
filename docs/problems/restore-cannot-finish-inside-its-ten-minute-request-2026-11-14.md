# A restore on d@e cannot finish inside its ten-minute request

Recorded 2026-11-14T22:57Z. Not repaired.

## Problem

`choir computer restore` is one blocking request with no async form and no
status route. Every budget on its path is ten minutes (proxy
`PROXY_REPLAY_COMPLETENESS_TIMEOUT=10m` in `nix/node-b.nix:961`; the guest's
write deadline in `internal/agentcore/api_self_development.go:31`). The work
is the scan, the install of the newest snapshot at or below the checkpoint,
the replay of the tail at about 113 events a second, then the witness check,
then the flip. A checkpoint that sits just below a snapshot falls back to the
snapshot before it. On d@e that was 74,000 events: 16 to 20 minutes of work.
The client gives up, the cancel aborts the replay, nothing is flipped, and
the caller is told nothing.

Three smaller defects seen on the way:

- A witness mismatch is not in `rematerializeRefusalStatus`
  (`rematerialize.go:374`), so it would answer 500, and only after the whole
  replay. Read from code, not observed.
- The restore holds `currentReleaseMu` for its whole run; the hourly cadence
  checkpoint waited on it and logged `held appends 6m17s`. That figure is the
  bind's wall time, not an append hold (`checkpoint_cadence.go:295`).
- The recovery bound is in events (720,000, `recoveryplan/recovery_plan.go:22`)
  with no matching time budget.

## Evidence

- Step 7 of the step 2 proof, run by an operator agent on d@e under N7.
  Checkpoint 189a91ed..., 21:34:02Z, sequence 108,784, `owner_recovery`,
  verdict `unverified`/`no_base`. d@e's own key carries `owner:acts`.
- Request 1, 22:38:33Z: Caddy status 499 after 74.9 s (the CLI's default
  timeout). Request 2, 22:43:29Z with a nine-minute timeout: 499 after 539.9 s.
- Guest log: staged store opened 22:47:35Z; `replay applied 33332 events
  through seq=68048 in 4m54s` at 22:52:29Z, the second the client gave up;
  the replay started after sequence 34,716. No flip, no witness line.
- After: d@e active, realization epoch 13706 unchanged, the revision written
  after the checkpoint still present, same process since 22:28:21Z.
- Guest memory read 1.2 to 1.7 GB against a 1,024 MiB ceiling during the
  second request; vmctl reclaimed each time; no kill.

## Belief

The two requests did nothing durable beyond the restore intent event
(idempotent per checkpoint; not checked on the tape). A third request with
the same checkpoint would fail the same way. A checkpoint at or above the
newest snapshot would replay a short tail and could finish; the cadence bind
published one at sequence 110,778 at 22:52:48Z.

Even with time, the restore's witness is the strict comparison, and d@e's
replay differs from live on the event index
(`replay-of-de-differs-on-the-event-index-only-2026-11-14.md`). The restore
may refuse on that. Not observed.

## Remaining error

- Whether the staging directory was removed on the guest; what request 1
  reached before the cancel.
- The repair: restore as an accepted act with a status to poll, its budget
  tied to the events to replay, and a witness mismatch answered as a refusal.
  Not designed.
- Steps 7d to 7f of the step 2 proof are not done.

## Third request, with a checkpoint above the snapshot (2026-11-14T23:05Z)

Claim C89, entered first; true.

- Checkpoint df408396..., 22:52:48Z, sequence 110,778. The newest snapshot is
  `interval snapshot at head 108786`, 21:34:58Z, so the tail was 1,992 events.
- Restore 22:58:30Z to 23:02:39Z: http 500, `rematerialize: reconstructed
  computer_event_index does not match checkpoint`. Replay took 15.6 s; the
  3m44s before the staged store opened is the scan and the snapshot install.
- Nothing flipped: epoch 13706 before and after, the revision written after
  the checkpoint still answers 200.
- d@e's console also shows snapshots refused for free space under the 2 GiB
  floor from 20:06Z to 21:17Z, before the boot prune freed the volume.

So on d@e today no checkpoint can be restored: the restore's witness compares
the event index strictly and a replayed index differs from the live one
(`replay-of-de-differs-on-the-event-index-only-2026-11-14.md`). Making the
probe tolerant is not a repair of this; the witness that a checkpoint records
and a restore recomputes needs the same canonical form, versioned so an old
checkpoint still fails closed. The refusal also answers 500 where it should be
a refusal status. Whether the paper or any other computer is in the same state
is not known; nothing was run there.

Steps 7d to 7f of the step 2 proof stay open behind this.


## Why the base snapshot was so old (2026-11-14T23:08Z; code and console logs, read-only)

- A snapshot is due when the head is 540,000 events past the newest one or
  seven days have passed (`internal/agentcore/snapshot_producer.go`,
  `snapshotDefaultTailEvents`, `snapshotDefaultInterval`); the producer ticks
  every ten minutes. It is not linked to the checkpoint cadence, which is a
  separate loop (`checkpoint_cadence.go` names no snapshot).
- d@e's snapshot fell due on the seven-day rule about 2026-11-13T22:10Z and was
  refused every ten minutes until 21:17Z on 11-14: 0.9 GB free against the
  2 GiB floor. The boot prune freed 10.6 GB at 21:31Z and the next tick took
  it (head 108,786, 11 s). That the 34,716 snapshot was the only older one is
  inferred; logs before 11-12 are gone.
- The 21:34Z checkpoint sits two events below that snapshot only because the
  cadence waits two minutes after boot and the producer three. Not a rule:
  the binds at 18:29Z, 19:29Z and 20:31Z had no snapshot after them.
- The worst case by design: 540,000 events of tail, about 80 minutes of replay
  at d@e's 113 events a second, against a ten-minute request.
- The paper's newest snapshot is a day old because none is due: about 282,000
  events since (inferred from the effective release's head, not the live
  head). No producer line in its retained log (from 17:43Z).
- A refusal that lasted 23 hours was visible only in the console log.

Repairs, not designed: restore as an accepted act with a status, refusing up
front when tail events over the measured replay rate exceed its bound; a
snapshot trigger in time to replay, not events alone; the snapshot refusal
surfaced where someone reads it.

## Investigation for the replay-time trigger (lane/snapshot-by-replay-time; code, read-only)

Written before the build. Mutation class of the build: orange (runtime
behavior of the producer; the snapshot and its tape record are unchanged).

(i) How chunks are stored, shared and pruned.

- `internal/store/snapshot_to.go` is the capsule store copy
  (`SnapshotTo`, `:77`), not the snapshot capture. The capture is
  `internal/snapshot/snapshot.go`. It writes no copy of the store: one
  read-only transaction streams the pages of the WAL snapshot
  (`snapshot.go:532`).
- Chunks are content-addressed and shared across successive snapshots of one
  computer. The page stream is cut into fixed page ranges, 4 MiB
  (`ChunkSize`, `snapshot.go:66`; chunk i is pages `[i*N,(i+1)*N)`,
  `:19`). Each is sealed with `filecas.SealChunk`, deterministic per key and
  plaintext (`snapshot.go:441`), so an unchanged range has the same digest.
  The uploader HEADs every digest and PUTs only the ones the host does not
  hold (`snapshot.go:450-475`; stats `ChunksHeld`, `ChunksUploaded`,
  `BytesUploaded`). A new snapshot therefore costs the changed ranges, not a
  full copy. A range counts as changed if any page in it changed, so a
  store that rewrites pages all over (the paper) shares little: the comment
  at `snapshot_producer.go:56-60` records 20 GB a day in October and the
  paper's 11-13 record gives 4.8 GB of sealed chunks added by a 46.7 GB,
  37 min 40 s snapshot.
- Retention, host side: each snapshot registers a root (digest list)
  (`snapshot/http_chunks.go:100`), and `pruneSnapshotRoots`
  (`internal/platform/snapshot_roots.go:203`) keeps the newest 4 roots the
  tape names, 2 when the store is over 2048 chunks (8 GiB)
  (`snapshot_roots.go:42-49`). Chunks no live root names become collectable
  by the artifact sweep (`platform/artifact_gc.go:182`, after its grace).
  Restore considers at most the newest 4 snapshots (`snapshot.MaxCandidates`,
  `snapshot.go:896-899`), so on a large store a checkpoint older than the
  second newest snapshot has no base.
- Retention, guest side: nothing is kept locally. Only the
  `snapshot_recorded` event stays on the tape, and the producer removes a
  leftover VACUUM INTO copy (`snapshot_producer.go`, `snapshotLegacyTempDirName`).

(ii) What the replay rate depends on, and whether it is measured.

- The tail replay (`computerevent/appender.go:1396-1430`) pages events from
  the host in 1,024-event pages (fetch), then reduces, verifies receipts,
  projects rows and commits. The log line breaks the time into fetch,
  payload prefetch, reduce, verify, prepare, finalize and commit. So the rate
  depends on host fetch latency, payload fetches, the per-event projection
  (which grows with the store's indexes and the cold page cache) and the
  commit cadence. Measured: 206 to 380 events/s on d@e in October and early
  November, 113 events/s on 2026-11-14 (restore of 33,332 events in 4 m 54 s).
  The paper's replay rate has not been measured in the records.
- At runtime it is a log line only: `replay applied N events ... (X events/s`
  (`appender.go:1423`) is computed in a deferred function and kept nowhere.
  `ReplaySnapshot().AppliedRows` is zeroed per replay (`:1404`). No counter
  of the rate exists. The build adds one.

(iii) Cost of a capture as a function of store size.

- Time: the stream and the witness both read every page, so both scale with
  the store. The paper's 46.7 GB took 37 min 40 s; the witness alone was 22 to
  35 minutes on 21 GiB (`snapshot_producer.go:79-84`); d@e's 1 GiB took 11 s.
- Held: one read transaction on the read pool for the whole capture
  (`ReadTxTime`). Writers and appends do not wait, but the transaction pins
  the WAL, which grew 3.7 GB in 30 minutes under load (comment at
  `snapshot_producer.go:81-84`), so free space is the resource held; the
  producer refuses under 2 GiB (`snapshotFreeFloor`) and the capture checks
  it during the stream. Memory is a few 4 MiB buffers.
- Written: sealed bytes to the host for the changed ranges only
  (`BytesUploaded`), plus a probe per chunk, and the host keeps unreferenced
  chunks through its grace.

Consequence for the trigger: the cost of a capture (seconds on d@e, 37 minutes
and several GB on the paper) is known after the fact from `Stats`, so the
trigger can derive its own floor between captures from the last capture.
