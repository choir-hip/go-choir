# Management Delegates: Management Tasks Instead Of One Serial Turn

Date: 2026-10-14, about 08:40 UTC. Problem:
[management-serial-turn-blocks-paper-2026-10-14](../problems/management-serial-turn-blocks-paper-2026-10-14.md).
Owner direction, 03:55 UTC: "Yeah we can't do serial. Management should
delegate to subrlms." The Jev half (support judgement leaves management)
has landed: `choir.Judge` is gone from the kernel
(`internal/yaegikernel/choir_parity_test.go:422-449`), and no
`support_pairs` record reached management after 05:xx UTC on the live
paper. This note covers the other half: management delegating.

Status: design only. Nothing here is implemented, pushed or deployed. An
agentic-consensus panel comes before code, as the problem doc says.
The note is green. The work it describes is **red** (management runtime,
records routes and inbox authority, run admission, gateway model-call
spacing, restart closures).

Evidence classes used below:
- **code**: file:line on this branch (base `0257fac3`);
- **live**: product routes on the live paper (computer
  `computer-1355b83b…`), read 08:30 UTC with the paper credential
  (`/api/runs?channel_id=management:<owner>`, `/api/runs/<id>/events`,
  `/api/inboxes/management:<owner>/records`). Counts, states and timings
  only;
- **hypothesis**: labelled as such and not yet confirmed.

## 1. The short version

- Management today is one run at a time over one inbox cursor, and every
  kind of work goes through that run. Doctrine already forbids this:
  "management … is not a concurrency limiter"
  (`docs/choir-doctrine.md:424-430`, I26).
- Change: management hands bounded pieces of work to **management tasks**.
  A task is a sub-RLM: its own run, desk session and model loop, with a
  narrow verb set and a lease on the records it was given. It acts
  outward as `management:<owner>` (so a Texture still has one
  conversation with management) with the task named in provenance, and
  it ends with a final Report into management's inbox.
- The parent never waits on a task. Delegation is a staged act that
  commits and returns a receipt.
- Capture intake (open, feed, decline) moves to tasks first, then a
  Texture's Asks and Notes. Engineering admission, Verify, the product
  API, commitments and escalation stay with the parent.
- Later, capture arrivals go straight to intake tasks through a standing
  route, so the paper's arrival path never touches management's turn.
- Acceptance: on the live paper, capture arrival to first disposition
  p95 at most 5 minutes and max at most 15 minutes over 6 hours; no
  parent turn over 10 minutes (p95 at most 2 minutes); the same holds
  while a planted 20-minute task runs.

## 2. What management does serially today

### 2.1 The mechanism (code)

1. **One run per desk.** `WakeRecordsInbox` starts a management run only
   when no run of `management:<owner>` is active
   (`internal/agentcore/inbox_records.go:656-660`). A record that arrives
   during a run wakes nobody. The post-turn re-check starts one more turn
   (`inbox_records.go:708-748`).
2. **One cursor.** Each cell shows the records in (cursor, head] of
   `management:<owner>` and commits a compare-and-set cursor advance with
   its acts (`inbox_records.go:125-190`, `:242-247`). There is one reader,
   so there is one place where any record gets answered.
3. **The prompt batches.** "Several polls may have arrived since your
   last turn; answer all of them in this turn"
   (`internal/promptstore/defaults/management.yaml:24`). One desk_go_eval
   call per model turn, step by step
   (`internal/runtimeprompts/overlays/rlm_management_runtime.yaml:14`).
4. **One verb set does everything.** The management module set holds
   intake (ReadCapture, Stories, Feed, Decline, Cast to `texture:new`),
   conversation (Ask, Note, Reply, ReportPacket), engineering (Cast,
   CancelAssignment, Verify), commitments (Precommit, Resolve,
   Disagreement), Escalate and ProductAPI
   (`internal/yaegikernel/choir.go:296-298`, actions `:137-141`).
5. **No host deadline on a cell.** A cell is bounded only when the model
   sets `timeout_ms` (`internal/agentcore/tools_desk.go:442-444`).
   Nothing bounds a management turn.

### 2.2 The work, counted (live)

Management's inbox from seq 1 (10-13 15:22 UTC) to seq 514 (10-14 08:23
UTC), about 17 hours:

| record (sender, kind) | count | what management must do | delegable |
|---|---|---|---|
| `source:<id>` captures arrival | 133, naming 475 captures | open, feed or decline every capture | yes: intake task |
| `texture:<doc>` Ask (listed as `precommit`) | 35 | read captures, answer with passages (e.g. "Does capture … add any material finding …? send the exact passage(s)") | yes: answer task |
| `texture:<doc>` Note or Cast (`directive`) | 11 | read, answer; a Cast may need engineering | Notes yes; a Cast that needs engineering stays with the parent |
| `texture:<doc>` system `support_pairs` | 335 | none now: Jev judges off the turn | already delegated (to Jev) |
| `engineering:<id>` Reports | 0 on the paper | read, Verify, final Report to the Texture | stays with the parent (open decision O4) |

Also, not counted from records: restarting source schedules through
ProductAPI after a restart, and story listing.

Management runs on the paper in the same window (154 runs, all
`request_source: inbox_record`): 148 completed, 5 failed, 1 cancelled.
Duration p50 53 s, p90 282 s, p95 812 s, max 3,601 s. Management was in
a run 46% of the window (27,927 s of 61,264 s). 105 of 132 capture
arrivals found management already in a run.

### 2.3 How long arrivals wait (live, estimated)

Estimated from run boundaries, not from the disposition records. The
exact join (arrival record to the first record whose `Disposition`
names the capture, `inbox_records.go:334-337`) is slice S0's instrument.

| estimate | p50 | p90 | p95 | max |
|---|---|---|---|---|
| lower: end of the first management run that ends after the arrival | 137 s | 1,210 s | 1,274 s | 1,279 s |
| upper: end of the first management run that starts after the arrival | 275 s | 1,438 s | 1,520 s | 2,128 s |
| wait for the run in flight at arrival to end | 111 s | 1,208 s | 1,217 s | 1,279 s |

So today the p95 wait is about 21 to 25 minutes and the worst about 35
minutes, and most of it is waiting for a turn already running.

Observation that corrects the problem doc's emphasis: the inbox has 1, 0
and 1 capture arrivals in the 02, 03 and 04 UTC hours. During the
38-minute run (1565c043, 03:13-03:52) what waited was mostly Texture
requests. The capture cost shows in the busy 07 UTC hour (27 arrivals),
below.

### 2.4 After Jev took support judgement: run 305a582d (live)

Run `305a582d-85c9-44ae-8a31-ef63084769c0`, 07:12:08-07:40:08 UTC,
deepseek-v4.1-flash, 1,680 s. From its events:

- Cell 1 lists one record: a Texture Ask (seq 469: "Please identify the
  supplied source for the coding-agents document cast …").
- Cells 2-6 search `choir.Pack()` for that document by string matching.
- **Cell 6 ran 860,565 ms and ended with `frame: read header: EOF`**:
  the worker died after 14.3 minutes, and nothing timed it out.
  Hypothesis H1: the cell's hand-written index loop
  (`for i := 0; ; { j := strings.Index(s[i:], …); … i = i + j + 8 }`)
  never advanced under the interpreter and grew `offs` until the worker
  died. Not confirmed; the worker's exit cause was not recorded.
- Cells 7-57 read the remaining records (captures seq 475 onward), call
  `choir.Stories` 27 times, `choir.ReadCapture` 55 times and
  `choir.ProductAPI` 56 times.
- **The first act committed at 07:34:44, 22.6 minutes into the run.** In
  total: 27 Casts to Texture inboxes, 27 Notes (feeds and declines), 7
  Replies.
- 58 model calls. Input grew from 13,668 to 137,087 tokens per call:
  **4,107,528 input tokens** and 74,704 output in one turn.

The other management runs after the schedules came back (06:49-08:26),
by duration: 4, 8, 35, 41, 54, 68, 95, 114, 126, 142, 198, 345, 401 and
1,680 s, plus 4 failed runs (3 with no model call). Intake-heavy turns
used 13k-67k input tokens per committed act (d7465501 13k, 3f2a4412 16k,
3983499c 33k, 0306409b 37k, 305a582d 67k).

So the serial turn costs three ways: latency (everything waits for the
slowest thing in the turn), fate-sharing (one stuck cell or slow model
call stops all intake), and tokens (one transcript grows over 58 calls).

## 3. The management task

### 3.1 What it is

A management task is a conversation on records plus a run, like an
engineering assignment without a capsule:

- **Identity.** Inbox `management-task:<task_id>`. Its conversation with
  management is (`management:<owner>`, `management-task:<id>`). A round
  opens with the delegating Cast and closes with the task's final Report
  or a system closure, by the existing round rules (`store/inbox.go:236-240`
  and the C1e records model). The task's own cursor reads its own inbox.
  Exact inbox spelling is an implementation choice; it must not parse as
  `management:<subject>`, because `recordsRoute` checks that subject
  against the owner (`inbox_records.go:66-68`, `:95`). The records route
  also requires the reduction scope's desk agent id to name the run's
  profile (`inbox_records.go:427`), so a task's scope carries two ids:
  `management:<owner>` as the sender (§4.2) and the task inbox as the
  reader whose cursor advances.
- **Not parent/child.** Control, liveness, cancellation and recovery hang
  on the task's round and run, never on the parent run
  (`docs/choir-doctrine.md:323-325`, I3). `requested_by_run_id` is
  provenance only. The parent run ending does not end its tasks.
- **A run.** Each task is a run with a run record, events and a channel,
  so it shows in `/api/runs`, in the trace and in `/api/costs`. This is
  cut 1's L5 shape ("child runs leave events, report only to the parent
  inbox, and the parent cell does not block on them",
  `docs/definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md:139`).
  Research children today are not this: no run record, results returned
  to a parent cell that blocks on `wg.Wait()`
  (`internal/agentcore/research_children.go:133-135`, `:154-172`).
- **Its own session.** A fresh desk worker for the task, released at the
  task's end, as research children do
  (`research_children.go:198-199`). It does not take a slot in the idle
  cache (`tools_desk.go:52-53`).
- **Bounded.** One task kind, a fixed set of records, a deadline (start:
  10 minutes, like `researchCallWindow`, `research_children.go:46`), and a
  host cell deadline (slice S1).

Reuse, not a third mechanism: engineering assignments already have the
pieces (an assignment row bound to a parent round, a run started on the
Cast, the closing record, crash and ended-run closures,
`internal/agentcore/engineering_assignment_runtime.go:163`,
`inbox_records.go:565-590`, `:800-905`). Recommendation: generalize that
row to a kind (`engineering` | `management_task`) rather than build a
parallel table. L5 (sub-researchers as runs) should land on the same
substrate.

### 3.2 Task kinds

| kind | given | done when |
|---|---|---|
| `intake` | one or more capture arrivals (their items and Jev labels) | every leased capture has a disposition (open, feed, decline) or is returned as a residual |
| `answer` | one Texture record (an Ask or a Note) and its conversation | the Ask is answered (Reply) or the Note acknowledged, or it is returned as a residual ("needs engineering", "needs research") |
| `verify` | one engineering assignment's final Report and bundle digest | open decision O4; not in the slices below |

Unit of intake: one arrival record (one poll; 475 / 133 = 3.6 captures
on average). Jev already marks same-event siblings inside a poll
(L2a R2-3), so a poll is a natural unit. A queued intake task that has
not started yet absorbs later arrivals, up to 20 captures, so a burst
becomes fewer, larger tasks and same-event captures from different
sources are more often decided together.

### 3.3 How a task is opened

Two routes, landing in this order:

1. **Parent-directed.** `choir.Delegate(kind, objective, refs)` on
   management's surface stages a Cast to a new task inbox. `refs` are
   record ids from the parent's `choir.Records()`. The Cast copies the
   records' captures (items and the label version shown) or the Texture
   record into its body, so the task's `choir.Records()` shows them as a
   captures record or a Texture record and the existing kernel checks
   apply unchanged (Decline's "named by a captures record of this turn",
   `choir.go:706-729`). The parent's cell commits the Cast and its cursor
   advance in one transaction, as it does now. Receipt:
   `rlm:delegate:N to management-task:<id>`. The parent does not wait.
2. **Standing intake route** (slice S4). With a computer setting
   `management.intake_tasks` on (off by default, set by an owner act like
   `intake.labels`), a capture arrival is addressed to an intake task
   inbox instead of `management:<owner>`, and the runtime opens or extends
   the task. The arrival path then never involves management's turn. The
   setting is management's standing delegation, recorded and reversible;
   turning it off is the rollback.

## 4. Authority

### 4.1 Verbs

| verb | parent (management) | intake task | answer task |
|---|---|---|---|
| Records, Context, Pack | yes | yes (own inbox) | yes (own inbox) |
| Stories | yes | yes | yes |
| ReadCapture | yes until S5 (O9) | yes | yes |
| Feed, Decline | until S5 | yes, leased captures only | no |
| Cast `texture:new` | until S5 | yes, leased captures only; library check (§6) | no |
| Note or Cast to `texture:<doc>` | yes | yes, naming leased captures | Note to the leased conversation |
| Reply, ReportPacket to a Texture | yes | no | yes, leased conversation only; a final Report only when the round has no engineering assignment (D6, `inbox_records.go:565-590`) |
| Delegate, CancelTask | yes | no (one level, as research children: owner PD1) | no |
| Cast engineering, CancelAssignment, Verify | yes | no | no |
| ProductAPI | yes | no | no |
| Precommit, Resolve, Disagreement, Escalate, CancelAct | yes | no | no |

The parent keeps what changes computer-wide coherence: admitting
engineering, verifying a candidate, the owner's product routes
(schedules, settings, run acceptances), commitments on the ledger,
escalation to the owner, delegation and cancelling tasks. A task changes
only the stories and conversations its lease names.

### 4.2 Delegated sender

A task's outward records carry `SenderInbox = management:<owner>`, so a
Texture keeps one conversation with management
(`management.yaml:15`: "You hold one conversation with every Texture of
your owner") and round rules see one caster. Provenance adds
`acting_task: <task_id>` and the task's run id. Choir.Pack shows the act
as management's, with the task named.

The reducer, not the store, checks the lease: a task's act is accepted
only if every capture it names and every conversation it addresses is in
its open round's Cast. The store keeps validating well-formedness only
(`store/inbox.go:57-58`).

### 4.3 The lease is the open round

There is no lease table. A capture or Texture record is leased to a task
exactly while the task's round that names it is open (single authority,
standing question 5). Consequences:

- delegating a record already leased is refused at the parent's reduce;
- the parent's own act on a leased record is refused until the round
  closes or the parent cancels the task;
- a closed round (final Report, crash closure, deadline, cancel) frees
  the lease with no extra write.

## 5. How results return

- A task ends with one final Report to `management:<owner>`, closing its
  round. The body lists what it committed (record ids per capture), its
  residuals (captures or requests it did not settle, each with a reason),
  its error if any, and its tokens.
- **A clean final Report does not wake the parent**: a Report with
  `work_disposition: completed` and no residual is recorded and stays
  unread until the parent's next turn, like a support pass writing only
  its memo. A Report with residuals, a failure, or a deadline closure
  wakes the parent. This needs one rule in `inboxRecordWakes`
  (`store/inbox.go:209-219`). Without it, every task would wake a parent
  turn and the serial queue would come back (failure mode 2).
- The parent reads residuals in a later turn and acts: re-delegates,
  casts engineering for an answer task's "needs engineering", or
  escalates.

## 6. Ordering and conflicts

1. **Two tasks open a story for the same event.** Example: two sources'
   polls report one new event in the same minute, and both intake tasks
   cast `texture:new`. Guard: an optimistic check on the story library.
   The store keeps a library head (count of story opens), advanced in the
   commit of a `texture:new` Cast. A task's session remembers the head
   its last `choir.Stories()` read. A `texture:new` Cast from a task whose
   seen head is behind is refused with the stories opened since (headline,
   captures). The task re-decides: feed one of them, or cast again with
   `distinct_from: [doc ids]`, which commits and is recorded as an
   override for Jev's examples (L2a A3). With 4 tasks and about 27
   arrivals in the busiest hour, refusals should be rare (hypothesis H5,
   measured by AC4).
2. **Two tasks feed the same story.** Feeds are Notes or Casts into
   `texture:<doc>`. Each record takes the next sequence in that inbox
   inside its own commit, so they serialize and the Texture reads both in
   one turn. A Cast into an open round is accepted from the same caster
   (`store/inbox.go:236-240`); both tasks send as `management:<owner>`.
   Tasks may add hypotheses but not withdraw them, so one task cannot
   withdraw another's.
3. **One capture in two tasks.** Impossible while the lease is the open
   round (§4.3).
4. **A capture disposed twice** (a late commit racing a closure and a
   re-delegation). A second disposition of a capture is refused at reduce
   unless it is the parent's explicit re-disposition, which is recorded as
   an override.
5. **The store's single writer.** Every commit serializes on the engine
   lock (`internal/objectgraph/engine_mutex.go:10-14` already records
   wait and hold time per caller). Tasks hold no lock across a model call;
   a commit is milliseconds. Hypothesis H3: at K=4, lock waits stay small;
   the engine-mutex metrics measure it, and the read/write lock (SA slice
   4) waits on that measure.
6. **Parent and task on one Texture conversation.** Excluded by the lease;
   the parent acts after the round closes or after CancelTask.

## 7. Concurrency and admission

- **K tasks run at once per computer, the rest queue; nothing is
  refused.** Start K = 4, as research children
  (`research_children.go:25`) and intake labels
  (`internal/agentcore/intake_labels.go:44`). A queued task is a
  committed round whose run waits for a slot; its records are leased and
  visible as queued.
- **Little's law (estimate).** The busiest hour had 27 arrivals (0.45 a
  minute). At about 2 minutes per intake task, about 1 task runs on
  average, so K = 4 leaves room for answer tasks and bursts.
- **Memory.** A desk worker costs about 17 MiB private
  (`docs/evidence/guest-memory-split-balloon-2026-10-14.md`). K = 4 tasks
  add about 68 MiB at peak, released at task end. The daemon's per-run
  memory is unmeasured (hypothesis: small next to its 690 MiB).
- **Model calls.** The gateway allows 60 calls a minute per computer
  (`internal/gateway/ratelimit.go:12`). Research children already share
  one computer-wide spacer at 40 a minute
  (`research_children.go:31-38`). Tasks join that spacer, so sub-RLMs of
  every desk together leave about 20 a minute for the root desks. The
  split is open decision O5.
- **No spend caps.** These bounds limit concurrency for memory and the
  rate limit. They never refuse or cap model spend (owner ruling,
  `docs/reports/cost-track-sa-s3-sp-investigation-2026-10-14.md:253-255`).

## 8. Restart semantics

- **Crash.** A task's wake is an `inbox_record` or `initial_dispatch`
  occurrence, both restart work kinds
  (`internal/agentcore/planned_restart.go:161-173`), so a crash boot
  starts no task. The boot closure closes every open round, task rounds
  included, as `interrupted_by_restart`, and wakes no one
  (`inbox_records.go:756-795`). Leases free with the rounds. Captures a
  crashed task had not disposed keep the visible fate "interrupted by a
  restart" in the closure record in management's inbox. They are not
  re-delegated at boot. The parent's next live turn reads the closure and
  may re-delegate them; that is new work by a live turn. The standing
  intent (the source schedule) survives; the occurrence does not.
- **Run ends without a final Report** (failed, deadline, cancelled). The
  ended-run closure gives the round a system final Report and wakes the
  caster (`inbox_records.go:800-868`). For a task the caster is the
  parent; the closure lists the leased records still unsettled.
- **Planned restart.** Recommendation for v1: tasks are not resumed.
  Before a planned restart the runtime drains: it stops starting queued
  tasks, lets running tasks finish within their deadline (at most 10
  minutes), then restarts with no task running. Queued tasks are
  delivered after a boot that consumed the marker, once, which the rule
  allows. This gives an always-busy paper a bounded way to become idle,
  which the open problem in
  `docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md`
  ("how an always-busy computer takes updates") needs. Who requests the
  drain (the deploy's refresh) is outside this note. Open decision O6.

## 9. Cost

- **Model calls per task (estimate).** An intake task needs about 4-6
  calls: read records, read a capture or two where the title is unclear,
  check stories, dispose, end. At 12k-20k input tokens per call (the
  management system prompt is 22,931 characters, about 6k tokens, plus
  the records and a few cells), that is about 50k-120k input tokens per
  task, or 14k-33k per capture. Today's intake-heavy turns spent 13k-67k
  per committed act (§2.4). Hypothesis H2: tasks cost the same or less
  per capture, because each task starts a fresh transcript instead of
  growing one over 58 calls. AC7 checks it.
- **Recorded.** Tasks are runs, so their `tool_loop` events feed
  `/api/costs` like any desk (priced once the rates are in the table;
  tokens count either way).
- **Jev instead of a model.** Jev labels already exist per capture at
  about $0.0001 each (L2a A1). A label with high probability of "same
  event as story X" could be fed by a system act with no model call, and
  tasks would handle only new and uncertain captures. That changes L2a's
  settled "labels are advice, management decides"
  (`docs/reports/l2a-synthesis-intake-design-2026-10-14.md:73-77`, "Labels are
  advice; management decides"), so it
  is the owner's call (O3) and not in the slices.
- **Model for tasks.** Bounded intake wants a fast model. A free preview
  model with 5- to 9-minute calls would eat a task's 10-minute deadline.
  Open decision O5 (a `management_task` role in the model policy,
  defaulting to management's).

## 10. How the parent turn stays short

1. **No waiting verb.** Delegate returns a receipt. There is no
   `choir.Wait`, no barrier, and no polling of task state (banned pattern
   7, `docs/choir-doctrine.md`). A task's outcome arrives as a record.
2. **The long work is not in the parent's verb set.** After S5, the
   parent cannot read captures in a loop or dispose of them, so a turn
   cannot become an intake marathon.
3. **Clean task Reports do not wake it** (§5), so the parent turns over
   for residuals, Texture Casts and engineering, not for every task.
4. **A host cell deadline** (S1) bounds any one cell, closing the
   14-minute stuck cell seen in 305a582d.
5. **The prompt says triage.** "Answer all of them in this turn"
   (`management.yaml:24`) becomes: delegate captures and Texture requests,
   act on residuals, engineering and the computer, end the turn.

No iteration cap on the parent: the owner ruled out budget caps, and the
structure above is what keeps the turn short. AC2 measures it.

## 11. Failure modes (written before any code)

1. The parent blocks on a task (a wait verb, a barrier, or a commit that
   waits for the task's run). Guard: Delegate commits a Cast only; tested
   with a task whose model blocks until released, while the parent's turn
   ends.
2. Task Reports wake the parent on every task, rebuilding the serial
   queue. Guard: the wake rule in §5; tested.
3. Duplicate stories from concurrent opens. Guard: the library check
   (§6.1); measured by AC4.
4. Two tasks lease one capture, or the parent acts on a leased record.
   Guard: the lease is the open round; refused at reduce; tested.
5. A task acts outside its lease (other captures, other conversations,
   engineering, ProductAPI). Guard: the task module sets plus the reduce
   check; reverse parity test (as `choir_parity_test.go:422-449`) for
   both task kinds.
6. A task's acts look like the parent's with no attribution. Guard:
   `acting_task` and the run id on every record; Pack shows them.
7. Tasks spawn without bound (14 sources, a burst). Guard: K slots, a
   queue, coalescing of queued intake tasks, the shared spacer.
8. The queue grows faster than tasks finish. Guard: the queue depth and
   the oldest queued age are reported per computer; coalescing makes
   tasks larger, not more numerous. No refusal.
9. A crash resumes tasks at boot. Guard: restart work kinds plus the boot
   closure; drill AC6.
10. A planned restart strands a task half-done or resumes it into a loop.
    Guard: drain before the restart; no resume in v1.
11. A task hangs (stuck cell, slow model) and holds its lease. Guard: the
    task deadline closes the round, frees the lease and returns the
    captures to the parent as residuals; the host cell deadline.
12. A capture is disposed twice (late commit after a closure). Guard: a
    second disposition is refused unless the parent re-disposes it
    explicitly.
13. An answer task closes a Texture round that has an engineering
    assignment. Guard: D6 (`inbox_records.go:565-590`) applies to tasks
    unchanged; tasks cannot cast engineering.
14. Tasks and research children together exceed the gateway's
    per-computer limit and get 429s. Guard: one shared spacer.
15. The parent keeps doing intake itself (a dual path, I5,
    `docs/choir-doctrine.md:331`). Guard: S5 removes intake verbs from
    the parent and its prompts; parity test.
16. A crash-closed task's captures are never disposed. Accepted by the
    restart rule; they stay visible as interrupted and are counted
    separately in AC5. The parent may re-delegate them on a live turn.
17. Tasks cost more than the serial turn. Guard: AC7; tokens per task
    recorded as runs.
18. The owner cannot see the tasks. Guard: tasks are runs in
    `/api/runs`; open task rounds show in the inbox route's
    `open_rounds`.

## 12. Acceptance

Measured, on the product path:

- **AC1 latency.** Live paper, 14 sources at 15-minute intervals,
  intake.labels on, 6-hour window: for every capture, arrival (arrival
  record `committed_at`) to first disposition (first record whose
  `Disposition.Captures` names it, or a `texture:new` Cast whose spec
  names it). **p95 at most 5 minutes, max at most 15 minutes.**
  Crash-closed captures are counted separately. Baseline (§2.3,
  estimated): p95 about 21-25 minutes, max about 35.
- **AC2 short parent.** Same window: **no management parent turn longer
  than 10 minutes; p95 at most 2 minutes.** Baseline: p95 812 s, max
  3,601 s.
- **AC3 long-task drill.** Minted test computer with the paper fixture:
  a planted answer task held for 20 minutes (a model fake that stalls)
  runs while 4 sources poll. AC1's and AC2's bounds hold during it.
- **AC4 coherence.** Fixture stories-per-event no worse than the serial
  baseline, and no two stories for one judge-scored event opened within
  10 minutes of each other by two tasks (`scripts/l2a_paper_window.py`
  pairs).
- **AC5 completeness.** Every capture has exactly one disposition or the
  visible fate "interrupted by a restart".
- **AC6 crash.** Kill the guest with at least 2 tasks running: the boot
  starts no task, closes their rounds `interrupted_by_restart`, frees
  their leases; the next poll's arrivals get new tasks.
- **AC7 cost and memory.** Input tokens per disposed capture at most 1.2
  times the window before S2; peak concurrent task workers at most K.

## 13. Slices, in order

Each slice is its own commit series, pushed, CI'd, deployed and run on
the paper before the next (the Landing Loop).

- **S0, instruments (yellow).** The 860 s stuck cell is already recorded
  in the problem doc (this commit; problem documentation first). S0 adds
  the exact AC1 join and the AC2 turn statistics as a script over product
  routes, extending `scripts/l2a_paper_window.py`. Take the baseline
  window.
- **S1, host cell deadline (red, small).** A default deadline on
  management cells when `timeout_ms` is unset (start: 120 s; management
  has no SearchWait). A cell that hits it is killed with its session, as
  a model-chosen timeout is today (`tools_desk.go:475-485`).
- **S2, the task substrate and intake tasks, parent-directed (red).** The
  assignment row generalized to `management_task`; `choir.Delegate`; the
  task run with its own session; task module sets; delegated sender and
  lease check; the library check; the clean-Report wake rule; K slots and
  the shared spacer; crash, deadline and ended-run closures. Tests for
  failure modes 1-13 first. Prompt: delegate captures. Paper run; AC1 and
  AC2 read.
- **S3, answer tasks (red).** A Texture's Asks and Notes go to answer
  tasks; "needs engineering" comes back as a residual. Paper run.
- **S4, standing intake route (red).** `management.intake_tasks` setting;
  arrivals addressed to intake tasks with coalescing; the parent stops
  receiving capture arrivals. Fixture first, then the paper; AC1-AC7.
- **S5, cutover (red).** The parent loses Feed, Decline and `texture:new`
  (and ReadCapture if O9 says so); prompts rewritten; parity tests. Closes
  the dual path.
- Not scheduled: `verify` tasks (O4); Jev auto-feeds (O3); L5 research
  children moving onto the same substrate (cut 1 L5, C1e step 5).

Rollback: each slice reverts whole. S4 rolls back by turning the setting
off; arrivals then go to `management:<owner>` again and the parent
handles them with the S2 path.

## 14. Ceremony (red)

- **Conjecture delta.** Before: management's coherence role required
  that one serial turn do every piece of management work. After:
  coherence needs the parent to hold admission, commitments and the
  computer's settings; disposition of captures and answering a Texture's
  requests are bounded tasks that run in parallel under leases.
  Falsified if concurrent tasks fragment stories (AC4 fails) or the
  parent cannot keep coherence from task Reports.
- **Protected surfaces.** `internal/agentcore/inbox_records.go`
  (routes, wake, cursor, closures), `engineering_assignment_runtime.go`
  (the assignment row), `tools_desk.go` (sessions, cell deadline),
  `research_children.go` (spacer), `internal/store/inbox.go` (wake rule,
  library head), `internal/yaegikernel/choir.go` and `surface.go`
  (module sets, Delegate), management prompts and overlay, gateway call
  pacing.
- **Admissible evidence.** Local store-backed tests through real reduce
  and commit paths with model fakes, for the failure modes. Acceptance
  only from deployed proof: the paper's records and runs through product
  routes (AC1, AC2, AC5, AC7) and a minted test computer for the drills
  (AC3, AC4, AC6).
- **Rollback.** Revert per slice; S4's setting off.
- **Heresy delta.** Discovered: management is a concurrency limiter,
  against I26; a management cell can run 14 minutes with no host bound;
  research children are not runs and block their parent cell (already
  named by cut 1 L5). Introduced (risk, intended none): a delegated
  sender, where `management:<owner>` acts that a task authored, mitigated
  by `acting_task` provenance on every record. Repaired (when S2-S5 land):
  management stops being the paper's concurrency limit; the parent's
  intake dual path is deleted.

Standing questions, briefly: (1) the direction is the owner's, 03:55
UTC; (2) it follows the settled records model, the restart rule and L2a,
except O3, which would change L2a; (5) the lease is the open round, with
no second table; (6) success is measured from records and runs, not
narration; (8) tasks hold nothing durable beyond records, and a crash
closes them.

## 15. Open decisions

- **O1 Routes.** Parent-directed delegation only, or also the standing
  intake route? Recommended: both, standing for captures (the 24/7 path),
  parent-directed for the rest.
- **O2 Sender.** Tasks send as `management:<owner>` with `acting_task`
  (recommended), or from their own inbox, which would split a Texture's
  conversation with management into one per task.
- **O3 Jev auto-feed.** Feed high-probability same-event labels by a
  system act with no model call. It changes L2a's "labels are advice".
  Owner's call.
- **O4 Verify.** Delegate Verify to a task? Recommended: no. Verify is in
  the self-development acceptance chain, and once intake runs in tasks a
  long Verify no longer blocks the paper.
- **O5 Model and pacing.** Which model tasks use, and how the 60
  calls-a-minute budget splits between root desks, research children and
  tasks.
- **O6 Planned restart.** Drain then restart (recommended), or resume
  tasks once on a marker.
- **O7 Numbers.** K = 4, a 10-minute task deadline, a 120 s cell
  deadline, coalescing up to 20 captures: starting values, to be measured.
- **O8 Clean Reports.** Confirm that a clean task Report does not wake
  the parent.
- **O9 ReadCapture.** Does the parent keep ReadCapture after S5 to check
  coherence?

## 16. Hypotheses in this note

- H1: the 860 s cell in 305a582d was a non-advancing loop that grew
  memory until the worker died. Unconfirmed; the exit cause was not
  recorded.
- H2: tasks cost the same or fewer input tokens per capture than the
  serial turn.
- H3: engine-lock waits stay small at K = 4.
- H4: the arrival-wait estimates in §2.3 bracket the true latency. S0's
  exact join replaces them.
- H5: library-check refusals are rare at the paper's arrival rate.
