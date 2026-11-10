# Unsettled obligations: a Wait that nothing owns (clustering assessment, 2026-11-09)

Status: assessment, no code. Mutation class of this document: green.
Required by CLAUDE.md "Root Cause Clustering" (3+ bugs in one subsystem within
a week). Evidence: code at origin/main 0ebab72a7; b@c's trajectories through
the `choir` CLI (read-only); one observability read of d@e and one of b@c
(about 01:30Z, 2026-11-10Z). Each member record carries its own
"Investigation 2026-11-09 evening" section.

## Members and non-members

| record | shares the cause? |
|---|---|
| [theme-rounds-stuck](theme-rounds-stuck-2026-11-09.md) B (quota cancel) | yes: Texture Waits for a reset nothing will announce |
| theme-rounds-stuck C (parked on the owner, or on an apply record) | yes: six live b@c trajectories |
| [final-report-strands-unread-cast](final-report-strands-unread-cast-2026-11-05.md) | the race is fixed (020276dde); its point 3 (a Wait the runtime does not own) is the same cause |
| [passivated-final-turn-never-settles](passivated-final-turn-never-settles-trajectory-2026-11-05.md) | the settle road is fixed (f58ba6707); the wait road is the same cause |
| theme-rounds-stuck A (late before screens) | no: a leaf rule disagreeing with another rule |
| undeclared-texture-reads | no: custody gate; fixed (0d97fe9d7), gate in enforce with zero counts |
| yaegi race | no: dependency; fixed by the fork pin |
| engineering artifacts | no: two capsule defects, both fixed on main |
| activation-wait-times | a sibling: obligations run late behind a fence, not lost |
| rematerialized stack / shot review | a sibling: a pair claim stands, the notice is the only wake |

## The shared substrate

One fact, read from the trace and the code together: **an open Texture work
item is the only durable obligation of a waiting trajectory, and nothing owns
closing it when the Texture turn that parks it names a Wait.**

Trace (b@c, 6 of 6 live task trajectories): the last event is a
`texture_turn_committed` whose reason ends "waiting: Waiting ..." (for a quota
reset, for the owner's decision, for a record confirming an apply). The work
item is `open`, there is no live assignment and no open round, the Texture run
is passivated until the 00:04Z boot cancels it, and nothing happens for 2 to 15
hours. Chrome Pastel, whose last turn said Done, settled in one minute.

Code, what the runtime does with a Wait:

1. `turnNamesNextActor` (`internal/textureowner/texture_next_actor.go:52`)
   treats any non-empty `Wait` as proof the turn "names a next actor", so the
   unattended guard `refuseUnattendedTurn` accepts it.
2. The backstop `closeUnattendedRequest` (same file, :134) runs only after the
   runtime's own Idle turn, never after a Wait turn.
3. The runtime holds no timer or wake for a Wait; `activation_budget_deadline`
   and the other durable deadline updates (`continuation_schedule.go`) cover
   activations and assignments, not a desk's Wait.
4. A passivation settles the trajectory only when the work items are already
   closed (`lifecycleSettlementReady`, f58ba6707); a Wait leaves one open.
5. The crash-boot closure (`closeOpenObligationsAtBoot`, `deskreconcile.go:105`)
   closes runs, rounds and install reviews, not work items. The b@c boot at
   00:04Z terminalized the six parked Texture runs and left the six work items
   open, which breaks the restart rule's "every open obligation (run, wake,
   control, directive, assignment, work item) is closed with the visible fate
   interrupted by a restart".

The predicate that says "nothing serves this request" already exists:
`unattendedTextureRequest` (:93): open owner request, no live engineering
assignment, no round the desk cast, no live self-development operation, and
the doc is not a Texture root. It is exactly the state of all six
trajectories. It is simply not asked after a Wait turn or at boot.

This is a substrate problem, not six symptoms: the state machine lets a
trajectory sit live with an open work item and an empty set of things that can
wake it. The earlier fixes (f58ba6707, 020276dde) closed two roads into the
state; this record's roads (quota cancel, owner park, apply record) are
others.

## Is there an existing replacement that would fix the class if wired in?

Yes, in three pieces, all already in the tree:

1. **`closeUnattendedRequest`**: refuses the work item with
   `UnattendedRequestReason`, records a typed `WaitKindNoNextActor` wait
   (`types.WaitState`), and commits an `UnattendedRequestNotice` to
   management's admitter inbox. That is a settle (the refused work item lets
   the trajectory settle, and the owner's next revise opens new work) and a
   wake (management reads the notice and can re-cast) in one idempotent
   commit. It is wired for the Idle turn only.
2. **`types.WaitState` with `ExpectedUntil` and `Class`**: the typed wait
   record the quota path already produces (`WaitKindProviderQuota`) and the
   frame already shows to management. A Texture Wait could carry one.
3. **Durable `not_before` actor updates** (the dispatcher's due timer D2, used
   by `activation_budget_deadline` and `assigned_engineering_fate_deadline`):
   the mechanism for "wake me at a time" that survives a restart without
   being a boot-time resumer. Only needed for a Wait with a known expected
   time (a quota reset); it is not needed for the minimum fix.

Not found: any code that wakes a parked Texture at a quota reset, and any
delivery of a self-development "applied" record to the Texture of the
trajectory whose candidate was applied (the Neo Vector Heart and Utopian
Scholastic shape; hypothesis, to be confirmed from the tape).

## The substrate action (the next action, per CLAUDE.md)

Wire piece 1 into the two places the state is entered, and nothing else:

- after any Texture turn that commits a Wait or a Done-less end, evaluate
  `unattendedTextureRequest` post-commit (so a same-turn Cast or assignment
  open is visible) and, if nothing serves the request, run
  `closeUnattendedRequest` with the Wait text as the reason;
- after the crash-boot closure of runs, for each open Texture work item
  whose agent has no live run, the same close with the cause "interrupted by
  a restart".

This deletes a class instead of patching six trajectories. It adds no
runtime switch and no timer. It is a closing action, not a resumer; on a
planned restart with a marker the runs are resumed, not cancelled, so the
boot leg must run only on the crash path (marker absent), after the marker is
consumed, which `closeOpenObligationsAtBoot` already is. A refused work item
ends the trajectory's liveness visibly ("nothing is working on this; revise
to continue") and the owner or management re-opens by an act. For a Wait
that has a real server the predicate returns false and nothing changes.

The residual after this: a Wait that *is* served (a live assignment) but whose
server then ends without waking Texture. The assignment-end path already
wakes the caster (the cancel report reached Texture twice on Liquid Scholar),
so this is the "apply record" leg, and it needs its own trace read.

## Ranked fix list

| # | fix | records | class and tier | size (Sonnet builder) | restart rule |
|---|---|---|---|---|---|
| 1 | Close a parked Texture request that nothing serves: post-Wait check and crash-boot leg (above). Reuse `unattendedTextureRequest` and `closeUnattendedRequest` | theme B, C; final-report point 3; passivated (wait road) | red, lifecycle refusal; Wait leg tier 1, boot leg tier 2 (boot path, schema untouched) | 3 to 4 files (`texture_next_actor.go`, the Texture turn commit caller, `deskreconcile.go`/`runtime.go`, tests); 5 to 6 hours | a close, never a resume; boot leg only on the markerless path |
| 2 | Run the late-before check only where a before and after pair is required (`needsBeforeAfterPair`) | theme A | red (loosens a Freeze refusal); tier 1 | 2 or 3 files and a table test; 2 hours | none |
| 3 | `cause=` and queued kinds on the dispatcher's activation-wait line | activation-wait-times | orange | 1 file and a test; 1.5 to 2 hours | none |
| 4 | Confirm and, if confirmed, deliver the self-development "applied" outcome to the Texture of the trajectory (the Neo Vector Heart and Utopian Scholastic road); first a read of the tape | theme C (record-that-never-comes) | red if built; the read is green | read: 1 hour; build unknown until confirmed | a wake from a commit, no resumer |
| 5 | A Wait carries a typed `WaitState` (kind quota/owner/record, `ExpectedUntil`) and, when time-known, arms one durable `not_before` re-check that runs the same close or re-wakes management | theme B at scale | red, new durable wake; tier 2 | 6 to 8 hours | the re-check is a durable update armed in a commit; a crash closes by fix 1's boot leg, never resumes |
| 6 | For a clean-rebase operation, a failed shot *job* (not a judged fail) releases the pair for one re-attempt | rematerialized / shot review | red, tier 2 | 3 hours, after the verdict-reuse build lands | none new |
| 7 | Housekeeping: stale status lines, delete the yaegi `replace` when #1752 and #1739 release | yaegi | green | minutes | none |

Worth building tonight: 1, 2, 3. Fix 1 is the substrate action and clears the
six live b@c trajectories on their next boot or Wait turn without touching the
old data. Fix 2 is a leaf, independent, and it is what failed Tide Pool twice
and Ink Sakura once. Fix 3 is observation, and d@e shows the number it
explains growing by the hour. Fix 5 is the physics of a Wait with an
expected time and is not needed until 1 has run: with 1, a quota Wait ends
as a refused work item plus a management notice, which is slower than a timed
re-dispatch but loses nothing.

## What could not be verified

- Whether fix 1's post-commit check races with a same-turn Cast: the design
  must evaluate after the Cast's commit; I read the predicate, not a trace of
  a turn that Casts and Waits together.
- The Neo Vector Heart and Utopian Scholastic road (an apply record that
  never reaches Texture): the CLI does not expose wake rows.
- Whether the six live trajectories would settle cleanly once their work items
  are refused (the settlement rule needs the `artifact` subject ref, which
  they have, so it should; not exercised).
- Staging proof of anything that landed today (L7 acceptance, quota
  mitigation, deliverables): code and logs only.

## Build: failure list (written before the code, 2026-11-10)

Mutation classes: A red (lifecycle refusal on Texture work, boot path), B red
(loosens a Freeze refusal), C orange (log text only). Under the data-system
operating tempo the tiers are suspended; the runtime checks are the verifier.
Each row names the test that pins it. A is built as one helper,
`closeUnservedTextureRequest`, that wraps `unattendedTextureRequest` and the
shared body of `closeUnattendedRequest` (the Idle backstop keeps its
behaviour), called from two places: after a Texture turn that committed a Wait
(`CommitCellTextureAuthor`), and from `closeOpenObligationsAtBoot` after the
rounds are closed (a markerless boot only; the marker branch never reaches it).

### A. Close work nobody serves

What can legitimately serve a waiting Texture request, read from the code:

1. an open round the Texture desk cast (`OpenRoundsCastBy`, read from the
   conversation heads, which commit atomically with the Cast; so a Cast and a
   Wait in one turn are visible to the post-commit check, no child run needed);
2. a live (non-terminal) engineering assignment on the trajectory (this covers
   a running capsule job: a capsule belongs to an assignment);
3. a live self-development operation on the trajectory;
4. new here: an unread waking record in the Texture desk's own inbox (a Report
   that closed the round between the Cast and the check; the run-end records
   re-check wakes it), by `store.RecordWakesReader`;
5. new here: a pending, undelivered lifecycle update packet addressed to the
   Texture agent (a control not yet bound to a run);
6. new here: another live activation of the same Texture agent (a wake already
   dispatched), and an owner revision that moved the head after the turn (it
   opened new work and wakes the desk);
6b. an Ask or Escalate this desk sent that is still being worked. They open
   no round (a round opens only for a Cast), and foldTextureDispatch turns an
   Ask-only cell into a Wait, so this is the common case (Devin's review).
   Durable fact: an Ask is a precommit with a Resolver, outstanding until a
   Report in the desk's inbox relates to it or the desk retracts it; an
   Escalate has no reply, so it is outstanding until its recipient has read it
   (`Store.OutstandingDispatches`). At a crash boot an unanswered Ask is NOT
   counted: the asked desk's run was closed and its wake interrupted, so
   nothing serves it.
7. not served, by design: a Wait on the owner. The product has no owner-ask
   path for Texture other than the document itself; the owner's next revise is
   the wake. Closing it with the visible fate and a management notice is the
   cluster's stated end state for the owner-decision shape.
8. no scheduled wake exists for a Wait (nothing arms one; fix 5 would). A
   durable `not_before` update for the Texture agent is not consulted because
   the actor tape is not visible to agentcore; the build adds none.

| failure | test |
|---|---|
| closing served work: each of 1 to 6 | `TestWaitKeepsTheRequestOpenWhenSomethingServesIt` (a subtest per kind) |
| Cast and Wait in one turn: closed before the cast round is visible | `TestWaitWithACastInTheSameTurnStaysOpen` |
| Write and Wait in one turn closes the request | `TestWaitWithNothingServingClosesTheRequestAndTellsManagement` |
| double close: a replayed turn, or the Idle backstop after a Wait close | same test (replay), notice count 1 |
| close racing a late wake: the wake arrives after the close | the refusal is idempotent and the owner's next revise reopens work; `TestWaitCloseIsReopenedByTheOwnersNextRevise` |
| closing at a planned-restart boot (marker present) | `TestPlannedBootDoesNotCloseTextureWork` (Start with `bootRestartPlanned`) |
| crash boot with thousands of open items | the boot leg is capped at 200 closes and 60 s per boot, oldest first, and logs the remainder; `TestCrashBootClosureIsBounded`. The remainder stays open and is closed by the next crash boot or its own next Wait; a cap hit is a log line, not a silent drop |
| the management notice wakes a run that waits again (loop) | the notice is a system Note that `RecordWakesReader` refuses; asserted in the Wait test, and the boot test asserts no run or wake is created |
| work parked on the owner by a legitimate question | the product's only owner question is the document revision; the closed item reopens on the owner's revise (above) |
| another desk's work items matched | the boot leg takes only items assigned to a `texture:` agent with the document-custodian profile; `TestCrashBootClosureTakesOnlyTextureWork` |
| a Texture work item whose agent has a live run at the boot | skipped; `TestCrashBootClosureTakesOnlyTextureWork` |
| story, publication and the Texture root | exempt (`openTextureRequest`, `textureRootFor`); `TestNextActorReadsTheTrajectory` exists |
| a boot closer error | logged, the boot goes on; never blocks the boot |
| b@c's six live stranded trajectories | see below |

b@c's six stranded trajectories: each has an open Texture work item, no live
run (the 00:04Z boot cancelled them) and no round. At a crash boot the boot leg
refuses each work item ("no one is working on this request: interrupted by a
restart"), puts one system Note per item in management's inbox, and the
trajectory settles; that is the desired visible fate and the restart rule's
text. At a planned, marker-backed refresh the leg does not run (the marker
branch passivates and resumes runs, and these runs are already terminal), so
the six stay as they are: no regression, no repair. Nothing ever re-dispatches
them. A new Wait turn on any of them would close it. b@c must never be crashed
or deleted to see this; the proof is on d@e.

### B. The late-before check

| failure | test |
|---|---|
| a new-package diff that needs no pair is refused for a late before shot | table test, new package only: not refused |
| a diff that modifies or deletes an existing file, or adds to an existing package root, is no longer refused for late shots | table test: still refused |
| a diff that mixes a new package and a changed existing surface | table test: a pair is required, so the late check runs |
| the predicate drifts from `requireScreens` | both call one function, `needsBeforeAfterPair` |

### C. The dispatcher's wait line

| failure | test |
|---|---|
| the cause line changes behaviour | the cause is computed from values the skip already holds; the tests assert a busy, held or roomless agent is still not activated: `TestWaitLineNamesTheSerialFenceAndLeavesTheDecisionAlone`, `TestWaitLineNamesARetryHoldAndAFullSlotTable` |
| log volume | no new line: the fields ride the existing wait line, which prints once per activation whose oldest due update waited 30 s or more (not per skipped tick); the kinds are capped at four plus a count; `TestWaitLineIsQuietForAShortWaitAndCapsKinds` |
