# Engineer reuse: a rejection returns to the same engineer; an app's engineer resumes from its lineage (2026-11-04)

Status: design, revision 2: revision 1 reviewed (Devin and DeepSeek v4.1
flash, both "revise"); their blocking findings (the retained-candidate
composition; a passed candidate lost to a run end before its Complete
commits) and the non-blocking ones are folded in below and listed in
"Review round 1". Mutation class of this doc: green. The change
it designs is red: management's Verify (run acceptance), the assignment's
terminal saga, the self-development operation's identity, and the
engineering prompt. Code refs are at `d64f85c82`.

Owner, 2026-11-04: "Yes, rejection shouldn't cost a new cast. We should be
reusing our agents. Even over durations. Even successful agents maybe
should be resumed like to add a feature to an app built by Eng last week."
The goal: "minimal overhead over the raw speed of writing, building,
testing code".

## What exists

Measured on staging (docs/evidence/selfdev-trace-review-slides-next-r1-2026-11-02.md,
docs/problems/frozen-candidate-dies-with-build-deadline-2026-11-04.md):

1. **The round.** Management casts (`choir.Cast("engineering", ...)`,
   `planEngineeringCast`, engineering_assignment_runtime.go:238); the
   assignment opens with its own conversation
   (`management:<owner>` with `engineering:<assignment>`), and the runtime
   spawns, binds and activates one capsule and one engineer run
   (`spawnBindActivateAssignment`, :827). The engineer builds, runs
   choir-shot, and stages `choir.Freeze` (`commitFreezeIntent`,
   rlm_reduce.go:1530 -> `freezeCapsuleEffectBundle`, tools_capsule.go:181).
   The Freeze opens the assignment's one self-development operation
   (`openFreezeCandidate`, freeze_candidate.go:24, keyed by
   `store.EngineeringFreezeIdempotencyKey(assignmentID)`) and leaves the
   capsule live (engineering prompt: "A successful freeze also leaves the
   capsule live").
2. **Complete ends the engineer.** `choir.Complete(result="completed")`
   runs the terminal saga (`recordAssignedEngineeringReportOnce`,
   engineering_assignment_fate.go:883): freeze intent, physical capsule
   freeze, receipt binding, candidate export, **capsule revoke**, then the
   terminal report and the final Report to management in one transaction.
   The run ends (`fate_terminal`).
3. **The judge runs after the engineer is gone.** Management's
   `choir.Verify` refuses any assignment that is not `completed`
   (`commitVerifyIntent`, rlm_reduce.go:1604, "only a completed
   assignment's candidate is verified"). A `fail` moves the operation
   `frozen -> failed` with "independent judgment rejected the frozen
   bundle: <reason>" (`recordSelfDevelopmentVerdict`, tools_capsule.go:751).
4. **A rejection costs a new cast.** The carry (carried_patch.go) offers
   the failed candidate's patch to management's *next* Cast for the same
   request; that Cast opens a new assignment, a new capsule and a fresh
   engineer that re-reads the code. Pool's engineer read for 17 minutes
   before its first build; the round trip costs 20 to 40 minutes.
5. **Waits already park in the loop.** A cell that polls its running job
   parks the turn on the job's end with no model call between
   (`engineeringJobPark`, capsule_jobs.go:423, through
   `toolregistry.WithToolBatchWait`); the park holds nothing durable and a
   crash ends it with the process.
6. **The engineer's conversation persists only as its run's message log and
   the trace on the tape.** Nothing reads it after the run ends; the carry
   hands the next round a patch and a reason, not context.
7. **No continuation path exists.** Every assignment is `attempt` 1; a Cast
   onto an open assignment is refused and names the engineer's inbox
   (`refuseCastOntoOpenAssignment`, :447); management can only Note a live
   engineer.

The parallel fix on branch `freeze-decision-deadline`
(docs/problems/frozen-candidate-dies-with-build-deadline-2026-11-04.md)
retains a candidate frozen when its build's clock ends, and admits Verify
on that cancelled assignment's retained candidate. A retained candidate
has no live engineer, so a rejection of it takes today's path (item 4).
The two changes compose only under the merge rules in A6.

## The decision

**Phase A: the judge runs while the engineer is still alive.** The
Complete that follows a Freeze becomes a *review request*: the platform
sends management the engineer's report as a progress Report that asks for
Verify now, and parks the engineer's turn (capsule live, conversation in
memory) on the verdict. Pass: the platform commits the engineer's own
Complete, and the round ends exactly as today. Fail: the turn resumes with
management's reason; the engineer fixes the work in the same capsule and
Freezes again, which opens the assignment's next operation revision. The
bound is three candidates per assignment: the third Complete takes
today's path (Complete, then Verify), so a third rejection ends the round
with the judge's reason.

**Phase B: an app's engineer resumes from a durable handoff, not a
transcript.** At the end of an assignment that froze a candidate, the
platform records one lineage entry per app the candidate changed: the
engineer's own summary (with a Handoff paragraph the prompt asks for:
decisions, open issues), the files it changed, the base, the operation, and
the run id that names the full transcript on the tape. A later Cast whose
request names that app starts its new engineer with that brief in its
objective.

Phase A reuses the existing cast and assignment model: no new assignment,
no new attempt, no new cast. The continuation is the same bound run of the
same assignment, re-entered by the park. The only identity that changes is
the operation's revision.

## Phase A mechanism

### A1. The review request (engineering cell, reducer)

`commitCompleteIntent` (rlm_reduce.go:1809) diverts when all hold:

- the intent's result is `completed`;
- the assignment's current operation (A4) is `frozen` with a bundle;
- that operation's revision is below the bound (`reviewInLoopMaxRevisions`
  = 3), so revisions 1 and 2 review in the loop and revision 3 does not.

Diverted, it builds the report exactly as today (receipts resolved,
screens note attached) and commits it as a **progress Report**
(`result=partial`, the existing non-final path through
`commitAssignedEngineeringReport`), with:

- the summary prefixed "Review requested (candidate N of 3): ";
- outcome refs `selfdev-operation:<op>`, `bundle:<digest>` and
  `review:<op>`, so management's Verify can name the bundle as it does from
  a final Report (`assignmentOutcomeRefs`, which today returns early for a
  partial);
- a report id derived from the operation (`rlm-review:<run>:<op>`), so a
  replayed cell replays the same Report and Verify can find it (A2).

The full terminal report and its intent identity are kept in memory on
the runtime, keyed by the run, for the park (A3); the batch wait parks
when the run has a kept review. The cell is not terminal.

`refusePartialCompleteAfterFreeze` is unchanged: the engineer never
stages a partial after a Freeze; only the platform converts its
`completed`.

### A2. The verdict (management cell, reducer)

`commitVerifyIntent` admits, besides a completed assignment (and, with the
retained-candidate fix, a retained one), an **open assignment in review**:
not terminal, bound, its current operation is the one named and `frozen`,
and the review Report for that operation exists in the store (looked up by
its derived report id under the bound run). Every other check is
unchanged (desk, bundle digest, verifier refs, verdict event).

After the verdict is recorded:

- it wakes the parked engineer (an in-memory signal keyed by the
  assignment; the park re-reads durable state on wake, A3);
- a `fail` in review settles none of the assignment's `verify_pass`
  precommits: the claim is about the assignment's candidate, which a later
  revision may be; a pass, or the round's end (`SettleRuleAtTerminal`:
  "no completed candidate"), settles it;
- on `fail`, it mints one platform Note to the Texture the round serves,
  through the progress relay's record shape and gate
  (`engineering.progress_to_texture`): "Management rejected candidate
  <op> (N of 3): <reason>. The engineer is fixing it in the same capsule."
  This is the only Texture line; no Texture or management turn is needed
  for the fix.

Management's prompt (rlm_management_runtime.yaml) gains one sentence: a
progress Report that begins "Review requested" and names `review:<op>` asks
for Verify now, exactly as a final Report would; the engineer waits with
its capsule live; a fail returns the reason to the same engineer, and its
next candidate arrives as another Report; do not Cast for it. A final
Report for a candidate already verified needs no second Verify (a repeat
pass replays).

### A3. The verdict park (engineering tool loop)

The engineering batch wait runs the review park before the job park. When
the run has a kept review (A1), the park waits, with no model call, until one of:

| outcome | how it is read | what the park does |
|---|---|---|
| pass | operation `verified`, `awaiting_approval`, `accepted`, `materializing` or `applied` | commits the kept Complete through `recordAssignedEngineeringReport` (the terminal saga, under the saga mutex, detached from the activation with its own 3-minute deadline, as `commitTray` commits a cell's Complete), then ends the loop (new `Terminal` on the batch-wait result); if the commit fails, resumes the turn with the error, and that candidate's next Complete is not diverted again |
| fail, revisions left | operation `failed` with the judgment-rejected error | resumes the turn: "Management rejected candidate <op> (N of 3): <reason>. Your capsule and your work are as you left them. Fix what the reason names, re-run the build and the checks Freeze cites, choir.Freeze again, then choir.Complete." plus the activation budget line |
| other operation end | any other state: failed for another cause, `rejected` by the owner, `degraded`, `rollback_pending`, `rolled_back` | resumes the turn with the fact; the completion guard handles the rest as today |
| no verdict in `reviewParkMax` (20 min) | timer | commits the kept Complete (today's order: the round ends, the candidate stays frozen for management's Verify on the final Report); the candidate is not diverted again |
| run context ends | ctx | returns its error, as today |

The park reads only durable state (the operation) and registers its wake
before the first read, so a verdict that lands between the cell and the
park is not missed. A Verify signals the wake; an owner act or a close on
the operation does not, so the park also re-reads every 30 seconds
(`reviewParkRecheck`). The re-read is a bounded backstop on one row for at
most 20 minutes, not a delivery mechanism. It is bounded by `reviewParkMax`, the
activation budget and the assignment deadline. It is one completion-guard
wait (bounded by `maxCompletionGuardWaits`).

### A4. Operation revisions

The assignment's operation identity becomes a revision series: revision 1
keeps today's key and id (`EngineeringFreezeIdempotencyKey(assignment)`,
`EngineeringFreezeOperationID(computer, assignment)`); revision n > 1 is
the same with `:rev:<n>` appended. One resolver,
`assignmentOperation(ctx, computer, assignment)`, returns the highest
revision that exists; every reader that resolved the operation by the
freeze key (about fifteen: Freeze, Verify, the partial refusal, precommit
rules, carry, the cancellation choke point, outcome refs, the close at the
end, screens) goes through it.

`openFreezeCandidate` opens revision n+1 only when revision n is `failed`
by a judgment rejection, the assignment is open, and n+1 <= 3. "By a
judgment rejection" is the verdict's terminal error *and* the verification
event it stamps on the operation (`VerifierRefs`); no cancellation close
stamps one. "The assignment is open" holds by construction: only the bound
live run of an open assignment can Freeze. Any other terminal state of the
current revision keeps today's refusal. Parsers of the key (not only
resolvers) go through `store.EngineeringFreezeAssignmentOf` and
`store.IsEngineeringFreezeKeyOf` (`rebase_content.go`,
`openOperationHold`).

The review admission's report id is the one derivation
(`partialReportID`) the store-side partial report also uses, so the two
cannot drift.

**A passed candidate survives its run's end.** Between the pass and the
commit of the kept Complete the assignment is open and its candidate is
`awaiting_approval`, a state no open assignment reached before. When the
run ends in that window (activation budget, deadline, cancel, crash), the
cancellation choke point (`failBoundSelfdevOperation`) leaves an
`awaiting_approval` candidate for the owner's decision instead of failing
it, as a completed assignment's candidate is left today; the assignment
itself still ends. A trajectory's cancellation still closes it
(`failTrajectorySelfdevOperations`).

### A6. Merge rules with the retained-candidate fix

Whichever of the two lands second applies these:

1. Every reader on `retained_candidate.go` that resolves the assignment's
   operation by `EngineeringFreezeIdempotencyKey` (`frozenCandidateOf`,
   `decideRetention`, `retainedFor`, `recordRetention`) resolves through
   `assignmentOperation`, so a build-end cancel during rework retains the
   current revision; the retention record names that revision's operation.
2. `closeOrphanedFrozenCandidatesAtCrashBoot` parses the assignment with
   `store.EngineeringFreezeAssignmentOf`, never `CutPrefix("freeze-open:")`,
   so a revision n > 1 orphan is closed at the crash boot.
3. management's Verify admits the union: a completed assignment, a
   cancelled assignment's retained candidate, or an open assignment in
   review.
4. `closeEndedAssignmentSelfdevOperation` keeps the retained exclusion for
   a cancelled assignment's retained frozen candidate. The
computer's one-open-operation rule is unchanged: revision n is terminal
before n+1 opens, and admission holds one live assignment.

### A5. The bound

Three candidates per assignment. Revisions 1 and 2 are reviewed in the
loop; revision 3's Complete takes today's path, so its rejection ends the
round with the judge's reason and the carry offers the patch to
management's next Cast as today. Each candidate is its own operation on
the tape (frozen, verified or failed with its reason) and its own Report in
management's conversation; each rejection is a verification event on the
tape and a line in Texture.

## Phase B mechanism

### B1. Handoff, not transcript resume

| | literal transcript resume | context handoff (chosen) |
|---|---|---|
| context cost | the whole log: Pool's engineer was 171 messages; weeks later no prompt cache holds it, so every call pays it in full and it crowds the window before the new work | a bounded brief (<= 6 KB) |
| staleness | the log narrates a tree that has since moved: tool results show old file contents, old receipts, old job ids | the brief names its base and files; the engineer is told to trust the code and check `git log` on those files |
| doctrine | I20: persisted model-authored source is inert, and reusing it never restores an activation capability or handle; a replayed log is full of receipts and handles that look live | the brief is data in the objective |
| restart rules | a resumed log is the same work continuing | a new owner request, a new Cast, a new assignment and capsule |

The full transcript stays reachable: the brief names the run id, and the
run's trace is on the tape (`trace export`).

### B2. The lineage record

One durable object per (computer, app), kind `choir.engineering_lineage`,
in the computer's object graph (one home per kind, I27), written when an
implementation assignment's terminal report commits with a frozen
candidate (completed, or failed after it froze). Fields: app, assignment
id, run id (the transcript ref), operation id, bundle digest, base ref,
the changed files under the app, the engineer's summary (bounded), and the
time. A later assignment overwrites the app's record: the lineage is the
app's last engineer, and older ones stay reachable through their own
assignments on the tape. The operation's state is not copied: the brief
reads it live from the operation store (single authority).

Apps are derived from the candidate's changed paths: a package
(`frontend/src/packages/<app>/...`) or a first-party app component
(`frontend/src/lib/<App>App.svelte`, the app being the lowercased name);
a change touching neither writes no lineage. A rebase round writes none:
it adds no work of its own.

The engineering prompt asks every Complete summary to end with a
"Handoff:" paragraph: the decisions made and the open issues.

### B3. The resume

When a new implementation assignment's run context is built, after the
carry notice, the runtime looks up the lineage of the apps the request
names: the app names that have a lineage record and appear as words in
the run's objective and the opening Cast's objective and body ("calendar",
"Calendar app", "CalendarApp"). Management's prompt asks it to name the
app in the objective. The brief is computed once per run, so the run
context stays the same across its turns (the prompt cache).
Each record found (at most two) adds a "Prior work on <app>" section to the
engineer's notice, built like the carry notice: who built it and when,
the operation's current state, the files, the summary and handoff, the run
id, and "the code may have changed since; read these files first and trust
them over this brief". A record written by the same trajectory's carry is
still offered (the carry gives the patch; the lineage gives the context).

## Authority boundaries

- Management still admits engineering through its delegated Cast; Phase
  A opens nothing, and Phase B only adds data to an assignment management
  cast.
- Management is still the only judge: Verify stays management's, with the
  same digest, verifier-ref and event checks. The new admission is a
  narrower case of an open assignment of its own cast, proven by the
  durable review Report.
- Engineering stays capsule-bound: the rework runs in the same bound
  capsule under the same assignment capability; nothing crosses capsules.
- Texture stays the only writer of the document: the rejection line is a
  platform Note into its inbox, as the progress relay is.
- The platform commits the engineer's own Complete on a pass: the intent
  is the engineer's, staged in its cell; only the time of commit moves.

## Restart-rule compliance

- The review park holds nothing durable, like the job park: a crash ends
  it with the process. The boot closes the run and the assignment as
  "interrupted by a restart"; the open operation fails at the crash boot
  (with the retained-candidate fix, retained candidates fail there too).
  Nothing resumes the review.
- A planned restart does not occur while the capsule is live (vmctl's
  live-capsule refusal); if one did, the run is lost and nothing re-arms
  the park.
- **Phase B is not a resumer.** Resuming an app's engineer weeks later is a
  new owner-initiated request: Texture's Cast, management's Cast, a new
  assignment and capsule, which start from the durable lineage. Nothing
  reads the lineage at boot, and nothing starts work from it.

## Failure modes

1. Management never answers the review Report (it ends its turn, casts
   instead, or is busy): the park ends at `reviewParkMax` and commits the
   Complete; Verify then runs on the final Report as today. Lost: only
   the in-loop fix.
2. The verdict lands before the park registers: the park reads the
   operation first and resumes at once.
3. A Verify on an open assignment that is not in review (an engineer
   still building): refused; the review Report is the proof.
4. A Verify after the park timed out and the Complete committed: the
   assignment is completed, today's admission.
5. The engineer's run ends while parked (activation budget, assignment
   deadline, cancel): the cancellation choke point runs as today (with the
   retained-candidate fix, the frozen candidate is retained); the verdict
   then takes today's path.
6. The engineer ignores the rejection and Completes `completed` without a
   new Freeze: the current revision is `failed`, not frozen, so the Complete
   does not divert; the terminal saga runs, and management's Verify finds
   a failed candidate. The round ends with the judge's reason.
7. The rework Freeze while the current revision failed for another cause
   (cancel, apply failure): refused as today.
7a. The build clock ends while revision n > 1 is frozen or under review:
    the cancellation choke point resolves the current revision (A4); with
    the retained-candidate fix, A6 rule 1 retains it.
7b. management's Verify on the review is refused (digest, store error):
    the park waits; at `reviewParkMax` it commits the Complete and the old
    order takes over.
7c. Pass, then the run ends before the kept Complete commits: the
    candidate stays `awaiting_approval` for the owner (A4, last paragraph);
    the assignment ends; the restart rule is unaffected (an
    `awaiting_approval` candidate is a decided object, not work, and it
    survives a crash today for a completed assignment).
8. Two Freezes race a rejection: revision n+1 opens through the store's
   idempotency key; a replay returns it.
9. The kept Complete is lost (crash): there is no park to commit it; the
   restart rule closes the round.
10. Pass, then the terminal saga fails: the saga's own watchdog
    re-drives it, as for any Complete (`armAssignedEngineeringFateWatchdog`).
11. Precommits: a rejection in review settles nothing; the final verdict
    or the round's end settles `verify_pass` (A2).
12. The activation budget runs out during rework: the park's resume carries
    the budget line; at the end, today's cancellation (or retention) runs.
13. Phase B: the lineage names an app the request does not concern (a word
    match): the brief says it may be unrelated; the cost is a bounded
    paragraph. A lineage whose files were deleted since: the engineer
    reads the code, as told.
14. Phase B: the lineage write fails: logged; the round is unaffected
    (the lineage is advice, never a gate).

## Rollback

Git revert of the commits. No migration: revision 1 keeps today's key, so
an operation of either version resolves under both; a reverted runtime
reads only revision 1 and treats an assignment with revisions as today's
single-candidate assignment (its revision 1 failed). Lineage objects are
inert data a reverted runtime never reads. There is no runtime switch (the
2026-11-03 rule); rollout is the ring-ordered deploy.

## Residuals

- R1. **Gate findings after the judge** (an owner rejection, a
  materialization failure after approval) do not return to the live
  engineer. The park ends at Verify: waiting through approval would hold
  the live capsule and the computer's engineering slot for an owner
  decision of unknown latency, and a self-development apply that needs a
  planned restart is refused while a capsule is live, so the park and the
  apply would wait on each other. Those findings reach the next round by
  the carry (same request) and by the lineage (same app). The Freeze's own
  gate (a refused Freeze) already returns in the cell.
- R2. Precommits stay per assignment: `freeze_within`, `tests_pass` and
  `paths_confined` settle at the first Freeze and are not re-scored for a
  later revision.
- R3. The rework shares the build's activation budget; a fresh budget per
  revision would need the run to passivate and re-activate (the durable
  park I21 asks for), a later step.
- R4. I21 says an activation parks rather than staying alive to wait for
  another actor. The review park keeps the activation in its loop while
  management judges, as the job park does for a job: it is bounded
  (20 minutes), holds nothing durable, and keeps exactly what the owner
  asked to keep (the conversation and the capsule). The durable form is
  the run's passivation, with the verdict delivered to its inbox; named
  for the later step with R3.
- R5. Phase B's app match is a word match over the request and objective;
  management is asked to name the app. No typed Cast `"app"` field is
  added: the cast spec stays as it is.
- R6. The Texture rejection line rides the progress relay's gate
  (`engineering.progress_to_texture`, default on): a computer that stored
  it off gets no line, as for every relay.
- R8. An assignment cancelled (not its trajectory) after its candidate
  passed review leaves that candidate `awaiting_approval`, an open state
  that holds the computer's one-open-operation rule until the owner decides
  it, as a completed assignment's candidate does today (A4, 7c).
- R9. The divert-to-pass spine (a real Complete diverted, then a pass
  committing the kept Complete through the terminal saga) has no unit test;
  the staging proof is its check.
- R10. freeze_minutes is anchored at the Cast and settles at the first
  successful Freeze; a re-Freeze after a rejection carries no minutes
  prediction of its own. The rework's minutes show on the round receipt
  only.
- R7. A computer that runs both this and the retained-candidate fix needs
  the A6 merge rules; until the second lands, its own tests are the check.

## Review round 1

Devin (revise) and DeepSeek v4.1 flash (revise), on revision 1:

- Blocking, both: the retained-candidate fix's readers resolve revision 1
  only, and its crash-boot sweep misparses revision keys; Verify needs the
  union. Fixed: A6.
- Blocking, DeepSeek: a pass is lost when the run ends before the kept
  Complete commits, because the cancellation close fails an
  `awaiting_approval` candidate. Fixed: A4's last paragraph, failure mode
  7c, and the commit detached from the activation (A3).
- Non-blocking, both: the park had no wake for non-Verify transitions
  (30 s re-read, A3); a repeated pass in the commit window was refused
  (admission includes `awaiting_approval`); the report-id derivation was
  duplicated (shared); "awaiting_approval or later" admitted `degraded`
  (states enumerated, A3); the timeout path could re-divert (not diverted
  again); precommits settled `no` at the first rejection (A2); the
  rejection test was a string prefix (plus the verification event, A4);
  the Texture line's gate is named (R6); the Cast has no `"app"` field yet
  (phase B adds it).

## Code review, phase A

Devin and DeepSeek v4.1 flash, round 1 on the phase A commit: both
accept_with_nits, no blocking finding. Folded in: admission includes
`verified`; a stranded terminal saga is not diverted; the timeout re-reads
the operation before it commits; the "ended" resume names an owner
rejection; the commit-failure resume says how to end. Named: R8, R9.

## Staging proof

Phase A, on a test computer (never the paper or the owner's computers):
a request whose first candidate management rejects. The ledger shows, for
one assignment and one bound run id: Freeze (revision 1), a "Review
requested" progress Report, a Verify `fail` event, the Texture Note
"rejected: ...; the engineer is fixing it", the same run's next cells
within minutes, a Freeze opening revision 2 (`...:rev:2`), a second review
Report, and a Verify `pass` followed by the assignment's completed final
Report, with no second Cast and no second capsule
(`lifecycle capsule-evidence <traj> <assignment> --attempt 1`).

Phase B: two requests about one app, a week apart in intent but minutes
apart on staging; the second engineer's first model input contains "Prior
work on <app>" naming the first assignment's run id.

## Merge with the retained-candidate fix and the calibration cuts

Applied on main `8a9050b80` (A6): `retained_candidate.go`'s
`frozenCandidateOf` resolves the current revision; the crash-boot orphan
sweep, the round receipt and the owner-act settlement parse keys with
`store.EngineeringFreezeAssignmentOf`; the claims read resolves the
current revision; Verify admits completed, retained-cancelled or in
review. Calibration: a rejection in review settles nothing; a pass
settles `verify_pass` and `judge_accepts`; a round that ends on an
in-review rejection settles them (and `owner_approves`,
`approval_minutes`) at the assignment's end as that final rejection
(`settleEngineeringPrecommitsAtTerminal`). `approved_within_rounds`
counts operations with a bundle, so each revision's Freeze once. The
spin detector resets across a review wait, and the review-request cell is
never observed as a no-op.

Merge review (Devin accept_with_nits; DeepSeek revise, one blocking: A6
rule 4 was not applied, so a late report on a cancelled assignment closed
its retained candidate). Fixed, with the test; also fixed: the owner's
claims stay open for a candidate that passed review and awaits the owner
when its run ends. Named residuals:

- R11. A crash boot that closes a retained candidate does not settle its
  `verify_pass`/`judge_accepts` (inherited from the retained-candidate
  fix); they stay open.
- R12. The retention decision and the fail are two reads of the current
  revision; a rework Freeze landing between them is failed, not retained
  (milliseconds wide).
- R13. A current revision that ends decided-not-frozen (owner rejected, or
  failed for another cause) on an assignment that still completes leaves
  `verify_pass`/`judge_accepts` open; and a kept Complete that commits
  inside a Verify's write window can leave them open. Open, never wrong.
