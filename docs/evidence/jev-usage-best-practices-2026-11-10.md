# Jev usage best practices: what the vendor says, and how this repo compares

Date: 2026-11-10. Mutation class: green (evidence only). All web pages below
were fetched on 2026-11-10 (the access date for every quote). Repo state:
`origin/main` at d71bb1751. No secrets, no production calls were made.

Method note: pages were read through a fetch tool that summarises the page with
a small model, so quotes are the short strings that tool returned; they were
not re-checked against raw HTML. Pages that failed to fetch are listed in
section 6. Nothing here comes from memory.

## 1. Plain-language summary

- The owner's framing is correct and is the vendor's own recommendation: put
  every question that shares one context into one request. TypeSafe documents
  it ("Send every question that uses the same state in one request") and
  publishes a benchmark where 13 questions in one call were 12.2x cheaper than
  13 calls.
- Cost is real but tiny in absolute terms here. Input is $0.042 per million
  tokens and output is free. Our whole Jev bill at about 12,000 calls a day is
  of the order of one dollar a day even if every call re-sent a 1,800 token
  context (an assumption, section 5). Batching is worth doing for request-rate
  headroom and tidiness, not for the money.
- Most of our calls are already "one context, all its questions": intake labels
  ask `event` and `topic` about one capture (2 questions, one state), and the
  scorer asks every question about one story. That is the documented pattern.
  What we do not do is share one context across many items. The documented
  saving ("nearer a full Nx the bigger the document") only applies when the
  context is large and genuinely shared. Our intake call re-sends the story
  candidate list per capture; that is the one place a shared context exists.
- The vendor says questions are independent, parallel and add little latency,
  and its benchmark found no accuracy change from batching. It also lists real
  accuracy hazards that matter if we pack unrelated items into one state:
  "Accuracy falls as the state grows with content unrelated to the decision,"
  and choice options are order-biased ("Jev leans toward the option that comes
  first"). No source gives a recommended question count, and none says accuracy
  varies with the number of questions. The vendor test is one document, one
  model, 5 runs.
- Vendor rate limits are per organisation in tokens per second and requests
  per second, with no numbers on the vendor pages we could fetch. Third parties
  quote two different sets of numbers. Whether OpenRouter applies its own
  limit to the alpha route is unknown.
- Start with what we do (one item, all its questions), keep the existing
  12-question cap, and measure the one change worth testing: packing several
  captures that share a stories list into one request, with an answer-agreement
  check against the single-capture call.

## 2. What Jev is in this codebase (step 1)

- Model and path: gateway pins `~typesafe/jev-latest` (`internal/gateway/jev.go:22`)
  and POSTs to OpenRouter's alpha Decisions endpoint
  `https://openrouter.ai/api/alpha/decisions` (`jev.go:23`). Served model in
  receipts: `typesafe/jev-1.14-20261014` (`docs/evidence/jev-measurement-2026-11-10.md` section 0).
- Vendor: TypeSafe (docs at docs.typesafe.ai). Jev is described there as
  TypeSafe's flagship "System One" model: it returns typed decisions with
  probabilities rather than generated text. OpenRouter is the reseller; there
  is no direct TypeSafe endpoint in this repo.
- Request shape: `{model, state, questions{name: {type: noul|choice|score,
  instructions, criteria}}}` (`jev.go:153-165`); the response has
  `answers{name: {choice, probabilities, confidence}}`, `model`, `provider`,
  `usage{input_tokens, output_tokens, cost}`. No logprobs: probabilities are
  the model's own output. Choice, noul and score are the only question types
  the repo uses; box (image) is not used.
- Caps in the repo: 12 questions per call (`internal/yaegikernel/system1.go:26`
  `System1MaxQuestions`), 1 MiB gateway body (`jev.go:24`).
- Pricing in `internal/llmcost/pricing.go:106-112`: `jev-1.13`, `jev-1.14`,
  `jev-latest` all `InputPerMillion: 0.042, OutputPerMillion: 0`, no cached
  rate listed. This matches the vendor and OpenRouter pages (section 3.2).

## 3. Findings by topic

Labels: **[V]** vendor documentation (docs.typesafe.ai), **[OR]** OpenRouter
documentation, **[3P]** third party, **[G]** general research not about Jev.

### 3.1 Many questions against one state

- [V] "Send every question that uses the same state in one request."
  https://docs.typesafe.ai/primitives (fetched 2026-11-10).
- [V] "Every answer is independent": one question's answer is not context for
  another, so adding or removing a question does not change the others'
  answers. A question that needs another's answer needs a second request.
  Same page.
- [V] "There is no limit on the number of questions in a request." The
  constraint is size: the request must fit the per-request and per-question
  context; if not, split across requests that share the state. Same page.
  Contrast: the API reference "does not state a maximum number of questions"
  ([OR] https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request.md)
  and the errors page "doesn't set limits for these" (https://docs.typesafe.ai/api/errors.md).
- [V] Fan-out pattern: "Send many questions in a single request, including
  speculative ones, and let your code decide what's relevant."
  https://docs.typesafe.ai/patterns/fan-out.md. Irrelevant answers still come
  back and must be ignored in code.
- [OR] "All questions in the request are answered in parallel and cannot see
  each other's answers." https://openrouter.ai/docs/guides/community/jev-tutorial.md
- [OR] "Independent questions belong in one request ... five questions cost
  one round trip." https://openrouter.ai/blog/tutorials/how-to-use-jev/
  Same post: keep judgments narrow and atomic.
- [OR] Answers return keyed by your question names: Jev "answers all in one
  pass, each answer returned under the key you gave."
  https://openrouter.ai/blog/insights/what-is-jev/
- [V] Delimiting: there is no delimiter; questions are keys of a JSON
  `questions` object and the shared context is the `state` object (a string,
  object or array; "Use an object for most requests"). https://docs.typesafe.ai/concepts/state.md
- [OR] A `refusal` answer can appear for one question while "other answers in
  the same request can still be present" (OpenRouter API reference above), so
  per-question handling is needed.

### 3.2 Cost

- [V] "You pay per input token. Output tokens are free." Jev 1.14 and 1.13
  are "$42 / $0.042" per billion / per million input tokens.
  https://docs.typesafe.ai/models.md. [OR] same: "Jev is billed based on the
  number of input tokens, and output tokens are free," each response carries
  `usage.cost`. https://openrouter.ai/docs/guides/community/jev-tutorial.md
- Billing consequence (derived, not a quoted claim): cost is input tokens of
  `state` plus all question text, so the shared state is billed once per call.
  N questions in one call pay the state once and each question's text once;
  N calls pay the state N times. [V] benchmark: 13 questions about a 53,777
  character article, 5 runs each: one call $0.000497 vs 13 calls $0.006090,
  "12.2x cheaper and 11.8x faster", batched means within 0.004 of single-call
  means, "Batching adds no meaningful noise." Caveats on the page: one model,
  one document, 5 runs, cached results, single-call time is the sum of 13
  sequential latencies (concurrent single calls shrink the time gap but "the
  12.2x cost gap stays"), and "The bigger the document, the nearer that saving
  comes to a full Nx."
  https://docs.typesafe.ai/cookbooks/parallel_questions.md
- [OR] Sample costs: a 3-question ticket call used 447 input tokens, about
  $0.000019 (https://openrouter.ai/blog/insights/what-is-jev/); 400
  classification items at about 610 input tokens each cost $0.010246
  (https://openrouter.ai/docs/cookbook/evaluate-and-optimize/jev-classification.md).
- [G] Batch prompting (Cheng et al., https://arxiv.org/abs/2301.08721):
  "the inference costs decrease almost inverse linearly with the number of
  samples in each batch" and "the number of samples in each batch and the
  complexity of tasks affect its performance". General, on Codex-era models,
  not Jev.

### 3.3 Accuracy

- No accuracy benchmark was found; OrcaRouter [3P] says TypeSafe's
  benchmark page "is still marked pending"
  (https://www.orcarouter.ai/blog/jev-limitations).
- Number of questions: nothing says accuracy changes with count. The only
  direct evidence is the vendor's 13-question benchmark above (no change).
- [V] Unrelated content hurts: "Accuracy falls as the state grows with
  content unrelated to the decision." Filter in code or use a noul to test
  relevance. https://docs.typesafe.ai/model-jaggedness.md. This is the main
  hazard of packing several unrelated items into one `state`.
- [V] Option order: "Jev leans toward the option that comes first." Reorder
  options and check answers stay consistent. Same page.
- [V] Question phrasing: avoid "Hiding several judgments inside one question";
  double negatives and inverted mappings (`true` meaning no) do worse; "Jev
  does not count reliably." Same page. A companion noul detects "no option
  fits" (https://docs.typesafe.ai/patterns/companion-noul.md, listed in the
  vendor index; page itself not fetched).
- [V] Long context: the defaults are 64k tokens per request and 32k per
  question; the header `TypeSafe-Danger-Zone-Long-Context: true` raises both
  to 262k, which is "slower and less accurate than staying within the default
  limits." https://docs.typesafe.ai/models.md. [OR] says OpenRouter enables
  long-context mode "when a request carries more than about 32,000 bytes of
  text or any image" (https://openrouter.ai/blog/insights/what-is-jev/).
  That byte figure is much lower than the vendor's token limit and bears on
  our 1 MiB body cap and 24 KiB event-layer state cap.
- [V] Probabilities and confidence: confidence "measures how concentrated the
  probabilities are, not whether the answer is right"; for a choice, the
  chosen option's probability is the correctness figure; noul has no
  confidence field, "about 0.7 for yes and 0.3 for no" as starting thresholds;
  thresholds depend on "your domain and the performance of the model" so "test
  with your own data". https://docs.typesafe.ai/confidence.md (paraphrase by
  the fetch tool except where quoted).
- [OR] Probabilities "shift slightly between runs" (a noul read 0.42 then
  0.44), so use bands, not exact values; index `probabilities` by name, key
  order varies; thresholds must be retuned per model version, and
  `typesafe/jev-1.13` can be pinned "when you need thresholds tuned against
  one specific version" (tutorial and blog above). The gateway deliberately
  uses `~typesafe/jev-latest`, so thresholds can shift on a model bump.
- [OR] Cookbook calibration sample (author-built, not a benchmark): items with
  confidence at or above 0.8 matched the human label 114 of 122, 0.5-0.8 9 of
  18, below 0.5 4 of 10. https://openrouter.ai/docs/cookbook/evaluate-and-optimize/jev-classification.md
- [3P] OrcaRouter quotes a TypeSafe page: a question and its negation can sum
  to 1.19, and score levels are "weak in numerical calibration"
  (https://www.orcarouter.ai/blog/jev-limitations). Second-hand; the
  vendor jaggedness page confirms weak score calibration.
- [G] Multi-problem prompts, general: "Evaluating LLMs with Multiple Problems
  at once" (https://arxiv.org/html/2406.10786v3) reports comparable accuracy to
  one-by-one for many classification tasks with conditions where it falls short
  (second-hand from a search snippet, full text not read). Batch prompting
  (above) says batch size and task complexity affect performance. A 2026 paper
  on multi-instance processing (https://www.alphaxiv.org/abs/2603.22608.md,
  snippet only) says only a small number of instances can be processed reliably
  before accuracy degrades. All general LLM work, not Jev.

### 3.4 Latency, rate limits, concurrency

- [V] "All questions are evaluated in parallel, so adding more questions
  usually has little effect on response time." https://docs.typesafe.ai/patterns/fan-out.md
  "Usually", not guaranteed. No vendor latency figures were found.
- [V] Rate limits are "per organization" in tokens per second and requests per
  second; over either gives 429; the client SDKs "retry these requests by
  default and follow the `retry-after` header." https://docs.typesafe.ai/models.md.
  The page gives no numbers. 429 text: "Back off and retry after a short delay";
  529 is "TypeSafe is temporarily overloaded" (https://docs.typesafe.ai/api/errors.md).
- [3P] Numbers quoted by third parties disagree. OpenTweet: 250,000 input
  tokens/s and 1,200 requests/min (about 20/s), citing the vendor models page
  as read 18 September 2026, "adjusted dynamically and may change without
  notice" (https://opentweet.io/jev/limits). OrcaRouter: 100K tokens/s and 40
  requests/s, citing docs.typesafe.ai/models.md (https://www.orcarouter.ai/blog/jev-limitations).
  The current vendor page shows no numbers to our fetch. The repo's own
  evidence doc marks the 20 rps / 250k tps figures "unverified"
  (`docs/evidence/jev-measurement-2026-11-10.md` section 4). Treat all as
  unknown for our key.
- [3P] OrcaRouter measured serving latency: median 151 ms, p95 247 ms over 7
  days to 2026-09-30, one day p95 2,448 ms; vendor quote "For higher
  cardinality choices, we do a 2 stage-system of scoring independently."
  Our own data: median 109 ms for a serial 3-question call
  (`docs/evidence/jev-measurement-2026-11-10.md` section 2).
- [OR] Concurrency guidance is bounded workers with retry: the cookbook ran 8
  workers, 400 requests in 8.6 s, retrying 429/5xx with backoff from 1 s to
  30 s and honouring a numeric `Retry-After`. It is one request per item, with
  all of an item's tags and category in that request.
  https://openrouter.ai/docs/cookbook/evaluate-and-optimize/jev-classification.md
  No source says "prefer few large calls over many small parallel calls" for
  latency. The vendor's stated reasons for fan-out are cost and speed relative
  to sequential calls and avoided round trips.
- [3P, author arithmetic, "not a measurement"] OpenTweet computes a crossover
  at about 12,500 input tokens per request: below it the request-per-second cap
  binds, above it the token cap does. https://opentweet.io/jev/limits. A Towards
  AI article titled "Asking Jev 13 questions one call at a time caps you at 92
  docs a minute, one request gets 1,200" exists (fetch returned 403, so its
  content was not read; do not rely on it).

### 3.5 Caching

- Nothing. Neither the vendor models page, the OpenRouter docs nor the Jev
  guides mention prompt or context caching. The OpenRouter response schema
  carries `usage.input_tokens_details.cached_tokens`
  (https://openrouter.ai/docs/api/api-reference/alphadecisions/submit-a-decisions-request.md),
  and `pricing.go:106` records no cached rate. Unknown whether any caching
  applies; we cannot assume a discount for repeating the same state in
  separate calls. The vendor's own message is that sharing the state in one
  call is the way to avoid paying for it twice.

### 3.6 Failure modes and pitfalls reported

Vendor unless marked: not a calculator, not a date model, not a counter
(https://docs.typesafe.ai/model-jaggedness.md); adversarial text in state "can
move the answer"; choice always picks a listed option, so add an `other`/`none`
choice [OR cookbook]; missing `confidence` or `usage.cost` should be treated as
a failure, not free [OR cookbook]; probabilities vary run to run [OR]; image
and box features are "early" and may change [OR blog]; the endpoint is alpha
and "the shape may change" [OR API reference]. Third-party opinion on
limitations is thin and mostly derivative of the vendor docs.

## 4. Vendor-stated vs third-party vs unknown

| Question | Status |
|---|---|
| Put all questions on one state in one request | Vendor-stated (and OpenRouter) |
| Questions independent, parallel, little latency impact | Vendor-stated ("usually") |
| 12.2x cheaper / 11.8x faster for 13 questions, no answer change | Vendor-stated benchmark, 1 doc, 1 model, 5 runs |
| Max number of questions | Vendor says no limit; only size limits. No recommended count |
| Accuracy vs question count, question position | Not documented anywhere found |
| Unrelated content in state lowers accuracy; option order bias | Vendor-stated |
| Per-org rate limit numbers | Not on fetched vendor page; third parties disagree; OpenRouter route limit unknown |
| Latency vs question count | Vendor: little effect. No numbers. Our median: 109 ms (3 questions) |
| Caching of shared state across calls | Unknown; undocumented |
| Best-practice posts specific to batch size | None found; the model is new (1.14 dated 2026-10-14) and public material is the vendor docs, OpenRouter guides and thin third-party summaries |

## 5. Comparison with this repo

Tokens are not logged per site in what we read, so context sizes below are
assumptions to be replaced by `usage.input_tokens` from the receipts
(`/var/lib/go-choir/gateway/jev-transport-receipts.jsonl` records it). All
cost figures use $0.042 per million input tokens.

| Best practice | Repo status | Where |
|---|---|---|
| All questions on one state in one call | Followed for the intake label (event + topic, one capture); 5,768 of about 8,000 calls today carry 2 questions | `internal/agentcore/intake_labels.go:405-455` |
| Same | Followed by the scorer: every question of a story in one call, up to 12 | `internal/frontimportance/scorer.go:249` (`maxQuestionsPerCall = 12`), `:686` |
| Same | Followed by the event layer: up to 12 captures per call | `internal/agentcore/event_layer.go:48` (`eventChunkSize = 12`) |
| Same | Followed by the sub-call, desk-cast and inert judges: up to 20 children per call | `internal/agentcore/subcall_judge.go:115`, `desk_cast_judge.go:61`, `inert_judge.go:156-159` |
| Share one large context across many items | Not followed at intake: the stories list is re-sent with each capture; calls are parallel at `intakeLabelConcurrency = 12` | `intake_labels.go:55`, `:405-412` |
| Stay under size limits | Followed: 12-question cap, 24 KiB state cap in the event layer; but the OpenRouter long-context trigger is about 32,000 bytes, so a state above that runs "slower and less accurate" | `system1.go:26`, `event_layer.go` |
| Bounded concurrency with 429 backoff | Partly: per-site pools (intake 12, scorer 12, support 8) and a gateway limiter; sub-call judge serial with 2 s spacing and 429 backoff | `intake_labels.go:55`, `scorer.go:177`, `support_judge.go:60`, `subcall_judge.go:104` |
| Handle `refusal`, missing `confidence`/`usage.cost` | Not verified in this pass | n/a |
| Pin the model version for stable thresholds | Not followed on purpose: the gateway pins the `~latest` alias | `jev.go:22` |
| Companion noul / "none" option | Followed at intake (`new` and `none` choices) | `intake_labels.go:416-444` |

Representative cost, intake labels (assumed sizes, not measured): stories block
about 1,500 tokens, capture plus task plus question text about 300 tokens.

- Today, one capture per call: about 1,800 input tokens per call, so $0.0000756.
  At about 12,000 intake-shaped calls a day (measured fleet rate was 440-579
  calls an hour on 2026-11-09/10) that is 21.6M tokens, about $0.91 a day.
- Shared stories block, 12 captures per call: (1,500 + 12 x 300) / 12 = 425
  tokens per capture, $0.0000179, about $0.21 a day. Saving about 76 percent,
  about $0.70 a day, about $255 a year. That is the upper bound of the money at
  stake; the 12.2x vendor figure assumed one very large document.
- Scorer (one story per call, all its questions together): already the
  documented pattern; no state is shared across stories, so no further saving.
  Splitting it into one call per question would multiply the story tokens by
  the question count; the repo correctly does not.
- Request count: moving intake from 12,000 to 1,000 requests a day cuts load
  on the per-organisation request-per-second limit, whose real value we do not
  know. This is a better reason to batch than the dollars.

Rate-limit arithmetic (third-party numbers, unverified): the paper runs about
8-9 calls a minute, far below the 20 or 40 requests a second quoted. The
measured fleet peak was 11 completions in a second, so a 20 rps cap is within
a factor of two at peak.

## 6. Not found / could not fetch

- Fetch failures: https://openrouter.ai/models/typesafe/jev-1.14 (404, so the
  OpenRouter model page and its pricing were not read directly; pricing was
  confirmed from the vendor models page and the OpenRouter tutorial pages
  instead); https://openrouter.ai/docs/guides/community/jev-tutorial (404; the
  `.md` variant worked); the Towards AI article (403).
- Not fetched: vendor `changelog.md`, `patterns/companion-noul.md`, the
  individual `primitives/*.md` pages, and the Python SDK advanced
  "performance tuning" page. Not read: arXiv full texts for the general
  batching claims (abstract or snippet level only).
- Not found anywhere: a recommended question count; any statement that
  accuracy depends on the number of questions or on question position;
  OpenRouter-specific rate limits for the alpha Decisions route; vendor
  caching behaviour or cached-token price; latency versus question count; vendor
  rate-limit numbers on the current page; Jev-specific best-practice posts
  beyond the vendor and OpenRouter guides.

## 7. Recommendations

1. Keep every question about one item in one call (already done). Do not split
   the questions of a single item into separate calls.
2. Starting size for multi-item batching: 4 items per call for intake, then 8,
   then 12, never above the existing 12-question cap, and only where the items
   share a large state (the stories list). The sources set no number, so this
   is a measure-then-raise start, not a documented value.
3. Because the vendor warns about unrelated content and first-option bias:
   keep each question's `instructions` pointing at its own sub-object (as the
   repo already does with `state.stories.cN`), and rotate choice order in the
   shadow test.
4. Measure before changing production behaviour. Shadow run: for 500 captures,
   ask the single-capture call and the batched call and compare (a) the chosen
   `event` and `topic` agreement rate, (b) mean absolute difference of the
   chosen option's probability (the vendor test saw at most 0.015 movement),
   (c) `usage.input_tokens` per capture, (d) latency p50/p95, (e) 429 count.
   Record single-call repeat-run noise first, so batching noise is compared
   against it.
5. Log `usage.input_tokens` per site (the receipts already hold it) so the
   cost table above is measured, and log provider response headers
   (`x-ratelimit-*`, `Retry-After`) to learn the real rate limit for our key.
6. Watch the 32,000 byte OpenRouter long-context trigger: cap batched state
   below it.
7. Retune thresholds after every `~typesafe/jev-latest` model bump, using the
   served model recorded in each receipt.
