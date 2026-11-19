# Paging over published stories would skip rows that share a second, if any row carried a fraction (2026-11-19)

**Corrected 2026-11-19T08:15Z.** As first written this record said the front
and the pieces index can skip rows today. They cannot: every write of an
object row cuts its times to whole seconds (`objectTime`,
`internal/platform/objectgraph_store.go`), so for rows written by this code
the whole-second cursor was exact. The second review seat of the fix
established this by reading the writers and probing a real publish. The
defect is latent. What follows is the corrected record.

## Problem

The front list, the pieces index and the pieces search page over published
stories by a cursor of `(updated_at, canonical_id)`, newest first. The cursor
kept `updated_at` as whole seconds (`frontCursor.UpdatedAt`,
`internal/platform/publication_front.go`), while the SQL compares it with
the column, which holds microseconds. A row whose time carried a fraction
would fall between two pages and be skipped: never listed on a later page,
never indexed by the pieces index pass that walks the same way.

No writer in the tree stores a fraction, so no row is skipped today. The
cursor was correct only because of a rule in another file that nothing
tested from the paging side: a migration, a repair or a new writer that
stored the database's own `now()` would have made every walk lose rows
without an error.

## Evidence

- Found by a review seat of the vector change on 2026-11-19, reading
  `publication_front.go`.
- The reproductions (the vector walk's test and the paging tests) write
  rows with sub-second times by direct SQL. That shows what a fraction
  would do. It is not evidence that production holds such rows.
- Read from the code, not from the paper's database: whether any stored
  row carries a fraction. One read-only count would say
  (`updated_at <> date_trunc('second', updated_at)` over `og_objects`);
  it has not been run.

## What was done, and the remaining error

A guard, not a repair (lane `paging-skip`, orange; it lands with the
vectors' Go half). One cursor form for every caller, built only by
`frontCursorAt`: the row's time to the nanosecond and the `canonical_id`
tie-break the SQL orders by. The two front loops, `scanPublicationRows`,
the pieces index walk and the vector walk use it; the whole-second
construction is deleted. Tests (`publication_paging_second_test.go`) page
twelve stories of one second, some at identical times, through each caller
and require each row once.

The cost of the guard: a cursor minted before it (no nanoseconds) cannot
say where in its second the page ended, so it is read as the end of that
second. Rows of that second already listed are listed once more; none is
skipped. A reader of the web front who holds an old cursor across the
deploy can see a story twice once. A new cursor shown to an old process
during a deploy is read as whole seconds, as before.

Remaining: the count above, if anyone wants to know whether a fraction ever
reached the table.
