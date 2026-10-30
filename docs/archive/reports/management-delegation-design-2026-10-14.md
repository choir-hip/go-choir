# Management Delegates: Management Tasks Instead Of One Serial Turn

Date: 2026-10-14, about 08:40 UTC. Problem:
[management-serial-turn-blocks-paper-2026-10-14](../problems/management-serial-turn-blocks-paper-2026-10-14.md).
Owner direction, 03:55 UTC: "Yeah we can't do serial. Management should
delegate to subrlms." The Jev half (support judgement leaves management)
has landed: `choir.Judge` is gone from the kernel
(`internal/yaegikernel/choir_parity_test.go:422-449`), and no
`support_pairs` record reached management after 05:xx UTC on the live
paper. This note covers the other half: management delegating.

Status: design only, **round 2**. Nothing here is implemented, pushed or
deployed. A six-member agentic-consensus panel reviewed round 1: five voted
"accept with amendments" and one "revise". The adjudication is
[management-delegation-adjudication-2026-10-14](management-delegation-adjudication-2026-10-14.md).
Every round-2 amendment below rests on a finding the adjudication verified
against code. The note is green. The work it describes is **red**
(management runtime, records routes and inbox authority, run admission,
gateway model-call spacing, restart closures).

Round 2 changelog (amendment: the adjudication finding behind it):
- **R2-1** §3.1a: a task identity table. It gives the run profile and role,
  the reader inbox, two senders, and every profile-keyed switch S2 changes.
  It also corrects the round-1 citation for the inbox spelling (B1, B2).
- **R2-2** §3.1a: the task role is registered in one commit everywhere it
  is keyed. The task's action list excludes Exec, WriteFile, Assign and
  ProductAPI. A reverse-parity test covers it (B3).
- **R2-3** §5: one wake predicate, used by both `inboxRecordWakes` and the
  post-turn re-check (B4).
- **R2-4** §4.2 and §6.4: the lease is a store precondition checked under
  the commit lock, not a reducer check. A per-capture disposition object
  refuses a second disposition (B5).
- **R2-5** §6.1: the library head is carried on the intent and checked in
  the store, for the parent's `texture:new` too (B6).
- **R2-6** §7: a durable slot starter. A queued task is an open round with
  no run, started by an outbox wake of a restart work kind (B7).
- **R2-7** §3.1: a `management_task` row with its own validation. It reuses
  the assignment row's patterns, not the engineering binding type (B8).
- **R2-8** §5: the host mints the task's final Report and computes its
  residuals from durable dispositions, so "clean" is not the model's claim
  (B9).
- **R2-9** §9 and §13: the task model is pinned in S2 to a non-preview,
  bounded-latency model with a per-call timeout. The 10-minute deadline
  stays (B10).
- **R2-10** §12: acceptance that cannot be gamed:
  - declines cannot satisfy latency (AC1 counts opens and feeds; AC8 checks
    the disposition mix);
  - crash-closed captures are bounded at 2%;
  - AC10 asserts that serialization is gone;
  - duplicates are counted across the whole window;
  - AC7 pins the model mix;
  - AC9 measures task completion (B11).
- **R2-11** §13: S2's exit criteria. AC2 on intake-only parent turns
  gates S2, and the full AC2 gates S3, the slice that moves Ask turns. The
  full AC1 gates S4 (B12).
- **R2-12** §8: a running task's fate at a planned boot. It gets the
  ended-run closure with residuals. Queued tasks start once. A busy
  computer is never forced (B13).
- **R2-13** §8 and §13: an answer task holds its Texture round against the
  parent's ended-run closure and D6. This is S3's first test (B14).
- **R2-14** §3.3: batch `choir.Delegate` (B15).
- **R2-15** §12: AC3 plants a task that stalls until its deadline (not "20
  minutes"). S3 repeats it with K answer tasks. Failure mode 13 moves to
  S3 (B16).
- **R2-16** §4.1 and §4.3: the lease on a Texture record is the right to
  Reply or send the final Report. Feeds that name the task's own leased
  captures go to any story (N1).
- **R2-17** §8: a capture interrupted by two crashes is closed with that
  cause and is never re-delegated (N2).
- **R2-18** §7: from S3, 2 of the K slots are reserved for intake (N3).
- **R2-19** §13 S1: the cell deadline is independent of delegation. Its
  default comes from S0's cell durations, max(300 s, 2 × p99), and every
  kill is logged with the worker's exit cause (N4).
- **R2-20** §4.1: CancelTask semantics (N6).
- **R2-21** §13: the slices are split into S2a and S2b and run in the order
  S2, S4, S3 (N7, N8).
- **R2-22** §1 and §9: deterministic intake is named as the proposed
  destination, conditional on the owner's O3 (N9).
- **R2-23** §15: O2 is settled by the code. Each open decision is marked
  settled or the owner's (N10).
- **R2-24** §3.2: coalescing is a further Cast into the queued round, with
  a cap of 12 (N11).
- **R2-25** §16: H6, story-open Reports still wake the parent (N12).
- **R2-26** §13 rollback: the S4 setting off drains, and an S2 revert waits
  until no task round is open (N14). Also AC6 asserts task runs (N15), the
  §6.2 wording (N16), and fixtures need intake.labels on (N13).

Evidence classes used below:
- **code**: file:line on this branch (round 1 base `0257fac3`; round-2
  citations re-checked on `380ad9a19`);
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
- Acceptance on the live paper, over 6 hours:
  - capture arrival to first open or feed: p95 at most 5 minutes, max at
    most 15 minutes. Declines do not count toward latency, and the
    disposition mix stays near the baseline;
  - no parent turn over 10 minutes, and p95 at most 2 minutes;
  - parent turns start only for records that need the parent;
  - the bounds hold while a task stalls to its deadline (R2-10, R2-15).
- Separate from delegation, S1 gives every management cell a host
  deadline (R2-19).
- Proposed destination after S4, if the owner rules O3 in: same-event
  feeds that Jev labels with high probability go in as system acts with no
  model call, and intake tasks handle only new and uncertain captures
  (R2-22).

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
  The inbox must not be spelled `management:<anything>`:
  `WakeRecordsInbox` never wakes a management id other than the
  persistent one (`inbox_records.go:642-645`). (Round 1 cited `:66-68`
  and `:95` here; those lines check sends *to* management.) The full
  identity is in §3.1a (R2-1).
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

Reuse the patterns, not the type (R2-7). Engineering assignments already
have the pieces:
- an assignment row bound to a parent round;
- a run started on the Cast, with a spawn wake of a restart work kind;
- the closing record;
- crash and ended-run closures.

(`internal/agentcore/engineering_assignment_runtime.go:163`,
`inbox_records.go:565-590`, `:800-905`.)

Round 1 proposed generalizing that row to a kind. That does not work
as stated: `EngineeringAssignmentBinding.Validate` requires a trajectory,
a runtime-derived parent decision, a control id and an assigned agent
(`internal/types/engineering_assignment.go:119-139`), and an intake task
has no trajectory or capsule. So a `management_task` row lives in the same
store family with its own `Validate`. Its fields: owner, computer, task id,
kind, parent run, task conversation, parent conversation and round (answer
tasks only), deadline, leased refs, state. It uses the same wake kinds and
closures. L5 (sub-researchers as runs) should land on the same row.

### 3.1a Task identity (R2-1, R2-2)

| | value |
|---|---|
| run profile | `management`. This keeps the profile-keyed behaviour: label overrides (`inbox_records.go:551-556`), the ended-run closure case (`:810-813`) and `recordsDesk` (`:32-38`). One exception: marker-boot passivation makes a spawned work item for any management run (`runtime.go:1492-1500`, `:2491`). A task-role run is excluded from that, and gets the §8 closure instead, because tasks are not resumed. |
| run role | a new role, `management_task`, carrying the task kind in run metadata |
| reader inbox and cursor | `management-task:<id>` |
| sender of acts to `texture:<doc>`, `texture:new`, `source:<id>` | `management:<owner>`, plus host-derived `acting_task` and run id. Forced by the code: `story_open.go:68-71` (only management opens a story) and `store/inbox.go:238-240` (only the caster of an open round may Cast into it). |
| sender of the task's closing Report to the parent | `management-task:<id>`. Only the round's recipient closes it (`store/inbox.go:232-234`). The host mints this Report (§5). |

`ReductionScope` gains a reader inbox separate from its sender. Every site
below changes in S2a, with a test through the real reduce and commit path:

- `recordsRoute` (`inbox_records.go:48-103`): routes from a task-role run
  to `texture:<doc>`, `texture:new` and `source:<id>` as management, and
  to `management:<owner>` (the closing Report only, minted by the host);
- `commitRecordsRoute`'s desk check (`:427`): accept the task reader inbox
  for a task-role run;
- `inboxRecordForIntent`'s sender (`:309`): the outward sender, not the
  reader;
- `WakeRecordsInbox` (`:631-686`): a `management-task:` prompt, the task
  row instead of an agent record, and the slot starter (§7) instead of a
  direct start;
- `closeRoundsOfEndedRun` (`:800-866`): close the task round, then mint
  the residual Report (§5);
- `recheckRecordsInbox` (`:708-748`): runs for task runs too, using the
  shared predicate (§5);
- the vocabulary, in one commit: `agentprofile.Canonical`
  (`agentprofile.go:91-103`, because `resolveRunIdentity` canonicalizes the
  role, `runtime.go:599`), `normalizeDeskRole` (`choir.go:172-178`,
  which today **fails open to engineering**), `deskModuleSets`
  (`choir.go:288-298`), and `NewChoirScope`'s action list
  (`choir.go:119-139`, which today gives every non-research role Exec,
  WriteFile and Assign). The task's action list keeps ReadCapture and
  Stories and excludes Exec, WriteFile, Assign and ProductAPI. A
  reverse-parity test asserts both the module set and the handle's
  actions.

### 3.2 Task kinds

| kind | given | done when |
|---|---|---|
| `intake` | one or more capture arrivals (their items and Jev labels) | every leased capture has a disposition (open, feed, decline) or is returned as a residual |
| `answer` | one Texture record (an Ask or a Note) and its conversation | the Ask is answered (Reply) or the Note acknowledged, or it is returned as a residual ("needs engineering", "needs research") |
| `verify` | one engineering assignment's final Report and bundle digest | open decision O4; not in the slices below |

Unit of intake: one arrival record (one poll; 475 / 133 = 3.6 captures
on average). Jev already marks same-event siblings inside a poll
(L2a R2-3), so a poll is a natural unit. From S4, a queued intake task
that has not started yet absorbs later arrivals, up to 12 captures
(R2-24; round 1 said 20). A burst then becomes fewer, larger tasks, and
same-event captures from different sources are more often decided
together. Absorption is a further Cast into the queued task's open round
by the same caster, which the round rules accept
(`store/inbox.go:236-241`). Records are immutable, so the lease is every
capture named by the round's Casts, not only the opening one.

### 3.3 How a task is opened

Two routes, landing in this order:

1. **Parent-directed, batched (R2-14).** `choir.Delegate(kind, objective,
   refs, grouping)` on management's surface stages one Cast per new task
   inbox. `refs` are record ids from the parent's `choir.Records()`.
   `grouping` is either `per_record` (one intake task per arrival, the
   default) or explicit groups up to the coalescing cap. One cell commits
   every task Cast and the cursor advance. The prompt delegates the whole
   backlog in at most two cells. One Delegate per cell would not do: at
   305a582d's 29 s per model call, 27 arrivals would cost about 13
   minutes of delegation alone. The Cast copies the
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
| Note or Cast to `texture:<doc>` | yes | yes, to any story, naming only its leased captures (R2-16) | Note to the leased conversation |
| Reply, ReportPacket to a Texture | yes | no | yes, leased conversation only; a final Report only when the round has no engineering assignment (D6, `inbox_records.go:565-590`) |
| Delegate, CancelTask | yes | no (one level, as research children: owner PD1) | no |
| Cast engineering, CancelAssignment, Verify | yes | no | no |
| ProductAPI | yes | no | no |
| Precommit, Resolve, Disagreement, Escalate, CancelAct | yes | no | no |

The parent keeps what changes computer-wide coherence: admitting
engineering, verifying a candidate, the owner's product routes
(schedules, settings, run acceptances), commitments on the ledger,
escalation to the owner, delegation and cancelling tasks. A task disposes
only the captures its lease names, and answers only the Texture round its
lease names.

**CancelTask (R2-20).** It calls the existing `CancelRun`
(`runtime.go:1604`) on the task's run. The ended-run closure (§8) then
closes the task round `run_cancelled` and mints the residual Report.
A commit the cancelled task had in flight fails the §4.2 precondition. A
queued task, which has no run, is cancelled by a system closure of its
round. The parent may re-delegate the residuals in the same or a later
cell.

### 4.2 Delegated sender

A task's records addressed to a Texture, to `texture:new` or to a source
carry `SenderInbox = management:<owner>`. A Texture therefore keeps one
conversation with management (`management.yaml:15`: "You hold one
conversation with every Texture of your owner"), and the round rules see
one caster. Provenance adds `acting_task: <task_id>` and the task's run
id. The host derives both from the run; the model does not supply them.
The store requires them on every record committed from a task-role
scope. Choir.Pack shows the act as management's, with the task named. The
task's closing Report to the parent is the one record sent from
`management-task:<id>` (§3.1a).

**The store, not the reducer, checks the lease (R2-4).** Round 1 put this
check in the reducer. That races: `commitRecordsRoute` reads conversation
state (`inbox_records.go:476`) well before it commits (`:534`). A deadline
or cancel closure can commit in between, and a feed Note into
`texture:<doc>` is in a different conversation from the task round, so
`stepConversation` would not catch it. So every commit from a task-role
scope carries a precondition: "round N of
`management:<owner>|management-task:<id>` is open, and every capture this
commit disposes is leased by it". The store checks it under `trajectoryMu`
(`store/inbox.go:578`), as conditions in the same `PutBatchConditional`
(`:587`), the way the cursor compare-and-set already works. A failed
precondition refuses the whole cell. This is a check on state the store
already holds (round heads and the disposition objects of §6.4), so the
store still scores nothing.

### 4.3 The lease is the open round

There is no lease table. A capture or Texture record is leased to a task
exactly while the task's round that names it is open (single authority,
standing question 5). The task row's leased refs are written in the same
commit as the round's Casts and are never edited apart from them. What a
lease grants (R2-16):

- a leased **capture**: the sole right to dispose of it (open, feed,
  decline);
- a leased **Texture record**: the sole right to Reply and to send the
  final Report in that round. Round 1 made a lease exclusive over the
  whole conversation. That contradicted §4.1, where an intake task feeds
  any story, and it would have blocked intake feeds into a Texture whose
  Ask was being answered. Feeds that name the feeding task's own leased
  captures go to any Texture. A Note after a round closes is a legal
  round-0 note (`store/inbox.go:245-253`).

Consequences:

- delegating a record already leased is refused by the store precondition
  (§4.2);
- the parent's own disposition of a leased capture, or its final Report
  into a leased round, is refused until the round closes or the parent
  cancels the task;
- a closed round (final Report, crash closure, deadline, cancel) frees
  the lease with no extra write.

## 5. How results return

- **The host mints the task's final Report (R2-8).** The model ends a task
  with a staged `choir.Done(note)`. If it does not, the deadline, a
  cancel, or the run's end does. The host then mints one final Report from
  `management-task:<id>` to `management:<owner>`, which closes the task
  round. This is how engineering's Complete already mints its final Report
  (`inbox_records.go:325-331`). The body lists:
  - the record ids committed per capture;
  - **residuals = leased refs minus durable dispositions**, computed from
    §6.4's disposition objects, each with the model's reason if it gave
    one;
  - the error, if any, and the tokens used.

  The crash boot closure carries no capture list
  (`types/commitment.go:442-456`), so AC5's join uses the same
  computation: the round's leased refs minus their dispositions.
- **A clean final Report does not wake the parent.** A Report is clean
  when its residual set is empty and the task had no error. The host
  computes that; the model does not claim it. A clean Report is recorded
  and stays unread until the parent's next turn, like a support pass that
  writes only its memo. A Report with residuals, a failure, or a deadline
  closure wakes the parent.
- **One wake predicate (R2-3).** Round 1 put the rule only in
  `inboxRecordWakes` (`store/inbox.go:209-219`). The post-turn re-check
  starts another turn for *any* unread record committed after the ending
  turn began (`inbox_records.go:735-742`), so clean Reports landing
  mid-turn would still re-arm the parent. Both paths call one predicate,
  `recordWakesReader(rec)`. Without it, every task would wake a parent
  turn and the serial queue would come back (failure mode 2). Test: three
  clean Reports land during a parent turn and no new turn starts; one
  residual Report starts exactly one turn.
- The parent reads residuals in a later turn and acts: re-delegates,
  casts engineering for an answer task's "needs engineering", or
  escalates.

## 6. Ordering and conflicts

1. **Two tasks open a story for the same event.** Example: two sources'
   polls report one new event in the same minute, and both intake tasks
   cast `texture:new`. Guard: an optimistic check on the story library
   (R2-5). Today `planStoryOpenings` opens a story for each `texture:new`
   Cast, conditioned only on its new ids being absent
   (`store/story_open.go:56-95`), so the check needs plumbing:
   - the store keeps a computer-scoped library head (a count of story
     opens), advanced in the commit of each `texture:new` Cast;
   - the host records the head per run when it serves `choir.Stories()`,
     as `recordsRunShown` works, and stamps it on every `texture:new`
     intent. A run that never read `Stories()` has seen head 0;
   - the store compares the stamped head with the current head under the
     commit lock. If the stamped head is behind, it refuses the whole cell
     with the stories opened since (headline, captures);
   - the check applies to the parent's `texture:new` too, until S5.
     Otherwise a parent and a task can still open duplicates.

   The task re-decides in its next cell: feed one of the listed stories,
   or cast again with `distinct_from: [doc ids]`. That commits, and is
   recorded as an override for Jev's examples (L2a A3) and counted by AC4.
   The task prompt teaches "read Stories() before opening". With 4 tasks
   and about 27 arrivals in the busiest hour, refusals should be rare.
   Hypothesis H5 is measured at the minute of a burst, not averaged over
   the hour. The store does not judge whether two captures report the same
   event; that is a judgement, and the store scores nothing.
2. **Two tasks feed the same story.** Feeds are Notes or Casts into
   `texture:<doc>`. Each record takes the next sequence in that inbox
   inside its own commit, so they serialize, and the Texture reads both in
   one or more turns. A Cast into an open round is accepted from the same caster
   (`store/inbox.go:236-240`); both tasks send as `management:<owner>`.
   Tasks may add hypotheses but not withdraw them, so one task cannot
   withdraw another's.
3. **One capture in two tasks.** Impossible while the lease is the open
   round (§4.3).
4. **A capture disposed twice** (a late commit racing a closure and a
   re-delegation). Guard (R2-4): a computer-scoped disposition object,
   keyed by capture id, written in the batch that carries the disposition
   record and conditioned on being absent. A second disposition fails the
   condition under the commit lock. The parent's explicit re-disposition
   replaces the object conditionally on the old version and is recorded as
   an override. The object indexes the disposition record, which stays the
   content; the two commit atomically, so there is one authority. The
   object also feeds AC1's join and the residual computation (§5).
5. **The store's single writer.** Every commit serializes on the engine
   lock (`internal/objectgraph/engine_mutex.go:10-14` already records
   wait and hold time per caller). Tasks hold no lock across a model call;
   a commit is milliseconds. Hypothesis H3: at K=4, lock waits stay small;
   the engine-mutex metrics measure it, and the read/write lock (SA slice
   4) waits on that measure.
6. **Parent and task on one Texture conversation.** Only the lease holder
   may Reply or send the final Report in a leased round (§4.3). Others may
   send Notes. The parent finalizes after the round closes or after
   CancelTask.
7. **The parent's run ends while an answer task holds a Texture round
   (R2-13; S3).** Today, when a management run ends,
   `closeRoundsOfEndedRun` closes every round where management is the
   recipient. Only engineering assignments hold a round open
   (`inbox_records.go:836-842`, `childAssignmentsHoldRound` `:869-899`).
   Once the parent delegates an Ask and ends its turn, the Texture round
   would close `run_completed`, and the task's answer would then be
   refused, because the round is no longer open
   (`store/inbox.go:229-230`). Guard: the answer-task row binds the
   Texture round it serves (parent conversation and round).
   `childAssignmentsHoldRound` and D6's `requireChildOutcomesRead`
   (`:565-590`) cover open task rows as well as engineering rows. This is
   S3's first test.

## 7. Concurrency and admission

- **K tasks run at once per computer, the rest queue; nothing is
  refused.** Start K = 4, as research children
  (`research_children.go:25`) and intake labels
  (`internal/agentcore/intake_labels.go:44`). A queued task is a
  committed round whose run waits for a slot; its records are leased and
  visible as queued.
- **The slot starter is durable and derived (R2-6).** Today
  `WakeRecordsInbox` starts a run for any desk with unread records and no
  active run (`inbox_records.go:656-665`). Nothing counts slots, so a
  burst would start one run and one worker per task. The starter works
  like this:
  - a queued task is an open task round with no run. It is not an
    in-memory queue;
  - its start is an outbox wake of a kind listed in `RestartWorkKinds`
    (`planned_restart.go:161-173`), modelled on engineering's
    `delegated_assignment_spawn_deadline`. A crash boot therefore
    interrupts it; a marker boot delivers it once. Boot itself delivers
    nothing (`runtime.go:793-797`);
  - the starter takes a slot when one is free, and runs again when a task
    run ends and on each such wake. No run and no worker exist until a
    slot is held.

  S0's script reports the queue depth and the oldest queued age, from the
  open task rounds.
- **Intake reserve (R2-18, from S3).** Two of the K slots are reserved for
  intake tasks, so answer tasks on a slow model cannot starve the paper's
  arrival path.
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
  (`inbox_records.go:756-795`). Pre-boot task runs close with the same
  fate (`closePreBootRunsAfterCrash`, `runtime.go:768`). Leases free with
  the rounds. Captures a crashed task had not disposed keep the visible
  fate "interrupted by a restart". The closure record lists no captures,
  so that fate is derived as leased refs minus dispositions (§5). They
  are not re-delegated at boot. The parent's next live turn reads the
  closure and may re-delegate them; that is new work by a live turn. The
  standing intent (the source schedule) survives; the occurrence does not.
- **Crash budget per capture (R2-17).** A capture whose task round was
  crash-closed twice is closed with that cause and is never re-delegated,
  as a source schedule whose tick crashes the computer twice is closed
  (CLAUDE.md, "Restarts End Work"). Otherwise a capture that kills the
  guest would crash it once per poll tick, through live re-delegation.
  The count is a field on the capture's disposition object (§6.4).
- **Run ends without a final Report** (failed, deadline, cancelled). The
  ended-run closure gives the round a system final Report and wakes the
  caster (`inbox_records.go:800-868`). For a task the caster is the
  parent; the closure lists the leased records still unsettled.
- **Planned restart (R2-12).** Tasks are not resumed in v1.
  - **Drain.** Before a planned restart the runtime stops starting queued
    tasks and lets running tasks finish within their deadline (at most 10
    minutes). The host writes the marker and restarts only once no task
    is running. The drain never forces a restart on a busy computer:
    deploys update idle computers and leave busy ones to their next idle
    moment (CLAUDE.md, "Restarts End Work").
  - **A task that is running anyway at a marker boot.** This happens on a
    drain timeout or a restart requested outside the drain. Today such a
    run would be passivated with its round open and no wake: a marker boot
    passivates and closes no rounds (`runtime.go:765-770`, `:2481-2511`).
    So at a marker boot, a passivated task run gets the ended-run closure
    (`run_passivated`), and the host mints its residual Report (§5),
    which wakes the parent. The consumed marker permits that one wake.
    The task itself is not resumed.
  - **Queued tasks** start once through §7's outbox wake after a marker
    boot. If that boot then crashes, the crash boot closes them.

  This gives an always-busy paper a bounded way to become idle, which the
  open problem in
  `docs/problems/planned-refresh-closes-paper-schedules-2026-10-14.md`
  ("how an always-busy computer takes updates") needs. Who requests the
  drain (the deploy's refresh) is the owner's deploy-shape decision (O6).
  S2 does not depend on it.

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
  is the owner's call (O3) and not in the slices. If the owner rules it in,
  it is the proposed destination after S4 (R2-22). S4's standing route is
  designed label-first, so the route can send high-probability same-event
  captures to a system feed and only new and uncertain captures to intake
  tasks.
- **Model for tasks (R2-9, an S2 deliverable).** Bounded intake needs a
  fast model. The problem doc records free-preview calls taking "from
  seconds to nine minutes per call" (problem doc, line 53). One such call
  would use most of a 10-minute task deadline, and the re-delegations that
  followed would thrash. Run 305a582d on deepseek-v4.1-flash averaged 29 s
  a call. So S2 adds a `management_task` role to the model policy:
  - pinned to a non-preview, bounded-latency model; the proposed default
    is deepseek-v4.1-flash;
  - with a per-call timeout below the task deadline;
  - a free preview model cannot be selected for it.

  The owner may override the model (O5; model routing is red). The
  deadline stays at 10 minutes. Raising it would hold leases longer and
  hide a slow model; AC9 measures how often the deadline fires.

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
   14-minute stuck cell seen in 305a582d. It does not bound a model call
   between cells. So AC2's maximum is measured, not guaranteed: it also
   depends on the model's latency, which S0 records per turn.
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
   Guard: the lease is the open round; refused by the store
   precondition (§4.2); tested.
5. A task acts outside its lease (other captures, other conversations,
   engineering, ProductAPI). Guard: the task module sets plus the store
   precondition; reverse parity test (as `choir_parity_test.go:422-449`) for
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
12. A capture is disposed twice (late commit after a closure). Guard: the
    disposition object (§6.4) refuses a second disposition under the
    commit lock unless the parent re-disposes it explicitly.
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
    separately in AC5 and bounded by AC1 (at most 2%). The parent may
    re-delegate them on a live turn, at most once more (R2-17).
17. Tasks cost more than the serial turn. Guard: AC7; tokens per task
    recorded as runs.
18. The owner cannot see the tasks. Guard: tasks are runs in
    `/api/runs`; open task rounds show in the inbox route's
    `open_rounds`.
19. A task run is not recognized by a profile-keyed switch: it gets no
    route, no wake, no ended-run closure or no re-check. Guard: the §3.1a
    site list, each tested through the real reduce and commit path (R2-1).
20. The task role falls open to engineering's module set and filesystem
    actions (`choir.go:172-178`). Guard: one commit registering the role
    everywhere, and a reverse-parity test on the handle's actions (R2-2).
21. The post-turn re-check re-arms the parent on clean Reports. Guard: one
    wake predicate (§5, R2-3).
22. A late commit lands after the closure that freed the lease. Guard: the
    store precondition (§4.2, R2-4).
23. A burst starts unbounded runs. Guard: the durable slot starter (§7,
    R2-6).
24. A running task is stranded passivated at a marker boot. Guard: the
    `run_passivated` closure with residuals (§8, R2-12).
25. The parent's run end closes a Texture round an answer task holds.
    Guard: task rows hold the round (§6.7, R2-13; S3).
26. A capture crashes the guest on every re-delegation. Guard: the crash
    budget (§8, R2-17).
27. The task model is too slow for the deadline, and tasks thrash. Guard:
    the pinned model (§9, R2-9) and AC9.

Failure mode 13 (D6 on answer tasks) and failure mode 25 are tested in S3,
not S2 (R2-15).

## 12. Acceptance (round 2: R2-10, R2-11, R2-15)

The criteria are measured on the product path, by S0's script over
`/api/runs`, `/api/runs/<id>/events` and the inbox routes. Round 1's
criteria could be met by declining quickly. Its crash exclusion was
unbounded. Nothing in it asserted that the parent stopped serializing.

- **AC1 latency (opens and feeds).** Live paper, 14 sources at 15-minute
  intervals, intake.labels on, a 6-hour window. For every capture, the
  latency runs from arrival (the arrival record's `committed_at`) to its
  first effective disposition (from §6.4's disposition object). It is
  reported per kind: open, feed, decline. **The bound holds for opens and
  feeds alone: p95 at most 5 minutes, max at most 15 minutes.** Declines
  cannot satisfy it. **Crash-closed captures are at most 2% of the
  window's captures**, and every crash in the window is named with its
  cause. Baseline (§2.3, estimated): p95 about 21-25 minutes, max about
  35.
- **AC2 short parent.** Same window. Only parent runs count: agent id
  `management:<owner>`, not task-role runs. **No parent turn longer than
  10 minutes; p95 at most 2 minutes.** Each turn is reported with its
  model, call count and call durations, so a miss can be attributed (the
  cell deadline does not bound a model call, §10). Baseline: p95 812 s,
  max 3,601 s.
- **AC3 stalled-task drill.** Minted test computer with the paper
  fixture, 4 sources polling. In S2, an intake task's model fake stalls
  until the task deadline (10 minutes). The deadline closes the round,
  frees the lease and returns residuals, which the parent re-delegates.
  AC1 (opens and feeds), AC2 (intake-only turns) and AC10 hold
  throughout. In S3 the drill repeats with K stalled answer tasks, which
  also tests the intake reserve. (Round 1's "held for 20 minutes"
  contradicted the 10-minute deadline.)
- **AC4 coherence.** On the fixture, stories per event are no worse than
  the serial baseline. **No two stories for one judge-scored event across
  the whole window** (`scripts/l2a_paper_window.py` pairs), except
  recorded `distinct_from` overrides. Those are counted and judge-scored
  separately.
- **AC5 completeness.** Every capture has exactly one effective
  disposition, or the visible fate "interrupted by a restart", or "closed
  after two crashes" (R2-17). A parent re-disposition counts as the
  effective disposition and is listed separately as an override.
  Unsettled captures are derived as leased refs minus dispositions (§5).
- **AC6 crash.** Kill the guest with at least 2 tasks running and 1
  queued. The boot starts no task. It closes their rounds and their runs
  `interrupted_by_restart`, wakes no one, and frees their leases. The
  next poll's arrivals get new tasks.
- **AC7 cost and memory.** Parent plus task input tokens per disposed
  capture is at most 1.2 times the S0 baseline window, **on the same
  pinned model mix** (or reported per model when the mix differs). Total
  task tokens and cost are reported. Peak concurrent task workers is at
  most K.
- **AC8 disposition mix (new).** Opens, feeds and declines per arrival
  each stay within ±10 percentage points of the S0 baseline window. A
  judge-scored sample of 30 declines finds no more wrong declines than the
  same sample from the baseline.
- **AC9 task completion (new).** At least 90% of tasks end with a clean
  Report. Deadline closures, cancels and failures are reported per kind.
  A deadline-closure rate over 10% fails the model choice (§9), not the
  deadline.
- **AC10 no serialization (new).** Parent run starts in the window are at
  most the number of records that wake the parent (Texture requests,
  residual or failed task Reports, engineering, owner), and fewer than
  the number of tasks. Each parent start is reported with the record
  kinds that woke it. Clean task Reports appear in that breakdown zero
  times. After S4: **no captures record is ever addressed to
  `management:<owner>`.**

Fixtures and the paper need intake.labels on, or Feed and Decline are
refused (`inbox_records.go:432-439`).

## 13. Slices, in order (round 2: R2-11, R2-19, R2-21)

Order: **S0, S1, S2a, S2b, S4, S3, S5.** Each slice is its own commit
series, pushed, CI'd, deployed and run on the paper before the next (the
Landing Loop).

- **S0, instruments (yellow).** The 860 s stuck cell is already recorded
  in the problem doc (problem documentation first). S0 extends
  `scripts/l2a_paper_window.py` into a script over product routes that
  reports:
  - the exact AC1 join, per disposition kind;
  - AC2 split into all parent turns and intake-only turns, with model,
    call count and call durations per turn;
  - the AC8 mix;
  - the AC10 wake breakdown;
  - the cell-duration distribution (p50, p99, max) of management cells;
  - queue depth and oldest queued age, once tasks exist.

  Then it takes the baseline window. S0 needs no owner decision.
- **S1, host cell deadline (red, small; independent of delegation).** A
  default deadline on management cells when `timeout_ms` is unset.
  - The default is **max(300 s, 2 × S0's p99 cell duration)**, tightened
    later on evidence. Round 1's 120 s could kill legitimate parent cells,
    and a kill takes the worker and the session's values
    (`internal/yaegikernel/desk_session_worker.go:367-369`).
  - It is applied as `evalCtx`'s deadline (`tools_desk.go:442-446`).
    `armCellTerminalDeadline` keys on the outer context
    (`continuation_schedule.go:91-107`), so the deadline goes on the eval
    context, with the existing timeout handling (`tools_desk.go:475-485`).
  - Every kill is logged with the worker's exit cause, which settles H1.

  S1 closes the 14-minute stuck cell of 305a582d whether or not S2 lands.
  It needs no owner decision. Rollback: revert.
- **S2a, the task substrate at K = 1 (red).**
  - the `management_task` row (§3.1);
  - the identity and every switch in §3.1a;
  - the task role registered everywhere, with the reverse-parity test;
  - batch `choir.Delegate`;
  - the task run with its own session;
  - the outward sender and the host-minted closing Report;
  - the store lease precondition and the disposition objects;
  - one wake predicate for the store and the re-check;
  - crash, deadline, cancel, ended-run and marker-boot closures;
  - the crash budget per capture;
  - the pinned `management_task` model role.

  K = 1 means no concurrent opens yet. Tests first: failure modes 1-12,
  14-24, 26 and 27. Prompt: delegate captures, in at most two cells.
  Paper run.
- **S2b, concurrency (red).** K = 4, the durable slot starter and queue
  metrics, the library head (for the parent's opens too), and the shared
  spacer. Paper run. **S2's exit (S2a and S2b):** AC2 on intake-only parent
  turns, plus AC4, AC5, AC8, AC9 and AC10 on the paper, and the S2 AC3
  drill on a minted computer. AC1 is measured and reported but does not
  gate S2: in S2 the arrival still waits for a parent turn
  (`capture_arrival.go:59-65` addresses it to `management:<owner>`).
- **S4, standing intake route (red).** The `management.intake_tasks`
  setting (off by default; set by an owner act). With it on:
  - `capture_arrival.go` addresses arrivals to intake task inboxes, and
    queued tasks coalesce up to 12 captures (§3.2);
  - the route is designed label-first, so O3 can plug in (§9);
  - the parent stops receiving capture arrivals.

  Fixture first, then the paper. **The full AC1 and AC10's "no captures
  record to the parent" gate S4.** S4 comes before S3 because the paper's
  arrival path is the problem doc's live symptom, and S4 depends only on
  S2.
- **S3, answer tasks (red).** A Texture's Asks and Notes go to answer
  tasks. "Needs engineering" comes back as a residual. First test: an
  answer task holds its Texture round against the parent's ended-run
  closure and D6 (§6.7, R2-13). Failure modes 13 and 25 are tested here.
  From S3, two slots are reserved for intake. The S3 AC3 drill uses K
  stalled answer tasks. **The full AC2 gates S3**, since Ask turns are
  what still run long after S2.
- **S5, cutover (red).** The parent loses Feed, Decline and `texture:new`.
  It keeps ReadCapture, read-only (O9). Prompts are rewritten and covered
  by parity tests. This closes the dual path.
- Not scheduled: `verify` tasks (O4); Jev auto-feeds (O3, the owner's);
  L5 research children moving onto the same row (cut 1 L5, C1e step 5).

Rollback: each slice reverts whole.
- **S4** rolls back by turning the setting off. New arrivals then go to
  `management:<owner>` and the parent handles them with the S2 path. Open
  task rounds run to completion or to their deadline; they are not
  stranded.
- **An S2 revert** is a marker boot, which closes no rounds. So before
  reverting S2, turn S4's setting off, stop delegation in the prompt, and
  wait until no task round is open (`open_rounds` on the inbox route).
  Only then revert.

## 14. Ceremony (red)

- **Conjecture delta.** Before: management's coherence role required
  that one serial turn do every piece of management work. After:
  coherence needs the parent to hold admission, commitments and the
  computer's settings. Disposing of captures and answering a Texture's
  requests are bounded tasks that run in parallel under leases the store
  enforces. Falsified if:
  - concurrent tasks fragment stories (AC4 fails);
  - tasks meet latency only by declining (AC8 fails);
  - the parent keeps serializing on task Reports (AC10 fails);
  - the parent cannot keep coherence from task Reports.
- **Protected surfaces.**
  - `internal/agentcore/inbox_records.go`: routes, the reader and sender
    split, wake, re-check, cursor, closures, D5 and D6;
  - `engineering_assignment_runtime.go` (the patterns reused) and the new
    `management_task` row;
  - `tools_desk.go`: sessions, the cell deadline;
  - `research_children.go`: the spacer;
  - `runtime.go`: marker-boot passivation, `CancelRun`;
  - `planned_restart.go`: the slot-start wake kind;
  - `internal/store/inbox.go`: the wake predicate, the lease
    precondition, disposition objects;
  - `internal/store/story_open.go`: the library head;
  - `capture_arrival.go`: the S4 address;
  - `internal/agentprofile`: the role token;
  - `internal/yaegikernel/choir.go` and `surface.go`: the role, module
    sets, handle actions, Delegate;
  - the model policy: the `management_task` role;
  - management prompts and the overlay;
  - gateway call pacing.
- **Admissible evidence.** Local store-backed tests through the real
  reduce and commit paths with model fakes, for the failure modes.
  Acceptance comes only from deployed proof: the paper's records and runs
  through product routes (AC1, AC2, AC5, AC7, AC8, AC9, AC10), and a
  minted test computer for the drills (AC3, AC4, AC6).
- **Rollback.** Revert per slice. S4's setting off. An S2 revert waits for
  no open task round (§13).
- **Heresy delta.**
  - **Discovered:**
    - management is a concurrency limiter, against I26;
    - a management cell can run 14 minutes with no host bound;
    - research children are not runs and block their parent cell (already
      named by cut 1 L5);
    - `normalizeDeskRole` fails open to the engineering module set
      (`choir.go:172-178`);
    - the post-turn re-check ignores the wake predicate
      (`inbox_records.go:735-742`).
  - **Introduced** (a risk; the intended count is none): a delegated
    sender, where `management:<owner>` makes acts that a task authored.
    Mitigated by host-derived `acting_task` provenance, which the store
    requires on every task-scope record.
  - **Repaired** (when S2-S5 land): management stops being the paper's
    concurrency limit, and the parent's intake dual path is deleted.

Standing questions, briefly (numbered as in
`docs/standing-questions.md`):
- (1) The direction is the owner's, 03:55 UTC.
- (2) It follows the settled records model, the restart rule and L2a,
  except O3, which would change L2a.
- (5) The lease is the open round. The disposition object indexes a
  record in the same commit, so it is not a second authority.
- (6) Success is measured from records and runs, not narration.
- (8) Tasks hold nothing durable beyond records and their row. A crash
  closes them.

## 15. Open decisions (round 2: adjudicated)

| | answer | whose call |
|---|---|---|
| O1 Routes | Both. The standing route carries captures (S4), the 24/7 path. Delegate carries Texture requests, residuals and re-delegation. Parent-directed intake is the S2 canary, and after S4 only a way to recover residuals. | settled |
| O2 Sender | Outward acts go as `management:<owner>` with host-derived `acting_task`. The closing Report goes from `management-task:<id>`. Forced by the code (`story_open.go:68-71`, `store/inbox.go:232-240`). | settled |
| O3 Jev auto-feed | Not in this mission. It is the proposed destination after S4, and it changes L2a's settled "labels are advice; management decides". | **owner** |
| O4 Verify | Verify stays with the parent. | settled |
| O5 Model and pacing | A `management_task` role pinned in S2 to a non-preview, bounded-latency model (proposed: deepseek-v4.1-flash), with a per-call timeout. Tasks join the shared sub-RLM spacer (40 a minute), leaving about 20 a minute for root desks. A reserved task share waits for S0 to show 429s. | default proceeds; **owner may override the model** |
| O6 Planned restart | Drain, then restart, with no task resume in v1. A task passivated anyway gets a residual closure that wakes the parent. Queued tasks start once. A busy computer is never forced (§8). | Mechanism settled. **Owner:** who requests the drain and how a deploy waits for an always-busy paper (deploy shape). S2 does not depend on it. |
| O7 Numbers | K = 4 (S2a K = 1). From S3, 2 slots reserved for intake. Task deadline 10 minutes. Cell deadline max(300 s, 2 × S0 p99). Coalescing cap 12. All are starting values, to be measured. | settled as starting values |
| O8 Clean Reports | A clean Report does not wake the parent. "Clean" is computed by the host, and one predicate serves both wake paths. | settled |
| O9 ReadCapture | The parent keeps it, read-only. Revisit if AC2 fails after S3. | settled |

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
- H6 (R2-25): story opens and Cast feeds by tasks still wake the parent,
  through each Texture's final Report to `management:<owner>` (a desk
  record always wakes its recipient, `store/inbox.go:215-217`). Parent
  turns get shorter, but they may not get fewer. AC10's wake breakdown
  measures it. If these wakes dominate, the wake predicate is extended to
  a Texture Report answering a task-authored Cast, which would be read on
  the parent's next turn.

## Owner rulings, 2026-10-14 (about 10:40 UTC)

- **O3 (Jev settles high-confidence feeds): yes.** "let [Jev] settle high
  confidence feeds." This amends L2a's earlier "labels are advice": a Jev
  label at high confidence that names an existing story becomes a system
  Feed without a model call; management (or its tasks) still decides
  everything else, and every system Feed is a recorded act that can be
  overridden. Deterministic intake is the destination named in the
  delegation design (R2-22).
- **Snapshot hibernation resumes overnight:** at least 24 h (set to 36 h,
  `VM_SNAPSHOT_MAX_AGE`).

## 17. S2a build record (branch `management-delegation-s2a`, not pushed)

Built 2026-10-14 on `origin/main` `e3437f1cd` (S0, S1 and the failed-turn
records fix landed). Nothing is pushed or deployed. Commits, in order:
`4a7590209` (role vocabulary), `aa9aa1234` (store), `ad1435543` (problem
doc), `16a64bae0` (runtime), `452c1961c` (prompts), `e9bd9bc15` (probe).

### 17.1 What each amendment became in code

| amendment | code |
|---|---|
| R2-1 identity | Task run: profile `management`, role `management_task`, agent and reader inbox `management-task:<id>`. `outwardSender` sends as `management:<owner>`; `recordsRoute` has a task case (stories, `texture:new`, sources; engineering and management refused); `commitRecordsRoute` accepts the task reader inbox; `loadCellRecords` shows the opening Cast and the delegated captures records cut to the lease; `WakeRecordsInbox` on a task inbox is the slot starter; `closeRoundsOfEndedRun` and `terminalizeRun` close a task run's task; `recheckRecordsInbox` re-arms nothing for a task. The parent run is provenance only (`management_task_parent_run_id`, not `requested_by_run_id`, whose terminal-outcome machinery would hang a task on its parent). |
| R2-2 role in one commit | `agentprofile.ManagementTask` (Canonical; not IsLive, so nothing addresses it), `yaegikernel.SessionRoleManagementTask` with module set Cast, Note, ReadCapture, Stories, Feed, Decline, Done and handle ReadFile, ListDir, ReadCapture, Stories. `normalizeDeskRole` fails closed: an unknown or empty role gets observation exports and a ReadFile/ListDir handle. Reverse-parity tests on exports, denied verbs and handle actions. |
| R2-3 one wake predicate | `store.RecordWakesReader`; `inboxRecordWakes` calls it and the re-check skips late records it rejects. A clean task Report and a crash closure wake no one; a Report with residuals, an error or a deadline wakes the parent. |
| R2-4 store lease, disposition objects | `planManagementTasks` in `planInboxCommand` under `trajectoryMu`: task-scope records need `acting_task`, the row's run, a running row and an open round; `choir.capture_disposition` (per capture) refuses a task's second disposition, a disposition outside the lease, and anyone else's disposition of a leased capture; a parent re-disposition of a free capture is kept as an override. Task conversation heads read are conditioned in the batch. |
| R2-5 library head | `choir.story_library_head` advanced by each open; a management or task cell that opens a story carries the head its run last read with `choir.Stories()` (first page), and a stale head refuses the cell naming up to 20 stories opened since; the refusal itself counts as having seen the head. Applies to the parent too (with intake labels on; with them off `choir.Stories()` is off and nothing is checked). |
| R2-6 durable slot starter | A queued task is an open task round with no run. Its start is the `inbox_record` wake of its opening Cast; a task's closure mints, in its own commit, an `inbox_record` wake to the oldest queued task (not at a crash boot). `startQueuedManagementTasks` claims under one mutex, starts the run, binds it. K = 1 (`managementTaskSlots`). |
| R2-7 own row | `types.ManagementTask` (`choir.management_task`) with its own `Validate`, committed with its opening Cast; no trajectory, capsule or verifier fields. |
| R2-8 host-minted Report | `types.NewTaskClosingReport` (system, from the task inbox, final); the store fills `task_outcome` (dispositions, residuals = leased minus disposed, clean) from the index; Done's note is each residual's reason. Tokens are not in the outcome (they are on the run's events). |
| R2-9 pinned model | `modelpolicy`: role `management_task` resolves to opencode-go/deepseek-v4.1-flash with no failover; a pool, free or preview model named for it is not honoured; an explicit provider and model is (O5). Per model call: `WithToolLoopProviderCallTimeout(3 min)`. |
| R2-12 planned boot | `passivateInterruptedActivations` ends a task run (cancelled, not parked) and closes its task `run_passivated` with residuals (one parent wake); the queued task's start wake is minted by that closure. The drain itself (O6) is not built. |
| R2-14 batch Delegate | `choir.Delegate(kind, objective, refs, grouping)`: refs are captures record ids or a task Report's id (its residuals again); `per_record` or explicit groups, at most 12 captures a task; disposed or leased captures are left out. One parent cell commits every row and Cast with its cursor. |
| R2-17 crash budget | A crash closure of a task round adds one to each undisposed leased capture's crash count; at two the capture's fate is `closed_after_two_crashes` and delegation refuses it. |
| R2-20 CancelTask | Running: `CancelRun`, and `terminalizeRun` closes the task `run_cancelled`. Queued: the round closes `cancelled`. Both Reports list the residuals. |
| R2-19 (S1), R2-15 (AC3), R2-16, R2-24 | S1 landed before. Task cells get management's host cell deadline. AC3 needs the minted drill (below). R2-16: a task feeds any story, naming only its leased captures. R2-24 coalescing is S4. |

Choices made without the owner (No Blocking Asks):
- **K = 1 and the library head both in S2a.** §13 puts K = 1 in S2a and the
  library head in S2b, while its S2a test list includes failure mode 3; the
  head was built here, K stays 1.
- **Re-delegation by Report id.** §5 says the parent re-delegates residuals;
  Delegate takes a task Report's id for that, since the arrival records
  are already read.
- **Parent re-disposition is implicit.** Any parent disposition of a free,
  already-disposed capture is an override; there is no separate verb.
- **Task Report kind** reads `task_report` in `choir.Records()`, with
  `r.TaskOutcome` on the view.

### 17.2 Tests (all through the real reduce and commit paths)

- `internal/yaegikernel/management_task_role_test.go`: reverse parity,
  unknown roles fail closed, task prompt teaches only task verbs.
- `internal/modelpolicy/management_task_model_test.go`: the pin (FM27).
- `internal/store/management_tasks_test.go`: FM2, FM3, FM4, FM5, FM6,
  FM12, FM22, FM26, R2-3 predicate, R2-6 next-task start.
- `internal/agentcore/management_tasks_test.go`, real desk workers: FM1,
  FM2, FM6, FM9, FM11, FM19, FM21, FM23, FM24, FM27, AC6, N5, R2-20 and
  re-delegation, FM3 from a task cell.
Not tested in S2a: FM7/FM8 at K > 1 and FM14 (shared spacer) are S2b;
FM10's drain is O6; FM13 and FM25 are S3; FM15 is S5.

### 17.3 What remains for S2b

- K = 4, with the shared sub-RLM call spacer for task model calls (FM14).
- A per-run token count in the task Report (AC7 reads run events now).
- R-CANCEL: research and management runs ended through `terminalizeRun`
  still skip `closeRoundsOfEndedRun`
  (`docs/problems/cancelled-run-skips-round-closure-2026-10-14.md`).
- The drain before a planned restart (O6, the owner's deploy shape).
- Per-capture residual reasons (Done's note is shared by all residuals).

### 17.4 Staging acceptance for S2a (after push, CI and deploy)

1. Verify the deployed commit on `choir.news` health; the paper computer
   is on the build after its next idle moment.
2. Confirm intake.labels is on for the paper (N13); without it Delegate
   is refused.
3. Watch one burst: `/api/runs?channel_id=management:<owner>` shows
   parent turns ending after a Delegate cell; task runs appear under
   `channel_id=management-task:<id>` on deepseek-v4.1-flash, one at a
   time (K = 1).
4. After a window of at least 6 hours, run
   `scripts/management_latency_probe.py --cred <paper cred> --start <deploy+idle> --counts-only`
   and read: AC2 `intake_only_estimate` (p95 at most 120 s, max at most
   600 s; gates S2 together with S2b), AC8 mix within ±10 points of the S0
   baseline, AC9 `tasks.ac9_clean_share` at least 0.9 and
   `ac9_deadline_share` at most 0.1, AC10
   `ac10_parent_starts_by_clean_reports_only` = 0, `tasks.queue_depth_now`
   and `oldest_queued_age_s`, AC1 reported (not gating in S2), and AC5 via
   `ac1_latency_s.undisposed` plus task residuals.
5. AC4: `scripts/l2a_paper_window.py` pairs over the window; no two
   stories for one judge-scored event except recorded overrides.
6. Drills on a minted test computer with the paper fixture (AC3, AC6):
   a stalled task model to its deadline (residual Report, re-delegation),
   and a guest kill with one task running and one queued (rounds and runs
   `interrupted_by_restart`, no wake, the next poll delegates anew).

## 18. S2b: concurrency (branch `management-delegation-s2b`, not pushed)

Problem record first:
[management-intake-backlog-at-28-sources-2026-10-14](../problems/management-intake-backlog-at-28-sources-2026-10-14.md)
(149 of 154 captures undisposed after 34 minutes at 28 sources; one task
at a time, task p50 241 s, queue wait p50 425 s). S2b is §13's slice:
K = 4, the slot starter and queue at K > 1, the library head under real
concurrency, and the shared sub-RLM spacer. Red (management runtime,
gateway call pacing). Nothing pushed or deployed.

### 18.1 Failure modes (written before any code)

Numbers continue §11 where S2b adds a way to fail; §11's numbers are reused
where S2b exercises them at K > 1.

- **FM23 at K > 1.** A burst starts more than K task runs: two wakes run
  the starter at once and both see a free slot. Guard: the starter's
  mutex, the claim's compare-and-set, and a claimed task with no run yet
  counts as busy. Test: six queued tasks, concurrent wakes, exactly K
  running.
- **FM8, the queue stalls.** Queued tasks wait while slots are free,
  because nothing starts them. Guard: every non-crash closure mints a
  durable start wake (S2a), and one wake fills every free slot, not one.
  Test: the queue drains through the outbox wakes alone, with the peak of
  running tasks equal to K.
- **FM3 under real concurrency.** Two running tasks read the library at
  the same head and both open a story for one event. Guard: the library
  head (S2a, R2-5), checked by the store under the commit lock: the first
  open commits and moves the head, the second is refused with the first's
  story named, and its task feeds that story. Whether two captures report
  one event stays a judgement; the store only refuses an open from a run
  that has not seen the stories opened since. No topic-keyed claim: Jev's
  labels are advice (L2a), so a claim keyed by a label would make the
  store judge. Test: two tasks held at a barrier after reading the
  library, then both cast; one story, one open and one feed.
- **FM28, a task refused by its own open.** A run's mark is the head it
  read with `choir.Stories()`, so its own open moves the head past its
  mark and its next open is refused naming its own story, which invites
  feeding a different event into it. More tasks open more stories, so
  this grows with K. Guard: a checked open's receipt carries the head
  after the commit; the run had seen everything up to it (the check
  passed), so its mark advances. Test: one task opens two stories in two
  cells, with no refusal.
- **FM4, FM12 at K > 1.** Two running tasks dispose of one capture, or a
  re-delegation leases a capture a running task holds. Guard: the
  disposition index under the commit lock (S2a). Exercised by the
  concurrent tests (disjoint leases; the barrier test's captures).
- **FM14, 429s.** K tasks' model calls burst past the gateway's 60 a
  minute per computer. Guard: task model calls (and their compaction
  calls) join the research children's computer-wide spacer, 40 a minute
  for every sub-RLM together (O5). Test: the task run's provider is the
  spaced one, and concurrent task calls start at least one interval
  apart.
- **FM9 and AC6 at K > 1.** A crash with two tasks running and one queued
  resumes or starts any of them. Guard: unchanged (restart work kinds,
  the boot closure). Test: the S2a boot test at K = 2 with two running
  and one queued.
- **FM29, a failing start blocks or spins the queue.** The starter in S2a
  re-lists the queue after each attempt; a task whose start fails and
  whose closure also fails stays queued and is retried forever in one
  call, and a refused claim counts as a busy slot. Guard: one pass over
  one listing per call; a refused claim takes no slot. Test: covered by
  the drain test's slot count (no refused claim is counted).
- **FM30, K misconfigured.** K = 0 starves the queue; a large K
  overruns memory and the spacer. Guard: K is normalized (unset or
  below 1 is 4) and capped at 8.
- **FM31, no rollback without a deploy.** The paper is always busy, and
  deploys never restart a busy computer, so a K change by deploy may not
  reach it for hours. Guard: the owner setting `management.serial_tasks`
  (off by default; on means K = 1), read on every start, set through
  `/api/computer-settings`. Running tasks above 1 finish; none is
  cancelled. Test: with the setting on, a second task stays queued.
- **H3, lock waits.** K = 4 commits contend on the engine lock. Not a
  test: measured. The engine-mutex counters are added to
  `/api/runtime/observability` (product route), and the probe reports
  them.

### 18.2 Choices made without the owner (No Blocking Asks)

- **K's config**: `RUNTIME_MANAGEMENT_TASK_SLOTS` (default 4, at most 8)
  sets K; the owner setting `management.serial_tasks` forces K = 1 live.
  The setting is the conservative rollback path for an always-busy
  computer; the env var is for local runs and a later deploy.
- **The open guard is the library head**, as designed (§6.1), not a
  claim keyed by topic or event label (above).
- **No intake reserve** (R2-18 is S3's: only intake tasks exist in S2).
- **A queued task at a marker boot with no task running** starts at the
  next delegation's wake or the next closure, as in S2a. Boot delivers
  nothing (§7).

### 18.3 What S2b became in code

| failure mode | code |
|---|---|
| FM23, FM29 | `startQueuedManagementTasks` reads K once per call (`managementTaskSlots`), counts running rows (a claimed row with no run yet is busy; a row whose run ended is closed first), then makes one pass over one listing of the queue: a refused claim or a failed start takes no slot and is not retried in that call. The starter's mutex and the claim's compare-and-set are S2a's. |
| FM8 | Unchanged substrate: every non-crash closure mints a start wake for the oldest queued task in its own commit; one wake now fills every free slot. |
| FM3 | Unchanged guard (the library head, S2a). The task prompt says other tasks run beside it and may open the same event. |
| FM28 | `planStoryLibrary` returns the head after a checked open; `InboxCommitReceipt.StoryLibraryHead` carries it; the reduce sets the run's mark to it. An unchecked command's receipt carries 0. |
| FM14 | `executeWithToolLoop` wraps a task run's provider (the tool loop and its compactor) in `spacedProvider` on `subRLMCallSpacer` (renamed from `researchChildSpacer`; the same spacer research children use, 1.5 s, 40 a minute). |
| FM30 | `provideriface.Config.ManagementTaskSlots` from `RUNTIME_MANAGEMENT_TASK_SLOTS`, normalized by `NormalizeManagementTaskSlots` (below 1 → 4, above 8 → 8). |
| FM31 | `store.SettingManagementSerialTasks` (`management.serial_tasks`, default off) in `KnownComputerSettings`; on → K = 1 at the next start. |
| H3, queue | `/api/runtime/observability` gains `management_tasks` (slots in force, configured slots, serial flag, running, queued, oldest queued age) and `engine_mutex` (per call path since boot). Each task run's metadata carries `management_task_slots`. |

### 18.4 Tests (through the real reduce and commit paths, real desk workers)

- `internal/agentcore/management_tasks_s2b_test.go`:
  `TestManagementTasksRunKAtOnceAndTheQueueDrains` (K = 3 from config;
  six concurrent wakes start exactly three; their model calls are in flight
  together; the queue drains through swept outbox wakes alone; peak running
  ≤ K; task calls start at least half a spacer interval apart),
  `TestConcurrentTasksOpenOneStoryForOneEvent` (default K = 4; two tasks
  held at a barrier after reading the library, both cast; one open, one
  refusal, one feed into the opened story; head + 1),
  `TestATaskOpensTwoStoriesWithoutRefusingItself` (FM28; fails without the
  receipt head), `TestSerialTasksSettingIsTheLiveRollback` (FM31, and the
  observability block).
- `TestManagementTasksAtACrashBootAndAPlannedBoot` now runs at K = 2 with
  two tasks running and one queued (AC6 as written: the crash boot closes
  both runs and all three rounds, wakes no one, starts nothing; the planned
  boot closes both running tasks `run_passivated` with residual Reports and
  leaves the queued task a start wake).
- The S2a tests run at K = 1 (the fixture's default), which is also the
  rollback's behaviour.
- `internal/store/management_tasks_test.go`: the receipt head (FM28).
- `internal/provideriface/config_test.go`: K's default, bounds and env.

### 18.5 How the probe shows S2b working

`scripts/management_latency_probe.py --cred <paper cred> --start <deploy+idle>`
adds `tasks.concurrency`: `peak_running` and
`share_of_window_2plus_running` against `slots_at_start` (K per task run),
`library_refusals` and `tasks_with_a_refusal` (H5), `now` (K, running,
queued from observability), and `engine_lock_since_boot_top5` (H3). The
stderr line now ends with the queue wait p50/p95, peak running and K, and
the undisposed count. S2b works on the paper when, at 28 sources:

- `tasks.queue_wait_s` p50 and p95 fall well below the S2a window's 425 s
  and 1,506 s, and `tasks.oldest_queued_age_s` stays bounded;
- `ac1_latency_s.undisposed.n` and `older_than_900s` stop growing over the
  window (S2a: 149 undisposed, 81 over 15 minutes after 34 minutes);
- `peak_running` ≤ K and `share_of_window_2plus_running` > 0 (it ran
  concurrently);
- AC4 holds: `scripts/l2a_paper_window.py` pairs find no second story for
  one event, and `library_refusals` are few (H5);
- AC9 holds (`ac9_clean_share` ≥ 0.9, `ac9_deadline_share` ≤ 0.1) on the
  pinned model, and `engine_lock_since_boot_top5` shows no commit path with
  a max wait in seconds (H3).

### 18.6 Rollback

1. Live, no deploy: set `management.serial_tasks` on
   (`POST /api/computer-settings {"key":"management.serial_tasks","value":true}`).
   The next start holds K = 1, which is S2a's behaviour; running tasks
   finish within their deadline.
2. Then revert the S2b commits. The receipt field is additive
   (`omitempty`), the setting key becomes unknown again (a stored row is
   ignored), and the observability fields disappear. The S2a rollback note
   (§13: an S2 revert waits for no open task round) still holds for
   reverting S2a itself.
