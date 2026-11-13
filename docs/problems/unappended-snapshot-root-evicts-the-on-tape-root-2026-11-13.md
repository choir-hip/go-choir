# A snapshot root the tape never named can evict the one it did

Date: 2026-11-13. Status: fix built on lane/s4-stage0-roots, in review, not landed (Stage 0 of
docs/design/snapshots-cost-the-change-2026-11-13.md). Found by a
review seat reading the S4 design; confirmed in the code and on Node B.

## Problem

`TakeSnapshot` registers a root with the host before it appends
`snapshot_recorded` to the tape (`internal/snapshot/snapshot.go:335-364`).
`recordSnapshotRoot` prunes to the retention count in the same transaction,
by head sequence (`internal/platform/snapshot_roots.go:90-101`); the paper's
retention is 2. Boot restore only uses snapshots the tape names
(`internal/autoputer/projection_base.go:165`), but the prune and
`RecoveryBase` (`internal/platform/file_cas.go:80-92`) count every row. Two
registrations in a row whose appends fail leave only roots the tape does not
name: the newest restorable root is evicted, the collector takes its chunks
after the unrooted window, and the host still advertises a recovery base
that boot cannot use. A rebuild would then need a replay from genesis, which
the 720,000-event recovery bound refuses for the paper.

## Evidence

Node B, 2026-11-13T19:10Z, read-only (each kept root, and whether a
`snapshot_recorded` receipt names it): 22 of 23 kept roots are on the tape.
The exception is the paper's second root, 4861382b769b (head 10,914,341,
2026-11-12T19:18Z): registered, never appended. So the paper holds exactly
one restorable root (a12c1997b1f7, 23:28Z) and one that restores nothing.
On 2026-11-12 appends failed at 16:11Z, 18:55Z and 19:18Z
(docs/evidence/paper-chunk-drain-trace-2026-11-12.md); for about four hours
both kept rows were probably off the tape.

## Belief

One more failed append leaves the paper as it is now (the new unappended
root replaces the old unappended one). Two in a row evict a12c1997b1f7. The
snapshot backoff (20 minutes to 3 hours) and the 24 hour unrooted window give
about a day between the second failure and the chunks going.

## Remaining error

Whether the 2026-11-12 window actually collected an on-tape root's chunks is
not established (the root-time chunk check, afe50a926, landed after it).
Whether `bundle.Import` carries receipts with its roots, and whether receipt
compaction can delete the naming receipt, must be checked before the fix
lands.

## Stage 0 build notes (2026-11-13)

- Checked: `bundle.Import` carries the naming receipt with its root (verify
  refuses a bundle whose root is not named, `bundle/verify.go:170`); receipt
  compaction rewrites only the body column and no code deletes receipts except
  the destroyed-computer purge, which deletes roots with them. Tests N9, N10.
- Corrected and closed: with no named root the watermark route answers 404,
  and `recoveryplan.PlanRecovery` refuses an empty-store start with no base,
  while `projection_base.go` (165-181) finds no candidate and replays a chain
  of at most `MaxRecoveryTailEvents` (720,000) from genesis. The first version
  of the fix let that refusal stand, which refused for good a boot that
  completes (the computer cannot boot to take the snapshot that sets a base).
  `deterministicEmptyStoreRefusal` now admits when there is no watermark and
  the chain is within the bound, and refuses above it (the paper's
  10,914,341 stays refused).
- Added after review: unnamed roots registered within a 2 hour window (90
  minute capture budget plus 30 minutes) are kept: the root just registered
  (never pruned in its own transaction), the newest above the named roots, and
  2 more, so up to four unnamed rows, as many as main's newest-four rule at the
  small store's retention. An append that lands after a newer registration
  still finds its row. The named roots are ordered by the sequence of the
  event that names them, not by head: boot restore tries the newest naming
  event first, and after a head inversion (a point-in-time restore) the roots
  named last have the lower heads. `RecoveryBase` uses the same order, for the
  same reason.
- Open: `bundle.Import` does not prune, so many imports leave many rows; the
  collector reads every row's chunk list.
