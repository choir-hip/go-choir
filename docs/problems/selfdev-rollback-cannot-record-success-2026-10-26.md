# A self-development rollback applies, then cannot record its success (2026-10-26)

Status: documented; fix on lane/rollback-verify (tests first). Latent: staging
has never run a rollback (b@c, 2026-10-26: 9 applied, 17 failed, 17 rejected,
none rolled back). Class of the fix: red (checkpoint and route promotion for a
rollback, verifier certificate), review tier 1 or 2 by FP0.

Found by building the rollback tests of lane/base-join (the `rolled_back`
terminal note could not be reached end to end), and confirmed in production
code by review 316 (Devin), section "R7 -- confirmed true in production".
That review's trace, restated:

## The chain

1. `startSelfDevelopmentRollback` -> `selfdev.Store.StartRollback` copies the
   TARGET operation's `VerifierRefs` onto the new `rollback-<uuid>` operation
   (`internal/selfdev/operations.go`, `StartRollback`) and sets `BaseHead` to
   the target's `EffectiveHead`.
2. The verification payload recorded for the target names the target's
   operation: `"operation_id": operation.OperationID` (`tools_capsule.go`,
   the verification event payload).
3. `rollbackSelfDevelopmentOperation` applies the pinned prior release through
   the updater (files swapped, guest restarted under a planned marker), then
   calls `recordMaterializationApplied`, which calls
   `verificationEvidence(ctx, operation)`. That requires
   `payload.OperationID == operation.OperationID`
   (`self_development_materializer.go`, "materializer: verification payload
   refused"). A rollback's id is `rollback-<uuid>`, never the target's.
4. The checkpoint authority repeats the join on the certificate
   (`internal/platform/checkpoints.go`: `payload.OperationID !=
   certificate.OperationID` -> "verifier decision mismatch"), so the guest
   side cannot simply be loosened: the certificate must carry the operation id
   the payload names.

## Effect

Every rollback that reaches the updater applies, then fails to record: the
operation stays `rollback_pending`, each reconcile spends one of the durable
apply attempts (limit 3), then holds; a crash settlement reads the journal
`applied`, meets the same refusal and closes `degraded` (still open). There is
no exit: `failSelfdevOperation` covers only pre-decision states,
`rollback_pending` and `degraded` are `selfdev.OpenStates` (every later
`Store.Start` is `ErrOperationOpen`), and `plannedRestartBusy` counts
`rollback_pending` (every deploy is refused). The release was physically rolled
back while the checkpoint and route promotion never ran, so the route ledger
diverges from the running release.

Reproduced in the derivable fixture (lane/base-join's
`TestRollbackToAPlainReleaseRecordsTheDroppedBaseClaim` before its shim): the
rollback applies in the updater and the operation sticks in `rollback_pending`
with `materializer: verification payload refused`. No existing test completes a
rollback materialization end to end, which is why it was latent.

The operation-id join in `verificationEvidence` landed in 7619a17b9
("verifier certificate binds the recorded evidence, not operation row"),
before any rollback test existed.

## Fix shape (review 316's smallest correct change; FP0 to confirm)

No schema or tape change. The rollback row carries the target's
`EffectiveHead` as `BaseHead`. In `verificationEvidence`, when the payload's
operation id is not the operation's own:

- resolve the target with `GetByEffectiveHead(computerID, operation.BaseHead)`;
- require it `applied` or `rolled_back`, with `VerifierRefs[0]` equal to the
  rollback's `VerifierRefs[0]` and the payload's `operation_id` equal to the
  target's `OperationID`;
- return that verified operation id, and mint the verifier certificate with
  `OperationID` set to it, so the checkpoint authority's verbatim join holds.

Only a rollback operation (state `rollback_pending`, or `degraded` carrying a
route receipt) may take the target path; an apply keeps the strict self join.

## Tests (before the code)

A real rollback materialization end to end in the derivable fixture, no shim:
two applied plain releases, a rollback to the first through the owner API, the
operation reaches `rolled_back`, the checkpoint authority accepts the
certificate, the route ledger shows the rollback transition. Negative cases: a
rollback whose target's verifier ref or operation id does not match is still
refused; an apply whose payload names another operation is still refused.

## Rollback of the fix

Revert the commit; no data migration. Until it lands, an owner must not start
a rollback on any computer (it wedges that computer's self-development and
deploys as above); staging has none to date.

## Residuals to check while building

- The rollback-note text (the base claim a rollback to a plain release drops)
  is stored in `TerminalError` of the successful `rolled_back` operation; it
  becomes reachable only with this fix.
- Whether the new gate that hung `rollback_pending` should have an owner exit
  (a close act) regardless of this fix: out of scope, named.
