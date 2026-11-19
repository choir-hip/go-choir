# The vector worker's first statement never ends on the paper (2026-11-19)

## Problem

The vector index and the ranked search deployed at 12:03Z (`ad0f7856c`).
Twenty-five minutes later nothing had been embedded: no `embed_gap` line, no
`pass start` line, only the sweep's line at 12:01:20Z. The worker computes
both gaps before it embeds anything, and its capture gap count was still
running. An import file placed in the inbox would have waited behind it too:
the inbox is read at the start of a pass, and the pass never ended.

## Evidence

- `pg_stat_activity` on Node B at 12:26Z: one statement and its two parallel
  workers, running 24 minutes 52 seconds: `SELECT COUNT(*) FROM
  proj_capture_index v WHERE EXISTS (... og_objects o ...) AND NOT EXISTS (...
  vec_captures ...)`.
- Its plan (`EXPLAIN`, read only): the EXISTS is pulled up into a join driven
  from `og_objects`. A bitmap scan of the web captures, estimated at 2 rows
  after the filters and really 221,335, feeds a nested loop whose inner side
  is an index scan of `proj_capture_index` on (owner, computer), which
  matches every one of the computer's 216,942 captures and filters on the id.
  221,335 times 216,942 row checks.
- With `OFFSET 0` inside the EXISTS (the planner's fence), measured on Node B
  read only with `EXPLAIN ANALYZE`: a sequential scan of
  `proj_capture_index` with one primary-key read of `og_objects` per row,
  221,335 eligible, **2.3 seconds**.
- The searches were unaffected because the vector tables were empty: the
  ranked answer's lines show `vector=0 vector_state=exact`, a millisecond
  each.

## Belief

The predicate was written as a filter and the planner turned it into a join,
choosing the driving side from an estimate that is wrong by five orders of
magnitude: it has no statistics for `COALESCE(superseded_by, '') = ''` or for
a substring test on the metadata, and multiplies their default selectivities.
Every test ran on a database of a few rows, where any plan is instant. The
design measured recall and embedding rate on the Mac; no statement of the
worker was ever planned against the paper's tables before it deployed. The
plan could have been read on Node B, read only, any time after the host half
deployed.

## Fix

`vecCaptureEligibleSQL` ends in `OFFSET 0`. One test holds the shape for the
five statements built on it. Ledger claim C156.

## Remaining error

- The orphaned statement may outlive corpusd's restart; it is looked for
  after the deploy and cancelled if still running, and the report says so.
- The search's own list statements use the same predicate over
  `vec_captures`. Their plans cannot be read until the table holds the
  paper's rows; they are read (`EXPLAIN ANALYZE`, read only) after the import,
  before any claim about the ranked search's latency.
- One probe per capture per pass is 2.3 s of one core every pass on the
  paper. The eligibility facts belong in the projection row, not in a
  substring of the object's metadata; not changed here.
- The story predicate has the same form over a few thousand rows; it passed
  on the paper (the pass reached the capture count), and is not fenced.

Written 2026-11-19T12:29Z.
