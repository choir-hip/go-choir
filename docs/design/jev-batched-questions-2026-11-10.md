# Jev: many questions in one call, not many calls (design and evidence, 2026-11-10)

Mutation class: green (design and evidence; no product code). Rollback: revert
this commit. Source: `origin/main` at d71bb1751 plus read-only counts from the
Node B journal (counts and percentiles only; no question text, no answers, no
secrets). Nothing was run against the gateway, staging or d@e.

Owner correction (2026-11-10, verbatim): "we were supposed to run parallel
questions on the same Jev calls, not run many parallel Jev calls".

## 1. Summary

**What the owner meant.** Jev answers all the questions of one request in
parallel and the questions cannot see each other (OpenRouter's Jev tutorial:
"put every independent question about the same state in one request"; "All
questions in the request are answered in parallel and cannot see each other's
answers"). So the way to "call Jev more" is to put more questions into each
call: one `state`, many questions.

**What lane A built.** It read "many calls at once" as "many concurrent HTTP
calls". Commits dd9fba199 (gateway limiter: 15 per second, burst 30, 32 in
flight, an interactive reserve and per-computer caps), ecefb752a
(`internal/fanout` and a concurrency pool at every site: scorer 12, intake
labels 12, support judge 8, event-layer chunks 4, story-analysis chunks 4,
sub-call, cast and inert judges 4), fdf501db8 and 1688ab35a (`internal/jevrecord`,
one record per call). The calls per capture did not go down; the pools made them
cheaper to run in parallel, not fewer.

**The gap, in numbers (2026-11-10, Node B journal, `jev call` lines).**
8,011 calls so far today. 5,774 carry exactly 2 questions (72%). The paper
sent 220 to 1,315 calls an hour; in the last five whole hours 860 to 1,045 an
hour were 2-question calls. Every one of those 2-question calls is an **intake
label**: one capture, the "event" question and the "topic" question, asked
once per candidate chunk (section 2, row 1). The candidate story list in each
call is the same for every capture of a poll, and it is re-sent for every
capture and every chunk. That is the call where batching pays most.

**A finding that changes the plan.** The binding limit is not the 1 MiB body
cap. OpenRouter's Jev guide says: "Requests with more than about 32,000 bytes
of text, and every request with an image, run in TypeSafe's long-context mode",
which is "slower and less accurate than staying within its default limits".
(The repo already encodes this: `eventStateMaxBytes` 24 KiB.) An intake call
today is about 9,100 input tokens (token audit, 2026-11-09), which is near that
line already. Each intake question carries one criteria string per candidate
story, so the questions, not the state, are the bulk of a call. Batching
captures into one call therefore cannot simply share the state; it has to
shrink the per-question criteria first (section 3, section 5). This is unknown
until measured (section 4).

## 2. Per call site

"Unit" is what one question set is about. All timeouts and sizes from the code
at d71bb1751. "Calls/h" is the paper's current rate unless said otherwise.

| # | Site (file) | Unit | Questions per unit | Units per typical pass | Issued today | Shared context | One call for many units? | Partial failure | Timeout |
|---|---|---|---|---|---|---|---|---|---|
| 1 | Intake labels (`agentcore/intake_labels.go`, `askIntakeChunk` :400, `labelOneCapture` :244) | a new capture | 2 (event, topic; topic only if there is a candidate story) | a poll's new captures: about 3 on average (188 captures over 63 polls an hour), up to 12 | one call per capture per candidate chunk (80 candidates a chunk, up to 7 chunks over the 8 h window), pool of 12 (`intakeLabelConcurrency`) | the candidate story list (headline, time, lede for the newest 30) and the editor corrections, **identical for every capture of the poll**; siblings = the poll's other captures | yes: state = candidate chunk + the poll's captures; questions = `event_i`, `topic_i` per capture. Siblings are already in state | the answer of a capture that is missing or off-menu voids only that capture (its label stays unwritten, as now); a failed call voids the batch's captures, each left unlabelled | 5 s (`intakeLabelCallTimeout`); poll budget 15 s |
| 2 | Event layer (`event_layer.go` :387, `resolveEventChunk`) | a capture | 1 (`k<N>`) | a flush every 60 s, about 3 captures | already batched: chunk of up to 12 captures; chunks prefetched with a pool of 4 | candidate events | already one call per chunk | chunk's captures stay unresolved | 30 s |
| 3 | Support judge (`support_judge.go` :369) | a citation pair (or a citation group) | 1 (`p<N>`) | all the pairs of one Texture revision | already batched: one call per revision, runtime-wide pool of 8 revisions | none; each pair carries its own sentence and cited passage, so the request is the sum of the passages | already; no cap on pairs in one call, which is a request-size risk (a revision with many long passages crosses 32 KB) | a pair with no usable answer stays pending; the rest are recorded | 90 s |
| 4 | System 1 verb (`system1_verb.go` :274) | whatever the desk asks | up to 12 | one call per verb invocation | one call, serial in a cell | per-question context refs resolved by the host | already batched; the cap of 12 (`yaegikernel.System1MaxQuestions`, validated cell-side and host-side) is the limit | the verb returns the whole result; a failure is on the tape | 60 s |
| 5 | Story analysis (`story_analysis.go` :338) | a candidate story | about 1-3 (novelty, entity type, role) | up to 24 questions, 2 chunks of 12 | chunks of 12 through the verb, pool of 4 | per-story text from the host | two chunks of 12 could be one call of 24 once the cap lifts | a failed chunk sets `a.Error` for its candidates | 60 s |
| 6 | Prompt router (`prompt_router.go` :82; the prompt bar classifier shares the call) | an owner prompt | 1 | 1 | one, interactive scope | the prompt and the desk list | no, nothing to join; **adding questions to the same call is the opportunity** (edit/comment/request, route) | rule fallback to the default desk | 8 s |
| 7 | Disclosure gate (`disclosure_gate.go` :583) | an egress | 1 | 1 | one, interactive | the egress text | no | gate policy decides | 60 s |
| 8 | Sub-call judge (`subcall_judge.go` :566) | a delegated child's report | up to 7 | up to 20 children a call (`subCallJudgeBatchMax`), so up to 140 questions | already batched; pool of 4 across batches | the child's report is per-child (evidence in context) | already; the largest calls in the log (38 questions) probably come from here, though the log carries no site | an item with no usable answer closes `failed` with that reason; the others settle | 90 s |
| 9 | Desk-cast judge (`desk_cast_judge.go` :493) | a cast | up to 7 | a round's casts, 20 a batch | already batched; pool of 4 | per-cast evidence | already | the round marks failed | 90 s |
| 10 | Inert judge (`inert_judge.go` :173) | a sampled prediction record | 1 | a tray, 20 a chunk | already batched | the record text | already | records without an answer are left unjudged | 2 min |
| 11 | Front importance scorer (`frontimportance/scorer.go`, `askStory` :648; host door) | a front-page story | importance + promo + the house sections, up to 12 | 5-15 stories a half hour, 438 calls so far today | one call per story (12 questions, the `maxQuestionsPerCall` cap), pool of 12, no tape record | title and dek of one story (about 1,300 tokens per call) | possible: state = `stories.s1..sN`, questions = 12 per story with the story named in each question; per-story questions are the bulk, so the saving is calls, not tokens | one story's missing answer leaves its row; the cap is the scorer's own, not the door's | 45 s |
| 12 | Op-ed gate (`oped_gate.go` :242) | an op-ed | 4 | at most 5 per tick; 2 calls today | one call per op-ed, own scope | the story (6,000 chars) and the op-ed (8,000 chars) | no: the op-ed text is the unit's own and large; two op-eds on one story would share only the story. Too rare to matter | a verdict needs all 4 answers; one missing fails the op-ed (retry up to 3) | 90 s |
| 13 | Coding-agent desk judge (`deskjudge/choice.go` :84, `route.go`, `complete.go`) | an interim update or a completion | 1 | one per event | one, each its own context | the evidence for that event | no | fails safe per its own rule | 2 min route, 5 min complete |

Notes.

- The guest door logs only a **class** (bulk, interactive, oped_gate), not the
  site, so the question counts cannot be mapped to a site from the log alone.
  The mapping above is by the code: the only call that builds exactly two
  questions is row 1 (event + topic, topic omitted when no story is a
  candidate, which gives the 608 1-question bulk calls). The scorer (row 11)
  builds importance + promo + sections, which is the 10-12 counts, and the
  event layer (row 2) builds 12 (the 469 at 12 is both). Row 8-10 produce the
  3-20+ tail. The 129 interactive 1-question calls are rows 6, 7 and 13.
  Adding `site` to the limiter's log line is part of the change (section 5).
- The 2-question rate is not the capture rate. The token audit measured 182
  label calls an hour on 2026-11-09 with the shorter window; the 8 hour window
  (D2 fix, up to 80 candidates a chunk, up to 7 chunks) turned each capture
  into about 4 to 5 calls (900 calls an hour over about 190 captures an hour,
  assuming the capture rate held, which is not measured today).
- Rows 2, 3 and 8-10 already follow the owner's intent. Row 1 is the large one;
  it was never batched, and the event layer next to it already shows the shape
  (`eventQuestion`, chunk of 12, 24 KiB bound). The token audit (2026-11-09,
  `docs/evidence/token-efficiency-2026-11-09/intake-and-jev.md`, lever 2)
  proposed the same merge: "Merge labels and events into one batched Jev call
  per flush".
- The two layers (labels and events) ask the same identity question of the same
  captures; lane A did not change that. Whether the events layer still earns its
  calls is the audit's question, not this document's.

## 3. The physics

**Tokens.** Per call: the task text (about 150 tokens), the state, and each
question's text and criteria. Output is typed answers with probabilities and is
free ("Output tokens are free", OpenRouter Jev guide); about 240 bytes an answer
(720 bytes for 3 questions, from the host door log).

Intake today (row 1), from the audit's own accounting: about 9,100 input tokens
a call, of which the shared story block is about 6,000 and the two questions
with a criteria string per candidate about 2,500. For a batch of k captures in
one chunk call, the cost is roughly: shared stories S (about 6,000) + k x
(capture state about 400 + two questions about 2,500). So:

| captures in a call | questions | input tokens, current encoding | per capture | bytes (about 4 per token) |
|---|---|---|---|---|
| 1 | 2 | about 9,000 | 9,000 | about 36,000 |
| 4 | 8 | about 19,000 | 4,800 | about 76,000 |
| 16 | 32 | about 52,000 | 3,300 | about 208,000 |
| 32 | 64 | about 98,000 | 3,100 | about 392,000 |

These are estimates from the audit's component sizes, not measurements. They
show two things. (a) With the current encoding the per-question criteria (2,500
tokens a capture) dominate once k is above 2, so the sharing saves about
one-third of tokens at best, and the long-context line (32 KB, about 8,000
tokens) is crossed even by k = 1 at 9,000 tokens. (b) The way out is to shrink
the questions, not the state: the criteria text per candidate ("state.stories.c12
reports the same event as the capture, or its direct development") is a
boilerplate sentence repeated 80 times in each of two questions. Whether
`criteria` can be shortened or dropped, with the meaning carried once in
`instructions`, is unknown and is arm B of the measurement. If it works, a
capture's questions fall to well under 500 tokens and k = 8 to 16 fit a call
under 32 KB.

Dollars are not the issue: Jev input is about $0.042 per million tokens
(`docs/evidence/model-prices-2026-10-14.md`, 1.13 pricing), and 900 intake
calls an hour at 9,000 tokens is about 8 million tokens an hour, about $0.34 an
hour. Batching saves calls, queueing and long-context risk, not much money.
The payoff the owner is after is that **more questions about the same state
cost almost nothing extra**, which is the learning-loop point: the unit's state
is already paid for.

**Calls an hour, intake labels (row 1), before and after.** Before: about 900.
A call carries captures from one poll; a poll has about 3 captures (188 over 63
polls an hour); a flush that collects across polls (the event layer already
flushes every 60 s) gets about 3 a minute. Chunks stay (a capture still needs
each candidate chunk, 4 to 5 on average); batching reduces per-chunk calls by
the captures it joins.

| k questions per call | captures per call | with poll-level batching (about 3 captures available) | with a 60 s flush (about 3) | with a 5 min flush (about 16) |
|---|---|---|---|---|
| 1 (today) | 1 (2 q) | 900 | 900 | 900 |
| 8 | 4 | about 300 (3 captures per call) | about 300 | about 225 |
| 32 | 16 | about 300 | about 300 | about 60 |
| 64 | 32 | about 300 | about 300 | about 60 (all 16 captures fit) |

So k = 8 is worth about a threefold cut, and k = 32 or 64 changes nothing for
intake unless captures are held for minutes, which delays labels (labels do not
gate arrival, but the wake uses them; a hold of minutes is a product decision,
not a free choice). The call-count win of this change is therefore about
**900 to about 300 calls an hour for the paper (minus about 600 calls an hour, 60% of
the paper's Jev calls)**, plus the removal of the limiter queue that the pools
created (mean `queue_ms` of 2-question calls today is 151 ms, 10-question
calls 546 ms, versus 0 to 34 ms for the other sizes except 10, which is a pooled site too).

For the other sites the call counts barely move (they are already batched),
except: row 11 at k = 64 would fold 5 stories into one call (438 calls so far
today to about 90; small), and row 5 two calls of 12 to one of 24.

**Size against the cap.** The gateway body cap is 1 MiB and the answer about
240 bytes a question: 64 questions is about 15 KB of answer. The cap does not
bind. The 32 KB long-context line does. `validateJudgmentRequest` does not cap
question count.

**Latency.** From today's journal (successful calls, upstream time):

| questions | calls | upstream p50 ms | p90 ms |
|---|---|---|---|
| 1 | 737 | 83 | 129 |
| 2 (intake, large state) | 5,755 | 153 | 209 |
| 3 | 357 | 106 | 146 |
| 4-9 | 479 | 92-97 | 129-174 |
| 10 | 125 | 91 | 140 |
| 12 | 469 | 96 | 150 |
| 16 | 9 | 144 | 309 |
| 19 | 2 | 176 | |
| 28 (n=1) | 1 | 510 | |
| 38 (n=1) | 1 | 164 | |

Latency is **not** driven by question count in the 1 to 12 range: 12 questions
(small state) is 96 ms, a 2-question call with a large state is 153 ms. The 2
question intake call is the slowest common call because its request is the
largest. The working hypothesis is that input size, not output length, sets
latency (output is a few hundred bytes), and a batch that adds questions to a
small state is nearly free while a batch that grows the request is not.
Above 16 questions there are about 20 samples with ragged tails (309 ms p90 at
16; one 510 ms at 28), which is not enough to say. What is unknown: the curve
at 32 and 64, and whether the vendor's per-question work is sequential on the
backend ("answered in parallel" is the vendor's statement; we have not checked
it).

**Accuracy risk of large batches. Unknown; measure it.** What is claimed
(vendor): questions are answered in parallel and "cannot see each other's
answers", which argues against cross-contamination between questions through
the answers, but says nothing of positional effects in a long prompt, or of the
shared attention over a long question list. What the vendor says against: the
long-context mode (above about 32,000 bytes of text) is "slower and less
accurate" than default. Numbers for the effect of question count on accuracy
are not published on any page I could open (tutorial, guide, search results).
Do not infer from memory; section 4 is how to find out. Two further risks
specific to our shape: (1) batching with criteria strings that mention the
other captures (siblings) could leak one capture's identity into another's
choice; the sibling list is already in the state today, so the risk is not
new, but it will be tested; (2) a question that names its unit by alias
(`capture_3`) must be unambiguous when the instructions say "state.capture".

## 4. The measurement that was never made

Goal: choose k per site from evidence, not memory. Measure (a) accuracy: how
often a question's answer at k = 8, 32, 64 agrees with the same question asked
alone (k = 1) and with labelled truth, and (b) latency at k = 1, 8, 32, 64.

**Doors available without the guest bearer.**

1. The intake path on d@e. Row 1's `labelCaptures` runs on every poll of a
   d@e source, and the label rows carry the editor overrides
   (`RecentCaptureLabelOverrides`) where an editor has corrected a label, which
   is labelled truth. This needs no new door, but its call shape is the one
   under test; it is the **A/B vehicle** for the change itself (section 5, step
   2), not a free-form k sweep.
2. The host door `POST /provider/v1/host/judgments`. It takes any question
   count (the door's only checks are `validateJudgmentRequest` and the 1 MiB
   cap); the 12 in `scorer.go` (`maxQuestionsPerCall`) is the scorer's own
   constant, not the door's. A measurement run needs **one change**: either a
   second `hostJudgmentCallers` entry (for example `jev-measure`, in a reviewed
   change, as the comment at `host_judgment.go:21` requires) or a flag on the
   scorer's `plan`/`askStory` that packs several stories' questions into one
   call. The host caller is the better one: no guest, no tape, and the gateway
   already logs `questions=N` and the receipt keeps the distribution.
3. The System 1 verb in a research cell on d@e reaches Jev with the guest
   credential but needs no bearer in the operator's hands. It is capped at 12
   questions in two places (`yaegikernel/system1.go:26`, the cell-side refusal
   at :97, and the host re-validation). Reaching 32 or 64 needs the constant
   `System1MaxQuestions` raised (one number, validated in both places) and
   `System1MaxChoices` left as it is. Row 4's cap is also a candidate for
   permanent lifting once the measurement allows it.
4. `scripts/jev-probe` (build tag `jevprobe`, in tree) is the existing known-
   answer parity probe at k = 1, 8, 32, 64, but it runs inside the guest and
   needs the bearer. Use it only as the cheap position-effect check, not as the
   measurement.

Recorded questions cannot be replayed: `jevrecord` keeps a digest and a 160 rune
excerpt of the question and never the state, and the gateway receipts keep a
request digest and no question text. The set must be generated.

**Procedure** (run on d@e, on the host door; never the paper or an owner's
computer; the gateway is not called from anywhere else):

1. Build a question set per site from d@e data: **row 1 set** = 200 captures
   from d@e's captured sources, each with its real candidate list at the time
   (the story index) and its two real questions; **row 11 set** = 100 stories
   of d@e's front with the real 12 questions; **parity set** = 200 known-answer
   parity questions (the probe's).
2. Truth. Row 1: the d@e editor overrides where they exist, plus a labelled
   subset: 100 of the 200 captures labelled by a reviewer reading the headlines
   (owner or a Sonnet seat; label before looking at any model answer). Row 11:
   no editorial truth exists; use agreement with k = 1 only. Parity: truth by
   arithmetic.
3. Arms (each arm = same questions, only the packing differs):
   - A: k = 1 question per call (the baseline; also repeated 3 times to measure
     the model's own run-to-run noise, which the tutorial says is nonzero:
     "Your probabilities will differ slightly from run to run");
   - B: k = 8, 32, 64 questions per call with the **current** encoding where it
     fits (it will exceed 32 KB at k = 8 for row 1; record request bytes and
     tokens);
   - C: k = 8, 32, 64 with the **shortened criteria** encoding (section 3(b));
   - D: shuffled order within each batch (to read position effects) at k = 32
     and 64.
4. Per arm and k, per question, record: request bytes, input tokens, upstream
   ms, the choice, the top probabilities, and the question's position in the
   call. Calls per cell: 200 questions / k calls, three repetitions: at most
   about 600 calls for A, 75 for k = 8, 19 for k = 32, 10 for k = 64, across
   arms B to D: under 1,500 calls total. Pace to stay inside the limiter (15 a
   second, but the paper shares it; use 3 a second).
5. Metrics.
   - Agreement with k = 1: the fraction of questions whose top choice at k
     equals the k = 1 top choice. Baseline to beat: A's own agreement with
     itself on repeat (noise floor).
   - Accuracy against truth where it exists, and the total variation distance
     of probability vectors from the k = 1 median, per position decile.
   - Latency p50, p90 and p99 per k; request bytes against the 32 KB line.
6. **Pass bar** for a k at a site (to be fixed before the run, so the result
   cannot move it):
   - agreement with k = 1 is at least the noise floor minus 1 point, and never
     more than 2 points below it, at 200 or more questions (the 95% interval of
     a proportion at n = 200 is about plus or minus 3 points at 90%, so say
     the bar as "not lower, within the noise");
   - accuracy against truth is not lower than k = 1 by more than 2 points;
   - no position decile is more than 3 points worse than the best decile;
   - upstream p90 at k is not more than 2 times the k = 1 p90, and the request is
     under 32 KB;
   - no failure rate above the k = 1 rate.
   The chosen k is the largest that passes. If none passes at 8, the site stays
   at one unit per call.

## 5. The proposed change

Principle: **one call per unit group, one in-flight call per site, many
questions inside**. Smallest first. Mutation class orange for each step except
the gateway log line (red when the limiter files are touched, so a tier 1 Devin
review; the data-system tempo section no longer applies).

**Step 0 (green, now): this document and the measurement set.** Add `site` and
request bytes to the limiter's log line (`logJevCall`, `jev_limiter.go:338`,
which now carries `class` but not `site` or bytes). Without it the next
analysis repeats this one. Test: `TestJevLimiterLogCarriesSite`-style check on
the line.

**Step 1 (orange): intake labels batch the poll's captures (row 1).** This is
the only change with a large effect.
- `labelCaptures` builds, for each candidate chunk, one call: state = the
  chunk's candidates + the corrections + the poll's captures (as
  `captures.c1..cN`; the sibling list is the same list); questions =
  `event_i` and `topic_i` for each capture, with the siblings choices
  excluding `i`. Cap k_questions per call at the value the measurement
  picks; start at **8 questions (4 captures)**, which is the largest size the
  logs show working well and routinely (125 calls at 10, 469 at 12, with
  p50 about 95 ms), and shrink the criteria text (arm C) first, because the
  current encoding will not fit four captures under 32 KB. If arm C fails,
  the fallback is 2 captures a call (4 questions).
- Collect across polls only if the measurement shows the benefit is worth it;
  start with poll-level batches only (no new holding, no new queue, nothing
  that outlives a crash: a restart still closes the poll).
- The answer is read per capture: `event_i` and `topic_i` missing or off-menu
  voids that capture alone (it stays unlabelled), exactly as today; a failed or
  timed-out call leaves all its captures unlabelled, as today, and the next
  poll's captures are new work (no resume). The call's timeout stays 5 s;
  measure whether a 4-capture call stays under it.
- One record per call stays (`jevrecord.Record` has `Questions []` and
  `Answers map[name]` already, so each question's answer, the model, tokens,
  cost and latency are on the call's row, and a capture's `DecisionID` joins
  to the label row). Add the per-question decision id (the capture id) to
  `jevrecord.Question` so a batch joins per capture, not only to its first
  capture. Cost on the label row is the call's usage divided by its
  captures (today a capture holds its call's usage whole).
- Files: `internal/agentcore/intake_labels.go` (`labelCaptures`,
  `labelOneCapture`, `askIntakeChunk`), `internal/jevrecord/jevrecord.go`
  (per-question decision id), `internal/store/capture_label.go` (usage split,
  unchanged schema). Tests: `TestIntakeBatchesPollCapturesIntoOneCallPerChunk`
  (count calls with a fake judge), `TestIntakeBatchPartialAnswerLabelsTheRest`,
  `TestIntakeBatchFailureLeavesEveryCaptureUnlabelled`,
  `TestIntakeBatchRespectsKQuestionCap`, `TestIntakeBatchOffMenuChoiceVoidsOneCapture`,
  `TestIntakeBatchByteBound` (a call over the byte bound splits, as the event
  layer does), and the deployed proof: d@e's staged poll produces fewer `jev
  call` lines per capture than the baseline, with the labels equal.

**Step 2 (orange): measure on d@e before raising k.** Run section 4 against
the host door on d@e once the criteria arm exists; set k per site from it.

**Step 3 (orange): the lane A pools.** Delete rather than keep:
- `intakeLabelConcurrency` pool and `intakeSlots`: delete (one call per chunk,
  chunks are ordered and few).
- `scorerConcurrency` 12: shrink to 2 (the scorer's calls are 12 questions
  and small; once the scorer packs stories into calls the pool has nothing to
  hide).
- Event-layer prefetch (`eventCallConcurrency`, `prefetchEventCalls`,
  `eventCallKey`, the "ask again if the question changed" path): delete. The
  event layer already batches 12 captures and the prefetch exists only to
  run chunks in parallel, which the owner did not ask for; commits stay
  serial and each chunk is asked after the previous commit, which is the
  original behaviour and removes the invalidation complexity.
- Story analysis `analysisCallConcurrency` 4, sub-call, cast and inert pools of
  4 (`subCallJudgeConcurrency`): shrink to 1 in flight per site; the batch
  already carries the questions. Remove the serial 2 s spacing as already
  done; keep the 429 backoff in `callSubCallJudge`.
- Support judge pool of 8 revisions: keep as is, one call per revision is
  already the batched shape; it is bounded by revisions, not by a fan-out.
- `internal/fanout`: keep only where there are genuinely independent
  *chunks* beyond k (the overflow path: a unit group over k questions or over
  the byte bound splits into more than one call); the overflow runs with the
  existing helper, limit 2. If nothing but the overflow uses it after step 1,
  fold it back into the two remaining callers and delete the package.
- Gateway limiter (`jev_limiter.go`): the global rate (15 per second), burst
  and the 32 in flight were sized for the pools. Batched, the paper's total
  falls below 400 calls an hour, about 0.1 per second, so the rate and
  in-flight caps cannot bind. Keep the per-computer rate and in-flight cap as
  an abuse bound (a compromised guest), but **delete the interactive
  reserve, the interactive per-computer budget and the bulk/interactive
  split**: with one in-flight call per site and no burst of 80, nothing
  starves the router and the class machinery (`jevInteractiveReserve`,
  `jevInteractiveTokenReserve`, `jevInteractiveInFlight/Rate/Burst`,
  `inFlightFull` by class) is complexity with no workload. The op-ed gate's
  own bucket stays: it bounds stranger-driven traffic, not a throughput
  choice. Keep the shared backoff on an upstream 429 and the hold re-check
  (those are failure handling, not concurrency). Tests: remove the
  class tests, keep `TestJevLimiterPerComputerCap`,
  `TestJevLimiterSharedBackoffHonoursRetryAfter`, `TestJevLimiterQueueWaitBounded`.
  Red mutation class (gateway); do this last and only after step 1 is on
  staging and the call rate has been seen.

**Step 4 (orange): the other sites, one at a time and only where the
measurement allows.** Scorer packs up to 5 stories per call (new
`maxQuestionsPerCall` semantics: the scorer's cap rises, with the question
names carrying the story key); the System 1 verb cap goes from 12 to the
measured k (one constant in `yaegikernel/system1.go`, validated cell and host);
story analysis becomes one call of up to 24; the prompt router and the
disclosure gate add the questions they will need next to their one question,
rather than asking again (which is the learning-loop gain: more questions on
the same state at no extra call). The support judge gets a byte bound (a
revision's pairs split into calls under 32 KB); today it has none.

**Order to land:** 0, then 1 (with the shortened-criteria encoding, behind the
d@e proof of section 4 arm C, no flag: it is the code path), then step 2's
numbers set k, then 3 (deletions) and 4. Each lands as its own commit with
`Ring-Expedite` while the expedite policy holds.

**Rollback.** Each step is one revert. Step 1: reverting returns to one capture
per call (the old behaviour, 900 calls an hour, which the limiter and the pools
carried). Step 3 is the only step whose revert re-adds machinery; land it
after step 1 has run on staging for a soak, so the pools are not needed when it
goes. No data migrates: label rows keep their schema.

## 6. What I could not determine

- Accuracy at any k above 1: nothing measured, nothing published that I could
  open. The vendor says questions are answered in parallel and cannot see each
  other; it does not say what a long question list does to each answer.
- Whether "more than about 32,000 bytes of text" counts the questions or only
  the state, and what the slow mode costs in accuracy. This decides the intake
  design and is the first thing to measure (arm B bytes against arm B accuracy).
- Whether the `criteria` strings can be dropped or shortened without losing
  accuracy (arm C); the code treats them as required, and I did not find where
  the vendor documents them as optional.
- The real request size of an intake call: the receipts keep a digest, and
  the log line carries no bytes. The 9,000-token figure is the token audit's
  from 2026-11-09, before the 8 hour window (up to 80 candidates a chunk)
  landed; today's calls are probably larger.
- The capture rate today, which fixes the 900 to 300 estimate (it is a
  derived figure from the 188 an hour of 2026-11-09 and the chunk count).
- Which site the 3 to 38 question calls come from: the log has no site. Rows
  8 to 10 are my reading, not an observation.
- The vendor's true rate and token limits: the sources conflict (80 requests
  a second and 100K tokens a second on one page; 1,200 requests a minute and
  250K tokens a second on another), and neither is the vendor's own page I
  could open. After batching the paper is far below any of them.
- Whether the events layer is still worth its calls next to the labels (the
  audit says it is a shadow); this design does not touch it beyond deleting its
  prefetch.

Sources: OpenRouter, "Jev" guide (https://openrouter.ai/docs/guides/community/jev.md)
for the 32,000 byte long-context line and "slower and less accurate"; OpenRouter,
"jev tutorial" (https://openrouter.ai/docs/guides/community/jev-tutorial.md) for
"put every independent question about the same state in one request", "answered
in parallel and cannot see each other's answers", and run-to-run variation.
In-repo: `docs/evidence/jev-measurement-2026-11-10.md`,
`docs/evidence/token-efficiency-2026-11-09/intake-and-jev.md`.

## 7. Scorer packing: ways it can fail (written before the change, 2026-11-10)

Owner rule (2026-11-10): "It's important that we don't call Jev concurrently,
but ask single Jev calls many questions in parallel." The front importance
scorer (`internal/frontimportance/scorer.go`) asked one call a story through a
pool of 12. It now packs the questions of as many stories as fit under a
30,000-byte request bound into one call, stories keyed by handle in `state`
(`stories.s1`, `stories.s2`, ...), overflow as further calls one after another.
Mutation class orange; rollback: revert.

What can go wrong, and the test or fact that answers each:

1. **A handle mismatch attributes one story's answers to another.** The answer
   map is read only by exact key `<handle>_importance`, `<handle>_promo`,
   `<handle>_sec_<hash>`, never by prefix scan (`s1` is a prefix of `s10`; the
   underscore separator and exact lookup keep them apart). Handles are assigned
   from the batch position at build time and the same slice maps back. Test:
   answers returned in a different order than asked still land on the right
   story, and 12 stories in one call (`s1`..`s12`) do not cross.
2. **A story larger than the bound alone.** A story is title (clipped 300) and
   dek (clipped 600) plus at most a dozen questions, about 8 KB, so it cannot
   exceed 30 KB; if it ever did it goes alone in its own call (the bound is a
   packing target, never a refusal). Test: bound of 1 byte gives one story a
   call.
3. **Duplicate question names.** Every name carries its handle prefix, so two
   stories asking the same section cannot collide; handles are unique in a
   call. Test: a call of several stories has as many distinct names as
   questions.
4. **A partial answer set from the judge.** A story whose answers are missing
   or unusable fails only that story, with today's handling (importance
   unusable fails the row, the old row is kept; a section answer unusable is
   left unasked and tried next run). The other stories in the call are
   unaffected. A failed whole call fails exactly the stories in it. Tests for
   both.
5. **Cache keys change and everything is re-asked.** The cache is the score row
   (revision, criteria version, judge version, section question hash); none of
   it involves a wire question name or the state shape, so the change re-asks
   nothing. The existing cache tests (kept row asks nothing; edited question
   asks only that question) must pass unchanged in meaning.
6. **Ordering.** Stories are planned and packed in front-list order so the
   visible page goes first and a bounded run defers the archive; batches run
   one after another in that order. `MaxCalls` now counts packed calls; stories
   in batches beyond it are deferred with their old rows kept, as before.
7. **The host door's own validation.** `HandleHostJudgment` (`host_judgment.go`)
   applies the 1 MiB body cap (`bodyRouteJudgment`, `body_caps.go:109`,
   `jevTransportMaxBodySize` `jev.go:24`) and `validateJudgmentRequest`
   (`jev.go:235`), which checks the state is an object, at least one question,
   each question a named object of type noul, choice or score. It has no cap on
   question count. The scorer's old `maxQuestionsPerCall = 12` was the scorer's
   own constant (its comment cited `System1MaxQuestions`, which binds the System
   1 verb, not this door); it is deleted. The real binding limit is the 32,000
   byte long-context line (section 1), hence 30,000.
8. **A stale deadline mid-run.** The run budget is checked before each call; a
   spent budget defers the rest of the batches (old rows kept), as before for
   stories.

### Measurement before packing across stories

The scorer's packing machinery stays, but `maxStoriesPerCall` is 1 (a tuning
constant in `internal/frontimportance/scorer.go`, not a setting). Stories are
separate contexts with no shared state, and the vendor's rule is one request per
state. Packing three stories puts up to 36 questions and three unrelated
stories in one state, where accuracy above 12 questions and cross-story
anchoring of a comparative judgment (importance) are both unmeasured. Calls are
serial either way.

The evidence that would raise it (not built now): ask at least 100 real stories
both ways, one story per call and packed (k = 2, 3, 4 stories), through the host
door on d@e, then compare
- importance agreement with the one-story call, exact and within one point;
- the rank correlation (Spearman) of the resulting front order;
- the yes-probability delta per section question (mean and 95th percentile
  absolute difference);
- the noise floor: the one-story arm asked twice.
Pack across stories only if packed agreement is within the one-story arm's own
run-to-run noise and the rank correlation is not lower than the noise floor's.
Budget note: a front page is at most 100 stories (`frontMaxLimit`), so even a
cold start asked one story a call is at most 100 serial calls, about 15 to 25 s
against the 3-minute budget; packing is not needed for time.
