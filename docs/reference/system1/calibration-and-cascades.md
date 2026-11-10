# Calibration, thresholds, abstention and cascades for System 1

Reference subpage (convention: [../README.md](../README.md)). Siblings:
[model-space.md](model-space.md), [dual-process.md](dual-process.md),
[evaluation.md](evaluation.md). Related: [../JEV.md](../JEV.md). Fetched
2026-11-10. Tags: `[V]` vendor; `[T]` third party; `[P]` paper (peer-reviewed or
arXiv); `[R]` relayed, unverified. Worked examples use **made-up numbers**
(labelled "illustrative") computed with small scripts; the method is sourced,
the data is not.

## Surprises

1. **Decision models are over-confident on hard knowledge and calibrated on
   tool and safety tasks.** AA found accuracy below the stated probability
   across almost all probability levels, driven by the knowledge sets, while
   most models were "relatively well calibrated" on When2Call and WildGuardTest
   `[T]`. Calibrate per question family, not per model.
2. **The leaderboard now scores calibration, so rankings moved.** AA Decision
   Index v2 headline = 50 x (1 + accuracy - Brier). A model with a 24% raw
   pass rate (Celeris-1.1) matched one with 36% on that score because its Brier
   was 0.15 against 0.27 `[T]`.
3. **Abstention is a first-class output and it can be gamed.** AA reports a
   "non-hallucination rate" (of the questions not answered correctly, the
   share where the model said "I don't know"): Microsoft-Decision-1 40%, Jev 1.13
   35%, d1-ultra 85% with accuracy 32% against 41% for d1 `[T]` `[R]`. Baseten's
   models lead consistency tests "largely because they abstain often and
   confidently across variants" `[T]`.
4. **A single scalar fixes much of it.** One reviewer's Banking77 run: Jev
   reported 88% average confidence at about 80% accuracy; temperature scaling
   cut calibration error by about two thirds `[T]` (blog; reproduce).
   Guo et al.: for deep nets, temperature scaling, "a single-parameter variant
   of Platt scaling", is surprisingly effective `[P]`.
5. **Verbalised confidence beats logits on chat models.** For RLHF models,
   confidences the model writes as tokens are typically better calibrated than
   its token probabilities, often cutting ECE by a relative 50% `[P]`. Decision
   models invert this: their probability readout is the trained output.
6. **Cascades save money and make the tail worse.** Escalating 30% of
   decisions from Jev (93 ms) to a reasoning model (32.65 s median per AA task)
   cuts cost by 67% and lifts the mean latency to 9.9 s; p70 stays 93 ms, p90
   is 32 s (section 7). Cost was never the whole story for a voice product.
7. **The quality estimator is the whole cascade.** The cascade-routing paper
   identifies "good quality estimators as the critical factor" for routing and
   cascading `[P]`; the paper gives optimal strategies but the gains depend on
   how well confidence ranks correctness.
8. **A model alias that moves breaks thresholds silently.** `~typesafe/jev-latest`
   resolved to `jev-1.14-20261014` on 2026-11-10 (JEV.md, measured by us).
   Thresholds, bias priors and calibration maps are per served version.
9. **Option bias is a calibration problem.** The prior over option IDs
   ("A" favoured) can be removed with a label-free estimate from about 5% of
   samples (PriDe) `[P]`, and content-free calibration lifted accuracy by up to
   30 points absolute for GPT-style few-shot `[P]`.

## 1. What "calibrated" means and how to check it

A decision of type `noul` returns P(yes); `choice` returns a probability per
choice (JEV.md). Define the confidence of a decision as the probability of the
chosen answer. The model is calibrated if, among decisions with confidence
about c, a fraction about c are right.

- **Reliability diagram:** bin decisions by confidence, plot mean confidence
  against observed accuracy per bin. Points under the diagonal mean
  over-confidence.
- **ECE (Expected Calibration Error):** the weighted mean absolute gap between
  confidence and accuracy over M bins; Naeini et al. 2015, used by Guo et al.
  `[P]`. ECE depends on binning; report the bin scheme and the bin counts.
- **Brier score:** mean squared error between probability and 0/1 outcome.
  **Log score:** mean negative log of the probability assigned to the outcome.
  Both are proper scoring rules (the forecaster's best strategy is to report
  its true belief). AA uses accuracy and Brier together in its v2 headline `[T]`.

### 1.1 Worked example: computing ECE (illustrative data)

1,000 logged yes/no decisions, the model's confidence in its chosen answer, five
bins:

| Bin | n | Mean confidence | Observed accuracy | Gap | n/N x gap |
|---|---|---|---|---|---|
| 0.5-0.6 | 120 | 0.55 | 0.52 | 0.03 | 0.0036 |
| 0.6-0.7 | 150 | 0.65 | 0.58 | 0.07 | 0.0105 |
| 0.7-0.8 | 210 | 0.75 | 0.66 | 0.09 | 0.0189 |
| 0.8-0.9 | 260 | 0.85 | 0.74 | 0.11 | 0.0286 |
| 0.9-1.0 | 260 | 0.96 | 0.83 | 0.13 | 0.0338 |

ECE = 0.0036 + 0.0105 + 0.0189 + 0.0286 + 0.0338 = **0.0954**. Mean confidence
0.792 against accuracy 0.696: the model claims 79% and delivers 70%. The shape
(gap grows with confidence) is the AA over-confidence pattern.

### 1.2 Worked example: temperature scaling when you only have probabilities

Hosted decision models give probabilities, not logits. For a binary decision,
recover a logit z = ln(p / (1 - p)) and rescale: p' = sigmoid(z / T). T > 1
softens. Fit T by minimising log loss (or ECE) on logged outcomes; it is one
parameter, so a few hundred labelled decisions suffice for a first fit
(my rule of thumb, not sourced). On the table above:

| Raw p | z | T = 1.8 | Observed |
|---|---|---|---|
| 0.96 | 3.18 | 0.854 | 0.83 |
| 0.85 | 1.73 | 0.724 | 0.74 |
| 0.75 | 1.10 | 0.648 | 0.66 |
| 0.65 | 0.62 | 0.585 | 0.58 |
| 0.55 | 0.20 | 0.528 | 0.52 |

ECE falls from 0.0954 to **0.0146** at T = 1.8 (0.0298 at T = 1.5, 0.0160 at
T = 2.0). Pseudo-code:

```text
for T in grid(0.5..4.0): loss[T] = mean(logloss(sigmoid(logit(p)/T), y))   # on dev split
T* = argmin loss ; store T* with {model_alias, question_id, date}
at decision time: p' = sigmoid(logit(p)/T*) ; then apply the threshold
```

For `choice` questions with k options use p_i' proportional to p_i^(1/T) (a
standard softmax-temperature generalisation). If the reliability curve is not a
smooth S, use isotonic regression or Platt scaling with an intercept (Guo et al.
discuss both) and a held-out split `[P]`. Calibrating on the same data you
evaluate on is the usual mistake.

## 2. Proper scoring in practice (forecasts, not just verdicts)

Score the probability, not the argmax. Four forecasts, illustrative:

| p(yes) | outcome | squared error | -log of assigned prob |
|---|---|---|---|
| 0.9 | yes | 0.01 | 0.105 |
| 0.9 | no | 0.81 | 2.303 |
| 0.6 | yes | 0.16 | 0.511 |
| 0.5 | no | 0.25 | 0.693 |

Brier = 0.3075, log score = 0.903. Accuracy alone would call the 0.9-wrong case
one error among four; the scores show it cost more than the other three
combined. AA's per-question Decision Score is 50 x (1 + c - (p - c)^2) with c = 1
if correct (right and confident approaches 100; wrong and confident
approaches 0; unsure ~ 50) `[T]`; for an evaluation it equals 50 x (1 + accuracy -
Brier). Example: accuracy 0.8 with Brier 0.15 scores 82.5; the same accuracy
with Brier 0.30 scores 75.0.

## 3. Picking a threshold from logged outcomes

Goal: the least-expensive rule "act on the System 1 answer if confidence >= t,
else escalate or ask". Method:

1. Log, per decision: question id, served model version, calibrated confidence,
   System 1 answer, later outcome (gold label or the System 2 / listener
   result). The Jev record (`internal/jevrecord`) already keeps the first
   three.
2. Sort by confidence, compute for each t the **coverage** (fraction acted on)
   and the **precision** among acted decisions.
3. Choose t by cost: act if (1 - p_correct) x C_wrong < C_escalate.

Illustrative table from the same 1,000 decisions (raw confidences):

| Threshold t | Coverage | Precision of acted decisions |
|---|---|---|
| 0.9 | 0.26 | 0.830 |
| 0.8 | 0.52 | 0.785 |
| 0.7 | 0.73 | 0.749 |
| 0.6 | 0.88 | 0.720 |
| 0.5 | 1.00 | 0.696 |

Reading it: no threshold reaches 95% precision; the top bin tops out at 0.83.
A "yes above 0.9" rule on this question is wrong 17% of the time and that is
**information about the question**, not about the threshold: ask it with better
instructions, add a feature, or never auto-act on it. Cost rule: if a wrong
act costs $0.01 and escalating costs $0.0035, act when p_correct >= 0.65; with
temperature-scaled probabilities the 0.85 bin (p' = 0.72) passes and the
0.65 bin (p' = 0.585) does not. Vendor starting points (about 0.7 for yes and
0.3 for no) are in JEV.md `[R]`; tune on our data.

Two traps: (a) **stratify by question family**, because the same model is
calibrated on tool-calling and over-confident on knowledge `[T]`; (b) **guard
against selection**: if only escalated decisions get a gold label you cannot
estimate precision above the threshold; keep a random 2 to 5% audit slice
(my suggestion).

## 4. Conformal prediction: sets with a guaranteed miss rate

Split conformal gives a set of options that contains the truth at least
1 - alpha of the time, with no model-internal assumptions beyond
exchangeability `[P]`. For multiple choice with an LLM, Kumar et al. use
the "least ambiguous classifier" score s = 1 - p(true answer) and find that
set size is tightly correlated with accuracy, so set size doubles as an
uncertainty signal; they also test the exchangeability assumption by moving to
out-of-subject questions (it matters) `[P]`.

Recipe (ours):

```text
calibration: n labelled decisions; for each, s_i = 1 - p_i[true option]
alpha = 0.10 ; k = ceil((n + 1) * (1 - alpha)) ; q = k-th smallest s_i
   n = 200 -> k = 181, i.e. the 181st smallest score (quantile level 0.905)
at decision time: set = { option j : 1 - p_j <= q }  =  { j : p_j >= 1 - q }
   |set| = 1  -> act on it
   |set| >= 2 -> escalate (or ask the listener)   |set| = 0 -> abstain / escalate
```

Toy numbers (illustrative): suppose q = 0.62, so options with p >= 0.38 are in
the set. p = (0.55, 0.30, 0.15) -> set {A}: act. p = (0.45, 0.40, 0.15) ->
set {A, B}: escalate. p = (0.34, 0.33, 0.33) -> empty: abstain. With `none` as
an option (our `choice` questions) include it in the set like any other.
The guarantee is marginal (average over questions), not per family; run one
calibration set per family. The quantile index k = ceil((n+1)(1-alpha)) is the
standard finite-sample correction, as in the ACI paper `[P]`.

**Online version.** Gibbs and Candes' adaptive conformal inference tracks drift
with alpha_(t+1) = alpha_t + gamma (alpha - err_t), where err_t is 1 when the
set missed the truth `[P]`. Illustrative with alpha = 0.10, gamma = 0.005: a miss
moves the working level from 0.1000 to 0.0955, two correct decisions move it
to 0.0960 and 0.0965, another miss to 0.0920. Larger gamma adapts faster and
is noisier. It needs a label per step, so use it on the audited slice.

## 5. Selective prediction and abstention in decision models

- AA's AA-Omniscience-Decision offers an "I don't know" option and scores
  non-hallucination; v2 re-words it in ten ways to test that abstention is
  not an artefact of one phrase `[T]`.
- Models differ: Microsoft-Decision-1 abstains more (non-hallucination 40%),
  Jev 35%, Mercury Decide 32%; v1.0 says no other decision model reaches 17%
  `[T]`. Abstention costs accuracy: d1-ultra's higher abstention is
  paired with accuracy 32% versus 41% `[R]`.
- Prefer an explicit **`none` / `unsure` choice with its own criteria** plus a
  threshold, to a free-floating confidence. Reword it and re-test (AA's ten
  phrasings are the template).
- GLiDE advertises "extra reasoning only when an initial decision is uncertain"
  `[R]`: a cascade inside the vendor.

## 6. Cascades: the published curves

| Result | Number | Tag |
|---|---|---|
| FrugalGPT: LLM cascade with a learned scorer | matches best single LLM (GPT-4) with up to 98% cost reduction, or +4% accuracy at the same cost | `[P]` |
| AutoMix: small model self-verifies, POMDP router picks the escalation | over 50% cost reduction at comparable performance, five models, five datasets | `[P]` |
| RouteLLM: routers trained on preference data | over 2x cost reduction without quality loss | `[P]` |
| Cascade routing (routing plus cascading) | +8% on RouterBench, +14% on SWE-Bench versus the best baseline; outperforms existing cascades by over 10% in several scenarios | `[P]` |
| DataCamp advice | decision model first, escalate to a reasoning model on low confidence | `[R]` |
| Our own start | Jev first; System 2 agents are already the "escalation" | design |

Caveat: all four use LLM or preference-model confidence; for decision models
the probability is the trained output and may be better or worse as an
escalation signal. Measure it (AUROC of confidence for correctness) before
setting a threshold.

## 7. Worked cascade cost and latency (AA v2 task costs)

Task costs from AA v2: Jev 1.14 $155.55 per 1M tasks (93 ms), Haiku 5.5 (Max)
$6,240 per 1M tasks (32.65 s) `[T]`. Cascade: Jev always, escalate fraction f.

| Escalated f | Cost per 1,000 tasks | Saving vs all-strong ($6.24) | Mean latency |
|---|---|---|---|
| 0.0 | $0.156 | 97.5% | 0.09 s |
| 0.1 | $0.78 | 87.5% | 3.4 s |
| 0.2 | $1.40 | 77.5% | 6.6 s |
| 0.3 | $2.03 | 67.5% | 9.9 s |
| 0.5 | $3.28 | 47.5% | 16.4 s |

Break-even: the cascade costs more than all-strong only when f > 97.5%.
Quality (illustrative): cheap model 94% right on the 70% it accepts, 74% on
the 30% it would escalate, strong model 93% on the escalated: cascade accuracy
0.7 x 0.94 + 0.3 x 0.93 = 0.937, above the strong model's 0.93 because the
cheap model is better on its accepted subset. That is the FrugalGPT +4%
mechanism, not a promise. Latency: the cascade's p90 is the strong model's
latency, so for a spoken product escalate **asynchronously** (acknowledge from
System 1, let System 2 revise; see [dual-process.md](dual-process.md)).

## 8. Drift and re-calibration

- Track, per question family and served model version, a rolling Brier and ECE
  on the audited slice; alarm on a step change in either, and on any change of
  `model` in the response (the alias moved once already, JEV.md).
- Rabanser et al. compared shift detectors: two-sample tests on features of a
  pretrained classifier, used for dimensionality reduction, performed best
  across the shifts they tried `[P]`. Practical form: compare the
  distribution of System 1 probabilities (or encoder embeddings) this week with
  the calibration week.
- Re-fit T per version; keep the old T until the new fit has a few hundred
  audited decisions. Index and version changes alone moved published scores
  within weeks (Jev 1.13: 58 on v1.0; Jev 1.14: 67.0 on v2; different index
  versions) `[T]`, so expect shifts.

## 9. Debiasing options order (cheap before expensive)

| Method | Cost | Evidence |
|---|---|---|
| Shuffle `choices` order per call; keep `none` in a fixed place | free | training with shuffled options "made a bigger difference than more advanced techniques" for d1-3B `[V]` |
| Content-free calibration: ask with "N/A" as the state, divide out the prior | one extra call per question | up to 30 points absolute (GPT-3, few-shot) `[P]` |
| PriDe: estimate the option-ID prior from permutations on ~5% of samples | 5% x k! calls once | transferable across domains `[P]` |
| Full permutation averaging | k x questions | a strong baseline in the PriDe paper `[P]`; Jev questions are blind to each other, so put each permutation in as its own question in the same call and average in code |
| Symmetry test set (reordered options, swapped labels, negated yes/no) | eval only | AA-Symmetry: 440 variants of 120 questions `[T]` |

## Could not verify / open questions

- No source found for how a hosted decision model's probabilities are
  produced (logit softmax versus a trained readout), so temperature scaling on
  its output is an empirical fix, not a derived one.
- The two-thirds ECE reduction (blog) and the 88% / 80% figure are single-source.
- No published cascade curve that uses a decision model's probability as the
  escalation signal.
- AUROC of Jev confidence for correctness: unmeasured by us.
- The size of calibration set needed for stable isotonic maps: no number found.
- Conformal guarantees under our shift (stories lists change daily): untested.

## Sources (fetched 2026-11-10)

1. AA Decision Index v1.0, https://artificialanalysis.ai/articles/artificial-analysis-decision-index `[T]`
2. AA Decision Index v2, https://artificialanalysis.ai/articles/decision-index-v2 `[T]`
3. Guo et al., On Calibration of Modern Neural Networks, https://arxiv.org/abs/1706.04599 `[P]`
4. Tian et al., Just Ask for Calibration, https://arxiv.org/abs/2305.14975 `[P]`
5. Kumar et al., Conformal Prediction with LLMs for Multi-Choice QA, https://arxiv.org/abs/2305.18404 `[P]`
6. Gibbs and Candes, Adaptive Conformal Inference, https://arxiv.org/abs/2106.00170 `[P]`
7. FrugalGPT, https://arxiv.org/abs/2305.05176 `[P]`; AutoMix, https://arxiv.org/abs/2310.12963 `[P]`; RouteLLM, https://arxiv.org/abs/2406.18665 `[P]`; A Unified Approach to Routing and Cascading, https://arxiv.org/abs/2410.10347 `[P]`
8. Rabanser et al., Failing Loudly, https://arxiv.org/abs/1810.11953 `[P]`
9. Zheng et al., PriDe, https://arxiv.org/abs/2309.03882 `[P]`; Zhao et al., Calibrate Before Use, https://arxiv.org/abs/2102.09690 `[P]`
10. MindStudio Jev vs BERT, https://www.mindstudio.ai/blog/jev-vs-classic-classifiers-benchmark `[T]`
11. Liquid Open d1, https://www.liquid.ai/blog/d1-open `[V]`
12. DataCamp, https://www.datacamp.com/blog/top-ai-decision-models `[R]`; AICoder d1-ultra, https://aicoder.com/news/news-20261017-liquid-ai-d1-ultra-decision-model-aa-index-1 `[R]`
13. Our measurements and alias: [../JEV.md](../JEV.md) (M, 2026-11-10)
