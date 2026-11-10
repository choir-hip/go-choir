# A second Cast into an open round is stranded by the round's final Report (2026-11-05)

Status: Cast-over-final race FIXED by 020276dde (2026-11-04, see Investigation 2026-11-09 evening); the Wait-with-no-wake residue is open. Mutation class of this doc: green. Evidence: the trace bundle of
trajectory 5c725f71-360f-5d20-a779-df8e879c6a07 (doc
ccf8c833-a796-58cd-ac85-cb413453a7e3) on d@e
(computer-59c902c5d46caecc0a180a8557164e72), read-only, and code at
origin/main cc34288a5.

## Evidence

Conversation `conv:research:ccf8c833...|texture:ccf8c833...` (caster texture,
recipient research). All times 2026-11-05Z.

| time | inbox / seq | record | round |
|---|---|---|---|
| 02:19:10.741 | research/1 | Cast #1 (directive), Texture run 2a11085d | 1 (opens it) |
| 02:19:44 .. 02:21:06 | texture/1-4 | four interim Reports, research run f642e636 | 1 |
| 02:21:28.220 | research/2 | **Cast #2** (directive), Texture run 165c2405, "focused second round" | **1** (not 2) |
| 02:21:39.934 | texture/5 | **final Report**, run f642e636 (cell, `final: true`) | 1, **closes the round** |
| 02:21:42.277 | | run f642e636 completes; 71 ms later research run b7ce6b88 starts (woken by research/2) | |
| 02:21:46 | | b7ce6b88 `choir.Records()` returns exactly one record: seq=2, kind=cast, round=1 | |
| 02:22:03 | | b7ce6b88 completes with no Report (intents 0 on every cell): "I haven't completed the requested research yet, so I can't send the final report" | |
| 02:22:36.336 | | Texture turn 5f870e6f committed, reason "waiting: Waiting on the second research round ..."; run 165c2405 passivates | |

State afterwards: both inboxes `open_rounds: []`; texture cursor 5 = head 5;
research cursor 2 = head 2; no record after texture/5; no wake row. Texture
Cast #2 left the Texture turn reason at 02:21:28 as "Waiting on a focused
research round", and the same run was still reading when the final Report
landed 11 s after Cast #2.

## Hypothesis check

The stated hypothesis is confirmed in effect, with one correction to the
mechanism. Round accounting does not mis-assign the final Report. There is no
"round 2": a Cast into an open round **joins that round**.

`stepConversation` (internal/store/inbox.go:283):

- `case rec.IsCast(): if head.Open { ... round = head.Round }` : Cast #2
  was committed while round 1 was open and cast by the same sender, so it was
  stamped round 1 (confirmed: both directives carry round 1). Only a Cast on
  a closed head opens round `Round+1`.
- `case rec.Final:` requires `head.Open` and `sender == head.RecipientInbox`,
  then `closed = true`. It looks at nothing but the head. It does not ask
  whether the recipient has read every Cast of the round.

So the final Report of run f642e636 (which composed itself from Cast #1 and
had not read Cast #2; research's cursor did not pass seq 2 until b7ce6b88)
closed round 1 including the unread Cast #2.

Why nothing repaired it:

1. `closeRoundsOfEndedRun` (internal/agentcore/inbox_records.go:1218) for
   b7ce6b88 calls `OpenRoundsAddressedTo(research)` (internal/store/inbox.go:1039).
   The head is closed, so it returns nothing and no closure is minted and no
   caster is woken. `roundsOpenedAfterRunBegan` (:1315) is irrelevant (it only
   filters open rounds). Run f642e636's own end (02:21:42) also found nothing:
   the round was already closed.
2. `choir.Records()` shows research Cast #2 as an ordinary record with
   round=1 and no marker that round 1 is closed, so b7ce6b88 had no way to
   know that a Report would be a round-0 note (a non-final Report on a closed
   head gets `round = 0`) and a final Report would be refused ("final report
   ...: conversation ... has no open round"). The model never tried.
3. Texture's `Wait` is free text that names a next actor. The unattended guard
   (`unattendedTextureRequest`, internal/textureowner/texture_next_actor.go:~90)
   only refuses a turn that names nobody; it accepts a Wait whenever it names
   something. `OpenRoundsCastBy` returned empty at 02:22:36, but the Wait was
   accepted anyway because the turn named a next actor. The runtime holds no
   timer or wake for a Wait. The post-turn re-check
   (`inbox_records.go` ~1130) wakes only for unread records, and texture/5 was
   already read.

Hypothesis part that stays unconfirmed: whether Texture's model saw the
`final: true` flag on texture/5 before it wrote "Waiting on the second research
round" (its cells at 02:21:48-02:22:11 read blocks; the cell outputs were not
inspected). The store-side facts do not depend on it.

## Substrate or symptom

Substrate. The defect is in the round state machine (a one-slot head: open or
closed, no notion of Casts the recipient has not yet read). Any desk pair has
the same race: caster adds a follow-up Cast while the recipient is composing
its final Report. It is not about research or Texture.

## Did tonight's slices cause it?

No. `stepConversation` Cast-joins-open-round and final-closes-head logic comes
from dee099747 (2026-10-13, records substrate) and a06191f9e (2026-10-13);
`closeRoundsOfEndedRun`, `OpenRoundsAddressedTo`, `roundsOpenedAfterRunBegan`
are older still. MA1.2 (de5960fa1, 2bae9cbdd), MA1.3 (7826565c8, 658a0c324)
and MA1.5 (90a26080e, d3d6f9ee2) did not touch `stepConversation`, final
handling or round closure (inbox.go changes: MA1.3 line 41 `rec.IsCast()`
validation only, MA1.5 adds the head read helper used by OpenRoundsCastBy
callers). What is new tonight is only the exposure: the MA1.3 wake prompt for
research (`Your inbox has new records from Texture: read them with
choir.Records()`) and Texture issuing a quick second Cast while research is
mid-final. It is a race that needs a Cast landing within the recipient's final
turn (here 11 s); the d@e acceptance trajectories are the first to run research
at this pace with interim Reports. Pre-existing, latent.

## Smallest fixes (deletion/substrate first)

1. **Refuse a final Report while a Cast of the round is unread by the
   recipient** (substrate, one check). In `stepConversation`/the plan step that
   validates `rec.Final`, compare the recipient's reader cursor (or the
   head's last Cast seq, a field the head would carry) with the seq of the
   latest Cast of this round. If the recipient has not read it, refuse with
   "round N has a Cast you have not read (seq S); read it with choir.Records()
   and answer it first". The final Report then cannot close over unread work,
   the research model re-reads, answers Cast #2 and closes the round properly.
   No new wake, no timer. This removes the race.
2. **Net for strands that still happen** (symptom net, also small): at run end
   in `closeRoundsOfEndedRun`, if the run's request record is a Cast whose
   round is closed and no Report of the run answered it, mint a system final
   Report "round N was closed before this run read its Cast" addressed to the
   caster and wake the caster (same code path as the `run_<state>` closure).
   Only worth landing with (1) if (1) cannot be made atomic with the cursor.
3. Not recommended: letting a Cast on a round that was closed by the final
   Report open round N+1 retroactively; the Cast was committed before the
   close, so reordering history is the wrong direction. Also not
   recommended: a Texture-side timer on Wait (adds a switch-like mechanism; a
   Wait already names its wake, here the wake simply could not come).
4. Authoring side (prompt text, optional): `choir.Records()` could print
   "(round N closed)" beside a record whose head is closed so the desk knows a
   final Report is refused.

## Test that pins it

A store-level deterministic test (the repo prefers E2E, but this is a pure
state-machine race): commit Cast #1 (texture to research), commit Cast #2 in
the same conversation before the recipient reads (cursor at seq 1), then
commit research's final Report; assert refusal naming the unread Cast. Then
advance research's cursor past seq 2 and commit the final Report; assert the
round closes. The E2E counterpart: staging acceptance trajectory in which
Texture casts a follow-up mid-run and the trajectory completes (trace ends with
a Report answering round 1 after the second Cast, or a new round 2), never
"live" with both inboxes at `open_rounds: []` and no pending wake. Rollback:
revert the check.

## Residual (not decided here)

The stuck trajectory 5c725f71 needs an owner act (revise the doc, which opens a
new Texture activation); nothing in the system will wake it on its own.


## Investigation 2026-11-09 evening

Read-only: code at origin/main 0ebab72a7.

### (e) Already fixed: the race

020276dde (2026-11-04 22:13 -0500, which is 03:13Z on 11-05, about an hour
after the trace this record reads): "a final Report cannot close a round over
an unread Cast; a Cast in a closed round wakes its caster". This is smallest
fix 1 and part of fix 2 of this record, built as proposed.
`refuseFinalOverUnreadCast` (`internal/store/inbox.go:665`, called at :527)
refuses a recipient's final Report while the unread range of its inbox holds a
Cast of the same conversation and round; system closures, engineering and
management-task inboxes are exempt. Tests: `internal/store/inbox_unread_cast_test.go`,
`internal/agentcore/inbox_unread_cast_test.go`. The race cannot recur as
described; I did not reproduce it.

### (a) Current truth of the rest

The stuck trajectory 5c725f71 was an owner-act residual, not touched here. The
substrate fact the record named in its point 3 is unchanged and is now the
live defect: Texture's `Wait` is free text (`turnNamesNextActor`,
`internal/textureowner/texture_next_actor.go:52`), the runtime holds no timer
or wake for it, and a Wait with nothing behind it is accepted. Tonight's
b@c theme rounds are six trajectories ending that way
([theme-rounds-stuck-2026-11-09.md](theme-rounds-stuck-2026-11-09.md), B and
C). So the Cast-over-final race is the same defect class reached by another
road: the cluster doc
[unsettled-obligations-cluster-2026-11-09.md](unsettled-obligations-cluster-2026-11-09.md)
carries the open part, with fix 1 (a Wait with nothing serving the request
closes the work item and wakes management). Substrate for the open part;
the closed part was a store state-machine bug.
