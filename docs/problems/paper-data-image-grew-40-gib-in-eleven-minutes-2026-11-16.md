# The paper's data image grew 40 GiB in eleven minutes after a deploy

2026-11-16, recorded when seen. No fix is proposed here.

## Problem

Node B's free space fell from 182.5 GiB to 146.2 GiB between the hourly
status readings of 00:29Z and 00:40Z. The whole fall is the paper's data
image: its allocated size went from 43.2 GiB (the status line's "data
image" figure at 00:29Z) to 83 GiB (`du` on the file at 00:40Z). The image
is a 128 GiB sparse file, so this is real blocks written inside the guest.

## Evidence

- `scripts/status-hourly` at 00:29:12Z: free 182.5 GiB, paper footprint
  "chunks 11.5 GiB + data image 43.2 GiB".
- The same script at 00:40:12Z: free 146.2 GiB; the forecast lines call
  themselves stale because live `df` differs by over 10 GiB.
- `du` on the paper's `data.img` at 00:40:34Z: 83 GiB. Two readings 45
  seconds apart at 00:41Z: 84,105 MiB then 84,106 MiB. The growth had
  stopped.
- The capture-trigger fix (1c2ffa7a9) was deployed at 00:26:58Z. The host
  log shows vmctl restarting at 00:25:58Z, reattaching the paper, and an
  expedited ring walk whose reason is that fix. The paper's console at
  00:41Z shows it running and calling models (uptime about 913 s, so it
  booted near 00:26Z), and a line "store: telemetry retirement: 11 ops in
  4m29s".
- The paper was publishing throughout (1,821 paper lines in the last 20
  minutes at 00:40Z).

## Belief

Most likely the snapshot capture that the fix was written to start: the
paper had gone 50 hours without one, and a capture of a store this size
may write a full copy inside the guest before chunks leave it. Entered as
claim C108 at 0.55. The other candidates are the store's telemetry
retirement (a rewrite that does not release blocks) and a boot-time
migration.

## Remaining error

- Which writer it was is not read from the guest yet.
- Whether the blocks are released afterwards (the image is sparse; a
  delete inside the guest does not return blocks to the host without
  discard).
- Whether every later capture costs the same. At 40 GiB a capture, three
  more would fill the disk; the host has 146 GiB free and the forecast
  line is not yet aware of the step.

The next hourly reading will show whether the snapshot root moved and
whether the image is still at 84 GiB.

## Settled, 01:40Z

It was the snapshot capture. The paper's console shows the store held by
`internal/snapshot.copyPages` and "a store snapshot is streaming" at
00:35:58Z, then "checkpoint after snapshot capture" at 00:36:09Z. The
snapshot is 41.76 GB: a whole copy of the store, written inside the guest.
A second capture finished at 01:12:56Z (snapshot root f51787694d0a, head
12,028,074) and the image stayed at 84 GiB, so the copy reuses the same
blocks and the 40 GiB is paid once, not per capture.

What is still open:

- The paper now needs twice its store on disk for as long as captures are
  whole copies. At 42 GB and growing, that is the largest single user of
  the host's disk and it is not in the forecast.
- During the copy the console prints `store_stalled` lines with the
  snapshot and object-snapshot readers holding the store. Whether a turn
  waited on them is not measured.
- The host's fast-drain warning fired for this step (39.9 GiB in 6 h). It
  will clear by itself when the step leaves the window; it is not a drain.

## Seen again, 18:43Z to 20:36Z

The same step happened in the evening and the growth ledger shows it is the
pattern, not an accident of one deploy.

- The paper's console shows the store held by `internal/snapshot.copyPages`
  from 18:43Z and "checkpoint after snapshot capture" at 18:48:27Z. The
  host's hourly ledger row at 19:00Z has the data image at 86 GiB (45 GiB an
  hour earlier) and free space at 125 GiB (166 GiB an hour earlier). Nothing
  else moved by more than 2 GiB.
- The console still said "a store snapshot is streaming" at 19:18Z and
  19:33Z. The snapshot root `0416b20b5f2e` (head 12,235,390, 10,385 chunks)
  is dated 19:36:54Z, so the upload took about 48 minutes and finished about
  a minute before the deploy refresh restarted the paper at 19:37Z.
- At 20:35Z the image was back at 45.2 GiB and free space at 165.3 GiB. The
  console shows a `snapshot-scratch-sweep` at boot and an hourly trim of the
  guest data volume; together they return the blocks.
- The ledger has the same step on 2026-11-15 near 01:00Z (+35 GiB, back by
  04:00Z) and on 2026-11-16 near 01:00Z (+39 GiB, back by 02:00Z).

## Belief

Every capture writes one whole copy of the store inside the guest (41 GiB
now), holds it for the length of the upload (about 48 minutes now), and the
space returns within an hour of the scratch copy being removed. So the host
needs the store's size free for most of an hour per capture, and both the
size and the time grow with the store.

## Remaining error

- A deploy refresh that restarts the paper during the upload would discard
  the capture. This one finished a minute before the restart. Whether the
  refresh waits for a streaming snapshot was not read from the code.
- The lead's reading at 19:35Z blamed the Texture deploy; the capture began
  35 minutes before it. A later reading said the deploys had killed the
  snapshot; the root dated 19:36:54Z shows it completed.
- Why captures happen when they do (01:12Z and 18:43Z today) is not read.


## Seen again, 2026-11-19T08Z, and one open item answered

The hourly reading at 08:36Z had free space at 87 GiB (141 GiB an hour
earlier) and the paper's image at 98.6 GiB (51.5 GiB an hour earlier): the
same step, now 47 GiB. The growth ledger's hourly rows show it six times in
three days, each for one or two rows (11-16T19, 11-17T06 and 07, 11-17T17,
11-18T05, 11-18T18, 11-19T08), with the image back at its baseline after.

The open item "whether the refresh waits for a streaming snapshot" is
answered from the host's log: it does. The deploy of `d26a202aa` asked the
paper to refresh from 08:08Z; vmctl deferred it 28 times, a minute apart,
with `snapshot_in_flight: true` (`selfdev_active_operations: 0`,
`live_capsules: 0`), and refreshed it at 08:36:07Z, eight seconds after
the snapshot root `e5ab44ea87c6` was dated. During the copy the host marked
the paper unhealthy three times (08:26Z to 08:28Z) and the console printed
a `store_stalled` line at 08:28Z.

So a deploy that lands during a capture reaches the paper up to an hour
late, and the copy needs the store's size free on the host: 47 GiB today
with 141 GiB free before it. The space returned: the hourly reading at
09:36Z has 134.2 GiB free and the image at 51.6 GiB.

## Update 2026-11-20: the same step, 48 GiB, and how close it came to the floor

At about 00:03Z on 2026-11-20 the paper took a store snapshot ("snapshot at
head 13079421: 51391889408 bytes ... in 1h22m51s", logged 01:25:27Z). The
image went from 52.6 GiB to 100.9 GiB and host free space from 120 GiB
(00:00Z) to 72 GiB (00:15Z), where it stayed for the 83 minutes of the
upload; the controller logged 20 failed health checks from 00:02Z to
00:18Z and no restart. The growth ledger shows seven such steps in four
days (the one before: 07:00Z to 09:00Z on 2026-11-19, 51 GiB to 98 GiB and
back).

What this adds: the copy is the size of the store (51 GiB now and
growing) and is held for the whole upload, so a capture needs the store's
size free on the host for over an hour. At 76 GiB free, a second capture
before the first is trimmed, or the store growing another 16 GiB, breaches
the 60 GiB floor. What schedules a capture is still not read. Whether the
deploy gate defers while a capture is in flight is not read. The fix is
structural (stream the pages without a scratch copy, or copy only changed
pages) and is not designed.

## What changed, 2026-11-20 (S1 built, not deployed)

S1 of `design/snapshot-without-scratch-copy-2026-11-20.md` (lane
`snapshot-stream`) stops the capture writing the store into `snapshot-tmp`: it
runs the witness and then streams the pages from the one read transaction that
read the head. The cost moves from the store's size on the data volume to the
WAL the reader pins. Not yet deployed; the record stays open until the paper's
first capture after the deploy shows no data-image step over 15 GiB in the
growth ledger and `last_pin_wal_frame_bytes` under 12 GiB. **The trade is unproven**: the 2 MiB/s WAL rate rests on one reading (3.67 GB in 30 minutes on 2026-11-12, post-restart load, a 22 GB store; another that day was 1.1 MiB/s) and a 71-minute pin with a multi-GiB WAL has not been measured for read latency or for the checkpoint after it (1 min 21 s on 2026-11-12). S1 must be measured on a test computer with a large store under sustained writes before the paper takes it: `design/snapshot-without-scratch-copy-2026-11-20.md`, "Measuring S1 before the paper takes it". Rollback leaves a
binary that sweeps `snapshot-tmp` at boot.
