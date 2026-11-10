# Jev, the vendor side: architecture, wire format, limits, pricing, variants

Subpage of the System 1 reference (to be folded into SYSTEM1.md by the
synthesiser). Siblings: [question-design.md](question-design.md),
[field-reports.md](field-reports.md). Fetched 2026-11-10 (today), all pages
this session. Our own usage is in [SYSTEM1.md](../SYSTEM1.md); this page is only what
is knowable from outside.

Tags: **[V]** vendor (TypeSafe docs/blog, or OpenRouter docs about its own
route), **[T]** third party (measured or reasoned by someone else), **[R]**
relayed or unverified (could not be traced to a fetched primary page).
Source keys in brackets, e.g. `[V models]`, resolve in the Sources section.
Method caveat: the fetch tool summarises pages with a small model, so figures
come from summaries, not raw HTML. Two PDFs (arXiv 2609.29429, a UCLA preprint)
were read as raw text locally: the most reliable numbers here.

## 1. What changed since the old JEV.md was written: a direct vendor API exists

The old JEV.md said "no direct TypeSafe endpoint in this repo". The vendor documents one:

| Item | Value | Tag |
|---|---|---|
| Direct endpoint | `POST https://api.typesafe.ai/v1/systemone`, bearer key | [V api] |
| Model list | `GET /v1/models` (aliases only; versioned IDs also accepted) | [V models] |
| OpenRouter, Decisions route (ours) | `POST https://openrouter.ai/api/alpha/decisions`, `typesafe/jev-1.14`, `~typesafe/jev-latest`, `typesafe/jev-1.13` | [V or-jev] |
| OpenRouter, vendor-shaped route | `POST https://openrouter.ai/api/v1/systemone`, accepts bare `jev-1.13` / `jev-latest` and maps them into the `typesafe/` namespace; same schema as Decisions | [V or-sysone] |
| OpenRouter Extractions | `POST /api/alpha/extractions` (typed fields) | [V or-jev] |
| Access | waitlist at launch (2026-09-15); one third party says the waitlist was removed 2026-09-21 with $5 credit (about 120M input tokens at list price) | [T orca-know] |
| Python SDK | `typesafe-sdk`, first public release v0.5.7 on 2026-09-14, v0.7.5 on 2026-10-14 | [V sdk-changelog] |
| JS SDK | exists (`TypeSafeClient`, classes per question type) | [V llms] |

Model naming differs by door: vendor `jev-1.14.0`, `jev-latest`, `jev-preview`;
OpenRouter `typesafe/jev-1.14`; served build in our receipts
`typesafe/jev-1.14-20261014` (M, SYSTEM1.md). OpenRouter's own example response
shows a dated build string `typesafe/jev-1.13-20260917` [V or-sysone].

## 2. What Jev is, as far as published

Vendor claims [V intro-blog] (all marked claims by TypeSafe, none verified by them):

- "System One" model: typed answers with probabilities, no free text. Described
  as a "new model architecture, a parallel sampler" plus a training method called
  RLCD (Reinforcement Learning for Calibrated Decisions). Outputs are "produced
  in a single query rather than token by token". No paper, no weights, no
  architecture disclosure; the FAQ questions "why a new training algorithm",
  "where does data come from", "is it just a smaller LLM" are listed without answers.
- The docs say Jev is not fine-tuned per customer, is trained with RLCD, and the
  same weights serve every account; not trained on customer requests [V models].
- "Cannot hallucinate / no type errors" is a schema statement: the outputs are
  always valid members of the answer space. Every reviewer reads it as "can
  still return the wrong valid value" [T pearpages][T hn-via-search].
- CEO said "Noul" is short for Bernoulli [T simonw].
- Calibration is defined in aggregate: "a 0.8 should be right about 80% of the
  time across many answers", not per answer [V ml-primer][V or-whatis].

Reconstruction from outside (independent team, jev-1.13.0, September 2026,
revision 3 dated 2026-10-03; unaffiliated, Unlicense, 16,379 live requests
plus a 987-call architecture probe) [T jevresearch]:

| Finding | Value | Confidence tag in the report |
|---|---|---|
| Output is a probability read-out over caller options, not generated text | 704,277 published probabilities in 7,887 vectors (K=2 to 255), all on a 0.01 grid, zero off-grid | confident |
| Displayed sums | 0.99 or 1.00, never above 1.00; 46.4% of flat K=255 vectors sum 0.99 | confident |
| Choice `confidence` | equals chance-corrected top probability `(pmax-1/K)/(1-1/K)` exactly for 1,405 of 2,289 vectors, within one quantum for 822 more | confident |
| Score `confidence` | fits that formula poorly (100 of 121 beyond two quanta); Noul has none | measured |
| Latency model | upstream time = 73.1 ms + 6.05 ms per 1k input tokens, 859 to 28,859 tokens, R2 0.857; no visible quadratic term | measured (header-based) |
| Marginal cost of a question | about 55 input tokens, +0.44 ms; 1 to 192 questions moved upstream time 71 to 153 ms | measured |
| Marginal cost of an option | about 18 input tokens, +0.10 ms; 2 to 255 options moved 65 to 90 ms | measured |
| Concurrency 1 to 32 | median upstream 74 to 86 ms, flat | measured |
| Empty-state template | about 316 tokens | measured |
| Size band | active 4 to 9B plausible (4 to 14B dense-equivalent by capability) | inferred; MoE not excluded |
| Inert padding options | mean p(gold) 1.000 at K=2, 0.992 at K=255; padding mass 0.00; options scored near-absolutely, not by a set softmax | measured (200 items) |
| Nondeterminism | 30 byte-identical payloads gave 13 distinct responses; repeat TVD 0.034 to 0.122 on flat 254-option menus | measured |
| Posterior for "own small foundation model with read-out head" | 0.80; distillation contributed 0.35; wrapper over a frontier API 0.02 | the report's own judgement |

Vendor also says options beyond 255 use "a two-stage process" that
"occasionally slows" [V intro-blog, via summary] and OrcaRouter repeats it as
"larger option sets run slower because of a two-stage scoring process" [T
orca-lim]; but the API schema caps `choice` at 255 entries [V api], so the
claim is either historical or about internal handling. Unresolved.

Capability snapshot of 1.13 [T jevresearch]: MMLU-Pro 82.7% (n=12,032), GPQA
Diamond 76.5% (n=196), ARC-Challenge 97.9%, HLE multiple-choice subset 21.9%
with mean top probability 0.634 (overconfident). Recognition beats production:
MATH-500 as multiple choice 83.1% versus 13.6% when it must read digits. Weak
on 2025 events (1 of 12 correct), arithmetic, free-form generation.

## 3. Question types and the real wire format

Names, from the API reference [V api]: `noul`, `choice`, `score`, `box`.
There are exactly four; "noul" is the yes/no type and answers one number.

| Type | Request fields | Answer fields | Limits [V api] |
|---|---|---|---|
| `noul` | `instructions`; optional `criteria` = `{true, false}` descriptions | `noul` (P(yes), 0..1); **no `confidence`** | none stated |
| `choice` | `instructions`; `criteria` required: map name -> description | `choice` (argmax), `confidence` 0..1, `probabilities` | 1 to 255 entries; a choice with no description is read by name |
| `score` | `instructions`; `criteria` required: ordered array, position = level, **0-indexed** | `score` (probability-weighted mean, fractional), `confidence`, `legend` (`"0"`..), `probabilities` | 1 to 10 levels (the Score page says at least 2) |
| `box` | `instructions` (required); no `criteria`; header `TypeSafe-Danger-Zone-Images: true`; Jev 1.14 only | `box{top,bottom,left,right}` fractions 0..1, `confidence` (may be `null`) | OpenRouter: 400 on 1.13 |
| (OpenRouter only) `refusal` | n/a | an answer of type `refusal` for one question; other answers may still return | [V or-api] |

Shared: the question key is "your key for the answer ... not sent to the model"
[V primitives][T alignbench confirms]. `instructions`, `criteria` entries and
`true`/`false` may each be a string, object, array or null [V advanced]; JSON
structure is passed through rather than flattened to a template string. Nesting
limit 64 levels per `state`, `instructions`, `criteria` [V errors].

Direct request and response (vendor example, abridged to one question) [V api]:

```json
{"model": "jev-latest",
 "state": "I was charged twice for my subscription this month. Please refund the extra charge.",
 "questions": {"tone": {"type": "choice",
   "instructions": "What is the tone of this message?",
   "criteria": {"angry": "An upset or hostile message",
                "calm": "A neutral or polite message",
                "excited": "An enthusiastic or eager message"}}}}
```
```json
{"model": "jev-latest",
 "answers": {"tone": {"type": "choice", "choice": "calm", "confidence": 0.7,
   "probabilities": {"angry": 0.15, "calm": 0.8, "excited": 0.05}}},
 "usage": {"input_tokens": 120, "output_tokens": 12}}
```

(Confidence 0.7 for top probability 0.8 over three options matches the
chance-corrected formula.) Score answer in the same doc: `score: 1.7`, `confidence:
0.55`, `legend {"0":..,"1":..,"2":..}`, `probabilities {"0":0.1,"1":0.1,"2":0.8}`.
Note 0.1*1 + 0.8*2 = 1.7: the score is the expected level, not the argmax.

OpenRouter response adds `id`, `provider`, `usage.cost` (USD), and optional
`usage.cost_details`, `usage.input_tokens_details.cached_tokens`,
`usage.is_byok`, `annotations` [V or-api]. Request extras on OpenRouter:
`provider` routing prefs, `session_id` (max 256 chars), `trace`, `user` (max 256)
[V or-api]. Example cost on OpenRouter: 476 input tokens, cost 0.000019992
[V or-tutorial], consistent with $0.042 per million.

What `state` may contain [V state][V images]: a string, an object or an array
(an object recommended: named parts, related facts together; "the material you
would give a panel of experts"); do not put questions in the state. Images may
sit in `state`, in one question's `instructions`, or in `criteria` (example
pictures for options), via an `assets` map of placeholders; every asset must be
referenced or the request is a 400; audio and video are not accepted ("yet").
OpenRouter alone documents `input_audio` and `video_url` parts in `state`, but
adapter support depends on the provider [V or-api]; Jev's own docs say no.

Instruction versus criteria versus choices [V choice][V score][V noul]:
`instructions` ask the question about the state and point at it by path in
backticks; `criteria` define what each answer means (choice option text, score
level situations, noul true/false boundary); the `choices` array in our
gateway body is ours, the vendor has only the `criteria` map whose keys are the
choices. Score levels are judged one at a time; the model "sees only the
descriptions, not the numbers or neighboring levels" [V score].

## 4. Hard limits, one table

| Limit | Value | Tag |
|---|---|---|
| Context per request (state plus all questions) | 64k tokens (model card 65,536 per a reseller) | [V models][T orca-lim] |
| Context per question (state plus any one question) | 32k tokens: the binding constraint | [V models] |
| Questions per request | no stated limit; only the two budgets bind | [V models][V api] |
| Questions tested per call by others | up to 192 (latency 71 to 153 ms); mean 11.4, range 4 to 28 over 23,411 calls | [T jevresearch][T alignbench] |
| Choice options | 255 (one doc says "roughly 240" in a cookbook) | [V api][V cookbook-sic] |
| Score levels | 1 to 10 | [V api] |
| Long-context mode (direct) | header `TypeSafe-Danger-Zone-Long-Context: true` lifts both limits to 262k tokens; same price; "slower and less accurate"; works on 1.13 and 1.14 | [V models] |
| Long-context mode (OpenRouter) | "more than about 32,000 bytes of text" and every image request run in long-context mode, context 262,144 | [V or-jev] |
| Images | PNG or JPEG; 16 MiB a file; 280 to 1568 px each side; 78,400 to 2,458,624 pixels; out-of-range images rejected not resized; one bad image fails the request; billed as input tokens; header required; "dreamer-ready, not production-ready" | [V images] |
| Body | 32 MiB compressed and decompressed; `Content-Encoding` gzip or zstd (zstd window at most 8 MiB), one layer; reference keys 1,024 bytes each, 65,536 total | [V errors][V images] |
| Nesting | 64 levels | [V errors] |
| Rate limits | per organisation, tokens/s and requests/s, in the developer console, 429 when exceeded; **no numbers on the docs page**; increases via a request form | [V models] |
| Rate limit figures quoted elsewhere | 20 rps (1,200 rpm) and 250,000 tokens/s [T opentweet, read 2026-09-18, cites TypeSafe]; 40 rps and 100K tokens/s [T orca-lim, calls them vendor-published]; "80" in our notes has no source found | **R, contradictory, do not build on** |
| Timeouts | SDK default 10 s per HTTP operation, `RetryPolicy.timeout` 30 s total per call; server timeout unpublished | [V constants][V retries] |
| Throughput claim in the open | an independent run: concurrency 1 to 32 with flat upstream time | [T jevresearch] |

Tension to resolve before trusting the 32,000-byte figure: the vendor states
limits in tokens (64k/32k) and puts long-context behind a header; OpenRouter
states a byte threshold with no header. Whether OpenRouter flips the header
itself above 32,000 bytes, whether the bytes count questions, and whether the
accuracy loss applies on the route are unpublished. The independent latency fit
shows no cliff up to 28,859 tokens on the direct API (about 115 KB of English),
so the threshold is not a latency cliff there. The accuracy cost of
long-context mode has no published number.

Tokenizer facts that decide byte budgets [T jevresearch]: Latin prose about
0.26 tokens per codepoint (roughly 4 characters a token), but digits 0.94,
Cyrillic 0.94, common CJK 0.92, Hangul 0.92, emoji 1.71, rare scripts up to 1
token per UTF-8 byte; whitespace is normalised (300 spaces cost the empty-state
template). A regression over 23,411 calls: input tokens = 412 + 0.224 x state
characters + 0.210 x question characters (R2 0.990) [T alignbench]. The
question text is billed and read in every call: in that study the question
battery was 59% of input tokens.

## 5. Probabilities, confidence, calibration

Summary only; the full treatment with numbers is in
[question-design.md](question-design.md) sections 3 and 4. Choice confidence is
the chance-corrected top probability [V confidence]; every probability is a
multiple of 0.01 and 66.9% of entries are exactly 0.00 [T meng]; calibration
differs per question form and task (ECE 0.020 to 0.091 on the same model) [T
meng]; byte-identical requests are not byte-identical in answer [T jevresearch].
The vendor's own repeat-call test: least-stable decision flips 4.0% on the
average post, 0 of 40 above 1 in 10, versus 8.6 to 19.9% for seven LLM
conditions [V cookbook-consist-c]. That is the vendor's test, not an audit.

## 6. Pricing and OpenRouter

| Item | Value | Tag |
|---|---|---|
| Price | $0.042 per million input tokens = $42 per billion; output free; same for 1.13 and 1.14; same price in long-context mode and for image tokens | [V models][V images] |
| Verified spend | 23,459 calls, 85.2M input tokens, $3.58 total (matches list) | [T jevresearch] |
| Repeated state caching | not documented; OpenRouter's response schema has an optional `input_tokens_details.cached_tokens`, and no Jev page states whether it is ever non-zero or discounted | [V or-api]; absence = open |
| Cookbook note | vendor cookbooks give each call a unique `uid` "so no cache returns an earlier answer", which suggests their own client cache, not a server cache | [V cookbook-consist-c] (inference) |
| Free-tier / credits | new direct accounts got $5 credit | [T orca-know] |
| OpenRouter price | listed $0.042 per million on 2026-10-16; the model page (`openrouter.ai/models/typesafe/jev-1.14`) returned 404 this session; no OpenRouter fee on a Decisions request is documented on any fetched page | [V or-whatis]; fee **unknown** |
| OpenRouter limits page | credit limits, in-flight budget (402 `in_flight_budget_exhausted`, retry after `Retry-After`; 402 `weight_exceeds_budget` do not retry), key budgets (403), free-model limits; **no Decisions-specific rate limit** | [V or-limits] |
| Worked costs | 3 questions, 476 input tokens: $0.000019992; 13 questions over a 53,777-character document, one call: $0.000497 versus $0.006090 in 13 calls | [V or-tutorial][V cookbook-parallel] |
| Reseller markups | one reseller passes through at 0% markup | [T orca-lim] |

## 7. Errors, retries, headers

Direct API [V errors]: 400 (bad UTF-8, nesting over 64, non-finite number,
malformed compression, or `max_tokens_exceeded` when over context), 401, 413
(over 32 MiB), 415, 422 (`{"detail":[{"loc","msg","type"}]}`), 429 ("back off
and retry after a short delay"), 529 overloaded. No response headers are
documented on the errors page. The SDK shows two: `x-typesafe-request-id`
(exposed as `request_id`) and `Retry-After` / `retry-after-ms` (honoured by
default) [V exceptions][V retries]. OpenRouter errors: 400, 401, 402, 403, 404,
413, 429, 500, 502, 503, 524 (edge timeout), 529, shape `{error{code,message,
metadata}, openrouter_metadata, user_id}`; `x-session-id` is the only
documented extra header [V or-api]. A 400 comes before anything is sent
upstream if the endpoint cannot serve the request (for example box on 1.13).

SDK retry defaults [V retries]: `max_retries` 2, backoff 0.5 s doubling to 5.0 s
with 25% jitter, retries 408, 429, all 5xx, connection errors and timeouts,
total budget 30 s. High-rate advice [V sdk-advanced]: reuse clients; zstd on
bodies of 1 KiB or more; HTTP/1.1 for high rates; 100 connections, 30 s
keep-alive. The failure rate seen by others: 0.12% of 23,411 calls [T
alignbench], 0.49% over 76.2M tokens on one reseller [T orca-lim], 19
contract-invalid responses in about 16k calls [T jevresearch].

## 8. Versions, aliases, deprecation

| Item | Value | Tag |
|---|---|---|
| 1.13.0 | launched 2026-09-15 (early access); "multimodal all along" per the 1.14 post | [V jev114-blog] |
| 1.14.0 | 2026-10-14; "small, subtle improvements to everything", biggest gain multilingual; adds `box` (1.14 only); images in state/instructions/criteria and long context also on 1.13; no accuracy numbers published | [V changelog][V jev114-blog] |
| Aliases | `jev-latest` and `jev-preview` both point at `jev-1.14.0` on 2026-11-10; "aliases can change when a new release ships"; send a versioned ID for stability | [V models] |
| Deprecation | models page: not mentioned. Blog (2026-10-14): `jev-1.13.0` stays "for a while"; a deprecation policy is promised but unreleased; summary of notice periods (60 days, 30 days) is ambiguous | [V models][V jev114-blog] |
| 1.13 vs 1.14 accuracy | one small Korean test found no clear gain and calibration moving in opposite directions on different sets (seen only through a search summary; the page returned 403) | [R wikidocs] |
| Roadmap | incremental releases (stability, small gains, deployment features); "larger projects ... unannounced"; more "machine-native models" after a $870M series A | [V jev114-blog][V series-a] |

## 9. On-device, self-hosted, open weights, variants

- **No official open weights, no official on-prem, no paper** [V or-whatis]
  ("Open source: No. proprietary, no published weights or paper"). The
  enterprise page lists "Can we run Jev in our own VPC or on-premises?" without
  showing the answer; it offers higher rate limits on request [V enterprise].
  Zero data retention is offered to enterprise; a DPA exists [V legal].
- A listing claims a 0.6B "NanoJev" enables self-hosting; no TypeSafe source
  supports it [R aggregator]. Treat as nonexistent.
- Official-adjacent: Browser Use "Jev Ultrafast" is an MIT-licensed browser agent
  that still calls the hosted Jev [R search-summary]; median task time 9.45 s
  down to 7.1 s over three runs, caveated by its sources.
- Open recreations (not TypeSafe's; none claims distillation from Jev): **Kev**
  by Jared Palmer, Qwen3.5 0.8B/4B/9B with a LoRA adapter and a pointer-style
  read-out head, about $95 of H100 time, "no Jev outputs used", hosted Jev 0.857
  versus Kev-9B 0.812 on its development set [T runtimewire, one secondary
  article]; at a 5% accepted-error limit Kev covers 45 to 57% versus Jev 70% on
  its dev split [T trilogy]. **Laya** (421M bidirectional encoder with a
  decision head), **AnyJev** (read-out levels on a causal LM), **OpenJev** and
  **DIY-Jev** (Rust server, Jev-compatible API over local GGUF models; Qwen3-4B
  65.0%, Qwen3 27B 75.3%, 32,235 examples) [T trilogy][T prismix-diy]. Details
  belong with the wider decision-model survey.
- Because the wire format is small and public, these servers expose a Jev-compatible API: a gateway speaking the vendor shape can swap backends.

## 10. Could not verify / open questions

1. The real rate limit for our OpenRouter key, and OpenRouter's own cap on the
   alpha route: unpublished (the only numbers are two contradictory third-party
   quotes and nothing for "80").
2. Whether the 32,000-byte long-context line counts questions, and what
   long-context mode costs in accuracy: no number anywhere.
3. Caching of repeated state and any cached-token price: undocumented.
4. OpenRouter's fee on Decisions requests and the model page (404 this session).
5. 1.14 accuracy and calibration against 1.13: only one unverifiable small test.
6. What "two-stage" scoring above 255 options means given the 255 cap; and whether
   the confidence quirks audited on 1.13.0 hold on 1.14.
7. Pages returning 403 and therefore unread: towardsdatascience.com (Jev vs
   LLMs), wikidocs.net (two Korean eval posts), pub.towardsai.net (the Jev-on-Ollama
   emulation post), fourweekmba.com.
8. HN launch thread, X posts, LessWrong/AF and SDK GitHub issues were not
   reachable or not found (searches returned secondary coverage only); several
   vendor cookbooks (Twitch, cart, function calling) were not read.

## Sources (fetched 2026-11-10 unless noted)

Vendor docs (docs.typesafe.ai): `api` /api/system-one.md; `errors` /api/errors.md; `models` /models.md; `llms` /llms.txt; `primitives` /primitives; `choice` `score` `noul` `advanced` /primitives/{choice,score,noul,advanced}.md; `state` /concepts/state.md; `images` /images.md; `confidence` /confidence.md; `changelog` /changelog.md; `ml-primer` /introduction/machine-learning-primer.md; `legal` /legal.md; cookbooks `cookbook-parallel` /cookbooks/parallel_questions.md, `cookbook-consist-c` /cookbooks/consistency_choice_cookbook.md, `cookbook-consist-n` /cookbooks/consistency_noul_cookbook.md, `cookbook-sic` /cookbooks/classification_using_confidence.md; SDK `sdk-changelog` /sdk/python/changelog.md, `constants` `retries` `exceptions` /sdk/python/api/{constants,retries,exceptions}.md, `sdk-advanced` /sdk/python/usage/advanced.md.
Vendor blog (typesafe.ai): `intro-blog` /blog/introducing-system-one-models-and-jev; `jev114-blog` /blog/jev-1-14; `series-a` /blog/series-ai; `enterprise` /enterprise.
OpenRouter (openrouter.ai): `or-jev` /docs/guides/community/jev.md; `or-api` /docs/api/api-reference/alphadecisions/submit-a-decisions-request.md; `or-sysone` /docs/api/api-reference/systemone/submit-a-system-one-request.md; `or-tutorial` /docs/guides/community/jev-tutorial.md; `or-whatis` /blog/insights/what-is-jev/; `or-limits` /docs/api/reference/limits; /models/typesafe/jev-1.14 returned 404.
Third party: `jevresearch` https://github.com/JevResearch/Jev-Research (+ ARCHITECTURE-ANALYSIS.md; summary https://www.remio.ai/post/jev-benchmark-finds-a-useful-decision-model-not-a-frontier-reasoner); `meng` https://escholarship.org/content/qt4t4449jv/qt4t4449jv.pdf and `alignbench` https://arxiv.org/pdf/2609.29429 (both read as raw PDF text); `opentweet` https://opentweet.io/jev/limits; `orca-lim` https://www.orcarouter.ai/blog/jev-limitations; `orca-know` https://www.orcarouter.ai/blog/jev-typesafe-system-one-what-we-know; `simonw` https://simonwillison.net/2026/Sep/21/jev/; `pearpages` https://pearpages.com/blog/2026/09/16/jev-sorted-what-typesafes-system-one-model-actually-is-and-what-is-still-just-a-claim; `trilogy` https://trilogyai.substack.com/p/jev-open-decision-models; `prismix-diy` https://prismix.dev/news/6c3dc6a5371c; `runtimewire` https://runtimewire.com/article/jared-palmer-kev-qwen35-decision-models.
Search summaries only (not fetched): `hn-via-search`, `wikidocs`, `aggregator`, `search-summary`.
