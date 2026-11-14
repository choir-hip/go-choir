# The store-tables reading cuts the big tables on the one computer that needs it (2026-11-14)

Status: open. Mutation class of this doc: green.

## Problem

`GET /api/runtime/observability?store_tables=detail&store_tables_seconds=100`
(`internal/storetables/storetables.go`, served from
`internal/agentcore/api.go`) shares its wall-time budget across every b-tree
by a flat weight (table 4, index 1; about 126 trees). On a 47 GB state.db each
big table gets about 2 s and is cut. The reading cannot answer "what fills the
store" on the computer where it matters.

## Evidence

Staging, origin/main d4a27b2a2, the paper's 47.4 GB state.db: the reading saw
4.23 GB (8.9% of the file) in 67 s. og_bodies, run_memory_entries, og_objects,
og_key_index and about 25 indexes were incomplete. og_bodies alone walked
1.09 GB in 2.4 s (about 460 MB/s), so each of the three big tables could be
read whole inside one budget if it were given the budget.

## Belief

The cause is the flat share: the budget is divided by tree count, not by where
the bytes are. Confidence high (the walk rate and the cut sizes agree with the
share arithmetic). MaxTreeRead (15 s) is a second cut: at 400 MB/s it ends a
tree at about 6 GB, so a table over that is a floor however the budget is
shared; that is a separate, smaller limit and is kept for WAL pinning.

## Remaining error

- Whether the three big tables fit one budget each is untested on the paper.
- dbstat cannot be read in rowid ranges, so a tree over about 6 GB cannot be
  read whole by any request shape; only its floor is available.

## Fix

A request may name the trees to read (`store_tables_only=a,b`) so the whole
budget goes to them; without it, a cached earlier reading orders and weights
the trees by the bytes it saw.
