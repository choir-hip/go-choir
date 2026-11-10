# Evaluating a System 1 model: question sets, agreement, robustness and cost

Reference subpage (convention: [../README.md](../README.md)). Siblings:
[model-space.md](model-space.md), [calibration-and-cascades.md](calibration-and-cascades.md),
[dual-process.md](dual-process.md). Related: [../SYSTEM1.md](../SYSTEM1.md) section 5.6
(the record), [../ON-DEVICE-MODELS.md](../ON-DEVICE-MODELS.md) section 6
(measurement plan). Fetched 2026-11-10. Tags: `[V]` vendor; `[T]` third party;
`[P]` paper; `[R]` relayed, unverified; `M` measured by us (SYSTEM1.md); "ours" is
arithmetic I ran on stated assumptions.

## Surprises

1. **Percent agreement lies.** Llama-3 8B as a judge scored Scott's pi 59 while
   its percent agreement with humans stayed "well above 80%"; high percent
   agreement can still give scores 10 to 20 points from the human-assigned ones
   `[P]`. Report a chance-corrected statistic and the confusion table.
2. **Skew makes kappa collapse.** 92% raw agreement on a 94%-prevalence label
   gives Cohen's kappa 0.29 (ours, section 2). A rare "yes" is the common
   shape of our questions.
3. **The ceiling is the humans.** MT-Bench: GPT-4 agreed with humans 85% of the
   time (ties excluded), above the 81% humans agreed with each other `[P]`. A System 1 cannot be scored above the labelling noise.
4. **Your gold is a model too, and it moves the score.** The fixed-schema
   router scored routing agreement 0.742 against its LLM-ensemble gold and 0.88
   on a 100-item subset relabelled by a stronger model (+/- 8 points at that
   size) `[V]`.
5. **Compounding.** An AA-Paperclip document needs all 15 answers right: a model
   with 97% per-answer accuracy passed 63% of documents, Haiku with 99% passed
   92% `[T]`. A 12-question Jev call at 97% per question is all-correct 69% of
   the time (ours: 0.97^12). Decide per question whether a call-level failure
   matters.
6. **Reordering the options changes 1 answer in 3 for some models.** AA-Symmetry
   (120 base questions, 440 variants: reordered options, swapped labels, negated
   yes/no): the same answer across variants 99% of the time for Mammoth 27B, 91%
   Drex Max, 88% GLiDE, 67% Clef `[T]`.
7. **Judges are fooled by almost nothing.** "Master key" openers such as
   "Thought process:" or a blank answer made LLM judges and reward models
   emit false positives, up to 80 to 90% in some settings, including GPT-4o
   and Claude-4 `[R]` (search summary of arXiv 2507.08794; not read in full).
8. **Position bias in judges is large and measurable.** With two near-equal
   answers, swapping order kept GPT-4's verdict only 65.0% of the time; Claude-v1
   23.8%; GPT-3.5 46.2% `[P]`. Swapping and requiring agreement is the cheap fix.
9. **A trained 22M encoder beat Jev zero-shot on Banking77 (93.2 vs 80.1)
   and lost badly on Yelp stars (51.9 vs 67.2).** The winner depends on whether
   the label set is stable and labelled `[T]`.
10. **On-device marginal cost is energy, and it is tiny:** Apple's DistilBERT
    figures imply 1.58 millijoules per inference at 0.454 W for 3.47 ms, or
    0.68 mJ at 0.072 W for 9.44 ms (ours, from Apple's pairs `[V]`).

## 1. Building a question set for a decision model

A decision question has: a state, an instruction, closed-set criteria, and a
gold answer. Build the set so that it can fail in the ways the product fails.

1. **Draw from the record.** Our Jev record (internal/jevrecord) keeps the
   request and answer but, per the turns note, "Jev's record has no state";
   so the set needs regenerated states from kept turns. Freeze the states
   with the questions.
2. **Stratify** by question family, by answer (include enough `none` / "no"
   items: the rare class carries the risk), by language, and by state length
   (the phone pays for length, section 4 of model-space.md).
3. **Label** with a panel, not one judge: the fixed-schema router's gold was the
   plurality of five open LLMs with at least three votes per retained item,
   34,671 train / 2,275 dev / 2,175 test, deduplicated against the public
   JevBench items `[V]`; add a human or stronger-model relabel on a subset to
   estimate the gold's own error (surprise 4).
4. **Add the variants** that AA-Symmetry uses: reorder choices, swap label
   names, negate yes/no, and re-word the abstain option (AA words "I don't know"
   ten ways) `[T]`. Add paraphrases of the instruction. A decision model that
   flips under these is measuring format, not meaning. Strands Decider's
   reviewer saw a yes/no go from 0.209 to 0.912 by rewording "blocked right now"
   to "needs help today" `[R]`.
5. **Add adversaries** in the state: instruction-like text inside a story, empty
   state, a state with only punctuation (judge false-positive studies, surprise 7).
6. **Split once, early:** dev for thresholds and temperature, test touched
   rarely, and an "audit" slice from live traffic for drift
   (calibration page sections 3 and 8). Never calibrate and score on the same
   items.
7. **Sample size:** to estimate accuracy to +/- 3 points at 95% with no prior,
   n = 1.96^2 x 0.25 / 0.03^2 = 1,068 items (ours). Wilson intervals (ours):

   | Correct / n | 95% interval |
   |---|---|
   | 90 / 100 | 0.826 to 0.945 |
   | 450 / 500 | 0.871 to 0.923 |
   | 900 / 1,000 | 0.880 to 0.917 |
   | 1,800 / 2,000 | 0.886 to 0.912 |

   Per question family the n is smaller; a 100-item family cannot distinguish
   two models 5 points apart.

Public yardsticks (for cross-checking, not substitutes): AA's four v1 sets
(AA-Omniscience-Decision with 500 private questions and a 5-option format
including "I don't know", HLE-MultiChoice with 513 items, NVIDIA's When2Call,
Ai2's WildGuardTest), and in v2 AA-Paperclip (1,500 private documents, 5
decisions and 10 facts each), AA-Symmetry, and an O*NET benchmark from TypeSafe
at 15% `[T]`. 45% of v2 is private, the point being that public sets leak into
training `[T]`.

## 2. Agreement metrics and ceilings

| Metric | Use | Watch for |
|---|---|---|
| Percent agreement | quick look | inflated by skew; 10 to 20 point errors vs the truth `[P]` |
| Cohen's kappa | two raters, chance-corrected | collapses at skew; prevalence and bias paradoxes |
| Scott's pi | judge vs human alignment | the judge study moved from Cohen's kappa to Scott's pi because it separated judges better `[P]` |
| Per-class precision and recall | rare "yes" | the headline accuracy hides them |
| Brier, log score, Decision Score | probabilities | see calibration page; AA v2 headline `[T]` |
| ECE, reliability diagram | calibration | bin-dependent |
| AUROC of confidence for correctness | cascade quality | the cascade lives or dies by it `[P]` |
| Agreement with the teacher on each head | distillation | the student inherits the teacher's mistakes (tinydistill README `[R]`) |

Worked kappa (ours), two raters, 1,000 items:

```text
                 B yes   B no
A yes            420     40         p_observed = (420+480)/1000 = 0.90
A no              60    480         p_chance   = .46*.48 + .54*.52 = .5016
                                    kappa = (0.90 - 0.5016)/(1 - 0.5016) = 0.799
```

Same raters on a skewed label (94% "yes"):

```text
A yes 900  B no 40 / A no 40  B no 20    p_observed = 0.92
p_chance = .94*.94 + .06*.06 = 0.8872      kappa = (0.92 - 0.8872)/(1 - 0.8872) = 0.291
```

Rules: (1) measure inter-annotator agreement on a double-labelled subset
first; that is the ceiling for every later number. (2) A System 1 with kappa to
the gold within the humans' kappa to each other is as good as the labels allow.
(3) Promote on the lower bound of an interval, not the point value.

## 3. Robustness of judges and decision models

| Test | How | Published reference |
|---|---|---|
| Order swap | ask twice with choices (or answers) reversed; count verdict changes | GPT-4 consistent 65.0% (default prompt) to 66.2% (renamed assistants); GPT-3.5 46.2% to 51.2%; Claude-v1 23.8% to 56.2%; swapping and calling inconsistent results a tie raised GPT-4 consistency from 65.0% to 77.5% `[P]` |
| Option-ID prior | permute option contents; look at per-ID recall spread (RStd) | gold moved to D: gpt-3.5-turbo 67.2 to 60.9; Llama-30B picks A/B/C/D 34.6/27.3/22.3/15.8% on balanced items `[P]` |
| Label swap and negation | AA-Symmetry style | 99% vs 67% same-answer rate `[T]` |
| Instruction paraphrase | 5 to 10 re-wordings per question | 0.209 vs 0.912 anecdote `[R]` |
| Verbosity and repetition | repeat a list in the answer | "repetitive list" attack: Claude-v1 and GPT-3.5 failed 91.3%, GPT-4 8.7% `[P]` |
| Master-key and blank answers | empty, punctuation, "Thought process:" | false positives up to 80 to 90% `[R]` |
| Batch contamination | put an injected instruction in one of N batched items | an attack benchmark exists for batch prompting `[R]` |
| Question count | 1, 3, 6, 12, 20 questions in one call | Jev up to 12 "unmeasured above 12" (SYSTEM1.md); LLM multi-problem: classification 75.5% single vs 72.3% batched `[P]` |

For Jev, questions are answered independently; the realistic risk is the
shared `state`. Add a test where a story's text contains a command.

## 4. Cost per thousand decisions, from fetched prices

Workload (ours, adjustable): one shared state of 800 tokens, 6 questions of 60
tokens each (instruction plus criteria), so 1,160 input tokens per call. A
"decision" is one answered question; 1,000 decisions = 167 calls of 6 questions.

| Approach | Price used (source) | Input tokens billed per decision | $ per 1,000 decisions | $ per 1M decisions |
|---|---|---|---|---|
| Jev, one call of 6 | $0.042/M input, output free (SYSTEM1.md `[V]`) | 193 | **0.0081** | 8.1 |
| d1-ultra, state billed per question | $0.042/M (`[R]`; per-question billing `[R]`) | 860 | 0.0361 | 36.1 |
| Perplexity Decider hosted, one call | $0.02/M (`[R]`; another source says $0.04) | 193 | 0.0039 (0.0077 at $0.04) | 3.9 |
| OpenAI Decisions (gpt-6-luna), one call | $0.10/M, output and cache free (`[R]`) | 193 | 0.0193 | 19.3 |
| Claude Haiku 5.5, one prompt per question, 5 output tokens | $0.10/M in, $0.50/M out (`[V]` pricing page) | 860 in + 5 out | 0.0885 | 88.5 |
| Haiku 5.5, state cached (read $0.01/M), per question | cache read $0.01/M (`[V]`) | 800 cached + 60 + 5 out | 0.0165 | 16.5 |
| Haiku 5.5, 6 questions in one prompt, 60 JSON output tokens | as above | 193 in + 10 out | 0.0243 | 24.3 |
| On-device model | $0 marginal; about 1 to 2 mJ per encoder pass (surprise 10) | 0 | 0 | 0 |

(Cache writes, minimum cacheable length, and rate limits are ignored; Haiku
5.5 above 100K-token prompts costs five times more `[V]`.)

Observations: (a) sharing the state beats caching it: Jev beats cached Haiku by
2x because the state is billed once per call at a lower rate and there is no
output; (b) a per-question-billing API is 4.5x dearer than Jev on this shape;
(c) the cheapest option is not the one with the lowest token price (AA: cost per
task varies over 60x and "token price is not the main driver") `[T]`; (d) the
real cost of a cloud System 1 is the dollars plus the 100 to 300 ms; the
real cost of an on-device one is engineering plus the accuracy gap.

Cross-check with AA's per-task costs, which include larger tasks `[T]`:

| Model | $ per 1M tasks (v1.0) | $ per 1M tasks (v2) | Time per task |
|---|---|---|---|
| Jev 1.13 / 1.14 | 42 | 156 | 0.13 s / 93 ms |
| Haiku 5.5 low-effort (v1.0) / Max (v2) | 184 | 6,240 | 1.4 s / 32.65 s |
| GPT-6 Luna low / Max | 131 | 1,530 | 1.5 s / 11.34 s |
| Celeris-1.1 Decision | n/a | 14 | 56 ms |

Break-even for building a phone model (ours): at $0.0081 per 1,000 Jev
decisions, a fleet doing 1M decisions a day pays about $8 a day (about $3,000 a
year; the old JEV.md estimated the fleet at about $1 a day; measured spend is $7.47 a day, see SYSTEM1.md section 7). A
distilled phone model does not pay for itself in dollars at this volume. Its
case is latency, offline use and privacy (no text leaves the phone), as the
turns note says.

## 5. A promotion bar for a phone model (proposal)

A phone model earns a question when, on the frozen test set for that question
family:

```text
n >= 300 items (>= 50 of the rare class)
agreement_with_Jev (Scott's pi or kappa) lower 95% bound >= human-human kappa - 0.05
Brier(phone) <= Brier(Jev) + 0.02 after temperature scaling on dev
ECE(phone) <= 0.03 ; no confidence bin with n >= 30 off by more than 0.10
symmetry: same answer under reorder/negate on >= 95% of variants
shadow period: >= 7 days of live traffic scored against Jev, no drift alarm
```

The numbers are mine and meant to be tightened by data. Record every phone
decision like Jev's and score it the same way (turns note). Shadow mode costs
nothing on the cloud side because Jev is already called.

## Could not verify / open questions

- The master-key paper (2507.08794) was seen only as a search summary.
- Any published learning curve of labelled examples versus agreement with an
  LLM judge (none found); numbers for the 22M encoder on Banking77 come from a
  blog.
- Whether decision models degrade with more than 12 questions per call: no
  vendor or independent measurement found; ours is "unmeasured above 12".
- Per-question billing for d1-ultra, GLiDE and Decider Flash comes from AA
  (GLiDE, Flash) and an aggregator (d1-ultra); the Liquid pricing page was not
  fetched.
- Energy per inference for a decision model on an A16: no figure; the
  DistilBERT numbers are for an A15-class Neural Engine and a different model.
- Human labelling cost per item: no source fetched.

## Sources (fetched 2026-11-10)

1. Zheng et al., Judging LLM-as-a-Judge with MT-Bench, https://arxiv.org/abs/2306.05685 `[P]`
2. Thakur et al., Judging the Judges, https://arxiv.org/abs/2406.12624 `[P]`
3. Zhao et al., One Token to Fool LLM-as-a-Judge, https://arxiv.org/abs/2507.08794 `[R]` (search result only)
4. AA Decision Index v1.0, https://artificialanalysis.ai/articles/artificial-analysis-decision-index `[T]`; v2, https://artificialanalysis.ai/articles/decision-index-v2 `[T]`; BenchLM snapshot, https://benchlm.ai/benchmarks/artificial-analysis-decision-index `[T]`
5. System1models router card, https://huggingface.co/system1models/s1-llm-auto-router `[V]`
6. MindStudio, https://www.mindstudio.ai/blog/jev-vs-classic-classifiers-benchmark `[T]`
7. Anthropic pricing, https://platform.claude.com/docs/en/about-claude/pricing `[V]`
8. Apple ANE transformers, https://machinelearning.apple.com/research/neural-engine-transformers `[V]`
9. Zheng et al., PriDe, https://arxiv.org/abs/2309.03882 `[P]`; Evaluating LLMs with Multiple Problems at once, https://arxiv.org/abs/2406.10786 `[P]`
10. Pinggy, https://pinggy.io/amp/blog/best_ai_decision_models/ `[R]`; AICoder, https://aicoder.com/news/news-20261017-liquid-ai-d1-ultra-decision-model-aa-index-1 `[R]`; AI Weekly (Perplexity), https://aiweekly.co/alerts/perplexity-open-sources-27b-decider-edges-jev-on-11-test-panel `[R]`
11. Batch prompting attack benchmark, https://arxiv.org/abs/2503.15551 `[R]`
12. Distillation search results (COLING 2025 BERT distillation paper; tinydistill README) `[R]`
13. Our measurements: [../SYSTEM1.md](../SYSTEM1.md) (M, 2026-11-10); [../../design/autoradio-turns-2026-11-10.md](../../design/autoradio-turns-2026-11-10.md)
