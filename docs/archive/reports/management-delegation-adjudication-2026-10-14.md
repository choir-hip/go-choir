# Management Delegation: Panel Adjudication

Date: 2026-10-14. Subject:
[management-delegation-design-2026-10-14](management-delegation-design-2026-10-14.md)
(round 1, commit `0ae6ddbea`). Problem:
[management-serial-turn-blocks-paper-2026-10-14](../problems/management-serial-turn-blocks-paper-2026-10-14.md).
Owner direction, 03:55 UTC: "we can't do serial. Management should delegate
to subrlms."

Panel: agentic-consensus, convergent mode, six members, all `ok`. The prompt
asked about task shape, authority and conflicts, restart semantics,
acceptance, slice order, O1-O9, and S2-on-staging blockers. Raw outputs are
session diagnostics, not durable evidence.

| member | verdict | duration |
|---|---|---|
| claude (opus) | accept with amendments | 140 s |
| deepseek-v4.1-flash | accept with amendments | 351 s |
| gemini-3.8-flash | accept with amendments | 292 s |
| glm-5.3-flash | accept with amendments | 672 s |
| muse-spark-1.3 | accept with amendments | 112 s |
| gpt-6-luna | **revise** ("keep the shape; do not implement S2 from this note yet") | 407 s |

All six keep the shape: a management task is its own run and session with a
records conversation, sends outward as `management:<owner>`, and holds a
lease while its round is open. All six reject the alternative of parallel
management turns over inbox partitions, because a single cursor and a single
active run (`inbox_records.go:656-660`) would need the same lease machinery
anyway. All six treat deterministic intake as a complement, not a
replacement. Luna's "revise" is about contracts the note leaves unwritten,
not about the shape. The other five list the same gaps as amendments. This
adjudication takes the stricter reading: S2 does not start until the
amendments below are in the note. Round 2 of the note makes them.

Mutation class of this document: **green**. The work it amends is **red**.

Method: a finding is adopted only if the code (file:line on `origin/main`
`380ad9a19`) or the cited evidence supports it. Each finding gives who raised
it, the check, and the decision. Counts are members who raised the finding
independently.

## 1. Blocking findings

### B1. A task has no identity the records machinery recognizes (5: claude, gemini, glm, luna, deepseek)

The claim is that a run reading `management-task:<id>` fails every
profile-keyed switch, so on staging it would get no route, no wake, no
ended-run closure and no re-check.

Verified:
- `recordsDesk` accepts only texture, research, management and engineering
  (`internal/agentcore/inbox_records.go:32-38`).
- `recordsRoute` switches on the sender's desk prefix and has no case for a
  task desk (`:48-103`). The `default` returns `ok=false`, so a task's
  Decline errors with "only management declines a source's captures"
  (`:466-467`).
- `commitRecordsRoute` skips the records route when the scope agent id's
  desk is not the run profile (`:427`).
- `WakeRecordsInbox` looks up a prompt by profile (`:635-638`). For
  management, it refuses any agent id other than the persistent one:
  `if deskAgentID != persistentManagementAgentID(ownerID) { return nil, nil }`
  (`:642-645`). **So a task inbox spelled `management:<anything>` is never
  woken, and one spelled `management-task:<id>` has no prompt.**
- `closeRoundsOfEndedRun` returns at `default:` for an unknown profile
  (`:810-824`). A task run that ends without a final Report would hold its
  lease until the next crash boot.
- The record's sender is the scope's desk agent id
  (`inboxRecordForIntent`, `:309`).
- `agentprofile.Canonical` rejects unknown tokens
  (`internal/agentprofile/agentprofile.go:91-103`). `resolveRunIdentity`
  canonicalizes both the profile and the role (`internal/agentcore/runtime.go:583`,
  `:599`), so a new role is a vocabulary change, not only an inbox spelling.

Correction to round 1 of the note: §3.1 cites `inbox_records.go:66-68` and
`:95` as the reason a task inbox must not parse as `management:<subject>`.
Those lines check the destination subject on sends *to* management, from a
Texture and from engineering. The real constraints are `:642-645` (the wake)
and `:427` (the reduce).

**Adopted.** Round 2 adds an identity table (§3.1a). It names the run's
profile and role, the reader inbox, the outward sender and the sender of the
task's own final Report, and lists every switch S2 must change, with tests
through the real reduce path.

### B2. The task's own final Report must come from the task inbox (1: luna; implied by the B1 identity work)

Verified: a final Report closes a round only if its sender is the round's
recipient: `if sender != head.RecipientInbox` refuses it
(`internal/store/inbox.go:232-234`). The task round is
(`management:<owner>` → `management-task:<id>`), so its recipient is the
task inbox. Round 1 said a task's outward records carry
`SenderInbox = management:<owner>` (§4.2), which as written covered every
record a task sends. A final Report to `management:<owner>` sent from
`management:<owner>` would not close the task round. It would land in the
degenerate conversation `management:<owner>|management:<owner>`.

**Adopted.** Two senders. Acts addressed to Texture or a source go out as
`management:<owner>` with host-derived `acting_task` and run id. The closing
Report to the parent goes out as `management-task:<id>`. The host mints that
Report (B9), not the model.

### B3. An unknown desk role fails open to engineering (1: deepseek)

Verified: `normalizeDeskRole` maps any role it does not know to
`"engineering"` (`internal/yaegikernel/choir.go:172-178`). `NewChoirScope`
gives every role except research and research-child Exec, WriteFile and
Assign (`:119-139`); today that includes management. So if `Canonical` learns
a task role and `normalizeDeskRole` does not, the task silently gets the
engineering module set plus filesystem actions. A happy-path staging run
would not show it.

**Adopted.** One commit registers the task role in `Canonical`,
`normalizeDeskRole`, `deskModuleSets` (`choir.go:288-298`) and
`NewChoirScope`'s action list. The task's action list excludes Exec,
WriteFile, Assign and ProductAPI. A reverse-parity test checks the module
set and the handle's actions (as `choir_parity_test.go:422-449` does).

### B4. The post-turn re-check bypasses the clean-Report wake rule (3: claude, glm, muse)

Verified: `recheckRecordsInbox` starts another turn when any unread record
was committed after the ending turn began (`inbox_records.go:735-742`). It
never consults `inboxRecordWakes` (`store/inbox.go:209-219`). Clean task
Reports that land during a parent turn would start the next parent turn,
which is the note's own failure mode 2. Gemini's F3 restates round 1's §5
rule and is counted as confirmation, not as a new finding.

**Adopted.** One predicate decides whether a record wakes its reader, and
both `inboxRecordWakes` and the re-check's late scan call it. The test lands
three clean Reports mid-turn and expects no new turn, then one residual
Report and expects exactly one new turn.

### B5. The lease check races a closure because it runs before the commit (4: claude, luna, deepseek, gemini)

Verified: `commitRecordsRoute` reads `ConversationState` (`inbox_records.go:476`),
builds the records, and only later calls `CommitInboxRecords` (`:534`). A
reducer-side lease check has the same gap: a deadline or cancel closure can
commit between the check and the commit. A feed Note into `texture:<doc>`
belongs to a different conversation from the task round, so
`stepConversation` would not catch it either. The fix is feasible, because
the store commits under `trajectoryMu` (`store/inbox.go:578`) through
`PutBatchConditional` with object conditions (`:587`), as the cursor
compare-and-set already does.

**Adopted.** Each task commit carries a precondition: "task round N of
`management:<owner>|management-task:<id>` is open". The store checks it under
the lock. A second disposition of a capture is refused in the same way, by a
per-capture disposition object written in the disposition's own batch and
conditioned on being absent (deepseek F4). That object indexes the record;
it is not a second authority, because the record stays the content and the
two commit atomically. The parent's re-disposition override writes the
object explicitly. Round 1's sentence "the reducer, not the store, checks
the lease" is withdrawn.

### B6. The story-library check has no plumbing and does not cover the parent (3: muse, claude, deepseek)

Verified: `planStoryOpenings` opens a story for each `texture:new` Cast,
conditioned only on its new ids being absent (`internal/store/story_open.go:56-95`).
There is no library head. The reduce cannot tell a stale task from a
current one unless the commit carries the head the task saw.

**Adopted.** The host records the library head per run when it serves
`choir.Stories()`, the way `recordsRunShown` works, and stamps it on every
`texture:new` intent. The store compares the head and advances it in the
opening commit. The check applies to the parent's `texture:new` too, until
S5. A refusal refuses the whole cell, which is the existing
`commitRecordsRoute` semantics. It returns the stories opened since. A task
that never read `Stories()` has seen head 0, so its first open is refused
with the list.

### B7. K slots have no mechanism; queued tasks have no durable start (3: muse, deepseek, claude)

Verified: `WakeRecordsInbox` starts a run whenever the desk has unread
records and no active run (`inbox_records.go:656-665`). Nothing counts
slots across task inboxes, so a burst would start one run and one worker per
task. Boot does not deliver work directly; the actor-wake outbox is the only
delivery path after a restart (`runtime.go:793-797`).

**Adopted.** The slot starter is durable and derived. A queued task is an
open task round with no run. Its start is an outbox wake of a kind listed in
`RestartWorkKinds` (`planned_restart.go:161-173`), so a crash boot
interrupts it, and a marker boot delivers it once. Engineering's
`delegated_assignment_spawn_deadline` is the model to copy. The starter runs
when a task run ends and on the wake. Queue depth and the oldest queued age
are reported per computer (in S0's script, from the routes).

### B8. The engineering assignment binding cannot hold a management task (2: gemini, luna)

Verified: `EngineeringAssignmentBinding.Validate` requires `trajectory_id`,
`parent_decision_id` (runtime-derived), `parent_control_id`,
`assigned_agent_id` and more (`internal/types/engineering_assignment.go:119-139`).
An intake task serves several sources' polls and has no trajectory or
capsule.

**Adopted.** Reuse the patterns, not the type. A `management_task` row gets
its own `Validate`: owner, computer, kind, parent run, parent conversation
and round (answer tasks), task conversation, deadline, and leased refs. It
has no trajectory, capsule or verifier fields. It sits in the same store
family as the assignment row and uses the same wake kinds and closures.
Round 1's "generalize that row to a kind" is narrowed to this.

### B9. "Clean" must be computed by the host, not claimed by the model (luna, glm; strengthens O8)

Verified basis: engineering's final Report is already minted by the host,
not staged by the model (`inbox_records.go:325-331`: "only its Complete …
mints the final Report"). The boot closure record names no captures
(`internal/types/commitment.go:442-456`), so a residual list cannot come
from closures (claude, minor).

**Adopted.** A task ends with a staged `choir.Done()` or with its deadline.
The host then mints the final Report from the task inbox, with
`residuals = leased refs - durable dispositions`, which it computes from the
B5 disposition objects. A Report is clean only when that set is empty and
the task had no error. AC5's join uses the same computation.

### B10. A 10-minute task deadline does not fit the management model's latency (4: deepseek, glm, luna, muse)

Evidence: the problem doc records free-preview calls taking "from seconds to
nine minutes per call" (`docs/archive/problems/management-serial-turn-blocks-paper-2026-10-14.md:53`).
Run 305a582d on deepseek-v4.1-flash averaged 29 s per call (1,680 s over 58
calls, design §2.4). At 4-6 calls a task, a 9-minute call alone would close
most tasks at their deadline. The re-delegations that followed would thrash
(glm).

**Adopted in part.** S2 pins the task role to a non-preview model with
bounded latency and a per-call timeout below the task deadline. The model is
an S2 deliverable, not a later O5 tuning. S0 records each turn's model, call
count and call durations, so a miss on AC1 or AC2 can be attributed. Glm's
remedy, raising the deadline to 20-30 minutes, is **rejected**: longer
leases raise AC1's maximum and hide a slow model. Fix the model, keep 10
minutes, and measure the deadline-closure rate (AC9).

### B11. AC1, AC2 and AC4 can be met by declining; nothing asserts that serialization is gone (5: claude, deepseek, muse, luna, glm)

Verified against the note: AC1 counts any first disposition, so a fast
decline counts. AC4 only checks duplicates, so declining a new event makes
it pass. AC1 excludes crash-closed captures, and no bound limits that share.
AC5 says "exactly one disposition", but §6.4 allows a parent re-disposition.
AC4's 10-minute window lets later duplicates through. AC7's numerator
leaves out the parent's tokens and does not pin the model. AC2 could count
task runs as management runs (glm). Nothing measures that clean Reports do
not wake the parent (deepseek).

**Adopted.** Round 2's §12 replaces the criteria:
- **Declines cannot satisfy latency.** AC1 holds for opens and feeds alone.
  AC8 keeps the mix of opens, feeds and declines per arrival within a band
  of the S0 baseline, and judge-scores a sample of declines.
- **Crash-closed captures are bounded.** At most 2% of the window's
  captures, and every crash in the window is named.
- **AC10 asserts serialization is gone.** Parent run starts are at most the
  count of waking records other than clean task Reports, and below the task
  count. After S4, no captures record appears in the parent's inbox.
- **AC4** counts duplicates across the whole window, and `distinct_from`
  overrides separately.
- **AC5** counts one effective disposition per capture, and overrides
  separately.
- **AC7** counts parent plus task tokens per disposed capture, on a pinned
  model mix.
- **AC9** is the task completion rate.
- **AC2** filters to parent runs.

### B12. S2 cannot be judged by AC1 and AC2 in full (5: claude, gemini, muse, luna, deepseek)

Verified against the note: in S2 a capture still reaches `management:<owner>`
(`internal/agentcore/capture_arrival.go:59-65`), and the parent's turn
must run before Delegate. In S2, Texture Asks are still answered serially.
A 305a582d-style Ask turn (28 minutes) would break AC2's 10-minute maximum
however well intake works.

**Adopted.** S2's exit is:
- AC1, measured and reported, but not gating;
- AC2 on intake-only parent turns (turns whose new records are only capture
  arrivals and task Reports);
- AC4, AC5, AC9 and AC10.

The full AC1 gates S4. The full AC2 gates S3.

### B13. A running task's fate at a planned boot is undefined (3: deepseek, muse, luna)

Verified: a marker boot runs `passivateInterruptedActivations` and does not
close rounds (`runtime.go:765-770`, `:2481-2511`). Boot sweeps that re-fire
work are deleted (`:793-797`). A task the drain did not finish would be left
passivated with its round open, its lease held, and no wake. AC5 would
break.

**Adopted.** On a marker boot, a passivated task run gets the ended-run
closure (`run_passivated`), with its residuals per B9, and wakes the parent.
The marker permits that one wake. Tasks are not resumed in v1. Queued task
rounds start once through B7's outbox wake. A crash boot closes everything
(`closePreBootRunsAfterCrash` and `closeOpenRoundsAfterCrash`,
`runtime.go:768-769`). The drain never forces a restart on a busy computer;
the deploy waits for idle, per CLAUDE.md "Restarts End Work". Luna and muse
asked for this.

### B14. The parent's ended-run closure closes a Texture round an answer task still holds (1: claude; blocks S3, not S2)

Verified: when a management run ends, `closeRoundsOfEndedRun` closes every
round where `management:<owner>` is the recipient, unless
`childAssignmentsHoldRound` holds it (`inbox_records.go:833-842`). That
check lists engineering assignments only (`:869-899`). Once the parent
delegates an Ask and ends its turn, the Texture round closes `run_completed`
and the Texture wakes with no answer. The task's final answer is then
refused, because the round is no longer open (`store/inbox.go:229-231`).

**Adopted for S3.** Bind the answer-task row to the Texture round it serves.
`childAssignmentsHoldRound` and D6's `requireChildOutcomesRead`
(`inbox_records.go:565-590`) then cover task rows. The fix is listed as
S3's first test.

### B15. Parent-directed delegation needs a batch Delegate (1: muse)

Evidence: at 29 s per call (305a582d), one Delegate per cell for 27
arrivals is about 13 minutes of delegation alone, which breaks AC2.

**Adopted.** `choir.Delegate` takes many refs and a grouping, either one
task per arrival or explicit groups up to the coalescing cap. One cell
commits all the task Casts and the cursor advance. The prompt delegates the
backlog in at most two cells.

### B16. AC3 contradicts the task deadline (2: muse, luna)

Verified in the note: §3.1 sets a 10-minute task deadline, yet AC3 plants a
task "held for 20 minutes". AC3 also needs answer tasks, which arrive only
in S3 (luna).

**Adopted.** S2's AC3 plants a stalled intake task. The stall runs to the
deadline, and the drill checks that the deadline closes the round, frees the
lease and returns residuals, while AC1 (open and feed), AC2 (intake-only)
and AC10 hold. S3's AC3 plants K stalled answer tasks, which also checks
N3. Failure mode 13 (answer tasks and D6) moves to S3's tests.

## 2. Non-blocking findings

| # | finding (raised by) | check | decision |
|---|---|---|---|
| N1 | Round 1's lease is too coarse: §4.2 refuses any act addressing a conversation not in the task's Cast, yet §4.1 lets intake tasks feed any story. Feeds into a Texture whose Ask is leased would be refused (claude M1). Luna adds: one finalizer per conversation. | Internal contradiction between §4.1 and §4.2. Notes after a round closes are legal round-0 notes (`store/inbox.go:245-253`). | **Adopted.** A lease on a Texture record is the right to Reply or send the final Report in that round. Feed Notes that name the task's own leased captures are allowed in any conversation. |
| N2 | Re-delegating crash-interrupted captures can crash-loop, once per tick (claude M2). | Consistent with the schedule rule in CLAUDE.md: a tick that crashes the computer twice is closed. | **Adopted.** A per-capture crash count. A capture interrupted twice is closed with that cause and is not re-delegable. |
| N3 | Answer tasks on a slow model can fill all K slots and starve intake (claude M3). | Follows from one shared pool (§7). | **Adopted for S3.** Reserve 2 slots for intake. |
| N4 | A 120 s default cell deadline can kill legitimate parent cells. The kill takes the worker and the session's values (deepseek F7, muse, glm). | Verified: a cancelled eval kills the worker (`internal/yaegikernel/desk_session_worker.go:367-369`). The deadline applies only through `timeout_ms` (`tools_desk.go:442-446`). `armCellTerminalDeadline` mints a fate wake only when the outer ctx has a deadline (`continuation_schedule.go:91-107`). | **Adopted.** S1's default comes from S0's cell-duration data: max(300 s, 2 × p99), tightened later. Every kill is logged with the worker's exit cause, which settles H1. Deepseek's "task cells only" is **rejected**: the 860 s stuck cell was a parent cell. |
| N5 | Label overrides are recorded only for the management profile (gemini F4, glm). | Verified: `inbox_records.go:551-556`. | **Adopted.** Task runs keep profile `management` (§3.1a), so the check applies. A test asserts an override recorded from a task. |
| N6 | CancelTask has no mechanism (glm F-B4, muse). | `CancelRun` exists (`runtime.go:1604`). With the task profile in `closeRoundsOfEndedRun` (B1), a cancel closes the round `run_cancelled`. B5 refuses late commits. | **Adopted** as one paragraph of semantics. Glm's "remove CancelTask from v1" is **rejected**. |
| N7 | S2 is too large to bisect (muse). | Judgement. | **Adopted.** S2a: substrate, identity, batch Delegate, lease precondition, wake predicate, closures, at K = 1, with no concurrent opens. S2b: K = 4, slot starter, library head, spacer. |
| N8 | Order: the standing route matters more than answer tasks; pull S4 forward (claude: into S2; deepseek: before S3). | The problem doc's live symptom is the paper's arrival path. S4 depends only on S2. | **Adopted in part.** The order is S2, S4, S3. Claude's merge of S4 into S2 is **not** adopted: luna, muse, gemini and glm keep parent-directed intake as the first canary. |
| N9 | Name deterministic intake as the destination (glm F-N1; claude and deepseek call it a complement). | It changes L2a's settled rule (O3). | **Adopted as a named destination, conditional on the owner's O3.** S4 is designed label-first, so it can plug in. |
| N10 | O2 is not open: the delegated sender is forced by the code (deepseek, glm). | Verified. `texture:new` is refused unless the sender is `management:<owner>` (`store/story_open.go:68-71`). A Cast into an open round must come from that round's caster (`store/inbox.go:238-240`). | **Adopted.** O2 is settled by the code. |
| N11 | Coalescing: the lease must cover every capture named by the round's Casts, not only the opening Cast. Cap at 12, not 20 (muse). | A further Cast into an open round by the same caster is legal (`store/inbox.go:236-241`). Records are immutable. | **Adopted.** Coalescing is a further Cast into the queued round, and the cap starts at 12 in S4. |
| N12 | Story opens by tasks still wake the parent through each Texture's final Report (deepseek). | Verified. The opening Cast's caster is `management:<owner>`, and a desk record always wakes its recipient (`store/inbox.go:215-217`). | **Adopted as residual R-W.** Measured by AC10's wake breakdown. If those wakes dominate, extend the predicate to a Texture Report that answers a task-authored Cast. |
| N13 | Fixtures need intake.labels on, or Feed and Decline are refused (deepseek). | Verified (`inbox_records.go:432-439`). | **Adopted** as a fixture prerequisite. |
| N14 | Rollback: turning S4 off, or reverting S2, could strand open task rounds (deepseek). | A revert deploy is a marker boot, and a marker boot closes no rounds (`runtime.go:765-770`). | **Adopted.** Turning the setting off routes nothing new and lets open task rounds finish or reach their deadline. Before an S2 revert: setting off, then wait until no task round is open. |
| N15 | AC6 should assert that task runs end with the interrupted fate, not only their rounds (luna). | `closePreBootRunsAfterCrash` covers all pre-boot runs. | **Adopted** as one more assertion. |
| N16 | §6.2 overclaims that "the Texture reads both in one turn" (luna). | The order of commits says nothing about turn boundaries. | **Adopted.** Reworded to "in one or more turns". |

## 3. Rejected findings

- **Drain bounded to 60-120 s because of deploy or systemd stop timeouts
  (gemini O6 and risk 4).** No stop timeout applies to the drain. Deploys
  never restart a busy computer; they wait for idle (CLAUDE.md, "Restarts
  End Work"). The drain happens before the host writes the marker. Gemini's
  "interrupt the remaining tasks on the marker and resume them once" also
  conflicts with "no resume in v1", which five members chose.
- **Task deadline of 20-30 minutes (glm F-B2 remedy).** See B10. A slow
  model is fixed by changing the model, not by holding leases longer.
- **Same-event duplicate prevention as an atomic store rule (luna 3, strong
  form).** The store "validates well-formedness only and scores nothing"
  (`store/inbox.go:57-58`). Whether two captures report the same event is a
  judgement for a model or Jev. The library-head check (B6), window-wide
  AC4, and recorded `distinct_from` overrides cover the risk.
- **Remove CancelTask from v1 (glm F-B4).** See N6.
- **Resume once instead of drain (deepseek O6).** A minority of one. Its
  substance, that the planned-boot fate was undefined, is adopted as B13.
  The decision between the two remains O6.
- **Remove ReadCapture from the parent after S5 (deepseek, luna).** Four of
  six would keep it. The S1 cell deadline and AC2 bound the parent's turn.
  Coherence checks on residuals need to read captures. Revisit only if AC2
  fails after S3.
- **Merge S4 into S2 (claude M5).** See N8.
- **Scope the cell deadline to task cells only (deepseek F7).** See N4.

## 4. Open decisions O1-O9

| | panel | adjudicated answer | whose call |
|---|---|---|---|
| O1 Routes | 6/6 both | Both. The standing route carries captures (S4) and is the 24/7 path. Delegate carries Texture requests, residuals and re-delegation. Parent-directed intake is the S2 canary only, and after S4 it remains only to recover residuals. | settled |
| O2 Sender | 6/6 delegated sender | Outward acts go as `management:<owner>`, with host-derived `acting_task` and run id. The task's closing Report goes from `management-task:<id>` (B2). The code forces this (N10). | settled |
| O3 Jev auto-feed | 5 no in v1, glm "destination" | Not in this mission. It is named as the proposed destination after S4 (N9). It changes L2a's settled "labels are advice; management decides". | **owner** |
| O4 Verify | 6/6 no | Verify stays with the parent. | settled |
| O5 Model and pacing | 6/6 a dedicated fast role | A `management_task` model role, pinned in S2 (B10) to a non-preview, bounded-latency model with a per-call timeout. The proposed default is deepseek-v4.1-flash, which run 305a582d used. Tasks join the shared sub-RLM spacer (40 a minute, `research_children.go:31-38`), leaving about 20 a minute for root desks. A reserved share for tasks waits until S0 shows 429s. | default proceeds; **owner may override the model** (red: model routing) |
| O6 Planned restart | 5 drain with no resume, 1 resume once | Drain, then restart, with no task resume in v1. A task passivated anyway gets a closure with residuals that wakes the parent (B13). Queued rounds start once after a marker boot (B7). A busy computer is never forced. | Mechanism settled. **Owner:** who requests the drain and how a deploy waits for an always-busy paper. That is a deploy-shape tradeoff, tied to [planned-refresh-closes-paper-schedules](../problems/planned-refresh-closes-paper-schedules-2026-10-14.md). S2 does not depend on it. |
| O7 Numbers | provisional | K = 4 (S2a K = 1). From S3, 2 slots reserved for intake. Task deadline 10 min. Cell deadline max(300 s, 2 × p99 from S0). Coalescing cap 12 (S4). All measured, not fixed. | settled as starting values |
| O8 Clean Reports | 6/6 yes | Yes. The host computes "clean" (B9), and both wake paths use one predicate (B4). | settled |
| O9 ReadCapture | 4 keep, 2 remove | The parent keeps ReadCapture, read-only, and loses Feed, Decline and `texture:new` at S5. | settled (revisit on AC2) |

## 5. Can S0 and S1 start without the owner?

Yes. Neither depends on O3, O5 or O6.

- **S0 is yellow.** It is a script over product routes that adds the exact
  AC1 join, AC2 split into parent and intake-only turns, AC8's disposition
  mix, AC10's wake breakdown, per-turn model and call durations, the cell
  duration distribution, and queue metrics. It also takes the baseline.
- **S1 is red but small.** It is a default deadline on management cells. S1
  waits only for S0's cell-duration distribution, to set the default (N4),
  and it logs each kill with the worker's exit cause. It closes the
  14-minute stuck cell independently of delegation. It does not change
  routes, wakes, authority or restart behavior. Rollback is a revert.

S2a needs the round-2 note: B1-B13 and B15-B16 written into it. B14 blocks
S3, not S2. S2a needs no owner decision: O5's default model proceeds unless
the owner overrides it.

## 6. Heresy delta of this adjudication

- **Discovered.** `normalizeDeskRole` fails open to the engineering module
  set (`choir.go:172-178`). The post-turn re-check ignores the wake
  predicate (`inbox_records.go:735-742`). Round 1 cited the wrong lines
  for the task-inbox spelling constraint.
- **Introduced.** None. This document is design only.
- **Repaired.** None in code. The note's lease check moves from the reducer
  to a store precondition, which removes a race the design would have
  introduced.
