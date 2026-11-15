# Autoradio turn replies are returned by the desk pack, and calibration claims enter server-side scans (2026-11-15)

Status: open, recorded with station L1a of
docs/definitions/choir-autoradio-learning-engine-2026-11-14.md; not fixed there.
Class when fixed: orange (a read path of commitment records). Owner: the
station that first writes more Autoradio records (L1), unless the owner rules
otherwise.

## Problem

Every Autoradio record carries an owner-private label (design 12.2), and the
store now refuses to append one without it. A label is not a gate on reads.
Design 6.3 says "no other read path (generic claims, commitment listings by
addressee, other desks) returns them". For the calibration kinds (forecast,
settlement) that holds, because the desk pack query leaves out any record with
a `calibration` body. It does not hold for the records of the existing turn
route (`autoradio.turn`), and will not for kinds the later stations write that
have no calibration body.

## Evidence

- `TestAutoradioTurnRecordsReadPaths` in `internal/calibration/review1_test.go`
  writes the two records the turn route writes
  (`internal/agentcore/api_autoradio.go:360-425`: a directive notice with the
  autoradio addressee, and a report with none) and reads them through every
  store read path. Seen to pass on the build VM: the turn request is excluded
  from `ListCommitmentClaimRecords` (it is a directive); the turn reply, a
  plain report with the `autoradio_turn` label, is returned by it. The
  addressee listing of another desk and the generic claims routes do not return
  either. The test pins the reply as returned, so a fix is noticed.
- `ListCommitmentClaimRecords` is the query that fills a desk cell's pack
  (`internal/store/commitment.go`). Its comment says it drops directives,
  calibration records and sub-calls only.
- Server-side scans read calibration claims too: `ListCommitmentRecords` with
  no addressee returns them to `agentcore/calibration_feedback.go` and
  `inert_judge.go`, and `types.InertRates`/`InertCounts`
  (`internal/types/inertness.go:106-118`) count a claim as a request
  precommit under desk "agent". Nothing returns their text to a route, but the
  DJ's forecasts will enter the fleet inert view once L1 writes them.

## Belief

The turn reply (the radio's sentences to the listener) reaches any desk pack
that reads the owner's newest claim records, which is an owner-private record
shown to desks that have no lease for it, and a rule-2 gap. About 70%: the
pack's consumers may filter by label downstream (not read). The inert-view
count is a distortion of a fleet statistic, not a disclosure, about 85%.

## Remaining error

Not read: whether anything downstream of `ListCommitmentClaimRecords` filters by
label or by addressee before a model sees the record; whether a deployed desk
pack has in fact contained a turn reply (read it from a live pack on d@e before
fixing). The fix is a filter in the pack query on the `autoradio.` record_kind
prefix (and the owner-private labels), plus `Calibration != nil` in the inert
scans; each needs its own test and its own mutation class.
