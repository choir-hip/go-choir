# The Texture root desk was sent back six times and is stopped for the owner

Date: 2026-11-20. Station W2 of the mission "Exact, search and entities".
Lane `root-desk-r3`, last commit `7a6d66dc4`. Nothing of it is on main.
Claim C165 (the root desk lands and proves on d@e, p 0.5) stays open until its
settle-by time and is expected to settle false.

## Problem

Turning the Texture root desk on everywhere needs it to digest every changed
document exactly once, with no document lost and no run storm. Six versions
have been written. Each was sent back by review. The contract's rule 7 (three
rounds without convergence: stop patching, change the approach, then the
owner) was applied after round four by changing the structure. The changed
structure was sent back twice more. The work stops here.

## Evidence

- Rounds one to four kept two authorities (pending sets and flags in memory,
  and the store) and were sent back for the ways they disagreed.
- Round five (`cf31b8d2d`) made the store the one authority with a durable
  watermark. Both seats sent it back: the cut-off was stamped after the read,
  read errors still moved it, more than 50 changed documents were dropped
  silently, restarts stacked runs.
- Round six (`7a6d66dc4`) answered each of those. Both seats sent it back
  again (reviews in the session scratchpad, `review-root6`). They agree on
  three blocking defects:
  1. The ledger is cut at 32 KiB after the watermark is computed, so
     documents the model never saw are marked digested.
  2. One unreadable document holds the watermark, and with more than 200
     documents changed after it the mark list evicts its oldest entries, so
     the same 50 documents are read every hour forever and newer ones never.
  3. The one-minute overlap is a guess. A write's time is stamped before the
     write is visible, on a serialized connection with a 30 s busy timeout; a
     queued or retried write can land behind the watermark and is then lost.
- Both seats say the same thing about the design: one authority in the store
  is sound, and a wall-clock time is the wrong cursor.

## Belief

The defect is not in the patches. The store has no per-document sequence
assigned at commit, so any cursor built on `updated_at` needs an overlap, and
any overlap needs a mark list, and the mark list needs a bound, and each bound
is a new way to lose or repeat a document. Six rounds found six sets of
boundary conditions of the same missing thing.

The fix is in the substrate: a monotonic sequence assigned when a document or
its trajectory commits (the store already numbers lifecycle and reducer
events; the object graph does not expose one number per document change), a
scan by that sequence, a per-document retry so one broken document cannot
hold the rest, and stories excluded in SQL. That is a change to the store and
the object graph, red, with its own design and its own two seats. It is not a
seventh patch.

## Remaining error

- I have not verified that a commit sequence can be added to the object graph
  without a migration of existing rows that is itself risky on the paper
  (about 35,000 documents).
- The lead decided during round six that the root desk does not digest
  stories. The owner has not ruled on that.
- Whether the root desk is still wanted on every computer, given what it
  costs to make it correct, is the owner's question.

## What the owner is asked

1. Build the commit sequence in the store and object graph first, then the
   root desk on it (the lead's recommendation; several days, red).
2. Or leave the root desk where it is today, on the computers that already
   have it, and remove W2 from this mission.
3. Either way: should the root desk digest stories?

Default taken while the owner sleeps: nothing lands; the lane is kept; no
further rounds.
