# Store Split Closing Report 2026-11-03

Written early on Tuesday, November third. It covers the store split mission from Sunday evening, November first, to this morning, about a day and a half, and it replaces the checkpoint notes in the ledger.

The store split did what it was for: the paper reads about a third as much from disk for the same work, and its big read storms are gone, but its quiet minutes now read more than before.

Here is where it stands. The paper and your computer run on the new layout, where every record keeps its small header apart from its large body. The safety copies of the old store are deleted, as you asked, which freed about twenty two gigabytes inside the paper's disk. The mission is settled as complete. One finding did not go the way I expected. The typical minute used to read about three megabytes and now reads about forty. I filed that as its own problem with a next probe, because the bigger numbers all moved the right way and I do not yet know whether the split caused the floor or tonight's extra work did. There is nothing you need to decide. If you want one thing checked, it is whether the paper's disk gives the freed space back to the host, which it has not yet.

## What the change was

Before this mission, the paper kept each record's searchable fields after its body. Any question about a record's kind or age made the database read the whole body to get there. The nightly cleanup of old telemetry read about eight and a half gigabytes to remove a few hundred rows. The split moves the bodies into their own table, so those questions touch only the small headers.

On Sunday night I designed it, built it, and tried it on a copy of the paper's store, where it took under nine minutes. Monday it ran on the test computer, then on the paper, which migrated at boot in under ten minutes, and on your computer in eight seconds. Both reviewers found one real defect, in how the smaller telemetry files were migrated, and the fix passed its re-review.

## What the meter day showed

I compared a full day on the new layout, from five in the morning Monday to five this morning, against the evening before the split.

The paper made about the same number of model calls a minute in both stretches, thirty two against twenty eight. For each call it now reads about two megabytes from disk, down from almost seven. The average minute fell by about two thirds, and so did the busiest tenth of minutes. The telemetry cleanup now reads between half a gigabyte and one and a quarter gigabytes, instead of eight and a half. The first fifteen minutes after a boot read about two gigabytes, where they used to read about five and a half. The paper's memory peak fell from almost eight gigabytes to under five.

The median went the other way. The quiet, typical minute rose from about three megabytes to about forty. My guess is that the paper now rereads its hot index pages steadily against a memory ceiling that keeps its cache small. That is a guess, not a finding. The next probe is to line up one hour of reads against the statements that caused them.

## What to remember

The volume claim held, and the cost of a single question about a record no longer depends on how big that record is. The lesson is the same one this work kept teaching: the measurement after the fix showed something the fix did not predict.

If I were holding the phone, I would let the next probe run during the day, and I would check whether the guest discards freed blocks, because twenty two gigabytes freed inside the paper are not yet free on the host.

## Names and receipts

Goal file docs/definitions/choir-store-header-body-split-2026-11-02.md, settled complete, with its ledger beside it. Successor problem docs/problems/paper-steady-read-floor-after-split-2026-11-03.md. Meter file /var/lib/go-choir/growth-meter/mem-2026-11.jsonl on Node B, paper VM vm-1669020adcece51c2f5ff9b6df399ef5. Baseline window 2026-11-01T19:32Z to 2026-11-02T04:00Z; meter day 2026-11-02T05:00Z to 2026-11-03T05:00Z. Read per minute excluding boots: median 3.1 to 38.7, mean 75.8 to 49.4, p90 132.7 to 104.5 MiB. Inference requests 14,375 and 46,859. Finalize freed 22,382,874,624 bytes on the paper and 473,993,216 on the owner's computer. Partition fix b80fe36bd, Devin re-review approve. Migration commits 1d33cafce through 81f09e677.
