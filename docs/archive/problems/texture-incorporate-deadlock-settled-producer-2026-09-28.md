# Texture desk incorporate deadlocks when the producer work item already settled

**Status:** DIAGNOSED 2026-09-28 on staging (deployed `92a41612`); fix commit
follows per `docs/memo-problem-documentation-first.md`.

## Symptom

M11 episode probe op `selfdev-4b4d7c57008d47cbbe6ff68dd7dc2077`, computer
`computer-90a799d71f77e941ca8f546dba450cf2` (guest serving `92a41612fc`):
the engineering assignment **completed** at 12:35 with a real report
(`report:sha256:076eca…`). From 12:45 onward the texture desk entered a
~45 s `apply_owner_revision` respawn loop — 33+ consecutive failed runs
(`df2c232b`, `69d2e39e`, `0e8542d1`, `fd695e00`, `573db77f`, `e58f6072`,
`4bd3af7f`, `4f76324a`, `23babf59`, …) all ending
`tool loop: required write tool did not succeed after 2 retries` while
`agent_revision_pending` stayed true and `ownerHeadPending` kept the
reconcile path armed.

## Root cause

Run `df2c232b` event tape, iteration 2: the desk emitted a correct cell —

```
choir.ApplyTexture({op:"apply", base_revision_id:"03a50110…",
  work_disposition:"open", content:"# M11_SELFDEV_EPISODE_…",
  update_dispositions:[{update_id:"assignment-report:report:sha256:076eca…",
    disposition:"incorporated"}]})
```

— and the reducer rejected it with
`apply atomic Texture lifecycle turn: lifecycle invalid transition`.

Guard at `internal/store/texture_turn.go` requires the **producer's** work
item to still be `open` when the desk incorporates its report
(`work.Status != types.WorkItemOpen → ErrLifecycleInvalidTransition`). The
engineering run settled its own work item at completion (12:35), so every
incorporate of a completed-producer report is deterministically invalid.
The pending update can never be dispositioned, the owner-revision mutation
stays pending, and every respawned desk run hits the same wall. `decide`
turns cannot escape either: the inbound pending update must still be
dispositioned in the same atomic turn.

## Why it is a substrate bug

Two lifecycle surfaces disagree about ownership of the producer work
item's terminal transition: the assignment runtime settles it at run
completion; `ApplyTextureTurn` assumes it stays open until the desk's
disposition. Either settlement order is individually valid — they must
compose. The guard conflates "work item must be open to be settled by this
turn" with "update cannot be incorporated unless work is still open."

## Fix shape (separate commit)

`texture_turn.go`: when the inbound disposition's producer work item is
already terminal, verify the trajectory/assignment binding fields and
accept `work_disposition` values of `open` or the work item's current
terminal status (idempotent — work already settled); skip re-settling.
Mismatched terminal dispositions (e.g. `completed` disposition against
`refused` work) still fail invalid-transition.

## Residuals

- Tool-loop behavior under deterministic reducer rejections remains a
  separate defect: the required-write retry re-invokes the same call under
  the same `call_id` and the model on retry often emits read probes
  (`ReadDoc`/`Updates`, `intents:0`) instead of a write, wasting the retry
  window. With the deadlock fixed this stops mattering for the M11 path,
  but any future deterministic rejection will still burn the run.
- `package_placeholder` and missing `import "fmt"` cell slips seen in op6
  turns — prompt hygiene, not blocking.
