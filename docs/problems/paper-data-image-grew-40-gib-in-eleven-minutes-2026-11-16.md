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
