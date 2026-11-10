# Activation, wake and dispatch waits: an inventory and a hypothesis (2026-11-09)

Status: investigation only. No timer was changed. Owner, 2026-11-09: "we have
to get rid of all wait times"; then: "it's not something we can remove
immediately. We need to really investigate them ... holistically, not just
try to remove timers ad hoc, because we should be making these changes while
improving the architecture or maintaining it."

Mutation class: green (docs). Rollback: revert.

## 1. Problem and evidence

d@e (computer-59c902c5d46caecc0a180a8557164e72), 2026-11-09 00:27:31Z:

    dispatcher: activation wait agent=texture:0987cfca-... waited=2m49s updates=4 slots=2/8 due_agents=2

The owner's prompt-bar request was at 00:24:41. Read as "the Texture desk
waited 2m49s to activate with free slots", this looks like a scheduler wait.
The trace does not say that.

Run e28256e4 events (GET /api/runs/e28256e4-.../events), the run for that doc:

| seq | time (Z)     | event |
|-----|--------------|-------|
| 1   | 00:24:44.178 | loop.submitted |
| 2   | 00:24:44.575 | texture.agent_revision.started |
| 3   | 00:24:44.708 | loop.started |
| 4   | 00:27:29.837 | loop.progress, `provider_call_ms=164932` |
| 6   | 00:27:30.839 | texture.document_revision.created |
| 11  | 00:27:31.136 | activation.passivated |

Facts the trace establishes:

- The run was submitted 3 s after the request and started 0.5 s after that.
  The desk activated at 00:24:44, not at 00:27:31.
- The turn's only model call took 164.9 s, on
  `provider=openrouter model=recursive/flint-v1.0:free`. That is 2m45s of the
  2m49s. The same log shows run 90c960b5 (same free model) with a 2-minute
  gap between tool-loop iterations 2 and 3 (00:24:28 to 00:26:30) in the same
  window. Both stalls are consistent with a slow free-tier provider, not a
  local scheduler.
- `model call:` is logged after the response returns
  (internal/toolregistry/toolloop.go:1156), so the 00:27:29 line is the end of
  the call, not its start.

What the "activation wait" line measures
(internal/actor/dispatcher.go:694-724, `logActivationWait`): at the start of
an activation, now minus the oldest due update's `max(CreatedAt, NotBefore)`.
It is called from `activate()` (line 487) after the activation snapshots its
backlog. It says how long these occurrences sat in the actor log, never why.
At 00:27:31 it fired, so a dispatcher activation for `texture:0987cfca`
began at 00:27:31, 2m49s after its oldest of 4 due updates (about 00:24:42).

### Hypothesis (not confirmed by the log): the serial fence

H1. The Texture run executes synchronously inside a dispatcher handler. The
handler calls `ExecuteActivationSyncChecked`
(internal/actorruntime/handler.go:580, 647, 710, 727), which runs the whole
turn, model call included, before `HandleUpdate` returns. The dispatcher holds
`running[agentID]` for all of it (dispatcher.go:355-377, the
serial-per-actor fence). Occurrences for the same agent that arrive after the
activation took its snapshot (initial_dispatch's reconcile wake, inbox-record
wakes, coagent results) are skipped by `dispatchPending` as busy and drained
by the next activation only after the 2m47s turn ends. That next activation
logged the wait. slots=2/8 fits: this activation plus one other desk (run
90c960b5), with 6 free.

Why the log cannot confirm it: `dispatchPending` skips a busy, held or
slotless agent silently (dispatcher.go:357-361). Nothing records which of the
three reasons applied or which activation held the fence. The 124-line
recent_log window has no dispatcher line for the first activation. The
"recovered raced activation" line (texture_agent_revision.go:748, 00:24:44)
only shows that the lifecycle's initial_dispatch wake and the API's submit
raced and the submit lost.

H2 (alternative). The activation that ran the turn took the initial_dispatch
in its snapshot, and the 4 later updates are redundant wakes for the same run.
Under H1 and H2 the delay the owner saw is the provider call, and the 2m49s
is a misattributed queueing metric.

H3 (alternative). `activate()` blocked in the SQLite reads at its top (Epoch,
LoadSnapshot, Unprocessed, dispatcher.go:473-486) behind a writer. The
capsule spawn at 00:22:36-00:24:04 (1m28s, `computer_store` phase 1m26s)
ran on the same box in that window. A 2m45s block is unlikely but nothing
rules it out; it needs a timing line.

A second wait in the same log is real: `engineering-admission` waited 1m41s
with `slots=1/1` (a lane of one) behind a delegated-assignment spawn saga
that held the lane's only slot for 1m28s inside the handler
(actorruntime/handler.go:401, "wake latency"). Its second update (8230be02, a
`boot` wake for the same assignment) waited 1m31s behind the first. That is
the same fence-by-synchronous-handler shape, and there the log does confirm
the cause (`capsule ... spawn total=1m28.034s`).

Per CLAUDE.md, "review the traces before attributing behavior": H1 stays a
hypothesis until the dispatcher records why it skipped a due agent.

## 2. Inventory

Scope: every wait between "a commit makes work due" and "the work is
running", in internal/actor, internal/actorruntime, internal/agentcore and
internal/textureowner. "Event exists" means a signal already fires at the
moment the wait would end.

### 2.1 Dispatcher (internal/actor/dispatcher.go)

| # | Wait | Trigger | Bound | Job | Event that exists | Depends on it |
|---|------|---------|-------|-----|-------------------|---------------|
| D1 | `time.After(PollInterval)` in `Run`, line 338 | idle loop | 250 ms default (197, 252) | crash and missed-signal backstop: re-read the projection | `Notify()` on `Send` (actor.go:191) and at every activation end (line 379) | a missed `Notify` costs at most 250 ms |
| D2 | due timer, lines 306-325 | `NextDue` of an update with `not_before` | exact (the schedule) | scheduled events (source ticks, deferrals, `not_before` wakes) | none needed: it is the schedule | scheduled wakes, deferral re-fire |
| D3 | `heldUntil` retry hold, lines 386-420; `handlerRetryBackoff` 36-45 | handler error | 250 ms doubling to 10 s, jitter 0.5-1 | rate-shape retries of a failing handler; process memory only | the handler's failure | MaxAttempts (8) poison bound |
| D4 | `DeferUpdate` not_before, lines 556-572; `deferralBackoff` 20-29 | `ErrDeferUnprocessed` | 500 ms doubling to 30 s; MaxDeferrals 64 | "wait for an out-of-band wake"; stops hot looping | sometimes (see A1, A2) | every gate and transient-failure deferral |
| D5 | serial fence `running[agentID]`, lines 355-377 | an in-flight activation of the same agent | the handler's whole duration, up to the run's wall | serial-per-actor ordering | activation end calls `Notify()` | handlers run whole turns inside it |
| D6 | slot cap, `hasRoomLocked` 443-452; `SetSharedSlots` 148-155 | all shared slots busy | the longest running activation | bound goroutine fan-out | activation end calls `Notify()` | cost, memory on 4 GiB guests |
| D7 | lanes (engineering-admission 1/1) | lane full | handler duration | keep I/O-bound actors off the desk slots | activation end calls `Notify()` | spawn saga ordering |
| D8 | snapshot at activation start, lines 473-486 | updates arriving after the snapshot | one full activation | fenced-commit consistency | none until the activation ends | exactly-once incorporation |

D5 and D8 are the shape behind H1. They are not timers. A turn is a
synchronous call inside a handler, so the mailbox is unavailable for the
turn's duration even though the occurrences that queue behind it carry no
work the running turn does not read itself ("the wake carries no work: the
desk reads its records in (cursor, head] when its turn runs", handler.go
`handleInboxRecord` comment).

### 2.2 Handler and adapter (internal/actorruntime)

| # | Wait | Where | Bound | Job | Event that exists |
|---|------|-------|-------|-----|-------------------|
| A1 | `applyHoldsWork` defer | handler.go:182-186 | D4 backoff, up to 30 s | self-development apply checkpoint | the apply checkpoint ending |
| A2 | hibernate fence `EnterWork` | handler.go:188-192; agentcore/hibernate.go:165-185 | fence `until` | host snapshot of the guest | `lifted` closes (the wait itself is event-driven), but the handler turns a failure into a D4 deferral, which adds up to 30 s |
| A3 | "execute initial activation" defer | handler.go:710-713 | D4 backoff | any failure of the synchronous turn becomes a deferral | none: a failure |
| A4 | Texture and records wakes defer on error | handler.go:529, 553, 564, 592, 648 | D4 backoff | transient lookup failures | none: failures |
| A5 | synchronous turn inside `HandleUpdate` | handler.go:580, 647, 710, 727 | the whole turn | the cause of D5 | n/a |
| A6 | boot dispatch retry | adapter.go:459 | 50 ms loop until `Send` succeeds | boot-time append retry | none: polls a failing append |
| A7 | `ensureSurfaceWithRetry` | adapter.go:701 | fixed delay, attempts | boot surface bootstrap | none |
| A8 | engineering admission wake latency | handler.go:401 | spawn saga, about 88 s here | delegated assignment spawn | the lane slot freeing |

### 2.3 Wake outbox projector (internal/agentcore)

| # | Wait | Where | Bound | Job | Event that exists |
|---|------|-------|-------|-----|-------------------|
| W1 | audit timer | actor_wake_projector.go:21, 96-99, 111 | 60 s | exposes a missed store signal | `store.ActorWakeSignal()` is the primary driver |
| W2 | retry backoff | actor_wake_projector.go:20, 53-67 | 1 s doubling, 5 attempts, then exhausted | failed `dispatch` | the failure |
| W3 | resweep pacing | actor_wake_projector.go:22, 101-103 | 50 ms | 4 dispatches per desk and 64 per pass (runtime.go `sweepPerTargetLimit`, `sweepDispatchBudget`) | none; self-clocked |
| W4 | announced vs full listing | runtime.go:3105-3125 | none if announced; full listing was 2-4 s on the paper | avoid listing every row ever written | `TakeAnnouncedActorWakes` keys |
| W5 | `wakeLatencyLogAfter` | runtime.go:3100, 3195 | 2 s log threshold | observability only | n/a |

The wake outbox is a second queue in front of the dispatcher's own log: a
commit mints an outbox wake, the projector turns it into an actor-log `Send`,
and the dispatcher turns that into an activation. Three hops, three signals
(store signal, `Notify`, activation end), each with a backstop timer (W1
60 s, D1 250 ms).

### 2.4 Run start, Texture hand-off and neighbours

| # | Wait | Where | Bound | Job |
|---|------|-------|-------|-----|
| R1 | raced activation recovery | textureowner/texture_agent_revision.go:739-780 | no timer; two paths start the same run | `StartRunWithMetadata` loses to the lifecycle's wake; the loser adopts the winner's run. Not a wait, but proof that two paths reach an activation |
| R2 | activation budget deadline | agentcore/activation_deadline.go | per-activation budget, hard wall | bound a runaway turn; parks suspend it |
| R3 | engineer review park | agentcore/engineer_reuse.go:339-341 | `reviewParkMax`, `reviewParkRecheck` ticker | management's verdict on a candidate |
| R4 | SSE tickers | textureowner/texture_observation.go:341-360; agentcore/api_trajectory.go:197-220 | 1 s tick, error backoff to 10 s, 15 s heartbeat | UI streams; not on the activation path |
| R5 | judges | agentcore/support_judge.go:60 (2 min queue wait); subcall_judge.go:109, 171, 248, 444, 630-640; desk_cast_judge.go:129 | seconds to 2 min | batching and retry of judge verdicts; off the activation path |
| R6 | event layer flush | agentcore/event_layer.go:37, 831 | 60 s or a full chunk | coalescing event writes; delays event visibility, not activations |
| R7 | platform update retries | agentcore/platform_update.go:807, 1010 | 250 ms steps | retry polling around self-development apply |
| R8 | checkpoint and snapshot cadence | checkpoint_cadence.go:390-399; snapshot_producer.go:78-79 | 3 min initial, 10 min tick | persistence cadence |
| R9 | model provider latency | toolregistry/toolloop.go:1156 | unbounded within the activation budget | the model call: 164.9 s in the d@e case |

## 3. How the waits relate

1. The commitment ledger and the records inboxes are the event source. A
   commit mints an outbox wake (W), the projector sends it to the actor log
   (D), the dispatcher activates the agent. The signal chain
   store signal -> `Notify` -> activation exists end to end. D1 (250 ms) and
   W1 (60 s) are its backstops and fire only when a signal was lost; the
   audit logs "missed signal" when it finds work. Legitimate. Keep as the
   crash and lost-signal backstop.

2. D2 (due timer), D3 (retry hold) and W2 (outbox retry) wait on time
   itself: a schedule or a retry shape. Legitimate. One caveat: D3 and W2
   back off every failure the same way, whether or not the thing waited for
   has a release event.

3. D4 is the one real symptom of a missing event. A deferral says "wait for
   an out-of-band wake" and arms a clock (backoff to 30 s). A1 (apply
   checkpoint) and A2 (hibernate fence) wait on conditions that clear at a
   known instant with an event; A3 and A4 are failures with no event. A1/A2
   are the only waits in the inventory where an existing event could end the
   wait and does not.

4. D5, D6, D7, D8 and A5 are one substrate shape: a handler runs the whole
   turn synchronously, so the dispatcher's unit of exclusion (the
   activation) is the same as the unit of work (a multi-minute turn).
   Everything queued for that agent, and for the engineering lane, waits for
   the turn. For Texture the queued occurrences are mostly wakes that carry
   no work, so what is held back is bookkeeping, not the user's request. For
   engineering-admission the queued item is a real saga step. This shape also
   explains why the 2m49s line looks like a scheduler fault when the dominant
   cost is the model call.

5. R1 shows two writers can start the same activation: the API's
   `StartRunWithMetadata` and the lifecycle's wake. One path would delete the
   race and its recovery code. This is architecture, not a timer.

6. R5, R6 and R8 (judges, event flush, checkpoint cadence) are coalescing
   and cadence timers off the activation path. They are not the cause of this
   report; they delay event visibility and the judges hold model capacity.

Net: the signal chain is already event-driven (D1 and W1 are backstops). D4's
clock on A1/A2 is the removable clock. The serial fence around synchronous
turns (D5, D8, A5) is the structural wait, and it is what the 2m49s line may
have shown.

## 4. The physics

Floor: an activation should start within one scheduler turn of its
triggering commit when a slot is free. Components:

- commit to outbox signal: one in-process channel send, microseconds;
- outbox projector sweep for one announced key: a keyed read, low
  milliseconds (W4); the unannounced full listing was 2-4 s on the paper;
- `Send` append to the actor log (SQLite WAL write): about 1-5 ms;
- `Notify` to `PendingAgents` and goroutine start: one grouped query over
  `actor_updates`, milliseconds;
- activation prologue (`Epoch`, `LoadSnapshot`, `Unprocessed`): three SQLite
  reads, milliseconds.

So the floor is tens of milliseconds. Observed on d@e: 3.7 s from the prompt
(00:24:41) to `loop.started` (00:24:44.7), including the HTTP request and
lifecycle commits. Backstops bound the lost-signal case at 250 ms (D1) and
60 s (W1). Where the time went in the d@e case:

| Segment | Time | Share |
|---------|------|-------|
| request to `loop.started` | about 3.7 s | 2% |
| provider call (free model) | 164.9 s | 98% |
| authoring and commit | about 1.3 s | under 1% |

The dispatcher's own view (four queued updates, H1) does not add to the
owner's wait. The engineering lane's 1m28s is I/O: the capsule
`computer_store` phase linked 3,261 files (34 MB) in 1m26s on the same box,
and that contends with anything touching the disk.

## 5. Design space, cheapest to dearest

1. Make the waits visible (no behavior change). Record in the dispatcher why
   a due agent was skipped (busy fence, hold, no room), for how long, and when
   the holder started; add `cause=` to the activation wait line, so "queued
   behind own in-flight activation" is told from "no slot" and "retry hold".
   This decides H1/H2/H3 on the next occurrence and removes the
   misattribution. Keeps the architecture, adds a field.

2. Event-driven deferrals (small). Where a deferral waits for a condition
   with a release event (A1 apply end, A2 `lifted`), the release clears the
   `not_before` and calls `Notify` instead of waiting out D4. The 30 s cap
   stays as the crash backstop. Deletes A1/A2 latency, keeps D4 for failures.
   Needs a `KernelLog` method to re-arm deferred updates (interface and test
   fakes change). Improves the architecture: one wake path, clock only as
   backstop.

3. One path to activation (medium). Remove the second starter so that the
   lifecycle commit, observed by the dispatcher, is the only thing that
   executes the run (or the reverse). Deletes `recoverRacedTextureActivation`
   and the class in
   docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md.
   The Texture hand-off must stop executing inline.

4. Collapse the outbox hop (medium to dear). The outbox wake and the
   actor-log `Send` are two queues for one fact. A commit that mints a wake
   would append the actor-log row in the same transaction, or the dispatcher
   would read the outbox as its pending projection. Deletes W1-W3, the
   audit-found-missed-signal path and the announced/full split; D1 becomes
   the only backstop. Needs one transaction or one DB: a substrate change.

5. Fence by work, not by turn (dearest). Run turns outside the activation:
   the handler incorporates the occurrence and starts the turn on a run
   goroutine under the run's own lease; wakes for a resident run fold into
   the run's inbox cursor and never queue behind it. D5/D8/A5 stop being
   coupled to turn length, and the engineering lane of one is no longer
   forced by handler duration. It changes the delivered-equals-incorporated
   invariant and crash semantics, so it needs its own design and drills.

Recommended order: 1 first (it decides H1 and is cheap), then 2 and 3 as the
"one wake path" cleanup, then 4. Option 5 only if H1 is confirmed and
no-op wakes behind a turn turn out to delay real work. All five keep the
sweep (D1, W1) as the only crash and lost-signal backstop, and none adds a
switch or a timer. What each deletes: 1 nothing; 2 the A1/A2 latency; 3 the
raced-activation recovery; 4 the projector's retry, audit and pacing
machinery; 5 the serial-fence coupling and the lane special case.

## 6. Open questions

1. Which activation held `texture:0987cfca`'s fence from 00:24:44 to
   00:27:31, and what were the 4 queued updates (kinds, creation times)?
   Needs the `actor_updates` rows for that agent or the option 1 line.
2. Is Texture's first turn on the owner's path using a `:free` model by
   design on d@e? The 164.9 s provider call dominates every other number.
   Model routing is outside this doc, but it decides whether scheduler work
   is visible to the owner at all.
3. How often does a deferral hit its backoff when the release event had
   already fired (A1, A2)? Counting `dispatcher: deferred ... deferrals=N`
   lines by cause on the paper would size option 2.
4. Does a wake for a resident, running Texture run ever need delivery before
   the turn ends? The records-inbox contract says the turn reads its inbox in
   its next cell, which suggests no and is the premise of option 5. Confirm
   against the Texture inbox code and the B4/B8 breakage notes first.
5. Does the outbox hop (option 4) provide anything the actor log lacks
   (durability, idempotency keys, pre-boot interruption)?
6. Is the engineering-admission lane of one a correctness requirement (saga
   ordering per assignment) or a resource guard? A per-assignment fence would
   let unrelated assignments run.
7. Do the judges' waits (support_judge 2 min queue) ever hold a shared slot
   while a desk activation is due?


## Investigation 2026-11-09 evening

Read-only: dispatcher code at origin/main 0ebab72a7 (`internal/actor/dispatcher.go`
last changed 2026-10-26, ce0b41d93, so nothing here is fixed or instrumented
since this record), and d@e's `recent_log` at about 01:30Z (build 03300ad5f).

### Is there an action? Yes: one, small, and it is option 1

New evidence for H1, from d@e tonight. The log shows four activation waits for
one agent, `management:c061de6f`:

| time (Z) | waited | updates | slots | due_agents |
|---|---|---|---|---|
| 00:50:38 | 3m15s | 29 | 1/8 | 1 |
| 01:04:16 | 8m16s | 33 | 2/8 | 2 |
| 01:21:52 | 54s | 42 | 2/8 | 2 |
| 01:26:15 | 2m23s | 80 | 2/8 | 2 |

Slots in use never exceed 2 of 8, so D6 (no free slot) is excluded. The
retry hold (D3) is for failing handlers and nothing in the log shows a failure
for this agent. What is left is the serial fence, D5: management is inside one
long synchronous activation (a delegated engineering assignment was running
at tool-loop iteration 91-104 in the same window; the log does not show which
activation held management's fence) and its occurrences queue behind it, 80 of them by 01:26. That is H1 at
a magnitude that is no longer one provider call. It remains a hypothesis
(the dispatcher records no cause), but it is the leading one, and the point
that matters is that the record still cannot say. An unexplained number gets
a read-only record when seen (CLAUDE.md), and this is the number.

Do the 80 queued updates delay real work? Unknown, for the same reason. The
records-inbox contract says a wake carries no work (the turn reads its inbox
itself), which suggests the held-back occurrences are bookkeeping; but 8m16s
for management is long enough that a real directive could sit behind it.
Open question 4 of this record is therefore the next read: what kinds the
queued updates were.

### The smallest correct fix

Option 1 of section 5, nothing more: in `dispatchPending`
(`dispatcher.go:355-377`) record why a due agent was skipped (busy fence, hold,
no room) and, for the busy case, the holder's start time; carry
`cause=busy|held|noroom holder_started=...` and the queued kinds into the
`activation wait` line (`logActivationWait`, :694-724). One file plus a test
on the log line. Class orange (a log field; no scheduling change). About 1.5
to 2 hours for a Sonnet builder. No restart-rule implication (process memory,
no resumer). Not built.

### What not to do

Options 2 to 5 stay unbuilt until the cause field has run for a day. The data
already says options 2 (event-driven deferrals A1/A2) and 4 (the outbox hop)
are not what the 8m16s was: no dispatcher deferral or hold line appears in the 512-line log window (00:49 to 01:31Z). Option 5 (fence
by work, not by turn) is the only one that would remove an 8-minute fence, and
it changes the delivered-equals-incorporated invariant; it needs the cause
field and the queued kinds first. This is also the same substrate shape as
the cluster below in one respect: an obligation (the queued update) is held
by a unit of exclusion longer than its own work, with nothing visible.
Cross-reference:
[unsettled-obligations-cluster-2026-11-09.md](unsettled-obligations-cluster-2026-11-09.md)
(this record is a sibling, not a member: the updates here do run, late).

### (b) to (e)

Cause: H1 leading, unconfirmed; what would confirm it is the cause field
(fix above) on the next wait. Substrate (the synchronous-turn fence), but the
action tonight is observation. Fixed on main: no.
