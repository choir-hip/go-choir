# Management ends with a frozen candidate unverified, and nothing re-wakes it

Date: 2026-11-04. Class: green (this doc). Owner ruling, 2026-11-04: eradicate
the waits, and never lose work to a wait.

## Evidence

Staging (live8), luna test arm, computer-2bf3cab1e3ebf3eb24a515c4603cf954.
Read from `choir trace export` of management trajectory
197c45b8-ac94-495f-94ba-55d5cfcfc309 and document trajectory
0ee62cc4-9159-544b-80f9-fa37a8f0029e.

- 06:03:29Z operation selfdev-a8b3f7d14067dcd4bce52c416b6eb748 froze under
  assignment-6f2647f4fb24b81538df7e3f (Calendar round 3). The engineer's
  Complete committed at 06:04:12Z; its final Report woke management.
- 06:04:12Z management run 197c45b8 (deepseek-v4.1-flash, persistent desk
  management:<owner>) started. Seven cells: `choir.Records()`, a Pack read
  (one compile error), `choir.Pack()`, `choir.Context()` and
  `GET /api/texture/documents`, a second `Records()`, one
  `POST /api/model-policy/verify` image check on screenshot 01, and one
  `choir.ReportPacket` with work_disposition completed (`rlm:report:5`). No
  `choir.Verify`. No refusal, no error.
- 06:04:37Z `loop.completed`, 8 iterations. The run's own result says "no
  engineering assignments of mine still open — nothing left pending".
- The Pack it read held `predict:verify_pass` "Management's Verify passes
  this round's candidate", unresolved. Nothing said the Verify was owed.
- 06:04:45Z Texture (run 8ed68998) wrote "Calendar (new) round 3 is frozen
  and holds up." The 06:04:27Z revision before it said "judgment and the
  owner's call open"; management's final Report ("confirmed by the
  independent image check") turned that into "holds up". The Texture frame
  carries the records only; nothing in it says the candidate is unverified.
- Then nothing: no management run, no gateway call, no wait record, until the
  owner revised the document at 06:25:36Z and Texture cast the Verify request
  (management run 29f0be12, 06:25:52Z).
- Contrast: crouton's management called `choir.Verify(..., "fail", ...)` 25 s
  after its engineer completed. The behaviour is model-dependent; the
  platform did not enforce it.

## Cause

Verify is the one act that moves a frozen candidate (to verified, awaiting
approval, or failed), and only management can do it. Three things are each
missing:

1. No turn-end check. The engineering desk has a completion guard
   (`engineeringCompletionGuard`: no ending without a terminal fate);
   management has none. A management turn can end with its cast
   assignment's candidate frozen and unjudged.
2. No backstop. A management activation that ends with a Verify owed leaves
   nothing armed. Management wakes only on unread records; the final Report
   was read, so no later event wakes it. The candidate stays frozen and holds
   every engineering admission on the computer.
3. Texture's language is not tied to the operation's state. A completed
   management Report reads as a verdict, and the frame does not say the
   candidate is frozen and unverified.

## When a Verify is owed

Verify is owed on an assignment management cast when its current operation is
frozen with a bundle and `commitVerifyIntent` would admit it:

- the assignment completed (the luna case);
- the assignment was cancelled by its build's end and the candidate was
  retained for Verify (`retained_candidate.go`);
- with engineer reuse (branch engineer-reuse), the assignment is open and its
  engineer asked for review (a review Report names the operation).

## Failure modes the fix must handle

1. The model ends its turn with a Verify owed: remind it, naming the
   assignment, operation and bundle.
2. The model ends anyway after the reminders: the run ends (no error, no
   endless loop), and the backstop takes over.
3. The activation ends by Done, failure, cancel or passivation, not by an
   end turn: the backstop still arms.
4. The engineer completes while management is idle and the record's wake is
   lost or deferred: a continuation armed at the Report re-checks.
5. Management is running when the continuation fires: do nothing; that run's
   end arms again.
6. Management verified in the meantime, the owner abandoned the candidate, or
   a later revision replaced it: the continuation does nothing.
7. Management never verifies: the re-wakes are bounded per operation, and the
   wait stays visible in the pack and on the document.
8. A crash: a continuation recorded before a crash boot is interrupted, never
   run, and nothing at boot re-arms it (CLAUDE.md restart rules). A later
   activation may arm again; that is new work on a booted computer.
9. A Verify that is refused (bundle mismatch, wrong state): it is not owed
   any more than Verify admits, so the guard never asks for an impossible act.
10. Texture writing a verdict before one exists: the frame says the candidate
    is frozen and unverified while it is.

## Fix chosen

(1) Guard. Management's persistent desk gets a completion guard: a turn that
ends with a Verify owed gets the harness line "Verify owed on <assignment>:
call choir.Verify(<assignment>, pass|fail, reasons) before ending.", with the
operation and bundle. At most two reminders per activation, then the turn is
accepted.

(2) Backstop. When a management activation ends with a Verify owed, and when
an engineer's Report commits on an assignment with a frozen candidate, the
runtime schedules a `verify_owed` continuation about 75 to 90 s out. If the
Verify is still owed and management is idle, it commits a system Note
"Verify owed on <assignment>" to management's inbox (record
`verify-owed:<operation>:<n>`, at most two per operation), which wakes
management, and tells the Texture the round serves through the wait relay.
Management's pack shows each owed Verify as a typed wait (kind
`verify_owed`, class policy).

(3) Texture. While the document's live operation is frozen, the frame says
so: frozen and not yet verified, so do not write that it holds up, passed or
is verified.

(4) Restart. `verify_owed` is a restart work kind: a crash boot interrupts a
pre-boot occurrence. The notice is minted only by the live continuation.
Nothing at boot arms one.

## Residuals

- Engineer reuse's review state needs `assignmentInReview` (branch
  engineer-reuse); until it lands, the in-review case is a stub that answers
  no, and merging the branch makes it one call.
- Merging engineer reuse: `verifyOwedInReview` becomes
  `rt.assignmentInReview`, and `freezeKeyAssignment` can become
  `store.EngineeringFreezeAssignmentOf`; that branch already moves
  `frozenCandidateOf` to the current revision, which this guard reads.
- A lost notice wake (review r1, Devin; r1b, DeepSeek B1): each notice
  arms one re-check of its own. Only that re-check, finding the notice
  unread with management idle, wakes management for it again, so a notice
  gets at most one re-wake however many turns end without reading it; other
  re-checks spend no notice while the last one is unread. A re-check that
  finds management working arms one more per run.
- A management task desk never casts engineering (records bindings are
  management:<owner> only), so only the persistent desk is guarded.
