# System 1 (Jev) as it runs here

Measurement page, a subpage of [JEV.md](../JEV.md) (what Jev is, price, rules). This page is what our own
system does with it. Code lines are at `origin/main` dc637a84f (2026-11-10). Marks: **M** measured by us
(window and method given), **C** read from code, **U** unexplained (stated; no cause invented).

Windows:

- **J**: gateway journal `jev call` lines, 2026-11-10 04:08:12Z to 21:45:53Z (17.6 h, 16,133 calls). The
  line first existed with the limiter deploy (dd9fba199); there is no latency or queue figure for any
  earlier time.
- **R**: gateway receipts, numeric fields only (`created_at`, computer prefix, answer count,
  `usage.input_tokens/output_tokens/cost`): the last 17,500, 2026-11-10 00:09Z to 21:47Z, one bounded
  `tail | jq` on Node B. 16,137 fall in J's window against 16,107 successful `jev call` lines; the 30
  extra receipts are not explained (edge effects at both ends, or calls the journal logged as failed). No
  state, question or answer text was read.
- **H**: `host judgment ... served/failed` lines from 2026-11-09 12:08Z (counts).

Nothing here touched a guest or issued a call. J is one day (a Tuesday), so "time of day" is one sample.

## 1. Latency anatomy (J)

`upstream_ms` is the gateway's time in the OpenRouter call; `queue_ms` is time in our limiter first
(`logJevCall`, `jev_limiter.go:343`).

Status: 16,107 `status=200`; 26 `status=0 failed`; no 429, no 5xx. All 26 failures are paper bulk
2-question calls with `queue_ms` 4,993 to 5,013 and `upstream_ms=0`: consistent with the guest's 5 s
intake call timeout (`intakeLabelCallTimeout`, `intake_labels.go:57`) expiring while the call waited in
our own queue (the gateway logs `context canceled`). Three waves: 18 at 07:15:27-38Z, 1 at 08:16Z, 7 at
14:18:14-17Z. Zero upstream errors. H shows 0 host-door failures.

### 1.1 Upstream latency overall and by question count (status 200)

Overall (n = 16,107): p50 146, p90 213, p99 1,018, p99.9 1,720, max 3,123, mean 168 ms. The journal's
three own hour summaries say p50 137 to 163.

| questions/call | calls | questions | p50 | p90 | p99 | max | mean | ms/question (sum/sum) |
|---|---|---|---|---|---|---|---|---|
| 1 | 1,520 | 1,520 | 94 | 157 | 481 | 2,007 | 116 | 116.0 |
| 2 | 9,859 | 19,718 | 155 | 219 | 1,112 | 3,123 | 183 | 91.6 |
| 3-4 | 1,504 | 5,274 | 131 | 198 | 586 | 1,552 | 148 | 42.2 |
| 5-8 | 1,625 | 11,413 | 154 | 217 | 597 | 1,578 | 166 | 23.6 |
| 9-12 | 1,518 | 17,517 | 110 | 177 | 1,000 | 1,896 | 142 | 12.3 |
| 13+ | 81 | 1,408 | 135 | 211 | 510 | 510 | 154 | 8.9 |

Method: `scripts/proofs/jev-call-stats.sh` (percentile element `int(p*n)`).

**Does latency grow with question count? Not in 1 to 38.** p50 stays between 94 and 155 ms in every bin
and the 9-12 bin is faster than the 2-question bin. The bins order by something else: input size. R gives
median input tokens by shape (paper, from 04:08Z):

| shape | questions | median input tok | median output tok | J p50 / p90 ms |
|---|---|---|---|---|
| intake, 1 capture, before 14:55Z | 2 | 12,668 | 1,607 | 158 / 224 |
| intake, 1 capture, after 15:10Z | 2 | 7,910 | 1,562 | 146 / 208 |
| intake, 2 / 3 / 4 captures (after) | 4 / 6 / 8 | 9,851 / 11,828 / 13,700 | 3,219 / 4,941 / 6,587 | 156/215, 162/242, 173/223 |
| event layer; front scorer | 12 | about 3,060; 2,214 | 599; 539 | 99-124; 111 |
| single question | 1 | 687 to 705 | 32 | 86 to 108 |

Readings: with similar input (12.7k vs 13.7k tokens), 2 and 8 questions take 155 and 173 ms, about 15 ms
more for 4x the questions; a 20-fold input change (700 to 12.7k tokens) moves p50 by about 60 ms. Rough
model: **a floor of 90 to 110 ms plus a few ms per thousand input tokens**. The model emits about 800
output tokens per intake question (6,587 / 8), so output is real work even though it is free in price
(section 3). R and J share no id, so these are bin-level pairings, **not a regression**: per-call tokens
against latency is not measured.

Per question: an 8-question call at p50 173 ms is 22 ms a question against 94 to 155 ms alone. Questions
per second per call in flight (1 / mean ms-per-question): about 11 at 2 per call, 42 at 5-8, 81 at 9-12.
This is the empirical basis for "many questions in one call".

### 1.2 Queue (class is in the log line)

| class | calls | queued (>0 ms) | p99 | max |
|---|---|---|---|---|
| bulk | 15,927 | 1,566 | 2,206 | 6,176 ms |
| interactive | 204 | 3 | 0 | 1,866 ms |
| oped_gate | 2 | 0 | 0 | 0 |

- Before 15:00Z: 10,007 bulk calls, 1,566 queued (15.7%), mean 131 ms, p99 2,664, max 6,176. After 15:10Z:
  5,833 bulk calls, **none queued**. The last queued call of the day is 14:41:29Z. Limiter constants were
  not edited in between (15/s burst 30, 10/s per computer, `jev_limiter.go:23-60`).
- 1,482 of the queued are paper 2-question calls, 67 front importance. 337 of 1,569 queued calls (incl.
  interactive) fell in seconds with fewer than 10 calls, so a one-second burst count alone does not
  explain the queue. **U**
- What ended the queueing cannot be separated: a gateway restart (14:55:19Z) and the paper taking
  c53cd09cf (first 8-question call 15:09:35Z) both fall after the last queued call and before the first
  quiet hour. **U**
- Interactive: 202 of 204 are one question; p50 87, p90 148, p99 264 ms. Three were queued: 8 ms
  (07:15:27Z), **1,866 ms (08:16:07Z)**, 344 ms (14:18:15Z), each next to a bulk cancel failure. A prompt
  route (8 s budget) held 1.9 s behind our own limiter is the one user-facing delay we caused; why the
  interactive reserve did not prevent it is **U**.
- Gateway process restarts: 27 processes inside J; 55 `Started` lines in H's 31.7 h (one per 35 min).
  Limiter state and the `jev hour summary` counters are per process, so only 3 hour summaries printed in
  J's 17.6 h.

### 1.3 Time of day (UTC hour; 04 and 21 partial)

Calls (questions) per hour, 04Z to 21Z: 336 (2,021), 896 (2,169), 597 (1,875), 504 (1,544), 1,053 (3,296),
844 (2,495), 1,255 (3,474), 1,485 (4,615), 1,082 (2,775), 916 (2,915), 1,191 (3,624), 1,113 (4,942), 1,126
(5,290), 659 (2,978), 935 (3,958), 831 (3,492), 913 (3,768), 397 (1,671). Hourly upstream p50 is 106 to
172 ms (all but hour 04 within 133-172); hourly p90 is 169 to 366, and only hour 14 exceeds 249 (366).
Calls over 1 s by hour: 05: 11, 06: 9, 08: 2, 11: 15, 12: 5, 13: 5, **14: 78**, 15: 14, 16: 6, 17: 1, 18:
9, 19: 10, 20: 4, 21: 1. No daily rhythm is visible in one day: the load is the paper's polling and what
it ingested, not the clock.

### 1.4 Tail events (status 200, upstream over 1,000 ms)

170 of 16,107 (1.06%): 127 two-question (paper intake), 13 twelve-question (10 front, 3 paper), 10
one-question, 20 other shapes; none interactive. Clustered: 111 of 170 are within 2 s of another tail.
They are not concentrated in the busiest seconds: 38 of 170 fall in seconds with 10 or more calls, which
carry 41.5% of all calls. Hour 14Z holds 78: 48 between 14:10 and 14:20Z (46 two-question) within 644
calls and up to 36 in one second, about 22 minutes after a restart (13:48:43Z) and just before the 14:18
cancel wave. Only 4 tails occur within 120 s after any restart. Cause of the 14Z wave: **U** (upstream
tail, our connection pool and load are not separable from the log).

## 2. Volume and shape

### 2.1 Totals (J)

16,133 calls, 56,934 questions (3.5 per call); 914 calls and 3,224 questions an hour on average. By
computer label: paper vm-1669020a 14,889 (92.3%); `host:front-importance` 992; vm-70070cf7 245; one
candidate-fleet computer 7. Class: bulk 15,927, interactive 204, op-ed gate 2.

Questions per call: 1: 1,520; 2: 9,885; 3: 742; 4: 762; 5: 204; 6: 419; 7: 137; 8: 865; 9: 110; 10: 154;
11: 61; 12: 1,193; 13 to 38: 81 (13: 15, 14: 14, 15: 8, 16: 12, 17 to 19: 16, 20 to 38: 16). J is a single
day, so this is the per-day histogram. Receipts before J (R, 00:09-04:08Z, 1,363 calls): q=1 257, q=2 564
(41%), q=3 284, q=12 48.

### 2.2 Before J, from R

Hours 01-03Z: 433, 325, 258 calls; 1.79M, 1.43M, 1.02M input tokens; mean 4,322 tokens per call, 342 calls
and 1.48M tokens an hour over 00:09-04:08Z. J's first full hour (05Z): 896 calls, 9.06M tokens (10.1k per
call). So **calls per hour about 2.6x, tokens per call about 2.3x, input tokens per hour about 6x**. Three
commits were deployed in that window (section 2.5); which one did what is **U**.

### 2.3 Paper shape by hour (J, calls by questions)

| hour | calls | 1 q | 2 q | 3 q | 4 q | 6 q | 8 q | 12 q |
|---|---|---|---|---|---|---|---|---|
| 05 | 874 | 89 | 708 | 38 | 6 | 3 | 1 | 5 |
| 15 | 1,025 | 87 | 499 | 59 | 108 | 71 | 126 | 14 |
| 17 | 616 | 36 | 271 | 40 | 94 | 22 | 131 | 5 |
| 20 | 852 | 120 | 375 | 50 | 88 | 52 | 129 | 13 |

The batched intake shapes (4, 6, 8 questions = 2, 3, 4 captures) appear at hour 15Z. Before 14:55Z
2-question calls are 78% of paper calls (7,272 of about 9,350; 675 an hour); after 15:10Z 45% (2,350 of
about 5,240; 375 an hour). Paper calls an hour barely moved (867 to 836); questions an hour rose (2,166 to
3,163); **input tokens an hour fell** (section 3.3).

### 2.4 Concurrency and bursts (same log second)

Calls in a log second: 1 call in 4,475 seconds; 2: 304; 3: 86; 4: 124; 5 to 9: 575; 10 to 19: 330; 20 or
more: 100 (max 39). Seconds with 5 or more calls: 1,005, carrying 10,296 of 16,133 calls (63.8%). The
traffic is **bursty and concurrent**. Maximum in one second: paper 39, front importance 28, vm-70070cf7
21.

- Paper: intake chunks in parallel (`intakeLabelConcurrency = 2`, `intake_labels.go:79`), event-layer
  prefetch, support judge 8 at once (`support_judge.go:60`), sub-call judge 4.
- Front importance: a timer every 10 min (`nix/node-b.nix:1249`), 12 stories at once (`scorerConcurrency =
  12`, `scorer.go:177`). J: 91 runs, 992 calls, most runs done within 0 to 2 s. JEV.md and the 2026-11-10
  evidence page call the scorer serial; at origin/main it is not (C, plus 28 calls in a second M).
- Latency by burst size (p50 / p90 ms): alone 106/173; 2 calls 127/244; 3-4 157/258; 5-8 158/222; 9+
  157/217 (n = 7,167). A burst costs about 50 ms at the median and does not grow from 5 to 39; the "alone"
  calls are mostly small 1-question calls, so this is partly a size effect.

After 15:10Z the maximum was 28 calls a second (before: 39); seconds with 10 or more calls: 99 of 2,549
(3.9%) against 330 of 3,394 (9.7%).

### 2.5 Deploys

| commit | time (-05:00) | what | visible in |
|---|---|---|---|
| dd9fba199 | 11-09 22:49 | shared limiter; latency in the `jev call` line | first line 04:08:12Z, after the 04:08:08Z gateway restart |
| ecefb752a | 11-09 22:49 | every site issues its judgments concurrently | seconds with 10+ calls (330 before 14:55Z) |
| 1e5fd14a3, ee2082f14 | 11-09 23:05 | intake windows of 8 h of stories, in chunks | R: 4.3k to 10.1k tokens per call |
| 3c1cf9c9c | 11-09 22:50 | scorer asks section yes/no questions, 12 per call | front calls are 12 (893) or 10 (94) questions |
| c53cd09cf | 11-10 09:49 | one intake call per chunk, up to 4 captures | 8-question calls from 15:09:35Z |

The gateway restarted 27 times inside J; the paper guest refreshes on its own schedule, so a gateway
restart time is not a paper deploy time.

## 3. Cost model

**Tokens are not in the `jev call` line.** It carries computer, class, question count, queue ms, upstream
ms and status. Tokens and dollars come from R (receipts exist for every success).

### 3.1 Billing checked against R

- Receipt cost / input tokens is 0.042 per million at p1, p50 and p99: **input tokens only, output billed
  at 0**, as JEV.md section 6 says.
- J window: 130.8M input and 21.6M output tokens in 16,137 calls: **$5.49 in 17.7 h = $0.311/h =
  $7.47/day** at $0.042 per M input. JEV.md's "order of $1 a day" is 7.5x lower (not edited here).
- By computer: paper $5.37, front importance $0.09, others $0.03. Paper 2-question calls alone are $4.20
  of $5.49.

### 3.2 Request bytes (C)

Bounds: intake 30,000 bytes (`intake_labels.go:74`), event layer 24 KiB (`event_layer.go:75`), gateway 1
MiB. Bytes per token is **not logged** (the receipt holds a request digest, not a size). Observed input:
max 17,481 tokens, p99 15,766; 8,651 of 16,137 calls (54%) over 8,000 tokens. Whether a call crosses the
vendor's 32,000-byte long-context line cannot be told from tokens. **U**

### 3.3 Rate by window

| window (UTC) | hours | calls/h | questions/h | input tok/h | $/h | $/day |
|---|---|---|---|---|---|---|
| 00:09-04:08 (R, before the 04:05Z deploys) | 3.98 | 342 | n/a | 1.48M | 0.062 | 1.49 |
| 04:08-14:55 (before batching) | 10.63 | 952 | 2,885 | 8.47M | 0.356 | 8.54 |
| 15:10-21:47 (after) | 6.62 | 894 | 3,912 | 6.05M | 0.254 | 6.10 |
| J window | 17.66 | 914 | 3,224 | 7.41M | 0.311 | 7.47 |

Assumptions: the JEV.md price holds; R is complete for its span; no caching discount (the cost ratio is
constant, so none is visible).

### 3.4 What one call with many questions saves (intake, measured)

Intake asks 2 questions per capture. Before c53cd09cf each capture was its own call and re-sent the
candidate list (about 80 stories); after, up to 4 captures share one list.

Arithmetic (paper, after 15:10Z, batched calls only): 610 four-question, 319 six-question and 785
eight-question calls = 1,714 calls covering 610 x 2 + 319 x 3 + 785 x 4 = 5,317 captures. Actual input:
17.74M tokens ($0.745). Billed per capture at the pre-batching 2-question median (12,668 tokens): 5,317 x
12,668 = 67.4M tokens ($2.83). Saved 49.6M tokens (73.7%), $2.08 in 6.62 h = $0.31/h = **$7.55/day**. Per
call at medians: 4-q 9,851 against 2 x 12,668 (61% saved), 6-q 11,828 against 3 x (69%), 8-q 13,700
against 4 x (73%).

Conservative baseline, the post-deploy single-capture median (7,910 tokens, why it shrank is **U**): 5,317
x 7,910 = 42.1M ($1.77); saved 24.3M (57.8%), $1.02 in 6.62 h = $0.15/h = **$3.70/day**. Truth lies
between: **58% to 74% of billed input tokens on batched intake, $3.70 to $7.55 a day.** 2,451 2-question
calls after 15:10Z remain (17.7M tokens, $0.74, 45% of the paper's tokens then); how many are
single-capture polls that cannot batch is **U**.

Latency effect: 22 ms per question against 92, about +15 ms per call.

## 4. Call-site atlas

All calls reach `JevTransport.Decide` (`jev.go:104`). "Rec" is whether the call is written by `jevrecord`.
Options and bounds are from code; no question text is quoted.

| # | Site, call at | Unit and question shapes | State and bound | Answer to action | On failure | Rec |
|---|---|---|---|---|---|---|
| 1 | Intake `agentcore/intake_labels.go:779` (plan :578) | up to 4 captures, 2 `choice` each (event, topic); options are `criteria` keys: up to 80 stories + siblings + `none`; criteria used | story list once under handles; 30,000 B (:74); 7.9k-13.7k tokens | max-p choice; p stored (`EventP`, `TopicP` :815-822, `store/capture_label.go:54`); no threshold at read | capture stays unlabelled; 5 s (:57) | yes `intake_labels` |
| 2 | Event layer `event_layer.go:460` | up to 12 captures (:48), 1 `choice` each (`k1..kN`) over events, siblings, `none`; queued at `source_schedule.go:284` | 24 KiB (:75), summaries clipped, chunk halves past it | `p>=0.92` join (:71), `0.5<=p<0.92` tentative (:72), else none; applied :535-545 | unanswered skip the fold (:560); 30 s (:41) | yes, threshold text recorded |
| 3 | Support judge `support_judge.go:369` | 1 `choice` yes/no per citation pair of a revision, `p1..pN`, N uncapped | sentence, quote, cited passage; no byte bound | `yes` and `p(yes)>=0.5` supported (:408); p kept in basis points | pairs stay pending; 90 s (:62), 8 at once (:60) | yes |
| 4 | System 1 verb `system1_verb.go:286` (`system1Call` :216, `Ask` :134,150) | a desk's closed `choice` questions, 2-6 options, up to 12 (`yaegikernel/system1.go:26`) | host-resolved refs: 4 a question, 16 a call (:31-32) | returned to the cell with probabilities; replay by digest | request on the tape then answered or `failed`; 60 s (:74) | yes (NoText) |
| 5 | Story analysis `story_analysis.go:355` (via 4) | novelty, entity type, role per candidate; chunks of 12, 2 chunks | story text, host side | probabilities and top choice (:382) to the cell | `a.Error`; no answer | yes (via 4) |
| 6 | Prompt router `prompt_router.go:82` (`askRouter` :199, `askChoice` :212, via 4) | 1 `choice` over up to 8 destinations | owner text + destination descriptions | top choice if `p>=0.6` (`Theta`, `promptroute.go:51`; decided :279), else abstain to default + 3 alternatives | default route; 8 s (:39) | yes + frozen route record |
| 7 | Prompt classifier `prompt_class.go:58` (same flow) | 1 `choice` over input classes | owner text | same `Theta` | default class | as 6 |
| 8 | Disclosure gate `disclosure_gate.go:583` (via 4) | 1 `choice` yes/no/unsure per gated egress | request + sink summary, host-bound | allow only `P(yes)>=0.8` and `P(no)<=0.1` (`disclosure/rules.go:319-321`) | **fails closed**: escalates to the owner | yes + precommit record |
| 9 | Sub-call judge `subcall_judge.go:573` (`judgeCall` :629) | up to 20 children (:115), each up to 7 questions | each child's report; desk-written wording | argmax per question (:716-760), distribution kept; `ClassifySubCall` (`types/subcall.go:391`), gap 0.4 (:83) | work closed `failed`; 2 s backoff in 90 s (:96) | yes (NoText) |
| 10 | Desk-cast judge `desk_cast_judge.go:493` | up to 20 casts (:61), each up to 7 questions | cast objective + questions | as 9 | round failed | yes (NoText) |
| 11 | Inert judge `inert_judge.go:181` | up to 20 sampled records, 1 yes/no each | prediction text | `yes/(yes+no)<0.5` means inert (:226) | left unjudged, never retried; 2 min (:41) | yes (NoText) |
| 12 | Front importance `frontimportance/scorer.go:686` (host door) | one story a call: `importance` (10 options) + `promo` (yes/no) + section yes/no questions, 12 a call (:249) | title 300 + dek 600 runes; about 2.2k tokens | importance = chosen option (p unused); section `P(yes)>=threshold`, all 10 in `sectionlist/house.json` are 0.5 (`scorer.go:423`) | keeps old score, `Deferred++`; 45 s | yes, own `jev-calls.jsonl` |
| 13 | Op-ed gate `oped_gate.go:242` | 4 yes/no (abusive, off topic, unclear, redundant) | the op-ed + its story | first question with `P(yes)>=0.5` rejects (:274) | **never accepts on failure**; third failure "unavailable" (:289); 90 s | yes |
| 14 | Desk judge `deskjudge/choice.go:84` (route :65, complete :116) | 1 `choice` per event | evidence entries in `state.context` | choice accepted only if its share `>=0.6` (`decisive` :38) | error to `Admit`; direction not traced | not via `judgeCall`; bound `Judge` not traced |

`autoputer/run.go:1135` is the adapter every guest site uses, not a site.

- **Probabilities or top choice.** Against a threshold: 2 (0.92, 0.5), 3 (0.5), 6 and 7 (0.6), 8 (0.8 and
  0.1), 11 (0.5), 12 sections (0.5), 13 (0.5), 14 (0.6 share). Top choice only: 1 (p stored, not used), 9
  and 10 (argmax, distribution retained), 12 importance. `noul` and `score` are used nowhere; every site
  asks `choice`.
- **Thresholds are guesses, not calibrated against recorded outcomes.** 0.92 was raised from 0.85 after
  false merges on staging (`event_layer.go:68-71`), an empirical move on a failure. `Theta` is documented
  as "a starting parameter". The 0.5 values (support, sections, op-ed, inert) are the default; the
  vendor's starting 0.7 / 0.3 for noul are used nowhere; 0.8 / 0.1 in the disclosure rule cites no source.
- **Shape.** Sites 3, 9, 10, 11 batch different items; 2 batches captures; 1 packs two questions about
  each capture; 12 packs one story's questions. Sites 6, 7, 8 and 14 are one question a call and are the
  ones that could carry more.
- Questions a site can send: 8 (1), 12 (2), uncapped (3), 12 (4), 12 a chunk (5), 1 (6, 7, 8, 14), up to
  140 (9, 10), 20 (11), 12 (12), 4 (13).

## 5. The record (`internal/jevrecord`)

Schema `choir.jev_call.v1` (`jevrecord.go`), one line per call, success or failure; non-blocking (a queue
of 2,048 drops and counts).

- Fields: `call_id`, `at`, `site`, `decision_id` (the join key), `run_id`, `scope`, `threshold` (text),
  `questions[]` (`name`, `type`, `options` up to 12 + `more_options`, `digest`, `excerpt` 160 runes,
  per-question `decision_id`), `state_digest`, `state_bytes`, `answers{name: {choice, p}}` (top 6
  probabilities), `answers_cut`, `requested_model`, `model`, `provider`, `response_id`, `input_tokens`,
  `output_tokens`, `cost_usd`, `latency_ms`, `ok`, `error` (200 runes).
- Size: 8 KiB per record (`MaxRecordBytes`; over it excerpts go first, then answers, then questions).
  Files 32 MiB, 4 kept, so at most 128 MiB a computer, rotated by size. Estimate (schema, not measured;
  the files are in guests and were not read): 1.5 to 3 KiB a record, so 10,000 to 20,000 per file, about
  12 to 24 hours of paper calls at 850 an hour, 2 to 4 days in four files.
- Where: guest `jev-calls/calls.jsonl` beside the store (`jevRecorder`, `intake_labels.go:905`); the
  scorer writes `jev-calls.jsonl` on the host beside its scores (`scorer.go:295`).
- Text policy: `NoText` for `system1_verb`, `subcall_judge`, `desk_cast_judge`, `inert_judge`
  (`intake_labels.go:891-895`): digest and name only, options counted. Other sites keep a 160-rune excerpt
  and option names (handles, not stories). State is never stored, only digest and size.
- **Join to outcomes**: `decision_id` ties a call to the label, route, settlement or pair it fed (batched
  calls also carry per-question ids). Calibrate by joining `answers[name].p` of the chosen option to the
  settling outcome: support verdict vs editor override; router `p` vs route correction; intake `p` vs a
  later label change; disclosure `P(yes)` vs the owner's tap; event `p` vs a later rejoin.
- **Missing today**: (1) records sit in guests and rotate in days, with no export to a central store, so
  no fleet-wide calibration set exists; (2) the gateway receipt (central, has `usage`) and the guest
  record both carry `response_id`, so a join exists, but only if guest records are exported first; (3)
  outcomes are not a field of the record and live in other stores (labels, tape, route record), so each
  site needs its own join; (4) no state is kept, so a call cannot be replayed under a new threshold or
  after a model bump; (5) `threshold` is free text, not a number.

## 6. Surprises and anomalies

1. **Cost is 7.5x JEV.md's assumption** ($7.47 a day against about $1), and input tokens an hour rose
   about 6x between 00-04Z and 05Z with three deploys in the window (section 2.2). Attribution is **U**.
2. **Calls with 13 to 38 questions**: 81 calls (70 paper, 11 vm-70070cf7, 2 candidate fleet). By code only
   the support judge (uncapped per revision) and the sub-call and desk-cast judges (up to 140) can exceed
   12 questions. vm-70070cf7 sent 28 and 16 at 07:00:00 and 07:00:03Z and 38, 29, 18, 17, 17, 16, 14, 14
   at 11:28-11:33Z. Which site cannot be told from the host (the line carries a class, not a site; records
   are in guests). **U** Their latency is ordinary (p50 135 ms; 28 questions took 510 ms, the maximum).
3. **Front importance is concurrent** (28 calls in one second) though two pages call it serial. (C, M)
4. **An interactive call was queued 1,866 ms** (08:16:07Z) despite the reserve. **U**
5. **All 26 failures are our own 5 s queue wait**, not Jev: 0 upstream errors, 0 429s in 16,133 calls.
6. **The queue vanished after 14:41Z** with no limiter edit. **U**
7. **Hour 14Z** has 78 tails and the day's highest p90 (366 ms). **U**
8. **Post-batching 2-question calls are smaller** (7,910 against 12,668 median tokens) with no stated
   reason. **U**
9. **Output tokens are large** (21.6M against 130.8M input), billed at 0 but real work and the part of
   latency that scales with questions.
10. **Hour summaries almost never print**: 3 in 17.6 h, because the counter resets at each gateway
    restart. The `jev call` line is the only complete gateway record.
11. The line cannot name its site; adding `site` to the guest request (never sent upstream) is JEV.md's
    design step 0 and would settle items 2.

Not measurable from the host: per-call tokens against latency (no shared id); which site made a call; the
guest records and hence outcomes and calibration; bytes per token; whether any call crossed the
32,000-byte line; vendor-side queueing or cache behaviour; paper deploy time by hour.

## 7. How to re-run

All commands are read-only (counts and timings only).
```
# Pull the lines once (the grep runs on Node B; analysis is local):
ssh node-b "journalctl -u go-choir-gateway --no-pager -o short-iso --since '2026-11-10 04:00' \
  | grep -E 'jev (call|hour)|Jev judgment|host judgment|Started|body caps:'" > jevall.log

# Sections 1 and 2:
scripts/proofs/jev-call-stats.sh jevall.log        # or: --fetch "2026-11-10 04:00"

# Receipts, numeric fields only (section 3; one bounded read on Node B):
ssh node-b 'tail -n 17500 /var/lib/go-choir/gateway/jev-transport-receipts.jsonl \
  | nice jq -r "[.created_at, (.request.computer_id[0:11]), (.response.distribution.answers|length), \
    .response.distribution.usage.input_tokens, .response.distribution.usage.output_tokens, \
    .response.distribution.usage.cost] | @tsv"' > rcpt.tsv
awk -F'\t' '$1>="2026-11-10T04:08" {n++; i+=$4; o+=$5; c+=$6} END{print n, i, o, c}' rcpt.tsv
# Saving (3.4): same file, paper rows with 4, 6, 8 answers after 15:10Z: captures = answers/2, sum $4.
```

J is one day: rerun after the next full day before reading a trend; the 13+ bin (81 calls) is sparse.
