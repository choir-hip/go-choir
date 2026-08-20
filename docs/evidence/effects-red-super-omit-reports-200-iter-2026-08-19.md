# Effects Super omit-reports 200-iter without assign_co_super — 2026-08-19

**Boundary:** diagnose. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch. Super `f515dd0f` already terminal.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Prior continuation:** `docs/evidence/effects-red-super-omit-cancel-reports-2026-08-19.md`
(`9a55b756` deployed, epoch 330, Super `f515dd0f` started with
assign prompt and omitted claimed cancel reports).

## Live observation (2026-08-20T00:34Z)

Staging `/health` `deployed_commit`
`9a55b75636a8104d1033845799fbacc7b68afdf4` (`deployed_at` 23:18:07Z).
Retained computer `computer-03335285269bdba4f94377e56879f9e6` epoch
**330**, `propose_only` generation 1. Guest autoputer `9a55b756`.
Effects remain OFF.

Super `f515dd0f-ae2a-4bf4-9a64-4cbbf9f6ea02` **failed** at 23:40:17Z:

```text
tool loop: exceeded 200 iterations without end_turn
```

Created 23:19:19Z. Prompt is `Prior implementation CoSuper
assignment is terminal. Open a fresh implementation CoSuper
assignment with assign_co_super.` `cosuper_replacement_requested`
true. `cosuper_replacement_omit_reports` true. Nine
`producer_report_ids` claimed. `worker_update_ids` null.
`lifecycle_control_bindings` absent. `self_development_operation_id`
absent. Same Super work `fd43ecca`.

No new CoSuper. Newest Super at 00:34:16Z is still `f515dd0f`
(~54 minutes after fail). No restorm. Operation
`selfdev-ccf0f1ec0e851750f253fe5f5ed97974` remains `executing`,
empty `bundle_digest`, `updated_at` still 17:45:05Z. Pre-A
checkpoint `99949fe2` unchanged. No HTTP operations POST.

This is not the continuation storm. `9a55b756` did its job: one
omit-reports Super started with the assign prompt, skipped cancel
report bodies, claimed the IDs, and did not restorm. The unpaid
behavior is that Super still never called `assign_co_super`.

## Source

Report-continuation Super is `request_source=lifecycle_texture_control`
without bound Texture Control packets. `reconcilePersistentSuperActor`
skips `bindLifecycleControlsToRun` on `reportContinuation`. Super
therefore has an assign prompt and Super tools, but no Texture
`execution_request` naming `operation:selfdev-…`.

The Super that *did* call `assign_co_super` was `f009f383` (17:45:04Z–
17:45:27Z), started from Texture `execution_request` via
`ensureSelfDevelopmentTextureJoin` / `startSelfDevelopmentPersistentSuper`.
That path binds Control packets and stamps
`self_development_operation_id`.

`ensureSelfDevelopmentTextureJoin` already mints
`turn:selfdev-texture-rewake` when the latest persistent Super is
terminal. HTTP `ensureSelfDevelopmentRun` is the only production
caller of that rewake today. `persistSystemCoSuperCancellation`
writes a producer_report and `wakeUpdatedCoagent`; it does not mint
Texture Control. `maybeContinuePersistentSuperInbox` then starts
the report-continuation Super instead of the proven Texture rewake.

Raising `maxToolLoopIterations` only lengthens the same unbound
loop. HTTP Super-start / operations POST would mint the Texture
rewake and remains forbidden as a substitute.

## What this is not

- Not a restorm. Flagged omit-reports Super stayed newest for 54
  minutes after fail.
- Not cancel-report dump (`e4141127`). Those bodies were omitted.
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
fence. Refresh receipt `01a01c52` is a forward record.
