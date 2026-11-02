# Stale open self-development operations block all engineering on a computer (2026-11-02)

Mutation class of this doc: green. Evidence read-only (CLI self-dev list/head, node-b console.log, code).

## Problem
Any self-development operation in an OpenStates state (internal/selfdev/operations.go:49) holds every
engineering admission on its computer: `openOperationHold` (internal/agentcore/carried_patch.go:418) feeds
`operation_open` (internal/agentcore/engineering_admission.go:197-205), retried every 60 s with no cap. The cause is
visible only in the console. No owner act can close an operation that will not settle.

## Evidence
- d@e (computer-59c902c5...): `rollback-4dca8039...` degraded since 2026-10-27 (updated 2026-10-30), target = applied
  `selfdev-4585da37...` (release 63a13ae9, gen 1). Head: desired fcddd72b (the rollback event) != effective ce1737e7,
  pending empty. Console (node-b vm-70070cf7.../console.log): 106 lines of
  "rollback-4dca... in degraded needs retry: rollback release unavailable: updater: pinned release unavailable", one
  per 60 s retry wake (not several per second).
- b@c: a 6-day-old awaiting_approval operation blocked engineering until the orchestrator rejected it.
- Pinned release availability inside the guest (`<updater root>/releases/63a13ae9...`) is not readable from the host
  (guest data.img). Host guest-releases/ holds VM images, not the updater store. The updater itself never deletes
  releases (no RemoveAll of releases in internal/updater/updater.go), so "retention removed it" is a hypothesis.

## Why no existing act settles d@e
- A degraded rollback leaves degraded only via `rollbackSelfDevelopmentOperation`
  (internal/agentcore/self_development_materializer.go:789), whose first step is `ReadPinnedManifest`
  (internal/updater/updater.go:822-830). It fails, so the operation can never reach rolled_back/failed.
- `choir self-dev rollback --to-operation X` (cmd/choir/selfdev.go) creates a NEW rollback operation
  (selfdev.StartRollback, operations.go:393); it does not touch 4dca. 4dca stays open, so engineering stays blocked.
  A rollback to any applied op may even succeed (if that release is retained) and still leave the hold.
- The "manual re-bind" named in the cause string (self_development_materializer.go:271) does not exist as a command
  or route; grep finds only that string and a comment at :966.

## Cluster (3 symptoms, one substrate)
awaiting_approval for days (b@c), degraded with unavailable release (d@e), rebase blocked behind a lock
(docs/problems/selfdev-blocked-behind-unperformed-rebase-2026-10-27.md): an open operation with no terminal path
that the owner or management can invoke, plus an unbounded hold whose cause is not on the ledger.
The state machine already allows the closing edges: awaiting_approval->failed, accepted->failed, degraded->failed
(operations.go:675-698), but only the materializer writes them, and only from journaled outcomes.

## Recommendation
1. Smallest fix for d@e: in the materializer, a degraded/rollback_pending rollback whose
   `ReadPinnedManifest` fails closes `degraded -> failed` (existing edge) with TerminalError "pinned release unavailable;
   computer stays on its current release", at self_development_materializer.go:789. Risk: head desired(fcddd) !=
   effective(ce17) stays; later ops bind to the live head so this is tolerated, but verify on d@e.
2. Common substrate fix: an owner act `self-dev abandon --operation --reason` (route beside rollbacks at
   api_self_development.go:204; CLI beside cmd/choir/commands.go:646, scope owner:acts) that moves any open,
   non-materializing operation to `failed` with the reason recorded on the tape (as recordMaterializationFailed does).
3. Surface the hold: after N retries in deferEngineeringAdmission (engineering_admission.go ~:213) write the
   blocker to the management/Texture commitment ledger, so it is not console-only.
Do 1 now; 2 and 3 are the class fix. Retention never deleting a pinned release is not shown to be the cause.

## Journal-first settlement (round 3 design, revised in round 4, 2026-11-02)
Review rounds 1-2 found three wedges in the settle paths themselves (discharge before the row CAS; "no journal" read
as "nothing in flight"; the rollback re-drive reading the pinned manifest before the journal). Round 3 fixed those with
journal phases; round 4 found the phase-based cut still left holds (a dead journal past its swap whose bundle or pinned
release was GC'd; an apply whose attempts are spent; a corrupt journal). The step back: one rule, grounded in the
updater's `current` pointer, which is what is installed. No apply input is needed.

The rule (updater.HoldApplyKey, agentcore settleSelfDevelopmentOperation / failSettledRow / dischargeSettledPending):
1. Hold the apply key: take the per-key lock without waiting. `Apply` holds it from before its journal read to its
   return. Held by another: **in flight**, refuse (409) and, on a crash boot, arm a retry wake.
2. A terminal journal is the authority: `applied` settles forward from its signed receipts; any other outcome settles
   failed with its result.
3. A **dead** journal (lock free, and missing, non-terminal or corrupt) is arbitrated on `current`:
   - no journal, or `current` == its prior release: nothing installed -> failed;
   - `current` == its target: installed -> forward (applied / rolled_back), the note says "recorded from the current
     pointer", the event payload says `arbitrated_on_current` (there are no receipts);
   - neither (or corrupt journal, unreadable pointer): failed, with "the effective release is unknown: ..." in the cause.
     materialization_failed is the reducer's only failed discharge; its payload says `unknown_effective`.
4. An older updater daemon (no `operations/.key-lock-v1` marker, written by every updater of this build at start) takes
   no lock, so a free lock proves nothing: a missing or non-terminal journal is **lock_unproven**. The owner's abandon
   refuses it (409, saying the daemon predates the lock); the materializer's own paths keep their pre-lock behavior.
   Terminal journals need no lock, so d@e's case (terminal applied rollback) settles under either daemon. The daemon
   ships in the base image (`goChoirPackages.updater`); the guest core runs from the same base image unless a layered
   release execs its own binary, which is bound to the base it was built on, so the two meet only in that layered case
   or a base-image rollback that leaves the marker behind.
5. Order, under selfdevMaterializeMu: failed = row CAS, then discharge pending (only this operation's decision), then
   fence; forward = applied event (if pending is still this decision), row CAS, fence. A lost CAS changes nothing,
   journal included. The fence is a terminal `refused` journal written under the held lock, so no queued or resumed
   `Apply` acts on a settled key.
6. Pending never stays bound to a settled operation: the reconciler's sweep applies the same rule to a pending decision
   of a failed row (forward when its journal is applied or its target current, failed otherwise).

Table (open state x verdict -> terminal path). Every verdict but in flight / lock_unproven is terminal.
| state | verdict | pending | terminal path |
|---|---|---|---|
| requested..verified | (no decision, no key) | not theirs | abandon: failed; crash boot closes them |
| awaiting_approval, no decision on tape | no key | empty | abandon: failed; a late decide's pending -> sweep |
| awaiting_approval, decision on tape | - | the decision | materializer recovers it (abandon 409) |
| accepted | nothing installed | own | abandon / materializer: failed, discharge, fence |
| any post-decision | in flight | own | 409; the updater finishes; crash-held ops arm a retry wake |
| any post-decision | lock_unproven | own | abandon 409 (old daemon); materializer as before the lock |
| materializing / rollback_pending / degraded | terminal applied | own or empty | materializer: event + tail (rollback: journal first, tail over the installed release, forward without it when that release is unusable); abandon: forward without the tail |
| " | terminal other | own | failed + discharge with the journal's result |
| " | dead, current = prior (or no journal) | own | failed + discharge + fence (abandon; pinned release unusable; crash boot) |
| " | dead, current = target | own or empty | forward, arbitrated (abandon; pinned release unusable; crash boot). Attempts spent / bundle or pinned release GC'd: abandon applies this rule |
| " | dead, current = neither / corrupt | own | failed, unknown effective named, + discharge + fence |
| failed (orphan) | any but in flight | its decision | sweep: forward if applied/installed, else discharge failed |

Removed in round 4: the 202 re-admit of a crash-held apply (the rule closes it instead, so there is no owner act to
record). Out of scope, stated: a pending bound to a rollback_requested event whose StartRollback never landed (the
owner's replay of that rollback start finishes it); a materializer-settled operation whose settlement event append
failed keeps the row's projection on the tape but not the recovery_recorded event; per-key `.lock` files are kept
(removing one under a waiter would split the lock); a settle-forward row has no checkpoint or route receipt, so a later
rollback *to* it is refused at start (a clean 409), and the source projection follows the next apply, not this one.
