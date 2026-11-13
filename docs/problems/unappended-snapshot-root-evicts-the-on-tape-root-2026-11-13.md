# A snapshot root the tape never named can evict the one it did

Date: 2026-11-13. Status: open; fix designed (Stage 0 of
docs/design/snapshots-cost-the-change-2026-11-13.md), not built. Found by a
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

Node B, 2026-11-13T17:25Z, read-only (each kept root, and whether a
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

## Structural assessment after three review rounds, 2026-11-13T18:50Z

`lane/s4-stage0-roots` went to review three times (tips d45f020d0,
32113c79a, f2744bb36). One seat accepted every round; the other sent every
round back. The rounds did not repeat a finding, but each repair opened a new
surface:

| round | blocking finding | where |
|---|---|---|
| 1 | the package suite failed on a receipt-writer tripwire; a late append named a root the prune had dropped | the prune, a test gate |
| 2 | a fresh start with no named root was refused forever | vmctl recovery admission |
| 3 | a computer already holding a stored `base_missing` condition stays refused | vmctl condition re-evaluation |

What every round accepted: the prune keeps the roots the tape names; the
collector protects every kept row; imports and purges stay consistent.

What kept failing: the lane also changed what the host advertises as the
recovery base (named roots only), and that forced changes to vmctl's
admission policy, a different subsystem with its own stored state. Rounds 2
and 3 are both admission-policy defects. The eviction, which is the data-loss
risk, does not need that change: once a named root can no longer be evicted,
the guest finds it on its tape whatever base the host advertises.

Decision: split the lane.

- Lane A, built first: the prune rule, the collector's live set, the
  tripwire, the in-flight window (shortened to the append bound, since
  registration follows the capture). `RecoveryBase` and vmctl admission stay
  exactly as on main.
- Lane B, later and on its own: advertise only named roots, with the
  admission rule and the stored-condition release designed together.

Remaining error on main after lane A: the host can still advertise an
unnamed root's head as the watermark, so vmctl's tail estimate can be too
optimistic for a computer whose newest registration never reached the tape.
That is today's behaviour, not a new one.
