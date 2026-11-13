# The tape sweep's walk probes every segment for every event

Date: 2026-11-13. Status: walk repaired (dd43ff6b1); open until a dry run finishes.

## Problem

The tape sweep's dry run cannot finish on the paper. Its references phase
walks the paper's tape (about 11 million events) at about 60 events a second:
13.3 minutes per 50,000 events. The whole walk is then 25 to 50 hours of run
time, and every deploy restarts corpusd and ends the job. Since 421288af7 the
progress is kept and a new run reuses it, but each restart is by hand, so no
manifest for the destroyed computers' tape has been produced yet.

## Evidence

- `tape_sweep_ref_chunks` on Node B, run `ts-20261113T065544Z-903217d8`
  (started 06:55:44Z): the paper's chunks 1 to 3 were written at 07:09:39,
  07:22:59 and 07:36:27Z. The 14 small computers finished within seconds.
- corpusd during the walk: about two cores busy, IO wait 0 to 2%, memory 1.5
  to 2.7 GB. Postgres idle; the receipt query is paged (1,000 an `IN`).
- The paper's tape is 1,784 segments (3,567 files, 71 GB), index files 0.9 to
  1.7 MB each.
- Code (read at e392ac824): `walkTapeChain` (`internal/platform/tape_sweep.go`)
  calls `tapeReader().Read` once per event. `Reader.Read` reaches
  `readSegment` (`internal/tapestore/reader.go`), which on every call lists
  the computer's segment directory, sorts it, runs `retireSealed` under the
  reader's lock, and then probes the segments newest first, opening each
  index file, until one holds the digest. An event in an old segment costs
  about 1,784 probes.

## Belief

The per-event point lookup is the cost: about 16 ms an event fits 1,784
probes at 8 to 10 microseconds plus the directory listing. This was inferred
from the code and the host's CPU and IO readings, not profiled. The rate
should improve as the walk reaches newer events (fewer segments to probe
before a hit), if event order tracks segment order; that is assumed, not
checked.

The same lookup serves every other read of one event from a long tape, so
the fix belongs in the reader, not only in the sweep.

## Repair

Walk with one view per computer: list the directory and validate each index
once, and probe from the segment of the last hit, falling back to the full
probe on a miss. The record's CRC and digest checks, the corrupt-record stop
and the rescan of the open segment stay as they are. The reference set must
be byte-identical to the per-event path's (`tapeRefFormat` stays 1). Class
red: the reference set decides which tape is deleted.

## Remaining error

- No profile was taken; a second cost may sit under the first.
- Until the repair lands, the sweep advances only while a hand-started run is
  alive between deploys.

## Update, 09:35Z: the first repair did not help on the paper

f4276a0ba (one view per walk, probe from the last hit) is deployed. On the
paper the walk's log line for events 450,000 to 500,000 reads 1,789 segments
and 46,733,852 probes, about 935 an event, and the chunk took 10 minutes
against 13.3. The directory is listed once now, but the hint almost never
points at the next event's segment: either the early history was repacked
into segments in an order unrelated to sequence, or about half the events
are in no segment and take a full miss. The benchmark that showed 38 times
used a fixture laid out in order. The belief above that the rate improves
toward newer events is therefore also unsupported.

Next repair (lane/sweep-walk-digest-index): while the view validates each
index it builds a lookup from event digest prefix to segment, so a read
probes only the segments that can hold the digest and an absent digest
probes no sealed segment. About 10 to 12 bytes an event, 110 to 130 MB for
the paper.

## Update, 11:27Z: repaired

Cause confirmed from the code: `internal/taperepack/join.go` appended legacy
events one first-digest-byte bucket at a time, so consecutive events sit in
unrelated segments. dd43ff6b1 (the digest lookup, both seats accepted on
round 2) is deployed. Dry run ts-20261113T111247Z-a6fb14f1 writes a paper
chunk row every 65 seconds, about 12 times faster; the remaining walk is
about 3.5 hours if the rate holds. Status: repaired for the walk; open until
a dry run finishes and its manifest is read. The count of absent or
legacy-only events is printed when the paper's walk ends. `Reader.Read`
itself still lists the directory and probes every segment for callers that
do not use a view.
