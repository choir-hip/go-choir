# A test computer's replay is not equivalent in self_development_operations (2026-10-23)

Date: 2026-10-23, 07:15 UTC. Found by: FP0, running D10's proof (F-c deploy
1, `a49ae398a`) step 1 and step 5 on test computer computer-71afbf14…
(batching-e2e-1022, class test). Status: open, problem record before any
fix. Mutation class of a fix: red (tape and evidence, self-development).

## Evidence

`GET /api/computers/<id>/self-development/replay-completeness` on
computer-71afbf14 answered in about 20 s, at 07:10 and again at 07:11
after an operator refresh (crash boot). The full report is
`_runs/deployD10/t71-replay.json`.

- `eligibility`: `{manifest_version: 4, eligible: false, reason:
  "reconstructed projection is not equivalent to live state"}`
- `result.status`: `not_equivalent`, with two differences, both
  `kind: dolt_head`:
  - `dolt:texture:content_root`, live `sha256:441f795b…` against
    reconstructed `sha256:0117656…`
  - `dolt:texture:table:self_development_operations`, live
    `sha256:47c0cd36…` against reconstructed `sha256:0bc2031b…`

Nothing in the report names `choir_events`. D10's own checks pass: the
manifest is version 4, and the empty table is neither unknown, missing nor
observed.

## Not caused by D10 (from the diff)

D10's diff (`663023143..a49ae398a`, 32 files under `internal/`) touches
neither `self_development_operations` nor `internal/selfdev`. No reading
from before the deploy exists for this computer, so it is not yet proved
that the difference predates D10.

## Hypothesis (from code, not yet confirmed)

`internal/selfdev/operations.go` writes `self_development_operations` with
raw SQL:

- inserts at lines 263, 385 and 432;
- a state-machine `UPDATE` at line 495;
- `UPDATE ... SET last_intent_error=?` at line 524.

The tape projection also inserts rows (`internal/store/project.go:432`). If
any of these direct writes has no tape event that the projection replays,
the live table and the replayed one diverge. `last_intent_error` and
`updated_at` are likely candidates. A replay that is not equivalent makes
the computer replay-ineligible, and an ineligible computer refuses
checkpoint publication: self-development apply, platform-follow's
post-apply checkpoint and owner recovery checkpoints. So this bears
directly on self-development on real computers.

## Open

- Which rows and columns differ: compare the live table with a replayed
  copy, by operation_id and column.
- Which write paths have no tape event.
- Whether the paper and the owner's computer are also ineligible. The
  paper's report did not answer in 10 minutes (it replays the whole
  chain). The proof should not run that report on the paper: see the D10
  ledger entry.
- Whether the right fix is to put the write on the tape, or to classify the
  column as live-only in the witness manifest.

## Heresy delta

- Discovered: a table in the replay witness has writers outside the tape
  (hypothesis). A read-only proof step defaults to the paper and runs a
  full replay there.
- Introduced: none.
- Repaired: none.
