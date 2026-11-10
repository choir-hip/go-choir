# Intake at the desk's pace: sourcing scales, language-model work stays flat (2026-11-10)

Status: design, not built. Mutation class of this document: green.
Rollback: revert. The build it describes is orange (management task
selection, arrival minting) with a revert rollback.

## The owner's statement (2026-11-10)

- "We should decouple the sources from the AI model processing, because we
  could be scaling up the sourcing faster while keeping the language model
  and System 1 model usage constant."
- "We can scale up Jev usage as long as we don't scale up the language
  model usage. Especially because we're making Jev much more efficient."
- "We shouldn't be processing captures on a first-in, first-out basis,
  because then that will just make us increasingly be behind the news. We
  should be sampling more stochastically, weighted towards what is brand
  new."
- "Freshness and importance are key. Having a little bit of stochastic
  sampling is also important, to keep ourselves honest that the importance
  score is correctly calibrated."

## What happens today

Read from code at `origin/main` 2b780e257 and from the gateway log
(20:46 to 22:46Z); the investigation is summarised here and each number says
where it is from.

1. A source poll holds one of four per-computer slots
   (`internal/actorruntime/adapter.go:53,170`) through fetch, capture
   write, System 1 labelling and the arrival commit
   (`internal/agentcore/source_schedule.go:201-348`).
2. Every poll with a new capture commits an arrival and, in the same
   batch, mints one management intake task (`arrival_task.go:93`). Arrivals
   are not coalesced.
3. Intake tasks run four at a time (`management_tasks.go:50-55`). The rest
   queue. The start loop takes the newest queued task first and expires
   tasks older than 24 hours (`management_tasks.go:258-340`).
4. On 2026-11-09 the paper had 1,763 queued tasks, the oldest 70 hours old,
   growing by 11 an hour (evidence:
   `docs/evidence/token-efficiency-2026-11-09/intake-and-jev.md`).

So language-model work is already nearly flat, because the task slots are
full. The cost is that most captures are never read, and which ones are
read is decided by arrival order alone. Scaling sourcing today would
lengthen the queue, not raise the quality of what is read.

System 1 work grows with captures: about 665 calls and 1,900 questions an
hour for the paper (log). The owner accepts this growth.

## The frame

The queue of tasks is the wrong object. A task is a unit of desk work; a
capture is a unit of news. Minting a task per arrival makes the desk's work
list a copy of the arrival stream, and then throws most of it away by age.

The desk has a fixed amount of attention per hour. The question each time a
slot frees is "what is the best use of this slot right now", asked over
everything unread. That is a selection over captures, made at the desk's
pace. Arrivals do not need to mint anything.

## Design

### 1. An arrival is a record, not a trigger

The arrival commit keeps naming its captures and stops minting a task. The
record already has a `held` flag that means this
(`capture_arrival.go`, `arrival_task.go`). Per-arrival minting is deleted.

### 2. Every capture gets an importance score from System 1

One more question on the intake label call, answered with a probability:
would the desk act on this capture (open or advance a story). It rides the
same call as the existing label questions, so it adds questions, not calls.

The score is trained against nothing at first; it is a zero-shot question
and is expected to be imperfect. Section 4 is how it is checked. The
existing event-corroboration probability is not used as the score: the
2026-11-09 evidence found it predicts publication poorly.

Question wording follows `docs/reference/SYSTEM1.md` (Patterns): one direct
question, a choice form rather than yes/no/unclear, and the probability
kept soft (no threshold).

### 3. When a slot frees, the desk selects

A selector runs when a task slot frees and when an arrival lands while a
slot is free. It is not a timer. It considers every capture that is unread
and not leased to a running task.

Each capture has a weight:

```
weight = importance × 0.5 ^ (age / half_life)
```

Selection is in two parts.

- **Most slots take the top of the weighting.** The selector fills a task
  with the highest-weight captures (up to the existing 12 per task),
  grouped so one task holds captures that belong together where the label
  says so.
- **A small share is drawn at random.** With probability `explore` the
  selector instead draws the task's first capture at random in proportion
  to weight, from the whole unread set. This is the honesty check: it reads
  captures the score would have passed over.

Every selection records the capture's importance, age, weight, whether it
was a top pick or a random draw, and the probability it had of being drawn.
That record is what makes the outcomes scorable later.

There is no queue of tasks and no 24-hour expiry. An old capture is never
deleted from consideration; its weight decays until it is effectively never
drawn. Nothing accumulates except the captures themselves, which are
already on the tape.

### 4. Keeping the score honest

The random draws are a fair sample of what the score ranks low. For each
selected capture the desk's outcome is already recorded (story opened,
advanced, published, or closed with nothing). Two readings follow.

- **Calibration.** Among captures scored near p, did about p of them lead
  to desk action? Read from the random draws, which are not biased by the
  score.
- **Lift.** Do top picks lead to action more often than random draws? If
  not, the score is not earning its place and the question is rewritten.

A tracked proof script reads the selection records and prints both. The
same records are the training set for a small on-device or encoder model
later (`docs/reference/system1/dual-process.md`).

### 5. The two numbers

- `half_life`: set from measurement before the build ships. The quantity to
  measure on the paper is the delay from capture to the desk action it led
  to, for captures that did lead to one; the half-life starts at about the
  median of that delay. Until measured, this design names no value.
- `explore`: small, because the owner asked for "a little bit". The build
  starts at one task in ten, and the calibration reading says whether the
  sample is large enough to judge the score within a day.

Neither is a switch. Both are constants in the code with the measurement
that set them in a comment, changed by a commit.

### 6. The poll stops waiting for labels

Separately, the label stage commits the arrival after its flush, and the
poll returns once its captures are written. A slot is then held only for
fetch and write. This needs the pending-arrival retry list to move from the
schedule record to the label stage (`source_schedule.go:318-337`). It is
the second build, after sections 1 to 4, because it only matters when the
four slots bind, and that has not been measured yet.

## What this deletes

- Per-arrival task minting.
- The queued-task backlog, newest-first start order and the 24-hour expiry.
- Later: the poll's wait on its labels and the schedule's pending-arrival
  list.

## Rules it must keep

- **Restarts.** Unread captures are data, not obligations. A crash closes
  the tasks that were running, with the visible fate. Nothing is re-minted
  at boot; the next slot that frees, or the next arrival, runs the selector
  as new work. Source schedules are standing intents and survive as today.
- **No limits on our own model calls.** Language-model work is bounded by
  the desk's task slots, which exist today. No rate limiter is added. If the
  desk should do more, the slot count changes in code.
- **System 1.** The importance question rides the existing label call. One
  serial call stream; no concurrent calls.
- **No switches.** The selector replaces the start loop; it does not sit
  beside it.

## Ways it can fail

Written before the build.

1. The importance question is uninformative. Then top picks are no better
   than random draws; the lift reading shows it within days, and until it
   is fixed the selector behaves as freshness-weighted sampling, which is
   still better than today.
2. A burst of fresh, low-value captures (one prolific source) crowds out
   everything. Importance is in the weight to prevent this; if it still
   happens, the grouping step caps captures per source per task.
3. A capture is selected twice. The lease index that guards this today is
   kept; the selector reads it.
4. The selector scans every unread capture each time a slot frees. At the
   paper's volume (about 188 new captures an hour on 2026-11-09) and a
   half-life of hours, the live set is thousands. If the scan is slow, it
   keeps an in-memory index rebuilt at boot from the tape, which is derived
   state, not a resumer.
5. No slot ever frees because tasks hang. Unchanged from today; task
   deadlines bound it.
6. An arrival lands while all slots are busy and nothing wakes the selector
   later. Slot completion is the wake; a test covers "arrival while full,
   then a slot frees".
7. The random draw picks a very old capture and the desk wastes a slot.
   That is the price of the check, bounded by `explore` and by the decay.

## Unknowns and the probe for each

| Unknown | Probe |
| --- | --- |
| Poll durations; whether the four slots bind | the schedules route's `PollMS` and `TotalMS` on the paper |
| Today's queued-task count and age | the paper's management task state |
| Delay from capture to desk action | join captures to story opens on the paper's tape |
| Share of the paper's language-model calls that are intake tasks (inferred at about 110 of 1,300 an hour) | match prompt sizes to desks in code, or read run roles |
| Whether desk outcomes are recorded per capture in a form the proof can read | read the override recorder and story records |

## Precommitment

Entered in `docs/calibration/claude-ledger.md` when the build starts: that
top picks lead to desk action at least twice as often as random draws
within three days of the paper running the selector, and that the paper's
language-model calls per hour stay within 15% of the week before while
captures read by the desk shift toward fresher ones.
