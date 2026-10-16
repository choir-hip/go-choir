# Two records of one event go to two intake tasks, which open two stories

Found 2026-10-16 about 02:07 UTC on the V3a test computer `v3-events-0146`
(build 4b66f9ef2; design
docs/reports/event-layer-at-ingestion-design-2026-10-16.md §15–§18).

## Evidence (the ledger)

- Management's inbox for event `ev-69ac2c3d040ad3f0ecf5` (the Bend,
  Oregon school shooting) holds two records:
  - seq 26, `event-corroborated:ev-69ac…`, which wakes management: Al
    Jazeera, DW and DW, 3 items;
  - seq 29, `event-grew:ev-69ac…:75ae…`, held: 2 BBC items from a later
    poll.
- One management turn read both and called
  `Delegate("intake", …, "per_record")`. That gave two tasks:
  - `mt-18579cb9…` got the 3 corroborating captures and opened story
    50222ec3 ("Bend, Oregon elementary school shooting kills four",
    02:02:08);
  - `mt-80ac996b…` got the 2 BBC captures and opened story f166f47c
    ("Four killed in Bend school shooting", 02:02:03).
- So one event became two stories, 5 seconds apart. Neither task knew the
  other was opening the same event. The story-library head check (R2-5)
  did not refuse the second open.

## Classification

This is substrate, in V3a. The design made two records for one event
(corroboration, then growth). `Delegate`'s `per_record` default gives
each record its own task. The overlay never says records sharing
`Event.EventID` are one event. V3b's `Covered` does not help: neither
record's captures were disposed of when the growth record was minted.

## Fix direction

`Delegate` puts captures records of one event (the same
`Captures.Event.EventID`) in one task, whatever the grouping, when they
fit in `MaxTaskCaptures`. The overlay says records sharing an event id are
one event. A test: two records of one event, delegated `per_record`, give
one task.

## Fixed, and the residual

The fix is f81e729c5: `Delegate` gives one event's records one task.

Residual: a growth record minted while the first task is still opening
its story carries no `Covered` yet, because the capture is leased but not
disposed of. A later turn may delegate it to a second task. The R2-5
story-library check refuses an open from a task that has not seen the
first open, which narrows the window. If the test computer or the paper
shows it, the next step is to annotate `Event.Leased` (the task holding
the event's other members) from `CaptureDisposition.LeaseTask`.
