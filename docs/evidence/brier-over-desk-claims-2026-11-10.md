# Brier scores over existing desk claims: the claim inventory, the read path and one real score (2026-11-10)

Mutation class: green (evidence and proposal; no code). Exploration X6, branch
`x6-brier`, from `origin/main` 313e5bae3. No staging access, so no live data
was scored. It continues
[tasks-precommit-icrl-2026-10-24](../archive/learning/tasks-precommit-icrl-2026-10-24.md)
(§7.1 chose Brier; §7.3 the hidden audit),
[calibration-seed-2026-11-03](../design/calibration-seed-2026-11-03.md) and the
archived [ICRL experiment](../archive/reports/icrl-calibration-experiment-2026-10-14.md).
It does not restate them. Its finding is that **the Brier machinery is already
built and running; the open questions are what it can and cannot see.**

Source tags: [C] computed from code I read; [M] measured from a committed file;
[A] assumed.

## 1. What is already built [C]

- `internal/calibration/math.go` and `calibration.go`: `BrierScore(p,y)=(p-y)^2`;
  Murphy resolution over five bins (`<0.5, 0.5-0.7, 0.7-0.85, 0.85-0.95, >=0.95`);
  ECE; window 400, `MinN` 20, `MinBin` 5 (`DefaultOptions`).
- `internal/agentcore/calibration_feedback.go` `collectCalibration` turns the
  tape into `Observation{Desk, ClaimType, P, Y, ModelID, Arm, HeldOut,...}`.
  `calibrationSnapshotFor` caches one build per computer.
- The tasks-precommit design's task claims (`TaskClaim.P`, `task:<tid>:start`)
  are **not built**; only the pre-existing sources below are.

## 2. Which stored fields are claims, and what settles each [C]

| # | Claim field (where stored) | Desk | Settling observation | When known | Settler | Status |
|---|---|---|---|---|---|---|
| 1 | Cast hypothesis `probability` (`types/commitment.go:420`, `CommitmentCast`) | management | outcome of the hypothesis: `Report.Settled[].Outcome` by the protocol, or a `calibration:` score record's `Answers` from an off-computer scorer (`collectCalibration`) | next report (minutes to hours) or when the scorer posts | protocol / independent | built; E6a baseline n=559 |
| 2 | `p_complete` on a final research Report packet (stored as a `self` score) | research | `CalibrationScoreCoverage` probability in a score record (the question coverage a blind judge found) | when the off-computer scorer runs | independent | built; continuous y in [0,1] |
| 3 | `pSupported` on Texture `choir.Write`, on each new citation's check memo | Texture | Jev support verdict memo (`SupportVerdict`) for that citation | seconds to minutes (in guest) | independent (Jev) | built |
| 4 | Rule precommits (`Precommit.Rule`, `YesProbability()`): `verify_pass`, `judge_accepts`, `owner_approves`, `approved_within_rounds`, clock kinds `freeze_minutes`, `approval_minutes` | engineering (derived from management's Cast `predictions`, id `<cast>:predict:<kind>`) | the rule scorer's resolve record (`RuleScorerAgentID`), from the verify result, the owner's accept or reject event, the Freeze commit time | minutes (verify) to days (owner approval) | rule | built; `judge_accepts` can be self-fulfilling (the seed doc says so) |
| 5 | Mission-ledger probabilities (`*.precommit.md`, `p = 0.x`) | the orchestrator and builders, not a computer desk | a quoted observation, settled by rule or by Devin | days | rule / Devin | prose only; read by `cmd/trustharness` |
| 6 | `Predicts:` commit trailers, task claims `TaskClaim.P` | platform authors, engineering | ring walk, receipts | hours to 7 days | rule | **not built** |

Two facts matter for scoring. First, row 2's y is continuous (coverage), so
its "Brier" is a squared error against a fraction, not a binary Brier; it
must be reported as its own claim type and never pooled. Second, rows 1, 3
and 4 differ in who controls the settling observation. Row 3's settler is
Jev, which the desk's own text can influence (the support judge reads the
passage the desk chose). Row 4's `owner_approves` rests on an owner act and
is the cleanest. `DigestExcluded` already drops `paths_confined` (it held 96%
of the time and padded n).

## 3. The computation and its decomposition

For N settled binary claims of one type with stated p_i and outcome y_i:

    BS = (1/N) sum (p_i - y_i)^2
    BS = REL - RES + UNC   (Murphy 1973, over K bins)
    REL = (1/N) sum_k n_k (pbar_k - obar_k)^2     calibration error, lower is better
    RES = (1/N) sum_k n_k (obar_k - obar)^2       resolution, higher is better
    UNC = obar (1 - obar)                         base-rate variance, set by the world
    BSS = 1 - BS/UNC                              skill vs always saying the base rate

With coarse bins the identity holds only up to a within-bin variance term
(the computation in §6 shows 0.194 against a true 0.189 at N=8). The code
reports BS, RES and ECE per claim type and per model already; it does not
report REL, UNC or BSS in the digest. The tasks-precommit design asks for BSS
(§7.1); `UNC` is the number that tells a reader whether a claim type is
informative at all. **Recommendation:** report `UNC` and BSS beside BS for
every type, with "no skill measurable" when UNC < 0.01, as the design says.

Per-type, per-model and per-desk strata only; a pooled score mixes base
rates (Simpson), which `calibration.go`'s header already records as C4.

## 4. Minimum sample [A, with arithmetic]

The standard error of BS is roughly sd((p-y)^2)/sqrt(N). For claims at
p near 0.7 with a 70% base rate, per-claim scores run 0.09 (hit) or 0.49
(miss), sd about 0.18, so SE(BS) is about 0.04 at N=20, 0.018 at N=100. The
difference between a calibrated and a merely hedged desk (BS 0.21 vs 0.25 at
50% base rate) is 0.04, so:

- N=20 (`MinN`) detects only gross failure; it supports a "something is
  wrong" line, never a trend or a model comparison.
- Comparing two arms or two models at an effect of 0.04 needs about N=100 per
  arm per claim type (two-sample, 80% power, [A]). The design's trigger (200
  audited items per desk over 6 weeks) is consistent with this.
- Resolution needs more than Brier: with 5 bins and `MinBin` 5, RES is noisy
  below N=100. Report RES only at N>=100.
- Clustering: claims on one story or one Cast share a state of the world.
  Effective N is the number of clusters; `Observation.Story` exists for the
  story-clustered bootstrap. Use it for any interval.

## 5. The read path from a computer's own tape [C]

`calibrationSnapshotFor` does three reads per computer, cached by age:

1. `store.ListCommitmentRecords(owner, computer, "", 20000)` (the
   `calibrationListLimit`).
2. `ListCitationChecks(..., 20000)` and `ListSupportVerdicts(..., 20000)`,
   read as the `choir.citation_check` / `choir.support_verdict` custodian.

then `collectCalibration` joins settlements to claims in memory. Costs and
bounds:

- Memory is bounded by 3 x 20000 decoded records. At the sizes the harness
  saw (1,107 precommitments on the paper, `ma7-9-harness` report [M]) this is
  far under the limit today, but the E6a baseline cites about 650
  hypotheses a day on the paper, so commitment records can pass 20000 in
  roughly a month. **Risk:** `ListCommitmentRecords` documents its ordering
  as "by canonical record id (stable, content-derived)", not by time. If the
  list truncates at the limit, the kept set is an arbitrary subset, not the
  newest. I did not verify `ogListByOwnerAndBody` ordering ([A]); it is the
  first thing to check on staging, because a truncated snapshot would give a
  confident, wrong Brier with no error.
- The path scans all records to score a window of 400 observations. That is a
  full-ledger scan per snapshot. A kind-scoped read (`ListCommitmentRecordsBySubjectRef`,
  or the `choir.task_open` metadata index design in the tasks doc §2.7) would
  bound it by the window, not by tape age.
- Recommended bound: read settlement records first (one kind, newest first,
  until 400 per claim type or 90 days), then fetch only their parents by id.
  This turns an O(tape) scan into O(window) and removes the truncation risk.

## 6. A real score from committed evidence [M]

The only committed file with probabilities and outcomes is the modular-
architecture mission ledger, scored by `cmd/trustharness`
(`docs/evidence/ma7-9-harness/2026-11-06-59c902c5/report.md`,
`mission-ledger-settlements.json`). Eight probability claims settled
(p, y): (0.80,1) (0.50,0) (0.40,1) (0.75,1) (0.70,1) (0.55,0) (0.60,1) (0.50,1).
Recomputed here, matching the harness's 0.189375:

| quantity | value |
|---|---|
| N | 8 |
| Brier | 0.1894 |
| base rate | 0.75 |
| UNC | 0.1875 |
| BSS vs base rate | -0.01 (no skill over saying 0.75) |
| BSS vs 0.5 | +0.2425 (harness figure) |
| REL / RES (5 bins) | 0.069 / 0.063 |
| bootstrap 95% interval of Brier (20000 resamples) | [0.114, 0.267] |

(Bin summary: <0.5: 1 claim, observed 1.0; 0.5-0.7: 4 claims, mean p 0.54,
observed 0.5; 0.7-0.85: 3 claims, mean p 0.75, observed 1.0.) The same file
reports 80% intervals covering 3 of 6 and a mean log-ratio near zero with
mean abs 0.74.

**How representative:** poorly, and the reasons are specific.
- N=8 is under `MinN`=20; the interval spans a 2x range.
- The author is the orchestrating agent writing its own mission's claims, not
  a computer desk, so it measures none of rows 1 to 4.
- The claims are chosen by the claimant (no requester criteria); the design
  names this as the Goodhart channel that requester-set criteria remove.
- The settlers include Devin reviewing documents the same author wrote.
- The data-architecture ledger (`...selection-2026-10-28.precommit.md`) holds
  about 35 stated probabilities and some settled records, but most settled
  items are intervals or numeric ranges, and I did not extract a clean
  (p,y) set from its prose; its settled-probability count is unknown.
- The trust harness itself served no `prereg_units` and 0 replay pairs on the
  paper's build; the `inert` table shows 72 of 76 Texture precommitments from
  one prompt version were inert (`no_typed_question`), so even the
  per-desk claim stream was mostly not scoreable. Brier over a stream that is
  95% inert is a score over the remaining few, not over the desk.
- The only larger real baseline is archived E6a (management, n=559, mean p
  0.63 vs 0.79 held; Brier 0.116 resolved-only, reliability 0.031,
  resolution 0.013). I took those figures from the experiment doc, not
  recomputed, and they predate the digest.

## 7. How the score can be gamed or leak into a desk's prompt

1. **Vagueness.** Calibration improves by hedging toward the base rate; REL
   falls while RES falls with it. The digest prints resolution beside Brier
   for this reason; BSS vs UNC makes it a single comparable number. A desk
   whose BSS stays near 0 with falling REL is hedging.
2. **Claim farming.** Padding with near-certain claims lowers Brier
   (`paths_confined`, C4). Defense in code: `DigestExcluded`, per-type bins.
   Open: a new trivial claim type is only caught after someone looks. A
   standing check: UNC < 0.01 means exclude.
3. **Choosing the claim.** Rows 1 to 3 let the desk choose what to claim and
   when. Only row 4 (management's Cast-derived predictions) and the proposed
   requester criteria remove the choice. Late statements are already excluded
   (`LateRulePrecommit`, R20); restatements too.
4. **Self-fulfilling claims.** `judge_accepts` is management predicting its own
   verdict. Row 3's settler (Jev) reads text the desk wrote. Report these in a
   separate "own steps" stratum, as the tasks design does.
5. **Held-out and arms.** Salted HMAC of the record id. If the salt leaks (it
   is derived from the guest key), a desk can infer its arm. The digest-arm
   and control-arm design depends on the salt staying out of any desk's
   reach; a desk-readable `Observation.HeldOut` or `Arm` field would break it.
6. **Prompt leak.** By ruling the digest arm shows the desk its own digest at
   run start; that is the feedback, not a leak. Leaks are the other
   channels: (a) a Texture revision or Cast quoting a score of the desk's open
   work (the design adds an exact-match refusal); (b) the owner-facing
   `GET /api/calibration` response (it includes `Pending` with each claim's p
   and held-out marks) readable through any route a desk can call; (c) a
   score line copied into a Report and relayed to another desk. The audit
   gap is explicitly never shown.
7. **Optimizing the digest text.** Anyone tuning the prompt that frames the
   digest on its measured effect is hill-climbing the metric the digest
   reports. CLAUDE.md already bars tuning on the score; the arms test, not
   the score, must be the judge of any framing change.

## 8. What this adds

1. The Brier path is not hypothetical: it runs today for rows 1 to 4.
   The task-claim (row 6) work adds a stream, not a score.
2. The one real number (N=8, BSS -0.01 vs the base rate) says nothing about
   the desks and should not be quoted as evidence of calibration.
3. The read path scans the whole ledger and possibly truncates by id order
   at 20000 records; this is the single technical risk to a trustworthy score.

## 9. Mission proposal (converges on a small one)

**Outcome.** A per-computer, per-desk, per-claim-type Brier report that is
correct at scale: it reads a bounded window by settlement, reports BS, REL,
RES, UNC, BSS and a clustered interval, and says "no skill measurable" or
"n too small" instead of a number when it should.

**First three do-now items.**
1. Read-only check on staging (an owner-route call, no SSH): count commitment
   records on the paper and on d@e, and compare against 20000; confirm whether
   `ListCommitmentRecords` truncates, and in what order.
2. Add REL, UNC, BSS and the story-clustered interval to the digest data
   (`GroupStats`), and the guard that withholds a number under UNC<0.01 or
   N<`MinN`.
3. Replace the full-ledger scan with the settlement-first read of §5, kept
   inside `calibrationSnapshotFor`.

**Proof.** Per the repo's testing rule, an end-to-end artifact: a staging
`GET /api/calibration` response whose N per type matches a count taken
independently from the settlement records, and which is unchanged when the
tape grows past 20000 unrelated records. Compare against the E6a baseline
for the management stream.

**Rollback.** Revert; the digest falls back to the present computation. No
data migration; no new tables.

**Mutation class.** Orange (database queries, a product API's payload, a
digest change that feeds prompts). Ring-Expedite applies per the 2026-11-09
ruling when it lands.

**Deciding observation if this is not yet justified:** item 1. If the paper
holds under about 15000 commitment records and the list orders by time, the
truncation risk is dormant and the mission shrinks to item 2.

## Appendix: the script for §6

```python
P=[(.8,1),(.5,0),(.4,1),(.75,1),(.7,1),(.55,0),(.6,1),(.5,1)]
n=len(P); bs=sum((p-y)**2 for p,y in P)/n; yb=sum(y for _,y in P)/n
unc=yb*(1-yb); bss=1-bs/unc          # 0.1894, 0.1875, -0.01
```
