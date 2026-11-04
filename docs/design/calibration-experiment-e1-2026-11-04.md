# Calibration experiment E1: the protocol (2026-11-04)

Status: design and protocol (green). The mechanisms it names are built in
the commits that follow this one (red: they change what a round records on
the tape and what run acceptance reads). E1 is runnable once they deploy; it
is **not started**: no computer holds an E1 arm until the owner's
precondition (below) is met and the arms are set.

Source: [calibration-wiring-permutations-2026-11-04.md](calibration-wiring-permutations-2026-11-04.md)
section 11, wave 1 (E1-M and E1-E), with the sample-size arithmetic of its
section 9. Seed design: [calibration-seed-2026-11-03.md](calibration-seed-2026-11-03.md).

## Precondition

1. C2 to C7, C9 and C11 of the permutations report are deployed to the five
   test arms (claude, deepseek, luna, pool, crouton). C1, C8 and C10 wait on
   the owner and are not part of E1's baseline.
2. A 12-hour window with no other prompt, model or calibration change on the
   five arms, logged in each arm's ledger.
3. The arm settings below are set on each arm by the owner's act (the
   computer settings route). Nothing else starts E1.

## The arm assignment (an experiment arm, not a feature switch)

Every arm is a computer setting the computer holds. A computer that holds
none is not in E1 and runs the shipped behaviour: engineering's digest is
pushed to half its rounds (the cut-2 A/B), and management states its Cast
predictions once. That default is the comparator, not a disabled feature;
the settings are registered in `switches/registry.yaml` as `life:
experiment`.

| Setting | Arm it assigns | Default |
|---|---|---|
| `calibration.e1_restate` | E1-M: management restates each engineering Cast's predictions once (two-stage) | off: one statement per Cast |
| `calibration.e1_pull` | E1-E (b) pull: no digest turn; `choir.Calibration()` returns the record to every engineer run | off |
| `calibration.e1_stub` | E1-E (c) stub: a one-to-two-line headline (about 35 tokens) pushed at activation start, naming `choir.Calibration()` for the rest; every engineer run | off |
| `calibration.e1_two_stage` | E1-E (d3) two-stage at the engineer's first predicting cell | off |
| `calibration.e1_baseline` | E1-E no seed: nothing pushed, nothing to pull | off |

None of the four engineering settings on means (a) push, the default. More
than one on is a misconfiguration: the computer falls back to push and logs
it, and every record says which delivery it actually got
(`calibration_delivery`), so the readout never trusts the setting.

Planned assignment (report 11, wave 1): claude push (no setting), deepseek
`e1_pull`, pool `e1_stub`, luna `e1_two_stage`, crouton `e1_baseline`;
`e1_restate` on all five.

## E1-M: two-stage restatement at management's Cast

Mechanism. Management's engineering Cast states the round's predictions
(p0) exactly as today; they are recorded at the open as
`<cast>:predict:<kind>` with `calibration_stage=p0`. On a computer with
`calibration.e1_restate`, the cell result that committed the Cast carries a
restatement request. In the **record** arm it shows management's own record
for each claim kind it stated (from its Cast claims only, held-out
excluded): n, mean stated p against the share that held (for a clock kind, the share
whose interval held), Brier, resolution, and for the two clock kinds the
median of estimate over actual minutes. In the **blind** arm the request has no record. Both
arms ask for exactly one restatement:

```go
choir.Restate("<cast record id>", map[string]any{"verify_pass": <p>, ..., "freeze_minutes": map[string]any{...}, ...})
```

The restatement takes the Cast's predictions block, strictly decoded
(every mandatory kind). It is recorded as p1, `<p0 record id>:restate` (so `<cast>:predict:<kind>:restate`), with
the same rule, assignment and clock anchor as p0, so the rule scorer settles
p0 and p1 from the same event. The arm is the Cast's (C2: salted hash of the
Cast record id), so it is independent per Cast and of the engineer's arm.
The engineer never sees either statement (P3).

Unit: the Cast. Randomisation: `calibration_arm` of the Cast (digest =
record shown, control = blind), stamped on p0 and p1.

## E1-E: the engineering delivery screen

One delivery per computer, each against no seed:

- **(a) push** (claude): the cut-2 A/B, unchanged; arm by the engineer's
  thread (`calibration_arm_unit=thread`).
- **(b) pull** (deepseek): no digest turn. `choir.Calibration()` returns the
  record to every engineer run; the read marks (C6) say whether and when
  it was read. Every run is treated (`calibration_arm=digest`).
- **(c) stub** (pool): a short headline pushed at activation start, built
  from the same record, naming `choir.Calibration()` for the rest. Every
  run is treated.
- **(d3) two-stage** (luna): the engineer's rule Precommits are p0 (`calibration_stage=p0`). The
  result of the cell that first commits them carries a restatement
  request; in the record arm (by the engineer's thread hash) it shows the
  engineer's own record for those kinds, in the blind arm nothing. The
  engineer restates each kind once, `choir.Restate("<kind>", map[string]any{"p": <p>})`
  (a clock kind may restate its estimate and interval), recorded as p1
  `<p0 record id>:restate`. Pull returns nothing here, so the record
  reaches the run only through the request.
- **baseline** (crouton): nothing pushed, nothing to pull.

## Failure modes, written before the code

- F1 The restatement is not a second look but a second claim that doubles
  n. -> p1 is left out of every digest, say-do view and round receipt; only
  the claims API lists it, with `calibration_stage=p1` and `restates=<p0>`.
- F2 A restatement made after evidence is visible flatters p1. -> p1 is
  marked late when the candidate froze, when (engineer, clock kind) the run
  started a build or shot job, or when it comes more than 15 minutes after
  the Cast (management); a late p1 is kept and excluded from the primary.
  A restatement of a first statement that already settled is refused: its
  p1 would never settle.
- F3 The two arms differ in more than the record (the second look itself
  moves p). -> both arms are asked to restate; the arm decides only whether
  the record is shown.
- F4 A desk restates twice, or restates a claim it never stated. -> one
  restatement per p0, refused otherwise with the reason; no p0, no p1.
- F5 The restatement request is missed because it rides a result the desk
  does not read. -> it is the committing cell's own result, beside the
  receipts; the read marks and the p1 rate measure uptake.
- F6 The arm setting is wrong or ambiguous. -> each record stamps the
  delivery it actually got; two engineering settings on falls back to push
  and logs.
- F7 Model and mode are confounded across computers (E1-E). -> E1-E only
  prunes; the confirmatory comparison is E2, fleet-wide.
- F8 The record shown includes the claim being restated or held-out
  outcomes. -> it is built from settled, non-held-out p0 observations only;
  the open round's claims are pending, never settled.
- F9 A crash between the Cast and the restatement. -> the run ends (a crash
  never resumes), p0 stands alone and the pair drops out as unpaired.

## Metrics

Primary, E1-M and E1-E (d3): the paired difference in differences of
Brier, `(Brier(p1) - Brier(p0))` record arm minus blind arm, over the binary
claims, on held-out-eligible p0 (every pair whose p0 settled and neither is
late). Co-primary for the clock kinds: the change in absolute log ratio of
estimate to actual, `|ln(est1/actual)| - |ln(est0/actual)|`, record minus
blind, on settled, uncensored pairs. Report beside them, per claim type: the
resolution and sharpness (mean |p - 0.5|) of p1 against p0, so a uniform
shrink toward 0.5 is visible; the restatement rate (p1 per p0); the mean
|p1 - p0|.

Primary, E1-E (b) and (c): `read_before_predict`, the share of treated
engineer runs whose first rule claim carries `calibration_read_at`;
go/no-go at 0.25 after 20 treated runs. Secondary: `read_any` and
`reads_per_run` (the `calibration.read` events), `read_latency_s`.

Primary, E1-E (a): mean bias (stated p minus outcome) of the engineer's own
claims, digest arm against control, accumulating toward E2.

Cost, all arms: tokens per round (run usage), and for two-stage the
restatement turn's tokens against the round's management tokens.

## Minimum n

- E1-M: 112 Casts with a settled, non-late pair, in all (56 per arm),
  assuming the sd of the round-mean paired difference is 0.04; recompute
  from the first 20 pairs and keep the larger n. About 28 hours at the
  fleet's four rounds an hour.
- E1-E reads, (b) and (c): 20 treated engineer runs per computer (about 25
  hours at 0.8 rounds an hour).
- E1-E (d3) on one computer: about 140 hours for its full readout; E1-E
  only screens it, E2 confirms it.
- Inconclusive is reported as inconclusive. No early stop for success.

## Harm stops

Checked at each readout; any one stops that arm and returns its computer to
the default:

- Brier on a treated arm worse than its comparator by 0.03 over at least
  40 rounds.
- Tokens per round up more than 10% (5% for push and stub).
- The restatement turn above 5% of the round's management tokens (E1-M).
- Late share up 10 points.
- Time from the Cast to the engineer's first build up 20% (a read or
  restatement crowding out work).

## Readout with the claims API

`GET /api/calibration/claims` on each arm (the owner's read; no desk reads
it) lists every rule claim with its settlement. The fields E1 reads:

- `id`, `stated_by` (`cast`, `precommit` or `restate`),
  `calibration_stage` (`p0`, `p1`), `restates` (a p1's p0 id);
- `arm`, `arm_unit` (`cast` or `thread`), `calibration_delivery` (`push`,
  `pull`, `stub`, `two_stage`, `baseline`), `record_shown` (a p1's arm:
  `true` record, `false` blind);
- `p`, `estimate_minutes`, `low_minutes`, `high_minutes`, `kind`,
  `assignment`, `cast`, `late`;
- `answer` (`yes`, `no`, `unresolved`), `actual_s`, `censored`,
  `censored_unscored` (C7: left out);
- `calibration_read_at`, `calibration_reads` (C6).

Pairs: join each p1 to its p0 by `restates`; keep pairs with both answers
settled to the same `answer` (they share the event), neither `late`, and
`censored_unscored` false. `GET /api/calibration?detail=1` gives the held-out
mark per observation (p0 only); the pair's held-out set is p0's.
`choir trace export` carries the same claims (`calibration_claims`) and the
read marks (`calibration_reads`) per trajectory, for audit.

## Stopping E1

Clearing the settings ends it (the rollback); a code revert of the E1
commits must keep the p1 filter in the digest readers, or the p1 records
already on the tape would count as second claims (F1). Once cleared, the claims stay on the tape with their
stage and delivery marks. A stopped arm's computer runs the default from
its next activation.
