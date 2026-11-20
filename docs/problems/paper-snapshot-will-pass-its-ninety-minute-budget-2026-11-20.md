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
