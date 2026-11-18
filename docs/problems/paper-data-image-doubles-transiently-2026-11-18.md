# The paper's data image nearly doubles for an hour or more, once or twice a day (2026-11-18)

Status: open; cause not confirmed. Nothing was changed. Found by the hourly
status tick, which flagged a fast drain at 05:38Z on 2026-11-18 (free 154 GiB
to 102 GiB) that had recovered by the next tick.

## Problem

The sparse data image of the paper's computer
(`fs.vm-state.vm-1669020a….data.img` in the growth meter) grows to about 1.9
times its size and falls back within one to three hours. While it is large,
Node B's free space falls by the same amount. The image's resting size has
doubled this month (23 GiB on the 1st, 49 GiB on the 18th), so the space the
swing needs has doubled too.

## Evidence

Growth meter ledger on Node B (`/var/lib/go-choir/growth-meter`, hourly rows,
read only), every hour-to-hour change of more than 8 GiB in November:

| Up (UTC) | Size, GiB | Back down | Free at the peak, GiB |
| --- | --- | --- | --- |
| 11-01 04:00 | 23.2 to 42.2 | 06:01 | 203 |
| 11-02 05:01 | 23.8 to 44.6 | 11-03 06:01 | 168 |
| 11-09 05:00 | 37.6 to 70.5 | 06:00, up again 07:00, down 10:02 | 68 |
| 11-15 01:00 | 49.4 to 84.2 | 04:00 | 135 |
| 11-16 01:01 | 43.2 to 82.1 | 02:00 | 145 |
| 11-16 19:00 | 44.8 to 85.8 | 20:00 | 125 |
| 11-17 06:01 | 46.1 to 87.4 | 08:00 | 122 |
| 11-17 17:01 | 47.4 to 90.5 | 18:01 | 118 |
| 11-18 05:00 | 48.8 to 93.2 | 06:01 | 103 |

The 11-18 swing was in progress at 05:00:25Z; the paper's snapshot root
0222b95d01bf is stamped 05:05:13Z. No swing between the 3rd and the 9th or
the 9th and the 15th appears in the ledger. The times are not a fixed hour.

## Belief

Something inside the guest rewrites nearly the whole of its store into new
blocks and then frees the old ones (a compaction, or a snapshot that stages a
full copy inside the image), and the host gets the space back when the guest
discards. The ratio is 1.8 to 1.9, not 2, which fits a rewrite of the largest
store rather than a copy of the image file. I have not read the guest's logs
for those hours, so this is a hypothesis. The one coincidence with a snapshot
is a sample of one.

## Why it matters

The swing needs free space equal to the image. At the present rate of growth
the image reaches 75 GiB in a few weeks and the swing would then take 75 GiB
at once. On the 9th free space fell to 68 GiB during one. The forecast unit
reads daily minima and calls this "not declining"; the fast-drain warning
fires and clears. Neither says "a swing the size of the image is coming".

## Remaining error and next step

- Not known: what runs in the guest at those times (read the paper's console
  log and the guest's store log for 04:30Z to 06:10Z on 11-18 and 16:30Z to
  18:10Z on 11-17; read only).
- Not known: whether other computers' images swing the same way at a smaller
  size.
- If it is a compaction: the storage mission's "store once" work (the guest
  store as a working set) is the place it gets fixed, not a threshold.
