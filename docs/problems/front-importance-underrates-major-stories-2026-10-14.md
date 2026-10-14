# Front importance underrates major stories, 2026-10-14

Station LF, second pass on the Autopaper front-page importance rank.
Mutation class of the fix: orange (the host scorer's judgement prompt,
which changes the front page's order; no provider routing change).

## Evidence

- Owner, about 14:10 UTC: "we're not getting the story of [...] the OpenAI
  employee, a Saudi national, who put a listening device in OpenAI's
  executive conference room. Obviously I don't want to hard code that
  story as super important, but our general system should be able to know
  that that is like a hugely important story." Earlier: "I would trust
  models to understand that the OpenAI spying story is more important than
  some story about Codex, which is basically an ad."
- Live front row (GET https://choir.news/api/platform/publications/front?limit=100,
  read about 14:10 UTC): "OpenAI engineer arrested for bugging executive
  conference room", section tech_ai, 7 sources, 1114 words, **importance
  5**. Its dek, as the scorer sees it, is cut at about 240 characters and
  does not say who the engineer is or why it matters ("...accusing him of
  hiding it there to…"). Jev also marked "Both Russia and Ukraine use
  AI-powered drones on the battlefield" and "Badenoch–Farage dispute over
  Chagos Islands and Trump" as promo; both have broken deks (writer process
  text, "No verified execution findings are available..." and "This
  document will explain..."), a separate problem upstream of the ranker.
- The scorer (`internal/frontimportance/scorer.go`, `corpusd
  front-importance`, every 10 minutes) asks Jev (typesafe/jev-1.13, the
  OpenRouter Decisions API) per story, in isolation, from title and dek:
  importance "1".."10", promo yes/no, section. The front page ranks
  importance × depth × 0.5^(age/24h) (`frontend/src/lib/autopaper.ts`).
  Recomputed with that formula over the live rows, the bugging story sits
  **23rd** on the front page.

## Evaluation

Set: the 100 live front stories at about 14:10 UTC (titles and deks;
saved outside the repo at `~/.config/choir-qa/traces/lf/items.json`, runs
under `~/.config/choir-qa/traces/lf/judge/`).

Reference: Gemini 3.8 Flash (`omp -p --model
google-antigravity/gemini-3.8-flash --thinking high --no-tools`), one
listwise call ranking all 100 as a serious newspaper's front-page editor,
sampled twice; the reference is the mean of the two rankings. Its
self-agreement bounds what any candidate can show: sample 2 vs sample 1
is Spearman 0.95 over all 100, 0.93 on the reference top 30. Owner
anchors, held out (no candidate saw them as anchors): the bugging story
should rank near the top; the 1Password/Codex productivity story is
promotional and low. The reference itself puts the bugging story **22nd**
of 100 (18th and 24th in the two samples) and the Codex story 87th: a
general-news editor puts the Ukraine power loss, the Nord Stream trial, the
Pope's surgery, US inflation and the Mauritius riots above it.

Metrics: Spearman over all 100 (ρ100), Spearman and Kendall τ-b over the
reference top 30 (the order among the stories that matter), overlap with
the reference top 10 and top 30, and each anchor's position (ties
averaged). Jev candidates are ordered by importance, promo capped at 2,
ties broken by the answer's expected value.

| candidate | ρ100 | ρ ref-top30 | τ ref-top30 | top10 ∩ | top30 ∩ | bugging | Codex | cost / 100 stories |
|---|---|---|---|---|---|---|---|---|
| reference sample 2 vs 1 (judge noise) | 0.95 | 0.93 | 0.79 | 7 | 27 | 18 | 83 | — |
| A. Jev isolated, current criteria | 0.81 | 0.70 | 0.52 | 8 | 22 | 36 | 71 | $0.0033 |
| B. Jev isolated, general editorial criteria | **0.90** | 0.67 | 0.49 | 8 | 25 | **11** | 81 | $0.0043 |
| C. Jev isolated, B + the day's 100 titles in state | 0.94 | 0.57 | 0.44 | 8 | 26 | 10 | 84 | $0.0123 |
| D. deepseek-v4.1-flash listwise, one call (rank) | 0.92 | 0.36 | 0.27 | 7 | 27 | 9 | 87 | 1 call, 7k in / 21k out, 146 s |
| D. same, rank mapped to 1-10 tiers | 0.88 | 0.35 | 0.28 | 6 | 27 | 12.5 | 82 | same |
| D, second sample (rank) | 0.83 | 0.61 | 0.46 | 6 | 26 | 5 | 92 | 7k in / 21k out, 146 s |
| D, second sample (tiers) | 0.89 | 0.63 | 0.52 | 6 | 26 | 4.5 | 82.5 | same |

D is noisy: its two samples differ more from each other on the top 30
(τ 0.27 vs 0.46) than B differs from the reference, and it never beats B
on top-10 overlap.

Front-page effect (frontend formula over the live rows' depth and age):
the bugging story moves from 23rd under A to **9th** under B; the Codex
story stays near the bottom (72nd → 77th). Under B the two broken-dek
stories are no longer promo (drones importance 6, Badenoch 1). B is
stable: a second run of all 100 gave identical importance for 92 and
identical promo for 99, and the bugging story 7 both times. The shipped
scorer prompt (B plus the section question, called through
`scoreStory` and `gateway.JevTransport`) gave: bugging 7, not promo,
tech_ai; Codex 2, promo; Ukraine power loss 8; Mauritius riots 8;
drones 1 and Badenoch 3, neither promo (their broken deks now score low
instead of being labelled promo).

Hypotheses:

- (a) *isolated absolute scoring is poorly calibrated; a listwise judge by
  a stronger model ranks better* — **not confirmed**. The pool's listwise
  judge (D, deepseek-v4.1-flash, reachable in-process with the gateway's
  own `provider.NewOpenCodeGoProviderFromEnv` and the static OpenCode key
  already in the unit's environment file) matched the whole-list
  correlation of B but ordered the top 30 much worse (τ 0.27 vs 0.49), and
  it read the broken-dek drones story as the day's second most important.
  It spent 21k reasoning tokens and 146 seconds per call (an 8k token
  budget returned no answer at all). The Decisions API has no list
  question type (`noul`, `choice`, `score` only;
  `internal/gateway/jev.go`), so a listwise Jev is not available; C, which
  gives Jev the whole list as context, is the nearest thing and did not beat
  B on the top 30 at three times the cost. gpt-6-luna (pool provider
  `chatgpt`) was not tried from the host: its OAuth refresh rewrites
  `/var/lib/go-choir/codex-auth.json`, which the unit cannot write and which
  would race the gateway's own token.
- (b) *title + dek is too little context* — plausible, not tested. The
  bugging story's dek is truncated before the facts that make it large;
  the reference, reading the same dek, also has it 22nd. Giving judges the
  article's first paragraphs needs the body on the internal front route;
  deferred.
- (c) *the criteria lack dimensions of seriousness* — **confirmed as the
  largest lever**. The current text weighs "consequence for many people,
  public interest, and novelty" only. B adds general editorial criteria:
  crime, espionage and security breaches, corruption, abuse of power,
  scandal or wrongdoing at a powerful institution, deaths and conflict; a
  charge, arrest, leak or investigation involving a powerful institution
  is serious even when few people are directly affected; a company's own
  announcement, case study or advocacy post is promotional however large
  the company; reporting on war, politics, crime or public affairs is not
  promo even when its dek is weak. No story, company or person is named.
  Caveat: the criteria were written after reading the owner's anchor, so
  the anchor's improvement (36th → 11th) is not independent evidence; the
  whole-list gains (ρ100 0.81 → 0.90, top-30 overlap 22 → 25) are.

## Decision

Ship B: the general editorial criteria in the existing Jev scorer, plus a
criteria version on each score row so rows scored under the old criteria
are rescored (within the existing 80-call bound per run, so the 100-story
list converges in two runs). Fallback is the scorer's existing behaviour:
a failed Jev call keeps the story's previous score, and a front-list
failure leaves the scores file untouched. The scores.json schema keeps
version 1; the new `criteria` field is additive and corpusd ignores it.
Cost: about $0.0043 per 100 stories scored, and only new or revised
stories are scored after convergence.

Not shipped: the listwise pool judge (D), which is slower, costlier, worse
on the top 30, and would add a host-side model provider path (red class).

## Residuals

- Broken deks (writer process text as the dek) mislead every judge; fix at
  the writer, not the ranker.
- The dek is cut at about 240 characters before the scorer sees it;
  hypothesis (b) needs the article's lead paragraph on the internal front
  route.
- The importance scale is absolute per story; days with many 7s still tie.
  Revisit a comparative judge once (b) gives judges more than a dek.
