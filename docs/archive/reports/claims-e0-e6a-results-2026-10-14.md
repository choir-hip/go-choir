# Claims E0 and E6a: First Results

Date: 2026-10-14. Mutation class: **green/yellow**. This is an offline
study: no runtime wiring, no Store B writes, no change on any computer.
The paper was read through the QA owner credential, read only. Design:
[claims, events and entities](../../reports/claims-events-entities-ontology-2026-10-14.md)
§12.3 (E0, E6a). Historical half:
[backtesting](../../research/historical-precommitment-backtesting-2026-10-14.md)
§3.4 (H0). Protocol, frozen before the labelled live runs:
[docs/evidence/claims-e0/protocol.md](../../evidence/claims-e0/protocol.md).

## Bottom line

1. **E0 does not stop the layer.** Settlement feasibility passes both
   falsifiers with room to spare:
   - the judge routes 94% of extracted assertions to a primary record or
     to independent sources (81% to a primary record);
   - checkable commitments and forecasts appear at about 16 and 26 per
     100 captures.

   The routing diagnostic beats the control. Under the design's exit
   rule, that means the **merging-first order** (E1, E2, E3), with E6a
   running alongside.
2. **Extraction is not ready for E1 as built.**
   - Quote grounding is excellent: 99.3% of quotes retained (gpt-6-luna)
     and 97.0% (deepseek-v4.1-flash). The bar is 95%.
   - **Modality fails its bar for both models**: 0.79 and 0.70 against
     0.85.
   - **Speaker accuracy fails for luna**: 0.83 against 0.90. Deepseek
     passes at 0.92.
   - The extractor's own Wikidata guesses are right only 30–50% of the
     time, so entity identity must come from a lookup and a judgment
     (D5), never from the model's guess.
   - The 12-claim cap is what limits recall on full-text documents:
     0.36–0.44 raw, 0.76–0.96 within the cap.
3. **Count must settle, and a judge may only fill gaps.** On the
   historical slice:
   - a deterministic count settled all 16 guided company metrics that
     have resolved;
   - the blind judge agreed with count on only 2 of 14 (luna's
     extractions) and 3 of 13 (deepseek's), and mostly answered
     "unresolved" on numeric ranges;
   - the judge also "confirmed" a vague Fed line ("The Committee will
     deliver price stability"), which a specificity gate should have
     marked unscorable;
   - closed-book, the judge knew none of the 2026 outcomes, so these
     count settlements are leakage class **L0**.
4. **The desks' first box score (E6a)** covers 373 published stories and
   2,931 desk records:
   - **management** is under-confident and barely discriminates. Mean
     p = 0.63, against 0.79 of resolved hypotheses confirmed.
     Brier 0.141 with unresolved counted as wrong; resolution 0.028;
     45% of its probabilities are hedges in [0.4, 0.6].
   - **research** quotes faithfully: 98.5% of 10,113 checkable quotes
     match their capture. But its "completed" is **over-confident**: in
     a sample of 120 rounds it called completed, it answered only 0.65
     of the questions it was asked (Brier 0.25 for the implied p = 1).
     A further 33% of research rounds close as `run_failed`.
   - **Texture**: 98.4% of judged citations are supported. But **47%
     of current published citations were never judged** (pending).
   - **engineering**: no assignments on the paper, so n = 0.
5. **The desks never score themselves today.** The paper's records hold
   no `Report.Settled`, no `resolve` and no `scores`. Precommit records
   (42) carry no probability. And **`model_id` is empty on all 2,931
   records**, so no per-model box score is possible yet. §5 sketches how
   E6a's scores go back into the desks as in-context learning (owner
   note 1).

## 1. What was built

| Path | What |
|---|---|
| `internal/claimslab/` | Extraction schema v0 (claims, mentions, event, five uncertainties); the quote check, which is `quotematch` unchanged (attempted vs retained, tier, block-hint mismatch); span matching; Brier with the Murphy decomposition; n_eff Beta intervals. Failure modes are written first in `doc.go`, with tests on them. No runtime import. |
| `internal/claimslab/cmd/claimslab` | A JSON-in, JSON-out command (`verify`, `match`, `rate`, `brier`) so the scripts reuse the Go quote checker and scoring math |
| `scripts/claims_ledger_pull.py` | Read-only pull: front stories, published versions, `texture:`/`research:` inbox records, the captures they name, the newest 500 captures |
| `scripts/claims_e0.py` | `freeze`, `overhead`, `label`, `extract`, `verify`, `judge`, `wikidata`, `route`, `score` |
| `scripts/claims_h0.py` | Historical slice: extraction, count settlement, blind judge, closed-book control |
| `scripts/claims_e6a.py` | Desks' say–do from existing ledger data; second scorer; self-scores |
| `scripts/claims_prompts.py`, `claims_lib.py` | Frozen prompts (hashes in the protocol); omp runner with a content-addressed call cache |
| `docs/evidence/claims-e0/` | Protocol, evalset manifest (ids only), `results.json`, `recall-by-body-kind.json`, `h0-result*.json`, `route-result.json` |
| `docs/archive/evidence/claims-e6a/box-score.json` | The desks' box score |

**Models.** Every model ran through omp. Extractors: `openai-codex/gpt-6-luna`
and `opencode-go/deepseek-v4.1-flash`. Labeller and judges:
`google-antigravity/gemini-3.8-flash`. Second scorer in E6a: luna. Data
with article text is in `~/.config/choir-qa/traces/claims/` (pull, e0,
e6a, h0, historical, and the call cache that is the audit trail).

**Deepseek ran out of funds part way.** The `opencode-go` account
returned `402 Insufficient account funds` after 103 of 200 captures. The
other 97 failed, and they include every non-English capture. Model
comparisons therefore use the **103 captures both extractors completed**
(the `@common` rows of `results.json`). Luna's numbers on all 200 are
reported separately.

## 2. E0: the frozen evaluation set (manifest `fd5286c8…`)

- **Pull.** 376 published stories (373 with a document) and 2,931 desk
  records, committed between 2026-10-13 15:24Z and 2026-10-14 18:15Z.
  3,550 captures with blocks.
- **What the paper's captures are.**
  - Of the 657 captures that management's records name, **626 are
    single-block feed items** (a title plus a summary).
  - Full text exists almost only for research reads.
  - 97% of captures are English, and every non-English capture is a
    research read.
- **Sample.** 200 captures from 110 outlets, seed 20261014.
  - By selection: 130 scheduled and 70 targeted.
  - By body: 95 feed items, 90 full text, 15 HN stubs.
  - By language: en 113, Latin script that could not be classified 37,
    fr 11, ja 10, es 10, de 8, pt 4, it 3, zh 3, ru/uk 1.
  - By section: world, us_politics, tech_ai, conflict, business, politics
    and others.
- **Splits.** Tune 20, label 50 (35 scheduled and 15 targeted), and
  unlabelled 130.
- **Labels.** These are model-assisted and **need a human audit**
  (protocol §3.5).
  - Gemini made two blind passes over the 50 label-split captures.
  - 1,216 labelled quotes, of which 4 failed the quote check.
  - Consensus gold is 541 claims (472 material). Pass-to-pass F1 is
    0.89.
  - A spot read of two feed items found the labels sensible: the
    labeller splits a relay into a reporting claim and an embedded claim
    without being told to.

## 3. E0 results

### 3.1 Extraction validity

The blind judge scored the label split plus 30 seeded unlabelled
captures. Intervals are 95%.

| Measure | Bar | luna, all 200 | luna on the 103 common | deepseek on the 103 common |
|---|---|---|---|---|
| quotes retained / attempted | ≥ 0.95 | **0.993** [0.985, 0.996] (1,066/1,074) | 0.997 | **0.970** [0.948, 0.983] |
| tolerant-tier matches | shown | 19 | 2 | 9 |
| is a real claim | — | 0.976 | 0.974 | 0.937 |
| speaker correct | ≥ 0.90 | **0.834** [0.797, 0.865] ✗ | 0.827 ✗ | **0.917** [0.872, 0.948] ✓ |
| modality correct | ≥ 0.85 | **0.790** [0.750, 0.825] ✗ | 0.786 ✗ | **0.699** [0.633, 0.758] ✗ |
| readings non-vacuous | ≥ 0.90 (proposed) | 0.987 ✓ | 0.985 ✓ | 0.951 ✓ |
| entity QID guess precision | ≥ 0.90 | **0.30** (190 of 627 guesses) ✗ | — | **0.50** (93 of 187) ✗ |
| claims per capture | — | 5.37 | 3.17 | 3.86 |
| captures over the 12 cap | 0 | 7 | 2 | 0 |

**What the uncertainty fields carry.**
- U1, attribution: luna almost never doubts it. p_attr is below 0.6 on
  0.3% of claims, so U1 is recorded but does not discriminate.
- U2, readings: luna gives more than one reading on 0.2% of claims;
  deepseek on 19%.
- U3, hedges: the hedged share is 19% for luna and 16% for deepseek.
- U4, extraction confidence: recorded on every claim.

So U1 and U2 are recorded on every claim but are near-degenerate for
luna. The prompt needs worked examples before they can carry signal.

**By language** (luna, quotes retained): de 0.987, es 1.0, fr 0.99,
it 0.968, ja 1.0, pt 1.0, zh 1.0, ru/uk 1.0 (n = 4). Deepseek has no
non-English data, because those captures fell after its funds ran out.

### 3.2 Recall against the consensus labels

| Body kind (label split) | luna recall of material claims | within the 12 cap | deepseek | within cap |
|---|---|---|---|---|
| feed item, scheduled (25) | 0.794 | 0.862 | 0.841 | 0.914 |
| full text, scheduled (5) | 0.359 | 0.902 | 0.383 | 0.961 |
| full text, targeted (15) | 0.438 | 0.755 | n/a (no funds) | n/a |
| all material (luna 50, deepseek 35) | 0.464 [0.419, 0.509] | — | 0.534 [0.463, 0.603] | — |

- **Span precision against either labelling pass:** 0.92 for luna and
  0.93 for deepseek.
- **The cap binds on full text.** Full-text documents carry 20 to 50
  consensus claims, so a 12-claim cap reads as low recall. The proposed
  0.70 bar passes on feed items and, within the cap, on full text. The
  cap needs a decision: keep it and measure recall within the cap, or
  raise it for full text.

### 3.3 Settlement feasibility

Measured on the live sample. These are the judge's estimate of the
settle route, not settlements.

| Measure | luna (74 captures, 452 claims) | deepseek (47, 206) |
|---|---|---|
| passes the specificity check | 0.808 [0.769, 0.841] | 0.786 |
| assertions settleable by count (primary record or independent sources) | **0.940** [0.908, 0.961] (n = 315) | 0.931 (n = 159) |
| … by a primary record alone | 0.810 | 0.818 |
| commitments per 100 captures (all judged specific) | 16.2 | 0.0* |
| forecasts per 100 / checkable forecasts per 100 | 31.1 / 25.7 | 21.3 / 10.6 |

\* On the same 47 captures, luna gives 8.5 commitments per 100. The
judge reclassified deepseek's few commitments as other modalities, so
modality errors cost commitments directly.

**Falsifiers.**
- CE-1 is falsified if count covers less than 20% of assertions. It is
  **not falsified**: the judged route is 94%. Live settlement is still
  owed at 30 days (E6).
- CE-2 is falsified if fewer than 1 in 5 commitments are checkable. It
  is **not falsified**, but n is small (12 to 35 commitments).

### 3.4 The historical slice (H0)

**The slice.** Three FOMC statements (2026-04-29, 06-17, 07-29), each
settled by the next statement. Three 8-K Exhibit 99.1 earnings releases
with numeric guidance, each settled by the next quarter's release:
- NVIDIA, Q2 FY27;
- Cisco, Q4 and full-year FY26;
- Salesforce, Q2 FY27.

The SEC acceptance times and the Fed release times are the witnessed
times. The manifest and sha256 values are in `historical/manifest.json`
(outside the repo; public-domain documents and SEC filings). Every value
behind a count settlement was re-found verbatim in the saved outcome
document (integrity check: 0 missing).

**Gold.** 28 questions written from the documents before any extraction
was read:
- 16 count questions (guided ranges);
- 5 pending (fiscal 2027 full-year guidance, not yet due);
- 7 Fed forward-looking lines, judged or settled by primary record.

| | luna v0 | deepseek v0 | luna v1* | deepseek v1* |
|---|---|---|---|---|
| quotes retained | 63/64 | 67/67 | 65/68 | 65/65 |
| recall, all 28 questions | 0.46 | 0.50 | 0.89 | 0.86 |
| recall, 16 count questions (bar 0.85) | **0.19** | **0.38** | 0.88 | 0.81 |
| blind judge agrees with count | 0/3 | 0/4 | 2/14 | 3/13 |
| closed-book judge knew the outcome | 0 | 0 | 0 | 0 |

\* `EXTRACT_V1_FORWARD` ("first include every forward-looking statement,
one claim per guided metric") was written after reading v0's recall and
run only on these six documents, so its numbers are optimistic.

**Reading.**
- The generic prompt spends its 12 claims on headline results. A
  forward-first rule recovers the guidance.
- The judge answers "unresolved" on most numeric ranges. That fits the
  memo's warning ("weak at counting… compare quantitative predictions in
  code").
- On the Fed lines, the judge "confirmed" "will deliver price stability"
  twice, where the hand reading is unscorable. **The specificity check
  must gate before any judge.**
- The Committee's April easing-bias sentence ("the extent and timing of
  additional adjustments…") was missed by both extractors. That is the
  one Fed line a reading would settle as contradicted, because the next
  move was a September hike.

**Guidance box (count, L0).**

| Company | Guided items settled | Within range | Above range | Below range | Pending |
|---|---|---|---|---|---|
| NVIDIA | 5 | 4 | 1 | 0 | 1 |
| Cisco | 8 | 1 | 7 | 0 | 0 |
| Salesforce | 3 | 1 | 2 | 0 | 4 |

All 10 misses are beats, never shortfalls. Read as a say–do pattern,
that is **conservative guidance**, which is the kind of directional
signal a box score should show beside the hit rate. n is tiny. The
example shows the shape; it is not a finding about these companies.

### 3.5 Routing diagnostic

The test set is the after-window of `docs/archive/problems/l2a-merge-recall-on-the-paper-2026-10-14.md`,
titles only: 47 items and 77 pairs. Gold has three parts:
- 46 cross-outlet same-event pairs, from the registered judge;
- 21 same-story pairs that the hand reading called legitimate;
- 10 that the hand reading called wrong (topic feeds).

Method: luna describes each title's event, and deepseek clusters the
events in one call.

| | cross-outlet same-event kept together (46) | legitimate same-story kept (21) | wrong merges kept apart (10) |
|---|---|---|---|
| **event-structured** | **44** (0.96) | 16 | **10** |
| titles only, same clusterer | 46 | 16 | 5 |
| control: management's actual routing | 8 (0.17) | 21 (by construction) | 0 |

**Limits.**
- This is a diagnostic, not E2. It is one-shot clustering, not arrival
  routing.
- The gold same-event pairs came from a judge grouping the same titles.
- The window's captures are past the capture route's newest-500 listing,
  so no full text was available.

Within those limits, the event structure separates topic from event
(10 of 10 wrong merges kept apart, against 5 of 10 from titles alone)
for a cost of 2 missed pairs.

### 3.6 Cost per capture

Costs are at list prices from `internal/llmcost/pricing.go`, net of
omp's own harness input (about 6.2k tokens per call, measured).

| | luna | deepseek |
|---|---|---|
| net input / output tokens (mean) | 1,526 / 1,093 (all 200) | 1,127 / 2,029 |
| USD per capture | **$0.00070** | **$0.00277** |
| per day at 1,700 captures | **$1.19** | $4.71 |
| median latency | 29.5 s | 33.4 s |

- Labels cost $0.018 per capture for two gemini passes (omp-reported).
  Gemini is not in `llmcost`.
- The design's estimate was $0.0006–0.002 per capture. Luna is at the
  bottom of that range. Deepseek costs four times as much, mostly in
  output: it writes more readings.

### 3.7 Stop/go

| Exit criterion (ontology §12.3) | Result |
|---|---|
| Routing beats the control | **yes** on the diagnostic: recall 0.96 vs 0.17, false merges 0 of 10 vs 10 of 10. Owed: E2's measured windows |
| Settlement feasibility passes | **yes**: 94% of assertions routed to count, CE-1 and CE-2 not falsified; on H0, count settles 16 of 16 resolved guidance items |
| Both fail → stop | not met |

**Decision: go, merging-first** (E1 shadow extraction, then E2 events).
E6a runs alongside. E1 should not start until these are done:
1. **Modality.** Write a v1 prompt with definitions and worked examples
   (commitment vs intent vs forecast; attribution relays), tuned on the
   `tune` split only, then rerun the label split. The bar is 0.85.
2. **Speaker.** Either take deepseek's speaker accuracy at four times
   the cost, or keep luna and add a speaker-resolution judgment (it is a
   judgment in the design anyway, §4.3). D11 stays open: luna wins on
   cost, quote retention and modality; deepseek wins on speaker and on
   use of U2. Neither passes modality.
3. **Forward-looking first**, as a rule of the prompt (the H0 lesson).
   **The cap**: count it within the cap and raise it for full text, or
   chunk.
4. **Entity identity** from a Wikidata lookup plus a choice. The model's
   QID guesses are 30–50% right.
5. **A specificity gate before any judge**, and count for every numeric
   question.
6. **A human audit** of 10 label-split captures (protocol §3.5).
7. **A selection decision** (finding F1 below).

## 4. E6a: the desks' say–do box score (paper, read-only)

**Definition.**
- Each desk act that asserts something about its own work, or predicts
  something, is a *say*.
- Its *do* is settled by the cheapest independent method: count where
  possible, then a blind judge that never sees the desk's probability.
- No score is written back to the computer, so I26 holds.
- In P4's terms, unresolvable counts as wrong for agents; the
  resolved-only figures are shown beside the strict ones.

### Box score: Autopaper paper computer

The data runs from 2026-10-13 15:24Z to 2026-10-14 18:15Z: 373 stories
and 2,931 records. The scorers are count, gemini-3.8-flash (blind) and
gpt-6-luna (a second blind scorer on a sample).

| Desk | Say–do measure | Result |
|---|---|---|
| management | **M1. Story-opening Cast hypotheses with probability** (730 in 373 stories) | **Outcomes:** confirmed 363, qualified 154, contradicted 42, unresolved 93, no research evidence 78.<br>**Probabilities:** mean p 0.63; hedge share (p in [0.4, 0.6]) **45%**.<br>**Brier:** 0.141 with unresolved counted as wrong (n = 652, skill +0.36 against the base rate). Resolved only: 0.116 (n = 559).<br>**Confirmed share of resolved:** 0.79 [0.75, 0.82].<br>**Calibration (stated p → observed):** 0.37 → 0.62; 0.57 → 0.75; 0.74 → 0.84; 0.88 → 0.90; 0.96 → 0.83 (n = 9). Under-confident below 0.85.<br>**Murphy decomposition:** reliability 0.009, resolution 0.028, uncertainty 0.160. The probabilities barely discriminate |
| management | **The scorer, scored.** A second blind scorer (luna) on 98 hypotheses in 60 stories | Exact agreement 0.59 (luna says "qualified" where gemini says "confirmed"). Binary agreement 0.92 (confirmed or qualified, against the rest). Management's Brier is 0.108 under gemini and 0.118 under luna, so the conclusion does not depend on the scorer |
| research | **R1. "This is a verbatim passage of capture X."** Every `text_quote` selector research handed Texture, checked by quotematch | 10,236 quotes. Of the 10,113 checkable: matched **0.985** [0.983, 0.987]; exact 0.964; not found 151. Also 81 on unreadable captures and 42 with no capture id. Self-claim Brier 0.015, which is well calibrated |
| research | **R2. A final Report saying `completed`** against how many of Texture's questions it answers (blind judge; 120 sampled rounds, 400 questions) | **Answered** 188, **partial** 147, **unanswered** 65. Coverage **0.65** [0.61, 0.70]. Read as a self-score with p = 1, Brier **0.25**: over-confident. Separately, of 725 final Reports, **242 (33%) are `run_failed`** and 14 are `interrupted_by_restart` |
| texture | **T1. A citation asserts that the passage supports the sentence.** Settled by the support judge (Jev) and the quote check | 3,483 citations in current published versions. Supported 1,800, failed 30, **pending 1,653 (47%)**. Supported share of judged **0.984** [0.977, 0.988]. Quotes: exact 3,280, tolerant 134 |
| engineering | Assignments' stated outcomes against Freeze results | **n = 0.** The paper has no engineering work. This needs a computer that runs engineering, such as the c1e test computers |
| all desks | **Self-scores on the ledger** | **None.** No `Report.Settled`, no `resolve`, no `scores` on any of 2,931 records. The 42 Precommit (ask) records carry no distribution. `model_id` is empty on all 2,931 records |

**Caveat on M1's evidence.** The settling evidence is research's
findings and the published story's support-checked quotes. Research
worked on questions Texture derived from management's frame, so some
confirmation is self-fulfilling. That would make the observed confirmed
share an upper bound and management's under-confidence a lower bound.
The 42 contradictions are the informative tail.

## 5. Feeding the scores back: the in-context learning loop (owner note 1)

The owner's ruling: precommitment records and their settled scores live
in the desk's Yaegi REPL. They are retrievable on demand and packed into
context, so feedback works as in-context reinforcement learning without
weight updates. Desks score themselves, but they are not the only
scorers, and calibrated self-scoring is a capability that is itself
scored.

The loop E6a should become:

```text
1 commit      desk writes the prediction with p           (exists: management's Cast hypotheses;
                                                            missing: p on Texture's casts and on Precommit asks, P3)
2 self-score  at round close, the desk that owns the        (missing: 0 Report.Settled today)
              prediction writes its own resolve, with a p,
              BEFORE it sees any independent settlement
3 settle      independent scorers: count first, then blind  (E6a's scorers; multi-scoring is the default;
              judges (Jev, a second model), then a panel     each scorer has its own record,
              on disagreements                               scored against count)
4 record      one score record per scorer on the tape,      (§5.4 home rule: a desk's own records stay on the tape)
              with model_id                                  (missing: model_id is empty)
5 read        choir.TrackRecord(desk, kind, since):         (new read view in the desk's symbol table)
              the desk's settled predictions as records:
              statement, own p, own self-score,
              independent outcomes, discrepancy, evidence
              refs
6 pack        per (model, task type): the largest           (memo "context packs"; needs 4's model_id)
              discrepancies and calibration bins, e.g.
              "your 0.55-0.70 hypotheses held 75% of the
              time (n=250)"
7 measure     does the desk's next calibration improve      (the memo's "behaviour change is the evidence")
              after exposure? A/B packs on or off per story;
              a held-out audit sample never shown
```

**Guards** (memo, and I26 reconciled with owner note 1):
- The desk sees its *own* settled outcomes and its calibration bins. It
  never sees a scorer's raw verdict on a pending item. Exposure is
  delayed until settlement and sampled.
- A held-out 10% stays unseen, to detect learning the scorer rather
  than the world.
- The **self-score gap** is a monitored number: self-score minus the
  independent outcome, tracked per desk and model. R2 is its first
  measurement: research says "completed" with implied p = 1, against
  0.65 coverage.

**What E6a's numbers would teach today.**
- Management: raise p on routine factual frames, because it is
  under-confident below 0.85. Spread probabilities apart: resolution is
  0.028.
- Research: say `partial` when questions are unanswered.
- Texture: none yet, because half its citations are never judged.

## 6. Dependencies on undecided owner decisions (D1–D13)

**Built without them.**
- No new kinds (D3).
- No Store B writes and no projection (D4).
- No entity linker (D5); the study only measures the model's QID guesses.
- No publication (D10, E8).
- No scoring of named public actors: the guidance box above is an
  internal example.

**Informed by E0, still open.**
- **D11** (extraction model): the evidence in §3.7.2.
- **D12** (action-class vocabulary): guidance ranges map to "publish a
  figure". H0's count rule `h0-approx-2pct` turns "approximately X"
  into X ± 2%. That is an operationalization choice the vocabulary
  should version.
- **D8 and D13** (horizons and estimative mapping): not exercised. No
  live forecast has matured yet.
- **D9** (human audit): owed on the labels.

**D1 meets finding F1.**
- Scheduled captures are 95% feed summaries.
- Full text comes almost only through research's **targeted** reads.
- §4.1 never scores targeted claims.

As designed, track records would therefore rest on headlines and
summaries. Two options:
- fetch the full text of scheduled items (the deepen fetch), or
- count a research read of a scheduled item's own URL as scheduled.

Either needs a ruling alongside D1.

**D6 is now settled** (owner, 2026-10-14 evening): public figures and
people who publish for their content to be consumed by AI, not private
individuals. E0 records the speaker's type but no `actor_scope`. E1
should add `actor_scope` with the settled scope.

## 7. Findings and heresy delta

**Discovered** (named here, not repaired; this study is offline):
- **F1.** Scheduled captures are feed summaries; full text is targeted
  (§6).
- **F2.** `model_id` is empty on every desk record (2,931 of 2,931), so
  no per-model calibration or context pack is possible. Scoring design
  §1 item 5 predicted this; it is still true.
- **F3.** The support judge left 47% of current published citations
  `pending`. Possibly the Jev peer-pin defect of the L2a problem doc;
  the trace has not been checked, so this is a hypothesis.
- **F4.** Management's hypotheses are never settled on the ledger:
  0 `Report.Settled`. The product's only probability-bearing desk
  predictions go unscored in-product.
- **F5.** 33% of research's final Reports are `run_failed`.
- **F6.** The extractor's Wikidata guesses are 30–50% right.
- **F7.** The blind judge cannot settle numeric ranges, and it confirms
  vague lines.
- **F8.** Operational: the `opencode-go` deepseek account ran out of
  funds mid-run.

**Introduced:** none. Code is under `internal/claimslab` and `scripts/`,
with no wiring.

**Repaired:** none.

## 8. Residuals and next

- The human audit of labels (10 captures).
- An E0 round 2: modality and speaker v1 tuned on `tune` only; rerun
  `label`. Rerun deepseek's 97 failed captures (including all
  non-English ones) once the account has funds; the call cache makes
  the rest free.
- E2-grade routing on full captures of a fresh window: re-pull within
  the newest-500 horizon.
- H0 widened toward backtesting §3.2 sizes, with the counterfactual
  evidence-swap test, before any judged historical settlement is used.
- E6a as a standing job (each settlement on the tape with scorer and
  model) once F2 is fixed. Then the §5 loop's read view and pack, with
  the A/B.
- Engineering's say–do on a computer that runs engineering.

## Evidence and reproduction

- Branch `claims-e0-e6a` (not pushed). Mutation class green/yellow; no
  protected surface touched.
- Reproduce, from the repo root:

  ```text
  python3 scripts/claims_ledger_pull.py --cred ~/.config/choir-qa/test-computers/autopaper-paper.json
  python3 scripts/claims_e0.py freeze|overhead|label|extract|verify|judge|wikidata|route|score
  python3 scripts/claims_h0.py [v1]
  python3 scripts/claims_e6a.py
  ```

  Every model call is cached by sha256(model, prompt), so a rerun
  reproduces the numbers without calls.
- `go test ./internal/claimslab/` passes.
