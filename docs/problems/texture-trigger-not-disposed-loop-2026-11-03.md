# Problem 17: a Texture activation that disposes neither trigger loops a model call every few seconds (2026-11-03)

Part of the self-development round blockers cluster
([selfdev-round-blockers-cluster-2026-11-03.md](selfdev-round-blockers-cluster-2026-11-03.md)).
First recorded in the arm-rounds problem doc as problem 17 (local commit f437e9860).

## Evidence

Deepseek arm, computer-e80d229b0d78c35d47656b8c66426a43 (vm-ab6bcd58), document ddbdd1ae, trajectory 8048fb68.
Guest console on Node B (`console.log.1`, `console.log`) and a trace export of 8048fb68 taken 11:42:43Z.

- 11:02:07Z planned restart for the self-development apply selfdev-152fe036. It is the first layered release on
  this computer: the console shows `layering release /mnt/persistent/choir-updater/store/a967jdk2...-autoputer-0.1.0/bin/autoputer over private store`
  (and again at the 11:40 boot). The earlier apply (selfdev-574077b3, 06:12Z) was a plain release and exec'd the base.
- 11:02:16Z onward, every `desk_go_eval` on this computer fails:
  `desk_go_eval: spawn worker: start desk session worker: fork/exec /mnt/persistent/choir-updater/store/a967jdk2bcvxlmyjf0lwnbaybixpb656-autoputer-0.1.0/bin/desk-worker: permission denied`.
  158 such tool results in the trace (runs 28388731, ac3bf1ad, 1996fb51), the first at 11:02:16Z and the last at
  11:40:15Z. The last successful desk cell in the trajectory is 06:11:34Z. The Texture model says so in prose:
  "Thirty-third attempt, same error... Host fix needed: exec bit on bin/desk-worker".
- 11:10:02Z hibernate fence; 11:19:39Z resume, clock stepped 576108 ms.
- 11:20:59Z owner revision 021721b8 (round 6) opens Texture work d1d6bc53. Two actor updates reach
  `texture:ddbdd1ae`: 15190521 (kind `owner_revision`, from the store's actor-wake outbox row) and bf854235 (kind
  `coagent_result`, from `dispatchTextureRevisionWake` in the revise path).
- From 11:21:40Z the two alternate: each delivery runs the run again ("tool loop: iteration 1, executed 1 tools,
  continuing"), the turn ends parked, and the handler defers with "Texture activation returned without disposing
  exact trigger", discarding one emitted update, the same id `010abbdd...` for both occurrences on every try.
  Deferral counts reached 24 per occurrence by 11:33:24Z; the backoff grew to the 30 s cap, so the pair kept one
  model call about every 15-35 s between them.
- 11:33:54Z nudge revision: a new pair (238c6662 `owner_revision`, b87ba80a `coagent_result`) starts the same loop
  at deferral 1, and the old pair goes terminal (head moved).
- After the owner restart at 11:40:37Z (layered release still current), a new document 799ed6d2 (trajectory
  6be571d8, revision 11:43:35Z) loops the same way with one occurrence (e979af8f), deferral 14 by 11:49:40Z. The arm
  was stopped at about 11:52Z.

## Findings and hypotheses

- FINDING (trace, code): the Texture turn cannot dispose its trigger because no desk cell can run. A layered
  release execs the daemon from the updater's private store, and `deskWorkerBinaryFor` puts the desk worker beside
  `os.Executable()`, under `/mnt/persistent/choir-updater` (`d ... 0700 root root`, nix/autoputer-vm.nix). The
  parent stats the file as root and finds it executable. The child setuids to the cell uid
  (`planCellConfinement`) before `execve`, and the cell uid cannot search the 0700 directory, so exec returns
  EACCES. With no cell, no Write, Cast, Wait, Done or Reject commits, so no `texture_turn_committed` event
  consumes the head and no producer report is disposed. The release's store is overlaid on `/nix/store` in the
  daemon's mount namespace, so the same file is at `/nix/store/<name>/bin/desk-worker`, a path the cell uid can
  traverse.
- FINDING (code): the "two occurrences" are one trigger delivered twice, not one per revision. A user revision
  writes an `owner_revision` outbox row in its commit (store `actorWakeOutboxFromObject`), and the revise path also
  dispatches a `coagent_result` carrying the encoded document-revision occurrence. Kinds and contents differ, so the
  actor log does not dedupe them. Normally the first turn disposes the head and the second goes terminal. When the
  turn does not dispose, both run the model.
- FINDING (code): a deferral re-executes the model. `handleCoagentResult` reconciles and runs
  `ExecuteActivationSyncChecked` on every delivery, so a deferral that waits for "the next wake" re-runs the same
  parked run with the same context every time it comes due. The only bound was `MaxDeferrals` (64), about 64 model
  calls per occurrence, and every new revision started a fresh 64.
- FINDING (code): the discarded emitted update is the run's `activation_budget_deadline` wake (content is the run
  id, so the id is the same on every try and for both occurrences, `ExecuteActivationSync`). Discarding it is harmless
  for a run that has already parked; it is evidence that the model was called again, not a lost obligation.
- FINDING: the hibernate resume and its 9.6-minute clock step are not causal. The desk-worker failure began at
  11:02:16Z, before the hibernate, and the post-restart document 799ed6d2 loops with no resume in between. Deferral
  backoff compares wall clock with wall clock (`time.Now()` against the stored `not_before`), and the deferral
  timestamps step up normally across the run.
- FINDING: the model is not the cause. Run metadata shows pool:default served
  `synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017` for this Texture, and the model did call `desk_go_eval` on
  every iteration ("iteration 1, executed 1 tools"). Every call returned the exec error.
- HYPOTHESIS: any computer running a layered self-development release has no working desk cells (Texture,
  management, engineering cells). Only this arm ran a layered release on Node B at 11:50Z.

## Failure modes, written before the fix

The fix has three parts: run the desk worker from a path the cell uid can execute, give one trigger one model turn
however many times it is delivered, and close an undisposed trigger with a visible fate.

Desk worker path:
1. The daemon runs from `$CHOIR_UPDATER_ROOT/store/<name>/bin/autoputer`, and the desk worker beside it is
   unreachable by the cell uid. Guard: map it to `/nix/store/<name>/...` when that path is an executable regular
   file.
2. The overlay is not mounted (the boot script's direct-exec fallback). Then `/nix/store/<name>` does not exist.
   Guard: keep the old path; cells still fail, and the loop bound below closes the trigger visibly.
3. A path outside the updater store, or a non-layered release. Guard: leave it unchanged.
4. A crafted name (`..`, an empty name, the store's `.staging-` dirs) maps outside `/nix/store`. Guard: map only
   a clean single component that does not start with a dot.

Trigger loop:
5. One trigger arrives as two actor updates (`owner_revision` and `coagent_result`). Guard: key the
   "already had its turn" record on the canonical trigger (document revision: scope, document and head revision;
   producer report: scope, producer and producer update), not on the actor update id.
6. A delivery after an undisposed turn runs the model again. Guard: once a trigger's turn has ended undisposed
   (the run parked idle in that activation: `actor_sleep_state` "idle", stamped after the activation began), later
   deliveries only re-check the trigger and never execute the run.
7. Another turn disposes the trigger (a sibling report's turn consumes the head, or the owner writes a newer
   revision). Guard: the re-check runs after `ResolveTextureActorOccurrence`, whose terminal answer consumes the
   occurrence and forgets the record.
8. Nothing ever disposes it. Guard: once the trigger has been re-checked `textureUndisposedRecheckLimit` (6) times
   and at least `textureUndisposedMinWait` (30 s) has passed since its turn (the wait does not shrink when the two
   updates share the count), it is closed with the reason "trigger not disposed: ..." and the occurrence is consumed
   as durably invalid (`delivery_invalid` to the sink). An owner revision: its Texture work is refused (a
   `work_refused` event on the trajectory's ledger, rendered by Texture), and that refusal, unlike a desk's own,
   reopens work on the owner's next revise. A producer report: the report is settled with the reason (an
   `update_late` event), so it is not left pending for a later turn to consume unread, and Texture's work stays open
   for the owner (review round 1: refusing the work for a report could refuse work a newer revision needs).
   A tombstone keeps the trigger closed, so its other update is consumed with no work and no model call.
9. The refusal fails (store error or version race). Guard: defer and retry the refusal on the next delivery,
   still without a model call; the dispatcher's `MaxDeferrals` bounds that.
10. A restart. Crash: pre-boot work is interrupted at boot as before. Planned restart: the in-memory record is lost,
    so a trigger gets at most one more model turn per planned restart, never a loop.
11. A turn that ended with the run terminal and the trigger pending is consumed as before, and is recorded, so the
    trigger's other update re-checks instead of running the model. A provider or execute error, or a park for a runtime failure (`runtime_injection_append_failed`: the trigger
    never became model-visible), so no turn was spent. Unchanged: that deferral retries the execution under the
    existing `MaxDeferrals` bound, because no model turn was spent.
12. The record grows without bound. Guard: forget on terminal or disposal, tombstone on fate, and prune every entry
    older than an hour on each lookup.
