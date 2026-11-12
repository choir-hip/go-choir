# The paper writes about 6 GiB of snapshot chunks an hour; Node B reaches the deploy floor in under a day

Date: 2026-11-12. Class: green (record only; nothing changed). Evidence:
[node-b-storage-review-2026-11-12](../evidence/node-b-storage-review-2026-11-12.md)
(read-only Sonnet review), the paper's observability dump of 19:14Z.

## Problem

Node B's free space fell from 133 GiB at 08Z to 84 GiB at 16Z on
2026-11-12. It is 84 GiB at 20:15Z, 24 GiB above the 60 GiB floor under
which a staging deploy is refused. The drain is the paper's file-cas
chunks.

## Evidence

- The paper's chunk directory on the host grew from 21.5 GiB (2026-11-11)
  to 53.0 GiB. By hour today (UTC, GiB of files by mtime): 08Z 2.9, then
  6.0 to 6.2 every hour to 14Z, 15Z 2.7, none at 16Z and 17Z, 18Z 3.5,
  19Z 6.9. About 9,400 files an hour.
- The collector deleted 18.2 GiB of the paper's chunks today. It keeps a
  chunk at least 24 hours, so a steady 6 GiB an hour holds about 146 GiB
  before deletes catch up. The disk has 84 GiB.
- The paper's own log shows a store snapshot streaming continuously
  (`cache return ... reason=snapshot_in_flight detail="a store snapshot is
  streaming"` every 10 minutes from 17:44Z to 19:14Z) and `store: wal
  bound: checkpoint after snapshot capture` about every 23 minutes
  (17:46, 18:10, 18:33, 18:55Z). The goroutine dump at 19:14Z had four
  `snapshot.(*chunkUploader)` workers. Its store is 21 GiB.
- The writes continued at 18Z and 19Z while the paper was wedged and
  could not write to its store (see
  [paper-stops-after-devin-stream-timeouts-2026-11-12](paper-stops-after-devin-stream-timeouts-2026-11-12.md)).

## Belief

The checkpoint cadence snapshots the whole 21 GiB store about every 23
minutes, and each snapshot uploads about 2.3 GiB of chunks the host has
not seen. Unconfirmed: whether the chunking defeats deduplication
(page-scattered SQLite changes, or a WAL fold that rewrites the file), and
why it began at 08Z today. Before today the net chunk growth was near
zero.

## Remaining error

- The cause is not traced. The next step is read-only: the snapshot and
  chunk code, and the host's chunk and collector logs for the 08Z start.
- The growth meter does not track chunks and the alarm fires only at the
  floor that already refuses a deploy.
- A deploy of the appender wedge fix needs the 60 GiB floor. If the drain
  continues at this rate the floor is crossed within about a day.
- Deleting chunks early or shortening the collector's window is the
  owner's call.

## Update 20:30Z: cause found, a failed-snapshot retry loop

Read-only trace (Sonnet investigator; code, host metadata, the guest
console log). Calibration claim C59 was half right: the snapshots do not
dedupe, but the reason there are so many is that none completes.

- `snapshotProducer.run` (`internal/agentcore/snapshot_producer.go`)
  advances its position only on success and has no backoff. With a tail of
  540,000 events since the last good snapshot (2026-11-11), the trigger
  stays due, and every failed attempt is followed by another full attempt
  at the next 10-minute tick.
- Attempts fail: at 20:21Z one aborted at the 30-minute budget inside the
  witness pass with the WAL at 3.7 GB; at 16:11Z one failed on the append
  of its record because the store was wedged. The witness takes 22 to 35
  minutes over 3.4 million rows. Earlier aborts are inferred (30-minute
  spacing; the 08:30Z to 14:50Z console log has rotated away).
- Each attempt uploads before it knows it can finish. Chunks are a fixed
  4 MiB cut with deterministic sealing. On a live 21 GiB store about 88%
  of chunks differ after 30 minutes, about 4,700 new files an attempt.
  Dedup works only on a frozen store: the two recorded roots at the wedge
  head share 10,510 of 10,510 chunks.
- The host keeps a chunk no root names for 24 hours after its last claim
  (`fileCASChunkUploadGrace`).
- Correction to the evidence above: the 18Z and 19Z figures were mtime
  touches of existing chunks (a claim refreshes mtime). By birth time 18Z
  had 4 new files. The wedge did not drain the disk; new chunks did, from
  08Z to 15Z, and again from 19:49Z after the refresh.
- Free space 82 GiB at 20:25Z, falling about 6.2 GiB an hour with nothing
  collectable until about 08:40Z on 2026-11-13. The 60 GiB floor is about
  four hours away.

Acts under the owner's go (2026-11-12, "make every change, deleting all
dead stuff"): a fix lane (`lane/snapshot-loop`: backoff after a failed
attempt, witness before upload, a 90-minute attempt deadline, a 3-hour
window for unrooted chunks) is being built; the destroyed-computer sweep
is in dry run to release about 35 GiB first.
