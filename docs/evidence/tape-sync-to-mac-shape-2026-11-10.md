# The paper's tape synced to a Mac for a day: bytes and lag, from the shape alone (2026-11-10)

Status: measurement and estimate only, mutation class green (docs). Item X6
of [the overnight mission](../definitions/choir-autopaper-overnight-2026-11-09.md),
first half ("the paper's tape synced to a Mac for a day, in bytes and lag").
No paper content was copied to this Mac. Everything below comes from file
sizes and times on Node B, a synthetic-payload link test between this Mac and
Node B, and the repo's own follow code and cost test. Companion:
[sealed segments as 64 KiB blocks](segment-block-packing-exploration-2026-11-10.md).

Labels: **[M]** measured in this run (2026-11-10, 10:46 to 11:25 UTC);
**[C]** read from code; **[E]** estimated, arithmetic shown; **[A]** assumed.
GB and MB are 10^9 and 10^6 bytes.

## Summary in plain words

1. **A day of the paper's tape is small next to the link.** The paper's
   segments grow by about 105 MB an hour over the last 24 hours (2.5 GB a
   day) and 135 MB an hour over the 35 minutes I sampled [M]. That is about
   18,000 events an hour (5 a second) at about 5.7 KB each [E]. In the bundle
   form the follower pulls (events, payloads, pins and head receipts), a day
   is about 3.2 GB (2.5 to 3.9) [E]. The link carries 28 MB/s down [M], so a
   day of tape crosses it in under 2 minutes of transfer.
2. **Lag is a polling choice, not a bandwidth question.** Syncing on seal
   leaves the Mac a mean 15 minutes behind (p95 33 minutes, worst 111 in the
   last 48 hours) [E from M seal times], because 68% of recent segments are
   sealed short. Tail-following at the existing 30-second poll leaves it a
   mean 15 s behind; at 5 s, 3 s [E]. Appends touch the open segment
   continuously (the newest segment's last write was under a second old at all 8 samples) [M], so a
   tail has something to follow at any moment.
3. **The first full sync is where the bytes are.** A bootstrap pulls the
   newest snapshot (21 to 31 GB, a figure from 2026-10-31 that I did not
   re-measure) and the tape after it: 13 to 19 minutes of transfer at
   28 MB/s [E]. Pulling the paper's whole segment history (65.7 GB) is 39
   minutes; catching up after a 24-hour disconnect is 3.1 GB and under 2
   minutes [E]. Disk on the Mac: about 3.2 GB a day plus the 21 GB snapshot,
   before any retention.

## Method

1. **Growth and seal cadence.** Eight samples at 5-minute spacing over 35
   minutes (10:47 to 11:22 UTC; script in the session scratchpad): per sample
   one `find ... -printf` of paper tape-seg files modified in the last 25
   minutes (size, mtime, name) and a total of segment-file bytes. Plus the
   one-shot listing of all 7,698 files from the companion note, giving seal
   times (index mtimes), segment sizes and, from index sizes, record counts
   [(index size - 1068) / 45, exact on the two indexes checked].
2. **Events per hour.** The platform's journals carry no per-event line I
   could count by prefix (corpusd printed retrieval lines, vmctl lifecycle
   lines), and a PostgreSQL count was out of the metadata-only rule. So events
   are records in sealed indexes divided by 4, the record mix per event
   measured on 2026-10-30
   ([tape window sizing](tape-window-sizing-2026-10-30.md) section 2.3: one
   event, one payload, two pin receipts per projection batch, 99.3% of the
   paper's events). That is [E], not a count of events.
3. **Link.** From this Mac to Node B (public address, ssh, no compression;
   the Mac is a residential or office link, not characterised further):
   three 50 MB downloads of `/dev/zero` through `ssh node-b 'head -c ...'`
   piped to `wc -c`; two 20 MB uploads; five `ssh node-b true` handshakes;
   25 one-byte echoes over one open session (first 3 dropped). A single TCP
   stream through ssh's cipher, so it is a floor for HTTPS streaming, not a
   ceiling.
4. **Follow code.** Read `internal/tapefollow` (the Mac side),
   `internal/platform/tape_follow.go` (the host route) and
   `internal/bundle/follow.go` (the format); ran the repo's
   `TestTapeFollowCost` locally (`CHOIR_TAPE_FOLLOW_COST=1`, synthetic
   400-event computer).

## Measurements

### Growth [M]

Paper tape-seg segment bytes by sample (minutes from first sample):

| t (min) | Segment bytes (all paper `.seg`) | Open segment | Open size | Age of last write |
|---:|---:|---|---:|---:|
| 0 | 65,745,705,293 | 1647 | 16.1 MB | 0.1 s |
| 5 | 65,755,709,554 | 1647 | 26.1 MB | 0.0 s |
| 10 | 65,767,768,383 | 1647 | 38.1 MB | 0.0 s |
| 15 | 65,780,601,524 | 1647 | 51.0 MB | 0.0 s |
| 20 | 65,793,144,808 | 1648 | 3.9 MB | 0.0 s |
| 25 | 65,803,928,775 | 1648 | 14.7 MB | 0.1 s |
| 30 | 65,814,995,192 | 1648 | 25.7 MB | 0.1 s |
| 35 | 65,824,930,860 | 1648 | 35.7 MB | 0.0 s |

(Segment 1647 was sealed at 59.6 MB between the 15- and 20-minute samples,
about 26 minutes after the previous seal, and its `.idx` appeared; the
length is not the 64 MiB limit, so this was a short seal. At the 20-minute
sample 1647's last write was 98 s old, at later samples 399 to 1,001 s,
consistent with a sealed file.)

Growth over the 35 minutes: 79.2 MB, 135.5 MB/h; per 5-minute interval 10.0
to 13.0 MB (33 to 43 KB/s). Over the preceding windows, from sealed-segment
sizes and index mtimes [M]:

| Window | Segments sealed | Bytes | MB/h | Records | Events/h [E] | Bytes/record | Bytes/event |
|---|---:|---:|---:|---:|---:|---:|---:|
| 1 h | 2 | 94.6 MB | 94.6 | 66,320 | 16,580 | 1,427 | 5,707 |
| 6 h | 14 | 583 MB | 97.2 | 401,142 | 16,714 | 1,453 | 5,814 |
| 24 h | 60 | 2,530 MB | 105.4 | 1,791,390 | 18,660 | 1,412 | 5,650 |
| 48 h | 111 | 4,942 MB | 103.0 | 3,470,162 | 18,074 | 1,424 | 5,697 |
| 7 days | 430 | 18,923 MB | 112.6 | 13,998,132 | 20,831 | 1,352 | 5,407 |

Daily sealed bytes (UTC): 11-02 1.97 GB, 11-03 1.87, 11-04 2.47, 11-05 2.31,
11-06 3.57, 11-07 3.32, 11-08 2.31, 11-09 2.85 [M]. Mean 2.6 GB, peak
3.6 GB. 2026-11-01 sealed 15.3 GB in 599 segments, about 6 times a normal
day, which I did not investigate. On 2026-10-29 the host receipts counted
342,000 events; the figure here (about 450,000 a day) is higher, and the 4
records per event assumption is the weak link. If some events carry no
payload record the true event rate is higher and bytes per event lower.

### Seal cadence [M]

Over the last 48 hours: 111 paper segments sealed, 76 under 64 MiB. Interval
between consecutive seals: median 26.6 min, p90 39 min, longest 111 min.
Interval between consecutive full (64 MiB) seals: median 50 min, p90 172 min.

### Link [M]

| Test | Result |
|---|---|
| Download, 50 MB x3 | 2.12, 1.69, 1.76 s: 23.6, 29.6, 28.4 MB/s (188 to 237 Mbit/s), mean 27.2 MB/s |
| Upload, 20 MB x2 | 13.8, 11.7 MB/s |
| ssh handshake plus command, x5 | 0.34 to 0.40 s (median 0.386) |
| Round trip inside one session, 22 echoes | min 18.0, median 19.7, max 21.4 ms |

Only the download direction matters for
a follower. The test ran once, at about 10:55 UTC; I did not repeat it at
other hours, so diurnal variation is unmeasured. The 28 MB/s figure also
includes ssh encryption on both ends.

### The existing follow mechanism [C, M]

- The follower pulls `GET /api/computers/<id>/tape/bundle?branch=main`,
  bootstrap without `have_seq`, then deltas with `have_seq`/`have_head`, at
  most 2,000 events a pull; 204 means current. Default poll interval 30 s,
  backoff to 5 min, store bound 64 GB, stall timeout one minute
  (`internal/tapefollow/syncer.go`, `client.go`).
- A bundle is a format 2 tar: `receipts.jsonl`, `objects.idx`, `objects.pack`,
  the snapshot's chunks on bootstrap, `manifest.json` last
  (`internal/bundle/follow.go`). The host reads PostgreSQL rows and artifact
  files, never the guest, so a poll does not wake a hibernated computer.
  Private payloads travel as ciphertext.
- The route admits 10 requests of burst and one a second per computer, one
  export in flight per computer, two on the host
  (`tapeFollowBurst`, `tapeFollowRefill`, `tapeFollowGlobal`).
- `TestTapeFollowCost`, local, 400 synthetic events with 32 KB payloads:
  14.75 MB streamed in 248 ms (2,000 per pull) or 329 ms (100 per pull);
  host-side CPU 15.0 to 15.8 ms per MB; 3.7 to 3.9 MB allocated per MB;
  23 ms for a one-event pull, 71 to 77 ms for 100 [M]. Per event that test
  carried about 36.7 KB, of which about 3.9 KB was not payload (event, head
  receipt, pins) [M].
- Owner direction of 2026-10-31
  ([one-tape durability](../design/one-tape-durability-2026-10-31.md) 4a):
  the Mac follows user computers, "not replicating the paper on the Mac".
  This note therefore measures the paper as the worst case for the
  owner's computer, not as a plan.

## Model

Inputs: 18,000 events/h (5.0/s) [E]; bundle bytes per event 7.2 KB [E: 5.7 KB
of records, plus about 1.5 KB for the head receipt row that the follower
receives and the segment does not carry; the test's 3.9 KB non-payload
overhead is the check on this magnitude, high because its payload is
synthetic]; link 28 MB/s down, 12 MB/s up, 19.7 ms RTT [M].

### Bytes per day

| Form | Per event | Per day | Basis |
|---|---:|---:|---|
| Sealed segment bytes (what Node B stores) | 5.7 KB | 2.5 GB (1.9 to 3.6, [M] daily range) | [M] |
| Segment plus 2.7% index | | 2.6 GB | [M] |
| Bundle as the follower pulls it | 7.2 KB | 3.2 GB (2.5 to 3.9) | [E] |
| Rate while the paper runs | 36 KB/s | 0.13% of the 28 MB/s link | [E] |
| Disk growth on the Mac, no retention | | 3.2 GB/day, 97 GB/month | [E] |

The 2026-10-31 review's bound was "3 to 10 GB a day" and "1 to 5 GB a day of
growth on the Mac"; today's tape sits in the lower half.

### Steady-state lag

| Scheme | Mean | p95 | Worst | Notes |
|---|---:|---:|---:|---|
| Sync on seal | 14.8 min (24 h), 17.0 min (48 h) | 33 min (24 h), 41 min (48 h) | 51 min (24 h), 111 min (48 h) | [E] event-weighted: an event waits for its segment's seal; plus 2.4 s to move 64 MiB |
| Tail-follow, poll 30 s (the code's default) | 15 s plus transfer | 29 s | about 31 s | [E] N/2 + RTT + export; one pull carries about 150 events, 1.1 MB, 0.04 s at link speed, plus 23 to 80 ms host export [M test] |
| Tail-follow, poll 5 s | 2.5 s | 4.8 s | 5.2 s | 25 events per pull; 17,280 requests/day, most 204s |
| Tail-follow, poll 1 s | 0.5 s | 1 s | 1.2 s | at the route's refill of 1 a second; 86,400 requests/day |
| Push per group-commit window (the fabric design's "2 s") | about 1 s | 2 s | 2.3 s | [E] needs the inverse route, not built |

The seal-lag rows are the event-weighted mean of T/2 over the seal
intervals T (an event is uniform within its segment's life): sum(T^2/2) /
sum(T) over 59 intervals (24 h) and 110 (48 h). The p95 and worst are over
the same intervals. The numbers assume a seal means a visible, complete
file; a seal that waits on rule F.2's 10 s delayed rewrite adds 10 s.

### Catch-up after a disconnect [E]

Backlog = 3.2 GB x hours / 24; drain at link speed with the follower's 2,000
events per pull (about 14 MB, 0.5 s a pull):

| Disconnect | Backlog | Pulls (2,000 events) | Transfer at 28 MB/s | At 12 MB/s | Drain factor |
|---|---:|---:|---:|---:|---:|
| 1 h | 0.13 GB | 9 | 5 s | 11 s | 780x real time |
| 8 h | 1.04 GB | 72 | 37 s | 86 s | 780x |
| 24 h | 3.11 GB | 216 | 111 s | 259 s | 780x |

Per-pull overhead (rate limiter at one request a second after the 10-request
burst, RTT, export setup at 23 ms) adds about 1 s a pull when pulls are
back to back, so a 24 hour catch-up is bounded by about 216 s of limiter
time, 2 to 4 minutes either way. The server spends 15 ms of CPU per MB, so a
24 hour backlog costs about 47 CPU seconds on the host [E from M test].

### First full sync [E]

| What | Bytes | At 28 MB/s | At 12 MB/s |
|---|---:|---:|---:|
| Bootstrap: newest snapshot (21 GB compacted; 31 GB pre-compaction) plus tape after it | 21 to 31 GB + up to 3 GB | 13 to 19 min | 29 to 43 min |
| The paper's full segment history, 10-13 to now | 65.7 GB (+ 1.8 GB indexes) | 39 min | 91 min |
| Both (bootstrap and history) | about 90 to 100 GB | 54 to 60 min | 2.1 to 2.4 h |

The 21 and 31 GB are from the small-object design's 2026-10-31 anatomy
[M then; not re-measured now]. Snapshot chunks are encrypted, so zero
transfer compression is assumed; the 64 GB store bound in the syncer would
refuse the "both" row (`DefaultMaxStoreBytes`). A bootstrap at 2,000 events
a pull is limited by the chunk stream, not the pull count.

## What is measured, estimated, assumed

- Measured: segment growth every 5 minutes for 35 minutes; seal times and
  sizes for 7 days; link bandwidth, latency; the repo's cost test.
- Estimated: events per hour (records divided by 4); bundle bytes per event;
  all lag and catch-up figures; snapshot size (stale).
- Assumed: 28 MB/s holds at the time of use; HTTPS through the edge matches
  ssh; the follower can read an event whose payload sits in the open segment
  (the host route reads artifacts through the tape reader; I read the code
  only far enough to see it does not read the guest, and did not confirm the
  open-segment case).

## Limits

- 35 minutes is a short window for growth: the daily range in sealed bytes
  is 1.9 to 3.6 GB, and 11-01 was 15 GB. A real day contains a lull and
  bursts that this window cannot show.
- The record-to-event ratio is carried over from 2026-10-30. Journals gave
  no per-event count and I did not query PostgreSQL.
- One link test at one hour from one machine; the upload direction is 2x
  slower than the download but irrelevant to following.
- No HTTPS test: `choir.news` needs credentials I do not have a reason to
  use for synthetic traffic.
- I did not measure the Mac side: disk write rate, encryption at rest or
  verification cost (the follower verifies every bundle).

## What a real one-day experiment would need

- **Authority.** An owner decision, since the paper is the Autopaper
  computer and the 2026-10-31 ruling says the Mac follows user computers
  first. The follower is read-only and sends only GETs [C], and the route
  needs an owner-authenticated session joined to the computer's owner in
  vmctl; the paper's owner credential would be used, so the owner runs it.
  No agent should use the owner's session.
- **Encryption at rest on the Mac.** The paper's tape holds user and paper
  data; payloads arrive as ciphertext but events, head receipts and snapshot
  chunks do not all. FileVault covers a laptop; a dedicated encrypted
  APFS volume (or sparse bundle) holds the store; the key for payloads is
  not served (the Keychain design is later).
- **Bundle format.** Format 2 bundle chain exactly as `tapefollow` writes it,
  one directory per pull under `bundles/<start>-<head>/` (so about 220
  directories a day at 2,000 events, 2,880 at the default 30 s poll with
  data in every pull): the per-pull directory count is a
  cost the number of files does not show here.
- **Instruments.** The follower's own status (head seq, source head seq,
  synced at), logged every poll, gives lag directly: source head minus held
  head in events, divided by 5/s.

What it would settle that this estimate cannot: the real events per day and
bytes per event of a bundle (not 4 records a guess); the real lag
distribution at 30 s against the seal model; throughput through the
production edge (HTTPS, Caddy, proxy) and limiter; Mac disk and CPU cost of
verification; and what happens across the paper's frequent restarts
(a segment seal every 26 minutes) and a planned refresh.

## What this suggests

- **Resource fabric: sync the tail, not the seal.** Seal-based sync gives
  15 to 111 minutes of lag; the open segment is written continuously, and
  tail-following the same bytes at a 5-second poll gives 2.5 s for 17,000
  small requests a day. The fabric's unit of sync wants to be an
  append-ordered byte range (an event range or a block range), with seals
  only marking immutability for verification and retention.
- **The cost is the snapshot, not the tape.** 36 KB/s of tape is trivial;
  the first sync is 13 to 43 minutes of snapshot chunks, and every future
  re-bootstrap repeats it. Anything that cuts the 4 MiB-chunk churn (small
  object design rule B) cuts the Mac's cost far more than packing the tape.
- **Small-object link.** A 64 KiB block is about 2 seconds of paper tape
  (35 KB/s), a natural frame for a push. Block packing would not change the
  daily bytes much (see the companion note: 18 to 40 GB of 82 GB across the
  whole history, nothing on a day's 3 GB beyond that ratio), but it fits a
  2 s window.
- **The retention question is real.** 3.2 GB a day is 97 GB a month; the
  syncer's 64 GB bound is about 20 days of paper tape. For a user computer
  (a few hundred events a day) the same arithmetic is kilobytes, which is
  the case the owner asked for.
