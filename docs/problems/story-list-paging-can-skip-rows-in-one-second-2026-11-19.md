# Paging over published stories can skip rows that share a second (2026-11-19)

## Problem

The front list, the pieces index and the pieces search page over published
stories by a cursor of `(updated_at, canonical_id)`, newest first. The cursor
keeps `updated_at` as whole seconds (`frontCursor.UpdatedAt`, an `int64` of
Unix seconds, `internal/platform/publication_front.go`). The SQL that reads
the next page compares the cursor with the row's full-precision
`updated_at`. A row whose time is later than the cursor's truncated second
but earlier than the last row actually read falls on neither page.

So when a page ends inside a second that holds more rows, the rest of that
second's rows can be skipped: never listed on the front's later pages, never
indexed by the pieces index pass that walks the same way.

## Evidence

- Found by review seat 2 of the vector change on 2026-11-19, reading
  `publication_front.go` (the cursor type and `frontPublicationWindowSQL`).
- Reproduced by the vector builder in a test: eight publications in one
  second across a batch boundary, walked with the whole-second cursor, and
  the walk saw three of the eight
  (`TestVectorStoryWalkSeesEveryPublicationOfOneSecondAcrossABatchBoundary`,
  which fails without the fix and passes with it).
- The callers that build the whole-second cursor today:
  `publication_front.go` (two places), `publication_pieces_index.go`,
  `publication_pieces.go`.

## Belief

Real and small in production: the paper publishes a few stories a minute at
most, so several in one second at a page boundary is rare, but a backfill or
an import writes many rows in one second, and then a walk loses most of
them without any error. Not measured on the paper.

## What was done, and the remaining error

Fixed (lane `paging-skip`, orange): one cursor form for every caller. The
cursor always carries the row's full-precision time (`u` whole seconds, `n`
nanoseconds) and the `canonical_id` tie-break the SQL orders by, built only by
`frontCursorAt`. The two front list loops, `scanPublicationRows` and the
pieces index walk no longer build a whole-second cursor; that path is deleted.
Tests (`publication_paging_second_test.go`) put twelve stories in one second,
some with identical times, and page them through the front list, the section
list, the scan and the index: each row once.

A cursor minted before this fix (no `n` member) cannot say where in its second
the page ended, so it is read as the end of that second: the next page starts
at the newest row older than the following second. Rows of that second that
were already listed may be listed again once; none is skipped. The cursor is
opaque to its clients (the web app, the guest's research tools and management's
listing hand back the string they were given; only `internal/platform` parses
it). A new cursor shown to an old process during a deploy is not handled: the
old process reads whole seconds and may skip as it always could.

Remaining: not yet known how many published rows on the paper share a second
with another (one read-only count over `og_objects` would say), nor whether any
story is missing from the pieces index because of it; the index repairs itself
only for rows that change.
