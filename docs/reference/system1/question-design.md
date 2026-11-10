# Writing questions for Jev: the vendor's rules, the failure modes, calibration, a gallery

Subpage of the System 1 reference; siblings [jev-vendor.md](jev-vendor.md),
[field-reports.md](field-reports.md). Sources fetched 2026-11-10. Tags:
**[V]** vendor, **[T]** third party, **[R]** relayed or unverified, **[C]**
constructed here to illustrate a sourced rule (not a measured result). Source keys resolve in Sources. Method caveat: pages were
read through a summarising fetch tool; two PDFs were read as raw text.

## 1. The ten rules the vendor states

1. **One judgment per question.** "Hiding several judgments in one question" is
   on the vendor's avoid list; a two-condition noul "means less": ask two and
   combine in code [V jagged][V noul]. A broad question "hides several
   judgments behind one answer" [V howto].
2. **Phrase so a high value means yes.** "Is the message free of personal data?"
   inverts the meaning and is "easy to misread later" [V noul]. Contradiction
   between `instructions` and `criteria` (a noul whose `true` means "no")
   "performs worse" [V jagged].
3. **Point at the state by path.** Backticked paths such as
   `support.tickets[0].message`; keep question text short, put examples and
   background in separate fields [V howto].
4. **Send only what the question needs.** Irrelevant state is a distractor and
   accuracy drops as it grows; no numbers are published for how fast [V jagged].
   Filter in code; if you cannot, use a noul to filter for relevance first.
5. **Do exact things in code**: counting, arithmetic, dates, colour-value
   comparison, sorting. Ask Jev for the judgment and compute the rest [V jagged].
6. **Always give a choice a way out, and prefer a companion noul to `other`.**
   `other` "competes with the product options for probability"; the vendor's
   tests found a separate noul with concrete true/false criteria works better.
   The Choice page also warns a "none of the above" option "may not work well
   in many cases" [V companion][V choice].
7. **Describe options by what they cover and what they do not**, as objects
   with fields of your own naming, when two options are easy to confuse
   [V choice][V advanced]. A matching example moved one Score from confidence
   0.35 to 0.96; an unrelated example did nothing [V score].
8. **Score levels are situations, not degrees.** "Broken or degraded feature,
   but a workaround exists" works; "moderately severe" does not. The model sees
   only the descriptions, not the numbers or neighbours. Use as many levels as
   you can describe distinctly (3 is fine; the API takes 1 to 10) [V score].
9. **Keep thresholds in code, not wording.** "Thresholds belong in your code"
   [V noul]; changing a constant is a cheaper change than rewording a question
   [V cookbook-rag].
10. **Reorder options and check the answer is stable.** Jev "leans toward the
    first option" [V jagged]. The vendor gives no number (section 2).

The vendor's coding-agent skill says "Agents aren't great at writing questions,
so expect to edit collaboratively" and tells reviewers to focus on the questions
and threshold constants [V agent-skill].

## 2. Documented failure modes (vendor list of nine, plus what others measured)

| Mode | Vendor statement [V jagged] | Independent evidence |
|---|---|---|
| Literal reading | negations, scope words, implied conditions taken at face value; "if you find yourself explaining what you meant, add that to the instruction" | wrong criteria wording is catastrophic: accuracy fell from 95.9% to 16.7% in one study (a reseller's summary of a "pre-registered study", original not found) [R orca-know] |
| Counting and numbers | counts unreliably (characters, occurrences, items); weak on hex/RGB; Score is weakly calibrated numerically | MATH-500: 83.1% choosing the right option, 13.6% producing digits [T jevresearch] |
| Dates | read as text; ordering and windows unreliable; extract parts with a choice including "not stated", compute in code | vendor date cookbook: 6 of 6 correct with code-side math, one deliberate "not stated" routed to review [V cookbook-date] |
| Indirection | double negatives, properties of properties, multi-hop | an author found a question and its negation summed to 1.19 (0.72 and 0.47) [T orca-lim, relayed from vendor material] |
| Large irrelevant state | "context rot" | no numbers; long-context mode "slower and less accurate" [V models] |
| Adversarial content | injected instructions or self-arguing text can change the answer; "improvement expected later" | independent reviewers: prompt injection through supplied content works [T jevresearch]; vendor RAG cookbook scored a planted injection 0.98 on a dedicated noul and still says the filter "is one filter, not a security boundary" [V cookbook-rag] |
| Contradictory instructions/criteria | worse performance | not measured elsewhere |
| Option order | "leans toward the first option" | see below |
| Generation | chaining choices to generate text "works poorly and is very slow" | n/a |

Option order and position, the numbers that exist:

| Measurement | Result | Tag |
|---|---|---|
| Rotation audit, 419 observations of 140 items x 3 orderings | 22 discordant pairs, 5.3% flip rate; native 86.6% versus rotated 86.2% (mean diff +0.36 pts, CI -2.5 to +3.3) | [T jevresearch] |
| Eight shuffled presentations per item (Jev 1.13 and 1.14, twelve reproductions, six LLMs) | accuracy varies under 2 points, yet the cumulative failure set grows 27.3 to 28.5% over the original order; reorder loses more initially correct answers than fixed-order repetition; voting helps modestly; unanimous answers are often wrong | [T arxiv-shuffle] (abstract only) |
| Pairwise preference, base order only | JudgeBench: preferred answer in slot A 75.5% right versus slot B 81.9% for Jev; GPT-6 93.5 versus 92.8 | [T judge-paper] |
| Creative menus, 11 orderings of one flat menu | 10 distinct winners; Spearman against native order 0.26 to 0.44 on flat steps | [T jevresearch] |
| Inert padding options (200 gold items, K=2 to 255) | p(gold) 1.000 to 0.992; padding gets 0.00 mass: irrelevant options do not dilute | [T jevresearch] |

Reading: order matters most when the options are close (flat distribution);
extra obviously-wrong options are cheap. For a pair, ask both orders and
average [T judge-paper]; for a menu, rotate and compare.

Other documented limits: 1.13 is English-first, no streaming, no conversation
[T orca-lim]; text-only beyond images (images are "dreamer-ready") [V images];
2025 events are poorly known (1 of 12 correct) [T jevresearch]; Box always
returns a box, so pair it with a noul asking if the target is present [V
primitives].

## 3. Probabilities and confidence: what they are, how far to trust them

- Choice confidence is `(pmax - 1/n)/(1 - 1/n)` [V confidence]; a choice's
  confidence "is not the probability that the selected option is correct" and
  with a fixed n it is a transform of the top probability, so route on the
  probability of the option you will act on, or the top-to-second ratio
  [V confidence][V probability-routing]. Noul has no confidence: use the number
  or `|2p-1|`. Score: check confidence first (is the answer settled?), then
  route on the score; for a boundary decision sum the probabilities on the side
  that matters, because two adjacent top levels put the score between them with
  only a small confidence penalty [V confidence-routing].
- **Calibration is aggregate, not per answer** [V ml-primer]. The vendor
  publishes no calibration curve, Brier score or ECE; the confidence page "does
  not describe a formal calibration procedure or make calibration claims" [V
  confidence]. A reviewer's list of what would settle it: a calibration study
  on human-labelled data outside the launch workflows [T pearpages].
- **Quantisation**: all probabilities are multiples of 0.01 [T meng][T
  jevresearch]; 66.9% of returned entries were exactly 0.00; 71.6% of AG News
  four-class answers returned top class exactly 1.00 and were 94.3% correct, so
  thresholds on top probability reach only 53 operating points on 1,500 items;
  thresholding on `confidence` reaches 68; probability then confidence as a
  tie-break 108 [T meng].
- **`confidence` carries a little extra** beyond the printed probabilities:
  51 distinct vectors map to more than one confidence; inside the 1.00 bucket
  confidence 0.99 had 14.3% errors (n=28) versus 5.45% at 1 (n=1,046) on AG
  News, but the reverse on SST-2 [T meng]. Do not rely on it as a tie-break
  without checking on your data.
- **Calibration depends on question form and task** (Jev 1.13.0, SST-2 and AG
  News, 3,244 evaluations) [T meng]:

| Condition | Accuracy | ECE (15 bins) |
|---|---|---|
| SST-2, noul form | 0.948 | 0.091 (underconfident) |
| SST-2, choice form, same sentences | 0.961 | 0.020 (near calibrated) |
| AG News, four-way choice | 0.867 | 0.089 |
| AG News, scores below top bin | n=258: stated 0.750, observed 0.612 | overconfident as a block |

  One fitted temperature repaired held-out ECE to 0.023 to 0.028 where it was
  miscalibrated (T 0.47 for noul, 3.46 for AG News) but the fitted T moved from
  1.28 to 6.95 with the clipping floor, so only its direction is trustworthy.
  Split conformal prediction hit its 95% marginal target but singleton answers
  were wrong 5.93% at a nominal 5%. Label descriptions alone moved Jev's gap to
  an open NLI classifier from +6.93 to +9.67 points; the choice of rival moved
  it about 8 points and flipped its sign: benchmark choices rival model choices.
- **Across other work**: pooled ECE 0.047 over 44 alignment benchmarks, median
  per-benchmark ECE 0.168 versus a null of 0.074 ("ranks well, does not
  transfer as a threshold") [T alignbench]; HLE multiple choice: mean top
  probability 0.634 against accuracy 0.219 [T jevresearch]; MMLU-Pro near
  calibrated on selected answers (median p(gold) 0.93) [T jevresearch]; on
  HaluEval Brier 0.176, ECE 0.070 against a benchmark label, which after human
  relabelling of 28 of 240 items became ECE 0.017 [T judge-paper]. Figures from
  one reseller (ECE 0.107 against a 0.024 noise floor, score primitive 44.7%
  accurate, 900 synthetic tickets) could not be traced to a primary source
  [R orca-know].
- Soft beats hard. Keep answers as probabilities: reading the 3-level Score as
  its expected level beat the generic noul on 19 of 31 benchmarks (median
  +0.016 AUROC); argmax readouts lost nearly everywhere (Choice 2/0/28, Score
  5/2/24 wins/ties/losses); thresholding atoms at 0.5 before combining lost to
  one direct question on 9 of 10 benchmarks (median -0.137 AUROC) [T alignbench].
- Repeatability: byte-identical requests gave 13 distinct answers out of 30;
  TVD between repeats 0.034 to 0.122 on flat 254-option menus [T jevresearch];
  a noul read 0.42 then 0.44 (ours, JEV.md); vendor-measured least-stable flip
  4.0% on the average moderation post [V cookbook-consist-c]. Use bands.

## 4. Calibration procedure and threshold starting points

Vendor guidance, assembled [V confidence][V noul][V howto]:

1. Start conservative; test on your own data; adjust. Plot confidence against
   accuracy on labelled data ("calibrate thresholds by plotting confidence
   against accuracy on your data").
2. Three bands: act, confirm, route to a person/stronger model. Riskier action
   means higher threshold; destructive or costly actions higher than read-only.
3. Noul: 0.7 yes, 0.3 no, the band between its own outcome; raise the yes
   threshold if a false yes is costly (refund), lower it if a miss is (safety).
   Do not fit thresholds tightly to a few cases.
4. Thresholds are product decisions: the same probabilities give different
   actions under strict (review 0.35, action 0.70) and permissive (0.70, 0.85)
   policies [V cookbook-guardrails].
5. Keep stored answers and re-route offline: changing thresholds then costs no
   calls [V cookbook-rag][V or-moderation].

Threshold values used in vendor and OpenRouter examples (illustrations, tune
on your data):

| Use | Threshold | Source |
|---|---|---|
| Noul yes / no | 0.7 / 0.3 | [V noul] |
| Choice, human review floor | confidence 0.5; confirm unless above 0.9 for high stakes | [V confidence] |
| Choice, voice action | probability 0.7 for any action; 0.9 for a transfer | [V probability-routing] |
| Companion noul gate | noul below 0.3 = no option fits; both above 0.7 = route | [V companion] |
| Score + confidence | confidence below 0.5 to triage; then score cuts 1.5, 2.5 | [V confidence-routing] |
| Citation check | auto-accept at confidence 0.8 | [V cookbook-cite] |
| 75-way SIC choice | confidence 0.9, else fall back to coarser division (stated 70% always-group, 93% on confident, 78% on unsure-as-division) | [V cookbook-sic] |
| Date extraction | review below 0.60 | [V cookbook-date] |
| Cascade verifier | P(rule broken) above 0.7 in text; best operating point 0.29 | [V cookbook-sde] |
| Tag classification (TweetTopic) | review below 0.8; per-tag threshold sweep 0.3 to 0.9 | [V or-classification] |

OpenRouter's worked calibration (TweetTopic, 150 calibration items, 400 test)
[V or-classification]: category right on 127 of 150 (84.7%); by confidence
band: >= 0.8: 114/122 (93.4%); 0.5 to 0.8: 9/18 (50.0%); < 0.5: 4/10 (40.0%);
400-item batch 87.5%; the same sweep shows the right threshold differs by tag
(`sports` clean at every threshold; `celebrity` precision 0.24 to 0.41 at any
threshold, whose lesson was "fix the question rather than the threshold").

## 5. Published benchmarks relevant to question design

| Question | What is published | Tag |
|---|---|---|
| Batching: one call versus many | 13 questions (8 noul, 2 choice, 3 score) over a 53,777-character document, jev-1.14.0, 5 runs each: one call $0.000497 and 0.24 s; 13 calls $0.006090 and 2.81 s (12.2x cheaper, 11.8x faster, sequential assumption); largest gap in mean answer 0.004; 7 of 13 identical in all runs; worst run-to-run std 0.0141 | [V cookbook-parallel] |
| Same, independent | 23,411 calls, 11.4 questions per call (4 to 28): latency uncorrelated with question count (r = -0.037) or tokens (r = -0.002); error rate 0.12%; median 0.313 s (p90 0.55, p99 1.86) | [T alignbench] |
| Accuracy versus question count per request | **none published.** Decomposition "neither helps nor hurts on average" (median 0.000 AUROC split-half) | [T alignbench] |
| Accuracy versus state size | **none published**; vendor says it falls; long-context mode costs accuracy, no number | [V jagged] |
| Accuracy versus option count | none; latency flat 2 to 255 options (65 to 90 ms); inert options cost nothing in accuracy | [T jevresearch] |
| Question wording | generic wording "matters little"; targeted wording +0.006 AUROC out of sample (CI includes zero); naming the construct +0.017 to +0.029; statement versus question forms equivalent (-0.003) | [T alignbench] |
| Context | adding deployed-monitor context helped on 1 of 7 benchmarks; a reference that defines the label gave large gains (0.79 to 0.95) | [T alignbench] |
| Where the label lives | the answer is only as good as the state: labels the state cannot reveal cannot be recovered | [T alignbench] |
| Noul versus choice | equal accuracy once both carry the same definitions; Brier favours noul by 0.0115 (BoolQ) | [T alignbench][T jevresearch] |
| 255-option choice | vendor cookbook ran 194 skills and 75 SIC groups in one choice; chunking suggested beyond | [V cookbook-skill][V cookbook-sic] |

The "13-question benchmark" in our notes is the first row.

## 6. Gallery: good and bad

Format: bad, good, why. Examples marked [V] are from vendor pages; [C] are
constructed here from a sourced rule; none is a measured result unless a
number is given.

**1. Compound noul.** Bad [C]: `Does the message ask for a refund and mention an order id?`
Good: two nouls, `refund_requested`, `mentions_order_id`, combined in code.
Why: a two-condition value "means less" [V noul].

**2. Inverted phrasing.** Bad [V]: `Is the message free of personal data?`
Good: `Does the message contain personal data?` Why: high must mean yes [V noul].

**3. Counting.** Bad [C]: `How many times is "refund" mentioned?` as a score.
Good [V]: one noul per candidate item, threshold at 0.5, sum in code.
Why: counting is unreliable and worsens with size [V jagged].

**4. Dates.** Bad [C]: `Is the deadline before the end of the quarter?`
Good [V]: choices for month, day, year with `none` and `out_of_range`; code does
the calendar and uses the weakest confidence among the parts [V cookbook-date].

**5. No-fit choice.** Bad [V]: a three-product choice with an `other` option
(probability splits for a message half about a product). Good [V]: the product
choice plus a noul "is it about any of these products"; trust the choice only
when that noul is above 0.7. Real numbers: a desk-lamp message got `desk_pro`
0.96 yet companion noul 0.02 [V companion].

**6. Score levels.** Bad [C]: `["minor", "moderate", "serious"]`. Good (paraphrasing the vendor's example) [V]:
`["Cosmetic only, no behaviour affected", "Broken or degraded feature, but a
workaround exists", "Blocking: no workaround"]`. Why: situations are matchable,
labels are not [V score]. Result in the doc: a misaligned button 0.0 at
confidence 1.0; a Safari-only crash 1.43 at 0.35 (levels 1 and 2 split).

**7. Ambiguous option boundary.** Bad [C]: `return_policy: "returns"`,
`return_status: "returns"`. Good [V]: object descriptions stating what each
covers and what it does not; the ticket resolved at confidence 1.0
[V choice].

**8. Several judgments per question.** Bad [C]: one choice over twenty business
rules with "pick the rule that applies". Good [V]: six independent nouls and
code that combines them (a customer-service flow with six exit paths); wrong
routing then traces to a judgment or a rule, and a new outcome adds a judgment
without disturbing the rest [V dont-call-it]. Caveat: independent evidence
finds decomposition neutral on accuracy on average [T alignbench]; its benefit
is observability and reuse.

**9. Evidence discipline in the state.** Bad [C]: pasting the whole thread plus
a policy when the question is about one message. Good [V]: object state with
named parts and the question pointing at `ticket.message`; for retrieval, one
request per query-passage pair with four nouls (`is_relevant`,
`contains_answer_evidence`, `contradicts_query_premise`, `contains_prompt_injection`)
and routing in code in a fixed order: injection above 0.70 excludes first,
then contradiction above 0.70, relevance below 0.45, evidence above 0.55
[V cookbook-rag].

**10. Boilerplate criteria.** Bad (ours, JEV.md): per-candidate criteria repeated
80 times, a 1.14 intake call of about 9,000 tokens, past the long-context line.
Good [C]: candidates once in state, each question naming its handle, short
parallel criteria. Whether criteria can be shortened or dropped is untested.

**11. Asking a model to do the grader's job.** Bad [C]: `Rate 1 to 10 how good
this answer is`. Good [V]: three narrow checks (grounded, answers the question,
declines) as a choice `supported / unsupported / declined`, with the criterion
that the answer must address the user's question, added after a grounded
off-topic draft passed [V or-cascade].

**12. Pair preference without order control.** Bad [T]: ask once in one order.
Good [T]: ask both orders and average aligned probabilities; or at least
accept the slot-A/slot-B gap (75.5% versus 81.9% on JudgeBench) as a bias to
test for [T judge-paper].

**13. Misaligned wording in criteria.** Bad [T relayed]: criteria descriptions
that name the wrong thing take zero-shot accuracy from 95.9% to 16.7% [R
orca-know]. Good: test criteria on inputs with known labels first; higher
confidence alone does not show a description is better [V score].

## 7. Could not verify / open questions

1. Any vendor number for accuracy against state size, question count or option
   count; none was found on any fetched page.
2. The "pre-registered study" (5,721 calls; 95.9% to 16.7%; 0 of 30 out-of-scope
   flagged) and the "calibration study" (900 synthetic tickets, ECE 0.107): both
   appear only in one reseller's summary; searches found no primary source [R].
3. The first-option lean for 1.14 in plain choices (not pairs); whether the
   0.7/0.3 noul defaults are calibrated for any workload (no curve published).
4. Detailed numbers of the shuffle paper (only the abstract was fetched) and
   the remaining 74k characters of the judge paper (offset 200,000 unread).
5. The Towards Data Science cascade article and two Korean eval posts (403).
6. Score `confidence`: only a qualitative description is published.

## Sources (fetched 2026-11-10)

Vendor docs (docs.typesafe.ai): `jagged` /model-jaggedness.md; `confidence` /confidence.md; `choice` `score` `noul` `advanced` /primitives/{choice,score,noul,advanced}.md; `howto` /concepts/how-to-build-with-system-one.md; `companion` /patterns/companion-noul.md; `probability-routing` /patterns/probability-routing.md; `confidence-routing` /patterns/confidence-routing.md; `ml-primer` /introduction/machine-learning-primer.md; `models` /models.md; `images` /images.md; `agent-skill` /agent-skill.md; cookbooks under /cookbooks/: `cookbook-parallel` parallel_questions.md, `cookbook-consist-c` consistency_choice_cookbook.md, `cookbook-rag` classifying_rag_passages.md, `cookbook-guardrails` llm_guardrails.md, `cookbook-cite` citation_check.md, `cookbook-sic` classification_using_confidence.md, `cookbook-date` date_extraction_cookbook.md, `cookbook-sde` sde_cascade.md, `cookbook-skill` skill_suggestion.md. `dont-call-it` https://typesafe.ai/blog/dont-call-it-a-decision-model.
OpenRouter: `or-classification` https://openrouter.ai/docs/cookbook/evaluate-and-optimize/jev-classification.md; `or-moderation` https://openrouter.ai/blog/tutorials/how-to-use-jev/ ; `or-cascade` https://openrouter.ai/docs/cookbook/evaluate-and-optimize/jev-verified-cascade; `or-tutorial` https://openrouter.ai/docs/guides/community/jev-tutorial.md.
Third party: `jevresearch` https://github.com/JevResearch/Jev-Research (ARCHITECTURE-ANALYSIS.md); `meng` https://escholarship.org/content/qt4t4449jv/qt4t4449jv.pdf; `alignbench` https://arxiv.org/pdf/2609.29429; `judge-paper` https://arxiv.org/html/2609.26550v5 (first 200,000 of 274,637 characters); `arxiv-shuffle` https://arxiv.org/abs/2610.35826 (abstract); `pearpages` https://pearpages.com/blog/2026/09/16/jev-sorted-what-typesafes-system-one-model-actually-is-and-what-is-still-just-a-claim; `orca-lim` https://www.orcarouter.ai/blog/jev-limitations; `orca-know` https://www.orcarouter.ai/blog/jev-typesafe-system-one-what-we-know.
