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
  19:04 to 19:06), which suggests upstream conditions, not size. **What the
  failure was, I do not know:** the extract has the flush lines only, not the
  `call of N questions failed` lines that carry the error. Read those lines at
  the three times before choosing the fix.

**Defect to name:** `jev_failure.go:72-76`, halving on `call_failed`. The
amendment to the design: a failed call is asked again whole once (same state,
same questions), and halved only when the failure is a size refusal
(`vendor_refused`, the gateway 413), as the owner's rule allows. A timeout of a
call far under the limit is not evidence of size; whether a timeout of a big
call is, is a question for the error lines. This is a fix in `jev_failure.go`
and its tests, its own commit, red (provider calls), ahead of the gather
commit: gathering makes each failed call lose more captures, so the retry
policy has to be right first. Failure item 3 in 2.5 is amended to read "asked
once more whole; halved only on a size refusal; what still fails is
unlabelled".
