# Effects Super 200-iter without assign_co_super after cancel claim — 2026-08-19

**Boundary:** diagnose. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch.

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Prior storm-stop:** `docs/evidence/effects-red-super-continuation-storm-stopped-2026-08-19.md`
(`3654d925` deployed, epoch 328, Super `bab919a0` claimed cancel
reports and did not restorm).

## Live observation (2026-08-19T21:53Z)

Staging `/health` `deployed_commit`
`3654d9255606cf90f76a213cca1bbe3bba142d35` (`deployed_at` 21:33:35Z).
Retained computer `computer-03335285269bdba4f94377e56879f9e6` epoch
**328**, `propose_only` generation 1. Effects remain OFF.

Super `bab919a0-3e05-4860-bce6-88f040698db9` **failed** at 21:48:33Z:

```text
tool loop: exceeded 200 iterations without end_turn
```

Created 21:36:19Z. Prompt remains `Process pending coagent update
packets for privileged execution.` `request_source=lifecycle_texture_control`.
`requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`.
Nine `producer_report_ids` claimed. `worker_update_ids` null. Same Super
work `fd43ecca`. No new CoSuper in `/api/runs`. Newest Super at
21:53:45Z is still `bab919a0`.

Operation `selfdev-ccf0f1ec0e851750f253fe5f5ed97974` remains
`executing`, empty `bundle_digest`, `updated_at` still 17:45:05Z.
Pre-A checkpoint `99949fe2` unchanged. No HTTP operations POST.

This is not the continuation storm. The storm repair did its job:
terminal claimed Super did not start another Super. The unpaid
behavior is that the claimed Super never called `assign_co_super`,
so capsule authorship has no live CoSuper and no automatic Super
wake left.

## Source

`reconcilePersistentSuperActor` starts report-continuation Super
with the same generic prompt used for Texture control packets
(`internal/agentcore/super_controller.go`). It claims
`producer_report_ids` and does not bind Control packets.

`prependInitialCoagentUpdatePackets` cold-delivers pending
undelivered lifecycle updates into that Super because
`request_source=lifecycle_texture_control`. The cancel reports stay
undelivered by design (injector source). Super therefore sees cancel
text plus "privileged execution" and tool-loops.

Texture rewake (`wakeToken != ""` in
`internal/agentcore/selfdev_texture_join.go`) carries an explicit
note: `Open a fresh implementation CoSuper assignment with
assign_co_super.` Report-continuation Super does not get that note
and is not a Texture `execution_request`. HTTP Super-start would
mint `turn:selfdev-texture-rewake` and is forbidden as a substitute
for automatic continuation.

After `3654d925`, `listPendingPersistentSuperAdmissibleReports`
skips the claimed IDs. `maybeContinuePersistentSuperInbox` after
`bab919a0` therefore reconciles to no Super. Authorship is stuck
until a new producer_report or Texture control appears.

Raising `maxToolLoopIterations` only lengthens the same loop.

## What this is not

- Not a restorm. Claimed Super stayed newest for >5 minutes after
  fail (pre-repair restorm was 1s).
- Not CoSuper capsule `getpgrp` (already repaired).
- Not Super non-rewake (already repaired, then stormed, then
  storm-stopped).

## Forbidden

- freeze / propose / promote
- live send / restore
- HTTP Super-start / operations POST
- raising `maxToolLoopIterations`
- SQL-empty or replace the retained computer
- cancelling Super to force a new assignment

## Rollback

This file is docs-only. Checkpoint `99949fe2` remains the pre-A
fence. Refresh receipt `01a01bf4` is a forward record.
