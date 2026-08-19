# Effects Super assign-prompt max_tokens without assign_co_super — 2026-08-19

**Boundary:** diagnose. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Prior continuation:** `docs/evidence/effects-red-super-assign-prompt-continuation-2026-08-19.md`
(`5e01ac3a` deployed, epoch 329, Super `e4141127` started with
assign prompt and did not restorm).

## Live observation (2026-08-19T22:42Z)

Staging `/health` `deployed_commit`
`5e01ac3a5ab8d699cc65eed1ebde6b66bc08e545` (`deployed_at` 22:34:00Z).
Retained computer `computer-03335285269bdba4f94377e56879f9e6` epoch
**329**, `propose_only` generation 1. Effects remain OFF.

Super `e4141127-26aa-44d2-b08a-5a1995a0e2df` **failed** at 22:38:16Z:

```text
tool loop: model stopped at max_tokens after 3 continuation attempts (iteration 7)
```

Created 22:35:29Z. Prompt is `Prior implementation CoSuper
assignment is terminal. Open a fresh implementation CoSuper
assignment with assign_co_super.` `cosuper_replacement_requested`
true. Nine `producer_report_ids` claimed. `worker_update_ids` null.
Same Super work `fd43ecca`. No new CoSuper in `/api/runs`. Newest
Super at 22:42:10Z is still `e4141127`.

Operation `selfdev-ccf0f1ec0e851750f253fe5f5ed97974` remains
`executing`, empty `bundle_digest`, `updated_at` still 17:45:05Z.
Pre-A checkpoint `99949fe2` unchanged. No HTTP operations POST.

This is not the continuation storm. `5e01ac3a` did its job: one
replacement Super started with the assign prompt, claimed the
cancel reports, and did not restorm. The unpaid behavior is that
the replacement Super still never called `assign_co_super`.

## Source

`prependInitialCoagentUpdatePackets` still cold-delivers pending
undelivered lifecycle updates into a Super with
`request_source=lifecycle_texture_control`. Cancel reports stay
undelivered by design (injector source). Super therefore sees nine
cancel-report bodies plus the assign prompt.

`internal/toolregistry/toolloop.go` `maxTokenContinuationRetries=3`.
The model stopped at `max_tokens` with text three times and never
emitted `assign_co_super`. Super has that tool via
`RegisterAssignedCoSuperTools`.

Texture rewake (`wakeToken != ""` in
`internal/agentcore/selfdev_texture_join.go`) still carries the
same assign note as a Texture `execution_request`. Report
continuation is still not a Texture control. HTTP Super-start
would mint `turn:selfdev-texture-rewake` and remains forbidden.

After `5e01ac3a`, `listPendingPersistentSuperAdmissibleReports`
skips IDs claimed by a Super that requested replacement.
`maybeContinuePersistentSuperInbox` after `e4141127` therefore
reconciles to no Super. Authorship is stuck until a new
producer_report or Texture control appears.

Raising `maxToolLoopIterations` does not address max_tokens.
Raising `maxTokenContinuationRetries` only lengthens the same
prose dump.

## What this is not

- Not a restorm. Flagged Super stayed newest for >3 minutes after
  fail (pre-repair restorm was 1s).
- Not Super 200-iter looping (this failed at iteration 7).
- Not CoSuper capsule `getpgrp` (already repaired).

## Forbidden

- freeze / propose / promote
- live send / restore
- HTTP Super-start / operations POST
- raising `maxToolLoopIterations` or `maxTokenContinuationRetries`
- SQL-empty or replace the retained computer
- cancelling Super to force a new assignment

## Rollback

This file is docs-only. Checkpoint `99949fe2` remains the pre-A
fence. Refresh receipt `01a01c2a` is a forward record.
