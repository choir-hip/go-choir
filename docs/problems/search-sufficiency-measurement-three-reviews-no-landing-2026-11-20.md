# The search sufficiency measurement (L2) was sent back three times and is stopped

Recorded 2026-11-21T00:14Z by the lead. Lane `search-leads-l2`, last commit `260acad32`.
Nothing of it is on main. The contract's rule 7 (three rounds without
convergence) stops the patching.

## Problem

L2 of `docs/definitions/corpus-first-search-one-verb-2026-11-19.md` was to
measure, with no behaviour change, how often the corpus already held a
sufficient answer when a research run sent a web search: one log line per
search and a replay script that says what share a rule would have held.
Three reviews by seat 2 each found that the number it would produce was wrong
in a new way.

## Evidence

1. Review 1 (`1e925aa37`): the conversation's leads were not read and words
   were not prepared as the guest prepares them; "same terms" could be true
   for a different query.
2. Review 2 (`399f0268a`): those were closed. New: one 30-minute reuse
   window where the design says 10 for news words; one failed index build
   turned the conversation read off until restart, silently.
3. Review 3 (`260acad32`): those were closed. New: the measurement starts
   when the search starts and reads with no "as of the search's start"
   bound, while the search writes its own leads and its own query time when
   it ends. A first-ever search can log `same_terms=true age=0` and count
   its own 20 results as held leads; a real repeat's age is overwritten to
   zero. The reviewer reproduced it. And the commit's tree reverted later
   work on main (the About Build row, the no-carry-over rule in CLAUDE.md,
   the new goal file, the semantic-layer documents, ledger rows C182 to
   C184): a rebase had written an older tree. Had it landed it would have
   undone a day of main.

Reviews are in the session scratchpad (`review-l2/r1`, `r2`, `r3`).

## Belief

Each round fixed what was named and exposed the next consequence of one
choice: measuring from the side, after the fact, a decision the search path
makes in the moment. A side measurement has to reconstruct what the corpus
held at the instant of the search from tables the search itself is changing.
The honest way to know "what did we hold when we asked" is for the search
path to read it once, before it sends, and record what it read with the
decision. That is the one verb (L3), and under the goal of 2026-11-20
(evening) it is part of the semantic layer's work: a run starts from what the
paper holds about its happening, and the read that tells it so is the same
read that is logged.

So the lane is not patched a fourth time. What it built that is sound and
reusable: `internal/captureterms` (one word-preparation function for guest
and host), the conversation index on `search_leads`, and the replay
script's shape.

## Remaining error

- Whether the corpus would have been sufficient for some share of today's
  3,200 searches is still unmeasured. The duplicate-story measurement gives
  17% from duplicates and follow-ups alone.
- The builder was asked how the tree reversion happened; a brief for any
  rebased lane must now say: after a rebase, `git diff origin/main...HEAD
  --stat` must list only the lane's own files.

## Next

The read-before-send with its record is designed inside the semantic layer
(`docs/design/semantic-layer-2026-11-20.md`, the `choir.World` surface and
the first-run-from-what-is-held landing), not as a side measurement.
