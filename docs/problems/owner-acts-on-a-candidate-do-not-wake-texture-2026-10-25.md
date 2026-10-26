# Owner acts on a self-development candidate do not wake the Texture of the document that supervises it (2026-10-25)

Status: documented, fix follows in a later commit. Register row SD-08
(R5-5) in docs/evidence/residual-register-2026-10-26.md. Mutation class of
the fix: `red` (it adds records to a Texture inbox and so wakes Texture),
additive and behind a per-computer switch that is off by default. Texture
stays the only writer of the document.

## Evidence

Ledger, docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.ledger.md,
entry 2026-10-25 07:1x: selfdev-de06563b was approved and applied on b@c
(applied 07:02:59). reader-4's head text still said "the Reader is not
verified or accepted": no Texture turn followed the approval or the apply.
The ledger names it the P7 class (owner acts on a candidate do not wake its
Texture). d28fb01d8 (P7, trace review round 4) fixed only the evidence-embed
case: an embed revision was misread as owner input, so a later records wake
took the wrong shape. That fix does not touch the decision path.

What the evidence does not contain: the b@c Texture inbox and wake rows
for doc 5db2578d between 07:00 and 07:12. The trace below is from code at
bf531f4e2, and "nothing woke Texture" is a reading of the code that the
ledger's observation agrees with, not a read of the staging ledger.

## The trace of an owner act, to where it stops

1. The owner approves or rejects with `choir self-dev approve|reject`,
   which posts `/api/computers/<id>/self-development/operations/<op>/decision`
   (`cmd/choir/commands.go:603-627`, proxied by `internal/proxy/self_development.go`).
2. `decideSelfDevelopment` (`internal/agentcore/api_self_development.go:310`)
   appends the effect_accepted or effect_rejected event and moves the
   operation to accepted or rejected. On accept it calls
   `triggerSelfDevelopmentReconcile`. It writes no commitment record and
   no inbox record.
3. The reconciler applies an accepted operation:
   `materializeSelfDevelopmentOperation` ->
   `recordMaterializationApplied` (`self_development_materializer.go:473`,
   `:569`) appends materialization_applied and moves the operation to
   applied. It also writes no inbox record.
4. The only record the self-development path mints is the decision-boundary
   observation (`notifySelfDevelopmentDecisionBoundary`,
   `self_development_materializer.go:1000`), addressed to management, at
   awaiting_approval: before the owner act, not after it.
5. Texture is woken by a pending owner revision, a producer report, open
   work, or an unread record in `texture:<doc>`
   (`internal/textureowner/texture_controller.go:620-680`). None of the
   steps above produces any of these. The document's Texture therefore
   keeps the last thing it was told, which is management's verify report
   ("awaiting the owner"), until some unrelated event wakes it.

So the act is durable on the tape and in the operation row, and the desk
that renders the owner's supervision surface never hears of it.

## Existing mechanism to connect (not a new channel)

The engineering progress relay (`d0beb6cf1`,
docs/problems/engineering-reports-never-reach-texture-2026-10-24.md) is the
shape: a platform-authored system Note committed by `CommitInboxRecords`
into `texture:<doc>`, from a non-desk sender inbox, which mints an
`inbox_record` actor wake through the same transaction (`store/inbox.go:546`)
because `RecordWakesReader` (`store/management_tasks.go:66`) admits it. A
records-route Texture turn needs no open work item
(`texture_agent_revision.go:545-551`). The fix mints one such Note per owner
act, so the owner act is the cause of the Texture turn and Texture decides
what the document says.

The document is found from the operation: `operation.TrajectoryID` is the
lifecycle trajectory; its `SubjectRefs["doc_id"]` names the document, as the
evidence embed already resolves it (`texture_evidence_embed.go:84-90`).

## Failure modes of the fix (written before it)

1. An act still does not wake Texture (no record, or a record that
   `RecordWakesReader` refuses).
2. One act mints two records or two wakes: a replay of the same decision
   key, the decision observer racing the handler, or a reconcile pass that
   records the same apply again.
3. Approve, apply and reject collapse into one record, so Texture cannot
   tell "accepted, applying" from "applied".
4. An operation with no bound document (a rollback or genesis trajectory,
   a trajectory without `doc_id`, a document bound to another trajectory)
   errors, or wakes some other Texture.
5. The record lacks the act or the operation id, so Texture cannot say
   what happened to which candidate.
6. The decision path writes document text itself (a Texture-sole-writer
   violation) instead of waking Texture.
7. A crash boot wakes Texture: the reconciler's crash settlement
   (`settleSelfDevelopmentOperationAfterCrash`, journal only) or a
   boot-time decision recovery mints an act record, so a restart resumes
   work. A wake minted before a crash and still pending is closed by the
   generic crash-boot fate (`MarkActorWakeInterrupted`), never
   re-dispatched.
8. A failure to mint the wake fails or reverses the decision. The decision
   is the owner's and is already on the tape; the wake is observational.
9. Texture reads the record as a round "closure" (the default kind of a
   system record in its frame), so it takes no action.
10. The switch is off and something changes.

## Belief state

- The cause is the missing write in steps 2 and 3 (confidence high, from
  code; the staging ledger was not read).
- The fix does not cover failed or degraded applies, or rollbacks. Those
  are owner-visible outcomes too; they are named as residuals, not fixed
  here.
