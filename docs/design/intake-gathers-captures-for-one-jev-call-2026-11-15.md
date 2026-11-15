# Intake gathers the captures that are waiting into one Jev call

Date: 2026-11-15. Class of this document: green (design and investigation; no
code, no tests, nothing compiled, nothing run). The change it designs is
orange (runtime behavior) and touches a red surface (provider calls), so it is
reviewed as red when it lands. Code lines are at `718ea5ed4` (`origin/main`
plus 8b15ec2e9, the Jev entry point) and drift.

Sources read: [CLAUDE.md](../../CLAUDE.md) (the Jev rule under "No limits on
our own model calls"), [the Jev reference](../reference/SYSTEM1.md) and its
subpages [jev-vendor.md](../reference/system1/jev-vendor.md) and
[our-usage.md](../reference/system1/our-usage.md), the problem record
[intake-flushes-single-captures-as-their-own-jev-call-2026-11-15.md](../problems/intake-flushes-single-captures-as-their-own-jev-call-2026-11-15.md)
(on `origin/main`; it holds the counts used here), commit 8b15ec2e9, and the
code named below. I did not have the paper's console log; every count below is
the problem record's, and every number I derive is marked **derived**.

Marks: **C** read from code, **M** measured (by the problem record, not by me),
**derived** computed here from M, **invented** a value with no source.

> **Status after review round 1 (section 5).** A repair round repairs by
> subtraction. Section 5 supersedes: the poll waiting on the label (so also
> the `NextDue` change and the 120 s arrival delay of 4.1), the re-hold of 4.5
> item 3, intake's in-flush whole retry of a non-rate failure, the sleeping
> flusher of 2.2, the lone-capture-only fix of 2.4, and the failure list of
> 2.5. Where a section below disagrees with section 5, section 5 wins. One
> finding of the code contradicts the repair's premise and is **the decision
> brought back** (5.1).

## 1. Investigation

### 1a. What triggers an intake flush, and why 44% hold one capture

**Trigger: a capture arriving, with no wait at all.** A source's poll
(`HandleSourcePollDue`, `source_schedule.go:201`) calls
`enqueueIntakeLabels` (`source_schedule.go:280`; `intake_labels.go:328`), which
appends the poll's new captures to the computer's queue and starts the
runtime's one flusher goroutine if it is not running
(`startIntakeFlusherLocked`, `:372`). The flusher (`runIntakeFlusher`, `:412`)
takes every capture held for the queue whose first capture waited longest and
flushes it (`labelHeld`, `:517`), then loops until nothing is held. This is a
group commit: the first capture is asked at once, and captures that arrive
while a call is in flight ride the next flush (the comment at `:97` to `:103`
says so; "no timer" is the design). Every flush therefore reports
`waited_ms=0` (`:593`): the wait is the time between enqueue and the flusher
waking, a goroutine switch.

**Why 44% are single.** The poll that enqueues also blocks on the flush
(`select { case <-rt.enqueueIntakeLabels(...) ... }`, `source_schedule.go:280`),
so one source cannot enqueue again until its captures are labelled; the only
batching is polls of different sources landing while a call is in flight. A
call is about 150 to 200 ms upstream (our-usage.md 1.1, M for 2 to 8
questions), so the in-flight overlap is a fraction of a second, against polls
that arrive tens of seconds apart (73 flushes in 7,200 s, one every 99 s on
average, **derived**). A source with one new item is one single-capture flush
unless another source's poll lands in that fraction of a second. The 13
twelve-capture flushes are single polls (one source with a full page), not
gathering. So the 32 single-capture flushes are single new items from
sources polled on their own schedules; nothing in the code could have joined
them.

**Where the wait for a label would land, and who reads labels.**

| Consumer | What it does with the label | Reference |
| --- | --- | --- |
| The poll itself | waits for the label before announcing the arrival to management ("so labels exist before the wake"), then enqueues event resolution, announces, and only then records the poll and sets `NextDue = now + interval` | `source_schedule.go:271-294`, `:355` |
| Management | reads `view.Says` and `P` on the arrival record (`captureLabelView`, `intake_view.go:42`); an absent label reads `unlabelled` | `inbox_records.go:130-135`, `:228-232` |
| Editor overrides | when an editor disagrees with a label, the label's `Says` text is stored as `JevSaid` and fed back to later flushes as a "correction" | `intake_overrides.go:110-125`, `intake_job.go:31` |
| Event layer | does not read labels; it resolves the same captures on its own 60 s timer | `event_layer.go:39`, `:944` |

With `intake.events` and `intake.event_wakes` both on, labels retire entirely
(`source_schedule.go:269-271`); the computers measured here are not in that mode
(they flush labels).

**What the deadline can be named from.** The code comment says labels "never
gate the arrival", but the poll does wait for them, so a label wait is an
arrival wait and the next poll's due time moves by the same amount (the tick is
re-armed after the poll, `:355`; no overlap, a drift). No document I found
states a freshness promise per capture. The product facts that exist in code
are: the shortest a source may be polled is one minute
(`minSourcePollInterval`, `source_schedule.go:41`; the default is 15 minutes,
`:40`), and the event layer, the same pipeline's other capture queue, already
holds captures 60 s "so captures from many sources share one call"
(`eventFlushInterval`, `event_layer.go:39`; its 60 s flush is tabulated in
jev-batched-questions-2026-11-10.md section 3). So: **no owner statement names
a deadline.** The one I propose, 60 s, is the finest source cadence the
product offers and the wait the same pipeline already takes, named from those
two constants; it is a code-derived value, not an owner quote, and section 2.2
says what replaces it if the replay refutes it.

### 1b. The three labels written with no Jev call

**There is one path, and it is a default, not a judgment.** In
`labelPollWith` (`intake_labels.go:1080`): for each capture, if it has no
candidate story and no sibling capture, the label is written as
`event=new, p=1.0, model="none"` and the capture is not asked (`:1098-1104`,
constant `intakeNoCandidatesModel`, `:134`). `labelHeld` then appends it like
any other label (`AppendCaptureLabel`, `:577`). Zero live captures means zero
calls and `unlabelled=0`, exactly the three flushes seen.

**How the candidate set is empty.**

1. The story index read failed or timed out. `labelHeld` reads the index under
   one 15 s budget for three reads (`intakeLabelPrepBudget`, `:78`; publish
   `:546`, list `:548`, corrections `:558`). On either index error it logs
   "labelling against siblings only" and goes on with an empty library
   (`:546-549`). A flush of one capture has no siblings, so it has no
   candidates: it gets the default. This is the two logged cases (10:34:26 and
   11:42:34). A flush of two or more captures in the same situation is asked
   against siblings only, a real but impoverished judgment; only a lone
   capture is defaulted.
2. The index is read and genuinely empty or all older than 72 h
   (`intakeShortlistSplit`, `:171`: only stories updated within
   `intakeShortlistHorizon` count; the newest 30 are always candidates once
   inside it). On a fresh computer this is true and the default is honest.
3. The third case (12:11:09) has no logged cause, so no failure line was
   written. The code allows two silent ways: a successful list that returned
   no summaries, or summaries with a zero `UpdatedAt`, which the 72 h filter
   drops (`now.Sub(zero)` is huge, `:174-176`). I do not know which;
   **unverified, a hypothesis**. The default path logs nothing at all, which
   is the defect that made this one invisible.

**Is it a correctness bug? Yes for cases 1 and 3, no for case 2.** What the
label says: management reads "new: there was no story or other new capture to
compare it with" (`intake_view.go:51`), with `P=1`. For case 2 that is true.
For case 1 it is false: stories existed and were not read, and the text says
the opposite, at probability 1. Management is the trusting reader: a capture
of an event the paper already covers is presented as new, which is the
duplicate-story path. Nothing is dropped and nothing is published unjudged by
this alone (the arrival is announced either way), but a default is dressed as
a verdict. Second-order: if an editor overrides it, the stored `JevSaid` is that
text, and the next flushes receive it as a correction to Jev
(`intake_job.go:31`), training the prompt against a judgment Jev never made.
The honest outputs are either an `unlabelled` capture (the existing, visible
state, which management already handles) or a label the log can tell apart
(reason recorded). Section 2.4 fixes it.

### 1c. Every Jev caller, as the code structures it

One entry point exists for jobs (`jevJob`, `jev_job.go`) and one planner
(`planJevCalls`, `jev_split.go:26`) for the rest. Callers by
`scripts/jev-entry-baseline.txt` and SYSTEM1.md 3.4. "Shared state" is what the
code or the measured record says; I did not measure the others, so unmeasured
cells say so.

| Caller | Job (the unit one call serves) | Questions per call | Shared state | Could its questions ride another caller's call? |
| --- | --- | --- | --- | --- |
| Intake labels (`intake_job.go`, `intake_labels.go`) | every capture waiting for a computer (today: whatever is held at that instant) | 2 per capture per part of at most 252 options; 4 for a lone capture over a large index, 24 for 6 | the story index (up to 480 stories, about 100 bytes each) plus the captures; about 23,400 tokens at one capture, **M** | It is the anchor. |
| Event layer (`event_layer.go:312`) | every queued capture, flushed 60 s after the first | 1 per capture, choosing among shortlisted events and siblings | the shortlisted events (30 recent, 10 overlap, 10 corroborated) plus the captures; 3,060 tokens median for 12 questions, M 2026-11-10 | **Yes, the same captures** and a near-copy of the same candidates (every story has an event). Section 3 |
| Support judge (`support_judge.go:369`) | the citation pairs of one revision | 1 per pair, split on the 32k limit | pairs: sentence plus cited passage; unmeasured | No: different units (draft sentences, not captures) |
| Sub-call, desk-cast and inert judges (`subcall_judge.go:351`, `desk_cast_judge.go:222`, `inert_judge.go:152`) | up to 20 children, casts or records of one pass | up to 7 each, 1 each for inert | agent run records; unmeasured | They share one planner and one funnel with each other; they ask about children and casts, not captures |
| `choir.System1` verb (`system1_verb.go`) and story analysis | a desk's ask | at most 12 (our cap, a cell-side bound); 24 questions is 2 calls | refs resolved host-side | Not intake; the cap of 12 is our own limit, not a vendor one |
| Front importance (`frontimportance/scorer.go`) | one story (host door) | 12, packed under a 30,000-byte guard of ours | one story | No |
| Disclosure gate (`disclosure_gate.go:583`) | one egress | 1, fails closed | the egress | No: real time, one decision |
| Prompt router, prompt classifier (`prompt_router.go:82`, `prompt_class.go:58`) | one owner prompt | 1 each, in two calls | the prompt | **Yes with each other**: two calls over one prompt. Interactive, outside intake |
| Op-ed gate (`oped_gate.go:242`) | one op-ed | 4 yes/no | story plus op-ed | No |
| Desk judge (`deskjudge/choice.go:84`) | one event | 1 | the event | Possibly with event resolution, a different job |

The answer to "do several callers ask about the same captures": two do, intake
labels and the event layer. They ask nearly the same thing (which known thing
is this capture a report of, and which sibling) over nearly the same
candidates, once each, 60 s or less apart, on separate calls and separate
contexts. No other pair shares captures.

### 1d. The vendor's limits and what one call holds

From the reference (SYSTEM1.md 3.3, jev-vendor.md section 4), all [V] unless
marked: 64k tokens a request; 32k tokens for the state plus any one question
(the one that binds); 255 options a choice question; **no stated limit on
questions per request**; no published rate numbers; a long-context mode above
about 32,000 bytes on OpenRouter whose cost is unpublished (watched, not
guarded; SYSTEM1.md section 10 item 2). Third-party measurements [T,
jev-vendor.md section 2]: upstream time 73 ms plus 6 ms per thousand input
tokens; a question about 55 tokens; an option about 18. Our latency is flat in
question count (our-usage.md 1.1, M). Our estimator is `jevbudget.Tokens`,
weighted above the tokenizer on purpose (`jevbudget.go`).

**Per capture, derived from two measured facts.** A one-capture flush is about
23,400 tokens and 4 questions (M: the problem record). Twelve captures need
exactly two calls (M: 13 of 13 such flushes). Writing a call as S + n m (S the
stories and shared text, m the per-capture marginal: its entry, its siblings'
entries, and its two to four questions, each listing every option key):

- S + m = 23.4k.
- Twelve do not fit one call: S + 11 m above 64k, so m above 3.7k.
- Twelve fit two calls of six: S + 6 m at most 64k, so m at most 8.1k.

So m is between 3.7k and 8.1k and S between 15.3k and 19.7k (**derived**).
**One call holds between 6 and 12 captures: 12 to 24 questions at two per
capture, up to 48 at four** (**derived**; the code's own comment says "up to
24 questions and 48k estimated tokens", `intake_labels.go:62-66`). The 25
flushes of 2 to 4 captures all fit one call, consistent with this. Two
consequences matter for the design:

1. **Most of a capture's cost is not the shared state.** Per added capture the
   cost is m, 3.7k to 8.1k, because each of its questions repeats its option
   keys; only S, 15.3k to 19.7k, is shared. Gathering saves about S per
   capture that joins another's call; it does not make a capture cheap.
2. **A burst of twelve is already near the call's capacity.** Gathering cannot
   grow a call past 6 to 12 captures; past that it is split by the 64k limit,
   as today, and logged.

## 2. Design: the flush gathers waiting captures into one call

### 2.1 The job, and how calls per job are counted

**The job is "label the captures that arrived for this computer inside one
gather window".** One window, one `jevJob` (it already is: `planIntakeJobCalls`
builds one job per flush), and the job's calls are as few as the vendor's limits
allow (`jevJob.Plan`, which already names the limit behind every split). What
changes is only which captures are in the flush: all those that arrived in the
window, not those that happened to be held at the instant a goroutine woke.

Counting: the flush line already carries `captures`, `calls` (calls actually
made, retries and halves included, `intake_labels.go:593`), `questions`, and
`waited_ms`. The design adds to that line: `sources=<n>` (distinct sources
gathered), `trigger=deadline|stop`, `forced_by=<limit>` when a job needed more
than one call without a failure, `nocall=<n>` (labels written with no call) and
`nocall_reason=<index_unread|index_empty>`. **Calls per job** is `calls` of
one flush line; **calls per capture** over a period is the sum of `calls`
divided by the sum of `captures`; the **single-capture share** is the count of
flush lines with `captures=1` over the count of flush lines. All three read
from the console log with one grep; no new store, no new record.

### 2.2 The mechanism and its deadline

The flusher stops starting a flush the moment anything is held. Instead:

1. The first capture enqueued into an empty queue arms a timer for that
   computer, set to `intakeGatherDeadline` after that capture's arrival. Later
   arrivals do not move it (no sliding window: a steady trickle must not hold
   the first capture past its deadline).
2. When the timer fires, the flusher takes every capture held for that
   computer, in arrival order, and flushes it as one job. If a flush is in
   flight when the timer fires, the flusher takes the queue when that flush
   ends (one flush at a time, as now; never concurrent).
3. A stop flushes at once what is held (`stopIntakeLabels`, `:457`), whatever
   the timer says; `trigger=stop` is logged.

**The deadline: `intakeGatherDeadline = minSourcePollInterval` (60 s).** Its
source is code, not the owner: the shortest source poll interval the product
allows (`source_schedule.go:41`), equal to `eventFlushInterval`
(`event_layer.go:39`). The argument is that a label wait of at most one minimum
poll interval cannot make a capture later than the source's own fastest
cadence, and it is the wait the event layer already takes. **No product
document or owner quote sets it; the value is borrowed, and I mark it so.** It
is a compiled constant, not a setting (CLAUDE.md "No switches").

Whether 60 s is long enough to matter is **not known**, and a back-of-envelope
bound says it may not be. If arrivals were Poisson at the measured rate (one
flush per 99 s), a 60 s window from the first arrival holds a lone capture with
probability exp(-60/99), about 55%, which is no better than today's 44%. Real
arrivals cluster (four flushes within 35 s are in the record), which helps, but
I cannot say by how much without the 73 timestamps. **Before any code, replay
the log:** from the two-hour console log, take each flush's time and capture
count, form windows of W seconds from the first arrival, and compute the
single-capture share and calls per capture for W in {30, 60, 120, 300}. That is
a read-only step and it sets the value. The rule: the deadline is the smallest
W whose replay brings single-capture flushes under 25% and calls per capture
down by at least 20%; **if that W is above 60 s there is no code-named bound
for it and it is an owner decision** (a longer wait is a longer arrival delay
for every source; the jev-batched-questions design said a hold of minutes "is a
product decision, not a free choice"). Conservative default until then: 60 s,
marked **invented-by-borrowing**. **Superseded by section 4.1: the replay was
run on 5 hours of the largest computer's log; 60 s misses the 25% target and
120 s is chosen from that data, pending the owner.** Sections 2.6 and 2.7
below carry the first two-hour numbers; section 4.3 replaces their targets;
section 3 is reordered by section 4.2.

No early fire on a full call is proposed. It would need the plan (a story-index
read and a build) to know when a call is full, or a capacity constant, which
the vendor does not publish and 1d bounds only from 6 to 12; so a burst waits
its window like anything else. This is the cost of having no invented
constant; see open question 1.

### 2.3 Limits this adds, and does not add

The wait is a deadline, not a limit on calls (CLAUDE.md names deadlines as
staying). There is no rate limit, no in-flight cap and no queue of calls: one
flush is one job and its calls are sent in sequence, as today. What changes is
the context per call (more captures, up to the vendor's capacity), and so the
calls per capture.

### 2.4 The no-judgment label, fixed in the same slice

Part 1b's defect sits in `labelHeld` and `labelPollWith`, the same functions
the gather touches, and the proof "no label without a Jev call" cannot hold
unless it is fixed; with gathering, lone-capture flushes become rarer, so the
failed-read case would hide rather than disappear. So it ships in the **same
landing, as its own commit before the gather commit** (orange; rollback:
revert), so a revert of one does not revert the other:

- If the story index could not be read, the flush does not default. With more
  than one capture in the flush it asks against siblings only, as now, and the
  label carries the call's model as every judged label does; with a lone
  capture and no siblings it writes **no label**, leaving `unlabelled`, and
  logs `intake labels: flush: <id> unlabelled: story index unread`.
- If the index was read and the capture still has no candidates, the label is
  the honest default as today, but the log line gives the reason
  (`nocall_reason=index_empty`, with the count of summaries read and how many
  were inside the 72 h horizon), so the 12:11:09 case is explained the next
  time it happens.
- The unlabelled capture is not retried automatically (the existing contract,
  `intake_labels.go:301-304`: nothing resumes it). See open question 2.

### 2.5 Failure list

Each item names a test written first (CLAUDE.md "Proof"; fake clock, the
production gateway client for failures as in `intake_failure_test.go`).

1. **A capture waiting forever on a quiet source.** The timer is armed by the
   first capture and not moved by later ones, so a lone capture flushes at the
   deadline as a one-capture job. Test: one capture, advance the clock to the
   deadline, expect one flush and one call; one capture then a steady arrival
   every 10 s, expect the first flushed at the deadline, not after the trickle
   ends.
2. **A burst larger than one call.** Unchanged and kept: `jevJob.Plan` splits at
   the vendor's limit and the log names it (`jev split ... forced_by`). Test:
   the existing scale test with captures arriving from three sources inside
   one window, expect the same calls and limits as one source with the same
   captures.
3. **A failed call losing a whole batch.** A bigger call loses more captures
   when it fails, but a call is bounded at 6 to 12 captures. A timeout, 429 or
   5xx is asked once more after a backoff and halved at a unit boundary if it
   fails again; a size refusal halves; a 4xx other than 429 or a wrong-shaped
   answer fails once (`jevNextStep`, `jev_failure.go:55`, unchanged). What
   still fails is logged `unlabelled` with its error and stays unlabelled, and
   no default label replaces it. Test: a call that times out twice with 8
   gathered captures; expect the halves asked and, if they fail, 8 captures
   `unlabelled` and none written.
4. **A silent default label.** Fixed by 2.4. Test: index read fails with one
   capture, expect no label written and an `unlabelled` line; index read fails
   with three captures, expect a call against siblings; index empty with one
   capture, expect a `none` label and `nocall_reason=index_empty`.
5. **Ordering.** Captures are flushed in arrival order across sources, so
   handles `n1..nk` follow it; each source's own order is preserved; labels are
   appended in handle order; each poll's wait handle resolves when its own
   captures are written (`intakeTicket`, `:249`), not when the whole flush
   ends. Test: two sources interleaved, expect per-source order preserved and
   each ticket closing after its captures.
6. **The same capture held twice.** A longer wait raises the chance that a
   replayed wake enqueues a capture already held. Dedupe by `CaptureID` at the
   gather; the second enqueue's ticket resolves with the first. Test: the same
   capture enqueued twice inside a window, expect one question set.
7. **Restart mid-wait** (CLAUDE.md "Restarts"). The queues stay in memory; a
   crash drops what is held and those captures are unlabelled, exactly as a
   failed label; **no boot step resumes them and no marker is written**. A
   planned stop flushes what is held at once (2.2 step 3) within the drain
   deadline. The crash-loop bound counts a poll in flight at a crash
   (`PollInFlightSince`, `source_schedule.go:226`), so the wait lengthens the
   exposure of each poll by up to the deadline; a source whose polls crash the
   computer twice is closed, which is the intended behavior. Test: stop with
   held captures, expect one flush with `trigger=stop`; a simulated crash,
   expect no label and nothing resumed at the next boot.
8. **Latency seen by publication.** Each poll's announcement to management is
   later by at most the deadline plus the call (0.2 to 1 s upstream, M and T),
   and the source's next tick drifts by the same. If the event layer is on, its
   own 60 s timer starts after the labels, so event resolution is later by the
   same wait. This is the cost of the change and the reason the deadline is a
   product question. Test: a flush's `waited_ms` is at most the deadline plus
   the flush in flight at the time.
9. **A flush in flight when the deadline passes.** The next flush starts when
   the first ends, with the captures held then; never concurrent. Test with a
   blocked first call.
10. **A poll whose context ends while it waits.** The poll returns; its
    captures stay held and are labelled with the rest (existing behavior,
    `source_schedule.go:279-283`); the label is written, unread by that poll.
    Test: cancel the poll's context inside the window.
11. **Several computers.** One timer per computer queue; the one flusher
    serializes flushes, oldest deadline first (as it orders now). Test: two
    computers with different first arrivals.
12. **Larger calls and the call deadline.** `intakeCallTimeout` already scales
    with estimated tokens (5 s plus 1 s per thousand, about 69 s at 64k,
    `intake_labels.go:1438`) and is unchanged. A call that takes much longer
    than the 0.5 s the third-party latency model predicts for 64k would show
    in `upstream_ms` before it timed out; the flush line carries
    `est_tokens_max` for it.

### 2.6 Cost per capture, before and after

Before, measured over the two hours (M, the problem record): 73 flushes, 83
calls (57 one, 13 two, 3 none), about 263 captures (32 singles, 13 x 12 = 156,
25 flushes of 2 to 4 taken at 3 = 75; the 25 are my assumption, so the total
is 247 to 281). Calls per capture is about 0.32 (0.30 to 0.34). A
single-capture call costs 23.4k tokens.

After, as a function of what the replay finds, not a promise: every capture
that joins another's call saves S, 15.3k to 19.7k tokens, and saves one call.
Worked case: if the 32 singles were gathered in pairs, calls fall by 16 (83 to
67, 0.32 to 0.25 per capture, a 20% fall) and tokens by 245k to 315k; if
gathered in fours, calls fall by 24 (0.23), tokens by 370k to 470k. Both are
**derived scenarios**, not predictions. The dollar side is small (Jev input is
about $0.042 per million tokens, jev-batched-questions-2026-11-10.md); the
point is fewer calls per job, which is the owner's stated rule.

### 2.7 Proof

Tests first, in this order, each failing before the code: the twelve above in
2.5; then the mechanism. Landing per CLAUDE.md: the touched package's whole
uncached suite (`internal/agentcore` through `scripts/go-test-runtime-shards`
or the build VM), the packages that import it, `scripts/check-jev-entry` (the
entry-point count must not rise), two seats (red surface: provider calls), and
`Ring-Expedite: intake sends one Jev call per capture because a flush fires at
the first capture`. Calibration: enter the claims below in
`docs/calibration/claude-ledger.md` with their probabilities before the push.

**Deployed proof, read-only from the largest computer's console log, over two
hours after the deploy** (the same window length as the baseline):

1. Single-capture flushes fall from 44% of flushes to **at most 25%**.
   Probability: at 60 s, 0.35 (the Poisson bound above says it is not
   assured); at the replay-chosen W, 0.7. Settle against the flush lines.
2. Calls per capture fall from about 0.32 to **at most 0.26**, a fall of a
   fifth or more. Probability at 60 s: 0.4.
3. **Zero** flush lines with `nocall>0` and no `nocall_reason`, and zero labels
   written after a read error (`story index unread` lines against labels).
   Probability: 0.9 (it is a code path, not a rate).
4. `waited_ms` at most the deadline plus one flush, on every flush line.
5. No flush line with `calls` above 1 and no `forced_by`, unless a failure
   line precedes it.

Check 3 is the only pass or fail; 1 and 2 are scored claims, and the replay in
2.2 is done first so that they are not hill-climbed after the fact (CLAUDE.md
"Understand before tuning").

### 2.8 Rollback

Orange, red surface (provider calls). Rollback is `git revert` of the gather
commit, which restores group commit with no timer; the no-judgment fix is a
separate commit and stays on a revert of the gather. No data migration: labels
are append-only objects with the same shape. Nothing about a deploy refresh or
a restart changes (the queues are memory only, before and after).

### 2.9 Open questions, with conservative defaults

1. **The deadline.** Default 60 s, borrowed from `minSourcePollInterval` and
   `eventFlushInterval`. A longer one is the owner's, with the replay's
   numbers. An early fire on a full call is not added; revisit once the logged
   capacity (`forced_by` with its capture count) is known.
2. **Retry an unlabelled capture in the live process** (after a failed index
   read, once, at the next window). Default: no; it stays `unlabelled`, which
   management already sees. A retry in a live process is not a boot resumer, so
   CLAUDE.md "Restarts" allows it, but it adds a state labels do not need today.
3. **Whether the poll should wait at all.** The poll holds its arrival for
   labels; the code comment says labels never gate it. Decoupling would let the
   deadline be longer with no arrival delay, but it changes what management
   sees on first read (an unlabelled capture). Default: unchanged, flagged.
4. **The 12:11:09 cause.** Default: leave it a hypothesis until the logged
   `nocall_reason` appears in staging.

## 3. The next slice, from the callers table (named, not designed)

**Merge the event layer's questions into the intake job.** Intake labels and the
event layer ask about the same captures with the same kind of candidates and
the same siblings, on two timers 60 s or less apart and two contexts. Once
intake gathers inside a 60 s window the two flushes are aligned in time; the
event questions (one per capture, over the shortlisted events) can then ride
the label job as another question group over the same captures block, one call
per window instead of two.

Estimate (**derived**, rough): the event layer flushes at most once a minute,
so at most 120 calls in two hours, and it makes no call for a minute with no
capture; the labels made 83 calls in two hours. If the event layer's calls are
near the labels' flush count (73), merging removes the smaller of the two
families, **on the order of 40% of the intake family's calls** (roughly 35 to
60 of 150 to 160). It is bounded by the same call capacity: a merged call holds
fewer captures (the event questions add to m), so bursts split sooner.
Uncertainties to read before designing it: the event layer's real call count in
the same two hours (its `resolved N captures in M calls` lines are the count
and I did not have them); whether label and event candidates can share one
candidate list (events are not stories: a story's event is one of several);
and that on a computer with `intake.event_wakes` on, labels retire and the
event layer is the only job, so the merge pays only where both run. The
stronger alternative the code comments already point at is to retire labels in
favor of the event layer on the largest computer (a setting the code has);
that is an owner decision, outside this slice.

Behind it, in order of calls saved and ease: the prompt router and classifier
(two calls over one owner prompt, one call), then the three sub-call judges
(one funnel, three calls a pass). Neither is designed here.

**Reordered by section 4.2:** the event-layer merge is slice 3. Slice 2 is the
per-capture option lists, which set how many captures a call holds.

## 4. Amendment: the replay, capacity, and the four-call flushes

Data: the coordinator's read-only extract of the largest computer's flush
lines, 14:04Z to 19:07Z on 2026-11-15 (224 lines, 212 flushes; 5.0 h). It
replaces the two-hour baseline of section 2.6 (which was 44% single and 0.32
calls per capture; this window is worse).

**Baseline, measured (M):** 212 flushes, 653 captures, 258 calls, 0.40 calls
per capture; 113 single-capture flushes (53%); calls per flush: 0 for 3, 1 for
174, 2 for 30, 4 for 4, 8 for 1.

### 4.1 The replay, and the deadline chosen from it

The replay forms a window from the first arrival (later arrivals do not move
it) and counts calls as ceil(captures / capacity). It assumes no failures.

| Window | Jobs | Single-capture jobs | Calls per capture at 6 per call | At 9 per call |
| --- | --- | --- | --- | --- |
| today (no wait) | 212 | 53% | 0.40 (measured, failures included) | |
| 30 s | 147 | 34% | 0.29 | 0.27 |
| 60 s | 126 | 31% | 0.27 | 0.24 |
| 120 s | 82 | 10% | 0.22 | 0.19 |
| 300 s | 47 | 4% | 0.20 | 0.15 |
| 600 s | 26 | 0% | 0.18 | 0.13 |

My Poisson bound in 2.2 was right to doubt 60 s: it leaves 31% single, above
the 25% rule I set before looking. By the rule of 2.2 (the smallest window that
gets singles under 25% and calls per capture down by a fifth) the answer is
120 s: singles fall from 53% to 10%, calls per capture from 0.40 to 0.19 to
0.22. The 0.40 includes failures, which the replay leaves out, so the like for
like fall is smaller (4.3). From 120 s to 600 s calls per capture fall only
from about 0.2 to 0.13 to 0.18: past 120 s the limit is capacity per call, not
gathering (4.2).

**Chosen: `intakeGatherDeadline` = 120 s, chosen from this replay, pending the
owner's word.** It is no longer borrowed from a code constant, because 60 s
does not meet the target; it is two minimum poll intervals and has no product
source beyond the replay. A value past 120 s is not recommended: the extra
saving is small and the delay doubles.

**What 120 s costs downstream (from 1a):**

- Every poll that carries captures waits up to 120 s plus the call before it
  announces to management (`source_schedule.go:279-283`, announce after). That
  delay is paid whether or not the window finds company.
- The next poll moves by the same wait. `now` is taken after the labels
  (`source_schedule.go:334`) and `NextDue = now + interval` (`:355`), so the
  cycle of a source with captures is interval + wait. At the default 15
  minutes that is 13% slower; at the minimum 60 s it is three times slower
  (about 180 s): a source that asks to be polled every 60 s would be polled
  every three minutes whenever it brings something new. I cannot say how many
  sources are set under two minutes; the extract has no source intervals.
  **Unverified.**
- The smallest correct decoupling is not to unblock the poll but to stop the
  wait moving its next tick: set `NextDue` from the poll's start, with the
  present end time as a floor (`max(now, started + interval)`; `started` is
  already in hand in `HandleSourcePollDue`). Then a source whose interval is at
  least the wait loses no cadence, and a shorter one polls as fast as its poll
  finishes, never less than once per wait. It is a small orange change, its own
  commit, with a test for a poll that outlasts its interval. Without it, the
  largest deadline that leaves cadence alone is 0, because any wait is added to
  the cycle; so I recommend 120 s together with this change. The alternative,
  a shorter deadline, costs singles: 60 s leaves 31%.
- Unchanged: the poll still holds its ticket for the wait; a crash in it counts
  against the crash-loop bound (failure item 7), now for up to 120 s.

### 4.2 What fills a call; slice 2

**Measured per-capture cost.** From the 174 one-call flushes in the extract,
mean `est_tokens_max` by captures in the flush: 1: 28.1k (106 flushes), 2: 34.1k
(43), 3: 39.9k (15), 4: 47.9k (4), 5: 53.7k (4), 6: 56.7k (2). The slope is
about **5.7k estimated tokens per added capture** (4 questions each) over a
**fixed part of about 22.4k**. This replaces my derived range of 3.7k to 8.1k
in 1d with a measured value. Twelve captures need two calls at about 57k to
64k each. Capacity at the 64k limit, by the estimator, is (64 - 22.4) / 5.7 =
about **7 captures a call**; the estimator is weighted above the real tokenizer
(`jevbudget.go`), so the vendor's own count may allow more; I did not measure
real input tokens.

**What the 5.7k is.** A capture whose options exceed 252 (the window's stories
plus the other captures) is asked in two parts, each an event question and a
topic question (`intake_job.go`, the parts loop). A choice question's options
are the keys of its `criteria` map, `"c123":null,` each, about 10 tokens
(`intakeOptionTokens`; `intakeCriteria`, `intake_labels.go:929`). So a
capture's questions list about N story keys twice (event, topic) and its
siblings once; for N of about 250 that is about 5k of the 5.7k. The rest is the
capture's own entry and the question text. (This is a derivation from the code
and the slope; I did not count tokens in a real request.)

**Are those lists the same stories repeated per capture? Yes.** Every capture
is compared with the whole recent window (`v.order` starts with every `recent`
handle, `newIntakePoll`, `intake_labels.go`) and only its own up to 10 overlap
picks differ (`intakeShortlistOverlap`). The lists are the same N keys, written
again in each capture's questions. They are not repeated in the state: each
story entry is in `state.stories` once (the fixed 22.4k).

**But the repetition cannot be removed by stating the candidates once and
referring to them.** The context already does that. It does not reach the
question, because the vendor's choice question has only the `criteria` map and
its keys are the options (jev-vendor.md section 3; SYSTEM1.md 3.1: "the vendor
has only the `criteria` keys"); the reference pages show no way to say "the
options are the handles in state.stories". I did not test the vendor. So the
lever is the number of key lists per capture, not where the stories are
stated. **Slice 2, named and sized, not designed:** cut the per-capture option
lists, in the order the reference allows:

1. The topic question repeats the whole story list to learn which story shares
   the capture's topic. If the topic can be taken from the event question's own
   `probabilities` (a distribution over the same stories), or from a short
   list, the marginal falls by about half, 5.7k to about 3k, and capacity from
   about 7 to about 14 captures a call: twelve captures fit one call and calls
   per capture at a 120 s window fall from about 0.2 toward 0.1. Whether the
   topic label is worth its list is a product question (management reads
   `TopicHeadline`, `intake_view.go:42`).
2. Shorter handles: `"c123"` costs about 10 tokens because digits count a
   token each; shorter keys would save perhaps a fifth of the list, about 1k a
   capture, capacity about 8.

Estimate (**derived**, rough): captures per call 7 now; about 14 without the
topic list; about 9 with shorter handles alone. This is **slice 2, ahead of
merging the event layer** (section 3, now slice 3): it raises the capacity that
bounds every window, where the merge adds the event questions to a call that
is already near full.

### 4.3 Proof targets, redone from the replay

Baseline: 53% single, 0.40 calls per capture (5 h, M). Window 120 s, capacity
7 (between the replay's 6 and 9, about 0.21 calls per capture):

1. Single-capture flushes at most **15%** (replay 10%). Probability 0.7: the
   replay runs the window over the real arrivals; the remaining error is that
   gathering shifts when polls run.
2. Calls per capture at most **0.28**, a fall of 30% from 0.40 (replay 0.19 to
   0.22 without failures). Probability 0.65. The 0.40 includes 24 calls spent
   on seven captures that all failed (4.4).
3. Zero labels without a call and unexplained (unchanged), and no flush line
   with more than one call unless a vendor limit is named (4.4 changes what
   counts as a named reason).
4. `waited_ms` at most 120 s plus one flush on every flush line, and no
   source's poll cycle stretched past its configured interval plus one call if
   the `NextDue` change of 4.1 lands (readable from the poll lines).

### 4.4 Flushes with four or eight calls: a defect

Five flushes in the extract made 4 or 8 calls (14:09:35, 17:57:37, 19:04:48,
19:05:50, 19:06:53). **All five are fully unlabelled** (`unlabelled` equals
`captures`): four are one capture with `calls=4 questions=12`, one is two
captures with `calls=8 questions=32`. None is a split by a vendor limit. From
the code:

- A one-capture flush has four questions in two units (two parts).
  `jevNextStep` (`jev_failure.go:55-78`) sends any failure that is not a rate
  refusal, a size refusal or a deterministic 4xx to: retry once (`retries < 1`,
  `:72`), then halve (`canHalve`, `:74`), with `forced_by="call_failed"`. The
  calls are the first (4 questions), its retry (4), then the halves (2 and 2),
  each failing and, at one unit, given up: 4 + 4 + 2 + 2 = 12 questions in 4
  calls. The two-capture flush: 8 + 8, halves 4 + 4, halves of those 2 + 2 + 2
  + 2 at depth 2: 32 questions in 8 calls. Both match the logged counts
  exactly. The loop is in `labelPollWith` (`intake_labels.go`, the `work`
  loop), with `jevHalvingDepth = 2` (`jev_failure.go:40`).
- That is what commit 8b15ec2e9 says it does ("any other failure is asked once
  more, then halved"). **It contradicts the rule it was written for** ("a
  second call is right only when the vendor's limits refuse the first, and the
  code says which limit"): a halve forced by "call_failed" is not a vendor
  limit, it is a guess that the failure was size. A halved call sends the
  shared state again in each half, so a failing one-capture flush spends about
  4 x 27k = 110k estimated tokens to end unlabelled, and a failing two-capture
  flush about 8 x 31k = 250k.
- Cost: 24 of the 258 calls (9%), about 19 beyond one per flush, spent on
  seven captures that got no label. They sit in three clusters (14:09, 17:57,
  19:04 to 19:06), which suggests upstream conditions, not size.
- **The evidence, read by the coordinator from the paper's console
  (read-only):** every one of the 13 Jev failure lines today is
  `dial tcp <host>:8084: connect: connection refused` to the gateway, logged as
  `jev split: site=intake_labels items=4 calls=2 forced_by=map[call_failed:1]`
  (also one `site=event_layer items=2` and some support-judge lines). The three
  clusters are the three gateway restarts today: 14:09 and 17:57 (deploys) and
  19:02 to 19:07 (a credentials restart that drained for 4.5 minutes). So the
  halving happened on a transport failure where **no request reached the
  gateway, let alone the vendor**; a half cannot succeed where the whole cannot,
  and each half only adds a doomed call. (My reading of the extract is now
  confirmed; I did not read these lines myself.)

**Defect to name:** `jev_failure.go:72-76` (and the header comment's rule
"any other failure is asked once more after a backoff ... then halved",
`jev_failure.go:27-30`): halving on `call_failed`.

### 4.5 The settled fix: its own commit, ahead of the gather commit

**Rule: a failure that is not a size refusal is never halved.** Halving stays
only for the two size signals the file already names, `jevSizeRefused` (the
text hints) and a gateway 413 (`vendor_refused`, `gateway_body_cap`). The
`case canHalve: return jevHalve, "call_failed"` arm and the loop's
`call_failed` log are deleted; `jevNextStep` then has no path that halves
without a named vendor or transport size limit, which is what "a second call
only when a vendor limit refuses the first, and the code says which" asks.

**Connection refused (and any dial failure to the gateway): no immediate
retry, no halving.** No request left the guest, so asking again at once is a
second doomed call inside the same second; the gateway is ours and is being
restarted, so this is not a provider refusal and the allowance to back off
after "a provider's real refusal" (CLAUDE.md) does not apply. What exists in
code for backing off is the rate path: `jevRateRefused` (429, 529) with
`jevRateTries = 2` and `intakeRetryBackoff` (2 s, then 4 s,
`intakeRetryBackoff`, `intake_labels.go`). That path is for the provider's own refusal and
stays as is. For a transport failure I add no limiter and no new wait; the
wait is the gather deadline. Concretely:

1. A new classification `jevTransportDown(err)`: the error is a dial or
   connect failure (a `*net.OpError` with `Op == "dial"`, or the text
   "connection refused" as the file's other checks are text-based) and no
   typed HTTP status was received. `jevNextStep` returns `jevFail` for it with
   no `forcedBy`, on the first failure, at any depth.
2. The flush logs one line per failed call, as it does for `jevFail` today,
   naming the error, and its captures are counted `unlabelled`.
3. **The captures are re-held once, in the live process, for the next gather
   window** (intake only; held in memory like any queued capture; the same
   deadline, 120 s, is the back-off, no new timer and no new limit). A second
   failure leaves them unlabelled, visible to management as `unlabelled`. A
   crash drops them as it drops every held capture, and no boot step resumes
   them (CLAUDE.md "Restarts": a re-hold in a live process is not a boot
   resumer). With gathering the wait is at most one deadline, so a 20 s
   restart is recovered and the 4.5 minute drain of 19:02 to 19:07 is not
   (two windows fail, then unlabelled); I accept that over a loop with no
   bound. Their wait handles were resolved when the first flush ended, so the
   poll has already announced them unlabelled; the label, appended later as a
   new version, is what management reads at its next record read.
4. The event layer shares `jevNextStep` (`event_layer.go:480`) and so gets the
   no-halve rule and the no-immediate-retry for transport failure; its own
   handling of what then fails (captures left unresolved, not retried) is
   unchanged. The support judge does not call `jevNextStep` (only intake and
   the event layer do); its `connection refused` lines are outside this fix and
   I did not read its retry path, so it is a named residual.

**One residual, named.** The header comment says the gateway answers any
upstream failure, a vendor size refusal included, with 502, so the caller
cannot tell a size refusal from another 502; today the halve on `call_failed`
was the only cure for a vendor size refusal returned as a 502. The estimator is
weighted above the real tokenizer and a call is split before it is sent, so a
true size refusal should be rare; I have not seen one in the log, and the
evidence above is zero size refusals and 13 transport failures. If one appears,
the cure is for the gateway to relay the vendor's reason as a 413 or 422 (a
gateway change, red, its own slice), not a guessed halve. Until then a 502 is
asked once more whole (`retries < 1`) and then fails unlabelled.

**Class and reason: red.** It changes how calls to the provider path fail,
which is "gateway and provider calls" in CLAUDE.md's red row, and it is shared
by two callers (intake and the event layer). A defect here would be silent: a mis-classified failure that is
never retried, or a size refusal no longer halved, loses captures with only a
log line. So two seats, a conjecture delta, protected surfaces (the provider
call path), the evidence above as evidence class, rollback by revert, and a
heresy delta (the old rule was written in 8b15ec2e9 and is reversed here).
Subject ends `(red; rollback: revert)`. It lands **before** the gather commit
and does not depend on it: the gather makes each failed call lose more
captures, so the failure policy has to be right first. The re-hold (item 3) is
orange and rides the gather commit, since it needs the window.

Failure item 3 in 2.5 is amended to read: "a failed call is asked again whole
once only when the failure is a rate refusal, a transport-level 5xx or a short
body; a dial failure is not asked again at once; halved only on a size refusal;
what still fails is unlabelled, and re-held once when it was a dial failure."
Proof tests for it, written first: a dial error never produces a second call
and never a halve (the production gateway client against a closed port, as
`jev_gateway_test.go` does); a 413 still halves; a 502 is asked once whole; a
flush whose call is refused by a closed port re-holds its captures once, and
the second refusal leaves them unlabelled with one log line each. Deployed
proof: after the next gateway restart, the paper's console shows one failed
call per flush, no `forced_by=map[call_failed` line, and the held captures
labelled at the next window if the gateway is back. **(Re-hold, the rest of 4.5
item 3, and "asked once more whole" in the amended item 3 are withdrawn by
5.3.)**

## 5. Review round 1: the repaired design

Two seats read the design: omp-deepseek-v41-flash (REPAIR, five blockers) and
claude (ACCEPT with conditions). A reproducible minority blocker outweighs the
pass, so this is a repair round, by subtraction: no new mechanism. The
coordinator's adjudication is applied below, each point checked against the
code at `718ea5ed4`; one point it does not survive (5.1).

### 5.1 The poll no longer waits for the label, and the one thing that needs it

**Adjudication:** the poll announces its captures and enqueues the event layer
without waiting for the gather window; the label arrives later as advice
(`source_schedule.go:262-277`: "Labels are their own objects and never gate the
arrival"; `yaegikernel/intent.go:290-293`: "advice, not a decision").

**What changes in code (all verified):**

- `source_schedule.go:279-283`: the `select` on the enqueue's channel is
  deleted; the poll calls `enqueueIntakeLabels` and goes on to
  `enqueueEventResolution` (`:294`) and `announceCaptures` (`:305`).
- `intakeTicket`, `newIntakeTicket`, `resolveIntakeHeld`, the `ticket` field of
  `intakeHeld` and the channel `enqueueIntakeLabels` returns are deleted
  (`intake_labels.go:249-286`, `:328`). **Nothing waits on a ticket any more,**
  so a wedged queue (5.4) can lose a label but can block no poll, no lane slot,
  no tick and no hibernate fence.
- Deleted with it, because they existed only for the wait: the `NextDue =
  max(now, started + interval)` change and its commit (d); the 120 s arrival
  delay; the cadence cost on sources with an interval under the wait (the poll
  returns as soon as it has announced); the hibernate-fence cost (a poll is in
  flight for its fetch, not the window); the one-source-computer delay (the
  poll is not held; a lone source gets its label at the next window, no
  worse); the event layer's timer starting up to 120 s late (`:294` is no
  longer behind the wait); and the lost-arrival-on-restart defect of DeepSeek
  B1 (a crash mid-window now loses a *label*, never an arrival: the arrival is
  committed in the poll's own path before the poll ends, `source_schedule.go:305`).

**What reads the label, and when (the check the adjudication asked for):**

| Reader | Reads | What it does at announce time | With the label up to 120 s later |
| --- | --- | --- | --- |
| `announceCaptures` (`capture_arrival.go:130`) | the capture ids only | nothing about labels | unchanged |
| `enqueueEventResolution` (`event_layer.go:916`) | the captured items only | nothing about labels | unchanged |
| Management's record read (`inbox_records.go:130-135`) | the label at the time of each read, `LatestCaptureLabel` (`intake_view.go:31`) | shows `unlabelled` if none yet | a read inside the window says `unlabelled`; a later read shows the label (labels are fresh per read) |
| Override pass (`intake_overrides.go:110-125`) | the newest label when an editor disposition commits | skips a capture with no label | a disposition inside the window records no correction, since no label existed (not a wrong one) |
| **The intake task's opening turn** (`management_tasks.go:107-124`, via `loadCellRecords` and `captureLabelViews`) | the labels at the moment the task starts | **the arrival's own commit starts the task** (`capture_arrival.go:108-116`, `withIntakeTask`), so the task's first turn is built at announce time | **the first turn shows `unlabelled` for every gathered capture** |

**This last row contradicts the adjudication's premise that the label is only
documented advice.** It is advice, but the platform feeds it to the intake
task in the task's *first turn*, which is built within the arrival's own
commit, and the task prompt says "do not fetch what came" and "Feed the story
Jev names unless you read otherwise"
(`rlm_management_task_runtime.yaml:12-24`; the management overlay,
`rlm_management_runtime.yaml:39-40`: "the task ... with the labels in front of
it"). With the poll decoupled, a gathered label (arriving up to 120 s after the
announce) is not in the task's first turn. The task decides open, feed or
decline without the advice it was written to follow, unless it calls
`choir.Records()` again, which its prompt does not tell it to do. I read no
test of how often a task re-reads, and I did not measure task latency, so how
much this costs is **unverified**; but it is the one thing the label is for.

**So the repair, as ordered, cannot be taken without a decision, and I bring
it back rather than keep the block:**

- **(A) Decouple as ordered.** All of 5.1's deletions land. The label becomes
  late advice. The intake task's first turn shows `unlabelled` for any capture
  whose label has not landed; the gather window then makes that the normal case
  (today it is the failure case). Management's read inside the window and the
  override pass degrade as in the table. This keeps the design a pure
  subtraction. Whether the first turn matters is the owner's call and needs
  one measurement first: the share of intake tasks that call `choir.Records()`
  a second time, or that feed against the label.
- **(B) Keep the poll's wait**, so the label exists at the task's first turn,
  with every cost that returns: the 120 s arrival delay, the poll in flight
  for the window (hibernate fence, crash-loop exposure, a lane slot), the
  `NextDue` drift (review: B1 lost arrival on a mid-window crash unless the
  captures are persisted first, which is a new mechanism). I do not design
  these here; they are in DeepSeek B1 and Claude 2, 4, 5.

**Default taken (CLAUDE.md "no blocking asks"):** the design below is written
for (A), because it is the subtraction ordered and the only one that adds
nothing; and because the gather's gain (calls per capture) is independent of
whether the poll waits. The conservative reading for the *product* is (B), and
I record that I would not land the gather commit until the coordinator or the
owner has chosen. Landing (a) and (b) (5.6) does not depend on the choice.

### 5.2 The deadline under (A)

120 s stays the value chosen from the replay (4.1), pending the owner's word.
Under (A) it costs no poll anything; it costs only label lateness, which is
the 5.1 question. Under (B) it would cost every poll 120 s. The deadline is
unchanged; what it costs depends on the choice above.

### 5.3 Failures: what is deleted

- **Re-hold deleted** (DeepSeek B4; Claude 7). A capture whose call failed on a
  dial error stays unlabelled; management has it as `unlabelled`, the arrival is
  already announced, and nothing is written. Measured yield of the re-hold:
  about 7 captures in 5 h, all during gateway restarts (the 24 calls of 4.4:
  three clusters, the three restarts), and none recovered by a 20 s retry
  because the drain of the credentials restart lasted 4.5 minutes. The
  override-against-a-label-nobody-saw hazard (B4) and the double-version hazard
  go with it. The `jevTransportDown` classification stays in (a): a dial
  failure fails once at any depth, never halved and never retried at once.
- **Intake's in-flush whole retry deleted.** `jevNextStep` returns `jevRetry`
  for any non-rate, non-deterministic failure when `retries < 1`
  (`jev_failure.go:72-73`). On the intake path that is a second call with no
  named limit: it is the same request again, not a rate backoff and not a size
  split. At the call site (`intake_labels.go:1154-1160`) a `jevRetry` on an
  error that is not `jevRateRefused` becomes `jevFail`; the rate backoff
  (`intakeRetryBackoff`, `jevRateTries`) stays, being a real provider refusal.
  The event layer calls `jevNextStep` itself (`event_layer.go:480`) and keeps
  its retry as it is; the change is in intake's loop, not in `jevNextStep`, so
  there is no per-caller parameter and no new mechanism.
- **Not changed (5):** halving on a size refusal (`vendor_refused`,
  `gateway_body_cap`) stays as (a) says. **Residual, recorded, not designed:**
  the gateway answers a vendor size refusal with a bare 502
  (`internal/gateway/jev.go:433`, the reason only in the journal at `:417`), so
  nothing halves it; the estimator is above the real tokenizer and no size
  refusal appears in the log (13 of 13 failures were `connection refused`).
  Relaying the vendor's size refusal as a 413 is a **separate later slice,
  outside this design**.

### 5.4 The timer: the pattern the repo already has

DeepSeek B2 and B3 and Claude 1 are the same finding from the code:
`stopIntakeLabels` waits for the flusher goroutine (`intake_labels.go:486`
`l.flushes.Wait()`) and flushes only while `ctx.Err() == nil` (`:490-492`);
the only cancel is the drain deadline (10 s default,
`telemetry_batching.go:34-39`). A flusher *sleeping* on a gather deadline would
make a planned stop wait the full drain deadline and then skip the flush, so
every held label is lost on a planned refresh. And the enqueue starts the
flusher only `if !l.running` (`:364`) while the flusher exits when nothing is
due (`:424-425`), so a deadline armed only in that branch is never armed for a
queue enqueued while another computer's flush is in flight.

The repair uses the shape the event layer already has (`event_layer.go:944`
`time.AfterFunc` per queue, `:957-968` start under the lock, `:1018-1029` stop):

- **No sleeping flusher, no `running` flag.** The one flusher goroutine and
  `l.running` are deleted. Each queue carries `timer *time.Timer` and
  `inFlight bool`.
- **Arming rule** (independent of anything else): after any change under
  `l.mu` (an enqueue, the end of a flush), if the queue is non-empty and not
  `inFlight` and has no `timer`, arm `time.AfterFunc(max(0, items[0].at +
  deadline - now))`. A queue whose oldest capture is already past its deadline
  (it waited behind another flush) fires at once.
- **On fire:** under `l.mu`, `timer = nil`; unless stopped or `inFlight`, set
  `inFlight` and start the flush goroutine, which takes the queue's captures in
  arrival order. At the end of the flush, under the lock, `inFlight = false`
  and the arming rule runs again, so captures that arrived during the flush
  get a deadline measured from their own first arrival. A runtime-level mutex
  held across a flush keeps today's serialization: a flush for one computer
  does not run beside a flush for another (the owner's "no concurrent Jev
  calls"); it is a lock, not a queue of calls.
- **Liveness invariant:** every non-empty queue has either a flush in flight or
  a pending timer. Tests assert it after every transition: enqueue into an empty
  queue; enqueue while another computer's flush is in flight (the DeepSeek B3
  interleaving); a flush ending with captures still held; a flush that panics.
- **Panic recovery** (`intake_labels.go:380-392` today): the recovery sets
  `inFlight = false` under the lock and runs the arming rule, so the held
  captures of that queue are not stranded; the panicking flush's own captures
  stay unlabelled, as now.
- **Stop and a planned restart** (`stopIntakeLabels`, `:457`): under the lock,
  every queue's timer is `Stop()`ped and the held captures taken (as `rest`
  already is); the stop then flushes them at once, bounded by the drain
  deadline, after the in-flight flush finishes. The test asserts **labels
  written** (`AppendCaptureLabel` called for the held captures with a fake
  judge), not just a log field. A crash writes nothing, and nothing resumes at
  boot: no marker, no resumer (CLAUDE.md "Restarts").

### 5.5 (b): every batch size

DeepSeek B5: `(b)` as written fixed only the lone capture; with an unread
index, a flush of two or more captures is asked against siblings only
(`intake_labels.go:546-549`, an empty library) and writes `new` or a sibling
label with Jev's model and probability, rendered "Jev: new (p ...)"
(`intake_view.go:49-59`), indistinguishable from a full judgment. Verified.
With a gather window, multi-capture flushes become the normal shape, so the
defect would have moved from rare to common. **Repair, by subtraction: when the
story index could not be read (the publish or the list errors at `:546-549`),
every capture of that flush stays unlabelled, lone or not.** The "siblings
only" path is deleted for that case; `labelHeld` sets a flag at the two error
sites and returns each capture with its error, as it does for any capture it
could not label, and logs once `story index unread; N captures unlabelled`. No
new label field. A read that succeeds and returns no usable stories (the
`index_empty` case) keeps the honest default, with the reason in the flush log
line as 2.4 said.

### 5.6 Land order, each landable and revertible alone

1. **(a)** the failure rule: delete the `call_failed` halve
   (`jev_failure.go:72-76`), classify a dial failure as `jevTransportDown`
   (fails once, never halved), and turn intake's non-rate `jevRetry` into
   `jevFail` (5.3). Red, two seats.
2. **(b)** every capture of an unread-index flush stays unlabelled (5.5).
   Orange. Independent of (a).
3. **(c)** the gather: the AfterFunc timer (5.4), the deadline, the deleted
   ticket and the decoupled poll (5.1, if (A) is chosen). Orange, red surface.
   It does not need (a) or (b) to build, but it should follow them: a gathered
   call that fails loses more captures. There is no `(d)` and no
   `jevTransportDown` dependency in (c), since the re-hold is gone, so
   reverting (a) alone cannot break the build of (c).

Tests that pin the old contract and are **rewritten, not deleted**, in (a):
`intake_failure_test.go:39` (`TestIntakeGateway502OnABigCallIsHalvedAndLosesNoCapture`:
a 502 is now asked once at most, never halved, ends unlabelled),
`:66` (`...PersistentFailureIsAFixedSmallNumberOfCalls`: the fixed number is now
one call for a dial failure, and the rate path's tries for a 429), `:111` with
`:127` (`TestJevNextStepOrdersTheChecks`: the `jevHalve ... "call_failed"`
assertion becomes a `jevFail`; the 413 and 429 orderings stay). In (c) the
tests that wait on the ticket (`intake_flush_test.go`, `intake_flush_safety_test.go`)
are rewritten for the AfterFunc and for no wait; I have not read them, so their
exact assertions are unverified.

### 5.7 Corrections of fact adopted, and the proof targets

- The label wait is a cost only under (B); under (A) it is label lateness.
- The first replay data of 4.1 stands. Proof target 4 now reads: **`waited_ms`
  at most the deadline plus one flush in flight**, since a queue can wait
  behind another computer's flush (DeepSeek, should-fix).
- Proof targets 1 to 3 of 4.3 stand (single-capture flushes at most 15%, calls
  per capture at most 0.28, zero labels with no call and no reason). Added for
  (b): zero labels written in a flush that logged `story index unread`.
- The `index_empty` default stores `EventP = 1.0`
  (`intake_labels.go:1102`); the view prints no probability for it but the
  `P` field carries 1. Left as is, recorded as a residual (not a blocker; a
  field change would be the new mechanism this round avoids).
- `jevSizeRefused` is dead weight through the gateway today (its own comment,
  `jev_failure.go:92-95`); kept, because the gateway slice that relays the size
  refusal will use it.
- Claude's read was on `main` (824a01aab), not on the reference page; it did
  not check the vendor limits. DeepSeek's B1 frequency (70% of refreshes in a
  wait) assumed the blocking shape and is moot under (A).
- Claude 6, "after a dial failure, do not send the rest of the flush's calls":
  adopted into (a) as a deletion of attempts (a dial failure of one call stops
  the flush's remaining calls; they would fail the same second).

### 5.8 Failure list, recounted (14)

1. A capture waiting forever on a quiet source: the timer is armed by the first
   capture and not moved; test with a fake clock, including a steady trickle.
2. A burst larger than one call: split by `jevJob.Plan`, logged with the limit.
3. A failed call: a dial failure fails once, never halved, never retried at
   once, and stops the flush's remaining calls; a non-rate failure is not
   retried on intake; a rate refusal backs off as now; a size refusal halves;
   what fails stays unlabelled, no default, no re-hold.
4. A flush with an unread story index: every capture unlabelled, any batch
   size (5.5).
5. Ordering: arrival order across sources; per-source order preserved; labels
   in handle order.
6. The same capture held twice: deduped by `CaptureID` at the gather.
7. A planned stop or restart: timers stopped under the lock, held captures
   flushed at once within the drain deadline, labels asserted written; a crash
   drops held captures unlabelled, no resumer.
8. The liveness invariant: every non-empty queue has an in-flight flush or a
   pending timer, asserted after enqueue-while-another-computer-flushes, after
   a flush ending with captures held, and after a panic.
9. Panic recovery re-arms the queue (5.4).
10. A flush in flight when a deadline passes: the next starts when it ends, one
    at a time across computers (the runtime-level mutex).
11. Several computers: one queue and one timer each, serialized.
12. Label lateness: `waited_ms` at most the deadline plus one flush in flight;
    under (A) no poll, tick, lane slot or fence is held, and no wait handle
    exists to wedge; the intake task's first turn may show `unlabelled`
    (5.1, the decision).
13. Larger calls: `intakeCallTimeout` scales with the request (about 69 s at
    64k, against the gateway's 2 minutes, Claude 3); a timeout of a big call
    ends unlabelled and its `est_tokens` is on the failure line.
14. A dial failure inside a multi-call flush stops the rest of the flush (3,
    5.7), so a closed gateway costs one call per flush.

(Dropped from the earlier list: the poll whose context ends while it waits and
its ticket, since nothing waits; the re-hold items.)

### 5.9 Every finding, and its disposition

DeepSeek (REPAIR):

| Finding | Check | Disposition |
| --- | --- | --- |
| B1 restart mid-window loses the arrival | true of the blocking shape: announce at `:305` is after the wait at `:280` | removed by 5.1 (poll no longer waits); a crash now loses a label only |
| B2 timer makes the planned-stop flush impossible | true: `intake_labels.go:486`, `:490-492`, drain 10 s | repaired by 5.4 (AfterFunc, stop under the lock, labels asserted written) |
| B3 liveness invariant unstated; `!l.running` at `:364` wedges a queue | true as described | repaired by 5.4 (no `running`, arming rule, invariant, test) |
| B4 re-hold feeds a wrong learning signal | true (`intake_overrides.go:110-125`, `intake_job.go:31`) | re-hold deleted, 5.3 |
| B5 (b) fixes only the lone capture | true (`:546-549`) | repaired by 5.5 (every capture of an unread-index flush) |
| Relay the vendor's size refusal | gateway `jev.go:417`, `:433` | separate later slice, recorded residual, 5.3 |
| One-source computers: window is pure delay | true | the poll no longer waits; the label is no worse than the next window; not adding a flush-at-once rule (a new mechanism) |
| Delete intake's in-flush retry | true (`jev_failure.go:72-73`) | adopted at intake's call site, 5.3; event layer's kept |
| `jevSizeRefused` dead weight | true | kept for the gateway slice, 5.7 |
| Rewrite the pinning tests | `intake_failure_test.go:39`, `:66`, `:111`/`:127` | listed, 5.6 |
| Rollback dependency of the re-hold on (a) | true | gone with the re-hold, 5.6 |
| `(d)` leaves a 60 s source at 130 to 180 s | true | `(d)` deleted (5.1) |
| Proof target 4 | true | corrected, 5.7 |
| Record the residual; drop `EventP=1.0` | | residual recorded in 5.7 and for the problems doc and ledger at landing; `EventP` left |
| Direct answer 1: not a forbidden queue | agree | none |

Claude (ACCEPT with conditions):

| Finding | Check | Disposition |
| --- | --- | --- |
| 1 stop must wake the sleeping flusher | same as DeepSeek B2 | 5.4 |
| 2 poll parked holds the hibernate fence (`handler.go:189-193`, `hibernate.go:107-113`) | not read by me | removed by 5.1; unverified |
| 3, 4 land (d) first; implement it as `max(now, started+interval)` | | `(d)` deleted (5.1) |
| 5 event layer's timer starts late | true of the blocking shape | removed by 5.1 |
| 6 stop the flush's other calls after a dial failure | | adopted, 5.7 and failure 14 |
| 7 re-hold details | | re-hold deleted, 5.3 |
| (a) acceptable, a 502 size refusal is a residual; timeout class lost halving | | accepted; est_tokens on the failure line, failure 13 |
| `(b)` safe: readers handle no label | `intake_view.go:31`, `intake_overrides.go:111-115` | confirmed; the intake task's first turn is a reader of the label **it missed** (5.1) |
| "Keep the current default" on open question 3 | | the adjudication ordered the opposite; the code finding in 5.1 is why this is the one decision returned |

**Contradicted by the code:** the premise of adjudication point 1, as stated in
5.1 (the intake task's first turn reads labels at announce time). Nothing in
either review was contradicted except Claude's closing preference to keep the
poll's wait, which is a choice, not a fact, and its read on `824a01aab`.

