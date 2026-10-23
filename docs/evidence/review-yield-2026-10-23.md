# Review yield, FP0 launch first pass (2026-10-23)

What the model code reviews of the launch first pass found, seat by seat and
surface by surface, over 150 bunches (2026-10-21 to 2026-10-22). Input to
the risk-tiered review rule in `CLAUDE.md` (Mutation Classes). Data:
[review-yield-2026-10-23.tsv](review-yield-2026-10-23.tsv), 348 seat runs in
150 bunches (numbered 1 to 147 plus lettered splits) and 4,478 seat-minutes, plus 148 `#EXTRA` lines (31
`MISSED_BY_REVIEW`, 4 `STAGING_BREAK`, the rest notes). Confidence: medium.

## Table 1: by seat, normalized by calls

A seat run is one seat reviewing one bunch. "Blocking-real" is the run's top
finding classed BLOCKING-REAL after FP0's adjudication (reproduced or read
from code). A usable run is one that returned a verdict (MiniMax timed out
without one in 17 runs, Devin in 5).

| Seat | Runs | Usable | Blocking-real | Per run | Per usable run | Per 100 seat-minutes | Seat-minutes | Red share of runs |
|---|---|---|---|---|---|---|---|---|
| DeepSeek v4.1 flash | 136 | 136 | 74 | 0.54 | 0.54 | 4.6 | 1,597 | 76% |
| GPT-6.1-sol | 6 | 6 | 4 | 0.67 | 0.67 | 10.0 | 40 | 100% |
| Sonnet | 28 | 28 | 18 | 0.64 | 0.64 | 3.1 | 572 | 100% |
| Devin | 53 | 48 | 22 | 0.42 | 0.46 | 3.2 | 678 | 85% |
| Gemini 3.8 flash | 92 | 92 | 20 | 0.22 | 0.22 | 2.7 | 748 | 89% |
| MiniMax M3.1 flash | 33 | 16 | 9 | 0.27 | 0.56 | 1.1 | 843 | 82% |

Sol's six runs (bunches 1 to 4) are too few to rank. Per 100 seat-minutes
DeepSeek leads the seats with real sample sizes; Sonnet and Devin are close to
each other; Gemini is lower; MiniMax is lowest and mostly produced nothing.

### The selection confound

The seats did not review the same candidates. Sonnet reviewed red
candidates only (28 of 28), Devin and Gemini mostly red (85% and 89%), and
DeepSeek less so (76%), which if anything lowers its rate. The class column
does not show difficulty within red: the coordinator's reading is that Sonnet
and Devin were called mostly on the hardest red candidates (late deltas, work
the earlier seats had already seen), which would raise their per-run rates;
the data here cannot test that. DeepSeek was also the only usable seat
in 53 of the 150 bunches (Devin in one more), so raw "unique" counts are
inflated: a seat that is alone cannot be second-guessed, and 40 of DeepSeek's
74 blocking-real runs are marked unique. The per-seat "unique" column in the
data should not be read as a ranking.

### Table 1b: sole-finder counts where there is a counterfactual

Among the 96 bunches with two or more usable seats, how often one seat alone
found a blocking-real defect (no other usable seat found one):

| Seat | Sole finder |
|---|---|
| DeepSeek v4.1 flash | 15 |
| Sonnet | 2 |
| Gemini 3.8 flash | 1 |
| Devin | 0 |
| MiniMax, Sol | 0 |

Devin's three unique finds by the data's own marking (among them the
host-internal service token sent to guests by `paper_boot_window_probe.py`,
bunch 40) came in bunches where another seat also found a different
blocking-real defect, so they do not count here. Devin attended 47 of the 96
bunches, Gemini 92, Sonnet 28.

Other facts from the data:
- Gemini's five "unique" finds are in bunches 1, 2b, 3, 6 and 28; it has
  none after bunch 28 and accepted candidates in which another seat found a
  real blocking defect in several bunches (80, 140 to 142).
- 4 of Devin's 5 no-verdict runs came from the old `--sandbox` mode stopping
  on a confirmation (dangerous mode, 2026-10-22, fixed that); 1 was a timeout.
- Sol ran in bunches 1 to 4 only and was stopped for quota reasons (owner,
  2026-10-22), not for yield. Sonnet ran in bunches 53 to 122.

## Table 2: by surface

Share of bunches touching a surface in which any seat found a blocking-real
defect (a bunch can touch several surfaces).

| Surface | Bunches | With a blocking-real find | Share |
|---|---|---|---|
| docs | 45 | 12 | 27% |
| concurrency | 43 | 30 | 70% |
| host_config | 38 | 24 | 63% |
| vmctl_lifecycle | 35 | 21 | 60% |
| proof_scripts | 30 | 20 | 67% |
| tape_evidence | 27 | 17 | 63% |
| boot_path | 25 | 17 | 68% |
| store_schema | 16 | 11 | 69% |
| ci_scripts | 15 | 10 | 67% |
| deletes | 13 | 10 | 77% |
| gateway | 12 | 7 | 58% |
| auth | 11 | 7 | 64% |
| texture_writes | 3 | 2 | 67% |

By class: red 72 of 117 bunches (62%), orange 8 of 21, yellow 4 of 8, green 1
of 4. Red first reviews 30 of 43; red delta re-reviews 36 of 66.

## Conclusions

Only as strong as the normalized numbers allow.

1. Every non-doc red surface yields a real defect 58 to 77% of the time;
   deletes are highest (77%). Docs-only bunches yield 27%. That is a default
   for allocation, not a rule: a docs-only bunch that is an architectural
   design (a new mechanism, a doctrine change, a design a build will follow)
   deserves the review of the change it designs.
2. DeepSeek is the strongest routine seat per call and per minute, and the
   only one with a meaningful sole-finder count (15 of 96 multi-seat
   bunches). Devin matches the Sonnet-class rate per minute and is the
   owner's go-to reviewer and runs code, but the data cannot show it adds
   defects beyond what a second seat finds (0 sole finds in 47 attended
   bunches; three marked unique, one a serious token leak). Gemini and MiniMax
   are the weakest per call and per minute and add almost nothing alone
   (1 and 0). So the tier 2 pair is Devin plus DeepSeek; this is a judgement
   from medium-quality evidence, not a measured optimum.
3. Red delta re-reviews found a blocking-real defect in 36 of 66, and the rate
   does not fall with round number. Fix rounds introduced new defects at least
   four times (bunches 36, 60, 84, 145). So a delta is re-reviewed whenever the
   previous round found a blocking-real defect, and not when it found only
   doc, nit or test-only items; after three rounds on one mechanism, stop and
   look at the substrate.
4. About 25 defects were caught by CI, the Linux -race mirror or staging, not
   by review: vendorHash, hang guards, -race, vocabulary gates, cross-lane
   integration, proof-script premises. Cheap checks run before any review.
5. 38 doc-real findings cluster in proof, rollback and procedure text for red
   deploys. That text is not tier 0; it gets the deploy's own review.

## Method caveats

- Classes and "unique" were assigned from FP0's adjudication notes and the
  ledger, after the fact, by a reader that knew the outcome. "Blocking-real"
  is FP0's judgement, not a second opinion.
- The seats' call counts and candidate mixes differ (see the selection
  confound above); per-call rates reduce, not remove, that bias.
- Seat runs are not independent trials: seats saw different candidates and
  rounds, and later seats often reviewed fixes of earlier seats' finds. A
  seat that came earlier has more chances at a unique find. The Gemini
  comparison (before and after bunch 28) controls for this only roughly.
- 79 of 348 durations are estimates (`duration_source` est).
- Surfaces are keyword tags, assigned per bunch; several can apply.
- Yield counts defects found, not defects that would have shipped: some
  would have been caught by CI or staging anyway (item 4).
- Sample sizes for texture_writes (3 bunches) and auth (11) are small.
