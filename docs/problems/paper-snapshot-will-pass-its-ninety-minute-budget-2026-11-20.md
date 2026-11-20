# The paper's snapshot will soon take longer than its budget, and then none will finish

Recorded 2026-11-20 06:35Z by the lead, from the snapshot design's reading
(`docs/design/snapshot-without-scratch-copy-2026-11-20.md`). Not fixed.

## Problem

A capture of the paper's store takes time in proportion to the store's size,
and the capture's budget is 90 minutes. When a capture passes the budget it
aborts and records nothing. The paper's store grows every day, so on its
present course every capture will abort, and the paper will have no new
restore point.

## Evidence

- The last measured capture: 82 minutes 51 seconds for 51.39 GB, which is
  1.61 minutes per GB.
- At that rate a capture reaches 90 minutes at about 56 GB.
- The store grows about 2.4 GB a day, so the wall is about two days from
  2026-11-20.
- The same capture is the one that writes a full scratch copy inside the
  image (`docs/problems/paper-data-image-grew-40-gib-in-eleven-minutes-2026-11-16.md`).

## Belief

The witness (the digest of the whole store read inside the capture) is the
slow part, not the upload. The design's first landing (S1: stream from the
read transaction, no scratch copy) removes the copy's time and moves the wall
to about 65 GB, roughly six days. It does not remove the wall.

## Remaining error

- How the 83 minutes splits between witness and upload is not measured: the
  paper's console had rotated and the producer's status was lost at the
  06:17Z refresh.
- The growth rate is one estimate, not a week's series.
- Whether anything alerts when a capture aborts on its budget has not been
  checked.

## Next

1. Build S1 (red, two seats, proof on d@e).
2. Measure the split at the next capture (the log line S1 adds).
3. Design the witness so its time does not grow with the whole store, or
   decide which gives way at the budget. The owner question is recorded in
   the design with its default (neither gives way; this record stands).

## S1 was built three times and is stopped for the owner (2026-11-20T10:20Z)

Lane `snapshot-stream`, last commit `0873aa1fc`. Nothing of it is on main.
Both review seats sent each of the three versions back; rule 7 of the
operating contract stops the patching here.

What the three rounds found, in order:

1. The first version measured WAL growth wrongly, claimed a 12 GiB bound it
   had no mechanism for, and left the large checkpoint after the pin on the
   store's one write connection.
2. The second moved that checkpoint to a side connection. SQLite's automatic
   checkpoint (1,000 frames, never overridden here) still ran the backfill
   inside the first commit on the write connection, so writers and `/health`
   would stall about 22 s per GiB: about 3 minutes on the paper, under a 1 s
   health budget.
3. The third turned the automatic checkpoint off for the capture and looped
   the side checkpoint. Both seats found the loop ends after its first pass
   while commits keep appending, so the tail still lands on the write
   connection; the two other callers (compact copy, capsule copy) were given
   the loop without the suspension; the suspension is not counted, so two
   callers can restore it under each other; and a store left with the
   automatic checkpoint off has nothing that notices.

Belief. Each round fixed what the round before named and exposed the next
consequence of the same trade: a 71-minute read pin on a store that takes
writes the whole time makes a multi-GiB WAL, and every way of draining it
touches the one write connection or the health read. Review cannot settle
this; a measurement can. The design's own precondition, a capture of a large
store on a test computer before the paper takes it, was never met: no route
exists to make a test computer's store large.

Both seats answered "fit to deploy to every computer at once?" with no.

What the owner is asked:

1. The wall is still about two days away (56 GB at the measured rate). The
   smallest relief is the budget itself: 90 minutes is a constant. Raising it
   buys days, costs nothing in design, and leaves the scratch copy in place.
   The lead's recommendation is to raise it to 150 minutes now (red, two
   seats, a small diff) and treat S1 as a measured project, not an overnight
   one.
2. S1 needs a way to make a test computer's store tens of GiB, then one
   measured capture there, before any further review round.
3. The standing question from the design: when the budget is hit, which
   gives way, the snapshot or the budget?

Default taken while the owner sleeps: nothing lands; the lane is kept.
