# Jev

Reference page (convention: [README.md](README.md)). Observed 2026-11-10 at
`origin/main` 5831feeac. Vendor pages re-fetched 2026-11-10 for this page
(docs.typesafe.ai `models.md`, `primitives`; OpenRouter `guides/community/jev.md`)
through a fetch tool that summarises pages with a small model, so quotes are
short strings and were not checked against raw HTML. Detail and the full
source list: [best practices](../evidence/jev-usage-best-practices-2026-11-10.md),
[measurement](../evidence/jev-measurement-2026-11-10.md),
[batched-questions design](../design/jev-batched-questions-2026-11-10.md).

Marks: **V** verified (how, when), **R** relayed-unverified, **M** measured by
us (from our own logs, date given).

## 1. What Jev is

Jev is TypeSafe's "System One" judgment model. It does not write text: it
answers fixed-form questions about a shared `state` and returns typed answers
with probabilities. Alpha endpoint through OpenRouter; there is no direct
TypeSafe endpoint in this repo. The gateway pins `~typesafe/jev-latest`
(`internal/gateway/jev.go:22`) and POSTs to
`https://openrouter.ai/api/alpha/decisions` (`jev.go:23`). Served model in
every sampled receipt on 2026-11-10: `typesafe/jev-1.14-20261014` (M). The
alias moves, so thresholds can shift on a model bump.

Question types on our wire (`validateJudgmentRequest`, `jev.go:235`):

- `choice`: pick one of a listed set; answer has `choice` and `probabilities`.
- `score`: a graded level; the vendor says score calibration is weak.
- `noul`: yes/no; no `confidence` field. Vendor starting thresholds are about
  0.7 for yes and 0.3 for no, to be tuned on our data (R: from the fetch
  summary of docs.typesafe.ai/confidence.md, not re-fetched).

`box` (image) questions are not used. The endpoint is alpha: "the shape may
change" (R, OpenRouter API reference, not re-fetched).

## 2. The one rule

**One call asks many questions about one state. The questions are answered in
parallel and are blind to each other. Do not issue Jev calls concurrently to
"call Jev more": put more questions in the call.**

- Vendor (V, 2026-11-10): "Send every question that uses the same state in
  one request." "Every answer is independent. One question's answer is not
  hidden context for another." "There is no limit on the number of questions
  in a request"; the constraint is size.
- OpenRouter (R, tutorial fetched 2026-11-10 by the earlier evidence pass):
  "All questions in the request are answered in parallel and cannot see each
  other's answers."
- Owner, 2026-11-10: "we were supposed to run parallel questions on the same
  Jev calls, not run many parallel Jev calls." Separate contexts (different
  states that cannot share one request) can justify concurrent calls, but
  that is the exception and never plan A.
- A question that needs another question's answer needs a second request
  built in code.

Correct: one capture's questions, one state, one call.

```json
{"state": {"task": "Label the capture.", "captures": {"c1": "..."},
           "stories": {"s1": "...", "s2": "..."}},
 "questions": {
   "event_c1": {"type": "choice",
     "instructions": "Which story in state.stories reports state.captures.c1's event? none if no story does.",
     "criteria": {"s1": "state.stories.s1 reports the same event.",
                  "s2": "state.stories.s2 reports the same event.",
                  "none": "No story reports this event."},
     "choices": ["s1", "s2", "none"]},
   "topic_c1": {"type": "choice", "instructions": "...", "criteria": {"...": "..."}, "choices": ["..."]}}}
```

Incorrect: the same `state` sent twice, once per question (`event_c1` call,
then `topic_c1` call), or a pool of goroutines each sending one question over
the same stories list. The state is billed N times, the request count grows
N-fold, and nothing is gained: the vendor's 13-question benchmark was 12.2x
cheaper and 11.8x faster in one call than in 13 (V benchmark page, 1 document,
1 model, 5 runs; single-call time was the sum of 13 sequential calls).

## 3. The wire format we send

Gateway request (`JudgmentRequest`, `internal/gateway/jev.go:30`):
`{state, questions{name: {type, instructions, criteria, choices}}, scope}`.
`scope` is ours (`interactive`, `intake_labels`, `oped_gate`, or empty =
bulk) and is never sent upstream. `model` is absent on purpose; the gateway
adds it. Upstream body (`call`, `jev.go:153`): `{model, state,
questions}`. Rules enforced: `state` is a JSON object; at least one question;
every question a named JSON object of a known type
(`validateJudgmentRequest`). The gateway does not cap question count.

Response: the exact upstream body is returned unmodified: `answers{name:
{choice, probabilities, confidence}}`, `model`, `provider`, `id`,
`usage{input_tokens, output_tokens, cost}`. Probabilities are the model's own
output (no logprobs), shift slightly between runs, and key order varies:
index by name. A `refusal` answer can appear for one question while the
others are present (R). Treat a missing `confidence` or `usage.cost` as a
failure, not as free (R).

## 4. Shared and per-question context in one request

- `state` is the shared context, a JSON object (vendor: "Use an object for
  most requests"). Name sub-objects by handle (`state.stories.c12`,
  `state.captures.c1`) and have each question's `instructions` point at its
  own handle. The vendor warns that accuracy falls as `state` grows with
  content unrelated to the decision, so a question should name the entries
  it judges.
- System 1 verb: a research cell's question carries `context_refs` (capture
  ids or `text:<digest>` passages). The host resolves each ref to text itself
  (`system1ResolveContext`, `internal/agentcore/system1_verb.go:449`,
  `system1ReadRef` after it), aliases it `x1..xN` in `state.context`, and the
  question's instructions end "Evidence: state.context.x1, ..."
  (`system1Request`, `system1_verb.go:533`). A cell cannot paste text as
  evidence. Bounds: 4 refs a question, 16 distinct refs a call
  (`internal/yaegikernel/system1.go:31-32`).
- Per-question wording rides in `instructions`, never in `state`.

## 5. Bounds

| Bound | Value | Status |
|---|---|---|
| Long-context mode trigger | "more than about 32,000 bytes of text", and any image; "slower and less accurate" | V 2026-11-10 (OpenRouter jev.md fetch) |
| Vendor default context | 64k tokens per request, 32k per question; header raises to 262k, slower and less accurate | V 2026-11-10 (docs.typesafe.ai/models.md fetch) |
| Our intake guard | 30,000 bytes (`intakeRequestMaxBytes`, `intake_labels.go:69-74`); a group over it splits, never trims | code |
| Event-layer state guard | 24 KiB (`eventStateMaxBytes`, `event_layer.go:75`) | code |
| Gateway body cap | 1 MiB (`jevTransportMaxBodySize`, `jev.go:24`); also bounds the response read | code; live boot log 2026-11-10 (M) |
| System 1 verb | 12 questions per call (`System1MaxQuestions`, `yaegikernel/system1.go:26`, checked cell-side and host-side), 6 choices, 400 rune question text | code. **Accuracy above 12 questions is unmeasured.** |
| Vendor question count | no limit stated | V 2026-11-10 |
| Vendor rate limits | per organisation, in tokens/s and requests/s, 429 when exceeded; **the page shows no numbers** (limits are in the console) | V 2026-11-10 |
| "20 requests a second, 250k tokens a second" | quoted by third parties; another third party says 40 rps and 100K tps | **R, unverified, disagreeing. Not a basis for any limiter.** |
| Upstream 429s seen | none in a 6 hour gateway log window (the 429 lines there were other routes) | M 2026-11-10 |
| OpenRouter's own cap on the alpha route | unknown | unknown |

Questions per call, sample of 30,000 receipts (M, 2026-11-10): 1: 4,124;
2: 14,581; 3: 5,748; 4-11: about 4,200; 12: 782; 13-20+: about 500. Calls of
12 and above are routine and have not failed. Observed overlap: a peak of 11
completions in one second (paper), no failures.

## 6. Billing

Per input token; output is free. $0.042 per million input tokens for 1.14
and 1.13 (V 2026-11-10; `internal/llmcost/pricing.go:106-112` matches; no
cached-token rate is listed and caching is undocumented, so assume none).
Each response carries `usage.cost`. Consequence: `state` plus all question
text is billed once per call, so N questions in one call pay the state once;
N calls pay it N times. Dollars are small (order of $1 a day for the whole
fleet, assumption in the evidence doc); the real gains are request count,
queueing and staying under the long-context line.

## 7. Latency (M, 2026-11-10)

Upstream p50/p90 from the gateway journal: 1 question 83/129 ms; 2 questions
with a large state (intake) 153/209 ms; 3 questions 106/146; 4-9 about
92-97/129-174; 12 questions 96/150; 16 questions 144/309 (9 calls).
Question count does not drive latency in 1 to 12; request size seems to.
Above 16 there are about 20 samples, too few to say. The vendor says only
that more questions "usually have little effect on response time". Latency
is not in the receipts; the gateway log line carries it per call.

## 8. The doors

1. Guest door `POST /provider/v1/judgments` (`HandleJudgment`,
   `internal/gateway/jev.go:374`): bearer auth, peer pin, scope validation.
   The runtime client is `gatewayruntime.JudgmentClient.Decide`
   (`internal/gatewayruntime/jev.go:58`); sites reach it through
   `Runtime.judgeCall` (`internal/agentcore/intake_labels.go:890`), which
   times the call (`callJudgeWithin`, :918) and records it.
2. Host door `POST /provider/v1/host/judgments` (`HandleHostJudgment`,
   `internal/gateway/host_judgment.go:28`): host-internal credential and a
   caller table (`hostJudgmentCallers`, :23; today only `front-importance`).
   It refuses a `scope`. Adding a caller is a reviewed change.
3. The System 1 verb: a research cell's `choir.System1`
   (`internal/agentcore/system1_verb.go`, `system1Call` :216, judge call :286),
   up to 12 closed choice questions, on the interactive scope; the host
   re-validates (`yaegikernel.ValidateSystem1Questions`). Every call is on the
   tape (request, then answered or failed), with replay for identical
   questions over identical context in a run.
4. Client route `POST /api/system1/judgments`: being added on branch
   `lane/ar-system1-route` (for the Autoradio phone app). Not on main as of
   this page.

All doors converge on `JevTransport.Decide` (`jev.go:104`).

## 9. Scopes, classes and our own limiter

Scope names a class: `interactive` (prompt router, disclosure gate, a desk's
own question), `intake_labels`, `oped_gate` (small own bucket for
stranger-driven traffic), empty = bulk (`judgmentRateScope`,
`jev_limiter.go:89`). Today `jev_limiter.go` holds a process-wide token
bucket (15/s, burst 30), 32 in flight, an interactive reserve, per-computer
caps and a shared backoff on upstream 429 that honours `Retry-After`
(`jev_limiter.go:23-60`, `acquire` :163, `backoff` :270).

**These caps are under removal.** Owner rule, in CLAUDE.md: "too many model
calls is a code problem to fix, not a limit to impose". The 15/s figure was
derived from the relayed, unverified 20 rps. What stays as failure handling:
the shared 429 backoff, the post-queue hold re-check, and the small op-ed
gate bucket. Do not add a limit, pool or reserve to Jev; batch the questions
instead.

## 10. The record

`internal/jevrecord` writes one record per call, success or failure
(`Schema = "choir.jev_call.v1"`, `jevrecord.go`), observation not authority:
a guest-local rotated JSONL, `jev-calls/calls.jsonl` beside the store
(`Runtime.jevRecorder`, `intake_labels.go:905`), 32 MiB a file, 4 files
kept, record capped at 8 KiB, non-blocking (a full queue drops and counts).

Stored: call id, time, site, decision id (the join key to the label, route,
settlement or System 1 event; per-question `QuestionDecisions` for batched
calls), run id, scope, threshold in force, per question the name, type,
options (up to 12), a digest and a 160-rune excerpt, the state **digest and
byte count (never the state)**, per answer the choice and top 6
probabilities, requested and served model, provider, response id, input and
output tokens, cost, latency, ok and clipped error. For sites whose
questions an agent writes (`system1_verb`, `subcall_judge`,
`desk_cast_judge`, `inert_judge`) `NoText` keeps only digest and name
(`judgeCall`, `intake_labels.go:890-895`). Recorded questions cannot be
replayed (no state kept); a measurement set must be generated. The gateway
separately keeps a receipt per success (request digest, full response,
`jev-transport-receipts.jsonl`, no latency).

## 11. Writing good questions

From the vendor jaggedness page and cookbook (R unless re-fetched; summarised
in the best-practices evidence doc):

- One judgment per question. "Hiding several judgments inside one question"
  does worse; avoid double negatives and inverted mappings.
- `instructions` say what to judge and point at `state` paths; `criteria`
  say what each option means. Our current criteria are long per-candidate
  boilerplate; whether they can be shortened is untested.
- Option design: choice always picks a listed option, so include `none` or
  `other`. Jev leans toward the first listed option: rotate order in tests
  and check answers stay consistent. A companion `noul` can detect "no option
  fits".
- Jev does not count, do dates, or arithmetic. Adversarial text in `state`
  can move the answer: treat state text as data.
- Thresholds: confidence measures how concentrated the probabilities are,
  not whether the answer is right; for a choice use the chosen option's
  probability. Use bands, not exact values (a noul read 0.42 then 0.44).
  Calibrate on our own record: join `jevrecord` to the settling outcome
  (editor override, route correction, support verdict) and set the threshold
  where the outcome flips. Retune after every `~latest` model bump, using
  the served model in the record. An OpenRouter cookbook sample (author-built,
  not a benchmark) had items at confidence 0.8 or above matching the human
  label 114 of 122.
- Keep the state to what the questions need; split by relevance, not by
  count.

## 12. Call sites (13)

Conversion to one call per unit of work is **in progress on 2026-11-10**:
intake labels already batch captures (`intakeBatchCaptures = 4`,
`intake_labels.go:69-74`); several sites still carry the lane A
concurrency pools that the batched-questions design deletes. Lines are from
`origin/main` 5831feeac and drift.

| # | Site | Call (file:line) | Unit | Questions per call |
|---|---|---|---|---|
| 1 | Intake labels | `agentcore/intake_labels.go:779` | up to 4 captures per candidate chunk | 2 per capture (event, topic), up to 8 |
| 2 | Event layer | `agentcore/event_layer.go:460` | chunk of captures | up to 12, 24 KiB state |
| 3 | Support judge | `agentcore/support_judge.go:369` | all citation pairs of a revision | one per pair; no byte bound |
| 4 | System 1 verb | `agentcore/system1_verb.go:286` | a desk's ask | up to 12 |
| 5 | Story analysis | `agentcore/story_analysis.go:355` (via 4) | candidate stories | chunks of 12, 2 chunks |
| 6 | Prompt router | `agentcore/prompt_router.go:154` (`askRouter` :199, via 4) | an owner prompt | 1; room to add more |
| 7 | Disclosure gate | `agentcore/disclosure_gate.go:560` (via 4) | an egress | 1 |
| 8 | Sub-call judge | `agentcore/subcall_judge.go:629` | up to 20 children | up to 7 each |
| 9 | Desk-cast judge | `agentcore/desk_cast_judge.go:493` | a batch of casts | up to 7 each |
| 10 | Inert judge | `agentcore/inert_judge.go:181` | sampled records, up to 20 | 1 each |
| 11 | Front importance (host door) | `frontimportance/scorer.go:686` | one story | 3 to 12; cap `maxQuestionsPerCall = 12` (:249); the design packs several stories per call |
| 12 | Op-ed gate | `agentcore/oped_gate.go:242` | an op-ed | 4 |
| 13 | Coding-agent desk judge | `deskjudge/choice.go:84` | an event | 1 |

Mapping of log question counts to sites is by reading the code; the guest
log carries a class, not a site (adding `site` is design step 0).

## 13. Rules we follow

1. Every question about one unit of work in one call; never split a unit's
   questions across calls.
2. Add the next question to the existing call rather than making another.
3. Share a state across items only when it is large and genuinely shared, and
   keep the request under 30,000 bytes; split the overflow, do not trim.
4. Point each question at its own sub-object of `state`.
5. No limiter, pool, reserve or switch to hold Jev back; no hard-coded
   vendor rate figure.
6. Record every call (`judgeCall`); keep the decision id so outcomes can
   settle it.
7. Measure before raising a batch size: agreement with the one-question call
   against the repeat-run noise floor, latency, bytes. The first batch size
   (4 captures, 8 questions) is a start, not a measured value.
8. Do not answer about Jev's limits, prices or accuracy from memory.

## 14. Mistakes already made

- 2026-11-09: lane A read "call Jev more" as more concurrent calls. It added
  a gateway limiter and a concurrency pool at every site (scorer 12, intake
  12, support 8, event-layer prefetch 4, story analysis 4, sub-call, cast and
  inert 4). Calls per capture did not fall; about 900 two-question intake
  calls an hour (72% of the day's calls) each re-sent the same story list.
  The owner corrected it on 2026-11-10 (quote in section 2).
- A 60-a-minute per-computer bucket (and a 2 s serial gate at the sub-call
  judges) sized for a limit nobody had read: it capped Jev at 1 a second per
  computer, below the fleet's need.
- The relayed 20 requests a second figure, which was marked "unverified" in
  the evidence, was hard-coded as the process-wide limiter (15/s, burst 30).
  The vendor page shows no numbers and third parties disagree.
- A 1.14 intake call was about 9,000 tokens, past the long-context line,
  because per-candidate criteria strings were repeated 80 times in two
  questions.
- The first measurement design said "8 KiB" for the body cap; it is 1 MiB.

## 15. Open and could not verify

- The vendor and OpenRouter rate limits for our key; whether a second gateway
  process shares the key.
- Accuracy against question count, position, and above 12 questions: nothing
  measured, nothing published found.
- Whether the 32,000 bytes counts questions or only state.
- Whether `criteria` can be shortened or dropped.
- Caching behaviour and any cached-token price.
- Third-party material (OpenTweet, OrcaRouter, Towards AI) was not re-fetched.

## Sources (fetched 2026-11-10 unless noted)

docs.typesafe.ai: `models.md` (price, limits, context), `primitives`
(one request, independence). openrouter.ai/docs/guides/community/jev.md
(32,000 bytes, billing). Earlier same-day fetches, relayed here: `jev-tutorial.md`,
`cookbooks/parallel_questions.md`, `model-jaggedness.md`, `confidence.md`,
`patterns/fan-out.md`, the OpenRouter blog and API reference, and the two
third-party pages. Repo evidence listed at the top.
